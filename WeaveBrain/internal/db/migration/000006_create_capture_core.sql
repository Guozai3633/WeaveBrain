-- +goose Up
-- +goose StatementBegin
ALTER TABLE projects
    ADD CONSTRAINT projects_user_id_id_unique
    UNIQUE (user_id, id);

CREATE TABLE IF NOT EXISTS captures (
    user_id               UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    id                    UUID NOT NULL,
    kind                  VARCHAR(20) NOT NULL,
    original_text         TEXT,
    captured_at           TIMESTAMPTZ,
    captured_at_precision VARCHAR(20) NOT NULL DEFAULT 'unknown',
    timezone              VARCHAR(100),
    source                VARCHAR(50) NOT NULL,
    collection_id         BIGINT,
    privacy_mode          VARCHAR(30) NOT NULL DEFAULT 'cloud_allowed',
    request_hash          CHAR(64) NOT NULL,
    client_version        INTEGER NOT NULL,
    version               BIGINT NOT NULL DEFAULT 1,
    lifecycle_status      VARCHAR(20) NOT NULL DEFAULT 'active',
    created_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at            TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at            TIMESTAMPTZ,
    PRIMARY KEY (user_id, id),
    CONSTRAINT captures_collection_owner_fk
        FOREIGN KEY (user_id, collection_id)
        REFERENCES projects(user_id, id)
        ON DELETE SET NULL (collection_id),
    CONSTRAINT captures_kind_check
        CHECK (kind IN ('text', 'audio', 'import', 'share')),
    CONSTRAINT captures_precision_check
        CHECK (captured_at_precision IN ('exact', 'date_only', 'estimated', 'unknown')),
    CONSTRAINT captures_privacy_check
        CHECK (privacy_mode IN ('cloud_allowed', 'no_ai')),
    CONSTRAINT captures_lifecycle_check
        CHECK (lifecycle_status IN ('active', 'archived', 'trashed', 'deleted')),
    CONSTRAINT captures_text_content_check
        CHECK (kind <> 'text' OR NULLIF(BTRIM(original_text), '') IS NOT NULL),
    CONSTRAINT captures_client_version_check
        CHECK (client_version > 0)
);

CREATE TABLE IF NOT EXISTS memory_cards (
    id                UUID PRIMARY KEY,
    user_id           UUID NOT NULL,
    capture_id        UUID NOT NULL,
    primary_type      VARCHAR(30) NOT NULL DEFAULT 'uncategorized',
    title             VARCHAR(300) NOT NULL,
    processing_status VARCHAR(30) NOT NULL DEFAULT 'ready',
    version           BIGINT NOT NULL DEFAULT 1,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at        TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT memory_cards_capture_fk
        FOREIGN KEY (user_id, capture_id)
        REFERENCES captures(user_id, id)
        ON DELETE CASCADE,
    CONSTRAINT memory_cards_capture_unique
        UNIQUE (user_id, capture_id),
    CONSTRAINT memory_cards_processing_check
        CHECK (processing_status IN ('pending', 'processing', 'ready', 'needs_input', 'failed'))
);

CREATE TABLE IF NOT EXISTS capture_outbox (
    id            BIGSERIAL PRIMARY KEY,
    user_id       UUID NOT NULL,
    capture_id    UUID NOT NULL,
    event_type    VARCHAR(100) NOT NULL,
    payload       JSONB NOT NULL DEFAULT '{}',
    status        VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempt_count INTEGER NOT NULL DEFAULT 0,
    next_run_at   TIMESTAMPTZ NOT NULL DEFAULT now(),
    created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
    processed_at  TIMESTAMPTZ,
    CONSTRAINT capture_outbox_capture_fk
        FOREIGN KEY (user_id, capture_id)
        REFERENCES captures(user_id, id)
        ON DELETE CASCADE,
    CONSTRAINT capture_outbox_event_unique
        UNIQUE (user_id, capture_id, event_type),
    CONSTRAINT capture_outbox_status_check
        CHECK (status IN ('pending', 'processing', 'processed', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_captures_user_captured
    ON captures(user_id, captured_at DESC NULLS LAST, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_captures_user_created
    ON captures(user_id, created_at DESC, id DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_memory_cards_user_updated
    ON memory_cards(user_id, updated_at DESC, id DESC);

CREATE INDEX IF NOT EXISTS idx_capture_outbox_pending
    ON capture_outbox(status, next_run_at, id)
    WHERE status IN ('pending', 'failed');
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_capture_outbox_pending;
DROP INDEX IF EXISTS idx_memory_cards_user_updated;
DROP INDEX IF EXISTS idx_captures_user_created;
DROP INDEX IF EXISTS idx_captures_user_captured;
DROP TABLE IF EXISTS capture_outbox;
DROP TABLE IF EXISTS memory_cards;
DROP TABLE IF EXISTS captures;
ALTER TABLE projects
    DROP CONSTRAINT IF EXISTS projects_user_id_id_unique;
-- +goose StatementEnd
