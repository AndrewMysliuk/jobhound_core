# Contract: collector environment (T2 + Tier 3 browser)

**Spec**: `005-job-collectors`  
**Last Updated**: 2026-04-17  
**Status**: Draft

**Tier 3 — shared document fetch:** **`internal/collectors/browserfetch`** (go-rod) loads **URL → HTML** for collectors that opt in (**Built In** first; **LinkedIn** reuses the same module — see **`browser-fetch.md`**). **Per-source** session/cookies (e.g. LinkedIn) are **not** defined in this section until that collector ships; they live beside the source collector + `internal/config`.

### Shared browser / Rod (`browser-fetch.md`, `internal/config/collectors_browser.go`)

| Variable | Default (when unset) | Meaning |
| -------- | --------------------- | ------- |
| `JOBHOUND_BROWSER_ENABLED` | **on** (unset) | **Default:** Tier-3 is **enabled** — **`internal/collectors/bootstrap`** launches a shared Chromium (go-rod) and wires **`browserfetch.HTMLDocumentFetcher`** into Built In when **`JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER`** is on (see **Built In** table below). Set to `0`, `false`, `no`, or `off` to **disable** the browser (e.g. no local Chrome, CI, or forcing HTTP-only). Explicit `1` / `true` / `yes` / `on` keeps it enabled. |
| `JOBHOUND_BROWSER_BIN` | (unset) | Optional path to Chromium/Chrome; empty uses go-rod launcher defaults (may download a revision). |
| `JOBHOUND_BROWSER_USER_DATA_DIR` | (unset) | Optional persistent Chromium user-data directory; empty uses launcher temp profile. |
| `JOBHOUND_BROWSER_NAV_TIMEOUT_MS` | `120000` | Per-URL navigation + load + HTML extraction timeout for **`browserfetch.RodFetcher.FetchHTMLDocument`**. |
| `JOBHOUND_BROWSER_NO_SANDBOX` | (unset / off) | When `1`/`true`/`yes`/`on`, launches Chromium with **`--no-sandbox`** and **`--disable-dev-shm-usage`** (via go-rod). **Required** for Chromium running **as root in Docker** (see repo **`Dockerfile`** / **`docker-compose.yml`**). Avoid on untrusted multi-tenant hosts. |

Built In uses **Rod** for listing and detail HTML when **`JOBHOUND_BROWSER_ENABLED`** is on (default) and **`JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER`** is on (default). Set **`JOBHOUND_BROWSER_ENABLED=0`** to skip Chromium entirely (Built In falls back to **`net/http`**), or **`JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER=0`** to keep the browser for future sources but force **`net/http`** for Built In only — see **Built In** table below.

Existing Built In delay: **`JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS`** (below) still applies between sequential fetches in both T2 and T3.

**MVP HTML collectors (Europe Remotely, Working Nomads, DOU.ua)** are **T2** — shared HTTP timeouts and User-Agent live in `internal/collectors/utils`; DOU-specific politeness and caps are below.

## DOU.ua (`internal/config/collectors_dou.go`)

| Variable | Default (when unset) | Meaning |
| -------- | --------------------- | ------- |
| `JOBHOUND_COLLECTOR_DOU_SEARCH` | `go` | Broad `search` query for `GET /vacancies/?search=…&descr=1` (stage-1 string for this source until slot wiring). |
| `JOBHOUND_COLLECTOR_DOU_INTER_REQUEST_DELAY_MS` | `400` | Pause between consecutive HTTP calls (listing, each `xhr-load`, each detail). Set `0` to disable (tests / local only). |
| `JOBHOUND_COLLECTOR_DOU_MAX_JOBS_PER_FETCH` | `100` | Upper bound on jobs returned per `Fetch` (hard-capped in code at 500). |

## Himalayas (`internal/config/collectors_himalayas.go`)

| Variable | Default (when unset) | Meaning |
| -------- | --------------------- | ------- |
| `JOBHOUND_COLLECTOR_HIMALAYAS_DISABLED` | (unset) | When set to `1`, `true`, `yes`, or `on`, the Himalayas collector is not constructed (no ingest map entry; debug route returns 500 if no stub). |
| `JOBHOUND_COLLECTOR_HIMALAYAS_MAX_PAGES` | `0` | Caps browse/search API pages per `Fetch`: `0` uses collector default (`5`); negative values mean unlimited pages; positive values cap rounds. |
| `JOBHOUND_COLLECTOR_HIMALAYAS_SEARCH` | (unset) | When non-empty, ingest uses Himalayas search (`GET …/jobs/api/search` with `q=…`) instead of the full browse feed. When unset or empty, behavior is browse-only (same as before this knob existed). |

## Djinni (`internal/config/collectors_djinni.go`)

| Variable | Default (when unset) | Meaning |
| -------- | --------------------- | ------- |
| `JOBHOUND_COLLECTOR_DJINNI_INTER_REQUEST_DELAY_MS` | `400` | Pause between consecutive HTTP calls (each listing page, each detail `GET`). Set `0` to disable (tests / local only). |
| `JOBHOUND_COLLECTOR_DJINNI_MAX_JOBS_PER_FETCH` | `100` | Upper bound on jobs returned per `Fetch` (implementation may hard-cap higher ceiling). |

## Built In (`internal/config/collectors_builtin.go`)

| Variable | Default (when unset) | Meaning |
| -------- | --------------------- | ------- |
| `JOBHOUND_COLLECTOR_BUILTIN_INTER_REQUEST_DELAY_MS` | `1000` | Pause between **every** consecutive document fetch (listing pages per country, each detail) — **T2** or **T3**. Set `0` to disable (tests / local only). Runs can be **large** (many countries × listing pages + deduped details) — delay avoids hammering the origin. |
| `JOBHOUND_COLLECTOR_BUILTIN_USE_BROWSER` | on | When **`JOBHOUND_BROWSER_ENABLED`** is on, Built In uses rod for HTML unless this is `0` / `false` / `no` / `off` (force **`net/http`** for Built In only). |

## Local debug HTTP (agent, optional)

- `JOBHOUND_DEBUG_HTTP_ADDR` — if non-empty, `cmd/agent` listens on this address for **local** debug routes (`GET /health`, `POST /debug/collectors/europe_remotely`, `POST /debug/collectors/working_nomads`, `POST /debug/collectors/dou_ua`, `POST /debug/collectors/himalayas`, `POST /debug/collectors/djinni`, `POST /debug/collectors/builtin`) instead of running the one-shot pipeline. The `-debug-http-addr` flag overrides this when set. Prefer a loopback bind (e.g. `127.0.0.1:3001`). Not a public API — see `../spec.md` and `debug-http-collectors.md` for the JSON request contract (`limit`, per-source optional fields).

## Related

- `../spec.md`
- `browser-fetch.md` — Tier-3 shared fetch contract
- `debug-http-collectors.md`
- `.specify/memory/constitution.md`
