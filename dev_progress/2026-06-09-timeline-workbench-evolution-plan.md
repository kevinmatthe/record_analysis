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

### 2026-06-09 第二批落地：当前片段任务总览

- 连续片段预览面板新增“当前片段任务”总览。
- 总览聚合当前范围内的 `topic_summary` 和 `summary_merge` work item。
- 展示总任务数、已完成数、排队/运行数、token 总消耗。
- 展示已完成、分析中、排队中、失败的状态 chips。
- 若存在 running 或 failed 任务，展示最近活跃任务的 kind、时间范围和错误信息。
- 提供“插队下一条”按钮，优先插队 queued/failed work item。
- `BucketStatusStrip` 和摘要状态显示改为中文状态文案，减少 raw status 泄露到页面。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'TestDecorateTimelineBucketsWithWorkItems|TestBuildTimelineClustersUsesBucketAnalysisMetadata'` 包编译通过，但当前基线没有匹配到这些测试用例。

### 2026-06-09 第三批落地：词云升级与时间轴视觉增强

- 词云从中文 bigram 频次统计升级为 `gojieba` 中文分词。
- 词云排序改为片段内 TF-IDF：
  - 每条消息作为一个 document 计算 document frequency。
  - 排序使用 `log(1+tf) * idf`，降低纯高频口水词的支配。
  - API 输出仍保留 `count`，前端继续展示原始出现次数。
- 继续过滤用户名、发送者前缀、短词、停用词和明显非文本 token。
- 增加词云单元测试，覆盖中文分词、TF-IDF 特征词排序和用户名过滤。
- 时间轴增加仪表条：
  - 当前时间桶数量。
  - 已完成、排队/运行、失败数量。
  - 状态图例。
  - 当前框选区间提示。
- 时间轴图表背景增加轻量网格和层次化边框，未分析状态改为灰色，避免和已完成混淆。

验证：

- `go test ./internal/service -run WordCloud -v` 通过。
- `go test ./internal/server ./internal/service -run 'WordCloud|Timeline|Decorate'` 通过。
- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。

### 2026-06-09 第四批落地：空片段保护与摘要轨道

- 修复 branch 运行时可能进入 `cannot analyze empty participant message set` 的问题。
- `startBranchRun` 在进入后台 goroutine 前先按 branch 时间窗过滤消息，并校验是否包含 `PERSON_A` 或 `PERSON_B` 消息。
- 空片段直接返回中文错误：当前片段没有可分析的双方聊天文本，需要重新选择包含双方消息的时间段。
- 前端对 `message_count <= 0` 的 branch 禁用“运行分析”，并展示“当前片段没有可分析消息”。
- 时间轴面板顶部新增三步交互动线：选择时间、插队分析、阅读证据。
- 时间轴下方新增“摘要轨道”：
  - 聚合 `topic_summary` 和 `summary_merge` work item。
  - 按时间顺序展示摘要标题、状态和 token。
  - 点击摘要卡片会联动选择对应 bucket 或时间范围。
- 摘要轨道为空时展示清晰的空态说明。

验证：

- `go test ./internal/server ./internal/service -run 'Branch|Timeline|WordCloud|Participant'` 通过。
- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。

### 2026-06-09 第五批落地：微信结构清洗与整段周期摘要范围修复

- 新增 `internal/textclean`，集中清洗微信 XML/appmsg/CDATA/结构化字段噪音。
- 清洗目标包括 `nickname`、`type`、`appid`、`msg`、`cdata`、`null`、`msgid` 等微信结构字段。
- 新导入消息：
  - `RawContent` 保留原始 payload。
  - `Content` 写入清洗后的自然语言文本。
  - 清洗后没有自然语言的结构消息会被过滤。
- 旧数据库消息：
  - Postgres 读取 job messages 时动态清洗。
  - 非自然文本不再进入时间轴、词云、summary 和 branch 分析。
- LLM 输入层二次防护：
  - `llmMessages` 会跳过微信结构噪音。
  - 传给模型的 `content` 使用清洗后的文本。
- 词云层二次防护：
  - 词云构建前再次清洗并过滤非自然文本。
- 修复“片段摘要只生成某一天”的范围问题：
  - 后端 `PreviewBranch` 不再返回第一个重叠 cluster，而是按用户请求范围聚合所有重叠 bucket。
  - 前端只要存在框选 `selectedRange`，生成摘要、聚合摘要、搜索都优先使用框选范围，而不是被 `branchPreview` 缩窄后的范围。
  - 当前片段摘要列表按框选范围包含的 work item 展示。

验证：

- `go test ./internal/textclean ./internal/importer ./internal/llm ./internal/service -run 'Clean|WeChat|WordCloud|LLMMessages|NormalizeRows'` 通过。
- `go test ./internal/service -run PreviewBranch -v` 通过。
- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。

### 2026-06-10 第六批落地：清洗统计可见性与范围语义固定

- `importer.ParseChatFileWithStats` 新增解析统计：
  - 原始行数。
  - 清洗后保留行数。
  - 过滤行数。
  - 参与者消息数。
  - 系统消息数。
- `ParseChatFile` 保持兼容，内部调用带统计的新函数。
- job 创建后新增事件：
  - `解析完成：N 条可处理消息`
  - `文本清洗：原始 X 条，保留 Y 条，过滤微信结构/非文本噪音 Z 条`
- 前端当前片段面板新增“操作对象”提示：
  - 当前框选的完整时间范围。
  - 当前连续对话簇。
  - 当前选中时间桶。
- server 测试更新到新的范围语义：branch preview/create 不再扩展到相邻 cluster，而是严格使用用户选择范围。

验证：

- `go test ./internal/server ./internal/service -run 'JobBranch|PreviewBranch|Branch'` 通过。
- `go test ./internal/textclean ./internal/importer ./internal/llm ./internal/service -run 'Clean|WeChat|WordCloud|LLMMessages|NormalizeRows|PreviewBranch|ParseStats'` 通过。

### 2026-06-10 第七批落地：完整周期摘要覆盖度保护

- 修复“合并摘要看起来只总结某一天”的另一个根因：`summary_merge` 以前只合并范围内已经完成的 bucket 摘要，即使缺失大部分周期也会继续生成。
- 后端 `processSummaryMergeWorkItem` 新增覆盖度校验：
  - 先按 merge work item 的完整时间范围重新计算应覆盖的 bucket。
  - 查询该范围内已完成的 `topic_summary`。
  - 如果已完成摘要数少于应覆盖 bucket 数，merge 任务失败并提示先生成缺失 bucket 摘要。
- 前端当前片段任务总览新增“摘要覆盖”：
  - 显示 `已完成/应覆盖`。
  - 缺失时展示 warning。
  - “聚合完整周期摘要”按钮只有覆盖完整时才可用。
- “生成片段摘要”按钮在缺失时显示缺失数量，例如 `生成缺失摘要 (3)`。

验证：

- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。
- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。

### 2026-06-10 第八批落地：摘要轨道折叠与页面自适应

- 摘要轨道从平铺 grid 改为横向 scroll-snap 轨道，避免长周期摘要把页面撑得过长。
- 摘要轨道默认只展示前 6 个结果。
- 超过 6 个时提供“展开全部 / 折叠摘要轨道”按钮。
- 摘要卡片固定响应式宽度，标题最多两行截断。
- 粒度切换 segmented control 支持横向滚动，避免小屏挤压。
- 时间轴操作按钮支持换行。
- 移动端优化：
  - 三步交互动线改为单列。
  - 时间轴统计信息改为更紧凑布局。
  - 摘要轨道卡片宽度适配移动 viewport。
  - 操作对象提示在窄屏下改为上下布局。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

OpenSearch 是 Sprint 5 的规模化能力，不应阻塞 Sprint 1-3 的核心探索体验。

### 2026-06-10 第九批落地：理想态典型动线与运行分析闭环

理想态页面动线固定为：

1. 上传聊天记录，系统完成清洗、解析、对象存储落盘，并立即生成可查询 job。
2. 进入 job 详情，先看到全局时间轴、可处理消息量、过滤量、任务状态和 token 消耗。
3. 用户通过缩放、粒度切换、点击点位或框选时间范围，确定当前探索片段。
4. 当前片段面板展示消息预览、词云、摘要覆盖度、队列状态和已完成摘要。
5. 用户对选中范围插队生成 bucket 摘要；已完成的 bucket 立即可读，未完成的继续排队。
6. 当选中范围的 bucket 摘要覆盖完整后，允许聚合生成完整周期摘要。
7. 用户基于某个片段创建 branch，运行深度分析，并在详情页阅读 Markdown 报告和证据。
8. 后续所有分析、branch、work item 都可在历史记录里恢复，刷新页面不丢上下文。

本批修复：

- Postgres 消息表当前存储的是展示名，branch 分析回读时会动态恢复为 `PERSON_A`/`PERSON_B` 等参与者 ID。
- 修复有效聊天片段被误判为 `cannot analyze empty participant message set` 的问题。
- branch 分析启动前补齐 `RelationshipID`，避免报告上下文为空。
- 前端 `运行分析` 请求失败时直接把错误写回 branch 状态，避免用户看不到失败原因。

下一步推进：

- 让当前片段面板成为主交互入口：点击时间轴点位后，词云、摘要、消息预览、work item 覆盖度必须全部随选区变化。
- 将摘要轨道进一步压缩为“摘要卡片 + 展开抽屉”，默认只展示中文摘要和状态。
- 将 job 历史页强化为可恢复工作台，而不是简单列表。

### 2026-06-10 第十批落地：片段联动与探索工作台摘要条

- 修复框选范围后词云不随选区变化的问题：
  - `selectedRange` 存在时，词云 work item 按 `start_time/end_time` 落在选区内过滤。
  - 没有框选但存在 branch preview 时，词云同时支持按 bucket id 和时间范围匹配。
- 时间轴下方新增“当前探索片段”摘要条：
  - 展示当前操作对象类型：框选范围、连续对话簇、单个时间桶。
  - 展示当前时间范围、主题提示、消息数、覆盖 bucket 数、摘要覆盖度、词云任务数。
  - 目的是让用户在执行“生成摘要/聚合/保存 Branch”前明确知道操作对象。
- 摘要轨道增加选中态：
  - 当前选区内的摘要卡片会高亮。
  - 点击摘要卡片仍会反向选择对应范围。
- 片段摘要列表默认只展示 3 个完成摘要，超过后手动展开，避免长周期结果撑爆页面。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 将“桶内消息”升级为“当前片段消息预览”，默认使用当前 `selectedRange/branchPreview` 拉取分页消息。
- 保留单桶消息作为次级 drill-down，而不是主证据入口。
- 历史页改造为可恢复工作台：最近 job、最近 branch、正在运行 work item 分区展示。

### 2026-06-10 第十一批落地：大时间线舞台第一版

用户目标：

- 页面主角应是一根很大的时间轴，类似电影里的时间线。
- 时间轴上有大量可探索点位。
- 点位附近漂浮词云、摘要、节点，而不是只在下方列表展示。
- 用户点击或框选后，在固定位置直接看到下钻、分析、聚合、保存 branch 等操作。

本批实现：

- 时间轴区域升级为 `timelineStage` 主舞台：
  - 高度从普通图表提升为 26rem。
  - 保留一根清晰横向时间线。
  - 增加背景网格、轻量发光和空间层次，弱化表格感。
- ECharts 图层升级：
  - 主轴仍是 line + scatter。
  - 新增虚线轨迹层，让时间点有漂浮感。
  - 新增浮动节点 scatter：
    - 已完成 bucket 摘要显示为 `摘要 xxx`。
    - 已完成词云显示 top terms，例如 `关键词A / 关键词B / 关键词C`。
    - 节点在时间轴上下多 lane 漂浮，最多采样展示 30 个，避免长周期过载。
- 右上角新增 `timelineActionDock` 固定操作台：
  - 下钻：对当前选区进入局部小时级时间线。
  - 摘要：对当前片段生成 bucket 摘要。
  - 聚合：摘要覆盖完整后生成完整周期摘要。
  - Branch：保存当前片段。
- 交互语义调整：
  - 框选时间范围不再自动下钻，避免用户刚框选就被强制切走视图。
  - 下钻改为显式按钮，由用户确认。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。
- `curl --noproxy '*' -I http://localhost:5173/` 返回 200。
- 当前环境未安装 `agent-browser`、`playwright` 或 `puppeteer`，未完成截图级视觉验证。

下一步推进：

- 给时间线浮动节点增加点击行为：点击词云/摘要节点后，右侧直接切到对应结果详情。
- 将“当前片段消息预览”接到同一选区，形成证据闭环。
- 对超大数据量引入节点聚合/碰撞规避，避免浮动节点重叠。

### 2026-06-10 第十二批落地：Branch 作为时间线分支

用户校准：

- 核心不是必须使用 ECharts，而是要形成有空间感的大时间轴。
- Branch 生成后不应只出现在下方列表里，而应作为从主时间线长出来的分支。

本批实现：

- 暂时继续基于 ECharts 推进，避免过早替换渲染引擎导致业务语义中断。
- `TimelineChart` 新增 Branch 图层：
  - Branch 按 `start_time/end_time` 的中点挂载到主轴对应位置。
  - Branch 节点固定漂浮在主轴下方多条 lane 上。
  - 使用 dotted guide line 从主轴垂到分支节点，形成“主干 -> 分支”的空间关系。
  - 节点 label 显示 `分支 xxx`。
  - 节点颜色随状态变化：completed/running/failed/default。
- Branch 点击行为：
  - 点击轴上的分支节点，会选中对应时间范围。
  - 同步展开下方已保存 Branch 的结果。
  - 选中的 Branch 节点在轴上加粗高亮。
- 框选行为修正：
  - Brush 选择明确只读取 `buckets` series，避免浮动节点/分支节点影响框选范围。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 点击摘要/词云浮动节点时，直接打开对应摘要或词云详情。
- 把“已保存 Branch”列表改成分支详情抽屉或右侧固定面板，减少页面向下滚动。
- 如果 ECharts 对空间感限制明显，再把时间轴主舞台抽象成独立 renderer，可切换到 Canvas/SVG/Three.js。

### 2026-06-10 第十三批落地：Branch 右侧固定详情面板

目标：

- 时间轴仍然是主舞台。
- 点击轴上的 Branch 后，不应要求用户滚到页面底部阅读结果。
- Branch 列表应变成索引，详情应在固定位置承接。

本批实现：

- 新增 `BranchInspector`：
  - 桌面端作为时间线工作区右侧 sticky 面板。
  - 移动端自然落为单列。
  - 未选中 Branch 时展示空态。
  - 选中 Branch 后展示标题、时间范围、消息数、状态、token、摘要、运行按钮、错误和 Markdown 报告。
- 时间线工作区重排：
  - `timelineWorkspaceMain` 保留当前片段、桶信息、摘要、词云、消息搜索。
  - `BranchInspector` 固定在右侧，承接轴上分支节点点击结果。
- 底部“已保存 Branch”降级为索引：
  - 保留运行分析入口。
  - “查看结果/收起结果”改为“打开面板/关闭面板”。
  - 不再在列表内展开 Markdown 报告，避免页面过长。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 摘要/词云浮动节点点击后，也应打开同一个右侧详情面板，只是内容类型切换为 insight。
- 当前片段消息预览需要接入同一选区，形成“轴上选择 -> 右侧详情 -> 下方证据”的一致动线。

### 2026-06-10 第十四批落地：下方面板折叠为时间轴浮动面板

问题：

- 时间轴已经成为主视觉，但“选中时间桶 / 连续片段预览 / 桶内消息 / 搜索”等内容仍以窄列形式堆在时间轴下方。
- 这会把用户视线从主时间轴拉走，且布局显得割裂。

本批实现：

- 在 `timelineStage` 内新增 `timelineFloatDock`：
  - `当前片段`
  - `时间桶`
  - `证据`
- 点击 dock 按钮后，在时间轴舞台内展开对应 `timelineFloatPanel`。
- `当前片段` 浮动面板展示：
  - 当前操作对象类型。
  - 选区时间范围。
  - 主题提示。
  - 消息数、bucket 数、摘要覆盖、词云任务数。
- `时间桶` 浮动面板展示：
  - 当前 bucket 时间范围。
  - bucket preview / summary title。
  - 消息数、参与人、摘要状态、tokens。
  - bucket topics。
- `证据` 浮动面板展示：
  - 当前 bucket 的前几条消息证据。
- 原 `timelineWorkspaceMain` 暂时隐藏，避免窄面板继续挤在下方。
- 主舞台高度从 26rem 提升到 34rem，移动端提升到 31rem，给浮动面板和时间轴留出空间。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 将右侧 Branch 面板也进一步融入时间轴主舞台，形成真正的“右侧悬浮详情”而不是下方工作区。
- 把浮动面板里的证据预览从 bucket 级升级到当前选区级。
- 给浮动面板增加 insight 类型：点击摘要/词云节点后直接展开详情。

### 2026-06-10 第十五批落地：右侧详情融入主舞台与 Insight 节点点击

目标：

- 时间轴是唯一主舞台。
- Branch、摘要、词云节点点击后，详情都在时间轴舞台内展开。
- 不再依赖时间轴下方区域展示核心详情。

本批实现：

- 新增 `selectedInsightID` 状态。
- 时间轴浮动 insight 节点从“按 bucket 合并标签”改为“一个 work item 一个节点”：
  - 节点携带 `work_item.id`。
  - 点击节点可定位到具体摘要或词云任务。
- `TimelineChart` 新增：
  - `selectedInsightID`
  - `onSelectInsight`
  - insight 节点选中高亮。
- 点击摘要/词云节点后：
  - 选中对应 work item。
  - 清空 Branch 选中态。
  - 同步选中该 work item 的时间范围。
  - 尝试定位对应 bucket。
- 新增 `InsightInspector`：
  - 摘要节点展示 title、summary、topics、key events、uncertainty、tokens。
  - 词云节点展示 top terms。
  - queued/failed 节点支持插队处理。
- `BranchInspector` 和 `InsightInspector` 统一通过 `timelineStageInspector` 在时间轴舞台右侧悬浮展示。
- 原下方 `BranchInspector` 移除，不再作为时间轴外的详情区。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 将 `证据` 浮动面板从 bucket 级升级为当前选区级。
- 时间线节点过多时增加碰撞规避和层级压缩。
- 把 `TimelineChart` 的 scene 构建逻辑抽到 `web/src/timeline/buildTimelineScene.ts`，为未来替换渲染引擎做准备。

### 2026-06-10 第十六批落地：证据面板升级为当前选区级

目标：

- 框选范围、点击 Branch、点击摘要/词云节点后，证据预览都应围绕同一个时间范围。
- 不再只展示单个 bucket 的消息。

本批实现：

- 新增 `rangeMessages`、`rangeMessagesPage`、`loadingRangeMessages` 状态。
- 复用现有 `/api/jobs/:id/messages/search`：
  - query 传空字符串。
  - range 使用当前 `selectedRange/branchPreview/selectedBucket`。
  - page size 使用 8。
- 打开 `证据` 浮动面板时自动加载当前范围消息。
- 当前范围变化时重置证据分页。
- `证据` 面板展示：
  - 当前范围时间。
  - 当前范围内消息总数。
  - 前 6 条证据消息。
  - 上一页/下一页分页按钮。
- 如果当前范围消息未加载，才 fallback 到 bucket messages。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 将时间线 scene 构建从 `App.tsx` 中拆出，降低主组件复杂度。
- 增加 insight 节点碰撞规避和分层压缩，避免长周期节点互相遮挡。

### 2026-06-10 第十七批落地：Timeline Scene Builder 与轻量碰撞规避

目标：

- 降低 `App.tsx` 中时间线数据拼装复杂度。
- 为未来替换 ECharts renderer 做准备。
- 先保持视觉和交互等价，不同时改变渲染引擎。

本批实现：

- 新增 `web/src/timeline/types.ts`：
  - `TimelineBucketNode`
  - `TimelineInsightNode`
  - `TimelineBranchNode`
  - `TimelineScene`
- 新增 `web/src/timeline/buildTimelineScene.ts`：
  - 输入 buckets、word cloud work items、summary work items、branches。
  - 输出统一 scene：主轴 bucket 节点、insight 节点、branch 节点。
- `TimelineChart` 改为消费 `buildTimelineScene` 输出。
- 从 `App.tsx` 移除本地 `timelineFloatNodes`、`timelineBranchNodes`、`insightNodeTime`。
- insight 节点分配逻辑调整：
  - 一个 work item 一个节点。
  - 先按时间排序，再采样。
  - 根据时间间隔选择可用 lane。
- Branch 节点分配逻辑调整：
  - 按时间中点排序。
  - 根据时间间隔选择可用 lane。
  - 减少同一时段分支节点重叠。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 将 ECharts renderer 从 `App.tsx` 拆到独立文件。
- 让 `App.tsx` 只保留页面状态和业务动作，不再承载图表配置细节。

### 2026-06-10 第十八批落地：合并摘要进入 Insight 图层

目标：

- 完整周期摘要不应只在下方面板或摘要轨道中出现。
- `summary_merge` 结果应和 bucket 摘要、词云一样，成为时间轴上的可点击 insight 节点。

本批实现：

- `TimelineChart` 的 `summaryItems` 输入改为 `summaryItems + mergeItems`。
- `buildTimelineScene` 现有摘要识别逻辑兼容 `summary_merge` 的 result 结构，因此无需后端改动。
- 点击合并摘要节点后，仍进入右侧 `InsightInspector`。

验证：

- `npm run build` 通过，仍有既有 Vite chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 拆出 ECharts renderer。
- 给 timeline 模块增加更明确的边界说明。

### 2026-06-10 第十九批落地：ECharts Renderer 拆分与懒加载

目标：

- `App.tsx` 只保留页面状态和业务动作。
- ECharts 配置迁移到独立 renderer。
- 降低首屏业务 bundle 体积。

本批实现：

- 新增 `web/src/timeline/EChartsTimelineRenderer.tsx`：
  - 接收 buckets、work items、branches 和 selection props。
  - 内部调用 `buildTimelineScene`。
  - 内部负责 ECharts option、click、brush、resize、dispose。
- `App.tsx` 改为渲染 `EChartsTimelineRenderer`。
- 从 `App.tsx` 删除内联 `TimelineChart` 和渲染器专用 helper。
- ECharts 改为组件内 `import('echarts')` 动态加载。
- `EChartsTimelineRenderer` 增加舞台内 loading overlay，首次加载图表 chunk 时不再空白。
- bundle 结果：
  - 主业务 chunk 从约 1.39MB 降到约 267KB。
  - ECharts 被拆到独立懒加载 chunk，仍然超过 500KB，但不再阻塞主业务入口。

验证：

- `npm run build` 通过。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 增加 renderer 懒加载中的舞台内 loading 状态。
- 继续把时间线相关组件从 `App.tsx` 拆出，降低主页面体积。

### 2026-06-10 第二十批落地：摘要轨道收进时间轴主舞台

目标：

- 时间线下方不再堆列表式摘要轨道。
- 摘要轨道成为主舞台的一部分，继续强化“大时间线 + 周边悬浮组件”的交互形态。

本批实现：

- 将 `TimelineSummaryRail` 从 `timelineStage` 外部移入主舞台内部。
- 新增 `timelineSummaryOverlay`：
  - 桌面端位于主舞台底部中间，避开左下角浮动面板 dock。
  - 移动端位于底部 dock 上方。
  - 使用半透明背景和 blur，保持悬浮感。
- 点击摘要轨道卡片时：
  - 选中对应时间范围。
  - 选中对应 insight id。
  - 清空 Branch 选中态。
  - 尝试定位对应 bucket。

验证：

- `npm run build` 通过，仍有 ECharts 懒加载 chunk 偏大警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 移动端浮动层避让优化。
- 继续拆出时间线相关 UI 子组件。

### 2026-06-10 第二十一批落地：Timeline Renderer 与 Summary Rail 组件化

目标：

- 继续削减 `App.tsx` 的时间线 UI 细节。
- 将渲染器、scene builder、摘要轨道收敛到 `web/src/timeline` 模块。

本批实现：

- 新增 `web/src/timeline/EChartsTimelineRenderer.tsx`：
  - 承接原 `TimelineChart` 的 ECharts 配置和事件处理。
  - 内部消费 `buildTimelineScene`。
  - 内部动态加载 `echarts`。
  - 内部展示 renderer loading overlay。
- 新增 `web/src/timeline/TimelineSummaryRail.tsx`：
  - 承接摘要轨道展示、展开/折叠、选中态。
  - 保持原交互语义不变。
- 新增 `web/src/timeline/index.ts`：
  - 作为 timeline 模块统一导出入口。
- `App.tsx` 改为从 `./timeline` 导入 `EChartsTimelineRenderer` 和 `TimelineSummaryRail`。
- `App.tsx` 删除内联 ECharts renderer 和摘要轨道组件。

验证：

- `npm run build` 通过，ECharts 仍作为独立懒加载 chunk 超过 500KB。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 抽出舞台内浮动面板组件。
- 抽出右侧详情 inspector 组件。

### 2026-06-10 第二十二批落地：Timeline 浮动面板与 Inspector 组件化

目标：

- 继续把时间轴主舞台周边 UI 从 `App.tsx` 拆出。
- 保持“大时间线主体 + 周边悬浮操作/阅读面板”的交互方向不变。

本批实现：

- 新增 `web/src/timeline/TimelineFloatPanels.tsx`：
  - 承接当前片段、时间桶、证据预览三个浮动面板。
  - 证据分页仍由 `App.tsx` 管理，组件只负责展示和翻页回调。
- 新增 `web/src/timeline/TimelineInspectors.tsx`：
  - 承接 `BranchInspector` 和 `InsightInspector`。
  - Branch 报告继续使用 markdown 懒加载渲染。
  - Insight 支持摘要结果和词云结果两类展示。
- 更新 `web/src/timeline/index.ts`：
  - 统一导出 renderer、summary rail、floating panels、inspectors、scene builder 和类型。
- 更新 `web/src/App.tsx`：
  - 删除内联浮动面板 JSX。
  - 删除内联 Branch/Insight inspector。
  - 继续保留任务状态、选区、证据分页、运行分析等数据编排逻辑。

验证：

- `npm run build` 通过；ECharts/markdown 懒加载 chunk 仍有体积警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 抽出时间轴操作 dock 和 header 控件，进一步减小 `App.tsx`。
- 优化舞台悬浮层在窄屏和中等宽度窗口下的避让关系。

### 2026-06-10 第二十三批落地：Timeline Header 与 Action Dock 组件化

目标：

- 继续将时间轴交互控件收敛到 `web/src/timeline`。
- 让 `App.tsx` 更聚焦在数据加载、状态选择和页面路由。

本批实现：

- 新增 `web/src/timeline/TimelineControls.tsx`：
  - `TimelineHeaderControls` 承接粒度切换、缩放、返回全局、词云预聚合入口。
  - `TimelineActionDock` 承接下钻、摘要、聚合、保存 Branch 操作。
- 更新 `web/src/timeline/index.ts`：
  - 统一导出 `TimelineHeaderControls` 和 `TimelineActionDock`。
- 更新 `web/src/App.tsx`：
  - 删除内联 `timelineHeader` 控制块。
  - 删除内联 `timelineActionDock` 控制块。
  - 保留原来的回调和权限判断，避免改变业务行为。

验证：

- `npm run build` 通过；仍有 ECharts/markdown 懒加载 chunk 体积警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 继续处理窄屏/中屏下悬浮面板互相遮挡的问题。
- 将隐藏的旧 workspace 内容逐步拆除或改造成可按需展开的辅助抽屉。

### 2026-06-10 第二十四批落地：中等宽度时间轴悬浮层避让

目标：

- 修正平板/窄桌面宽度下，操作 dock、详情 inspector、摘要轨道、浮动面板互相挤压的问题。
- 保持时间轴主体可读，不让周边面板把主画布完全盖住。

本批实现：

- 在 `web/src/styles.css` 新增 `768px - 1279px` 断点：
  - 时间轴舞台高度提升到 `36rem`。
  - 操作 dock 横向铺开到舞台顶部，避免右上角堆叠。
  - 当前片段/时间桶浮动面板下移，避开顶部操作 dock。
  - 证据面板和 inspector 避开底部摘要轨道。
  - 摘要轨道固定在底部 dock 上方，并限制最大高度可滚动。
- 保留移动端已有底部 dock 布局，不改变手机端规则。

验证：

- `npm run build` 通过；仍有 ECharts/markdown 懒加载 chunk 体积警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 把旧隐藏 workspace 中仍有价值的能力迁移成正式浮动抽屉：
  - Branch 标题编辑。
  - 当前片段任务概览。
  - 桶内消息/消息搜索。
- 迁移完成后删除隐藏 DOM，避免维护两套交互。

### 2026-06-10 第二十五批落地：旧 Workspace 迁移为时间轴辅助抽屉

目标：

- 删除已经隐藏的旧 workspace DOM，避免页面维护两套相同能力。
- 将 Branch 标题、片段任务、桶内消息、消息搜索迁入时间轴主舞台。

本批实现：

- 新增 `web/src/timeline/TimelineUtilityDrawer.tsx`：
  - 提供舞台内辅助 dock：范围、任务、消息、搜索。
  - 提供可复用抽屉外壳，内容由 `App.tsx` 按当前状态注入。
- 更新 `web/src/App.tsx`：
  - 新增 `openUtilityPanel` 状态。
  - 将 Branch 标题编辑、保存 Branch、任务概览、摘要/聚合入口、词云结果、桶内消息分页、消息搜索迁入抽屉。
  - 删除原 `timelineWorkspace` / `timelineWorkspaceMain` 隐藏 DOM。
- 更新 `web/src/styles.css`：
  - 新增 `timelineUtilityDock` 和 `timelineUtilityDrawer` 样式。
  - 桌面端抽屉贴近舞台右侧，中屏/移动端转为底部工具入口。
  - 移动端摘要轨道上移，避开新增工具 dock。
  - 删除旧 workspace 和 active scope 未使用样式。

验证：

- `npm run build` 通过；仍有 ECharts/markdown 懒加载 chunk 体积警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 把抽屉内容继续组件化，减少 `App.tsx` 中的大块 JSX。
- 处理 Branch 索引：从列表式面板进一步转成时间轴上的分支管理层或可折叠侧栏。

### 2026-06-10 第二十六批落地：Branch 索引收进时间轴抽屉

目标：

- Branch 已经作为节点出现在时间轴上，列表索引不应继续占用主页面下方空间。
- 将 Branch 管理能力变成时间轴舞台的按需打开层。

本批实现：

- 扩展 `TimelineUtilityDrawer`：
  - 新增 `branches` 面板类型。
  - 工具 dock 新增“分支”入口。
  - 移动端工具 dock 从四列改为五列。
- 更新 `web/src/App.tsx`：
  - 将原底部 Branch 列表迁入“分支”抽屉。
  - 保留运行分析、打开/关闭 inspector、错误展示、token 展示等能力。
  - 删除 `branchIndexPanel` 页面块。
- 更新 `web/src/styles.css`：
  - 删除 `branchIndexPanel` 残留样式。
  - 新增 `utilityBranchList` 抽屉内列表样式。

验证：

- `npm run build` 通过；仍有 ECharts/markdown 懒加载 chunk 体积警告。
- `go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'` 通过。

下一步推进：

- 将抽屉各面板内容拆成组件，进一步瘦身 `App.tsx`。
- 根据真实页面截图继续微调舞台层级关系，尤其是抽屉、inspector、摘要轨道同时打开时的优先级。
