# Feature: Scheduled events and run history

**Feature Branch**: `008-events-and-run-history`
**Created**: 2026-03-29
**Status**: Draft

## Goal

**Event** entity: saved search parameters + schedule (e.g. hourly). **Temporal** workflow runs **incrementally**; each run appends **history** (found N, found 0, errors). Aligns with cap/policy from `007`.

## Scope

- Store last successful watermark / run time for incremental eligibility.
- No full re-scan of old vacancies each hour.

## Out of scope

- Public HTTP CRUD for events (thin slice here if needed; full API in `011`).

## Dependencies

- `003`, `006`, `007`; `004` for stages inside activities.

## Implementation tasks

Concrete backlog (including DB stub migrations deferred from `002` plan D3): see [`tasks.md`](./tasks.md).

## Local / Docker

- Temporal + Postgres from Compose; worker executes schedules or external trigger (Cloud Scheduler later on GCP).
