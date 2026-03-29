# Feature: Job collectors

**Feature Branch**: `005-job-collectors`
**Created**: 2026-03-29
**Status**: Draft

## Goal

One **`Collector`** interface; implementations for planned sources (tiered: HTTP/API, **goquery**, **go-rod** + session file). Normalize to domain `Job`; respect rate limits and errors.

## Scope

- Per-source packages or files; shared HTTP client options where useful.
- Tests: fixtures / golden HTML; avoid mandatory live network in default `go test`.

## Out of scope

- Caching and upsert policy (see `006`).

## Dependencies

- `001` (`Job`, collector interface).

## Local / Docker

- Optional integration against real sites outside CI; document env for cookies/session path.
