# Feature: Manual search workflow

**Feature Branch**: `009-manual-search-workflow`
**Created**: 2026-03-29
**Status**: Draft

## Goal

Same pipeline as events, triggered **on demand** (API or internal call): return what matched (from **cache** and/or **fresh fetch** per rules in `006`). Temporal workflow optional but preferred for parity with `008`.

## Scope

- Request/response DTO stable for a future web UI; filter state can mirror **query-string** semantics on the client without server-side “presets”.

## Out of scope

- Full REST surface (see `011`).

## Dependencies

- `003`–`007`, `006`.

## Local / Docker

- Call workflow from API process or CLI; Temporal from Compose.
