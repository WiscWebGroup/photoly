CREATE EXTENSION IF NOT EXISTS vector;

CREATE TABLE photo (
    id         bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id    integer     NOT NULL,
    name       text        NOT NULL,
    mime       text        NOT NULL,
    size_bytes bigint      NOT NULL,
    sha256     bytea       NOT NULL,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX photo_user ON photo (user_id, id DESC);

-- One vector per photo and model profile. `source` records who computed it, so the server
-- can later re-check client vectors and replace them.
CREATE TABLE photo_embedding (
    photo_id   bigint       NOT NULL REFERENCES photo (id) ON DELETE CASCADE,
    profile    text         NOT NULL,
    source     text         NOT NULL CHECK (source IN ('client', 'server')),
    embedding  halfvec(768) NOT NULL,
    created_at timestamptz  NOT NULL DEFAULT now(),
    PRIMARY KEY (photo_id, profile)
);
CREATE INDEX photo_embedding_hnsw ON photo_embedding USING hnsw (embedding halfvec_cosine_ops);
