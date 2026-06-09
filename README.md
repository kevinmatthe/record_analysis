# record_analysis

基于聊天记录的关系互动观察与证据化分析工具。当前版本提供 Go 核心库和 CLI，支持上传/解析 `txt`、`csv`、`json` 三种聊天记录格式。默认路径只做导入、切片和可计算统计，不使用关键词规则伪造关系分析。

## 快速开始

```bash
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache go test ./...
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache go run ./cmd/rel-analyzer analyze records/某人/chat.csv \
  --relationship-id rel_demo \
  --output .data/demo_report.md \
  --json-output .data/demo_result.json
```

CLI 会先保存原始聊天文件，再执行解析、去重、切片和基础指标计算。如果配置了 `[minio_config]`，文件会上传到 MinIO；否则回退到本地对象存储目录 `.data/objects/`。未显式启用结构化 LLM extractor 时，报告会明确标记为“未分析”。

启动本地 API server：

```bash
RECORD_ANALYSIS_CONFIG_PATH=.dev/config.toml \
RECORD_ANALYSIS_AUTH_USERNAME=admin \
RECORD_ANALYSIS_AUTH_PASSWORD=secret \
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache \
go run ./cmd/rel-analyzer serve --addr :8080 --history-path .data/analysis/index.jsonl
```

如果配置了独立 Postgres `[db_config]`，账号、session、任务 job、消息预览和历史分析记录都会写入 `record_analysis_*` 表；未配置时才回退到 JSONL/内存。不要复用 BetaGo 数据库，建议新建 `record_analysis` 数据库。

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

建表并生成 gorm-gen 代码：

```bash
RECORD_ANALYSIS_CONFIG_PATH=.dev/config.toml \
GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache \
go run ./cmd/generate
```

启动前端 WebUI：

```bash
cd web
npm install --cache /mnt/RapidPool/tmp/record_analysis_npm_cache
npm run dev
```

打开 `http://localhost:5173` 登录后上传 `.txt`、`.csv` 或 `.json` 聊天文件。Go server 只提供 API：`/api/auth/*`、`POST /api/jobs`、`GET /api/jobs/{id}`、`GET /api/jobs/{id}/preview`、`GET /api/analyses`、`GET /api/analyses/{id}/report`。

## Docker Compose 本地启动

默认 compose 栈启动：

- `postgres`：事实状态库，API 启动时会自动建库/建表；
- `api`：Go API server；
- `web`：Nginx 托管的 Vite 构建产物，并反向代理 `/api/` 到 API；
- 本地对象存储 fallback：未配置 MinIO 时，上传文件保存到 compose volume 中的 `/app/.data/objects`。

```bash
docker compose up --build
```

打开 `http://localhost:5173`，默认账号密码为 `admin` / `secret123`。可以通过环境变量覆盖：

```bash
docker-compose run --rm \
  -e RECORD_ANALYSIS_AUTH_USERNAME=admin \
  -e RECORD_ANALYSIS_AUTH_PASSWORD=change-me \
  api serve --addr :8080
```

开发时如果想让前端热重载、Go 服务用本机 `go run`，可以只用 compose 启数据库/依赖：

```bash
docker-compose -f docker-compose.deps.yaml up
```

另一个终端启动 Go API：

```bash
RECORD_ANALYSIS_CONFIG_PATH=.dev/config.toml \
RECORD_ANALYSIS_AUTH_USERNAME=admin \
RECORD_ANALYSIS_AUTH_PASSWORD=secret123 \
go run ./cmd/rel-analyzer serve --addr :8080 --cors-origin http://localhost:5173
```

`.dev/config.toml` 已按本机 `go run` 访问依赖容器生成好：Postgres、OpenSearch 和 MinIO 都使用 `localhost`。MinIO 配置默认注释掉，因此本机 API 会先降级到 `.data/objects` 本地对象存储；需要 MinIO 时取消 `.dev/config.toml` 中的 `[minio_config]` 注释。

前端热重载有两种方式。直接本机运行：

```bash
cd web
npm install
npm run dev
```

或者用 compose 跑 Vite dev server：

```bash
docker-compose -f docker-compose.web-dev.yaml up
```

Vite 已在 `web/vite.config.ts` 里把 `/api` 代理到 `VITE_API_PROXY_TARGET`。本机 `npm run dev` 默认代理到 `http://127.0.0.1:8080`；compose 的 `web-dev` 容器会设置 `VITE_API_PROXY_TARGET=http://host.docker.internal:8080`，并通过 `extra_hosts` 把它解析到宿主机，所以可以访问本机 `go run` 的 API。页面访问 `http://localhost:5173`。

默认配置文件是 `.dev/config.compose.toml`，启用 Postgres，并配置了可选 OpenSearch 地址；如果没有启动 OpenSearch，写入失败会记录 warning 并继续使用 Postgres 降级查询。MinIO 默认不启用，上传文件会保存到本地对象存储 volume。

如需同时启动 OpenSearch 索引服务：

```bash
docker-compose -f docker-compose.yaml -f docker-compose.opensearch.yaml up --build
```

`[opensearch_config]` 兼容 BetaGo-Redefine 配置字段：`domain`、`user`、`password`、`lark_msg_index`、`lark_chunk_index`、`lark_card_action_index`。Record Analysis 会把上传消息索引到 `lark_msg_index`，把 bucket/merge 摘要索引到 `lark_chunk_index`，Branch 报告索引到 `record_branch_index` 或默认 `record_analysis_branches`。OpenSearch 写入失败不会让任务失败，会记录 job event 并继续使用 Postgres 降级查询。

如果需要同时启动可选 MinIO：

```bash
docker-compose -f docker-compose.yaml -f docker-compose.minio.yaml up --build
```

MinIO Console 地址为 `http://localhost:9001`。也可以将 `.dev/config.compose.minio.toml` 里的 `[minio_config]` 合并到自己的 `.dev/config.toml`。

启用 OpenAI-compatible 结构化 LLM 分析：

```bash
RECORD_ANALYSIS_CONFIG_PATH=.dev/config.toml GOCACHE=/mnt/RapidPool/tmp/record_analysis_gocache \
go run ./cmd/rel-analyzer analyze records/某人/chat.csv \
  --relationship-id rel_demo \
  --from 2026-06-01 \
  --to 2026-06-03 \
  --llm-profile reasoning \
  --enable-llm \
  --max-llm-messages 500 \
  --output .data/llm_report.md
```

LLM 配置参考 BetaGo_v2 的 `ark_config`，默认读取 `.dev/config.toml`，也可以用 `RECORD_ANALYSIS_CONFIG_PATH` 指定路径。示例见 `.dev/config.example.toml`。CLI 的 `--llm-base-url` 和 `--llm-model` 会覆盖配置文件。

MinIO 配置同样参考 BetaGo_v2 的 `minio_config`，使用 `internal` endpoint 上传，使用 `external` endpoint 生成访问 URL。真实 `.dev/config.toml` 已被 `.gitignore` 忽略。

周期分析使用 `--from` 和 `--to`，其中 `--from` 包含起始时间，`--to` 不包含结束时间。`--max-llm-messages` 会限制送入 LLM 的消息数；导入、对象存储和基础统计仍基于筛选后的完整聊天记录。

LLM adapter 使用 Chat Completions 的 `response_format.type=json_schema` 约束结构化输出。行为动作优先按 segment 批量抽取，避免对每条消息单独请求模型；旧的单条抽取接口仍保留为兼容路径。

## 代码结构

- `cmd/rel-analyzer/`：命令行入口。
- `internal/importer/`：三种聊天格式解析与发送者归一化。
- `internal/analyzer/`：导入后分析编排。默认不抽取行为/事件；只有传入 `llm.Extractor` 才生成 actions、events、dimensions 和分析报告。
- `internal/llm/`：结构化 LLM 任务接口和 segment-batch action/event/report 输入构造器。
- `internal/llm/openai_extractor.go`：OpenAI-compatible Chat Completions adapter。
- `llm/prompts/` 与 `llm/schemas/`：结构化输出 prompt 契约和 JSON schema。
- `internal/storage/`：对象存储接口、本地文件实现、MinIO 实现，MinIO 使用 BetaGo_v2 同款 internal/external 双端点模式。
- `internal/service/`：上传并分析的应用服务，WebUI/API 直接调用 `UploadAndAnalyzeWithOptions`。
- `internal/server/`：标准库 HTTP API、登录/注册/session、任务 job、CORS 和分析查询接口。
- `internal/server/postgres_store.go`：Postgres/GORM table struct 和 repository，表名统一为 `record_analysis_*`。
- `web/`：Vite + React + TypeScript 前端工作台。
- `internal/model/`：消息、片段、事件、指标和报告模型。

## WebUI 接入

WebUI 通过 Go API 提交 job 后立即获得 `job_id`，可查询可处理消息数、分页预览、当前阶段、进度、事件日志和失败原因。任务完成后写入分析历史，用于报告详情查询。接入模型时，实现 `internal/llm.Extractor` 并传入 analyzer；如果同时实现 `internal/llm.BatchActionExtractor`，会走“片段级行为识别 -> 片段级事件抽取 -> 维度生成 -> 周期报告”的完整链路。
