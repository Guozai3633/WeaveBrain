-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_ai_settings (
    user_id                UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    ai_memory_enabled      BOOLEAN NOT NULL DEFAULT FALSE,
    ai_completion_enabled  BOOLEAN NOT NULL DEFAULT FALSE,
    speech_to_text_enabled BOOLEAN NOT NULL DEFAULT FALSE,
    cloud_text_allowed     BOOLEAN NOT NULL DEFAULT FALSE,
    cloud_audio_allowed    BOOLEAN NOT NULL DEFAULT FALSE,
    revision               BIGINT NOT NULL DEFAULT 0,
    created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (user_id)
);

CREATE INDEX IF NOT EXISTS idx_user_ai_settings_revision
    ON user_ai_settings(user_id, revision);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_ai_settings_revision;
DROP TABLE IF EXISTS user_ai_settings;
-- +goose StatementEnd
