# Feature: Job collectors

**Feature Branch**: `005-job-collectors`  
**Created**: 2026-03-29  
**Last Updated**: 2026-03-30  
**Status**: Draft

## Goal

Per-source **`Collector`** implementations **fetch** listings and **normalize** to **`domain.Job`**. Stack is **tiered**: `net/http` + **goquery** and/or **`encoding/json`** first; **go-rod** + optional session only when a source requires it (constitution). Timeouts and structured errors — no silent junk. **No HTTP retries** on collector requests (one attempt per logical request; failure surfaces as error).

## Spec artifacts (this directory)

**Markdown only** under `specs/005-job-collectors/`. **Per-source wire notes** (endpoints, DOM/JSON shapes) live in **`resources/`**. Reference HTTP/JSON/HTML samples live in fenced blocks in **`contracts/test-fixtures.md`**, not as separate `.json` / `.html` files here.

Canonical **MVP normalization and error policy** (description text, `Remote`, `Position`, dates, strict vs soft failures): **`contracts/domain-mapping-mvp.md`**.

## Layout (implementation)

- **`internal/collectors/`** — one package (or subtree) per source.
- **`internal/collectors/utils`** — shared HTTP, `data/countries.json` matching, dates, URL normalization.
- **`internal/pipeline`** — calls **`pipeline.Collector`** only; **no** site-specific parsing there.

Details: **`contracts/collector.md`**.

## Pagination (MVP)

Listing UIs use buttons such as **“Load more”** / **“Show more jobs”**; the collector **does not** drive a browser. It repeats the same **HTTP** calls the UI uses (**`admin-ajax.php`** + `has_more` for Europe Remotely; **`jobsapi/_search`** with `from` / `size` for Working Nomads). See **`resources/europe-remotely.md`** and **`resources/working-nomads.md`**.

## Sources

Canonical list in **`contracts/sources-inventory.md`**.

- **MVP:** Europe Remotely, Working Nomads.
- **Planned:** six further rows in the inventory; sequenced, not dropped.

**Remote OK** is out of scope (inventory).

## Fetch stance (MVP, per inventory facts)

- **Europe Remotely:** `POST` `admin-ajax.php` JSON (`has_more` + HTML fragment) + job detail **`GET`** — **T2**. A **captured request body** from browser DevTools should be recorded in **`resources/europe-remotely.md`** when available (`action`, filters, pagination fields).
- **Working Nomads:** `POST` `jobsapi/_search` JSON — **T2**; HTML shell not required for core fields.
- **Rod:** only when a source cannot be served without JS/session.

## Normalized fields

MVP mapping and planned **`SalaryRaw` / `Tags` / `Position`**: **`contracts/domain-mapping-mvp.md`**.  
Persistence extension for **`jobs`**: **`contracts/jobs-table-extension.md`**.

## Temporary debug HTTP (before `011`)

To manually verify collectors without the public API spec, a **local-only** debug server may live under **`cmd/*`** (e.g. flag-gated `GET /health`, `POST /debug/run-collector`). It is **not** the product HTTP API; **`specs/011-http-public-api`** remains the contract for public endpoints. Do not expose debug routes in production builds.

## Tests

Offline: **`httptest`** + bodies from **`contracts/test-fixtures.md`** (or copies under `internal/collectors/.../testdata/`). Default **`go test ./...`** stays without mandatory live network. **Concrete cases** (Europe feed + detail, Working Nomads `_search`) and definition-of-done live in **`tasks.md`** — same style as **`specs/004-pipeline-stages/tasks.md`**.

## Out of scope

- Cache, upsert, watermarks, dedup policy — `006-cache-and-ingest`.
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

1. **Europe Remotely** and **Working Nomads** each implement **`pipeline.Collector`**, with **`Name()`** / **`Fetch`** semantics per **`contracts/collector.md`** and normalization per **`contracts/domain-mapping-mvp.md`**.
2. **No HTTP retries**; failures surface as **`error`** per collector contract (with Europe **date** soft-fail rule where specified).
3. **Unit tests** cover parsing/mapping using **`contracts/test-fixtures.md`** (or `testdata/` copies) — **no mandatory live network** for **`go test ./...`**; details in **`tasks.md`** sections D.2 and E.2.
4. Site-specific HTTP and DOM/JSON shapes stay in **`internal/collectors/...`**; **`internal/pipeline`** does not import per-site parsers.
5. **`contracts/*`** and **`resources/*`** remain the source of truth for wire behaviour; **`plan.md`** / **`tasks.md`** stay aligned after any spec edit.

## Related

- `contracts/collector.md` — boundary + errors + `Job.Source` strings
- `contracts/domain-mapping-mvp.md` — Europe Remotely + Working Nomads → `Job`
- `contracts/jobs-table-extension.md` — optional SQL columns
- `contracts/test-fixtures.md` — fenced sample bodies
- `contracts/sources-inventory.md`
- `contracts/environment.md` — T3 env placeholder
- `resources/europe-remotely.md`, `resources/working-nomads.md`
- `plan.md`, `tasks.md`, `research.md`, `checklists/requirements.md`
- `specs/000-epic-overview/spec.md`, `.specify/memory/constitution.md`
- `specs/004-pipeline-stages/spec.md` — consumes normalized `Job`
