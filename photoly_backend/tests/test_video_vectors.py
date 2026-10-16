"""Real FFmpeg decoding and isolated PostgreSQL media-level retrieval contracts."""
import hashlib
import io
import json
import subprocess
import time
from pathlib import Path

import numpy as np
import pytest
from PIL import Image
from sqlalchemy import select, func, delete, text
from sqlalchemy.exc import IntegrityError
from app.entities import Photo
from app.entities.ai import PhotoEmbeddingState as State, VideoFrameEmbedding as Frame, EmbeddingProfile, AIJob
from app.ai import video
from app.ai.hooks import enqueue_photo, photo_changed
from app.ai.snapshot import snapshot_photo
from app.ai.state import control, settings, utcnow
from app.ai.types import AIError
from app.ai.vector_store_pgvector import PgVectorStore
from app.scheduler import job_repository as jobs
from app.scheduler.photo_embedding_scheduler import PhotoEmbeddingScheduler
from app.scheduler.maintenance import Maintenance
from tests.test_ai import ai, ready
from tests.openapi_assertions import assert_documented_response


def ffmpeg(*args):
    subprocess.run(['ffmpeg', '-v', 'error', '-nostdin', '-y', *map(str,args)], check=True, timeout=20)


@pytest.fixture
def clip(tmp_path):
    path=tmp_path/'clip.mp4'
    ffmpeg('-f','lavfi','-i','testsrc2=s=320x180:r=10:d=2','-c:v','libx264','-threads','1',path)
    return path


def vec(i=0, dimension=768):
    vector=np.zeros(dimension, dtype=np.float32);vector[i]=1
    return vector


def video_ready(s, pid, photo_id=2, vectors=None):
    vectors = vectors if vectors is not None else [vec(1),vec(1),vec(),vec(1),vec(1)]
    photo=s.get(Photo,photo_id)
    state=State(photo_id=photo_id,profile_id=pid,content_version=photo.content_version,status='ready',
                sampling_version=video.SAMPLING_VERSION,frame_count=len(vectors))
    s.add(state);s.flush()
    s.add_all([Frame(state_id=state.state_id,profile_id=pid,frame_index=i,timestamp_ms=i*500,embedding=v)
               for i,v in enumerate(vectors)])
    return state


def install_clip(ai, clip):
    storage=ai.app.extensions['photoly'].storage
    with ai.sessions() as s:
        path=storage.media('user-alice',s.get(Photo,2))
    path.write_bytes(clip.read_bytes())


def test_decode_five_rgb_frames_and_streamed_snapshot(clip, monkeypatch, tmp_path):
    monkeypatch.setattr(video.tempfile, 'tempdir', str(tmp_path))
    class Bounded(io.BytesIO):
        def read(self, size=-1):
            assert 0 < size <= 1024*1024
            return super().read(size)
    result=video.snapshot_video(Bounded(clip.read_bytes()))
    assert result['hash']==hashlib.sha256(clip.read_bytes()).hexdigest()
    assert [f['timestamp_ms'] for f in result['frames']]==[100,500,1000,1500,1900]
    for frame in result['frames']:
        with Image.open(io.BytesIO(frame['data'])) as image:
            assert image.mode=='RGB' and image.size==(320,180)
    assert not list(tmp_path.glob('photoly-*'))


@pytest.mark.parametrize('count',[1,2,4])
def test_short_video_decodes_actual_frame_count(tmp_path,count):
    path=tmp_path/'short.mp4'
    ffmpeg('-f','lavfi','-i','testsrc2=s=64x48:r=30','-frames:v',count,'-c:v','libx264','-threads','1',path)
    frames=video.extract_frames(path)
    assert len(frames)==count
    assert frames[0]['timestamp_ms']==0
    assert len(set(f['timestamp_ms'] for f in frames))==count


def test_rotation_resize_and_vfr(tmp_path,clip):
    rotated=tmp_path/'rotated.mp4'
    ffmpeg('-display_rotation:v:0','90','-i',clip,'-c','copy',rotated)
    assert Image.open(io.BytesIO(video.extract_frames(rotated)[0]['data'])).size==(180,320)
    big=tmp_path/'big.mp4'
    ffmpeg('-f','lavfi','-i','color=red:s=1600x800:r=10:d=1','-c:v','libx264','-threads','1',big)
    assert Image.open(io.BytesIO(video.extract_frames(big)[0]['data'])).size==(1280,640)
    vfr=tmp_path/'vfr.mp4'
    ffmpeg('-i',clip,'-vf',"select='if(lt(n,10),not(mod(n,3)),1)'",'-vsync','vfr','-c:v','libx264','-threads','1',vfr)
    frames=video.extract_frames(vfr)
    assert 1<=len(frames)<=5 and frames[2]['timestamp_ms']==1000


def test_corrupt_no_video_timeout_limits_and_cleanup(tmp_path,clip,monkeypatch):
    with pytest.raises(AIError,match='invalid_video'):
        video.snapshot_video(io.BytesIO(b'not a movie'))
    audio=tmp_path/'audio.mp4'
    ffmpeg('-f','lavfi','-i','sine=frequency=1000:duration=0.2','-c:a','aac',audio)
    with pytest.raises(AIError,match='video_stream_missing'):video.extract_frames(audio)
    monkeypatch.setattr(video.tempfile,'tempdir',str(tmp_path))
    with pytest.raises(AIError,match='content_hash_mismatch'):
        video.snapshot_video(io.BytesIO(clip.read_bytes()),expected='wrong')
    monkeypatch.setattr(video,'max_video_bytes',lambda:1)
    with pytest.raises(AIError,match='video_too_large'):video.snapshot_video(io.BytesIO(b'xx'))
    assert not list(tmp_path.glob('photoly-*'))
    with pytest.raises(AIError,match='video_decode_timeout'):video.extract_frames(clip,deadline=time.monotonic()-1)
    def timeout(*args,**kwargs):raise subprocess.TimeoutExpired(args[0],kwargs['timeout'])
    monkeypatch.setattr(video.subprocess,'run',timeout)
    with pytest.raises(AIError,match='video_decode_timeout'):video.extract_frames(clip)


def test_middle_frame_matches_text_image_and_video_dedup_stable_pages(ai):
    with ai.sessions.begin() as s:
        ready(s,ai.ai_pid)
        video_ready(s,ai.ai_pid)
        p=Photo(photo_name='second video',user_id=2,ns_id=2,format='mp4');s.add(p);s.flush()
        other=p.photo_id;video_ready(s,ai.ai_pid,other,[vec(),vec(),vec()])
        foreign=Photo(photo_name='private',user_id=3,ns_id=4,format='mp4');s.add(foreign);s.flush()
        video_ready(s,ai.ai_pid,foreign.photo_id)
    response=ai.client.post('/ai/search/text',json={'text':'middle scene'},headers=ai.headers)
    assert_documented_response('POST','/ai/search/text',response)
    hits=response.json['t']['items']
    assert [h['photo']['photoId'] for h in hits]==[1,2,other]
    assert hits[1]['matchedTimestampMs']==1000
    response=ai.client.post('/ai/search/image',data={'file':(io.BytesIO(ai.jpeg),'image.jpg')},headers=ai.headers)
    assert [h['photo']['photoId'] for h in response.json['t']['items']]==[1,2,other]
    ai.runtime.after_encode=lambda:pytest.fail('SigLIP reuses stored frame vectors')
    first=ai.client.post('/ai/photos/2/similar/page',json={'pageSize':1},headers=ai.headers).json['t']
    assert first['items'][0]['photo']['photoId']==1 and first['hasMore']
    second=ai.client.post('/ai/photos/2/similar/page',json={'cursor':first['nextCursor']},headers=ai.headers).json['t']
    assert [h['photo']['photoId'] for h in second['items']]==[other] and not second['hasMore']
    with ai.sessions() as s:
        profile=s.get(EmbeddingProfile,ai.ai_pid)
        hits=PgVectorStore().search(s,2,profile,[vec(3),vec()],min_score=.99)
        assert [h['photo'].photo_id for h in hits]==[1,2,other]
        assert PgVectorStore().search(s,2,profile,vec(3),min_score=.1)==[]
        assert [h['photo'].photo_id for h in PgVectorStore().search(s,2,profile,vec(),gallery_id=2)]==[1]


def test_video_atomic_index_quota_overwrite_and_delete(ai,clip):
    install_clip(ai,clip)
    with ai.sessions.begin() as s:enqueue_photo(s,s.get(Photo,2),ai.ai_pid)
    with ai.sessions.begin() as s:
        task=jobs.claim(s,'worker',profile_id=ai.ai_pid)
        assert jobs.usage(s)['reserved_bytes']==5*jobs.vector_charge(768)
    with ai.sessions() as s:snapshot=snapshot_photo(s,ai.app.extensions['photoly'].storage,2)
    with ai.sessions.begin() as s:
        assert jobs.commit_embedding(s,task,snapshot,[vec()]*5,20)
    with ai.sessions.begin() as s:
        state=s.scalar(select(State).where(State.photo_id==2))
        assert state.status=='ready' and state.embedding is None and state.frame_count==5
        assert jobs.usage(s)['estimated_bytes']==5*jobs.vector_charge(768)
        assert jobs.usage(s)['reserved_bytes']==0
        photo=s.get(Photo,2);photo.content_version+=1;photo_changed(s,photo)
    with ai.sessions.begin() as s:
        assert s.scalar(select(func.count()).select_from(Frame))==0
        assert not jobs.commit_embedding(s,task,snapshot,[vec()]*5,20)
        video_state=s.scalar(select(State).where(State.photo_id==2));s.delete(video_state);s.flush()
        video_ready(s,ai.ai_pid)
    with ai.sessions.begin() as s:s.delete(s.get(Photo,2))
    with ai.sessions() as s:assert s.scalar(select(func.count()).select_from(Frame))==0


def test_database_rejects_partial_frames_and_wrong_dimension(ai):
    with pytest.raises(IntegrityError):
        with ai.sessions.begin() as s:
            s.add(State(photo_id=2,profile_id=ai.ai_pid,content_version=1,status='ready',sampling_version=1,frame_count=5))
    with pytest.raises(IntegrityError):
        with ai.sessions.begin() as s:video_ready(s,ai.ai_pid,vectors=[[1,2]])
    with ai.sessions.begin() as s:video_ready(s,ai.ai_pid)
    with pytest.raises(IntegrityError):
        with ai.sessions.begin() as s:s.execute(delete(Frame).where(Frame.frame_index==2))


def test_old_video_backfill_once_failures_do_not_loop(ai):
    with ai.sessions.begin() as s:
        ready(s,ai.ai_pid)
        s.add(State(photo_id=2,profile_id=ai.ai_pid,content_version=1,status='unsupported'))
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions['photoly'].storage)
    scheduler.tick() # Fixture video is corrupt: explicit error, no successful frames.
    with ai.sessions() as s:
        state=s.scalar(select(State).where(State.photo_id==2))
        assert state.sampling_version==1 and state.status=='unsupported' and state.error_code=='invalid_video'
        image=s.scalar(select(State).where(State.photo_id==1));assert image.status=='ready'
    scheduler.user_cursor=0;scheduler.tick()
    with ai.sessions() as s:
        assert s.scalar(select(func.count()).select_from(AIJob).where(AIJob.kind=='embedding'))==1


def test_sequential_inference_failure_commits_no_partial_frames(ai,clip):
    install_clip(ai,clip)
    with ai.sessions.begin() as s:enqueue_photo(s,s.get(Photo,2),ai.ai_pid)
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions['photoly'].storage)
    calls=[]
    def fail():
        calls.append(1)
        if len(calls)==3:raise AIError('invalid_embedding',502)
    ai.runtime.after_encode=fail
    scheduler.tick()
    with ai.sessions() as s:
        assert len(calls)==3
        assert s.scalar(select(func.count()).select_from(Frame))==0
        job=s.scalar(select(AIJob).where(AIJob.photo_id==2));assert job.attempts==1 and job.status=='queued'
    ai.runtime.after_encode=None
    with ai.sessions.begin() as s:s.scalar(select(AIJob).where(AIJob.photo_id==2)).next_run_at=utcnow()
    scheduler.tick() # May claim image discovered by reconciliation first.
    scheduler.tick()
    with ai.sessions() as s:assert s.scalar(select(State.frame_count).where(State.photo_id==2))==5


def test_timeout_is_bounded_retry_and_cancel_between_frames(ai,clip,monkeypatch):
    install_clip(ai,clip)
    scheduler=PhotoEmbeddingScheduler(ai.sessions,ai.runtime,ai.app.extensions['photoly'].storage)
    with ai.sessions.begin() as s:
        ready(s,ai.ai_pid);enqueue_photo(s,s.get(Photo,2),ai.ai_pid)
    original=video.extract_frames
    def timeout(*args,**kwargs):raise AIError('video_decode_timeout',504)
    monkeypatch.setattr(video,'extract_frames',timeout)
    for _ in range(5):
        with ai.sessions.begin() as s:s.scalar(select(AIJob).where(AIJob.photo_id==2)).next_run_at=utcnow()
        scheduler.tick()
    with ai.sessions() as s:
        job=s.scalar(select(AIJob).where(AIJob.photo_id==2));assert job.attempts==5 and job.status=='failed'
    scheduler.user_cursor=0;scheduler.tick()
    with ai.sessions() as s:assert s.scalar(select(func.count()).select_from(AIJob).where(AIJob.photo_id==2))==1
    monkeypatch.setattr(video,'extract_frames',original)
    with ai.sessions.begin() as s:enqueue_photo(s,s.get(Photo,2),ai.ai_pid)
    calls=[]
    def cancel():
        calls.append(1)
        with ai.sessions.begin() as s:
            photo=s.get(Photo,2);photo.content_version+=1;photo_changed(s,photo)
    ai.runtime.after_encode=cancel;scheduler.tick()
    assert len(calls)==1
    with ai.sessions() as s:assert s.scalar(select(func.count()).select_from(Frame))==0


def test_qwen_video_query_reencodes_once_and_cursor_holds_all_vectors(ai,clip):
    from app.services.ai_admin import AIAdmin
    install_clip(ai,clip)
    with ai.sessions.begin() as s:
        pid=AIAdmin(s).create_profile('qwen3_vl')['profileId']
        profile=s.get(EmbeddingProfile,pid);profile.index_status='ready';ai.runtime.spec=dict(profile.spec)
        row=control(s);row.desired={**settings(s),'search_profile_id':pid,'loaded_profile_id':pid}
        ready(s,pid,vector=vec(dimension=2048))
        video_ready(s,pid,vectors=[vec(dimension=2048)]*5)
        photo=Photo(photo_name='other',user_id=2,ns_id=2,format='mp4');s.add(photo);s.flush()
        video_ready(s,pid,photo.photo_id,[vec(dimension=2048)]*3)
    calls=[]
    ai.runtime.after_encode=lambda:calls.append(1)
    response=ai.client.post('/ai/photos/2/similar/page',json={'pageSize':1},headers=ai.headers)
    assert response.status_code==200 and len(calls)==5
    cursor=response.json['t']['nextCursor']
    assert cursor
    response=ai.client.post('/ai/photos/2/similar/page',json={'cursor':cursor},headers=ai.headers)
    assert response.status_code==200 and len(calls)==5


def test_frame_indexes_created_and_profile_purge_cascades(ai,migration_engine):
    Maintenance(ai.sessions,migration_engine).tick()
    with migration_engine.connect() as c:
        assert c.scalar(text('SELECT count(*) FROM pg_indexes WHERE indexname=:name'),{'name':f'ix_video_frame_profile_{ai.ai_pid}_hnsw'})==1
    with ai.sessions.begin() as s:video_ready(s,ai.ai_pid)
    with ai.sessions.begin() as s:s.delete(s.get(EmbeddingProfile,ai.ai_pid))
    with ai.sessions() as s:assert s.scalar(select(func.count()).select_from(Frame))==0


def test_large_search_frame_ann_reranks_and_threshold_falls_back(ai,migration_engine):
    from sqlalchemy import insert
    with ai.sessions.begin() as s:
        # Force the large-library branch; every irrelevant image has the same vector.
        ids=list(s.scalars(insert(Photo).returning(Photo.photo_id),[
            dict(photo_name=f'noise-{i}',user_id=2,ns_id=2,format='jpg') for i in range(5000)]))
        s.execute(insert(State),[dict(photo_id=pid,profile_id=ai.ai_pid,content_version=1,status='ready',embedding=vec(1)) for pid in ids])
        video_ready(s,ai.ai_pid)
    Maintenance(ai.sessions,migration_engine).tick()
    with migration_engine.connect().execution_options(isolation_level='AUTOCOMMIT') as c:
        c.exec_driver_sql('ANALYZE video_frame_embedding')
        c.exec_driver_sql('ANALYZE photo_embedding_state')
        c.exec_driver_sql('ANALYZE photo')
    with ai.sessions() as s:
        profile=s.get(EmbeddingProfile,ai.ai_pid)
        hits=PgVectorStore().search(s,2,profile,vec(),limit=10,min_score=.99,timeout_ms=10000)
        assert [h['photo'].photo_id for h in hits]==[2]
        assert hits[0]['matchedTimestampMs']==1000
        query,distance=PgVectorStore()._branch(profile,[vec()],PgVectorStore()._filters(2,profile),True)
        s.execute(text('SET LOCAL enable_seqscan = off'))
        sql=str(query.order_by(distance).limit(5).compile(dialect=s.bind.dialect,compile_kwargs={'literal_binds':True}))
        plan=s.execute(text('EXPLAIN '+sql)).all()
        assert f'ix_video_frame_profile_{ai.ai_pid}_hnsw' in str(plan)


def test_playlists_cannot_reference_server_files(tmp_path):
    playlist=tmp_path/'fake.mp4'
    playlist.write_text('ffconcat version 1.0\nfile /etc/passwd\n')
    with pytest.raises(AIError,match='invalid_video'):video.extract_frames(playlist)


def test_migration_roundtrip_keeps_image_vectors_and_requeues_video(ai,migration_engine):
    from alembic import command
    from app.infrastructure.schema import migration_config
    with ai.sessions.begin() as s:
        ready(s,ai.ai_pid);video_ready(s,ai.ai_pid)
    with migration_engine.begin() as c:
        before=c.execute(text("SELECT embedding::text,content_version FROM photo_embedding_state WHERE photo_id=1")).one()
        config=migration_config();config.attributes['connection']=c
        command.downgrade(config,'0002_ai_backend')
        assert c.scalar(text("SELECT status FROM photo_embedding_state WHERE photo_id=2"))=='unsupported'
        command.upgrade(config,'head')
        assert c.execute(text("SELECT embedding::text,content_version FROM photo_embedding_state WHERE photo_id=1")).one()==before
        assert c.scalar(text('SELECT count(*) FROM video_frame_embedding'))==0
        assert c.scalar(text('SELECT sampling_version FROM photo_embedding_state WHERE photo_id=2')) is None
        assert c.scalar(text('SELECT index_status FROM embedding_profile WHERE profile_id=:pid'),{'pid':ai.ai_pid})=='ready'


def test_unknown_short_frame_count_and_duration_fallback(clip,monkeypatch):
    original=video.run_media
    def probe(command,deadline):
        raw=original(command,deadline)
        if command[0]=='ffprobe' and 'stream=index' in ' '.join(command):
            data=json.loads(raw)
            data['streams'][0]['duration']='N/A'
            data['streams'][0].pop('nb_frames',None)
            return json.dumps(data).encode()
        return raw
    monkeypatch.setattr(video,'run_media',probe)
    assert len(video.extract_frames(clip))==5


def test_unknown_single_frame_count_degrades_without_duplicate_frames(tmp_path,monkeypatch):
    clip=tmp_path/'one.mp4'
    ffmpeg('-f','lavfi','-i','color=red:s=64x64:r=30','-frames:v','1','-c:v','libx264','-threads','1',clip)
    original=video.run_media
    def probe(command,deadline):
        raw=original(command,deadline)
        if command[0]=='ffprobe' and 'stream=index' in ' '.join(command):
            data=json.loads(raw);data['streams'][0].pop('nb_frames',None)
            return json.dumps(data).encode()
        return raw
    monkeypatch.setattr(video,'run_media',probe)
    frames=video.extract_frames(clip)
    assert len(frames)==1 and frames[0]['timestamp_ms']==0
