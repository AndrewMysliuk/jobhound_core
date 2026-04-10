# Djinni — extractable data (HTML / JSON-LD)

**Site**: [djinni.co/jobs](https://djinni.co/jobs/)  
**Inventory**: Row 5 (**MVP**) in `../contracts/sources-inventory.md`

Normative mapping to **`domain.Job`**: **`../contracts/domain-mapping-mvp.md`**.

**Status**: Shipped collector (`internal/collectors/djinni`); wire captured from page source (2026-04) — adjust if markup or JSON-LD shape changes.

---

## Tier

**T2 (fact)** — `net/http` + **goquery** on listing HTML; **`encoding/json`** for **`application/ld+json`** (listing may embed **many** `JobPosting` objects; detail page embeds **one**). No headless unless a future spike proves JS-only delivery.

**Politeness**: apply an **inter-request delay** between consecutive HTTP calls (listing pages and each job detail), same idea as DOU.ua — see **`contracts/environment.md`** (`JOBHOUND_COLLECTOR_DJINNI_INTER_REQUEST_DELAY_MS`).

---

## Listing (search + pagination)

### URL

`GET` `https://djinni.co/jobs/`

### Query parameters (normative for slot / full-text search)

| Param | Value | Meaning |
| ----- | ----- | ------- |
| `all_keywords` | non-empty string (URL-encoded) | Maps to product **`SlotSearchQuery`** / stage-1 string. |
| `search_type` | **`full-text`** | Default for programmatic search (explicit in requests). |
| `page` | integer ≥ `1` | Page index. |

Example:

```text
https://djinni.co/jobs/?all_keywords=frontend&search_type=full-text&page=1
```

### Pagination

- **~15** job rows per page (treat **&lt; 15** cards on a page as **last** page).
- Increment **`page`** until the listing has no further job links or count **&lt; 15**.

### Stable job identity

Canonical listing URL pattern:

```text
https://djinni.co/jobs/{numeric-id}-{slug}/
```

The numeric id appears in **`JobPosting.identifier`** in JSON-LD and in the path segment before the first `-` in the slug part.

---

## Listing row HTML (goquery hints)

Observed structure for each job card (embedded in list view). **Prefer detail page + JSON-LD** for normalized fields; listing selectors help discover **URLs** and optional **hints** (remote line, location, preview salary text).

| Data | Selector / note |
| ---- | ---------------- |
| Title | `h2.job-item__position` |
| Company (preview) | Adjacent block: `span.small.text-gray-800.opacity-75.font-weight-500` (first company name span in the header row). |
| Salary (preview only) | `div.col-auto div.fs-5 strong.text-success` — human text (e.g. `до $650`); may disagree slightly from JSON-LD; **authoritative salary for `SalaryRaw`** is **`baseSalary`** on detail JSON-LD when present. |
| Remote / location line | `div.fw-medium` row: `span.text-nowrap` tokens (e.g. Ukrainian **«Тільки віддалено»**), **`span.location-text`** for region/country text, middot-separated experience and language lines. |
| Tags (preview) | `div.job-item__tags span.badge` |
| Description preview | `div[id^="job-description-"] .js-truncated-text` (plain preview); full HTML may exist in sibling **`.js-original-text`** (often `d-none` until expanded). **Normative long description** for ingest: **detail** `GET` + JSON-LD **`description`** (or strip HTML from `.js-original-text` if product ever optimizes away N detail fetches). |
| Link to detail | Anchor wrapping the card header: **`href`** matching **`/jobs/\d+-`** — resolve to absolute `https://djinni.co` + path. |

---

## JSON-LD on listing page

The listing HTML may include **one** `<script type="application/ld+json">` whose text is a **JSON array** of objects, each **`@type": "JobPosting"`**.

- Each object includes at least: **`identifier`**, **`url`**, **`title`**, **`description`**, **`datePosted`**, **`hiringOrganization`**, **`category`**, optional **`baseSalary`** (`MonetaryAmount` with **`currency`**, **`value.minValue`** / **`value.maxValue`**, **`value.unitText`** e.g. **`MONTH`**), **`jobLocationType`** (e.g. **`TELECOMMUTE`**), **`applicantLocationRequirements`**, **`jobLocation`**, etc.
- Implementations may use this array to **validate** listing parse, **skip detail** for experiments, or **merge** with detail fetches; **MVP mapping** in **`domain-mapping-mvp.md`** assumes **detail `GET`** is the primary source for **`Description`** and stable normalization.

---

## Job detail page

`GET` canonical job URL — returns HTML with **one** primary `JobPosting` block in `application/ld+json` (same field families as array elements on the listing).

Use **`url`** from JSON-LD (or request URL) as **`Job.URL`** after canonicalization (strip query/fragment).

---

## `baseSalary` → opaque `SalaryRaw`

When present, `baseSalary` is **`MonetaryAmount`**:

- **`currency`** (e.g. `USD`)
- **`value`**: **`QuantitativeValue`** with **`minValue`**, **`maxValue`**, **`unitText`** (`MONTH`, etc.)

Build **`SalaryRaw`** as a single human-readable string (e.g. `2600–3000 USD / month` or `550–650 USD / month`). If **`minValue` == `maxValue`**, a single-amount string is fine. If `baseSalary` is absent, fall back to **listing** salary preview text from **`strong.text-success`** when available, else **`""`**.

---

## Apply URL

Public pages emphasize **login to apply** on Djinni. **`ApplyURL`**: use **`""`** unless a future spike finds a stable external apply link on the detail page.

---

## Related

- `../contracts/sources-inventory.md`
- `../contracts/domain-mapping-mvp.md`
- `../contracts/collector.md` — `Job.Source` **`djinni`**, **`FetchWithSlotSearch`**
- `../contracts/environment.md`
