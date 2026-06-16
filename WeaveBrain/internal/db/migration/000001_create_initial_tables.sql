-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name VARCHAR(100),
    avatar_url TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS identities (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    provider   VARCHAR(20) NOT NULL,
    provider_id VARCHAR(255) NOT NULL,
    phone      VARCHAR(20),
    UNIQUE(user_id, provider),
    created_at TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS projects (
    id             BIGSERIAL PRIMARY KEY,
    user_id        UUID REFERENCES users(id) ON DELETE CASCADE,
    name           VARCHAR(100) NOT NULL,
    default_project BOOLEAN DEFAULT false,
    created_at     TIMESTAMPTZ DEFAULT now(),
    updated_at     TIMESTAMPTZ DEFAULT now(),
    deleted_at     TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS ideas (
    id              BIGSERIAL PRIMARY KEY,
    project_id      BIGINT REFERENCES projects(id) ON DELETE CASCADE,
    user_id         UUID REFERENCES users(id),
    raw_input       TEXT NOT NULL,
    structured_data JSONB DEFAULT '{}',
    tags            TEXT[] DEFAULT '{}',
    created_at      TIMESTAMPTZ DEFAULT now(),
    updated_at      TIMESTAMPTZ DEFAULT now(),
    deleted_at      TIMESTAMPTZ
);

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    profile_data  JSONB DEFAULT '{}',
    last_updated  TIMESTAMPTZ DEFAULT now()
);

CREATE TABLE IF NOT EXISTS reminders (
    id                BIGSERIAL PRIMARY KEY,
    user_id           UUID REFERENCES users(id) ON DELETE CASCADE,
    project_id        BIGINT REFERENCES projects(id) ON DELETE SET NULL,
    trigger_time      TIMESTAMPTZ NOT NULL,
    message           TEXT NOT NULL,
    status            VARCHAR(20) DEFAULT 'pending',
    created_at        TIMESTAMPTZ DEFAULT now()
);

-- Indexes
CREATE INDEX idx_projects_user ON projects(user_id, deleted_at);
CREATE INDEX idx_ideas_project ON ideas(project_id, deleted_at);
CREATE INDEX idx_ideas_tags ON ideas USING GIN(tags);
CREATE INDEX idx_ideas_structured ON ideas USING GIN(structured_data);
CREATE INDEX idx_reminders_trigger ON reminders(status, trigger_time);

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_reminders_trigger;
DROP INDEX IF EXISTS idx_ideas_structured;
DROP INDEX IF EXISTS idx_ideas_tags;
DROP INDEX IF EXISTS idx_ideas_project;
DROP INDEX IF EXISTS idx_projects_user;

DROP TABLE IF EXISTS reminders;
DROP TABLE IF EXISTS user_profiles;
DROP TABLE IF EXISTS ideas;
DROP TABLE IF EXISTS projects;
DROP TABLE IF EXISTS identities;
DROP TABLE IF EXISTS users;
-- +goose StatementEnd
