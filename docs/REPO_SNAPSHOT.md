## 1. Purpose

Repository remote `AndrewMysliuk/jobhound_core` (`go.mod` module: `github.com/andrewmysliuk/jobhound_core`; README: “jobhound_core”) is the JobHound backend: ingest job listings from public boards, persist one row per vacancy in PostgreSQL, score that pool with in-process phrase rules, and expose a JSON HTTP API. Role: backend / worker / debug collector process. The product UI is the sibling Nuxt app `jobhound_frontend` (section 2.6).

MVP is **single-tenant** (no auth). The unit of search is a **profile** loaded from YAML, not a slot. Two profiles are shipped: `andrew` (`domain: software`) and `lada` (`domain: architecture`). Product HTTP is API-first (`cmd/api`). Collectors debug HTTP (`cmd/agent`) is development-only. Search phrases exist only in `config/profiles/*.yaml`. The browser does not send queries or scoring phrases.

## 2. How the system works

### 2.1 Product model

A profile file (`config/profiles/<id>.yaml`) is the only copy of:

1. **`sources`** — which collectors to poll. The allow-list is the nine software aggregators, the twelve architecture boards, and the Consider and Getro source ids in section 2.4. A file lists only the ids it polls.
2. **`queries`** — stage-1 search strings only. They do not add score. `frontend` and `full-stack` are not query strings.
3. **`children`** — optional map of source id → slugs. A listed source gets one ingest child per slug and does not copy `queries`. The slug `catalog` is an empty query. A key that is not in `sources` makes the file invalid. Not returned by `GET /api/v1/profiles`.
4. **`wellfound_roles`** — Wellfound `/role/r/{slug}` segments. They address that one site. They are not search words for other boards and they do not affect the score. `lada.yaml` leaves this list empty.
5. **`gate_role`**, **`gate_hard`**, **`gate_location`**, **`signals_plus`**, **`penalties_soft`**, **`buckets.top`** — the score. `gate_location` holds `allow_countries`, `allow_regions`, and `reject_timezones`. Not returned by `GET /api/v1/profiles`.

`domain` is `software` or `architecture`. The shipped files are `andrew.yaml` and `lada.yaml`. There is no `architecture.yaml`. Profiles are read on every `List`/`Get` (no cache). They are not created, updated, or deleted over HTTP. The only user-writable field is `user_status` on a match (`NEW` or `HIDDEN`).

**Stage 1** is ingest. It does not score. A profile run expands `sources` into child `IngestSourceWorkflow`s. Keyword sources, including `getro_*` and any source with no `children` entry, get one child per `queries` phrase. Wellfound gets one child per `wellfound_roles` slug. `we_work_remotely`, `vue_jobs`, and `golang_cafe` get one catalog child (`queries` ignored). Each `consider_*` source gets one catalog child (`Queries` set, lock segment `catalog`). A source listed in `children` gets one child per slug and does not copy `queries`. Redis keys are `ingest:lock:{source_id}:{query_segment}` and `ingest:cooldown:{source_id}:{query_segment}`; an empty query uses the segment `catalog`. Architecture source ids use a fixed 12-hour cooldown. Other sources use `JOBHOUND_INGEST_COOLDOWN_TTL_SEC` (default 3600). Ingest drops a row whose `posted_at` is known and strictly older than 30 days, or 90 days when the source id is an architecture board. A zero `posted_at` is stored.

**Stage 2** scores stored rows whose `first_seen_at` is inside 10 days before `profile_runs.started_at`, and rows whose `posted_at` is already older than the profile cutoff: 30 days when `domain` is `software`, 90 when `domain` is `architecture`. All five steps always run. Nothing returns on the first hit. Score is the sum of signal deltas. The bucket is chosen at the end: any gate signal is `REJECTED`; otherwise `score >= buckets.top` is `TOP`; otherwise `REVIEW`, including 0 and negative. Phrase match is case-insensitive and word-bounded (`java` does not hit `javascript`). Country and region codes match the whole code, case-insensitive. Diacritics are not folded.

`gate_role` is title only: one `GATE_ROLE` per phrase, delta 0, no negation. `gate_hard` is title, description, and `location.raw`, negation-checked: one `GATE_HARD` per phrase that still hits. `gate_location`: a known country or region that misses both allow lists emits one `CUT_LOCATION` per code (another allowed code does not cancel it); empty allow lists, or empty job countries and regions, are not a reject. Timezone phrases match title, description, `location.raw`, and `location.timezone`, negation-checked, one `CUT_TIMEZONE` each. A known `posted_at` strictly older than that profile cutoff emits `CUT_POSTED_AT`; a zero `posted_at` is not a reject. `signals_plus` walks title terms, then body terms up to `text.max_total` (a term after the cap still fires with delta 0), then salary (parsed upper bound against `min_eur_year` or `min_usd_year`; no salary and an unparseable string add nothing), then freshness (`posted_at`, else `first_seen_at`: within 3 days `days_3`, else within 7 days `days_7`). `penalties_soft` emits one `PENALTY` per group: a title hit fires once even under `min_occurrences`; body hits fire at the threshold (`0` and `1` both mean one hit); negation runs only when the group sets `check_negation`. The negation window is the 50 characters before the phrase, clamped at the previous `.`, `!`, or `?`; `not`, `no`, `n't`, or a `non-` prefix drops the hit. `queries` do not score. Weights, the body cap, `buckets.top`, and `usd_per_eur` are profile YAML. There is no `min_score` parameter, no `PASSED` bucket, and no LLM. A signal is `{code, rule, phrase, delta, occurrences}` with codes `GATE_ROLE`, `GATE_HARD`, `CUT_LOCATION`, `CUT_TIMEZONE`, `CUT_POSTED_AT`, `PLUS`, `PENALTY`. Every list returns its scores, including 0 and below. The default bucket is `TOP`.

One vacancy is one `jobs` row. `id` is `company_key + U+001E + normalized title` (trim, collapse whitespace, lowercase); location is not part of `id`. Re-ingest appends an `openings` element `{country, url, apply_url}` deduped by normalized country + url + apply_url, unions `location.countries` and `location.regions`, and sets `location.raw` to the sorted country labels joined by `", "`. It keeps `first_seen_at`, moves `last_seen_at`, fills an empty `company_website`, and leaves description and salary from the first row. Parent `url` / `apply_url` are the first opening after that sort. A recompute overwrites score, bucket, and signals and keeps `user_status`.

There is no stage 3, no Anthropic call, no global profile text, and no client-side keyword default.

### 2.2 Processes

One Docker Compose stack (or local `make` binaries) runs several processes:

| Process | Binary | Role |
|---------|--------|------|
| `api` | `cmd/api` | Product JSON API on **:3000** (`/api/v1/...`). Composition only. Does not fetch boards. Loads the profile file store and the scoring service, and passes them to the five profile routes. `POST .../runs` starts `ProfileRunWorkflow` through Temporal. |
| `worker` | `cmd/worker` | Temporal worker on task queue `jobhound`. Registers ingest, job retention, and `ProfileRunWorkflow`. Owns collectors + Redis. Reads `JOBHOUND_PROFILES_DIR` for ingest and scoring. |
| `agent` | `cmd/agent` | Dev-only collector debug HTTP on **:3001** when `JOBHOUND_DEBUG_HTTP_ADDR` is set. Exits if that address is empty. |
| `migrate` | `cmd/migrate` | `golang-migrate` CLI (`up` / `down` / `version` / `force`). |
| `retention` | `cmd/retention` | One-shot hard-delete of jobs whose `last_seen_at` is older than **30 days** (`JOBHOUND_JOB_RETENTION_DAYS`; same cutoff as the Temporal schedule). |
| `ipv6proxy` | `cmd/ipv6proxy` | Host-only HTTP CONNECT proxy on **127.0.0.1:18080**. Not a Compose service. `make docker-up` starts it (`start-ipv6-proxy`); `make docker-down` stops it. Dials targets IPv6-first. `GET /health` → `ok`. |

Infra beside the app: **Postgres 16** (app data), **Redis 7** (ingest lock + cooldown), **Temporal** `auto-setup:1.29.1` on its **own** Postgres, Temporal UI **:8088**. The IPv6 proxy runs on the **host**, outside Compose: the worker container has no IPv6 route, and euremotejobs.com answers IPv4 with a SiteGround challenge.

```
browser / jobhound_frontend
        │  HTTP JSON  :3000
        ▼
     cmd/api  ──ExecuteWorkflow──►  Temporal  ◄── poll ──  cmd/worker
        │                              │                      │
        │                              │                      ├─ collectors (HTTP / goquery / rod)
        │                              │                      ├─ Redis lock/cooldown
        │                              │                      └─ phrase score (no network)
        └── GORM / Postgres ◄──────────┴──────────────────────┘
              ▲
              └── config/profiles/*.yaml (api and worker; JOBHOUND_PROFILES_DIR)
```

The arrow from `cmd/api` to Temporal is the run start in section 2.3. `cmd/api` executes `ProfileRunWorkflow`; `cmd/worker` polls that workflow and runs ingest plus the phrase score.

`cmd/agent` debug HTTP is a **side door**: it hits collectors directly and returns job JSON. It does not write profile runs or matches.

`Dockerfile` copies `config/profiles` next to the binaries. Compose sets `JOBHOUND_PROFILES_DIR=/app/config/profiles` on `api` and `worker`.

### 2.3 End-to-end happy path

**Start a run**

1. `POST /api/v1/profiles/{profile_id}/runs` with header `Idempotency-Key` (non-nil UUID) and an empty body. A JSON body is `400`.
2. The handler writes a `profile_runs` row (`RUNNING`) and starts `ProfileRunWorkflow`, workflow id `profile-run-{profile_id}`, reuse `ALLOW_DUPLICATE` after close. Input is `ProfileRunInput{ProfileID, RunID}` only. Activities re-read the YAML.
3. A second POST while that execution is open is **409** `PROFILES.RUN_ALREADY_RUNNING`. The same idempotency key returns the same run. The same key with a different profile is **409** `HTTP.IDEMPOTENCY_KEY_CONFLICT`.
4. Ingest children run at the same time across resources, and in order inside one resource. `builtin` and `golang_cafe` share the lane `browser` (one Chromium). `ejobs`, `dezeen_jobs`, and `world_architects` share `architecture-browser`. Every other source is its own lane. Workflow id is `ingest-{source}-{segment}` (`catalog` when the query is empty; spaces in the segment become hyphens). Each child takes a Redis lock and respects cooldown. Cooldown and a held lock finish the child workflow successfully with `Skipped` and are counted in `sources_skipped`; they are not activity failures and are not retried. A rate-limited, dead-tenant, or not-built collector child is skipped the same way on the profile run. It is not a failed run. Two profiles share the ingest cooldown.
5. After each child that stored jobs, and once more when every lane has finished, score jobs with `first_seen_at` inside `ScoreWindowDays` (10) before that run's `started_at`, and jobs whose `posted_at` is set and strictly older than the profile cutoff before that same clock (`PostedAtRejectDays` 30, or `PostedAtRejectDaysArchitecture` 90 when `domain` is `architecture`). A zero `posted_at` is not a reject by itself. The pass records every fired rule, including a stale `posted_at` as `CUT_POSTED_AT`, and picks the bucket only after that. Upsert `profile_matches` (`bucket`, `score`, `signals`, `run_id`, `updated_at`). `user_status` is not in the update set. The open run's `jobs_scored` and `sources_skipped` update as children finish. A failed score attempt does not delete existing matches.
6. Mark the run `SUCCEEDED` with those counters. `FAILED` only after Temporal retries. `GET /api/v1/profiles/{profile_id}/runs/latest` reads that row.

**Read / correct**

- `GET /api/v1/profiles` — file-name order. Item fields: `id`, `name`, `domain`, `sources`, `queries`.
- `GET /api/v1/profiles/{profile_id}/jobs` — `bucket` (`TOP`, `REVIEW`, or `REJECTED`; omitted means `TOP`), `user_status` (`NEW` or `HIDDEN`; omitted means `NEW`), `page` (≥1), `limit` (1–100). Sort `score DESC, first_seen_at DESC`. No query returns both statuses. Score 0 and below stay in the list. `bucket=PASSED` is `400`.
- `PATCH /api/v1/profiles/{profile_id}/jobs/{job_id}` body `{ "user_status": "NEW" | "HIDDEN" }`. `404` when this profile has no match.

**Retention:** hard-delete `jobs` where `last_seen_at` is older than 30 days. `JobRetentionWorkflow` Sunday 05:00 UTC (`jobhound-job-retention`), or `bin/retention run`. Matches cascade with the job.

### 2.4 Collectors (what “ingest” actually hits)

Every source implements `collectors.Collector` (`Name` + `Fetch`). Optional: `QueryFetcher`, `UnionQueryFetcher`. Bootstrap: `internal/collectors/bootstrap.MVPCollectors`. The query strings come from the profile file, not from a Go matrix. Profile **name** is not a board query.

| `Job.Source` | Site | Transport (fact) | Ingest children |
|--------------|------|------------------|-----------------|
| `europe_remotely` | euremotejobs.com | T2: WP `admin-ajax.php` HTML fragment + detail GET. Docker worker/agent send this source through `JOBHOUND_EUROPE_REMOTELY_PROXY` (`http://host.docker.internal:18080` → host `cmd/ipv6proxy`). Empty proxy dials directly (correct on a host that has IPv6). | one child per `queries` phrase |
| `working_nomads` | workingnomads.com | T2: Elasticsearch JSON `jobsapi/_search` | one child per `queries` phrase |
| `builtin` | builtin.com/jobs/remote | T2 parse; **T3 rod** for HTML by default (`browserfetch` + Chromium) | one child per `queries` phrase |
| `himalayas` | himalayas.app | T2: public JSON API (`/jobs/api`, `/jobs/api/search`) | one child per `queries` phrase |
| `remotify_europe` | remotifyeurope.com | T2: Next.js Flight POST (`Next-Action` discovered each run) + listing GET JSON-LD | one child per `queries` phrase |
| `we_work_remotely` | weworkremotely.com | T2: RSS only (`/remote-jobs.rss`); HTML is Cloudflare | catalog (`queries` ignored) |
| `wellfound` | wellfound.com | T2: HTML `/role/r/{slug}` + JobPosting JSON-LD; site mutex + 1s delay | one child per `wellfound_roles` slug |
| `vue_jobs` | vuejobs.com | T2: SSR `__NUXT_DATA__` remote catalog; skip PRO; `Job.URL` = `/jobs/{slug}` | catalog |
| `golang_cafe` | golang.cafe | **T3 rod** JSON API via shared `browserfetch` (plain HTTP = Vercel 429) | catalog |
| `bestjobs` | bestjobs.eu | T2: HTML keyword lists + `/api/proxy/v2/jobs` | one child per `queries` phrase (`-in-romania` and `-in-remote`) |
| `hipo` | hipo.ro | T2: HTML category pages | one child per `children.hipo` slug; the collector ignores the query and always loads `Arhitectura-Design-Proiectare` |
| `ejobs` | ejobs.ro | **T3 rod** (plain HTTP = Cloudflare 403) | one child per `queries` phrase (`/locuri-de-munca/{words}` and `/remote/{words}`) |
| `dezeen_jobs` | dezeenjobs.com | **T3 rod** (plain HTTP = Cloudflare 403) | one child per `children` category slug |
| `riba_jobs` | jobs.architecture.com | T2: HTML category pages | one child per `children` category slug |
| `landscape_institute` | jobs.landscapeinstitute.org | T2: HTML `/jobs/` (one page) | `children` slug `catalog`; the query is not sent |
| `archgee` | archgee.com | T2: HTML | one child per `children` slug (`architect`, `landscape-architect`, `remote`; other slugs fetch nothing) |
| `archinect` | archinect.com | T2: HTML remote region; site mutex + 5s delay | `children` slug `remote` only; any other query fetches nothing |
| `competitionline` | competitionline.com | T2: unfiltered `/api/jobs/` | `children` slug `catalog`; the query is not sent |
| `baunetz` | baunetz.de | T2: one `/stellenmarkt/` page | `children` slug `catalog` |
| `bdla` | bdla.de | T2: HTML job catalog | `children` slug `catalog` |
| `world_architects` | world-architects.com | **T3 rod** (plain HTTP = Cloudflare 403) | `children` slug `catalog`; the query is not sent |
| `consider_*` | one Consider host per fund | T1: `GET {host}/jobs` (CSRF) then `POST {host}/api-boards/search-jobs` | one catalog child (`Queries` set, lock segment `catalog`) |
| `getro_*` | api.getro.com | T1: `POST /api/v2/collections/{collection_id}/search/jobs` | one keyword child per `queries` phrase |

`andrew.yaml` queries: `vue` · `typescript` · `react` · `golang` · `node`. Wellfound slugs: `frontend-engineer` · `full-stack-engineer` · `backend-engineer`. `europe_remotely` is commented out of that file; the collector is still built. `lada.yaml` queries: `arhitect proiectant` · `peisagist` · `urbanist`. Its `children` slugs are the category lists in the file (`hipo`, `dezeen_jobs`, `riba_jobs`, `archgee`, and `catalog` for `landscape_institute`, `competitionline`, `baunetz`, `bdla`, `world_architects`, plus `archinect: remote`). `bestjobs` and `ejobs` are keyword sources on that file, so they fan `queries`.

Known source ids are the nine software aggregators, the twelve architecture boards above, plus these Consider and Getro ids. Consider: `consider_andreessen_horowitz` · `consider_lightspeed` · `consider_sequoia_capital` · `consider_gv` · `consider_bessemer_ventures` · `consider_balderton_capital` · `consider_notion_capital` · `consider_atlantic_labs` · `consider_battery_ventures` · `consider_panoramic_ventures` · `consider_ivp` · `consider_kleiner_perkins` · `consider_nea` · `consider_sequoia_capital_india` · `consider_union_square_ventures`. Getro: `getro_accel` · `getro_general_catalyst` · `getro_atomico` · `getro_hv_capital` · `getro_point_nine` · `getro_earlybird` · `getro_firstminute` · `getro_craft_ventures`. `djinni` and `dou_ua` are dropped. LinkedIn is **cancelled** (never shipped; no package, no session cookies, no `SessionProvider`). If `JOBHOUND_COLLECTOR_HIMALAYAS_DISABLED`, the worker omits `himalayas`. `golang_cafe` is omitted when the rod fetcher is nil. `ejobs`, `dezeen_jobs`, and `world_architects` stay in the map with a nil fetcher in that case; fetch then errors. A listed source with no collector in the worker map increments `sources_skipped`; the run still succeeds.

Debug HTTP (`cmd/agent`): `GET /health`, `POST /debug/collectors/{europe_remotely,working_nomads,himalayas,builtin,remotify_europe,we_work_remotely,wellfound,vue_jobs,golang_cafe}`, `POST /debug/collectors/consider`, `POST /debug/collectors/getro`. Architecture boards have no debug route.

### 2.5 What is *not* on the product path

- Stage 3, Claude / Anthropic, `internal/llm`, the global `user_profile` text, slots, the three-slot cap, and `ManualSlotRunWorkflow`.
- Client keyword defaults (`defaultStage2Exclude.ts`, `EUROPE_HIRING_OK`, a stage-2 rule builder). Phrases are not posted by the browser.
- A file named `architecture.yaml`. The architecture profile is `config/profiles/lada.yaml`, and its collectors are on the product path.
- Companies and ATS polling, a company-tag signal, `min_score`.
- Telegram / push, auth, users, payments, scheduled vacancy auto-refresh, GCP deploy.
- Collectors debug HTTP and `cmd/ipv6proxy` stay; they are not the product path.

### 2.6 Nuxt (`jobhound_frontend`)

Nuxt 4 under `app/`. Dev server listens on **:3002** (`NUXT_PUBLIC_API_BASE_URL` defaults to `http://localhost:3000`). Pages call `app/api/publicApi.ts`. `@pinia/nuxt` is installed; `app/store/index.ts` exports nothing. No keyword editor and no profile-text page. `apiFetch` sets `X-Correlation-ID`. `readApiJson` unwraps `{ ok: true, result }` when that wrapper is present and otherwise uses the JSON body as `T`.

| Route | Behavior |
|-------|----------|
| `/` | `getProfiles`. Each row shows `name` and links to `/profiles/{id}`. No create, no delete, no slot cap. The header’s only nav target is `/`. |
| `/profiles/{id}` | Latest run plus the job list (`PaginatedDataTable`, page size 50). Columns: row number, title, company, score, sources, location, first seen (local, minutes), posted (local, or a dash), hide or unhide. The run button calls `postProfileRun` with a new UUID and an empty body. While status is `RUNNING`, the page polls `getLatestProfileRun` and `getProfileJobs` every 2.5s. It shows status, `jobs_scored` (“Parsed N vacancies”), and `sources_skipped` when those fields are present. The default tab sends `bucket=TOP` and omits `user_status`, so it is `TOP` and `NEW`. Controls request `bucket=REVIEW` and `bucket=REJECTED`. One control requests `user_status=HIDDEN` and omits `bucket`, so the server default `TOP` still applies; it can patch a row back to `NEW`. Hide calls `patchProfileJob` with `HIDDEN`. Location prefers `openings` as links (country label, or `Link`); a Recruitee apply URL falls back to the listing or the careers index. With no opening link, countries and regions render as text. Empty countries, regions, and links render a dash. Source labels exist for the nine software aggregators; every other id, including Consider, Getro, and the architecture boards, renders the raw id. Failures render `error.message`, including an already-open run. `PROFILES.NO_RUN` on the latest-run call renders “No run yet”. Profile phrase blocks and per-job signals are not rendered. |

Client functions: `getProfiles`, `postProfileRun`, `getLatestProfileRun`, `getProfileJobs`, `patchProfileJob`. `postProfileRun` sends `Idempotency-Key` and no body. `getProfileJobs` sends `bucket`, `user_status`, `page`, and `limit` only when the caller set them.

## 3. Stack and versions

Sources: `go.mod` / `go.sum`, `Dockerfile`, `Dockerfile.migrate`, `docker-compose.yml`, `Makefile`.

| Layer | Fact |
|------|------|
| Language | Go **1.24.0** (`go.mod` `go 1.24.0`; Docker `golang:1.24-bookworm`) |
| HTTP | stdlib `net/http` ServeMux (method+path patterns). No Gin/Echo/Chi. |
| DB | PostgreSQL **16** + GORM **1.31.1** + `gorm.io/driver/postgres` **1.6.0**; pool via `pgx/v5` **5.6.0** |
| Migrations | `github.com/golang-migrate/migrate/v4` **v4.19.1**; SQL files in `migrations/` |
| Orchestration | Temporal SDK **v1.41.1**; Compose image `temporalio/auto-setup:1.29.1`, UI `temporalio/ui:2.48.1`. `go.temporal.io/api` **v1.62.2** is indirect. |
| Cache / ingest coord | Redis **7-alpine** + `github.com/redis/go-redis/v9` **v9.18.0** |
| HTML | `github.com/PuerkitoBio/goquery` **v1.10.3** |
| Headless | `github.com/go-rod/rod` **v0.116.2** (direct `require`; used by `collectors/browserfetch`). Image installs `chromium`. |
| Profiles | `gopkg.in/yaml.v3` **v3.0.1**. Directory from `JOBHOUND_PROFILES_DIR` (empty → `config/profiles`). |
| Scoring | In-process rules in `internal/scoring/impl`. No LLM SDK. |
| Logging | `github.com/rs/zerolog` **v1.34.0** |
| JSON Schema | `github.com/santhosh-tekuri/jsonschema/v6` **v6.0.2** (product API bodies) |
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
| `go.temporal.io/sdk` | v1.41.1 |
| `gopkg.in/yaml.v3` | v3.0.1 |
| `gorm.io/driver/postgres` | v1.6.0 |
| `gorm.io/driver/sqlite` | v1.6.0 |
| `gorm.io/gorm` | v1.31.1 |

`replace`: `github.com/docker/docker` → `v28.3.3+incompatible`; OpenTelemetry otel/metric/trace pinned to **v1.37.0** (Temporal transitive).

## 4. Folder structure

```
jobhound_core/
├── .cursor/                 # rules, feature templates, audit-feature-tasks skill
├── .github/workflows/       # ci.yml: gofmt, vet, lint, unit tests
├── cmd/
│   ├── agent/               # debug HTTP (composition only)
│   ├── api/                 # product HTTP (composition only)
│   ├── worker/              # Temporal worker (composition only)
│   ├── migrate/             # golang-migrate CLI
│   ├── retention/           # one-shot job hard-delete
│   └── ipv6proxy/           # host CONNECT proxy (IPv6-first); started by make docker-up, not Compose
├── config/profiles/         # andrew.yaml (software), lada.yaml (architecture)
├── data/                    # countries.json (ISO lookup for collectors)
├── docker/temporal/         # Temporal dynamic config for Compose
├── docs/                    # REPO_SNAPSHOT.md + feature slices (02-companies-and-ats)
├── internal/
│   ├── config/              # JOBHOUND_* names + typed loaders only
│   ├── domain/              # shared kernel: schema.Job, identity utils
│   ├── collectors/          # Collector contract, sources, schema/, debughttp, bootstrap, browserfetch
│   ├── ingest/              # Redis coordinator, watermarks, ingest workflows
│   ├── jobs/                # job persistence + retention workflow
│   ├── profiles/            # YAML file store (List / Get)
│   ├── scoring/             # phrase score, profile_runs / profile_matches
│   ├── publicapi/           # HTTP handlers, schema, utils, embedded JSON Schema
│   └── platform/            # pgsql, logging, temporalopts (not a product module)
├── migrations/              # 000001_initial_schema.{up,down}.sql
├── tests/                   # reserved; empty (integration tests are colocated + build tag)
├── Dockerfile               # agent + worker + api; copies data/ and config/profiles
├── Dockerfile.migrate
├── docker-compose.yml
├── Makefile
├── .editorconfig
├── .env.example             # JOBHOUND_* names; secrets empty
├── .golangci.yml
└── go.mod / go.sum
```

Principle for `internal/<module>/`: `contract.go` (interfaces + sentinel errors) → `impl/` (use cases, **no** Temporal SDK) → `storage/` (GORM) → `schema/` (module data model) → `handlers/` (HTTP) → `workflows/` + `activities/` (Temporal; required if the module uses Temporal) → `utils/`.

Modules under `internal/`: `collectors`, `config`, `domain`, `ingest`, `jobs`, `platform`, `profiles`, `publicapi`, `scoring`.

## 5. Configuration

| File | What is configured |
|------|----------------|
| `go.mod` | module path, Go 1.24.0, direct deps, `replace` pins |
| `Makefile` | `build` five app binaries to `bin/` (agent, worker, api, migrate, retention); `bin/ipv6proxy` is a separate target used by `start-ipv6-proxy`; `run` / `run-debug` / `run-worker`; `test` / `test-integration`; `fmt` / `vet` / `lint` (`golangci-lint run`) / `tidy`; migrate; `docker-up` starts the host proxy, then Compose `build --no-cache` + `up -d --force-recreate --pull always`; `docker-down` stops the proxy |
| `docker-compose.yml` | postgres 16, migrate one-shot, redis 7, temporal + UI + temporal-ready, agent :3001, worker, api :3000. Worker and agent get `JOBHOUND_EUROPE_REMOTELY_PROXY=http://host.docker.internal:18080`. `api` and `worker` get `JOBHOUND_PROFILES_DIR=/app/config/profiles`. The proxy process itself is not a Compose service |
| `Dockerfile` | multi-stage; CGO_ENABLED=0; debian-slim + chromium + `data/` + `config/profiles` |
| `Dockerfile.migrate` | migrate binary + `migrations/` |
| `.dockerignore` | `.git`, `bin/`, `.cursor`, `*.md`, `.env*` |
| `.gitignore` | `.env`, `.env.*`, `/bin/`, `vendor/`, sqlite, `cookies.txt` |
| `.cursor/rules/specify-rules.mdc` | always-applied stack + module layout, Canonical Enum, schema-first HTTP, Temporal separation |
| `.cursor/templates/` | concept / schemas-contracts / tasks for new `docs/<feature_slug>/` slices |
| `.cursor/skills/audit-feature-tasks/` | audit `*-tasks.md` against code |
| CI | `.github/workflows/ci.yml` on push and pull request: `gofmt -l` empty, `make vet`, `make lint`, `make test`. No integration tag. No deploy |
| `.env.example` | every key `internal/config` loads; secrets empty; default in a comment. README links it and does not paste the table |
| `commitlint` / OpenAPI / CSS | none |
| `.editorconfig` | `utf-8`, `lf`, final newline, trim trailing space. `*.go` tabs. `*.{yml,yaml,json,md}` indent 2. `*.md` does not trim trailing space |
| LICENSE | **none** |

Non-standard: env names live only in `internal/config`. Feature packages must not `os.Getenv` shared knobs. Temporal **connection** is config; workflow **code** is per-module `workflows/`.

## 6. Typing

- **Language:** Go. No TypeScript / OpenAPI codegen in this repo. The sibling UI is TypeScript.
- **Shared kernel:** `internal/domain/schema.Job`. Identity: `domain/utils.CompanyKey`, `StableJobID`, `NormalizeListingURL`, `AssignStableID`. No GORM and no Temporal SDK under `internal/domain/**`.
- **Module data model:** each feature’s `schema/` holds structs, enums, payloads, exported errors. HTTP request/response types for the product API live in `internal/publicapi/schema/`.
- **Canonical Enum Pattern:** exported string enums have `String` / `Equals` / `Pointer` / `FromValue` / `ValuesT` / `FromStringT`. Used by `profiles/schema.Domain`, `scoring/schema` `Bucket` / `UserStatus` / `RunStatus` / `SignalCode`, and `APIErrorCode` / `APIErrorClass`.
- **HTTP input:** embed JSON Schema (`//go:embed json_schema/*.schema.json`) → `publicapi/utils.ValidateJSONInstance` → decode with `DisallowUnknownFields()`. Schema on this path: `patch_user_status` (`NEW` | `HIDDEN`). POST runs has no body schema (`RequireEmptyBody`). Forbidden: `map[string]json.RawMessage` existence checks in handlers.
- **Error contract (product API):** registry in `internal/publicapi/schema` (`APIErrorCode`, `APIErrorSpec`, `Lookup`). Writer is `WriteError` in `internal/publicapi/utils`. Envelope `{ "error": { "code": "DOMAIN.CODE", "message": "..." } }`. No top-level `limit`. No `ok`, `class`, `fields`, `meta`, or `correlation_id` on the wire. `500` message is always `Internal server error.` (`INTERNAL.UNEXPECTED`); the cause is logged, not copied into the body. Handlers do not pick status or message. `impl/` does not import the registry. Debug HTTP (`cmd/agent`) stays plain-text `http.Error`.
- **Success bodies:** typed structs in `publicapi/schema` (profile list, run, job page, patch). `GET /api/v1/health` → `{ "status": "ok" }`. Success is not wrapped in `{ "ok": true, "result" }`. The Nuxt client accepts that wrapper when present and otherwise uses the JSON body as `T`.

Wire `code` values (registry order; generated copy `internal/publicapi/schema/generated/api-error-registry.json`): `HTTP.INVALID_JSON`, `HTTP.VALIDATION_FAILED`, `HTTP.INVALID_QUERY`, `HTTP.IDEMPOTENCY_KEY_REQUIRED`, `HTTP.INVALID_IDEMPOTENCY_KEY`, `HTTP.IDEMPOTENCY_KEY_CONFLICT`, `PROFILES.NOT_FOUND`, `PROFILES.INVALID_DEFINITION`, `PROFILES.RUN_ALREADY_RUNNING`, `PROFILES.NO_RUN`, `PROFILES.JOB_NOT_IN_SCOPE`, `PROFILES.INVALID_USER_STATUS`, `HTTP.METHOD_NOT_ALLOWED`, `INTERNAL.UNEXPECTED`. Wire struct: `APIErrorBody` / `APIErrorDetail` in `publicapi/schema/errors.go`.

Named score constants in `internal/scoring/schema`: `ScoreWindowDays` = 10, `PostedAtRejectDays` = 30, `PostedAtRejectDaysArchitecture` = 90. Scoring picks 90 when `profile.domain` is `architecture`. Ingest picks 90 when the source id is an architecture board. Weights, the body cap, `buckets.top`, and the currency rate are profile YAML, not Go constants. The 10-day window is not an env key. The 12-hour architecture cooldown is a Go constant (`43200s`), not an env key.

## 7. Code conventions

Facts from `.cursor/rules/specify-rules.mdc`:

- **Naming:** Go defaults (`PascalCase` exported, `camelCase` unexported). Package directories match module names (`publicapi`, `debughttp`, `browserfetch`).
- **Module layers:** contract → impl → storage; Temporal mapping in `workflows/` (not `impl/`). Handlers: `handler.go` + `registerRoutes()` + one file per route; **no** package-level helpers in a route file (`publicapi` helpers live in `publicapi/utils/`). Product failures go through `WriteError`; the handler does not pass a status, a code string, or `err.Error()` into the body.
- **Lint:** `make lint` (golangci-lint v2) plus `make fmt` / `make vet`. `depguard` denies `go.temporal.io/sdk` under `impl/` and GORM under `internal/domain`.
- **Composition:** `cmd/*` is thin — open DB, dial Temporal, construct repos/services, `ListenAndServe` / `worker.Run`.
- **State:** PostgreSQL is system of record; profile YAML is the search definition; Redis is **only** ingest lock/cooldown (no search-result cache); Temporal holds workflow execution state.
- **Logging:** `platform/logging.NewRoot`; `RequestIDMiddleware` (`X-Request-ID`); field keys `handler` / `method` / `workflow` / `service` / `request_id` / `workflow_id` / `run_id` / `source_id`. Default format **console**; `JOBHOUND_LOG_FORMAT=json` for GCP-style stdout.
- **Comments:** names over banners; document invariants, not every method.
- **Tests:** colocated `*_test.go`, same package. Test **handlers / impl / storage**. Do not add trivia tests for thin utils/enums. Integration: `//go:build integration`.

## 8. Authentication and payments

### Authentication — absent (MVP)

- No JWT, sessions, OAuth, or API keys on `/api/v1`.
- CORS allowlist from `JOBHOUND_API_CORS_ORIGINS` (default `http://localhost:5173,http://localhost:3002`). Allowed headers: `Content-Type, Idempotency-Key, X-Correlation-ID`.

### Payments — absent

No Stripe/Paddle/billing modules.

## 9. Database

- **DBMS:** PostgreSQL 16 (Compose: user/db `jobhound`). Temporal uses a **second** Postgres (`temporal` user) — not the app schema.
- **ORM:** GORM `1.31.1` (`internal/platform/pgsql.Open`, `GormGetter`). Models in `jobs/storage` and `scoring/storage`. App schema is owned by **SQL migrations**, not GORM AutoMigrate.
- **Migrate:** `github.com/golang-migrate/migrate/v4` file source. Single revision `000001_initial_schema`.
- **Tables:** `jobs`, `profile_runs`, `profile_matches`, `ingest_watermarks`. Profiles are files, not rows.
- **Job identity:** `jobs.id` TEXT is the dedup key (`StableJobID`). `sources` JSONB. `location` JSONB `{type, regions, countries, timezone, raw}`. `openings` JSONB `[{country, url, apply_url}]`, default `[]`. `company_key`, `company_website`, `first_seen_at`, `last_seen_at`. Indexes on `first_seen_at`, `last_seen_at`, `company_key`.
- **Runs:** `profile_runs.status` ∈ `RUNNING` | `SUCCEEDED` | `FAILED`. Unique `idempotency_key`. Counters `jobs_scored`, `sources_skipped`.
- **Matches:** composite PK `(profile_id, job_id)`. `bucket` ∈ `TOP` | `REVIEW` | `REJECTED`. `user_status` ∈ `NEW` | `HIDDEN`, default `NEW`. `signals` JSONB. Index `(profile_id, bucket, user_status, score DESC)`. Scoring upsert omits `user_status`.
- **Watermarks:** PK `(source_id)` only.
- **Seeds:** none. `andrew.yaml` and `lada.yaml` are the phrase sources.
- **Retention:** hard-delete `jobs` where `last_seen_at` is older than `JOBHOUND_JOB_RETENTION_DAYS` (default 30). `profile_matches.job_id` is `ON DELETE CASCADE`.
- **SQLite:** test-only GORM driver for storage unit tests — not a runtime target.

## 10. Build, tests, CI

### Makefile

| Target | Purpose |
|--------|---------|
| `build` | `bin/agent`, `bin/worker`, `bin/api`, `bin/migrate`, `bin/retention` (`bin/ipv6proxy` is not part of `build`) |
| `run` | build + run agent (loads `.env` if present) |
| `run-debug` | agent with `JOBHOUND_DEBUG_HTTP_ADDR=127.0.0.1:3001` |
| `run-worker` | Temporal worker |
| `test` | `go test ./...` (excludes `integration` tag) |
| `test-integration` | `go test -tags=integration ./...` |
| `fmt` / `vet` / `lint` / `tidy` | `go fmt`, `go vet`, `golangci-lint run`, mod tidy |
| `migrate-up` / `down` / `version` | `bin/migrate` |
| `start-ipv6-proxy` / `stop-ipv6-proxy` | Build `bin/ipv6proxy` if needed; listen on `127.0.0.1:18080` (`-detach`, pid in `bin/ipv6proxy.pid`, log `bin/ipv6proxy.log`). Health check is `GET /health` |
| `docker-up` / `docker-down` / `docker-ps` / `docker-logs` / `docker-migrate` | Compose. `docker-up` depends on `start-ipv6-proxy`; `docker-down` runs `stop-ipv6-proxy` first |

### Tests

- **Runner:** `go test`. **55** `*_test.go` files.
- **Unit (default):** handlers (`httptest`) for the five profile routes, `profiles/impl` YAML load (including `lada.yaml` and `children`), `scoring/impl` phrase table, `scoring/storage` and `jobs/storage` (often sqlite), collector parse fixtures (software and architecture boards), ingest coordinator (miniredis).
- **Integration (`//go:build integration`):** `internal/platform/pgsql/migrations_integration_test.go`; `internal/ingest/coordinator_integration_test.go`; `internal/collectors/browserfetch/rod_integration_test.go`. Need env / Compose (`JOBHOUND_DATABASE_URL`, Chromium as applicable).
- **`tests/`:** empty directory (constitution allows optional `tests/integration/`).
- **Frontend unit tests:** live in `jobhound_frontend` (`pnpm test:unit`), not in this repo.

### CI

`.github/workflows/ci.yml` on push and pull request (`ubuntu-latest`, Go from `go.mod`): `gofmt -l` must be empty, then `make vet`, `make lint` (golangci-lint **v2.13.2**), `make test`. No `integration` tag. No deploy job.

## 11. Deploy

- **Production target (constitution):** GCP runtime + secrets. **No** Cloud Run workflow, Dockerfile deploy target, or Artifact Registry config is in this repo yet.
- **Local / current runtime:** Docker Compose on a laptop (`make docker-up`). Published ports: **5432** Postgres, **6379** Redis, **7233** Temporal gRPC, **8088** Temporal UI, **3000** API, **3001** agent debug HTTP. Host proxy **127.0.0.1:18080** is not a Compose port.
- **Images:** `jobhound-core:local` (agent/worker/api); migrate uses `Dockerfile.migrate`. Chromium in the app image for Built In, Golang Cafe, and the architecture rod boards `ejobs`, `dezeen_jobs`, and `world_architects` (`JOBHOUND_BROWSER_NO_SANDBOX=1` as root). `config/profiles` is in the app image.
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
| `JOBHOUND_LOG_LEVEL` | default `info` |
| `JOBHOUND_LOG_FORMAT` | default `console` (`json` for structured) |
| `JOBHOUND_DATA_DIR` | empty → `data` cwd-relative (`countries.json`) |
| `JOBHOUND_DEBUG_HTTP_ADDR` | agent debug listen; empty → agent exits |
| `JOBHOUND_JOB_RETENTION_SCHEDULE_UPSERT` | default true (worker creates weekly schedule) |
| `JOBHOUND_JOB_RETENTION_DAYS` | default 30; non-positive rejected at load. Cutoff is `last_seen_at`. |
| `JOBHOUND_PROFILES_DIR` | empty → `config/profiles` |
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
| `JOBHOUND_EUROPE_REMOTELY_PROXY` | HTTP CONNECT proxy for euremotejobs.com only. Empty dials directly. Compose sets `http://host.docker.internal:18080` |

`.env.example`: present. Lists every key in the table above; secrets empty; default in a comment. `README.md` links that file and does not paste a second list. Compose interpolates `JOBHOUND_API_CORS_ORIGINS` from host env.

## 12. AI setup

| Artifact | Present? | Contents (list of rules/structure only, no paraphrased “meaning”) |
|----------|----------|------------------------------------------------------------------|
| `.cursorrules` | no | — |
| `AGENTS.md` / `CLAUDE.md` | no | — |
| `.cursor/rules/specify-rules.mdc` | yes | Always-applied: stack, `cmd/` vs `internal/`, collectors `schema/`, debughttp layout, Canonical Enum, publicapi JSON Schema, lint gate (`make lint` + fmt/vet; CI), product error registry (`WriteError`, handlers do not pick status or message), anti-patterns, testing, make targets |
| `.cursor/templates/` | yes | `concept.md`, `schemas-contracts.md`, `tasks.md`, README |
| `.cursor/skills/audit-feature-tasks/` | yes | audit `*-tasks.md` vs code |
| `docs/` | `REPO_SNAPSHOT.md`, `02-companies-and-ats/` | — |

## 13. Documentation

| File | Contents | Currency (factual mismatches) |
|------|----------|-------------------------------|
| `README.md` | One paragraph + pointer to `.env.example` + Docker two-liner + host `cmd/ipv6proxy` sentence + migrate hint | Does not list binaries, API routes, or Compose ports (those are in `docker-compose.yml` comments). Env key table lives in `.env.example`, not a second copy in the README. Opening line still says “pipeline”. |
| `.cursor/rules/specify-rules.mdc` | Engineering constitution | Live sections list `cmd/api`, `internal/profiles`, `internal/scoring`, and `internal/ingest`. Older recent-changes lines still name the removed LLM and pipeline layout as history. |
| `LICENSE` | — | **Missing** |

## 14. Scaffold vs business logic

### Reusable skeleton (backend template)

- Go module layout: `cmd/*` composition + `internal/<feature>/{contract,impl,storage,schema,handlers,workflows,utils}`
- `internal/config` as the only `os.Getenv` surface
- `internal/platform/pgsql` + `golang-migrate` SQL
- `internal/platform/logging` (zerolog, request id, field names)
- Temporal: per-module `workflows/` + `RegisterWorkflow` in `New`/`Register`; no SDK in `impl/`
- Product HTTP: `net/http` mux, CORS, embedded JSON Schema + typed decode, error registry + `WriteError`
- golangci-lint v2 (`.golangci.yml`) and GitHub Actions (`gofmt`, vet, lint, unit tests)
- Docker Compose Postgres + Temporal + worker + API
- Feature docs: `.cursor/templates/` → `docs/<feature_slug>/`; audit via `.cursor/skills/audit-feature-tasks`

### Project-specific (remove when turning into a template)

- Job-board collectors (the nine software aggregators, the twelve architecture boards, Consider `consider_*`, Getro `getro_*`), `browserfetch` / Chromium, and host `cmd/ipv6proxy` for Europe Remotely from Docker
- Profile YAML (`config/profiles/andrew.yaml`, `config/profiles/lada.yaml`, including `children`) and gated phrase scoring (`gate_role` / `gate_hard` / `gate_location` / `signals_plus` / `penalties_soft`, buckets `TOP` | `REVIEW` | `REJECTED`)
- Architecture-board cooldown of 12 hours and the 90-day `posted_at` cutoff for `domain: architecture` and for architecture source ids
- Content-key job id and `profile_runs` / `profile_matches`
- Redis ingest lock/cooldown keys `ingest:lock|cooldown:{source_id}:{query_segment}`
- `data/countries.json`
- Job retention 30 days on `last_seen_at` + Temporal schedule `jobhound-job-retention`
- Debug HTTP `/debug/collectors/*`
- Compose service names `jobhound-*`, default CORS localhost frontend ports
- Nuxt profile list and profile page in `jobhound_frontend`

## 15. Debt and unfinished spots

- **LinkedIn:** cancelled, not planned. No package, no cookies file, no `SessionProvider`.
- **`lada.yaml`:** shipped (`domain: architecture`). There is no file named `architecture.yaml`.
- **Worker without Redis URL:** ingest workflows register with nil coordinator; ingest activities cannot take locks (fail closed / incomplete ingest).
- **No LICENSE**, no OpenAPI. CI and `.env.example` exist (`.github/workflows/ci.yml`, repo-baseline).
- **README** points at `.env.example` and stays thinner than Compose (no routes, no ports).
- **Scheduled vacancy auto-refresh:** explicit backlog. Retention cron is unrelated (delete old `jobs` rows).
- **Auth / multi-user / Telegram push:** out of MVP.
- **`min_score`:** not a query parameter. Lists keep score 0 and below. The default list is `TOP` and `NEW`, sorted by score.
- **`tests/`:** empty placeholder.
- **GCP deploy:** named as production target; not implemented in-repo.
- **Himalayas nil:** if `JOBHOUND_COLLECTOR_HIMALAYAS_DISABLED`, debug route returns 500; worker omits the source from the collector map. A profile that lists it counts a skipped source.
- **Golang Cafe nil:** constructed only when rod `HTMLDocumentFetcher` exists (`JOBHOUND_BROWSER_ENABLED` + Built In browser path). Without it, the ingest map has no `golang_cafe`.
- **Architecture browser nil:** `ejobs`, `dezeen_jobs`, and `world_architects` stay in the worker map when rod is off. Fetch then returns `browser fetcher is nil`, and that error is not a skip.

---

### Candidates for a shared layer

| Element | File / folder | Likely same thing exists in my other repos |
|---------|---------------|---------------------------------------------|
| Feature module layout (contract/impl/storage/schema/handlers/workflows) | `internal/*`, `.cursor/rules/specify-rules.mdc` | yes (`backend-api-core` `src/internal`; omg-bo style) |
| Env-only in `config` | `internal/config` | yes (Saynest `src/config`) |
| Schema-first JSON body validation | `publicapi/handlers/json_schema` + jsonschema/v6 | yes (Saynest Ajv/Zod; omg-ap embed) |
| Temporal per-feature `workflows/` | `ingest/workflows`, `jobs/workflows` | don’t know (this repo’s Go pattern) |
| Zerolog + request id | `internal/platform/logging` | no (Saynest Winston); same *idea* |
| Feature docs templates + audit skill | `.cursor/templates/`, `.cursor/skills/audit-feature-tasks/` | yes (Monterra `docs/<slug>/`) |
| Docker Compose Postgres + migrate | `docker-compose.yml` | don’t know |
| GORM + golang-migrate SQL | `platform/pgsql`, `migrations/` | no (Saynest is Mongoose) |
| Collector / rod | `internal/collectors` | no |
| API error envelope `{error:{code,message}}` | `publicapi/schema` registry + `errors.go`; writer `publicapi/utils.WriteError` | related but **different** from Saynest `{ok, result\|error}` registry. Codes are `DOMAIN.CODE`. |
| CORS + Idempotency-Key | `publicapi/utils/cors.go` | no |
