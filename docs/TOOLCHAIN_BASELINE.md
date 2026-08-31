# WeaveBrain 工具链基线

> 状态：R0 基线  
> 日期：2026-08-16

## 1. 编译与 SDK

| 工具 | 基线版本 |
|---|---|
| Go | 1.25.5 windows/amd64 |
| Flutter | 3.44.2 stable |
| Dart | 3.12.2 stable |
| DevTools | 2.57.0 |
| PostgreSQL 主版本 | 16 |
| Redis 主版本 | 7 |

Go 版本由 WeaveBrain/go.mod 固定。Dart SDK 约束由 weave_flutter/pubspec.yaml 固定。

## 2. 标准检查命令

后端：

- 在 WeaveBrain 目录运行 go test ./...；
- 在 WeaveBrain 目录运行 go vet ./...。

Flutter：

- 在仓库根目录运行 scripts/flutter_check.ps1 -Task all；
- 只分析：scripts/flutter_check.ps1 -Task analyze；
- 只测试：scripts/flutter_check.ps1 -Task test；
- 依赖首次未安装时附加 -PubGet。

## 3. 中文路径兼容

当前 Windows Flutter Analysis Server 在中文工作区路径下会出现 LSP JSON 输入截断。scripts/flutter_check.ps1 在检测到非 ASCII 路径时：

1. 从 W:、V:、U: 选择空闲盘符；
2. 临时映射仓库根目录；
3. 通过 ASCII 路径运行 analyze/test；
4. 在 finally 中移除映射；
5. 返回真实分析和测试退出码。

脚本不得覆盖已占用盘符，也不得删除或移动项目文件。

## 4. 容器版本说明

仓库目前存在根目录和 WeaveBrain 目录两份 compose 文件；PostgreSQL 均为 16 系列。Ollama 仍使用 latest，且两份 compose 的 Temporal 版本不同。

这些服务不在 R0 V3 契约测试的运行路径。进入 R13 发布候选前必须：

- 确认唯一正式 compose；
- 将所有 latest 镜像改为固定版本或 digest；
- 删除或明确标记旧 compose；
- 完成数据库备份和恢复演练。
