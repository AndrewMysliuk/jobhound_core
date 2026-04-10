# Himalayas — public JSON jobs API

**Site**: [himalayas.app](https://himalayas.app/)  
**Product API article**: [Remote Jobs API](https://himalayas.app/api)  
**Inventory**: row 4 in `../contracts/sources-inventory.md`

This document is the **wire shape** of the public JSON API. **Mapping** to `domain.Job` is in **`../contracts/domain-mapping-mvp.md`** (Himalayas section).

The marketing site job browse UI uses **Next.js RSC** (`text/x-component`, `?_rsc=...`); **collectors must not** depend on that stream. Ingest uses **only** the JSON endpoints below.

---

## Tier

**T2 (fact)** — `GET` + `encoding/json` only; no goquery, no headless browser for core fields.

---

## Terms (operator)

- **Attribution:** their docs ask to link to the job URL on Himalayas and credit Himalayas as the source.
- **Syndication limits:** do not submit Himalayas listings to third-party job aggregators they name (e.g. Jooble, Google Jobs, LinkedIn Jobs) — see the API page.
- **Rate limiting:** expect **HTTP 429** when exceeding fair use; backoff and/or contact them for higher volume.

---

## Canonical listing URL (stable job identity)

Use the **HTTPS job page URL** from the API as the single canonical listing URL for **`Job.URL`** and **`StableJobID`**:

- Prefer **`guid`** when non-empty; otherwise **`applicationLink`**.
- Observed responses often set **`guid` == `applicationLink`** (absolute URL, path under `/companies/{companySlug}/jobs/...`).
- **Normalize** with the shared collector URL helper: strip **fragment** and **query** (e.g. drop `?ref=…`) so one vacancy does not produce multiple IDs.

Internal numeric identity is not exposed as a separate stable key; **`guid`** / normalized URL is the contract.

---

## Endpoints

### Browse (full feed)

`GET https://himalayas.app/jobs/api`

| Query param | Meaning |
| ----------- | ------- |
| `offset` | Jobs to skip (pagination). |
| `limit` | Page size — **maximum 20** per request (enforced server-side as of 2025-03-24 per their docs). |

Example: `https://himalayas.app/jobs/api?limit=20&offset=0`

**Pagination:** increment `offset` by `limit` until the returned **`jobs`** array is empty or shorter than `limit`, or `offset + len(jobs) >= totalCount` when **`totalCount`** is present and trustworthy (prefer matching on empty/shorter page for robustness).

### Search (filtered)

`GET https://himalayas.app/jobs/api/search`

| Query param | Meaning (summary) |
| ----------- | ----------------- |
| `q` | Free-text query. |
| `country` | Country filter (ISO alpha-2, names, slugs). |
| `worldwide` / `exclude_worldwide` | Worldwide-friendly toggles. |
| `seniority` | One or more: Entry-level, Mid-level, Senior, Manager, Director, Executive. |
| `employment_type` | Full Time, Part Time, Contractor, etc. |
| `company` | Comma-separated **company slugs** (use `companySlug` from responses). |
| `timezone` | e.g. `-5`, `UTC+05:30`. |
| `sort` | `relevant`, `recent`, `salaryAsc`, `salaryDesc`, `nameAToZ`, `nameZToA`, `jobs`. |
| `page` | **1-based** page index. |

Example: `https://himalayas.app/jobs/api/search?q=vue&page=1`

**Response envelope** for search matches browse-style pagination fields in practice: **`offset`**, **`limit`** (e.g. 20), **`totalCount`**, **`jobs`**. Advance **`page`** until no jobs or page exceeds total pages implied by **`totalCount`**.

---

## Response envelope (browse and search)

Top-level JSON (field names observed in live responses; tolerate extra fields):


| Field | Type | Meaning |
| ----- | ---- | ------- |
| `comments` | string | Human-readable API changelog notes (ignore for parsing). |
| `updatedAt` | number | Server timestamp (ignore unless product needs it). |
| `offset` | number | Echo / effective offset for this response. |
| `limit` | number | Page size for this response (≤ 20 on browse). |
| `totalCount` | number | Total jobs matching the query (search) or feed size hint. |
| `jobs` | array | Job objects (see below). |

---

## Job object (fields used by mapping)

Observed JSON keys (snake_case is **not** used — API uses **camelCase**):


| Field | Type | Notes |
| ----- | ---- | ----- |
| `title` | string | |
| `excerpt` | string | Short text; used as extra input to **`Remote`** MVP rule (see domain mapping). |
| `companyName` | string | Display name. |
| `companySlug` | string | Canonical slug for `company` search filter. |
| `companyLogo` | string | URL; optional for `Job` mapping. |
| `employmentType` | string | e.g. `Full Time`. |
| `minSalary` / `maxSalary` | number or null | |
| `currency` | string | e.g. `USD`. |
| `seniority` | array of string | |
| `locationRestrictions` | array of string | Country names or regions; **CountryCode** resolution via `data/countries.json`. |
| `timezoneRestrictions` | array of number | UTC offsets as decimals (e.g. `5.5`, `-3.5`); maps to **`Job.TimezoneOffsets`** (see **`jobs-table-extension.md`**). |
| `categories` | array of string | Slug-like tokens (e.g. `Remote-Software-Engineer`). |
| `parentCategories` | array of string | |
| `description` | string | **HTML**; strip to plain text for **`Job.Description`**. |
| `pubDate` | number | **Unix epoch seconds** (UTC). |
| `expiryDate` | number | Optional: may inform freshness filtering later; not required for core `Job` fields. |
| `applicationLink` | string | Often the same as listing URL on himalayas.app. |
| `guid` | string | Stable string ID; often identical to **`applicationLink`**. |

**Note:** public docs sometimes mention `category` (singular); live responses may expose **`categories`** — implement parsers to accept **`categories`** as authoritative.

---

## HTTP

- **Method:** `GET` only for both endpoints.
- **Auth:** none.
- **Headers:** sensible **User-Agent**; `Accept: application/json` is fine.
- **No retries** per collector contract — one attempt per request (see **`../contracts/collector.md`**).

---

## OpenAPI

Machine-readable spec (optional for implementers): `https://himalayas.app/docs/openapi.json`.

---

## Related

- `../contracts/domain-mapping-mvp.md` — Himalayas → `Job`
- `../contracts/collector.md` — errors, `Job.Source` value `himalayas`
- `../contracts/test-fixtures.md` — minimal JSON sample for tests
