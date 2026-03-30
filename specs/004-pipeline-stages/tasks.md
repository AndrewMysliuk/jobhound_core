# Tasks: Pipeline stage services (pure domain logic)

**Input**: `spec.md`, `plan.md`, `research.md`, `contracts/*`  
**Tests**: REQUIRED — unit tests per stage **without** real network/API; stage 3 uses **mocks**.

## A. Contracts & documentation

1. [ ] **Freeze env contract for LLM wiring** — Definition of done: `contracts/environment.md` matches `internal/config` for Anthropic; README mentions key name if real scoring is wired; no secrets in repo.
2. [ ] **Freeze stage I/O and semantics** — Definition of done: `contracts/pipeline-stages.md` matches types and behaviour (date window UTC, keyword rules, filter vs error, minimum scoring output).

## B. Domain & migrations (if needed)

1. [ ] **Extend `domain.Job` for stage 1** — Definition of done: optional **remote** and **country** (or agreed names) present; zero/empty means “unknown” where spec requires; unit tests for helpers if any.
2. [ ] **SQL migration (if persisting new fields)** — Definition of done: migration under `migrations/` + storage mapping if `002` schema must track new columns; **or** explicit note in tasks that fields are runtime-only until `006` — must match plan.

## C. Stage 1 — broad filter

1. [ ] **Implement broad filter API** — Definition of done: accepts `[]domain.Job` + rules struct(s); returns filtered slice; **no** Temporal imports; UTC window with injectable clock; default 7-day window when bounds unset.
2. [ ] **Unit tests — stage 1** — Definition of done: date edge cases, synonyms, remote-only unknown rejects, country allowlist + unknown rejects.

## D. Stage 2 — keywords

1. [ ] **Implement keyword filter** — Definition of done: include/exclude on title+description; semantics per contract (all includes, any exclude); case-insensitive per plan.
2. [ ] **Unit tests — stage 2** — Definition of done: empty lists, only include, only exclude, combined.

## E. Stage 3 — LLM scoring

1. [ ] **Define scorer interface + types** — Definition of done: provider interface in `internal/pipeline`; minimum **score** + **rationale** on result type; **no** `os.Getenv` in feature code for shared knobs (key via config at wire-up).
2. [ ] **Mock provider** — Definition of done: tests use mock/fake implementation; no mandatory `JOBHOUND_ANTHROPIC_API_KEY` for `go test ./...`.
3. [ ] **Unit tests — stage 3** — Definition of done: parse/merge errors surface as `error` from scorer API if applicable; happy path returns `ScoredJob`-compatible shape.

## F. Integration with `internal/pipeline`

1. [ ] **Wire `impl` (or orchestration)** — Definition of done: pipeline applies stage 1 → 2 → 3 in order; **no** persistence or Telegram inside stage functions.
2. [ ] **Update mocks** — Definition of done: `pipeline/mock` implements updated interfaces where applicable.

## G. Quality gates

1. [ ] **`make test` / `go test ./...`** — Definition of done: passes without network; no integration tag required for stage tests.
2. [ ] **`make vet` / `make fmt`** — Definition of done: clean for touched packages.

## H. Optional / deferred (do not block `004` closure)

1. [ ] **Real Anthropic client implementation** — Optional follow-up; interface + mock sufficient for `004` per spec.
2. [ ] **Temporal activities invoking stages** — Owned by later specs (`006`+).
