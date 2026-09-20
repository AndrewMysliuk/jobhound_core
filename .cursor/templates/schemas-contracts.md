# {{FEATURE_TITLE}} — schemas and contracts

> Template: implementation contracts. Types, HTTP, Temporal, storage, env, errors.
> Frontend reference if needed: `jobhound_frontend/docs/{{feature_slug}}/{{feature-slug}}-schemas-contracts.md`

---

## Modules

| Module | HTTP | Persistence | Temporal |
|--------|------|-------------|----------|
| `internal/{{module_a}}` | — | `{{table_a}}` | `{{WorkflowName}}` |
| `internal/publicapi` | `/api/v1/{{route}}/*` | — | — |

`{{ADJACENT_MODULE}}` — **outside {{CORE}}**. [how it connects: reads run outcome / slot_id / job_id].

---

## Types (`internal/{{module}}/schema`)

Module-local data model: structs, enums, named constants, payloads, exported errors. Pure functions stay in `utils/` or `impl/`.

```go
type {{Entity}}Status string

const (
	{{Entity}}Status{{A}} {{Entity}}Status = "{{status_a}}"
	{{Entity}}Status{{B}} {{Entity}}Status = "{{status_b}}"
)

type {{Entity}} struct {
	ID        string
	Status    {{Entity}}Status
	CreatedAt time.Time
	UpdatedAt time.Time
}

type {{Create}}Params struct {
	Name string
}

type {{Run}}Payload struct {
	{{Entity}}ID string
	// workflow / activity input
}
```

Every exported string enum follows the Canonical Enum Pattern (`String` / `Equals` / `Pointer` / `FromValue` / `ValuesT` / `FromStringT`). A bare `Valid() bool` is not enough.

HTTP request/response types for the product API live in **`internal/publicapi/schema/`**. Feature `impl/` params may live in **`internal/{{module}}/schema/`**.

---

## Contract (`internal/{{module}}`)

`contract.go` / `api.go` / `errors.go` / `temporal.go` — interfaces and sentinel errors only. No helpers at module root.

```go
type API interface {
	Create(ctx context.Context, p schema.{{Create}}Params) ({{Result}}, error)
}

var (
	Err{{NotFound}} = errors.New("{{entity}} not found")
	Err{{Conflict}} = errors.New("{{entity}} conflict")
)
```

---

## Storage

| Table / column | Role |
| -------------- | ---- |
| `{{table}}` | [entity rows] |
| `{{column}}` | [status / watermark / fk] |

**Migration:** `migrations/{{NNN}}_{{name}}.up.sql` (+ `.down.sql`). GORM models only in `internal/{{module}}/storage`.

---

## HTTP — `internal/publicapi`

Prefix: `/api/v1`. Envelope on non-2xx:

```json
{
  "error": {
    "code": "machine_readable_snake",
    "message": "Human-readable explanation"
  }
}
```

Bodies: embed JSON Schema in `handlers/json_schema/{{name}}.schema.json`, then `publicapi/utils.ReadValidatedJSON` + typed struct + `DisallowUnknownFields()`. One file per route; shared helpers in `publicapi/utils/`. Register in `handlers/handler.go` `registerRoutes()`.

### `{{METHOD}} /api/v1/{{route}}`

[One-line purpose.]

**Body**

```json
{
  "{{field}}": "string"
}
```

**Response `{{CODE}}`** — [shape / key fields].

**Errors:** `{{CODE}}` [when].

---

### `GET /api/v1/{{route}}/{id}`

**Response `200`** — [shape]. **Errors:** `404` when missing.

---

## Temporal (if this slice uses it)

| Kind | Name | Module |
|------|------|--------|
| Workflow | `{{WorkflowName}}` | `internal/{{module}}/workflows` |
| Activity | `{{ActivityName}}` | `internal/{{module}}/workflows/activities` |

`New...` constructor lists every `RegisterWorkflow`. Mapping Temporal types → domain lives in `workflows/mappers.go` (or `workflows/utils/`). **`impl/` must not import `go.temporal.io/sdk/...`.**

**Start / id / concurrency:** [workflow id scheme; 409 if already running].

**Payloads:** [fields; what is not in the payload].

---

## Config

| Env | Meaning | Default |
| --- | ------- | ------- |
| `JOBHOUND_{{KEY}}` | [purpose] | [default or required] |

Names and parsing only in `internal/config`. Feature packages must not `os.Getenv` shared knobs. Document the same row in this file (or a sibling `environment.md` if the slice has many keys).

---

## Errors

| HTTP / sentinel | `code` | When |
| --------------- | ------ | ---- |
| `400` | [validation] | malformed JSON / schema fail |
| `404` | [not_found] | missing entity |
| `409` | [conflict] | cap, already running, idempotency clash |
| `500` | [internal] | no secrets in body |

---

## Tests (boundaries)

| Layer | Where | What |
| ----- | ----- | ---- |
| Handlers | `internal/publicapi/handlers/*_test.go` | mux + mocked `impl` |
| Impl | `internal/{{module}}/impl/*_test.go` | use cases |
| Storage | `internal/{{module}}/storage/*_test.go` | persistence; `//go:build integration` if real Postgres |

Do not add tests whose only job is enum plumbing or trivial `utils/`.
