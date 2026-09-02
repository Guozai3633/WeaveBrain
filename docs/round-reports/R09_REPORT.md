# 织脑（WeaveBrain）R9 报告 — 单条与批量导入（G7）

> 轮次：R9 — 单条与批量导入
> 目标：G7（单条文字导入；多段文本/TXT/Markdown/CSV/JSONL 解析；字段映射 + 前 10 条预览；AI 补全缺失项 + 去重 + Commit + 行级错误隔离；结果报告与错误报告下载）
> 日期：2026-09-01
> 范围：**后端**（迁移 000014 + 单条导入 capture 管道扩展 + import_jobs/import_rows 仓储 + 解析器 + ImportService + 处理器 + 测试）+ **前端**（imports feature：单条/批量 UI + 设置页/捕捉页入口 + 路由 + 测试）+ **文档**（API_V3_CONTRACT §12 / DEVELOPMENT_GOALS G7 / MEMORY.md Sprint 21 / 本报告）
> 前置：R8 完成（后端 301 无 env / 308 真实 PG 全绿、前端 144 全绿、R8 已提交）

## 1. 阶段结论

R9 单条与批量导入（G7）代码全部完成，单元 + 迁移契约测试全绿：

- `go build ./...` / `go vet ./...` 通过；
- `go test ./...`（无 env，单元 + 迁移契约，集成测试自动 skip）**337** 用例全绿（R8 301 → +36）；
- `WEAVEBRAIN_TEST_DATABASE_URL=postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test go test ./...`（真实 PostgreSQL）**356** 用例全绿（R8 308 → +48）—— 迁移 000001—000014 + import_repository 集成 + R3/R4/R7 既有集成回归全部实际运行；
- `flutter test` **184** 用例全绿（R8 144 → +40）；`dart analyze lib test` 0 问题（`flutter analyze` 的 analysis server 在本机仍崩溃，改走 `dart analyze`，见 §5）。

G7 七项完成条件全部达成（详见 §6）：单行错误不影响其他行、1 千行 MVP 导入稳定（8 MiB body + 单事务建任务）、重复 Commit 不产生重复 Capture、补全失败可按原样导入、Commit 前展示字段映射/错误/疑似重复、地点/时间缺失保持 unknown、可下载错误报告。

**关键决策**（用户批准 4 项均按推荐执行）：后端解析批量文本/文件；单条导入仅文字 + 元数据（复用 `POST /captures` kind=import）；入口在设置页 + 捕捉页；去重 = `(user_id, source_name, external_id)` 精确硬性跳过 + 规范化正文 hash 标记疑似由用户决定。

## 2. 交付能力

| 能力 | 说明 |
|---|---|
| 单条导入 | 复用 `POST /captures` kind=import：`external_id/source_name/title/tags/primary_type` 透传；`(user_id, source_name, external_id)` 精确命中 → 409 + `existing_capture_id`；`content_hash` 命中 → 201 + `dedupe.status=suggested`；初始修订 `source=import`；提交后跳详情页可直接用 R8 补全面板 |
| 批量解析 | `import_parser.go`：plainTextParser（单独一行等于 separator 分段，默认 `---`）/ csvParser（UTF-8、表头大小写不敏感映射、`tags` `|` 分隔、`captured_at` ISO-8601 + date_only、经纬度成对且范围校验）/ jsonlParser（逐行 JSON，坏行 `invalid_json` 行号稳定）；`txt/text/markdown/md` 归一化 plain_text；流式 `bufio.Scanner` |
| 去重 | 两级：`duplicate_external`（外部 ID 精确，Commit 硬跳过）+ `suggested`（content_hash 命中，用户经 `duplicate_content_action`/`row_actions` 决定）；DB 部分唯一索引 `uq_captures_user_source_external` 并发安全网 |
| 三阶段工作流 | 解析（CreateJob 原子落库 + 预计算去重）→ 预览（前 10 + column_mapping + 错误/疑似 chip，状态 previewed 幂等）→ AI 补全（门控 AICompletion && CloudTextAllowed；单行 LLM 失败记 error 不阻塞）→ Commit |
| Commit 幂等 + 行级隔离 | `capture_id = import_rows.id` 确定性；每行独立短事务（`store.BeginTx`），幂等靠 Capture CTE `ON CONFLICT (user_id,id) DO NOTHING` + imported 行快进；失败行标 `failed` 其他行照常；accepted 提案出 `source=ai` 修订 |
| 结果与错误报告 | Commit 汇总 imported/failed/skipped/needs_input；`GET error-report?format=csv|json`（CSV 带 Content-Disposition 下载）；前端结果页统计 + Web Blob 下载 / IO 剪贴板复制 |
| 前端 imports feature | `import_api.dart`（ImportGateway 抽象 + ImportApi + 409 → ImportDuplicateException）+ `import_notifier.dart`（sealed ImportState）+ `import_screen.dart`（单条/批量 SegmentedButton）；设置页「导入旧记忆」入口 + 捕捉页 AppBar 导入按钮（已认证）+ `/imports` 路由；IO/Web 条件导出文件读取器 |

## 3. 设计要点

- **确定性 `capture_id = import_rows.id`**（关键）：重复 Commit 幂等不靠状态竞态，而靠 Capture 仓储 CTE `ON CONFLICT (user_id,id) DO NOTHING` + 已 imported 行快进 —— 标准 3 双保险。已记入 API_V3_CONTRACT §12.2.6。
- **去重收敛两级**（用户决策）：外部 ID 精确硬跳过 + content_hash 疑似提示；merge / 资产 SHA-256 / 近似时间去重留 R10+。部分唯一索引是并发安全网。
- **provenance 语义**：导入初始修订 `source=import`（`initialEnrichmentRevision` 参数化 source，不再硬编码 `'fallback'`）；AI 补全修订 `source=ai`。前端记忆流已有 import 标签/图标。
- **行级事务隔离**：Commit 每行独立 `store.BeginTx`（`NewCaptureService(txStore.Capture, settings)`），失败只标该行 `failed`，绝不阻塞其他行（标准 1）。
- **地点/时间缺失保持 unknown**：`captured_at`/地点字段缺失不伪装导入时间、不猜测坐标（标准 6）；`location_name`/经纬度只解析校验进 `normalized_payload`，captures 无列（落库留后续）。
- **单条导入 AI 补全免费获得**：提交后跳 `/memories/$captureId`，R8 补全面板直接可用，无需新端点。
- **解析/补全/Commit 三阶段独立**：补全失败单行记 `completion_error`，其他行照常；「按原样导入」默认可用（标准 4）。
- **门控分层**：批量补全把内容发给 LLM → `AICompletionEnabled && CloudTextAllowed`；关闭 → 409 FEATURE_NOT_ENABLED。
- **8 MiB body + 单事务建任务**：MVP 预算；超 1000 行分批/流式 Commit 与两段上传预留。

## 4. 测试情况

### 后端单元 + 契约（无 env，337 全绿；R8 301 → +36）

- **000014_import_tables_migration_contract_test.go（1）**：Up 结构（captures 3 列 + 部分唯一索引 + hash 索引 + revision CHECK 加 import）、import_jobs/import_rows 全结构、Down 顺序；
- **import_parser_test.go（表驱动）**：plain-text 分隔符分割/多段落、CSV 表头映射/`|` tags/ISO-8601/经纬度成对 + 范围/空 content/坏行隔离、JSONL 合法/坏行/缺 content；
- **import_service_test.go（+11）**：CreateJob 持久化 + 去重标记；GetPreview 前 10 + previewed；Commit 行级隔离（一行坏 → 其余导入）；Commit 幂等（重提 → 无新 capture、计数不变）；duplicate_external 硬跳过；suggested 用户导入/跳过；补全未开启/失败按原样导入；accepted 提案出 `source=ai` 修订；title/tag override；Cancel + error report；
- **capture_service_test.go（扩展）**：kind=import 归一化（空文本拒绝、Source 默认 import、external_id/source_name 校验、override 生效、content_hash 设置、初始修订 source=import、外部 ID 精确 → ErrDuplicateExternalID、content_hash → Dedupe.suggested）；
- **import_handler_test.go**：401 / 400 / 404 / 409 / 409 FEATURE_NOT_ENABLED / 500 映射、三阶段 happy path、错误报告 CSV content-type + 附件头；
- **capture_handler_test.go（扩展）**：kind=import 透传、409 duplicate 映射（existing_capture_id）；
- 既有回归：api / migration / repository / service / stt / crypto 全部保持绿。

### 后端集成（真实 PostgreSQL，356 全绿；R8 308 → +48）

- **import_repository_integration_test.go（新）**：种子 → CreateJob 原子性（job+rows 同事务）→ ListRows / GetRowsByNumbers → MarkRowState / SetRowCompletion → FindExternalDuplicates / FindContentHashMatches → 唯一索引兜底（并发重复插入报错）；
- **import_service 集成（+11）**：CreateJob 持久化 + 去重标记、GetPreview 前 10 + previewed、Commit 行级隔离 / 幂等 / duplicate_external 硬跳过 / suggested 用户策略 / 补全未开启 / 补全失败按原样导入 / accepted 提案出 `source=ai` 修订 / title+tag override / Cancel + error report —— 均在真实 PG 上实际运行；
- 既有集成回归（R3 / R4 / R7 的 capture / cross-user / orphan-recovery / soak / traceability）全部保持绿；
- 迁移 000001—000014 在 weavebrain_test 库全量应用；**000014 Down 可逆**（见 §5）。

### 前端（flutter test 184 全绿；R8 144 → +40）

- **import_api_test.dart**：模型蛇形映射 + 默认值、ImportRowModel 去重/状态 helper、SingleImportDraft.toRequest（含 omit 空选填）、8 端点路径/body/unwrap、importSingle Idempotency-Key + 409 → ImportDuplicateException、非 2xx 重抛；
- **import_notifier_test.dart（16）**：createJob → Created / 失败 → Error；preview → Previewed；completionPreview overlay / 409 FEATURE_NOT_ENABLED 文案；completionApply selections + 消息；commit → Committed / 幂等（重复调用）/ 404 文案；loadErrorReport；importSingle 成功回 Idle / duplicate → Error + existing id / 网络错误文案；reset / clearMessage；
- **import_screen_test.dart（4）**：单条提交成功跳 `/memories/cap-new`；单条 duplicate 错误横幅 + 「查看已存在的记忆」链接；批量 粘贴 → 解析预览（「重复·跳过」chip）→ AI 补全勾选 → 采用 → commit → 「导入完成」统计 → 错误报告（「缺少内容」）；批量重复内容策略切换「仍导入」；
- **widget_test.dart（更新）**：guest/登录全链路回归（新增 importGatewayProvider fake override）；
- **settings / capture 测试**：新入口/按钮回归。

### 工具链

- `go build ./...` / `go vet ./...`：通过；
- `dart analyze lib test`：0 issue。

## 5. 修复的问题

1. **前端 `_applySelections` 错位**：遍历 `state.preview.rows`（无提案）而不是 `state.completion.rows` 覆盖层 → 改为优先用非空 completion 覆盖层（widget 测试暴露的真实代码 bug）；
2. **`ImportJobModel.fromJson(const {})` 抛异常**：`id`/`user_id` 严格 `as String` 对空 map 抛 `Null is not a subtype of String`（notifier commit 兜底路径）→ 改防御性 `?? ''`；
3. **批量 widget 测试按钮不可点**：ListView 内补全/提交按钮在 600px 视口外 → 测试视口放大到 1200×2600；
4. **`ApiException` 未导入**：import_api.dart 只导了 api_client.dart（不复导出）→ 补 `api_exception.dart`；
5. **迁移 000014 Down 违反既有行（SQLSTATE 23514）**：Down 重加 `memory_card_revisions_source_check`（不含 `import`）时，真实 PG 上已有 `source='import'` 修订（集成测试写入 63 行）→ 先在 Down 里 `UPDATE … SET source='user' WHERE source='import'` 再重加约束；契约测试补充断言（update 必须先于 drop constraint）。修复后 down/up 可逆验证通过（63 行 import → user，再 up 回 v14）；
6. **`flutter analyze` analysis server 崩溃（exit 255）**：本机环境问题（R4—R8 沿用），`dart analyze lib test` 干净替代。

## 6. 完成条件核对（G7）

- [x] 单行错误不影响其他行 —— Commit 每行独立短事务，失败行标 `failed` 其他行照常（import_service_test：行级隔离）；
- [x] 1 千行 MVP 导入稳定 —— 单事务建 job + rows（预计算去重），8 MiB body 预算 + 1000 短事务 MVP 可接受；
- [x] 重复 Commit 不产生重复 Capture —— `capture_id=row.id` 确定性 + CTE `ON CONFLICT DO NOTHING` + imported 行快进（import_service_test：幂等，无新 capture 计数不变）；
- [x] 补全失败可按原样导入 —— completion/preview 单行 LLM 失败记 error 不阻塞；Commit 按原样导入（import_service_test）；
- [x] Commit 前展示字段映射、错误和疑似重复 —— 前端预览表格 + 去重 chip（重复·跳过 / 疑似重复 / 待补充）+ 校验错误（import_screen_test）；
- [x] 地点/时间缺失时保持 unknown —— captured_at/地点缺失不伪装导入时间、不猜测坐标；落库保持 unknown（import_service_test + capture_service_test）；
- [x] 可下载错误报告 —— GET error-report?format=csv（text/csv + attachment 头）与 JSON；前端 Web 下载 CSV / IO 复制（import_handler_test + import_screen_test）。

## 7. 已知限制与后续

- **1 千行以上分批/流式 Commit 留后续**：MVP 单次 8 MiB body + 1000 短事务；两段上传预留。
- **地点/坐标只解析校验不落库**：captures 无地点列（R10+ 落库）。
- **`flutter analyze` 本机崩溃**：沿用 `dart analyze` 替代（R4—R8 已记录）。
- **真机 / E2E（Ollama smoke）待验证**：单条导入 + 批量补全的真实 LLM 调用路径。

## 8. 下一步（G8 / R10）

Web 回顾端、同步与工作流预留：Web 记忆流/搜索/详情/导入/设置、登录后游客 Capture 合并、同步游标与冲突处理、`/workflows` 规划中页、`GET /api/v3/capabilities`。

## 9. 证据

- `go build ./...` / `go vet ./...`：通过
- `go test ./...`（无 env）：**337** 用例全绿（R8 301 → +36；集成测试自动 skip）
- `WEAVEBRAIN_TEST_DATABASE_URL=postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test go test ./...`：**356** 用例全绿（R8 308 → +48；迁移 000001—000014 + import 仓储/服务集成 + R3/R4/R7 既有集成回归全部实际运行）
- `goose` 迁移 000014：down/up 可逆验证通过（Down 先重映射 import 修订 → user 再收紧 CHECK）
- `flutter test`：**184** 用例全绿（R8 144 → +40）；`dart analyze lib test`：0 issue
- docs/API_V3_CONTRACT.md §12（导入接口：去重模型 / 单条 kind=import / 8 批量端点 / 数据模型 / 错误映射 / 自动化证据）
- docs/DEVELOPMENT_GOALS.md G7（5 步 + 7 完成条件全部勾选；顶层表 G7 → R9 完成）
- MEMORY.md Sprint 21
