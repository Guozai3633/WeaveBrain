# 织脑 (WeaveBrain) - 系统架构设计 (TRD)

## 1. 系统架构概述

### 1.1 分层架构

```
┌─────────────────────────────────────────────────────┐
│                 展示层 (Presentation)                 │
│   Flutter Mobile App  │  Web Admin (Phase 2)         │
└─────────────────────┬───────────────────────────────┘
                      │ WebSocket / REST API
┌─────────────────────▼───────────────────────────────┐
│                  API 层 (API Gateway)                 │
│   Gin HTTP Server (REST) │ WebSocket (/ws/voice)     │
│   中间件：鉴权、日志、限流                             │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│                Agent 层 (Agent Orchestration)         │
│   Eino Supervisor Agent + ReAct Loop                │
│   Worker Agents: Environment / Profile / Tool       │
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│                 工具层 (MCP / Tool Layer)             │
│   golang-mcp MCP Servers (Notion, Email, Calendar...)│
└─────────────────────┬───────────────────────────────┘
                      │
┌─────────────────────▼───────────────────────────────┐
│                 数据层 (Data Layer)                   │
│   PostgreSQL + pgvector │ Temporal Workflow Store    │
└─────────────────────────────────────────────────────┘
```

### 1.2 请求流（核心路径）

```
Flutter App 录音
  → WebSocket 音频流 → Go Backend /ws/voice
  → 代理到 STT 提供商 (阿里云/讯飞)
  → 流式文字返回 App 显示
  → LLM 审核文本 → Agent 处理
  → Eino Graph: Supervisor → Workers → Tool Executor
  → MCP 工具调用 (如需)
  → 结构化结果返回 App → 存入想法簿
```

## 2. 后端架构

### 2.1 Go 服务目录结构

```
WeaveBrain/
├── cmd/
│   └── weavebrain/
│       └── main.go              # 入口
├── internal/
│   ├── api/                     # HTTP handlers (REST + WebSocket)
│   ├── agent/                   # Agent 定义
│   │   ├── supervisor.go        # 主智能体
│   │   ├── environment.go       # 环境上下文智能体
│   │   ├── profile.go           # 用户画像智能体
│   │   └── tool_executor.go     # 工具智能体
│   ├── mcp/                     # MCP 工具定义
│   │   ├── notion.go            # Notion MCP 工具
│   │   ├── email.go             # 邮箱 MCP 工具
│   │   └── calendar.go          # 日历 MCP 工具
│   ├── workflow/                # Temporal 工作流
│   │   ├── idea_workflow.go     # 想法处理工作流
│   │   └── reminder_workflow.go # 提醒工作流
│   ├── db/                      # 数据库层
│   │   ├── repository/          # Repository 实现
│   │   └── migration/           # goose 迁移文件
│   └── service/                 # 业务逻辑服务层
├── pkg/                         # 共享工具包
│   ├── auth/                    # 认证工具
│   └── crypto/                  # 加密工具
└── go.mod
```

### 2.2 Eino Agent 编排

#### 2.2.1 图拓扑 (Graph Topology)

```
Graph 编排 (Eino):

┌──────────┐    ┌─────────────────────────┐    ┌──────────────┐
│ Input    │───→│ Supervisor Node          │───→│ Synthesize   │
│ Node     │    │ (LLM + ReAct Loop)       │    │ Node         │
└──────────┘    │                          │    └──────────────┘
                │ Fan-out:                 │
                │ ├─ Env Context Agent     │
                │ ├─ User Profile Agent    │
                │ └─ Tool Executor (cond.) │
                └─────────────────────────┘
```

- **Pregel 模式**：ReAct 循环（工具反复调用直到 LLM 判断完成）
- **DAG 模式**：线性 Pipeline（结构化响应生成）
- **Type Alignment**：每个节点声明 Go 结构体的输入输出类型，不滥用 `map[string]any`
- **StateHandler**：`StatePreHandler` 在每个节点执行前转换输入，`StatePostHandler` 更新全局状态

#### 2.2.2 关键配置

```go
// Supervisor Agent 配置
config := &agent.Config{
    Model:       chatModel,       // LLM 模型（MiMo/Ollama）
    Tools:       allTools,        // MCP 工具集合
    MaxIter:     10,              // 最大迭代次数
    MessageModifier: customModifier, // 注入系统提示词和当前时间
}

// ReAct 循环控制
// 推理节点: tool_choice="auto"（模型自主决定是否调用工具）
// 执行节点: tool_choice="required"（强制调用工具）
```

#### 2.2.3 消息修饰

- **MessageModifier**：每次 LLM 调用前，动态注入系统提示词和时间戳，不持久化到全局历史
- **MessageRewriter**：对话超过 5000 token 时触发上下文压缩，持久化重写后的历史

### 2.3 Temporal 工作流调度

#### 2.3.1 为什么 Temporal

- 事件驱动、持久化、Saga 支持、内置可观测性
- 替代 Cron + 重试逻辑的脆弱架构
- 进程崩溃后可精确恢复执行状态

#### 2.3.2 工作流定义

| 工作流 | 描述 | 触发方式 |
|---|---|---|
| `IdeaWorkflow` | 接收想法 → STT → Agent 处理 → 存储结果 | 用户触发 |
| `ReminderWorkflow` | 定时触发 → 检查待跟进想法 → 推送通知 | Temporal Schedule |
| `BatchAggregationWorkflow` | 夜间聚合相关想法、更新用户画像 | Temporal Schedule |

#### 2.3.3 重试与错误处理

```go
// Temporal 重试策略
policy := &workflows.RetryPolicy{
    InitialInterval:  time.Second,
    MaximumInterval:  time.Second * 10,
    MaximumAttempts:  3,
    NonRetryableErrors: []string{"USER_AUTH_ERROR"}, // 不重试的错误
}
```

- **Saga 补偿**：如果 MCP 调用在管道中途失败，补偿已成功完成的 MCP 操作
- **超时**：每个 Activity 独立超时，默认 30s
- **确定性**：工作流中不使用 `time.Now()`，改用 `workflow.Now(ctx)`

#### 2.3.4 HITL 中断恢复

```go
// 用户确认后恢复执行
err := runner.ResumeWithParams(ctx, checkpointID, interruptID, params)
```

### 2.4 golang-mcp 服务器

#### 2.4.1 架构

- **Server-First 设计**：每类工具一个 MCP Server 进程（Notion Server, Email Server, Calendar Server）
- **Schema 自动生成**：Go struct tag → JSON Schema，无需手动维护
- **传输层**：远程工具用 SSE over HTTP，本地工具用 stdio

#### 2.4.2 与 Eino 集成

```go
// 将 MCP 工具动态注入 Eino
mcpTools, err := mcpp.GetTools(ctx, mcpClient)
toolsNode, err := agent.NewToolsNode(chatModel).AddTools(mcpTools...)
```

### 2.5 数据库设计 (PostgreSQL)

#### 2.5.1 表结构

```sql
-- 用户表
CREATE TABLE users (
    id         UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    display_name VARCHAR(100),
    avatar_url TEXT,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);

-- 身份凭证表
CREATE TABLE identities (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    provider   VARCHAR(20) NOT NULL,  -- 'wechat', 'qq', 'phone'
    provider_id VARCHAR(255) NOT NULL, -- 第三方平台返回的 ID
    phone      VARCHAR(20),
    UNIQUE(user_id, provider)
);

-- 项目表
CREATE TABLE projects (
    id         BIGSERIAL PRIMARY KEY,
    user_id    UUID REFERENCES users(id) ON DELETE CASCADE,
    name       VARCHAR(100) NOT NULL,
    default_project BOOLEAN DEFAULT false,
    created_at TIMESTAMPTZ DEFAULT now(),
    updated_at TIMESTAMPTZ DEFAULT now(),
    deleted_at TIMESTAMPTZ
);
CREATE INDEX idx_projects_user ON projects(user_id, deleted_at);

-- 想法表
CREATE TABLE ideas (
    id                BIGSERIAL PRIMARY KEY,
    project_id        BIGINT REFERENCES projects(id) ON DELETE CASCADE,
    user_id           UUID REFERENCES users(id),
    raw_input         TEXT NOT NULL,
    structured_data   JSONB DEFAULT '{}',
    tags              TEXT[] DEFAULT '{}',
    created_at        TIMESTAMPTZ DEFAULT now(),
    updated_at        TIMESTAMPTZ DEFAULT now(),
    deleted_at        TIMESTAMPTZ
);
CREATE INDEX idx_ideas_project ON ideas(project_id, deleted_at);
CREATE INDEX idx_ideas_tags ON ideas USING GIN(tags);
CREATE INDEX idx_ideas_structured ON ideas USING GIN(structured_data);

-- 用户画像表
CREATE TABLE user_profiles (
    user_id       UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
    profile_data  JSONB DEFAULT '{}',
    last_updated  TIMESTAMPTZ DEFAULT now()
);

-- 提醒表
CREATE TABLE reminders (
    id           BIGSERIAL PRIMARY KEY,
    user_id      UUID REFERENCES users(id),
    project_id   BIGINT REFERENCES projects(id),
    trigger_time TIMESTAMPTZ NOT NULL,
    message      TEXT NOT NULL,
    status       VARCHAR(20) DEFAULT 'pending', -- pending, triggered, cancelled
    created_at   TIMESTAMPTZ DEFAULT now()
);
CREATE INDEX idx_reminders_trigger ON reminders(status, trigger_time);
```

#### 2.5.2 RAG 存储 (pgvector)

```sql
CREATE EXTENSION IF NOT EXISTS vector;

ALTER TABLE ideas ADD COLUMN embedding vector(1536);

-- 向量相似度索引
CREATE INDEX idx_ideas_embedding ON ideas USING ivfflat (embedding vector_cosine_ops);
```

#### 2.5.3 迁移策略

- 使用 `goose` 做数据库迁移
- 每个变更一个迁移文件，必须写 Up 和 Down
- 迁移文件存放在 `internal/db/migration/`

## 3. 移动端架构 (Flutter)

### 3.1 目录结构（功能优先）

```
WeaveFlutter/
├── lib/
│   ├── features/
│   │   ├── voice/          # 语音输入
│   │   │   ├── ui/         # VoiceButton, VoiceInputScreen
│   │   │   ├── domain/     # UseCases
│   │   │   └── data/       # Repositories
│   │   ├── ideas/          # 想法簿
│   │   ├── projects/       # 项目管理
│   │   └── settings/       # 设置页
│   ├── shared/
│   │   ├── widgets/        # 共享 Widget
│   │   └── utils/          # 工具函数
│   └── main.dart
├── test/
└── pubspec.yaml
```

### 3.2 STT 流式客户端

- 音频捕获：`audio_recorder` 包采集 PCM 数据
- WebSocket 流：`web_socket_channel` 连接到后端 `/ws/voice`
- 连接管理：指数退避自动重连
- UI 反馈：实时波形显示 + 流式文字展示

### 3.3 本地通知

- 使用 `flutter_local_notifications`
- Android notification channels, iOS categories
- 深链接到对应想法详情页

## 4. Agent 架构

### 4.1 Supervisor Agent 设计

```go
// Supervisor 节点定义
supervisorNode := agent.NewNode("supervisor", chatModel).
    WithTools(allTools...).
    WithMessageModifier(systemModifier).
    WithStateHandler(preHandler, postHandler)
```

**任务分解**：Supervisor LLM 调用产生结构化输出（指定要调用的 Worker Agent 列表）

**动态路由**：根据想法类型决定调用哪些 Worker

**结果合成**：综合所有 Worker 输出，生成最终回答

### 4.2 Worker Agents

| Agent | 职责 | 技术实现 |
|---|---|---|
| Environment Context | 确定项目，检索相关历史想法 | RAG 向量检索 + PostgreSQL 查询 |
| User Profile | 长期用户画像维护 | PostgreSQL JSONB 查询 + LLM 分析 |
| Tool Executor | MCP 工具执行 | Eino EnhancedInvokableTool + golang-mcp |

### 4.3 ReAct 循环

- **工具调用决策**：LLM 基于历史信息决定是否调用工具
- **结果注入**：工具结果作为 observation 追加到对话历史
- **终止条件**：LLM 返回"done"或达到最大迭代次数（10次）

### 4.4 HITL 中断机制

```go
// 包装为审批型工具
approvableTool := agent.InvokableApprovableTool{
    BaseTool:    notionCreatePageTool,
    Description: "在 Notion 中创建页面",
}

// 触发中断
runner.Run(ctx, graph, input) // 在 HITL 节点暂停
// ... 用户确认后 ...
runner.ResumeWithParams(ctx, checkpointID, interruptID, confirm)
```

- **Checkpoint 持久化**：图状态、局部变量序列化到 PostgreSQL
- **超时**：24 小时未响应自动拒绝

## 5. MCP 集成架构

### 5.1 工具发现

- MCP Server 启动时注册所有工具到注册表
- Eino 通过 `mcpp.GetTools()` 获取当前用户所有可用工具
- 工具动态更新无需重启（热加载）

### 5.2 SSE 通信

- 后端作为 SSE Client 连接 MCP Tool Server
- 请求格式：标准 MCP `tools/call`
- 超时：30s（可按工具配置）

### 5.3 安全隔离

- **认证**：每个工具需要 API Key（存储在 PostgreSQL，加密）
- **授权**：工具调用验证用户身份和项目范围
- **沙盒**：MCP 工具运行在独立 Docker 容器
- **审计**：每次工具调用记录日志（user_id, tool_name, input, output, timestamp）

## 6. 部署架构

### 6.1 Docker 容器

```
┌───────────────────────────────────────────────────────┐
│ Docker Network (weavebrain-network)                   │
│  ┌──────────┐ ┌──────────┐ ┌──────────┐              │
│  │ backend  │ │ mcp-     │ │ mcp-     │  ...         │
│  │ (Go)     │ │ notion   │ │ email    │ │              │
│  └──────────┘ └──────────┘ └──────────┘              │
│  ┌──────────┐ ┌──────────┐                             │
│  │ postgres │ │ temporal │                             │
│  │ (+pgvector)│ │ server  │                             │
│  └──────────┘ └──────────┘                             │
└───────────────────────────────────────────────────────┘
```

### 6.2 docker-compose.yml

```yaml
version: "3.8"
services:
  backend:
    build: ./WeaveBrain
    ports: ["8080:8080"]
    depends_on: [postgres, temporal]
    environment:
      DATABASE_URL: postgresql://weavebrain:secret@postgres:5432/weavebrain
      TEMPORAL_ADDRESS: temporal:7233

  postgres:
    image: pgvector/pgvector:pg16
    ports: ["5432:5432"]
    environment:
      POSTGRES_DB: weavebrain
      POSTGRES_PASSWORD: secret

  temporal:
    image: temporalio/auto-setup:latest
    ports: ["7233:7233"]
```

### 6.3 本地开发环境

- **无需云服务**：所有组件本地运行
- **本地 LLM**：Ollama 运行兼容模型（Qwen / Mistral）
- **Mock STT**：本地 Mock STT 服务（返回预录文本）用于测试
- **依赖工具**：Go 1.22+, Flutter 3.x, Docker, PostgreSQL 16

## 7. 技术选型决策

### 7.1 Go + Eino + Temporal

| 维度 | 选择 | 理由 |
|---|---|---|
| 语言 | Go | 静态类型、高并发(goroutine)、适合高吞吐量 API |
| Agent 编排 | Eino | 编译期类型安全、原生 Pregel+DAG 执行、优于 LangChain |
| 工作流调度 | Temporal | 持久化执行、Saga 支持、可观测性、消除自定义 Cron |

### 7.2 Flutter

- 跨平台（iOS + Android）单代码库
- 完善的音频录制 API
- Agent 代码生成生态成熟
- 后续可复用至桌面端

### 7.3 PostgreSQL

- JSONB 灵活存储（想法数据、用户画像）
- pgvector 支持 RAG 语义搜索
- ACID 合规、成熟生态
- 免费、开源、无厂商锁定

### 7.4 STT 提供商对比

| 维度 | 阿里云 ASR | 科大讯飞 |
|---|---|---|
| 成本 | 低，免费额度大 | 较高 |
| 普通话准确率 | 高（95%+） | 极高（98%+） |
| 抗噪能力 | 中等 | 强 |
| WebSocket 实时 | 支持 | 支持 |
| MVP 推荐 | 首选 | 备选 |

### 7.5 本地模型（开发阶段）

| 环境 | 方案 | 说明 |
|---|---|---|
| 开发 | Ollama + Qwen/Mistral | 全本地、零成本、OpenAI 兼容 API |
| 测试 | 真实 LLM API | CI 中调用真实模型验证 |
| 生产 | 小米 MiMo API | OpenAI 兼容端点，或直接切换到任意兼容提供商 |
