-- +goose Up
-- +goose StatementBegin
CREATE TABLE IF NOT EXISTS user_mcp_configs (
    id SERIAL PRIMARY KEY,
    user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    tool_namespace VARCHAR(100) NOT NULL, -- e.g., 'notion', 'email'
    encrypted_credentials BYTEA NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT true,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE (user_id, tool_namespace)
);

CREATE INDEX IF NOT EXISTS idx_user_mcp_configs_user_id ON user_mcp_configs(user_id);
-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin
DROP INDEX IF EXISTS idx_user_mcp_configs_user_id;
DROP TABLE IF EXISTS user_mcp_configs;
-- +goose StatementEnd
