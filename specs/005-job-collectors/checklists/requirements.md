# Specification Quality Checklist: Job collectors

**Purpose**: Validate specification completeness before / during implementation  
**Created**: 2026-03-30  
**Last Updated**: 2026-04-02  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Goal (per-source collectors → `domain.Job`, tiered stack, no retries) is explicit
- [x] Pipeline boundary clear (`internal/collectors` vs `internal/pipeline`)
- [x] MVP sources named; inventory for later rows separate
- [x] Out of scope: cache/dedup (`006`), pipeline stages (`004`)
- [x] Product alignment: slot / stage-1 orchestration vs collector scope traceable to **[`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md)** and **`spec.md`** “Product alignment”

## Requirement Completeness

- [x] `Job.Source` values and error semantics traceable to **`contracts/collector.md`**
- [x] Normalization (description, dates, remote, country, salary/tags/position, `UserID` policy) traceable to **`contracts/domain-mapping-mvp.md`**
- [x] Europe vs Working Nomads transport documented in **`resources/*`**
- [x] Tests: no mandatory live network — reflected in **`tasks.md`** and **`spec.md`**

## Feature Readiness

- [x] Dependencies on `001` / `002` acknowledged
- [x] **`plan.md`** phases and **`tasks.md`** checklist align with **`spec.md`** acceptance criteria
- [x] Constitution alignment (collectors layout, testing policy) reflected

## Notes

- **Implementation locks** (package names, exact function splits) live in code; **behaviour** locks live in **`contracts/*`**.
- Update this checklist when **`spec.md`** or **`plan.md`** changes materially.
