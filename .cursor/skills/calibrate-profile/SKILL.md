---
name: calibrate-profile
description: >-
  Calibrates a JobHound search profile from pasted TOP vacancies and match
  signals: tags false positives, proposes YAML/scorer fixes. Use when the user
  calibrates a profile, reviews TOP jobs, tunes andrew.yaml / lada.yaml, or
  pastes vacancy text for profile scoring feedback.
---

# Calibrate profile

Read this skill, then calibrate. **Do not edit** `config/profiles/*.yaml` or
scorer code unless the user explicitly asks to apply changes after the review.

## Inputs (required)

1. **Profile id** — e.g. `andrew`, `lada`. Path: `config/profiles/<id>.yaml`.
2. **Vacancies** — pasted JD text and/or titles from TOP (batch of ~5–15, not the
   full pool). Optional: job id, company, UI row.

If the profile id is missing, ask once and stop.

## Prerequisites

**Local stack must be up** so the agent can query Postgres (`localhost:5432`,
Compose defaults `jobhound` / `jobhound` / db `jobhound`). Without DB, reviews
are JD+YAML only and miss score/signals/facts — say so and ask the user to
`make docker-up` (or at least Postgres) before continuing a serious batch.

## Load context

1. Read `config/profiles/<id>.yaml` (gates, `gate_location`, `signals_plus`,
   `penalties_soft`, `buckets.top`, `facts.*`).
2. Query local Postgres for each vacancy: `profile_matches` + `jobs` +
   `job_verdicts` (bucket, score, signals, location, salary_raw, facts). Prefer
   DB signals over re-guessing from the JD alone.
3. Scoring behavior lives in `internal/scoring/impl/` (`evaluate.go`,
   `fact_signals.go`). Remote location = allow-list **intersection**; collector
   timezone UTC `-8`/`-7` → Pacific cut; LLM facts only when verdict is ready.
4. Product snapshot (only if needed): `docs/SYSTEM.md` Stage 2 / gates.

Do **not** re-ingest or dump hundreds of full JDs into the model.

## Per vacancy

Classify in the user's language. One line verdict + why.

| Tag | Meaning |
| --- | --- |
| `ok` | Fits profile intent; TOP is fair |
| `us_only` | US/CA/NA-only; Europe remote cannot work it |
| `office` | Office/hybrid; not remote-from-residence |
| `salary_low` | Below profile min (or absurdly low) |
| `tz` | Pacific / incompatible hours |
| `geo` | LatAm / Asia / denied country / bad region |
| `agency` | Recruiter spam / empty company |
| `domain` | Wrong domain (security, sales, etc.) |
| `role` | Manager/lead/coach or wrong IC level |
| `stack` | Wrong stack; title keywords inflated score |
| `facts_gap` | Would be rejectable if facts/location were filled |
| `other` | Short free-text reason |

Also note: **score**, **bucket**, whether TOP is deserved under current rules vs
under the user's real intent.

## Batch close-out

After the batch (or when the user says stop):

1. Count tags.
2. Propose **at most 1–3** concrete fixes (YAML keys and/or scorer rules), tied to
   the top tags. Prefer profile YAML over code.
3. Do **not** apply edits until asked.
4. Do **not** invent broad phrase lists from one odd JD.

## Output shape

Short. Same language as the user.

```markdown
## Vacancy reviews
- **Title** — tag(s); one-line why; score/bucket if known

## Tag counts
- tag: n

## Proposed fixes (not applied)
1. …
```

## Out of scope

- Full-pool LLM review (~1500 jobs)
- Replacing enrich/normalization in this skill
- Frontend UI changes
- Applying profile/scorer patches without an explicit ask
