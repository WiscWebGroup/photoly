package photo

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/WiscWebGroup/photoly/photoly_server/internal/app"
	"github.com/WiscWebGroup/photoly/photoly_server/internal/storage"
)

type sourceSnapshot struct {
	source               storage.Source
	rootID               int64
	root, identity, work string
	entries              map[string]diskEntry
	prepared             map[string]*Prepared
	originals            map[string]string
	failures             map[string]string
	pairs                map[string]string
	livePreviews         map[string]string
	force                bool
	scope                string
	job                  storage.ScanJob
	waiting              int
	attachments          []*scanAttachment
	tree                 storage.TreeStore
}

func ScanWorker(ctx context.Context, a *app.App) {
	s := For(a)
	_, _ = a.Pool.Exec(ctx, `UPDATE storage_scan_job SET state='queued' WHERE state='running'`)
	slots := make(chan struct{}, 3)
	var running sync.Map
	var wg sync.WaitGroup
	defer wg.Wait()
	_, _ = a.Pool.Exec(ctx, `UPDATE storage_source SET scan_requested=true WHERE enabled AND kind<>'s3'`)
	lastPrune := time.Time{}
	run := func() {
		if time.Since(lastPrune) > time.Hour {
			_, _ = a.Pool.Exec(ctx, `DELETE FROM storage_scan_job WHERE finished_at < now()-interval '7 days'`)
			lastPrune = time.Now()
		}
		rows, e := a.Pool.Query(ctx, `SELECT id FROM storage_source WHERE enabled AND kind<>'s3' AND (scan_requested OR EXISTS(SELECT 1 FROM storage_scan_job j WHERE j.source_id=storage_source.id AND j.state='queued') OR scan_finished_at IS NULL OR coalesce(full_scanned_at,scan_finished_at) + make_interval(secs=>CASE WHEN is_default THEN $1 ELSE scan_interval END)<=now()) ORDER BY scan_finished_at NULLS FIRST,id`, max(60, int(ScanInterval().Seconds())))
		if e != nil {
			return
		}
		var ids []int64
		for rows.Next() {
			var id int64
			if rows.Scan(&id) == nil {
				ids = append(ids, id)
			}
		}
		rows.Close()
		for _, id := range ids {
			if _, exists := running.LoadOrStore(id, true); exists {
				continue
			}
			select {
			case slots <- struct{}{}:
			default:
				running.Delete(id)
				continue
			}
			wg.Add(1)
			go func(id int64) {
				defer wg.Done()
				defer func() { <-slots; running.Delete(id) }()
				_ = s.ScanSource(ctx, id)
			}(id)
		}
	}
	run()
	timer := time.NewTicker(time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			run()
		}
	}
}
func (s *Service) Scan(ctx context.Context, uid int64) error {
	var id int64
	if e := s.Pool.QueryRow(ctx, `SELECT id FROM storage_source WHERE user_id=$1 AND is_default AND enabled`, uid).Scan(&id); e != nil {
		return e
	}
	return s.ScanSource(ctx, id)
}
func (s *Service) ScanSource(ctx context.Context, id int64) (err error) {
	// Session lock only coordinates this source's scanners; it is not a user lock.
	conn, e := s.Pool.Acquire(ctx)
	if e != nil {
		return e
	}
	defer conn.Release()
	var locked bool
	if e = conn.QueryRow(ctx, `SELECT pg_try_advisory_lock(482611,$1::integer)`, id).Scan(&locked); e != nil {
		return e
	}
	if !locked {
		return nil
	}
	defer conn.Exec(context.WithoutCancel(ctx), `SELECT pg_advisory_unlock(482611,$1::integer)`, id)
	snap := &sourceSnapshot{entries: map[string]diskEntry{}, prepared: map[string]*Prepared{}, originals: map[string]string{}, failures: map[string]string{}, pairs: map[string]string{}, livePreviews: map[string]string{}}
	snap.source, e = storage.GetSource(ctx, s.Pool, id)
	if e != nil {
		return e
	}
	if snap.source.Kind == "s3" {
		return nil
	}
	if e = s.Pool.QueryRow(ctx, `SELECT n.ns_id,s.root_identity,s.scan_force FROM storage_source s JOIN namespace n ON n.source_id=s.id AND (n.ns_parent_id=-1 OR n.is_mount) WHERE s.id=$1`, id).Scan(&snap.rootID, &snap.identity, &snap.force); e != nil {
		return e
	}
	snap.job, e = storage.ClaimScan(ctx, s.Pool, id)
	if e != nil {
		return e
	}
	snap.force = snap.job.Force

	defer func() {
		status := "ready"
		phase := "complete"
		jobState := "complete"
		if snap.waiting > 0 {
			phase = "waiting"
		}
		var message any
		if len(snap.failures) > 0 {
			message = fmt.Sprintf("%d media files need retry", len(snap.failures))
		}
		if err != nil {
			status = "error"
			phase = "error"
			jobState = "error"
			message = err.Error()
		}
		_, _ = s.Pool.Exec(context.WithoutCancel(ctx), `UPDATE storage_source SET scan_status=$2,scan_error=$3,scan_finished_at=now(),scan_requested=scan_requested OR $4,scan_phase=$5,scan_waiting=$6,full_scanned_at=CASE WHEN $7 THEN now() ELSE full_scanned_at END WHERE id=$1`, id, status, message, err != nil && err.Error() == "stale_scan_retry", phase, snap.waiting, snap.scope == "")
		_, _ = s.Pool.Exec(context.WithoutCancel(ctx), `UPDATE storage_scan_job SET state=$2,finished_at=now(),error=$3 WHERE id=$1`, snap.job.ID, jobState, message)
	}()
	snap.work, e = os.MkdirTemp("", "photoly-source-scan-")
	if e != nil {
		return e
	}
	defer os.RemoveAll(snap.work)
	snap.root, _, e = storage.Folder(ctx, s.Pool, s.Root, snap.rootID)
	if e != nil {
		return e
	}
	if snap.job.NamespaceID != nil {
		folder, _, err := storage.Folder(ctx, s.Pool, s.Root, *snap.job.NamespaceID)
		if err != nil {
			return err
		}
		if snap.source.Tree() {
			snap.scope = folder
		} else {
			snap.scope, e = filepath.Rel(snap.root, folder)
			if e != nil {
				return e
			}
		}
		if snap.scope == "." {
			snap.scope = ""
		}
	}
	lastProgress := time.Now()
	reportDiscovery := func() {
		if time.Since(lastProgress) > 250*time.Millisecond {
			_, _ = s.Pool.Exec(ctx, `UPDATE storage_source SET scan_discovered=$2 WHERE id=$1`, id, len(snap.entries)-1)
			lastProgress = time.Now()
		}
	}
	if snap.source.Tree() {
		snap.tree, e = storage.TreeSource(snap.source)
		if e != nil {
			return e
		}
		info, e := snap.tree.Property(ctx, "")
		if e != nil {
			return e
		}
		if !info.Directory {
			return errors.New("storage_root_not_directory")
		}
		if snap.identity != "" && snap.identity != info.Identity {
			return errors.New("storage_root_identity_changed")
		}
		snap.identity = info.Identity
		snap.entries["."] = diskEntry{path: ".", dir: true, identity: info.Identity, version: info.Version}
		if e = snap.tree.Walk(ctx, snap.scope, func(v storage.TreeEntry) error {
			snap.entries[v.Path] = diskEntry{path: v.Path, identity: v.Identity, size: v.Size, mtime: v.Modified, dir: v.Directory, version: v.Version}
			reportDiscovery()
			return nil
		}); e != nil {
			return e
		}
	} else {
		st, err := os.Lstat(snap.root)
		if err != nil {
			return err
		}
		snap.entries["."] = diskEntry{path: ".", dir: true, identity: storage.Identity(st)}
		e = filepath.WalkDir(filepath.Join(snap.root, snap.scope), func(p string, d os.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if ctx.Err() != nil {
				return ctx.Err()
			}
			if d.Name() == storage.SystemDirectory {
				if d.IsDir() {
					return filepath.SkipDir
				}
				return nil
			}
			if d.Type()&os.ModeSymlink != 0 {
				return nil
			}
			st, e := d.Info()
			if e != nil {
				return e
			}
			if !st.IsDir() && !st.Mode().IsRegular() {
				return nil
			}
			rel, e := filepath.Rel(snap.root, p)
			if e != nil {
				return e
			}
			snap.entries[rel] = diskEntry{path: rel, identity: storage.Identity(st), size: st.Size(), mtime: st.ModTime().UnixNano(), dir: st.IsDir()}
			reportDiscovery()
			return nil
		})
		if e != nil {
			return e
		}
		if snap.identity != "" && snap.identity != snap.entries["."].identity {
			return errors.New("storage_root_identity_changed")
		}
		snap.identity = snap.entries["."].identity
	}
	for p := snap.scope; p != "" && p != "."; p = filepath.Dir(p) {
		if _, ok := snap.entries[p]; ok {
			continue
		}
		if snap.tree != nil {
			v, err := snap.tree.Property(ctx, p)
			if err != nil {
				return err
			}
			snap.entries[p] = diskEntry{path: p, dir: v.Directory, identity: v.Identity, size: v.Size, mtime: v.Modified, version: v.Version}
		} else {
			v, err := os.Lstat(filepath.Join(snap.root, p))
			if err != nil {
				return err
			}
			snap.entries[p] = diskEntry{path: p, dir: v.IsDir(), identity: storage.Identity(v), size: v.Size(), mtime: v.ModTime().UnixNano()}
		}
	}
	_, _ = s.Pool.Exec(ctx, `UPDATE storage_source SET scan_discovered=$2 WHERE id=$1`, id, len(snap.entries)-1)

	// Read-only observations determine what can be prepared. Reconciliation validates
	// the revision again under the user lock, so stale work is discarded.
	observations := map[string]diskEntry{}
	rows, e := s.Pool.Query(ctx, `SELECT path,identity,size,mtime,version FROM storage_observation WHERE source_id=$1`, id)
	if e != nil {
		return e
	}
	for rows.Next() {
		var v diskEntry
		if e = rows.Scan(&v.path, &v.identity, &v.size, &v.mtime, &v.version); e != nil {
			rows.Close()
			return e
		}
		observations[v.path] = v
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	type known struct {
		identity, hash, version string
		size, mtime             int64
	}
	knownByID := map[string][]known{}
	knownByPath := map[string]known{}
	rows, e = s.Pool.Query(ctx, `WITH RECURSIVE dirs AS(SELECT ns_id,''::text AS path FROM namespace WHERE source_id=$1 AND (is_mount OR ns_parent_id=-1) UNION ALL SELECT n.ns_id,CASE WHEN d.path='' THEN n.ns_name ELSE d.path||'/'||n.ns_name END FROM namespace n JOIN dirs d ON n.ns_parent_id=d.ns_id WHERE n.source_id=$1) SELECT CASE WHEN d.path='' THEN p.file_name ELSE d.path||'/'||p.file_name END,p.fs_identity,p.fs_size,p.fs_mtime,coalesce(p.content_sha256,''),p.remote_version FROM photo p JOIN dirs d ON d.ns_id=p.ns_id WHERE p.source_id=$1`, id)
	if e != nil {
		return e
	}
	for rows.Next() {
		var p string
		var v known
		if e = rows.Scan(&p, &v.identity, &v.size, &v.mtime, &v.hash, &v.version); e != nil {
			rows.Close()
			return e
		}
		if !snap.contains(p) {
			continue
		}
		knownByPath[p] = v
		if v.identity != "" {
			knownByID[v.identity] = append(knownByID[v.identity], v)
		}
	}
	e = rows.Err()
	rows.Close()
	if e != nil {
		return e
	}
	if e = s.loadScanAttachments(ctx, snap); e != nil {
		return e
	}
	completed := 0
	tasks := map[string]diskEntry{}
	unchangedCount := 0
	for p, ent := range snap.entries {
		if !snap.contains(p) || ent.dir || !ImageFormats[suffixOf(p)] && !VideoFormats[suffixOf(p)] {
			continue
		}
		old, exists := knownByPath[p]
		if exists && !snap.force && old.identity == ent.identity && old.size == ent.size && old.mtime == ent.mtime && old.version == ent.version && (snap.tree == nil || ent.version != "" || ent.mtime != 0) {
			unchangedCount++
			continue
		}
		if !snap.force && ent.identity != "" && len(knownByID[ent.identity]) == 1 {
			old = knownByID[ent.identity][0]
			if old.size == ent.size && old.mtime == ent.mtime && old.version == ent.version {
				ent.sha = old.hash
				snap.entries[p] = ent
				continue
			}
		}
		prev, ok := observations[p]
		if !ok || prev.identity != ent.identity || prev.size != ent.size || prev.mtime != ent.mtime || prev.version != ent.version {
			snap.waiting++
			continue
		}
		tasks[p] = ent
	}
	_, _ = s.Pool.Exec(ctx, `UPDATE storage_source SET scan_phase='processing',scan_total=$2,scan_waiting=$3,scan_unchanged=$4 WHERE id=$1`, id, len(tasks), snap.waiting, unchangedCount)
	for p, ent := range tasks {
		old := knownByPath[p]
		filename := filepath.Join(snap.work, fmt.Sprintf("%d%s", completed, filepath.Ext(p)))
		completed++
		_, _ = s.Pool.Exec(ctx, `UPDATE storage_source SET scan_completed=$2,scan_retries=$3 WHERE id=$1`, id, completed-1, len(snap.failures))
		out, e := os.Create(filename)
		if e != nil {
			return e
		}
		if snap.tree != nil {
			e = snap.tree.CopyTo(ctx, p, out, ent.tree())
		} else {
			var in *os.File
			in, e = storage.OpenFile(filepath.Join(snap.root, p))
			if e == nil {
				_, e = io.Copy(out, in)
				in.Close()
			}
		}
		closeErr := out.Close()
		if e == nil {
			e = closeErr
		}
		if e != nil {
			snap.failures[p] = e.Error()
			continue
		}
		if !snap.unchanged(ctx, p, ent) {
			snap.failures[p] = "file_changed_during_scan"
			continue
		}
		ent.sha, e = hashFile(filename)
		if e != nil {
			snap.failures[p] = e.Error()
			continue
		}
		snap.entries[p] = ent
		snap.originals[p] = filename
		if ent.sha != old.hash {
			prepDir := filepath.Join(snap.work, fmt.Sprintf("prepared-%d", completed))
			if e = os.Mkdir(prepDir, 0700); e != nil {
				return e
			}
			prep, e := s.Prepare(ctx, filename, prepDir)
			if e != nil {
				snap.failures[p] = e.Error()
				continue
			}
			snap.prepared[p] = prep
		}
		if !snap.unchanged(ctx, p, ent) {
			snap.failures[p] = "file_changed_during_processing"
			continue
		}
		keepForLive := false
		if prep := snap.prepared[p]; prep != nil {
			ident, _ := prep.Meta["ContentIdentifier"].(string)
			keepForLive = VideoFormats[suffixOf(p)] && ident != ""
		}
		for _, a := range snap.attachments {
			if a.path == p || a.identity != "" && a.identity == ent.identity {
				keepForLive = true
			}
		}
		if !keepForLive {
			_ = os.Remove(filename)
			delete(snap.originals, p)
		}
		_, _ = s.Pool.Exec(ctx, `UPDATE storage_source SET scan_completed=$2 WHERE id=$1`, id, completed)
	}
	// Pair only unambiguous embedded identifiers. A shared basename never pairs
	// unrelated files. The movie keeps its actual name and its own namespace.
	type pairGroup struct{ still, movie []string }
	groups := map[string]*pairGroup{}
	for p, prep := range snap.prepared {
		ident, _ := prep.Meta["ContentIdentifier"].(string)
		if ident == "" || snap.failures[p] != "" {
			continue
		}
		group := groups[ident]
		if group == nil {
			group = &pairGroup{}
			groups[ident] = group
		}
		if ImageFormats[suffixOf(p)] {
			group.still = append(group.still, p)
		} else if VideoFormats[suffixOf(p)] {
			group.movie = append(group.movie, p)
		}
	}
	for _, group := range groups {
		if len(group.still) != 1 || len(group.movie) != 1 {
			continue
		}
		still, movie := group.still[0], group.movie[0]
		if _, indexed := knownByPath[movie]; indexed {
			continue
		}
		preview := filepath.Join(snap.work, fmt.Sprintf("live-%d.mp4", len(snap.pairs)))
		snap.pairs[still] = movie
		if e = s.EncodeLive(ctx, snap.originals[movie], preview, false); e == nil {
			snap.livePreviews[still] = preview
		} else {
			snap.failures[movie] = "live_preview_processing_failed"
		}
	}
	s.prepareScanAttachments(ctx, snap)

	_, _ = s.Pool.Exec(ctx, `UPDATE storage_source SET scan_phase='committing',scan_completed=$2,scan_retries=$3 WHERE id=$1`, id, completed, len(snap.failures))
	return s.reconcileSource(ctx, snap)
}
func (e diskEntry) tree() storage.TreeEntry {
	return storage.TreeEntry{Path: e.path, Directory: e.dir, Size: e.size, Modified: e.mtime, Identity: e.identity, Version: e.version}
}
func (snap *sourceSnapshot) unchanged(ctx context.Context, p string, ent diskEntry) bool {
	if snap.tree == nil {
		return unchanged(filepath.Join(snap.root, p), ent)
	}
	v, e := snap.tree.Property(ctx, p)
	return e == nil && storage.SameEntry(ent.tree(), v)
}

func (snap *sourceSnapshot) verifyRemote(ctx context.Context) error {
	if snap.tree != nil {
		check := map[string]diskEntry{}
		for p, v := range snap.entries {
			if v.dir && p != "." {
				current, e := snap.tree.Property(ctx, p)
				if e != nil {
					return e
				}
				if current.Identity != v.identity {
					return errors.New("storage_directory_identity_changed")
				}
			}
		}
		info, e := snap.tree.Property(ctx, "")
		if e != nil {
			return e
		}
		if info.Identity != snap.identity {
			return errors.New("storage_root_identity_changed")
		}
		if e = snap.tree.Walk(ctx, snap.scope, func(v storage.TreeEntry) error {
			check[v.Path] = diskEntry{path: v.Path, identity: v.Identity, version: v.Version, size: v.Size, mtime: v.Modified, dir: v.Directory}
			return nil
		}); e != nil {
			return e
		}
		expected := 0
		for p := range snap.entries {
			if p != snap.scope && p != "." && snap.contains(p) {
				expected++
			}
		}
		if len(check) != expected {
			return errors.New("storage_changed_during_scan")
		}
		for p, v := range check {
			if !storage.SameEntry(v.tree(), snap.entries[p].tree()) {
				return errors.New("storage_changed_during_scan")
			}
		}
	}
	return nil
}

func (snap *sourceSnapshot) contains(p string) bool {
	return snap.scope == "" || p == snap.scope || strings.HasPrefix(p, snap.scope+"/")
}
