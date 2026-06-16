---
paths:
  - "WeaveFlutter/**"
---

# 前端开发规则 (Flutter)

## 目录结构

### 功能优先 (Feature-First)
```
lib/features/{feature_name}/
├── ui/          # 页面、Widget
├── domain/      # Use Cases
└── data/        # Repositories, Models
```

### 组件提取
- 提取可复用 Widget 到独立文件
- **优先提取 Widget 而非方法**（`Extract widgets, not methods`）
- 复杂 Widget 内联会导致状态丢失和重绘异常

## 状态管理

### Riverpod
- 使用 `riverpod` 做 DI 和状态管理
- 每个页面对应一个 `*Notifier` 类
- 使用 `sealed` 类表示状态:
```dart
sealed class IdeaState {}
final class IdeaLoading extends IdeaState {}
final class IdeaLoaded extends IdeaState {}
final class IdeaError extends IdeaState {}
```

### 导航
- 使用 `go_router` 声明式路由
- 命名路由 + 路径参数

## 空安全 (Null Safety)

### 绝对禁止 `!` 操作符
- **EVERYONE USE `!` IS A BUG**
- 使用模式匹配处理可空: `if (value != null) { ... }`
- 使用 `switch` 模式匹配: `switch (value) { String s => ..., null => ... }`
- 使用 `?` 运算符: `value?.length ?? 0`

### 构造函数字段
- 必须使用的参数加 `required`
- 仅在初始化确定后使用 `late`

## 不可变性

- 所有状态字段使用 `final`
- 尽可能使用 `const` 构造函数
- 自定义对象实现 `==` / `hashCode` 或使用 `Object.hash()`

## 测试要求

| 层级 | 最低覆盖率 | 说明 |
|---|---|---|
| Domain 层 | 70% | Use Case, Repository 实现 |
| Presentation 层 | 50% | 复杂 Widget 测试 |
- 测试中 mock HTTP/WebSocket，不碰真实网络
- 运行测试用静默模式，仅报告失败项和覆盖率
