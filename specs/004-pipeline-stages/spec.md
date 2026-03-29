# Feature: Pipeline stage services (pure domain)

**Feature Branch**: `004-pipeline-stages`
**Created**: 2026-03-29
**Status**: Draft

## Goal

Implement **three stages** as **testable packages** without Temporal inside:

1. Broad match: role / title / **time window** over normalized jobs.
2. Keyword **include** / **exclude** (no LLM).
3. **LLM** scoring using stored profile + job text; structured output (score, rationale, flags).

## Scope

- Clear function/API boundaries for activities to call later.
- Unit tests for edge cases (optional vs required stack mentions deferred to LLM prompt/schema in plan).

## Out of scope

- HTTP transport; Telegram formatting; persistence side effects inside stage functions (callers handle).

## Dependencies

- `001` domain types; profile shape may need `002` for loading from DB.

## Local / Docker

- None beyond Go; LLM tests use mocks or recorded responses.
