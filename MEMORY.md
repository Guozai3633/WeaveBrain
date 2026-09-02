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

---

## Sprint 12: R4 后端 — 可靠语音捕捉与 STT（AudioAsset + 分块上传 + 全文件 STT）
**状态**: ✅ 已完成

### 已完成

#### Part A: 数据模型（迁移 000007）
- [x] 创建数据库迁移 (000007_create_audio_capture.sql)
  - `audio_assets`: user_id+id 复合主键；capture_id 外键级联到 captures(user_id,id)；size_bytes>0、upload_state∈(initiated,uploading,complete,failed) CHECK
  - `transcript_revisions`: 不可变修订版本，revision=(max+1) 原子递增，source∈(stt,user) CHECK，(user_id,capture_id,revision) 唯一
- [x] 契约测试：迁移不 ALTER captures；Down 顺序正确（先 transcript 后 audio）
- [x] 实体: AudioAsset, TranscriptRevision, CaptureAudioAggregate (internal/entity/capture.go)

#### Part B: 仓储接口
- [x] AudioAssetRepository: Create/GetByID/GetByCapture/UpdateState/SetUploadedChunks/GetTranscriptRevisionCount
- [x] TranscriptRepository: AppendRevision/GetLatest/ListByCapture
- [x] 接入 DBStore (store.go NewFromPool + BeginTx)

#### Part C: AudioFileStore（分块暂存 + 最终文件）
- [x] AudioFileStore 接口: WriteChunk/Finalize/OpenFinal/RemoveStaging/ListStagedChunks
- [x] LocalAudioFileStore: staging/{assetID}/part-N 分块落盘（重发幂等）；final/ 拼接
- [x] **storage_path 服务端派生**：DB 存逻辑路径 `audio/{userID}/{assetID}.{ext}`，文件存储映射到物理基目录
- [x] 路径穿越防护（physicalPath 拒绝 `../` 逃逸）+ OpenFinal 拒绝目录（避免 Windows 句柄泄漏）

#### Part D: AudioService
- [x] Initiate（幂等）：校验 capture 存在 + kind=audio + mime/size/sha256/total_chunks
- [x] UploadChunk：每块 SHA-256 校验、index 范围、≤5 MiB、重试幂等
- [x] Complete：块连续齐全 + size 一致 + 全文件 SHA-256，失败清理 staging
- [x] TranscribeCapture：RecognizeFile 全文件 STT；未上传 412；stt_enabled=false 零调用；STT 失败保留原音频
- [x] CorrectTranscript（用户修正 → source=user, revision+1）
- [x] GetByCapture/GetByID/GetLatestTranscript（用户隔离 + 错误映射）
- [x] CaptureService 支持 kind=audio（空文本 → OriginalText nil, 标题「语音记录」）；拒绝 unsupported kind

#### Part E: STT RecognizeFile
- [x] STTProvider 接口扩展 RecognizeFile(ctx, audioPath)
- [x] mock provider: 脚本结果 / 缺文件 / 空文件 / FAIL 模拟失败 / 无脚本报错
- [x] funasr provider: RecognizeFile 实现

#### Part F: API 端点（/api/v3/）
- [x] POST /audio-assets（initiate，201/400）
- [x] GET /audio-assets/:id、GET /audio-assets/by-capture/:captureId
- [x] PUT /audio-assets/:id/chunks/:index（raw body + X-Chunk-SHA256，204/400）
- [x] POST /audio-assets/:id/complete（200/412）
- [x] POST /captures/:captureId/transcribe（201/400/404/412）
- [x] PATCH /captures/:captureId/transcript（用户修正）
- [x] GET /captures/:captureId/transcript
- [x] 新增 V3 错误码 PRECONDITION_FAILED（412）

#### Part G: 接线
- [x] server.go 注册 AudioAssetHandler（受保护 v3 分组）
- [x] service.go Services.Audio 字段
- [x] main.go: AUDIO_STORE_DIR 环境变量 + NewLocalAudioFileStore + NewAudioService

### 技术决策
- 完整音频作为 STT 输入（RecognizeFile），R4 禁止用 WebSocket 字幕替代完整音频资产
- 分块幂等：按 index 落盘 part-N，重发覆盖；重放不重复计数
- 完成时校验：块连续 + size + 全文件 SHA-256，任一失败 400 + 清理 staging
- STT 永删原音频：TranscribeCapture 失败只返回错误，原资产保持 complete
- 转写版本链：STT→revision=1 (stt)，用户修正→revision=2 (user)，全部不可变可追溯

### 修复的问题
1. **storage_path 不一致（真实缺陷）**：Complete 与 TranscribeCapture 路径不匹配 → 统一逻辑路径 + physicalPath 解析
2. GetLatestTranscript 错误未映射 → 统一 404 语义
3. 路径穿越防护 + OpenFinal 拒绝目录
4. ListStagedChunks 未导出补齐

### 验证
- [x] `go build ./...` 通过
- [x] `go vet ./...` 通过
- [x] `go test ./...` 通过（78 个含边界/对抗测试）
  - audio_service_test.go（~35）、audio_store_test.go（~12）、audio_handler_test.go（~13）、stt/mock provider_test.go（6）、capture_service_test.go（kind=audio 正向 + unsupported 拒绝）

### 报告
- [x] docs/round-reports/R04_BACKEND_REPORT.md

---

## Sprint 13: R4 前端 — 录音落盘 + 分块续传 + 转写修正 UI
**状态**: ✅ 已完成（真机/E2E 待验证）

### 已完成

#### Part A: 数据层
- [x] ApiClient putBytes：io/web 原始二进制 chunk 上传（X-Chunk-SHA256 头）；crypto 依赖
- [x] LocalCapture 扩展 kind + audio 元数据 + transcript 字段，toMap/fromMap/copyWith/toCreateRequest
- [x] Sembast 本地存储：saveAudioAsset / updateAudioUploadedChunks / saveTranscript（分块进度 + 转写可重启续传/恢复）
- [x] 条件导出：api_client_io/web、audio_chunk_source_io/web、audio_file_storage_io/web（Web 抛 UnsupportedError）

#### Part B: 上传服务
- [x] AudioRemoteGateway（AudioApi）：initiate / uploadChunk / complete / transcribe / correct / getLatest
- [x] AudioUploadService：create → initiate → 按缺块上传（跳过已传）→ complete → transcribe；失败 AUDIO_METADATA_MISSING / AUDIO_FILE_UNREADABLE / AUDIO_UPLOAD_FAILED
- [x] CaptureSyncService 集成：kind=audio + audioUploader 注入 → 完整链路；上传失败 markRetryable 继续（服务端幂等重放安全）

#### Part C: 录音与 UI
- [x] AudioRecorder 抽象 + AudioRecorderDevice（record 7.1.0，wav 44100 mono，dBFS→振幅归一化）
- [x] AudioFileStorage 抽象 + DefaultAudioFileStorage（path_provider 私有目录，size + 全文件 SHA-256，删除）
- [x] AudioCaptureController 状态机：idle/requestingMic/recording/stopping/saved/permissionDenied/micInUse/saveFailed/unsupported；计时 + 振幅 + 取消清理
- [x] RecordingScreen：暗色全屏、状态文案、mm:ss 计时、48 柱波形、开始/停止/重试、取消二次确认（canPop 守卫）
- [x] TranscriptEditScreen：加载本地+服务端最新转写、可编辑、guest 本地修正 / 登录 PATCH + 本地镜像
- [x] app.dart 路由 /record + /captures/:captureId/transcript（guest 可访问）；CaptureScreen 麦克风入口（Web 降级 SnackBar）
- [x] capture_providers.dart 接线：audioRecorderDeviceProvider / audioFileStorageProvider / audioRemoteGatewayProvider / audioUploadServiceProvider / audioCaptureControllerProvider

### 技术决策
- 音频 capture 复用 kind=audio LocalCapture 持久化（含分块进度 + 转写），重试续传可跨重启
- 全文件落盘后一次上传（R4 不用流式 WebSocket 字幕替代完整音频资产）
- 可注入抽象（AudioRecorder/AudioFileStorage/AudioUploader/AudioRemoteGateway）保证控制器/服务/UI 三层可测
- Web：仅提示能力差异 + 回退文字速记，不持久化音频文件

### 修复的问题
1. **Riverpod Notifier 无 dispose** → ref.onDispose 注册 ticker/振幅清理
2. **取消在根路由崩溃**（GoError: There is nothing to pop）→ context.canPop() 守卫
3. **stop 后 cancel 误删已保存文件** → 成功保存后清空 _recordingPath
4. **cancel 空闲时误 stop** → 仅在有活跃录音路径时 stop+delete
5. 测试环境 sembast 直写挂起（fake-async zone）→ tester.runAsync 包裹真实事件循环写入
6. 测试 pumpAndSettle 死锁（聚焦 TextField 光标闪烁永续帧）→ 改用离散 pump(Duration)

### 验证
- [x] `dart analyze lib test` 通过（0 error / 0 warning）
- [x] `flutter test` 45 个全绿
  - audio_upload_service_test（9）、capture_sync_service_test（11，含音频集成）
  - audio_capture_controller_test（9：权限/占用/开始/保存/取消/异常）
  - recording_screen_test（5 widget）、transcript_edit_screen_test（2 widget）
  - capture_screen_test（3）、widget_test（6）

### 报告
- [x] docs/round-reports/R04_FRONTEND_REPORT.md

### 待验证（真机/E2E）
- [ ] 移动端断网录音 → 重启找回音频（真实设备验证）
- [ ] 完整音频上传 + 后端 RecognizeFile 端到端（尾句不丢）
- [ ] Web 端 SnackBar 能力提示 + 文字速记回退

### 待决策
- 无

---

## Sprint 14: R5 后端步骤②③ — UserAISettings 数据层 + ai-settings API
**状态**: ✅ 已完成（数据层 + API）

### 已完成
- [x] 迁移 000008_create_user_ai_settings.sql：user_ai_settings（user_id PK FK users ON DELETE CASCADE；ai_memory_enabled / ai_completion_enabled / speech_to_text_enabled / cloud_text_allowed / cloud_audio_allowed 全部 BOOLEAN NOT NULL DEFAULT FALSE；revision BIGINT DEFAULT 0；created_at/updated_at）
- [x] 实体 internal/entity/ai_settings.go：UserAISettings + DefaultUserAISettings（隐私优先全 false、revision 0）
- [x] 仓储接口 UserAISettingsRepository：GetByUserID（无行返回 nil）/ Create（并发冲突→VERSION_CONFLICT）/ Update（CAS WHERE revision=$expected → RETURNING revision）
- [x] pg 实现 ai_settings_repository.go：Create 插入 revision 1（首次写入自隐式 0 默认值）；Update revision+1；GetByUserID 按 user_id 作用域
- [x] DBStore 接线：store.go NewFromPool + BeginTx 均注册 AISettings 仓储
- [x] Service ai_settings_service.go：Get 无行时物化默认；Update 部分补丁 + expectedRevision CAS → ErrAISettingsConflict（VERSION_CONFLICT）
- [x] service.go Services.AISettings 接线

### 技术决策
- revision 语义：无行默认态=0；任何写入（含首次 Create）后 revision=1，之后每次写 +1；多设备并发以 expectedRevision CAS 检测
- 部分更新：UpdateAISettingsInput 用指针字段，未出现的字段保持原值
- 创建竞态：Create ON CONFLICT DO NOTHING，rowsAffected=0 → ErrAISettingsVersionConflict

### 验证
- [x] `go build ./...` / `go vet ./...` 通过
- [x] `go test ./...` 全绿（新增 17：仓储 6 + 服务 10 + 迁移契约 1）
  - ai_settings_repository_test.go（6：无行/全列扫描/默认值 Create/Create 竞态/CAS miss/返回新 revision）
  - ai_settings_service_test.go（10：物化默认/读取已存/新用户 Create/匹配 revision 递增/过期 revision 冲突/部分补丁保留/仓储冲突映射/缺 user_id/Get 缺 user_id/Update 冲突映射）
  - user_ai_settings_migration_contract_test.go（1：Up/Down、默认 FALSE、PK、级联删除、索引先于表删除）

### 步骤③：GET/PATCH /api/v3/users/me/ai-settings
- [x] ai_settings_handler.go：Get（物化默认返回 200）+ Update（部分补丁，expected_revision 必填且非负，至少 1 个设置字段；成功 200 / 400 INVALID_ARGUMENT / 409 VERSION_CONFLICT）
- [x] server.go protectedV3 分组接线（s.services.AISettings != nil）
- [x] API_V3_CONTRACT.md §9（认证 / 读取 / 更新 / revision 语义 / 自动化证据）

### 步骤③验证
- [x] `go build ./...` / `go vet ./...` 通过
- [x] `go test ./...` 全绿（步骤③新增 10：ai_settings_handler_test.go）

### 下一步
- R5 步骤④⑤：AI 记忆整理总开关（默认 false）+ AI 补全/独立转写/云端授权开关（字段已就位，待接行为）
- R5 步骤⑥：Capture 策略快照 + PostgreSQL Outbox Worker

---

## Sprint 15: R5 后端步骤④⑤⑥⑦⑧ — 策略快照 + Outbox Worker + 关闭取消
**状态**: ✅ 已完成（后端；前端第九步见 Sprint 16）

### 已完成

#### 迁移 000009（capture_outbox 生命周期改造）
- [x] capture_outbox 增加 `policy_snapshot JSONB NOT NULL DEFAULT '{}'::jsonb`、`last_error TEXT`、`updated_at TIMESTAMPTZ NOT NULL DEFAULT now()`
- [x] 数据迁移：pending→queued、processed→ready；替换状态约束为 `('queued','retry_wait','processing','ready','failed','cancelled')`
- [x] 替换索引为 `idx_capture_outbox_due (status, next_run_at, id) WHERE status IN ('queued','retry_wait')`
- [x] Down 完整可逆（恢复 pending/processed 状态 + 旧索引）

#### 实体与策略快照
- [x] PolicySnapshot（5 个 AI 授权布尔，DefaultPolicySnapshot 全 false，FromAISettings 无行→默认）
- [x] CaptureOutbox 扩展：PolicySnapshot、LastError、UpdatedAt

#### OutboxRepository（pg）
- [x] ClaimDue：CTE + `FOR UPDATE SKIP LOCKED`，WHERE status IN ('queued','retry_wait') AND next_run_at<=now()，LIMIT $1，attempt_count+1 → processing，RETURNING 13 列（payload/policy 反序列化）
- [x] MarkReady / MarkFailed / MarkRetryWait（next_run_at=now()+backoff，last_error=$3）
- [x] CancelByUser（只取消 queued/retry_wait，返回受影响行数）
- [x] CountQueued（queued+retry_wait 计数）
- [x] ErrOutboxEventNotFound（MarkReady 0 行 → 判定）

#### CaptureRepository.Create 策略快照落库
- [x] Create(ctx, capture, card, policy)：outbox INSERT 增加 `policy_snapshot $18::jsonb`；payload 内嵌 privacy_mode

#### CaptureService 快照解析
- [x] AISettingsSnapshotSource 接口（Get(ctx,userID)），NewCaptureService(repo, settings)
- [x] resolvePolicySnapshot：nil 源或读取失败 → 隐私优先全 false（绝不阻塞捕获创建）；正常 → FromAISettings

#### EnrichmentPipeline + OutboxWorker
- [x] EnrichmentPipeline 接口（Organize/Embed/Relate/Recap）+ R5 no-op 默认实现（R6/R8 替换）
- [x] OutboxWorker：Run 轮询（PollInterval 默认 1s，ClaimBatch 20）；Start/Stop 幂等
- [x] processDue → processOne：快照 AI 关闭或 privacy_mode=no_ai → 直接 MarkReady 零 pipeline；否则 GetByID 捕获（不存在 → MarkFailed 终态）→ 4 阶段 pipeline → MarkReady
- [x] scheduleRetry：attempt>=MaxAttempts → MarkFailed；否则 MarkRetryWait + 指数退避（RetryBase×2^(n-1)，上限 RetryMax）

#### AISettingsService 关闭取消
- [x] 注入 OutboxRepository；Update 成功后若 AI 记忆整理 由开→关 → CancelByUser（尽力而为，失败仅日志）
- [x] 开启不取消、无关开关变更不取消（均有测试）

#### 接线
- [x] store.go：Outbox 仓储（NewFromPool + BeginTx）
- [x] service.go：AISettingsService 带 outbox；CaptureService 带 settings；构造 OutboxWorker（store 可用时）
- [x] main.go：Start 于 <-stop 前，Stop 于优雅关闭

### 技术决策
- 事务性 Outbox：捕获创建与 outbox 事件同语句写入，快照随事件冻结 → 「再开启只影响新 Capture」
- 隐私优先兜底：设置源故障绝不阻塞捕获创建（默认全 false，pipeline 零调用）
- 关闭取消是优化而非正确性必需：worker 快照门控本身保证不再触发 AI；CancelByUser 尽力而为清理
- organize-once：每个事件一次完整 pipeline（状态 processing 持锁），重试不重跑已成功阶段

### 验证
- [x] `go build ./...` / `go vet ./...` 通过
- [x] `go test ./...` 全绿
  - outbox_repository_test.go（7：ClaimDue 解码 + FOR UPDATE SKIP LOCKED / 空集合 / MarkReady / 未找到 / MarkFailed / MarkRetryWait 退避 / CancelByUser 作用域 / CountQueued）
  - outbox_worker_test.go（8：AI 关→零 pipeline ready / no_ai 跳过 / AI 开→全 pipeline / 失败→退避重试 / 超限→终态 failed / 捕获缺失→立即 failed / 退避指数与封顶 / Start-Stop 幂等）
  - outbox_processing_migration_contract_test.go（1：Up/Down 契约）
  - capture_service_test.go（+3：快照落库 / 设置源失败→隐私兜底 / 无源→隐私兜底）
  - ai_settings_service_test.go（+3：关闭→取消 queued / 开启→不取消 / 无关开关→不取消）
  - 更新：capture_repository_test.go、capture_repository_integration_test.go、capture_service_test.go、audio_service_test.go、ai_settings_service_test.go（Create 签名 + NewCaptureService/NewAISettingsService 新签名）

### 修复的问题
1. **cancelQueuedIfDisabled 守卫反转**：`wasEnabled || nowEnabled` → `!wasEnabled || nowEnabled`（关闭场景 wasEnabled=true 本应触发取消，原逻辑跳过）
2. **pgx.Rows 测试 mock Conn() 类型**：v5.10.0 中 `Rows.Conn()` 返回 `*pgx.Conn`（非 *pgconn.PgConn/Conn）

### 报告
- [x] docs/DEVELOPMENT_GOALS.md G3 步骤④⑤⑥⑦⑧ 已勾选 + 完成条件更新

### 下一步
- G4 / R6：记忆卡、记忆流与详情（fallback MemoryCard 完整字段、EnrichmentRevision + provenance + source_revision、记忆流倒序 + 筛选 + 全文搜索、详情按 ID 加载、修正/续写/置顶/归档/删除、主导航调整）

---

## Sprint 16: R5 步骤⑨ — Flutter AI 设置页 + 后端补整理端点
**状态**: ✅ 已完成

### 已完成

#### 后端：历史补整理（Reorganize）
- [x] OutboxRepository 增加 `CountPendingReorganize`（count ready+快照关闭）与 `ReorganizeByUser`（UPDATE → queued、盖当前快照、attempt=0、next_run=now，返回行数）
- [x] AISettingsService：`ErrReorganizeAIMemoryDisabled` 哨兵 + `CountPendingReorganize`/`Reorganize`（AI 关 → 哨兵；outbox nil → 优雅降级 0）
- [x] AISettingsHandler：GET 信封新增 `pending_reorganize`；POST `/users/me/ai-settings/reorganize` → `{reorganized, request_id}`；`ErrReorganizeAIMemoryDisabled` → 409 FEATURE_NOT_ENABLED
- [x] 测试：outbox(+3)、service(+4)、handler(+4，更新 fake 增加 count/reorganize) 全绿

#### 前端：/settings/ai 页
- [x] `ai_settings_api.dart`：AISettings 模型（5 开关 + revision + copyWith/fromJson）+ AISettingsResult 信封 + AISettingsGateway 抽象 + AISettingsApi + v3 ApiClient provider
- [x] `ai_settings_notifier.dart`：sealed Loading/Loaded/Error；load / setSwitch(携带 expected_revision) / reorganize(成功后 reload 刷新计数) / clearMessage；409 → 「设置已在其他设备修改，请刷新」
- [x] `ai_settings_screen.dart`：总开关「AI 记忆整理」默认关 + 独立「语音转写」+ 3 子开关（总开关关时置灰保留原值）+ revision 行 + 总开关开且 pending>0 时「补整理未处理记忆 (N 条)」按钮
- [x] 接线：settings_screen 加「AI 与自动化」入口 + app.dart `/settings/ai` 嵌套路由
- [x] 测试：api(8) / notifier(7) / screen(5) + 全量 `flutter test` 65 绿 + `dart analyze lib test` 无问题

### 技术决策
- 补整理目标集 = `status='ready' AND policy_snapshot->>'ai_memory_enabled'='false'`：正是「AI 关闭期间创建 → worker 直接 MarkReady 未跑 pipeline」的记忆；不含 failed/cancelled（保持「关闭即取消」显式语义）
- 补整理仅当 AI 记忆整理开启时允许；关闭 → 409 FEATURE_NOT_ENABLED
- 开关映射：ai_memory_enabled→总开关；speech_to_text_enabled→独立转写（关闭整理仍可转写 → TranscriptRevision）；ai_completion_enabled/cloud_text_allowed/cloud_audio_allowed→3 子开关（总开关关时置灰保留原值）
- 设置 revision 同源展示：手机与 Web 共用同一 GET /ai-settings 信封 → 满足「手机/Web 显示同一 revision」

### 验证
- [x] `go build ./...` / `go vet ./...` 通过；`go test ./...` 全绿
- [x] `dart analyze lib test` 无问题；`flutter test` 65 绿
- [x] docs/API_V3_CONTRACT.md §9.2 增加 pending_reorganize + 新增 §9.5 补整理端点
- [x] docs/DEVELOPMENT_GOALS.md G3 第九步勾选 + 完成条件「历史补整理显式触发」「手机/Web 同 revision」标 [x]

### 下一步
- G4 / R6：记忆卡、记忆流与详情（fallback MemoryCard 完整字段、EnrichmentRevision + provenance + source_revision、记忆流倒序 + 筛选 + 全文搜索、详情按 ID 加载、修正/续写/置顶/归档/删除、主导航调整）

---

## Sprint 17: R5 全面测试 + 总结
**状态**: ✅ 已完成

### 已完成
- [x] `go test ./...` 全绿（143 用例，0 失败）；`flutter test` 全绿（65 用例）；`dart analyze lib test` 0 error / 0 warning
- [x] docs/round-reports/R05_REPORT.md（阶段结论 / 交付能力 / 设计要点 / 测试情况 / 修复问题 / 完成条件核对 / 下一步）
- [x] docs/DEVELOPMENT_GOALS.md 顶层表 G3 → 「R5 完成（后端 143 + 前端 65 测试绿；真机/E2E 待验证）」
- [x] G3 九步 + 全部完成条件已勾选

### 下一步
- G4 / R6：记忆卡、记忆流与详情（fallback MemoryCard 完整字段、EnrichmentRevision + provenance + source_revision、记忆流倒序 + 筛选 + 全文搜索、详情按 ID 加载、修正/续写/置顶/归档/删除、主导航调整）

---

## Sprint 18: R6 记忆卡、记忆流与详情（G4）
**状态**: ✅ 已完成（真机 / E2E 待验证）

### 已完成

#### 后端：fallback 卡 + EnrichmentRevision + 记忆流 7 端点
- [x] 迁移 000010：pg_trgm 扩展、memory_cards 补列（summary / tags JSONB / key_points JSONB / is_pinned / pinned_at）、memory_card_revisions 表（revision 升序、card_version、source CHECK fallback|ai|user、changes/provenance JSONB、UNIQUE(user_id,capture_id,revision)）、三列 GIN（original_text / title / transcript text）+ pinned 部分索引 + tags 索引
- [x] fallback 原子落库：capture 创建单条 CTE 同时写 capture + memory_card（title 前 30 rune、summary 前 200 rune+…、tags=[]、key_points=[]、primary_type='uncategorized'）+ 初始 fallback revision（revision=1 / card_version=1 / source=fallback / source_revision=1）
- [x] MemoryRepository：List（倒序 is_pinned DESC, created_at DESC, id DESC + keyset 游标 base64url(JSON{p,t,i}) + 三列 ILIKE 搜索 q 转义 `\ % _` + kind/primary_type/pinned/lifecycle 筛选）、ListRevisions 降序、AppendRevisionAndUpdateCard（correct 原子 CTE，MAX(revision)+1，并发撞 UNIQUE → ErrVersionConflict）、AppendNoteAndBump（续写不改字段）、SetPinned（pinned_at）、SetLifecycle（archived 幂等 / trashed→deleted_at）
- [x] MemoryService：List 默认 limit 20 封顶 50 / 默认 active / 游标解码；GetDetail（capture 404 + audio/transcript 容忍 nil + revisions）；Correct（校验 title≤300、summary≤2000、tags≤20×100、key_points≤3×500、primary_type 白名单 + user revision + card.version+1）；AppendNote（≤10000 rune）；SetPinned / Archive（幂等）/ Delete（trashed）
- [x] MemoryHandler 7 端点：GET /memories（列表+搜索+游标）、GET /memories/:captureId（详情）、PATCH（修正）、POST notes（续写）、POST pin、POST archive、DELETE（软删）；错误映射 400/401/404/409（VERSION_CONFLICT）；每个响应含 request_id
- [x] server.go v3 鉴权组注册；AI pipeline 保持 no-op（R8 真实 AI 写 ai revision，失败不覆盖原文）

#### 前端：记忆流与详情页 + 主导航四标签
- [x] memory_api.dart：MemoryCardModel（summary/tags/keyPoints/isPinned/version）、MemoryListEntry/Result、MemoryRevision、MemoryDetail（audio/transcript 可空）+ MemoryGateway 抽象 + MemoryApi（v3 ApiClient）+ provider（测试 override 点）
- [x] memory_notifier.dart：MemoryListNotifier（load 重置游标 / loadMore / search 防抖 / setFilter / refresh + 动作后本地更新或 reload + message）+ MemoryDetailNotifier（load(captureId) 从路径参数 / correct / addNote / setPinned / archive / delete；409/404 提示）
- [x] memory_list_screen.dart：搜索框（300ms 防抖）+ 筛选 chips + RefreshIndicator + loadMore + 空/错态；卡片每卡恰一个主按钮（按 primary_type 映射）
- [x] memory_detail_screen.dart：按 captureId 加载（刷新按 ID 恢复）；头部（类型/状态/置顶）、原始记忆（原文/音频/转写入口）、整理字段、继续思考续写、修订历史、修正弹窗 / 归档 / 删除确认；全部测试 Key 化
- [x] 导航：StatefulShellRoute 4 分支（记忆/捕捉/回响/我的）；回响 = EchoScreen 占位「规划中」；/memories/:captureId 顶层隐藏路由按 ID 加载（pathParameters 非 extra）；guest /→/capture；/ideas /timeline /projects 降级为隐藏路由
- [x] app_scaffold.dart 4 目的地（library_books / mic / auto_awesome / person）

### 技术决策
- 修订而非覆写：用户编辑只追加 EnrichmentRevision + card.version+1，captures.original_text 全程只读；source_revision = captures.version 溯源锚点
- 并发安全：修订号在单条 CTE 内 MAX(revision)+1，撞 UNIQUE → 409 VERSION_CONFLICT；R6 不做卡片乐观锁
- keyset 游标：排序键 (is_pinned, created_at, id) 稳定，翻页携带相同筛选；base64url 编码
- pg_trgm GIN：CJK 子串 ILIKE 走索引；trigram ≥3 字符限制使 1–2 字符中文可能顺序扫描（MVP 接受，记入 research note）
- 软删语义：delete → trashed + deleted_at（列表/详情隐藏）；audio 资产暂不级联
- Flutter 三层可测抽象：MemoryGateway + Notifier（sealed state）+ 页面；Riverpod overrideWithValue 注入 fake

### 验证
- [x] `go build ./...` / `go vet ./...` 通过
- [x] `go test ./...` **227** 用例全绿（R5 143 → +84）
  - memory_repository_test.go（List 所有权/排序/搜索/筛选/游标/ListRevisions/AppendRevision 原子 CTE/SetPinned/SetLifecycle）
  - memory_service_test.go（Correct→user revision + version 递增 + provenance / 校验失败 / AppendNote 不改字段 / trashed→404 / SetPinned 切换 / Archive 幂等 / Delete→trashed / GetDetail nil 容忍 / fallback 字段派生）
  - memory_handler_test.go（list 200+next_cursor / 筛选透传 / detail 200+revisions / 404 / PATCH 200+400 / notes / pin / archive / delete / 401 / request_id）
  - migration_contract_test.go（迁移 000010 结构 + pg_trgm + GIN + source CHECK + Down 顺序）
  - 更新：capture_repository/service/handler + outbox_worker（Create 签名带初始 revision、fallback 派生、AI 关创建后 fallback revision 存在）
- [x] `dart analyze lib test` 0 error / 0 warning
- [x] `flutter test` **113** 用例全绿（R5 65 → +48）
  - memory_api_test.dart（20）/ memory_notifier_test.dart（14）/ memory_list_screen_test.dart（5）/ memory_detail_screen_test.dart（8）/ widget_test.dart（2：guest 重定向 + 4 目的地 + 详情按 ID 加载）

### 修复的问题
1. **详情页「取消归档」无后端对应**：R6 仅幂等 archive（无取消归档端点），改为固定「归档」按钮 + 契约记录限制
2. **Flutter widget 测试 warnIfMissed**：详情页动作按钮折叠线下方，tap 前补 ensureVisible
3. **测试文案冲突**：摘要默认值 '摘要' 与「整理字段」标签冲突 → 改 '这是一段摘要'
4. **analyzer 3 处告警**：两个测试未用 import + _FakeGateway 未用构造参数，清理后 0/0

### 报告
- [x] docs/round-reports/R06_REPORT.md（阶段结论 / 交付能力 / 设计要点 / 测试情况 / 修复问题 / 完成条件核对 / 已知限制 / 下一步）
- [x] docs/API_V3_CONTRACT.md §10 记忆流接口（7 端点 + 数据模型 + 搜索/游标/生命周期/错误契约 + 自动化证据）
- [x] docs/DEVELOPMENT_GOALS.md G4 八步 + 7 完成条件全部勾选
- [x] docs/topic_notes/r6_memory_stream_search.md（pg_trgm 选型决策记录）

### 已知限制（记入 R06 报告）
- 无取消归档 / 回收站还原（G13 回收站级联）；存量捕获无 fallback revision（可选一次性回填）；并发编辑 409 客户端重试；1–2 字符中文搜索可能不走 trigram 索引；AI 组织留 G6/R8

### 下一步
- G5 / R7：验收 B —— 核心捕捉与 AI 控制（原始捕捉丢失为 0、AI 关闭时后台整理调用为 0、95% 任务最终 ready 或明确失败、原音频/原文/修正版可追溯、体验样本、严重权限与数据串用户问题为 0、R7 验收报告）

---

## Sprint 19: R7 验收 B — 核心捕捉与 AI 控制（G5）
**状态**: ✅ 已完成（自动化证据全绿；G5 标准 5「20 样本人工验收」待真机回填）

### 已完成

#### 后端修复：v1 跨用户漏洞 + timeline bug + outbox 孤儿回收
- [x] VULN-1 `GET /api/v1/ideas` 跨用户读 → `GetByProjectID` / `Search` 增加 user_id，列表限定当前用户项目（他人项目 → 空列表）
- [x] VULN-2 `POST /api/v1/ideas` 跨用户写 → IdeaService 项目归属哨兵 `ErrProjectForbidden`，handler 403
- [x] VULN-3 `GET /api/v1/reminders` 跨用户读全部待办 → 新增 `GetPendingByUser`（保留 cron 系统级 `GetPending`），HTTP 用它
- [x] VULN-4 `GET /api/v1/agent/workflow/:id` 跨用户读 → 取回后比对 `workflow_runs.user_id` → 403
- [x] VULN-5 `GET /api/v1/users/:id` 他人资料可读 → 仅本人，他人一律 404
- [x] BUG-1 `GET /api/v1/users/me/timeline` 恒 401 → `getCurrentUserID` 修复类型断言
- [x] 孤儿回收：`ClaimDue(ctx, limit, lease)` 重领 stale `processing`（`updated_at <= now()-lease`）+ `FOR UPDATE SKIP LOCKED` + `OutboxWorkerConfig.ClaimLease`（默认 5min）+ 迁移 000011 stale-processing 部分索引

#### 真实 PG 集成运行新发现的真实缺陷修复
- [x] 迁移 000009 遗留 bug：`capture_outbox.status` DEFAULT 仍为 `'pending'`，与 CHECK（queued...cancelled）冲突，生产新建捕捉必崩 → 迁移 000012 `SET DEFAULT 'queued'` + 契约测试
- [x] 音频 `Initiate` 未持久化 `sha256` → `Complete` 空指针 → INSERT 补 sha256 列
- [x] `DBStore` 从未接线 Timeline 仓储 → timeline 端点恒 500 → store.go `NewFromPool`/`BeginTx` 补 `Timeline` 初始化
- [x] `MemoryRepository.SetLifecycle` `$3` 类型推断歧义（SQLSTATE 42P08）→ `$3::text` 显式转型 + 单测断言同步
- [x] 集成测试共享库污染：孤儿回收 / soak 测试种数据前 `DELETE FROM capture_outbox`

#### 测试设施 + 集成测试
- [x] Makefile：`TEST_DB_URL` + `db-create-test` / `migrate-test-up` / `test-integration` / `test-integration-fast`（独立 weavebrain_test 库）
- [x] README（WeaveBrain/README.md）：Testing 节 + 目录 + 根目录 docker-compose 重复副本清理备注
- [x] 单元/契约（无 env）**236** 全绿（R6 227 → +9）：idea_repo +2 / reminder_repo +1 / idea_service +3 / outbox_repo stale 回收 +1 / 迁移契约 000011 + 000012 +2 / SetLifecycle SQL 断言同步
- [x] 真实 PG **242** 全绿：6 个环境门控集成测试实际运行 —— R7 新增 TestR7CrossUserSecurityAcceptance / TestOutboxOrphanRecoveryIntegration / TestR7BatchSoakIntegration（40/40 终态，ready=31+failed=6+cancelled=3，AI 关 pipeline 0 调用）/ TestR7TraceabilityChainIntegration（[stt,user] + [fallback,user] 修订链、音频 sha256 不变）+ 既有 R3/R4 集成回归

#### 文档
- [x] docs/round-reports/R07_REPORT.md（阶段结论 / 交付能力 / 设计要点 / 测试情况 / 修复问题 / G5 7 项核对 / 已知限制 / 下一步）
- [x] docs/API_V1_SECURITY_CONTRACT.md（v1 行为变更契约：漏洞 → 状态码、404 vs 403、GetPendingByUser 偏离、残留暴露）
- [x] docs/round-reports/R07_MANUAL_UX_SAMPLE.md（20 条口语短语人工验收清单，判定 ≥65% = 13/20，待真机执行）
- [x] docs/DEVELOPMENT_GOALS.md G5：6 项自动化标准勾选；标准 5 待人工；顶层表 → 「R7 完成（后端 242 测试含真实 PG 集成全绿；20 样本人工验收待回填）」

### 技术决策
- GetPendingByUser 偏离：cron 需系统级 GetPending（遍历全用户投递），HTTP 用用户级方法；有据偏离记入契约
- 孤儿重领租约锚点 = `updated_at`（worker 每次处理刷新），cutoff 预计算绑 `timestamptz`（规避 pgx Duration→interval codec）
- 集成测试共享库隔离：随机 UUID 防主键撞车；全局操作（ClaimDue / 全表状态计数）前先清 `capture_outbox`
- 真实 PG 是唯一能暴露「迁移默认值 vs CHECK」「仓储漏列」「仓储接线遗漏」「类型推断歧义」的手段 —— 验收「不跑集成测试不合并」成立

### 验证
- [x] `go build ./...` / `go vet ./...` 通过
- [x] `go test ./...`（无 env）236 全绿；`WEAVEBRAIN_TEST_DATABASE_URL=... go test ./...` 242 全绿（goose 迁移 000001—000012 + 6 集成测试）
- [x] 集成测试设施在真实 PostgreSQL 上跑通（docker-compose postgres + weavebrain_test 库）

### 已知限制
- G5 标准 5（20 样本）待真机人工执行回填；残留暴露 `SearchByTags`/`SearchBySimilarity` 仅 project 作用域（记后续）；`captures.version` 恒 1 → `source_revision` 恒 1（语义缺口非泄漏，R7 不修）；迁移 000011/000012 需对 dev 库 `goose up`

### 下一步
- G6 / R8：AI 补全（CompletionProposal / FieldProposal、completion:preview / apply、evidence_spans、safe_auto/suggest_only/forbidden、逐字段采用 + 版本与撤销、详情页补全入口、旧提案过期）

---

## Sprint 20: R8 AI 补全（G6）
**状态**: ✅ 已完成（真机 / E2E 待验证）

### 已完成

#### 后端：completion_proposals + preview/apply/undo
- [x] 迁移 000013：completion_proposals（一行 = 一个字段提案，同批次共享 preview_id；field_name CHECK ∈ title/primary_type/summary/tags/key_points；provenance/apply_policy/status CHECK；evidence_spans JSONB；FK 级联 captures(user_id,id)；preview/capture_status 索引；Down 可逆）+ 契约测试
- [x] 实体 internal/entity/completion.go：ProposalStatus / ApplyPolicy（safe_auto/suggest_only/forbidden）/ Provenance / EvidenceSpan / CompletionProposal / CompletionPreviewResult / CompletionApplyResult
- [x] 仓储 completion_repository.go：CreateProposals / ListByIDs / ListPendingByCapture / ExpireAllPending / MarkAccepted（status='pending' 守卫幂等）/ MarkRejected / MarkExpired；interface.go + store.go（NewFromPool + BeginTx 两处装配）；真实 PG 集成测试（confidence 浮点容差）
- [x] LLM 生成器 completion_llm.go：FieldProposalGenerator 接口 + LLMFieldProposalGenerator（agent.NewChatModel，LLM_BASE_URL/API_KEY/MODEL 默认 localhost:11434/v1/ollama/qwen2.5:7b）；strict JSON 数组解析（取首个 `[` 至末个 `]` 容忍 code-fence）、过滤非法字段、长度校验 + primary_type 白名单、非数组 → ErrCompletionLLM（500）
- [x] 服务 completion_service.go + service.go 装配：Preview（门控 AICompletion && CloudTextAllowed；不改业务对象；新 preview 使旧 pending 过期；source_revision=card.Version）；Apply（仅门控 AICompletion；幂等全 accepted no-op；版本冲突 → 过期 + 409；非空字段保护 → rejected；ai 修订 `provenance._completion` 存 undo 原值 + card.version+1）；Undo（回滚「当前值仍==AI 所设」字段，用户后续编辑跳过；source=user 撤销修订；提案保持 accepted）；生成器 nil 容忍（Preview 500，Apply/Undo 正常）
- [x] 处理器 completion_handler.go + server.go：POST /captures/:captureId/completion/{preview,apply,undo}；错误映射 FEATURE_NOT_ENABLED / PRECONDITION_FAILED / VERSION_CONFLICT / NOT_FOUND / INVALID_ARGUMENT / INTERNAL；响应带 request_id

#### 前端：completions feature + 详情页接入
- [x] completion_api.dart：EvidenceSpanModel / CompletionProposalModel（proposedValues 解析 JSON 数组、canAutoApply）/ PreviewResult / ApplyResult（复用 MemoryCardModel）+ CompletionGateway 抽象 + CompletionApi + provider
- [x] completion_notifier.dart：sealed state；loadPreview / applyAllSafe（只发 safe_auto pending + sourceRevision）/ applyOne / undo / clearMessage；409 分码提示（FEATURE_NOT_ENABLED「AI 补全未开启」/ VERSION_CONFLICT「提案已过期，请重新生成」/ PRECONDITION_FAILED）
- [x] completion_panel.dart：入口开关 Key('completion_entry_button')「AI 补全」；提案卡片（字段中文标签 + 建议值 chips + 证据引文 chip + 「AI 建议/需确认」policy chip + 置信度）；「全部采用安全字段」批量 + 逐字段「采用」+「撤销上次补全」；disabled → SizedBox.shrink() 零调用
- [x] memory_detail_screen.dart 接入：`_aiCompletionAvailable`（AI 补全开启 && 卡片 ready 才展示）+ onApplied 刷新详情
- [x] 测试：completion_api_test（9）/ completion_notifier_test（11）/ completion_panel_test（8）/ memory_detail_screen_test（+3 面板开关）/ widget_test（aiSettingsGatewayProvider fake override）

### 技术决策
- undo 原值存 `provenance._completion.undo`（changes 是 flat map，无法表达 from/to）；已记入 API_V3_CONTRACT §11.4 与 R08 报告
- proposed_value 用 TEXT：tags/key_points 存 JSON 数组字符串，解码只在服务 apply + 前端 model 两处，配单测锁定格式
- source_revision：提案 = card.Version（并发守卫）；补全修订 = captures.Version（审计口径，与 Correct 一致）
- primary_type='uncategorized' 视为缺失（fallback 恒填）；title/summary 有 fallback 值 → 默认受保护，本轮不覆盖（改写留 R9+）
- 门控分层：preview 需 AICompletion && CloudTextAllowed（原文发 LLM）；apply 仅 AICompletion（不发文本）
- confidence REAL=float4 精度丢失 → 集成测试用 math.Abs 容差（保留计划 REAL 列）

### 验证
- [x] `go build ./...` / `go vet ./...` 通过
- [x] `go test ./...`（无 env）**301** 全绿（R7 236 → +65）；真实 PG **308** 全绿（R7 242 → +66；迁移 000001—000013 + completion 仓储集成实际运行）
- [x] goose 迁移 000013 up/down 可逆
- [x] `dart analyze lib test` 0 issue；`flutter test` **144** 全绿（R7 113 → +31）

### 报告
- [x] docs/round-reports/R08_REPORT.md
- [x] docs/API_V3_CONTRACT.md §11（AI 补全接口）
- [x] docs/DEVELOPMENT_GOALS.md G6（5 步 + 7 完成条件全部勾选）

### 已知限制
- title/summary 受保护不改写（R9+ 显式确认）；Ollama 严格 JSON 遵从度有限（解析容忍 code-fence，非数组 → 500）；`flutter analyze` 本机崩溃用 `dart analyze` 替代；真机 / E2E（Ollama smoke）待验证

### 下一步
- G7 / R9：单条与批量导入（单条文字/音频/文件、多段文本/TXT/Markdown/CSV/JSONL、字段映射 + 前 10 条预览、AI 补全缺失项 + 去重 + Commit + 行级错误隔离、结果/错误报告下载）

---

## Sprint 21: R9 单条与批量导入（G7）
**状态**: ✅ 已完成（后端 337 无 env + 356 真实 PG 集成全绿 + 前端 184 全绿）

### 已完成

#### 后端：迁移 000014 + 单条导入 capture 管道扩展
- [x] 迁移 000014：captures ADD COLUMN external_id / source_name / content_hash + 部分唯一索引 `uq_captures_user_source_external`（去重 DB 安全网）+ `idx_captures_user_content_hash`；`memory_card_revisions_source_check` 加 `'import'`；import_jobs / import_rows 全结构（format/status/dedupe_status CHECK、completion_proposals JSONB、UNIQUE(import_job_id,row_number)、job+hash 索引）+ 契约测试
- [x] Capture 管道：Capture 加 ExternalID/SourceName/ContentHash + CaptureDedupe；`normalizeAndHashContent`（trim + 折叠空白 + SHA-256）；CreateCaptureInput 加 ExternalID/SourceName/Title/PrimaryType/Tags override；kind=import 归一化（默认 source=import）；`initialEnrichmentRevision` 参数化 source；`ErrDuplicateExternalID` 哨兵 + `DuplicateExternalIDError`（带 existing_capture_id）；外部 ID 精确 → 409，content_hash 命中 → Dedupe.suggested 但仍创建
- [x] capture_repository CTE：INSERT captures 加 3 列 + `memory_card_revisions` source 参数化（`'fallback'` → `$27`）

#### 后端：批量导入（ImportService + 解析器 + 仓储 + 处理器）
- [x] 实体 internal/entity/import.go：ImportJob / ImportRow / ImportFieldProposal（复用 R8 类型）/ ImportPreviewResult / ImportCompletionResult / ImportCommitResult / ImportErrorReportEntry
- [x] 仓储 import_repository.go：CreateJob（单事务 job+rows 原子）/ GetJob / ListRows / GetRowsByNumbers / UpdateJobStatus / MarkRowState / SetRowCompletion / FindExternalDuplicates / FindContentHashMatches；interface.go + store.go（NewFromPool + BeginTx 两处装配）+ 真实 PG 集成测试
- [x] 解析器 import_parser.go：plainTextParser（separator 分段）/ csvParser（表头映射、tags `|`、captured_at ISO-8601、经纬度成对+范围、坏行隔离）/ jsonlParser（逐行 JSON，坏行 invalid_json）+ 表驱动测试
- [x] ImportService + service.go 装配：CreateJob（校验 + 解析 + 预计算去重 + 原子建 job）→ GetPreview（前 10 + 状态 previewed 幂等）→ CompletionPreview（门控 AICompletion&&CloudTextAllowed，单行 LLM 失败不阻塞）→ CompletionApply（accepted/rejected 幂等）→ Commit（**capture_id=row.id 确定性 + 每行独立事务 + CTE ON CONFLICT 幂等**；duplicate_external 硬跳过 / suggested 用户策略；行级隔离）→ GetErrorReport / Cancel
- [x] 处理器 import_handler.go + server.go：8 端点（POST /imports 201、GET /imports/:id、GET preview?limit&offset、completion/preview、completion/apply、commit、error-report?format=csv|json、cancel）+ 错误映射（400/401/404/409/409 FEATURE_NOT_ENABLED/500）+ 8 MiB body 上限；capture_handler 扩展（kind=import 透传 + Dedupe 响应 + 409 existing_capture_id）

#### 前端：imports feature + 入口/路由
- [x] import_api.dart：手写模型（蛇形映射）+ ImportGateway 抽象 + ImportApi（importSingle 发 POST /captures kind=import + Idempotency-Key + 409 → ImportDuplicateException）+ provider；text_file_reader（IO readAsString / Web FileReader 条件导出）
- [x] import_notifier.dart：sealed ImportState（Idle/Creating/Created/Previewed/Committing/Committed/Error）+ importNotifierProvider
- [x] import_screen.dart：单条/批量 SegmentedButton；单条（content 必填 + 选填元数据 → importSingle → 跳详情页 / 409 提示既有记忆链接）；批量（粘贴/文件选择、格式下拉、解析并预览前 10 行表格 + 去重 chip、AI 补全勾选采用、开始导入 + 重复内容策略、结果页 + 错误报告下载）
- [x] 入口：settings_screen「导入旧记忆」ListTile + capture_screen AppBar 导入 IconButton（已认证才显示）+ app.dart `/imports` 路由
- [x] 测试：import_api_test / import_notifier_test / import_screen_test + widget_test（ImportGateway fake override）+ settings/capture 回归

### 技术决策
- 确定性 `capture_id = import_rows.id`：重复 Commit 幂等靠 Capture CTE `ON CONFLICT (user_id,id) DO NOTHING` + imported 行快进，不靠状态竞态（标准 3 双保险）
- 去重收敛两级：external_id 精确硬跳过 + content_hash 疑似提示（用户决定导入/跳过）；部分唯一索引并发安全网；merge/资产 SHA-256/近似时间留 R10+
- provenance 语义：导入初始修订 `source=import`（initialEnrichmentRevision 参数化）；补全修订 `source=ai`；前端记忆流已有 import 标签/图标
- 行级事务隔离：Commit 每行独立 `store.BeginTx`，失败行标 failed 不影响其他行（标准 1）
- 地点/时间缺失保持 unknown：不伪装导入时间、不猜测坐标（标准 6）；地点字段只解析校验不落库（captures 无列）
- 单条导入 AI 补全免费获得：提交后跳 `/memories/$captureId`，R8 补全面板直接可用（无需新端点）
- 解析/补全/Commit 三阶段独立：补全失败单行记 error，其他行照常；「按原样导入」默认可用（标准 4）

### 验证
- [x] `go build ./...` / `go vet ./...` 通过
- [x] `go test ./...`（无 env）**337** 全绿（R8 301 → +36）
- [x] `WEAVEBRAIN_TEST_DATABASE_URL=... go test ./...`（真实 PG）**356** 全绿（R8 308 → +48）—— 迁移 000001—000014 + import 仓储/服务集成 + R3/R4/R7 既有集成回归全部实际运行；000014 down/up 可逆（Down 先重映射 import 修订 → user 再收紧 CHECK）
- [x] `dart analyze lib test` 0 issue；`flutter test` **184** 全绿（R8 144 → +40）

### 报告
- [x] docs/round-reports/R09_REPORT.md
- [x] docs/API_V3_CONTRACT.md §12（导入接口：去重模型 / 单条 kind=import / 8 批量端点 / 数据模型 / 错误映射 / 自动化证据）
- [x] docs/DEVELOPMENT_GOALS.md G7（5 步 + 7 完成条件全部勾选；顶层表 G7 → R9 完成）

### 已知限制
- 1 千行以上分批/流式 Commit 留后续（MVP 单次 8 MiB body + 1000 短事务）；两段上传预留
- 地点/坐标只解析校验不落库（captures 无列，R10+ 落库）
- 迁移 000014 Down 会先把 `source='import'` 修订重映射为 `user`（回滚可逆）；`flutter analyze` 本机崩溃沿用 `dart analyze` 替代
- 真机 / E2E（Ollama smoke）待验证

### 下一步
- 合并 R9（合并提交）
- G8 / R10：Web 回顾端、同步与工作流预留
