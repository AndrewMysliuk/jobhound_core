# STACK

Generated inventory: versions, config files, make targets, env names.
Regenerate with `make docs-stack` (`scripts/gen_stack_docs.go`).
Sources: `go.mod`, `docker-compose.yml`, `Dockerfile`, `Makefile`, `.env.example`, CI workflow.
Do not edit by hand — change the sources or the generator.

Product narrative (handwritten): [`SYSTEM.md`](./SYSTEM.md).

## 3. Stack and versions

Sources: `go.mod` / `go.sum`, `Dockerfile`, `Dockerfile.migrate`, `docker-compose.yml`, `Makefile`.

| Layer | Fact |
|------|------|
| Language | Go **1.24.0** (`go.mod` `go 1.24.0`; Docker `golang:1.24-bookworm`) |
| HTTP | stdlib `net/http` ServeMux (method+path patterns). No Gin/Echo/Chi. |
| DB | PostgreSQL **16** + GORM **1.31.1** + `gorm.io/driver/postgres` **1.6.0**; pool via `pgx/v5` **5.6.0** |
| Migrations | `github.com/golang-migrate/migrate/v4` **v4.19.1**; SQL files in `migrations/` |
| Orchestration | Temporal SDK **v1.41.1**; Compose image `temporalio/auto-setup:1.29.1`, UI `temporalio/ui:2.48.1`. `go.temporal.io/api` **v1.62.2** is a direct `require`. |
| Cache / ingest coord | Redis **7-alpine** + `github.com/redis/go-redis/v9` **v9.18.0** |
| HTML | `github.com/PuerkitoBio/goquery` **v1.10.3** |
| Headless | `github.com/go-rod/rod` **v0.116.2** (direct `require`; used by `collectors/browserfetch`). Image installs `chromium`. |
| Profiles | `gopkg.in/yaml.v3` **v3.0.1**. Directory from `JOBHOUND_PROFILES_DIR` (empty → `config/profiles`). |
| Scoring | In-process rules in `internal/scoring/impl` (phrase gates + fact signals from ready verdicts). |
| Enrich | `internal/enrich`: Anthropic Message Batches over stdlib `net/http` (no Anthropic SDK). Model reply JSON Schema embedded under `enrich/schema/json_schema/`. |
| Companies / ATS | `internal/companies` + `internal/collectors/ats/{greenhouse,lever,ashby}`; seed xlsx via `excelize` **v2.9.0** (`cmd/companies-import`). |
| Text / currency | `golang.org/x/text` **v0.34.0** (NFC for enrich hashes; currency/language helpers in coerce). |
| Logging | `github.com/rs/zerolog` **v1.34.0** |
| JSON Schema | `github.com/santhosh-tekuri/jsonschema/v6` **v6.0.2** (product API bodies + enrich model reply) |
| IDs | `github.com/google/uuid` **v1.6.0** |
| Tests | `github.com/stretchr/testify` **v1.11.1**; `miniredis/v2` **v2.37.0**; `gorm.io/driver/sqlite` **v1.6.0** for storage unit tests |
| Lint / format | golangci-lint **v2** (`.golangci.yml`: `errcheck`, `govet`, `staticcheck`, `unused`, `errorlint`, `depguard`; formatter `gofmt`). `make lint` / `make fmt` / `make vet`. CI installs **v2.13.2**. No Prettier/ESLint in this repo. |
| UI | Sibling `jobhound_frontend`: Nuxt **^4.2.1**, Vue **^3.5.25**. |

### direct `require` (`go.mod`)

| Package | Version |
|---------|---------|
| `github.com/PuerkitoBio/goquery` | v1.10.3 |
| `github.com/alicebob/miniredis/v2` | v2.37.0 |
| `github.com/go-rod/rod` | v0.116.2 |
| `github.com/golang-migrate/migrate/v4` | v4.19.1 |
| `github.com/google/uuid` | v1.6.0 |
| `github.com/jackc/pgx/v5` | v5.6.0 |
| `github.com/redis/go-redis/v9` | v9.18.0 |
| `github.com/rs/zerolog` | v1.34.0 |
| `github.com/santhosh-tekuri/jsonschema/v6` | v6.0.2 |
| `github.com/stretchr/testify` | v1.11.1 |
| `github.com/xuri/excelize/v2` | v2.9.0 |
| `go.temporal.io/api` | v1.62.2 |
| `go.temporal.io/sdk` | v1.41.1 |
| `golang.org/x/text` | v0.34.0 |
| `gopkg.in/yaml.v3` | v3.0.1 |
| `gorm.io/driver/postgres` | v1.6.0 |
| `gorm.io/driver/sqlite` | v1.6.0 |
| `gorm.io/gorm` | v1.31.1 |

`replace`: `github.com/docker/docker => github.com/docker/docker v28.3.3+incompatible`; `go.opentelemetry.io/otel => go.opentelemetry.io/otel v1.37.0`; `go.opentelemetry.io/otel/metric => go.opentelemetry.io/otel/metric v1.37.0`; `go.opentelemetry.io/otel/trace => go.opentelemetry.io/otel/trace v1.37.0`.

## 5. Configuration

| File | What is configured |
|------|----------------|
| `go.mod` | module path, Go version, direct deps, `replace` pins |
| `Makefile` | `build` six app binaries to `bin/` (agent, worker, api, migrate, retention, companies-import); `bin/ipv6proxy` is a separate target used by `start-ipv6-proxy`; `companies-import` runs the seed CLI; `eval` runs `go run ./cmd/eval`; `run` / `run-debug` / `run-worker`; `test` / `test-integration`; `fmt` / `vet` / `lint` (`golangci-lint run`) / `tidy`; migrate; `db-dump` / `db-restore` (full Postgres dump under gitignored `data/local/`, restore then migrate); `docker-up` starts the host proxy, then Compose `build --no-cache` + `up -d --force-recreate --pull always`; `docker-down` stops the proxy; `docs-stack` regenerates `docs/STACK.md` |
| `docker-compose.yml` | postgres 16, migrate one-shot, redis 7, temporal + UI + temporal-ready, agent :3001, worker, api :3000. Worker and agent get `JOBHOUND_EUROPE_REMOTELY_PROXY=http://host.docker.internal:18080`. `api` and `worker` get `JOBHOUND_PROFILES_DIR=/app/config/profiles`, `JOBHOUND_COMPANIES_DIR=/app/config/companies`, and optional `JOBHOUND_ANTHROPIC_API_KEY` / `JOBHOUND_ENRICH_MODEL`. The proxy process itself is not a Compose service |
| `Dockerfile` | multi-stage; CGO_ENABLED=0; debian-slim + chromium + `data/` + `config/profiles` + `config/companies` |
| `Dockerfile.migrate` | migrate binary + `migrations/` |
| `.dockerignore` | `.git`, `bin/`, `.cursor`, `*.md`, `.env*` |
| `.gitignore` | `.env`, `.env.*`, `/bin/`, `vendor/`, sqlite, `cookies.txt`, dump path under `data/local/` |
| `.cursor/rules/specify-rules.mdc` | always-applied stack + module layout, Canonical Enum, schema-first HTTP, Temporal separation |
| `.cursor/templates/` | concept / schemas-contracts / tasks for new `docs/<feature_slug>/` slices |
| `.cursor/skills/audit-feature-tasks/` | audit `*-tasks.md` against code |
| `.cursor/skills/calibrate-profile/` | calibrate YAML/scorer from pasted TOP vacancies |
| CI | `.github/workflows/ci.yml` on push and pull request: `gofmt -l` empty, `make vet`, `make lint`, `make test`. No integration tag. No deploy |
| `.env.example` | every key `internal/config` loads; secrets empty; default in a comment. README links it and does not paste the table |
| `commitlint` / OpenAPI / CSS | none |
| `.editorconfig` | `utf-8`, `lf`, final newline, trim trailing space. `*.go` tabs. `*.{yml,yaml,json,md}` indent 2. `*.md` does not trim trailing space |
| LICENSE | **none** |

Non-standard: env names live only in `internal/config`. Feature packages must not `os.Getenv` shared knobs. Temporal **connection** is config; workflow **code** is per-module `workflows/`.

## 10. Build, tests, CI

### Makefile

| Target | Purpose |
|--------|---------|
| `build` | `bin/agent`, `bin/worker`, `bin/api`, `bin/migrate`, `bin/retention`, `bin/companies-import` (`bin/ipv6proxy` and `cmd/eval` are not part of `build`) |
| `run` | build + run agent (loads `.env` if present) |
| `run-debug` | agent with `JOBHOUND_DEBUG_HTTP_ADDR=127.0.0.1:3001` |
| `run-worker` | Temporal worker |
| `eval` | `go run ./cmd/eval` (default `testdata/enrich/gold.jsonl` + `predictions.jsonl`) |
| `companies-import` | build + run `bin/companies-import` (seed `companies` from xlsx under `JOBHOUND_COMPANIES_DIR`) |
| `test` | `go test ./...` (excludes `integration` tag) |
| `test-integration` | `go test -tags=integration ./...` |
| `fmt` | `go fmt ./...` |
| `vet` | `go vet ./...` |
| `lint` | `golangci-lint run` |
| `tidy` | `go mod tidy` |
| `migrate-up` | `bin/migrate up` |
| `migrate-down` | `bin/migrate down` |
| `migrate-version` | `bin/migrate version` |
| `db-dump` | Full `pg_dump` via `JOBHOUND_DATABASE_URL` into gitignored `data/local/` |
| `db-restore` | Restore that dump, then run migrations |
| `start-ipv6-proxy` | Build `bin/ipv6proxy` if needed; listen on `127.0.0.1:18080` (`-detach`, pid in `bin/ipv6proxy.pid`, log `bin/ipv6proxy.log`). Health check is `GET /health` |
| `stop-ipv6-proxy` | Stop host ipv6 proxy via `bin/ipv6proxy.pid` |
| `docker-up` | Compose. Depends on `start-ipv6-proxy` |
| `docker-down` | Compose down; runs `stop-ipv6-proxy` first |
| `docker-ps` | Compose `ps` |
| `docker-logs` | Compose `logs -f` |
| `docker-migrate` | Compose `run --rm migrate` |
| `docs-stack` | Regenerate `docs/STACK.md` from go.mod / Compose / Makefile / `.env.example` |
| `build-retention` | Alias of `build` |
| `\` | (see Makefile) |

### Tests

- **Runner:** `go test`. **73** `*_test.go` files.
- **Unit (default):** handlers (`httptest`) for the five profile routes plus `GET /companies`, `profiles/impl` YAML load (including `architecture.yaml`, `children`, `facts`, `ats_polling`), `scoring/impl` phrase + fact tables, `scoring/storage` / `jobs/storage` / `enrich/storage` / `companies/storage` (often sqlite), companies impl + ATS fetcher fixtures (Greenhouse / Lever / Ashby), enrich batches/coerce/decode/eval, collector parse fixtures (software and architecture boards), ingest coordinator (miniredis).
- **Integration (`//go:build integration`):** `internal/platform/pgsql/migrations_integration_test.go`; `internal/ingest/coordinator_integration_test.go`; `internal/collectors/browserfetch/rod_integration_test.go`. Need env / Compose (`JOBHOUND_DATABASE_URL`, Chromium as applicable).
- **`tests/`:** empty directory (constitution allows optional `tests/integration/`). Offline enrich fixtures live under `testdata/enrich/`.
- **Frontend unit tests:** live in `jobhound_frontend` (`pnpm test:unit`), not in this repo.

### CI

`.github/workflows/ci.yml` on push and pull request (`ubuntu-latest`, Go from `go.mod`): `gofmt -l` must be empty, then `make vet`, `make lint` (golangci-lint **v2.13.2**), `make test`. No `integration` tag. No deploy job.

## 11. Deploy

- **Production target (constitution):** GCP runtime + secrets. **No** Cloud Run workflow, Dockerfile deploy target, or Artifact Registry config is in this repo yet.
- **Local / current runtime:** Docker Compose on a laptop (`make docker-up`). Published ports: **5432** Postgres, **6379** Redis, **7233** Temporal gRPC, **8088** Temporal UI, **3000** API, **3001** agent debug HTTP. Host proxy **127.0.0.1:18080** is not a Compose port.
- **Images:** `jobhound-core:local` (agent/worker/api); migrate uses `Dockerfile.migrate`. Chromium in the app image for Built In, Golang Cafe, and the architecture rod boards `ejobs`, `dezeen_jobs`, and `world_architects` (`JOBHOUND_BROWSER_NO_SANDBOX=1` as root). `config/profiles` and `config/companies` are in the app image.
- **CORS defaults:** `http://localhost:5173`, `http://localhost:3002` (frontend dev servers — not the API listen address).

### Environment variable names

From `internal/config` (single source). Values not listed. Rows below are taken from `.env.example` comments.

| Name | Where / default |
|------|-----------------|
| `JOBHOUND_DATABASE_URL` | App Postgres DSN. Required for DB use; no default. |
| `JOBHOUND_MIGRATE_DATABASE_URL` | Migrate DSN override. Default: falls back to JOBHOUND_DATABASE_URL. |
| `JOBHOUND_DB_MAX_OPEN_CONNS` | Pool max open. Default: 25. |
| `JOBHOUND_DB_MAX_IDLE_CONNS` | Pool max idle. Default: 5. |
| `JOBHOUND_DB_CONN_MAX_LIFETIME_SEC` | Conn max lifetime, seconds. Default: 3600. |
| `JOBHOUND_API_LISTEN` | cmd/api address. Default: 127.0.0.1:3000. |
| `JOBHOUND_API_CORS_ORIGINS` | Browser origins, comma-separated. Default when unset: http://localhost:5173,http://localhost:3002. Set empty to allow none. |
| `JOBHOUND_REDIS_URL` | Ingest lock Redis. Default: empty (ingest fails closed without it). |
| `JOBHOUND_INGEST_EXPLICIT_REFRESH` | Bypass cooldown. Default: false. |
| `JOBHOUND_INGEST_LOCK_TTL_SEC` | Lock TTL, seconds. Default: 600. |
| `JOBHOUND_INGEST_COOLDOWN_TTL_SEC` | Cooldown TTL, seconds. Default: 3600. |
| `JOBHOUND_LOG_LEVEL` | zerolog level. Default: info. |
| `JOBHOUND_LOG_FORMAT` | console or json. Default: console. |
| `JOBHOUND_DATA_DIR` | Dir with countries.json. Default: empty (data relative to the process). |
| `JOBHOUND_DEBUG_HTTP_ADDR` | cmd/agent debug listen. Default: empty (debug HTTP off). |
| `JOBHOUND_COLLECTOR_HIMALAYAS_DISABLED` | Skip Himalayas. Default: false. |
| `JOBHOUND_COLLECTOR_HIMALAYAS_MAX_PAGES` | Page cap. 0 collector default, -1 unlimited. Default: 0. |
| `JOBHOUND_COLLECTOR_HIMALAYAS_SEARCH` | Search query. Default: empty (full feed). |
| `JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS` | Built In delay, milliseconds. Default: 1000. |
| `JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER` | Built In via rod. Default: true. 0 / false / no / off forces net/http. |
| `JOBHOUND_BROWSER_ENABLED` | Rod on/off. Default: true. 0 / false / no / off disables. |
| `JOBHOUND_BROWSER_BIN` | Chromium binary. Default: empty. |
| `JOBHOUND_BROWSER_USER_DATA_DIR` | Chromium profile dir. Default: empty. |
| `JOBHOUND_BROWSER_NAV_TIMEOUT_MS` | Navigation timeout, milliseconds. Default: 120000. |
| `JOBHOUND_BROWSER_NO_SANDBOX` | Chromium --no-sandbox. Default: false. |
| `JOBHOUND_EUROPE_REMOTELY_PROXY` | Docker Compose sets http://host.docker.internal:18080; make docker-up listens there via cmd/ipv6proxy. |
| `JOBHOUND_TEMPORAL_ADDRESS` | Temporal frontend. Required for worker and API workflow start; no default. |
| `JOBHOUND_TEMPORAL_NAMESPACE` | Namespace. Default: default. |
| `JOBHOUND_TEMPORAL_TASK_QUEUE` | Task queue. Default: jobhound. |
| `JOBHOUND_JOB_RETENTION_SCHEDULE_UPSERT` | Worker creates the weekly schedule. Default: true. |
| `JOBHOUND_PROFILES_DIR` | Profile YAML directory. Empty → config/profiles (relative to the process). |
| `JOBHOUND_JOB_RETENTION_DAYS` | Retention window, days. Default: 30. Must be a positive integer. |
| `JOBHOUND_ANTHROPIC_API_KEY` | Anthropic Message Batches key. Empty disables vacancy enrich and ignores JOBHOUND_ENRICH_MODEL. |
| `JOBHOUND_ENRICH_MODEL` | Model id stored on the verdict row. Required when the key is set. |
| `JOBHOUND_ENRICH_MIN_DESCRIPTION_CHARS` | Skip enrich for vacancies with shorter descriptions. Empty or 0 → 800. Negative is rejected. |
| `JOBHOUND_ENRICH_WINDOW_DAYS` | first_seen_at window for enrich candidates (open jobs only). Empty or 0 → 30. Negative is rejected. |
| `JOBHOUND_COMPANIES_DIR` | Company registry dir (blocklist.yaml + seed). Empty → config/companies. |
| `JOBHOUND_COMPANIES_POLL_MAX_PER_RUN` | Cap on board requests per run (duration / politeness), not memory; memory is bounded by ATSConcurrency. Default: 300. Must be a positive integer. |
| `JOBHOUND_COMPANIES_POLL_INTERVAL_HOURS` | Per-company due interval, hours. Default: 6. Must be a positive integer. |
| `JOBHOUND_COMPANIES_ATS_CONCURRENCY` | Parallel polls per ATS. Default: 5. Must be a positive integer. |
| `JOBHOUND_COMPANIES_RESOLVE_RETRY_DAYS` | UNRESOLVED → next_resolve_at offset, days. Default: 14. Must be a positive integer. |

`.env.example`: present. Lists every key in the table above; secrets empty; default in a comment. `README.md` links that file and does not paste a second list. Compose interpolates `JOBHOUND_API_CORS_ORIGINS`, `JOBHOUND_ANTHROPIC_API_KEY`, and `JOBHOUND_ENRICH_MODEL` from host env.
