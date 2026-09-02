# 织脑（WeaveBrain）R8 报告 — AI 补全（G6）

> 轮次：R8 — AI 补全
> 目标：G6（CompletionProposal / FieldProposal、completion:preview / apply、evidence_spans / safe_auto/suggest_only/forbidden、逐字段采用 + 全部采用安全字段 + 版本与撤销、详情页补全入口 + 旧提案过期）
> 日期：2026-08-31
> 范围：**后端**（completion_proposals 迁移 + 实体/仓储/服务/LLM 生成器/处理器 + 测试）+ **前端**（completions feature：preview/apply/undo 面板 + 详情页接入 + 测试）+ **文档**（API_V3_CONTRACT §11 / DEVELOPMENT_GOALS G6 / MEMORY.md Sprint 20 / 本报告）
> 前置：R7 完成（后端 242 含真实 PG 集成全绿、前端 113 全绿）

## 1. 阶段结论

R8 AI 补全（G6）全部完成，后端在真实 PostgreSQL 上全绿：

- `go build ./...` / `go vet ./...` 通过；
- `go test ./...`（无 env，单元 + 迁移契约，集成测试自动 skip）**301** 用例全绿（R7 236 → +65）；
- `WEAVEBRAIN_TEST_DATABASE_URL=<test-db> go test ./...` **308** 用例全绿 —— 新增 `completion_repository_integration_test.go` 在真实 PG 上实际运行（R7 242 → +66）；
- `flutter test` **144** 用例全绿（R7 113 → +31）；`dart analyze lib test` 0 问题（`flutter analyze` 的 analysis server 在本机仍崩溃，改走 `dart analyze`，见 §5）。

G6 七项完成条件全部达成（详见 §6）：preview 不改业务对象、apply 只填空字段、用户/导入值受保护、事实型建议有证据、source_revision 变化后旧提案不可应用、应用后可撤销、AI 补全关闭时不展示或调用入口。

**关键决策**（用户批准 3 项均按推荐执行）：真实 LLM 生成（本地 Ollama，`LLM_BASE_URL/LLM_API_KEY/LLM_MODEL` 默认 `http://localhost:11434/v1` / `ollama` / `qwen2.5:7b`，45s 超时）；完整撤销（后端 undo 端点）；不加 completion_mode 设置项（apply_policy 由静态分类函数按字段计算，机制预留 suggest_only/forbidden）。

## 2. 交付能力

| 能力 | 说明 |
|---|---|
| 字段提案模型 | `completion_proposals` 表（迁移 000013）：一行 = 一个字段提案，同批次共享 preview_id；`field_name` CHECK ∈ title/primary_type/summary/tags/key_points；provenance / apply_policy / status CHECK；evidence_spans JSONB；FK 级联到 captures(user_id,id) |
| preview（不改业务对象） | 门控（AICompletion && CloudTextAllowed）→ 校验存在 + ready → 计算缺失字段 → 新 preview 使旧 pending 全部过期 → 调 Ollama 严格 JSON 生成（FieldProposal + 证据引文）→ 落库；`source_revision=card.Version`；对 capture/card 零改动 |
| apply（只填空 + 版本安全） | 门控（仅 AICompletion）→ 幂等（全 accepted no-op）→ 版本守卫（pending.SourceRevision != card.Version → 过期 + 409）→ 非空字段保护（置 rejected 跳过）→ 一条 `source=ai` 修订（`provenance._completion` 存 undo 原值）+ `card.version`+1 → 提案置 accepted |
| undo（完整撤销） | 找最新一条 `source=ai` 且带 `_completion` 的修订 → 只回滚「当前值仍 == AI 所设」字段（用户后续编辑跳过）→ `source=user` 撤销修订；提案保持 accepted 审计留痕 |
| apply_policy 机制 | 5 个目标字段有证据 → safe_auto（批量采用）；无证据 → 降级 suggest_only（逐字段确认）；其他 → forbidden；机制为 R9 导入复用预留 |
| LLM 生成器 | `FieldProposalGenerator` 接口 + `LLMFieldProposalGenerator`（Eino `agent.NewChatModel`）；容忍 code-fence/杂散文本（取首个 `[` 至末个 `]`）；过滤非法字段与不在 missingFields 的项；长度校验（title≤300 / summary≤2000 / tags≤20×100 / key_points≤3×500）+ primary_type 白名单；非数组 → `ErrCompletionLLM`（500）绝不静默不应用 |
| 前端补全面板 | `completion_api.dart`（模型 + Gateway + Api）+ `completion_notifier.dart`（sealed state：preview/applyAllSafe/applyOne/undo/clearMessage；409 分码提示）+ `completion_panel.dart`（入口开关 / 提案卡片 + 证据 chip + 「AI 建议/需确认」chip + 置信度 / 批量采用 + 逐字段采用 + 撤销）；详情页 `_aiCompletionAvailable` 门控（AI 开启 && ready） |

## 3. 设计要点

- **undo 原值存 `provenance._completion.undo`**（关键偏差）：`EnrichmentRevision.changes` 是 flat map（`{field: newValue}`），无法表达逐字段 from/to；故 AI apply 修订在 `provenance["_completion"]` 存 `{"preview_id":…, "undo":{field:原值}}`，undo 时据此回滚。已记入 API_V3_CONTRACT §11.4。
- **`proposed_value` 用 TEXT**：tags/key_points 存 JSON 数组字符串，解码逻辑只在服务 apply + 前端 model 两处，配单测锁定格式（`completion_api_test.dart` / `completion_service_test.go`）。
- **source_revision 语义**：提案的 `source_revision = MemoryCard.Version`（并发守卫）；补全修订的 `source_revision = captures.Version`（与 Correct 一致，审计口径）。
- **primary_type="uncategorized" 视为缺失**：fallback 恒填 `uncategorized`，否则永不建议；title/summary 已有 fallback 值 → 默认受保护，本轮不覆盖（改写留 R9+）。
- **门控分层**：preview 把原文发给 LLM → 需 `AICompletionEnabled && CloudTextAllowed`；apply 不发文本 → 仅 `AICompletionEnabled`。前端详情页在 AI 关闭 / 卡片未 ready 时渲染 `SizedBox.shrink()` 且零调用（标准 7）。
- **preview 使该 capture 全部 pending 过期**：新 preview 取代旧批次，保证同时只活跃一套提案。
- **undo 跳过用户后续编辑**：只回滚当前值仍等于 AI 所设值的字段，绝不覆盖用户改过的值。
- **confidence REAL 精度**：Postgres float4 存 0.9 得 0.899999976，集成测试用 `math.Abs` 容差断言（保留计划里的 REAL 列）。
- **生成器 nil 容忍**：Ollama 不可用时 `NewLLMFieldProposalGenerator` 失败 → 服务内 generator 为 nil → Preview 返回 `ErrCompletionLLM`（500），服务仍可启动；Apply/Undo（不触网）不受影响。

## 4. 测试情况

### 后端单元 + 契约（无 env，301 全绿；R7 236 → +65）

- **completion_service_test.go（17）**：preview 门控关闭（不调 generator）/ not found / not ready / 无缺失字段（不调 LLM 且旧 pending 过期）/ 正常生成+落库 safe_auto+证据 / LLM 错误；apply 门控 / 正常（只填空、ai 修订带 `_completion` provenance、提案 accepted、card.version+1、revision MAX+1）/ 幂等全 accepted no-op / 版本冲突（过期 + 409）/ 保护非空字段 / 混合 pending+accepted；undo 无补全 → ErrNothingToUndo / 正常（回滚 title/summary/tags、user-source undo 修订、跳过用户改过的字段）/ 连续两次 undo 第二次 ErrNothingToUndo；
- **completion_llm_test.go（3）**：strict JSON 数组解析、field 过滤 + 长度校验 + primary_type 白名单、code-fence / 杂散文本容忍；
- **completion_handler_test.go（7）**：401、非法 captureId 400、三端点 happy path 断言 JSON 形状 + 输入透传、请求体非法 400、错误码映射表（FEATURE_NOT_ENABLED / PRECONDITION_FAILED / VERSION_CONFLICT / NOT_FOUND / INVALID_ARGUMENT / INTERNAL）；
- **000013_completion_proposals_migration_contract_test.go（1）**：Up 结构 + 三个 CHECK + 索引 + Down 顺序；
- 既有回归：api / migration / repository / service / stt / crypto 全部保持绿。

### 后端集成（真实 PostgreSQL，308 全绿；R7 242 → +66）

- **completion_repository_integration_test.go（新）**：种子 capture+card → CreateProposals（confidence 浮点容差）→ ListByIDs / ListPendingByCapture → MarkAccepted（幂等守卫）→ ExpireAllPending → 状态迁移全绿；
- 既有集成回归（R3 / R4 / R7 的 capture / cross-user / orphan-recovery / soak / traceability）全部保持绿。

### 前端（flutter test 144 全绿；R7 113 → +31）

- **completion_api_test.dart（9）**：模型蛇形映射（scalar/array/confidence null）、suggest_only/accepted 不可自动采用、preview/apply/undo 端点 + 路径/body/unwrap、畸形信封 → FormatException、非 2xx 重抛 ApiException；
- **completion_notifier_test.dart（11）**：loadPreview 发布 loaded / 失败发布 error、applyAllSafe 只发 safe_auto pending ids + source_revision、suggest_only 跳过、空 → null 不调 apply、applyOne、版本冲突 409 →「提案已过期，请重新生成」、FEATURE_NOT_ENABLED →「AI 补全未开启」、undo、undo 409 → 保留预览、clearMessage；
- **completion_panel_test.dart（8）**：disabled 隐藏 + 零调用、入口加载并渲染提案 + 证据 chip + 置信度、apply-all-safe ids、逐字段 apply、undo、suggest_only 隐藏逐字段 apply +「需确认」、空提案「暂无缺失字段」、preview 错误提示 + 重试；
- **memory_detail_screen_test.dart（+3）**：AI 关闭隐藏面板、卡片非 ready 隐藏、开启 + ready 显示入口；
- **widget_test.dart（更新）**：guest/登录全链路回归（新增 aiSettingsGatewayProvider fake override，详情页补全面板不再触发真实 HTTP）。

### 工具链

- `go build ./...` / `go vet ./...`：通过；
- `goose` 迁移 000013 up/down 可逆（down 后重 up 验证）；迁移 000001—000013 在 weavebrain_test 库全量应用；
- `dart analyze lib test`：0 issue。

## 5. 修复的问题

1. **集成测试 `completion_proposals` 表不存在（SQLSTATE 42P01）**：迁移 000013 未应用 → `goose up` 到版本 13；
2. **confidence 浮点精度（REAL=float4）**：`*tagsProp.Confidence != confidence` 断言失败 → 改 `math.Abs(*tagsProp.Confidence-confidence) > 1e-3` 容差（保留计划 REAL 列）；
3. **前端 panel `_proposalTile` 缺 ref**：`Undefined name 'ref'` → 签名传 `WidgetRef ref`；
4. **panel 测试「灵感 findsOneWidget」重复值**：两个默认提案同值 `["工作","灵感"]` → key_points 提案改独立值 `["关键点"]` + 补断言；
5. **notifier undo 409 测试状态错位**：未先 loadPreview → 当前为 Initial 出 CompletionError → 先 `loadPreview` 再 undo（真实 UI 只能从 Loaded 进入）；
6. **widget_test pumpAndSettle 超时**：详情页新补全面板经 `_aiCompletionAvailable` 微任务触发真实 `AISettingsApi` HTTP → `_pumpApp` 加 `aiSettingsGatewayProvider` fake override；
7. **`flutter analyze` analysis server 崩溃（exit 255）**：本机环境问题（R4—R7 沿用），`dart analyze lib test` 干净替代。

## 6. 完成条件核对（G6）

- [x] preview 不修改业务对象 —— `CompletionService.Preview` 对 capture/card 零改动（服务测试 + handler 契约）；
- [x] apply 默认只填空字段 —— `fieldEmpty`（primary_type=uncategorized 视为缺失）；非空字段跳过（服务测试）；
- [x] 用户值和导入原值受保护 —— apply 非空字段提案置 rejected 绝不覆盖；undo 只回滚「当前值仍 == AI 所设」字段（服务测试）；
- [x] 事实型建议有证据片段 —— `buildEvidenceSpans` 逐字定位原文首现字节偏移；无证据降级 suggest_only（服务测试 + 前端证据 chip）；
- [x] source_revision 变化后旧提案不可应用 —— pending.SourceRevision != card.Version → MarkExpired + 409 VERSION_CONFLICT（服务测试）；
- [x] 应用后可以撤销 —— undo 端点回滚最近一次 ai 补全（source=user 撤销修订，提案保持 accepted 审计留痕）（服务测试 + 前端 undo）；
- [x] AI 补全关闭时不展示或调用入口 —— preview 门控 AICompletion && CloudTextAllowed；前端 `_aiCompletionAvailable` 未开启/未 ready → `SizedBox.shrink()` 零调用（widget 测试）。

## 7. 已知限制与后续

- **title/summary 受保护，本轮不改写**：fallback 已派生 title/summary → 默认 safe 保护；「在用户确认后改写」留 R9+（需显式确认 UI）。
- **真实 LLM 严格 JSON 遵从度有限**：`qwen2.5:7b` 偶发 code-fence/杂散文本 → 解析器取首个 `[` 至末个 `]` 容忍；非数组 → 500（ErrCompletionLLM），绝不静默不应用。
- **Ollama 不可用**：Preview 返回 500；Apply/Undo 正常（不触网）。docker-compose Ollama 服务就绪后即可 smoke。
- **`flutter analyze` 本机崩溃**：沿用 `dart analyze` 替代（R4—R7 已记录）。
- **G7 / R9 待办**：单条与批量导入（复用 apply_policy 机制）、title/summary 显式确认改写、回收站级联等。

## 8. 下一步（G7 / R9）

单条与批量导入：单条文字/音频/文件导入、多段文本/TXT/Markdown/CSV/JSONL 解析、字段映射 + 前 10 条预览、AI 补全缺失项 + 去重 + Commit + 行级错误隔离、结果/错误报告下载。

## 9. 证据

- `go build ./...` / `go vet ./...`：通过
- `go test ./...`（无 env）：**301** 用例全绿（R7 236 → +65；集成测试自动 skip）
- `WEAVEBRAIN_TEST_DATABASE_URL=postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test go test ./...`：**308** 用例全绿（迁移 000001—000013 + completion 仓储集成测试实际运行；R7 242 → +66）
- `goose` 迁移 000013：up/down 可逆验证通过
- `flutter test`：**144** 用例全绿（R7 113 → +31）；`dart analyze lib test`：0 issue
- docs/API_V3_CONTRACT.md §11（AI 补全接口：总则与门控 / 提案模型 / preview / apply / undo / 错误映射 / 自动化证据）
- docs/DEVELOPMENT_GOALS.md G6（5 步 + 7 完成条件全部勾选；顶层表 G6 → R8 完成）
- MEMORY.md Sprint 20
