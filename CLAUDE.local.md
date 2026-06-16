# CLAUDE.local.md
# 本地配置 - 仅在开发环境生效, 不提交到 git
# 存放敏感配置和本地路径

# 本地模型配置（开发阶段使用 Ollama）
ANTHROPIC_BASE_URL=http://localhost:11434/v1
ANTHROPIC_API_KEY=ollama

# 本地 Docker 容器名（如有自定义）
DOCKER_COMPOSE_PROJECT_NAME=weavebrain-dev

# 本地数据库端口映射（如自定义）
POSTGRES_PORT=5432
