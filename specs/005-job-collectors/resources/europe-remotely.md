# Europe Remotely — extractable data (HTML / wire)

**Site**: [euremotejobs.com](https://euremotejobs.com/)  
**Inventory**: MVP row 1 in `../contracts/sources-inventory.md`

This document is the **interface of what the site exposes** that we can pull from responses and DOM. **No** mapping to `domain.Job` here — that is decided later.

---

## How data arrives (transport)


| Piece                | Mechanism                                                 | Notes                                                                                                           |
| -------------------- | --------------------------------------------------------- | --------------------------------------------------------------------------------------------------------------- |
| Feed continuation    | `POST` `https://euremotejobs.com/wp-admin/admin-ajax.php` | Body: same fields as in browser **Network** (e.g. `action`, filters, page) — capture from DevTools              |
| Feed JSON (observed) | JSON payload                                              | Contains **`has_more`** (`bool`) and **`html`** (`string`): HTML **fragment** of job cards, not a full document |
| Job detail           | `GET` job page URL on `euremotejobs.com`                  | Full HTML document                                                                                              |


**Tier**: T2 — `net/http` + goquery, no headless for this source.

### Pagination vs UI

The site shows controls such as **“LOAD MORE JOBS”** / **“Show more jobs”**; those buttons trigger the same **`POST admin-ajax.php`** flow documented here. The collector **repeats that HTTP request** (with captured body fields for `action`, page/filters, etc.) — **no headless browser** for MVP.

Pagination: follow **`has_more`**; merge with any cards already in the initial page HTML and **dedupe by job page URL**.

### Relative “posted” times

Listing and detail often show **relative** English phrases (e.g. `Posted 12 hours ago`, `2 weeks ago`). Normalization rules (anchor **`time.Now().UTC()`**, parser strategy, soft failure → zero `PostedAt` + warn) are **normative** in **`../contracts/domain-mapping-mvp.md`**. Add newly seen phrases to implementation + tests as the site copy changes.

Record a **real captured `admin-ajax.php` request body** (from DevTools → Copy as cURL / Copy request payload) in this file when available so implementers do not guess WordPress `action` / nonce fields.

---

## Interface: `FeedBatch` (one AJAX success)

Logical shape after parsing JSON (exact key names may match WP — confirm in Network):


| Field      | Type   | Meaning                                                           |
| ---------- | ------ | ----------------------------------------------------------------- |
| `has_more` | bool   | More pages available via same endpoint                            |
| `html`     | string | Concatenated listing card markup (parse with goquery as fragment) |


---

## Interface: `ListingCard` (one `.job-card`)

One row in the feed. All string fields are **trimmed text** unless noted.


| Field               | Type   | Present             | Where in HTML                                                                                                        |
| ------------------- | ------ | ------------------- | -------------------------------------------------------------------------------------------------------------------- |
| `title`             | string | usually             | `h2.job-title`                                                                                                       |
| `company_name`      | string | usually             | `div.company-name`                                                                                                   |
| `location_raw`      | string | often               | `div.meta-item.meta-location` — free text, may be **comma-separated** regions                                        |
| `employment_type`   | string | often               | `div.meta-item.meta-type` (e.g. “Full Time”)                                                                         |
| `experience_level`  | string | optional            | `div.meta-item.meta-level`                                                                                           |
| `category`          | string | optional            | `div.meta-item.meta-category`                                                                                        |
| `posted_display`    | string | often               | `div.job-time` — often **relative** (“Posted 5 days ago”)                                                            |
| `compensation_raw`  | string | optional            | `div.meta-item` **without** `meta-`* class — **opaque** (e.g. salary range, “$5500 net/month”); do not parse in spec |
| `logo_url`          | string | often               | `img.company_logo` `@src` — real asset or WP Job Manager placeholder path                                            |
| `logo_alt`          | string | optional            | `img.company_logo` `@alt` — sometimes company name                                                                   |
| `high_salary_badge` | bool   | optional            | `true` if root has class `has-high-salary` / presence of `high-salary-badge`                                         |
| `job_page_url`      | string | required for ingest | Absolute URL to the **job listing page** on this site — from the card link (wrapper: confirm in fixture)             |


---

## Interface: `JobDetail` (one GET job page)

Header block: `div.page-header` — `h1.page-title` + `h3.page-subtitle` > `ul.job-listing-meta.meta`.


| Field                    | Type     | Present  | Where in HTML                                                                                                                                                                   |
| ------------------------ | -------- | -------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `title`                  | string   | usually  | `h1.page-title`                                                                                                                                                                 |
| `job_type_entries`       | []string | optional | **Every** `ul.job-listing-meta li.job-type` — text only, in DOM order. Examples: `"Full Time"`; `"high salary"` from `li.job-type.high-salary` (tag/badge, not a salary figure) |
| `high_salary_in_meta`    | bool     | optional | `true` if any `li.job-type` has class `high-salary`                                                                                                                             |
| `location_raw`           | string   | optional | `ul.job-listing-meta li.location` — prefer link text of `a.google_map_link`, else whole `li` text                                                                               |
| `location_map_query_url` | string   | optional | `a.google_map_link` `@href` if present (Google Maps query URL)                                                                                                                  |
| `posted_display`         | string   | optional | `ul.job-listing-meta li.date-posted`                                                                                                                                            |
| `compensation_meta_raw`  | string   | optional | `ul.job-listing-meta li.wpjmef-field-salary` — **opaque** full line (e.g. `Salary: 180,000 USD/year`); plugin-specific class, may be absent                                     |
| `company_name`           | string   | optional | `ul.job-listing-meta li.job-company` — link text or `li` text                                                                                                                   |
| `company_page_url`       | string   | optional | `li.job-company a` `@href` (company directory on same site)                                                                                                                     |
| `description_inner_html` | string   | usually  | Inside `div.job_listing-description` — overview, headings, lists (keep or strip tags later)                                                                                     |
| `tags_raw`               | string   | optional | `p.job_tags` — e.g. “Tagged as: …”                                                                                                                                              |
| `apply_url`              | string   | optional | `a.application_button_link` `@href` — often external ATS                                                                                                                        |
| `company_website_url`    | string   | optional | Company social widget, e.g. link labeled Website                                                                                                                                |
| `category_url`           | string   | optional | `div.job_listing-categories a.job-category` `@href`                                                                                                                             |
| `category_label`         | string   | optional | same node text                                                                                                                                                                  |
| `logo_url`               | string   | optional | Sidebar `img.company_logo` `@src` (may duplicate placeholder)                                                                                                                   |


`ListingCard.compensation_raw` and `JobDetail.compensation_meta_raw` can both exist on the same job (same or different wording); both stay **opaque strings** until mapping rules exist.

---

## Risks

- Theme / WP Job Manager / AJAX `action` or body shape can change without notice.
- `admin-ajax.php` is not a public API — test against **saved HTML fixtures**.

## Related

- `../spec.md`
- `../contracts/sources-inventory.md`
- `../contracts/domain-mapping-mvp.md` — → `domain.Job`
- `../contracts/test-fixtures.md` — sample AJAX + HTML excerpts

