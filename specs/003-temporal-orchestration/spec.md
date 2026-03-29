# Feature: Temporal orchestration foundation

**Feature Branch**: `003-temporal-orchestration`
**Created**: 2026-03-29
**Status**: Draft

## Goal

Integrate **Temporal** client and **worker**; define the pattern: workflows orchestrate, **activities** call domain services. Support **local Docker** Temporal stack and a path to **GCP** deployment.

## Scope

- Namespace, task queues, timeouts/retries defaults.
- Minimal workflow proving end-to-end worker connectivity (placeholder activities OK).

## Out of scope

- Full business workflows (manual search, events) — those land in `008` / `009`.

## Dependencies

- `001` for packages; `002` optional if first activity touches DB.

## Local / Docker

- Temporal (+ UI if desired) in Compose; worker connects to `TEMPORAL_ADDRESS` (or equivalent).
