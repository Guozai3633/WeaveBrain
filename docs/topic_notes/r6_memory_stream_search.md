# R6 记忆流中文全文搜索选型决策记录

> 轮次：R6（G4 记忆卡、记忆流与详情）
> 日期：2026-08-30
> 决策人：织脑开发
> 状态：已采纳

## 1. 背景

记忆流列表需要「全文搜索」：用户输入关键词 `q`，在**记忆卡标题**、**捕获原文**、**语音转写文本**三列做子串匹配。产品以中文为主，因此搜索必须支持 CJK 无空格子串（如输入「织脑」应命中「我用织脑记录想法」）。

## 2. 需求与约束

- 中文子串/包含匹配（ILIKE `%q%` 语义）；
- 三列同时命中：`memory_cards.title` / `captures.original_text` / `transcript_revisions.text`；
- 现有技术栈：PostgreSQL（Docker Compose，未加额外扩展依赖）；
- MVP 允许短查询退化（1–2 字符中文）不要求强实时索引。

## 3. 备选方案对比

| 方案 | 中文支持 | 部署成本 | 索引命中 | 结论 |
|---|---|---|---|---|
| `pg_trgm` GIN（`gin_trgm_ops`） | CJK 子串 `ILIKE '%q%'` 可走 GIN；trigram ≥ 3 字符 | **零成本**（contrib 自带，`CREATE EXTENSION`） | 标题/原文/转写各一列 GIN | ✅ 选定 |
| `pg_bigm` GIN | bigram 支持 2 字符子串更稳，但需额外扩展包 | 需安装第三方扩展 | 更好（2-gram） | MVP 不用，可后续换 |
| `zhparser` + 全文检索 | 中文分词 | 需 `zhparser` 扩展 + 分词词典 + 索引同步 | 精确 | 重量级，MVP 过度 |
| 纯 `ILIKE`（无索引） | 任意 | 零 | 顺序扫描 | 数据量大后不可接受 |

## 4. 决策

**选定 `pg_trgm` GIN `gin_trgm_ops`**：

- 是 PostgreSQL 标准 contrib 扩展，`CREATE EXTENSION IF NOT EXISTS pg_trgm` 即用，不引入第三方依赖；
- CJK 子串 `ILIKE '%关键词%'` 经 `gin_trgm_ops` 可用 GIN 索引；每列独立 GIN（`idx_captures_original_text_trgm` / `idx_memory_cards_title_trgm` / `idx_transcript_revisions_text_trgm`，见迁移 000010）；
- `\ % _` 在 `escapeLikePattern` 中转义，保证用户输入按字面子串匹配（memory_repository.go）。

## 5. 已知权衡（记入契约）

- **trigram ≥ 3 字符限制**：PostgreSQL trigram 索引对少于 3 个字符的查询（常见短中文词 1–2 字符）可能不走索引退化为顺序扫描。MVP 数据量小可接受；若后续规模上升，可换 `pg_bigm` 或引入 `zhparser`。
- **游标与搜索**：分页采用 keyset 游标（`base64url(JSON{p,t,i})`，编码 `(is_pinned, created_at, id)` 元组），`next_cursor` 契约要求翻页时携带与首页一致的筛选/搜索参数，否则游标位置无意义（API_V3_CONTRACT §10.2）。

## 6. 关联实现

- 迁移：`WeaveBrain/internal/db/migration/000010_create_memory_revisions_and_search.sql`
- 搜索/分页查询：`WeaveBrain/internal/db/repository/memory_repository.go`（`List`）
- API 契约：`docs/API_V3_CONTRACT.md` §10
