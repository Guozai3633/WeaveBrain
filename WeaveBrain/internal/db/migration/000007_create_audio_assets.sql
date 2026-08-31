-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS audio_assets (
    user_id           UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    id                UUID NOT NULL,
    capture_id        UUID NOT NULL,
    mime_type         VARCHAR(100) NOT NULL,
    duration_ms       INTEGER,
    size_bytes        BIGINT NOT NULL,
    sha256            CHAR(64),
    storage_path      VARCHAR(500) NOT NULL,
    upload_state      VARCHAR(20) NOT NULL DEFAULT 'initiated',
    total_chunks      INTEGER NOT NULL DEFAULT 0,
    received_chunks   INTEGER NOT NULL DEFAULT 0,
    stt_enabled       BOOLEAN NOT NULL DEFAULT FALSE,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at        TIMESTAMPTZ,
    PRIMARY KEY (user_id, id),
    CONSTRAINT audio_assets_capture_fk
        FOREIGN KEY (user_id, capture_id)
        REFERENCES captures(user_id, id)
        ON DELETE CASCADE,
    CONSTRAINT audio_assets_size_check
        CHECK (size_bytes > 0),
    CONSTRAINT audio_assets_upload_state_check
        CHECK (upload_state IN ('initiated', 'uploading', 'complete', 'failed'))
);

CREATE TABLE IF NOT EXISTS transcript_revisions (
    id            BIGSERIAL PRIMARY KEY,
    user_id       UUID NOT NULL,
    capture_id    UUID NOT NULL,
    revision      INTEGER NOT NULL DEFAULT 1,
    text          TEXT NOT NULL,
    source        VARCHAR(20) NOT NULL,
    confidence    REAL,
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT transcript_revisions_capture_fk
        FOREIGN KEY (user_id, capture_id)
        REFERENCES captures(user_id, id)
        ON DELETE CASCADE,
    CONSTRAINT transcript_revisions_source_check
        CHECK (source IN ('stt', 'user')),
    CONSTRAINT transcript_revisions_revision_unique
        UNIQUE (user_id, capture_id, revision)
);

CREATE INDEX IF NOT EXISTS idx_audio_assets_user_capture
    ON audio_assets(user_id, capture_id)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_audio_assets_user_state
    ON audio_assets(user_id, upload_state)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_transcript_revisions_capture
    ON transcript_revisions(user_id, capture_id, revision DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_transcript_revisions_capture;
DROP INDEX IF EXISTS idx_audio_assets_user_state;
DROP INDEX IF EXISTS idx_audio_assets_user_capture;
DROP TABLE IF EXISTS transcript_revisions;
DROP TABLE IF EXISTS audio_assets;
-- +goose StatementEnd
