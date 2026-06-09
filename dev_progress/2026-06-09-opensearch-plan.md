# 2026-06-09 OpenSearch 接入子计划与实施记录

## 目标

这是“时间线探索工作台理想态演进计划”的数据检索子计划，对应 Sprint 5：检索与规模化。

保留 Postgres 作为状态与事务事实库，引入 OpenSearch 作为聊天消息、摘要、时间线聚合和全文检索索引。OpenSearch 不阻塞 Sprint 1-3 的核心探索闭环建设。

## 存储边界

Postgres 继续负责用户、session、job、work item、branch、token 用量、状态流转、幂等约束和任务抢占。

OpenSearch 负责原始消息 doc、时间范围检索、全文搜索、date histogram 时间线聚合、词云/高频词、摘要与 branch report 检索，以及后续相似片段/向量检索。

## Index 规划

- `record_analysis_messages`：原始消息，字段包括 `job_id`、`relationship_id`、`msg_id`、`sender`、`display_sender`、`msg_time`、`msg_type`、`content`、`content_clean`、`content_length`、`bucket_minute`、`bucket_hour`、`bucket_day`。
- `record_analysis_topic_summaries`：bucket/merge 摘要，字段包括 `job_id`、`work_item_id`、`scope_id`、`granularity`、`start_time`、`end_time`、`title`、`summary`、`topics`、`key_events`、`evidence_msg_ids`、`confidence`、`model_name`、`total_tokens`。
- `record_analysis_branch_reports`：branch 深挖报告，字段包括 `job_id`、`branch_id`、`title`、`start_time`、`end_time`、`report_markdown`、`topic_hint`、`evidence_msg_ids`、`model_name`、`total_tokens`。
- `record_analysis_timeline_buckets`：可选缓存索引，字段包括 `job_id`、`granularity`、`bucket_id`、`start_time`、`end_time`、`message_count`、`sender_count`、`status`、`summary_status`、`summary_title`、`summary_topics`。

## 推进步骤

1. 新增 `search_config`，支持 endpoint、username、password、index_prefix、enabled。
2. 新增 `internal/search` 包，定义 `Indexer` 接口、no-op 实现和 OpenSearch 实现。
3. 启动 server 时根据配置初始化 search indexer；未配置时使用 no-op。
4. job 上传解析并写入 Postgres 后，bulk index messages 到 OpenSearch；索引失败不让 job 失败，只记录 warning event。
5. work item 完成后把 topic summary / merge summary 写入 OpenSearch。
6. branch 完成后把 branch report 写入 OpenSearch。
7. 增加 index status / rebuild API。
8. timeline、preview、search、word-cloud API 逐步迁移到 OpenSearch，Postgres 保留降级查询。

## 实施记录

- 已创建本计划文件，后续每次代码变更和验证结果追加记录。
