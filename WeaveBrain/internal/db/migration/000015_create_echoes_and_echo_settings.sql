-- +goose Up
-- +goose StatementBegin
-- 回响设置：enabled + cadence 由服务端管控（镜像 user_ai_settings 的
-- revision CAS），投递时刻/静默时段属客户端本地调度，不入库。
CREATE TABLE IF NOT EXISTS user_echo_settings (
    user_id    UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    enabled    BOOLEAN     NOT NULL DEFAULT FALSE,
    cadence    VARCHAR(20) NOT NULL DEFAULT 'daily',
    revision   BIGINT      NOT NULL DEFAULT 0,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id),
    CONSTRAINT user_echo_settings_cadence_check
        CHECK (cadence IN ('daily', 'every_other_day', 'weekly'))
);

CREATE INDEX IF NOT EXISTS idx_user_echo_settings_revision
    ON user_echo_settings(user_id, revision);

-- 一次回响 = 一条记忆卡片 + 说明原因。status：open=当前待反馈；
-- done/later/not_relevant=用户反馈；expired=超窗未答在读取时滚动关闭。
-- 同一卡片冷却期后可再次回响（不建 (user_id,capture_id) 唯一）。
-- captures 主键是复合键，外键必须成对引用 (user_id, capture_id)。
CREATE TABLE IF NOT EXISTS user_echoes (
    id          UUID PRIMARY KEY,
    user_id     UUID NOT NULL,
    capture_id  UUID NOT NULL,
    status      VARCHAR(16) NOT NULL DEFAULT 'open',
    reason_code VARCHAR(24) NOT NULL,
    created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at TIMESTAMPTZ,
    CONSTRAINT user_echoes_capture_fk
        FOREIGN KEY (user_id, capture_id)
        REFERENCES captures(user_id, id)
        ON DELETE CASCADE,
    CONSTRAINT user_echoes_status_check
        CHECK (status IN ('open', 'done', 'later', 'not_relevant', 'expired')),
    CONSTRAINT user_echoes_reason_check
        CHECK (reason_code IN ('first_echo', 'pinned', 'oldest', 'reminder'))
);

CREATE INDEX IF NOT EXISTS idx_user_echoes_user_created
    ON user_echoes(user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_user_echoes_user_capture
    ON user_echoes(user_id, capture_id, created_at DESC);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_echoes_user_capture;
DROP INDEX IF EXISTS idx_user_echoes_user_created;
DROP TABLE IF EXISTS user_echoes;
DROP INDEX IF EXISTS idx_user_echo_settings_revision;
DROP TABLE IF EXISTS user_echo_settings;
-- +goose StatementEnd
