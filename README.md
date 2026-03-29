# jobhound_core

Backend for a **personal job agent**: collect vacancies, narrow them in **three stages** (role/time → keywords → CV-aware LLM), persist in **PostgreSQL**, run **manual** and **scheduled** flows via **Temporal**, and optionally notify via **Telegram** (short messages). A **web UI** is planned in a separate repository; this repo will expose an HTTP API. Auth is out of scope for now; the data model stays open for a future user id.

## Stack (target)

Go 1.24, PostgreSQL + GORM, Temporal, Claude API for scoring, Telegram Bot API for delivery. Local development is meant to use **Docker Compose** for Postgres and Temporal (see epic `002` / `003` in `specs/`).

## Repo layout

- `cmd/agent` — entrypoint and wiring
- `internal/domain`, `internal/ports`, `internal/app`, `internal/adapters` — layered core (see `.cursor/rules` for conventions)
- `config/` — configuration
- `specs/` — feature specs (`000` overview, `001`–`012` index in `specs/000-epic-overview/spec.md`)

## Quick start

```bash
# .env in repo root is gitignored — edit it with your keys/paths
make build
make run
make test
```

`make help` lists targets: `build`, `run`, `test`, `fmt`, `vet`, `tidy`.

## Documentation

- [Epic overview and feature index](specs/000-epic-overview/spec.md)
- Constitution and principles: `.specify/memory/constitution.md`
