# Specification Quality Checklist: HTTP public API

**Purpose**: Validate specification completeness before / during implementation  
**Created**: 2026-04-04  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [ ] Goal (stable JSON API for browser: slots, profile, stage 2/3 runs, job lists, manual bucket) is explicit — traceable to **`spec.md`** §Goal and [`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) §6–§7
- [ ] **Normative MVP rules** (no stage-1 restart via API; stage 2 wipes 2+3; stage 3 wipes 3 only; concurrency; slot cap 3; backend-only sources) — traceable to **`spec.md`** §MVP rules
- [ ] **Error envelope** + **StageState** + **stage object** shape — traceable to **`spec.md`** §Conventions and [`contracts/http-public-api.md`](../contracts/http-public-api.md)
- [ ] **Out of scope** (no UI repo, no auth, no OpenAPI blocker, no stage-1 POST) — traceable to **`spec.md`** §Out of scope
- [ ] **Dependencies `002`–`008`** and **HTTP → run kind** mapping — traceable to **`spec.md`** §Dependencies and [`contracts/http-public-api.md`](../contracts/http-public-api.md) §7

## Requirement Completeness

- [ ] Every route in **`spec.md`** §Resources has a matching row in [`contracts/http-public-api.md`](../contracts/http-public-api.md) §3 with method, path, success status
- [ ] **409** codes **`slot_limit_reached`** (with **`limit`**) and **`stage_already_running`** — traceable to **`spec.md`** and [`contracts/http-public-api.md`](../contracts/http-public-api.md) §6
- [ ] **CORS** config **`JOBHOUND_API_CORS_ORIGINS`** — traceable to **`spec.md`** §Conventions and [`contracts/environment.md`](../contracts/environment.md)
- [ ] **Job list** pagination, sort, optional **`bucket`** — traceable to **`spec.md`** §Job lists and [`contracts/http-public-api.md`](../contracts/http-public-api.md) §4.6
- [ ] **`PATCH` bucket** only for **stage 2 and 3** — traceable to **`spec.md`** §Manual correction
- [ ] **Filter invalidation** on stage-2 run and profile-driven stage-3 reset — traceable to [`../../008-manual-search-workflow/contracts/filter-invalidation.md`](../../008-manual-search-workflow/contracts/filter-invalidation.md) and **`spec.md`** / product draft §5

## Feature Readiness

- [ ] **`plan.md`** phases and **`tasks.md`** sections align with **`spec.md`** and [`contracts/http-public-api.md`](../contracts/http-public-api.md)
- [ ] Constitution alignment (thin **`cmd/api`**, **`handlers/`** + **`schema/`**, **`internal/config`**) reflected in **`plan.md`** constitution check
- [ ] **`contracts/environment.md`** lists every new **`JOBHOUND_*`** for **`009`** and matches **`internal/config`** after implementation
- [ ] Resolved decisions **D3–D6** in **`plan.md`** are closed in code (stage `error` shape, `stage_3_rationale`, `max_jobs` zero case, empty include/exclude)

## Notes

- If **`008`** and **`009`** disagree, **`spec.md`** for **`009`** states **`009`** wins for public HTTP; update **`008`** contracts or this spec via explicit product decision.
- **`ManualSlotRunWorkflow`** input field names must match [`manual-workflow.md`](../../008-manual-search-workflow/contracts/manual-workflow.md) and `internal/manual/schema` — the API layer translates HTTP JSON → that input.
