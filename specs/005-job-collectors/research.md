# Research Notes: Job collectors

**Branch**: `005-job-collectors`  
**Spec**: `specs/005-job-collectors/spec.md`  
**Date**: 2026-03-30  
**Last Updated**: 2026-04-10

Short inventory and pointers. **Wire/DOM/JSON detail** is normative in **`resources/europe-remotely.md`**, **`resources/working-nomads.md`**, **`resources/dou.md`**, and **`resources/himalayas.md`**, not duplicated here.

---

## 1. MVP sources (facts)

| Source | Transport | Parser stack |
|--------|-----------|----------------|
| Europe Remotely | `POST` `admin-ajax.php` → JSON + HTML fragment; `GET` job page | `encoding/json` + **goquery** on fragment + full page |
| Working Nomads | `POST` `jobsapi/_search` → Elasticsearch-shaped JSON | **`encoding/json`** only for core fields |
| DOU.ua | `GET` listing + `POST` `xhr-load` → JSON + HTML fragment; `GET` detail | `encoding/json` + **goquery**; **`http.Client` with cookie jar** for CSRF cookie |
| Himalayas | `GET` `/jobs/api` and `/jobs/api/search` → JSON | **`encoding/json`** only; **no** RSC/HTML parse (`internal/collectors/himalayas`) |

## 2. Test strategy (aligned with `004`)

- **No live site** in default unit tests.
- **Golden bodies**: fenced blocks in **`contracts/test-fixtures.md`**; optional copies under `testdata/`.
- **Europe**: relative posted times need **fixed clock** in tests if `PostedAt` is asserted.
- **Working Nomads**: **`pub_date`** is structured — parse failure policy per **`domain-mapping-mvp.md`** (strict).

## 3. Dependencies

- **`001`** — `domain.Job`, `StableJobID`.
- **`002`** / **`jobs-table-extension.md`** — when persisting new job fields.
- **`004`** — consumes normalized `Job` (e.g. `PostedAt`, remote, country).
- **`006`** — slot-scoped ingest, upsert, watermarks, Redis coordination; **not** implemented in **`005`**.
- **[`000` product concept draft](../000-epic-overview/product-concept-draft.md)** — MVP behavior for slots and stage-1 vs later stages; epics must stay consistent.

## 4. Risks (summary)

- Undocumented `admin-ajax.php` / `_search` body shapes — mitigate with fixtures and occasional manual capture in **`resources/*.md`**.
