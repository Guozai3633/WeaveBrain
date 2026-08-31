# 织脑（WeaveBrain）领域模型、导入与 AI 调用规范 V3

> 状态：开发前契约草案  
> 日期：2026-08-16  
> 核心约束：所有入口只负责产生 Capture；模型只负责产生可删除、可重做、有来源的派生结果。

## 1. 后端不变量

1. Capture Service 是原始记忆唯一写入者，Agent 无权决定是否保存。
2. 原始文字、原始音频和原始转写不可被 AI 覆盖。
3. 客户端生成 UUID，服务端以 `(user_id, capture_id)` 保证幂等。
4. 本地同步状态、服务端处理状态、记忆生命周期必须分开。
5. 所有异步任务持久化；禁止使用裸 goroutine 承担业务任务。
6. 模型失败、预算不足或用户关闭 AI，只影响派生内容。
7. 所有 Repository 读写显式携带 `user_id`，所有权约束进入 SQL。
8. 语音、文字、快捷入口、分享和导入统一进入 Capture Pipeline。
9. 所有补充字段记录来源：`user | device | import | inherited | ai`。
10. AI 不得把推测写成原始事实。

## 2. 逻辑领域模型

### 2.1 User

| 字段 | 必填 | 来源 | 说明 |
|---|---:|---|---|
| id | ✓ | system | UUID |
| display_name | — | user | 展示名 |
| timezone | ✓ | device/user | 默认时区 |
| locale | ✓ | device/user | 语言和区域 |
| account_status | ✓ | system | guest/active/suspended/deleting |
| created_at/updated_at | ✓ | system | 审计时间 |

### 2.1.1 UserAISettings

该对象是用户可见设置的唯一事实源，不能只存在客户端本地。

| 字段 | 类型/枚举 | 说明 |
|---|---|---|
| ai_memory_enabled | bool | AI 记忆整理总开关 |
| ai_completion_enabled | bool | 是否提供 AI 补全入口；不代表自动覆盖字段 |
| speech_to_text_enabled | bool | 语音转写独立开关 |
| completion_mode | suggest_only / auto_safe_fields | MVP 默认 suggest_only |
| allow_cloud_audio | bool | 是否允许上传音频处理 |
| allow_cloud_text | bool | 是否允许发送文本给模型 |
| pending_task_policy | finish_running_cancel_queued | MVP 固定策略 |
| enabled_at/consented_at | timestamp? | 用户明确开启和同意时间 |
| revision | integer | 设置版本，供 Capture 快照 |
| updated_at | timestamp | 审计时间 |

初始值：ai_memory_enabled=false、ai_completion_enabled=true、completion_mode=suggest_only。AI 补全只有在用户主动请求或导入预览明确勾选时才调用模型；它不能绕过 allow_cloud_text。语音转写与 AI 整理相互独立。
### 2.2 Capture

原始记忆事实，是系统最重要的对象。

| 字段 | 必填 | 来源 | 说明 |
|---|---:|---|---|
| id | ✓ | client | 客户端 UUID/幂等键 |
| user_id | ✓ | auth | 游客使用本地临时用户，登录后迁移 |
| kind | ✓ | client | audio/text/import/share |
| original_text | 条件必填 | user/import | 与资产至少存在一个 |
| captured_at | — | device/import/user | 真实发生时间，可未知 |
| captured_at_precision | ✓ | system/import | exact/date_only/estimated/unknown |
| created_at | ✓ | server | 服务端接收时间 |
| timezone | — | device/import | 不用 created_at 猜测 |
| source | ✓ | client/import | app/android_tile/ios_shortcut/widget/desktop_hotkey/share_sheet/import |
| context_snapshot_id | — | device/user | 可选情境 |
| collection_id | — | user/ai | 默认 Inbox，不阻塞保存 |
| privacy_mode | ✓ | user | cloud_allowed/no_ai；local_only 后续 |
| sync_revision | ✓ | system | 乐观锁 |
| lifecycle_status | ✓ | user/system | active/archived/trashed/deleted |
| client_version | ✓ | client | 客户端模型版本 |
| ai_policy_revision | ✓ | settings | 创建时快照的 UserAISettings.revision |
| ai_processing_requested | ✓ | settings/user | 本条是否允许自动整理；后续切换总开关不改写历史意图 |
| client_platform | ✓ | client | mobile_android/mobile_ios/web；其他端仅预留 |
| deleted_at | — | system | 软删除/级联删除依据 |

### 2.3 CaptureAsset

| 字段 | 必填 | 说明 |
|---|---:|---|
| id/capture_id/user_id | ✓ | 归属关系 |
| kind | ✓ | audio/image/file |
| object_key/local_ref | ✓ | 云端或本地引用 |
| mime_type/size_bytes | ✓ | 校验 |
| duration_ms | — | 音频时长 |
| sha256 | ✓ | 完整性与去重 |
| upload_status | ✓ | pending/uploading/uploaded/verified/rejected |
| retention_policy | ✓ | forever/30_days/delete_after_transcript/local_only |
| purge_at | — | 自动清理时间 |

### 2.4 TranscriptRevision

| 字段 | 必填 | 说明 |
|---|---:|---|
| id/capture_id | ✓ | 版本归属 |
| revision | ✓ | 单调递增 |
| raw_stt | — | Provider 原始输出 |
| cleaned_text | — | 清理后的转写 |
| user_corrected_text | — | 用户修正版，优先级最高 |
| language | — | 识别语言 |
| provider/model | — | 来源追踪 |
| confidence | — | 仅内部或高级详情使用 |
| source | ✓ | stt/ai/user/import |
| created_at | ✓ | 版本时间 |

当前生效文本使用明确的 `active_transcript_revision_id`，不得覆盖历史版本。

### 2.5 MemoryCard

MemoryCard 是 Capture 的当前用户投影，不是原始事实。

| 字段 | 必填 | 说明 |
|---|---:|---|
| id/capture_id/user_id | ✓ | 一条 Capture 对应一张主卡 |
| primary_type | ✓ | idea/question/action/reflection/reference |
| title | ✓ | 未整理时使用原文 fallback |
| essence | — | 一句话核心 |
| key_points | — | 最多 3 条 |
| open_question | — | 最多 1 条 |
| next_step | — | 最多 1 条 |
| processing_status | ✓ | pending/processing/ready/needs_input/failed |
| pinned | ✓ | 独立于生命周期 |
| inbox_state | ✓ | inbox/kept/archived |
| active_enrichment_revision_id | — | 当前 AI/用户版本 |
| open_count | ✓ | 聚合字段，不是事件真相 |
| last_opened_at | — | 常用排序 |
| continuation_count | ✓ | 续写数量 |
| relation_count | ✓ | 关联数量 |
| updated_at/version | ✓ | 排序和乐观锁 |

### 2.6 EnrichmentRevision

- title、essence、key_points、open_question、next_step、suggested_type、suggested_tags；
- 每个字段的 provenance 和 confidence；
- source transcript revision；
- provider/model、prompt_version、schema_version、config_version；
- created_at、accepted_at、rejected_at；
- 用户编辑产生新版本，不直接修改 AI 版本。

### 2.6.1 CompletionProposal / FieldProposal

AI 补全必须先产生提案，不能直接改 Capture 或 MemoryCard。

| 字段 | 说明 |
|---|---|
| id/user_id/subject_id | 归属 Capture、ImportRow 或 MemoryCard |
| source_revision | 提案所依据的正文/转写版本 |
| field_name | title/type/summary/tags/next_step/captured_at 等 |
| original_value/proposed_value | 前后对比 |
| provenance | ai/inherited/import/user/device |
| evidence_spans | 支持建议的原文片段；没有证据时不得建议事实字段 |
| confidence/risk_level | 内部决策与界面提示 |
| apply_policy | safe_auto/suggest_only/forbidden |
| status | pending/accepted/rejected/expired |
| provider/model/prompt/config_version | 可追溯 |
| accepted_by/accepted_at | 用户确认审计 |

同一 source_revision 的提案可幂等重算；正文变化后旧提案变为 expired。应用提案生成新的 EnrichmentRevision 或用户字段版本，并支持撤销。
### 2.7 ContextSnapshot

| 字段 | 阶段 | 说明 |
|---|---|---|
| captured_at/timezone | MVP | 时间情境 |
| device_type/app_entry | MVP | 手机、桌面、快捷入口 |
| activity | MVP/Beta | running/walking/commuting/unknown，用户或设备提供 |
| location_name | MVP 可选 | 粗粒度地点 |
| latitude/longitude/accuracy | Beta 可选 | 明确授权才保存 |
| weather | Later | 外部派生，不进入原始事实 |
| user_note | Beta | 用户补充“当时在做什么” |

地点为空时必须保持为空；模型不得猜测。

### 2.8 MemoryRelation

- from_card_id/to_card_id；
- relation_type：similar/supports/contradicts/continues/derived_from；
- score；
- reason；
- source：user/algorithm/ai；
- model/config version；
- user_feedback：useful/unrelated/hidden；
- created_at/expired_at。

MVP 每张卡最多展示 1—3 条关系。

### 2.9 MemoryInteraction

事件类型：

- opened；
- audio_played；
- edited；
- continued；
- reminder_created/reminder_completed；
- marked_useful/marked_unrelated；
- shared/exported；
- pinned/archived/deleted；
- resurfaced/resurface_dismissed。

字段：user_id、card_id、event_type、session_id、source_surface、occurred_at、metadata。

打开事件同一用户/卡片/会话或 30 分钟内只聚合一次有效回看。

### 2.10 Reminder / ResurfaceItem

Reminder：用户显式创建，包含 trigger_at、timezone、reason、status、delivery_channel。

ResurfaceItem：系统生成，包含 card_id、reason、score、scheduled_for、feedback、fatigue_key。

两者不能混成一个概念：提醒是用户承诺，回响是系统建议。

### 2.11 ImportJob / ImportRow

ImportJob：文件、格式、映射、总数、成功/失败/待补充数量、状态、错误报告。

ImportRow：row_number、external_id、raw_payload、normalized_payload、validation_errors、dedupe_status、capture_id、status。

### 2.12 ProcessingTask

| 字段 | 说明 |
|---|---|
| task_type | stt/organize/complete_fields/embed/relate/recap/import_parse/workflow_validate |
| subject_id | Capture/Card/Import ID |
| source_revision | 防止旧任务覆盖新内容 |
| status | queued/running/succeeded/retry_wait/needs_input/permanently_failed/cancelled |
| attempt_count/max_attempts | 重试控制 |
| next_run_at | 退避时间 |
| error_code/error_message | 安全化错误 |
| provider/model/config_version | 调用来源 |
| token/cost/duration | 成本统计 |
| lease_owner/lease_until | Worker 崩溃恢复 |

推荐唯一约束：

```text
(user_id, subject_id, task_type, source_revision, config_version)
```

## 3. Capture Pipeline

```text
本地音频/文字落盘
→ POST /captures 幂等创建原始记录
→ 音频初始化上传
→ 分片/可重试上传并校验 SHA-256
→ raw_ready + Outbox
→ STT
→ 轻量整理
→ Embedding
→ 关联记忆
→ MemoryCard ready
```

流式 WebSocket 只用于尽力而为的实时字幕，最终转写必须基于已校验的完整音频资产。

任何后一步失败不得回滚前一步。
### 3.1 设置驱动的分支规则

服务端创建 Capture 时读取 UserAISettings，并把 revision 与 ai_processing_requested 写入 Capture。客户端传来的同名字段只能作为提示，不能绕过服务端设置和隐私策略。

处理决策固定为：

1. 任意设置下都先保存原始记录并生成可用的 fallback 卡片；
2. 音频存在且 speech_to_text_enabled=true 时才进入 STT；
3. ai_memory_enabled=false 时，到原文/转写可用即停止，不创建 organize、embed、relate、recap 任务；
4. ai_memory_enabled=true 时，才按子开关创建 organize、embed、relate 和 recap 任务；
5. complete_fields 只在用户主动请求或导入预览明确勾选时创建，不因 ai_completion_enabled=true 自动运行；
6. 用户关闭总开关后，运行中的任务可结束，queued/retry_wait 的 AI 整理任务取消；不得影响 STT、同步和导出；
7. 再开启只影响新 Capture，历史记录必须通过显式“补整理”操作进入队列。

因此，“AI 已关闭”的可测试定义是：除用户单次主动请求外，新增记录的 organize/embed/relate/recap 模型调用数为零。

## 4. 核心 API 草案

### 4.1 创建 Capture

`POST /api/v3/captures`

Header：

```text
Authorization: Bearer <token>
Idempotency-Key: <capture UUID>
```

请求：

```json
{
  "capture_id": "5a59d96a-76dd-4dd2-bb03-6f67178ce4ab",
  "kind": "audio",
  "text": null,
  "captured_at": "2026-08-16T20:10:30+08:00",
  "captured_at_precision": "exact",
  "timezone": "Asia/Shanghai",
  "source": "android_tile",
  "context": {
    "activity": "running",
    "location_name": null
  },
  "privacy_mode": "cloud_allowed",
  "client_version": 1
}
```

响应：首次 `201`；幂等重放 `200`；同 key 不同内容 `409 IDEMPOTENCY_CONFLICT`。响应不等待 STT 或 LLM。

### 4.2 音频资产

- `POST /captures/{id}/assets:init`；
- 上传至签名 URL 或可重试文件端点；
- `POST /captures/{id}/assets/{asset_id}/complete`；
- 服务端校验 MIME、大小和 SHA-256 后才触发 STT。

### 4.3 读取、同步和重试

- `GET /captures/{id}`；
- `PATCH /captures/{id}`，使用 `If-Match` 或 `expected_version`；
- `POST /captures/{id}/retry?stage=stt|organize|embed|relate`；
- `DELETE /captures/{id}`；
- `GET /sync/changes?cursor=...`，包含更新和删除 tombstone；
- `GET /memory-cards?sort=...&cursor=...`；
- `GET /memory-cards/{id}`；
- `POST /memory-cards/{id}/interactions`；
- `POST /memory-cards/{id}/continue`；
- `POST /memory-cards/{id}/reminders`。

列表使用稳定 cursor `(sort_value, id)`，不继续使用 offset 作为大数据分页方案。


### 4.4 AI 设置与补全 API

- GET /users/me/ai-settings；
- PATCH /users/me/ai-settings，使用 expected_revision；
- POST /captures/{id}/completion:preview；
- POST /captures/{id}/completion:apply，携带 proposal_ids 与 source_revision；
- POST /imports/{id}/completion:preview；
- POST /imports/{id}/completion:apply；
- POST /memory-cards/bulk-completion:preview（Beta）；
- POST /captures/{id}/organize-once，关闭总开关时的单次授权。

preview 只创建 CompletionProposal，不修改业务对象；apply 必须幂等、校验提案仍基于当前版本，并返回可撤销 revision。

### 4.5 平台与工作流能力探测

GET /api/v3/capabilities 至少返回：

- mobile_capture=true；
- web_review=true；
- workflow_designer=false；
- workflow_execution=false；
- supported_capture_sources 列表。

预留 /api/v3/workflows、/api/v3/workflows/{id}/draft、:validate、:publish、:run 命名空间。MVP 不实现工作流持久化与执行；若客户端误调用写入或运行，统一返回 501 FEATURE_NOT_ENABLED，不得创建后台任务。

## 5. 单条导入规范

单条导入最终复用 Capture API，`source=import`。

### 5.1 表单字段

必填条件：`content` 非空，或至少上传一个音频/文件资产。

选填：

- external_id；
- title；
- captured_at；
- captured_at_precision；
- timezone；
- source_name/source_url；
- primary_type；
- tags；
- location_name；
- latitude/longitude；
- activity；
- collection；
- attachments。

### 5.2 JSON 示例

```json
{
  "external_id": "old-note-2024-031",
  "content": "跑步时想到：卡片不应该只是摘要，而应该保留继续思考的入口。",
  "title": null,
  "captured_at": "2024-05-01T21:30:00+08:00",
  "captured_at_precision": "estimated",
  "timezone": "Asia/Shanghai",
  "tags": ["产品", "灵感"],
  "source_name": "旧备忘录",
  "source_url": null,
  "context": {
    "activity": "running",
    "location_name": null
  }
}
```

### 5.3 缺失字段规则

| 缺失项 | 行为 |
|---|---|
| 标题 | AI 可建议，标记 `provenance=ai` |
| 标签 | AI 可建议，不伪装成原标签 |
| 时区 | 可继承账户时区，标记 `inherited` |
| 产生时间 | 保持 unknown，只记录 imported_at |
| 地点 | 保持为空，禁止推测 |
| 正文但有音频 | 允许，后续 STT |
| 正文和资产都没有 | 拒绝或进入 needs_input |
| 类型 | AI 建议，用户可改 |

### 5.4 单条导入的 AI 补全

导入预览先执行确定性校验，再显示“AI 补全缺失项”。用户可预览标题、类型、标签、摘要和下一步建议；估算时间、来源和合集必须逐项确认；地点、坐标和原文没有提供的事实保持未知。

AI 补全默认只写空字段。任何用户填写值、导入原值都受保护，不允许被预览或 Commit 静默覆盖。

## 6. 批量导入规范


### 6.1 流程

```text
创建导入任务
→ 上传文件/粘贴多段文本
→ 解析
→ 字段映射
→ 前 10 条预览
→ 校验与重复检查
→ 用户确认 Commit
→ 每行进入 Capture Pipeline
→ 成功/失败/跳过/待补充报告
```

### 6.2 格式阶段

| 阶段 | 格式 |
|---|---|
| MVP | 多段文本、TXT、Markdown、UTF-8 CSV、JSONL |
| Beta | JSON 数组、批量音频、ZIP、字段映射模板 |
| Later | Notion、Obsidian、Apple Notes、微信等专用适配器 |

### 6.3 多段文本

默认使用单独一行 `---` 分隔记录，空行只作为段落。导入前展示分割结果，用户可更换分隔符。

```text
第一条记忆正文，可以有多个段落。

这是同一条的第二段。
---
第二条记忆正文。
```

### 6.4 CSV 模板

```csv
external_id,content,title,captured_at,timezone,tags,source_url,location_name,latitude,longitude,activity
note-001,"跑步时想到……","",2024-05-01T21:30:00+08:00,Asia/Shanghai,"产品|灵感",,, , ,running
```

约束：

- UTF-8；
- `content` 为核心字段；
- tags 使用 `|` 分隔；
- captured_at 使用 ISO-8601；
- 经纬度必须同时出现且通过范围校验；
- 单行失败不回滚整个批次。

### 6.5 JSONL 模板

```json
{"external_id":"note-001","content":"第一条记忆","captured_at":null,"captured_at_precision":"unknown","tags":["灵感"]}
{"external_id":"note-002","content":"第二条记忆","captured_at":"2025-02-03T08:20:00+08:00","timezone":"Asia/Shanghai"}
```

### 6.6 去重

优先级：

1. `(user_id, source_name, external_id)`；
2. 资产 SHA-256；
3. 规范化正文 hash + 近似时间；
4. 仅提示疑似重复，不静默删除。

用户选择：跳过、仍然导入、合并。批次重试必须幂等。

### 6.7 批量导入的 AI 补全

字段映射与确定性校验完成后，用户可以选择仅对缺失字段生成补全预览。界面必须先展示抽样结果、预计处理条数、预计耗时/额度以及不可补全字段；Commit 时逐行记录 FieldProposal 和 provenance。

解析、补全、Commit 是三个独立阶段。补全失败不阻止原始行导入；用户可选择“按原样导入”。
### 6.8 工作流预留领域契约

本阶段只冻结接口，不创建真实执行能力：

- WorkflowDefinition：id、user_id、name、description、status=draft、active_version_id；
- WorkflowVersion：workflow_id、version、trigger、conditions、created_at；
- WorkflowNode：id、type、type_version、config、input_schema、output_schema、required_permissions；
- WorkflowEdge：from_node/port、to_node/port；
- WorkflowRun：仅保留未来命名，不建运行入口与 Worker。

节点类型版本化接口统一为 type、version、input_schema、output_schema、config、permissions。计划节点包括 organize、complete_fields、relate、reminder、export；任何外部副作用节点必须显式授权。

MVP capabilities 必须声明不可用，服务端不得因为路由或类型已预留就执行方案。Beta 若开放草稿，只允许本地/服务端保存、模板复制和静态 Schema 校验。

## 7. 模型调用分层

建立统一 `ModelGateway`，按 purpose 路由，不能再用一个模型和统一温度承担所有工作。

| Purpose | 输入 | 输出 | 默认策略 |
|---|---|---|---|
| stt | 完整音频 | 原始转写、语言、时间戳 | 可重试中文 STT |
| clean_transcript | 原始转写 | 清理文本 | 小模型/规则，低温度 |
| organize | 生效转写/文字 | 卡片结构 | 严格 JSON Schema，温度 0—0.2 |
| complete_fields | 生效文本 + 缺失字段 | FieldProposal 列表 | 严格 Schema；事实字段必须附证据；只预览不直写 |
| embed | 生效文本 | 向量 | 内容版本变化才重算 |
| relate | 向量召回候选 | 1—3 条关系与理由 | 先召回，必要时小模型 rerank |
| memory_chat | 问题 + 检索卡片 | 带来源回答 | 用户主动触发 |
| recap | 一组记忆 | 每日/每周回顾 | 可因预算跳过 |
| import_parse | 文件/行 | 结构化字段与错误 | 先确定性解析，AI 只辅助 |
| tool_agent | Later | 外部操作 | 与记忆链路隔离 |

## 8. 分阶段输出，而非同步多轮轰炸

| 阶段 | 时间 | 用户看到什么 | 模型 |
|---|---|---|---|
| T0 | 录音停止后 <1 秒 | 已安全保存 | 无模型 |
| T1 | 稍后 | 转写完成，可修正 | STT |
| T2 | AI 整理开启时 | 标题、核心、最多 3 要点、1 个下一步 | organize |
| T3 | 后台 | 1—3 条关联及理由 | embed/relate |
| T4 | 次日/每周 | 回响与主题回顾 | recap/规则 |

用户点击“继续思考”时才进入对话或深度启发。默认捕捉流程不等待 T1—T4。

## 9. 模型路由配置

### 9.1 运营/服务端配置

每条 ModelRoute：

```text
purpose
provider
model_id
endpoint_secret_ref
timeout_ms
temperature
max_output_tokens
prompt_version
schema_version
fallback_route_id
max_attempts
per_call_budget
daily/monthly_budget
privacy_class_allowed
enabled
config_version
```

配置优先级：

```text
系统安全策略 > 运营模型路由 > 用户隐私偏好
```

MVP 只允许管理员配置 allowlist Provider。普通用户不得填写任意 Base URL，避免 SSRF 和凭据安全风险。BYOK 放到 Later。

### 9.2 用户设置

- AI 记忆整理总开关（初始关闭，明确同意后开启）；
- AI 补全开关（初始开启入口，但只在用户主动请求时调用）；
- 仅记录 / 智能整理 / 深度启发模式；
- 云端转写独立开关；
- 自动摘要、标签、关联、回响子开关；
- “补整理未处理记忆”一次性操作，不默认回溯；
- 地点记录开关；
- 云端文本、云端音频授权；
- 语言与专有词；
- 音频保留周期；
- Web 与手机展示同一设置 revision 和最后更新时间。

### 9.3 调用审计

记录 purpose、task、provider、model、Prompt/Schema 版本、Token、估算成本、耗时、错误码和隐私分类。

默认不记录完整原文、原音频、完整 Prompt、密钥或短期播放 URL。

## 10. 任务、重试和降级

- 音频上传：断点/重试 + checksum；
- STT：指数退避，允许跨 24 小时；
- organize：3—4 次，Schema 错误可进行一次修复；
- complete_fields：preview 可重试；apply 不调用模型且必须幂等，旧版本提案不可应用；
- embedding：持久任务，允许长时间重试；
- relate/recap：允许降级或跳过；
- 401、非法文件、超限为不可重试；
- timeout、429、5xx 为可重试；
- 超过次数进入 permanently_failed，用户仍可查看原始记录并手动重试。

MVP 使用 PostgreSQL Outbox + `FOR UPDATE SKIP LOCKED` 即可。Temporal 暂不进入单条 Capture 热路径。

成本不足时按顺序降级：recap → rerank → relation reason → deep insight；绝不跳过保存。

## 11. 排序和索引

建议索引：

- `captures(user_id, captured_at DESC, id DESC)`；
- `memory_cards(user_id, pinned DESC, updated_at DESC, id DESC)`；
- `memory_cards(user_id, last_opened_at DESC, id DESC)`；
- `memory_cards(user_id, open_count DESC, id DESC)`；
- `processing_tasks(status, next_run_at)`；
- `import_rows(import_job_id, status, row_number)`；
- `memory_interactions(user_id, card_id, occurred_at DESC)`；
- 向量索引按 user/collection 范围过滤；
- 所有外键访问同时验证 user_id。

列表使用 cursor；搜索相关度排序只出现在搜索结果；回响使用单独队列，不改变主记忆流顺序。

## 12. 登录和会话契约

API：

- `POST /auth/otp/request`；
- `POST /auth/otp/verify`；
- `POST /auth/refresh`；
- `POST /auth/logout`；
- `GET /devices/sessions`；
- `DELETE /devices/sessions/{id}`。

要求：

- OTP 由服务端发送和校验；
- access token 10—20 分钟；
- refresh token 轮换、仅保存 hash、按设备撤销；
- Token 使用 Authorization Header；WebSocket 使用受保护握手或首帧，不放 URL query；
- 游客 Capture 登录后幂等归属到用户；
- 请求频控、验证码和异常登录审计；
- OAuth 后续使用 Authorization Code + PKCE，并验证服务端 token。

## 13. 隐私、保留和删除

- 地点默认关闭；
- 音频云端默认保留 30 天，用户可选永久、仅本机或转写后删除；
- 原始文字/用户修正版持续保留，除非用户删除；
- 对象存储私有桶、短期签名 URL、服务端加密；
- 日志不输出 JWT、音频 URL、完整文本和密钥；
- 删除覆盖 Capture、Asset、Transcript、Enrichment、Embedding、Relation、Reminder 和搜索索引；
- 回收站 30 天后永久清理；用户选择“立即永久删除”时跳过等待期；
- 备份保留期需要在隐私政策中明确；
- 导出包含原始内容、用户编辑、AI 派生、时间情境和附件清单。

## 14. MVP 验收门禁

### 14.1 捕捉

- 飞行模式连续录制 20 条，杀进程、重启后全部存在；
- 恢复网络后最终同步率 ≥99%，重复记录为 0；
- 相同 capture_id 提交 100 次只产生一条；
- STT、LLM、Worker 全停时原始捕捉仍成功；
- 服务端创建 Capture P95 <500ms，不等待模型；
- 完整音频 checksum 与本地一致。

### 14.2 任务

- Worker 任意阶段崩溃后可续跑；
- 同一 revision 不产生重复派生；
- 95% 可处理任务最终 ready 或明确错误；
- 429/5xx 自动退避，业务错误不无限重试；
- 预算耗尽时卡片仍显示原始内容。

### 14.3 权限和隐私

- 用户不能通过 Capture/Import/Task/Project ID 访问他人数据；
- 所有列表 SQL 带 user_id；
- 会话可撤销；
- 删除可证明完成级联清理；
- 日志、任务表和错误响应没有密钥与完整敏感内容。

### 14.4 导入

- 单行错误不影响其他行；
- 导入可中断恢复；
- 重复提交不生成重复卡片；
- 缺时间、地点时不伪造事实；
- Commit 前展示字段映射、错误数和疑似重复数；
- 1 万行作为 Beta 压测目标，MVP 先保证 1 千行稳定。
### 14.5 AI 开关与补全

- 关闭 ai_memory_enabled 后连续新增 50 条记录，organize/embed/relate/recap 模型调用数为 0；
- 关闭 AI 整理但开启转写时，音频仍可生成 TranscriptRevision，且不会继续生成 AI 派生；
- 设置 revision 被 Capture 快照，多设备切换不造成旧记录意外补处理；
- 再开启后历史记录不会自动入队，只有显式“补整理”才创建任务；
- 补全 preview 不修改任何字段；
- apply 不覆盖用户值或导入原值，失效 source_revision 返回冲突；
- 每个建议都可看到 provenance；事实型建议没有 evidence_spans 时不得应用；
- 单条与批量补全均可跳过，原始内容仍能正常导入；
- 应用建议后可以撤销到前一 revision。

### 14.6 双端与工作流预留

- 同一 Capture 在手机与 Web 的原文、版本、处理状态最终一致；
- Web 可完成搜索、详情查看、导入、AI 设置和数据管理；
- Web 关闭时不声称支持后台浏览器录音或系统级热键；
- capabilities 对工作流设计和执行返回 false；
- 工作流写入/运行请求返回 FEATURE_NOT_ENABLED，且不会产生 ProcessingTask、WorkflowRun 或外部副作用；
- 路由、Schema 和枚举预留不影响 MVP 数据迁移与普通捕捉链路。

