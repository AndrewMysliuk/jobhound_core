# DOU.ua vacancies — extractable data (HTML / wire)

**Site**: [jobs.dou.ua](https://jobs.dou.ua/) — listing entry [vacancies with search-in-description](https://jobs.dou.ua/vacancies/?descr=1)  
**Inventory**: Planned row 3 in `../contracts/sources-inventory.md`

This document is the **interface of what the site exposes** that we can pull from responses and DOM. **No** mapping to `domain.Job` here — see **`../contracts/domain-mapping-mvp.md`**.

**Status**: Wire captured from DevTools (2026-04); adjust if the site changes markup or endpoints.

---

## How data arrives (transport)

| Piece | Mechanism | Notes |
| ----- | --------- | ----- |
| Listing (first batch) | `GET` `https://jobs.dou.ua/vacancies/` | Query: **`search`** = broad keyword string (stage-1 string); **`descr=1`** always — search includes **job description**, not title-only. Returns full HTML document with initial **~20** `li.l-vacancy` rows. |
| Listing (more batches) | `POST` `https://jobs.dou.ua/vacancies/xhr-load/` | Same **`search`** and **`descr=1`** in the **query string** as on the listing page. Body: **`application/x-www-form-urlencoded`** — see below. Response: **JSON** with an **`html`** string (fragment of `<li class="l-vacancy">…</li>` nodes) plus pagination flags. |
| Job detail | `GET` absolute vacancy URL | Canonical pattern `https://jobs.dou.ua/companies/{company-slug}/vacancies/{id}/`. Full HTML document. **Same pattern as Europe Remotely**: listing gives preview + link; **full description** comes from the detail page (Working Nomads is different — full text in JSON). |

**Tier**: **T2** — `net/http` + **goquery** on listing HTML, JSON envelope for `xhr-load`, then goquery on the `html` fragment; **no headless** unless a future spike proves otherwise.

---

## Search query (normative)

| Query param | Value | Meaning |
| ----------- | ----- | ------- |
| `search` | non-empty string (URL-encoded) | Broad keywords; maps to product stage-1 string for this source. |
| `descr` | **`1`** (always) | Include description in search scope. |

Example listing URL:

```text
https://jobs.dou.ua/vacancies/?search=frontend&descr=1
```

The `xhr-load` POST must use the **same** `search` and `descr` in its query string as the listing page used for `Referer` / first paint.

---

## `POST …/vacancies/xhr-load/` (load more)

### URL

`https://jobs.dou.ua/vacancies/xhr-load/?search=<same>&descr=1`

### Body (`application/x-www-form-urlencoded`)

| Field | Meaning |
| ----- | ------- |
| `csrfmiddlewaretoken` | Django CSRF — same family as cookie **`csrftoken`**; read from **hidden input** on the listing HTML (`<input name="csrfmiddlewaretoken" value="…">`). Observed: **stable for the session** (does not rotate on every `xhr-load`). |
| `count` | **Total number of vacancies already shown** on the listing before this batch (not “page size”). Flow: after first `GET`, typically **20** cards → first `POST` uses `count=20`; server returns more rows and **`num`** reflects new total (e.g. 40) → next `POST` uses `count=40`, until **`last`** is true. |

### Response JSON (observed shape)

| Field | Type | Meaning |
| ----- | ---- | ------- |
| `html` | string | HTML **fragment**: more `<li class="l-vacancy">…</li>` (escaped in JSON). Parse with goquery as a fragment, same idea as Europe Remotely’s `html` field. |
| `last` | bool | **`true`** when there are **no** more vacancies to load. |
| `num` | int | Server-reported total count after this batch (e.g. 40 after second wave). Implementations may rely primarily on **`last`** + **`count`** for pagination; use `num` only if it simplifies consistency checks. |

### Headers (recommended for parity with browser)

Use a single **`http.Client` with a cookie jar** so `Set-Cookie: csrftoken=…` from the initial `GET` is sent on `POST`.

| Header | Value / notes |
| ------ | ------------- |
| `Content-Type` | `application/x-www-form-urlencoded; charset=UTF-8` |
| `Referer` | Full listing URL, e.g. `https://jobs.dou.ua/vacancies/?search=frontend&descr=1` (must match current `search` / `descr`). |
| `Origin` | `https://jobs.dou.ua` |
| `X-Requested-With` | `XMLHttpRequest` |
| `Accept` | `application/json, text/javascript, */*; q=0.01` (or minimal `application/json`) |
| `User-Agent` | Realistic desktop UA — align with **`internal/collectors/utils`** defaults. |

CORS response headers (`Access-Control-*`) matter for browsers only; **not** required for server-side `net/http`.

### CSRF (Django)

1. **`GET`** listing → response sets cookie **`csrftoken`** (and delivers HTML with hidden **`csrfmiddlewaretoken`**).  
2. **`POST`** `xhr-load` → send **Cookie** `csrftoken` **and** body field **`csrfmiddlewaretoken`** matching the hidden input from step 1.

---

## Collector behaviour (this source — product defaults)

Documented here so implementation matches ingest expectations; env keys belong in **`contracts/environment.md`** when wired.

| Rule | Value |
| ---- | ----- |
| Max jobs per `Fetch` | **100** — stop collecting once 100 `domain.Job` rows are built (after detail fetches), even if `last` is still false. |
| Politeness delay | **Non-zero delay** between consecutive HTTP calls (listing `GET`, each `xhr-load`, each detail `GET`), e.g. hundreds of ms — exact default and knob TBD in config; goal is to avoid hammering `jobs.dou.ua` / Cloudflare. |
| Pagination | Merge cards from initial `GET` and all `xhr-load` **`html`** fragments; **dedupe by vacancy URL**; advance **`count`** as observed in browser; stop when **`last`** is true **or** max jobs reached. |

---

## Interface: `ListingCard` (one `li.l-vacancy` on list / in `html` fragment)

Strings are **trimmed** unless noted.

| Field | Type | Where in HTML |
| ----- | ---- | --------------- |
| `posted_display` | string | `div.date` — human date (Ukrainian), e.g. `9 квітня` |
| `title` | string | `div.title a.vt` text |
| `job_page_url` | string | `div.title a.vt` `@href` — absolute URL to detail |
| `company_name` | string | `div.title strong a.company` text |
| `company_listing_url` | string (optional) | `a.company` `@href` |
| `location_raw` | string (optional) | `span.cities` or related geo span in `div.title` |
| `preview_text` | string (optional) | `div.sh-info` — short preview; **not** full description |

**Stable vacancy id**: numeric segment in path — `/vacancies/353313/` → id `353313` for dedup / stable id helpers (together with source string in **`contracts/collector.md`** when assigned).

---

## Interface: `JobDetail` (detail page)

Primary container: **`div.l-vacancy`** with **`itemprop="description"`** (schema.org).

| Field | Type | Where in HTML |
| ----- | ---- | --------------- |
| `posted_display` | string | `div.date` — may include year, e.g. `8 квітня 2026` |
| `title` | string | `h1.g-h2` |
| `location_raw` | string | `div.sh-info span.place` |
| `salary_raw` | string (optional) | `div.sh-info span.salary` when present |
| `description_html` | string | `div.b-typo.vacancy-section` — main body (normalize to plain text per **`domain-mapping-mvp.md`**) |
| `badge_links` | n/a (optional) | `a.badge` in date row — category/topic links; map to tags only if product agrees |

---

## Pagination vs UI

The site shows **“Більше вакансій”** (load more); that triggers **`POST xhr-load`** with the **`count`** / **`csrfmiddlewaretoken`** flow above. The collector **repeats that HTTP sequence** — **no headless browser** for this wire.

---

## Relative / locale dates

Listing and detail use **Ukrainian** month names and formats. Normalization to `PostedAt` is **normative** in **`../contracts/domain-mapping-mvp.md`** (parser strategy, strict vs soft failure). Extend implementation + **`contracts/test-fixtures.md`** as new phrases appear.

---

## Related

- `../contracts/sources-inventory.md`
- `../contracts/collector.md` — `Job.Source` string when this collector is added
- `../contracts/domain-mapping-mvp.md`
- `../contracts/test-fixtures.md`
- `europe-remotely.md` — analogous “JSON + HTML fragment + detail GET” pattern
