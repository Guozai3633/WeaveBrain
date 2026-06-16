---
paths:
  - "WeaveBrain/internal/db/**"
  - "migrations/**"
---

# 数据库开发规则 (PostgreSQL)

## 表命名
- 表名: 复数、小写、snake_case（`ideas`, `projects`, `identities`）
- 关联表: 两个实体名排序后加 `_`（`user_identities`）

## 字段命名
- 主键: 实体名小写 `id`（`idea_id` 或自增 `id BIGSERIAL`）
- 外键: 单数表名 `_id`（`user_id`, `project_id`）
- 时间戳: `created_at`, `updated_at`, `deleted_at`（TIMESTAMPTZ）
- 软删除: 所有主实体必须有 `deleted_at TIMESTAMPTZ`

## 索引策略

| 索引类型 | 适用场景 | 示例 |
|---|---|---|
| btree | 等值查询、范围查询 | `CREATE INDEX idx_ideas_project ON ideas(project_id, deleted_at)` |
| GIN | JSONB 列、数组 | `CREATE INDEX idx_ideas_tags ON ideas USING GIN(tags)` |
| GIST / IVFFLAT | 向量相似度 | `CREATE INDEX idx_ideas_embedding ON ideas USING ivfflat (embedding vector_cosine_ops)` |

## 迁移规范

- 使用 `goose`，版本号递增
- 每个迁移文件必须包含 `Up` 和 `Down`
- 迁移文件命名: `000001_create_users_table.up.sql`
- 禁止在迁移中做破坏性操作（如 DROP COLUMN），用单独的迁移处理

## 事务

```go
tx, err := db.BeginTx(ctx, nil)
if err != nil {
    return err
}
defer tx.Rollback()
// 操作...
return tx.Commit()
```

- 多操作原子性用事务
- 嵌套操作用 Savepoint
- 不要在一个事务中做耗时操作（如 LLM 调用）

## 查询规范

- 通过 Repository 层访问数据库，不直接在 Handler 中写 SQL
- 使用 `pgx` 或 `gorm`，优先推荐 `pgx`（性能好、类型安全）
- 避免 N+1 查询，用 JOIN 或批量查询
