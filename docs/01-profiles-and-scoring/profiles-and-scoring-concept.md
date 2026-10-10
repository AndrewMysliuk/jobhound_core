# Search profiles and rule scoring (v2) — concept

> Product and architecture spec. No Go types or HTTP bodies here — see `profiles-and-scoring-schemas-contracts.md`.
> Nuxt tasks live in `profiles-and-scoring-tasks.md`. There is no `jobhound_frontend/docs/01-profiles-and-scoring/`. The UI does not collect or send search rules.
> **Sequence 1**, done. Next: [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md), then [`docs/03-vc-portfolio-boards/`](../03-vc-portfolio-boards/vc-portfolio-boards-concept.md), then [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).

## Why

**Problem today:** v1 has up to three anonymous **slots**, a single global CV-style profile text, and a stage 3 that asks Claude to score every row. Three things are wrong with that. Scoring is opaque — the only explanation is an LLM rationale, so there is no way to tell a bad listing from a bad prompt. The unit of search is a slot, but the real unit is *a person with a fixed set of stack words and exclude phrases*, and there are exactly two of those (software, architecture), not N throwaway slots. And every filter edit wipes stage-2/stage-3 outcomes, so manual decisions ("not interested") are lost on the next run. The browser also owns the keyword and exclude lists (`defaultStage2Exclude.ts`, the stage-2 geo rule, `collectors/schema.Queries`), so the same phrases exist as hardcoded copies.

**New solution:**

1. A **search profile** is a YAML file: sources, stage-1 queries, Wellfound URL slugs, two exclude lists, and a penalty list. Two profiles today; the contract is ready for Postgres later. Those lists are the only copy. Nothing in Go or in the frontend hardcodes them.
2. **Stage 1 stays the ingest it is now.** It still expands a profile's queries into child fetches. It does not score, and it does not know about job titles.
3. **Stage 2 becomes a score.** Title excludes first, then whole-text excludes, then one point per stack word found in the vacancy, then visa and "must be based" penalties. The phrases that fired are stored next to the score.
4. **Stage 3, LLM, Anthropic and the global profile text are deleted**, along with slots and the three-slot cap. The browser no longer builds a stage-2 payload.
5. One vacancy is **one row** regardless of how many boards show it, with `first_seen_at` / `last_seen_at` that survive re-ingest. Retention stays `last_seen_at` older than 30 days. A scoring run only looks at rows whose `first_seen_at` is inside the 10 days before the run started.
6. Per-profile results live in `profile_matches`, where a recompute overwrites score and bucket but **keeps `user_status`**.

Product context: single user, no auth, two people's searches run from two separate buttons. Primary entity: **Profile**. The job title is not a signal. A vacancy counts because of the words in it, not because it is named frontend or full-stack.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| HTTP | **`net/http`** | `internal/publicapi` |
| Profile definitions | **YAML files** (`config/profiles/*.yaml`) | `internal/profiles` |
| Persistence | **PostgreSQL + GORM + migrations** | `internal/jobs/storage`, `internal/scoring/storage` |
| Orchestration | **Temporal** | `internal/scoring/workflows`, `internal/ingest/workflows` |
| Ingest | **http / goquery / go-rod** | `internal/collectors` |
| Scoring | **in-process rules** (no network, no LLM) | `internal/scoring/impl` |

Not on this path: collectors debug HTTP (`cmd/agent`, stays as-is), `cmd/ipv6proxy`. Deleted outright: `internal/llm`, Anthropic, the 001 skeleton `pipeline/impl.Pipeline.Run`, the noop path in `cmd/agent`, and any tech-dictionary module.

## Profile

**Input** (one YAML file per profile, `config/profiles/<id>.yaml`):

- **`id`**, **`name`** — file-level identity and the label in the UI.
- **`domain`** — `software` / `architecture`.
- **`sources`** — which aggregator collectors to poll for this profile.
- **`queries`** — stage-1 search strings, and the only words that add a point at stage 2. Replaces `collectors/schema.Queries`. No job-title words: `frontend` and `full-stack` are not in this list.
- **`wellfound_roles`** — Wellfound `/role/r/{slug}` segments. This is how that one site is addressed. The slugs are not search words for other boards and they do not affect the score.
- **`exclude_title`** — phrases that reject when they hit the title. Checked first.
- **`exclude_text`** — phrases that reject when they hit the title or the description, including country locks (`uk only`, Germany, Philippines, and the rest of that kind). Checked after the title list.
- **`penalties`** — phrases that mean the employer wants the person in a country: visa, work permit, work authorization, right to work, must be based, must reside, citizenship. Each hit subtracts 10. The vacancy stays in the list.

There is no `roles` field, no must/nice tech split, no seniority field, and no country allow-list in code. Europe and USA are allowed because the YAML does not exclude them. A phrase that rejects the USA as a whole is not copied into either exclude list.

**Lifecycle:** files are read on every request, not cached across runs — editing a YAML and pressing the button is enough, no restart. A malformed file or an unknown source id fails the request for that profile only; the other profile keeps working.

**No API-side editing.** Profiles are not created, updated or deleted over HTTP. The browser does not send queries, excludes, or country rules. The only user-writable field in this slice is `user_status` on a match.

**Seed, then delete the old copies.** The first `andrew` YAML is filled from `jobhound_frontend/app/utils/defaultStage2Exclude.ts` and from the query list in `internal/collectors/schema/queries.go`, classified by the rules above. Those files, the frontend geo constant `EUROPE_HIRING_OK`, and the client stage-2 rule builder are then deleted. They are not a second source.

**Two profiles today:**

- `andrew` — software, nine current sources. Queries are the old matrix without `frontend` and `full-stack`: `vue`, `typescript`, `react`, `golang`, `node`, `AI native`, `AI engineer`, `LLM`. Four Wellfound slugs stay as URLs only.
- `architecture` — **not shipped in this slice**. The domain value stays so a later file can use it. Collectors, per-source slug lists, and `architecture.yaml` are [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md).

## Runtime

**Start:** `POST /api/v1/profiles/{id}/runs` with `Idempotency-Key`. The handler writes a `profile_runs` row and starts `ProfileRunWorkflow` with workflow id `profile-run-{profile_id}`. A second POST while that execution is open is a 409. The body is empty. The browser does not attach a keyword list.

**Main flow** (one run):

1. **Ingest** — expand `sources` × `queries` into child ingest workflows: keyword children for q-list sources, `wellfound_roles` children for Wellfound, one catalog child for `we_work_remotely` / `vue_jobs` / `golang_cafe`. Different resources run at the same time. Queries for one source stay in order. `builtin` and `golang_cafe` share one lane because they share Chromium. Child workflow id is `ingest-{source}-{segment}`; an empty query uses `catalog`. Each child takes a Redis lock and respects a cooldown. Keys are `ingest:lock:{source_id}:{query_segment}`.
2. **Score** — after each child that stored jobs, and once more when every lane has finished. Load jobs whose `first_seen_at` is within 10 days before `profile_runs.started_at`, evaluate the profile in memory, upsert `profile_matches`. Counters on the open run update as children finish.
3. Mark the run `SUCCEEDED` with counters.

Two profiles are never scored in one run. The only thing they share is the ingest cooldown: if the other profile just polled `working_nomads` with `vue`, that child is skipped.

**Outcomes per run:** `RUNNING` → `SUCCEEDED` when both steps finish; `FAILED` on a technical error after Temporal retries. A source that is rate-limited, locked, on cooldown, or whose collector this worker did not build is **not** a failure — it is a skipped child, counted in the run row. The skipped source id is logged.

**Progress** lives in the `profile_runs` row (status, counters, timestamps), not in logs. `GET /api/v1/profiles/{id}/runs/latest` reads that row.

## One scoring pass (core loop)

Stage 2 only. Stage 1 has already stored the vacancies. Per job, in this order. A reject stops the pass. Matching is case-insensitive and on word boundaries, so `java` does not hit `javascript`.

1. **Load** — jobs with `first_seen_at` inside the 10 days before the run started. The text is the title plus the description. No section split.
2. **`exclude_title` → `REJECTED`.** A listed phrase in the title. This list runs before the whole-text list.
3. **`exclude_text` → `REJECTED`.** A listed phrase in the title or the description. Country locks live here, not in a geo table.
4. **Score the rest.** Start at 0.
   - Each `queries` phrase found once in the title or description: **+1**.
   - Each `penalties` phrase found once: **−10**. The row stays `PASSED`.
5. **Bucket:** `REJECTED` from step 2 or 3. `PASSED` otherwise, including a score of 0 or below. There is no `UNKNOWN` bucket and no "must have matched" gate.
6. **Persist** — upsert `profile_matches` with bucket, score, the list of phrases that fired, and the run id. `user_status` is never written by scoring.

The default job list is `PASSED` and `user_status` `NEW`, sorted by score. `REJECTED` rows are stored too, with the phrase recorded, and are available through the bucket filter. Hidden rows stay out of that default list until the client asks for `HIDDEN`.

A vacancy titled "Frontend Engineer" gets no point for that title. It gets a point only when a `queries` phrase such as `vue` is actually in the text.

### Failure at any step

Temporal retries the activity. The `profile_runs` row goes `FAILED` only after retries are exhausted; `profile_matches` from the previous run stay untouched, so a failed run never empties the list. In logs: `failure_stage` = `ingest` | `score`.

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| Second run POST while one is open | 409, no second execution |
| Profile YAML missing or malformed | 404 for unknown id; 400 for malformed, other profiles unaffected |
| Same vacancy on two boards | one `jobs` row; `id` is the content key; `sources` accumulates |
| Vacancy seen again on a later run | `last_seen_at` moves, `first_seen_at` does not |
| Vacancy disappears from every board | row survives until `last_seen_at` is 30 days old, then hard-deleted |
| Vacancy first seen more than 10 days before the run | not scored on this run; the row stays until retention |
| Recompute after a YAML edit | score, bucket and signals overwritten; `user_status` preserved |
| Job hidden by the user, then re-scored higher | stays hidden; only the user can un-hide |
| `min_score` | **not a filter in v2.** The default list is `PASSED`, sorted by score; a threshold is chosen later, by eye |
| Catalog source with a non-empty `queries` | queries ignored, one catalog fetch |
| Wellfound | children use `wellfound_roles` only; those slugs are not `queries` and do not score |
| Browser sends keywords or exclude phrases | no such field exists on the run endpoint |
| Profile source whose collector this worker did not build | skipped child, `sources_skipped` increments, run still `SUCCEEDED` |
| Job list with `user_status` omitted | `NEW` only. `HIDDEN` is an explicit filter |

**Hard rules:** no network and no LLM inside scoring; at most one open run per profile; no Temporal SDK in `impl/`; profiles are read-only over HTTP; search phrases exist only in profile YAML.

## Screen

The Nuxt app is part of this slice. Its tasks are in `profiles-and-scoring-tasks.md`.

- **Home** lists profiles from the API. Each row shows the profile name and opens that profile. No create, no delete, no slot cap. With only `andrew.yaml` shipped, the list has one row; the screen still reads it from the API.
- **Profile page** has a run button, the latest run, and the job list. The button starts a run with a new idempotency key and an empty body. It does not send queries, excludes, or penalties.
- **List** uses the API default (`PASSED`, `NEW`). The user can switch to `REJECTED` and to `HIDDEN`. Hide writes `HIDDEN`. From the hidden list the user can write `NEW` again.
- **While the run is `RUNNING`**, the page polls the latest run and the job list. An already-open run is shown as the error message.
- **Removed screens:** the searches list, slot details, keyword textareas, and the global profile-text page. The header does not link to that page.

## Modules

| Module | Purpose |
| ------ | ------- |
| **`internal/profiles`** | profile store contract + YAML file store; no persistence yet |
| **`internal/scoring`** | `internal/pipeline` renamed: scoring use cases, `profile_runs` / `profile_matches` storage, `ProfileRunWorkflow`. Keeps the phrase-matching helpers from `pipeline/utils` |
| **`internal/jobs`** | dedup-key upsert, `first/last_seen_at`, retention by `last_seen_at` |
| **`internal/ingest`** | Redis keys and watermarks lose the slot segment. Query strings come from the profile, not from `collectors/schema` |
| **`internal/collectors`** | unchanged transports; `schema.Queries` deleted, `apply_url` / company website audit |
| **`internal/publicapi`** | `/api/v1/profiles/*`; slot, stage and profile-text routes removed |

**Deleted:** `internal/llm`, `internal/profile`, `internal/slots`, `internal/manual` (its parent workflow moves to `internal/scoring/workflows`), `pipeline/impl` and `pipeline/mock`. No `internal/dictionary`.

## Layers

- **Saved** — profile YAML (read-only), `jobs` (shared by both profiles), `user_status`.
- **Run / session** — `profile_runs` row, child ingest results, Redis lock/cooldown.
- **Wire** — HTTP bodies in `publicapi/schema`, workflow/activity payloads in `scoring/schema`. The wire does not carry queries or excludes.

## Implementation order

1. **Cleanup.** Delete stage 3, `internal/llm`, Anthropic env, `internal/profile` + `user_profile`, `internal/slots` + slot tables, the noop pipeline path, dead `config.Config` fields. Small debt in the same pass: `go-rod` becomes a direct require, `DefaultIngestSourceIDs` stops listing sources the worker may not have built.
2. **Profiles.** `internal/profiles` + `config/profiles/andrew.yaml`, seeded from the old query list and `defaultStage2Exclude.ts`, then those hardcoded copies deleted. `collectors/schema.Queries` removed; stage 1 reads `queries` and `wellfound_roles` from the profile.
3. **Job identity.** Content-key id (normalized title and `location.raw`; `apply_url` is a column), `sources`, `first/last_seen_at`, `location` jsonb, `company_key`, `company_website`; retention by `last_seen_at`; ingest keys without slot. Audit the nine collectors for `apply_url` and company website.
4. **Scoring.** Title exclude, text exclude, +1 per query phrase, −10 per penalty, 10-day `first_seen_at` window, `profile_runs` / `profile_matches`, `ProfileRunWorkflow`. A source with no built collector is a skipped child.
5. **API and frontend.** Profile routes and the Nuxt screens in the tasks file: profile list, run button, run status, scored list with hide and un-hide. The screen does not edit or send search phrases.

Steps 1–5 are a working v2. Early check: one `ProfileRunWorkflow` test with a stub collector and a two-job fixture, asserting buckets and that `user_status` survives a second pass.

## Decisions taken

- **Migrations:** `000001_initial_schema` is rewritten for the v2 shape, not followed by a `000002` full of `DROP`s. There is no deploy, the database lives in a container and job identity changes anyway, so there is nothing to preserve.
- **Job identity:** `jobs.id` *is* the dedup key — one row per vacancy, `sources` accumulates boards. No separate `dedup_key` column. The key is `company_key` + `\x1e` + normalized title + `\x1e` + normalized `location.raw`. Title and `location.raw` are trimmed, runs of whitespace collapse to a single space, then the string is lowercased. Empty raw is an empty segment. No stemmer, no stop-word list, no geo coder. `apply_url` is stored on the row and is not part of `id`, so a later ATS link does not create a second row.
- **Location:** the six location columns collapse into one `location` jsonb for display. Country policy is phrases in the profile YAML, not a code allow-list and not `EUROPE_HIRING_OK`.
- **Module names:** `internal/pipeline` → `internal/scoring`, the parent workflow moves there, `internal/manual` is deleted.
- **Search phrases:** only in profile YAML. Stage 1 fetches them. Stage 2 scores them. The frontend does not substitute defaults.
- **Titles:** job-title words are not queries and not points. Wellfound slugs are the exception, and only as URLs.
- **Score:** +1 once per `queries` phrase, −10 once per `penalties` phrase. Title exclude, then text exclude.
- **Missing collector:** a YAML source whose collector this worker did not build is skipped and counted. The run still succeeds.
- **Job list:** omitted `user_status` means `NEW`.
- **UI:** both repositories are tasked from this folder. The screen lists profiles, starts a run, and shows the scored list.

Still open: the eventual `min_score`, chosen by eye after the first real run. It is not a filter in this slice. The +1 and −10 values are the rule for this slice, not a draft table.

## Out of scope

- Companies and ATS polling — `docs/04-companies-and-ats/`. No company-tag signal in this slice.
- VC portfolio job boards — `docs/03-vc-portfolio-boards/`.
- Architecture collectors and `architecture.yaml` — [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md). LLM in any form, CV parsing, profile editing in the UI, auth and multi-user, scheduled auto-refresh, GCP deploy.
