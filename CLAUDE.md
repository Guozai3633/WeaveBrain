# CLAUDE.md - 织脑 (WeaveBrain) 主指令

## 项目概述

**织脑 (WeaveBrain)**：以语音为入口、Agent为引擎的个人思维外脑。
- Go 后端 (Eino Agent 编排 + Temporal 工作流调度 + golang-mcp)
- Flutter 移动端 (Agent 生成，人类审查)
- 核心能力: 语音→结构化想法 pipeline，多 Agent 协同，MCP 工具执行

## 核心开发规则

1. **>3 文件修改必须先 Plan Mode**: 任何涉及超过 3 个文件的变更，必须先用 Plan Mode 生成 `plan.md`，人类审核后再写代码
2. **Go 类型对齐**: Eino 节点必须用 Go 结构体声明 I/O 类型，严禁滥用 `map[string]any`
3. **Flutter 空安全**: 绝对禁止 `!` 操作符。使用 Dart 3+ 模式匹配处理可空类型
4. **Flutter 不可变性**: 尽可能使用 `final`/`const`。优先提取 Widget 而非方法
5. **测试安静模式**: 运行测试使用静默标志，仅报告失败项和覆盖率
6. **更新 MEMORY.md**: 每次完成任务后更新 MEMORY.md，清理已完成项
7. **不确定就问**: 不确定的需求时，读取 docs/PRD.md，不要假设
8. **跨领域拆分**: 多领域任务拆分 Subagent 并行处理

## Token 效率规则

- 使用 path-scoped rules (`.claude/rules/*.md`)，不要膨胀 CLAUDE.md
- 按需读取文件，不要加载整个代码库上下文
- 用 `/agents` 处理并行任务，上下文过期时用 `/clear` 清理

## 文档索引

| 文档 | 路径 | 内容 |
|---|---|---|
| 产品需求 | `docs/PRD.md` | 产品功能规格、用户故事、数据模型 |
| 系统架构 | `docs/ARCHITECTURE.md` | 技术栈决策、Agent 架构、数据库设计 |
| MCP 规范 | `docs/MCP_INTEGRATION.md` | MCP 工具 Schema、安全模型 |
| 后端规则 | `.claude/rules/backend.md` | Eino、Temporal、API 设计、数据库 |
| 前端规则 | `.claude/rules/frontend.md` | Flutter 架构、空安全、测试 |
| 数据库规则 | `.claude/rules/database.md` | PostgreSQL 规范、迁移 |
| Agent 名册 | `AGENTS.md` | 子智能体列表、调用方式 |
| Sprint 状态 | `MEMORY.md` | 当前任务状态 |
