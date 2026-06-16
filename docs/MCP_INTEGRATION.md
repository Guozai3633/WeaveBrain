# 织脑 (WeaveBrain) - MCP 集成规范

## 1. MCP 协议概述

### 1.1 什么是 MCP
MCP（Model Context Protocol）是一项开放标准，用于规范化 AI 系统与外部数据源及工具的通信。它提供：
- 标准化的工具发现协议（`tools/list`）
- 标准化的工具调用协议（`tools/call`）
- 类型安全的 JSON Schema 定义
- 双向通信（服务端和客户端均可发起请求）

### 1.2 为什么用 MCP 而非自定义 API
- 统一的工具发现机制，新增工具无需修改 Agent 代码
- 自动 JSON Schema 生成，模型调用更准确
- 一致的错误处理和返回值格式
- 社区工具生态可复用（Notion、GitHub、Slack 等已有 MCP Server）

### 1.3 在我们的架构中的角色
- 后端作为 MCP Client（消费工具）和 MCP Server host（暴露工具）
- Eino Agent 通过 `eino-ext/mcpp` 获取 MCP 工具
- 每个 MCP Server 进程独立运行，可单独部署和扩缩容

## 2. MCP 服务器架构

### 2.1 实现库
- 使用 `golang-mcp`（Go 实现，Server-First 设计）
- 通过 Go 反射自动生成 JSON Schema

### 2.2 Server 设计

```go
// MCP Server 初始化
server := mcp.NewServer("weavebrain-notion", "1.0.0")

// 注册工具
server.MustRegisterTool(
    "create_page",
    "在 Notion 工作区中创建一个新页面",
    CreatePageInput{},
    func(ctx context.Context, input *CreatePageInput) (*CreatePageOutput, error) {
        // 实际实现
        return nil, nil
    },
)
```

### 2.3 传输层

| 方式 | 适用场景 | 配置 |
|---|---|---|
| SSE over HTTP | 远程 MCP Server（跨机器部署） | Gin handler 转发 SSE 事件 |
| stdio | 本地 MCP Server（同容器内） | 标准输入输出流 |

### 2.4 Schema 自动生成

通过 Go struct tag 自动映射为 JSON Schema：

```go
type CreatePageInput struct {
    DatabaseID string `json:"database_id" jsonschema_description:"Notion 数据库 ID"`
    Title      string `json:"title" jsonschema_description:"页面标题"`
    Properties map[string]any `json:"properties" jsonschema_description:"页面属性"`
}
```

`golang-mcp` 自动将其转为：
```json
{
  "type": "object",
  "required": ["database_id", "title"],
  "properties": {
    "database_id": { "type": "string", "description": "Notion 数据库 ID" },
    "title": { "type": "string", "description": "页面标题" },
    "properties": { "type": "object", "description": "页面属性" }
  }
}
```

## 3. 工具注册表（MVP 工具）

### 3.1 Notion 工具集

#### `create_page` - 创建 Notion 页面
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `database_id` | string | 是 | Notion 数据库 ID |
| `title` | string | 是 | 页面标题 |
| `properties` | object | 否 | 页面属性（可选） |
| `parent_page_id` | string | 否 | 父页面 ID（可选） |

**返回**：`{page_url: string, page_id: string}`
**场景**：用户说"记到 Notion"，Agent 自动创建页面

#### `update_page` - 更新 Notion 页面
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `page_id` | string | 是 | 页面 ID |
| `properties` | object | 是 | 要更新的属性 |
| `blocks` | array | 否 | 要追加的内容块 |

**返回**：`{updated_page_url: string}`
**场景**：用户说"更新昨天在 Notion 的笔记"

#### `query_database` - 查询 Notion 数据库
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `database_id` | string | 是 | 数据库 ID |
| `filter` | object | 否 | 过滤条件 |
| `sorts` | array | 否 | 排序规则 |

**返回**：`{results: array}`
**场景**：Agent 搜索现有 Notion 内容

### 3.2 163 邮箱工具集

#### `send_email` - 发送邮件
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `to` | string | 是 | 收件人邮箱 |
| `subject` | string | 是 | 邮件主题 |
| `body` | string | 是 | 邮件正文（纯文本） |
| `html_body` | string | 否 | 邮件正文（HTML 格式） |
| `attachments` | array | 否 | 附件列表 |

**返回**：`{message_id: string, sent_at: string}`
**场景**：用户说"把这个想法发到我的邮箱"
**认证**：SMTP 凭据存储在 PostgreSQL，加密

### 3.3 日历/闹钟工具集

#### `create_reminder` - 创建提醒
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `title` | string | 是 | 提醒标题 |
| `description` | string | 是 | 提醒描述 |
| `trigger_time` | string | 是 | 触发时间（ISO 8601） |
| `timezone` | string | 否 | 时区（默认 Asia/Shanghai） |

**返回**：`{reminder_id: string, status: string}`
**场景**：Agent 检测到时间敏感的想法 → 创建提醒 → Temporal 调度通知

#### `set_scheduled_notification` - 定时通知
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `notify_time` | string | 是 | 通知时间（ISO 8601） |
| `notification_content` | string | 是 | 通知内容 |
| `notification_type` | string | 是 | 类型：push / email |

**返回**：`{notification_id: string}`
**场景**：Agent 决定"明天提醒用户继续这个想法"

### 3.4 文件系统工具集

#### `read_file` - 读取文件
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `path` | string | 是 | 沙盒内文件路径 |

**返回**：`{content: string, mime_type: string}`

#### `write_file` - 写入文件
| 参数 | 类型 | 必填 | 说明 |
|---|---|---|---|
| `path` | string | 是 | 沙盒内文件路径 |
| `content` | string | 是 | 文件内容 |
| `overwrite` | bool | 否 | 是否覆盖已有文件（默认 false） |

**返回**：`{bytes_written: int}`

## 4. MCP 工具 Schema 定义规范

### 4.1 Go struct 到 JSON Schema 映射规则

```go
type ReminderInput struct {
    Title        string    `json:"title" jsonschema_description:"提醒标题"`
    TriggerTime  time.Time `json:"trigger_time" jsonschema_description:"触发时间, ISO 8601 格式"`
    Description  string    `json:"description,omitempty" jsonschema_description:"可选, 提醒详细描述"`
}
```

生成的 Schema：
```json
{
  "type": "object",
  "required": ["title", "trigger_time"],
  "properties": {
    "title": { "type": "string", "description": "提醒标题" },
    "trigger_time": { "type": "string", "format": "date-time", "description": "触发时间, ISO 8601 格式" },
    "description": { "type": "string", "description": "可选, 提醒详细描述" }
  }
}
```

### 4.2 命名约定
- 工具名：`{action}_{target}`（如 `create_reminder`、`send_email`）
- 参数名：snake_case
- JSON Schema description：中文描述，便于 LLM 理解

## 5. 安全模型

### 5.1 认证
| 工具 | 认证方式 | 存储位置 |
|---|---|---|
| Notion | Notion API Key | PostgreSQL 加密存储，按用户隔离 |
| 163 邮箱 | SMTP 用户名/密码 | PostgreSQL 加密存储，按用户隔离 |
| 文件系统 | 沙盒路径 | 内置限制，无需额外认证 |

### 5.2 授权
- 每次工具调用验证用户身份（JWT 中的 UserID）
- 工具操作限制在用户的项目范围内
- 管理员工具需额外权限验证

### 5.3 沙盒隔离
- MCP 工具运行在独立 Docker 容器中
- 无主机文件系统直接访问权限
- 网络限制到批准的端点
- 输出大小限制（防止滥用）

### 5.4 审计日志
每条 MCP 工具调用记录：
```json
{
  "event": "mcp_tool_call",
  "user_id": "uuid",
  "project_id": "bigint",
  "tool_name": "create_reminder",
  "input": {...},
  "output": {...},
  "timestamp": "2026-01-15T10:30:00Z",
  "status": "success"
}
```

## 6. 工具发现与注册流程

### 6.1 启动注册

```go
// MCP Server 启动流程
func main() {
    server := mcp.NewServer("weavebrain-calendar", "1.0.0")

    // 连接 PostgreSQL 加载用户工具配置
    configs := loadUserToolConfigs(db)

    for _, cfg := range configs {
        tool := newCalendarTool(cfg) // 根据配置创建工具
        server.RegisterTool(tool.Name(), tool.Description(), tool.InputSchema(), tool.Handler())
    }

    // 启动 SSE server
    http.HandleFunc("/sse", sseHandler(server))
    http.ListenAndServe(":8081", nil)
}
```

### 6.2 Eino 集成

```go
// Agent 启动时获取 MCP 工具
mcpClient, err := mcpp.Dial(ctx, "http://mcp-notion:8081")
tools, err := mcpClient.GetTools(ctx)
// tools 直接注入到 Eino ToolsNode
toolsNode := agent.NewToolsNode(chatModel).AddTools(tools...)
```

### 6.3 动态扩展
- 用户在设置页配置新工具后，对应 MCP Server 热加载新工具
- 不需要重启整个后端服务
- 失败的工具注册仅记录日志，不影响其他工具

## 7. 高级玩家扩展

### 7.1 自定义 MCP 端点
- 设置页提供 UI 让用户注册自己的 MCP Server
- 配置项：endpoint URL、认证方式（Bearer Token / API Key）、超时时间
- 信任模型：用户显式启用每个自定义工具

### 7.2 规范校验
- 自定义 MCP Server 必须符合 MCP 规范（`tools/list`、`tools/call`）
- Eino 通过标准协议发现工具，无需额外适配
- 不可达的自定义 Server 降级为"工具不可用"提示
