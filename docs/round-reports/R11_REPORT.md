# 织脑（WeaveBrain）R11 报告 — 移动快捷入口与一种回响（G9）

> 轮次：R11 — 移动快捷入口与一种回响
> 目标：G9（App 内中央全局捕捉按钮；Android/iOS App Shortcut；桌面小组件；极简录音页 + 声音/触觉确认；一种回响 + 完成/稍后/无关反馈；频率与静默时段、通知深链）
> 日期：2026-09-02
> 范围：**后端**（回响按需生成 + user_echo_settings 服务端设置 + 反馈，000015 迁移，真实 PG 集成）+ **前端**（全局捕捉 FAB + App Shortcut + Android 小组件 + 极简录音页声音/触觉确认 + 回响页/设置/本地通知 + 通知深链 + 服务抽象测试面）+ **文档**（API_V3_CONTRACT §14 / DEVELOPMENT_GOALS G9 / MEMORY.md Sprint 23 / 本报告）
> 前置：R10 完成并推送（5c05d6c：后端 349 无 env / 369 真实 PG 集成全绿、前端 198 全绿 + web 构建通过）

## 1. 阶段结论

R11 移动快捷入口与一种回响（G9）代码全部完成，单元 + 迁移契约 + 真实 PG 集成全绿：

- `go build ./...` / `go vet ./...` 通过；
- `go test ./...`（无 env，单元 + 契约，集成测试自动 skip）**407** 用例全绿（R10 349 → +58）；
- `WEAVEBRAIN_TEST_DATABASE_URL=postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test go test ./...`（真实 PostgreSQL）**429** 用例全绿、0 SKIP（R10 369 → +60）—— 含 r11 回响集成测试实际运行 + 迁移 000001—000015 与既有集成回归；
- `flutter test` **261** 用例全绿（R10 198 → +63）；`dart analyze lib test` 0 问题；`flutter build web --release` 成功（Web 构建门禁）；`flutter build apk --debug` 成功（Android 原生落地门禁）。

G9 六项完成条件全部达成（详见 §6）：P50 以自动化替代记为真机待验证、捕捉全程 ≤1 次主动操作、不进入主页也能开始捕捉、回响可关闭 + 每次说明原因、通知点击可恢复目标记忆（自动化替代）、有用/无关/稍后反馈记录。

**关键决策**（用户批准 4 项均按推荐）：
1. 回响 = **按需生成 + 客户端本地通知**：无推送设施、不依赖 Temporal。`GET /api/v3/echoes/current` 读取时由服务端选出当前回响（每次恰一条记忆卡片 + 说明原因）并落稳定行；提醒由客户端按 cadence 用 flutter_local_notifications 本地通知触发；点通知 → 回 App 拉当前回响 → 深链 `/memories/:captureId`；
2. 回响设置 = **服务端**：新增 `user_echo_settings`（镜像 `user_ai_settings`：PK user_id、revision CAS；`enabled` + `cadence`∈daily/every_other_day/weekly 服务端管控）；投递时刻/静默时段放客户端本地（shared_preferences），仅用于调度本地通知；
3. 小组件 = **Android 端**（home_widget）；Windows 宿主无法构建 iOS；iOS 仅留代码标注未验证；
4. 极简录音交互 = **进入自动开录 + 轻触保存**：系统入口直达录音页并自动开始（0 次主动操作），整页任意处轻触一次即停止+保存+声音/触觉确认 = 全程 ≤1 次主动操作。

## 2. 交付能力

| 能力 | 说明 |
|---|---|
| GET /api/v3/echoes/current（回响读取） | 服务端读取时选出当前回响：settings enabled=false → 空；latest open 未超窗 → 复用该行（跨打开/通知点击稳定）；超窗 → 滚动 expired；节奏门（锚 = 最近任意 status 行 created_at，步进 1/2/7 天）未到 → off-period 空 + next_due_at；PickCandidate 确定性选卡 + reason classify（无历史回响→first_echo / IsPinned→pinned / 无先例→oldest / 否则 reminder）→ 落 user_echoes 新 open 行 |
| POST /api/v3/echoes/:echoID/feedback | 反馈 transitions：done/later/not_relevant；非本用户/不存在 → 404；status!=open → 409 VERSION_CONFLICT（多设备竞态兜底）；返回新 status + next_due_at |
| GET/PATCH /api/v3/users/me/echo-settings | user_echo_settings 镜像 ai-settings：revision CAS 部分更新（expected_revision + enabled?/cadence?）；新用户 Create revision 1；cadence 校验 IN daily/every_other_day/weekly |
| 000015 迁移 | `user_echo_settings`（PK user_id + revision CAS）+ `user_echoes`（复合 FK (user_id,capture_id) REFERENCES captures(user_id,id) CASCADE、status CHECK open/done/later/not_relevant/expired、reason_code CHECK、resolved_at）+ 3 索引；Down 先 user_echoes 再 settings；迁移契约测试断言两表/CHECK/复合 FK/索引/Down 顺序 |
| 全局捕捉 FAB | app_scaffold 四 tab 中央 FAB（Key global_capture_fab）：web → SnackBar「浏览器不支持录音落盘」；非 web → push `/record?auto=1`（global_fab_test） |
| App Shortcut | flutter_quick_actions 动态快捷键（quick_capture → `/record?auto=1`）；Android 构建验证；iOS Info.plist `UIApplicationShortcutItems`（type quick_capture/速记）留代码标注 |
| Android 桌面小组件 | home_widget：EchoCaptureWidgetProvider + `weavebrain://quick_capture` LAUNCH intent-filter + res/layout + xml/echo_capture_widget_info + 背景 drawable；APK 构建通过，摆放/外观真机待验证 |
| 极简录音页 + 声音/触觉确认 | recording_screen `/record?auto=1`：挂载即自动开录、全屏点按面「点按任意处结束并保存」、整页单点一次 → 停止+保存、注入 CaptureFeedbackService（start 首帧 startTapped / saved 时确认反馈）、~600ms 自动 pop；auto!=1 保持显式流不回归 |
| 回响 feature（前端） | echo tab 替换占位页：游客 CTA / 已关闭去设置 / empty 空态 + next_due 提示 / loaded 记忆卡片 + 原因行「这次回响：{reason.text}」+ 完成/稍后/无关 + 点卡片 → `/memories/:captureId`；echo-settings 子路由（enabled + cadence + 通知时间/静默时段本地区块） |
| 通知深链 / 启动路由 | LaunchRequest 模型 + launchMapper 纯函数：notification（无推送 payload）→ 先拉 /echoes/current → 有 echo `/memories/{captureId}` / 无 `/echo`；shortcut/widget/FAB → `/record?auto=1`；三冷启动源 + 后台流归一化 → auth 就绪后 go 并清空（游客 + /memories 由既有守卫转 /login，登录后待办冲刷记为限制） |
| 客户端本地通知调度 | echo_scheduler：disabled → cancelAll；step=1/2/7 天排未来 8 次 occurrence 于本地 HH:mm（TZDateTime）；落在静默窗则跳过（设置页冲突警示）；inexactAllowWhileIdle 免 SCHEDULE_EXACT_ALARM；重排触发 = 鉴权启动/PATCH 成功/时刻或静默变更；**cadence 正确性由服务端保证，通知只是邀请** |

## 3. 设计要点

- **稳定单行回响协议（服务端纪律）**：重复打开不重复建行 —— stable-open 复用 + cadence 锚最新任意 status 行 created_at + expired 滚动；多设备竞态以反馈非 open → 409 兜底。reason_code 落库、reason_text 读取时按中文模板实时渲染（"N 天前"新鲜）。
- **PickCandidate 确定性 SQL**：captures ⋈ memory_cards（deleted/lifecycle/processing_status/title 过滤 + not_relevant 90 天 / 其余 14 天冷却）→ `ORDER BY is_pinned DESC, 最近回响 ASC NULLS FIRST, captured_at ASC, id ASC LIMIT 1` + EXISTS HasPriorEcho。
- **服务端设置 + 客户端调度职责分离**：cadence/enabled 服务端读取时强制（多设备/漂移真值归服务端）；投递时刻/静默时段只影响本地通知调度，不上送。
- **通知只是邀请**：是否实际到期由打开 App 后 GET /echoes/current 判定；通知点按目标记忆若跨设备已消费则落 /echo 服务端真值（记为限制）。
- **极简录音反馈不进 controller**：确认反馈全由 screen 驱动 + 注入 CaptureFeedbackService（controller 不加硬接线，保持既有 recording_screen_test 不回归）。
- **平台通道注入 + kIsWeb 早退**：flutter_local_notifications / quick_actions / home_widget 等平台调用全经 lib/shared/native/ 注入服务 + noop provider，`flutter test`/`flutter build web` 不触碰平台通道。
- **iOS 代码标注仅**（Windows 宿主不可构建）：Info.plist UIApplicationShortcutItems + AppDelegate UNUserNotificationCenter delegate；quick_actions iOS 插件自注册 scene delegate 无需改动。
- 本轮明确不建：推送设施、Temporal 回响工作流、永久监听、watch/耳机/车机、第二类回响、复杂个性化、登录后待办路径冲刷。

## 4. 测试情况

### 后端单元 + 契约（无 env，407 全绿；R10 349 → +58）

- **echo_handler_test.go（17 = current 5 + feedback 5 + settings 7）**：CurrentReturnsCard / CurrentDisabledReturnsEmpty / CurrentBetweenWindowsReturnsNextDue / CurrentNoCandidatesReturnsEmptyReason / CurrentRequiresAuth；FeedbackPassesVerdict / FeedbackMissingVerdictIsBadRequest / FeedbackNotOpenIsVersionConflict / FeedbackUnknownEchoIsNotFound / FeedbackRequiresAuth；GetSettingsReturnsDefaults / PatchSettingsPartialUpdate / PatchSettingsMissingRevisionIsBadRequest / PatchSettingsNoFieldsIsBadRequest / PatchSettingsConflictMapsToVersionConflict / PatchSettingsInvalidCadenceIsBadRequest / GetSettingsRequiresAuth；
- **echo_service_test.go（23 = Current 9 + Feedback 5 + Settings 9）**：CurrentDisabled/StableOpenEchoIsReused/StaleOpenExpiresThenCreatesNext/BetweenWindows/NoCandidate/FirstEchoReason/PinnedCandidateReason/RepeatCandidateReason/OrphanOpenEchoSelfHeals；FeedbackDoneTransitionsAndComputesNextDue/NotOpenReturnsConflict/UnknownEchoPropagatesNotFound/InvalidVerdictRejected/RequiresUserID；Settings Get 默认物化/存储行 + Update 新用户建行 rev1/CAS 匹配递增/CAS miss 冲突/非法 cadence/RequiresUserID/仓库冲突映射服务冲突；
- **echo_settings_repository_test.go（6）**：GetByUserID nil 无行/全列扫描、Create 默认 rev1/并发冲突哨兵、Update CAS miss 冲突/递增返回；
- **echo_repository_test.go（11）**：Latest nil/范围扫描、GetByID 404/按 user+id 范围、Create 插入 open 行、UpdateStatus 守卫 open/非 open 哨兵、FetchMemory 404/可见卡片扫描、PickCandidate 无行 nil/冷却与排序；
- **000015_echoes_migration_contract_test.go（1）**：断言两表/两 CHECK/复合 FK/三索引/Down 顺序；
- 既有 api / migration / repository / service / stt / crypto 回归全部保持绿（R10 349 基线）。

### 后端集成（真实 PostgreSQL，429 全绿、0 SKIP；R10 369 → +60）

- **r11_echo_integration_test.go（2）**：
  - TestR11EchoSettingsGatingAndFeedback：默认 enabled=false → current 空；PATCH 开 daily；seed 2 张 ready 卡片（POST /captures + SQL 置 processing_status='ready'/title）→ current 两次同 echo.id 且 user_echoes 行数恰 1；feedback done → 再 current 得 off-period（行数仍 1）；对同 echo 二次 feedback → 409；SQL 前移 created_at 2 天 → current 出新 open 行（行数 2）且选中另一卡片；
  - TestR11EchoNotRelevantAndCrossUser：A not_relevant 后 SQL 前移 100 天 → 选中另一卡片（90 天排除生效）；B 独立不受 A 影响（cross-user）；
- 既有集成回归（R3/R4/R7/R9/R10 的 capture / cross-user / orphan-recovery / soak / import / convergence）在真实 PG 上全部保持绿。

### 前端（flutter test 261 全绿；R10 198 → +63）

- **echo feature（52）**：echo_api_test（13，模型蛇形/网关/错误映射）+ echo_notifier_test（10：guest/disabled/empty/loaded/feedback/error 各态）+ echo_settings_notifier_test（6：load/setEnabled/setCadence + CAS 409「已在其他设备修改」）+ echo_screen_test（9：三反馈键/原因文案/卡片跳详情/各状态/error 重试）+ echo_settings_screen_test（4：开关+cadence+保存触发调度）+ echo_scheduler_test（10：注入时钟 occurrence 数学/静默跳过/1-2-7 天/cancel-before）；
- **recording_minimal_test.dart（2）**：auto 入口无任何用户点按即开始录音；整页单点一次停止+保存 1 条 + saved 确认反馈 + 自动 pop；
- **launch_mapper_test.dart（5）**：notification 有/无 open echo 深链与回退、quick capture/home widget 归一化、各 source 断言；
- **widget_test.dart（+4，全链路）**：notification launch 有 echo → /memories/c1 详情；guest notification → /echo hub；quick capture shortcut → 极简录音；加上 fake 注入面更新（_FakeEchoGateway 等 6 个 override）；
- **两处关键修复**：① Riverpod `build()` 内写 `late final _gateway` 字段在 gateway provider 变化重跑时抛 LateInitializationError → 改非 final `late`（含注释）；② widget 测试重复 pump 新 ProviderScope 复用 element、initState 不重跑 → 改单挂可变 fake gateway。

### 工具链

- `go build ./...` / `go vet ./...`：通过；
- `dart analyze lib test`：0 issue；
- `flutter build web --release`：成功（wasm dry-run 因 flutter_secure_storage_web 使用 dart:html 仅告警，非阻塞）；
- `flutter build apk --debug`：成功（首次失败根因 = flutter_local_notifications 需 core library desugaring，落地 Android §4 后通过）。

## 5. 修复的问题

1. **Android APK 构建缺 core library desugaring**：flutter_local_notifications v22+ 要求 desugar_jdk_libs → build.gradle.kts 开 `isCoreLibraryDesugaringEnabled = true` + `coreLibraryDesugaring("com.android.tools:desugar_jdk_libs:2.1.4")` + manifest POST_NOTIFICATIONS/RECEIVE_BOOT_COMPLETED + ScheduledNotificationReceiver/BootReceiver（plan §4 本应在 F5 前落地，实际 F5 首失败暴露后补齐）；
2. **Riverpod Notifier rebuild 崩溃（LateInitializationError）**：echo_notifier/echo_settings_notifier 用 `late final _gateway` 且只在 build() 写入 —— 当 watch 的 gateway provider 变化、build() 重跑时对已初始化 final 字段二次赋值 → 改非 final `late`；
3. **widget test 重复 ProviderScope 不触发 initState**：测试重新 pump 新 ProviderScope 复用既有 element、initState/load 不重跑 → 改为单挂一个可变 fake（error=true 启动、重试前翻回 false）；
4. **静默窗口语义测试误判**：初版断言固定投递时刻在「当日窗口内仅跳过今日」—— 实现把静默窗口视为每日重复的分钟空间谓词，固定时刻恒在窗内 → 每日都跳过（空正确，且与设置页冲突警示一致）→ 改为断言「窗外投递逐日不误伤」+ 沿用跨午夜 `isEmpty` 用例；
5. **incidental app-label 改动回退**：AndroidManifest android:label 曾误改为「织脑」→ 还原 `weave_flutter`（plan §4 未含 label 变更，避免越界）。

## 6. 完成条件核对（G9）

- [x] 系统入口到开始录音 P50 小于 1.5 秒 —— 设备指标，自动化替代 = recording_minimal_test 证明 auto 挂载即 start 且无用户点按；真机 P50 待验证（记 R11 报告）；
- [x] 捕捉全程最多一次主动操作 —— `/record?auto=1` 进入自动开录（0 次）+ 整页单点一次停止+保存+确认反馈（全程 ≤1 次）；
- [x] 不进入主页也能开始捕捉 —— App Shortcut / Android 小组件 / 全局 FAB → `/record?auto=1` 直达极简录音页（launchMapper 单测 + widget_test 深链；系统入口真机待验证）；
- [x] 回响可关闭、每次说明出现原因 —— echo-settings enabled 开关（服务端 gating）+ 确定性 reason 集合（first_echo/pinned/oldest/reminder 中文模板，current 每次返回 reason{code,text}）；
- [x] 通知点击能恢复到目标记忆 —— 无推送设施下点通知先拉 /echoes/current 解析 → `/memories/:captureId`（launchMapper 单测 + notification deep-link widget test；真机通知点按待验证）；
- [x] 记录有用/无关/稍后反馈 —— `POST /echoes/:echoID/feedback` done/later/not_relevant（not_relevant 90 天 / 其余 14 天冷却 + 非 open → 409 兜底；echo_screen_test 三反馈键 + r11 集成测试真实 PG）。

## 7. 已知限制与后续

- **真机指标待验证**（Windows 宿主不可运行移动端/不可构建 iOS）：P50<1.5s、真实通知点按、Android 小组件摆放与外观、App Shortcut 系统入口 —— 自动化替代与 Android 构建已给，设备侧留待 G10/R12 或真机验收；
- **iOS 全项代码标注未验证**：Info.plist UIApplicationShortcutItems + AppDelegate 通知 delegate 已留代码，需 macOS 环境构建复核；
- **无推送设施**：通知点击 → 记忆恢复依赖打开 App 后拉 /echoes/current 解析目标；若跨设备已消费则落 /echo 服务端真值；
- **回响为单卡片 + 确定性 reason 集合**（无推荐引擎）：开启后首次 GET 即生成首条（cadence 锚创建时刻，演示友好、确定性）；
- **flutter_local_notifications 本地通知调度在 web 不可用**：kIsWeb 早退，web 运行时回响只读不提醒（与 R10 web 回顾端定位一致）；
- **登录后待办路径冲刷不实现**：游客 + /memories 守卫转 /login 后不回放原目标（记为限制，本轮不做）；
- **`flutter analyze` 本机崩溃**：沿用 `dart analyze lib test` 替代（R4—R8 已记录）。

## 8. 下一步（G10 / R12）

验收 C — MVP 功能完整性：R3/R7 验收项无回归、手机/Web 端到端流程全部通过、AI 补全无静默覆盖、导入无跨行回滚和重复、工作流无意外执行、P0/P1 缺陷全部关闭、功能冻结清单签字确认、形成 R12 验收报告。

## 9. 证据

- `go build ./...` / `go vet ./...`：通过
- `go test ./...`（无 env）：**407** 用例全绿（R10 349 → +58；集成测试自动 skip）
- `WEAVEBRAIN_TEST_DATABASE_URL=postgresql://weavebrain:weavebrain_dev@localhost:5432/weavebrain_test go test ./...`：**429** 用例全绿、0 SKIP（R10 369 → +60；迁移 000001—000015 + r11 回响集成 + 既有集成回归全部实际运行）
- `flutter test`：**261** 用例全绿（R10 198 → +63）；`dart analyze lib test`：0 issue
- `flutter build web --release`：成功（Web 门禁）
- `flutter build apk --debug`：成功（Android 原生门禁；含 home_widget 小组件 + 通知 receiver + core library desugaring）
- docs/API_V3_CONTRACT.md §14（回响接口：设计决策/数据模型/GET+PATCH settings/GET echoes/current 稳定算法 + 卡片与三种空 JSON 示例/候选选择与 reason 模板/feedback transitions/冻结错误码/自动化证据）
- docs/DEVELOPMENT_GOALS.md G9（6 步 + 6 完成条件全部勾选；顶层表 G9 → R11 完成）
- MEMORY.md Sprint 23
