# 织脑（WeaveBrain）R2 验收报告

> 轮次：R2 — 本地优先文字捕捉  
> 开始日期：2026-08-16  
> 结束日期：2026-08-18  
> 状态：已通过  
> 下一候选轮次：R3 — 验收 A：可靠文字捕捉（尚未开始）

## 1. 计划目标

让手机端和 Web 端在离线、应用中断和服务端暂时不可用时，仍能先在本地可靠保存文字 Capture，并在条件恢复后使用同一 UUID 幂等同步。

## 2. 实际完成

- 冻结并实现 LocalCapture、LocalSyncState 与 LocalCaptureStore 契约；
- 移动端使用应用私有 Documents 目录内的 Sembast 文件数据库；
- Web 使用 Sembast Web，底层为 IndexedDB；
- Capture、持久同步队列和资产预留 Store 采用事务更新；
- 实现 saved_local → pending_sync → syncing → synced/retryable_error/conflict 状态机；
- 应用启动时恢复遗留 saved_local/syncing 记录；
- 本地生成 UUID，并同时作为 capture_id 与 Idempotency-Key；
- 实现游客本地记录；未登录时不发送网络请求；
- 在启动、登录变化、网络恢复、应用回到前台和手动操作时触发同步；
- 可重试失败按持久化 nextRetryAt 自动再次尝试；
- 实现极简文字捕捉页、本地列表、同步状态和手动重试入口；
- 本地事务完成后立即清空输入并显示“已安全保存”。

## 3. 验收条件

| 条件 | 结果 | 可复查证据 |
|---|---|---|
| 飞行模式保存 20 条文字 | 通过 | 文件数据库测试在不调用远端的情况下连续写入 20 条 |
| 杀进程并重启后 20 条全部存在 | 通过 | 关闭并重新打开同一 Sembast 文件后仍为 20 条 |
| 恢复网络后最终同步且无重复 | 通过 | 重试测试两次请求复用同一 UUID，最终状态为 synced；R1 服务端幂等契约保证单条落库 |
| 服务端 5xx 不导致本地草稿消失 | 通过 | 500/503 后原文不变，状态进入 retryable_error |
| 手机与 Web 使用相同 Capture 契约 | 通过 | IO/Web 仅替换数据库适配，复用 LocalCapture.toCreateRequest 与 CaptureSyncService；双端均编译成功 |
| UI 本地事务完成后显示“已安全保存” | 通过 | CaptureScreen Widget 测试 |
| Local 单元和 Widget 测试通过 | 通过 | flutter analyze 零问题，flutter test 12/12 通过 |

## 4. 修改范围

客户端主要新增或调整：

- weave_flutter/lib/features/capture/domain：本地模型、Store 接口、Provider 与保存控制器；
- weave_flutter/lib/features/capture/data：Sembast 实现、平台数据库、V3 API 与同步服务；
- weave_flutter/lib/features/capture/ui/capture_screen.dart：极简文字捕捉与本地列表；
- weave_flutter/lib/shared/api：IO/Web 条件实现与自定义请求头；
- weave_flutter/lib/app.dart、lib/main.dart：游客入口、生命周期同步和默认捕捉页；
- weave_flutter/test/features/capture：持久化、同步、错误和 Widget 测试；
- weave_flutter/pubspec.yaml：Sembast、Sembast Web、Connectivity Plus、UUID 与 Path 依赖；
- weave_flutter/android/gradle.properties：关闭跨盘符不稳定的 Kotlin 增量缓存。

数据库迁移：无。  
服务端 API 变更：无；复用 R1 的 POST /api/v3/captures。

## 5. 自动化与构建证据

- scripts/flutter_check.ps1 -Task all：通过；
- flutter analyze：No issues found；
- flutter test：12 个测试全部通过；
- flutter build web --debug：通过，产物为 weave_flutter/build/web；
- flutter build apk --debug：通过，产物为 weave_flutter/build/app/outputs/flutter-apk/app-debug.apk；
- go test ./...：通过；
- go vet ./...：通过；
- go build ./cmd/weavebrain：通过。

Android 首次构建因 Windows 中文用户名进入 JNI/CMake 路径而失败；切换到纯英文 PUB_CACHE 后构建通过，确认属于本机工具链路径兼容问题。Web JavaScript 构建通过，但现有 flutter_secure_storage_web 仍有 WebAssembly 兼容警告，本轮不启用 Wasm。

## 6. 安全与失败路径

- Token 不写入 LocalCapture 或错误字段；
- 服务端仍以 JWT user_id 做数据隔离；
- 409 不自动换 UUID，进入 conflict，避免静默覆盖；
- 网络、格式、401 和 5xx 均保留本地原文；
- 本轮没有录音、AI、导入、快捷入口、回响或完整卡片视觉，未提前扩展范围。

## 7. 已知问题与 R3 验收重点

- 尚未在真实 Android/iOS 设备上执行杀进程、飞行模式和恢复网络的手工矩阵；R3 必须真机复验；
- 游客记录在登录后归入当前账户。设备切换多个账户时的本地归属与清理策略尚未实现，R3 必须做跨账户安全评审；
- 当前 Web API Host 是开发期配置，发布环境配置留待后续发布加固；
- 本轮只保证 JavaScript Web 构建，不保证 Wasm；
- 性能 P95、超长内容和真实服务端断网恢复属于 R3 独立验收项。

## 8. 偏离计划

没有功能范围偏离。额外加入的自动定时重试属于“待同步队列和重试”的既定范围；纯英文 Pub 缓存仅用于解决本机构建兼容性。

## 9. 结论

R2 的开发完成条件均有自动化或跨端编译证据，状态标记为“已通过”。允许进入 R3，但 R3 是独立验收轮，只能执行测试、真机验证和缺陷修复，不得开始语音或 AI 开发。
