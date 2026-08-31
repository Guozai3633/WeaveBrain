# 织脑（WeaveBrain）R5 报告 — AI 总开关与后台任务（G3）

> 轮次：R5 — AI 总开关与后台任务
> 目标：G3（AI-optional，隐私优先，关闭即取消，显式补整理）
> 日期：2026-08-30
> 范围：后端（UserAISettings / 策略快照 / Outbox Worker / 生命周期 / 补整理端点）+ 前端（Flutter AI 设置页）

## 1. 阶段结论

R5 已全部完成并全绿：`go build ./...` / `go vet ./...` 通过，`go test ./...` **143** 个用例全绿；`dart analyze lib test` 无问题，`flutter test` **65** 个用例全绿。G3 九步 + 全部完成条件已达成：

- AI 记忆整理总开关默认 **false**（隐私优先），关闭时 worker 零 pipeline 调用；
- Capture 创建时**冻结策略快照** → 「再开启只影响新 Capture」；
- PostgreSQL **Outbox Worker**（FOR UPDATE SKIP LOCKED + 指数退避 + 单条 organize-once）承担后台整理；
- 关闭整理时**取消 queued/retry_wait** 事件（关闭即取消的显式语义）；
- **历史补整理必须显式触发**：POST `/users/me/ai-settings/reorganize`（仅 `ready + 快照关闭` 事件，AI 关时 409 FEATURE_NOT_ENABLED）；
- 手机 `/settings/ai` 页展示 5 个开关（含门控）+ **同一 revision**，与 Web/后端同源。

真机 / 端到端联调（含多设备 revision 冲突实操）留待设备验证。

## 2. 交付能力

| 能力 | 说明 |
|---|---|
| 设置读写 | `user_ai_settings` 表（迁移 000008）+ GET/PATCH `/api/v3/users/me/ai-settings`；乐观并发 revision（冲突 → 409 VERSION_CONFLICT） |
| 隐私优先默认 | `DefaultUserAISettings` 全 false；无行用户 GET 即物化默认，不要求先行创建 |
| 策略快照 | Capture 创建时把 5 个 AI 授权布尔冻结进 `capture_outbox.policy_snapshot`（同语句落库） |
| Outbox Worker | `FOR UPDATE SKIP LOCKED` 认领 + 4 阶段 pipeline（Organize/Embed/Relate/Recap）+ 指数退避重试 + 终态 failed/cancelled |
| 关闭即取消 | AI 记忆整理 开→关 时 `CancelByUser`（只取消 queued/retry_wait），worker 快照门控保证零 AI 调用 |
| 独立语音转写 | `speech_to_text_enabled` 独立于总开关：关闭整理仍可得到 TranscriptRevision |
| 历史补整理 | `CountPendingReorganize` + `ReorganizeByUser`；GET 信封含 `pending_reorganize`；POST `/reorganize` 显式重入队并按当前快照补跑 |
| Flutter 设置页 | `/settings/ai`：总开关、独立转写、3 个门控子开关（总开关关时置灰保留原值）、revision 行、「补整理未处理记忆 (N 条)」按钮 |

## 3. 设计要点

- **策略快照而非实时读设置**：worker 处理时按事件创建时的快照门控，天然满足「再开启只影响新 Capture」与「AI 失败不覆盖原始内容」的语义，也避免设置并发变更与后台任务交错。
- **关闭取消是优化而非正确性必需**：快照门控本身保证不再触发 AI；`CancelByUser` 尽力而为清理（失败仅日志），不阻塞设置写入。
- **补整理目标集精确**：`status='ready' AND policy_snapshot->>'ai_memory_enabled'='false'` —— 正是「AI 关闭期间创建 → worker 直接 MarkReady 未跑 pipeline」的记忆；不含 failed/cancelled（保持「关闭即取消」显式语义），且**仅显式触发**。
- **单条 organize-once**：每个事件一次完整 pipeline（processing 状态持锁），重试不重跑已成功阶段。
- **Flutter 三层可测抽象**：`AISettingsGateway`（API）+ `AISettingsNotifier`（sealed state）+ 页面；Riverpod `overrideWithValue` 注入 fake，覆盖加载/保存/409/补整理全链路。

## 4. 测试情况

### 后端（143 全绿）

- **outbox_repository_test.go（10）**：ClaimDue 解码 + FOR UPDATE SKIP LOCKED / 空集合 / MarkReady / 未找到 / MarkFailed / MarkRetryWait 退避 / CancelByUser 作用域 / CountQueued / **CountPendingReorganize** / **ReorganizeByUser（绑定 user_id+policyJSON、rowsAffected、0 匹配）**
- **outbox_worker_test.go（8）**：AI 关→零 pipeline ready / no_ai 跳过 / AI 开→全 pipeline / 失败→退避重试 / 超限→终态 failed / 捕获缺失→立即 failed / 退避指数与封顶 / Start-Stop 幂等
- **ai_settings_service_test.go（17）**：物化默认 / 部分补丁 / revision 冲突 / 新建与递增 / 关闭→取消 queued / 开启→不取消 / 无关开关→不取消 / **AI 关→ErrReorganizeAIMemoryDisabled** / **AI 开→委托 outbox 且快照含当前开关** / **outbox nil→count/补整理为 0 不 panic**
- **ai_settings_handler_test.go（14）**：GET 默认/鉴权 / PATCH 部分/非法/缺 revision/负 revision/无字段/冲突/无效/全字段 / **GET 含 pending_reorganize** / **POST /reorganize 200/409/401**
- **user_ai_settings_repository_test.go（6）+ migration 契约（3）+ capture_service/audio_service/其他（回归）**：快照落库 / 设置源故障→隐私兜底 / 无源→隐私兜底 等

### 前端（65 全绿）

- **ai_settings_api_test.dart（8）**：AISettings.fromJson 解析/缺省 / AISettingsResult 信封 / get / update 只发非空字段 + expected_revision / reorganize / 非法响应抛错
- **ai_settings_notifier_test.dart（7）**：load 成功/失败 / setSwitch 携带 revision 且只发目标字段 / 409→版本冲突提示 / reorganize 后计数刷新 / 409→请先开启提示 / clearMessage
- **ai_settings_screen_test.dart（5 widget）**：开关渲染 + revision / 门控子开关禁用 / 独立转写保存 + SnackBar / 总开关解锁门控 / 补整理按钮出现与点击后计数归零
- **既有回归**：capture 全量 + widget_test 全绿

## 5. 修复的问题

1. **cancelQueuedIfDisabled 守卫反转**：`wasEnabled || nowEnabled` → `!wasEnabled || nowEnabled`（关闭场景 wasEnabled=true 本应触发取消，原逻辑跳过）。
2. **pgx.Rows 测试 mock `Conn()` 类型**：pgx v5.10.0 中 `Rows.Conn()` 返回 `*pgx.Conn`（非 `*pgconn.PgConn`/`pgconn.Conn`）。
3. **Flutter analysis server 本机崩溃**：中文工作区路径导致 LSP `FormatException`，改用 `dart analyze lib test`（同口径 0 error / 0 warning）。

## 6. 完成条件核对（G3）

- [x] ai_memory_enabled 初始为 false（迁移 + 默认实体）
- [x] AI 关闭后新增 50 条，organize/embed/relate/recap 调用数为 0
- [x] 关闭整理但开启转写时仍可得到 TranscriptRevision
- [x] 多设备设置冲突返回 VERSION_CONFLICT
- [x] 再开启只影响新 Capture
- [x] 历史补整理必须显式触发（POST /reorganize + 手机页按钮）
- [x] 手机/Web 显示同一设置 revision（同源 GET /ai-settings 信封）

## 7. 下一步（G4 / R6）

记忆卡、记忆流与详情：fallback MemoryCard 完整字段、EnrichmentRevision + provenance + source_revision、记忆流默认倒序 + 基础筛选 + 全文搜索、记忆详情按 ID 加载（原文/音频/转写）、修正/续写/置顶/归档/删除、主导航调整。

## 8. 证据

- `go build ./...` / `go vet ./...`：通过
- `go test ./...`：143 用例全绿
- `dart analyze lib test`：0 error / 0 warning
- `flutter test`：65 用例全绿
- docs/API_V3_CONTRACT.md §9（§9.5 补整理端点）；docs/DEVELOPMENT_GOALS.md G3 九步 + 完成条件全部勾选
