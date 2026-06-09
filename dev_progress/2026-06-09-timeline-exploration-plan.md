# Plan: Timeline Exploration Workflow

**Generated**: 2026-06-09  
**Estimated Complexity**: High

## Overview

Shift the product from "submit and wait for one final report" to a timeline-first exploration workflow. The timeline becomes the primary UI. Users should see message distribution immediately, zoom into time ranges, inspect bucket previews, select ranges, create analysis branches, and receive incremental outputs before full LLM enrichment finishes.

## Assumptions

- Keep Go as the API/runtime and React + TypeScript as the WebUI.
- Keep PostgreSQL as the system of record.
- Reuse MinIO/object storage only for raw file persistence.
- LLM remains optional and should run only on explicit branch analysis, not on the whole file by default.

## Sprint 1: Timeline Data Foundation
**Goal**: Persist timeline-ready raw data and return demoable bucketed views without LLM.
**Demo/Validation**:
- Upload a file and immediately get a `job_id`.
- Query timeline buckets for a time range and zoom level.
- See counts, participant mix, and previews within 1-2 seconds after parse completes.

### Task 1.1: Add timeline tables
- **Location**: `internal/server/postgres_store.go`, `cmd/generate/main.go`, `internal/infrastructure/db/*`
- **Description**: Add code-defined tables for `raw_messages`, `timeline_buckets`, `timeline_clusters`, `analysis_branches`, `branch_artifacts`.
- **Acceptance Criteria**:
  - Tables are migrated by `go run ./cmd/generate`.
  - gorm-gen output is regenerated in one command.
- **Validation**: run `go run ./cmd/generate` and inspect generated query/model files.

### Task 1.2: Persist raw messages with stable indexes
- **Location**: `internal/service/service.go`, `internal/importer/importer.go`, `internal/server/jobs.go`
- **Description**: Persist every parsed message with source file, sender, original sender, timestamp, normalized content, and sequence index.
- **Acceptance Criteria**:
  - A job record can expose exact parsed message count.
  - Messages are queryable by time range and sequence.
- **Validation**: add repository/service tests for persisted message count and ordering.

### Task 1.3: Build adaptive bucket generator
- **Location**: `internal/service/timeline.go` (new), `internal/model/`
- **Description**: Generate buckets at multiple granularities: day, hour, 15m, 5m. Store message counts, participant counts, first/last message pointers, and short preview text.
- **Acceptance Criteria**:
  - Bucket generation works without LLM.
  - Empty time spans are omitted or compressed explicitly.
- **Validation**: add unit tests covering sparse and dense timelines.

### Task 1.4: Expose timeline APIs
- **Location**: `internal/server/server.go`, `internal/server/jobs.go`
- **Description**: Add APIs:
  - `GET /api/jobs/{id}/timeline?granularity=hour`
  - `GET /api/timeline/{bucket_id}/messages?page=1&page_size=50`
- **Acceptance Criteria**:
  - Job detail page can query bucket summaries and message pages.
- **Validation**: add API tests for range filters and pagination.

## Sprint 2: Timeline UI and Immediate Visibility
**Goal**: Replace the report-first workspace with a timeline-first workbench.
**Demo/Validation**:
- Open a job and see a large interactive timeline.
- Zoom from days to hours.
- Click a bucket and inspect messages without leaving the page.

### Task 2.1: Add timeline route and job detail shell
- **Location**: `web/src/App.tsx`, `web/src/api.ts`
- **Description**: Add `#/job/<id>` route with separate sections for overview, timeline, bucket preview, and branch results.
- **Acceptance Criteria**:
  - Refresh preserves current job route.
  - History cards open job detail, not only final report.
- **Validation**: `npm run build`.

### Task 2.2: Implement responsive timeline canvas/SVG
- **Location**: `web/src/components/TimelineView.tsx` (new), `web/src/styles.css`
- **Description**: Render buckets as a zoomable horizontal strip with status colors, hover preview, time ruler, and pan/zoom controls.
- **Acceptance Criteria**:
  - Dense timelines remain readable.
  - Desktop and mobile layouts do not overlap.
- **Validation**: build plus manual viewport checks.

### Task 2.3: Add bucket inspection panel
- **Location**: `web/src/components/BucketDetail.tsx` (new)
- **Description**: Show selected bucket stats, preview text, top participants, and paginated messages.
- **Acceptance Criteria**:
  - Selecting a bucket updates the side panel instantly.
  - Message paging works independently from timeline zoom.
- **Validation**: API integration checks in browser.

## Sprint 3: Lightweight Clustering and Selection
**Goal**: Give users meaningful candidate segments before any heavy LLM pass.
**Demo/Validation**:
- Timeline shows cluster overlays.
- User can click or brush-select a range and see suggested expanded boundaries.

### Task 3.1: Implement heuristic cluster builder
- **Location**: `internal/service/timeline.go`, `internal/analyzer/`
- **Description**: Merge adjacent buckets into `timeline_clusters` using time gap, sender alternation, message density, and lexical overlap.
- **Acceptance Criteria**:
  - Clusters are deterministic and explainable.
  - Cluster status begins as `unseen`.
- **Validation**: unit tests for split/merge boundaries.

### Task 3.2: Add selection and brush APIs
- **Location**: `internal/server/server.go`
- **Description**: Add:
  - `POST /api/jobs/{id}/branches/preview`
  - `POST /api/jobs/{id}/branches`
  Preview returns expanded time bounds and candidate clusters; create persists a branch.
- **Acceptance Criteria**:
  - Users can analyze a cluster or arbitrary brushed range.
- **Validation**: API tests for expansion logic.

### Task 3.3: Add UI for click/brush selection
- **Location**: `web/src/components/TimelineView.tsx`
- **Description**: Support click-to-select, drag-to-select, and explicit "create branch" action.
- **Acceptance Criteria**:
  - Selection state is visually distinct.
  - Users can clear or refine selection without page navigation.
- **Validation**: browser smoke test.

## Sprint 4: Branch-Based Incremental Analysis
**Goal**: Run analysis against selected branches and stream visible progress/artifacts back into the timeline.
**Demo/Validation**:
- Creating a branch immediately shows `queued/running` status.
- Artifacts appear incrementally: summary, topic labels, event cards, report.

### Task 4.1: Persist branch jobs and event stream
- **Location**: `internal/server/postgres_store.go`, `internal/service/service.go`
- **Description**: Store branch-level lifecycle, progress counters, stage, and artifacts separately from file import jobs.
- **Acceptance Criteria**:
  - Branches survive server restart as records.
  - History page can list branches under a job.
- **Validation**: repository tests for branch status transitions.

### Task 4.2: Refactor analyzer into staged artifact producers
- **Location**: `internal/analyzer/analyzer.go`, `internal/llm/`
- **Description**: Split output into small persisted artifacts:
  - topic summary
  - event extraction
  - action extraction
  - final markdown report
- **Acceptance Criteria**:
  - UI can render partial results before final report completion.
  - `quick` mode becomes the default branch mode.
- **Validation**: analyzer/service tests for partial artifact persistence.

### Task 4.3: Add live branch status UI
- **Location**: `web/src/App.tsx`, new branch components
- **Description**: Show per-branch progress, current stage, processed message window, and artifacts above/below timeline.
- **Acceptance Criteria**:
  - Users can tell whether the system is parsing, clustering, extracting, or finished.
  - Failed branches show actionable errors.
- **Validation**: build plus manual polling check.

## Sprint 5: Product Hardening
**Goal**: Make the workflow reliable for repeated exploration.
**Demo/Validation**:
- Restart server and recover jobs/branches/history.
- Query old jobs and reopen branch results from history.

### Task 5.1: Job/branch recovery and retries
- **Location**: `internal/server/`, `internal/service/`
- **Description**: Mark interrupted work, add retry endpoints, and recover view state from DB.

### Task 5.2: Observability and performance controls
- **Location**: backend + frontend
- **Description**: Add stage timings, latest-N LLM window visibility, and explicit branch message counts.

### Task 5.3: Export and report consolidation
- **Location**: service + UI
- **Description**: Allow a branch to be promoted into a saved report/export rather than forcing every branch to produce one.

## Testing Strategy

- Backend: unit tests for bucketing, clustering, branch expansion, and repository persistence.
- API: route tests for timeline, branch preview/create, and status polling.
- Frontend: build verification and manual route/interaction smoke tests.
- End-to-end: upload -> timeline visible -> create branch -> partial artifacts visible -> final report visible.

## Potential Risks & Gotchas

- Cluster quality can stall the UX if heuristics are too weak; keep boundaries explainable and tune with fixtures first.
- Large timelines can overload the browser; aggregate aggressively and virtualize detail lists.
- Long-running branch jobs need clear DB-backed status or the UI will again look stuck.
- Do not let full-file LLM analysis remain the default path; it recreates the current latency problem.

## Rollback Plan

- Keep existing report endpoints during the transition.
- Gate timeline workbench behind a new route while preserving current upload/history flows.
- If branch analysis is unstable, keep Sprint 1-2 as a standalone non-LLM exploratory release.
