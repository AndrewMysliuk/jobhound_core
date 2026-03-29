# Feature: HTTP API for separate UI repo

**Feature Branch**: `011-http-public-api`
**Created**: 2026-03-29
**Status**: Draft

## Goal

HTTP API consumed by a **separate frontend** project: trigger manual search, manage events, read run history, **manual LLM** actions for backlog rows. **No auth in v1**; structure allows adding it later.

## Scope

- Stable JSON shapes; optional OpenAPI in same feature or follow-up.
- CORS and deployment notes for GCP when applicable.

## Out of scope

- Frontend implementation.

## Dependencies

- `008`, `009`, `007`, `002`.

## Local / Docker

- API server on host or compose service pointing at Postgres + Temporal.
