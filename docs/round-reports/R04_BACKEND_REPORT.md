# 织脑（WeaveBrain）R4 后端阶段报告

> 轮次：R4 — 可靠语音捕捉与 STT
> 阶段：后端（迁移 / 实体 / 仓储 / 服务 / 路由 / 测试）
> 日期：2026-08-29
> 前端阶段（录音→落盘→上传→能力降级）尚未开始

## 1. 阶段结论

R4 后端已完成并全绿：`go build ./...`、`go vet ./...`、`go test ./...` 全部通过。音频资产采用「分块上传 + 每块 SHA-256 + 完成时全文件 SHA-256/size 校验」，完整音频作为 STT 输入（RecognizeFile），STT 失败保留原音频。为前端阶段提供完整、可测试的 API 契约。

## 2. 数据模型（迁移 000007）

- `audio_assets`：user_id+id 复合主键；capture_id 外键级联到 captures(user_id,id)；size_bytes>0、upload_state∈(initiated,uploading,complete,failed) CHECK；storage_path 由服务端派生，客户端不可指定。
- `transcript_revisions`：不可变修订版本，revision=(max+1) 原子递增，source∈(stt,user) CHECK，(user_id,capture_id,revision) 唯一。
- 契约测试断言：迁移不 ALTER captures；Down 顺序正确（先 transcript 后 audio）。

## 3. 后端能力与契约

| 端点 | 方法 | 用途 | 关键校验 |
|---|---|---|---|
| /api/v3/audio-assets | POST | 发起分块上传（幂等） | capture 必须存在且 kind=audio；mime/size/sha256/total_chunks 校验 |
| /api/v3/audio-assets/:id | GET | 读取资产 | 用户隔离 |
| /api/v3/audio-assets/by-capture/:captureId | GET | 读取 capture+audio+最新转写 | 用户隔离 |
| /api/v3/audio-assets/:id/chunks/:index | PUT | 上传单块（raw body + X-Chunk-SHA256） | 每块 SHA-256；index 范围；≤5 MiB |
| /api/v3/audio-assets/:id/complete | POST | 完成上传 | 块齐全且连续；size 一致；全文件 SHA-256 |
| /api/v3/captures/:captureId/transcribe | POST | 全文件 STT | 上传已 complete 且 stt_enabled；失败保留原音频 |
| /api/v3/captures/:captureId/transcript | PATCH | 用户修正转写 | 非空、合法 UTF-8、≤100,000 字符 |
| /api/v3/captures/:captureId/transcript | GET | 读取最新转写 | 用户隔离 |

新增 V3 错误码：`PRECONDITION_FAILED`（412，用于 complete 前置条件不满足）。

## 4. 设计要点

- **storage_path 服务端派生**：形如 `audio/{userID}/{assetID}.{ext}`，存入 audio_assets.storage_path，客户端不可写入；文件存储把逻辑路径映射到物理基目录，并拒绝路径穿越。
- **分块幂等**：按 index 落盘 part-N，重发同一块覆盖；重放不重复计数。
- **完成时校验**：块集合必须连续无缺；拼接后 size 必须等于声明；全文件 SHA-256 必须等于声明，任一失败返回 400 并清理 staging。
- **STT 不删音频**：TranscribeCapture 失败只返回错误，原音频资产保持 complete 且 checksum 不变，可随时播放或重试。
- **转写版本链**：STT 写入 revision=1（source=stt），用户修正追加 revision=2（source=user），全部不可变可追溯。
- **AI 独立开关预留**：stt_enabled=false 时不调用 STT provider（provider 调用数为 0 有测试覆盖）。

## 5. 测试情况

共新增/更新 78 个通过测试（全部边界 + 对抗）：

- **capture_service_test.go**：新增音频 capture（kind=audio、空文本、标题「语音记录」）正向测试、unsupported kind 拒绝测试；移除 R1 时代错误的「audio 不支持」断言。
- **audio_service_test.go**（约 35 项）：initiate 校验矩阵（nil user/asset/capture、空 mime、零/负/超大 size、零块、坏 sha、负时长）；文本 capture 拒绝；缺失 capture 拒绝；幂等 initiate；分块校验（checksum 不匹配不计数、越界、过大、资产不存在、重试幂等）；complete（成功、缺块、乱序、size 不匹配、全文件 sha 不匹配、幂等、零块）；transcribe（成功更新卡片标题、未上传 412、stt_enabled=false 零调用、无音频 404、STT 失败保留原音频、空转写报错、revision 递增）；用户修正（成功、空/超长/非法 UTF-8 拒绝、stt 后 user revision=2）；GetByCapture/GetByID/GetLatestTranscript 用户隔离。
- **audio_store_test.go**（约 12 项）：真实 LocalAudioFileStore 往返、分块重写幂等、缺块 finalize 失败、路径穿越拒绝（OpenFinal/Finalize）、OpenFinal 目录拒绝、RemoveStaging 幂等、ListStagedChunks、空 base dir 拒绝。
- **stt/mock provider_test.go**（6 项）：脚本结果、缺文件、空文件、FAIL 模拟失败、无脚本报错。
- **audio_handler_test.go**（约 13 项）：initiate 201/400、chunk 上传 204/400（坏 sha、坏 index、checksum 映射）、complete 200/412、transcribe/修正 201、修正空文本 400、GetByCapture/ByID/transcript 200、404 映射、401 未认证。

## 6. 修复的问题

1. **storage_path 不一致（真实缺陷）**：Complete 将文件写入 `{base}/final/{assetID}.{ext}`，而 TranscribeCapture 以 `asset.StoragePath`（`audio/{userID}/{assetID}.{ext}`）调用 OpenFinal，两路径不匹配，会导致 STT 永远「invalid final audio path」。重构为：DB 存逻辑路径，文件存储统一解析到物理基目录，OpenFinal/Finalize 一致。
2. **GetLatestTranscript 未映射错误**：跨用户/无转写时返回原始仓储错误，统一映射为 404 语义。
3. **路径穿越防护**：LocalAudioFileStore 拒绝 `../` 逃逸；OpenFinal 拒绝目录（避免 Windows 句柄泄漏）。
4. **ListStagedChunks 未导出**：接口要求大写导出，小写实现补齐。

## 7. 下一步（前端阶段）

- record v6.2.0 录音 → 应用私有目录落盘 → 本地 Capture UUID 先行保存；
- 分块 SHA-256 上传 + 重试；
- 完整音频 STT 接入 transcribe 端点；
- 用户修正转写界面；
- 麦克风拒绝/占用、取消、异常恢复降级；手机/Web 能力差异提示。

## 8. 证据

- `go build ./...`：通过
- `go vet ./...`：通过
- `go test ./...`：通过（78 个含边界/对抗的测试用例）
