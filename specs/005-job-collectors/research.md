# Research Notes: Job collectors

**Branch**: `005-job-collectors`  
**Spec**: `specs/005-job-collectors/spec.md`  
**Date**: 2026-03-30

Short inventory and pointers. **Wire/DOM/JSON detail** is normative in **`resources/europe-remotely.md`** and **`resources/working-nomads.md`**, not duplicated here.

---

## 1. MVP sources (facts)

| Source | Transport | Parser stack |
|--------|-----------|----------------|
| Europe Remotely | `POST` `admin-ajax.php` → JSON + HTML fragment; `GET` job page | `encoding/json` + **goquery** on fragment + full page |
| Working Nomads | `POST` `jobsapi/_search` → Elasticsearch-shaped JSON | **`encoding/json`** only for core fields |

## 2. Test strategy (aligned with `004`)

- **No live site** in default unit tests.
- **Golden bodies**: fenced blocks in **`contracts/test-fixtures.md`**; optional copies under `testdata/`.
- **Europe**: relative posted times need **fixed clock** in tests if `PostedAt` is asserted.
- **Working Nomads**: **`pub_date`** is structured — parse failure policy per **`domain-mapping-mvp.md`** (strict).

## 3. Dependencies

- **`001`** — `domain.Job`, `StableJobID`.
- **`002`** / **`jobs-table-extension.md`** — when persisting new job fields.
- **`004`** — consumes normalized `Job` (e.g. `PostedAt`, remote, country).

## 4. Risks (summary)

- Undocumented `admin-ajax.php` / `_search` body shapes — mitigate with fixtures and occasional manual capture in **`resources/*.md`**.
