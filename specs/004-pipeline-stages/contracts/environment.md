# Contract: environment variables (pipeline / LLM)

**Feature**: `004-pipeline-stages`  
**Consumers**: Wiring code that constructs a **real** Anthropic (Claude) client for stage 3 — typically `cmd/agent`, `cmd/worker` activities (later), or tests behind `-tags=integration` if ever added.

**Canonical names**: real API keys are read only through **`internal/config/anthropic.go`** — `config.EnvAnthropicAPIKey` (`JOBHOUND_ANTHROPIC_API_KEY`) and `config.LoadAnthropicAPIKeyFromEnv()` — not ad-hoc `os.Getenv` in `internal/pipeline` stage logic.

## LLM (Anthropic / Claude)

| Variable | Required | Description |
|----------|----------|-------------|
| `JOBHOUND_ANTHROPIC_API_KEY` | Yes for **real** Claude calls | API key for Anthropic; **omit or empty** for unit tests and mock-only runs. |
| `JOBHOUND_ANTHROPIC_MODEL` | No | Claude model id for the Messages API; defaults to `claude-3-5-haiku-20241022` when unset (see `config.DefaultAnthropicModel`). |

**Do not** commit keys; document the **name** only in README and this file.

## Stage rule parameters

**Not environment variables**: date windows, role synonyms, keyword lists, remote-only flag, country allowlist, and **user profile text** for scoring are passed **per run / event** (workflow input, activity payload, or equivalent). Callers assemble rule structs; stage functions do **not** read global env for those semantics.

## Relationship to other contracts

- **Product behavior**: `specs/000-epic-overview/product-concept-draft.md` — stage numbering vs **`004`** implementation is documented in `specs/004-pipeline-stages/spec.md`.
- **Temporal**: `specs/003-temporal-orchestration/contracts/environment.md` — unrelated to stage math; activities may call stages and load Anthropic key at wire-up.
- **Database**: `specs/002-postgres-gorm-migrations/contracts/environment.md` — persistence remains outside stage functions per spec.
