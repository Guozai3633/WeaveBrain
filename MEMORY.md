# MEMORY.md - 织脑 (WeaveBrain) 当前 Sprint 状态

## Sprint: 文档阶段
**状态**: ✅ 已完成

### 已完成
- [x] 清理旧目录结构 (`claudeConfig/`, `AgentTeams_and_Subagents_config/`, `动态开发流转/`)
- [x] 创建新目录结构 (`.claude/rules/`, `docs/topic_notes/`)
- [x] docs/PRD.md - 产品需求文档
- [x] docs/ARCHITECTURE.md - 系统架构设计
- [x] docs/MCP_INTEGRATION.md - MCP 工具规范
- [x] .env.example - 环境变量模板
- [x] .gitignore - Git 忽略规则
- [x] CLAUDE.local.md - 本地配置
- [x] .claude/rules/backend.md - 后端开发规则
- [x] .claude/rules/frontend.md - 前端开发规则
- [x] .claude/rules/database.md - 数据库开发规则
- [x] AGENTS.md - 子智能体名册
- [x] CLAUDE.md - 主指令文件

---

## Sprint 1: 后端骨架
**状态**: ✅ 已完成

### 已完成
- [x] 初始化 Go 项目 (`go mod init weavebrain`)
- [x] 创建目录结构 (`cmd/`, `internal/`, `pkg/`)
- [x] 配置 PostgreSQL 本地开发环境 (docker-compose.yml)
- [x] 配置 Temporal 本地开发环境 (docker-compose.yml)
- [x] 配置 Ollama 本地开发环境 (docker-compose.yml)
- [x] 创建初始数据库迁移 (000001_create_initial_tables.sql)
  - users, identities, projects, ideas, user_profiles, reminders
- [x] 定义实体模型 (internal/entity/model.go)
- [x] 实现 Repository 接口 (internal/db/repository/)
- [x] 实现 Service 层 (internal/service/)
- [x] 实现 API Handler (internal/api/)
- [x] 实现 HTTP Server + 路由 (internal/api/server.go)
- [x] 实现 main.go 入口 (cmd/weavebrain/main.go)
- [x] 验证代码编译通过
- [x] 启动 Docker 开发环境
- [x] 运行数据库迁移
- [x] 测试 API 端点 (健康检查成功)

### 技术栈
- Go 1.25 + Gin + pgx
- PostgreSQL 16 (标准版，后续添加 pgvector)
- Temporal (auto-setup，待启动)
- Ollama (本地 LLM，待启动)

### 阻塞项
- 无

### 待决策
- STT 提供商最终选择（阿里云 vs 讯飞）
- Flutter 状态管理最终选择（Riverpod vs Bloc）

---

## Sprint 2: Agent 集成
**状态**: ✅ 已完成

### 已完成
- [x] 集成 Eino 框架 (v0.9.7) + eino-ext OpenAI connector (v0.1.13)
- [x] 实现 LLM 客户端封装 (internal/agent/llm.go) - Ollama OpenAI 兼容
- [x] 实现 Supervisor Agent (internal/agent/supervisor.go) - react.Agent ReAct 循环
- [x] 实现 Worker Agents (internal/agent/workers.go) - 5 个内置工具
  - get_environment_context: 项目上下文查询
  - get_user_profile: 用户画像查询
  - create_idea: 创建想法
  - query_ideas: 查询想法
  - create_reminder: 创建提醒
- [x] 集成 MCP 工具注册表 (internal/mcp/registry.go) - golang-mcp v0.54.1
- [x] 实现 HITL 中断机制 (internal/agent/hitl.go) - ApprovableTool + CheckPointStore
- [x] 实现 Agent Service (internal/service/agent_service.go)
- [x] 实现 Agent API Handler (internal/api/agent_handler.go)
  - POST /api/v1/agent/process: 处理用户输入
  - GET /api/v1/agent/status: Agent 状态查询
  - GET /api/v1/agent/tools: 工具列表查询
- [x] 验证代码编译通过

### 技术决策
- 使用 Eino react.Agent 实现 ReAct 循环 (MaxStep=10)
- 使用 MessageModifier 注入系统提示词和时间戳
- MCP 工具通过 transport 层适配 (SSE/stdio)
- HITL 使用 compose.StatefulInterrupt 实现中断
- Agent 初始化为非阻塞 (go routine)，失败不影响主服务

### 阻塞项
- 无

### 待验证 (需要 Docker 环境)
- [ ] 启动 Ollama 并验证 LLM 调用
- [ ] 测试 Agent process 端点
- [ ] 连接实际 MCP 服务器

### 待决策
- STT 提供商最终选择（阿里云 vs 讯飞）
- Flutter 状态管理最终选择（Riverpod vs Bloc）

---

## Sprint 3: Temporal 工作流集成
**状态**: ✅ 已完成

### 已完成
- [x] 集成 Temporal SDK (v1.44.1)
- [x] 创建 docker-compose.yml (PostgreSQL 16 + Temporal auto-setup + Temporal UI + Ollama)
- [x] 创建 Temporal 动态配置 (config/dynamicconfig/development.yaml)
- [x] 创建数据库迁移 (000002_create_workflow_runs.sql) - workflow_runs 表
- [x] 定义 WorkflowRun 实体模型 (internal/entity/model.go)
- [x] 实现 WorkflowRunRepository 接口 + PostgreSQL 实现 (internal/db/repository/)
- [x] 实现 Temporal 客户端初始化 (internal/workflow/client.go)
- [x] 定义工作流 I/O 类型 + 重试策略 (types.go, retry.go)
- [x] 实现 3 个工作流 (internal/workflow/workflows.go)
  - IdeaWorkflow: 用户触发的想法处理
  - ReminderWorkflow: 定时提醒检查
  - BatchAggregationWorkflow: 每日想法聚合
- [x] 实现 Activity 层 (internal/workflow/activities.go) - 包装现有 Service 层
  - ProcessIdeaActivity: 调用 AgentService
  - SaveIdeaActivity: 调用 IdeaService
  - GetPendingRemindersActivity: 调用 ReminderService
  - TriggerReminderActivity: 调用 ReminderService
  - AggregateDailyIdeasActivity: 调用 IdeaService
- [x] 实现 Temporal Worker (internal/workflow/worker.go)
- [x] 实现 Workflow Dispatcher (internal/workflow/dispatcher.go)
  - DispatchIdeaProcess: 启动 IdeaWorkflow + 记录到 DB
  - DispatchReminderCheck: 启动 ReminderWorkflow
  - DispatchBatchAggregation: 启动 BatchAggregationWorkflow
  - GetWorkflowStatus: 查询工作流状态
- [x] 接入 main.go - 优雅降级 (Temporal 不可用时回退到同步模式)
- [x] 定义 service.Dispatcher 接口 (避免 import cycle)
- [x] 更新 AgentService - 添加 Dispatcher 字段 + SetDispatcher/Dispatcher 方法
- [x] 更新 agent_handler.go - 条件分发 (Temporal 用 202 Accepted，同步用 200 OK)
- [x] 新增 API 端点: GET /api/v1/agent/workflow/:id - 查询工作流状态
- [x] 更新 GET /api/v1/agent/status - 显示 Temporal 连接状态
- [x] 验证代码编译通过 (go build ./...)

### 技术决策
- 活动使用独立函数 + 包级 `SetActivities()` 注入，避免 Temporal SDK 函数注册问题
- `service.Dispatcher` 接口打破 import cycle (service 不导入 workflow)
- `workflow.Dispatcher` 通过 Go 结构化类型自动实现 `service.Dispatcher` 接口
- 所有工作流/活动 I/O 使用具体 Go 结构体，非 `map[string]any`
- Temporal 不可用时 main.go 降级运行，日志警告但不阻塞
- 重试策略: initial=1s, max=10s, backoff=2x, max_attempts=3

### 新增 API
| 方法 | 路径 | 说明 |
|------|------|------|
| POST | /api/v1/agent/process | 有 Temporal 时返回 workflow_id (202)，否则同步 (200) |
| GET | /api/v1/agent/workflow/:id | 查询工作流执行状态 |
| GET | /api/v1/agent/status | 新增 temporal_connected 字段 |

### 阻塞项
- 无

### 待验证 (需要 Docker 环境)
- [ ] `docker compose up -d` 启动全部服务
- [ ] 运行数据库迁移 000002
- [ ] 测试 Temporal 工作流端到端执行
- [ ] 访问 Temporal Web UI (http://localhost:8088)

### 待决策
- STT 提供商最终选择（阿里云 vs 讯飞）
- Flutter 状态管理最终选择（Riverpod vs Bloc）

---

## Sprint 4: 语音管道 + Worker 工具接线
**状态**: ✅ 已完成

### 已完成

#### Part A: 语音 WebSocket 管道
- [x] 添加 gorilla/websocket 依赖 (v1.5.3)
- [x] 定义 STT Provider 接口 (internal/stt/provider.go)
  - `STTProvider.StreamRecognize(ctx, audioStream) (<-chan TranscriptionResult, error)`
  - `TranscriptionResult { Text, IsFinal, Confidence }`
- [x] 实现 Mock STT Provider (internal/stt/mock/provider.go)
  - 可配置脚本响应，逐字返回 partial→final 结果
  - 用于开发阶段无需阿里云凭证
- [x] 实现 STT Service (internal/service/stt_service.go)
  - 包装 STTProvider，提供 `RecognizeStream()` 委托调用
- [x] 实现 Voice WebSocket Handler (internal/api/voice_ws.go)
  - gorilla/websocket 协议升级
  - 客户端→服务端: binary=PCM 音频 / text=JSON 控制消息 (start/stop)
  - 服务端→客户端: text=JSON 转写结果 (partial/final + confidence)
  - 会话生命周期管理: start→音频泵送→stop/断开→清理
- [x] 接入 main.go - Mock STT Provider 初始化
- [x] 更新 server.go - 条件注册 WebSocket 路由 (STT 可用时)
- [x] 移除 user_handler.go 中的占位符

#### Part B: Worker 工具接线
- [x] 定义 5 个 Provider 接口 (internal/agent/workers.go)
  - `EnvironmentProvider`: 获取项目上下文
  - `UserProfileProvider`: 获取用户画像
  - `IdeaCreator`: 创建想法
  - `IdeaQuerier`: 查询想法
  - `ReminderCreator`: 创建提醒
- [x] 重构所有 5 个工具 - 接受 Provider 依赖注入
  - 移除所有 `// TODO: Replace with actual service layer calls`
  - 每个工具的 InvokableRun 调用 Provider 接口
- [x] 实现 5 个 Service 适配器 (internal/service/tool_adapters.go)
  - `EnvironmentAdapter`: 包装 ProjectService + IdeaService + UserService
  - `UserProfileAdapter`: 包装 UserService + UserProfileService
  - `CreateIdeaAdapter`: 包装 IdeaService
  - `QueryIdeasAdapter`: 包装 IdeaService (ListByProject + SearchByTags)
  - `ReminderAdapter`: 包装 ReminderService
  - 所有适配器处理 UUID/int64 类型转换
- [x] 更新 AgentService - 接受 Provider 依赖
  - `NewAgentService(env, profile, ideaCreate, ideaQuery, reminder)` 替代 `NewAgentService()`
  - `Init()` 使用 `agent.NewTools(...)` 替代 `agent.DefaultTools()`
- [x] 更新 service.go - 创建适配器并注入到 AgentService

### 技术决策
- 使用接口打破 `agent` ↔ `service` import cycle
- Provider 接口使用纯数据类型 (agent 包内的 I/O struct)，不依赖 `entity` 或 `service`
- 适配器模式: service/tool_adapters.go 实现 agent 包定义的接口
- STT 使用 Provider 模式，支持后续替换为阿里云/讯飞实现
- WebSocket 协议: binary=音频, text=JSON 控制/结果
- Mock STT 逐字返回结果，中文按字符分割

### 新增/修改文件
| 操作 | 文件 | 说明 |
|------|------|------|
| 新增 | internal/stt/provider.go | STT Provider 接口 |
| 新增 | internal/stt/mock/provider.go | Mock STT 实现 |
| 新增 | internal/service/stt_service.go | STT 服务 |
| 新增 | internal/api/voice_ws.go | Voice WebSocket Handler |
| 新增 | internal/service/tool_adapters.go | 5 个 Service→Agent 适配器 |
| 修改 | internal/agent/workers.go | 接口定义 + 依赖注入重构 |
| 修改 | internal/service/agent_service.go | 接受 Provider 参数 |
| 修改 | internal/service/service.go | 创建适配器并注入 |
| 修改 | cmd/weavebrain/main.go | STT 初始化 + 关闭 |
| 修改 | internal/api/server.go | 条件注册 WebSocket 路由 |
| 修改 | internal/api/user_handler.go | 移除占位符 |
| 修改 | go.mod / go.sum | 添加 gorilla/websocket v1.5.3 |

### 阻塞项
- 无

### 待验证 (需要 Docker 环境)
- [ ] WebSocket 连接测试: `ws://localhost:8080/ws/voice`
- [ ] Agent 工具调用返回真实数据库数据
- [ ] Mock STT 音频→转写端到端

### 待决策
- STT 提供商最终选择（阿里云 vs 讯飞）
- Flutter 状态管理最终选择（Riverpod vs Bloc）

---

## Sprint 5: JWT 认证 + 路由保护
**状态**: ✅ 已完成

### 已完成
- [x] 集成 golang-jwt/jwt/v5 (v5.3.1)
- [x] 创建 JWT Token 服务 (pkg/auth/jwt.go)
  - TokenConfig: Secret, Issuer, Expiry
  - Claims: UserID + jwt.RegisteredClaims
  - GenerateToken: HS256 签名
  - ValidateToken: 解析 + 校验
- [x] 创建认证中间件 (pkg/auth/middleware.go)
  - JWTMiddleware: Authorization Bearer token 校验
  - WSJWTMiddleware: WebSocket ?token= query param 校验
  - extractBearerToken 辅助函数
- [x] 重组路由结构 (internal/api/server.go)
  - 公开路由: /auth/register, /auth/login, /health
  - 受保护路由: /users, /projects, /ideas, /reminders, /agent (JWT required)
  - WebSocket: /ws/voice (JWT via query param)
- [x] 实现真实登录端点 (internal/api/auth_handler.go)
  - POST /api/v1/auth/register: 注册 + 返回 JWT token
  - POST /api/v1/auth/login: 通过 provider+provider_id 登录 + 返回 JWT
  - LoginRequest 结构体
- [x] 统一上下文键 (internal/api/context.go)
  - 使用 auth.ContextKeyUserID 常量替代硬编码字符串
- [x] 所有权校验 — Project handler
  - handleGetProject: 校验 project.UserID == currentUserID
  - handleUpdateProject: 校验所有权
  - handleDeleteProject: 校验所有权
- [x] 所有权校验 — Idea handler
  - handleGetIdea: 校验 idea.UserID == currentUserID
  - handleUpdateIdea: 校验所有权
  - handleDeleteIdea: 校验所有权
  - handleListIdeas: 添加 getCurrentUserID 检查
- [x] Reminder handler 补全
  - handleCreateReminder: 移除 user_id 请求体，改从 JWT 获取
  - handleListReminders: 添加 getCurrentUserID 检查
  - handleGetReminder: 从 stub 实现为完整逻辑 + 所有权校验
  - handleUpdateReminderStatus: 实现 triggered/cancelled 状态切换 + 所有权校验
  - handleDeleteReminder: 实现删除 + 所有权校验
- [x] WebSocket 认证 (internal/api/voice_ws.go)
  - VoiceWSHandler 新增 tokenCfg 字段
  - RegisterRoute 使用 WSJWTMiddleware 保护
- [x] 环境变量: JWT_SECRET (默认 dev-secret-change-in-production)
- [x] 验证代码编译通过

### 技术决策
- JWT HS256 签名，72 小时过期
- Authorization: Bearer <token> 用于 HTTP，?token=<jwt> 用于 WebSocket
- 所有权校验在 handler 层执行（获取实体后比对 UserID）
- 公开/受保护路由通过 gin.RouterGroup + Use() 中间件实现
- auth 包独立于 internal/api，可复用

### 新增/修改文件
| 操作 | 文件 | 说明 |
|------|------|------|
| 新增 | pkg/auth/jwt.go | JWT TokenConfig, Claims, GenerateToken, ValidateToken |
| 新增 | pkg/auth/middleware.go | JWTMiddleware, WSJWTMiddleware, extractBearerToken |
| 修改 | go.mod / go.sum | 添加 golang-jwt/jwt/v5 v5.3.1 |
| 修改 | cmd/weavebrain/main.go | JWT_SECRET env + TokenConfig 传入 NewServer |
| 修改 | internal/api/server.go | NewServer 签名 + 公开/受控路由分组 |
| 修改 | internal/api/context.go | 使用 auth.ContextKeyUserID 常量 |
| 修改 | internal/api/auth_handler.go | 注册返回 JWT + 真实登录逻辑 |
| 修改 | internal/api/project_handler.go | Get/Update/Delete 添加所有权校验 |
| 修改 | internal/api/idea_handler.go | Get/Update/Delete/List 添加所有权校验 |
| 修改 | internal/api/reminder_handler.go | 移除 user_id 请求体 + 补全 3 个 stub |
| 修改 | internal/api/voice_ws.go | 添加 WSJWTMiddleware |

### 阻塞项
- 无

### 待验证 (需要 Docker 环境)
- [ ] 注册/登录返回有效 JWT token
- [ ] 无 token 访问受保护路由返回 401
- [ ] 所有权校验: 跨用户访问返回 403
- [ ] WebSocket ?token= 认证正常
- [ ] Reminder CRUD 端到端测试

### 待决策
- STT 提供商最终选择（阿里云 vs 讯飞）

---

## Sprint 6: Flutter 基础 + 语音输入 MVP
**状态**: ✅ 已完成

### 已完成
- [x] 创建 Flutter 项目 (`weave_flutter`, org: com.weavebrain)
- [x] 配置 pubspec.yaml 依赖
  - flutter_riverpod v2.6.1 (状态管理)
  - go_router v14.8.1 (声明式路由)
  - dio v5.9.2 (HTTP 客户端)
  - flutter_secure_storage v9.2.4 (Token 持久化)
  - web_socket_channel v3.0.3 (WebSocket 语音流)
  - json_serializable + freezed (代码生成)
- [x] 实现共享层
  - ApiClient: Dio + JWT 拦截器 + 401 自动登出
  - AuthRepository: flutter_secure_storage token/user 读写
  - AuthNotifier: Riverpod 状态机 (AuthInitial/Authenticated/Unauthenticated)
  - 4 个数据模型: User, Project, Idea, Reminder (json_serializable)
- [x] 实现认证模块
  - LoginScreen: provider + provider_id 输入，MVP 默认 test/test-user-001
  - RegisterScreen: provider + provider_id + display_name
  - GoRouter redirect: 未登录→/login，已登录+/login→/
- [x] 实现语音模块
  - VoiceRepository: WebSocket 连接 + start/stop 控制 + binary 音频发送
  - VoiceNotifier: 状态机 (Idle→Connecting→Recording→Processing→Result)
  - VoiceScreen: 项目选择器 + 大麦克风按钮 + 流式文字显示
  - VoiceButton: 独立 Widget，idle/recording/processing 三态 + 脉冲动画
- [x] 实现想法列表模块
  - IdeaApi: listIdeas + getIdea
  - IdeasNotifier: 分页加载 + 下拉刷新
  - IdeasScreen: 列表 + 空状态引导
  - IdeaCard: 展开/折叠 + tags + structured_data
- [x] 实现路由 + 导航
  - StatefulShellRoute 底部 3 tab: 语音/想法簿/设置
  - AppScaffold: NavigationBar + NavigationDestination
  - GoRouter redirect: 认证守卫
- [x] 实现设置页 (SettingsScreen: 关于 + 退出登录)
- [x] 更新 main.dart (ProviderScope + MaterialApp.router + 主题)
- [x] 更新 widget_test.dart
- [x] dart analyze 通过 (0 error, 0 warning, 1 info)

### 技术决策
- Riverpod 状态管理（最终选择，替代 Bloc）
- go_router StatefulShellRoute 实现底部导航 + 嵌套导航
- Dio interceptor 自动注入 JWT token
- 401 响应自动清除 token 并跳转登录
- WebSocket 认证使用 ?token= query param（与后端 WSJWTMiddleware 对齐）
- Voice 状态使用 Dart 3 sealed class 模式匹配
- Android emulator 使用 10.0.2.2 访问宿主机 localhost

### 新增文件
| 目录 | 文件 | 说明 |
|------|------|------|
| lib/ | main.dart, app.dart | 入口 + GoRouter 配置 |
| shared/api/ | api_client.dart, api_exception.dart | Dio + JWT 拦截器 |
| shared/auth/ | auth_repository.dart, auth_state.dart | Token 持久化 + 认证状态机 |
| shared/models/ | user.dart, project.dart, idea.dart, reminder.dart | 数据模型 |
| shared/widgets/ | app_scaffold.dart, loading_indicator.dart | 底部导航壳 + loading |
| features/auth/ui/ | login_screen.dart, register_screen.dart | 登录/注册页 |
| features/voice/data/ | voice_repository.dart | WebSocket 管理 |
| features/voice/domain/ | voice_notifier.dart | 语音状态机 |
| features/voice/ui/ | voice_screen.dart, voice_button.dart | 语音页 + 麦克风按钮 |
| features/ideas/data/ | idea_api.dart | 想法 API |
| features/ideas/domain/ | ideas_notifier.dart | 想法列表状态 |
| features/ideas/ui/ | ideas_screen.dart, idea_card.dart | 想法列表 + 卡片 |
| features/settings/ui/ | settings_screen.dart | 设置页 stub |

### 阻塞项
- 无

### 待决策
- 音频采集方案优化 (当前使用 record 包录制 WAV 文件 + 定时分块发送，后续可改为原生 PCM 流)

---

## Sprint 7: 核心循环闭环
**状态**: ✅ 已完成

### 已完成

#### Part A: 后端 Agent 结构化输出
- [x] 定义 IdeaResult 结构体 (internal/agent/result.go)
  - Tags, BaseInput, BelongProject, AiMeanEnv, Feasibility, Suggestions, Reply
  - ParseIdeaResult(): 从 LLM 输出中提取 ```json``` 代码块并解析
  - EmptyIdeaResult(): 兜底默认值
- [x] 更新系统提示词 (internal/agent/supervisor.go)
  - 追加 JSON 输出格式要求（tags, feasibility, suggestions, reply 等字段）
- [x] 更新 AgentService.ProcessInput (internal/service/agent_service.go)
  - 返回类型: `(string, error)` → `(agent.IdeaResult, error)`
  - 新增 projectID 参数
  - 使用 ParseIdeaResult 解析 LLM 输出，失败时兜底
- [x] 更新 ProcessInputResponse (internal/api/agent_handler.go)
  - 新增字段: Tags, Feasibility, Suggestions, BaseInput
  - 同步分支从 IdeaResult 填充所有字段
- [x] 更新 ProcessIdeaActivity (internal/workflow/activities.go)
  - StructuredData 从 fake `{"agent_response": response}` 改为真实结构化数据
  - Tags 从 nil 改为 result.Tags
- [x] 更新 IdeaProcessOutput (internal/workflow/types.go)
  - 新增字段: Tags, Feasibility, Suggestions
- [x] 更新 IdeaWorkflow (internal/workflow/workflows.go)
  - 从 agentResult.StructuredData 提取 feasibility 和 suggestions 传递到输出

#### Part B: Flutter 音频采集
- [x] 添加 record v7.1.0 + path_provider 依赖
- [x] 新增 AgentResponse 模型 (shared/models/agent_response.dart)
  - 解析 API 响应: response, tags, feasibility, suggestions, baseInput
- [x] 重写 VoiceNotifier (features/voice/domain/voice_notifier.dart)
  - 集成 AudioRecorder 录音到临时 WAV 文件
  - 麦克风权限检查
  - 每 500ms 读取文件并通过 WebSocket 发送音频块
  - VoiceResult 持有 AgentResponse 替代 Map<String, dynamic>
  - 录音结束后清理临时文件

#### Part C: Flutter 结构化结果展示
- [x] 重写 VoiceScreen (features/voice/ui/voice_screen.dart)
  - 改为 ConsumerStatefulWidget 支持项目选择器状态
  - 转写结果 → 标签 (Wrap + Chip) → 可行性指示器 (圆点) → 建议行动列表 → Agent 回复
  - 颜色编码: high=绿, medium=橙, low=红
  - "确认保存" + "重新录制" 双按钮

#### Part D: Flutter 项目选择器
- [x] 新增 ProjectApi (features/voice/data/project_api.dart)
  - listProjects(): GET /projects
- [x] 新增 ProjectSelector Widget (shared/widgets/project_selector.dart)
  - DropdownButton 样式，默认选中 defaultProject
  - Riverpod FutureProvider 获取项目列表
  - 加载/错误/空状态处理
- [x] VoiceScreen 集成 ProjectSelector (替换硬编码"默认项目")
- [x] IdeasScreen 集成 ProjectSelector (顶部项目筛选)

#### Part E: Flutter 想法详情页
- [x] 新增 IdeaDetailScreen (features/ideas/ui/idea_detail_screen.dart)
  - 原始输入 + 标签 (Chip) + 可行性评估 (圆点指示器) + AI 语义分析 + 建议行动列表
  - 原始结构化数据 (ExpansionTile) + 创建时间
- [x] 注册路由 /ideas/:id (app.dart)，通过 state.extra 传递 Idea 对象
- [x] IdeaCard 简化为 StatelessWidget，点击跳转 IdeaDetailScreen

### 技术决策
- Agent 结构化输出通过系统提示词指令 + 正则提取 JSON 代码块实现
- ParseIdeaResult 支持 ```json ... ``` 代码块和裸 JSON 两种格式
- 音频采集使用 record 包录制到临时文件 + Timer 定时分块发送（MVP 方案）
- 项目选择器使用 Riverpod FutureProvider 全局缓存，VoiceScreen 和 IdeasScreen 共享
- IdeaDetailScreen 通过 GoRouter extra 参数接收 Idea 对象，避免重复 API 调用

### 新增/修改文件

#### Go 后端 (1 新建, 5 修改)
| 操作 | 文件 | 说明 |
|------|------|------|
| 新建 | internal/agent/result.go | IdeaResult + ParseIdeaResult + EmptyIdeaResult |
| 修改 | internal/agent/supervisor.go | 系统提示词追加 JSON 输出格式 |
| 修改 | internal/service/agent_service.go | ProcessInput 返回 IdeaResult + 新签名 |
| 修改 | internal/api/agent_handler.go | ProcessInputResponse 扩展 + 同步分支更新 |
| 修改 | internal/workflow/activities.go | ProcessIdeaActivity 使用 IdeaResult |
| 修改 | internal/workflow/types.go + workflows.go | IdeaProcessOutput 扩展 |

#### Flutter (4 新建, 6 修改)
| 操作 | 文件 | 说明 |
|------|------|------|
| 新建 | shared/models/agent_response.dart | Agent 响应模型 |
| 新建 | shared/widgets/project_selector.dart | 项目选择器组件 |
| 新建 | features/voice/data/project_api.dart | Projects API 客户端 |
| 新建 | features/ideas/ui/idea_detail_screen.dart | 想法详情页 |
| 修改 | pubspec.yaml | 添加 record + path_provider |
| 修改 | features/voice/domain/voice_notifier.dart | 音频采集 + AgentResponse |
| 修改 | features/voice/ui/voice_screen.dart | 结构化卡片 + 项目选择器 |
| 修改 | features/ideas/ui/idea_card.dart | 点击跳转详情页 |
| 修改 | features/ideas/ui/ideas_screen.dart | 项目筛选下拉 |
| 修改 | app.dart | 注册 /ideas/:id 路由 |

### 阻塞项
- 无

### 待验证 (需要 Docker 环境)
- [ ] `docker compose up -d` 启动全部服务
- [ ] 端到端: 录音 → Agent 结构化输出 → 结果卡片展示
- [ ] 项目选择器加载真实项目列表
- [ ] 想法详情页展示完整结构化数据

### 待决策
- 音频采集方案优化 (当前 record 包定时发送文件块，可改为原生 PCM 流)
- "确认保存"按钮行为（当前重置状态，后续应调用 Idea API 创建）

---

## Sprint 8: 确认保存 + FunASR 本地语音 + 项目管理 + 想法搜索
**状态**: ✅ 已完成

### 已完成

#### Part A: "确认保存" 接线
- [x] ApiClient.post 新增 queryParams 支持
- [x] IdeaApi 新增 createIdea() 方法 (POST /ideas?project_id=X)
- [x] VoiceNotifier 新增 VoiceSaving/VoiceSaved 状态 + saveIdea() 方法
- [x] VoiceScreen "确认保存" 按钮接线 (保存中 spinner → 成功提示 → 自动 reset)

#### Part B: FunASR 本地语音识别
- [x] docker-compose.yml 添加 FunASR 服务 (Paraformer-online 流式中文 ASR)
- [x] 实现 FunASR WebSocket STT Provider (internal/stt/funasr/provider.go)
  - 首帧 JSON 配置 + binary PCM 流 + is_speaking=false 结束
  - 2pass-online=partial, 2pass-offline=final
- [x] Config 驱动 STT Provider 选择 (STT_PROVIDER=mock|funasr)
- [x] .env.example 更新 (STT_PROVIDER + FUNASR_ADDR)
- [x] Flutter PCM 增量发送 (AudioEncoder.pcm16bits + _lastSentOffset 增量)

#### Part C: Flutter 项目管理
- [x] ProjectApi 扩展: createProject, updateProject, deleteProject
- [x] ProjectNotifier (Riverpod Notifier: load/create/update/delete)
- [x] ProjectListScreen (ListView + FAB + PopupMenu: 编辑/设为默认/删除)
- [x] ProjectFormDialog (name + defaultProject 双模式对话框)
- [x] SettingsScreen 添加 "项目管理" 入口
- [x] app.dart 注册 /projects 路由

#### Part D: 想法搜索/筛选
- [x] 后端: IdeaRepository.Search 接口 + 动态 SQL (ILIKE + tags &&)
- [x] 后端: IdeaService.Search 方法
- [x] 后端: handleListIdeas 解析 search/tags 查询参数
- [x] Flutter: IdeaApi.listIdeas 新增 search/tags 可选参数
- [x] Flutter: IdeasNotifier 搜索支持 (search/searchByTag/clearSearch)
- [x] Flutter: IdeasScreen SearchBar (300ms debounce) + 标签筛选 Chip
- [x] Flutter: IdeaCard 标签改为 ActionChip，点击触发标签搜索

### 技术决策
- FunASR Paraformer-online: 免费本地部署的流式中文 ASR，替代 Mock STT
- PCM 16-bit 原始音频直接流式发送，避免 WAV header 处理
- 增量发送: _lastSentOffset 跟踪已发送位置，每次只发送新增部分
- 动态 SQL 使用参数化查询 ($1, $2...) 防止 SQL 注入
- SearchBar 使用 Timer 实现 300ms debounce 避免频繁请求

### 新增/修改文件

#### Go 后端 (1 新建, 5 修改)
| 操作 | 文件 | 说明 |
|------|------|------|
| 新建 | internal/stt/funasr/provider.go | FunASR WebSocket STT Provider |
| 修改 | cmd/weavebrain/main.go | Config 驱动 STT provider 选择 |
| 修改 | docker-compose.yml | 添加 FunASR 服务 |
| 修改 | .env.example | STT_PROVIDER + FUNASR_ADDR |
| 修改 | internal/db/repository/interface.go | IdeaRepository.Search 接口 |
| 修改 | internal/db/repository/idea_repository.go | Search 动态 SQL 实现 |
| 修改 | internal/service/idea_service.go | Search 方法 |
| 修改 | internal/api/idea_handler.go | search/tags 查询参数解析 |

#### Flutter (3 新建, 8 修改)
| 操作 | 文件 | 说明 |
|------|------|------|
| 新建 | features/projects/domain/project_notifier.dart | 项目管理状态机 |
| 新建 | features/projects/ui/project_list_screen.dart | 项目列表页 |
| 新建 | features/projects/ui/project_form_dialog.dart | 项目表单对话框 |
| 修改 | shared/api/api_client.dart | post() 添加 queryParams |
| 修改 | features/ideas/data/idea_api.dart | createIdea + search/tags 参数 |
| 修改 | features/voice/domain/voice_notifier.dart | saveIdea + PCM 增量发送 |
| 修改 | features/voice/ui/voice_screen.dart | 确认保存接线 + 新状态 UI |
| 修改 | features/ideas/domain/ideas_notifier.dart | 搜索方法 |
| 修改 | features/ideas/ui/ideas_screen.dart | SearchBar + 标签筛选 |
| 修改 | features/ideas/ui/idea_card.dart | ActionChip + onTagTapped |
| 修改 | features/voice/data/project_api.dart | CRUD 方法 |
| 修改 | features/settings/ui/settings_screen.dart | 项目管理入口 |
| 修改 | app.dart | /projects 路由 |

### 阻塞项
- 无

### 待验证 (需要 Docker 环境)
- [ ] FunASR 容器启动 + 首次模型下载 (~1.5GB)
- [ ] STT_PROVIDER=funasr 端到端语音转写

### 已验证 (E2E 2026-06-16)
- [x] 注册/登录 + JWT 生成
- [x] WebSocket /ws/voice 连接 + mock STT partial→final 流式返回
- [x] 想法 CRUD: POST/GET/GET:id 正常
- [x] 想法搜索: text search (FunASR/AI) + tag search (FunASR/AI) 正常
- [x] 项目管理: create/update/delete + list 正常
- [x] Bug 修复: user_service.go nil pointer (identity.CreatedAt → time.Now())

### 待决策
- 无

---

## Sprint 9: 设置页补全 (个人信息 + STT 显示 + API 配置占位)
**状态**: ✅ 已完成

### 已完成

#### Part A: 后端 — 用户 API 增强
- [x] 传递 STT provider 名称到 Server (server.go sttProvider 字段 + NewServer 参数)
- [x] 新增 `GET /api/v1/config/stt` 端点 (protected)，返回 `{"provider": "mock"}`
- [x] 新增 `handleGetMe` — `GET /users/me`，从 JWT 取当前用户
- [x] `handleUpdateUserProfile` 添加所有权校验 (currentUserID != userID → 403)
- [x] `handleUpdateUserProfile` 成功后返回更新后的 User 对象
- [x] 路由注册: `/me` 在 `/:id` 前面 (Gin 优先匹配字面路径)
- [x] main.go 更新 `api.NewServer()` 传入 `sttProviderName`

#### Part B: Flutter 数据层
- [x] User 模型扩展: 新增 `updatedAt` 字段 + `copyWith()` 方法
- [x] build_runner 重新生成 `user.g.dart`
- [x] AuthNotifier 新增 `updateUser(User)` 方法 (更新 Riverpod state + 持久化)
- [x] 新建 `UserApi`: `getMe()` + `updateProfile()`

#### Part C: Flutter UI
- [x] 新建 `ProfileEditDialog`: AlertDialog + TextField (昵称编辑)
- [x] 重写 `SettingsScreen`:
  - 个人信息区 (CircleAvatar + 昵称 + 编辑按钮)
  - 语音识别引擎 (只读，FutureProvider 显示 STT provider)
  - API 配置 (占位对话框 "即将支持")
  - 项目管理 / 关于 / 退出登录 (保留)

### 技术决策
- `/me` 路由在 `/:id` 之前注册 (Gin first-match 语义)
- `apiClientProvider` 在 `auth_state.dart` 中定义，settings_screen 通过 auth_state 导入使用
- STT 配置使用 `FutureProvider` 缓存 (自动获取一次，后续使用缓存)
- `updateUser()` 同时更新 Riverpod state 和 AuthRepository 持久化

### 新增/修改文件
| 操作 | 文件 | 说明 |
|------|------|------|
| 修改 | `WeaveBrain/cmd/weavebrain/main.go` | 传递 sttProviderName 到 NewServer |
| 修改 | `WeaveBrain/internal/api/server.go` | sttProvider 字段 + /config/stt 端点 |
| 修改 | `WeaveBrain/internal/api/user_handler.go` | /me + 所有权校验 + 返回 User |
| 修改 | `weave_flutter/lib/shared/models/user.dart` | updatedAt + copyWith |
| 重新生成 | `weave_flutter/lib/shared/models/user.g.dart` | build_runner |
| 修改 | `weave_flutter/lib/shared/auth/auth_state.dart` | updateUser() |
| 新建 | `weave_flutter/lib/features/settings/data/user_api.dart` | UserApi |
| 新建 | `weave_flutter/lib/features/settings/ui/profile_edit_dialog.dart` | 编辑对话框 |
| 修改 | `weave_flutter/lib/features/settings/ui/settings_screen.dart` | 全面重写 |

**总计: 6 修改 + 2 新建 + 1 重新生成 = 9 文件**

### 阻塞项
- 无

### 待验证 (需要 Docker 环境)
- [ ] `GET /api/v1/users/me` (带 JWT) → 返回当前用户
- [ ] `PUT /api/v1/users/:id/profile` (带 JWT) → 更新成功 + 返回 User
- [ ] `PUT /api/v1/users/:other_id/profile` (带 JWT) → 403 Forbidden
- [ ] `GET /api/v1/config/stt` → `{"provider":"mock"}`
- [ ] Flutter: 设置页显示个人信息 + STT 引擎名 + API 配置占位
- [ ] Flutter: 编辑昵称 → 保存 → 切 tab 回来昵称已更新

### 待决策
- 无

---

## Sprint 10: RAG 语义搜索 (pgvector + Ollama Embedding)
**状态**: ✅ 已完成

### 已完成

#### Part A: 基础设施
- [x] docker-compose.yml 切换 `pgvector/pgvector:pg16` 镜像
- [x] 创建数据库迁移 (000003_add_pgvector_embeddings.sql)
  - `CREATE EXTENSION vector`
  - `ALTER TABLE ideas ADD COLUMN embedding vector(768)`
  - HNSW 索引 (vector_cosine_ops)
- [x] 添加 `github.com/pgvector/pgvector-go` v0.4.0 依赖
- [x] 更新 .env.example (EMBEDDING_BASE_URL, EMBEDDING_API_KEY, EMBEDDING_MODEL, EMBEDDING_DIM)

#### Part B: Embedding Provider 包
- [x] 定义 Provider 接口 (internal/embedding/provider.go)
  - `Embed(ctx, text) ([]float32, error)` + `Dimension() int` + `Close() error`
- [x] 实现 Ollama Provider (internal/embedding/ollama/provider.go)
  - 调用 Ollama OpenAI 兼容端点 `POST /v1/embeddings`
  - 30s 超时，net/http + encoding/json
- [x] 实现 Mock Provider (internal/embedding/mock/provider.go)
  - 确定性伪向量 (基于 text 长度 sin 函数)

#### Part C: Entity + Repository 层
- [x] Idea 实体添加 `Embedding *pgvector.Vector` 字段 (`json:"-"`)
- [x] IdeaRepository 接口扩展 3 个方法:
  - `SearchBySimilarity(projectID, embedding, limit)` — 余弦距离排序
  - `UpdateEmbedding(ideaID, embedding)` — 更新向量
  - `GetWithoutEmbedding(limit)` — 获取未向量化的想法
- [x] PostgreSQL 实现: `<=>` 余弦距离操作符 + `pgvector.NewVector()`

#### Part D: Embedding Service 层
- [x] 创建 EmbeddingService (internal/service/embedding_service.go)
  - `GenerateAndStore`: fire-and-forget embedding 生成
  - `SearchSimilar`: 语义搜索
  - `BackfillEmbeddings`: 批量补全历史 embedding
- [x] Services 结构体添加 `Embedding *EmbeddingService` 字段
- [x] IdeaService 添加 `SetEmbeddingService()` + Create() 异步触发

#### Part E: Agent 层集成
- [x] 更新 EnvironmentProvider 接口签名 (新增 projectID, query 参数)
- [x] EnvironmentContextOutput 新增 `SemanticIdeas []IdeaSummary` 字段
- [x] EnvironmentAdapter 添加 `SetEmbeddingService()` + 语义搜索逻辑
- [x] AgentService 添加 `SetEmbeddingService()` 方法
- [x] 系统提示词更新: 追加语义搜索使用说明

#### Part F: main.go 接线
- [x] 初始化 Ollama Embedding Provider
- [x] 创建 EmbeddingService 并注入到 IdeaService 和 AgentService
- [x] 新增 `backfill-embeddings` 子命令
- [x] 添加 embedding provider 优雅关闭

### 技术决策
- 768 维向量 = nomic-embed-text 模型 (Ollama 本地)
- HNSW 索引优于 IVFFlat (无需训练，适合 <100k 行)
- Fire-and-forget 模式: `go s.embedding.GenerateAndStore(context.Background(), ...)` — 使用 Background context
- 优雅降级: embedding provider 不可用时，想法创建不受影响，Agent 降级到仅返回最近想法
- backfill 命令: `go run ./cmd/weavebrain backfill-embeddings` — 循环处理直到全部完成

### 新增/修改文件

#### Go 后端 (6 新建, 10 修改)
| 操作 | 文件 | 说明 |
|------|------|------|
| 新建 | `internal/embedding/provider.go` | Provider 接口 |
| 新建 | `internal/embedding/ollama/provider.go` | Ollama embedding 实现 |
| 新建 | `internal/embedding/mock/provider.go` | Mock embedding |
| 新建 | `internal/service/embedding_service.go` | EmbeddingService |
| 新建 | `internal/db/migration/000003_add_pgvector_embeddings.sql` | pgvector 迁移 |
| 修改 | `docker-compose.yml` | pgvector/pgvector:pg16 镜像 |
| 修改 | `go.mod` / `go.sum` | pgvector-go v0.4.0 |
| 修改 | `.env.example` | EMBEDDING_* 变量 |
| 修改 | `internal/entity/model.go` | Idea.Embedding 字段 |
| 修改 | `internal/db/repository/interface.go` | 3 个新方法 |
| 修改 | `internal/db/repository/idea_repository.go` | 3 个方法实现 |
| 修改 | `internal/service/service.go` | Services.Embedding 字段 |
| 修改 | `internal/service/idea_service.go` | 异步 embedding 触发 |
| 修改 | `internal/agent/workers.go` | 接口签名 + SemanticIdeas |
| 修改 | `internal/service/tool_adapters.go` | 语义搜索路径 |
| 修改 | `internal/service/agent_service.go` | SetEmbeddingService |
| 修改 | `internal/agent/supervisor.go` | 系统提示词 |
| 修改 | `cmd/weavebrain/main.go` | 接线 + backfill 命令 |

**总计: 6 新建 + 12 修改 = 18 文件**

### 验证
- [x] `go build ./...` 通过
- [x] `go vet ./...` 通过

### 阻塞项
- 无

### 待验证 (需要 Docker 环境)
- [ ] Docker 重建 postgres (pgvector 镜像) + 运行迁移 000003
- [ ] `ollama pull nomic-embed-text` — 下载 274MB embedding 模型
- [ ] `go run ./cmd/weavebrain backfill-embeddings` — 为历史想法生成 embedding
- [ ] 创建两条想法: "机器学习模型训练优化" vs "今天天气不错"
- [ ] 调用 `POST /api/v1/agent/process` 输入 "深度学习性能提升"
- [ ] 验证 Agent 返回的 semantic_ideas 包含 ML 相关想法
- [ ] 停止 Ollama → 创建新想法仍成功 (embedding 静默失败) → Agent 降级到仅返回最近想法

### 待决策
- 无

---

## Sprint 11: 基础补全 (Audit Log + AI Review + Cron Schedules + Rate Limiting)
**状态**: ✅ 已完成

### 已完成

#### R16: MCP 审计日志
- [x] 创建数据库迁移 (000004_create_mcp_audit_logs.sql)
  - mcp_audit_logs 表: user_id, tool_name, server_name, input, output, duration_ms, success, error_msg
  - 复合索引: (user_id, created_at), (tool_name, created_at)
- [x] MCPAuditLog 实体模型 (internal/entity/model.go)
- [x] MCPAuditLogRepository 接口 + PostgreSQL 实现 (internal/db/repository/)
- [x] Integrate MCPAuditLog into DBStore (store.go NewFromPool + BeginTx)
- [x] AuditTool 装饰器 (internal/agent/audit.go)
  - AuditLogger 接口 + AuditEvent 结构体 (避免 import cycle)
  - AuditTool 包装 InvokableTool，计时执行 + fire-and-forget 日志
  - WrapWithAudit 批量包装
- [x] auditLoggerAdapter (service→repository 桥接)
- [x] AgentService 集成: NewAgentService 接受 auditRepo，Init 中 WrapWithAudit

#### R2: AI 内容审查
- [x] ReviewProvider 接口 (internal/review/provider.go)
- [x] Mock Provider (internal/review/mock/provider.go) — 直通不修改
- [x] LLM Provider (internal/review/llm/provider.go) — Eino ChatModel 清理 STT 文本
- [x] ReviewService (internal/review/service.go) — 错误时 graceful degradation
- [x] VoiceWSHandler: reviewService + writeMu + async "reviewed" 消息
- [x] main.go: REVIEW_PROVIDER 环境变量 (mock|llm)
- [x] .env.example: REVIEW_PROVIDER=mock

#### R22: Temporal 定时调度
- [x] ScheduleManager (internal/workflow/scheduler.go) — 幂等 schedule 创建
- [x] ReminderWorkflow: 每 5 分钟 / BatchAggregationWorkflow: 每天午夜
- [x] AggregateDailyIdeasActivity: ProjectID==0 no-op

#### R24: 请求限流中间件
- [x] RedisLimiter: Redis Sorted Set 滑动窗口
- [x] MemoryLimiter: Token Bucket 降级方案
- [x] Gin Middleware: user_id/IP 限流 + fail-open + 429 响应
- [x] docker-compose.yml: redis:7-alpine + go-redis/v9

### 验证
- [x] `go build ./...` 通过
- [x] `go vet ./...` 通过

### 新增/修改文件: 12 新建 + 12 修改 = 24 文件

### 待决策
- 无
