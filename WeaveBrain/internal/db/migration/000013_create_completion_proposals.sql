-- +goose Up
-- +goose StatementBegin
-- AI 补全提案：一行 = 一个字段提案；一次 preview 批次共享 preview_id。
-- 提案依据卡片的 source_revision（MemoryCard.version）；正文/卡片变化后旧提案
-- 由服务层标为 expired，禁止再 apply。
CREATE TABLE IF NOT EXISTS completion_proposals (
    id               UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id          UUID NOT NULL,
    capture_id       UUID NOT NULL,
    preview_id       UUID NOT NULL,
    source_revision  BIGINT NOT NULL,
    field_name       VARCHAR(30) NOT NULL,
    original_value   TEXT NOT NULL DEFAULT '',
    proposed_value   TEXT NOT NULL,
    provenance       VARCHAR(20) NOT NULL DEFAULT 'ai',
    apply_policy     VARCHAR(20) NOT NULL DEFAULT 'safe_auto',
    confidence       REAL,
    risk_level       VARCHAR(20),
    evidence_spans   JSONB NOT NULL DEFAULT '[]'::jsonb,
    status           VARCHAR(20) NOT NULL DEFAULT 'pending',
    provider         VARCHAR(50),
    model            VARCHAR(100),
    config_version   VARCHAR(50),
    accepted_by      UUID,
    accepted_at      TIMESTAMPTZ,
    created_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT completion_proposals_capture_fk
        FOREIGN KEY (user_id, capture_id)
        REFERENCES captures(user_id, id)
        ON DELETE CASCADE,
    CONSTRAINT completion_proposals_field_check
        CHECK (field_name IN ('title', 'primary_type', 'summary', 'tags', 'key_points')),
    CONSTRAINT completion_proposals_provenance_check
        CHECK (provenance IN ('ai', 'inherited', 'import', 'user', 'device')),
    CONSTRAINT completion_proposals_policy_check
        CHECK (apply_policy IN ('safe_auto', 'suggest_only', 'forbidden')),
    CONSTRAINT completion_proposals_status_check
        CHECK (status IN ('pending', 'accepted', 'rejected', 'expired'))
);

CREATE INDEX IF NOT EXISTS idx_completion_proposals_user_capture
    ON completion_proposals(user_id, capture_id);

CREATE INDEX IF NOT EXISTS idx_completion_proposals_capture_status
    ON completion_proposals(capture_id, status);

CREATE INDEX IF NOT EXISTS idx_completion_proposals_preview
    ON completion_proposals(preview_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_completion_proposals_preview;
DROP INDEX IF EXISTS idx_completion_proposals_capture_status;
DROP INDEX IF EXISTS idx_completion_proposals_user_capture;
DROP TABLE IF EXISTS completion_proposals;
-- +goose StatementEnd
