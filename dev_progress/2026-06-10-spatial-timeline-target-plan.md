# 空间时间线目标拆解计划

## 目标定义

将 job 详情页演进为一个以“大时间线”为核心的探索工作台：

- 主视觉是一根有空间感的时间线，而不是普通图表。
- 聊天消息按时间聚合成可探索点位。
- 词云、摘要、关键事件作为漂浮 insight 节点围绕主轴展开。
- Branch 是从主时间线长出来的分支，而不是普通列表项。
- 用户点击点位、框选范围、点击 insight 或 Branch 后，固定详情面板立即展示对应内容和操作。
- 所有分析都是探索式、可插队、可渐进生成，而不是一次性等待全量完成。

## 非目标

- 不在当前阶段强行替换所有 ECharts 实现。
- 不一次性接入 OpenSearch 作为主存储。
- 不做复杂 3D 特效优先于业务闭环。
- 不把所有消息一次性送入 LLM。

## Sprint 1：统一右侧详情面板

目标：右侧面板从 Branch 专用升级为 Timeline Detail Panel。

任务：

- 新增前端状态 `selectedTimelineEntity`，支持 `bucket`、`range`、`summary`、`word_cloud`、`branch`。
- 将 `BranchInspector` 重命名/改造为 `TimelineDetailPanel`。
- 点击 Branch 节点时展示 Branch 详情。
- 点击摘要浮动节点时展示摘要详情，包括标题、摘要、topics、key events、tokens。
- 点击词云浮动节点时展示词云详情，包括 top terms、状态、tokens、插队按钮。
- 空态显示“点击时间线节点开始探索”。

验收：

- 点击轴上 Branch、摘要、词云节点，右侧面板内容正确切换。
- 底部列表不再承担主要详情展示。
- `npm run build` 通过。

## Sprint 2：当前片段消息预览

目标：证据预览跟随当前选区，而不是只显示单个 bucket。

任务：

- 新增 `selectedRangeMessages` 状态。
- 使用现有 `/api/jobs/:id/messages/search`，空 query + 当前范围拉取分页消息。
- 当前有 `selectedRange` 或 `branchPreview` 时，消息预览显示“当前片段消息”。
- 单个 bucket 消息降级为 drill-down 小入口。
- 搜索默认限定在当前选区。

验收：

- 框选一段时间后，消息预览展示该范围内消息。
- 点击 Branch 后，消息预览展示 Branch 范围内消息。
- 搜索结果不会跳出当前选区。

## Sprint 3：空间时间线渲染抽象

目标：把时间线渲染从业务页面中抽离，为未来替换 ECharts 做准备。

任务：

- 新建 `web/src/timeline/types.ts`，定义：
  - `TimelineNode`
  - `TimelineInsightNode`
  - `TimelineBranchNode`
  - `TimelineSelection`
- 新建 `web/src/timeline/buildTimelineScene.ts`：
  - 输入 buckets、work items、branches。
  - 输出主轴点、insight 节点、branch 节点、连接线。
- `TimelineChart` 只消费 scene，不再直接拼 work item/branch。
- 保留 ECharts renderer，但命名为 `EChartsTimelineRenderer`。

验收：

- 渲染结果与现有页面一致。
- 后续替换 Canvas/SVG/Three.js 时不需要重写业务数据组装。
- timeline scene builder 有单元测试。

## Sprint 4：更强空间感的时间线

目标：让视觉更接近“电影时间线”，而不是业务图表。

任务：

- 主轴更粗、更有纵深：背景网格、轻微透视感、分层连接线。
- insight 节点做碰撞规避和层级分布。
- Branch 分支从主轴下探，支持短标题、状态色、选中高亮。
- 当前选区用光带/范围罩层呈现。
- 右上角 action dock 保持固定，操作包括下钻、摘要、词云、聚合、Branch。

验收：

- 长时间范围不会横向撑爆页面。
- 100 个 bucket 内保持可读。
- insight 节点不大面积重叠。
- 移动端仍可操作，按钮不遮挡主轴。

## Sprint 5：渐进分析调度闭环

目标：用户可以一点一点生成结果，已完成的先看，未完成的可插队。

任务：

- 当前选区可创建 summary/word_cloud work items。
- insight 面板可对 queued/failed work item 插队。
- Branch 面板可运行深度分析。
- 任务状态实时刷新并反映到时间线节点颜色。
- token 消耗在节点、详情面板、job 概览中一致展示。

验收：

- 创建任务后立即在轴上看到 queued 状态。
- worker 处理完成后节点变为 completed。
- token 消耗不丢失。

## Sprint 6：历史恢复与深链

目标：刷新页面和直接打开 URL 都能恢复工作台上下文。

任务：

- hash/query 记录 job id、选区、选中 entity。
- 直接访问 `#/job/:id?range=...&entity=...` 可恢复时间线状态。
- 历史页展示最近 job、最近 Branch、正在运行任务。
- 点击历史 Branch 直接进入 job 并选中该分支。

验收：

- 刷新不回首页。
- 复制 URL 后能恢复同一 job 和选区。
- 历史页不再只是普通列表。

## Sprint 7：渲染引擎评估与替换决策

目标：决定继续 ECharts，还是切到 Canvas/SVG/Three.js。

评估维度：

- 交互能力：框选、点击、缩放、拖动。
- 节点布局：碰撞规避、连接线、分支。
- 性能：1k、10k、50k bucket 的表现。
- 维护成本：业务状态和渲染状态是否容易隔离。

决策：

- 如果 ECharts 足够支撑 1k bucket + 数百 insight 节点，继续使用。
- 如果空间布局受限，优先自研 Canvas 2D renderer。
- 只有需要真实 3D 旋转/深度时才引入 Three.js。

## 验证策略

- 前端：`npm run build`。
- 后端：`go test ./internal/server ./internal/service -run 'Summary|WorkItem|PreviewBranch|JobBranch|WordCloud'`。
- 场景测试：
  - 上传 job。
  - 加载时间线。
  - 框选范围。
  - 生成摘要。
  - 点击摘要节点。
  - 创建 Branch。
  - 点击 Branch 节点。
  - 刷新页面恢复上下文。

## 当前优先级

下一步先执行 Sprint 1：

1. 把 `BranchInspector` 升级为通用 `TimelineDetailPanel`。
2. 给摘要/词云浮动节点加点击事件。
3. 右侧面板根据节点类型切换内容。

这是最短路径，因为它不要求立刻替换渲染引擎，但能马上让“大时间线 + 浮动节点 + 固定详情面板”的核心动线成立。
