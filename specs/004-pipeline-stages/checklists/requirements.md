# Specification Quality Checklist: Pipeline stage services

**Purpose**: Validate specification completeness and quality before / during implementation  
**Created**: 2026-03-30  
**Last reviewed**: 2026-04-02 (aligned with `product-concept-draft.md`)  
**Feature**: [spec.md](../spec.md)

## Content Quality

- [x] Goal (three **local** implementation stages on ingested jobs, pure logic, no Temporal in stages) is explicit
- [x] **Product** stage 1 (ingest) vs **implementation** stage 1 (broad filter) distinction documented (`spec.md`, `contracts/pipeline-stages.md`)
- [x] Focused on domain stages (not collectors, not orchestration wiring)
- [x] Filter rejection vs execution error distinction is stated
- [x] Run-context rules vs global config distinction is stated
- [x] Out of scope boundaries clear (`005`, HTTP, persistence inside stages)

## Requirement Completeness

- [x] No unresolved [NEEDS CLARIFICATION] markers in spec
- [x] Stage 2 keyword semantics have a default (all includes / any exclude) — detailed in `contracts/pipeline-stages.md`
- [x] Date window default (7 days UTC) and explicit bounds behaviour stated
- [x] LLM behind abstraction; Anthropic env name referenced
- [x] Tests: no real API in unit tests — testable

## Feature Readiness

- [x] Dependencies on `001` / `002` acknowledged
- [x] Constitution alignment (stages before LLM, config rules) reflected
- [x] Acceptance criteria in `spec.md` map to verifiable outcomes

## Notes

- **Plan-level locks** (package layout, stage 3 error policy for callers) live in **`plan.md`**; update this checklist if spec and plan diverge.
- **Exact** Go type names and JSON schema for LLM output are frozen in **`contracts/pipeline-stages.md`** at implementation time.
- **Product vs code numbering** is documented in **`spec.md`** and the top of **`contracts/pipeline-stages.md`**; batch stage-3 policy is **`007`**.
