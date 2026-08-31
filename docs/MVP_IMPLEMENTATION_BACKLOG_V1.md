# 织脑（WeaveBrain）MVP 实施 Backlog V1

> 日期：2026-08-16  
> 状态：产品蓝图冻结后的开发执行基线  
> 上游文档：PRODUCT_BLUEPRINT_V3.md、DOMAIN_AND_AI_SPEC_V3.md、ROADMAP_V2.md  
> 执行轮次与状态总控：DEVELOPMENT_ROUNDS_PLAN_V1.md  
> 原则：用可独立验收的纵向切片推进；旧 V1 能力先隔离、后迁移，不在同一个提交里推翻全部代码。

## 1. 开工基线

### 1.1 已验证

- Go 全量测试当前通过；
- 现有后端仅 pkg/crypto 有实质测试，多数核心包显示 no test files；
- 当前核心实体是 Idea，且 project_id 强制存在；
- Idea 创建后使用裸 goroutine 生成 embedding；
- 捕捉流程先走 WebSocket 转写和 Agent，再由用户选择项目保存；
- Flutter 主导航仍是“录音 / 想法 / 动态 / 设置”；
- 设置页仍以 STT Provider、MCP 和项目管理为主；
- Flutter 尚无本地 Capture 数据库、离线队列和可恢复音频资产管理。

### 1.2 当前工具链问题

Flutter analyze 在当前中文工作区路径下因 Analysis Server 的 LSP JSON 输入截断而退出，尚不能据此判断代码是否有静态分析错误。BASE-001 必须先在 ASCII 路径或 CI 环境复现并恢复可靠分析。

## 2. 迁移策略

### 2.1 后端采用 V1/V3 并行过渡

- 保留现有 /api/v1/ideas，暂不让旧客户端立刻失效；
- 新增 /api/v3/captures、/memory-cards、/users/me/ai-settings 等契约；
- 新表与旧 ideas 表并行，禁止直接把 ideas 一次性改名为 captures；
- V3 主链路稳定后，再编写一次性迁移工具把旧 Idea 转换为 Capture + MemoryCard；
- Temporal、ReAct、MCP 和 Timeline 不进入单条 Capture 热路径。

### 2.2 Flutter 采用新 feature 并行替换

新增目录：

- lib/features/capture：语音/文字捕捉、本地状态机；
- lib/features/memories：记忆流、详情、搜索；
- lib/features/ai_settings：AI 总开关、补全与隐私设置；
- lib/features/imports：单条/批量导入；
- lib/features/workflows：仅预留页面；
- lib/shared/local_store：本地 Capture、资产和同步队列接口。

旧 ideas、timeline、projects 页面先从主导航隐藏，不立即删除，方便迁移对照和回滚。

### 2.3 数据真实性规则

- Capture 保存成功不依赖 STT、LLM、Embedding 或工作流；
- project/collection 可空，默认进入 Inbox；
- 原文、音频和用户修正版不可被 AI 覆盖；
- 所有读写在 Repository 层携带 user_id；
- AI 开关在创建 Capture 时形成策略快照；
- AI 补全先生成 FieldProposal，apply 才产生新版本；
- 工作流预留接口永远不得意外创建真实运行任务。

## 3. MVP 关键路径

依赖顺序：

1. BASE：工具链和 V3 契约；
2. CAP：Capture 数据模型、幂等 API 和所有权；
3. LOCAL：手机/Web 本地优先存储与同步队列；
4. AUDIO：可靠音频资产和独立 STT；
5. AISET：AI 总开关、转写开关与设置快照；
6. PROCESS：Outbox、ProcessingTask 和轻量整理；
7. MEMORY：记忆流、详情和全文搜索；
8. COMPLETE：AI 补全提案、预览、应用和撤销；
9. IMPORT：单条/批量导入；
10. QUICK：移动系统快捷入口；
11. WORKFLOW：页面和接口预留；
12. RELEASE：隐私、安全、监控与端到端验收。

任何后续 Epic 不得绕过 CAP 和 LOCAL 直接调用 Agent。

## 4. Epic 总表

| Epic | 目标 | 优先级 | 依赖 | 相对规模 |
|---|---|---:|---|---:|
| BASE | 恢复可靠构建、分析、测试和 V3 契约 | P0 | 无 | S |
| CAP | 原始记录无项目、幂等、安全写入 | P0 | BASE | M |
| LOCAL | 断网和崩溃后记录仍存在 | P0 | CAP 契约 | L |
| AUDIO | 音频先落盘、可重试上传、独立转写 | P0 | LOCAL | L |
| AISET | 用户能决定纯备忘录或智能整理 | P0 | CAP | M |
| PROCESS | 可恢复的后台整理任务 | P0 | CAP、AISET | L |
| MEMORY | 可用的记忆流、卡片和详情 | P0 | CAP、PROCESS | L |
| COMPLETE | AI 补全预览、应用、版本和撤销 | P0 | MEMORY、AISET | L |
| IMPORT | 低门槛单条/批量导入 | P0 | CAP、COMPLETE | L |
| WEB | Web 回顾、搜索、导入和设置 | P0 | MEMORY、IMPORT | M |
| QUICK | 手机系统级快速录音入口 | P1 | AUDIO | M |
| RESURFACE | 一种可关闭的回响 | P1 | MEMORY | M |
| WORKFLOW | 页面、路由、Schema 与能力预留 | P1 | WEB | S |
| RELEASE | 安全、隐私、删除、导出、监控 | P0 | 全部 | L |

S/M/L 只用于依赖和拆批，不作为上市日期承诺。

## 5. 详细任务

### BASE：开发基线

#### BASE-001 Flutter 分析环境

- 在 ASCII 路径映射或 CI 中运行 flutter analyze、flutter test；
- 固定 Flutter/Dart 版本；
- 记录依赖解析结果，避免每次分析隐式升级；
- 将 Analysis Server 异常与真实代码错误分开。

验收：flutter analyze 与 flutter test 能稳定运行三次；失败时输出可定位的源码错误而不是 LSP JSON 截断。

#### BASE-002 V3 API 与错误码约定

- 冻结 /api/v3 路由前缀；
- 统一错误体：code、message、request_id、details；
- 定义 IDEMPOTENCY_CONFLICT、VERSION_CONFLICT、FEATURE_NOT_ENABLED；
- 统一 cursor、时间、UUID 和枚举序列化。

验收：契约测试覆盖 201、200 幂等重放、400、401、403、404、409、501。

### CAP：Capture 核心

#### CAP-001 数据库迁移

新增：

- captures；
- memory_cards 的 fallback 最小字段；
- capture_outbox；
- 必要唯一约束和 user_id 索引。

暂不在首个迁移加入 Embedding、Relation、WorkflowRun 等非保存必需字段。

拟修改文件：

- WeaveBrain/internal/db/migration/000006_create_capture_core.sql；
- WeaveBrain/internal/entity/capture.go；
- WeaveBrain/internal/entity/memory_card.go。

验收：

- project_id/collection_id 可空；
- 唯一约束 (user_id, capture_id)；
- captured_at 与 created_at 分离；
- Down 迁移只删除本迁移创建的对象。

#### CAP-002 Repository 与所有权

新增 CaptureRepository：

- Create；
- GetByID(ctx, userID, captureID)；
- List(ctx, userID, cursor, filters)；
- UpdateRaw(ctx, userID, captureID, expectedVersion)；
- Trash/Delete。

拟修改文件：

- WeaveBrain/internal/db/repository/interface.go；
- WeaveBrain/internal/db/repository/capture_repository.go；
- WeaveBrain/internal/db/repository/store.go。

验收：Repository 的 ID 查询无法省略 user_id；跨用户读取返回 not found/forbidden 的统一策略。

#### CAP-003 幂等创建 API

实现 POST /api/v3/captures：

- 客户端 UUID 作为 capture_id 和 Idempotency-Key；
- 保存原始文字或资产占位；
- 同时创建 fallback MemoryCard；
- 同事务写 Outbox；
- 不等待任何模型；
- 不要求 project_id。

拟修改文件：

- WeaveBrain/internal/service/capture_service.go；
- WeaveBrain/internal/api/capture_handler.go；
- WeaveBrain/internal/api/server.go；
- 对应测试文件。

验收：

- 同一 UUID、同一内容提交 100 次只有一条；
- 同一 UUID、不同内容返回 409；
- STT、LLM、Worker 全停仍返回成功；
- 其他用户不能读取或覆盖；
- P95 目标小于 500ms。

### LOCAL：本地优先客户端

#### LOCAL-001 LocalCaptureStore 接口

先冻结接口，再分别实现手机与 Web 存储：

- saveDraft；
- saveAudioAsset；
- markPendingSync；
- listPending；
- applyServerRevision；
- markConflict；
- trash。

移动端使用持久数据库和应用私有文件；Web 使用浏览器持久存储。具体库先做一日 Spike，要求 UUID、事务、索引和 Web 支持，不允许只用内存 Provider。

#### LOCAL-002 Capture 状态机

状态拆分为：

- recording；
- saved_local；
- pending_sync；
- syncing；
- synced；
- processing；
- ready；
- retryable_error。

本地同步状态不得与服务端 AI processing_status 共用一个枚举。

#### LOCAL-003 文本纵向切片

- 手机/Web 输入文字；
- 点击保存后先写本地；
- UI 立即显示“已安全保存”；
- 后台调用 /api/v3/captures；
- 失败进入待同步，不丢草稿；
- 记忆流立即读取本地投影。

验收：飞行模式创建 20 条，杀进程并重启后全部存在；恢复网络后无重复。

### AUDIO：可靠语音

#### AUDIO-001 音频先落盘

废弃“只把 PCM 流发给 WebSocket”的唯一保存方式：

- 开始录音即创建本地 Capture UUID；
- 音频写应用私有文件；
- 流式字幕只作为即时反馈；
- 停止后先完成本地文件和 Capture 保存；
- 完整音频再分片/重试上传并校验 SHA-256。

#### AUDIO-002 独立 STT

- speech_to_text_enabled 独立于 ai_memory_enabled；
- 最终转写基于完整音频，不基于 WebSocket 尾句；
- 转写生成 TranscriptRevision；
- 用户修正生成新 revision；
- 关闭转写时仍保留并同步原始音频策略。

验收：断网录音、尾句、权限拒绝、麦克风占用、应用重启和上传重试均有测试。

### AISET：AI 总开关

#### AISET-001 UserAISettings

新增 user_ai_settings 表、Repository、Service：

- ai_memory_enabled；
- ai_completion_enabled；
- speech_to_text_enabled；
- allow_cloud_text/audio；
- completion_mode；
- revision；
- consented_at。

API：

- GET /api/v3/users/me/ai-settings；
- PATCH /api/v3/users/me/ai-settings。

验收：使用 expected_revision；多设备冲突返回 409；默认 ai_memory_enabled=false。

#### AISET-002 “我的”设置页面

新增：

- AI 记忆整理总开关；
- AI 补全开关；
- 语音转写开关；
- 云端文本/音频说明；
- 待处理任务提示；
- “补整理未处理记忆”显式操作。

验收：关闭 AI 后子能力不执行但偏好值保留；手机与 Web 展示同一 revision。

### PROCESS：后台任务

#### PROCESS-001 Outbox Worker

- PostgreSQL Outbox + FOR UPDATE SKIP LOCKED；
- Worker lease、attempt、next_run_at；
- task_type：stt、organize、complete_fields、embed、relate、recap；
- 同一 source_revision 幂等；
- 禁止裸 goroutine 承担业务任务。

#### PROCESS-002 设置驱动分流

- Capture 创建时快照 ai_policy_revision；
- AI 关闭时不创建 organize/embed/relate/recap；
- 关闭设置后取消 queued/retry_wait，running 可完成；
- 再开启只影响新 Capture；
- 手动 organize-once 单条授权。

验收：AI 关闭后新增 50 条，后台整理模型调用数为 0。

### MEMORY：记忆卡

#### MEMORY-001 fallback 卡

没有 AI 时：

- title 使用正文前 20—30 字；
- primary_type 为未分类或稳定默认值；
- 原文、时间、来源和状态可见；
- 不出现“整理失败”的误导提示。

#### MEMORY-002 enrichment

模型只输出：

- title；
- essence；
- 最多 3 个 key_points；
- 最多 1 个 open_question；
- 最多 1 个 next_step；
- suggested_type/tags。

所有字段记录 provenance、source_revision、prompt/schema/config version。

#### MEMORY-003 记忆流与详情

主导航调整为：

- 记忆；
- 中央全局捕捉；
- 回响（数据不足时藏在记忆页）；
- 我的。

Timeline、Projects、MCP 从主路径移除或隐藏。

验收：详情按 ID 请求，不依赖路由 extra；刷新 Web 详情页可恢复。

### COMPLETE：AI 补全

#### COMPLETE-001 补全提案

实现 CompletionProposal / FieldProposal：

- preview 不改原对象；
- 事实型字段必须有 evidence_spans；
- safe_auto、suggest_only、forbidden；
- source_revision 变化后提案过期。

#### COMPLETE-002 应用与撤销

API：

- POST /captures/{id}/completion:preview；
- POST /captures/{id}/completion:apply；
- POST /imports/{id}/completion:preview；
- POST /imports/{id}/completion:apply。

验收：

- 默认只填空字段；
- 不覆盖用户值或导入原值；
- 逐字段采用；
- apply 幂等；
- 可撤销到前一个 revision。

### IMPORT：导入

#### IMPORT-001 单条导入

- 粘贴文字或上传音频/文件；
- 标题、时间、来源、标签、合集均选填；
- 缺时间保持 unknown；
- AI 补全在预览页显式触发。

#### IMPORT-002 批量导入

- 多段文本、TXT、Markdown、CSV、JSONL；
- 解析、字段映射、抽样预览、去重、Commit 分阶段；
- 单行失败不回滚整个批次；
- 补全失败允许按原样导入；
- 结果报告可下载。

### WEB：Web 回顾端

MVP 页面：

- 记忆流；
- 搜索与筛选；
- 完整详情；
- 单条/批量导入；
- AI 与隐私设置；
- 数据导出/删除；
- 工作流规划入口。

Web 不承诺浏览器关闭后的后台录音或系统级快捷键。

### QUICK：移动快捷入口

- App 内中央捕捉按钮；
- Android/iOS App Shortcut；
- 至少选择一端做桌面小组件；
- 进入极简录音页后一次动作开始；
- 声音/触觉确认开始与已保存；
- 不做永久监听唤醒词。

### WORKFLOW：只预留不执行

#### WORKFLOW-001 页面与路由

- /workflows；
- /workflows/:id/designer；
- 空状态、示例方案、“规划中”标记；
- 不制作会让用户误以为能运行的假编辑器。

#### WORKFLOW-002 能力接口

GET /api/v3/capabilities 返回：

- workflow_designer=false；
- workflow_execution=false。

写入、发布和运行请求统一返回 FEATURE_NOT_ENABLED，不创建 ProcessingTask、WorkflowRun 或外部副作用。

### RELEASE：上市门禁

- 真实登录、Token 轮换和设备会话；
- HTTPS/WSS，Token 不放 WebSocket URL；
- 数据导出、回收站、永久删除和账号删除；
- 音频保留策略；
- 日志脱敏；
- Crash、任务失败、成本和请求监控；
- 手机/Web 端到端测试；
- 30 名种子用户四周内测。

## 6. 第一个可实施纵向切片：S0 文本 Capture

### 目标

在不触碰语音、Agent、Embedding 和旧 Idea UI 的前提下，先证明“无项目也能安全、幂等地保存一条原始记忆”。

### 范围

只包含：

1. 000006 Capture core migration；
2. Capture/MemoryCard 最小实体；
3. user-scoped CaptureRepository；
4. POST 和 GET /api/v3/captures；
5. fallback MemoryCard；
6. Outbox 记录但暂不消费；
7. Repository、Service、Handler 测试。

明确不包含：

- STT；
- LLM；
- Embedding；
- Flutter UI；
- 导入；
- 工作流；
- 旧 Idea 数据迁移。

### S0 完成定义

- go test ./... 通过；
- 新增测试覆盖幂等、跨用户、无项目、空内容、版本冲突；
- 同 UUID 同正文重放返回相同 Capture；
- 同 UUID 不同正文返回 IDEMPOTENCY_CONFLICT；
- 响应不等待任何外部 Provider；
- 数据库迁移 Up/Down 可重复验证；
- /api/v1/ideas 行为不变。

## 7. 提交与验收规则

每个任务单独提交，至少包含：

- 迁移/契约；
- 实现；
- 自动化测试；
- 失败与降级路径；
- 对应文档更新。

禁止把以下内容混入 S0：

- 重写整个 Flutter 导航；
- 删除旧 Idea/Project/Timeline；
- 接入新的 Agent 框架；
- 恢复 MCP；
- 把 Temporal 放回 Capture 热路径；
- 提前开发可执行工作流。

## 8. 下一执行顺序

当前立即进入：

1. BASE-002：冻结 V3 错误体和路由骨架；
2. CAP-001：编写 000006 Capture core migration；
3. CAP-002：实现 user-scoped Repository；
4. CAP-003：完成 POST/GET API 和测试；
5. S0 验收后，再进入 LOCAL-001，不跨级开发 AI UI。
