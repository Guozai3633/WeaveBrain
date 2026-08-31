# 织脑（WeaveBrain）R4 前端阶段报告

> 轮次：R4 — 可靠语音捕捉与 STT
> 阶段：前端（数据层 / 上传服务 / 录音 UI / 转写修正 / 测试）
> 日期：2026-08-30
> 后端阶段（迁移 / 服务 / 路由 / 测试）已完成，见 R04_BACKEND_REPORT.md

## 1. 阶段结论

R4 前端已完成并全绿：`dart analyze lib test` 通过（0 error / 0 warning），`flutter test` 45 个用例全部通过。音频流程为「录音即落盘 → 本地 Capture UUID 先行保存 → 分块 SHA-256 上传（重启可续传）→ complete → 全文件 STT → TranscriptRevision → 用户修正」。Web 端明确提示能力差异并回退文字速记。真机 / 端到端联调留待设备验证。

## 2. 交付能力

| 能力 | 说明 |
|---|---|
| 录音落盘 | record 7.1.0 `start(path:)` 写入应用私有目录 `<captureId>.wav`（44100 Hz mono WAV） |
| 本地先行保存 | 录音开始即用 capture UUID 建 kind=audio 空文本本地 Capture；停止时保存 size + 全文件 SHA-256 元数据 |
| 分块续传 | AudioUploadService：create capture → initiate → 按缺块上传（跳过已传，`updateAudioUploadedChunks` 持久化）→ complete → transcribe |
| 失败可重试 | 上传失败 markRetryable（AUDIO_UPLOAD_FAILED），服务端 Idempotency-Key 幂等重放安全，重启续传 |
| 转写修正 | TranscriptEditScreen：加载本地+服务端最新转写；guest 本地修正（source=user, v1）/ 登录 PATCH + 本地镜像 |
| 能力降级 | Web 端 SnackBar 提示 + 录音页不支持提示；权限拒绝/占用/保存失败/取消均有状态机降级 |

## 3. 设计要点

- **音频 capture 复用 kind=audio LocalCapture 持久化**：分块进度 + 最新转写都落在本地 capture 记录里，重启后可恢复，避免独立资产存储带来的同步复杂度。
- **全文件落盘后一次上传**：R4 明确不用流式 WebSocket 字幕替代完整音频资产，尾句不会因停止流而丢失。
- **三层可测抽象**：`AudioRecorder` / `AudioFileStorage` / `AudioUploader` / `AudioRemoteGateway` 均为可注入接口，控制器 / 服务 / UI 三层分别用 fake 覆盖，Riverpod `overrideWith` 注入。
- **条件导出隔离 dart:io**：`audio_chunk_source_io/web`、`audio_file_storage_io/web`、`api_client_io/web`，Web 平台抛 `UnsupportedError`。

## 4. 测试情况

共 45 个通过测试（边界 + 对抗 + widget）：

- **audio_upload_service_test.dart（9）**：单块全链路 + 转写落库；续传跳过已传块并持久化 [0,1]；尾块截断（42 字节）；stt_enabled=false 不调转写返回 null；缺元数据 → AUDIO_METADATA_MISSING；文件不可读 → AUDIO_FILE_UNREADABLE（发起过、未完成/转写）；缺本地路径；非音频 no-op；assetId 回退 capture.id。
- **capture_sync_service_test.dart（11，含音频集成）**：上传成功 → synced 且 uploader 被调用；上传失败 → AUDIO_UPLOAD_FAILED retryable、serverVersion null、仍 pending；失败后重试恢复 → synced；文本 capture 忽略 uploader；既有文本同步/冲突/拒收/网络错误用例。
- **audio_capture_controller_test.dart（9）**：权限拒绝不开始；start 失败 → micInUse；start 成功 → recording + captureId + 路径含目录且 .wav 结尾；stop 成功 → saved + kind=audio + size/sha/totalChunks/assetId/pendingSync + 持久化 durationMs；未开始 stop → saveFailed；stop 失败 → saveFailed 且不丢文件；元数据读取失败 → saveFailed；cancel → 停录 + 删文件 + 回 idle；空闲 cancel → 无操作。
- **recording_screen_test.dart（5 widget）**：idle→recording 状态切换（GoRouter）；权限拒绝 → 重试按钮且未开始；停止 → 保存 pendingSync capture + 「查看/修正转写」路由到修正页；停止失败 → 「保存失败」+ 重试；录音中返回 → 二次确认 → 清理文件回 idle。
- **transcript_edit_screen_test.dart（2 widget）**：加载既有转写 + guest 修正保存（source=user, v1）；空 store → 空编辑框 + 「转写生成中」提示。
- **capture_screen_test.dart（3）+ widget_test.dart（6）**：既有回归保持全绿。

## 5. 修复的问题

1. **Riverpod 2.x Notifier 无 overridable dispose** → 控制器在 `build()` 里注册 `ref.onDispose` 清理计时器/振幅订阅。
2. **录音页根路由取消崩溃**：`context.pop()` 在无上级路由时抛 `GoError: There is nothing to pop` → 用 `context.canPop()` 守卫。
3. **保存成功后 cancel 会误删已保存文件** → 成功保存后清空 `_recordingPath`，后续 cancel 不再触碰。
4. **空闲 cancel 误调 recorder.stop** → 仅当存在活跃录音路径时才 stop + delete。
5. **测试 fake-async 直写 sembast 挂起**：testWidgets 里直接 `await store.saveDraft(...)` 依赖真实事件循环，用 `tester.runAsync` 包裹。
6. **pumpAndSettle 死锁**：聚焦 TextField 光标闪烁永续调度帧，改用离散 `pump(Duration)` 等待 SnackBar 结束。

## 6. 下一步（G3 / R5）

AI 总开关与后台任务：UserAISettings 表 + GET/PATCH /users/me/ai-settings（revision 冲突）、AI 记忆整理/补全/语音转写独立开关、Capture 策略快照 + PostgreSQL Outbox Worker、ProcessingTask 生命周期、关闭后取消 queued/retry_wait。

## 7. 证据

- `dart analyze lib test`：通过（0 error / 0 warning）
- `flutter test`：45 个用例全绿
- 后端 `go test ./...`：78 个用例全绿（R04_BACKEND_REPORT.md）
