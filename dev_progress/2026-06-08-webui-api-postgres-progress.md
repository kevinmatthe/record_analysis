# WebUI/API/Postgres Development Progress

Date: 2026-06-08

## Current Direction

The project has moved from a single Go-rendered debug page to a separated architecture:

- Go remains the API server and analysis runtime.
- `web/` contains a Vite + React + TypeScript WebUI.
- Analysis execution is moving from synchronous requests to persisted jobs with queryable status.
- PostgreSQL is the target persistence layer. JSONL remains only as fallback while schemas stabilize.
- The PostgreSQL database must be independent from BetaGo. BetaGo is used only as a reference for GORM/gorm-gen style.

## Implemented Backend Changes

### API Server

`internal/server/` now exposes standard API endpoints instead of embedding the full UI:

- `POST /api/auth/login`
- `POST /api/auth/register`
- `POST /api/auth/logout`
- `GET /api/auth/me`
- `POST /api/auth/password`
- `POST /api/jobs`
- `GET /api/jobs/{id}`
- `GET /api/jobs/{id}/preview`
- `GET /api/analyses`
- `GET /api/analyses/{id}`
- `GET /api/analyses/{id}/report`
- `POST /api/analyze` remains for compatibility.

### Job Status and Progress

`AnalysisJob` tracks:

- `id`
- `status`: `queued`, `running`, `completed`, `failed`
- `stage`
- `relationship_id`
- `file_name`
- `message_count`
- `llm_message_limit`
- `llm_message_count`
- `processed_count`
- `progress`
- `preview_total`
- `result_record_id`
- `error`
- `events`

The server creates a job immediately after upload and parsing, then runs analysis in the background. The UI can query the job immediately.

Progress callbacks were added through:

- `service.UploadAnalyzeOptions.Progress`
- `analyzer.AnalyzeOptions.Progress`

Current progress stages include:

- `object_store_upload`
- `message_parse`
- `llm_action_extraction`
- `llm_event_extraction`
- `llm_dimension_generation`
- `llm_report_generation`
- `analysis_without_llm`
- `report_persist`

This fixes the earlier misleading frontend state where the UI stayed on “uploading to object storage”.

### Message Cap Behavior

`Max LLM Messages` is now a cap, not a rejection threshold:

- full upload still goes to object storage;
- full filtered import still contributes to basic metrics;
- only the messages sent to LLM are capped.

The job status exposes both:

- `message_count`: processable messages after parsing/filtering;
- `llm_message_count`: actual messages sent into LLM.

### Preview

Job preview supports pagination:

- `GET /api/jobs/{id}/preview?page=1&page_size=20`

Preview returns normalized message fields:

- `id`
- `sender`
- `time`
- `type`
- `content`

## PostgreSQL Design

### Database Scope

Use a new independent database, for example:

```toml
[db_config]
host = "localhost"
port = 5432
user = "postgres"
password = "***"
dbname = "record_analysis"
sslmode = "disable"
timezone = "Asia/Shanghai"
```

Do not reuse BetaGo's database.

### Tables

Current GORM table structs live in `internal/server/postgres_store.go`.

Table names are prefixed with `record_analysis_`:

- `record_analysis_users`
- `record_analysis_sessions`
- `record_analysis_jobs`
- `record_analysis_job_events`
- `record_analysis_job_messages`
- `record_analysis_records`

### gorm-gen Flow

BetaGo's gorm-gen flow reads existing tables and generates code. For this project, `cmd/generate` performs both steps in one command:

1. AutoMigrate the code-defined `record_analysis_*` table structs.
2. Reflect the resulting PostgreSQL tables with gorm-gen.
3. Generate model/query code under:
   - `internal/infrastructure/db/model`
   - `internal/infrastructure/db/query`

Command:

```bash
RECORD_ANALYSIS_CONFIG_PATH=.dev/config.toml \
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache \
go run ./cmd/generate
```

## Authentication

Implemented:

- login;
- register;
- logout;
- current user lookup;
- password change.

With PostgreSQL configured, users and sessions are persisted in:

- `record_analysis_users`
- `record_analysis_sessions`

Without PostgreSQL, auth can still use the configured fallback username/password for local development.

## Frontend Changes

`web/` now contains a Vite + React + TypeScript app.

Implemented views:

- login/register;
- analysis workspace;
- job status panel;
- paginated message preview;
- history search;
- report detail;
- account password change.

The frontend now submits jobs through `POST /api/jobs`, polls `GET /api/jobs/{id}`, and reads previews through `GET /api/jobs/{id}/preview`.

## Current Verification

Commands run successfully:

```bash
GONOSUMDB='*' GOSUMDB=off \
GOMODCACHE=/mnt/RapidPool/tmp/record_analysis_gomodcache \
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache \
go test ./...
```

```bash
cd web
npm run build
```

Frontend build output succeeded with Vite.

## Known Gaps / Next Work

- Generated gorm-gen output has not been committed yet because it requires a live PostgreSQL database configured through `[db_config]`.
- Existing repository code still uses a direct GORM repository implementation. After running `cmd/generate`, repository code should be moved to the generated query layer.
- Job progress currently reports stage-level progress and capped message counts. Per-message LLM progress depends on deeper extractor instrumentation.
- Running jobs are in-process goroutines. If the API server restarts, persisted jobs can be queried but not resumed yet.
- Failed jobs are persisted with an error string, but retry/cancel APIs are not implemented yet.
- The WebUI has a functional layout, but can still be improved visually and ergonomically.
