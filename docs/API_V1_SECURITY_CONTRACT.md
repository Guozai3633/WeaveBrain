# WeaveBrain API V1 安全契约（R7 变更记录）

> 状态：R7 冻结
> 日期：2026-08-30
> 适用范围：所有 /api/v1 HTTP 接口
> 背景：R7（G5 验收 B）对 /api/v1 遗留域做了跨用户授权收敛。本文档记录行为变更后的**契约**，供前端与集成测试对齐。

## 1. 变更总览

| 接口 | 变更前 | 变更后 | 状态码 |
|---|---|---|---|
| GET /api/v1/users/:id | 任意登录用户可读任意用户资料 | **仅本人**可读，他人一律 404（不泄露目标用户是否存在） | 本人 200 / 他人 404 |
| GET /api/v1/users/me | 不变 | 不变（返回本人） | 200 / 401 |
| GET /api/v1/ideas?project_id= | 只按 project_id 过滤（跨用户读泄漏） | **用户作用域**：仅返回「当前用户拥有的项目」下的 ideas | 200（空列表） |
| POST /api/v1/ideas | 不校验项目归属（跨用户写他人项目） | **项目归属校验**：`project.user_id != 当前用户` → 403 | 201 / 403 |
| GET /api/v1/reminders | 返回系统全部 pending（跨用户读泄漏） | **用户作用域**：仅返回当前用户的 pending 提醒 | 200（空列表） |
| GET /api/v1/agent/workflow/:id | 返回任意 workflow run（跨用户读泄漏） | **仅本人**：`workflow_runs.user_id != 当前用户` → 403 | 200 / 403 / 404 |
| GET /api/v1/users/me/timeline | 恒 401（中间件类型断言 bug） | 修复，返回本人 timeline | 200 / 401 |

## 2. 用户

### GET /api/v1/users/:id

- `:id` 非法 UUID → 400 `invalid user id`。
- 未认证 → 401。
- `id == 当前用户` → 200，返回 `{user}`。
- `id != 当前用户` → **404** `user not found`（与「用户不存在」同响应，避免枚举）。

### GET /api/v1/users/me

- 未认证 → 401。
- 已认证 → 200，返回本人 `{user}`。

## 3. 想法（Ideas）

### GET /api/v1/ideas

查询参数：`project_id`、`q`、`page`、`limit`、`tags`。

- 未认证 → 401。
- 列表结果**限定在当前用户拥有的项目**内：即使 `project_id` 指向他人项目，也返回**空列表**（200），而非他人数据或错误。

### POST /api/v1/ideas

- 未认证 → 401。
- `project_id` 指向的项目存在且属于当前用户 → 201。
- `project_id` 指向他人项目 → **403** `forbidden`。
- `project_id` 不存在 / 缺失 → 沿用既有 4xx/5xx 行为（不含数据串用户）。

## 4. 提醒（Reminders）

### GET /api/v1/reminders

- 未认证 → 401。
- 仅返回当前用户的 `status='pending'` 且 `trigger_time < before` 的提醒。
- 即便系统内存在其他用户的待办，也返回空列表，不泄漏。

### GET /api/v1/reminders/:id 与修改/删除

- 取回后比对 `reminder.user_id != 当前用户` → **403**（既有模式，R7 回归确认）。

## 5. Agent 工作流

### GET /api/v1/agent/workflow/:id

- 未认证 → 401。
- `workflow_runs.user_id != 当前用户` → **403**。
- workflow 不存在 → 404。

## 6. Timeline

### GET /api/v1/users/me/timeline

- 未认证 → 401。
- 已认证 → **200**，返回本人 timeline（R7 修复恒 401 缺陷）。

## 7. 设计说明

- **404 vs 403 语义**：`users/:id` 用 404（不泄露存在性）；ideas 项目归属、reminders、workflow 用 403（资源存在但无权限）。此选择沿用 v1 既有惯例。
- **GetPendingByUser 偏离**：Temporal cron 需要系统级 `GetPendingRemindersActivity`（遍历全用户投递），因此**保留**系统级 `GetPending`；HTTP 层改用新增的 `GetPendingByUser`。详见 R07_REPORT 设计要点。
- **残留暴露（不在本次清单）**：`SearchByTags` / `SearchBySimilarity`（embedding 路径）仍仅 project 作用域，未做用户收口；R7 记为后续工作项。
