# Contract: environment variables (observability)

**Feature**: `010-observability`  
**Consumers**: `cmd/agent`, `cmd/api`, `cmd/worker`, `cmd/retention` (optional alignment), tests.

**Canonical names**: parse only in **`internal/config`**; binaries receive a configured `zerolog.Logger` or logging options from `Load()` / a dedicated loader.

## Logging

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_LOG_LEVEL` | No | Minimum level: `debug`, `info`, `warn`, `error`, `fatal`, `panic`, `trace`, plus `disabled` / `off` / `none` (no logs). Parsed in **`internal/config`** (`DefaultLogLevel` = **`info`**) when unset — same default for `api`, `worker`, `agent`, `retention`. Invalid values fall back to **`info`**. |
| `JOBHOUND_LOG_FORMAT` | No | `console` (human-readable) vs `json`. **Default**: `console` (`DefaultLogFormat` in **`internal/config`**); set `json` in Compose/prod for GCP Cloud Logging. |

## Existing variables

- No change to Temporal, DB, or Redis env contracts; this epic **adds** the logging table above only.

## Compose / GCP

- For log drain to **Cloud Logging**, run processes with **`JOBHOUND_LOG_FORMAT=json`** and capture stdout.
