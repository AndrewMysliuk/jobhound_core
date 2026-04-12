# Built In — remote jobs (HTML + JSON-LD)

**Site**: [builtin.com/jobs/remote](https://builtin.com/jobs/remote)  
**Inventory**: Row 6 (**MVP**, before LinkedIn) in `../contracts/sources-inventory.md`

Normative mapping to **`domain.Job`**: **`../contracts/domain-mapping-mvp.md`** § Built In.

**Status**: Shipped — **`../tasks.md`** § L.

---

## Tier

**T2 (fact)** — `net/http` + **`encoding/json`** on embedded **`application/ld+json`** (listing **`ItemList`**, detail **`JobPosting`**). **goquery** is optional for discovery only; normative fields come from JSON-LD on listing + detail HTML responses. No headless unless a future spike proves JS-only delivery.

**Open:** Production/worker egress sometimes sees **HTTP 403** with Cloudflare challenge HTML (`Just a moment...`) instead of real pages — **follow-up** to resolve (see **`../spec.md`** § Follow-ups, **`../tasks.md`** § L.8).

---

## Product scope (geography)

- **EU-27** + **United Kingdom** + **Ukraine** only (**29** territories). **Not** in scope: Russia, Belarus, or other non-listed states.
- Wire uses **ISO 3166-1 alpha-3** in the **`country`** query parameter; **`domain.Job.CountryCode`** is always **alpha-2** (see mapping table below).

---

## Search-required / empty slot

- When **`SlotSearchQuery`** is **empty** (after trim), the collector performs **no HTTP** and returns an **empty** job slice (**`nil` error**). The same applies to **`Fetch`** when the board is driven only by slot search — see **`contracts/collector.md`** Built In exception.
- Non-empty **`SlotSearchQuery`** maps to listing query parameter **`search`** (URL-encoded).

---

## Politeness

- Apply an **inter-request delay** between **every** consecutive HTTP call (each listing page per country, each job detail). Default and env name: **`contracts/environment.md`** (`JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS`, default **`300`** ms; **`0`** disables — tests / local only).
- A full run can require **many** requests (see **Volume** below); operators should size timeouts and schedules accordingly.

---

## Listing (per country)

### URL

`GET` `https://builtin.com/jobs/remote`

### Query parameters (normative)

| Param | Value | Meaning |
| ----- | ----- | ------- |
| `country` | ISO 3166-1 **alpha-3** (e.g. `ROU`) | Built In’s remote filter for that territory. |
| `allLocations` | **`true`** | Required fixed flag (observed site contract). |
| `search` | non-empty string | From product **`SlotSearchQuery`** / **`FetchWithSlotSearch`**. |
| `page` | integer ≥ `1` | Pagination; **at most two** pages per country per run (see **Caps**). |

Example:

```text
https://builtin.com/jobs/remote?search=frontend&country=ROU&allLocations=true&page=1
```

### Listing data source

Parse the HTML response for **`script[type="application/ld+json"]`** blocks. Decode JSON; walk **`@graph`** (or top-level object) for an **`@type`** **`ItemList`** entry.

- Use **`itemListElement`** (`@type` **`ListItem`**): read **`url`** (absolute `https://builtin.com/job/{slug}/{numeric-id}`), **`name`**, and optional **`description`** (snippet only — **not** used for final **`domain.Job`** fields except as optional hints; **normative job fields come from detail **`JobPosting`**).

### Pagination and caps

- Built In serves **up to 20** jobs per listing page when more exist; fewer means the end of results for that country/page combination.
- Per **country**, fetch **`page=1`** and **`page=2`** only (**max 40** job URLs per country before dedup).
- If **`page=1`** returns **zero** items, do not request **`page=2`**. If **`page=1`** returns **&lt; 20** items, **skip** **`page=2`** (no further results for that country).
- **Empty** listing for a country is **normal** — continue to the next country.

### Dedup

- After collecting URLs across countries and pages, **deduplicate** by **canonical job URL** (or equivalent stable id from path) **before** issuing detail **`GET`s** so each vacancy is fetched at most once per run.
- The same logical job may appear under multiple country filters; dedup prevents redundant detail traffic.

---

## Detail

### URL

Absolute URL from listing **`ListItem.url`** (pattern `https://builtin.com/job/{slug}/{id}`).

### Data source

Parse **`application/ld+json`**; find **`@graph`** element with **`@type`** **`JobPosting`** (same pattern as **`resources/djinni.md`** detail).

All **normative** **`domain.Job`** fields for Built In are derived from this **`JobPosting`** (plus **fixed `Source`**, **`CountryCode`** rule, and **`StableJobID`** — see **`domain-mapping-mvp.md`**).

---

## Alpha-3 → alpha-2 (normative set)

Use **only** this table for **`country=`** values. **`Job.CountryCode`** for jobs discovered under that listing request = the **alpha-2** column.

| alpha-3 | alpha-2 |
| ------- | ------- |
| AUT | AT |
| BEL | BE |
| BGR | BG |
| HRV | HR |
| CYP | CY |
| CZE | CZ |
| DEU | DE |
| DNK | DK |
| EST | EE |
| ESP | ES |
| FIN | FI |
| FRA | FR |
| GRC | GR |
| HUN | HU |
| IRL | IE |
| ITA | IT |
| LVA | LV |
| LTU | LT |
| LUX | LU |
| MLT | MT |
| NLD | NL |
| POL | PL |
| PRT | PT |
| ROU | RO |
| SVK | SK |
| SVN | SI |
| SWE | SE |
| GBR | GB |
| UKR | UA |

---

## Volume (order-of-magnitude)

Per non-empty slot search: up to **29** countries × **2** listing pages + up to **29 × 40** detail **`GET`s** before cross-country dedup (typically fewer after dedup and early stop when **&lt; 20** on page 1). **Inter-request delay** applies between **all** sequential requests.

---

## Related

- `../contracts/collector.md` — **`Job.Source`** **`builtin`**, slot / **`Fetch`** exception
- `../contracts/environment.md` — delay env
- `../contracts/test-fixtures.md` — offline samples
