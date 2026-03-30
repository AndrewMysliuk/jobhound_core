# Tasks: Job collectors (MVP sources)

**Input**: `spec.md`, `plan.md`, `research.md`, `contracts/*`, `resources/*`  
**Tests**: REQUIRED — `**go test ./...`** without live network; `**httptest**` + bodies aligned with `**contracts/test-fixtures.md**`. Wire shapes and selectors: `**resources/europe-remotely.md**`, `**resources/working-nomads.md**`.

## A. Contracts & docs

1. [ ] **Contracts match intent** — Definition of done: `collector.md`, `domain-mapping-mvp.md`, `test-fixtures.md`, `resources/*` reviewed; no contradictions with `plan.md` Resolved decisions.

## B. Domain & persistence (if in scope for this PR)

1. [ ] `**domain.Job` + jobs table** — Definition of done: `SalaryRaw`, `Tags`, `Position` (and any other MVP fields) exist in domain and `**jobs-table-extension.md`** migration applied if product ships persistence in the same change set; **or** explicit note in PR that fields are collector-only until a follow-up.

## C. Shared `internal/collectors/utils`

1. [ ] **HTTP + URL + country helpers** — Definition of done: shared timeout/UA pattern, canonical URL / `StableJobID` inputs, country resolution per `**domain-mapping-mvp.md`**; no site-specific selectors here.

## D. Europe Remotely

1. [ ] **Implement collector** — Definition of done: `POST` feed JSON (`has_more` + `html`), parse cards per `**resources/europe-remotely.md`**, `GET` detail, map to `**domain.Job**` per `**domain-mapping-mvp.md**`; pagination via `has_more`; errors per `**collector.md**`.
2. [ ] **Unit tests — Europe** — Definition of done: decode sample **feed JSON** from `**test-fixtures.md`** → at least one card with expected **title, company, listing URL**; parse **detail HTML** excerpt → expected **title, apply URL, description plain text** (or agreed subset); **injectable clock** for relative **posted** strings if asserted; optional `**httptest`** end-to-end for one listing + one detail response.

## E. Working Nomads

1. [ ] **Implement collector** — Definition of done: `POST` `_search`, decode hits, canonical listing URL `https://www.workingnomads.com/jobs/{slug}` per `**resources/working-nomads.md`**, map `_source` → `**domain.Job**`; skip or error on `**expired**` per agreed rule; pagination via `from`/`size` and total.
2. [ ] **Unit tests — Working Nomads** — Definition of done: decode sample `**_search` JSON** from `**test-fixtures.md`** → one `**domain.Job**` (or intermediate struct) with expected **title, company, URL, `PostedAt` from `pub_date`**; optional `**httptest**` single-page fetch.

## F. Wire-up

1. [ ] **Register collectors** — Definition of done: agent/worker (or composition root) can construct and run both collectors without importing site packages into `**internal/pipeline`** beyond `**Collector**`.

## G. Quality gates

1. [ ] `**make test` / `go test ./...**` — Definition of done: passes without network.
2. [ ] `**make vet` / `make fmt**` — Definition of done: clean for touched packages.

## H. Optional / deferred

1. [ ] **Debug HTTP runner** — Flag-gated `cmd/*` hook per `**spec.md`**; not required to close `005`.
2. [ ] **Captured real `admin-ajax.php` body** — Paste into `**resources/europe-remotely.md`** when available (redact secrets).

