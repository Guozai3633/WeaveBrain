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
