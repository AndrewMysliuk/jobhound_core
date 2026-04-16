# Built In — remote jobs (HTML + JSON-LD)

**Site**: [builtin.com/jobs/remote](https://builtin.com/jobs/remote)  
**Inventory**: Row 6 (**MVP**, before LinkedIn) in `../contracts/sources-inventory.md`

Normative mapping to **`domain.Job`**: **`../contracts/domain-mapping-mvp.md`** § Built In.

**Status**: Shipped — **`../tasks.md`** § L.

---

## Tier

**Parsing (always):** **`encoding/json`** on embedded **`application/ld+json`** (listing **`ItemList`**, detail **`JobPosting`**). **goquery** is optional for discovery only; normative fields come from JSON-LD.

**Transport (how HTML is obtained):**

| Mode | Mechanism | When |
| ---- | --------- | ---- |
| **T2 (fact, default)** | **`net/http`** `GET` listing + `GET` each detail | Normal operation; same URLs as the public site. |
| **T3 (optional)** | Shared **`internal/collectors/browserfetch`** (go-rod): **URL → HTML** after headless navigation | When plain HTTP returns **403** / Cloudflare interstitial (`Just a moment...`) or similar; **same URLs** as T2 — only the client changes. |

**Operational note:** T3 requires **Chromium/Chrome** on the host and config per **`../contracts/environment.md`** / **`../contracts/browser-fetch.md`**. **LinkedIn** (planned) will reuse **`browserfetch`**; Built In does not own a one-off Rod fork.

### Failure mode: HTTP 403 and challenge HTML (Cloudflare)

- **`net/http`** may receive **403 Forbidden**, or **200** with an **anti-bot / “Just a moment…”** interstitial instead of pages that contain **`application/ld+json`** with **`ItemList`** / **`JobPosting`**.
- **Challenge HTML is not a listing:** body often references Cloudflare / **`cf-browser-verification`**, **`challenge-platform`**, or similar; JSON-LD job blocks are missing, so parsing fails or yields zero jobs — distinct from a real listing with an empty result set.
- **Mitigation:** enable T3 — **`JOBHOUND_BROWSER_ENABLED=1`** (see **`../contracts/environment.md`**) so **`bootstrap`** injects **`browserfetch.RodFetcher`** into **`builtin.BuiltIn`**; **same URLs** as T2. Use **`JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER=0`** only if you must force **`net/http`** for Built In while testing.

**Tracking:** Shipped — **`../tasks.md`** § **L.8**, § **M**.

---

## Product scope (geography)

- **Subset of EU** (large remote / IT hiring markets) + **United Kingdom** + **Ukraine** — **18** territories in fixed listing order (`DEU`, `NLD`, `POL`, `FRA`, `ESP`, `ITA`, `IRL`, `SWE`, `BEL`, `AUT`, `CZE`, `PRT`, `ROU`, `GRC`, `FIN`, `DNK`, `HUN`, then **`GBR`**, **`UKR`**). **Not** in scope: Russia, Belarus, or other non-listed states.
- Wire uses **ISO 3166-1 alpha-3** in the **`country`** query parameter; **`domain.Job.CountryCode`** is always **alpha-2** (see mapping table below).

---

## Search-required / empty slot

- When **`SlotSearchQuery`** is **empty** (after trim), the collector performs **no HTTP** and returns an **empty** job slice (**`nil` error**). The same applies to **`Fetch`** when the board is driven only by slot search — see **`contracts/collector.md`** Built In exception.
- Non-empty **`SlotSearchQuery`** maps to listing query parameter **`search`** (URL-encoded).

---

## Politeness

- Apply an **inter-request delay** between **every** consecutive **document fetch** (listing page per country, each job detail) — whether **T2** or **T3**. Default and env name: **`contracts/environment.md`** (`JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS`, default **`1000`** ms; **`0`** disables — tests / local only).
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
| `page` | integer ≥ `1` | Pagination; **default one** listing page per country per run; **at most two** when explicitly configured (e.g. debug **`builtin_max_listing_pages_per_country`**) — see **Caps**. |

Example:

```text
https://builtin.com/jobs/remote?search=frontend&country=ROU&allLocations=true&page=1
```

### Listing data source

Parse the HTML response for **`script[type="application/ld+json"]`** blocks. Decode JSON; walk **`@graph`** (or top-level object) for an **`@type`** **`ItemList`** entry.

- Use **`itemListElement`** (`@type` **`ListItem`**): read **`url`** (absolute `https://builtin.com/job/{slug}/{numeric-id}`), **`name`**, and optional **`description`** (snippet only — **not** used for final **`domain.Job`** fields except as optional hints; **normative job fields come from detail **`JobPosting`**).

### Pagination and caps

- Built In serves **up to 20** jobs per listing page when more exist; fewer means the end of results for that country/page combination.
- Per **country**, fetch **`page=1`** by default (**max 20** job URLs from listings per country before dedup). A **second** listing page (**`page=2`**) is **optional** and only used when **`MaxListingPagesPerCountry`** is set to **2** (e.g. local debug **`builtin_max_listing_pages_per_country`**); production default is **1** page to reduce origin traffic.
- If **`page=1`** returns **zero** items, do not request **`page=2`**. When a second page is enabled: if **`page=1`** returns **&lt; 20** items, **skip** **`page=2`** (no further results for that country).
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

Use **only** this table for **`country=`** values (matches the **18**-territory scope above). **`Job.CountryCode`** for jobs discovered under that listing request = the **alpha-2** column.

| alpha-3 | alpha-2 |
| ------- | ------- |
| DEU | DE |
| NLD | NL |
| POL | PL |
| FRA | FR |
| ESP | ES |
| ITA | IT |
| IRL | IE |
| SWE | SE |
| BEL | BE |
| AUT | AT |
| CZE | CZ |
| PRT | PT |
| ROU | RO |
| GRC | GR |
| FIN | FI |
| DNK | DK |
| HUN | HU |
| GBR | GB |
| UKR | UA |

---

## Volume (order-of-magnitude)

Per non-empty slot search: up to **18** countries × **1** listing page (default) + up to **18 × 20** detail **`GET`s** before cross-country dedup (typically fewer after dedup). With **`MaxListingPagesPerCountry` = 2**, up to **18 × 2** listing pages and up to **18 × 40** detail **`GET`s** before dedup (early stop when **&lt; 20** on page 1). **Inter-request delay** applies between **all** sequential requests.

### Per-request failures (listing / detail)

- Transient or data issues on a **single** listing page or detail document are **skipped** with a **warning** log; the run **continues** and returns whatever jobs were successfully parsed (possibly an empty slice). **Misconfiguration** (e.g. **`UseBrowser`** with no **`HTMLDocumentFetcher`**) still fails the whole fetch immediately.

---

## Related

- `../contracts/collector.md` — **`Job.Source`** **`builtin`**, slot / **`Fetch`** exception
- `../contracts/environment.md` — delay env; optional browser / T3 knobs when implemented
- `../contracts/browser-fetch.md` — shared Tier-3 document fetch (Built In + future LinkedIn)
- `../contracts/test-fixtures.md` — offline samples
