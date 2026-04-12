# Contract: local debug HTTP (`POST /debug/collectors/*`)

**Spec**: `005-job-collectors`  
**Implementation**: `internal/collectors/handlers/debughttp`

Local-only JSON body for **`POST /debug/collectors/europe_remotely`**, **`POST /debug/collectors/working_nomads`**, **`POST /debug/collectors/dou_ua`**, **`POST /debug/collectors/himalayas`**, **`POST /debug/collectors/djinni`**, and **`POST /debug/collectors/builtin`**.  
**`Content-Type: application/json`**. No URL query parameters for these options.

---

## Request body shape (TypeScript-style)

```ts
type DebugCollectorsRequest = {
  /**
   * Max jobs in the response.
   * Omit → 200. `0` = unlimited (full run). Integer 1…10000.
   * With `cmd/agent` and the real Europe Remotely collector, this value also stops the crawl after N jobs (no full-site fetch when you only need a sample).
   */
  limit?: number;

  /** Working Nomads only: Elasticsearch `query` object sent to `jobsapi/_search`. Omitted → default `match_all` in collector. */
  query?: object;

  /** Working Nomads only: Elasticsearch `sort` array. Omitted → collector default. */
  sort?: unknown[];

  /** Working Nomads only: maps to ES request `size` (page size). Positive integer; omitted → 100 in collector. */
  page_size?: number;

  /** Working Nomads only: ES `_source` field list. Omitted → collector default list (must include fields the parser needs, e.g. `slug`, `pub_date`). */
  _source?: string[];

  /**
   * Europe Remotely only: extra form fields merged into the feed POST (after bootstrap `action` / `nonce` / `website`).
   * Copy key names from DevTools → `admin-ajax.php` (e.g. future date filters). Ignored on `working_nomads`.
   */
  feed_form?: Record<string, string>;

  /**
   * Europe Remotely only: form field `search_keywords`. Applied after `feed_form` and overrides the same key.
   * Ignored on `working_nomads` and `dou_ua`.
   */
  search_keywords?: string;

  /**
   * DOU.ua only: listing / xhr-load query param `search` (see `resources/dou.md`).
   * Ignored on `europe_remotely` and `working_nomads`.
   */
  search?: string;

  /**
   * DOU.ua only: inter-request delay override in milliseconds (default from `JOBHOUND_COLLECTOR_DOU_INTER_REQUEST_DELAY_MS`).
   * Ignored on other routes.
   */
  dou_inter_request_delay_ms?: number;

  /** Himalayas only (when collector ships): free-text `q` for `/jobs/api/search`. */
  q?: string;

  /** Himalayas only: 1-based page for search mode. */
  page?: number;

  /**
   * Himalayas only: when true, use `/jobs/api/search` with `q` / `page`;
   * when false or omitted, use browse `/jobs/api` with offset/limit only (collector default).
   */
  use_search?: boolean;

  /** Himalayas only: stop after N API pages in browse or search mode (implementation-defined default). */
  max_pages?: number;

  /** Djinni only (when route exists): maps to listing `all_keywords` (`search_type=full-text` fixed in collector). */
  all_keywords?: string;

  /** Djinni only: 1-based listing `page=` query param; omitted → `1`. (Distinct from Himalayas `page`.) */
  djinni_page?: number;

  /**
   * Djinni only: inter-request delay override in milliseconds (default from `JOBHOUND_COLLECTOR_DJINNI_INTER_REQUEST_DELAY_MS`).
   * Ignored on other routes.
   */
  djinni_inter_request_delay_ms?: number;

  /** Built In only: maps to remote listing query param `search` (slot keyword). Empty or omitted → collector returns no jobs (same as production empty slot). */
  builtin_search?: string;

  /** Built In only: inter-request delay override in milliseconds (default from `JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS`). Ignored on other routes. */
  builtin_inter_request_delay_ms?: number;

  /** Built In only: max listing pages **per country** (1 or 2 per `resources/builtin.md`). Omitted → production default (`1`). */
  builtin_max_listing_pages_per_country?: number;

  /**
   * Built In only: when true/false, overrides **`UseBrowser`** for this debug request (rod vs `net/http`).
   * Requires the agent to have been started with a rod fetcher (**`JOBHOUND_BROWSER_ENABLED`**) when set to `true`; otherwise the collector returns an error.
   */
  builtin_use_browser?: boolean;
};
```

`query`, `sort`, `page_size`, and `_source` are **ignored** on `europe_remotely`, `dou_ua`, `himalayas`, and `djinni`.  
`feed_form` and `search_keywords` are **ignored** on `working_nomads`, `dou_ua`, `himalayas`, and `djinni`.  
`search` and `dou_inter_request_delay_ms` are **ignored** on `europe_remotely`, `working_nomads`, `himalayas`, and `djinni`.  
`q`, `use_search`, and `max_pages` are **only** read for `himalayas` (and `page` for Himalayas search mode).  
`all_keywords`, `djinni_inter_request_delay_ms`, and **`djinni_page`** are **only** read for `djinni` when wired.  
`builtin_search`, `builtin_inter_request_delay_ms`, **`builtin_max_listing_pages_per_country`**, and **`builtin_use_browser`** are **only** read for **`builtin`**.  
`europe_remotely`, `working_nomads`, and `dou_ua` ignore Himalayas, Djinni-only, and Built In–only keys. See **`../tasks.md`** § J, § K, and § L.

**`limit`** on **`dou_ua`**: when a concrete `*dou.DOU` is wired, maps to **`MaxJobs`** for that request (early stop). Omitted default `200` still applies to the JSON response cap for stub collectors; see **`../spec.md`**.

### Europe Remotely: keywords + ad hoc filters

```json
{
  "limit": 25,
  "search_keywords": "vue"
}
```

Date or other filters: copy exact field names from DevTools → Network → `admin-ajax.php` request payload into `feed_form` (values are strings). If the site does not expose a date filter on that endpoint, there is nothing to send.

---

## Date filtering (Working Nomads)

Indexed publication time is **`pub_date`** (ISO-8601 strings in documents). On the **site** side, filter with a normal Elasticsearch **`range`** query on `pub_date`, optionally combined with **`bool`** / **`must`** next to `match`:

- **`gte`**, **`lte`**, **`gt`**, **`lt`**: strings in ISO-8601 (e.g. `"2026-03-01T00:00:00Z"`) — same style as in `resources/working-nomads.md`.

Exact clause shapes depend on their index mapping; if something returns an error, copy a working **`query`** from browser DevTools (Network → `_search` request).

---

## Examples

### Title + date window + pagination cap

```json
{
  "limit": 80,
  "page_size": 25,
  "query": {
    "bool": {
      "must": [
        { "match": { "title": "frontend" } },
        {
          "range": {
            "pub_date": {
              "gte": "2026-03-01T00:00:00Z",
              "lte": "2026-03-30T23:59:59Z"
            }
          }
        }
      ]
    }
  }
}
```

### Only date range (no title match)

```json
{
  "limit": 50,
  "query": {
    "range": {
      "pub_date": {
        "gte": "2026-03-01T00:00:00Z"
      }
    }
  }
}
```

### DOU.ua — search + sample cap

```json
{
  "limit": 25,
  "search": "frontend",
  "dou_inter_request_delay_ms": 500
}
```

### Himalayas — search sample (when route exists)

```json
{
  "limit": 40,
  "use_search": true,
  "q": "vue",
  "page": 1,
  "max_pages": 2
}
```

### Djinni — listing + delay (when route exists)

```json
{
  "limit": 30,
  "all_keywords": "frontend",
  "djinni_page": 1,
  "djinni_inter_request_delay_ms": 500
}
```

### Built In — slot search + delay

```json
{
  "limit": 50,
  "builtin_search": "frontend",
  "builtin_inter_request_delay_ms": 500,
  "builtin_max_listing_pages_per_country": 2
}
```

---

## Related

- `../spec.md` — Temporary debug HTTP
- `../resources/working-nomads.md` — site `jobsapi/_search` wire notes
- `../resources/himalayas.md` — public `jobs/api` wire
- `../resources/builtin.md` — remote listing + JSON-LD
