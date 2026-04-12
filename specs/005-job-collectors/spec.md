# Feature: Job collectors

**Feature Branch**: `005-job-collectors`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-12  
**Status**: Implemented

## Goal

Per-source **`Collector`** implementations **fetch** listings and **normalize** to **`domain.Job`**. Stack is **tiered**: `net/http` + **goquery** and/or **`encoding/json`** first; **go-rod** + optional session only when a source requires it (constitution). Timeouts and structured errors — no silent junk. **No HTTP retries** on collector requests (one attempt per logical request; failure surfaces as error).

## Product alignment (MVP)

**Source of truth**: [`specs/000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — search slots, stage-1 broad ingest, reset rules, and multi-user **reservations** in schema.

- **This epic** owns **HTTP fetch + normalization per board** (`internal/collectors.Collector` and optional **`SlotSearchFetcher`** — see **`contracts/collector.md`**). It does **not** own **slot** lifecycle, **bound sources per slot**, **upsert**, **watermarks / delta refresh**, or **Redis ingest coordination** — those are **`006`** (and API shapes in **`008` / `009`**). The **slot keyword string** for stage-1 search is chosen by the client (**`009`** **`name`**) and passed into ingest as **`SlotSearchQuery`**; collectors implement **`FetchWithSlotSearch`** where the board supports it.
- **Orchestration** may run **one collector per bound source in parallel** for a slot’s stage-1 run; **failure of one source does not cancel others** unless a higher-level workflow defines otherwise — see **`contracts/collector.md`**.
- **`Job.UserID`** is **not** filled from site HTML/API by MVP collectors; orchestration/persistence may set it when writing **slot-scoped** rows (see **`contracts/domain-mapping-mvp.md`**).

## Spec artifacts (this directory)

**Markdown only** under `specs/005-job-collectors/`. **Per-source wire notes** (endpoints, DOM/JSON shapes) live in **`resources/`**. Reference HTTP/JSON/HTML samples live in fenced blocks in **`contracts/test-fixtures.md`**, not as separate `.json` / `.html` files here.

Canonical **MVP normalization and error policy** (description text, `Remote`, `Position`, dates, strict vs soft failures): **`contracts/domain-mapping-mvp.md`**.

## Layout (implementation)

- **`internal/collectors/`** — one package (or subtree) per source.
- **`internal/collectors/utils`** — shared HTTP, `data/countries.json` matching, dates, URL normalization.
- **`internal/pipeline`** — stage logic only; **no** site-specific parsing there. **`internal/ingest`** wires **`005`** collectors.

Details: **`contracts/collector.md`**.

## Pagination (MVP)

Listing UIs use buttons such as **“Load more”** / **“Show more jobs”**; the collector **does not** drive a browser. It repeats the same **HTTP** calls the UI uses (**`admin-ajax.php`** + `has_more` for Europe Remotely; **`jobsapi/_search`** with `from` / `size` for Working Nomads; **`xhr-load`** + `last` for DOU.ua; **public JSON** `jobs/api` + `jobs/api/search` for Himalayas — no RSC; **Djinni:** `GET` `/jobs/?all_keywords=…&search_type=full-text&page=` — ~**15** rows per page; **Built In:** `GET` `/jobs/remote` with `country` / `search` / `page` — up to **20** rows per page, **two** pages max per country — see **`resources/builtin.md`**). See **`resources/europe-remotely.md`**, **`resources/working-nomads.md`**, **`resources/dou.md`**, **`resources/himalayas.md`**, **`resources/djinni.md`**, and **`resources/builtin.md`**.

## Sources

Canonical list in **`contracts/sources-inventory.md`**.

- **MVP:** Europe Remotely, Working Nomads, DOU.ua, Himalayas (**`resources/himalayas.md`**), Djinni (**`resources/djinni.md`**), Built In (**`resources/builtin.md`**).
- **Planned:** LinkedIn, …; **rollout order** is in **`contracts/sources-inventory.md`** § Planned implementation order.

**Remote OK** is out of scope (inventory).

## Fetch stance (MVP, per inventory facts)

- **Europe Remotely:** `POST` `admin-ajax.php` JSON (`has_more` + HTML fragment) + job detail **`GET`** — **T2**. A **captured request body** from browser DevTools should be recorded in **`resources/europe-remotely.md`** when available (`action`, filters, pagination fields).
- **Working Nomads:** `POST` `jobsapi/_search` JSON — **T2**; HTML shell not required for core fields.
- **DOU.ua:** `GET` listing + `POST` `xhr-load` JSON (`html` / `last` / `num`) + detail **`GET`** — **T2**; cookie jar for CSRF.
- **Himalayas:** `GET` **`/jobs/api`** and **`/jobs/api/search`** — **T2**; JSON only (see **`resources/himalayas.md`**; `internal/collectors/himalayas`).
- **Djinni:** `GET` listing + `GET` each job detail — **T2**; **`application/ld+json`** **`JobPosting`** on listing (optional **array**) and detail; **inter-request delay** per **`resources/djinni.md`** / **`contracts/environment.md`** (`internal/collectors/djinni`).
- **Built In:** `GET` remote listing + `GET` each detail — **T2**; JSON-LD **`ItemList`** + **`JobPosting`**; **search-required** (no HTTP when slot empty); **inter-request delay** per **`resources/builtin.md`** / **`contracts/environment.md`**.
- **Rod:** only when a source cannot be served without JS/session.

## Follow-ups (operational)

- **Built In — Cloudflare / anti-bot:** Live `GET` to **`builtin.com`** from worker or datacenter egress can return **HTTP 403** with an HTML interstitial (**`Just a moment...`**, Cloudflare challenge) instead of listing/detail content — plain **`net/http`** is often blocked. **Follow-up:** choose mitigation (e.g. browser-tier **T3** fetch, egress/proxy strategy, partnership or official feed if any, per-territory graceful handling, or product decision to pause Built In ingest until resolved) and update **`resources/builtin.md`** + **`contracts/environment.md`** when a path is locked.

## Normalized fields

MVP mapping and planned **`SalaryRaw` / `Tags` / `Position` / `TimezoneOffsets`**: **`contracts/domain-mapping-mvp.md`**.  
Persistence extension for **`jobs`**: **`contracts/jobs-table-extension.md`**.

## Temporary debug HTTP (before `009`)

To manually verify collectors without the public API spec, a **local-only** debug server lives under **`cmd/agent`**: flag **`-debug-http-addr`** or env **`JOBHOUND_DEBUG_HTTP_ADDR`** (see **`contracts/environment.md`**). It serves **`GET /health`** and **one POST route per wired source** — `POST /debug/collectors/europe_remotely`, `POST /debug/collectors/working_nomads`, `POST /debug/collectors/dou_ua`, **`POST /debug/collectors/himalayas`**, **`POST /debug/collectors/djinni`**, **`POST /debug/collectors/builtin`** — so each site can be exercised in isolation (e.g. Postman, curl). It is **not** the product HTTP API; **`specs/009-http-public-api`** remains the contract for public endpoints. Do not expose debug routes in production builds.

**Implementation**: `internal/collectors/handlers/debughttp`.

### JSON request body (single contract, all MVP POST routes)

Use **`Content-Type: application/json`**. Body is optional; max size ~512 KiB. **URL query parameters are not used** for collector debug (everything below is JSON keys).

| Field | Type | Default | Meaning |
|-------|------|---------|---------|
| `limit` | int | `200` | Cap how many jobs appear in `jobs`. **`0`** = no cap (full collector run — can be large/slow). Maximum **`10000`**. Omitted key → default `200`. Invalid values → HTTP **400**. |

**Europe Remotely**: full `Fetch` first when `limit` ≠ `0`; the handler then **truncates** the returned slice to `limit`. When truncation happens, **`upstream_fetched`** in the response is the pre-truncation count.

**Working Nomads**: when the agent wires a concrete `*workingnomads.WorkingNomads`, `limit` maps to **`MaxFetchJobs`** on a **per-request copy** so pagination stops early.

**DOU.ua**: when the agent wires a concrete `*dou.DOU`, `limit` maps to **`MaxJobs`** on a **per-request copy** (listing + detail pagination stop early). When `limit` is **`0`**, the collector uses its configured default cap (**`JOBHOUND_COLLECTOR_DOU_MAX_JOBS_PER_FETCH`**, default 100).

Additional fields (Working Nomads only; **ignored** on `europe_remotely`):

| Field | Type | Meaning |
|-------|------|---------|
| `query` | object | Replaces default `match_all` (site `jobsapi/_search`). |
| `sort` | array | Replaces default sort. |
| `page_size` | int | ES `size` per page. |
| `_source` | string array | ES `_source` field list; must include fields the normalizer needs. |

Example:

```json
{
  "limit": 50,
  "query": { "match": { "title": "frontend" } },
  "page_size": 25
}
```

### JSON response (success)

- **`ok`**, **`collector`** (source name), **`count`** — length of **`jobs`** in this response.
- **`upstream_fetched`** — optional; set only when Europe Remotely (or another path that full-fetches then truncates) returned more rows than JSON `limit` after a full fetch.
- **`jobs`** — array of normalized vacancies: all MVP **`domain.Job`** fields exposed for debugging: `id`, `source`, `title`, `company`, `url`, `apply_url`, `description`, `posted_at` (RFC3339nano, UTC, if known), `remote`, `country_code`, `salary_raw`, `tags`, `position`, `timezone_offsets` (when present), `user_id`. This is for human verification of **`contracts/domain-mapping-mvp.md`** mapping, not the public API schema.

Working Nomads **`query` / `sort` / `page_size` / `_source`** are documented above as part of the **same** JSON object as `limit`. See **`resources/working-nomads.md`** for the site wire format; request field types and date examples: **`contracts/debug-http-collectors.md`**. **Pipeline** date/keyword rules remain **`specs/004-pipeline-stages`**; this body only exercises **site-side** filters.

If the agent does not pass a concrete Working Nomads pointer (e.g. some tests), `query` / `sort` / `page_size` / `_source` are not applied; JSON **`limit`** still truncates after `Fetch` like Europe Remotely.

## Tests

Offline: **`httptest`** + bodies from **`contracts/test-fixtures.md`** (or copies under `internal/collectors/.../testdata/`). Default **`go test ./...`** stays without mandatory live network. **Concrete cases** (Europe feed + detail, Working Nomads `_search`, DOU listing + `xhr-load` + detail) and definition-of-done live in **`tasks.md`** — same style as **`specs/004-pipeline-stages/tasks.md`**.

## Out of scope

- **Slot** model, **`slot_id`**, per-slot stage-1 parameters beyond what a **collector constructor** needs (e.g. site-specific query overrides for debug), **immutable broad string** after first ingest, **hard delete** of slot data — product rules in **`000`** / orchestration epics; not collector package logic.
- Cache, upsert, watermarks, dedup policy, Redis lock by **`source_id`** — `006-cache-and-ingest`.
- Pipeline filter/scoring rules — `004-pipeline-stages` (collectors only fill `Job`).

## Dependencies

- `001` — `domain.Job`, `StableJobID`.
- `002` — `jobs` table; extended per `jobs-table-extension.md` when salary/tags/position are implemented.

## Local / Docker

- T2: no extra services.
- T3: rod + cookie path env documented in **`contracts/environment.md`** when first T3 collector ships (and `internal/config`).

## Planning artifacts

- **`plan.md`** — phases, constitution check, resolved decisions
- **`research.md`** — short inventory and test pointers
- **`tasks.md`** — implementation checklist (including parser/HTTP test expectations)
- **`checklists/requirements.md`** — spec quality checklist

## Acceptance criteria

1. **Europe Remotely**, **Working Nomads**, **DOU.ua**, and **Himalayas** each implement **`collectors.Collector`** (wired from **`internal/pipeline`**), with **`Name()`** / **`Fetch`** semantics per **`contracts/collector.md`** and normalization per **`contracts/domain-mapping-mvp.md`**.
2. **No HTTP retries**; failures surface as **`error`** per collector contract (with Europe **date** soft-fail rule where specified).
3. **Unit tests** cover parsing/mapping using **`contracts/test-fixtures.md`** (or `testdata/` copies) — **no mandatory live network** for **`go test ./...`**; details in **`tasks.md`** sections D.2 and E.2.
4. Site-specific HTTP and DOM/JSON shapes stay in **`internal/collectors/...`**; **`internal/pipeline`** does not import per-site parsers.
5. **`contracts/*`** and **`resources/*`** remain the source of truth for wire behaviour; **`plan.md`** / **`tasks.md`** stay aligned after any spec edit.

## Related

- [`specs/000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md) — global MVP behavior (slots, stage 1–3, resets)
- `contracts/collector.md` — boundary + errors + `Job.Source` strings
- `contracts/domain-mapping-mvp.md` — Europe Remotely + Working Nomads + DOU.ua + Himalayas + Djinni + Built In → `Job`
- `contracts/jobs-table-extension.md` — optional SQL columns
- `contracts/test-fixtures.md` — fenced sample bodies
- `contracts/sources-inventory.md`
- `contracts/environment.md` — T3 env placeholder
- `contracts/debug-http-collectors.md` — debug POST JSON types + date `query` examples
- `resources/europe-remotely.md`, `resources/working-nomads.md`, `resources/dou.md`, `resources/himalayas.md`, `resources/djinni.md`, `resources/builtin.md`
- `plan.md`, `tasks.md`, `research.md`, `checklists/requirements.md`
- `specs/000-epic-overview/spec.md`, `.specify/memory/constitution.md`
- `specs/004-pipeline-stages/spec.md` — consumes normalized `Job`
