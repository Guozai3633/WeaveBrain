# WeaveBrain 织脑 — 后端

以语音为入口、Agent 为引擎的个人思维外脑。Go 后端（Eino Agent 编排 + Temporal 工作流调度 + golang-mcp），Flutter 移动端在 `../weave_flutter`。

## 快速开始

```bash
# 依赖：Go 1.22+、Docker、goose（数据库迁移）、make
docker-compose up -d postgres
goose -dir internal/db/migration postgres "postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain" up
go run ./cmd/weavebrain
```

## 测试

### 单元测试（无需数据库）

```bash
make test        # go test ./... -v
```

`WEAVEBRAIN_TEST_DATABASE_URL` 未设置时，所有真实 PostgreSQL 集成测试会自动 `t.Skip`，单元测试照常运行。

### 集成测试（真实 PostgreSQL）

集成测试连接独立的 `weavebrain_test` 库（与开发库 `weavebrain` 完全隔离，通过 docker 容器内的 `psql`/`createdb` 幂等创建）。

前置条件：

```bash
docker-compose up -d postgres   # 启动 weavebrain-postgres 容器
# 确保 goose 已安装：go install github.com/pressly/goose/v3/cmd/goose@latest
```

运行全量单元 + 集成测试：

```bash
make db-create-test      # 幂等创建 weavebrain_test 库
make migrate-test-up     # 对测试库执行 goose 迁移
make test-integration    # 全量：WEAVEBRAIN_TEST_DATABASE_URL=<test-db> go test ./... -v
```

只跑真实 PG 集成测试（跳过纯单元测试）：

```bash
make test-integration-fast
```

集成测试覆盖范围：

- **跨用户端到端**（`internal/api/r7_acceptance_integration_test.go`）：v1 数据串用户漏洞回归（ideas/reminders/users/workflow/timeline）。
- **可追溯链**（`internal/service/r7_traceability_integration_test.go`）：捕捉 → 音频上传（sha256）→ STT 转写 → 用户修正全链，原音频/原文/修订均保留。
- **批量 95% soak**（`internal/service/r7_soak_integration_test.go`）：~40 条混合 outbox 事件驱动 worker，全部到达终态且 ≥95% 可处理任务 resolve 为 ready 或明确失败。
- **孤儿回收**（`internal/db/repository/outbox_orphan_recovery_integration_test.go`）：`processing` 行租约过期后由 `ClaimDue` 重领。

## 目录

- `cmd/weavebrain` — 服务入口
- `internal/api` — HTTP / WebSocket 处理器
- `internal/service` — 领域服务与 outbox worker
- `internal/db/repository` — PostgreSQL 仓库
- `internal/db/migration` — goose 迁移
- `internal/workflow` — Temporal 工作流与 Activity

> 注：仓库根目录 `E:\织脑\docker-compose.yml` 是旧重复副本，开发以 `WeaveBrain/docker-compose.yml` 为准（后续清理）。
