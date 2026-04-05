# jobhound_core

Collects and processes job listings (pipeline, PostgreSQL, Temporal, HTTP API). Go 1.24.

Env vars and infra details: `specs/`; names and loaders: `internal/config`.

## Run

```bash
make build
make run                 # agent
make run-worker          # Temporal worker; set JOBHOUND_TEMPORAL_ADDRESS
make run-debug           # agent + debug HTTP on 127.0.0.1:8080
make test
make test-integration    # -tags=integration; needs env from specs
```

Migrations: set `JOBHOUND_DATABASE_URL`, then `make migrate-up`.
