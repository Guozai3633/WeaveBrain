# 织脑（WeaveBrain）R10 报告 — Web 回顾端、同步与工作流预留（G8）

> 轮次：R10 — Web 回顾端、同步与工作流预留
> 目标：G8（Web 记忆流/搜索/详情/导入/设置；登录后游客 Capture 合并；同步游标与冲突处理；`/workflows`、`/workflows/:id/designer`「规划中」页；GET /api/v3/capabilities workflow_* = false）
> 日期：2026-09-02
> 范围：**后端**（GET /api/v3/capabilities 公共端点 + /api/v3/workflows 501 守卫 + 多设备收敛集成测试）+ **前端**（capabilities feature + /workflows 规划中页/路由/设置入口 + 设置页合并状态区块 + Web 适配构建门禁 + widget_test 卫生）+ **文档**（API_V3_CONTRACT §13 / DEVELOPMENT_GOALS G8 / MEMORY.md Sprint 22 / 本报告）
> 前置：R9 完成并推送（376bacc：后端 337 无 env / 356 真实 PG 集成全绿、前端 184 全绿）

## 1. 阶段结论

R10 Web 回顾端、同步与工作流预留（G8）代码全部完成，单元 + 迁移契约 + 真实 PG 集成全绿：

- `go build ./...` / `go vet ./...` 通过；
- `go test ./...`（无 env，单元 + 契约，集成测试自动 skip）**349** 用例全绿（R9 337 → +12）；
- `WEAVEBRAIN_TEST_DATABASE_URL=postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test go test ./...`（真实 PostgreSQL）**369** 用例全绿、0 SKIP（R9 356 → +13）—— 含 B3 多设备收敛集成测试实际运行 + 迁移 000001—000014 与既有集成回归；
- `flutter test` **198** 用例全绿（R9 184 → +14）；`dart analyze lib test` 0 问题；`flutter build web --release` 成功（Web 构建门禁通过）。

G8 五项完成条件全部达成（详见 §6）：手机/Web 收敛（B3 幂等重放 + 冲突 409）、游客记录登录后无重复合并（F3 UI + 服务断言）、Web 回顾/导入/AI 设置构建 + widget 自动化、工作流守卫 501 无副作用、页面明确「规划中」。

**关键决策**（用户批准 3 项均按推荐 + 本轮 1 项实现修正）：
1. 同步深度 = **MVP server-authoritative**：不实现增量游标/推送同步引擎；收敛靠既有「capture_id 稳定幂等推送 + 服务端读取」；冲突 = 既有 409 IDEMPOTENCY_CONFLICT；增量同步契约文档预留（API_V3_CONTRACT §13.4）；
2. 游客合并 = **合并状态 UI + 无重复断言**：claimOwner（R3）不改；设置页只读「游客记录与同步」区块 + capture_sync_service_test 无重复断言；
3. Web 增量 = **Web 适配 + 端到端验证**：`flutter build web --release` 门禁 + widget 自动化；真实浏览器/Ollama E2E 按惯例记「待验证」限制；
4. 实现修正：合并状态区块必须读设备级未过滤的 `guestLocalCapturesProvider`——若读按会话过滤的 `localCapturesProvider`，登录态下游客记录（owner==null）会被过滤、条数恒为 0（见 §5.2）。

## 2. 交付能力

| 能力 | 说明 |
|---|---|
| GET /api/v3/capabilities（公共） | 能力广播：`mobile_capture=true / web_review=true / workflow_designer=false / workflow_execution=false / supported_capture_sources=[text,audio,import]`；与 /meta 同级注册（公共无鉴权）；sources 取 `entity.CaptureKind` 不硬编码 |
| /api/v3/workflows 501 守卫（受保护） | `Any("/workflows")` + `Any("/workflows/*any")` → 501 FEATURE_NOT_ENABLED + `details.{workflow_designer:false, workflow_execution:false}`；纯守卫无 service/DB 副作用；gin 通配符不吞兄弟路由，v3 NoRoute 不受影响；注册于 protected 前缀 → 游客不可访问 |
| 501 vs 409 语义 | 501 = 服务端不提供该能力（本守卫，SPEC §4.5 规范配对）；409 = R8/R9 每用户 AI 开关关闭 —— **既有 409 handler 一律不动**，避免破坏 R8/R9 客户端 |
| 多设备收敛（B3 集成） | 设备 A 创建 → Web 幂等重放（同 capture_id 同规范化内容 → 200 replayed，captures/memory_cards/capture_outbox 行数恰 1）→ GET 恰 1 条 → 冲突（同 id 不同内容）→ 409 行数仍 1 → workflows 带鉴权 501 且 capture_outbox 计数不变 |
| 前端 platform feature | `capabilities_api.dart`（模型 + CapabilitiesGateway/Api + provider）+ `capabilities_notifier.dart`（capabilitiesProvider，error → 全关闭回退，绝不暗示工作流可用）；workflow 页与设置入口依此呈现「规划中」 |
| workflows「规划中」页 | `workflow_list_screen.dart`（规划中 banner + 空状态 + 新建 → SnackBar「后续版本开放」**不跳假编辑器** + 示例卡跳 designer 占位）+ `workflow_designer_screen.dart`（占位页，无画布）；app.dart `/workflows`、`/workflows/:workflowId/designer` 路由（游客守卫自动拦 → /login）；设置页「工作流」入口 trailing 规划中 chip |
| 游客记录合并状态 | 设置页 Authenticated 区「游客记录与同步」只读区块：`guestLocalCapturesProvider`（设备级 owner==null 未过滤流）统计待认领条数 + 「登录后自动合并到账号，不产生重复」；不触发任何同步 |
| 无重复断言 | capture_sync_service_test 新增：游客（owner null）→ 登录 claimOwner 认领一次；二次 syncPending 无待处理不上传（本地只上传一次） |

## 3. 设计要点

- **MVP server-authoritative 收敛**（用户决策）：MVP 内不做游标/推送/冲突合并面板。收敛 = capture_id 幂等推送（R3 已建）+ 服务端读取。B3 用真实 PG 给出契约层证据：重放 200（行数 1）+ 冲突 409（行数仍 1）。增量游标/服务端推送只在契约 §13.4 预留。
- **501 = 服务端不提供**：`/workflows` 是保留命名空间而非用户可开关功能；guard 无依赖、无条件注册、任何方法 501。与 R8/R9「用户 AI 开关关闭 → 409 FEATURE_NOT_ENABLED」是同一 code 的两种 HTTP 语义，文档固化、客户端按场景区分（禁止 message 分支）。
- **无假工作流编辑器**（Blueprint §14.6 / WORKFLOW-001）：只做空状态 + 示例 + 「规划中」标记；`/workflows/:id/designer` 是占位页，明确「触发编排/条件配置/校验/发布不可执行」，避免被误认为可用。
- **游客合并仅 UI + 断言**（用户决策）：claimOwner 流程不改；本轮新增只读状态区块 + 服务层无重复断言，不引入任何新服务。
- **gin 通配符只守卫前缀**：`group.Any("/workflows")` + `group.Any("/workflows/*any")`；同前缀兄弟路由不受影响；v3 NoRoute 保持统一错误体。
- **capabilities 取值与实现一致**：`supported_capture_sources` 用 `entity.CaptureKind{Text,Audio,Import}` 生成（即 POST /captures 实际接受 kind），客户端不为显示而硬编码。

## 4. 测试情况

### 后端单元 + 契约（无 env，349 全绿；R9 337 → +12）

- **capabilities_handler_test.go（2）**：TestV3CapabilitiesEndpoint（200 + 5 键断言：mobile_capture/web_review true、workflow_* false、sources == text/audio/import）+ TestV3CapabilitiesIsPublic（无鉴权 200）；
- **workflow_guard_test.go（10 = 父 1 + 子 8 + 1）**：TestWorkflowGuardReturnsFeatureNotEnabled（create/list/get/draft/validate/publish/run/delete 全 → 501 + details 两 false）+ TestWorkflowGuardDoesNotShadowProtectedPrefixAuth（同前缀兄弟 `/captures` 仍 200）；
- 既有 api / migration / repository / service / stt / crypto 回归全部保持绿（R9 337 基线）。

### 后端集成（真实 PostgreSQL，369 全绿、0 SKIP；R9 356 → +13）

- **r10_convergence_integration_test.go（1）**：注册用户 → 设备 A `POST /captures {id:X, kind:text, text:…}` 201 → Web 同体重放 200 replayed（DB 断言 captures/memory_cards/capture_outbox 行数恰 1）→ GET capture 200 → 冲突（同 id 不同 text）409 IDEMPOTENCY_CONFLICT（行数仍 1）→ GET /api/v3/capabilities 公共 200（workflow_* false）→ POST /api/v3/workflows 带鉴权 501 + capture_outbox 计数不变（无副作用）；
- 既有集成回归（R3/R4/R7/R9 的 capture / cross-user / orphan-recovery / soak / import）在真实 PG 上全部保持绿。

### 前端（flutter test 198 全绿；R9 184 → +14）

- **platform（7）**：capabilities_api_test（模型蛇形映射 / disabled 回退 / fetch 路径）+ capabilities_notifier_test（data、error → 全关闭回退、workflowPlannedOnly）；
- **workflows（3）**：workflow_screens_test —— list 显示规划中 banner + 空状态 + 新建；点「新建」→ SnackBar（非编辑器）；designer 占位显示「规划中」+ workflowId；
- **widget_test（+3，全链路）**：游客访问 /workflows → 重定向 /login（登录页可见、WorkflowListScreen 不可见）；登录用户设置页显示「游客记录 1 条…不产生重复」（override guestLocalCapturesProvider）；无待合并显示「暂无待合并」+ 滚动点「工作流」入口 → /workflows 占位页；
- **capture_sync_service_test（+1）**：游客 owner-null → 登录 claimOwner 认领一次 + 二次 sync 不上传（本地只上传一次）。

### 工具链

- `go build ./...` / `go vet ./...`：通过；
- `dart analyze lib test`：0 issue；
- `flutter build web --release`：成功（dart2js；wasm dry-run 因 flutter_secure_storage_web 使用 dart:html 仅告警，非阻塞）。

## 5. 修复的问题

1. **B3 编译（captureID 作用域）**：计数闭包定义早于 `captureID := uuid.New()` → 把 captureID/canonicalBody 提到闭包之前；移除未用 bytes/httptest import；
2. **合并状态区块数据源（真实逻辑 bug）**：初版 watch 按会话过滤的 `localCapturesProvider`，登录态下游客记录（owner==null）经 `visibleCapturesForUser` 过滤不可见 → 条数恒 0。修正为新增设备级、未过滤的 `guestLocalCapturesProvider`（capture_providers.dart），设置页 `_MergeStatusCard` 改读该 provider（含注释说明），widget 测试 override 该 provider 断言真实条数；
3. **`hashCaptureRequest` 含 client_version**：B3 「Web 重放」必须复用同一规范化请求体（同 client_version）才能得到 200 replay；换不同 client_version 会 409 —— 集成测试按幂等语义实现；
4. **dart format 误伤存量文件**：对 capture_sync_service_test.dart 全文件 format 产生 ~145 行无关 churn → 还原到 HEAD 后仅补加 1 条测试（保持项目原格式基线，不全局 enforce format）。

## 6. 完成条件核对（G8）

- [x] 同一 Capture 在手机/Web 最终一致 —— B3 收敛集成测试（设备 A 创建 → Web 幂等重放 200、行数恰 1；冲突 409 行数仍 1；真实 PG）；
- [x] 游客记录登录后无重复合并 —— F3 设置页状态区块（guestLocalCapturesProvider 真实可见待认领数 + 「不产生重复」文案）+ capture_sync_service_test 无重复断言（claim 一次 / 二次 sync 不上传）；
- [x] Web 可完成回顾、搜索、导入和 AI 设置 —— F4 `flutter build web --release` 成功 + widget 自动化回归（记忆流/详情/导入/AI 设置/工作流入口全链路；浏览器/Ollama E2E 按惯例记「待验证」限制）；
- [x] 误调用工作流接口不创建任务或副作用 —— B2 守卫 501 + B3 断言 capture_outbox 计数不变（真实 PG）；
- [x] 页面明确展示「规划中」—— F2 workflows 列表/示例/新建 + designer 占位 + 设置入口 trailing 规划中 chip（Blueprint §14.6 / WORKFLOW-001，无假编辑器）。

## 7. 已知限制与后续

- **浏览器 / Ollama 真机 E2E 待验证**：secure storage web / IndexedDB 等运行时行为仅真实浏览器暴露；本轮自动化证据 = build web + widget 测试。
- **增量同步引擎 / 工作流为预留项**：增量游标、服务端推送、冲突合并面板、工作流保存/发布/运行均不在 MVP 实现（契约 §13.4 / §13.3 已预留命名空间）。
- **wasm dry-run 告警**：flutter_secure_storage_web 使用 dart:html，非 dart2js 阻塞；如需 wasm 后续引入兼容实现。
- **`flutter analyze` 本机崩溃**：沿用 `dart analyze lib test` 替代（R4—R8 已记录）。

## 8. 下一步（G9 / R11）

移动快捷入口与一种回响：App 内全局捕捉按钮、Android/iOS App Shortcut、桌面小组件（选一端）、极简录音页 + 声音/触觉确认、一种回响（每日/隔日或每周主题）+ 完成/稍后/无关反馈、频率与静默时段、通知深链。

## 9. 证据

- `go build ./...` / `go vet ./...`：通过
- `go test ./...`（无 env）：**349** 用例全绿（R9 337 → +12；集成测试自动 skip）
- `WEAVEBRAIN_TEST_DATABASE_URL=postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test go test ./...`：**369** 用例全绿、0 SKIP（R9 356 → +13；迁移 000001—000014 + B3 收敛 + 既有集成回归全部实际运行）
- `flutter test`：**198** 用例全绿（R9 184 → +14）；`dart analyze lib test`：0 issue
- `flutter build web --release`：成功
- docs/API_V3_CONTRACT.md §13（平台能力与工作流预留：capabilities JSON / 501 vs 409 语义区分 / workflows 保留命名空间 / server-authoritative 同步模型 / 自动化证据）
- docs/DEVELOPMENT_GOALS.md G8（5 步 + 5 完成条件全部勾选；顶层表 G8 → R10 完成）
- MEMORY.md Sprint 22
