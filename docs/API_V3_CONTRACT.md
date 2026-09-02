# WeaveBrain API V3 基础契约

> 状态：R0 冻结  
> 日期：2026-08-16  
> 适用范围：所有 /api/v3 HTTP 接口

## 1. 路由版本

- V1 旧接口继续使用 /api/v1；
- 新领域模型只进入 /api/v3；
- R0 只开放 GET /api/v3/meta；
- Capture、MemoryCard、AI Settings 等业务接口在后续轮次加入；
- 未知 V3 路由必须使用统一 V3 错误体；
- 未知 V1 路由保持旧响应形状。

## 2. 请求追踪

请求头：

- X-Request-ID：可选；客户端提供时必须是 UUID；
- 缺失或非法时由服务端生成 UUID；
- 响应头始终返回最终 X-Request-ID；
- JSON 成功/错误响应中的 request_id 与响应头一致。

后续 V3 接口预留：

- Idempotency-Key：创建 Capture 时使用客户端 UUID；
- If-Match：更新资源时使用版本号；
- Authorization：Bearer Token。

CORS 必须允许 X-Request-ID、Idempotency-Key、If-Match，并暴露 X-Request-ID。

## 3. 成功元信息接口

GET /api/v3/meta

响应状态：200。

响应字段：

- api_version：固定为 v3；
- request_id：本次请求追踪 UUID。

## 4. 统一错误体

所有 V3 错误返回 JSON：

{
  "code": "NOT_FOUND",
  "message": "resource not found",
  "request_id": "UUID",
  "details": {}
}

字段规则：

- code：稳定、机器可判断，不放自然语言；
- message：面向开发者的简短说明；
- request_id：必须存在；
- details：必须是 JSON 对象；没有细节时返回空对象；
- 不在错误体中返回堆栈、SQL、密钥、Token 或内部 Provider 响应。

## 5. 冻结错误码

| HTTP | code | 使用场景 |
|---:|---|---|
| 400 | INVALID_ARGUMENT | 请求字段、格式或状态不合法 |
| 401 | UNAUTHORIZED | 未认证或 Token 失效 |
| 403 | FORBIDDEN | 已认证但无权限 |
| 404 | NOT_FOUND | 资源不存在或按用户隔离后不可见 |
| 409 | IDEMPOTENCY_CONFLICT | 同一幂等键对应不同请求内容 |
| 409 | VERSION_CONFLICT | expected_version / If-Match 冲突 |
| 501 | FEATURE_NOT_ENABLED | 已预留但当前版本不开放 |
| 500 | INTERNAL | 未能安全归类的服务端错误 |

业务代码不得用 message 字符串代替 code 做分支判断。

## 6. 兼容与扩展

- 可以增加新的 details 字段；
- 不得删除或改名 code、message、request_id、details；
- 新增错误码必须更新本文档和契约测试；
- 修改现有错误码语义必须通过轮次变更记录；
- V3 业务 Handler 必须调用统一错误写入函数，禁止临时返回 gin.H{"error": ...}。

## 7. 自动化证据

实现：

- WeaveBrain/internal/api/contract_v3.go。

测试：

- WeaveBrain/internal/api/contract_v3_test.go；
- 覆盖 meta、未知 V3 路由、400/401/403/404/409/500/501；
- 覆盖非法 Request ID 替换；
- 覆盖 V1 未知路由响应不被 V3 契约改变。


## 8. R1 Capture 接口

### 8.1 认证与幂等

- POST /api/v3/captures 和 GET /api/v3/captures/{id} 均要求 Authorization: Bearer Token；
- POST 必须携带 Idempotency-Key，值为客户端生成的 Capture UUID；
- 请求体 capture_id 必须与 Idempotency-Key 相同；
- 幂等范围为 (authenticated_user_id, capture_id)；
- 同一用户、同一 UUID、同一规范化内容重放返回 200 和 replayed=true；
- 同一用户、同一 UUID、不同内容返回 409 / IDEMPOTENCY_CONFLICT；
- 首次创建返回 201 和 replayed=false。

### 8.2 创建文字 Capture

POST /api/v3/captures

R1 请求字段：

- capture_id：必填 UUID；
- kind：当前只接受 text，省略时按 text 处理；
- text：必填，原文精确保留，仅使用 trim 后结果判断是否为空；最多 100,000 个 Unicode code points；
- captured_at：可选 RFC3339 时间；
- captured_at_precision：exact/date_only/estimated/unknown，默认 unknown；
- timezone：可选；
- source：默认 api；
- collection_id：可选，不要求先建项目；若提供，数据库保证其属于当前用户；
- privacy_mode：cloud_allowed/no_ai，默认 cloud_allowed；
- client_version：必填正整数。

- 整个创建请求体最大 1 MiB；超过限制返回 400 / INVALID_ARGUMENT；
成功体包含：

- capture：原始事实；
- memory_card：同步生成的 fallback 卡片；
- replayed：是否为幂等重放；
- request_id：请求追踪 UUID。

R1 保存链路不调用 STT、LLM、Embedding、Agent 或 Worker。原始 Capture、fallback MemoryCard 和 capture.created Outbox 事件在同一条 PostgreSQL 语句中原子写入。

### 8.3 获取 Capture

GET /api/v3/captures/{id}

- 查询在 SQL 中同时限定 user_id 和 capture_id；
- 资源不存在或属于其他用户时统一返回 404 / NOT_FOUND，不暴露资源是否真实存在；
- 成功体包含 capture、memory_card 和 request_id。

### 8.4 R1 自动化证据

- internal/service/capture_service_test.go：100 次重放、异文冲突、无项目保存、原文保留、输入校验与用户隔离；
- internal/api/capture_handler_test.go：201/200/400/401/404/409 和请求追踪契约；
- internal/db/repository/capture_repository_test.go：SQL 所有权和原子 CTE 契约；
- internal/db/repository/capture_repository_integration_test.go：真实 PostgreSQL 幂等、行数、跨用户读取和 collection 外键；
- internal/db/migration/migration_contract_test.go：迁移结构与 Down 顺序。

## 9. R5 AI 设置接口

### 9.1 认证

- GET/PATCH /api/v3/users/me/ai-settings 均要求 Authorization: Bearer Token；
- 用户以认证主体为准，不读 URL 中的用户参数。

### 9.2 读取设置

GET /api/v3/users/me/ai-settings

- 未写入过设置的用户返回隐私优先默认值（全部 false、revision=0），不要求先行创建；
- 响应 200，成功体：

```json
{
  "settings": {
    "user_id": "UUID",
    "ai_memory_enabled": false,
    "ai_completion_enabled": false,
    "speech_to_text_enabled": false,
    "cloud_text_allowed": false,
    "cloud_audio_allowed": false,
    "revision": 0,
    "created_at": "RFC3339",
    "updated_at": "RFC3339"
  },
  "pending_reorganize": 0,
  "request_id": "UUID"
}
```

- `pending_reorganize`：可补整理的记忆条数。语义为「AI 记忆整理关闭期间创建、未经 pipeline 处理、已置为 ready 的 capture.created 事件」计数，即 `status='ready' AND policy_snapshot->>'ai_memory_enabled'='false'`。outbox 不可用时优雅降级为 0。

### 9.3 更新设置

PATCH /api/v3/users/me/ai-settings

请求字段：

- expected_revision：必填，客户端最近读到的 revision（乐观并发）；
- ai_memory_enabled / ai_completion_enabled / speech_to_text_enabled / cloud_text_allowed / cloud_audio_allowed：可选布尔；缺失的字段保持原值；
- 至少提供 1 个设置字段，否则 400 / INVALID_ARGUMENT。

响应：

- 200：更新后的 settings，revision 已 +1；
- 409 / VERSION_CONFLICT：expected_revision 与当前 revision 不一致（多设备并发或过期写入）；
- 400 / INVALID_ARGUMENT：请求体非法、缺少 expected_revision 或为负、无设置字段。

revision 语义：

- 无行默认态 = 0；
- 任何写入（含首次创建）后 = 1，之后每次写 +1；
- 客户端必须用最新响应里的 revision 作为下一次 expected_revision。

### 9.4 R5 自动化证据

- internal/db/repository/ai_settings_repository_test.go：无行、全列扫描、Create 默认值/revision1、Create 并发冲突、CAS miss、返回新 revision；
- internal/service/ai_settings_service_test.go：物化默认、部分补丁、revision 冲突、新建与递增；
- internal/api/ai_settings_handler_test.go：200/400/401/409 与请求追踪契约；
- internal/db/migration/user_ai_settings_migration_contract_test.go：迁移结构、默认 FALSE、Down 顺序。

### 9.5 历史补整理（Reorganize）

POST /api/v3/users/me/ai-settings/reorganize

目的：把「AI 记忆整理关闭期间创建、worker 已直接 MarkReady、memory_card 未含 AI 字段」的记忆显式重新入队，由 worker 按**当前**政策快照补跑 AI 整理。

- 目标集：`user_id` 下 `status='ready' AND policy_snapshot->>'ai_memory_enabled'='false'` 的 capture.created 事件；
- 成功时事件被置为 `queued`、`attempt_count=0`、`last_error=NULL`、`next_run_at=now()`，并盖上当前政策快照（`FromAISettings`）；
- 要求当前 `ai_memory_enabled=true`，否则 409 / FEATURE_NOT_ENABLED（「必须先开启 AI 记忆整理」）；
- 只显式触发，绝不自动补整理；`failed`/`cancelled` 事件不纳入（保持「关闭即取消」的显式语义）。

响应：

- 200：
```json
{
  "reorganized": 12,
  "request_id": "UUID"
}
```
- 409 / FEATURE_NOT_ENABLED：AI 记忆整理未开启；
- 401 / UNAUTHORIZED：未认证。

后端证据：

- internal/db/repository/outbox_repository_test.go：CountPendingReorganize 查询契约、ReorganizeByUser 绑定 user_id+policyJSON 与 rowsAffected；
- internal/service/ai_settings_service_test.go：AI 关 → ErrReorganizeAIMemoryDisabled；AI 开 → 委托 outbox 且快照含当前开关；outbox nil → count/补整理为 0 不 panic；
- internal/api/ai_settings_handler_test.go：GET 含 pending_reorganize；POST /reorganize 200/409/401。

## 10. R6 记忆流接口

### 10.1 认证与通用规则

- 全部 7 个端点均要求 `Authorization: Bearer Token`；未认证/失效 → 401 / UNAUTHORIZED；
- 用户以认证主体为准，不读 URL 或请求体中的用户字段；
- 每个成功/错误响应都带 `request_id`（与响应头 X-Request-ID 一致）；
- 列表、详情按 `captures.user_id = 当前用户` 隔离；资源不存在、属于其他用户或已软删时统一 404 / NOT_FOUND，不暴露资源是否存在；
- **生命周期语义**：`active`（进行中）/ `archived`（归档）/ `trashed`（软删，`deleted_at` 置位）。列表与详情只暴露 `deleted_at IS NULL` 的行；`trashed` 后不可再读取。R6 无「恢复归档/回收站还原」端点。

端点一览：

| 方法 | 路径 | 用途 |
|---|---|---|
| GET | /api/v3/memories | 记忆流列表 + 搜索 + 筛选 + 分页 |
| GET | /api/v3/memories/{captureId} | 记忆详情（原文/音频/转写/修订历史） |
| PATCH | /api/v3/memories/{captureId} | 修正字段（部分字段） |
| POST | /api/v3/memories/{captureId}/notes | 续写（追加笔记，不改原文） |
| POST | /api/v3/memories/{captureId}/pin | 置顶 / 取消置顶 |
| POST | /api/v3/memories/{captureId}/archive | 归档（幂等） |
| DELETE | /api/v3/memories/{captureId} | 软删（移入回收站） |

### 10.2 记忆流列表

GET /api/v3/memories

查询参数（均可选）：

- `cursor`：上一页 `next_cursor` 返回的不透明 keyset 游标；
- `limit`：默认 20、最小 1、最大 50；非法值 → 400 / INVALID_ARGUMENT；
- `q`：全文搜索关键词，匹配 `memory_cards.title` / `captures.original_text` / `transcript_revisions.text`（三列 ILIKE 子串，`\ % _` 已转义，走 `pg_trgm` GIN 索引）；
- `kind`：精确筛选 `captures.kind`（如 text / audio）；
- `primary_type`：精确筛选 `memory_cards.primary_type`；
- `lifecycle_status`：默认 `active`，只接受 `active` / `archived`；其他值 → 400 / INVALID_ARGUMENT；
- `pinned`：`true`/`false`，`true` 时只返回已置顶；非法值 → 400 / INVALID_ARGUMENT。

排序与游标：

- 排序 `is_pinned DESC, created_at DESC, id DESC`（置顶优先，再按创建时间倒序）；
- 游标编码最后一行 `(is_pinned, created_at, id)` 元组：`base64url(JSON {"p": bool, "t": RFC3339, "i": captureId})`；
- 服务端取 `limit+1` 行判 `next_cursor`：有更多行时返回 `next_cursor`，否则为 null；
- **next_cursor 契约**：请求下一页必须携带与首页完全相同的筛选参数（`q`/`kind`/`primary_type`/`lifecycle_status`/`pinned`），否则游标位置无意义；
- 游标解码失败 → 400 / INVALID_ARGUMENT。

响应 200：

```json
{
  "items": [
    {
      "capture": { "…Capture 字段…" },
      "memory_card": { "…MemoryCard 字段…" }
    }
  ],
  "next_cursor": "eyJwIjpmYWxzZSwidCI6Ii4uLiIsImkiOiIuLi4ifQ",
  "request_id": "UUID"
}
```

### 10.3 记忆详情

GET /api/v3/memories/{captureId}

- `captureId` 非法 UUID → 400 / INVALID_ARGUMENT；
- 资源不存在/他人/已软删 → 404 / NOT_FOUND；
- 响应 200：

```json
{
  "capture": { "…" },
  "memory_card": { "…" },
  "audio": { "…" },
  "transcript": { "…" },
  "revisions": [ { "…EnrichmentRevision…" } ],
  "request_id": "UUID"
}
```

- `audio` / `transcript` 不存在时省略（omitempty）；
- `revisions` 为修订历史，**revision 降序**（最新在前）；新捕获至少包含 1 条 `fallback` 修订；存量捕获可为空数组。

### 10.4 修正字段

PATCH /api/v3/memories/{captureId}

请求体：部分字段，缺失字段保持不变；请求体最大 1 MiB。

```json
{
  "title": "新标题",
  "summary": "新摘要",
  "primary_type": "idea",
  "tags": ["标签一"],
  "key_points": ["要点一"]
}
```

字段校验（任一失败 → 400 / INVALID_ARGUMENT）：

- `title`：trim 后非空，≤ 300 runes；
- `summary`：≤ 2000 runes；空字符串表示清除摘要；
- `primary_type`：`uncategorized` / `idea` / `question` / `action` / `reflection` / `reference` 之一；
- `tags`：≤ 20 项，每项 trim 后 ≤ 100 runes；
- `key_points`：≤ 3 项，每项 trim 后 ≤ 500 runes；
- 至少有一个字段实际变化，否则 400。

语义：

- 产生一条 `source=user` 的 EnrichmentRevision：`changes` 只含实际修改的字段，`provenance` 逐字段为 `"user"`，`source_revision = captures.version`；
- 卡片 `version` +1；
- 并发冲突（两写并发取 MAX+1 撞 UNIQUE）→ 409 / VERSION_CONFLICT，客户端刷新后重试；
- 响应 200：

```json
{
  "capture": { "…" },
  "memory_card": { "…" },
  "revision": { "…EnrichmentRevision…" },
  "request_id": "UUID"
}
```

### 10.5 续写

POST /api/v3/memories/{captureId}/notes

请求体：

```json
{ "text": "后续想法…" }
```

- `text` trim 后非空且 ≤ 10,000 runes，否则 400 / INVALID_ARGUMENT；
- 产生一条 `source=user` 修订，`changes={"note": text}`，`card.version` +1；**不修改** title/summary/primary_type/tags/key_points，也**绝不改写** `captures.original_text`；
- 并发冲突 → 409 / VERSION_CONFLICT；
- 响应同 §10.4（含 `revision`）。

### 10.6 置顶

POST /api/v3/memories/{captureId}/pin

请求体：

```json
{ "pinned": true }
```

- 置顶时 `is_pinned=true` 且 `pinned_at=now()`；取消置顶时 `is_pinned=false` 且 `pinned_at=NULL`；
- 响应 200：`{ "capture", "memory_card", "request_id" }`（不含 `revision`）。

### 10.7 归档

POST /api/v3/memories/{captureId}/archive

- 置 `lifecycle_status='archived'`，`deleted_at` 保持 NULL；
- **幂等**：对已归档记忆再次归档返回 200；
- 响应 200：`{ "capture", "memory_card", "request_id" }`；
- R6 无取消归档端点（详见 R06 报告已知限制）。

### 10.8 删除

DELETE /api/v3/memories/{captureId}

- 软删除：置 `lifecycle_status='trashed'` 且 `deleted_at=now()`；
- 删除后列表与详情均不可见（`deleted_at IS NULL` 过滤）；已 trashed 再次删除 → 404 / NOT_FOUND；
- 音频资产行暂不级联删除（列表 `idx_captures_user_created WHERE deleted_at IS NULL` 已隐藏）；
- 响应 200：`{ "capture", "memory_card", "request_id" }`。

### 10.9 数据模型

MemoryCard（列表行与详情共用）：

```json
{
  "id": "UUID",
  "user_id": "UUID",
  "capture_id": "UUID",
  "primary_type": "uncategorized",
  "title": "标题",
  "summary": "摘要（可省略）",
  "tags": ["标签一"],
  "key_points": ["要点一"],
  "processing_status": "completed",
  "version": 1,
  "is_pinned": false,
  "pinned_at": "RFC3339（可省略）",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

EnrichmentRevision：

```json
{
  "id": 1,
  "user_id": "UUID",
  "capture_id": "UUID",
  "revision": 1,
  "card_version": 1,
  "source": "fallback",
  "source_revision": 1,
  "changes": { "title": "…", "primary_type": "uncategorized", "tags": [], "key_points": [] },
  "provenance": { "source": "fallback" },
  "created_at": "RFC3339"
}
```

- `source` ∈ `fallback`（捕获创建时本地派生）/ `ai`（R8 真实 AI 预留）/ `user`（用户编辑）；
- `card_version` = 该修订产生时的 `memory_card.version`；
- `source_revision` = 派生依据的 `captures.version`（R6 设为捕获版本；R8 真实 AI 应指向实际转写修订）；
- **fallback 派生规则**：捕获创建时原子写入 `revision=1, card_version=1, source='fallback', source_revision=1`；title = 原文前 30 runes、summary = 前 200 runes（超出追加 `…`，无文本则 null）、tags=[]、key_points=[]、primary_type='uncategorized'。

### 10.10 错误映射

| 场景 | HTTP | code |
|---|---|---|
| 非法 limit / pinned / 非法 captureId / 请求体非法 / 非法游标 / 字段校验失败 | 400 | INVALID_ARGUMENT |
| 未认证或 Token 失效 | 401 | UNAUTHORIZED |
| 记忆不存在、属他人、已软删 | 404 | NOT_FOUND |
| PATCH / notes 并发修订冲突 | 409 | VERSION_CONFLICT |
| 未能安全归类的服务端错误 | 500 | INTERNAL |

### 10.11 R6 自动化证据

- internal/db/repository/memory_repository_test.go：List 所有权 + `deleted_at IS NULL` + 默认 active + 排序 + 三列搜索 q + tag/kind/pinned 过滤 + 游标谓词 + ListRevisions 降序 + AppendRevisionAndUpdateCard 原子 CTE + SetPinned pinned_at + SetLifecycle trashed→deleted_at；
- internal/service/memory_service_test.go：Correct→user revision + version 递增 + provenance；校验失败；AppendNote 不改字段；trashed→404；SetPinned 切换；Archive 幂等；Delete→trashed；GetDetail nil audio/transcript 容忍；fallback 字段派生；
- internal/api/memory_handler_test.go：list 200+next_cursor / 筛选透传 / detail 200+revisions / 404 / PATCH 200+400 / notes 200+空文本400 / pin / archive / delete / 401 / 各响应含 request_id；
- internal/db/migration/migration_contract_test.go：迁移 000010 结构、pg_trgm、GIN 索引、source CHECK、Down 顺序。

## 11. R8 AI 补全接口

### 11.1 总则与门控

- 全部 3 个端点均要求 `Authorization: Bearer Token`；未认证/失效 → 401 / UNAUTHORIZED；
- 用户以认证主体为准，不读 URL 或请求体中的用户字段；
- 每个成功/错误响应都带 `request_id`（与响应头 X-Request-ID 一致）；
- 补全目标字段 = MemoryCard 现有列中的 `title` / `primary_type` / `summary` / `tags` / `key_points`（**不含** captured_at / next_step / location / 身份）；
- **AI 补全关闭时不展示或调用入口**：preview 要求 `ai_completion_enabled && cloud_text_allowed`（原文会发给 LLM）；apply 只要求 `ai_completion_enabled`（apply 不发文本）——不满足均返回 409 / FEATURE_NOT_ENABLED；
- 卡片必须 `processing_status == 'ready'` 才可补全，否则 409 / PRECONDITION_FAILED；
- `primary_type == 'uncategorized'`（fallback 恒填值）与空 title/summary/tags/key_points 均视为「缺失字段」，是补全目标；非空字段（用户/导入值）受保护，apply 绝不覆盖。

端点一览：

| 方法 | 路径 | 用途 |
|---|---|---|
| POST | /api/v3/captures/{captureId}/completion/preview | 生成字段提案（不改业务对象） |
| POST | /api/v3/captures/{captureId}/completion/apply | 按提案填充空字段（幂等 + 版本安全） |
| POST | /api/v3/captures/{captureId}/completion/undo | 撤销最近一次已应用的补全 |

### 11.2 提案模型

`completion_proposals` 一行 = 一个字段提案；同一 preview 批次共享 `preview_id`。

CompletionProposal：

```json
{
  "id": "UUID",
  "user_id": "UUID",
  "capture_id": "UUID",
  "preview_id": "UUID",
  "source_revision": 1,
  "field_name": "tags",
  "original_value": "[]",
  "proposed_value": "[\"工作\",\"灵感\"]",
  "provenance": "ai",
  "apply_policy": "safe_auto",
  "confidence": 0.9,
  "risk_level": "low",
  "evidence_spans": [{ "start": 0, "end": 4, "quote": "工作" }],
  "status": "pending",
  "provider": "ollama",
  "model": "qwen2.5:7b",
  "config_version": "completion-prompt-v1",
  "created_at": "RFC3339",
  "updated_at": "RFC3339"
}
```

- `field_name` ∈ `title` / `primary_type` / `summary` / `tags` / `key_points`；
- `proposed_value` 为 TEXT：`tags` / `key_points` 存 JSON 数组字符串（`["a","b"]`），其余为普通字符串；
- `apply_policy` ∈ `safe_auto`（批量采用）/ `suggest_only`（需逐字段确认）/ `forbidden`（永不应用）；5 个目标字段有证据 → `safe_auto`，无证据 → 降级 `suggest_only`；
- `evidence_spans` 为原文中的逐字引文（`start`/`end` 为字节偏移，`quote` 为引文）；事实型建议必须有证据片段（标准 4）；
- `status` ∈ `pending` / `accepted` / `rejected` / `expired`；
- `source_revision` = 提案依据的 `MemoryCard.version`（乐观并发守卫）。

### 11.3 生成提案 preview

POST /api/v3/captures/{captureId}/completion/preview

- 无请求体；
- **绝不修改 capture 或 card**（标准 1）；
- 流程：门控（AICompletion && CloudText）→ 校验 capture 存在 + `ready` → 计算缺失字段 → 新 preview 使该 capture 全部 `pending` 提案过期（旧提案过期，标准 5）→ 缺失字段为空或原文为空 → 直接返回空提案 → 否则调 LLM（45s 超时）生成并落库；
- `source_revision` = 当前 `card.version`；
- 响应 200：

```json
{
  "preview": {
    "capture_id": "UUID",
    "source_revision": 1,
    "missing_fields": ["tags", "key_points"],
    "proposals": [ { "…CompletionProposal…" } ]
  },
  "request_id": "UUID"
}
```

### 11.4 采用提案 apply

POST /api/v3/captures/{captureId}/completion/apply

请求体（最大 1 MiB）：

```json
{
  "proposal_ids": ["UUID", "UUID"],
  "source_revision": 1
}
```

- `proposal_ids` 必填且非空；请求体非法 → 400 / INVALID_ARGUMENT；
- 语义：
  - **只填空字段**（标准 2）：当前字段非空 → 提案被 `rejected` 且跳过（绝不覆盖用户/导入值，标准 3）；
  - **幂等**：全部提案已 `accepted` → 200 no-op（返回当前 card + 全部 proposal_ids）；混合 → 只处理 `pending` 子集；
  - **版本安全**（标准 5）：`pending` 提案若 `source_revision != card.version` → 置 `expired` 并返回 409 / VERSION_CONFLICT；
  - 产生一条 `source=ai` 的 EnrichmentRevision：`changes` 只含实际填充字段，`provenance["_completion"]` 存 `{"preview_id":…, "undo":{field:原值}}`（undo 原值存 provenance，因 `changes` 是 flat map），逐字段 provenance `"completion"`；`source_revision = captures.version`；`card.version` +1；
  - 提案置 `accepted`（`accepted_by=userID`）；
- 响应 200：

```json
{
  "apply": {
    "memory_card": { "…MemoryCard…" },
    "revision": { "…EnrichmentRevision…" },
    "applied_proposal_ids": ["UUID"]
  },
  "request_id": "UUID"
}
```

### 11.5 撤销补全 undo

POST /api/v3/captures/{captureId}/completion/undo

- 无请求体；
- 语义：
  - 取最新一条 `source=ai` 且 `provenance["_completion"]` 存在的修订；无 → 409 / PRECONDITION_FAILED（「没有可撤销的补全」）；
  - 只回滚「当前值仍 == AI 所设值」的字段；用户之后编辑过的字段**跳过**（保护用户后续编辑）；
  - 无可回滚字段 → 409 / PRECONDITION_FAILED；
  - 产生一条 `source=user` 的 undo 修订（`provenance["_undo"]=true`、逐字段 `"undo"`），`card.version` +1；提案保持 `accepted`（审计留痕）；
- 响应 200：

```json
{
  "undo": {
    "memory_card": { "…MemoryCard…" },
    "revision": { "…EnrichmentRevision…" },
    "applied_proposal_ids": []
  },
  "request_id": "UUID"
}
```

### 11.6 错误映射

| 场景 | HTTP | code |
|---|---|---|
| proposal_ids 空 / 非法 captureId / 请求体非法 / id 缺失或跨 capture | 400 | INVALID_ARGUMENT |
| 未认证或 Token 失效 | 401 | UNAUTHORIZED |
| 记忆不存在、属他人、已软删 | 404 | NOT_FOUND |
| AI 补全未开启（preview：CloudText 也未开） | 409 | FEATURE_NOT_ENABLED |
| 卡片未 ready / 无补全可撤销 | 409 | PRECONDITION_FAILED |
| source_revision 变化后旧提案不可应用 | 409 | VERSION_CONFLICT |
| 生成失败（LLM 不可用 / 解析失败） | 500 | INTERNAL |

### 11.7 R8 自动化证据

- internal/db/migration/000013_completion_proposals_migration_contract_test.go：迁移结构、CHECK、Down 顺序；
- internal/db/repository/completion_repository_integration_test.go：真实 PostgreSQL 状态迁移（Create/List/Expire/MarkAccepted）与 confidence 浮点容差；
- internal/service/completion_service_test.go：preview 门控/不修改对象/无缺失字段/LLM 错误；apply 只填空/幂等/版本冲突/保护非空字段/混合状态；undo 无补全/正常/连续两次/跳过用户编辑字段；
- internal/service/completion_llm_test.go：strict JSON 解析、field 过滤、长度校验、primary_type 白名单、code-fence 容忍；
- internal/api/completion_handler_test.go：401 / 400 / 三端点 happy path / 各错误码映射；
- 前端 test/features/completions/：completion_api_test / completion_notifier_test / completion_panel_test + memory_detail_screen_test 补全入口开关（标准 7）。

---

# 12. 导入接口（R9 / G7）

> 功能：单条文字导入（复用 `POST /captures` kind=import）+ 批量导入（解析 → 预览 → AI 补全 → Commit → 错误报告，8 个端点）。
> 鉴权：全部端点位于 `protectedV3`，需 Bearer Token（未认证 → 401 / UNAUTHORIZED）。
> 批量请求体上限：8 MiB（`maxImportRequestBodyBytes`），超限 → 400 / INVALID_ARGUMENT。
> 格式别名：`txt` / `text` / `markdown` / `md` 归一化为 `plain_text` 解析器。

## 12.0 去重模型

批量导入的去重分两级，Commit 前在服务端预计算并写进每行的 `dedupe_status`：

| 级别 | 判定 | 处理 |
|---|---|---|
| `duplicate_external`（硬跳过） | 已有 Capture 的 `(user_id, source_name, external_id)` 精确相等 | Commit 一律跳过（不可被用户覆盖） |
| `suggested`（疑似） | `content_hash`（正文归一化 SHA-256）命中已有 Capture | 由用户通过 `duplicate_content_action` / `row_actions` 决定导入或跳过（默认导入） |

数据库安全网：`captures` 上部分唯一索引 `uq_captures_user_source_external(user_id, source_name, external_id) WHERE external_id IS NOT NULL AND source_name IS NOT NULL AND deleted_at IS NULL`；疑似查询走 `idx_captures_user_content_hash(user_id, content_hash)`。

## 12.1 单条导入（POST /captures，kind=import）

复用既有 `POST /api/v3/captures`，扩展 import 专属字段；`Idempotency-Key` 必须为合法 UUID 且 == `capture_id`（幂等）。

请求体：

```json
{
  "capture_id": "UUID",
  "kind": "import",
  "text": "第一条笔记",
  "external_id": "t1",
  "source_name": "旧备忘录",
  "title": "标题（选填，播种初始 MemoryCard）",
  "tags": ["工作"],
  "primary_type": "idea",
  "captured_at": "2026-08-01T00:00:00Z",
  "captured_at_precision": "date",
  "timezone": "Asia/Shanghai",
  "source": "import",
  "privacy_mode": "standard",
  "client_version": 1
}
```

- `external_id` ≤ 200 rune、`source_name` ≤ 100 rune（否则 400 / INVALID_ARGUMENT）；均可空（普通 capture 不设）。
- `external_id` + `source_name` 都非空时构成去重键：
  - 精确命中已有 Capture → **409 / PRECONDITION_FAILED**，`details.existing_capture_id`（「external_id 已导入过」），不创建；
  - `content_hash` 命中 → 仍创建，响应带 `dedupe`（见下）。
- 初始 `memory_card` 用 `title` / `primary_type` / `tags` 覆盖播种；缺省 fallback 派生。初始修订 `source=import`。

响应 201（新建）或 200（`Idempotency-Key` 重放）：

```json
{
  "capture": { "…Capture…" },
  "memory_card": { "…MemoryCard…" },
  "dedupe": {
    "status": "suggested",
    "existing_capture_id": "UUID"
  },
  "replayed": false,
  "request_id": "UUID"
}
```

- `dedupe` 仅在 `content_hash` 命中时出现（`omitempty`）；`status` ∈ `suggested`。
- 409 duplicate 响应：

```json
{
  "error": {
    "code": "PRECONDITION_FAILED",
    "message": "external_id 已导入过",
    "details": { "existing_capture_id": "UUID" }
  },
  "request_id": "UUID"
}
```

## 12.2 批量导入工作流

三阶段：**解析 → AI 补全（可选）→ Commit**。补全失败只记单行 `completion_error`，其他行照常；「按原样导入」始终可用。

### 12.2.1 创建任务（解析）

POST /api/v3/imports → 201

```json
{
  "format": "csv",
  "source_name": "旧备忘录",
  "content": "external_id,content\nt1,第一条\n,第二条",
  "separator": "---",
  "timezone": "Asia/Shanghai",
  "original_filename": "notes.csv",
  "privacy_mode": "standard"
}
```

- `format`：`plain_text` / `csv` / `jsonl`（别名 txt/text/markdown/md → plain_text）；`content` 必填非空。
- `separator`：仅 `plain_text` 使用，默认 `---`（单独一行等于 separator 分段）。
- 解析器语义：
  - plain_text：按单独一行等于 `separator` 分段，空段丢弃，段内空行保留为段落；只填 `content`。
  - csv（UTF-8，`encoding/csv`）：首行表头（trim + 大小写不敏感映射）；`tags` 用 `|` 分隔；`captured_at` 接受 ISO-8601（RFC3339 或 `2006-01-02` → date_only）；经纬度必须成对且 ∈ [-90,90]/[-180,180]，否则该行 `invalid_coordinates`；`content` 空 → `missing_content`。
  - jsonl：逐行 JSON，字段同名；坏行 → `invalid_json`（行号稳定）。
- 创建时即预计算去重（见 12.0），`dedupe_status` 写入每行。

响应：

```json
{
  "job": { "…ImportJob…" },
  "request_id": "UUID"
}
```

### 12.2.2 查询任务

GET /api/v3/imports/:id → 200（响应同上 `{job, request_id}`）

### 12.2.3 预览

GET /api/v3/imports/:id/preview?limit=10&offset=0 → 200

- `limit` 必须为正整数（默认 10）、`offset` 非负（默认 0），否则 400。
- 预览会把 job 状态置为 `previewed`（幂等）。

```json
{
  "preview": {
    "job": { "…ImportJob…" },
    "rows": [ { "…ImportRow…" } ],
    "next_cursor": "10"
  },
  "request_id": "UUID"
}
```

### 12.2.4 AI 补全预览

POST /api/v3/imports/:id/completion/preview

```json
{ "row_numbers": [1, 3] }
```

- `row_numbers` 缺省 = 全部行。
- 门控：`AICompletionEnabled && CloudTextAllowed`（内容发给 LLM）；关闭 → **409 / FEATURE_NOT_ENABLED**（「AI 补全未开启」）。
- 每行算缺失字段 → 复用 R8 `FieldProposalGenerator` 生成提案 → `SetRowCompletion` 持久化。**单行 LLM 失败只记该行 `completion_error`，其他行照常返回**。

```json
{
  "completion": {
    "rows": [ { "…ImportRow（含 completion_proposals）…" } ]
  },
  "request_id": "UUID"
}
```

### 12.2.5 采用提案

POST /api/v3/imports/:id/completion/apply

```json
{
  "row_selections": [
    { "row_number": 1, "proposal_ids": ["UUID"] }
  ]
}
```

- 选定提案置 `accepted`，同行其余 `pending` 置 `rejected`；幂等。
- 采用成功后，Commit 时会给对应行写一条 `source=ai` 的补全修订（provenance `{"source":"ai"}`）。

响应 200：`{ "completion": { "rows": [...] }, "request_id" }`。

### 12.2.6 Commit

POST /api/v3/imports/:id/commit → 200

```json
{
  "duplicate_content_action": "import",
  "row_actions": { "2": "skip" },
  "skip_needs_input": true
}
```

- `duplicate_content_action`：`import`（默认）/ `skip`，作用于 `suggested` 疑似重复行；`row_actions` 按 `row_number` 覆盖（`import`/`skip`）。`skip_needs_input` 本轮恒跳过 needs_input 行并计入报告（字段为前向兼容保留）。
- 提交语义：
  - 已 `imported` 行快进跳过（**幂等**：重复 Commit 不产生新 Capture，计数返回已存结果）；
  - `duplicate_external` → 跳过；`suggested` → 按策略；`needs_input`/校验错误 → 跳过并计入对应统计；
  - 其余行：**`capture_id = 行 id`（确定性 UUID）**，每行独立短事务（`store.BeginTx`）经 `NewCaptureService(txStore.Capture, settings).Create({Kind:import, Text:content, Source:"import", SourceName, ExternalID, …})` 落库，幂等靠 Capture CTE `ON CONFLICT (user_id,id) DO NOTHING`；有 accepted 提案则追加 `source=ai` 修订；成功 → 行 `imported`，失败 → 行 `failed`（**行级隔离：一行失败不影响其他行**）；
  - 全部处理完 → job 置 `completed` 并汇总计数。
- 地点/时间缺失：`captured_at`/地点字段缺失时保持 unknown（不伪装导入时间，不猜测坐标）。

```json
{
  "commit": {
    "imported": 1,
    "failed": 0,
    "skipped": 1,
    "needs_input": 0,
    "total": 2,
    "job_status": "completed",
    "committed_at": "2026-08-29T00:00:00Z"
  },
  "request_id": "UUID"
}
```

### 12.2.7 错误报告

GET /api/v3/imports/:id/error-report?format=json|csv

- 返回 Commit 后未成功导入的行（failed / skipped / needs_input）及 `validation_errors`。
- `format=csv` → `Content-Type: text/csv; charset=utf-8` + `Content-Disposition: attachment; filename="import_error_report.csv"`，表头 `row_number,external_id,status,dedupe_status,error_code,error_message`（多错误 `;` 连接）。
- 默认 JSON → 200：

```json
{
  "entries": [
    {
      "row_number": 2,
      "external_id": "t2",
      "status": "failed",
      "dedupe_status": "none",
      "validation_errors": [
        { "code": "invalid_coordinates", "message": "经纬度无效" }
      ]
    }
  ],
  "request_id": "UUID"
}
```

### 12.2.8 取消

POST /api/v3/imports/:id/cancel → 200（`{job, request_id}`），未完成 job 置 `cancelled`。

## 12.3 数据模型

**ImportJob**：`id`(UUID) / `user_id` / `source_name` / `format`(plain_text|csv|jsonl) / `original_filename`? / `raw_text` / `column_mapping`(JSONB) / `separator` / `timezone`? / `total_rows` / `valid_rows` / `invalid_rows` / `duplicate_rows` / `needs_input_rows` / `imported_rows` / `skipped_rows` / `failed_rows` / `status`(draft|previewed|completed|failed|cancelled) / `committed_at`? / `cancelled_at`? / `created_at` / `updated_at`。

**ImportRow**：`id`(UUID) / `import_job_id` / `user_id` / `row_number` / `external_id`? / `raw_payload`(JSONB) / `normalized_payload`(JSONB) / `content`? / `content_hash`? / `validation_errors`(JSONB) / `dedupe_status`(none|duplicate_external|suggested) / `capture_id`? / `status`(pending|needs_input|importing|imported|skipped|failed) / `completion_proposals`(JSONB) / `imported_at`? / `created_at` / `updated_at`。

**ImportFieldProposal**（复用 R8 类型）：`id` / `field_name` / `proposed_value` / `provenance`(ai|user|import|fallback) / `apply_policy`(safe_auto|suggest_only|forbidden) / `confidence` / `evidence_spans` / `status`(pending|accepted|rejected|expired)。

## 12.4 错误映射

| 场景 | HTTP | code |
|---|---|---|
| 请求体非法 / format 非法 / id 非 UUID / limit≤0 / offset<0 / external_id>200 / source_name>100 / content 为空 | 400 | INVALID_ARGUMENT |
| 未认证或 Token 失效 | 401 | UNAUTHORIZED |
| 任务不存在、属他人 | 404 | NOT_FOUND |
| 任务已 completed（再次 Commit/Cancel） | 409 | PRECONDITION_FAILED |
| AI 补全未开启（completion/preview 且 CloudText 未开） | 409 | FEATURE_NOT_ENABLED |
| 单条导入 external_id 重复 | 409 | PRECONDITION_FAILED（`details.existing_capture_id`） |
| 补全生成失败（LLM 不可用 / 解析失败） | 500 | INTERNAL |

## 12.5 R9 自动化证据

- internal/db/migration/000014_import_tables_migration_contract_test.go：迁移结构、CHECK、部分唯一索引、Down 顺序；
- internal/db/repository/import_repository_integration_test.go：真实 PostgreSQL 原子建任务 / ListRows / MarkRowState / SetRowCompletion / 去重查询 / 唯一索引兜底；
- internal/service/import_parser_test.go：plain-text / CSV / JSONL 表驱动解析；
- internal/service/import_service_test.go：CreateJob 持久化 + 去重标记、Commit 行级隔离、Commit 幂等、补全失败可按原样导入、accepted 提案出 `source=ai` 修订；
- internal/api/import_handler_test.go：401/400/404/409/500 映射、三阶段 happy path、错误报告 CSV content-type；
- internal/service/capture_service_test.go（扩展）+ internal/api/capture_handler_test.go（扩展）：kind=import 归一化、external_id 重复 → 409、content_hash → dedupe.suggested；
- 前端 test/features/imports/：import_api_test / import_notifier_test / import_screen_test + widget_test / settings / capture 入口回归。

## 13. 平台能力与工作流预留（R10）

### 13.1 GET /api/v3/capabilities（公共）

能力广播端点，**公共**（无需 Bearer Token，与 GET /api/v3/meta 同级注册于 `registerV3ContractRoutes`）。客户端据此呈现「规划中」而非可用的假入口；MVP 不据此隐藏既有功能。

GET /api/v3/capabilities → 200：

```json
{
  "mobile_capture": true,
  "web_review": true,
  "workflow_designer": false,
  "workflow_execution": false,
  "supported_capture_sources": ["text", "audio", "import"]
}
```

- `mobile_capture` / `web_review`：当前 MVP 构建恒 true；
- `workflow_designer` / `workflow_execution`：恒 false —— 工作流设计/执行已预留但本版本不开放；
- `supported_capture_sources`：与 `POST /api/v3/captures` 实际接受的 `kind` 一致（`entity.CaptureKindText/Audio/Import`，服务端生成，不硬编码字符串）。

### 13.2 FEATURE_NOT_ENABLED 双状态语义（501 vs 409）

同一 `code=FEATURE_NOT_ENABLED` 在不同资源上使用不同 HTTP 状态，客户端必须按场景区分（禁止用 message 分支）：

| HTTP | 场景 | 语义 | 位置 |
|---:|---|---|---|
| 501 | 服务端**不提供**该能力（预留命名空间 / 未实现） | 任何用户、任何开关下都不可用，纯守卫，无副作用 | `/api/v3/workflows` 守卫（R10 新增，见 §13.3） |
| 409 | 服务端提供、但**当前用户的 AI 开关关闭** | 用户可去设置开启后重试 | R8/R9 AI 补全 / 导入补全门控（§11 / §12，**不得改动既有 409**） |

### 13.3 预留 /api/v3/workflows 命名空间（受保护，501）

`/api/v3/workflows` 及 `/api/v3/workflows/*` 为本轮起**保留命名空间**。在 `setupRoutes` 的 protectedV3 块顶部无条件注册 `registerWorkflowGuardRoutes`（纯守卫、无 service 依赖），**任何**方法/路径返回 501 / FEATURE_NOT_ENABLED：

```json
{
  "code": "FEATURE_NOT_ENABLED",
  "message": "workflows are not available in this build",
  "request_id": "UUID",
  "details": { "workflow_designer": false, "workflow_execution": false }
}
```

- 需登录：注册于 protected 前缀，未带 Token → 401（守卫不遮蔽前缀鉴权）；游客不可访问（前端路由同步守卫）；
- **无副作用**：守卫不创建 run / 后台任务 / 外部调用，DB 计数不变（B3 集成测试断言）；
- 预留子资源（后续轮次开放）：`POST /workflows`（创建）、`GET /workflows`（列表）、`GET/PUT/DELETE /workflows/:id`、`PUT /workflows/:id/draft`、`POST /workflows/:id/validate`、`POST /workflows/:id/publish`、`POST /workflows/:id/run`；
- gin 通配符 `group.Any("/workflows")` + `group.Any("/workflows/*any")` 仅守卫该前缀，不吞兄弟路由，v3 NoRoute 不受影响。

### 13.4 同步与多设备收敛（MVP server-authoritative）

MVP **不做**增量游标 / 服务端推送同步引擎。收敛模型（G8 完成条件 1）为 **server-authoritative**：

- 客户端设备保存本地捕捉后，经既有「capture_id 稳定幂等推送」（R3 已建）上传；
- 服务端为权威：`(user_id, capture_id)` 幂等；**同规范化内容重放 → 200 + replayed=true**；**同 capture_id 不同内容 → 409 / IDEMPOTENCY_CONFLICT**；GET 拉取后各设备最终一致、无重复（同一 Capture 手机/Web 各一条）；
- 游客记录（本地 `ownerUserId == null`）登录后首次自动同步经 `claimOwner` 认领为当前账号；按 capture_id 幂等，重复同步不重复上传；
- 增量游标 / 服务端推送 / 冲突合并面板为**预留项**，不在 MVP 实现。

### 13.5 R10 自动化证据

- internal/api/capabilities_handler_test.go：200 + 5 键断言（mobile_capture/web_review true、workflow_* false、sources=text/audio/import）+ 公共无鉴权 200；
- internal/api/workflow_guard_test.go：create/list/get/draft/validate/publish/run/delete 全 → 501 + details 两 false；守卫不遮蔽 protected 前缀鉴权；
- internal/api/r10_convergence_integration_test.go（真实 PG）：设备 A 创建 → Web 幂等重放 200（行数恰 1）→ 冲突 409（行数仍 1）→ capabilities 公共 200 → workflows 带鉴权 501 且 capture_outbox 计数不变；
- 前端 test/features/platform/：capabilities_api_test（模型/路径）+ capabilities_notifier_test（data/error 回退全关闭）；
- 前端 test/features/workflows/workflow_screens_test.dart + test/widget_test.dart：规划中页 / SnackBar 非假编辑器 / 游客 /workflows → /login / 设置合并状态与入口跳转。

---

# 14. 回响接口（R11 / G9）

## 14.0 设计决策（约束本契约）

- **回响 = 按需生成 + 客户端本地通知，无推送设施、不依赖 Temporal**。服务端只在
  `GET /echoes/current` 被读取时按 cadence 决定「现在是否应该有一张卡片」，并**每次恰返回一条**
  记忆卡片 + 出现原因；提醒只由客户端本地通知调度（属于客户端行为，不在本契约范围内）。
- **服务端单行纪律**：`user_echoes` 里一个用户同一时刻至多一条 `open`。重复打开/通知点击
  都读到同一条 open 行（跨打开稳定）；超窗未答的 open 在下次读取时滚动为 `expired` 并生成下一条。
- **cadence 由服务端在读取时强制**（锚定最近一条任意状态 echo 行的 `created_at`），客户端设置的
  投递时刻/静默时段只决定「何时邀请」，不决定「何时真正到期」。
- **settings 服务端管控**：镜像 AI settings 的 revision CAS 乐观并发。
- 客户端登录限定：回响体验需要账号（卡片来自云端记忆）；游客不访问这些端点。

## 14.1 数据模型

`user_echo_settings`（每个用户至多一行）：

| 列 | 类型 | 说明 |
|---|---|---|
| user_id | UUID PK → users(id) ON DELETE CASCADE | |
| enabled | BOOL NOT NULL DEFAULT FALSE | 默认关闭，用户显式开启 |
| cadence | VARCHAR(20) CHECK in ('daily','every_other_day','weekly') | 步进天数 1/2/7 |
| revision | BIGINT NOT NULL DEFAULT 0 | 乐观并发 |
| created_at / updated_at | TIMESTAMPTZ | |

`user_echoes`（回响行）：

| 列 | 类型 | 说明 |
|---|---|---|
| id | UUID PK | 客户端深链 / 反馈目标 |
| user_id | UUID NOT NULL | |
| capture_id | UUID NOT NULL | FK (user_id,capture_id) → captures(user_id,id) ON DELETE CASCADE |
| status | VARCHAR(16) CHECK in ('open','done','later','not_relevant','expired') | 见 14.3 |
| reason_code | VARCHAR(24) CHECK in ('first_echo','pinned','oldest','reminder') | 见 14.4 |
| created_at / updated_at | TIMESTAMPTZ | |
| resolved_at | TIMESTAMPTZ NULL | 反馈/过期时间 |

索引：`(user_id, created_at DESC)`；`(user_id, capture_id, created_at DESC)`。
同一卡片冷却期后可再次回响 → 不建 (user_id, capture_id) 唯一。

## 14.2 GET /users/me/echo-settings

受保护。返回当前用户回响设置；无行时返回服务端默认值（不落库）。

请求：无。

响应 `200`：

```json
{
  "settings": {
    "user_id": "5f0c1a86-...",
    "enabled": false,
    "cadence": "daily",
    "revision": 0,
    "created_at": "2026-09-01T08:00:00Z",
    "updated_at": "2026-09-01T08:00:00Z"
  },
  "request_id": "3b6a0f70-..."
}
```

## 14.3 PATCH /users/me/echo-settings

受保护。部分更新，`expected_revision` 必填并参与 CAS。

请求体：

```json
{ "expected_revision": 0, "enabled": true }
{ "expected_revision": 1, "cadence": "every_other_day" }
```

- 至少一个业务字段（`enabled` / `cadence`）必须出现；`cadence` 必须 ∈ daily/every_other_day/weekly。
- 成功：`revision` 自增 1，返回 14.2 形状（`200`）。
- 首启：无行时服务端以 revision 1 建行（无需客户端先 GET）。
- 错误：401 UNAUTHORIZED（未鉴权）；400 INVALID_ARGUMENT（缺 `expected_revision` /
  负值 / 非法 cadence / 空 body）；409 VERSION_CONFLICT（`expected_revision` 与服务端不一致，
  body `{"error":{"code":"VERSION_CONFLICT",...}}`）。

## 14.4 GET /echoes/current

受保护。读取「当前回响」，必要时按 cadence 生成新行。HTTP 语义：**一切空态都返回 200 + 显式空负载**，
不报错。

### 判定顺序（服务端确定性算法）

1. 读 settings（无行取默认）。`enabled=false` → 空负载（仅 enabled/cadence/revision）。
2. 取该用户最近一条 echo 行（任意 status）：
   - 是 `open` 且未超窗（now < created_at + cadence 步长）→ **复用该 open**，返回它的卡片
     （重复读取/通知点击得到同一 `echo.id`）；
   - 是 `open` 但超窗 → 先更新为 `expired`，再走节奏门；
3. 节奏门（锚 = 最近一条 echo 行的 `created_at`）：now 仍在 created_at + 步长之前 →
   空负载 + `next_due_at`（此时没有新 open 产生）；
4. 创建下一回响：选择候选记忆（见 14.5）；无候选 → 空负载 + `empty_reason="no_candidates"`；
   有候选 → 插入一条 `open` 并返回其卡片。

### 响应 `200`（卡片态）

```json
{
  "enabled": true,
  "cadence": "daily",
  "revision": 1,
  "echo": {
    "id": "0d8f2c1a-...",
    "status": "open",
    "reason": { "code": "pinned", "text": "这条记忆被你置顶过，适合专门回看" },
    "memory": {
      "capture_id": "a1b2c3d4-...",
      "kind": "text",
      "title": "关于回响的设计",
      "summary": "…",
      "primary_type": "idea",
      "captured_at": "2026-08-01T09:00:00Z",
      "is_pinned": true
    },
    "created_at": "2026-09-01T09:00:00Z"
  },
  "request_id": "6f9b2..."
}
```

`memory.capture_id` 是深链目标：客户端「点通知/点卡片 → /memories/:captureId」。

### 响应 `200`（三种空态，`echo` 缺省）

```json
{ "enabled": false, "cadence": "daily", "revision": 0, "request_id": "..." }
{ "enabled": true, "cadence": "daily", "revision": 1,
  "next_due_at": "2026-09-02T09:00:00Z", "request_id": "..." }
{ "enabled": true, "cadence": "daily", "revision": 1,
  "empty_reason": "no_candidates", "request_id": "..." }
```

客户端据此分四态：disabled / off_period（有 `next_due_at`）/ no_candidates / loaded。

### 候选选择与原因（14.5）

候选：`captures` JOIN `memory_cards`，需 `captures.deleted_at IS NULL AND lifecycle_status='active'`
且卡片 `processing_status='ready'` 且标题非空；排除近窗已回响的卡片
（`not_relevant` 90 天内，其余 status 14 天内不再回响同一卡片）。
排序：`is_pinned DESC` → 最近回响时间 ASC（NULLS FIRST，未回响过优先）→ `captured_at ASC` → `id ASC`，
LIMIT 1。确定性、无推荐引擎。

`reason_code` 分类（确定性小集合）：

| code | 触发条件 | 文案（服务端实时渲染） |
|---|---|---|
| first_echo | 该用户尚无任何历史回响 | 从你较早记下、还没回看过的记忆开始 |
| pinned | 该卡片被置顶 | 这条记忆被你置顶过，适合专门回看 |
| oldest | 有历史回响但该卡片从未回响 | 这是你较早记下、还没有回看过的想法 |
| reminder | 其余情况（冷却期后再回来） | 距离上次看到它已经过了一段时间，再想想也许有新角度 |

## 14.6 POST /echoes/:echoID/feedback

受保护。对当前 open 回响提交完成/稍后/无关反馈；成功后该行离开 open，下一次读取按 14.4
节奏门生成下一条。

请求体：

```json
{ "verdict": "done" }
```

`verdict` ∈ `done` | `later` | `not_relevant`。

状态迁移：`open → {done, later, not_relevant}`；服务端侧由读取时完成 `open → expired`。

响应 `200`：

```json
{ "echo_id": "0d8f2c1a-...", "status": "done",
  "next_due_at": "2026-09-02T09:00:00Z", "request_id": "..." }
```

`next_due_at` = 该行 `created_at` + cadence 步长，供客户端展示「下次回响」空态。

错误：401 UNAUTHORIZED；400 INVALID_ARGUMENT（`echo_id` 路径参数非法 / 缺 `verdict` /
非法 verdict）；404 NOT_FOUND（echo 不存在或属于其他用户）；**409 VERSION_CONFLICT
（echo 已非 open —— 其他设备已处理 / 已过期）**。

## 14.7 错误码与冻结

本组接口只使用既有冻结错误码：UNAUTHORIZED / INVALID_ARGUMENT / NOT_FOUND /
VERSION_CONFLICT / INTERNAL；**不新增错误码**。

## 14.8 R11 自动化证据

- internal/api/echo_handler_test.go：settings GET/PATCH（开启/改 cadence/revision CAS 409/
  enabled=false→空）、current 卡片态与三种空态、feedback done/later 200、404 跨用户、409 非 open；
- internal/api/r11_echo_integration_test.go（真实 PG）：默认关闭 → 开启 daily → seed 2 卡片
  → current 两次同 echo.id 且行数恰 1 → feedback done → off_period 行数仍 1 → 重复反馈 409 →
  SQL 前移 created_at 2 天 → 出新 open 且选中另一卡片；not_relevant 后 90 天排除生效；跨用户隔离。
- 前端 test/features/echo/：echo_api / echo_notifier / echo_settings_notifier / echo_screen /
  echo_settings_screen / echo_scheduler + test/widget_test.dart 深链（通知 → /memories/:captureId）。

