# Contract: collector environment (T3 placeholder)

**Spec**: `005-job-collectors`  
**Status**: Draft

Headless collectors (**Tier 3**) use **go-rod** and optional **session/cookies** per `.specify/memory/constitution.md`.

When the first T3 collector ships, document here:

- `JOBHOUND_*` env names for rod, cookie file path, and related flags.
- Pointers to `internal/config` loaders (single source of truth for parsing).

**MVP (Europe Remotely, Working Nomads)** is **T2** only — no extra env beyond normal HTTP client settings (timeouts live in code or a small shared config as implemented).

## Related

- `../spec.md`
- `.specify/memory/constitution.md`
