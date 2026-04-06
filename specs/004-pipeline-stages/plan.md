# Implementation Plan: Pipeline stage services (pure domain logic)

**Branch**: `004-pipeline-stages`  
**Date**: 2026-03-30  
**Last Updated**: 2026-04-02  
**Spec**: `specs/004-pipeline-stages/spec.md`  
**Input**: Feature specification + `research.md`

## Summary

Implement **three local processing steps** (implementation stages 1–3) as **pure, testable Go packages** under `internal/pipeline` (and/or focused subpackages): **broad filter** (date window, role synonyms, remote-only, country allowlist), **keyword include/exclude** (no LLM), and **LLM scoring** behind a **provider interface** (Claude wiring deferred). **Product** “stage 1” (external ingest + broad keyword string) is **`006`/`005`** — this epic runs **on the ingested pool** only; see `spec.md` **Product vs implementation numbering** and `product-concept-draft.md`. **Run rules** (windows, lists, profile text) are passed **per invocation** from the event/run context — not a single global app config blob for stage semantics. **No Temporal SDK** imports inside stage implementations; **no real HTTP** in default unit tests (fixtures + mocks). Filter “reject” is **not** an error; execution failures (bad config, LLM errors) are **logged by callers** and distinguished from “did not match” (see Resolved decisions for stage 3 policy placeholder). **Batch** stage-3 cap, ordering, eligible pool, and idempotency are **`007`** (+ orchestration), not this plan’s core.

## Technical Context

**Language/Version**: Go 1.24  
**Domain**: `internal/domain.Job` (+ optional fields for remote/country per spec and `001`/`002`)  
**Module home**: `internal/pipeline` — extend `contract.go` / `impl/` or add `internal/pipeline/stage1`, `stage2`, `stage3` only if clarity demands it; constitution: interfaces at module boundary  
**LLM**: Interface in `internal/pipeline` (or `internal/pipeline/scoring`); **Anthropic** key name **`JOBHOUND_ANTHROPIC_API_KEY`** via `internal/config` (`internal/config/anthropic.go`)  
**Testing**: Colocated `*_test.go`; **no** mandatory network for `go test ./...`; stage 3 tests use **mocks**  
**Clock**: Stage 1 date window uses **UTC** reference clock (injectable `time.Now` or `Clock` interface in tests)

## Constitution check

*Reference: `.specify/memory/constitution.md`*

| Principle | Status |
|-----------|--------|
| II. Stages before blanket LLM | **PASS** (explicit three stages; LLM last) |
| VI. Config without secrets; config in `internal/config` | **PASS** (only **key name** for Anthropic in env contract; stage **rules** from run context) |
| Layering: no Temporal in domain stages | **PASS** (stages are plain functions / services; Temporal only in `003` worker paths) |
| Testing: `go test ./...` without Docker/network for stages | **PASS** (fixtures + LLM mock) |

## Phase outline

| Phase | Output |
|-------|--------|
| 0 Research | `research.md` |
| 1 Contracts | `contracts/environment.md`, `contracts/pipeline-stages.md` |
| 2 Domain alignment | Extend `domain.Job` (and migrations in `002` if columns required) for **remote** and **country** fields as per frozen contract |
| 3 Stage 1 | Broad filter: `PostedAt` window (explicit `from`/`to` or default last 7 days UTC), role synonyms on title+description, remote flag, country allowlist |
| 4 Stage 2 | Keywords: include/exclude on title+description; semantics frozen in code + tests (any include, any exclude rejects) |
| 5 Stage 3 | `Scorer` / LLM provider interface; structured result (score + rationale minimum); JSON parsing; **mock** implementation for tests |
| 6 Wire & trim | Align `internal/pipeline/impl` orchestration with new stage APIs; keep persistence / outbound notifications **out** of stage functions |
| 7 Docs | README pointers; env contract aligned with `contracts/environment.md` |

## Resolved decisions

| # | Topic | Decision |
|---|--------|----------|
| D1 | **Package layout** | Prefer **one module** `internal/pipeline` with clear files (`broadfilter.go`, `keywords.go`, `scoring.go`, …) unless file size forces subpackages; **no** `go.temporal.io` imports in these packages. |
| D2 | **Case folding** | English-oriented **simple** case-insensitive match (Unicode simple casefold or `strings.EqualFold` on normalized text per field); document in `contracts/pipeline-stages.md`. |
| D3 | **Default date window** | When `from`/`to` **not** set: **`now−7d` .. `now`** in **UTC**; `now` from injected clock in tests. |
| D4 | **Filter vs error** | Failing filter rules → job **omitted** from output slice / marked dropped — **not** `error`. **Invalid rule structs** (e.g. bad bounds) → `error` from validate or stage entry — **caller** logs and aborts or skips run per higher-level policy. |
| D5 | **Stage 3 failure policy** | **Not** fixed in this spec: implementation may return **`error`** from `Score` for LLM failures; **caller** (Temporal activity later) decides abort vs skip vacancy. Document in `contracts/pipeline-stages.md`. |
| D6 | **Scored output** | Reuse / extend `domain.ScoredJob` (`Score`, `Reason`); optional extra fields only if needed — document JSON shape in contract when Claude impl lands. |
| D7 | **Global config** | Stage **parameters** are **not** read from env inside stage functions (except LLM **API key** at wiring time for real client); **rules structs** passed in from run/event context. |

## Engineering follow-ups (non-blocking)

- Real Claude HTTP client, prompts, token limits — can ship after interface + mock.
- `005-job-collectors` — populating `PostedAt`, remote, country from sites.
- Temporal activities calling these stages — `006`+.

## Project structure (documentation)

```text
specs/004-pipeline-stages/
├── spec.md
├── plan.md
├── research.md
├── tasks.md
├── checklists/
│   └── requirements.md
└── contracts/
    ├── environment.md
    └── pipeline-stages.md
```

## Source structure (anticipated — implementation phase)

```text
internal/
├── domain/
│   └── job.go              # + optional Remote, Country (and related) per contract
└── pipeline/
    ├── contract.go         # Filter / Scorer interfaces (refined)
    ├── impl/               # orchestration glue
    ├── mock/               # noop / test doubles
    └── …                   # stage implementations (broad filter, keywords, scoring)
internal/config/
└── anthropic.go            # JOBHOUND_ANTHROPIC_API_KEY (already present)
```
