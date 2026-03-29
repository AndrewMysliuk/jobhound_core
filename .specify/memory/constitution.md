# jobhound_core Constitution

## Product

Personal job aggregator on Go: ingest vacancies from several sources, narrow in **three stages** (role/time → keywords include/exclude → CV-aware LLM), persist results and history, notify via Telegram on scheduled **events**. **Temporal** runs durable workflows for manual runs and hourly (or other) event schedules.

High-level flow: **collect / read cache → stage 1 → stage 2 → stage 3 (LLM) → persist → (optional) Telegram**. Same engine for interactive API triggers and cron-like event runs.

## Core Principles

### I. One collector interface

Every source implements a single `Collector` contract; adding a site does not require changing unrelated packages.

### II. Stages before blanket LLM

Stage 1 (broad role / time window) and stage 2 (keyword include/exclude) shrink the set; the LLM (stage 3) runs on that pool so we do not score obvious noise. Policy for auto vs manual LLM passes is defined in specs (e.g. cap for automatic Telegram path).

### III. Session abstraction for headless

LinkedIn (and similar) use a `session.Provider`; start with a cookie file, swap implementation without rewriting collectors.

### IV. Postgres as system of record

Vacancies, deduplication, event run history, user profile text, and scoring outcomes live in **PostgreSQL** (via GORM + migrations). No SQLite in the target architecture.

### V. Temporal for orchestration

Workflows coordinate activities (fetch, persist, score, notify). Retries, visibility, and local/cloud parity are first-class; auth is deferred but data models stay extensible for a future `user_id` (or equivalent).

### VI. Config without secrets in repo

Secrets and local paths live in `.env` (gitignored). The Makefile documents variable names only. Production targets **GCP** for runtime and secrets.

## Stack (target)

- Go 1.24
- PostgreSQL + GORM + migrations
- Temporal (worker + workflows + activities)
- Collectors: `net/http`, goquery, go-rod where needed
- Claude API for scoring; Telegram Bot API for delivery
- Local dev: **Docker Compose** (Postgres, Temporal stack) as specified in `specs/000-epic-overview`

## Governance

- Amend this file when architecture decisions change; keep it short and actionable.
- Feature details and order of implementation: `specs/` (per-feature folders, same style as `omg-bo/specs`).

**Version**: 1.1.0 | **Ratified**: 2026-03-29
