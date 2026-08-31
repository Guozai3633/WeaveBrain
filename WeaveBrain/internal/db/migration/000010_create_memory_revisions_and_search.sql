-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pg_trgm;

-- Memory cards gain the AI-claimed fields (summary / tags / key_points) plus
-- pinning. The fallback card fills these from the raw text at capture time;
-- real AI organization lands in a later round and writes new revisions.
ALTER TABLE memory_cards
    ADD COLUMN IF NOT EXISTS summary     TEXT,
    ADD COLUMN IF NOT EXISTS tags        JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS key_points  JSONB NOT NULL DEFAULT '[]'::jsonb,
    ADD COLUMN IF NOT EXISTS is_pinned   BOOLEAN NOT NULL DEFAULT FALSE,
    ADD COLUMN IF NOT EXISTS pinned_at   TIMESTAMPTZ;

-- Immutable revision trail for a memory card. source records the origin of a
-- change (fallback at creation, user correction, future AI organize); the
-- source_revision points at the capture version the derivation is based on.
CREATE TABLE IF NOT EXISTS memory_card_revisions (
    id              BIGSERIAL PRIMARY KEY,
    user_id         UUID NOT NULL,
    capture_id      UUID NOT NULL,
    revision        INTEGER NOT NULL DEFAULT 1,
    card_version    BIGINT  NOT NULL,
    source          VARCHAR(20) NOT NULL,
    source_revision BIGINT  NOT NULL,
    changes         JSONB   NOT NULL DEFAULT '{}'::jsonb,
    provenance      JSONB   NOT NULL DEFAULT '{}'::jsonb,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT memory_card_revisions_capture_fk
        FOREIGN KEY (user_id, capture_id)
        REFERENCES captures(user_id, id)
        ON DELETE CASCADE,
    CONSTRAINT memory_card_revisions_source_check
        CHECK (source IN ('fallback', 'ai', 'user')),
    CONSTRAINT memory_card_revisions_revision_unique
        UNIQUE (user_id, capture_id, revision)
);

CREATE INDEX IF NOT EXISTS idx_memory_card_revisions_capture
    ON memory_card_revisions(user_id, capture_id, revision DESC);

CREATE INDEX IF NOT EXISTS idx_memory_cards_pinned_partial
    ON memory_cards(user_id)
    WHERE is_pinned = TRUE;

-- Memory stream Chinese substring search (pg_trgm GIN, ships with contrib).
CREATE INDEX IF NOT EXISTS idx_captures_original_text_trgm
    ON captures USING GIN (original_text gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_memory_cards_title_trgm
    ON memory_cards USING GIN (title gin_trgm_ops);

CREATE INDEX IF NOT EXISTS idx_memory_cards_tags
    ON memory_cards USING GIN (tags);

CREATE INDEX IF NOT EXISTS idx_transcript_revisions_text_trgm
    ON transcript_revisions USING GIN (text gin_trgm_ops);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_transcript_revisions_text_trgm;
DROP INDEX IF EXISTS idx_memory_cards_tags;
DROP INDEX IF EXISTS idx_memory_cards_title_trgm;
DROP INDEX IF EXISTS idx_captures_original_text_trgm;
DROP INDEX IF EXISTS idx_memory_cards_pinned_partial;
DROP TABLE IF EXISTS memory_card_revisions;
ALTER TABLE memory_cards
    DROP COLUMN IF EXISTS pinned_at,
    DROP COLUMN IF EXISTS is_pinned,
    DROP COLUMN IF EXISTS key_points,
    DROP COLUMN IF EXISTS tags,
    DROP COLUMN IF EXISTS summary;
DROP EXTENSION IF EXISTS pg_trgm;
-- +goose StatementEnd
