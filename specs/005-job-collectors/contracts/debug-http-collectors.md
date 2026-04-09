# Contract: local debug HTTP (`POST /debug/collectors/*`)

**Spec**: `005-job-collectors`  
**Implementation**: `internal/collectors/handlers/debughttp`

Local-only JSON body for **`POST /debug/collectors/europe_remotely`**, **`POST /debug/collectors/working_nomads`**, and **`POST /debug/collectors/dou_ua`**.  
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
};
```

`query`, `sort`, `page_size`, and `_source` are **ignored** on `europe_remotely` and `dou_ua`.  
`feed_form` and `search_keywords` are **ignored** on `working_nomads` and `dou_ua`.  
`search` and `dou_inter_request_delay_ms` are **ignored** on `europe_remotely` and `working_nomads`.

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

---

## Related

- `../spec.md` — Temporary debug HTTP
- `../resources/working-nomads.md` — site `jobsapi/_search` wire notes
