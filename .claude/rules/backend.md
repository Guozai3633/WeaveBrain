---
paths:
  - "WeaveBrain/**"
---

# 后端开发规则 (Eino + Temporal + Go)

## Eino 框架规则

### 类型对齐 (Type Alignment)
- 所有 Eino 图节点必须声明 Go 结构体的输入输出类型
- **严禁**使用 `map[string]any` 做节点间数据传递
- 类型不匹配时使用 `StatePreHandler` / `StatePostHandler` 转换

### 编排模式选择
- 线性 Pipeline → `Chain` (DAG 模式)
- 分支/循环结构 → `Graph` (Pregel 模式)
- 复杂多步骤 → `Workflow` (DAG 模式)

### ReAct 循环配置
- 推理节点: `tool_choice="auto"`（模型自主决定是否调用工具）
- 执行节点: `tool_choice="required"`（强制调用工具）
- 最大迭代次数: 10
- 使用 `MessageModifier` 注入系统提示词和当前时间戳

### 消息修饰
- `MessageModifier`: 每次 LLM 调用前注入，不持久化到历史
- `MessageRewriter`: 对话超 5000 token 时触发上下文压缩，修改持久化

## Temporal 工作流规则

### 工作流定义
- 工作流函数必须是**确定性的**：不能直接调用 `time.Now()`、`rand.Int()`
- 使用 `workflow.Now(ctx)` 和 `workflow.Sleep(ctx, duration)`
- 使用 `workflow.ExecuteWorkflow` 启动，`workflow.ChildWorkflow` 嵌套

### Activity 规则
- 每个 Activity 是可重试的单位
- 重试策略: `initial_interval=1s`, `max_interval=10s`, `maximum_attempts=3`
- 非重试错误显式列出: `NonRetryableErrors`

### 超时与补偿
- 每个 Activity 设置超时（默认 30s）
- 跨系统操作使用 Saga 模式: 失败时触发补偿操作
- 长周期工作流使用 Signal 通道通信

## API 设计

### REST 端点
- 前缀: `/api/v1/`
- 资源: `/api/v1/ideas`, `/api/v1/projects`, `/api/v1/auth/*`
- WebSocket: `/ws/voice`

### 请求验证
- 所有 POST/PUT 请求必须用 struct + `validate` tag 验证
- 错误响应格式: `{"error": {"code": "INVALID_INPUT", "message": "..."}}`

### 分页
- 列表接口支持 `?page=1&limit=20`
- 游标分页返回 `next_cursor`

## 数据库规则

### 仓库模式
```go
type IdeaRepository interface {
    Create(ctx context.Context, e *entity.Idea) error
    GetByID(ctx context.Context, id int64) (*entity.Idea, error)
    List(ctx context.Context, q Query) ([]*entity.Idea, error)
    Update(ctx context.Context, e *entity.Idea) error
    Delete(ctx context.Context, id int64) error // 软删除
}
```

### 迁移
- 使用 `goose`，每个变更一个文件
- 必须写 Up 和 Down
- 存放在 `internal/db/migration/`

### 命名
- 表名: 复数小写（`ideas`, `projects`）
- 字段: snake_case
- 索引: btree 对外键，GIN 对 JSONB，GIST 对 vector
