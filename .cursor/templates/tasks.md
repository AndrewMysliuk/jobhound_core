# {{FEATURE_TITLE}} — tasks

> Agent implementation plan. Every checkbox must be finishable in code (schema, storage, impl, handlers, workflows, unit tests).
> Do **not** list manual QA, DoD, E2E scripts, browser walkthroughs, or “run the product”.
> Frontend tasks (if any): `jobhound_frontend/docs/{{feature_slug}}/{{feature-slug}}-tasks.md`

---

## Scope

**In this slice**

1. [required behavior]

**Paused (do not implement here)**

- [explicit non-goals]

---

## 0. Schema / contract

- [ ] **T1** — `internal/{{module}}/schema/{{file}}.go`: types + Canonical Enum methods
- [ ] **T2** — `internal/{{module}}/api.go` (or `contract.go`): interface + sentinel errors
- [ ] **T3** — public HTTP DTOs in `internal/publicapi/schema/` if this slice has routes

---

## 1. Config / migration (omit section if none)

- [ ] **G1** — `JOBHOUND_{{KEY}}` in `internal/config` only
- [ ] **M1** — `migrations/{{NNN}}_{{name}}.{up,down}.sql`

---

## 2. Storage

- [ ] **S1** — `internal/{{module}}/storage`: GORM model + repository methods
- [ ] **S2** — storage tests (or `//go:build integration` against Postgres)

---

## 3. Impl

- [ ] **I1** — `internal/{{module}}/impl`: use cases; no Temporal SDK
- [ ] **I2** — conflict / not-found mapping to sentinel errors
- [ ] **I3** — impl tests with fake storage / deps

---

## 4. Temporal (omit section if none)

- [ ] **W1** — `internal/{{module}}/workflows`: `RegisterWorkflow` in `New...`
- [ ] **W2** — activities + `workflows/mappers.go` for Temporal → domain
- [ ] **W3** — register from `cmd/worker`

---

## 5. HTTP (omit section if none)

- [ ] **H1** — `handlers/json_schema/{{name}}.schema.json` + `ReadValidatedJSON`
- [ ] **H2** — one file per route; register in `registerRoutes()`
- [ ] **H3** — error envelope `{"error":{"code","message"}}`; no secrets on 500
- [ ] **H4** — handler tests (`net/http/httptest`, mocked `impl`)

---

## 6. Wiring

- [ ] **C1** — `cmd/api` and/or `cmd/worker` composition only (no business rules)

---

## 7. Tests (extra coverage)

- [ ] **Q1** — [table-driven cases: 404, 409, happy path]

---

## Dependencies (brief)

```
T* → G* / M* → S* → I* → W* → H* → C*
Q* after I* + H*
```

**M1:** T*, G*, M*, S*  
**M2:** I*, W*, H*  
**M3:** C*, Q1
