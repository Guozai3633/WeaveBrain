# 织脑（WeaveBrain）R2 本地 Capture 契约

> 状态：R2 冻结  
> 日期：2026-08-16  
> 适用端：Android、iOS、Web  
> 唯一目标：网络、进程或服务端失败时，用户已提交的文字不丢失。

## 1. 本地模型

LocalCapture 是服务端 Capture 的本地投影，使用客户端生成的 UUID 作为唯一主键。

| 字段 | 类型 | 规则 |
|---|---|---|
| id | UUID string | 本地生成，同时作为 capture_id 和 Idempotency-Key |
| kind | string | R2 固定 text |
| text | string | 精确保留用户原文；trim 后不能为空 |
| captured_at | UTC timestamp | 用户点击保存时产生 |
| captured_at_precision | string | R2 固定 exact |
| timezone | string? | 来自设备；未知时为空，不猜测 |
| source | string | mobile_android/mobile_ios/web |
| privacy_mode | string | 默认 cloud_allowed |
| client_version | int | R2 固定 1 |
| sync_state | enum | 只表示本地到服务端的同步状态 |
| server_processing_status | enum | 独立字段；R2 不驱动 AI |
| server_version | int? | 同步成功后保存 |
| retry_count | int | 每次可重试失败递增 |
| next_retry_at | timestamp? | 指数退避时间 |
| last_error_code | string? | 稳定错误分类，不保存 Token/堆栈 |
| created_at/updated_at | timestamp | 本地审计时间 |
| trashed_at | timestamp? | 本地软删除 |

## 2. 状态机

本地同步状态：

```
saved_local → pending_sync → syncing → synced
                         ↘ retryable_error → syncing
                         ↘ conflict
```

规则：

- saved_local：本地事务已经成功，UI 此时即可显示“已安全保存”；
- pending_sync：已经进入持久同步队列；
- syncing：请求正在执行；应用重启时必须恢复为 pending_sync；
- synced：服务端确认 200 或 201；
- retryable_error：网络、超时、5xx 或暂时无法认证；原文仍保留；
- conflict：服务端返回 IDEMPOTENCY_CONFLICT，需要人工处理，不自动改 UUID；
- 服务端 MemoryCard.processing_status 存入 server_processing_status，不得覆盖 sync_state。

## 3. LocalCaptureStore

冻结接口：

- saveDraft(capture)
- saveAudioAsset(asset)：仅保留接口；R2 不实现录音流程
- markPendingSync(id)
- listPending(limit, includeDeferred)
- markSyncing(id)
- applyServerRevision(id, serverVersion, processingStatus)
- markRetryableError(id, errorCode, nextRetryAt)
- markConflict(id, errorCode)
- getById(id)
- listAll()
- watchAll()
- trash(id)

持久化要求：

- 移动端：Sembast 文件数据库，位于应用私有 Documents 目录；
- Web：Sembast Web，底层 IndexedDB；
- captures Store 以 UUID 为键；
- saveDraft、状态变更和重试计数使用数据库事务；
- 应用启动时把遗留 syncing 恢复为 pending_sync；
- 不允许用纯内存 Provider 作为正式存储。

## 4. 保存与同步顺序

1. 用户点击保存；
2. 生成 UUID 和 Capture DTO；
3. saveDraft 完成；
4. UI 清空输入并展示“已安全保存”；
5. markPendingSync；
6. 已登录时后台 POST /api/v3/captures；
7. 201/200：标记 synced；
8. 409 IDEMPOTENCY_CONFLICT：标记 conflict；
9. 网络、超时、401 或 5xx：标记 retryable_error；
10. 启动、登录成功、网络恢复、应用回到前台或用户手动重试时再次排空队列。

联网类型只用于触发尝试，不能替代真实 HTTP 成功判断。

## 5. 游客模式

- 未登录用户可以进入文字捕捉页；
- Capture 始终先保存在本机；
- 未登录时不发送网络请求，状态保持 pending_sync；
- 登录成功后使用当前账户排空待同步队列；
- R2 不实现多账户本地数据迁移与账户解绑清理；该风险在 R3 验收前必须再次评审。

## 6. V3 请求映射

POST /api/v3/captures：

- Authorization: Bearer token；
- Idempotency-Key: LocalCapture.id；
- capture_id: LocalCapture.id；
- kind: text；
- text: LocalCapture.text；
- captured_at: LocalCapture.capturedAt；
- captured_at_precision: exact；
- timezone/source/privacy_mode/client_version 按本地字段传递。

手机与 Web 必须复用同一个 LocalCapture.toCreateRequest()，禁止分别手写 JSON。

## 7. R2 非目标

- 不录音、不上传音频、不接 STT；
- 不调用 AI、不实现 AI 开关或补全；
- 不实现批量导入；
- 不实现系统快捷入口；
- 不实现回响；
- 不做完整 MemoryCard 详情与视觉系统；
- 不删除旧 Idea 前端。

## 8. 自动化验收

必须覆盖：

- 无网络连续保存 20 条，本地条数为 20；
- 关闭并重新打开数据库后 20 条仍存在；
- 同一批记录多次重试使用原 UUID，服务端无重复；
- HTTP 5xx 后仍能读取原文，状态为 retryable_error；
- 重启遗留 syncing 自动恢复；
- 409 进入 conflict，不静默换 ID；
- 未登录不发网络请求；
- Widget 保存后出现“已安全保存”；
- Android/移动实现与 Web 实现通过相同 Store 合约测试。

## 9. R3 安全修订

R3 验收发现并修复多账户共用本地数据库时的归属风险，本节优先于前述 R2 冻结状态机：

- LocalCapture 新增 owner_user_id；游客记录为空，已登录创建时立即写入当前 user.id；
- 游客记录首次同步前由当前账户事务性认领；已归属账号 A 的记录不能被账号 B 重新认领；
- 未登录只展示 owner_user_id 为空的记录；登录后只展示当前账户记录，其他账户本地数据不可见；
- 同步队列只发送未归属游客记录或当前账户记录，账号 B 不会发送账号 A 的待同步 Capture；
- 新增 rejected 本地终态；400—499 中除 401、408、409、425、429 外的永久错误移出重试队列；
- 客户端与服务端统一限制文字最多 100,000 个 Unicode code points；
- 创建请求体最大 1 MiB，避免无限请求体占用内存；
- 409 仍进入 conflict；401、408、425、429、网络错误和 5xx 仍保留原文并重试。
