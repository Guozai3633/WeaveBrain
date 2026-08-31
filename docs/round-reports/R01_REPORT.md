# 织脑（WeaveBrain）R1 验收报告

> 轮次：R1 — Capture 后端内核
> 日期：2026-08-16
> 结论：已通过
> 下一候选轮次：R2 — 本地优先文字捕捉（尚未开始）

## 1. 本轮结果

R1 已建立不依赖 Project、STT、LLM、Agent 或 Worker 的文字 Capture 保存内核。一次写入会在 PostgreSQL 中原子产生：

1. 原始 Capture；
2. 可立即展示的 fallback MemoryCard；
3. capture.created Outbox 事件。

新接口位于 /api/v3，旧 /api/v1/ideas 未删除、未改为新模型。

## 2. 已完成范围

- 新增 000006_create_capture_core.sql；
- 新增 Capture、MemoryCard、CaptureOutbox 最小实体；
- 新增显式携带 user_id 的 CaptureRepository；
- 新增 CaptureService，负责校验、规范化、请求哈希和 fallback 标题；
- 新增 POST /api/v3/captures；
- 新增 GET /api/v3/captures/{id}；
- 新增 V3 JWT 统一错误中间件；
- 新增 Idempotency-Key 与 capture_id 一致性约束；
- 新增真实 PostgreSQL 与纯单元/契约测试；
- 修复前置 000005 缺少 Goose Up/Down 标记、导致整个迁移链不可执行的问题。

## 3. 验收条件

| 条件 | 结果 | 证据 |
|---|---|---|
| project_id/collection_id 为空可保存 | 通过 | Service 100 次测试与 PostgreSQL 集成测试均未提供 collection |
| 同 UUID、同内容提交 100 次只有一条 | 通过 | 数据库最终为 1 Capture、1 MemoryCard、1 Outbox |
| 同 UUID、不同内容返回冲突 | 通过 | Service、Handler、PostgreSQL 三层测试 |
| 其他用户无法读取 | 通过 | SQL 强制 user_id + capture_id；跨用户返回 NOT_FOUND |
| 其他用户无法关联 collection | 通过 | (user_id, collection_id) 复合外键实库验证 |
| STT、LLM、Worker 不运行仍保存 | 通过 | CaptureService 只依赖 CaptureRepository |
| Up/Down 可验证 | 通过 | Goose 000001→000006 Up，000006 Down 后对象清空，再 Up 恢复 |
| V1 行为不变 | 通过 | V3 使用独立路由；既有全量测试及 V1 未知路由契约通过 |
| R1 自动化测试通过 | 通过 | go test ./...、go vet ./...、go build ./cmd/weavebrain 均成功 |

R1 没有增加修改或删除 Capture 的端点，因此不存在可被其他用户调用的 V3 修改/删除路径；后续增加这些端点时仍必须把 user_id 约束放入 SQL。

## 4. 真实数据库验收

环境：

- Docker PostgreSQL：pgvector/pgvector:pg16；
- Goose 迁移版本：6；
- 隔离数据库：weavebrain_r1_verify_20260816。

执行结果：

- 000001—000006 全部 Up 成功；
- 100 次相同 Capture Repository Create 成功，首次 replayed=false，其余 replayed=true；
- captures、memory_cards、capture_outbox 各 1 行；
- 同 ID 不同 request_hash 返回 ErrCaptureIdempotencyConflict；
- 其他用户 GetByID 返回 ErrCaptureNotFound；
- 其他用户引用 owner 的 project 被数据库复合外键拒绝，且 Capture 行数为 0；
- 000006 Down 成功，三张表与 projects 复合唯一约束均移除；
- 000006 再 Up 成功，三张表恢复；
- 验收后隔离数据库已删除，测试启动的 PostgreSQL 容器已停止。

## 5. 最终回归

后端：

- go test ./...：通过；
- go vet ./...：通过；
- go build ./cmd/weavebrain：通过；
- 带 WEAVEBRAIN_TEST_DATABASE_URL 的 PostgreSQL 集成测试：通过。

Flutter：

- scripts/flutter_check.ps1 -Task all：通过；
- flutter analyze：No issues found；
- flutter test：All tests passed。

## 6. 范围审计

本轮未实现：

- Flutter Capture UI；
- 本地离线队列；
- 音频上传和 STT；
- AI 总开关、AI 整理与补全；
- Import；
- Workflow；
- 旧 Idea 删除。

以上均保留在后续轮次，R1 没有提前扩展范围。

## 7. 下一步

R2 只负责手机端和 Web 端的本地优先文字捕捉、持久队列与同步状态机。开始 R2 前应先冻结客户端 Capture DTO、LocalCaptureStore 接口以及离线/重试状态转换，不在 R2 引入语音或 AI。
