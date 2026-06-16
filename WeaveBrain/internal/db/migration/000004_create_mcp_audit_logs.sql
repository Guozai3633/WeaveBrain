-- +goose Up
CREATE TABLE IF NOT EXISTS mcp_audit_logs (
    id          BIGSERIAL PRIMARY KEY,
    user_id     UUID,
    tool_name   VARCHAR(255) NOT NULL,
    server_name VARCHAR(255),
    input       JSONB NOT NULL DEFAULT '{}',
    output      JSONB,
    duration_ms INTEGER NOT NULL DEFAULT 0,
    success     BOOLEAN NOT NULL DEFAULT false,
    error_msg   TEXT,
    created_at  TIMESTAMPTZ DEFAULT now()
);

CREATE INDEX idx_mcp_audit_user ON mcp_audit_logs(user_id, created_at);
CREATE INDEX idx_mcp_audit_tool ON mcp_audit_logs(tool_name, created_at);

-- +goose Down
DROP INDEX IF EXISTS idx_mcp_audit_tool;
DROP INDEX IF EXISTS idx_mcp_audit_user;
DROP TABLE IF EXISTS mcp_audit_logs;
