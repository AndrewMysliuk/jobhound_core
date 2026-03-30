# Working Nomads — extractable data (JSON search API)

**Site**: [workingnomads.com](https://www.workingnomads.com/)  
**Inventory**: MVP row 2 in `../contracts/sources-inventory.md`

This document is the **interface of what the site exposes** via its Elasticsearch-shaped search endpoint. **No** mapping to `domain.Job` here — that is decided later.

---

## Canonical listing URL (stable job identity)

Use **path** form as the single canonical listing URL for dedup / stable IDs:

`https://www.workingnomads.com/jobs/{slug}`

- `{slug}` is `_source.slug` from the API (e.g. `senior-full-stack-developer-lemonio-1502763`).
- The SPA often opens the same vacancy as `https://www.workingnomads.com/jobs?job={slug}`. Treat that as an **equivalent alias**: when normalizing inbound URLs, **rewrite** `?job=` to the path form above so one vacancy never gets two keys.

Card markup uses a **relative** `href` `/jobs/{slug}` — resolve against `https://www.workingnomads.com`.

Internal numeric identity: `hits[i]._id` matches `_source.id` in observed responses; useful for logging, but **canonical URL** remains the path above (per product rule: source + canonical listing URL).

---

## How data arrives (transport)


| Piece                            | Mechanism                                              | Notes                                                                                                                                                                                                                                        |
| -------------------------------- | ------------------------------------------------------ | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| Search / listing / detail fields | `POST` `https://www.workingnomads.com/jobsapi/_search` | JSON body (Elasticsearch-style). Same response shape as [unfiltered `_search`](https://www.workingnomads.com/jobsapi/_search) when called without filters; UI applies a **`query`** subtree for tags, locations, category, text search, etc. |
| Optional HTML                    | `GET` listing or `GET` / `?job=` job shell             | **Not required** for ingest if `_source` from `_search` is sufficient (full `description` HTML is already in JSON).                                                                                                                          |


**Tier**: **T2 (fact)** — `net/http` + `encoding/json` only; no goquery required for core fields. Revisit **T3** only if the endpoint later requires session, non-browser headers, or returns errors for server-side clients.

### Pagination vs UI

In-browser **“Show more jobs”** / similar controls call the same **`POST .../jobsapi/_search`** endpoint with increasing **`from`** (and fixed **`size`**). The collector paginates **only via this API** — no headless browser for MVP.

### `pub_date` and incremental fetch

`_source.pub_date` is **ISO-8601** (e.g. `2026-03-26T14:03:18+00:00`). It supports **server-side date range filters** in the Elasticsearch `query` when the product needs `from`/`to` windows. **Watermarks, dedup, and ingest policy** live in **`006-cache-and-ingest`**; collectors expose accurate **`PostedAt`** from `pub_date`.

---

## Request body (logical shape)

Captured from browser **Network** (Copy as JSON / cURL). Field names below match typical UI requests.


| Field              | Type            | Meaning                                                                                                                                                      |
| ------------------ | --------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------ |
| `track_total_hits` | bool            | e.g. `true` — exact total in `hits.total`                                                                                                                    |
| `from`             | int             | Offset for pagination (0, 100, 200, …)                                                                                                                       |
| `size`             | int             | Page size (e.g. `100`)                                                                                                                                       |
| `min_score`        | number          | e.g. `2` — Elasticsearch `min_score`                                                                                                                         |
| `query`            | object          | **Filter-specific** bool / full-text clause — copy from DevTools for each saved preset (tag, location, category, keywords). Not stable to guess in the spec. |
| `sort`             | array           | Observed: `[{ "premium": { "order": "desc" } }, { "_score": { "order": "desc" } }, { "pub_date": { "order": "desc" } }]`                                     |
| `_source`          | array of string | Subset of `_source` field names to return — include every field the product needs (see table below).                                                         |


**Pagination**: advance `from` by `size` until `from + len(response.hits.hits) >= response.hits.total.value` (and handle `hits.total.relation` if ever not `eq`).

**Headers**: match browser or minimal `Content-Type: application/json` + sensible `User-Agent`; record any required cookies/headers if behavior changes.

### Local debug HTTP (`cmd/agent`)

`POST /debug/collectors/working_nomads` (see **`../spec.md`**) uses **one JSON object**: **`limit`** (default `200`, `0` = full index) plus optional **`query`**, **`sort`**, **`page_size`**, **`_source`**. Overrides apply to a **one-shot copy** of `WorkingNomads` for that request. No URL query parameters.

---

## Response: `SearchResponse` (top level)

Elasticsearch-style envelope:


| Field                 | Type   | Meaning                                                          |
| --------------------- | ------ | ---------------------------------------------------------------- |
| `took`                | int    | Server timing (ms)                                               |
| `timed_out`           | bool   |                                                                  |
| `_shards`             | object | Shard stats                                                      |
| `hits.total.value`    | int    | Total matching documents (when `track_total_hits` + exact count) |
| `hits.total.relation` | string | e.g. `eq`                                                        |
| `hits.max_score`      | number | null                                                             |
| `hits.hits`           | array  | List of `SearchHit`                                              |


---

## Interface: `SearchHit` (one `hits.hits[]` element)


| Field     | Type   | Meaning                                                             |
| --------- | ------ | ------------------------------------------------------------------- |
| `_index`  | string | e.g. `jobsapi`                                                      |
| `_id`     | string | Document id (matches `_source.id` as string in samples)             |
| `_score`  | number | Relevance score                                                     |
| `_source` | object | See **`JobDocument`** below                                         |
| `sort`    | array  | Tie-break values for search_after-style pagination if adopted later |


---

## Interface: `JobDocument` (`_source`)

All string fields **trim** where applicable. `description` is **HTML** (same order of richness as on-site job view).


| Field                  | Type          | Present  | Notes                                                                         |
| ---------------------- | ------------- | -------- | ----------------------------------------------------------------------------- |
| `id`                   | number        | usually  | Numeric job id                                                                |
| `title`                | string        | usually  | Display title (may disagree with `slug` wording — data quirk)                 |
| `slug`                 | string        | usually  | **Use for canonical URL** path segment                                        |
| `company`              | string        | usually  |                                                                               |
| `company_slug`         | string        | usually  | Company profile path segment (`/remote-company/{company_slug}`)               |
| `category_name`        | string        | usually  | e.g. `Development`                                                            |
| `description`          | string (HTML) | usually  | Full job body                                                                 |
| `position_type`        | string        | usually  | e.g. `ft` full-time, `fr` freelance/contract — **opaque** until mapping       |
| `tags`                 | []string      | often    | Skill / topic tags                                                            |
| `all_tags`             | []string      | optional | May be empty                                                                  |
| `locations`            | []string      | often    | Region / country labels                                                       |
| `location_base`        | string        | often    | Summary line                                                                  |
| `location_extra`       | string        | optional | Extra location text                                                           |
| `pub_date`             | string        | usually  | ISO-8601 timestamp                                                            |
| `apply_option`         | string        | usually  | `with_your_ats`                                                               |
| `apply_email`          | string        | optional | Populated when `apply_option` is `with_email`                                 |
| `apply_url`            | string        | optional | External ATS / apply URL when not email-only                                  |
| `external_id`          | string        | optional | Upstream id                                                                   |
| `source`               | string        | optional | e.g. `feed`                                                                   |
| `premium`              | bool          | usually  | Featured / premium listing                                                    |
| `premium_subscription` | bool          | optional | Observed on some documents                                                    |
| `instructions`         | string        | optional | Extra apply instructions (HTML possible); often empty                         |
| `expired`              | bool          | usually  | Skip or mark inactive if `true`                                               |
| `use_ats`              | bool          | optional |                                                                               |
| `salary_range`         | string        | optional | **Opaque** display string                                                     |
| `salary_range_short`   | string        | optional | **Opaque**; may be empty or placeholder                                       |
| `annual_salary_usd`    | number        | null     | optional                                                                      |
| `experience_level`     | string        | optional | e.g. `SENIOR_LEVEL`, `MID_LEVEL`, `ENTRY_LEVEL` — **opaque** enum from source |


**Apply mapping (logical, not `domain.Job`):**

- `with_your_ats` → primary apply target is `apply_url` when non-empty.
- `with_email` → `apply_email` when non-empty; `apply_url` may be empty.
- `with_our_ats` → site-hosted flow; still capture `apply_url` if present.

**Salary:** `salary_range` / `annual_salary_usd` may be empty while compensation exists only inside `description` HTML — keep **opaque** strings; do not parse in this contract.

---

## Interface: `FeedBatch` (one successful `_search` page)

Logical shape after JSON decode:


| Field   | Type            | Meaning                                     |
| ------- | --------------- | ------------------------------------------- |
| `total` | int             | `hits.total.value`                          |
| `jobs`  | []`JobDocument` | Each element is `hits.hits[i]._source`      |
| `from`  | int             | Request offset (for logging / continuation) |


No separate **ListingCard** vs **JobDetail** layer is required for Working Nomads when using full `_source`: **one hit = one full listing row** including description.

---

## Noise / out of scope

- Browser calls to ad / CMP endpoints (e.g. geoip, TCF vendor lists) are **not** part of job ingest.
- Do not rely on initial HTML shell for job fields; **JSON is authoritative**.

---

## Risks

- `query` body shape is **not** a documented public API — it can change with site deploys. Prefer **fixtures** + optional integration tests.
- `min_score` / sort order affect ordering and which hits appear; align collector presets with product filters (tag, location, category).
- Slug vs title mismatch possible — do not use title as URL key.

## Related

- `../spec.md`
- `../contracts/sources-inventory.md`
- `../contracts/domain-mapping-mvp.md` — → `domain.Job`
- `../contracts/test-fixtures.md` — sample `_search` JSON
- `europe-remotely.md` — MVP source 1 (HTML/AJAX contrast)

