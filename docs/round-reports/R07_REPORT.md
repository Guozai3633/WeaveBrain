# 织脑（WeaveBrain）R7 报告 — 验收 B：核心捕捉与 AI 控制（G5）

> 轮次：R7 — 验收 B：核心捕捉与 AI 控制
> 目标：G5（原始捕捉丢失为 0 / AI 关闭时后台整理调用为 0 / 95% 任务最终 ready 或明确失败 / 原音频、原文与用户修正版可追溯 / 20 名内测样本 >60% 认为卡片表达原意 / 严重权限与数据串用户问题为 0 / R7 验收报告）
> 日期：2026-08-30
> 范围：**后端**（5 个 /api/v1 跨用户漏洞 + timeline 恒 401 bug + outbox 孤儿回收 + 真实 PG 集成测试设施 + 4 个集成测试新发现的真实缺陷修复）+ 验收证据（独立测试库 + make 目标 + 5 个 R7 集成测试）+ 文档（R07 报告 / 人工 UX 清单 / v1 安全契约）
> 前端不动（R7 后端专用；G5 通过标准 5 为纯人工验收）

## 1. 阶段结论

R7 后端验收全部完成并**在真实 PostgreSQL 上全绿**：

- `go build ./...` / `go vet ./...` 通过；
- `go test ./...`（无 env，单元 + 迁移契约，集成测试自动 skip）**236** 用例全绿（R6 227 → +9）；
- `WEAVEBRAIN_TEST_DATABASE_URL=<test-db> go test ./...` **242** 用例全绿 —— 6 个环境门控集成测试首次在真实 PG 上实际运行并通过（含 5 个 R7 新增 + 既有 R3/R4 集成回归）。

**关键收获**：真实 PG 集成运行暴露了 4 个仅靠单测（fake pgx Conn）无法发现的**真实缺陷**——迁移 000009 遗留的 `capture_outbox.status` 默认值 bug、音频 `Initiate` 未持久化 sha256（导致 `Complete` 空指针）、`DBStore` 从未接线 Timeline 仓储（timeline 端点恒 500）、`SetLifecycle` 的 `$3` 类型推断歧义（SQLSTATE 42P08）。这正是验收「不跑集成测试不合并」的意义。4 个缺陷已全部修复并补充契约/回归断言。

G5 七项完成条件：6 项自动化证据全部达成；第 5 项（20 名内测样本 >60%）为纯人工验收，清单已产出（`R07_MANUAL_UX_SAMPLE.md`），**待真机执行回填**。详见 §6。

## 2. 交付能力

| 能力 | 说明 |
|---|---|
| v1 跨用户收敛 | `/api/v1/ideas` 列表用户作用域（他人项目 → 空列表）；`POST /ideas` 项目归属校验（他人 → 403）；`GET /reminders` 用户作用域（新 `GetPendingByUser`）；`GET /agent/workflow/:id` 仅本人（他人 → 403）；`GET /users/:id` 仅本人（他人 → 404）；`GET /users/me/timeline` 修复恒 401 |
| outbox 孤儿回收 | `ClaimDue(ctx, limit, lease)` 重领租约过期（`updated_at <= now()-lease`）的 stale `processing` 行；Go 侧预计算 cutoff 绑 `timestamptz` 规避 pgx Duration→interval codec；`FOR UPDATE SKIP LOCKED` 防双处理；`attempt_count+1` 仍受 `MaxAttempts` 封顶；`OutboxWorkerConfig.ClaimLease` 默认 5min |
| stale-processing 索引 | 迁移 000011：`idx_capture_outbox_stale_processing` 部分索引 `(status, updated_at) WHERE status='processing'` |
| 可追溯链 | 捕捉创建原子写原文（不可变）+ fallback 卡 + fallback 修订；音频上传全文件 sha256；STT 转写 → stt 修订；用户修正 → user 修订 + card.version+1；原音频/原文/各修订全部保留可溯 |
| 集成测试设施 | 独立 `weavebrain_test` 库（docker 容器内幂等创建）；`make db-create-test / migrate-test-up / test-integration / test-integration-fast`；README 文档化；无 env 自动 skip |
| v1 安全契约 | `docs/API_V1_SECURITY_CONTRACT.md`：R7 行为变更契约（漏洞 → 状态码、404 vs 403 语义、GetPendingByUser 偏离、残留暴露） |

## 3. 设计要点

- **GetPendingByUser 偏离**：Temporal cron `GetPendingRemindersActivity` 需要系统级 `GetPending`（遍历全用户投递），故**保留**系统级方法；HTTP 层改用新增用户级 `GetPendingByUser`。此为有据偏离，记入安全契约。
- **孤儿重领的租约语义**：`ClaimDue` 用 `updated_at` 作租约锚点（worker 每次处理都 `UPDATE ... SET updated_at=now()`，见 migration 000009 触发器/worker 写入），超过 lease（默认 5min）视为崩溃遗留。cutoff 在 Go 侧预计算为 `time.Time` 绑 `$2`，语义等同 `now()-lease`。
- **修订而非覆写（延续 R6）**：原文 `original_text` 全程只读；转写与卡片修正只**追加**修订（`transcript_revisions` / `memory_card_revisions`），card.version 单调递增。可追溯链由此自然成立。
- **集成测试共享库隔离**：6 个环境门控测试共享 `weavebrain_test` 库，各测试用随机 UUID 不撞主键；但 `ClaimDue` 为全局操作（按 `next_run_at,id` 排序），孤儿回收与 soak 测试在种数据前先 `DELETE FROM capture_outbox`，保证断言只看自己种的行。
- **真实 PG 发现的三类缺陷**：迁移默认值与 CHECK 不一致、仓储未持久化关键列（sha256）、仓储接线遗漏（Timeline）——全部在真实 PG 上才能暴露。

## 4. 测试情况

### 后端单元 + 契约（无 env，236 全绿；R6 227 → +9）

- **idea_repository_test.go（新，2）**：`GetByProjectID` SQL 含 `AND user_id = $2`、args 序 `[projectID, userID, limit, offset]`；`Search` base 含 user_id；
- **reminder_repository_test.go（新，1）**：`GetPendingByUser` SQL `WHERE user_id = $1 ... trigger_time < $2 ... LIMIT $3`；
- **idea_service_test.go（新，3）**：Create 他人项目 → `ErrProjectForbidden` 且不调 `Idea.Create`；本人项目成功；ListByProject 透传 userID；
- **outbox_repository_test.go（更新，+1）**：ClaimDue 新签名；新增「回收 stale processing」断言 SQL 含 `status='processing' AND updated_at <= $2` 且 lease 参数为窗口内 `time.Time`；
- **migration 契约（+2）**：000011 stale-processing 部分索引 Up/Down/谓词；000012 status 默认值 Up/Down；
- **memory_repository_test.go（更新）**：SetLifecycle SQL 断言随 `$3::text` 修复同步；
- **既有回归**：api / migration / repository / service / stt / crypto 全部保持绿。

### 后端集成（真实 PostgreSQL，242 全绿；6 个环境门控测试实际运行）

| 测试 | 位置 | 证明的 G5 标准 |
|---|---|---|
| TestR7CrossUserSecurityAcceptance | internal/api | 标准 6：B 读 A 的 ideas → 空列表；B 写 A 的 project → 403；B 读 reminders → 空；B 读 users/:id(A) → 404；B 读 A 的 workflow → 403；A 读 timeline → 200（原恒 401）；A 回归读自己数据正常 |
| TestOutboxOrphanRecoveryIntegration | internal/db/repository | 标准 3：stale `processing`（updated_at=now-10min）被 `ClaimDue(10, 5min)` 重领且 `attempt_count+1`；fresh `processing` 不回收；due `queued` 正常领取 |
| TestR7BatchSoakIntegration | internal/service | 标准 1/2/3：40 条混合 outbox（20 AI 开正常、8 AI 关、3 FAILME 重试耗尽、3 捕捉缺失、3 stale、3 预取消）全部终态，终态率 40/40=100% ≥ 95%；ready=31 / failed=6 / cancelled=3；AI 关行 pipeline 零调用；stale 行终态 ready |
| TestR7TraceabilityChainIntegration | internal/service | 标准 4：音频捕捉全链（Create → Initiate/Chunks/Complete 全文件 sha256 → STT 修订 → 用户转写修订 → 用户卡片修正 card.version=2）；原音频仍存在且 sha256 不变、原文未改写、转写修订 `[stt,user]`、卡片修订 `[fallback,user]` |
| TestCaptureRepositoryPostgresIntegration | internal/db/repository | 既有 R4 集成回归（真实 PG 实际运行） |
| TestR3CaptureFullHTTPPostgresAcceptance | internal/api | 既有 R3 全 HTTP 集成回归（真实 PG 实际运行） |

## 5. 修复的问题

### 修复的确认漏洞 / bug（A1–A7）

1. **VULN-1 CRITICAL** — `GET /api/v1/ideas` 跨用户读：`GetByProjectID` / `Search` 增加 `user_id`，列表限定当前用户项目；
2. **VULN-2 CRITICAL** — `POST /api/v1/ideas` 跨用户写：IdeaService 项目归属哨兵 `ErrProjectForbidden`，handler 映射 403；
3. **VULN-3 HIGH** — `GET /api/v1/reminders` 跨用户读全部待办：新增 `GetPendingByUser`，HTTP 用它；
4. **VULN-4 MEDIUM-HIGH** — `GET /api/v1/agent/workflow/:id` 跨用户读：取回后比对 `workflow_runs.user_id` → 403；
5. **VULN-5 LOW** — `GET /api/v1/users/:id` 他人资料可读：仅本人，他人一律 404（不泄露存在性）；
6. **BUG-1** — `GET /api/v1/users/me/timeline` 恒 401：中间件 `c.GetString` 类型断言失败，改用 `getCurrentUserID`。

### 真实 PG 集成运行新发现的缺陷（本次修复）

7. **迁移 000009 遗留默认值 bug**：`capture_outbox.status` DEFAULT 仍为 `'pending'`，而 CHECK 已改为 `('queued',...,'cancelled')`；任何省略 `status` 的 INSERT（如 `CaptureRepository.Create` CTE）在真实 PG 上直接违反约束，**生产新建捕捉必崩**。修复：迁移 000012 `ALTER COLUMN status SET DEFAULT 'queued'` + 契约测试；
8. **音频 Initiate 未持久化 sha256**：`AudioAssetRepository.Initiate` INSERT 漏了 `sha256` 列，`Complete` 解引用 `*asset.SHA256` 空指针（traceability 集成测试暴露）。修复：INSERT 补 `sha256` 列；
9. **DBStore 从未接线 Timeline 仓储**：`NewFromPool` / `BeginTx` 未初始化 `store.Timeline`，timeline 端点恒 500（cross-user 集成测试暴露）。修复：store.go 补 `Timeline: NewTimelineRepository(conn)`；
10. **SetLifecycle `$3` 类型推断歧义**：`$3` 同时用于 varchar 赋值与 `CASE WHEN $3 IN (...) ` 与 unknown 字面量比较 → SQLSTATE 42P08（soak 集成测试暴露）。修复：`$3::text` 显式转型 + 单测断言同步；
11. **集成测试共享库污染**：孤儿回收与 soak 测试对全局 `ClaimDue` / 全表状态计数断言，会读取其他测试遗留行。修复：两测试种数据前 `DELETE FROM capture_outbox`。

## 6. 完成条件核对（G5）

- [x] 原始捕捉丢失为 0 —— traceability 集成测试：原音频始终存在且 sha256 不变、`captures.original_text` 全程只读（音频捕捉恒 NULL 不被改写）；R3/R4 集成回归在真实 PG 通过
- [x] AI 关闭时后台整理调用为 0 —— soak 集成测试：8 条 AI 关快照行直接 MarkReady，`r7SoakPipeline` 对 AI 关 capture 调用数为 0
- [x] 95% 可处理任务最终 ready 或明确失败 —— soak 集成测试：40/40 = 100% 终态（ready=31 + failed=6 + cancelled=3）≥ 95%；无残留 queued/retry_wait/processing
- [x] 原音频、原文和用户修正版均可追溯 —— traceability 集成测试：音频资产（sha256）+ stt 转写修订 + user 转写修订 + fallback 卡修订 + user 卡修订全部保留，修订源计数 `[stt:1,user:1]` / `[fallback:1,user:1]`
- [ ] 20 名内测体验样本中，超过 60% 认为卡片表达原意 —— **纯人工验收**：清单已产出（`R07_MANUAL_UX_SAMPLE.md`，20 条短语 + 预期含义 + 判定 ≥65%），待真机执行回填（R7 为后端轮，真机语音链路需设备验证）
- [x] 严重权限与数据串用户问题为 0 —— cross-user 集成测试 + v1 安全契约（ideas/reminders/users/workflow/timeline 全量收口）
- [x] 形成 R7 验收报告 —— 本报告（并标记 G5 自动化证据达成；人工项待回填）

## 7. 已知限制与后续

- **G5 标准 5（20 样本）待人工**：需真机 + STT（mock/funasr）执行 `R07_MANUAL_UX_SAMPLE.md`，回填结果到本报告 §6。
- **残留暴露（不在本次清单）**：`SearchByTags` / `SearchBySimilarity`（embedding 路径）仍仅 project 作用域，未做用户收口；R7 记入安全契约，后续轮次处理。
- **`captures.version` 恒 1** → `source_revision` 恒 1：语义缺口非泄漏（原文不可变 + 修订追加），R7 不修，记入安全契约。
- **迁移需应用到 dev 库**：`capture_outbox.status` 默认值修复（000012）与 stale-processing 索引（000011）需对开发库 `goose up`；`make migrate-up` 即可。
- **根目录 `E:\织脑\docker-compose.yml` 为旧重复副本**：开发以 `WeaveBrain/docker-compose.yml` 为准，后续清理。
- **真实 AI pipeline 仍为 no-op**：G6/R8 以 `ai` 修订接入真实整理/embedding/relate/recap；届时 `source_revision` 指向实际转写修订。

## 8. 下一步（G6 / R8）

AI 补全：CompletionProposal / FieldProposal 模型、completion:preview / completion:apply、evidence_spans / safe_auto / suggest_only / forbidden、逐字段采用 + 版本与撤销、详情页补全入口、旧提案过期。

## 9. 证据

- `go build ./...` / `go vet ./...`：通过
- `go test ./...`（无 env）：**236** 用例全绿（集成测试自动 skip）
- `WEAVEBRAIN_TEST_DATABASE_URL=postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test go test ./...`：**242** 用例全绿（6 个集成测试实际运行；含 R7 新增 4 个 + R3/R4 回归 2 个）
- `make db-create-test` / `make migrate-test-up`（goose v3.27.1，迁移 000001—000012）：通过
- docs/API_V1_SECURITY_CONTRACT.md（v1 行为变更契约）
- docs/round-reports/R07_MANUAL_UX_SAMPLE.md（20 样本人工验收清单，待执行）
- docs/DEVELOPMENT_GOALS.md G5（自动化证据勾选；标准 5 待回填）；MEMORY.md Sprint 19
