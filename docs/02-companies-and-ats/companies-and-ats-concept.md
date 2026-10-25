# Company database and direct ATS polling — concept

> **Sequence 2.**
> Two phases in one file. **Phase 0** is measurement — no production code, output is numbers that close the gaps marked `GAP-n` below. **Phase 1** is the implementation slice; it is written in full and can be read now, but five decisions inside it stay open until Phase 0 reports.
> Contracts and tasks files are intentionally absent until Phase 1 is picked up for implementation.

## Goal

Flip the vacancy pipeline.

| Role | Before | After this slice |
| ---- | ------ | ---------------- |
| Aggregators (incl. fund boards already in the profile) | Vacancy feed | **Company-name intake only** — write / refresh company rows, hand over apply URL / website for resolve |
| Popular ATS (Greenhouse, Lever, Ashby) | Unused | **Vacancy feed** — poll every `RESOLVED` company board |
| Manual seed | Spreadsheet on disk | One-shot bootstrap of the company table |

**End state (Done):** one profile run (1) hits aggregators only to discover companies not yet stored, (2) resolves ATS + slug for `PENDING` rows, (3) polls all `RESOLVED` companies on Greenhouse / Lever / Ashby, (4) scores those ATS vacancies, (5) refreshes `tech_tags`, (6) UI highlights jobs whose company is in Andrew's applied list. Local DB survives Docker wipes.

**Done is measurable, not a feature list:**

- ≥70% of non-blocklisted companies in `RESOLVED`;
- more vacancies arriving from ATS boards than from aggregators in a run;
- LLM scoring calls per run no higher than before the flip (dedup by description hash works);
- a full `make db-restore` on an empty volume brings back jobs and scores (**dump/restore already ships via Makefile `db-dump` / `db-restore`**, not this slice).

Primary entity: **Company**.

## Why

Aggregators lag, truncate descriptions, and miss companies that never post there. The apply URL they already return usually points at Greenhouse, Lever or Ashby — enough to leave the aggregator as a name source and take vacancies from the ATS board.

That last sentence is the bet of the whole slice, and it is currently an assumption. Phase 0 tests it before anything is built.

---

# Phase 0 — RnD

No schema, no workflows, no production code. Throwaway scripts and SQL over data already collected are fine. Output is a short numbers file: `docs/companies-and-ats/rnd-findings.md`.

## Why this comes first

Three things could make Phase 1 as written wrong rather than merely incomplete:

1. If aggregators rarely hand over an apply URL or a website, resolve step 1 is dead weight and the slice rests on seed + careers pages.
2. If the companies Andrew actually targets mostly sit on Workday or hand-rolled boards, the ceiling of the slice is far below "ATS becomes the vacancy feed".
3. If ATS boards do not return descriptions in the list response, every new vacancy costs an extra request, which changes both runtime shape and scoring cost.

None of these are answerable from a chair. All three are an evening of poking at data already on disk.

## Measurements

### M1 — aggregator payload coverage

Over the raw aggregator responses already stored, per aggregator:

- % of hits carrying an `apply_url`;
- % carrying a company website;
- % whose `apply_url`, **after following redirects**, lands on `greenhouse.io` / `lever.co` / `ashbyhq.com`;
- % carrying neither a usable URL nor a distinct company name (anonymous / "Confidential" posts).

Redirects matter: short links (`grnh.se`) and aggregator tracking wrappers hide the real host. Count a raw-host match and a post-redirect match separately — the gap between the two numbers is exactly the value of adding redirect-following.

**Closes `GAP-1` (resolve step order) and `GAP-2` (careers parser needed at all).**

Decision gates:

| Post-redirect ATS hit rate | Consequence for Phase 1 |
| --- | --- |
| >50% | `APPLY_URL` is the primary resolve path; order as written below |
| 10–30% | Seed + careers page are primary; `APPLY_URL` is a bonus; reorder resolve |
| <10% | Discovery via aggregators is not worth automating. Slice shrinks to "hand-maintained company table + board polling", and the discover pass stays aggregator-vacancy-only |

### M2 — ATS reachability of real targets

Take ~20 companies Andrew already knows and wants (from the job tracker and the seed file). For each: which ATS, if any.

Produces the realistic ceiling. If half are on Workday, "ATS becomes the vacancy feed" is false for this profile no matter how good the resolve logic is, and `Out of scope` needs revisiting rather than the code.

**Closes `GAP-3` (ceiling, and whether a fourth fetcher belongs in this slice after all).**

### M3 — board response shape

For one live slug per ATS, by hand:

- does the list response include the full description, or only a title + link;
- if not, what the per-vacancy fetch costs (one request each? rate limits?);
- does the board send `ETag` / `Last-Modified`;
- what an unknown slug returns (404? 200 with an empty list? HTML?);
- what a company with zero open vacancies returns — **this is the one that matters**, because "empty board" and "dead slug" must not look the same to the poller.

**Closes `GAP-4` (whether a description-fetch step exists in the core loop) and `GAP-5` (conditional polling).**

### M4 — recruiting agencies in the current data

Eyeball the distinct company names already collected. How many are staffing agencies, outsourcers or anonymous posts?

An agency has a real, large Greenhouse board. Without a blocklist, resolve succeeds on it and the poller happily imports hundreds of other people's vacancies. The number here decides whether the blocklist can be a hand-written list or needs a heuristic.

### M5 — key collision check

Over the current `jobs.company_key` values: how often do two keys clearly mean one company (name-form vs domain-form, `Acme` vs `Acme Inc`, `acme.com` vs `acme.de`)?

Confirms how much merge logic Phase 1 step 1 actually needs.

## What Andrew does

- [ ] **M1** — SQL / script over stored aggregator payloads; redirect-following needs an actual HTTP pass over a sample of apply URLs, not a string match.
- [ ] **M2** — manual, ~20 companies, 30 minutes.
- [ ] **M3** — manual, three live slugs, curl.
- [ ] **M4** — eyeball distinct names, mark agencies.
- [ ] **M5** — SQL over existing keys.
- [ ] Write findings into `rnd-findings.md` as plain numbers.
- [ ] Decide the four deferred items below (they need judgement, not data).

## Decisions that are Andrew's, not the data's

These block nothing and have working defaults already written into Phase 1. Overrule them if they feel wrong.

| Open item | Default taken in Phase 1 |
| --- | --- |
| Where `domains` comes from for a new company | From the profile that discovered it, accumulated |
| `applied_companies` source of truth | `andrew.yaml`; the xlsx tracker stays a personal file, synced by hand |
| `tech_tags` recompute scope | Only companies touched in this run |
| "Known company + matching stack" score signal | **Deferred out of this slice** — `tech_tags` are computed from that same company's vacancies, so the signal largely restates the vacancy's own stack match. Revisit with real data |

---

# Phase 1 — implementation slice

Everything below is the slice as it should be built. Gap markers show where Phase 0 output lands.

## Flow (one run)

```text
aggregators (search)
    → extract company name (+ website / apply_url when present)
    → blocklist / anonymous check          ← new, before any insert
    → insert company if new (PENDING) / merge into existing row / no-op if known
    → resolve PENDING → ats + ats_slug (verified) → RESOLVED
    → poll every RESOLVED board due for polling
    → fetch vacancy text where the board does not inline it   ← GAP-4
    → upsert jobs from ATS (ATS text and location win on dedup)
    → close jobs that vanished from a successful poll
    → UpdateCompanyTags (touched companies only)
    → score (description-hash dedup, cheap gates before LLM)
```

Seed import is CLI/make, outside the run button. `db-dump` / `db-restore` already live in the Makefile (`data/local/`, gitignored) — this slice assumes they exist.

## Stack

| Layer | Tool | Module |
| ----- | ---- | ------ |
| Persistence | PostgreSQL + GORM + migrations | `internal/companies/storage` |
| Survive local wipes | already via `make db-dump` / `db-restore` | reuse; no second dump path |
| Orchestration | Temporal | `internal/companies/workflows` |
| ATS fetch | `net/http` (JSON boards) + goquery (careers → ATS links only) | `internal/collectors/ats/{greenhouse,lever,ashby}` |
| Tagging | dictionary `Tagger` | `internal/dictionary` |
| Applied highlight | `applied_companies` in profile YAML | `config/profiles/andrew.yaml` + profile API + frontend |

**No LLM inside this slice's own logic** — no extraction, no resolve-by-model. Scoring remains the existing LLM step downstream; this slice does not add LLM calls, but it does own the rules that keep their *count* from exploding (see Scoring below), because it is what changes the volume.

No headless browser for ATS boards. Hand-rolled careers HTML is never a vacancy parser — only a place to hunt ATS links during resolve.

## Company

**Key — fixed at birth.** The key is assigned when the row is created (website domain if known at that moment, else normalized name) and **never changes afterwards**. A later-discovered website is written to the `website` field, not promoted to key.

Normalization, same rule as `jobs.company_key` and applied-name matching: lowercased; `Inc` / `GmbH` / `Ltd` / `LLC` / `SRL` stripped. Domains additionally drop `www.` and `careers.` prefixes.

**Merge instead of insert.** Before inserting, look up by key, by `website`, and by `aliases`. On a hit, merge the new facts into the surviving row and record the other form in `aliases`; never create a second row. Without this, a company found by name on Monday and by domain on Tuesday becomes two companies — and both get polled.

**Fields:** name, `aliases`, website, `ats`, `ats_slug`, `board_url`, `board_etag`, `resolution_status`, `resolution_method`, `poll_state`, `next_resolve_at`, `domains` (`software` / `architecture`), `tech_tags` (tag → count over vacancies in the last 30 days), `first_seen_at`, `last_seen_at`, `last_polled_at`, `poll_fail_count`.

**Two independent statuses.** They answer different questions and must not share a column:

- `resolution_status`: `PENDING` → `RESOLVED` | `UNRESOLVED` — *did we find a board?*
- `poll_state`: `ACTIVE` | `INACTIVE` — *is that board producing vacancies?*

A company whose board goes quiet becomes `RESOLVED` + `INACTIVE`, keeping its verified slug. Collapsing these into one field throws the slug away and forces a full re-resolve when hiring resumes.

**Blocklist.** Staffing agencies, outsourcers and anonymous posts are never created as companies. Their vacancies are still stored, carrying whatever the aggregator gave. An agency's Greenhouse board is real and large, and polling it imports hundreds of other people's jobs. List lives next to the seed file; size and form per **M4**.

Companies are never deleted by retention. The company row outlives its vacancies.

## Manual seed (`config/companies/companies_template.xlsx`)

One-shot via `cmd/companies-import`, then the file is archive only.

| Seed | Behavior |
| ---- | -------- |
| Name (+ website) | Insert; skip example / `Fit=SKIP` |
| ATS + slug (Greenhouse / Lever / Ashby) | Fetch board; vacancies present → `RESOLVED`; 404/empty → `PENDING` without slug |
| Custom / hand-rolled URL | No vacancy HTML parse; `PENDING` → normal resolve (ATS hunt only) |

If **M1** lands in the <10% band, this file stops being a bootstrap and becomes the main intake — worth knowing before deciding how much care its import deserves.

## Applied companies (Andrew profile)

Not part of the `companies` table. Display only.

- Source of truth: the list on **`config/profiles/andrew.yaml`**. `config/companies/job_tracker.xlsx` is Andrew's personal tracker, copied across by hand. Two live sources would drift.
- Not in Postgres. Lada has no list.
- Matcher tries normalized name **and** domain — an applied entry written as a name must still match a company whose key is a domain.
- Profile API exposes the list; job list may also carry `applied: bool` from the same matcher.
- UI: color highlight only — never auto-`HIDDEN`.
- Tracker statuses (отказ / интервью / …) stay out; names only.

## Local persistence

Already shipped: `make db-dump` / `make db-restore`, gitignored `data/local/`, restore-then-migrate. This slice does not re-implement it. Once `companies` exist, the same full dump covers them; companies also rebuild from seed + discovery if a dump is missing.

## Runtime (inside `ProfileRunWorkflow`)

**Discover (aggregator pass):** for each aggregator hit, blocklist-check, then merge-or-insert the company. Aggregator cards are not the vacancy source for a company that is already `RESOLVED` **and** polling fine. Aggregator vacancy text still lands while the company is `PENDING` / `UNRESOLVED`, and also whenever a poll of a `RESOLVED` company returned nothing — see the fallback rule below.

**`ResolveCompanyWorkflow`** — `PENDING` only, one active resolve per company (`workflow ID = resolve-{company_key}`), stop at first verified slug:

1. **Parse apply URL** — follow redirects first, then match Greenhouse / Lever / Ashby URL shapes → `resolution_method = APPLY_URL`.
2. **Careers page** `{website}/careers`, `{website}/jobs` — ATS links / embeds only → `CAREERS_PAGE`.
3. **Guess + verify** — **deferred, not in the first build.** Slug candidates from the name, accepted only on evidence. Title overlap alone is too weak: aggregators rewrite titles, so it matches rarely, and where it does match (`Software Engineer`) it is generic enough to bind a different company with a similar name. If this step is revived, it needs two signals — board title overlap **and** the company name present on the board page. Until then, step 3 is "a human adds it to the seed file".
4. Else → `UNRESOLVED`, `next_resolve_at = now + 14d` (aggregator-only until then).

**`GAP-1`** — the order of steps 1–2 above assumes **M1** shows a healthy apply-URL rate. In the 10–30% band, careers-page hunting leads and apply-URL parsing becomes a cheap pre-check. **`GAP-2`** — if apply URLs cover nearly everything, step 2 may not be worth building at all.

**`PollCompaniesWorkflow`** — `RESOLVED` + `ACTIVE` companies whose `domains` intersect the running profile **and** that are due:

- per-company interval (6–24h) via `last_polled_at` — the existing field finally gets a rule;
- hard cap on polls per run, oldest-first, so a growing table lengthens the cycle instead of the run;
- conditional requests via `board_etag` where the board supports them (**`GAP-5`**, per **M3**);
- concurrency cap per ATS (~5);
- five consecutive empty-but-successful polls → `poll_state = INACTIVE`, re-checked weekly; a new aggregator card for that company also wakes it back to `ACTIVE`.

**Fetch vacancy text** — **`GAP-4`**: exists as a step only if **M3** shows boards return list-without-description. If so it is per new-or-changed vacancy, inside the per-company unit, and it is the dominant request cost of the slice.

**Fallback rule — never go silent.** If a `RESOLVED` company's successful poll returns zero vacancies while the aggregator is still showing jobs for it, do **not** suppress the aggregator path for that company. Log the discrepancy (`board_jobs=0, aggregator_jobs=N`) — that is the signal a slug has rotted, and suppressing it loses vacancies with nothing left to recover them from. Same applies to companies posting on a second ATS outside the supported three.

**Closing vacancies.** This is the real prize of polling a board: boards list only open roles. For a `RESOLVED` company, a vacancy absent from a **successful** poll is closed immediately. The 30-day `last_seen_at` window stays only for aggregator-sourced vacancies, where absence proves nothing.

**`UpdateCompanyTags`** — recount `tech_tags` over the 30-day vacancy window, for companies touched in this run only.

**Scoring** — existing gates / plus / penalties, unchanged in substance. What this slice adds, because it is what multiplies the volume:

- a description hash per vacancy; an upsert whose text is unchanged does not re-score;
- cheap deterministic gates (dictionary / stack / location) run **before** any LLM call;
- a ceiling on LLM calls per run, overflow deferred to the next run, oldest-first.

The "known company + matching stack" signal is **deferred** (see Phase 0 defaults).

**Run progress** — counters on the run row, not free-form logs: `companies_discovered`, `companies_merged`, `companies_blocked`, `resolved`, `polled`, `poll_failures`, `jobs_upserted`, `jobs_closed`, `scored`, `score_skipped_unchanged`. Without these there is no way to tell whether the flip is working.

## One company poll (core loop)

The unit of work. Sequential, **state committed only after the full chain succeeds**:

1. Load company; skip if not due.
2. Fetch board (conditional request where supported).
3. Fetch vacancy text for new-or-changed vacancies (**`GAP-4`**).
4. Upsert jobs; ATS text and location win on dedup.
5. Close jobs present last time and absent now.
6. Persist; `last_polled_at` set, `poll_fail_count` reset, run counters incremented.

Tagging and scoring run after the poll pass, not inside the unit — they are batch steps over touched companies.

## Failure at any step

A failed poll must never look like an empty board, or timeouts will march healthy companies into `INACTIVE`.

- **Transport failure** (timeout, 5xx, connection reset): Temporal retry; on exhaustion `poll_fail_count++`, company otherwise untouched, **no vacancy closed**. Closing requires a *successful* poll.
- **404 / unknown slug**: slug is suspect — `poll_fail_count++`, and at 3 in a row `resolution_status = PENDING` for a re-resolve, slug kept in `resolution_method` history.
- **200 with an empty list**: success, not failure. Feeds the `INACTIVE` counter and the fallback rule.
- **Parse failure on a changed board format**: terminal for that company this run, flagged loudly — it means a fetcher needs updating, not a retry.
- Business skips (blocklist hit, not due, no domain intersection) are never failures.

In logs: `failure_stage` = `DISCOVER` | `RESOLVE` | `POLL` | `FETCH_TEXT` | `UPSERT` | `TAG` | `SCORE`.

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| Apply URL on unknown host | follow redirects first; still unknown → fall through to careers |
| Company name on blocklist / anonymous | vacancy stored, **no company row** |
| Known company found again under another form | merge into existing row, record in `aliases`, never a second row |
| Seed slug board live | `RESOLVED` |
| Seed slug 404 / empty | `PENDING`, no slug |
| Custom careers | `PENDING` + ATS resolve only |
| Same job from ATS and aggregator | one row; **ATS text and location win** |
| Board empty five successful polls | `INACTIVE`, slug kept |
| `INACTIVE` company reappears on an aggregator | back to `ACTIVE` |
| `RESOLVED` board empty while aggregator still has jobs | keep aggregator path, log discrepancy |
| Job gone from a successful ATS poll | `CLOSED` immediately |
| Job gone from an aggregator | 30-day `last_seen_at` window as before |
| All company jobs expired | row kept; `tech_tags` → empty |
| Retention | `jobs` only, never `companies` |
| Paywalled aggregator | not intake, not vacancy source |
| Applied name in YAML | UI highlight only |

**Hard rules:**

- A company key never changes after creation.
- Slug only when verified: `APPLY_URL` — known ATS URL shape after redirects; seed — board fetch returns vacancies; `CAREERS_PAGE` — board live.
- At most one active resolve per company (`workflow ID = resolve-{company_key}`); unique index on key, insert via upsert. Two profiles running at once must not race.
- Vacancies are closed only on a successful poll.
- No LLM in this slice's own logic.

## Modules

| Module | Purpose |
| ------ | ------- |
| `internal/companies` | contract, resolve/poll use cases, merge + blocklist, storage, workflows |
| `internal/collectors/ats/{greenhouse,lever,ashby}` | board fetchers for this slice |
| `internal/dictionary` | `tech_tags` |
| `internal/jobs` | `company_id`, close-on-absent |
| `internal/profiles` + scoring | `applied_companies`; description-hash dedup, pre-LLM gates |
| `cmd/companies-import` | seed import + board verify |
| make `db-dump` / `db-restore` | **already in Makefile** — prerequisite, not work in this slice |

```go
type CompanyBoardFetcher interface {
	ATS() schema.ATS
	FetchBoard(ctx context.Context, slug string) ([]domain.Job, error)
}
```

## Layers

- **Saved** — company row: key, aliases, ats, slug, domains.
- **Run / session** — run counters, per-company poll outcome, `failure_stage`.
- **Wire** — board payloads and workflow inputs, in module `schema/`.

## Implementation order

Reordered: identity hygiene first. Everything downstream — resolve, polling, tags, scoring — builds on the company table, so merging and the blocklist cannot be bolted on later without a cleanup migration.

1. `companies` table with both status columns and the new fields; storage; `jobs.company_id`; **fixed key + merge-or-insert + aliases**.
2. **Blocklist** on the discover path.
3. Confirm `make db-dump` / `db-restore` still works after the companies migration (do not re-build dump tooling).
4. Seed import with board verify.
5. `applied_companies` on `andrew.yaml` + profile API + frontend highlight.
6. Resolve step 1 (apply URL, with redirect-following) — **order subject to `GAP-1`**.
7. Three ATS fetchers + `PollCompaniesWorkflow` with due-interval and per-run cap + wire the run so aggregators feed companies and ATS feeds vacancies.
8. Close-on-absent + the fallback rule + run counters.
9. Vacancy text fetch, if **`GAP-4`** says it is needed.
10. Scoring guards: description hash, pre-LLM gates, per-run ceiling.
11. Resolve step 2 (careers ATS hunt) — **only if `GAP-2` says it is needed**.
12. `UpdateCompanyTags` over touched companies.

Early check: one company, one seeded slug, one poll → vacancies upserted, counters moved, second poll with unchanged text scores nothing new.

## Prerequisites

- Phase 0 findings written down.
- `make db-dump` / `db-restore` already working before any destructive companies migration.
- Seed file and job-tracker names already under `config/companies/`.

## Out of scope

- Workday, SAP SuccessFactors, and any ATS board fetcher beyond **Greenhouse, Lever, Ashby** — revisit only if **M2** shows the supported three miss most real targets.
- Parsing hand-rolled careers HTML as a vacancy source.
- Crawling a whole company site for a careers page.
- LLM extraction.
- Guess + verify resolve (step 3) — deferred, see Runtime.
- "Known company + matching stack" score signal — deferred, see Phase 0 defaults.
- Company admin UI beyond debug `GET /api/v1/companies`.
- Re-fetching a known, healthily-polled company from aggregators for vacancies.
- Paying aggregators for jobs that sit on a public ATS.
- Applied statuses beyond a name list; Lada applied list.
- Extra company-intake sources (e.g. Hiring Cafe).
- Contracts / tasks markdown — written when Phase 1 implementation starts.
