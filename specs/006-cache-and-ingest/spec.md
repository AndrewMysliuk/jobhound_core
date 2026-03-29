# Feature: Cache and ingest

**Feature Branch**: `006-cache-and-ingest`
**Created**: 2026-03-29
**Status**: Draft

## Goal

**Postgres** as cache of record: upsert normalized jobs, **stable IDs**, **watermarks** / `since` per source for **incremental** fetches. Define behavior for **manual** API: serve from DB vs **explicit refresh** (query flag or separate operation).

## Scope

- Indexes supporting stage 1 queries (role, time, source).
- Dedup semantics aligned with Telegram/history (no duplicate “new” for same logical vacancy).

## Out of scope

- Event scheduler UI; full OpenAPI (see `011`).

## Dependencies

- `002`, `005`; coordinates with `001` ID rules.

## Local / Docker

- Postgres from Compose; seed/migrate as documented.
