# 2026-06-09 时间线探索工作台理想态演进计划

## 理想态

Record Analysis 的 job 详情页应演进为“时间线驱动的聊天分析工作台”，而不是上传文件后等待大报告的表单页面。用户面对几十万条聊天记录时，可以先看到全局时间分布，再通过点击、框选、缩放、插队分析和摘要合并逐步探索。

核心体验是：一根固定宽度、自适应粒度的时间轴贯穿页面；所有消息、摘要、任务状态、token 成本、证据链和历史分析都围绕时间轴联动。

## 用户工作流

1. 上传聊天记录后立即生成 job ID，并进入 job 详情页。
2. 页面展示总消息数、可处理消息数、时间范围、解析状态、索引状态和 token 统计。
3. 时间轴默认按全局范围自动选择合适粒度，大范围显示天/周，小范围显示小时/分钟。
4. 后台开始低优先级预聚合，已完成的 bucket 可立即阅读。
5. 用户点击 bucket/cluster 查看消息预览、词云、摘要、关键事件和证据。
6. 用户框选一段时间后，可以插队生成该范围摘要，或触发连续话题边界探索。
7. 系统优先复用已完成小 bucket 摘要；缺失部分按需补跑，平衡效果、耗时和 token。
8. 分析结果以中文 markdown 摘要优先展示，点击后展开完整结果和证据消息。

## 能力分层

### 1. 数据与索引底座

- Postgres 作为事实状态库：用户、session、job、work item、branch、token、状态流转、唯一约束。
- OpenSearch 作为检索与探索索引：原始消息、摘要、branch report、时间范围检索、全文搜索、时间聚合、词云。
- Postgres 是 source of truth，OpenSearch 可重建、可降级。

### 2. 时间轴 API

- `GET /api/jobs/:id/timeline?start=&end=&granularity=auto`
- `GET /api/jobs/:id/timeline/:bucket_id/messages`
- `POST /api/jobs/:id/branches/preview`
- `GET /api/jobs/:id/work-items?kind=&status=&granularity=`
- `POST /api/jobs/:id/work-items/seed`
- `POST /api/jobs/:id/work-items/:id/prioritize`
- `POST /api/jobs/:id/work-items/merge`

时间轴返回必须包含 bucket 的时间范围、消息数、状态、摘要状态、token 用量、topic hint 和可点击 ID。

### 3. 后台任务系统

- work item 类型：`word_cloud`、`topic_summary`、`summary_merge`、`branch_exploration`、`topic_boundary_detection`。
- 支持 queued/running/completed/failed、priority、claimed_at、completed_at、error、progress、token usage。
- 用户框选或点击后的任务可以插队。
- worker 每次 LLM 调用前后记录 job event 和 token。
- 所有状态都必须可查询，不能只在日志里可见。

### 4. 渐进式 LLM 分析

- 小 bucket 先摘要，例如小时级或分钟级。
- 多个小摘要合并成天/周/月摘要。
- 框选范围时优先复用已有摘要，缺失部分按需补跑。
- 连续话题边界探索用于判断用户选择片段前后最长连续话题范围。
- branch 深挖只针对用户确认的片段执行。
- 所有用户可读结果默认简体中文。

### 5. 前端工作台

- 顶部 job 状态栏：解析、索引、预聚合、worker、token。
- 中部固定宽度时间轴：缩放、拖拽、框选、粒度切换、状态颜色。
- 右侧详情面板：bucket/cluster/range 的摘要、词云、消息预览、任务状态。
- 下方结果区：摘要列表、branch 列表、任务队列、原始消息、搜索结果。
- 摘要默认展示标题和短摘要，展开后 markdown 渲染完整内容。
- 路由刷新必须稳定，`#/job/:id` 直接打开不能白屏。

## 实施里程碑

### Sprint 1：状态可信

- job、work item、branch、token 状态全部从 Postgres 查询。
- work item 去重和插队稳定。
- 前端展示 queued/running/completed/failed、当前处理对象和错误。
- 历史记录、详情路由、刷新恢复稳定。

### Sprint 2：时间轴可用

- 引入成熟时间轴/图表库。
- 固定页面宽度，不被聊天长度撑开。
- auto 粒度真正按范围变化。
- 点击 bucket/cluster 联动详情。
- 框选范围可生成 preview 和 merge work item。

### Sprint 3：渐进摘要

- bucket summary 稳定落库。
- summary merge 支持按范围合并。
- 已完成结果即时展示，未完成可插队。
- 中文输出、markdown 渲染、摘要/全文折叠。
- token 成本按 job/work item/branch 汇总展示。

### Sprint 4：探索式分析

- 话题边界判断。
- branch 去重。
- branch 摘要、全文、证据消息分层展示。
- 支持从摘要跳回时间轴和原始消息。

### Sprint 5：检索与规模化

- 接入 OpenSearch 作为检索索引。
- 消息全文搜索、摘要搜索、branch 搜索。
- 时间轴 date histogram 和词云聚合迁移到 OpenSearch。
- 支持索引状态、重建索引、降级到 Postgres。

## 当前优先级

短期最重要的是先把“探索闭环”做出来：

1. 时间轴点击/框选后，详情区域必须展示对应片段，而不是固定旧值。
2. 每个片段都能插队生成 bucket summary 或 range merge summary。
3. work item 状态、进度、token 和错误必须实时可见。
4. 摘要以中文短摘要优先展示，展开后 markdown 渲染完整结果。

## 实施记录

### 2026-06-09 第一批落地：片段范围内预聚合

- 后端 `SeedWorkItems` 支持 `start_time/end_time` 范围参数。
- `POST /api/jobs/:id/work-items/seed` 支持按当前片段只创建 `word_cloud` 或 `topic_summary` work item。
- 返回结果改为只返回该范围内相关 work item，方便前端局部更新。
- 前端 `seedWorkItems` 支持 range payload。
- job 详情页的“词云预聚合”和“生成片段摘要”改为优先使用当前 `branchPreview` 范围；没有片段时退回当前选中 bucket。
- 前端新增 `mergeWorkItems`，避免局部 seed 返回值覆盖掉页面上其他已有任务。
- 新增 `parseOptionalRFC3339Range` 单元测试。

验证：

- `go test ./...`
- `npm run build`

两项均通过。Vite 仍提示 bundle chunk 偏大，属于既有前端打包优化项。

OpenSearch 是 Sprint 5 的规模化能力，不应阻塞 Sprint 1-3 的核心探索体验。
