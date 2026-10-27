-- +goose Up
-- +goose StatementBegin
--
-- PostgreSQL database dump
--

-- Dumped from database version 17.11 (Debian 17.11-1.pgdg12+2)
-- Dumped by pg_dump version 17.11 (Debian 17.11-1.pgdg12+2)

--
-- Name: vector; Type: EXTENSION; Schema: -; Owner: -
--


--
-- Name: photoly_immutable_profile(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.photoly_immutable_profile() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
 IF (NEW.provider,NEW.dimension,NEW.fingerprint,NEW.spec) IS DISTINCT FROM (OLD.provider,OLD.dimension,OLD.fingerprint,OLD.spec) THEN
   RAISE EXCEPTION 'immutable profile' USING ERRCODE='23514';
 END IF;
 RETURN NEW;
END $$;

--
-- Name: photoly_invalidate_ocr(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.photoly_invalidate_ocr() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  DELETE FROM photo_ocr WHERE photo_id=NEW.photo_id;
  RETURN NEW;
END $$;

--
-- Name: photoly_smart_media_changed(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.photoly_smart_media_changed() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE uid integer;
BEGIN
  IF TG_OP = 'DELETE' THEN uid := OLD.user_id; ELSE uid := NEW.user_id; END IF;
  UPDATE smart_preference SET revision=revision+1 WHERE user_id=uid;
  IF TG_OP = 'UPDATE' THEN
    DELETE FROM tag_photo WHERE photo_id=NEW.photo_id AND tag_id IN
      (SELECT i.tag_id FROM smart_item i JOIN smart_member m ON i.item_id=m.item_id
       WHERE m.photo_id=NEW.photo_id AND m.manual<>1);
    DELETE FROM gallery_photo WHERE photo_id=NEW.photo_id AND ga_id IN
      (SELECT i.ga_id FROM smart_item i JOIN smart_member m ON i.item_id=m.item_id
       WHERE m.photo_id=NEW.photo_id AND m.manual<>1);
    UPDATE smart_member SET automatic=false, anchor=false, run=NULL WHERE photo_id=NEW.photo_id;
    DELETE FROM smart_media WHERE photo_id=NEW.photo_id;
  END IF;
  RETURN NULL;
END $$;

--
-- Name: photoly_smart_vector_ready(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.photoly_smart_vector_ready() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
BEGIN
  IF NEW.status = 'ready' THEN
    UPDATE smart_preference SET revision=revision+1 WHERE user_id=(SELECT user_id FROM photo WHERE photo_id=NEW.photo_id);
  END IF;
  RETURN NULL;
END $$;

--
-- Name: photoly_validate_embedding(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.photoly_validate_embedding() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE dim integer;
BEGIN
  SELECT dimension INTO dim FROM embedding_profile WHERE profile_id=NEW.profile_id;
  IF NEW.sampling_version IS NOT NULL THEN
    IF NEW.sampling_version <= 0 OR NEW.embedding IS NOT NULL OR
       (NEW.status='ready' AND NEW.frame_count NOT BETWEEN 1 AND 5) OR
       (NEW.status<>'ready' AND NEW.frame_count<>0) THEN
      RAISE EXCEPTION 'invalid video embedding state' USING ERRCODE='23514';
    END IF;
  ELSE
    IF NEW.frame_count<>0 THEN RAISE EXCEPTION 'image frame count' USING ERRCODE='23514'; END IF;
    IF NEW.status='ready' THEN
      IF NEW.embedding IS NULL OR vector_dims(NEW.embedding) <> dim OR l2_norm(NEW.embedding) <= 0 THEN
        RAISE EXCEPTION 'invalid embedding' USING ERRCODE='23514';
      END IF;
    ELSIF NEW.embedding IS NOT NULL THEN
      RAISE EXCEPTION 'nonready embedding' USING ERRCODE='23514';
    END IF;
  END IF;
  RETURN NEW;
END $$;

--
-- Name: photoly_validate_frame_count(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.photoly_validate_frame_count() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE expected integer; actual integer; sid bigint;
BEGIN
  sid=COALESCE(NEW.state_id,OLD.state_id);
  SELECT frame_count INTO expected FROM photo_embedding_state WHERE state_id=sid;
  IF FOUND THEN
    SELECT count(*) INTO actual FROM video_frame_embedding WHERE state_id=sid;
    IF actual<>expected THEN RAISE EXCEPTION 'incomplete video frames' USING ERRCODE='23514'; END IF;
  END IF;
  RETURN NULL;
END $$;

--
-- Name: photoly_validate_video_frame(); Type: FUNCTION; Schema: public; Owner: -
--

CREATE FUNCTION public.photoly_validate_video_frame() RETURNS trigger
    LANGUAGE plpgsql
    AS $$
DECLARE dim integer;
BEGIN
  IF TG_OP='UPDATE' AND (NEW.state_id<>OLD.state_id OR NEW.profile_id<>OLD.profile_id) THEN
    RAISE EXCEPTION 'frame ownership is immutable' USING ERRCODE='23514';
  END IF;
  SELECT dimension INTO dim FROM embedding_profile WHERE profile_id=NEW.profile_id;
  IF dim IS NULL OR vector_dims(NEW.embedding)<>dim OR l2_norm(NEW.embedding)<=0 THEN
    RAISE EXCEPTION 'invalid frame embedding' USING ERRCODE='23514';
  END IF;
  RETURN NEW;
END $$;

--
-- Name: ai_control_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_control_state (
    name character varying(32) NOT NULL,
    generation bigint DEFAULT '0'::bigint NOT NULL,
    desired jsonb DEFAULT '{}'::jsonb NOT NULL,
    observed jsonb DEFAULT '{}'::jsonb NOT NULL,
    lease_owner character varying(80),
    lease_token character varying(36),
    lease_expires_at timestamp with time zone,
    heartbeat_at timestamp with time zone
);

--
-- Name: ai_job; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.ai_job (
    job_id bigint NOT NULL,
    kind character varying(32) NOT NULL,
    photo_id integer,
    profile_id integer,
    content_version integer,
    payload jsonb DEFAULT '{}'::jsonb NOT NULL,
    result jsonb DEFAULT '{}'::jsonb NOT NULL,
    status character varying(16) DEFAULT 'queued'::character varying NOT NULL,
    priority integer DEFAULT 0 NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    next_run_at timestamp with time zone DEFAULT now() NOT NULL,
    lease_owner character varying(80),
    lease_token character varying(36),
    lease_expires_at timestamp with time zone,
    heartbeat_at timestamp with time zone,
    dedup_key character varying(160) NOT NULL,
    reserved_bytes bigint DEFAULT '0'::bigint NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    finished_at timestamp with time zone,
    CONSTRAINT ck_ai_job_status CHECK (((status)::text = ANY ((ARRAY['queued'::character varying, 'running'::character varying, 'succeeded'::character varying, 'failed'::character varying, 'cancelled'::character varying])::text[])))
);

--
-- Name: ai_job_job_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.ai_job ALTER COLUMN job_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.ai_job_job_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: cred; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.cred (
    cred_id integer NOT NULL,
    user_id integer,
    token character varying(255),
    "authorization" character varying(255)
);

--
-- Name: cred_cred_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.cred ALTER COLUMN cred_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.cred_cred_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: embedding_profile; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.embedding_profile (
    profile_id integer NOT NULL,
    provider character varying(32) NOT NULL,
    dimension integer NOT NULL,
    fingerprint character varying(64) NOT NULL,
    spec jsonb NOT NULL,
    index_status character varying(16) DEFAULT 'pending'::character varying NOT NULL,
    created_at timestamp with time zone DEFAULT now() NOT NULL,
    CONSTRAINT ck_profile_dimension CHECK ((dimension = ANY (ARRAY[768, 2048])))
);

--
-- Name: embedding_profile_profile_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.embedding_profile ALTER COLUMN profile_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.embedding_profile_profile_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: gallery; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gallery (
    ga_id integer NOT NULL,
    ga_name character varying(255),
    user_id integer,
    create_date timestamp without time zone,
    cover_id integer,
    cover_color character varying(255)
);

--
-- Name: gallery_ga_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.gallery ALTER COLUMN ga_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.gallery_ga_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: gallery_photo; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.gallery_photo (
    gp_id integer NOT NULL,
    ga_id integer,
    photo_id integer
);

--
-- Name: gallery_photo_gp_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.gallery_photo ALTER COLUMN gp_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.gallery_photo_gp_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: namespace; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.namespace (
    ns_id integer NOT NULL,
    ns_name character varying(255),
    ns_parent_id integer,
    user_id integer
);

--
-- Name: namespace_ns_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.namespace ALTER COLUMN ns_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.namespace_ns_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: namespace_photo; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.namespace_photo (
    np_id integer NOT NULL,
    ns_id integer,
    photo_id integer
);

--
-- Name: namespace_photo_np_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.namespace_photo ALTER COLUMN np_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.namespace_photo_np_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: photo; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.photo (
    photo_id integer NOT NULL,
    photo_name character varying(255),
    upload_date timestamp without time zone,
    format character varying(255),
    ns_id integer,
    visibility integer,
    token character varying(255),
    photo_uuid character varying(255),
    user_id integer,
    content_version integer DEFAULT 1 NOT NULL,
    content_sha256 character varying(64),
    CONSTRAINT ck_photo_content_version CHECK ((content_version > 0))
);

--
-- Name: photo_embedding_state; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.photo_embedding_state (
    state_id bigint NOT NULL,
    photo_id integer NOT NULL,
    profile_id integer NOT NULL,
    content_version integer NOT NULL,
    content_sha256 character varying(64),
    status character varying(16) NOT NULL,
    embedding public.halfvec,
    computed_at timestamp with time zone,
    duration_ms integer,
    error_code character varying(80),
    sampling_version integer,
    frame_count integer DEFAULT 0 NOT NULL,
    CONSTRAINT ck_embedding_frame_count CHECK (((frame_count >= 0) AND (frame_count <= 5))),
    CONSTRAINT ck_embedding_status CHECK (((status)::text = ANY ((ARRAY['pending'::character varying, 'ready'::character varying, 'failed'::character varying, 'unsupported'::character varying])::text[]))),
    CONSTRAINT ck_embedding_version CHECK ((content_version > 0))
);

--
-- Name: photo_embedding_state_state_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.photo_embedding_state ALTER COLUMN state_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.photo_embedding_state_state_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: photo_extra; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.photo_extra (
    extra_photo_id integer NOT NULL,
    f_photo_id integer NOT NULL,
    extra_photo_gps_lat double precision,
    extra_photo_gps_lon double precision,
    extra_photo_camera character varying(255),
    extra_photo_lens character varying(255),
    extra_photo_aperture double precision,
    extra_photo_focal integer,
    extra_photo_iso integer,
    extra_photo_shutter_sec integer,
    extra_photo_taken_date timestamp without time zone,
    extra_photo_note character varying(255)
);

--
-- Name: photo_extra_extra_photo_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.photo_extra ALTER COLUMN extra_photo_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.photo_extra_extra_photo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: photo_media; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.photo_media (
    photo_id integer NOT NULL,
    animated boolean DEFAULT false NOT NULL,
    has_preview boolean DEFAULT false NOT NULL,
    live boolean DEFAULT false NOT NULL,
    live_status character varying(20),
    live_error character varying(255)
);

--
-- Name: photo_ocr; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.photo_ocr (
    photo_id integer NOT NULL,
    content_version integer NOT NULL,
    text text NOT NULL,
    languages character varying(100) NOT NULL,
    engine_version character varying(255),
    revision integer NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    CONSTRAINT ck_ocr_text_length CHECK ((length(text) <= 100000)),
    CONSTRAINT ck_ocr_versions CHECK (((content_version > 0) AND (revision > 0)))
);

--
-- Name: photo_photo_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.photo ALTER COLUMN photo_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.photo_photo_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: setting; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.setting (
    setting_id integer NOT NULL,
    setting_name character varying(255),
    setting_value character varying(255),
    note character varying(255)
);

--
-- Name: setting_setting_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.setting ALTER COLUMN setting_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.setting_setting_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: smart_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.smart_item (
    item_id integer NOT NULL,
    user_id integer NOT NULL,
    stable_key character varying(100) NOT NULL,
    source character varying(16) NOT NULL,
    tag_id integer,
    ga_id integer,
    name character varying(255) NOT NULL,
    custom_name character varying(255),
    cover jsonb,
    suppressed boolean DEFAULT false NOT NULL,
    run character varying(36),
    basis jsonb DEFAULT '{}'::jsonb NOT NULL,
    CONSTRAINT ck_smart_source CHECK (((source)::text = ANY ((ARRAY['camera'::character varying, 'lens'::character varying, 'trip'::character varying])::text[])))
);

--
-- Name: smart_item_item_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.smart_item ALTER COLUMN item_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.smart_item_item_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: smart_lineage; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.smart_lineage (
    parent_id integer NOT NULL,
    child_id integer NOT NULL
);

--
-- Name: smart_media; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.smart_media (
    photo_id integer NOT NULL,
    user_id integer NOT NULL,
    content_version integer NOT NULL,
    captured_at timestamp with time zone,
    latitude double precision,
    longitude double precision,
    facts jsonb DEFAULT '{}'::jsonb NOT NULL,
    attempts integer DEFAULT 0 NOT NULL,
    error character varying(100)
);

--
-- Name: smart_member; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.smart_member (
    item_id integer NOT NULL,
    photo_id integer NOT NULL,
    content_version integer NOT NULL,
    automatic boolean DEFAULT false NOT NULL,
    manual smallint DEFAULT '0'::smallint NOT NULL,
    run character varying(36),
    anchor boolean DEFAULT false NOT NULL,
    CONSTRAINT ck_smart_manual CHECK ((manual = ANY (ARRAY['-1'::integer, 0, 1])))
);

--
-- Name: smart_preference; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.smart_preference (
    user_id integer NOT NULL,
    enabled boolean DEFAULT false NOT NULL,
    revision bigint DEFAULT '1'::bigint NOT NULL,
    completed_revision bigint DEFAULT '0'::bigint NOT NULL,
    profile_id integer,
    checkpoint jsonb DEFAULT '{}'::jsonb NOT NULL,
    reason character varying(100),
    updated_at timestamp with time zone
);

--
-- Name: tag; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag (
    tag_id integer NOT NULL,
    user_id integer NOT NULL,
    tag_name character varying(255) NOT NULL
);

--
-- Name: tag_photo; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.tag_photo (
    tp_id integer NOT NULL,
    user_id integer,
    tag_id integer,
    photo_id integer
);

--
-- Name: tag_photo_tp_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tag_photo ALTER COLUMN tp_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.tag_photo_tp_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: tag_tag_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.tag ALTER COLUMN tag_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.tag_tag_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: upload_batch; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.upload_batch (
    id character varying(36) NOT NULL,
    user_id integer NOT NULL,
    ns_id integer,
    name character varying(255) NOT NULL,
    sealed boolean DEFAULT false NOT NULL,
    cancelled boolean DEFAULT false NOT NULL,
    created_at timestamp with time zone NOT NULL,
    updated_at timestamp with time zone NOT NULL,
    folders json NOT NULL,
    dismissed boolean DEFAULT false NOT NULL,
    failures_acknowledged_at timestamp with time zone
);

--
-- Name: upload_item; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.upload_item (
    id character varying(36) NOT NULL,
    batch_id character varying(36) NOT NULL,
    path text NOT NULL,
    size bigint NOT NULL,
    state character varying(20) NOT NULL,
    error character varying(255),
    content_sha256 character varying(64),
    photo_id integer,
    ns_id integer,
    source_id character varying(36),
    entry_index integer,
    updated_at timestamp with time zone NOT NULL,
    kind character varying(20) DEFAULT 'upload'::character varying NOT NULL,
    options json,
    media_metadata json,
    pair_id character varying(36)
);

--
-- Name: user; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public."user" (
    user_id integer NOT NULL,
    user_name character varying(255),
    email character varying(255),
    create_date timestamp without time zone,
    role character varying(255),
    password character varying(255),
    uuid character varying(255),
    ai_search_enabled boolean DEFAULT false NOT NULL,
    group_id integer DEFAULT 1 NOT NULL,
    storage_override boolean DEFAULT false NOT NULL,
    max_storage_bytes bigint,
    used_bytes bigint,
    CONSTRAINT ck_user_storage CHECK ((((max_storage_bytes IS NULL) OR (max_storage_bytes >= 0)) AND ((used_bytes IS NULL) OR (used_bytes >= 0))))
);

--
-- Name: user_group; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.user_group (
    id integer NOT NULL,
    name character varying(255) NOT NULL,
    max_storage_bytes bigint,
    is_default boolean DEFAULT false NOT NULL,
    CONSTRAINT ck_group_storage CHECK (((max_storage_bytes IS NULL) OR (max_storage_bytes >= 0)))
);

--
-- Name: user_group_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.user_group ALTER COLUMN id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.user_group_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: user_user_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public."user" ALTER COLUMN user_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.user_user_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: video_frame_embedding; Type: TABLE; Schema: public; Owner: -
--

CREATE TABLE public.video_frame_embedding (
    frame_id bigint NOT NULL,
    state_id bigint NOT NULL,
    profile_id integer NOT NULL,
    frame_index integer NOT NULL,
    timestamp_ms bigint NOT NULL,
    embedding public.halfvec NOT NULL,
    CONSTRAINT ck_video_frame_index CHECK (((frame_index >= 0) AND (frame_index <= 4))),
    CONSTRAINT ck_video_frame_timestamp CHECK ((timestamp_ms >= 0))
);

--
-- Name: video_frame_embedding_frame_id_seq; Type: SEQUENCE; Schema: public; Owner: -
--

ALTER TABLE public.video_frame_embedding ALTER COLUMN frame_id ADD GENERATED BY DEFAULT AS IDENTITY (
    SEQUENCE NAME public.video_frame_embedding_frame_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1
);

--
-- Name: ai_control_state pk_ai_control_state; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_control_state
    ADD CONSTRAINT pk_ai_control_state PRIMARY KEY (name);

--
-- Name: ai_job pk_ai_job; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_job
    ADD CONSTRAINT pk_ai_job PRIMARY KEY (job_id);

--
-- Name: cred pk_cred; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cred
    ADD CONSTRAINT pk_cred PRIMARY KEY (cred_id);

--
-- Name: embedding_profile pk_embedding_profile; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embedding_profile
    ADD CONSTRAINT pk_embedding_profile PRIMARY KEY (profile_id);

--
-- Name: gallery pk_gallery; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gallery
    ADD CONSTRAINT pk_gallery PRIMARY KEY (ga_id);

--
-- Name: gallery_photo pk_gallery_photo; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gallery_photo
    ADD CONSTRAINT pk_gallery_photo PRIMARY KEY (gp_id);

--
-- Name: namespace pk_namespace; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace
    ADD CONSTRAINT pk_namespace PRIMARY KEY (ns_id);

--
-- Name: namespace_photo pk_namespace_photo; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace_photo
    ADD CONSTRAINT pk_namespace_photo PRIMARY KEY (np_id);

--
-- Name: photo pk_photo; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo
    ADD CONSTRAINT pk_photo PRIMARY KEY (photo_id);

--
-- Name: photo_embedding_state pk_photo_embedding_state; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_embedding_state
    ADD CONSTRAINT pk_photo_embedding_state PRIMARY KEY (state_id);

--
-- Name: photo_extra pk_photo_extra; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_extra
    ADD CONSTRAINT pk_photo_extra PRIMARY KEY (extra_photo_id);

--
-- Name: photo_media pk_photo_media; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_media
    ADD CONSTRAINT pk_photo_media PRIMARY KEY (photo_id);

--
-- Name: photo_ocr pk_photo_ocr; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_ocr
    ADD CONSTRAINT pk_photo_ocr PRIMARY KEY (photo_id);

--
-- Name: setting pk_setting; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.setting
    ADD CONSTRAINT pk_setting PRIMARY KEY (setting_id);

--
-- Name: smart_item pk_smart_item; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_item
    ADD CONSTRAINT pk_smart_item PRIMARY KEY (item_id);

--
-- Name: smart_lineage pk_smart_lineage; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_lineage
    ADD CONSTRAINT pk_smart_lineage PRIMARY KEY (parent_id, child_id);

--
-- Name: smart_media pk_smart_media; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_media
    ADD CONSTRAINT pk_smart_media PRIMARY KEY (photo_id);

--
-- Name: smart_member pk_smart_member; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_member
    ADD CONSTRAINT pk_smart_member PRIMARY KEY (item_id, photo_id);

--
-- Name: smart_preference pk_smart_preference; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_preference
    ADD CONSTRAINT pk_smart_preference PRIMARY KEY (user_id);

--
-- Name: tag pk_tag; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag
    ADD CONSTRAINT pk_tag PRIMARY KEY (tag_id);

--
-- Name: tag_photo pk_tag_photo; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_photo
    ADD CONSTRAINT pk_tag_photo PRIMARY KEY (tp_id);

--
-- Name: upload_batch pk_upload_batch; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_batch
    ADD CONSTRAINT pk_upload_batch PRIMARY KEY (id);

--
-- Name: upload_item pk_upload_item; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_item
    ADD CONSTRAINT pk_upload_item PRIMARY KEY (id);

--
-- Name: user pk_user; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT pk_user PRIMARY KEY (user_id);

--
-- Name: user_group pk_user_group; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_group
    ADD CONSTRAINT pk_user_group PRIMARY KEY (id);

--
-- Name: video_frame_embedding pk_video_frame_embedding; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.video_frame_embedding
    ADD CONSTRAINT pk_video_frame_embedding PRIMARY KEY (frame_id);

--
-- Name: cred uq_cred_token; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cred
    ADD CONSTRAINT uq_cred_token UNIQUE (token);

--
-- Name: embedding_profile uq_embedding_profile_fingerprint; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.embedding_profile
    ADD CONSTRAINT uq_embedding_profile_fingerprint UNIQUE (fingerprint);

--
-- Name: photo_embedding_state uq_embedding_profile_photo; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_embedding_state
    ADD CONSTRAINT uq_embedding_profile_photo UNIQUE (profile_id, photo_id);

--
-- Name: photo_embedding_state uq_embedding_state_profile; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_embedding_state
    ADD CONSTRAINT uq_embedding_state_profile UNIQUE (state_id, profile_id);

--
-- Name: gallery_photo uq_gallery_photo_ga_id_photo_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gallery_photo
    ADD CONSTRAINT uq_gallery_photo_ga_id_photo_id UNIQUE (ga_id, photo_id);

--
-- Name: photo_extra uq_photo_extra_f_photo_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_extra
    ADD CONSTRAINT uq_photo_extra_f_photo_id UNIQUE (f_photo_id);

--
-- Name: photo uq_photo_photo_uuid; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo
    ADD CONSTRAINT uq_photo_photo_uuid UNIQUE (photo_uuid);

--
-- Name: photo uq_photo_token; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo
    ADD CONSTRAINT uq_photo_token UNIQUE (token);

--
-- Name: setting uq_setting_setting_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.setting
    ADD CONSTRAINT uq_setting_setting_name UNIQUE (setting_name);

--
-- Name: smart_item uq_smart_identity; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_item
    ADD CONSTRAINT uq_smart_identity UNIQUE (user_id, stable_key);

--
-- Name: smart_item uq_smart_item_ga_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_item
    ADD CONSTRAINT uq_smart_item_ga_id UNIQUE (ga_id);

--
-- Name: smart_item uq_smart_item_tag_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_item
    ADD CONSTRAINT uq_smart_item_tag_id UNIQUE (tag_id);

--
-- Name: tag_photo uq_tag_photo_tag_id_photo_id; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_photo
    ADD CONSTRAINT uq_tag_photo_tag_id_photo_id UNIQUE (tag_id, photo_id);

--
-- Name: user uq_user_email; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT uq_user_email UNIQUE (email);

--
-- Name: user_group uq_user_group_name; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.user_group
    ADD CONSTRAINT uq_user_group_name UNIQUE (name);

--
-- Name: user uq_user_uuid; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT uq_user_uuid UNIQUE (uuid);

--
-- Name: video_frame_embedding uq_video_frame_position; Type: CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.video_frame_embedding
    ADD CONSTRAINT uq_video_frame_position UNIQUE (state_id, frame_index);

--
-- Name: ix_ai_job_claim; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_ai_job_claim ON public.ai_job USING btree (kind, status, next_run_at, priority);

--
-- Name: ix_ai_job_expiry; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_ai_job_expiry ON public.ai_job USING btree (lease_expires_at) WHERE ((status)::text = 'running'::text);

--
-- Name: ix_cred_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_cred_user_id ON public.cred USING btree (user_id);

--
-- Name: ix_gallery_photo_ga_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_gallery_photo_ga_id ON public.gallery_photo USING btree (ga_id);

--
-- Name: ix_gallery_photo_photo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_gallery_photo_photo_id ON public.gallery_photo USING btree (photo_id);

--
-- Name: ix_gallery_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_gallery_user_id ON public.gallery USING btree (user_id);

--
-- Name: ix_namespace_ns_parent_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_namespace_ns_parent_id ON public.namespace USING btree (ns_parent_id);

--
-- Name: ix_namespace_photo_ns_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_namespace_photo_ns_id ON public.namespace_photo USING btree (ns_id);

--
-- Name: ix_namespace_photo_photo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_namespace_photo_photo_id ON public.namespace_photo USING btree (photo_id);

--
-- Name: ix_namespace_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_namespace_user_id ON public.namespace USING btree (user_id);

--
-- Name: ix_photo_embedding_state_photo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_photo_embedding_state_photo_id ON public.photo_embedding_state USING btree (photo_id);

--
-- Name: ix_photo_ns_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_photo_ns_id ON public.photo USING btree (ns_id);

--
-- Name: ix_photo_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_photo_user_id ON public.photo USING btree (user_id);

--
-- Name: ix_smart_item_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_smart_item_user_id ON public.smart_item USING btree (user_id);

--
-- Name: ix_smart_media_user_time; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_smart_media_user_time ON public.smart_media USING btree (user_id, captured_at, photo_id);

--
-- Name: ix_smart_member_photo; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_smart_member_photo ON public.smart_member USING btree (photo_id);

--
-- Name: ix_smart_member_run; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_smart_member_run ON public.smart_member USING btree (run, item_id);

--
-- Name: ix_tag_photo_photo_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tag_photo_photo_id ON public.tag_photo USING btree (photo_id);

--
-- Name: ix_tag_photo_tag_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tag_photo_tag_id ON public.tag_photo USING btree (tag_id);

--
-- Name: ix_tag_photo_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tag_photo_user_id ON public.tag_photo USING btree (user_id);

--
-- Name: ix_tag_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_tag_user_id ON public.tag USING btree (user_id);

--
-- Name: ix_upload_batch_updated_at; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_upload_batch_updated_at ON public.upload_batch USING btree (updated_at);

--
-- Name: ix_upload_batch_user_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_upload_batch_user_id ON public.upload_batch USING btree (user_id);

--
-- Name: ix_upload_item_batch_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_upload_item_batch_id ON public.upload_item USING btree (batch_id);

--
-- Name: ix_upload_item_source_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_upload_item_source_id ON public.upload_item USING btree (source_id);

--
-- Name: ix_upload_item_state; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_upload_item_state ON public.upload_item USING btree (state);

--
-- Name: ix_user_group_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_user_group_id ON public."user" USING btree (group_id);

--
-- Name: ix_video_frame_embedding_profile_id; Type: INDEX; Schema: public; Owner: -
--

CREATE INDEX ix_video_frame_embedding_profile_id ON public.video_frame_embedding USING btree (profile_id);

--
-- Name: uq_ai_job_active; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_ai_job_active ON public.ai_job USING btree (dedup_key) WHERE ((status)::text = ANY ((ARRAY['queued'::character varying, 'running'::character varying])::text[]));

--
-- Name: uq_ai_operation_key; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_ai_operation_key ON public.ai_job USING btree (dedup_key) WHERE ((kind)::text = 'operation'::text);

--
-- Name: uq_default_group; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_default_group ON public.user_group USING btree (is_default) WHERE is_default;

--
-- Name: uq_user_email_lower; Type: INDEX; Schema: public; Owner: -
--

CREATE UNIQUE INDEX uq_user_email_lower ON public."user" USING btree (lower((email)::text));

--
-- Name: video_frame_embedding complete_video_frames; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER complete_video_frames AFTER INSERT OR DELETE OR UPDATE ON public.video_frame_embedding DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.photoly_validate_frame_count();

--
-- Name: photo_embedding_state complete_video_state; Type: TRIGGER; Schema: public; Owner: -
--

CREATE CONSTRAINT TRIGGER complete_video_state AFTER INSERT OR UPDATE ON public.photo_embedding_state DEFERRABLE INITIALLY DEFERRED FOR EACH ROW EXECUTE FUNCTION public.photoly_validate_frame_count();

--
-- Name: embedding_profile immutable_profile; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER immutable_profile BEFORE UPDATE ON public.embedding_profile FOR EACH ROW EXECUTE FUNCTION public.photoly_immutable_profile();

--
-- Name: photo invalidate_photo_ocr; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER invalidate_photo_ocr AFTER UPDATE OF content_version ON public.photo FOR EACH ROW WHEN ((old.content_version IS DISTINCT FROM new.content_version)) EXECUTE FUNCTION public.photoly_invalidate_ocr();

--
-- Name: photo smart_photo_insert_delete; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER smart_photo_insert_delete AFTER INSERT OR DELETE ON public.photo FOR EACH ROW EXECUTE FUNCTION public.photoly_smart_media_changed();

--
-- Name: photo smart_photo_replace; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER smart_photo_replace AFTER UPDATE OF content_version ON public.photo FOR EACH ROW WHEN ((old.content_version IS DISTINCT FROM new.content_version)) EXECUTE FUNCTION public.photoly_smart_media_changed();

--
-- Name: photo_embedding_state smart_vector_ready; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER smart_vector_ready AFTER INSERT OR UPDATE OF status, computed_at ON public.photo_embedding_state FOR EACH ROW EXECUTE FUNCTION public.photoly_smart_vector_ready();

--
-- Name: photo_embedding_state validate_embedding; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER validate_embedding BEFORE INSERT OR UPDATE ON public.photo_embedding_state FOR EACH ROW EXECUTE FUNCTION public.photoly_validate_embedding();

--
-- Name: video_frame_embedding validate_video_frame; Type: TRIGGER; Schema: public; Owner: -
--

CREATE TRIGGER validate_video_frame BEFORE INSERT OR UPDATE ON public.video_frame_embedding FOR EACH ROW EXECUTE FUNCTION public.photoly_validate_video_frame();

--
-- Name: ai_job fk_ai_job_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_job
    ADD CONSTRAINT fk_ai_job_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: ai_job fk_ai_job_profile_id_embedding_profile; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.ai_job
    ADD CONSTRAINT fk_ai_job_profile_id_embedding_profile FOREIGN KEY (profile_id) REFERENCES public.embedding_profile(profile_id) ON DELETE SET NULL;

--
-- Name: cred fk_cred_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.cred
    ADD CONSTRAINT fk_cred_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: gallery_photo fk_gallery_photo_ga_id_gallery; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gallery_photo
    ADD CONSTRAINT fk_gallery_photo_ga_id_gallery FOREIGN KEY (ga_id) REFERENCES public.gallery(ga_id) ON DELETE CASCADE;

--
-- Name: gallery_photo fk_gallery_photo_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gallery_photo
    ADD CONSTRAINT fk_gallery_photo_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: gallery fk_gallery_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.gallery
    ADD CONSTRAINT fk_gallery_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: namespace_photo fk_namespace_photo_ns_id_namespace; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace_photo
    ADD CONSTRAINT fk_namespace_photo_ns_id_namespace FOREIGN KEY (ns_id) REFERENCES public.namespace(ns_id) ON DELETE CASCADE;

--
-- Name: namespace_photo fk_namespace_photo_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace_photo
    ADD CONSTRAINT fk_namespace_photo_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: namespace fk_namespace_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.namespace
    ADD CONSTRAINT fk_namespace_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: photo_embedding_state fk_photo_embedding_state_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_embedding_state
    ADD CONSTRAINT fk_photo_embedding_state_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: photo_embedding_state fk_photo_embedding_state_profile_id_embedding_profile; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_embedding_state
    ADD CONSTRAINT fk_photo_embedding_state_profile_id_embedding_profile FOREIGN KEY (profile_id) REFERENCES public.embedding_profile(profile_id) ON DELETE CASCADE;

--
-- Name: photo_extra fk_photo_extra_f_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_extra
    ADD CONSTRAINT fk_photo_extra_f_photo_id_photo FOREIGN KEY (f_photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: photo_media fk_photo_media_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_media
    ADD CONSTRAINT fk_photo_media_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: photo fk_photo_ns_id_namespace; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo
    ADD CONSTRAINT fk_photo_ns_id_namespace FOREIGN KEY (ns_id) REFERENCES public.namespace(ns_id);

--
-- Name: photo_ocr fk_photo_ocr_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo_ocr
    ADD CONSTRAINT fk_photo_ocr_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: photo fk_photo_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.photo
    ADD CONSTRAINT fk_photo_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: smart_item fk_smart_item_ga_id_gallery; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_item
    ADD CONSTRAINT fk_smart_item_ga_id_gallery FOREIGN KEY (ga_id) REFERENCES public.gallery(ga_id) ON DELETE SET NULL;

--
-- Name: smart_item fk_smart_item_tag_id_tag; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_item
    ADD CONSTRAINT fk_smart_item_tag_id_tag FOREIGN KEY (tag_id) REFERENCES public.tag(tag_id) ON DELETE SET NULL;

--
-- Name: smart_item fk_smart_item_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_item
    ADD CONSTRAINT fk_smart_item_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: smart_lineage fk_smart_lineage_child_id_smart_item; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_lineage
    ADD CONSTRAINT fk_smart_lineage_child_id_smart_item FOREIGN KEY (child_id) REFERENCES public.smart_item(item_id) ON DELETE CASCADE;

--
-- Name: smart_lineage fk_smart_lineage_parent_id_smart_item; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_lineage
    ADD CONSTRAINT fk_smart_lineage_parent_id_smart_item FOREIGN KEY (parent_id) REFERENCES public.smart_item(item_id) ON DELETE CASCADE;

--
-- Name: smart_media fk_smart_media_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_media
    ADD CONSTRAINT fk_smart_media_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: smart_media fk_smart_media_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_media
    ADD CONSTRAINT fk_smart_media_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: smart_member fk_smart_member_item_id_smart_item; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_member
    ADD CONSTRAINT fk_smart_member_item_id_smart_item FOREIGN KEY (item_id) REFERENCES public.smart_item(item_id) ON DELETE CASCADE;

--
-- Name: smart_member fk_smart_member_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_member
    ADD CONSTRAINT fk_smart_member_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: smart_preference fk_smart_preference_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.smart_preference
    ADD CONSTRAINT fk_smart_preference_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: tag_photo fk_tag_photo_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_photo
    ADD CONSTRAINT fk_tag_photo_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE CASCADE;

--
-- Name: tag_photo fk_tag_photo_tag_id_tag; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_photo
    ADD CONSTRAINT fk_tag_photo_tag_id_tag FOREIGN KEY (tag_id) REFERENCES public.tag(tag_id) ON DELETE CASCADE;

--
-- Name: tag_photo fk_tag_photo_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag_photo
    ADD CONSTRAINT fk_tag_photo_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: tag fk_tag_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.tag
    ADD CONSTRAINT fk_tag_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: upload_batch fk_upload_batch_ns_id_namespace; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_batch
    ADD CONSTRAINT fk_upload_batch_ns_id_namespace FOREIGN KEY (ns_id) REFERENCES public.namespace(ns_id) ON DELETE SET NULL;

--
-- Name: upload_batch fk_upload_batch_user_id_user; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_batch
    ADD CONSTRAINT fk_upload_batch_user_id_user FOREIGN KEY (user_id) REFERENCES public."user"(user_id) ON DELETE CASCADE;

--
-- Name: upload_item fk_upload_item_batch_id_upload_batch; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_item
    ADD CONSTRAINT fk_upload_item_batch_id_upload_batch FOREIGN KEY (batch_id) REFERENCES public.upload_batch(id) ON DELETE CASCADE;

--
-- Name: upload_item fk_upload_item_ns_id_namespace; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_item
    ADD CONSTRAINT fk_upload_item_ns_id_namespace FOREIGN KEY (ns_id) REFERENCES public.namespace(ns_id) ON DELETE SET NULL;

--
-- Name: upload_item fk_upload_item_photo_id_photo; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.upload_item
    ADD CONSTRAINT fk_upload_item_photo_id_photo FOREIGN KEY (photo_id) REFERENCES public.photo(photo_id) ON DELETE SET NULL;

--
-- Name: user fk_user_group_id_user_group; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public."user"
    ADD CONSTRAINT fk_user_group_id_user_group FOREIGN KEY (group_id) REFERENCES public.user_group(id);

--
-- Name: video_frame_embedding fk_video_frame_embedding_state_id_photo_embedding_state; Type: FK CONSTRAINT; Schema: public; Owner: -
--

ALTER TABLE ONLY public.video_frame_embedding
    ADD CONSTRAINT fk_video_frame_embedding_state_id_photo_embedding_state FOREIGN KEY (state_id, profile_id) REFERENCES public.photo_embedding_state(state_id, profile_id) ON DELETE CASCADE;

--
-- PostgreSQL database dump complete
--


INSERT INTO user_group (name,max_storage_bytes,is_default) VALUES ('Default',50000000000,true);
INSERT INTO setting (setting_name,setting_value,note) VALUES ('OnlineDownload','0','Allow public image URL imports');

CREATE TABLE alembic_version (version_num varchar(32) NOT NULL PRIMARY KEY);
INSERT INTO alembic_version VALUES ('0008_upload_activity');
DO $$ BEGIN
  IF EXISTS (SELECT 1 FROM pg_roles WHERE rolname='photoly') THEN
    REVOKE INSERT, UPDATE, DELETE ON alembic_version FROM photoly;
  END IF;
END $$;
-- +goose StatementEnd

-- +goose Down
SELECT 1;
