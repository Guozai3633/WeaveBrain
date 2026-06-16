# AGENTS.md - 织脑 (WeaveBrain) 子智能体名册

## 可用 Subagent

| Agent | 职责 | 路径范围 | 触发方式 |
|---|---|---|---|
| `go-backend-expert` | Go 后端代码、Eino Agent 定义、Temporal 工作流 | `WeaveBrain/**` | `/agents go-backend-expert "描述"` |
| `flutter-frontend-expert` | Flutter 代码、Widget、状态管理 | `WeaveFlutter/**` | `/agents flutter-frontend-expert "描述"` |
| `database-schema-expert` | PostgreSQL 迁移、Repository、查询优化 | `migrations/**`, `WeaveBrain/internal/db/**` | `/agents database-schema-expert "描述"` |
| `test-coverage-expert` | 单元测试、Widget 测试、集成测试 | `*_test.go`, `*_test.dart` | `/agents test-coverage-expert "描述"` |
| `mcp-tool-engineer` | golang-mcp 工具定义、JSON Schema、安全封装 | `WeaveBrain/internal/mcp/**` | `/agents mcp-tool-engineer "描述"` |

## Agent Teams vs 单个 Subagent

| 场景 | 使用方式 | 示例 |
|---|---|---|
| 单一专注任务 | 单个 Subagent | "写 Ideas 表的 goose 迁移" |
| 跨域修改 | Agent Team | "加 Notion 工具 -- 后端 API + MCP 服务器 + Flutter UI" |

## 调用示例

```
# 单个 Agent
/agents go-backend-expert "实现 Supervisor agent 节点，包含 ReAct 循环配置"

# Agent Team
/agents "team" go-backend-expert flutter-frontend-expert "添加提醒创建流程 -- 后端端点、Temporal 工作流、Flutter 页面"

# 测试 Agent
/agents test-coverage-expert "为 IdeaRepository 写单元测试"
```
