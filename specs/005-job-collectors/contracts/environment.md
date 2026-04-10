# Contract: collector environment (T3 placeholder)

**Spec**: `005-job-collectors`  
**Status**: Draft

Headless collectors (**Tier 3**) use **go-rod** and optional **session/cookies** per `.specify/memory/constitution.md`.

When the first T3 collector ships, document here:

- `JOBHOUND_*` env names for rod, cookie file path, and related flags.
- Pointers to `internal/config` loaders (single source of truth for parsing).

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

## Local debug HTTP (agent, optional)

- `JOBHOUND_DEBUG_HTTP_ADDR` — if non-empty, `cmd/agent` listens on this address for **local** debug routes (`GET /health`, `POST /debug/collectors/europe_remotely`, `POST /debug/collectors/working_nomads`, `POST /debug/collectors/dou_ua`, `POST /debug/collectors/himalayas`, `POST /debug/collectors/djinni` when wired) instead of running the one-shot pipeline. The `-debug-http-addr` flag overrides this when set. Prefer a loopback bind (e.g. `127.0.0.1:3001`). Not a public API — see `../spec.md` and `debug-http-collectors.md` for the JSON request contract (`limit`, per-source optional fields).

## Related

- `../spec.md`
- `debug-http-collectors.md`
- `.specify/memory/constitution.md`
