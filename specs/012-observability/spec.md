# Feature: Observability and operations

**Feature Branch**: `012-observability`
**Created**: 2026-03-29
**Status**: Draft

## Goal

Structured **logging**, request/workflow **correlation** (e.g. Temporal `workflow_id` / `run_id` in log fields), and hooks suitable for **GCP** (Cloud Logging, traces later if needed).

## Scope

- Conventions for all long-running workers and HTTP handlers.
- Minimal dashboard story: Temporal Web + log queries.

## Out of scope

- Full SRE playbook; pager policies.

## Dependencies

- Best applied after `003` and `011` exist in some form.

## Local / Docker

- Same binaries; pretty console logs locally, JSON optional via env flag.
