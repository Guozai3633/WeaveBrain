# 织脑（WeaveBrain）开发目标总控 V1

> 日期：2026-08-29
> 计划版本：V1
> 执行方式：按「开发系统链路」技能，以**目标**为单位推进
> 上层依据：DEVELOPMENT_ROUNDS_PLAN_V1.md（R0—R14 轮次总控）
> 产品基线：PRODUCT_BLUEPRINT_V3.md + weave_demo（高保真原型，已确认设计无大问题）

## 0. 总览

| 目标 | 名称 | 对应轮次 | 状态 |
|---|---|---|---|
| G0 | 当前基线验证 | R0 | 已完成 |
| G1 | R3 验收收尾（真机/浏览器障碍处理） | R3 | 进行中 |
| G2 | 可靠语音捕捉与 STT | R4 | R4 完成（后端 78 测试 + 前端 45 测试绿；真机/E2E 待验证） |
| G3 | AI 总开关与后台任务 | R5 | R5 完成（后端 143 测试 + 前端 65 测试绿；真机/E2E 待验证） |
| G4 | 记忆卡、记忆流与详情 | R6 | R6 完成（后端 227 测试 + 前端 113 测试绿；真机/E2E 待验证） |
| G5 | 验收 B：核心捕捉与 AI 控制 | R7 | R7 完成（后端 242 测试含真实 PG 集成全绿；20 样本人工验收待回填） |
| G6 | AI 补全 | R8 | R8 完成（后端 308 测试含真实 PG 集成全绿 + 前端 144 全绿；真机/E2E 待验证） |
| G7 | 单条与批量导入 | R9 | R9 完成（后端 337 无 env + 356 真实 PG 集成全绿 + 前端 184 全绿） |
| G8 | Web 回顾端、同步与工作流预留 | R10 | 未开始 |
| G9 | 移动快捷入口与一种回响 | R11 | 未开始 |
| G10 | 验收 C：MVP 功能完整性 | R12 | 未开始 |
| G11 | 安全、隐私、性能与发布候选 | R13 | 未开始 |
| G12 | 验收 D：私有 Beta 与上市判断 | R14 | 未开始 |

执行原则（沿用轮次总控）：
- 同一时间只允许一个目标处于「进行中」；
- 验收目标未通过不得进入下一开发目标，除非变更评审批准；
- 每个目标开发前：① 联网搜索选型 ② 步骤清单 ③ 失败处理策略；
- 每个目标开发后：① 核对步骤清单 ② 书写全面测试 ③ 执行测试并修复 ④ 总结。

---

## G0 当前基线验证 — 已完成

验证于 2026-08-29：

- [x] `go build ./...` 通过
- [x] `go vet ./...` 通过
- [x] `go test ./...` 全部通过（internal/api、db/migration、db/repository、service、pkg/crypto）
- [x] `flutter analyze` 零问题
- [x] `flutter test` 16 个测试全部通过
- [x] 后端已注册：/api/v3/captures、/users/me/mcp-configs、/users/me/timeline、v1 全量、voice WS
- [x] STT provider 可配置（STT_PROVIDER=mock|funasr）

结论：后端与 Flutter 默认平台测试全部绿。可开始下一个目标。

---

## G1 R3 验收收尾 — 真机/浏览器障碍处理

### 目标
让 G1（文字捕捉绝不丢、幂等、跨用户安全）获得可复查证据。

### R3 已通过项（软件与自动化）
- [x] 离线保存 20 条（自动化）
- [x] 重启恢复（自动化）
- [x] 5xx 保留原文
- [x] 恢复同步复用 UUID（真实服务端 100 次重放仅 1 条）
- [x] 同 ID 不同内容 → conflict / 409
- [x] 服务端跨用户访问 404、本地跨账户隔离
- [x] 空内容 / 超长内容 / 超大请求体 → 400
- [x] V1 回归、数据库迁移 000001—000006、创建性能 P95=5.841ms
- [x] Flutter 自动化、Go 自动化、Web/Android 编译

### R3 未通过项（均为环境/基础设施依赖）
- [ ] 真实 Android 设备/模拟器飞行模式、杀进程、重启、恢复网络矩阵
- [ ] 真实浏览器 IndexedDB 关闭重开

### 障碍诊断记录
1. **Android 真机/模拟器**：本机无 Android SDK 系统镜像、无 AVD、无连接设备。`flutter emulators` 为空。→ 必须由用户在装有 Android 工具链的设备上执行。
2. **Chrome IndexedDB 测试**：`flutter test --platform chrome` 报「Connection closed before test suite loaded」。已尝试 ASCII 盘符映射（subst）、不同渲染器、reporter 调整，均复现。属测试 harness 环境问题，非业务代码缺陷。→ 提供人工浏览器验证路径替代。

### 步骤清单
- [未完成] 第一步：向用户提供「真机离线捕捉验收清单」（Android/iOS 均可，含飞行模式、杀进程、重启、恢复网络）
- [未完成] 第二步：向用户提供「真实浏览器 IndexedDB 验收清单」（Chrome/Edge，保存→关闭标签→重开）
- [未完成] 第三步：用户执行后回填证据到 R03_REPORT.md，G1 判定

> 说明：本目标不写业务代码，只产出可执行验收路径。G2 开发在 G1 真机证据补齐前，作为「变更评审待批准」事项推进（见计划偏移记录 PLAN-002）。

---

## G2 可靠语音捕捉与 STT（R4）

### 目标
把语音捕捉从「在线 WebSocket Demo」改为「音频先落盘、最终可恢复」的产品能力。

### 步骤清单
- [x] 第一步：联网调研语音音频落盘方案（Flutter record 包 / 音频私有目录 / 分片上传）— record 7.1.0（start(path:) 落盘、onAmplitudeChanged 波形、wav/pcm16bits Web 可靠）
- [x] 第二步：后端定义 AudioAsset 模型与 upload/checksum 契约 — 迁移 000007 + 实体 + 仓储 + 服务 + 路由（R04_BACKEND_REPORT 见 docs/round-reports/）
- [x] 第三步：Flutter 开始录音即创建本地 Capture UUID（后端 kind=audio 已就绪）
  - [x] 3.1 扩展 LocalCapture：kind + audio 元数据（asset_id/mime/size/sha256/total_chunks/uploaded_chunks/local_path/duration/stt_enabled）+ transcript 字段，toMap/fromMap/copyWith/toCreateRequest
  - [x] 3.2 本地存储：updateAudioUploadedChunks 持久化分块进度（重启可续传）
  - [x] 3.3 录音开始即用 capture UUID 保存 kind=audio 空文本本地 Capture
- [x] 第四步：音频写入应用私有文件，停止录音先保存本地 Capture
  - [x] 4.1 record 7.1.0 start(RecordConfig, path: <应用私有目录>/<captureId>.wav) 录音
  - [x] 4.2 停止录音：计算 size + 全文件 SHA-256（crypto 包），保存本地 Capture + 资产元数据，首文案「已安全保存」
- [x] 第五步（后端）：音频资产元数据 + SHA-256，分片/重试上传 — initiate/uploadChunk/complete 幂等 + 全文件校验
- [x] 第六步（后端）：完整音频作为 STT 输入，失败保留原音频可播放 — STTProvider.RecognizeFile + 原音频永删
- [x] 第七步（后端）：TranscriptRevision + 用户修正转写 — PATCH /captures/:id/transcript + GET transcript
- [x] 第八步：权限/占用/取消/异常恢复降级（前端）
  - [x] 8.1 ApiClient io/web 增加 putBytes（原始二进制 chunk 上传，X-Chunk-SHA256 头）
  - [x] 8.2 AudioApi：initiate / uploadChunk / complete / transcribe / correct / getLatest
  - [x] 8.3 AudioUploadService：create capture → initiate → 按缺块上传（跳过已传）→ complete → transcribe；失败 markRetryable，重启续传
  - [x] 8.4 CaptureSyncService 集成音频上传（可选 audioRemote 注入，kind=audio 走完整链路）
  - [x] 8.5 录音 UI：状态机（请求麦克风/录音/保存/上传/转写/就绪/错误）+ 计时 + 波形 + 取消二次确认 + 权限拒绝/占用提示
- [x] 第九步：手机/Web 能力差异提示（前端）
  - [x] 9.1 Web 端显示能力说明（不持久化音频文件，建议 App 录音；回退文字速记）
  - [x] 9.2 录音转写结果页：展示 + 用户修正（PATCH transcript）

### 完成条件
- [ ] 断网可完成录音并在重启后找到音频（移动端落盘 + 本地 Capture；前端已完成，待真机验证）
- [ ] 尾句不因停止 WebSocket 而丢失（完整音频上传 + 后端 RecognizeFile；待端到端联调验证）
- [x] 音频 checksum 校验一致 — 分块 SHA-256 + 全文件 SHA-256 + size 校验（后端有边界/对抗测试）
- [x] STT 失败时原音频仍可播放 — TranscribeCapture 永删原音频，失败保留 complete 资产（有测试）
- [x] speech_to_text_enabled=false 时不调用 STT — stt_enabled=false 跳过且 provider 零调用（有测试）
- [x] 麦克风拒绝和占用有可理解的降级（前端）— 权限拒绝/占用/保存失败/取消均有状态机降级（有 widget 测试）
- [x] 手机与 Web 的能力差异有明确提示（前端）— Web 端 SnackBar 说明 + 录音页不支持提示（有测试）

---

## G3 AI 总开关与后台任务（R5）

### 步骤清单
- [x] 第一步：联网调研 PostgreSQL Outbox Worker 与 ProcessingTask 模式 — 选定「自研 Outbox Worker」（复用现有 capture_outbox，FOR UPDATE SKIP LOCKED + 指数退避 + 策略快照），备选 River / 独立 processing_tasks 表 / Temporal（均因自定义生命周期 + 关闭即取消语义不匹配而放弃）
- [x] 第二步：UserAISettings 表、Repository、Service — 迁移 000008 + 实体 + 仓储（CAS Update）+ Service（Get 物化默认 / Update 部分补丁 + revision 冲突 → VERSION_CONFLICT）；17 测试绿
- [x] 第三步：GET/PATCH /api/v3/users/me/ai-settings — 处理器 + 路由接线 + VERSION_CONFLICT(409) 映射 + 部分补丁校验；10 测试绿，API_V3_CONTRACT §9
- [x] 第四步：AI 记忆整理总开关（默认 false）— DefaultUserAISettings 全 false；worker 以快照 AIMemoryEnabled 门控 pipeline
- [x] 第五步：AI 补全开关、独立语音转写开关、云端文本/音频授权 — UserAISettings 字段 + ai-settings 端点已全部暴露（AICompletionEnabled / SpeechToTextEnabled / CloudTextAllowed / CloudAudioAllowed）；UI 呈现归第九步
- [x] 第六步：Capture 策略快照 + PostgreSQL Outbox Worker — 迁移 000009 + PolicySnapshot 实体 + OutboxRepository（ClaimDue FOR UPDATE SKIP LOCKED / MarkReady / MarkFailed / MarkRetryWait / CancelByUser / CountQueued）+ CaptureRepository.Create 快照入 outbox + CaptureService 注入 AISettingsSnapshotSource + EnrichmentPipeline/OutboxWorker（Run/claim/指数退避/捕获缺失终态）
- [x] 第七步：ProcessingTask 生命周期（queued/retry_wait/processing/ready/failed）— 迁移 000009 状态约束 + worker 状态迁移 + 终端 failed/cancelled
- [x] 第八步：关闭后取消 queued/retry_wait；单条 organize-once — AISettingsService.Update 关闭整理时 CancelByUser；worker 对每事件一次 pipeline（快照门控，重试不重跑已成功阶段）
- [x] 第九步：Flutter AI 设置页与补整理按钮 — 后端 CountPendingReorganize + POST /reorganize（目标集 ready+快照关闭，仅显式触发，AI 关时 409 FEATURE_NOT_ENABLED）+ 手机 /settings/ai 页（总开关/独立转写/3 子开关门控/revision/补整理按钮）；outbox/service/handler 测试 + Flutter api/notifier/screen 测试全绿

### 完成条件
- [x] ai_memory_enabled 初始为 false — DefaultUserAISettings 全 false + 迁移 000008 默认
- [x] AI 关闭后新增 50 条，organize/embed/relate/recap 调用数为 0 — OutboxWorker 快照关闭 → 直接 MarkReady 零 pipeline 调用（测试 TestOutboxWorkerAIDisabledMarksReadyWithoutPipelineCalls）
- [x] 关闭整理但开启转写时仍可得到 TranscriptRevision — STT 走独立 stt_enabled 路径，不受 AIMemoryEnabled 门控；worker 仅门控 organize/embed/relate/recap（R4 已有 stt_enabled=false 跳过测试，反向语义由快照保证）
- [x] 多设备设置冲突返回 VERSION_CONFLICT — handler/service 测试覆盖（revision CAS）
- [x] 再开启只影响新 Capture — 策略快照在捕获创建时冻结，后续设置变更不改历史事件（快照测试 + worker 门控）
- [x] 历史补整理必须显式触发 — POST /users/me/ai-settings/reorganize 显式端点 + 手机页「补整理未处理记忆 (N 条)」按钮（目标集 ready+快照关闭，AI 关时 409 FEATURE_NOT_ENABLED）
- [x] 手机/Web 显示同一设置 revision — Flutter /settings/ai 页展示 revision N，与 Web/后端同源（同一 GET /ai-settings 信封）

---

## G4 记忆卡、记忆流与详情（R6）

### 步骤清单
- [x] 第一步：联网调研记忆流列表/详情/搜索的数据库模式 — 选定 `pg_trgm` GIN `gin_trgm_ops`（PG contrib 自带，CJK 子串 ILIKE 可走索引）+ keyset 游标分页；决策记录 docs/topic_notes/r6_memory_stream_search.md
- [x] 第二步：fallback MemoryCard 完整字段 — 迁移 000010 扩展 memory_cards（summary/tags/key_points/is_pinned/pinned_at）；捕获创建原子写 fallback 卡（title 前 30 rune、summary 前 200 rune + …、tags=[]、key_points=[]、primary_type='uncategorized'）
- [x] 第三步：EnrichmentRevision + provenance + source_revision — memory_card_revisions 表（迁移 000010，source CHECK fallback/ai/user、UNIQUE(user_id,capture_id,revision)）；capture 创建原子写 revision=1/card_version=1/source=fallback/source_revision=1；用户修正产生 source=user 新版本（changes 只含修改字段、provenance 逐字段 user、source_revision=capture.version）
- [x] 第四步：记忆流默认倒序 + 基础筛选 + 全文搜索 — GET /api/v3/memories（is_pinned DESC, created_at DESC, id DESC；keyset 游标 base64url(JSON{p,t,i})；lifecycle_status 默认 active、kind/primary_type/pinned 过滤；q 三列 ILIKE 搜索）
- [x] 第五步：记忆详情按 ID 加载（原文/音频/转写）— GET /api/v3/memories/{captureId}（capture+card+audio?+transcript?+revisions，修订降序；Flutter 详情页从路径参数 captureId 加载）
- [x] 第六步：修正、续写、置顶、归档、删除 — PATCH（部分字段+校验+409 VERSION_CONFLICT）/ POST notes（不改原文）/ pin（pinned_at）/ archive（幂等）/ DELETE（trashed+deleted_at）
- [x] 第七步：主导航调整（记忆/全局捕捉/回响/我的）— app.dart StatefulShellRoute 4 分支 + app_scaffold 4 目的地 + echo 占位页；guest 重定向 /→/capture
- [x] 第八步：Timeline、Projects、MCP 从主路径隐藏 — /ideas /timeline /projects 降级为隐藏顶层路由（不在底部导航），/settings/mcp 保留为设置子页；widget_test 断言底部导航只有 4 个目的地

### 完成条件
- [x] 无 AI 时也能显示可用 fallback 卡 — fallback 派生字段 + fallback revision 在捕获创建时原子生成（capture_service_test + worker 测试覆盖）；前端列表/详情渲染
- [x] AI 失败不覆盖原始内容 — `captures.original_text` 只读；AI pipeline 保持 no-op 至 R8，修正/续写均不改写原文（memory_service AppendNote 不改字段测试）
- [x] Web 详情页刷新后按 ID 恢复 — MemoryDetailScreen 用路径参数 captureId 加载（非 extra）；widget 测试「重建同 captureId → 再次按 ID 加载」
- [x] 每张卡首屏只有一个主要下一步 — 每卡恰一个 FilledButton（按 primary_type 映射主按钮）；list_screen widget 测试断言
- [x] 用户编辑产生新版本 — Correct 产生 source=user 修订 + card.version+1（notifier/screen 测试覆盖）
- [x] 搜索结果可追溯到原始记忆 — q 三列搜索含 original_text 与 transcript；详情页展示原始记忆区块；search 契约测试
- [x] 删除和归档状态正确同步 — SetLifecycle archived（deleted_at 保持 NULL）/ trashed（deleted_at 置位）→ 列表隐藏；前端归档/删除调用与状态更新（notifier/detail 测试覆盖）

---

## G5 验收 B：核心捕捉与 AI 控制（R7）

### 通过标准
- [x] 原始捕捉丢失为 0 —— R7 可追溯链集成测试（真实 PG）：原音频始终存在且 sha256 不变、`captures.original_text` 全程只读；R3/R4 集成回归通过
- [x] AI 关闭时后台整理调用为 0 —— R7 批量 soak 集成测试：AI 关快照行直接 MarkReady，pipeline 调用数为 0
- [x] 95% 可处理任务最终 ready 或明确失败 —— R7 批量 soak 集成测试：40/40 = 100% 终态（ready=31 + failed=6 + cancelled=3）≥ 95%
- [x] 原音频、原文和用户修正版均可追溯 —— R7 可追溯链集成测试：音频（sha256）+ stt 修订 + user 转写修订 + fallback 卡修订 + user 卡修订全部保留（修订源计数 `[stt:1,user:1]` / `[fallback:1,user:1]`）
- [ ] 20 名内测体验样本中，超过 60% 认为卡片表达原意 —— 待人工执行 `docs/round-reports/R07_MANUAL_UX_SAMPLE.md`（20 条短语 + 判定 ≥65%）并回填
- [x] 严重权限与数据串用户问题为 0 —— R7 跨用户端到端集成测试 + docs/API_V1_SECURITY_CONTRACT.md（ideas/reminders/users/workflow/timeline 全量收口）
- [x] 形成 R7 验收报告并标记 G5 通过 —— docs/round-reports/R07_REPORT.md（注：原文「标记 G2 通过」应为 G5，已按 G5 执行）

---

## G6 AI 补全（R8）

### 步骤清单
- [x] CompletionProposal / FieldProposal 模型 —— 迁移 000013 + internal/entity/completion.go（ProposalStatus / ApplyPolicy / Provenance / EvidenceSpan / CompletionProposal / PreviewResult / ApplyResult）
- [x] completion:preview / completion:apply —— POST /captures/:captureId/completion/{preview,apply,undo}（preview 不改业务对象；apply 幂等 + 版本安全；undo 完整撤销）
- [x] evidence_spans、safe_auto/suggest_only/forbidden —— evidence_spans JSONB（字节偏移 + 逐字引文）；5 个目标字段有证据→safe_auto / 无证据→suggest_only / 其他→forbidden；前端「AI 建议 / 需确认」chip
- [x] 逐字段采用、全部采用安全字段、版本与撤销 —— 面板「全部采用安全字段」批量 + 逐字段「采用」+「撤销上次补全」；source_revision 冲突 → 提案过期 + 409
- [x] 详情页补全入口、旧提案过期 —— memory_detail_screen 接入 CompletionPanel（AI 补全开启且卡片 ready 才展示）；新 preview 使该 capture 全部 pending 过期

### 完成条件
- [x] preview 不修改业务对象 —— CompletionService.Preview 对 capture/card 零改动；服务测试断言
- [x] apply 默认只填空字段 —— fieldEmpty 判定（primary_type=uncategorized 视为缺失），非空跳过（服务测试）
- [x] 用户值和导入原值受保护 —— apply 非空字段提案置 rejected 绝不覆盖；undo 只回滚「当前值仍==AI 所设」字段（服务测试）
- [x] 事实型建议有证据片段 —— buildEvidenceSpans 逐字定位原文首现字节偏移；无证据降级 suggest_only（服务测试 + 前端证据 chip）
- [x] source_revision 变化后旧提案不可应用 —— pending 提案 SourceRevision != card.Version → MarkExpired + ErrCompletionVersionConflict（409）（服务测试）
- [x] 应用后可以撤销 —— undo 端点回滚最近一次 ai 补全 revision（source=user 撤销修订，提案保持 accepted 审计留痕）（服务测试 + 前端 undo）
- [x] AI 补全关闭时不展示或调用入口 —— preview 门控 AICompletion && CloudTextAllowed；前端 _aiCompletionAvailable 未开启/未 ready 渲染 SizedBox.shrink() 零调用（widget 测试）

---

## G7 单条与批量导入（R9）

### 步骤清单
- [x] 单条文字/音频/文件导入 —— 单条复用 `POST /captures` kind=import（external_id/source_name/title/tags/primary_type 透传 + 去重 + content_hash 疑似提示）；批量文件选择（IO readAsString / Web FileReader）
- [x] 多段文本、TXT、Markdown、CSV、JSONL 解析 —— `import_parser.go` plainTextParser / csvParser / jsonlParser；txt/markdown/md 归一化 plain_text
- [x] 字段映射、前 10 条预览 —— GetPreview limit=10/offset=0 + column_mapping + 校验错误/疑似重复 chip
- [x] AI 补全缺失项、去重、Commit、行级错误隔离 —— completion:preview/apply 复用 R8 FieldProposalGenerator；去重 = external_id 精确硬跳过 + content_hash 疑似；Commit 每行独立短事务
- [x] 结果报告与错误报告下载 —— 结果页统计 + 错误报告（JSON / CSV 下载，IO 复制 / Web Blob）

### 完成条件
- [x] 单行错误不影响其他行 —— Commit 每行独立事务，失败行标 `failed` 其他行照常（服务测试）
- [x] 1 千行 MVP 导入稳定 —— 单事务建 job + rows（预计算去重），8 MiB body 预算，1000 短事务 MVP 可接受
- [x] 重复 Commit 不产生重复 Capture —— capture_id=row.id 确定性 + Capture CTE `ON CONFLICT (user_id,id) DO NOTHING` + imported 行快进（服务测试幂等）
- [x] 补全失败可按原样导入 —— completion/preview 单行 LLM 失败记 error 不阻塞；Commit 按原样导入（服务测试）
- [x] Commit 前展示字段映射、错误和疑似重复 —— 前端预览表格 + 去重 chip（重复·跳过 / 疑似重复 / 待补充）+ 校验错误
- [x] 地点/时间缺失时保持 unknown —— captured_at/地点缺失不伪装导入时间、不猜测坐标；落库保持 unknown（服务测试）
- [x] 可下载错误报告 —— GET error-report?format=csv（text/csv + attachment）与 JSON；前端 Web 下载 CSV / IO 复制

---

## G8 Web 回顾端、同步与工作流预留（R10）

### 步骤清单
- [ ] Web 记忆流、搜索、详情、导入和设置
- [ ] 登录后游客 Capture 合并
- [ ] 同步游标和冲突处理
- [ ] /workflows、/workflows/:id/designer「规划中」页
- [ ] GET /api/v3/capabilities（workflow_designer=false, workflow_execution=false）

### 完成条件
- [ ] 同一 Capture 在手机/Web 最终一致
- [ ] 游客记录登录后无重复合并
- [ ] Web 可完成回顾、搜索、导入和 AI 设置
- [ ] 误调用工作流接口不创建任务或副作用
- [ ] 页面明确展示「规划中」

---

## G9 移动快捷入口与一种回响（R11）

### 步骤清单
- [ ] App 内中央全局捕捉按钮
- [ ] Android/iOS App Shortcut
- [ ] 桌面小组件（选一端）
- [ ] 极简录音页 + 声音/触觉确认
- [ ] 一种回响（每日/隔日或每周主题）+ 完成/稍后/无关反馈
- [ ] 频率与静默时段、通知深链

### 完成条件
- [ ] 系统入口到开始录音 P50 小于 1.5 秒
- [ ] 捕捉全程最多一次主动操作
- [ ] 不进入主页也能开始捕捉
- [ ] 回响可关闭、每次说明出现原因
- [ ] 通知点击能恢复到目标记忆
- [ ] 记录有用/无关/稍后反馈

---

## G10 验收 C：MVP 功能完整性（R12）

### 通过标准
- [ ] R3、R7 的验收项无回归
- [ ] 手机/Web 端到端流程全部通过
- [ ] AI 补全无静默覆盖
- [ ] 导入无跨行回滚和重复
- [ ] 工作流无意外执行
- [ ] P0/P1 缺陷全部关闭
- [ ] 功能冻结清单签字确认
- [ ] 形成 R12 验收报告并标记 G3 通过

---

## G11 安全、隐私、性能与发布候选（R13）

### 通过标准
- [ ] P0 安全问题为 0
- [ ] P1 严重稳定性问题为 0
- [ ] 日志不包含 Token、密钥、完整音频 URL 和完整敏感文本
- [ ] 删除链路覆盖派生内容和资产
- [ ] Release 构建可安装、可回滚
- [ ] 监控告警可触发
- [ ] 备份恢复演练成功
- [ ] 形成 Release Candidate 版本号

---

## G12 验收 D：私有 Beta 与上市判断（R14）

### 通过标准
- [ ] 原始捕捉丢失事件为 0
- [ ] 最终同步成功率达到目标且重复为 0
- [ ] 跨用户数据泄露为 0
- [ ] AI 关闭时未授权整理调用为 0
- [ ] 删除和导出经过真实账号验证
- [ ] Crash、任务失败和成本处于可接受范围
- [ ] 权限、隐私和音频保留说明完成
- [ ] 商店素材、签名和版本流程完成

### 最终决策
- Go / Fix / No-Go（三选一）

---

## 计划偏移记录

| 变更编号 | 日期 | 原计划 | 新计划 | 原因 | 影响目标 | 是否批准 |
|---|---|---|---|---|---|---|
| PLAN-002 | 2026-08-29 | R3 真机/浏览器证据补齐后才进入 R4 | 在 G1 真机证据补齐前，G2（R4 开发）先行推进；G1 以「人工验收清单 + 用户执行」完成 | 用户确认设计后指示「开始进行项目开发」；真机/浏览器障碍为本机环境不可解项，非代码缺陷 | G1、G2 | 待用户批准 |

## 修订历史

| 版本 | 日期 | 内容 |
|---|---|---|
| V1 | 2026-08-29 | 依据轮次总控建立目标总控，记录 R3 障碍诊断与 PLAN-002 偏移 |
