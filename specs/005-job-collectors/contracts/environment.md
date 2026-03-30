# Contract: collector environment (T3 placeholder)

**Spec**: `005-job-collectors`  
**Status**: Draft

Headless collectors (**Tier 3**) use **go-rod** and optional **session/cookies** per `.specify/memory/constitution.md`.

When the first T3 collector ships, document here:

- `JOBHOUND_*` env names for rod, cookie file path, and related flags.
- Pointers to `internal/config` loaders (single source of truth for parsing).

**MVP (Europe Remotely, Working Nomads)** is **T2** only — no extra env beyond normal HTTP client settings (timeouts live in code or a small shared config as implemented).

## Local debug HTTP (agent, optional)

- `JOBHOUND_DEBUG_HTTP_ADDR` — if non-empty, `cmd/agent` listens on this address for **local** debug routes (`GET /health`, `POST /debug/collectors/europe_remotely`, `POST /debug/collectors/working_nomads`) instead of running the one-shot pipeline. The `-debug-http-addr` flag overrides this when set. Prefer a loopback bind (e.g. `127.0.0.1:8080`). Not a public API — see `../spec.md` for the single JSON request contract (`limit`, optional Working Nomads ES fields).

## Related

- `../spec.md`
- `.specify/memory/constitution.md`
