-- +goose Up
-- +goose StatementBegin
-- Capture 级导入元数据（单条 + 批量共用）。
-- 普通 capture 不设 external_id / source_name / content_hash；导入 capture 填之。
ALTER TABLE captures
    ADD COLUMN IF NOT EXISTS external_id   VARCHAR(200),
    ADD COLUMN IF NOT EXISTS source_name   VARCHAR(100),
    ADD COLUMN IF NOT EXISTS content_hash  CHAR(64);

-- 精确去重键：服务层先查重并跳过；该唯一索引是并发重复 Commit 的 DB 安全网。
-- 已删除的 capture 不阻挡重新导入。
CREATE UNIQUE INDEX IF NOT EXISTS uq_captures_user_source_external
    ON captures(user_id, source_name, external_id)
    WHERE external_id IS NOT NULL AND source_name IS NOT NULL AND deleted_at IS NULL;

-- 规范化正文 hash「疑似重复」查询（非唯一，由用户决定导入/跳过）。
CREATE INDEX IF NOT EXISTS idx_captures_user_content_hash
    ON captures(user_id, content_hash)
    WHERE content_hash IS NOT NULL;

-- 导入卡片初始修订使用 source='import'。
ALTER TABLE memory_card_revisions
    DROP CONSTRAINT IF EXISTS memory_card_revisions_source_check;
ALTER TABLE memory_card_revisions
    ADD CONSTRAINT memory_card_revisions_source_check
    CHECK (source IN ('fallback', 'ai', 'user', 'import'));

-- 一次批量导入 = 一个 job。
CREATE TABLE IF NOT EXISTS import_jobs (
    id                 UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    user_id            UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    source_name        VARCHAR(100) NOT NULL,
    format             VARCHAR(20) NOT NULL,             -- plain_text | csv | jsonl
    original_filename  VARCHAR(255),
    raw_text           TEXT NOT NULL,
    column_mapping     JSONB NOT NULL DEFAULT '{}'::jsonb,
    separator          VARCHAR(20) NOT NULL DEFAULT '---',
    timezone           VARCHAR(100),
    total_rows         INTEGER NOT NULL DEFAULT 0,
    valid_rows         INTEGER NOT NULL DEFAULT 0,
    invalid_rows       INTEGER NOT NULL DEFAULT 0,
    duplicate_rows     INTEGER NOT NULL DEFAULT 0,
    needs_input_rows   INTEGER NOT NULL DEFAULT 0,
    imported_rows      INTEGER NOT NULL DEFAULT 0,
    skipped_rows       INTEGER NOT NULL DEFAULT 0,
    failed_rows        INTEGER NOT NULL DEFAULT 0,
    status             VARCHAR(20) NOT NULL DEFAULT 'draft',
    committed_at       TIMESTAMPTZ,
    cancelled_at       TIMESTAMPTZ,
    created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT import_jobs_status_check
        CHECK (status IN ('draft', 'previewed', 'completed', 'failed', 'cancelled'))
);

CREATE INDEX IF NOT EXISTS idx_import_jobs_user_created
    ON import_jobs(user_id, created_at DESC);

-- 批量导入的一行。content_hash = 规范化正文 SHA-256。
CREATE TABLE IF NOT EXISTS import_rows (
    id                   UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    import_job_id        UUID NOT NULL REFERENCES import_jobs(id) ON DELETE CASCADE,
    user_id              UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    row_number           INTEGER NOT NULL,
    external_id          VARCHAR(200),
    raw_payload          JSONB NOT NULL DEFAULT '{}'::jsonb,
    normalized_payload   JSONB NOT NULL DEFAULT '{}'::jsonb,
    content              TEXT,
    content_hash         CHAR(64),
    validation_errors    JSONB NOT NULL DEFAULT '[]'::jsonb,
    dedupe_status        VARCHAR(20) NOT NULL DEFAULT 'none',
    capture_id           UUID,
    status               VARCHAR(20) NOT NULL DEFAULT 'pending',
    completion_proposals JSONB NOT NULL DEFAULT '[]'::jsonb,
    imported_at          TIMESTAMPTZ,
    created_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at           TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT import_rows_job_fk
        FOREIGN KEY (import_job_id) REFERENCES import_jobs(id) ON DELETE CASCADE,
    CONSTRAINT import_rows_capture_fk
        FOREIGN KEY (user_id, capture_id)
        REFERENCES captures(user_id, id)
        ON DELETE SET NULL,
    CONSTRAINT import_rows_job_row_unique
        UNIQUE (import_job_id, row_number),
    CONSTRAINT import_rows_dedupe_check
        CHECK (dedupe_status IN ('none', 'duplicate_external', 'suggested')),
    CONSTRAINT import_rows_status_check
        CHECK (status IN ('pending', 'needs_input', 'importing', 'imported', 'skipped', 'failed'))
);

CREATE INDEX IF NOT EXISTS idx_import_rows_job_status
    ON import_rows(import_job_id, status, row_number);

CREATE INDEX IF NOT EXISTS idx_import_rows_user_content_hash
    ON import_rows(user_id, content_hash)
    WHERE content_hash IS NOT NULL;
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_import_rows_user_content_hash;
DROP INDEX IF EXISTS idx_import_rows_job_status;
DROP TABLE IF EXISTS import_rows;
DROP INDEX IF EXISTS idx_import_jobs_user_created;
DROP TABLE IF EXISTS import_jobs;

-- 回滚前先把已写入的 import 修订映射为 user 来源，否则收紧 CHECK 会违反既有行。
UPDATE memory_card_revisions
    SET source = 'user'
    WHERE source = 'import';
ALTER TABLE memory_card_revisions
    DROP CONSTRAINT IF EXISTS memory_card_revisions_source_check;
ALTER TABLE memory_card_revisions
    ADD CONSTRAINT memory_card_revisions_source_check
    CHECK (source IN ('fallback', 'ai', 'user'));

DROP INDEX IF EXISTS idx_captures_user_content_hash;
DROP INDEX IF EXISTS uq_captures_user_source_external;
ALTER TABLE captures
    DROP COLUMN IF EXISTS content_hash,
    DROP COLUMN IF EXISTS source_name,
    DROP COLUMN IF EXISTS external_id;
-- +goose StatementEnd
