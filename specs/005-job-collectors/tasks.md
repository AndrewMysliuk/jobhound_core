# Tasks: Job collectors (MVP sources)

**Last Updated**: 2026-04-12  
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

---

## K. Djinni (fifth collector; HTML + JSON-LD)

**Wire**: **`resources/djinni.md`** — `GET` `https://djinni.co/jobs/?all_keywords=…&search_type=full-text&page=N` (~**15** jobs/page); **`GET`** each **`/jobs/{id}-{slug}/`**; parse listing cards (**`job-item__position`**, company span, link `href`); **detail** `JobPosting` in **`application/ld+json`**; listing page may embed a **JSON array** of `JobPosting` (optional use). **`baseSalary`** → opaque **`SalaryRaw`**. **Inter-request delay** (default **400 ms**, env — see **`contracts/environment.md`**). **`Job.Source`**: **`djinni`**. **`FetchWithSlotSearch`**: map **`SlotSearchQuery`** → **`all_keywords`**.

1. [x] **Spec + contracts** — Definition of done: `contracts/collector.md` already lists **`djinni`** and slot mapping; `contracts/domain-mapping-mvp.md` Djinni table + **`PostedAt`** rule; `contracts/sources-inventory.md` row 5 **T2 (fact)**; `contracts/environment.md` planned env names; **`resources/djinni.md`** matches captured selectors (listing meta row, JSON-LD array note).

2. [x] **Fixtures** — Definition of done: `contracts/test-fixtures.md` fenced samples — minimal listing HTML fragment (one card + link + optional `ld+json` array excerpt) and minimal detail HTML with one `JobPosting` script (include **`baseSalary`** min/max example).

3. [x] **Config** — Definition of done: `internal/config/collectors_djinni.go` loads **`JOBHOUND_COLLECTOR_DJINNI_INTER_REQUEST_DELAY_MS`** and **`JOBHOUND_COLLECTOR_DJINNI_MAX_JOBS_PER_FETCH`** per **`contracts/environment.md`**; defaults align with DOU-style politeness.

4. [x] **Implement collector** — Definition of done: `internal/collectors/djinni/` implements **`collectors.Collector`** + **`SlotSearchFetcher`**; pagination until **&lt; 15** jobs or empty; delay between HTTP calls; map JSON-LD → **`domain.Job`** per **`domain-mapping-mvp.md`** (**`Remote`**: **`TELECOMMUTE`** **or** **`RemoteMVPRule`** with listing meta hints); **`utils.CanonicalListingURL`** + **`AssignStableID`**; cap jobs per fetch; errors per **`collector.md`**.

5. [x] **Unit tests — Djinni** — Definition of done: **`httptest`** listing + detail without live network; at least one full **`domain.Job`** (or agreed field subset) including **`SalaryRaw`** from **`baseSalary`** and **`PostedAt`** from ISO **`datePosted`**.

6. [x] **Wire-up + debug HTTP** — Definition of done: `internal/collectors/bootstrap` registers Djinni when configured; `handlers/debughttp` adds **`POST /debug/collectors/djinni`** + `handler.go` registration; optional JSON keys **`all_keywords`**, **`djinni_page`**, **`djinni_inter_request_delay_ms`** documented in **`contracts/debug-http-collectors.md`** and `internal/collectors/schema/debug_http.go` if used; **`limit`** maps to **`MaxJobs`** when wired.

7. [x] **Inventory status** — Definition of done: after ship, set Djinni row **Status** to **MVP** or keep **Planned** per product phasing; **`spec.md`** acceptance criteria updated if Djinni becomes MVP.

---

## L. Built In (sixth collector; remote + JSON-LD)

**Wire**: **`resources/builtin.md`** — **`GET`** `https://builtin.com/jobs/remote` with **`country`** (ISO **alpha-3**), **`allLocations=true`**, **`search`** (from non-empty slot), **`page=1`** per country by default (optional **`page=2`** via debug **`builtin_max_listing_pages_per_country`** only; when enabled, stop early when **&lt; 20** on page 1); parse **`application/ld+json`** **`ItemList`** → job URLs; **`GET`** each detail → **`JobPosting`**. **`Fetch`** / empty **`FetchWithSlotSearch`**: **no HTTP**, **`[]Job`**. **18** territories (EU subset + **GB** + **UA**) per **`resources/builtin.md`**. **Dedup** URLs across countries/pages before details. **Per-request** listing/detail failures: **warn + skip**, return partial **`[]Job`**; **misconfiguration** (e.g. browser mode without fetcher) still **errors** the run. **Inter-request delay** (default **500 ms**, env — **`contracts/environment.md`**).

1. [x] **Spec + contracts** — Definition of done: **`resources/builtin.md`** matches product decisions; **`contracts/collector.md`** lists **`builtin`** `Job.Source`, slot/`Fetch` exception, and **`search`** mapping; **`contracts/domain-mapping-mvp.md`** Built In table + **`PostedAt`** rule; **`contracts/sources-inventory.md`** row 6 **T2 (fact)** + Notes; **`contracts/environment.md`** delay variable; **`contracts/debug-http-collectors.md`** Built In keys; **`spec.md`** / **`research.md`** reference **`resources/builtin.md`** where appropriate.

2. [x] **Fixtures** — Definition of done: **`contracts/test-fixtures.md`** fenced samples — minimal listing HTML with **`ItemList`** `@graph` and minimal detail HTML with **`JobPosting`** `@graph` (aligned with **`resources/builtin.md`**).

3. [x] **Config** — Definition of done: `internal/config/collectors_builtin.go` (or agreed name) loads **`JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS`** per **`contracts/environment.md`**; optional debug-only overrides wired through handler when implemented.

4. [x] **Implement collector** — Definition of done: `internal/collectors/builtin/` implements **`collectors.Collector`** + **`SlotSearchFetcher`**; country list + alpha-3→alpha-2 table from spec; listing pagination rules (default 1 page per country, optional 2 via debug, early stop when **&lt; 20** on page 1); URL dedup; delay between all requests; map detail JSON-LD → **`domain.Job`** per **`domain-mapping-mvp.md`** (**`CountryCode`** from request **`country`** param); **`utils.CanonicalListingURL`** + **`AssignStableID`**; errors per **`collector.md`**.

5. [x] **Unit tests — Built In** — Definition of done: **`httptest`** listing + detail chain **without live network** using fixtures; assert at least one **`domain.Job`** with expected **title, company, URL, `CountryCode`** from simulated **`country=`**; empty slot / empty **`builtin_search`** returns zero jobs without outbound calls (handler or collector unit test).

6. [x] **Wire-up + debug HTTP** — Definition of done: `internal/collectors/bootstrap` registers Built In when configured; `handlers/debughttp` adds **`POST /debug/collectors/builtin`** + `handler.go` registration; update **`internal/collectors/schema/debug_http.go`** for **`builtin_search`**, **`builtin_inter_request_delay_ms`**, **`builtin_max_listing_pages_per_country`** if used.

7. [x] **Inventory + status** — Definition of done: after ship, set Built In row **Status** to **MVP** or keep **Planned** per product phasing; **`tasks.md`** checkboxes updated.

8. [x] **Built In — Cloudflare / 403 (T3 path)** — Definition of done: failure mode documented (**403** + challenge HTML vs real listing); mitigation implemented via **§ M** (**generic `browserfetch`**) + Built In wiring (optional browser mode, default **`net/http`**); **`resources/builtin.md`**, **`contracts/environment.md`**, **`contracts/sources-inventory.md`** row 6 Notes updated; or explicit defer with owner/date in **`resources/builtin.md`** if product pauses Built In ingest.

---

## M. Tier-3 shared browser document fetcher (go-rod; Built In + future LinkedIn)

**Goal:** one **source-agnostic** mechanism: **`ctx` + URL → HTML bytes** after headless Chromium navigation (see **`contracts/browser-fetch.md`**). **Built In** is the first consumer; **LinkedIn** (planned) **must reuse** the same package/interface — no second copy of Rod bootstrap for each site.

**Non-goals in `browserfetch`:** JSON-LD / goquery / `domain.Job`; per-board pagination; login flows (LinkedIn session stays in **`internal/collectors/linkedin/`** or equivalent when specced, calling into **`browserfetch`** for document load only where appropriate).

1. [x] **Contracts** — Definition of done: **`contracts/browser-fetch.md`** reviewed; **`contracts/collector.md`** (layout: `browserfetch` is shared infra under **`internal/collectors/`**, not a `Collector`); **`spec.md`**, **`research.md`**, **`contracts/sources-inventory.md`** reference **`browser-fetch.md`** / § **M** where relevant.

2. [x] **Environment** — Definition of done: **`contracts/environment.md`** lists shared **`JOBHOUND_BROWSER_*`** (or agreed prefix): e.g. master enable, Chromium executable path, navigation/page timeout, optional user-data dir; plus Built In override if needed (**`JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER`** or “follow global only” — pick one and document); **`internal/config`** loaders named in same doc when code lands.

3. [x] **Package `internal/collectors/browserfetch`** — Definition of done: exported **fetch interface** + **Rod** implementation; **no** site-specific URLs or selectors inside; lifecycle documented (long-lived browser vs per-request — choose and note ops impact).

4. [x] **Unit tests (default `go test`)** — Definition of done: **fake/mock** fetcher implements the interface; **no** mandatory Chrome in CI; optional Rod smoke: **`integration`** tag and/or manual steps in **`browser-fetch.md`** or **`resources/builtin.md`**.

5. [x] **Built In integration** — Definition of done: **`builtin`** accepts injected **`HTMLDocumentFetcher`** (name as in code); **default** listing + detail loads use existing **`net/http`**; when browser mode enabled, **same URLs** go through **`browserfetch`**; JSON-LD parsing unchanged.

6. [x] **Bootstrap** — Definition of done: **`internal/collectors/bootstrap`** constructs optional rod-backed fetcher from config and passes into **`builtin.BuiltIn`**; T2 collectors unchanged.

7. [x] **Debug HTTP** — Definition of done: if overrides are useful, document **`builtin_use_browser`** (bool) in **`contracts/debug-http-collectors.md`** and **`internal/collectors/schema/debug_http.go`** when implemented.

8. [x] **Close § L.8** — Definition of done: checklist **L.8** marked complete when **§ M** items **1–7** (or agreed subset + explicit defer) and Built In doc/status are updated.
