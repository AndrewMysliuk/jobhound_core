# Feature: LLM policy, caps, and job statuses

**Feature Branch**: `007-llm-policy-and-caps`
**Created**: 2026-03-29
**Status**: Draft

## Goal

After stages 1–2, enforce **auto path limits** (e.g. **up to 5** vacancies for automatic Telegram in a run). Remaining matches get **other statuses** (visible in history / API) and support a **manual** action to run LLM / confirm match.

## Scope

- State machine or enum set for vacancy-in-run; persistence in Postgres.
- Idempotent transitions where possible.

## Out of scope

- Telegram formatting (see `010`); workflow scheduling (see `008`).

## Dependencies

- `004` (LLM call), `002` (persistence).

## Local / Docker

- Claude API key via env; tests mock provider.
