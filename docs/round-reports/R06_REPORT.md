# 织脑（WeaveBrain）R6 报告 — 记忆卡、记忆流与详情（G4）

> 轮次：R6 — 记忆卡、记忆流与详情
> 目标：G4（记忆流倒序+筛选+搜索、详情按 ID、修正/续写/置顶/归档/删除、主导航四标签）
> 日期：2026-08-30
> 范围：后端（迁移 000010 / fallback 卡 / EnrichmentRevision / 记忆流 7 端点）+ 前端（记忆流与详情页 + 主导航调整）

## 1. 阶段结论

R6 已全部完成并全绿：`go build ./...` / `go vet ./...` 通过，`go test ./...` **227** 个用例全绿（R5 143 → +84）；`dart analyze lib test` 无问题，`flutter test` **113** 个用例全绿（R5 65 → +48）。G4 八步 + 7 项完成条件全部达成：

- 捕获创建时**原子写入 fallback 记忆卡 + fallback 修订**（无 AI 也能显示可用卡，原文始终只读）；
- **EnrichmentRevision** 修订溯源（source=fallback/ai/user、card_version、source_revision、changes/provenance）；
- 记忆流 **倒序 + keyset 游标分页 + 基础筛选 + 三列全文搜索**（pg_trgm GIN）；
- 记忆详情**按路径参数 captureId 加载**（原文/音频/转写/修订历史）；
- **修正 / 续写 / 置顶 / 归档 / 删除** 全链路，并发修订冲突 → 409 VERSION_CONFLICT；
- 主导航调整为**四标签：记忆 / 全局捕捉 / 回响 / 我的**；Timeline/Projects/想法降级为隐藏路由。

真机 / 端到端联调（含真实 PostgreSQL 搜索索引命中、Web 刷新恢复实操）留待设备验证。

## 2. 交付能力

| 能力 | 说明 |
|---|---|
| fallback 卡 | 捕获创建时同语句原子写：title 前 30 rune、summary 前 200 rune（+…）、tags=[]、key_points=[]、primary_type='uncategorized' |
| 修订溯源 | `memory_card_revisions` 表（迁移 000010）：revision 升序、card_version、source（fallback/ai/user CHECK）、source_revision、changes/provenance JSONB、UNIQUE(user_id,capture_id,revision) |
| 记忆流列表 | GET /api/v3/memories：is_pinned DESC, created_at DESC, id DESC；keyset 游标 base64url(JSON{p,t,i})；lifecycle_status 默认 active、kind/primary_type/pinned 过滤 |
| 全文搜索 | `q` 三列 ILIKE 子串（title / original_text / transcript.text），`\ % _` 转义，pg_trgm GIN 索引 |
| 记忆详情 | GET /api/v3/memories/{captureId}：capture+card+audio?+transcript?+revisions（修订降序） |
| 修正 | PATCH：部分字段（title/summary/primary_type/tags/key_points），校验 + 产生 user 修订 + version+1；并发 → 409 |
| 续写 | POST /notes：追加笔记（changes={"note": text}），不改任何卡片字段与原文 |
| 置顶 / 归档 / 删除 | POST /pin（pinned_at）/ POST /archive（幂等）/ DELETE（trashed + deleted_at） |
| Flutter 记忆流 | 列表（搜索防抖 300ms + 筛选 chips + 下拉刷新 + loadMore + 空/错态）+ 详情（原文/字段/续写/修订历史/修正/置顶/归档/删除，全部测试 Key 化） |
| 主导航 | StatefulShellRoute 4 分支（记忆/捕捉/回响/我的）；guest 重定向 /→/capture；`/memories/:captureId` 顶层隐藏路由按 ID 加载 |

## 3. 设计要点

- **fallback 原子落库**：capture 创建的单条 CTE 同时写 capture + memory_card + 初始 fallback revision（revision=1/card_version=1/source=fallback/source_revision=1）。AI pipeline 保持 no-op 至 R8 → 天然满足「无 AI 可用卡」与「AI 失败不覆盖原文」。
- **修订而非覆写**：任何用户编辑（修正/续写）只**追加** EnrichmentRevision + `card.version+1`，`captures.original_text` 全程只读；`source_revision = captures.version` 提供溯源锚点。
- **并发安全**：修订号 = `MAX(revision)+1` 在单条 CTE 内计算，撞 `UNIQUE(user_id,capture_id,revision)` → 409 VERSION_CONFLICT；R6 不做卡片乐观锁（若两并发 correct，后写者需刷新重试）。
- **keyset 游标**：排序键 `(is_pinned, created_at, id)` 三元组稳定，游标编码最后一行 → 翻页不丢/不重；契约要求翻页携带相同筛选。
- **搜索退化接受**：pg_trgm trigram ≥ 3 字符限制使 1–2 字符中文可能顺序扫描，MVP 接受（research note 记录）。
- **软删语义**：delete 置 trashed + deleted_at（列表/详情 `deleted_at IS NULL` 隐藏）；audio 资产暂不级联删除。
- **Flutter 三层可测抽象**：`MemoryGateway`（API）+ `MemoryListNotifier`/`MemoryDetailNotifier`（sealed state）+ 页面；Riverpod `overrideWithValue` 注入 fake；详情页从 `pathParameters['captureId']` 恢复（非 extra）。

## 4. 测试情况

### 后端（227 全绿）

- **memory_repository_test.go（新增）**：List 所有权 + `deleted_at IS NULL` + 默认 active + 排序 + 三列搜索 q + tag/kind/pinned 过滤 + 游标谓词 + ListRevisions 降序 + AppendRevisionAndUpdateCard 原子 CTE + SetPinned pinned_at + SetLifecycle trashed→deleted_at；
- **memory_service_test.go（新增）**：Correct→user revision + version 递增 + provenance；校验失败；AppendNote 不改字段；trashed→404；SetPinned 切换；Archive 幂等；Delete→trashed；GetDetail nil audio/transcript 容忍；fallback 字段派生；
- **memory_handler_test.go（新增）**：list 200+next_cursor / 筛选透传 / detail 200+revisions / 404 / PATCH 200+400 / notes 200+空文本400 / pin / archive / delete / 401 / 各响应含 request_id；
- **migration_contract_test.go（更新）**：迁移 000010 结构、pg_trgm、GIN 索引、source CHECK、Down 顺序；
- **capture_repository/service/handler 与 outbox_worker（更新）**：`Create` 签名带初始 revision、fallback 派生字段、AI 关创建后 fallback revision 存在；
- **既有回归**：api / migration / repository / service / stt / crypto 全部保持绿。

### 前端（113 全绿）

- **memory_api_test.dart（20）**：fromJson / list query params（cursor/limit/q/filters）/ detail 路径 / correct 只发提供字段 / note `{text}` / pin `{pinned}` / archive/delete 路径 / 非 map 抛 FormatException；
- **memory_notifier_test.dart（14）**：list/loadMore 传 cursor/search 重置/筛选；detail 按 ID；correct 更新项 + message；addNote/setPinned/archive/delete；409/404 → 提示；
- **memory_list_screen_test.dart（5 widget）**：渲染卡片 + 每卡唯一主按钮 / 空态 / 错态重试 / 搜索防抖 / load-more 游标；
- **memory_detail_screen_test.dart（8 widget）**：按 captureId 加载 / 重建同 ID 再次按 ID 加载 / 修正 / 续写 / 置顶 / 归档 / 删除确认后 pop / 409 提示；
- **widget_test.dart（2）**：guest 重定向 + 底部导航恰 4 目的地（旧路由不在导航）/ 登录用户落记忆流 + 点卡进 `/memories/c1` 按 ID 加载；
- **既有回归**：capture 全量 + settings 全绿。

## 5. 修复的问题

1. **详情页「取消归档」无后端对应**：R6 只提供幂等 `archive`（无取消归档端点），原 UI 在已归档时显示「取消归档」会误导。改为固定「归档」按钮（幂等），并在 API 契约与本节记录限制。
2. **Flutter widget 测试 warnIfMissed**：详情页动作按钮在首屏折叠线下方，`tap` 前补 `ensureVisible`。
3. **测试文案冲突**：列表/详情测试的摘要默认值 `'摘要'` 与「整理字段」区块的「摘要」标签冲突，改为 `'这是一段摘要'`。
4. **Flutter analyzer 3 处告警**：两个测试文件未用 import、`_FakeGateway` 未用构造参数，清理后 `dart analyze lib test` 0 error / 0 warning。

## 6. 完成条件核对（G4）

- [x] 无 AI 时也能显示可用 fallback 卡（fallback 字段 + revision 原子生成；前端列表/详情渲染）
- [x] AI 失败不覆盖原始内容（original_text 只读；AI pipeline no-op 至 R8；续写不改字段）
- [x] Web 详情页刷新后按 ID 恢复（路径参数 captureId；widget 测试重建同 ID 再次加载）
- [x] 每张卡首屏只有一个主要下一步（每卡恰一个 FilledButton，按 primary_type 映射）
- [x] 用户编辑产生新版本（Correct → source=user 修订 + card.version+1）
- [x] 搜索结果可追溯到原始记忆（q 三列搜索含 original_text/transcript；详情展示原始区块）
- [x] 删除和归档状态正确同步（SetLifecycle archived/trashed + deleted_at；前端状态更新）

## 7. 已知限制与后续

- **无取消归档/回收站还原**：R6 归档幂等、删除软删，均不可逆操作；恢复/回收站属后续轮次（G13 回收站级联）。
- **存量捕获无 fallback revision**：仅新捕获原子写初始修订；存量卡列表可显示、详情修订历史为空（G4 条件针对新捕获；可选一次性回填）。
- **并发编辑**：两并发 correct 后写者收 409，客户端刷新重试；R6 未做卡片乐观锁。
- **1–2 字符中文搜索可能不走 trigram 索引**：数据量增大后可换 pg_bigm / zhparser。
- **AI 组织留 G6/R8**：真实 AI 写 `ai` 修订；`source_revision` 届时应指向实际转写修订。

## 8. 下一步（G5 / R7）

验收 B：核心捕捉与 AI 控制（原始捕捉丢失为 0、AI 关闭时后台整理调用为 0、95% 任务最终 ready 或明确失败、原音频/原文/修正版可追溯、体验样本、严重权限与数据串用户问题为 0、R7 验收报告）。

## 9. 证据

- `go build ./...` / `go vet ./...`：通过
- `go test ./...`：227 用例全绿
- `dart analyze lib test`：0 error / 0 warning
- `flutter test`：113 用例全绿
- docs/API_V3_CONTRACT.md §10（记忆流 7 端点 + 数据模型 + 搜索/游标/错误契约）；docs/DEVELOPMENT_GOALS.md G4 八步 + 7 完成条件全部勾选；docs/topic_notes/r6_memory_stream_search.md 选型记录
