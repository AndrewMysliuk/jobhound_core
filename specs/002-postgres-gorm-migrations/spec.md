# Feature: PostgreSQL, GORM, migrations

**Feature Branch**: `002-postgres-gorm-migrations`
**Created**: 2026-03-29
**Status**: Draft

## Goal

Connect to **PostgreSQL** with **GORM**, adopt a **migration** approach (tool TBD in plan), and create initial tables needed by ingest, events, and profile — evolving with `006` / `008` / `011` without SQLite.

## Scope

- Connection config via env; no secrets in repo.
- Base entities or migration files for jobs, runs, events (to be refined with downstream specs).

## Out of scope

- Full final schema for every column (co-evolve with cache/events specs).

## Dependencies

- `001-agent-skeleton-and-domain` (types and naming alignment).

## Local / Docker

- Postgres service in **Docker Compose** (documented in `000-epic-overview` / compose file when added).
