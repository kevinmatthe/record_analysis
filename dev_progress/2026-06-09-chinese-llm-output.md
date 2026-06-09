# 2026-06-09 中文化 LLM 输出约束

## 背景

时间线预聚合和片段合并已经通过 `topic_summary` 与 `summary_merge` work item 渐进执行，但实际联调中观察到部分摘要结果输出英文，影响前端阅读一致性。

## 已实施

- 将 `llm/prompts/topic_summary.md` 改为中文 prompt，明确要求 `title`、`summary`、`topics`、`key_events`、`uncertainty` 使用简体中文。
- 将 `llm/prompts/topic_summary_merge.md` 改为中文 prompt，覆盖更高层级摘要合并场景。
- 在 `internal/llm/openai_extractor.go` 中为 `SummarizeTopic` 和 `MergeTopicSummaries` 增加中文输出兜底：
  - 首次输出明显偏英文时，自动追加更强的中文约束并重试一次。
  - 专有名词、产品名、URL、代码标识允许保留原文，避免误伤技术聊天内容。
- 新增单元测试覆盖：
  - 英文 topic summary 自动重试并返回中文。
  - 含 `OpenAI SDK` 等产品名的中文摘要不会被误判。

## 验证

- `go test ./internal/llm`
- `go test ./...`

两项均通过。
