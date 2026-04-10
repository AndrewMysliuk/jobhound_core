# Tasks: Job collectors (MVP sources)

**Last Updated**: 2026-04-10  
**Input**: `spec.md`, `plan.md`, `research.md`, `contracts/*`, `resources/*`  
**Tests**: REQUIRED — `**go test ./...`** without live network; `**httptest**` + bodies aligned with `**contracts/test-fixtures.md**`. Wire shapes and selectors: `**resources/europe-remotely.md**`, `**resources/working-nomads.md**`, `**resources/dou.md**` (DOU.ua).

## A. Contracts & docs

1. [x] **Contracts match intent** — Definition of done: `collector.md`, `domain-mapping-mvp.md`, `test-fixtures.md`, `resources/*` reviewed; no contradictions with `plan.md` Resolved decisions.
2. [x] **`SlotSearchFetcher`** — Definition of done: `contracts/collector.md` documents optional interface, per-source wire mapping, and **non-mutation** rule; all MVP collectors implement **`FetchWithSlotSearch`**; **`006`** ingest calls it when **`SlotSearchQuery`** is set.

## B. Domain & persistence (if in scope for this PR)

1. [x] `**domain.Job` + jobs table** — Definition of done: `SalaryRaw`, `Tags`, `Position` (and any other MVP fields) exist in domain and `**jobs-table-extension.md`** migration applied if product ships persistence in the same change set; **or** explicit note in PR that fields are collector-only until a follow-up.

## C. Shared `internal/collectors/utils`

1. [x] **HTTP + URL + country helpers** — Definition of done: shared timeout/UA pattern, canonical URL / `StableJobID` inputs, country resolution per `**domain-mapping-mvp.md`**; no site-specific selectors here.

## D. Europe Remotely

1. [x] **Implement collector** — Definition of done: `POST` feed JSON (`has_more` + `html`), parse cards per `**resources/europe-remotely.md`**, `GET` detail, map to `**domain.Job**` per `**domain-mapping-mvp.md**`; pagination via `has_more`; errors per `**collector.md**`.
2. [x] **Unit tests — Europe** — Definition of done: decode sample **feed JSON** from `**test-fixtures.md`** → at least one card with expected **title, company, listing URL**; parse **detail HTML** excerpt → expected **title, apply URL, description plain text** (or agreed subset); **injectable clock** for relative **posted** strings if asserted; optional `**httptest`** end-to-end for one listing + one detail response.

## E. Working Nomads

1. [x] **Implement collector** — Definition of done: `POST` `_search`, decode hits, canonical listing URL `https://www.workingnomads.com/jobs/{slug}` per `**resources/working-nomads.md`**, map `_source` → `**domain.Job**`; skip or error on `**expired**` per agreed rule; pagination via `from`/`size` and total.
2. [x] **Unit tests — Working Nomads** — Definition of done: decode sample `**_search` JSON** from `**test-fixtures.md`** → one `**domain.Job**` (or intermediate struct) with expected **title, company, URL, `PostedAt` from `pub_date`**; optional `**httptest**` single-page fetch.

## F. Wire-up

1. [x] **Register collectors** — Definition of done: agent/worker (or composition root) can construct and run both collectors without importing site packages into `**internal/pipeline`** beyond `**Collector**`.

## G. Quality gates

1. [x] `**make test` / `go test ./...**` — Definition of done: passes without network.
2. [x] `**make vet` / `make fmt**` — Definition of done: clean for touched packages.

## H. Optional / deferred

1. [x] **Debug HTTP runner** — Definition of done: `cmd/agent` `-debug-http-addr` and optional `JOBHOUND_DEBUG_HTTP_ADDR`; `GET /health`, per-source `POST /debug/collectors/europe_remotely`, `POST /debug/collectors/working_nomads`, and `POST /debug/collectors/dou_ua` via `internal/collectors/handlers/debughttp`; full `domain.Job` JSON in `jobs`; single JSON body contract (`limit` + optional per-source fields); unit tests without network.
2. [x] **Captured real `admin-ajax.php` body** — Definition of done: `**resources/europe-remotely.md`** holds implementation-aligned reference body (fields + example); replace with DevTools capture when available (redact secrets).

---

## Version 2 — Product alignment with global MVP draft (2026-04-02)

**Input**: [`specs/000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md)  
**Scope**: Documentation and contract cross-links only — **no code changes** required for this pass.

1. [x] **`spec.md`** — Definition of done: “Product alignment (MVP)” section; out-of-scope clarifies slot/`006` boundary; Related links `product-concept-draft.md`.
2. [x] **`plan.md`** — Definition of done: summary references product draft and orchestration boundary.
3. [x] **`contracts/collector.md`** — Definition of done: “Relationship to orchestration (MVP)” + Related link.
4. [x] **`contracts/domain-mapping-mvp.md`** — Definition of done: `UserID` sourcing rule (collectors leave unset; ingest may set).
5. [x] **`research.md`** — Definition of done: dependencies include `006` and product draft pointer.
6. [x] **`contracts/sources-inventory.md`** — Definition of done: Related includes product draft (phasing).
7. [x] **`checklists/requirements.md`** — Definition of done: product-alignment checklist rows updated.

---

## I. DOU.ua (third collector)

**Wire**: **`resources/dou.md`** — `GET` listing (`search` + `descr=1`), `POST` `xhr-load` (CSRF + `count`), JSON `html` / `last` / `num`, **`GET` detail** for full description (Europe Remotely pattern). **Cap**: 100 jobs per `Fetch`; **delay** between HTTP calls per resource doc.

1. [x] **Contracts** — Definition of done: `contracts/collector.md` lists normative `Job.Source` for DOU (e.g. `dou_ua`); `contracts/environment.md` documents any new `JOBHOUND_*` knobs (inter-request delay, optional max jobs override) if exposed beyond code defaults.

2. [x] **Fixtures** — Definition of done: `contracts/test-fixtures.md` contains fenced samples for listing HTML (incl. hidden `csrfmiddlewaretoken`), `xhr-load` JSON (`html`, `last`, `num`), and detail HTML per `resources/dou.md` selectors.

3. [x] **Implement collector** — Definition of done: `internal/collectors/<package>/` (one package per source) implements `collectors.Collector`; shared `http.Client` with **cookie jar**; parse listing + fragments with **goquery**; loop `xhr-load` until `last` or **100** jobs collected; **`GET` each vacancy URL** for `Description` / fields per `domain-mapping-mvp.md`; Ukrainian **posted** date parsing + tests; errors / partial batch policy per `collector.md`.

4. [x] **Unit tests — DOU** — Definition of done: `httptest` chain (listing → optional second `xhr-load` → detail) without live network; asserts on parsed cards and at least one full `domain.Job` (or agreed field subset) from fixtures.

5. [x] **Wire-up + debug HTTP** — Definition of done: composition root (`internal/collectors/bootstrap` or equivalent) registers DOU with MVP collectors; `internal/collectors/handlers/debughttp` adds `POST /debug/collectors/dou_ua` (or agreed name) + `handler.go` registration; update `contracts/debug-http-collectors.md` and `internal/collectors/schema/debug_http.go` for any DOU-specific JSON keys (e.g. `search` override, `limit` mapping).

6. [x] **Inventory** — Definition of done: `contracts/sources-inventory.md` row 3: **Tier (fact) T2** and Notes updated once collector is verified; `research.md` one-line pointer to `resources/dou.md` if still useful.

---

## J. Himalayas (fourth collector; public JSON API)

**Wire**: **`resources/himalayas.md`** — `GET https://himalayas.app/jobs/api` (`offset` / `limit` ≤ 20) and/or `GET https://himalayas.app/jobs/api/search` (`page`, filters). **No** Next.js RSC / HTML crawl. Respect **429** (document backoff or surface error per product). **Operator**: attribution + syndication limits on [himalayas.app/api](https://himalayas.app/api).

1. [x] **Domain + persistence** — Definition of done: `TimezoneOffsets []float64` on **`domain.Job`** (see **`contracts/domain-mapping-mvp.md`**); migration + GORM field per **`contracts/jobs-table-extension.md`** (`timezone_offsets` JSON); JSON debug output includes offsets when wired.

2. [x] **Spec + contracts (Himalayas prose)** — Definition of done: `contracts/collector.md` lists **`himalayas`** `Job.Source`; `contracts/domain-mapping-mvp.md` Himalayas table + `PostedAt` / `Remote` / **`TimezoneOffsets`**; `contracts/sources-inventory.md` row 4 **T2 (fact)**; `contracts/test-fixtures.md` minimal envelope; **`tasks.md`** § J present.

3. [x] **Fixtures** — Definition of done: tests use fenced sample from **`contracts/test-fixtures.md`** (Himalayas section) or a copy under `internal/collectors/himalayas/testdata/`; asserts decode + at least one mapped **`domain.Job`** field subset (title, company, URL, `PostedAt` from Unix, `TimezoneOffsets`, `Remote` when excerpt/categories contain `remote`).

4. [x] **Implement collector** — Definition of done: `internal/collectors/himalayas/` (or agreed package name) implements **`collectors.Collector`**; paginate browse and/or search per **`resources/himalayas.md`**; map JSON → **`domain.Job`** per **`domain-mapping-mvp.md`** (including **`RemoteMVPRule`** with **`excerpt`** + joined **`locationRestrictions`** as extra hints, same idea as DOU location hints); **`utils.CanonicalListingURL`** + **`AssignStableID`**; errors per **`collector.md`**.

5. [x] **Unit tests — Himalayas** — Definition of done: **`httptest`** returns JSON body from fixtures — no live network; table-driven cases for browse pagination stop condition and one search page if both modes are implemented.

6. [x] **Wire-up + debug HTTP** — Definition of done: `internal/collectors/bootstrap` registers Himalayas when configured; `handlers/debughttp` adds **`POST /debug/collectors/himalayas`** (or agreed path) + `handler.go` registration; optional JSON keys **`q`**, **`page`**, **`use_search`** (bool) / **`max_pages`** documented in **`contracts/debug-http-collectors.md`** and `schema/debug_http.go` if used.

7. [x] **Inventory + spec glue** — Definition of done: after implementation verification, `contracts/sources-inventory.md` Notes unchanged unless wire changed; `spec.md` / `research.md` reference **`resources/himalayas.md`** in fetch/debug sections.
