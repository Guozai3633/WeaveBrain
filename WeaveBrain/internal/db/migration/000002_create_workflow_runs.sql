-- +goose Up
-- +goose StatementBegin
CREATE TABLE workflow_runs (
    id            BIGSERIAL    PRIMARY KEY,
    workflow_id   TEXT         NOT NULL UNIQUE,
    workflow_type TEXT         NOT NULL,
    user_id       UUID         REFERENCES users(id),
    status        TEXT         NOT NULL DEFAULT 'running',  -- running, completed, failed, cancelled
    input         JSONB,
    output        JSONB,
    error         TEXT,
    started_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    completed_at  TIMESTAMPTZ,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_workflow_runs_user_status ON workflow_runs(user_id, status);
CREATE INDEX idx_workflow_runs_type_started ON workflow_runs(workflow_type, started_at);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP TABLE IF EXISTS workflow_runs;
-- +goose StatementEnd
