-- +goose Up
CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE ideas ADD COLUMN embedding vector(768);

CREATE INDEX idx_ideas_embedding ON ideas USING hnsw (embedding vector_cosine_ops);

-- +goose Down
DROP INDEX IF EXISTS idx_ideas_embedding;
ALTER TABLE ideas DROP COLUMN IF EXISTS embedding;
