# Feature: Pipeline stage services (pure domain logic)

**Feature Branch**: `004-pipeline-stages`  
**Created**: 2026-03-29  
**Last Updated**: 2026-04-02  
**Status**: Implemented

## Goal

Implement **three sequential local processing steps** on **already-ingested, normalized** `domain.Job` values (the **stage-1 pool** in product terms — populated by collectors / ingest in **`006`**) as **pure, testable packages** with no Temporal inside and **no network calls** in unit tests:

1. **Broad filter (implementation stage 1):** publication date window, role synonyms (title + description), remote-only when required, optional country allowlist. **Not** the same as **product** “stage 1” (external broad ingest + keyword string); see **Product vs implementation numbering** below.
2. **Keywords (implementation stage 2):** include / exclude over text, **no LLM**.
3. **LLM scoring (implementation stage 3):** user profile (for now a **single text block**) + vacancy text; structured output (at minimum score and rationale; extra fields as needed).

Parsing HTML/API from sites and turning “human” dates into `PostedAt` is **out of scope for this feature**; see `005-job-collectors`.

## Product vs implementation numbering

Global MVP behavior is described in [`specs/000-epic-overview/product-concept-draft.md`](../000-epic-overview/product-concept-draft.md). **Numbering differs** between that narrative and this epic:

| Product (draft §1–§4) | What it is | Where it lives |
|------------------------|------------|----------------|
| **Stage 1** | External ingest: sources, normalize, persist; **single broad keyword string** per slot (immutable after first successful ingest). | Primarily **`006`** (and **`005`** collectors). |
| **Stage 2** | **Local** narrow: include/exclude (optional date TBD) **only** on the stage-1 pool for the slot. | **`004`** — **broad filter** then **keyword filter** in order (both are “narrow” on stored rows). |
| **Stage 3** | LLM on rows that **passed** product stage 2; cap, **deterministic ordering**, eligible pool, **idempotency** under retries. | **`004`** — per-job scorer + structured output; **batch** caps, ordering, and run-scoped idempotency are **`007`** (+ orchestration) per draft §4. |

This epic **does not** implement HTTP collectors or slot ingest; callers pass `[]domain.Job` and rule structs **per invocation**. **Reset** when filters change (draft §5) is enforced **outside** these pure functions — see **`008`** / **`009`** (workflows + HTTP).

## Behaviour: filter rejection vs error

- **Fails filter rules** (date, remote, country, role, keywords, etc.) → vacancy **drops out of the stream**. This is **not** an error; this spec does **not** introduce per-vacancy status tracking.
- **Execution failure** (invalid config, LLM call error, unparseable response, etc.) → **log**; do not conflate with “did not match criteria”. Policy for **abort whole run vs skip one vacancy** on stage 3 errors is a **separate decision** (can be pinned later).

## Rule configuration

- Stage parameters (date window, role synonyms, include/exclude, remote, countries, profile text for the LLM) are supplied **in the event / run context**, not as a single global app config.

## Stage 1 — broad filter

**Input:** normalized `domain.Job` values + run rules.

**Date window (`PostedAt`):**

- If rules specify **`from` / `to`** (timestamps), those are used.
- If **not** set, default is a **rolling window** from **now − 7 days** to **now** (semantically “last 7 days”). Reference clock: **UTC only** (all date comparisons and “now” for the window use UTC).

**Role synonyms:** array of strings (e.g. `frontend`, `frontend developer`, …). Match in **`Title` and `Description`** (case-insensitive; listings are mostly English, simple ASCII/Latin casefold is enough).

**Remote:** rule flag meaning **only remote** vacancies. When enabled, keep rows where the vacancy can **explicitly** be treated as remote (domain field populated by the collector). If remote **cannot** be determined → vacancy **does not pass** (rejected).

**Countries:** allowlist of country codes (e.g. ISO 3166-1 alpha-2). **Empty list** → **any** country (no country filter). Non-empty allowlist: pass only if the vacancy country is **known** and in the list; if country is **unknown** → **reject**.

Per-site geo parsing is **out of scope**; `Job` may carry optional fields the collector fills when the site exposes them.

## Stage 2 — keywords

- Search fields: **`Title` and `Description`**.
- **Include** and **exclude** lists are **both optional**; each is a **string array**.
- **Ignore case** (same convention as stage 1 for English text).

Semantics (any include, any exclude rejects, etc.) must be defined unambiguously in code and tests; default: **at least one include** must match if the list is non-empty, and **no exclude** may appear if the exclude list is non-empty.

## Stage 3 — LLM

- Input: user profile text (for now **one block**) + whatever job fields scoring needs.
- LLM provider sits **behind an abstraction** (interface); concrete **Claude** wiring comes later.
- Anthropic API key for real calls: environment **`JOBHOUND_ANTHROPIC_API_KEY`** (see `internal/config/anthropic.go`). Do **not** commit keys.
- Minimum response contract: **numeric score** + **short rationale**; optional fields (flags, etc.) as needed; JSON shape can be refined separately.

## Tests

- Unit tests for edge cases per stage **without** real HTTP/API calls: LLM mocks and `Job` fixtures.

## Out of scope

- HTTP, external notifications, persistence side effects **inside** stage functions (callers decide what to persist).
- Site-specific parsers and normalizing dates like “5 days ago” → **`005-job-collectors`**.

## Dependencies

- `001` — `domain.Job` and related types; extend fields (remote, country) in line with domain and `002` migrations when needed.

## Local development

- Go only; stage 3 tests use mocks, no mandatory Claude API.

## Acceptance criteria

1. **Implementation stage 1** (broad filter) implements date window (explicit `from`/`to` or default **last 7 days UTC**), role synonyms on **title + description**, optional **remote-only** and **country allowlist** per contract; filter drops are **not** errors.
2. **Implementation stage 2** (keywords) implements include/exclude keywords on **title + description** with unambiguous semantics (**all** non-empty includes match; **any** exclude rejects) and tests.
3. **Implementation stage 3** (LLM) exposes an **LLM scorer interface**; minimum output **score + rationale**; unit tests use **mocks** only — **no** real Anthropic calls in default `go test ./...`.
4. Stage packages contain **no** Temporal SDK imports and **no** network I/O inside pure filter/keyword logic; LLM calls only behind the scorer implementation used in tests (mock) or wired with **`JOBHOUND_ANTHROPIC_API_KEY`** at composition time.
5. Run **rules** (windows, lists, profile text) are supplied **per invocation**, not as the sole source from global app env (see `contracts/environment.md`).
6. **`contracts/pipeline-stages.md`** and **`contracts/environment.md`** match implemented behaviour and env names in **`internal/config`**.

## Planning artifacts

- `plan.md` — phases, constitution check, resolved decisions
- `research.md` — repo inventory and semantics notes
- `tasks.md` — implementation checklist
- `checklists/requirements.md` — spec quality checklist
- `contracts/environment.md` — Anthropic-related env vars; rules from run context
- `contracts/pipeline-stages.md` — stage behaviour, `Job` fields, LLM JSON minimum

## Related

- `specs/000-epic-overview/spec.md` — product context and feature order.
- `specs/000-epic-overview/product-concept-draft.md` — slots, three product stages, reset rules, stage-3 policy (§4).
- `specs/007-llm-policy-and-caps/spec.md` — caps, ordering, eligible pool, idempotency for **batch** stage 3.
- `specs/003-temporal-orchestration/spec.md` — orchestration (stages called from activities later).
