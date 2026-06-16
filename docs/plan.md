# Sprint 2: Agent 集成实施计划

## 目标
将 Eino Agent 框架集成到 WeaveBrain 后端，实现 Supervisor Agent + Worker Agents + MCP 工具 + ReAct 循环 + HITL 中断机制。

## 步骤

### 1. 添加 Eino 依赖
- `go get github.com/cloudwego/eino`
- `go get github.com/cloudwego/eino-ext`
- 验证依赖解析

### 2. 实现 LLM 客户端封装 (`internal/agent/llm.go`)
- 封装 Ollama OpenAI 兼容客户端
- 配置 ChatModel (支持 Qwen/Mistral)

### 3. 实现 Supervisor Agent (`internal/agent/supervisor.go`)
- 使用 `react.Agent` 创建 ReAct 循环
- 配置 MessageModifier (注入系统提示词 + 时间戳)
- 配置 MaxIter = 10
- 声明输入输出 Go 结构体类型

### 4. 实现 Worker Agents (`internal/agent/workers.go`)
- Environment Context Agent: 检索项目上下文和历史想法
- User Profile Agent: 查询用户画像
- Tool Executor Agent: MCP 工具执行

### 5. 集成 MCP 工具 (`internal/mcp/`)
- 创建工具注册表 (`internal/mcp/registry.go`)
- 实现占位 MCP 工具 (Notion, Email, Calendar)
- 使用 golang-mcp server-first 设计

### 6. 实现 HITL 中断机制 (`internal/agent/hitl.go`)
- 包装敏感工具为 ApprovableTool
- 实现 CheckPointStore (PostgreSQL 持久化)
- 实现中断触发和恢复逻辑

### 7. 集成到 API 层
- 创建 Agent Service (`internal/service/agent_service.go`)
- 添加 Agent API Handler (`internal/api/agent_handler.go`)
- 路由注册

### 8. 验证编译和测试
- `go build` 验证编译
- 测试 Agent 端点
