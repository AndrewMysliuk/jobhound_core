# Specification Quality Checklist: Observability (structured logging)

**Purpose**: Validate specification completeness before / during implementation  
**Created**: 2026-04-05  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [ ] Goal (structured logs, correlation, GCP-friendly stdout, **slot_id** / **user_id**) explicit — traceable to **`spec.md`** §Goal
- [ ] **No go-common**; **`internal/platform/logging`** — traceable to **`spec.md`** §Scope and [`../002-postgres-gorm-migrations/spec.md`](../../002-postgres-gorm-migrations/spec.md)
- [ ] **X-Request-ID** on debug HTTP **now** and public API — traceable to **`spec.md`** §Context contract
- [ ] Layering (**handler** / **method** / **workflow**, storage mostly silent) — traceable to **`spec.md`** §Where to log and [`contracts/logging.md`](../contracts/logging.md)
- [ ] Non-goals: metrics/traces vendors, dashboards, SRE playbook — traceable to **`spec.md`** §Out of scope
- [ ] **Security** (no secrets / bodies in logs) — traceable to [`contracts/logging.md`](../contracts/logging.md) §Security

## Requirement Completeness

- [ ] **Env contract**: `JOBHOUND_LOG_LEVEL`, `JOBHOUND_LOG_FORMAT` — [`contracts/environment.md`](../contracts/environment.md) ↔ `internal/config`
- [ ] **Field contract**: stable JSON keys — [`contracts/logging.md`](../contracts/logging.md) ↔ `internal/platform/logging`
- [ ] **Cmd** binaries use structured logger — traceable to [`tasks.md`](../tasks.md) §C
- [ ] **Debug HTTP** — all routes in [`tasks.md`](../tasks.md) §E covered
- [ ] **Public API** — all routes in [`tasks.md`](../tasks.md) §F covered
- [ ] **impl** services (`slots`, `profile`, `pipeline`) — [`tasks.md`](../tasks.md) §G
- [ ] **Activities** (ingest, pipeline, manual, retention) — [`tasks.md`](../tasks.md) §H
- [ ] **Workflows** (ingest, manual, retention, pipeline) — [`tasks.md`](../tasks.md) §I
- [ ] **collectors/multi** unstructured logs removed — [`tasks.md`](../tasks.md) §J

## Feature Readiness

- [ ] **`plan.md`** phases match **`spec.md`** and touch list matches repo layout
- [ ] **`tasks.md`** implementation order is coherent (platform → cmd → HTTP → domain → Temporal)
- [ ] Constitution: config centralization, no feature-level `os.Getenv` for logging

## Notes

- Avoid **duplicate Error logs** for the same failure (handler vs service); document the chosen split in the first PR touching both.
- Temporal **workflow** code: keep logs deterministic; heavy detail in **activities** only.
