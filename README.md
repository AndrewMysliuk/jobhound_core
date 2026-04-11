# jobhound_core

Collects and processes job listings (pipeline, PostgreSQL, Temporal, HTTP API). Go 1.24.

Env vars and infra details: `specs/`; names and loaders: `internal/config`.

## Docker

```bash
make docker-up    # build images and start all services
make docker-down  # stop and remove containers, volumes, images
```

Migrations: set `JOBHOUND_DATABASE_URL`, then `make migrate-up`.
