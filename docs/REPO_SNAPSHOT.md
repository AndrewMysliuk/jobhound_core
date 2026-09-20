## 1. Purpose

Repository remote `AndrewMysliuk/jobhound_core` (`go.mod` module: `github.com/andrewmysliuk/jobhound_core`; README: “jobhound_core”) is the JobHound backend: ingest job listings from several public boards, persist them in PostgreSQL, filter in three stages (broad ingest → local keywords → Claude scoring), and expose a JSON HTTP API for a separate UI. Role: backend / worker / debug collector process (not the product frontend).

MVP is **single-tenant** (no auth). The user has up to **3 search slots**; each slot is one hunt with an immutable stage-1 keyword string. Product HTTP is API-first (`cmd/api`). Collectors debug HTTP (`cmd/agent`) is development-only.

## 2. How the system works

### 2.1 Product model

The user does not crawl the whole web. They run a few **slots**. Each slot is:

1. A **name** (display label, e.g. `golang backend`). Immutable after the first successful ingest. The name is **not** sent to job boards.
2. A **stage-1 pool**: vacancies fetched from **all backend-configured sources** using a **fixed query matrix** (`collectors/schema.Queries`), upserted into `jobs`, linked via `slot_jobs`.
3. **Stage 2**: local include/exclude keywords on that pool only (no re-crawl).
4. **Stage 3**: Claude scores rows that passed stage 2 against a global **profile** (CV-style free text). Cap + deterministic ordering.

Manual corrections reuse the same passed/failed buckets (`PATCH` stage 2 or 3). Changing stage-2 filters wipes stage-2 and stage-3 outcomes for the slot; changing profile text wipes stage-3 only. Filter edits do **not** re-hit external sites.

There is **no** public HTTP path to re-run stage 1 on an existing slot. Incremental “pull new” and scheduled auto-refresh of listings are product backlog.

### 2.2 Processes

One Docker Compose stack (or local `make` binaries) runs several processes:

| Process | Binary | Role |
|---------|--------|------|
| `api` | `cmd/api` | Product JSON API on **:3000** (`/api/v1/...`). Starts Temporal **workflows**. Does not fetch boards or call Claude. |
| `worker` | `cmd/worker` | Temporal worker on task queue `jobhound`. Runs ingest, pipeline persist, manual parent workflow, job retention. Owns collectors + Redis + Anthropic. |
| `agent` | `cmd/agent` | Dev-only collector debug HTTP on **:3001** when `JOBHOUND_DEBUG_HTTP_ADDR` is set. Also a leftover in-process noop pipeline and a CLI to start `ManualSlotRunWorkflow`. |
| `migrate` | `cmd/migrate` | `golang-migrate` CLI (`up` / `down` / `version` / `force`). |
| `retention` | `cmd/retention` | One-shot hard-delete of jobs older than **7 days** (same logic as the Temporal schedule). |

Infra beside the app: **Postgres 16** (app data), **Redis 7** (ingest lock + cooldown), **Temporal** `auto-setup:1.29.1` on its **own** Postgres, Temporal UI **:8088**.

```
browser / jobhound_frontend
        │  HTTP JSON  :3000
        ▼
     cmd/api  ──ExecuteWorkflow──►  Temporal  ◄── poll ──  cmd/worker
        │                              │                      │
        │                              │                      ├─ collectors (HTTP / goquery / rod)
        │                              │                      ├─ Redis lock/cooldown
        │                              │                      ├─ Anthropic Messages API
        └── GORM / Postgres ◄──────────┴──────────────────────┘
```

`cmd/agent` debug HTTP is a **side door**: it hits collectors directly and returns `domain.Job` JSON. It does not write slots or pipeline runs.

### 2.3 End-to-end happy path

**Create slot (stage 1)**

1. `POST /api/v1/slots` with `{ "name": "..." }` and header `Idempotency-Key` (UUID). Cap **3** slots.
2. `slots/impl` inserts `slots` + `slot_idempotency_keys`, then starts Temporal workflow `ManualSlotRunWorkflow` with kind `INGEST_SOURCES`, workflow id `pubapi-slot-ingest-{slot_id}`.
3. The parent workflow expands `DefaultIngestSourceIDs` × `collectors/schema.Queries` into **child** `IngestSourceWorkflow`s: one per `(source, query)`. Q-list sources get 10 keyword children; Wellfound gets 4 role slugs; WWR / VueJobs / Golang Cafe get one empty-query (`catalog`) child. Non-`builtin` children run in parallel; `builtin` children run **serially**.
4. Each child activity: Redis `SET NX` lock `ingest:lock:{slot_id}:{source_id}:{query_segment}` (TTL 600s; empty query → segment `catalog`). If cooldown `ingest:cooldown:{slot_id}:{source_id}:{query_segment}` exists and explicit refresh is off, skip (TTL 3600s). Fail closed if Redis is down.
5. Collector `Fetch` (catalog) or `FetchWithSlotSearch(query)` (matrix keyword / Wellfound slug) / optional `FetchIncremental(cursor)`. Jobs get a stable id: `source + U+001E + normalized listing URL`.
6. `jobs.SaveIngest` upserts into `jobs` with `stage1_status = PASSED_STAGE_1`, then `slot_jobs` links the vacancy to the slot. Watermark cursor stored in `ingest_watermarks` when the source is incremental.
7. On success, cooldown key is set. Slot card `stage_1.state` is derived from the Temporal workflow (`idle` / `running` / `succeeded` / `failed`).

**Stage 2 (local keywords)**

1. `POST /api/v1/slots/{id}/stages/2/run` body `{ "include": [...], "exclude": [...] }` → **202**.
2. Service **deletes** existing `pipeline_runs` for the slot (CASCADE `pipeline_run_jobs`) — reset rule when stage-2 filters change.
3. Starts `ManualSlotRunWorkflow` kind `PIPELINE_STAGE2`, id `pubapi-slot-stage2-{slot_id}` (reuse policy `ALLOW_DUPLICATE` after close; 409 if still running).
4. Activities: create `pipeline_runs` row → list slot jobs with `PASSED_STAGE_1` → in-memory broad + keyword filters → persist `REJECTED_STAGE_2` / `PASSED_STAGE_2` per job. Stage-1 drops get **no** `pipeline_run_jobs` row.

**Profile + stage 3 (LLM)**

1. `PUT /api/v1/profile` `{ "text": "..." }` stores the single `user_profile` row (`id = 1`) and **clears** `stage3_status` / `stage3_rationale` on all slots.
2. `POST /api/v1/slots/{id}/stages/3/run` `{ "max_jobs": 1–100 }` → **202**. Requires a prior pipeline run and non-empty profile (else 422).
3. Workflow kind `PIPELINE_STAGE3` uses the latest `pipeline_runs.id`. Eligible pool: `PASSED_STAGE_2` without a current terminal stage-3 outcome, ordered `posted_at DESC`, `job_id ASC`.
4. Effective batch = `min(request max_jobs, JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN default 20)`. Worker `llm.Scorer`: Anthropic if `JOBHOUND_ANTHROPIC_API_KEY` is set, else `llm/mock` (score **0**).
5. Score ≥ **60** → `PASSED_STAGE_3`, else `REJECTED_STAGE_3`. Rationale stored on the row. Jobs beyond the cap stay eligible for a later stage-3 POST.

**Read / correct**

- `GET .../stages/{1|2|3}/jobs` paginates the pool (`page` / `limit`; stages 2–3 optional bucket/status).
- `PATCH .../stages/{2|3}/jobs/{job_id}` moves a row between passed/failed. Stage-2 patch also clears that row’s stage-3 fields.

**Delete slot**: `DELETE` hard-deletes the slot; FKs `ON DELETE CASCADE` remove `slot_jobs`, watermarks, pipeline runs, idempotency keys. Shared `jobs` rows remain until **7-day** retention (`JobRetentionWorkflow` Sunday 05:00 UTC, or `bin/retention run`).

### 2.4 Collectors (what “ingest” actually hits)

Every source implements `collectors.Collector` (`Name` + `Fetch`). Optional: `IncrementalCollector`, `SlotSearchFetcher`. Bootstrap: `internal/collectors/bootstrap.MVPCollectors`. Ingest child queries: `internal/collectors/schema.Queries` (q-list / Wellfound slugs / catalog). Slot **name** is not a board query.

| `Job.Source` | Site | Transport (fact) | Ingest children |
|--------------|------|------------------|-----------------|
| `europe_remotely` | euremotejobs.com | T2: WP `admin-ajax.php` HTML fragment + detail GET | q-list (10) |
| `working_nomads` | workingnomads.com | T2: Elasticsearch JSON `jobsapi/_search` | q-list (10) |
| `builtin` | builtin.com/jobs/remote | T2 parse; **T3 rod** for HTML by default (`browserfetch` + Chromium) | q-list (10), serial |
| `himalayas` | himalayas.app | T2: public JSON API (`/jobs/api`, `/jobs/api/search`) | q-list (10) |
| `remotify_europe` | remotifyeurope.com | T2: Next.js Flight POST (`Next-Action` discovered each run) + listing GET JSON-LD | q-list (10) |
| `we_work_remotely` | weworkremotely.com | T2: RSS only (`/remote-jobs.rss`); HTML is Cloudflare | catalog |
| `wellfound` | wellfound.com | T2: HTML `/role/r/{slug}` + JobPosting JSON-LD; site mutex + 1s delay | 4 role slugs |
| `vue_jobs` | vuejobs.com | T2: SSR `__NUXT_DATA__` remote catalog; skip PRO; `Job.URL` = `/jobs/{slug}` | catalog |
| `golang_cafe` | golang.cafe | **T3 rod** JSON API via shared `browserfetch` (plain HTTP = Vercel 429) | catalog |

Q-list (fixed): `vue` · `frontend` · `full-stack` · `typescript` · `react` · `golang` · `node` · `AI native` · `AI engineer` · `LLM`. Wellfound slugs: `frontend-engineer` · `full-stack-engineer` · `backend-engineer` · `artificial-intelligence-engineer`.

Live ingest is this nine-source set (`slots/utils.DefaultIngestSourceIDs`). `djinni` and `dou_ua` are dropped (historical `jobs` rows may remain until 7-day retention). LinkedIn is **cancelled** (never shipped; no package, no session cookies, no `SessionProvider`).

Debug HTTP (`cmd/agent`): `GET /health`, `POST /debug/collectors/{europe_remotely,working_nomads,himalayas,builtin,remotify_europe,we_work_remotely,wellfound,vue_jobs,golang_cafe}`.

### 2.5 What is *not* on the product path

- `pipeline/impl.Pipeline.Run` (collect → filters → score → mock dedup/notify) is the **001 skeleton**. `cmd/agent` still runs it when debug HTTP is unset. The **product** path is Temporal `ManualSlotRunWorkflow` + persist activities.
- Telegram / push: constitution and config struct leftover fields (`TelegramBotToken`, `TelegramChatID`) — **not loaded**, not wired. Out of MVP.
- Auth, users, payments: none. `jobs.user_id` exists for a later multi-user model.

## 3. Stack and versions

Sources: `go.mod` / `go.sum`, `Dockerfile`, `Dockerfile.migrate`, `docker-compose.yml`, `Makefile`.

| Layer | Fact |
|------|------|
| Language | Go **1.24.0** (`go.mod` `go 1.24.0`; Docker `golang:1.24-bookworm`) |
| HTTP | stdlib `net/http` ServeMux (method+path patterns). No Gin/Echo/Chi. |
| DB | PostgreSQL **16** + GORM **1.31.1** + `gorm.io/driver/postgres` **1.6.0**; pool via `pgx/v5` **5.6.0** |
| Migrations | `github.com/golang-migrate/migrate/v4` **v4.19.1**; SQL files in `migrations/` |
| Orchestration | Temporal SDK **v1.41.1** + `go.temporal.io/api` **v1.62.2**; Compose image `temporalio/auto-setup:1.29.1`, UI `temporalio/ui:2.48.1` |
| Cache / ingest coord | Redis **7-alpine** + `github.com/redis/go-redis/v9` **v9.18.0** |
| HTML | `github.com/PuerkitoBio/goquery` **v1.10.3** |
| Headless | `github.com/go-rod/rod` **v0.116.2** (listed **indirect** in `go.mod`; used by `collectors/browserfetch`). Image installs `chromium`. |
| LLM | Anthropic Messages HTTP API (no official Go SDK). Default model `claude-haiku-4-5`. Structured JSON via `output_config` + embedded JSON Schema. |
| Logging | `github.com/rs/zerolog` **v1.34.0** |
| JSON Schema | `github.com/santhosh-tekuri/jsonschema/v6` **v6.0.2** (public API bodies + LLM scoring schema) |
| IDs | `github.com/google/uuid` **v1.6.0** |
| Tests | `github.com/stretchr/testify` **v1.11.1**; `miniredis/v2` **v2.37.0**; `gorm.io/driver/sqlite` **v1.6.0** for storage unit tests |
| Lint / format | `gofmt` / `go vet` via Makefile. No golangci-lint config. No Prettier/ESLint. |

### direct `require` (`go.mod`)

| Package | Version |
|---------|---------|
| `github.com/PuerkitoBio/goquery` | v1.10.3 |
| `github.com/alicebob/miniredis/v2` | v2.37.0 |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 |
| `github.com/google/uuid` | v1.6.0 |
| `github.com/jackc/pgx/v5` | v5.6.0 |
| `github.com/redis/go-redis/v9` | v9.18.0 |
| `github.com/rs/zerolog` | v1.34.0 |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 |
| `github.com/stretchr/testify` | v1.11.1 |
| `go.temporal.io/api` | v1.62.2 |
| `go.temporal.io/sdk` | v1.41.1 |
| `gorm.io/driver/postgres` | v1.6.0 |
| `gorm.io/driver/sqlite` | v1.6.0 |
| `gorm.io/gorm` | v1.31.1 |

`replace`: `github.com/docker/docker` → `v28.3.3+incompatible`; OpenTelemetry otel/metric/trace pinned to **v1.37.0** (Temporal transitive).

Notable **indirect** used in app code: `github.com/go-rod/rod v0.116.2`.

## 4. Folder structure

```
jobhound_core/
├── .cursor/                 # rules, feature templates, audit-feature-tasks skill
├── cmd/
│   ├── agent/               # debug HTTP + leftover in-process pipeline + Temporal CLI
│   ├── api/                 # product HTTP (composition only)
│   ├── worker/              # Temporal worker (composition only)
│   ├── migrate/             # golang-migrate CLI
│   └── retention/           # one-shot job hard-delete
├── data/                    # countries.json (ISO lookup for collectors)
├── docker/temporal/         # Temporal dynamic config for Compose
├── docs/                    # REPO_SNAPSHOT.md (feature slices only while in flight)
├── internal/
│   ├── config/              # JOBHOUND_* names + typed loaders only
│   ├── domain/              # shared kernel: schema.Job / ScoredJob, identity utils
│   ├── collectors/          # Collector contract, sources, schema/, debughttp, bootstrap, browserfetch
│   ├── ingest/              # Redis coordinator, watermarks, ingest workflows
│   ├── jobs/                # job persistence + retention workflow
│   ├── pipeline/            # stages, persist activities, pipeline_run storage
│   ├── llm/                 # Scorer contract, anthropic/, mock/, scoring JSON Schema
│   ├── manual/              # ManualSlotRunWorkflow (parent orchestration)
│   ├── slots/               # slot use cases for public API
│   ├── profile/             # global profile text
│   ├── publicapi/           # HTTP handlers, schema, utils, embedded JSON Schema
│   └── platform/            # pgsql, logging, temporalopts (not a product module)
├── migrations/              # 000001_initial_schema.{up,down}.sql
├── tests/                   # reserved; empty (integration tests are colocated + build tag)
├── Dockerfile               # agent + worker + api
├── Dockerfile.migrate
├── docker-compose.yml
├── Makefile
└── go.mod / go.sum
```

Principle for `internal/<module>/`: `contract.go` (interfaces + sentinel errors) → `impl/` (use cases, **no** Temporal SDK) → `storage/` (GORM) → `schema/` (module data model) → `handlers/` (HTTP) → `workflows/` + `activities/` (Temporal; required if the module uses Temporal) → `utils/`.

Modules under `internal/`: `collectors`, `config`, `domain`, `ingest`, `jobs`, `llm`, `manual`, `pipeline`, `platform`, `profile`, `publicapi`, `slots`.

## 5. Configuration

| File | What is configured |
|------|----------------|
| `go.mod` | module path, Go 1.24.0, direct deps, `replace` pins |
| `Makefile` | `build` all five binaries to `bin/`; `run` / `run-debug` / `run-worker`; `test` / `test-integration`; `fmt` / `vet` / `tidy`; migrate; `docker-up` (build --no-cache + up -d --force-recreate --pull always) |
| `docker-compose.yml` | postgres 16, migrate one-shot, redis 7, temporal + UI + temporal-ready, agent :3001, worker, api :3000 |
| `Dockerfile` | multi-stage; CGO_ENABLED=0; debian-slim + chromium + `data/` |
| `Dockerfile.migrate` | migrate binary + `migrations/` |
| `.dockerignore` | `.git`, `bin/`, `.cursor`, `*.md`, `.env*` |
| `.gitignore` | `.env`, `.env.*`, `/bin/`, `vendor/`, sqlite, `cookies.txt` |
| `.cursor/rules/specify-rules.mdc` | always-applied stack + module layout, Canonical Enum, schema-first HTTP, Temporal separation |
| `.cursor/templates/` | concept / schemas-contracts / tasks for new `docs/<feature_slug>/` slices |
| `.cursor/skills/audit-feature-tasks/` | audit `*-tasks.md` against code |
| CI | **none** (no `.github/workflows`) |
| `.env.example` | **none** (local `.env` gitignored) |
| `commitlint` / OpenAPI / CSS | none |
| `.editorconfig` / LICENSE | none |

Non-standard: env names live only in `internal/config`. Feature packages must not `os.Getenv` shared knobs. Temporal **connection** is config; workflow **code** is per-module `workflows/`.

## 6. Typing

- **Language:** Go. No TypeScript / OpenAPI codegen.
- **Shared kernel:** `internal/domain/schema.Job` / `ScoredJob`. Identity: `domain/utils.StableJobID`, `NormalizeListingURL`, `AssignStableID`. No GORM and no Temporal SDK under `internal/domain/**`.
- **Module data model:** each feature’s `schema/` holds structs, enums, payloads, exported errors. HTTP request/response types for the product API live in `internal/publicapi/schema/`.
- **Canonical Enum Pattern** (constitution): exported string enums must have `String` / `Equals` / `Pointer` / `FromValue` / `ValuesT` / `FromStringT`. A bare `Valid() bool` is not enough — `pipeline.RunJobStatus` currently has **only** `Valid()` (debt).
- **HTTP input:** embed JSON Schema (`//go:embed handlers/json_schema/*.schema.json`) → `publicapi/utils.ValidateJSONInstance` → decode with `DisallowUnknownFields()`. Schemas: `create_slot`, `profile_put`, `stage2_run`, `stage3_run`, `patch_job_bucket`. Forbidden: `map[string]json.RawMessage` existence checks in handlers.
- **LLM output:** `internal/llm/schema/json_schema/job_scoring.schema.json` (`score` int, `rationale` string). Anthropic structured outputs; parsed in `llm/utils`.
- **Error contract (product API):** `{ "error": { "code": "snake_case", "message": "..." } }`. 500 message is always `"internal server error"`. Slot cap 409 also has top-level `"limit": 3`. No `ok` discriminator envelope (unlike Saynest backend-api-core).
- **Success bodies:** typed structs in `publicapi/schema` (slot card, job list, profile, stage-run accepted). `GET /api/v1/health` → `{ "status": "ok" }`. `DELETE` slot → **204** empty.

Error `code` strings in handlers: `method_not_allowed`, `idempotency_key_required`, `invalid_idempotency_key`, `validation_error`, `idempotency_key_conflict`, `slot_limit_reached`, `invalid_json`, `not_found`, `stage_already_running`, `no_pipeline_run`, `profile_required`, `invalid_stage`, `invalid_query`, `internal_error`.

## 7. Code conventions

Facts from constitution v1.8.3 and `.cursor/rules/specify-rules.mdc`:

- **Naming:** Go defaults (`PascalCase` exported, `camelCase` unexported). Package directories match module names (`publicapi`, `debughttp`, `browserfetch`).
- **Module layers:** contract → impl → storage; Temporal mapping in `workflows/` (not `impl/`). Handlers: `handler.go` + `registerRoutes()` + one file per route; **no** package-level helpers in a route file (`publicapi` helpers live in `publicapi/utils/`).
- **Composition:** `cmd/*` is thin — open DB, dial Temporal, construct repos/services, `ListenAndServe` / `worker.Run`.
- **State:** PostgreSQL is system of record; Redis is **only** ingest lock/cooldown (no search-result cache); Temporal holds workflow execution state.
- **Logging:** `platform/logging.NewRoot`; `RequestIDMiddleware` (`X-Request-ID`); field keys `handler` / `method` / `workflow` / `service` / `request_id` / `workflow_id` / `run_id` / `slot_id` / `user_id` / `pipeline_run_id` / `source_id`. Default format **console**; `JOBHOUND_LOG_FORMAT=json` for GCP-style stdout.
- **Comments:** names over banners; document invariants, not every method.
- **Tests:** colocated `*_test.go`, same package. Test **handlers / impl / storage**. Do not add trivia tests for thin utils/enums. Integration: `//go:build integration`.

## 8. Authentication and payments

### Authentication — absent (MVP)

- No JWT, sessions, OAuth, or API keys on `/api/v1`.
- Schema reserves `jobs.user_id` (nullable) for later isolation.
- CORS allowlist from `JOBHOUND_API_CORS_ORIGINS` (default `http://localhost:5173,http://localhost:3002`). Allowed headers: `Content-Type, Idempotency-Key`.

### Payments — absent

No Stripe/Paddle/billing modules.

## 9. Database

- **DBMS:** PostgreSQL 16 (Compose: user/db `jobhound`). Temporal uses a **second** Postgres (`temporal` user) — not the app schema.
- **ORM:** GORM `1.31.1` (`internal/platform/pgsql.Open`, `GormGetter`). Models in `*/storage/model.go` (and job/pipeline structs). App schema is owned by **SQL migrations**, not GORM AutoMigrate.
- **Migrate:** `github.com/golang-migrate/migrate/v4` file source. Single revision `000001_initial_schema` (consolidated former 000001–000005).
- **Tables:** `jobs`, `slots`, `slot_idempotency_keys`, `pipeline_runs`, `pipeline_run_jobs`, `ingest_watermarks`, `slot_jobs`, `user_profile` (singleton `id = 1`).
- **Job identity:** `jobs.id` TEXT = `StableJobID(source, listingURL)`.
- **Statuses:** `jobs.stage1_status` is `PASSED_STAGE_1` or NULL. `pipeline_run_jobs.stage2_status` ∈ `REJECTED_STAGE_2` \| `PASSED_STAGE_2`; `stage3_status` nullable `PASSED_STAGE_3` \| `REJECTED_STAGE_3` (only if stage 2 passed).
- **Seeds:** `INSERT INTO user_profile (id, text) VALUES (1, '')` in the up migration. No other seed package.
- **Retention:** hard-delete `jobs` where `created_at` older than 7 days; dependents via `ON DELETE CASCADE`.
- **SQLite:** test-only GORM driver for storage unit tests — not a runtime target.

## 10. Build, tests, CI

### Makefile

| Target | Purpose |
|--------|---------|
| `build` | `bin/agent`, `bin/worker`, `bin/api`, `bin/migrate`, `bin/retention` |
| `run` | build + run agent (loads `.env` if present) |
| `run-debug` | agent with `JOBHOUND_DEBUG_HTTP_ADDR=127.0.0.1:3001` |
| `run-worker` | Temporal worker |
| `test` | `go test ./...` (excludes `integration` tag) |
| `test-integration` | `go test -tags=integration ./...` |
| `fmt` / `vet` / `tidy` | gofmt, vet, mod tidy |
| `migrate-up` / `down` / `version` | `bin/migrate` |
| `docker-up` / `docker-down` / `docker-ps` / `docker-logs` / `docker-migrate` | Compose |

### Tests

- **Runner:** `go test`. **36** `*_test.go` files.
- **Unit (default):** handlers (`httptest`), `impl`, `storage` (often sqlite), collector parse fixtures, pipeline activities, ingest coordinator (miniredis), Temporal workflow tests via SDK test env (`manual_slot_run_test.go`).
- **Integration (`//go:build integration`):** `internal/platform/pgsql/migrations_integration_test.go`; `internal/ingest/coordinator_integration_test.go`; `internal/manual/workflows/client_integration_test.go`; `internal/collectors/browserfetch/rod_integration_test.go`. Need env / Compose (`JOBHOUND_DATABASE_URL`, Temporal, Chromium as applicable).
- **`tests/`:** empty directory (constitution allows optional `tests/integration/`).
- **e2e / frontend:** none in this repo.

### CI

**None.** No GitHub Actions under `.github/`.

## 11. Deploy

- **Production target (constitution):** GCP runtime + secrets. **No** Cloud Run workflow, Dockerfile deploy target, or Artifact Registry config is in this repo yet.
- **Local / current runtime:** Docker Compose on a laptop (`make docker-up`). Published ports: **5432** Postgres, **6379** Redis, **7233** Temporal gRPC, **8088** Temporal UI, **3000** API, **3001** agent debug HTTP.
- **Images:** `jobhound-core:local` (agent/worker/api); migrate uses `Dockerfile.migrate`. Chromium in the app image for Built In + Golang Cafe T3 (`JOBHOUND_BROWSER_NO_SANDBOX=1` as root).
- **CORS defaults:** `http://localhost:5173`, `http://localhost:3002` (frontend dev servers — not the API listen address).

### Environment variable names

From `internal/config` (single source). Values not listed.

| Name | Where / default |
|------|-----------------|
| `JOBHOUND_DATABASE_URL` | required for api/worker/migrate/retention |
| `JOBHOUND_MIGRATE_DATABASE_URL` | optional migrate-only DSN |
| `JOBHOUND_DB_MAX_OPEN_CONNS` | default 25 |
| `JOBHOUND_DB_MAX_IDLE_CONNS` | default 5 |
| `JOBHOUND_DB_CONN_MAX_LIFETIME_SEC` | default 3600 |
| `JOBHOUND_TEMPORAL_ADDRESS` | required for api/worker |
| `JOBHOUND_TEMPORAL_NAMESPACE` | default `default` |
| `JOBHOUND_TEMPORAL_TASK_QUEUE` | default `jobhound` |
| `JOBHOUND_API_LISTEN` | default `127.0.0.1:3000` (Compose `0.0.0.0:3000`) |
| `JOBHOUND_API_CORS_ORIGINS` | default `http://localhost:5173,http://localhost:3002` |
| `JOBHOUND_REDIS_URL` | worker ingest coordination; empty → no Redis coordinator |
| `JOBHOUND_INGEST_EXPLICIT_REFRESH` | default false |
| `JOBHOUND_INGEST_LOCK_TTL_SEC` | default 600 |
| `JOBHOUND_INGEST_COOLDOWN_TTL_SEC` | default 3600 |
| `JOBHOUND_PIPELINE_STAGE3_MAX_JOBS_PER_RUN` | default 20; clamp ≤ 10000 |
| `JOBHOUND_LOG_LEVEL` | default `info` |
| `JOBHOUND_LOG_FORMAT` | default `console` (`json` for structured) |
| `JOBHOUND_DATA_DIR` | empty → `data` cwd-relative (`countries.json`) |
| `JOBHOUND_DEBUG_HTTP_ADDR` | agent debug listen; empty → no debug server |
| `JOBHOUND_ANTHROPIC_API_KEY` | empty → mock scorer (score 0) |
| `JOBHOUND_ANTHROPIC_MODEL` | default `claude-haiku-4-5` |
| `JOBHOUND_JOB_RETENTION_SCHEDULE_UPSERT` | default true (worker creates weekly schedule) |
| `JOBHOUND_BROWSER_ENABLED` | default true; `0` disables rod |
| `JOBHOUND_BROWSER_BIN` | e.g. `/usr/bin/chromium` in Compose |
| `JOBHOUND_BROWSER_USER_DATA_DIR` | optional Chromium profile |
| `JOBHOUND_BROWSER_NAV_TIMEOUT_MS` | default 2 minutes |
| `JOBHOUND_BROWSER_NO_SANDBOX` | Compose sets `1` |
| `JOBHOUND_COLLECTOR_HIMALAYAS_DISABLED` | opt-out |
| `JOBHOUND_COLLECTOR_HIMALAYAS_MAX_PAGES` | 0 → collector default |
| `JOBHOUND_COLLECTOR_HIMALAYAS_SEARCH` | empty → browse feed |
| `JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS` | default 1000 |
| `JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER` | default true; `0` forces net/http for Built In |

`.env.example`: **absent**. Compose interpolates `JOBHOUND_ANTHROPIC_API_KEY` / `JOBHOUND_ANTHROPIC_MODEL` / `JOBHOUND_API_CORS_ORIGINS` from host env.

## 12. AI setup

| Artifact | Present? | Contents (list of rules/structure only, no paraphrased “meaning”) |
|----------|----------|------------------------------------------------------------------|
| `.cursorrules` | no | — |
| `AGENTS.md` / `CLAUDE.md` | no | — |
| `.cursor/rules/specify-rules.mdc` | yes | Always-applied: stack, `cmd/` vs `internal/`, collectors `schema/`, debughttp layout, Canonical Enum, publicapi JSON Schema, anti-patterns, testing, make targets |
| `.cursor/templates/` | yes | `concept.md`, `schemas-contracts.md`, `tasks.md`, README |
| `.cursor/skills/audit-feature-tasks/` | yes | audit `*-tasks.md` vs code |
| `docs/` | `REPO_SNAPSHOT.md` only (no in-flight feature slice) | — |

## 13. Documentation

| File | Contents | Currency (factual mismatches) |
|------|----------|-------------------------------|
| `README.md` | One paragraph + Docker two-liner + migrate hint | Does not list binaries, env vars, API routes, or Compose ports (those are in `docker-compose.yml` comments) |
| `.cursor/rules/specify-rules.mdc` | Engineering constitution | Mentions `cmd/api/` as future in one paragraph and as implemented elsewhere; `cmd/api` exists. Mentions “scheduled events”; listing cron is backlog, **job retention** schedule exists |
| `LICENSE` | — | **Missing** |

## 14. Scaffold vs business logic

### Reusable skeleton (backend template)

- Go module layout: `cmd/*` composition + `internal/<feature>/{contract,impl,storage,schema,handlers,workflows,utils}`
- `internal/config` as the only `os.Getenv` surface
- `internal/platform/pgsql` + `golang-migrate` SQL
- `internal/platform/logging` (zerolog, request id, field names)
- Temporal: per-module `workflows/` + `RegisterWorkflow` in `New`/`Register`; no SDK in `impl/`
- Product HTTP: `net/http` mux, CORS, embedded JSON Schema + typed decode
- Docker Compose Postgres + Temporal + worker + API
- Feature docs: `.cursor/templates/` → `docs/<feature_slug>/`; audit via `.cursor/skills/audit-feature-tasks`

### Project-specific (remove when turning into a template)

- Job-board collectors (`europe_remotely`, `working_nomads`, `builtin`, `himalayas`, `remotify_europe`, `we_work_remotely`, `wellfound`, `vue_jobs`, `golang_cafe`) and `browserfetch` / Chromium
- Slot / profile / three-stage pipeline semantics and `ManualSlotRunWorkflow` run kinds
- Redis ingest lock/cooldown keys `ingest:lock|cooldown:{slot_id}:{source_id}:{query_segment}`
- Anthropic scoring schema and pass threshold 60
- `data/countries.json`
- Job retention 7 days + Temporal schedule `jobhound-job-retention`
- Debug HTTP `/debug/collectors/*`
- Compose service names `jobhound-*`, default CORS localhost frontend ports
- Leftover Telegram fields on `config.Config`

## 15. Debt and unfinished spots

- **LinkedIn:** cancelled, not planned. No package, no cookies file, no `SessionProvider`.
- **`cmd/agent` noop path:** without `JOBHOUND_DEBUG_HTTP_ADDR`, runs `pipeline/impl.Pipeline` with mock scorer/dedup/notify and prints `noop pipeline run ok`. Not the product path.
- **`config.Config` dead fields:** `TelegramBotToken`, `TelegramChatID`, `HTTPUserAgent`, `IncludeKeywords`, `ExcludeKeywords` — declared, **not** filled by `Load()`.
- **`go-rod`:** used in production code but `// indirect` in `go.mod` (should be a direct require).
- **Canonical Enum:** `pipeline.RunJobStatus` only has `Valid()`.
- **`pipeline/stage_rules.go`:** still at module root; constitution says move to `pipeline/schema/` when touched.
- **Empty `JOBHOUND_ANTHROPIC_API_KEY`:** worker scores with mock **0** → all stage-3 rows `REJECTED_STAGE_3` (threshold 60). Easy to miss operationally.
- **Worker without Redis URL:** ingest workflows register with nil coordinator; ingest activities cannot take locks (fail closed / incomplete ingest).
- **No `.env.example`**, no LICENSE, no CI, no OpenAPI.
- **README** is thinner than Compose.
- **Scheduled vacancy auto-refresh:** explicit backlog. Retention cron is unrelated (delete old `jobs` rows).
- **Auth / multi-user / Telegram push:** out of MVP; `user_id` column unused.
- **`tests/`:** empty placeholder.
- **GCP deploy:** named as production target; not implemented in-repo.
- **Himalayas nil:** if `JOBHOUND_COLLECTOR_HIMALAYAS_DISABLED`, debug route returns 500; worker omits the source from the collector map (API `DefaultIngestSourceIDs` still lists `himalayas`).
- **Golang Cafe nil:** constructed only when rod `HTMLDocumentFetcher` exists (`JOBHOUND_BROWSER_ENABLED` + Built In browser path). Without it, ingest map has no `golang_cafe` while `DefaultIngestSourceIDs` still lists it.

---

### Candidates for a shared layer

| Element | File / folder | Likely same thing exists in my other repos |
|---------|---------------|---------------------------------------------|
| Feature module layout (contract/impl/storage/schema/handlers/workflows) | `internal/*`, `.cursor/rules/specify-rules.mdc` | yes (`backend-api-core` `src/internal`; omg-bo style) |
| Env-only in `config` | `internal/config` | yes (Saynest `src/config`) |
| Schema-first JSON body validation | `publicapi/handlers/json_schema` + jsonschema/v6 | yes (Saynest Ajv/Zod; omg-ap embed) |
| Temporal per-feature `workflows/` | `ingest/workflows`, `manual/workflows`, `pipeline/workflows`, `jobs/workflows` | don’t know (this repo’s Go pattern) |
| Zerolog + request id | `internal/platform/logging` | no (Saynest Winston); same *idea* |
| Feature docs templates + audit skill | `.cursor/templates/`, `.cursor/skills/audit-feature-tasks/` | yes (Monterra `docs/<slug>/`) |
| Docker Compose Postgres + migrate | `docker-compose.yml` | don’t know |
| GORM + golang-migrate SQL | `platform/pgsql`, `migrations/` | no (Saynest is Mongoose) |
| Collector / rod / Anthropic | `internal/collectors`, `internal/llm/anthropic` | no |
| API error envelope `{error:{code,message}}` | `publicapi/schema/errors.go` | related but **different** from Saynest `{ok, result\|error}` registry |
| CORS + Idempotency-Key | `publicapi/utils/cors.go` | no |
