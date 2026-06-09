# Timeline Sprint 1 Progress

Date: 2026-06-09

## Implemented

- Added backend timeline bucketing in `internal/service/timeline.go`.
- Supported granularities:
  - `hour` (default)
  - `day`
  - `15m`
  - `5m`
- Added bucket metadata:
  - `id`
  - `granularity`
  - `start_time`
  - `end_time`
  - `message_count`
  - `participant_count`
  - `participant_messages`
  - `first_message_id`
  - `last_message_id`
  - `preview`

## API Additions

- `GET /api/jobs/{id}/timeline?granularity=hour`
- `GET /api/jobs/{id}/timeline/{bucket_id}/messages?page=1&page_size=50`

These work for both:

- in-memory jobs
- PostgreSQL-backed jobs

For PostgreSQL-backed jobs, timeline reads currently reuse persisted `record_analysis_job_messages` and compute buckets on read.

## Token Usage Tracking

- Added LLM usage callback plumbing via `internal/llm/usage.go`.
- `OpenAICompatibleExtractor` now reports:
  - `prompt_tokens`
  - `completion_tokens`
  - `total_tokens`
- Job state now accumulates token totals in:
  - `prompt_tokens`
  - `completion_tokens`
  - `total_tokens`
- PostgreSQL `record_analysis_jobs` struct was extended with the same columns, so `AutoMigrate` can add them.

## Verification

Passed:

```bash
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache \
GOMODCACHE=/mnt/RapidPool/tmp/record_analysis_gomodcache \
go test ./...
```

```bash
cd web
npm run build
```

## Frontend MVP Added

- History page now opens job detail by default.
- Added hash route:
  - `#/job/<job_id>`
- Added timeline job detail workspace:
  - live job status
  - progress bar
  - token counters
  - time granularity switch
  - timeline buckets
  - selected bucket detail
  - paginated messages inside selected bucket

This is still Sprint 1/2 level functionality: it lets users see the timeline and inspect buckets immediately, but it does not yet support brush selection, clustering overlays, or branch analysis.

## Exploratory Increment Added

- Added heuristic `timeline_clusters` in backend service code, computed from adjacent buckets.
- Added branch preview API:

```text
POST /api/jobs/{id}/branches/preview
```

- Preview input:
  - `granularity`
  - `start_time`
  - `end_time`
- Preview output:
  - expanded `start_time`
  - expanded `end_time`
  - `message_count`
  - `bucket_ids`
  - `cluster_id`
  - `topic_hint`
  - `status`

- Timeline API now also returns `clusters`.
- Frontend job detail page now renders:
  - cluster cards under the timeline
  - selected cluster preview
  - expanded candidate segment summary

This is still not full branch execution. It is the first exploration layer: users can see likely continuous segments and inspect the backend-expanded candidate window before any deeper analysis runs.

## Branch Persistence Increment

- Added branch creation and listing APIs:

```text
POST /api/jobs/{id}/branches
GET  /api/jobs/{id}/branches
```

- Added `AnalysisBranch` backend model with:
  - `id`
  - `job_id`
  - `relationship_id`
  - `title`
  - `granularity`
  - `start_time`
  - `end_time`
  - `message_count`
  - `bucket_ids`
  - `cluster_id`
  - `topic_hint`
  - `status`

- In-memory path supports branch create/list.
- PostgreSQL path now has a code-defined `record_analysis_branches` table and store methods.
- Frontend job detail page now supports:
  - saving current branch preview as a Branch
  - listing saved Branch objects under the timeline workspace

This still stops short of branch execution. A saved Branch is now a durable exploration object, ready to become its own analysis job in the next increment.

## Branch Execution Increment

- Added branch run API:

```text
POST /api/jobs/{job_id}/branches/{branch_id}/run
```

- Branches now carry execution fields:
  - `stage`
  - `progress`
  - `prompt_tokens`
  - `completion_tokens`
  - `total_tokens`
  - `report_markdown`
  - `model_name`
  - `error`

- Backend branch execution currently reuses the analyzer directly against the branch time window.
- Minimal execution mode is wired as a branch-level quick analysis path.
- Frontend job detail page now supports:
  - starting branch analysis
  - polling running branch status
  - showing branch token usage
  - rendering branch report markdown inline

At this point, the timeline flow has a working minimal loop:

1. upload file
2. inspect timeline
3. inspect cluster
4. save branch
5. run branch analysis
6. see branch result inline

The next structural step is to promote Branch into a first-class detail route with its own history, artifacts, and progressive drill-down state.

## Current Limits

- Timeline buckets are computed on read, not pre-materialized.
- Bucket messages are filtered from persisted job messages; there is no dedicated `timeline_buckets` table yet.
- Token usage is aggregated at job level; stage-level persistence is not stored separately yet.
- Frontend has not been switched to the new timeline workbench yet; only backend primitives are in place.

## Next Recommended Step

Implement the job detail timeline UI in `web/` and make history entries open `#/job/<id>` by default instead of waiting for final reports.

## ECharts Timeline Increment

- Replaced the hand-built bucket rail with Apache ECharts in `web/src/App.tsx`.
- Timeline rendering is now a fixed-width, responsive chart container:
  - horizontal page overflow is avoided even for long chat histories
  - time navigation is handled by ECharts `dataZoom`
  - dragging on the chart performs rectangular brush selection
- Brush selection now feeds the existing branch preview flow:
  - selected bucket is updated to the first bucket in the brushed range
  - selected time range is sent to `POST /api/jobs/{id}/branches/preview`
  - cluster-card selection still works and clears manual brush range
- Added `echarts` to the web dependencies.

Verification:

```bash
cd web
npm run build
```

```bash
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache \
GOMODCACHE=/mnt/RapidPool/tmp/record_analysis_gomodcache \
go test ./...
```

Known follow-up:

- The first ECharts integration imports the full package and triggers a Vite chunk-size warning. A later cleanup should switch to selective ECharts imports or code-split the job detail route.

## Local Drill-Down Timeline Increment

- Timeline API now accepts optional range filters:

```text
GET /api/jobs/{id}/timeline?granularity=hour&start_time=...&end_time=...
```

- Backend behavior:
  - in-memory jobs filter messages before bucketing
  - PostgreSQL jobs apply `msg_time >= start_time` and `msg_time < end_time` in the query
  - invalid or reversed ranges return `400`
- Frontend behavior:
  - global automatic mode only chooses overview granularities: `year`, `month`, `week`, `day`
  - `hour`, `15m`, and `5m` are disabled until the user selects a local range
  - brushing an overview segment automatically enters a local window and loads hourly buckets for that range
  - the user can return to the full overview with `返回全局`

Added regression coverage:

```text
TestJobTimelineEndpointFiltersByRequestedRange
```

## Branch Reading Increment

- Branch creation now deduplicates by:
  - `job_id`
  - `granularity`
  - expanded `start_time`
  - expanded `end_time`
- PostgreSQL-backed creation returns the existing branch for duplicate windows instead of inserting another row.
- In-memory branch creation follows the same duplicate-window rule.
- Frontend branch list applies the same key as a defensive de-duplication pass for historical duplicate rows.
- Branch results are no longer rendered as a full inline `<pre>` block:
  - list rows show a short summary
  - full output is hidden behind `查看结果`
  - expanded results render Markdown through `react-markdown`

Added regression coverage:

```text
TestJobBranchCreateAndListEndpoints
```

## Preview Linkage Fix

- `连续片段预览` is controlled by `branchPreview`.
- The preview source is now resolved in this order:
  - brushed range (`selectedRange`)
  - clicked cluster (`selectedCluster`)
  - clicked bucket (`selectedBucket`)
- Clicking a timeline bucket now clears stale cluster selection and refreshes branch preview for that bucket's time window.
- This fixes the previous behavior where bucket selection changed the left detail panel but the continuous segment preview could remain on an older cluster.

## Word Cloud Work Item Increment

- Added non-LLM word cloud aggregation in `internal/service/wordcloud.go`.
- Added `record_analysis_work_items` with:
  - `kind`
  - `scope_type`
  - `scope_id`
  - `granularity`
  - `start_time`
  - `end_time`
  - `status`
  - `priority`
  - `progress`
  - `result_json`
- Added work item APIs:

```text
GET  /api/jobs/{id}/work-items?kind=word_cloud&granularity=day
POST /api/jobs/{id}/work-items/seed
POST /api/jobs/{id}/work-items/{work_item_id}/prioritize
```

- Backend worker now claims queued items by `priority desc, created_at asc` and completes word cloud aggregation incrementally.
- Frontend timeline workspace now supports:
  - starting word cloud pre-aggregation for the current timeline granularity
  - polling queued/running work items
  - showing selected bucket word cloud status and result
  - prioritizing queued/failed bucket work

Added regression coverage:

```text
TestBuildWordCloudCountsTerms
```

## Topic Summary Work Item Increment

- Added `topic_summary` as a second work item kind alongside `word_cloud`.
- Added LLM contract:
  - `llm.TopicSummarizer`
  - `llm.TopicSummaryInput`
  - `llm.TopicSummary`
- `OpenAICompatibleExtractor` now implements `SummarizeTopic`.
- Added structured LLM assets:
  - `llm/prompts/topic_summary.md`
  - `llm/schemas/topic_summary.schema.json`
- Work item seed API now accepts `kind`:

```json
{"kind":"topic_summary","granularity":"day"}
```

- Backend worker behavior:
  - claims queued work by priority
  - for `topic_summary`, reads only that bucket's time window
  - caps input at the latest 200 messages per bucket
  - records prompt/completion/total token usage on the work item
- Frontend behavior:
  - `连续片段预览` has a `生成片段摘要` action
  - selected bucket shows its summary task
  - selected cluster aggregates the summary tasks for all buckets in that cluster
  - completed summaries show title, summary, topics, key events, and token count
  - queued/failed summaries can be prioritized

This is the first layer of the multi-scale summary pipeline. The next layer should add summary-of-summaries work items so larger time ranges consume completed lower-level summaries instead of raw messages.

## Summary Merge Work Item Increment

- Added `summary_merge` as the second LLM layer for the timeline pipeline.
- `summary_merge` does not read raw chat messages. It only consumes completed `topic_summary` work items inside the selected time range.
- Added LLM contract:
  - `llm.TopicSummaryMerger`
  - `llm.TopicSummaryMergeInput`
- `OpenAICompatibleExtractor` now implements `MergeTopicSummaries`.
- Added merge prompt:
  - `llm/prompts/topic_summary_merge.md`
- Added Postgres helpers:
  - `CreateMergeWorkItem`
  - `CompletedTopicSummariesInRange`
- Added API:

```text
POST /api/jobs/{id}/work-items/merge
```

Request body:

```json
{"granularity":"day","start_time":"2026-04-01T00:00:00Z","end_time":"2026-04-08T00:00:00Z"}
```

- Merge work items are deduplicated by `job_id + kind + granularity + scope_id`.
- Merge work items record prompt/completion/total tokens like bucket summaries.
- Frontend behavior:
  - `连续片段预览` now has `聚合已完成摘要`
  - the button is enabled after at least one bucket summary in the selected range has completed
  - merge task status is polled with other work items
  - completed merge summaries are shown above the bucket-level summary list
  - queued/failed merge tasks can be prioritized

Verification:

```text
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache GOMODCACHE=/mnt/RapidPool/tmp/record_analysis_gomodcache go test ./...
npm run build -- --mode development
```
