# Feature: Telegram notifications

**Feature Branch**: `010-telegram-notifications`
**Created**: 2026-03-29
**Status**: Draft

## Goal

**Activity** (or small package) that sends **short** formatted messages to Telegram: **one message per vacancy**, **hard cap** per run aligned with `007` (e.g. max 5). Errors retried per Temporal policy.

## Scope

- Bot token via env; no secrets in git.

## Out of scope

- Rich UI; long-form rationale stays in DB/API for web.

## Dependencies

- `007` (which rows auto-notify); `008` for event-driven sends.

## Local / Docker

- Optional: test against Telegram test bot; unit tests with mocked HTTP.
