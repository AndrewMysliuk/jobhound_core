# jobhound_core

Collects and processes job listings (pipeline, PostgreSQL, Temporal, HTTP API). Go 1.24.

Env names and loaders: `internal/config`. Key list: [`.env.example`](.env.example).

## Docker

```bash
make docker-up    # build images and start all services
make docker-down  # stop and remove containers, volumes, images
```

`make docker-up` also starts a host proxy on `127.0.0.1:18080` (`cmd/ipv6proxy`). The Docker worker has no IPv6, and euremotejobs.com challenges that IPv4; Europe Remotely uses the proxy (`JOBHOUND_EUROPE_REMOTELY_PROXY`). `make docker-down` stops the proxy.

Migrations: set `JOBHOUND_DATABASE_URL`, then `make migrate-up`.
