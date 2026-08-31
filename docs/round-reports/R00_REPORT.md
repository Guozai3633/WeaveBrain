# R00 基线、契约与工具链验收报告

- 轮次：R0
- 开始日期：2026-08-16
- 结束日期：2026-08-16
- 状态：已通过
- 关卡：G0
- 下一轮：R1 Capture 后端内核（未开始）

## 1. 计划目标

1. 恢复可重复的 Flutter analyze/test；
2. 保持 Go 全量测试通过；
3. 冻结 /api/v3 前缀、请求 ID、统一错误体和基础错误码；
4. 不创建 Capture 表，不进入业务功能；
5. 形成可复查文档和验收证据。

## 2. 实际完成

### Flutter 基线

- 确认中文工作区路径会导致 Flutter Analysis Server LSP JSON 输入截断；
- 新增 scripts/flutter_check.ps1；
- 脚本在非 ASCII 路径下使用空闲临时盘符 W:/V:/U:；
- 脚本在 finally 中自动移除映射并返回真实退出码；
- 修复现有代码中 3 个编译级错误、警告和局部兼容 lint；
- flutter analyze 已达到 No issues found；
- flutter test 已通过。

### Go 与 API V3

- 新增全局 X-Request-ID 中间件；
- 非法或缺失 Request ID 自动替换为 UUID；
- 新增 GET /api/v3/meta；
- 新增 V3 统一错误体；
- 冻结 INVALID_ARGUMENT、UNAUTHORIZED、FORBIDDEN、NOT_FOUND、IDEMPOTENCY_CONFLICT、VERSION_CONFLICT、FEATURE_NOT_ENABLED、INTERNAL；
- 未知 V3 路由返回统一 JSON；
- 未知 V1 路由保持旧响应形状；
- CORS 允许 X-Request-ID、Idempotency-Key、If-Match，并暴露 X-Request-ID；
- 新增契约测试。

### 文档

- 新增 docs/API_V3_CONTRACT.md；
- 新增 docs/TOOLCHAIN_BASELINE.md；
- 更新开发轮次总控状态；
- 形成本文档。

## 3. 修改文件

### 新增

- scripts/flutter_check.ps1；
- WeaveBrain/internal/api/contract_v3.go；
- WeaveBrain/internal/api/contract_v3_test.go；
- docs/API_V3_CONTRACT.md；
- docs/TOOLCHAIN_BASELINE.md；
- docs/round-reports/R00_REPORT.md。

### 最小修改

- WeaveBrain/internal/api/server.go；
- WeaveBrain/internal/api/middleware.go；
- weave_flutter/lib/features/ideas/data/idea_api.dart；
- weave_flutter/lib/features/settings/ui/mcp_settings_screen.dart；
- weave_flutter/lib/features/settings/ui/settings_screen.dart；
- weave_flutter/lib/features/timeline/data/timeline_api.dart；
- weave_flutter/lib/features/timeline/ui/timeline_screen.dart；
- weave_flutter/lib/shared/api/api_client_io.dart；
- weave_flutter/lib/shared/api/api_client_web.dart；
- weave_flutter/lib/shared/models/timeline_event.dart。

工作区中其他既有修改属于先前工作，R0 未清理、重置或覆盖。

## 4. 数据库迁移

无。

R0 明确禁止创建 000006 Capture 迁移。

## 5. API 变更

新增：

- GET /api/v3/meta；
- X-Request-ID 响应头；
- V3 统一错误响应。

没有新增 Capture、MemoryCard、AI Settings 或 Workflow 业务接口。

## 6. 自动化测试证据

### Go

最终命令：

- go test -count=1 ./...；
- go vet ./...。

结果：

- 全量测试通过；
- weavebrain/internal/api 契约测试通过；
- go vet 通过。

### Flutter

标准命令：

- scripts/flutter_check.ps1 -Task all。

结果：

- flutter analyze：No issues found；
- flutter test：All tests passed；
- 修复完成后连续三次 analyze/test 成功；
- 每次结束后临时 W: 映射均被清理；
- 最终明确输出 ASCII_DRIVE_CLEANUP_OK。

## 7. 安全检查

- Request ID 只接受 UUID，避免任意追踪头进入日志链路；
- 错误响应不返回堆栈、SQL、Token 或 Provider 原文；
- CORS 只增加 V3 必需请求头；
- V1 路由和认证链路未迁移；
- 未引入外部网络调用；
- 未创建后台任务。

## 8. 已知问题

- 当前 Flutter 自动化只有一个基础 Widget 测试，覆盖率仍低；
- 多数 Go 业务包仍无测试；
- 仓库存在两份 compose 文件；
- Ollama 镜像仍使用 latest；
- 两份 compose 的 Temporal 版本不一致；
- 上述容器问题不在 R0 V3 契约测试路径，必须在 R13 前解决。

## 9. 偏离计划

有一项受控偏离：

- 为使 flutter analyze 通过，修复了 Timeline 页面中的既有参数拼写、API 查询参数名和局部 lint；
- 这些修改只恢复编译与静态分析，不新增产品能力；
- 未开始任何 R1 或后续功能。

## 10. 验收结论

- G0：通过；
- 是否允许进入下一轮：允许；
- R1 当前状态：未开始；
- 技术验收：Codex 自动化验证通过；
- 用户批准开始 R1：待用户指令。
