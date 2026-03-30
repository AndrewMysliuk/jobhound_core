# jobhound_core

Backend for a **personal job agent**: collect vacancies, pipeline (filters → LLM scoring), **PostgreSQL** storage, **Temporal** orchestration, optional Telegram. Go 1.24.

For modules, env vars, and Docker, see `specs/` and `.specify/memory/constitution.md`.

## Run

Requires Go 1.24.

```bash
make build    # bin/agent, bin/worker
make run      # agent
make test     # unit tests
```

Local debug HTTP for collectors (e.g. Postman: `GET /health`, `POST /debug/collectors/…`):

```bash
make run-debug
# or: JOBHOUND_DEBUG_HTTP_ADDR=127.0.0.1:8080 ./bin/agent
```

**Worker (Temporal)** — start Temporal (e.g. `docker compose up -d` from repo root), then:

```bash
export JOBHOUND_TEMPORAL_ADDRESS=localhost:7233
make run-worker
```

**Database and migrations** — set `JOBHOUND_DATABASE_URL`, then `make migrate-up`. Variable names and Compose DSN are documented in `specs/002-postgres-gorm-migrations/contracts/environment.md`.
