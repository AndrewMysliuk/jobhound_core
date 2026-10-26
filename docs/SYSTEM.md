# SYSTEM

How JobHound core works (product model, scoring, collectors, debt). Handwritten.
Stack / versions / make / env: [`STACK.md`](./STACK.md) (`make docs-stack`).

## 1. Purpose

Repository remote `AndrewMysliuk/jobhound_core` (`go.mod` module: `github.com/andrewmysliuk/jobhound_core`; README: “jobhound_core”) is the JobHound backend: ingest job listings from public boards, persist one row per vacancy in PostgreSQL, score that pool with in-process phrase rules (and optional Anthropic vacancy facts), and expose a JSON HTTP API. Role: backend / worker / debug collector process. The product UI is the sibling Nuxt app `jobhound_frontend` (section 2.6).

MVP is **single-tenant** (no auth). The unit of search is a **profile** loaded from YAML, not a slot. Two profiles are shipped: `software` (`domain: software`) and `architecture` (`domain: architecture`). Product HTTP is API-first (`cmd/api`). Collectors debug HTTP (`cmd/agent`) is development-only. Search phrases and the `facts` scoring block exist only in `config/profiles/*.yaml`. The browser does not send queries or scoring phrases. Vacancy enrich (`internal/enrich`) is optional: empty `JOBHOUND_ANTHROPIC_API_KEY` disables batches and sets every job’s `facts_status` to `off`.

## 2. How the system works

### 2.1 Product model

A profile file (`config/profiles/<id>.yaml`) is the only copy of:

1. **`sources`** — which collectors to poll. The allow-list is the ten software aggregators, the twelve architecture boards, and the Consider and Getro source ids in section 2.4. A file lists only the ids it polls.
2. **`queries`** — stage-1 search strings only. They do not add score. `frontend` and `full-stack` are not query strings.
3. **`children`** — optional map of source id → slugs. A listed source gets one ingest child per slug and does not copy `queries`. The slug `catalog` is an empty query. A key that is not in `sources` makes the file invalid. Not returned by `GET /api/v1/profiles`.
4. **`wellfound_roles`** — Wellfound `/role/r/{slug}` segments. They address that one site. They are not search words for other boards and they do not affect the score. `architecture.yaml` leaves this list empty.
5. **`ats_polling`** — when `true`, `ProfileRunWorkflow` runs the companies/ATS stage (discover → resolve → poll) after ingest and before the final score. When `false` or omitted, the profile stays on aggregators only. Default is `false` (a missing key disables ATS, it does not enable it). Not returned by `GET /api/v1/profiles`.
6. **`gate_role`**, **`gate_role_allow`**, **`gate_hard`**, **`gate_location`**, **`signals_plus`**, **`penalties_soft`**, **`buckets.top`** — the phrase score. `gate_location` holds `allow_countries`, `allow_regions`, and `reject_timezones`. `signals_plus` holds `title`, `text` (`terms`, `max_total`), `salary` (`min_eur_year`, `min_usd_year`, `usd_per_eur`, `bonus_if_above`, `penalty_if_below`), and `freshness` (`days_3`, `days_7`, `unknown_posted_at`). `penalties_soft` is a map of named groups (`weight`, `min_occurrences`, `check_negation`, `terms`). `buckets` holds `top`. Not returned by `GET /api/v1/profiles`.
7. **`facts`** — optional block that turns a ready enrich verdict into `LLM` / `GATE_HARD` (`facts.*`) / `CONFLICT` / soft `PENALTY` signals. Fields: `residence_regions`, `residence_countries` (optional; with `hiring_scope.narrow_penalty` soft-hits restricted 1–2 country scopes that omit them), `office` (allow lists; cut skipped when `work_mode` is remote and `hiring_scope` is worldwide or Europe-compatible), `hiring_scope` (`scored`, optional `delta` once for worldwide/EUROPE/EMEA/WORLDWIDE, optional `narrow_penalty`), `work_authorization` (`gates`; when a gated value fires, compares verdict `countries` against `residence_countries` / `residence_regions` — any match skips the cut; empty `countries` does not reject), `timezone` (`allow`), `compensation` (`allow_currencies`), `work_mode` (`gates` e.g. hybrid/office → REVIEW), `employment_load`, `engagement`. Each scored field has `scored` and optional `delta`. A missing block or `scored: false` skips cuts for that field (`LLM` with delta 0 still emits when scored is false and the answer is known). Not returned by `GET /api/v1/profiles`.

Each file sets `id` (must match the filename stem), `name`, and `domain` (`software` or `architecture`). The shipped files are `software.yaml` (`ats_polling: true`) and `architecture.yaml` (`ats_polling: false`). Profiles are read on every `List`/`Get` (no cache). They are not created, updated, or deleted over HTTP. The only user-writable field is `user_status` on a match (`NEW` or `HIDDEN`).

**Stage 1** is ingest. It does not score. A profile run expands `sources` into child `IngestSourceWorkflow`s. Keyword sources, including `getro_*` and any source with no `children` entry, get one child per `queries` phrase. Wellfound gets one child per `wellfound_roles` slug. `we_work_remotely`, `vue_jobs`, and `golang_cafe` get one catalog child (`queries` ignored). Each `consider_*` source gets one catalog child (`Queries` set, lock segment `catalog`). A source listed in `children` gets one child per slug and does not copy `queries`. Redis keys are `ingest:lock:{source_id}:{query_segment}` and `ingest:cooldown:{source_id}:{query_segment}`; an empty query uses the segment `catalog`. Architecture source ids use a fixed 12-hour cooldown. Other sources use `JOBHOUND_INGEST_COOLDOWN_TTL_SEC` (default 3600). Ingest drops a row whose `posted_at` is known and strictly older than 30 days. A zero `posted_at` is stored. ATS board upsert (`UpsertBoardJobs`) applies the same 30-day rule: a known `posted_at` older than 30 days is not written or refreshed.

**Stage 2** scores stored rows whose `first_seen_at` is inside 10 days before `profile_runs.started_at`, and rows whose `posted_at` is already older than 30 days (`PostedAtRejectDays`, every profile). Every phrase step always runs. When a ready verdict exists for the current input hash, fact signals append after the phrase pass. Nothing returns on the first hit. Score is the sum of signal deltas. The bucket is chosen at the end: any non-fact gate (`GATE_ROLE`, phrase `GATE_HARD`, `CUT_LOCATION`, `CUT_TIMEZONE`, `CUT_POSTED_AT`) is `REJECTED`; a fact cut (`GATE_HARD` with rule `facts.*`) alone is `REVIEW` (so model misses stay visible); otherwise `score >= buckets.top` is `TOP`, except `domain: architecture` also requires at least one `PLUS` with rule prefix `signals_plus.title`; otherwise `REVIEW`, including 0 and negative. Phrase match is case-insensitive and word-bounded (`java` does not hit `javascript`). Country and region codes match the whole code, case-insensitive. Diacritics are not folded for phrase match. Enrich input hashes normalize title/description/location/salary with Unicode NFC (`golang.org/x/text`).

`gate_role_allow` is title only: when the list is non-empty and the title matches none of the phrases, one `GATE_ROLE` with rule `gate_role_allow`, delta 0; an empty list leaves the allow gate off. `gate_role` is title only: one `GATE_ROLE` per phrase, delta 0, no negation. Level phrases `senior` / `sr.` / `lead` / `principal` that fall inside a title span like `junior or senior` / `junior/senior` (must include junior or jr) do not fire. `gate_hard` is title, description, and `location.raw`, negation-checked: one `GATE_HARD` per phrase that still hits. `gate_location`: non-remote jobs run a fixed Europe office tier first (country outside the Europe set → `CUT_LOCATION`; all-Europe and none RO and no relocation wording → one `PENALTY` −2), then emit one `CUT_LOCATION` per code that misses both allow lists (another allowed code does not cancel it). Remote jobs skip the Europe tier and use intersection: reject only when every listed country/region misses the allow lists (US+DE stays when DE is allowed). Empty allow lists, or empty job countries and regions (including remote), are not a reject. Collector `location.timezone` with UTC offsets `-8` or `-7` emits `CUT_TIMEZONE` (`gate_location.pacific_timezone`). Timezone phrases match title, description, `location.raw`, and `location.timezone`, negation-checked, one `CUT_TIMEZONE` each. A known `posted_at` strictly older than 30 days emits `CUT_POSTED_AT`; a zero `posted_at` is not a reject. `signals_plus` walks title terms, then body terms up to `text.max_total` (a term after the cap still fires with delta 0), then salary (parsed upper bound against `min_eur_year` or `min_usd_year` with `usd_per_eur` for cross-currency; hit applies `bonus_if_above` or `penalty_if_below`; no salary and an unparseable string add nothing), then freshness on `posted_at` only (within 3 days `days_3`, else within 7 days `days_7`; zero `posted_at` applies `unknown_posted_at` when that weight is non-zero). `penalties_soft` emits one `PENALTY` per group: a title hit fires once even under `min_occurrences`; body hits fire at the threshold (`0` and `1` both mean one hit); negation runs only when the group sets `check_negation`. The negation window is the 50 characters before the phrase, clamped at the previous `.`, `!`, or `?`; `not`, `no`, `n't`, or a `non-` prefix drops the hit. `queries` do not score. Weights, the body cap, `buckets.top`, and `usd_per_eur` are profile YAML. There is no `min_score` parameter and no `PASSED` bucket.

**Vacancy enrich (optional):** after the phrase score pass succeeds, `ProfileRunWorkflow` fire-and-forgets `EnrichBatchWorkflow` for job ids missing a row for the configured model and `PromptVersion` (1). Candidates are **open** vacancies (`closed_at IS NULL`) with `first_seen_at` inside `JOBHOUND_ENRICH_WINDOW_DAYS` (default 30, wider than scoring’s 10-day window), known `posted_at` not older than `PostedAtMaxAgeDays` (30; unknown `posted_at` stays eligible), and description length at least `MinDescriptionChars` (default 800; `JOBHOUND_ENRICH_MIN_DESCRIPTION_CHARS`). Empty missing list skips the child. A failed `ListMissing` or child start does not fail the profile run (`ParentClosePolicy` abandon). The child runs in drain mode (workflow id `enrich-{promptVersion}-{model}-drain`): it pages `ListMissing` at most `MaxJobsPerSubmit` (1000) ids per Temporal activity so payloads stay under the ~2 MiB blob limit, submits each page to Anthropic Message Batches (stdlib `net/http`, no Anthropic SDK), polls every 30s per page, stores coerced `ModelReply` JSON into `job_verdicts`, then `RescoreJobs` re-evaluates existing matches. Pending is the absence of a row; there is no status column. `facts_status` on the job page is `off` when enrich is disabled on the API, when the vacancy is filtered out (closed, outside the enrich window, stale `posted_at`, or short description) and has no ready verdict, else `pending` or `ready` for the current text hash. A ready verdict stays `ready` even if the job later closes or ages out of the window. Fact fields: `office`, `hiring_scope`, `work_authorization`, `timezone`, `compensation`, `work_mode`, `employment_load`, `engagement`. `employment_load` and `engagement` never cut. Fact cuts are `GATE_HARD` with rule `facts.<field>`. Known answers also emit `LLM` (delta from YAML when `scored`, else 0). `CONFLICT` fires when a mapped collector phrase disagrees with an allow-side model answer. A signal is `{code, rule, phrase, value, delta, occurrences}` with codes `GATE_ROLE`, `GATE_HARD`, `CUT_LOCATION`, `CUT_TIMEZONE`, `CUT_POSTED_AT`, `PLUS`, `PENALTY`, `LLM`, `CONFLICT`. Every list returns its scores, including 0 and below. The default bucket is `TOP`.

One vacancy is one `jobs` row. `id` is `company_key + U+001E + normalized title` (trim, collapse whitespace, lowercase); location is not part of `id`. Re-ingest appends an `openings` element `{country, url, apply_url}` deduped by normalized country + url + apply_url, unions `location.countries` and `location.regions`, and sets `location.raw` to the sorted country labels joined by `", "`. It keeps `first_seen_at`, moves `last_seen_at`, fills an empty `company_website`, and leaves description and salary from the first row. Parent `url` / `apply_url` are the first opening after that sort. A recompute / rescore overwrites score, bucket, and signals and keeps `user_status`.

There is no stage 3, no global profile text, no client-side keyword default, and no Anthropic call on the request path of `cmd/api` (API only reads `job_verdicts` for `facts_status` when enrich is enabled).

### 2.2 Processes

One Docker Compose stack (or local `make` binaries) runs several processes:

| Process | Binary | Role |
|---------|--------|------|
| `api` | `cmd/api` | Product JSON API on **:3000** (`/api/v1/...`). Composition only. Does not fetch boards. Loads the profile file store, the scoring service, and the companies read API (list + job `closed` filter). When enrich is enabled, wires `facts_status` reads from `job_verdicts` (no Anthropic HTTP client). `POST .../runs` starts `ProfileRunWorkflow` through Temporal. |
| `worker` | `cmd/worker` | Temporal worker on task queue `jobhound`. Registers ingest, job retention, `ProfileRunWorkflow`, companies resolve/poll workflows, and `EnrichBatchWorkflow` (or a no-op `ListMissing` stub when enrich is off). Owns collectors + Redis + ATS board fetchers (Greenhouse / Lever / Ashby) + companies blocklist. When enrich is on, owns the Message Batches client and post-store rescore. Reads `JOBHOUND_PROFILES_DIR` and `JOBHOUND_COMPANIES_DIR`. |
| `agent` | `cmd/agent` | Dev-only collector debug HTTP on **:3001** when `JOBHOUND_DEBUG_HTTP_ADDR` is set. Exits if that address is empty. |
| `migrate` | `cmd/migrate` | `golang-migrate` CLI (`up` / `down` / `version` / `force`). |
| `retention` | `cmd/retention` | One-shot hard-delete of jobs whose `last_seen_at` is older than **30 days** (`JOBHOUND_JOB_RETENTION_DAYS`; same cutoff as the Temporal schedule). |
| `companies-import` | `cmd/companies-import` | One-shot seed of `companies` from `config/companies/companies_template.xlsx` (verify Greenhouse / Lever / Ashby slugs live). `make companies-import`. Not a Compose service. |
| `eval` | `cmd/eval` | Offline enrich eval: compares `testdata/enrich/gold.jsonl` and `predictions.jsonl` (optional path args). `make eval` → `go run ./cmd/eval`. Not a Compose service. |
| `ipv6proxy` | `cmd/ipv6proxy` | Host-only HTTP CONNECT proxy on **127.0.0.1:18080**. Not a Compose service. `make docker-up` starts it (`start-ipv6-proxy`); `make docker-down` stops it. Dials targets IPv6-first. `GET /health` → `ok`. |

Infra beside the app: **Postgres 16** (app data), **Redis 7** (ingest lock + cooldown), **Temporal** `auto-setup:1.29.1` on its **own** Postgres, Temporal UI **:8088**. The IPv6 proxy runs on the **host**, outside Compose: the worker container has no IPv6 route, and euremotejobs.com answers IPv4 with a SiteGround challenge.

```
browser / jobhound_frontend
        │  HTTP JSON  :3000
        ▼
     cmd/api  ──ExecuteWorkflow──►  Temporal  ◄── poll ──  cmd/worker
        │                              │                      │
        │                              │                      ├─ collectors (HTTP / goquery / rod)
        │                              │                      ├─ companies discover / resolve / ATS poll (when ats_polling)
        │                              │                      ├─ Redis lock/cooldown
        │                              │                      ├─ phrase score (+ fact signals when verdict ready)
        │                              │                      └─ EnrichBatchWorkflow → Anthropic Message Batches (optional)
        └── GORM / Postgres ◄──────────┴──────────────────────┘
              ▲
              ├── config/profiles/*.yaml (api and worker; JOBHOUND_PROFILES_DIR)
              └── config/companies/ (blocklist + seed xlsx; JOBHOUND_COMPANIES_DIR)
```

The arrow from `cmd/api` to Temporal is the run start in section 2.3. `cmd/api` executes `ProfileRunWorkflow`; `cmd/worker` polls that workflow, runs ingest, optional companies/ATS stages, the phrase score, and may start `EnrichBatchWorkflow`.

`cmd/agent` debug HTTP is a **side door**: it hits collectors directly and returns job JSON. It does not write profile runs, matches, or verdicts.

`Dockerfile` copies `config/profiles` and `config/companies` next to the binaries. Compose sets `JOBHOUND_PROFILES_DIR=/app/config/profiles` and `JOBHOUND_COMPANIES_DIR=/app/config/companies` on `api` and `worker`, and passes `JOBHOUND_ANTHROPIC_API_KEY` / `JOBHOUND_ENRICH_MODEL` from the host `.env` (empty key disables enrich).

### 2.3 End-to-end happy path

**Start a run**

1. `POST /api/v1/profiles/{profile_id}/runs` with header `Idempotency-Key` (non-nil UUID) and an empty body. A JSON body is `400`.
2. The handler writes a `profile_runs` row (`RUNNING`) and starts `ProfileRunWorkflow`, workflow id `profile-run-{profile_id}`, reuse `ALLOW_DUPLICATE` after close. Input is `ProfileRunInput{ProfileID, RunID}` only. Activities re-read the YAML.
3. A second POST while that execution is open is **409** `PROFILES.RUN_ALREADY_RUNNING`. The same idempotency key returns the same run. The same key with a different profile is **409** `HTTP.IDEMPOTENCY_KEY_CONFLICT`.
4. Ingest children run at the same time across sources, and in order inside one source (one lane per source id). Shared Chromium for `builtin` / `golang_cafe` / architecture rod boards serializes inside `browserfetch`, not by holding back other boards' child workflows. Workflow id is `ingest-{source}-{segment}` (`catalog` when the query is empty; spaces in the segment become hyphens). Each child takes a Redis lock and respects cooldown. Cooldown and a held lock finish the child workflow successfully with `Skipped` and are counted in `sources_skipped`; they are not activity failures and are not retried. Any other failed ingest child (HTTP 5xx, parse error, rate limit, dead tenant, missing collector, …) is also counted in `sources_skipped` and does **not** fail the profile run — remaining sources keep going. Two profiles share the ingest cooldown.
5. Companies/ATS stages (discover → resolve children → poll) run **only when** the profile has `ats_polling: true`. When false or omitted, the stage is skipped and the run goes straight to the final score. Order is fixed: companies before score, so ATS text is in `jobs` when scoring runs. Discover upserts company rows from ingested jobs (blocklist + merge-or-insert, backfills `jobs.company_key_ref`). Resolve children (`resolve-{company_key}`) set `ats` + `ats_slug` primarily from apply URL hosts (`APPLY_URL`); recognised non-pollable ATS → `UNSUPPORTED_ATS`; unknown → `UNRESOLVED` with `next_resolve_at`. Poll (`poll-companies-{run_id}`) fetches due `RESOLVED` + `ACTIVE` boards (Greenhouse / Lever / Ashby), upserts vacancies, and on a complete poll closes board-linked rows no longer listed (`closed_by=ATS`). Aggregator re-ingest never clears an ATS close. A failed resolve/poll child does not fail the profile run. Counters on the run: `companies_*`, `polls_*`, `jobs_upserted_ats`, `jobs_closed`.
6. After ingest (and companies stages when enabled), score jobs with `first_seen_at` inside `ScoreWindowDays` (10) before that run's `started_at`, and jobs whose `posted_at` is set and strictly older than `PostedAtRejectDays` (30) before that same clock. A zero `posted_at` is not a reject by itself. The pass loads ready verdicts when enrich storage is wired, records every fired rule (including stale `posted_at` as `CUT_POSTED_AT` and fact signals when a verdict is ready), and picks the bucket only after that. Upsert `profile_matches` (`bucket`, `score`, `signals`, `run_id`, `updated_at`). `user_status` is not in the update set. The open run's `jobs_scored` and `sources_skipped` update as children finish. A failed score attempt does not delete existing matches.
7. Fire-and-forget enrich: probe `ListMissing` (limit 1) → optional child `EnrichBatchWorkflow` in drain mode (workflow id `enrich-{promptVersion}-{model}-drain`, reuse `ALLOW_DUPLICATE_FAILED_ONLY`, parent close abandon). Then mark the run `SUCCEEDED` with the score counters. Enrich finish and rescore happen after the run row is already `SUCCEEDED`. `FAILED` only after Temporal retries on the profile-run path. `GET /api/v1/profiles/{profile_id}/runs/latest` reads that row.

**Read / correct**

- `GET /api/v1/profiles` — file-name order. Item fields: `id`, `name`, `domain`, `sources`, `queries`.
- `GET /api/v1/profiles/{profile_id}/jobs` — `bucket` (`TOP`, `REVIEW`, or `REJECTED`; omitted means `TOP`), `user_status` (`NEW` or `HIDDEN`; omitted means `NEW`), `closed` (bool; default `false` excludes `closed_at IS NOT NULL`), `page` (≥1), `limit` (1–100). Sort `score DESC, first_seen_at DESC`. No query returns both statuses. Score 0 and below stay in the list. `bucket=PASSED` is `400`. Each job includes `closed_at`, `facts_status` (`off` | `pending` | `ready`) and `signals` (may include `LLM` / `CONFLICT` / `facts.*` gates after enrich).
- `PATCH /api/v1/profiles/{profile_id}/jobs/{job_id}` body `{ "user_status": "NEW" | "HIDDEN" }`. `404` when this profile has no match. Response includes `facts_status`.
- `GET /api/v1/companies` — read-only company registry. Query: `resolution_status`, `poll_state`, `ats`, `needs_review`, `page`, `limit` (default 50, max 100). Item fields omit `board_etag`.

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
| `dou_ua` | jobs.dou.ua | T2: RSS list (`/vacancies/feeds/?search={query}&remote`) + HTML detail; plain `net/http` | one child per `queries` phrase (empty query → no fetch) |
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

`software.yaml` queries: `vue` · `typescript` · `golang`. Wellfound slugs: `frontend-engineer` · `full-stack-engineer` · `backend-engineer`. `architecture.yaml` queries: `arhitect proiectant` · `peisagist` · `urbanist`. Its `children` slugs are the category lists in the file (`hipo`, `dezeen_jobs`, `riba_jobs`, `archgee`, and `catalog` for `landscape_institute`, `competitionline`, `baunetz`, `bdla`, `world_architects`, plus `archinect: remote`). `bestjobs` and `ejobs` are keyword sources on that file, so they fan `queries`.

Known source ids are the ten software aggregators, the twelve architecture boards above, plus these Consider and Getro ids. Consider: `consider_andreessen_horowitz` · `consider_lightspeed` · `consider_sequoia_capital` · `consider_gv` · `consider_bessemer_ventures` · `consider_balderton_capital` · `consider_notion_capital` · `consider_atlantic_labs` · `consider_battery_ventures` · `consider_panoramic_ventures` · `consider_ivp` · `consider_kleiner_perkins` · `consider_nea` · `consider_sequoia_capital_india` · `consider_union_square_ventures`. Getro: `getro_accel` · `getro_general_catalyst` · `getro_atomico` · `getro_hv_capital` · `getro_point_nine` · `getro_earlybird` · `getro_firstminute` · `getro_craft_ventures`. LinkedIn is **cancelled** (never shipped; no package, no session cookies, no `SessionProvider`). If `JOBHOUND_COLLECTOR_HIMALAYAS_DISABLED`, the worker omits `himalayas`. `golang_cafe` is omitted when the rod fetcher is nil. `ejobs`, `dezeen_jobs`, and `world_architects` stay in the map with a nil fetcher in that case; fetch then errors. A listed source with no collector in the worker map increments `sources_skipped`; the run still succeeds.

Debug HTTP (`cmd/agent`): `GET /health`, `POST /debug/collectors/{europe_remotely,working_nomads,himalayas,builtin,remotify_europe,we_work_remotely,wellfound,vue_jobs,golang_cafe,dou_ua}`, `POST /debug/collectors/consider`, `POST /debug/collectors/getro`. Architecture boards have no debug route.

### 2.5 What is *not* on the product path

- Stage 3 as a separate Anthropic vacancy-summary stage, the removed `internal/llm` package, the global `user_profile` text, slots, the three-slot cap, and `ManualSlotRunWorkflow`. Vacancy enrich via Message Batches (`internal/enrich`) is on the product path when `JOBHOUND_ANTHROPIC_API_KEY` is set; it is not a per-request LLM call from the browser or `cmd/api`.
- Client keyword defaults (`defaultStage2Exclude.ts`, `EUROPE_HIRING_OK`, a stage-2 rule builder). Phrases are not posted by the browser.
- Feature docs `docs/01-enrich/`, `docs/01-dou-ua/`, and `docs/02-companies-and-ats/` (removed after ship). A company-tag score signal, fetchers beyond Greenhouse / Lever / Ashby, `min_score`.
- Telegram / push, auth, users, payments, scheduled vacancy auto-refresh, GCP deploy.
- Collectors debug HTTP, `cmd/eval`, `cmd/companies-import`, and `cmd/ipv6proxy` stay; they are not the product HTTP path.

### 2.6 Nuxt (`jobhound_frontend`)

Nuxt 4 under `app/`. Dev server listens on **:3002** (`NUXT_PUBLIC_API_BASE_URL` defaults to `http://localhost:3000`). Pages call `app/api/publicApi.ts`. `@pinia/nuxt` is installed; `app/store/index.ts` exports nothing. No keyword editor and no profile-text page. `apiFetch` sets `X-Correlation-ID`. `readApiJson` unwraps `{ ok: true, result }` when that wrapper is present and otherwise uses the JSON body as `T`.

| Route | Behavior |
|-------|----------|
| `/` | `getProfiles`. Each row shows `name` and links to `/profiles/{id}`. No create, no delete, no slot cap. The header’s only nav target is `/`. |
| `/profiles/{id}` | Latest run plus the job list (`PaginatedDataTable`, page size 50). Columns: row number, title, company, score, sources, location, first seen (local, minutes), posted (local, or a dash), hide or unhide. The run button calls `postProfileRun` with a new UUID and an empty body. While status is `RUNNING`, the page polls `getLatestProfileRun` and `getProfileJobs` every 2.5s. It shows status, `jobs_scored` (“Parsed N vacancies”), and `sources_skipped` when those fields are present. The default tab sends `bucket=TOP` and omits `user_status`, so it is `TOP` and `NEW`. Controls request `bucket=REVIEW` and `bucket=REJECTED`. One control requests `user_status=HIDDEN` and omits `bucket`, so the server default `TOP` still applies; it can patch a row back to `NEW`. Hide calls `patchProfileJob` with `HIDDEN`. Location prefers `openings` as links (country label, or `Link`); a Recruitee apply URL falls back to the listing or the careers index. With no opening link, countries and regions render as text. Empty countries, regions, and links render a dash. Source labels exist for nine software aggregators (not `dou_ua`); every other id, including `dou_ua`, Consider, Getro, and the architecture boards, renders the raw id. Failures render `error.message`, including an already-open run. `PROFILES.NO_RUN` on the latest-run call renders “No run yet”. Types accept `facts_status` (`off` \| `pending` \| `ready`) and signal codes including `LLM` / `CONFLICT`. Profile phrase blocks, `facts_status`, and per-job signals are not rendered in the table UI yet. |

Client functions: `getProfiles`, `postProfileRun`, `getLatestProfileRun`, `getProfileJobs`, `patchProfileJob`. `postProfileRun` sends `Idempotency-Key` and no body. `getProfileJobs` sends `bucket`, `user_status`, `page`, and `limit` only when the caller set them.

## 3. Stack and versions

See [`STACK.md`](./STACK.md) §3 (`make docs-stack`).

## 4. Folder structure

```
jobhound_core/
├── .cursor/                 # rules, feature templates, audit-feature-tasks + calibrate-profile skills
├── .github/workflows/       # ci.yml: gofmt, vet, lint, unit tests
├── cmd/
│   ├── agent/               # debug HTTP (composition only)
│   ├── api/                 # product HTTP (composition only)
│   ├── worker/              # Temporal worker (composition only)
│   ├── migrate/             # golang-migrate CLI
│   ├── retention/           # one-shot job hard-delete
│   ├── companies-import/    # seed companies from xlsx
│   ├── eval/                # offline enrich gold vs predictions
│   └── ipv6proxy/           # host CONNECT proxy (IPv6-first); started by make docker-up, not Compose
├── config/profiles/         # software.yaml (ats_polling true), architecture.yaml (false); both include facts
├── config/companies/        # blocklist.yaml + companies_template.xlsx seed
├── data/                    # countries.json (ISO lookup); data/local/ holds gitignored Postgres dumps (+ .gitkeep)
├── docker/temporal/         # Temporal dynamic config for Compose
├── docs/                    # SYSTEM.md + STACK.md (feature slices removed after ship)
├── scripts/                 # gen_stack_docs.go (make docs-stack)
├── internal/
│   ├── config/              # JOBHOUND_* names + typed loaders only (includes enrich.go, companies.go)
│   ├── domain/              # shared kernel: schema.Job, identity utils
│   ├── collectors/          # Collector contract, sources, schema/, debughttp, bootstrap, browserfetch, ats/
│   ├── companies/           # company registry + ATS poll (impl, storage, workflows)
│   ├── ingest/              # Redis coordinator, watermarks, ingest workflows
│   ├── jobs/                # job persistence + retention workflow (+ board close)
│   ├── profiles/            # YAML file store (List / Get)
│   ├── scoring/             # phrase + fact score, profile_runs / profile_matches
│   ├── enrich/              # vacancy facts: Message Batches, job_verdicts, EnrichBatchWorkflow
│   ├── publicapi/           # HTTP handlers, schema, utils, embedded JSON Schema
│   └── platform/            # pgsql, logging, temporalopts (not a product module)
├── migrations/              # 000001_initial_schema.{up,down}.sql (includes job_verdicts + companies/ATS)
├── testdata/enrich/         # gold.jsonl + predictions.jsonl for make eval
├── tests/                   # reserved; empty (integration tests are colocated + build tag)
├── Dockerfile               # agent + worker + api; copies data/, config/profiles, config/companies
├── Dockerfile.migrate
├── docker-compose.yml
├── Makefile
├── .editorconfig
├── .env.example             # JOBHOUND_* names; secrets empty
├── .golangci.yml
└── go.mod / go.sum
```

Principle for `internal/<module>/`: `contract.go` (interfaces + sentinel errors) → `impl/` (use cases, **no** Temporal SDK) → `storage/` (GORM) → `schema/` (module data model) → `handlers/` (HTTP) → `workflows/` + `activities/` (Temporal; required if the module uses Temporal) → `utils/`.

Modules under `internal/`: `collectors`, `companies`, `config`, `domain`, `enrich`, `ingest`, `jobs`, `platform`, `profiles`, `publicapi`, `scoring`.

## 5. Configuration

See [`STACK.md`](./STACK.md) §5 (`make docs-stack`).

## 6. Typing

- **Language:** Go. No TypeScript / OpenAPI codegen in this repo. The sibling UI is TypeScript.
- **Shared kernel:** `internal/domain/schema.Job`. Identity: `domain/utils.CompanyKey`, `StableJobID`, `NormalizeListingURL`, `AssignStableID`. No GORM and no Temporal SDK under `internal/domain/**`.
- **Module data model:** each feature’s `schema/` holds structs, enums, payloads, exported errors. HTTP request/response types for the product API live in `internal/publicapi/schema/`.
- **Canonical Enum Pattern:** exported string enums have `String` / `Equals` / `Pointer` / `FromValue` / `ValuesT` / `FromStringT`. Used by `profiles/schema.Domain`, `scoring/schema` `Bucket` / `UserStatus` / `RunStatus` / `SignalCode`, `enrich/schema.FactsStatus` (`off` | `pending` | `ready`) and field enums, `companies/schema` `ATS` / `ResolutionStatus` / `ResolutionMethod` / `PollState` / `PollOutcome`, `jobs/schema.ClosedBy` (`ATS` | `AGGREGATOR`), and `APIErrorCode` / `APIErrorClass`.
- **HTTP input:** embed JSON Schema (`//go:embed json_schema/*.schema.json`) → `publicapi/utils.ValidateJSONInstance` → decode with `DisallowUnknownFields()`. Schema on this path: `patch_user_status` (`NEW` | `HIDDEN`). POST runs has no body schema (`RequireEmptyBody`). `GET /companies` and job-list filters are query enums (no JSON Schema body). Forbidden: `map[string]json.RawMessage` existence checks in handlers. Enrich model replies use a separate embedded schema under `enrich/schema/json_schema/model_reply.schema.json` (not a public HTTP body).
- **Error contract (product API):** registry in `internal/publicapi/schema` (`APIErrorCode`, `APIErrorSpec`, `Lookup`). Writer is `WriteError` in `internal/publicapi/utils`. Envelope `{ "error": { "code": "DOMAIN.CODE", "message": "..." } }`. No top-level `limit`. No `ok`, `class`, `fields`, `meta`, or `correlation_id` on the wire. `500` message is always `Internal server error.` (`INTERNAL.UNEXPECTED`); the cause is logged, not copied into the body. Handlers do not pick status or message. `impl/` does not import the registry. Debug HTTP (`cmd/agent`) stays plain-text `http.Error`.
- **Success bodies:** typed structs in `publicapi/schema` (profile list, run, job page, patch, company list). Job items include `closed_at`, `facts_status` and `signals` (with optional `value`). `GET /api/v1/health` → `{ "status": "ok" }`. Success is not wrapped in `{ "ok": true, "result" }`. The Nuxt client accepts that wrapper when present and otherwise uses the JSON body as `T`.

Wire `code` values (registry order; generated copy `internal/publicapi/schema/generated/api-error-registry.json`): `HTTP.INVALID_JSON`, `HTTP.VALIDATION_FAILED`, `HTTP.INVALID_QUERY`, `HTTP.IDEMPOTENCY_KEY_REQUIRED`, `HTTP.INVALID_IDEMPOTENCY_KEY`, `HTTP.IDEMPOTENCY_KEY_CONFLICT`, `PROFILES.NOT_FOUND`, `PROFILES.INVALID_DEFINITION`, `PROFILES.RUN_ALREADY_RUNNING`, `PROFILES.NO_RUN`, `PROFILES.JOB_NOT_IN_SCOPE`, `PROFILES.INVALID_USER_STATUS`, `HTTP.METHOD_NOT_ALLOWED`, `INTERNAL.UNEXPECTED`. Wire struct: `APIErrorBody` / `APIErrorDetail` in `publicapi/schema/errors.go`.

Named score constants in `internal/scoring/schema`: `ScoreWindowDays` = 10, `PostedAtRejectDays` = 30 (one threshold for every profile and every source — scoring, ingest, and board upsert). Enrich `PromptVersion` = 1 and `PostedAtMaxAgeDays` = 30 in `enrich/schema`. Phrase rules (`gate_role`, `gate_role_allow` → `GATE_ROLE` / rule `gate_role_allow`, `gate_hard`, `gate_location`, `signals_plus`, `penalties_soft`), weights, the body cap, `buckets.top`, `usd_per_eur`, and the `facts` block are profile YAML, not Go constants. The 10-day window is not an env key. The 12-hour architecture cooldown is a Go constant (`43200s`), not an env key.

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
- **ORM:** GORM `1.31.1` (`internal/platform/pgsql.Open`, `GormGetter`). Models in `jobs/storage`, `scoring/storage`, `enrich/storage`, and `companies/storage`. App schema is owned by **SQL migrations**, not GORM AutoMigrate.
- **Migrate:** `github.com/golang-migrate/migrate/v4` file source. Single revision `000001_initial_schema` (job_verdicts and companies/ATS folded into the initial schema while local-only; there is no separate `000002`).
- **Tables:** `companies`, `jobs`, `profile_runs`, `profile_matches`, `ingest_watermarks`, `job_verdicts`. Profiles are files, not rows. Company seed lives in `config/companies/companies_template.xlsx` (`make companies-import`), not SQL.
- **Companies:** `companies.key` TEXT PK (immutable normalised name). `aliases` TEXT[], `website`, `ats` / `ats_slug` / `board_url` / `board_etag`, `resolution_status` ∈ `PENDING` | `RESOLVED` | `UNRESOLVED` | `UNSUPPORTED_ATS`, `resolution_method`, `poll_state` ∈ `ACTIVE` | `INACTIVE`, `next_resolve_at`, poll streak counters, `needs_review`. Indexes for due pollable, website, aliases GIN, resolve queue.
- **Job identity:** `jobs.id` TEXT is the dedup key (`StableJobID`). `sources` TEXT[]. `location` JSONB `{type, regions, countries, timezone, raw}`. `openings` JSONB `[{country, url, apply_url}]`, default `[]`. `company_key`, `company_website`, optional `company_key_ref` → `companies` (`ON DELETE SET NULL`), `board_seen_at` (board poll saw the row), `closed_at` / `closed_by` (`ATS` | `AGGREGATOR`), `first_seen_at`, `last_seen_at`. Indexes on `first_seen_at`, `last_seen_at`, `company_key`, partial open `closed_at`. Board upsert (`UpsertBoardJobs`) skips a known `posted_at` older than 30 days (zero `posted_at` is kept). Aggregator upsert never clears `closed_at` when `closed_by=ATS`.
- **Runs:** `profile_runs.status` ∈ `RUNNING` | `SUCCEEDED` | `FAILED`. Unique `idempotency_key`. Counters `jobs_scored`, `sources_skipped`, plus `companies_discovered` / `merged` / `blocked` / `resolved` / `unsupported` / `polled`, `polls_not_modified` / `partial`, `poll_failures`, `jobs_upserted_ats`, `jobs_closed`.
- **Matches:** composite PK `(profile_id, job_id)`. `bucket` ∈ `TOP` | `REVIEW` | `REJECTED`. `user_status` ∈ `NEW` | `HIDDEN`, default `NEW`. `signals` JSONB. Index `(profile_id, bucket, user_status, score DESC)`. Scoring upsert omits `user_status`.
- **Verdicts:** unique `(job_id, prompt_version, model, input_hash)`. `facts` JSONB (`ModelReply`). Pending is no row. `job_id` references `jobs(id)` `ON DELETE CASCADE`.
- **Watermarks:** PK `(source_id)` only.
- **Seeds:** none. `software.yaml` and `architecture.yaml` are the phrase and facts sources.
- **Retention:** hard-delete `jobs` where `last_seen_at` is older than `JOBHOUND_JOB_RETENTION_DAYS` (default 30). `profile_matches.job_id` and `job_verdicts.job_id` are `ON DELETE CASCADE`.
- **SQLite:** test-only GORM driver for storage unit tests — not a runtime target.

## 10. Build, tests, CI

See [`STACK.md`](./STACK.md) §10 (`make docs-stack`).

## 11. Deploy

See [`STACK.md`](./STACK.md) §11 (`make docs-stack`).

## 12. AI setup

| Artifact | Present? | Contents (list of rules/structure only, no paraphrased “meaning”) |
|----------|----------|------------------------------------------------------------------|
| `.cursorrules` | no | — |
| `AGENTS.md` / `CLAUDE.md` | no | — |
| `.cursor/rules/specify-rules.mdc` | yes | Always-applied: stack, `cmd/` vs `internal/`, collectors `schema/`, debughttp layout, Canonical Enum, publicapi JSON Schema, lint gate (`make lint` + fmt/vet; CI), product error registry (`WriteError`, handlers do not pick status or message), anti-patterns, testing, make targets. Live “Active Technologies” line still says “No LLM” even though optional enrich is shipped |
| `.cursor/templates/` | yes | `concept.md`, `schemas-contracts.md`, `tasks.md`, README |
| `.cursor/skills/audit-feature-tasks/` | yes | audit `*-tasks.md` vs code |
| `.cursor/skills/calibrate-profile/` | yes | calibrate profile YAML/scorer from pasted TOP vacancies |
| `docs/` | `SYSTEM.md`, `STACK.md` | `01-enrich/`, `01-dou-ua/`, and `02-companies-and-ats/` removed after ship |

## 13. Documentation

| File | Contents | Currency (factual mismatches) |
|------|----------|-------------------------------|
| `README.md` | One paragraph + pointer to `.env.example` + Docker two-liner + host `cmd/ipv6proxy` sentence + migrate hint | Does not list binaries, API routes, or Compose ports (those are in `docker-compose.yml` comments). Env key table lives in `.env.example`, not a second copy in the README. Opening line still says “pipeline”. |
| `.cursor/rules/specify-rules.mdc` | Engineering constitution | Live sections list `cmd/api`, `internal/profiles`, `internal/scoring`, and `internal/ingest`. Does not yet name `internal/enrich` / Message Batches in Active Technologies. Older recent-changes lines still name the removed LLM and pipeline layout as history. |
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

- Job-board collectors (the ten software aggregators, the twelve architecture boards, Consider `consider_*`, Getro `getro_*`), `browserfetch` / Chromium, and host `cmd/ipv6proxy` for Europe Remotely from Docker
- Profile YAML (`config/profiles/software.yaml`, `config/profiles/architecture.yaml`, including `children` and `facts`) and gated phrase + fact scoring (`gate_role` / `gate_role_allow` / `gate_hard` / `gate_location` / `signals_plus` / `penalties_soft` / `facts.*`, buckets `TOP` | `REVIEW` | `REJECTED`)
- Vacancy enrich (`internal/enrich`, Anthropic Message Batches, `job_verdicts`, `EnrichBatchWorkflow`, `cmd/eval` + `testdata/enrich`)
- Companies / ATS (`internal/companies`, `collectors/ats/{greenhouse,lever,ashby}`, `ats_polling` on profiles, `cmd/companies-import`, `config/companies/`)
- Architecture-board cooldown of 12 hours
- Content-key job id and `profile_runs` / `profile_matches`
- Redis ingest lock/cooldown keys `ingest:lock|cooldown:{source_id}:{query_segment}`
- `data/countries.json`
- Job retention 30 days on `last_seen_at` + Temporal schedule `jobhound-job-retention`
- Debug HTTP `/debug/collectors/*`
- Compose service names `jobhound-*`, default CORS localhost frontend ports
- Nuxt profile list and profile page in `jobhound_frontend`

## 15. Debt and unfinished spots

- **LinkedIn:** cancelled, not planned. No package, no cookies file, no `SessionProvider`.
- **Enrich off by default:** empty Anthropic key disables batches; API reports `facts_status=off`. Phrase score still runs. Worker registers a no-op `ListMissing` so the parent can always call it.
- **Frontend enrich UI:** DTO types include `facts_status` and `LLM` / `CONFLICT` signals; the profile jobs table does not render them yet.
- **Companies UI:** `GET /api/v1/companies` is read-only inspection; the Nuxt app has no companies page yet. Closed jobs are filtered server-side (`closed` query); the UI does not surface `closed_at`.
- **ATS backlog:** no fourth fetcher (Workday / Teamtailor / … stay `UNSUPPORTED_ATS`), no company-tag score signal, no guess+verify resolve from name alone.
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
- **specify-rules Active Technologies:** still says “No LLM”; enrich is optional Anthropic batches, not the removed stage-3 `internal/llm`.

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
| Vacancy enrich / Message Batches | `internal/enrich` | no |
| API error envelope `{error:{code,message}}` | `publicapi/schema` registry + `errors.go`; writer `publicapi/utils.WriteError` | related but **different** from Saynest `{ok, result\|error}` registry. Codes are `DOMAIN.CODE`. |
| CORS + Idempotency-Key | `publicapi/utils/cors.go` | no |
