# Specification Quality Checklist: Job collectors

**Purpose**: Validate specification completeness before / during implementation  
**Created**: 2026-03-30  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [ ] Goal (per-source collectors → `domain.Job`, tiered stack, no retries) is explicit
- [ ] Pipeline boundary clear (`internal/collectors` vs `internal/pipeline`)
- [ ] MVP sources named; inventory for later rows separate
- [ ] Out of scope: cache/dedup (`006`), pipeline stages (`004`)

## Requirement Completeness

- [ ] `Job.Source` values and error semantics traceable to **`contracts/collector.md`**
- [ ] Normalization (description, dates, remote, country, salary/tags/position) traceable to **`contracts/domain-mapping-mvp.md`**
- [ ] Europe vs Working Nomads transport documented in **`resources/*`**
- [ ] Tests: no mandatory live network — reflected in **`tasks.md`** and **`spec.md`**

## Feature Readiness

- [ ] Dependencies on `001` / `002` acknowledged
- [ ] **`plan.md`** phases and **`tasks.md`** checklist align with **`spec.md`** acceptance criteria
- [ ] Constitution alignment (collectors layout, testing policy) reflected

## Notes

- **Implementation locks** (package names, exact function splits) live in code; **behaviour** locks live in **`contracts/*`**.
- Update this checklist when **`spec.md`** or **`plan.md`** changes materially.
