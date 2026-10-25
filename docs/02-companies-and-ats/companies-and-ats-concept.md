# Company database and direct ATS polling — concept

**Status:** Phase 0 — **done 2026-10-25** ([`rnd-findings.md`](rnd-findings.md)), Phase 1 — not started.

> **Sequence 2.**
> Two phases in one file. **Phase 0** was measurement — no production code, output is numbers that close the gaps marked `GAP-n` below. **All five gaps are now closed**; each marker carries its resolved value, and the numbers behind it live in [`rnd-findings.md`](rnd-findings.md). **Phase 1** is the implementation slice.
> Contracts and tasks files are intentionally absent until Phase 1 is picked up for implementation.

## Goal

Flip the vacancy pipeline.

| Role | Before | After this slice |
| ---- | ------ | ---------------- |
| Aggregators (incl. fund boards already in the profile) | Vacancy feed | **Company-name intake only** — write / refresh company rows, hand over apply URL / website for resolve |
| Popular ATS (Greenhouse, Lever, Ashby) | Unused | **Vacancy feed** — poll every `RESOLVED` company board |
| Manual seed | Spreadsheet on disk | One-shot bootstrap of the company table |

**End state (Done):** one profile run (1) hits aggregators only to discover companies not yet stored, (2) resolves ATS + slug for `PENDING` rows, (3) polls all `RESOLVED` companies on Greenhouse / Lever / Ashby, (4) upserts and **closes** vacancies from those boards, (5) scores them. Local DB survives Docker wipes.

**Single-domain.** The `companies` table serves `domain: software` only. The architecture boards (Dezeen, RIBA, Baunetz, Competitionline, …) host no Greenhouse / Lever / Ashby companies, so `lada` is untouched by this slice and there is no `domains` column. If architecture ever gains ATS companies, the column arrives with a migration.

**Done is measurable, not a feature list:**

- ≥70% of non-blocklisted companies in `RESOLVED`;
- **TOP + REVIEW per LLM call rises**, and at least some TOP arrive from the ATS path that no aggregator showed at all;
- LLM scoring calls per run no higher than before the flip (dedup by description hash works);
- a full `make db-restore` on an empty volume brings back jobs and scores (**dump/restore already ships via Makefile `db-dump` / `db-restore`**, not this slice).

> Raw vacancy volume is deliberately **not** a Done criterion. The baseline is already 1600 stored vacancies yielding 13 TOP, so "more vacancies from ATS than from aggregators" would be satisfied by importing thousands of irrelevant board rows. Measure yield per call, not throughput.

Primary entity: **Company**.

## Why

Aggregators lag, truncate descriptions, and miss companies that never post there. The apply URL they already return usually points at Greenhouse, Lever or Ashby — enough to leave the aggregator as a name source and take vacancies from the ATS board.

That last sentence is the bet of the whole slice. **Phase 0 confirmed it**: 42% of stored vacancies and 33% of stored companies resolve to a big-3 board from the raw apply URL, 47% of the hand-picked target list sits on one, and the 23 fund-board sources produced 0 TOP between them while the 9 text-carrying aggregators produced all 13. Fund boards are intake, not a feed.

---

# Phase 0 — RnD

**Done. Numbers in [`rnd-findings.md`](rnd-findings.md).** The measurement definitions below are kept as the record of what was asked; each one now states its result.

No schema, no workflows, no production code. Throwaway scripts and SQL over data already collected are fine. Output is a short numbers file: [`rnd-findings.md`](rnd-findings.md) (in this folder).

## Why this comes first

Three things could make Phase 1 as written wrong rather than merely incomplete:

1. If aggregators rarely hand over an apply URL or a website, resolve step 1 is dead weight and the slice rests on seed + careers pages. → **Not the case.** Consider boards carry an apply URL on ~100% of rows and a company domain on 100%.
2. If the companies Andrew actually targets mostly sit on Workday or hand-rolled boards, the ceiling of the slice is far below "ATS becomes the vacancy feed". → **Not the case.** Zero Workday in the 64-company target list, 2.3% in the whole corpus. The residue is Custom/unknown boards at 31% of the target list.
3. If ATS boards do not return descriptions in the list response, every new vacancy costs an extra request, which changes both runtime shape and scoring cost. → **Not the case.** All three inline the full description in one request.

None of these are answerable from a chair. All three are an evening of poking at data already on disk.

## Measurements

### M1 — aggregator payload coverage

Over the raw aggregator responses already stored, per aggregator:

- % of hits carrying an `apply_url`;
- % carrying a company website;
- % whose `apply_url`, **after following redirects**, lands on `greenhouse.io` / `lever.co` / `ashbyhq.com`;
- % carrying neither a usable URL nor a distinct company name (anonymous / "Confidential" posts).

Redirects matter: short links (`grnh.se`) and aggregator tracking wrappers hide the real host. Count a raw-host match and a post-redirect match separately — the gap between the two numbers is exactly the value of adding redirect-following.

**`GAP-1` CLOSED (resolve step order) and `GAP-2` CLOSED, narrowed (careers parser needed at all).**

**Result.** Raw host, no redirects: big-3 = 42% of 1600 vacancies, 33% of 857 companies. Per source, Consider boards hit 50–84% and carry a company domain on 100% of rows. Redirect pass over a 110-host sample of the unknown residue: **1 of 157 URLs became big-3**, 49 became another ATS, 107 stayed unknown. Decision gate → the **`>50%` band** for the intake sources that matter.

⇒ `APPLY_URL` is the primary resolve path; step order 1→2 as written below.
⇒ **Redirect-following is near-worthless as a resolve strategy** (1/157) — keep it as cheap URL normalisation, do not build the slice around it.
⇒ `GAP-2` narrowed: `himalayas.app` is the largest single unknown host (296 jobs, 18% of the corpus) and **25 of 25 sampled URLs do not redirect at all** — the outbound ATS link sits in the page markup. So resolve step 2 **is** needed, but as one narrow parser against the Himalayas detail page, **not** the generic `{website}/careers` crawler described below.

Decision gates (kept for the record; the `>50%` row is the one that fired):

| Post-redirect ATS hit rate | Consequence for Phase 1 |
| --- | --- |
| **>50% ← actual, for Consider/Getro** | `APPLY_URL` is the primary resolve path; order as written below |
| 10–30% | Seed + careers page are primary; `APPLY_URL` is a bonus; reorder resolve |
| <10% | Discovery via aggregators is not worth automating. Slice shrinks to "hand-maintained company table + board polling", and the discover pass stays aggregator-vacancy-only |

### M2 — ATS reachability of real targets

Take ~20 companies Andrew already knows and wants (from the job tracker and the seed file). For each: which ATS, if any.

Produces the realistic ceiling. If half are on Workday, "ATS becomes the vacancy feed" is false for this profile no matter how good the resolve logic is, and `Out of scope` needs revisiting rather than the code.

**`GAP-3` CLOSED (ceiling, and whether a fourth fetcher belongs in this slice after all).**

**Result.** `config/companies/companies_template.xlsx` turned out to hold 64 hand-picked companies, all `Fit=FIT` — far more than the ~20 this measurement asked for. big-3 = **30 of 64 (47%)**: Ashby 20, Greenhouse 7, Lever 3. **Workday = 0.** Custom/UNKNOWN/NONE = 20 (31%). Teamtailor 4, Workable 3, Recruitee 2, Personio 2, Rippling/Kula/Notion 3.

⇒ Ceiling is 47% with the supported three. **No fourth fetcher in this slice**; `Out of scope` stands as written. Workable + Teamtailor + Recruitee + Personio would reach 66% and is a second iteration.
⇒ All 30 big-3 slugs verified live (30/30 `200`, 542 open vacancies in 30 requests). **4 of 30 slugs are not derivable from the company name** (`541` for Form3, `primer.io`, `ecosia.org`, and the case-significant `Coder` / `Deepnote` / `Mastra`) — this confirms deferring "guess + verify" (resolve step 3) and adds a hard rule: **never normalise a slug.**

### M3 — board response shape

For one live slug per ATS, by hand:

- does the list response include the full description, or only a title + link;
- if not, what the per-vacancy fetch costs (one request each? rate limits?);
- does the board send `ETag` / `Last-Modified`;
- what an unknown slug returns (404? 200 with an empty list? HTML?);
- what a company with zero open vacancies returns — **this is the one that matters**, because "empty board" and "dead slug" must not look the same to the poller.

**`GAP-4` CLOSED — negatively, the step does not exist. `GAP-5` CLOSED — positively, all three support it.**

**Result.**

| Question | Greenhouse | Ashby | Lever |
| --- | --- | --- | --- |
| Full description in the list response | **only with `?content=true`** (8.2 KB → 185 KB for 14 jobs) | yes, `descriptionPlain` | yes, `descriptionPlain` |
| Per-vacancy fetch needed | no | no | no |
| `ETag` + `If-None-Match` → `304` | yes | yes | yes |
| Unknown slug | `404` JSON | `404` **plain text** `Not Found` | `404` JSON |
| Zero open vacancies | `200` `{"jobs":[]}` | `200` `{"jobs":[],"apiVersion":"1"}` | `200` `[]` |

⇒ **No `FETCH_TEXT` step in the core loop.** The dominant request cost the concept feared does not exist.
⇒ **Empty board and dead slug are unambiguous: `200` vs `404`.** Live examples appeared during slug verification — Prismic (Lever) and Ecosia (Ashby) both returned `200` with an empty list.
⇒ Greenhouse's list request **must** carry `?content=true`, otherwise it silently returns titles only and scoring finds nothing.
⇒ Ashby's `404` body is **not JSON** — check the status code before decoding, or a dead slug surfaces as a parse error and blames the wrong failure stage.
⇒ All three return `work_mode` and hiring geography as **structured fields** (`isRemote` / `workplaceType` / `secondaryLocations[]` / `categories.allLocations[]` / `offices[]`), plus `applyUrl` and timestamps. Those are the two enrich facts that most often come back `unknown`. Out of scope for this slice, but it is a second reason the flip pays.
⇒ Payloads are large (`ashby` 2.0 MB, `alpaca` 902 KB, `n8n` 895 KB; ≈9 MB per 30-company cycle). The per-run poll cap is a worker **memory** limit, not only board politeness — decode streaming.

### M4 — recruiting agencies in the current data

Eyeball the distinct company names already collected. How many are staffing agencies, outsourcers or anonymous posts?

An agency has a real, large Greenhouse board. Without a blocklist, resolve succeeds on it and the poller happily imports hundreds of other people's vacancies. The number here decides whether the blocklist can be a hand-written list or needs a heuristic.

**Result.** Name-token pass over 857 distinct keys: **13 companies, 20 jobs** (1.5% / 1.25%), largest `aida recruitment` at 5. Adding bare `consulting` catches `pradeepit consulting services pvt` (18 jobs). Staff-augmentation brands with no token in the name must be listed by hand: `lemon.io` (9 jobs), `proxify`.

⇒ **A hand-written blocklist is enough** — the entry list is ~15 names, no heuristic needed as the mechanism.
⇒ The footprint looks small only because aggregators expose a slice of an agency's board; the real risk is unmeasurable until the poller exists. Keep the token list as a **quarantine-for-review signal**, not an auto-block.

### M5 — key collision check

Over the current `jobs.company_key` values: how often do two keys clearly mean one company (name-form vs domain-form, `Acme` vs `Acme Inc`, `acme.com` vs `acme.de`)?

Confirms how much merge logic Phase 1 step 1 actually needs.

**Result.** 857 distinct keys: 828 name-form, 29 domain-form, and exactly **1 name/domain pair for one company** (`yld` vs `yld.com`). Domain-form keys come almost entirely from Consider's `companyDomain`.

⇒ Merge logic needs almost nothing **today** — but only because 97% of keys are name-form for lack of a domain source. Once companies are polled, every one has a board and usually a domain, so the domain-form share and the collisions both rise. Keep merge-or-insert and `aliases` (retrofitting needs a cleanup migration); the cost now is near zero.
⇒ **Do not add fuzzy name matching.** Real near-duplicate pairs that are different companies: `cloudbees` / `cloudbeds`, `insider` / `insider one`, `lemon.io` / `lemon.markets`, `betterme` / `betterment`, `creatio` / `creative chaos`. Match exact normalised name, exact domain, and `aliases` only.

## What Andrew does

- [x] **M1** — done. SQL over stored rows plus a real redirect pass (110-host sample). The full 538-URL pass is still open, but the sample result (1/157) makes it low value.
- [x] **M2** — done from `config/companies/companies_template.xlsx` (64 companies) plus live verification of all 30 big-3 slugs.
- [x] **M3** — done, three live slugs by curl, including the empty-board and unknown-slug cases.
- [x] **M4** — done as a name-token pass; the per-board magnitude stays unmeasurable until the poller exists.
- [x] **M5** — done, SQL over existing keys.
- [x] Write findings into [`rnd-findings.md`](rnd-findings.md) as plain numbers.
- [ ] Decide the four deferred items below (they need judgement, not data).
- [ ] Check why `consider_panoramic_ventures`, `getro_point_nine`, `getro_earlybird` and `getro_firstminute` have zero rows in `jobs` despite being listed in `andrew.yaml` — fix or remove. Not part of this slice, but it skews every source-level number above.

## Decisions that are Andrew's, not the data's

These block nothing and have working defaults already written into Phase 1. Overrule them if they feel wrong.

| Open item | Default taken in Phase 1 |
| --- | --- |
| Where `domains` comes from for a new company | **Resolved: no `domains` column.** The slice is single-domain (`software`); architecture boards have no big-3 companies |
| `applied_companies` source of truth | **Resolved: dropped from the slice entirely.** No applied list, no UI highlight — handled by hand outside the product |
| `tech_tags` recompute scope | **Resolved: dropped from the slice entirely.** `tech_tags` feed only the deferred "known company + matching stack" signal, so in Phase 1 the column would be written and never read |
| "Known company + matching stack" score signal | **Deferred out of this slice** — it would be computed from that same company's vacancies, so the signal largely restates the vacancy's own stack match. When it is revived, `tech_tags` come back with it, in whatever shape the signal actually needs |

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
      (one request per company; the board inlines the full text — GAP-4 closed,
       no separate fetch step. Greenhouse needs ?content=true)
    → upsert jobs from ATS (ATS text and location win on dedup)
    → close jobs that vanished from a COMPLETE poll
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

**No LLM inside this slice's own logic** — no extraction, no resolve-by-model. Scoring remains the existing LLM step downstream; this slice does not add LLM calls, but it does own the rules that keep their *count* from exploding (see Scoring below), because it is what changes the volume.

No headless browser for ATS boards. Hand-rolled careers HTML is never a vacancy parser — only a place to hunt ATS links during resolve.

## Company

**Key — fixed at birth.** The key is assigned when the row is created (website domain if known at that moment, else normalized name) and **never changes afterwards**. A later-discovered website is written to the `website` field, not promoted to key.

Normalization, same rule as `jobs.company_key` and applied-name matching: lowercased; `Inc` / `GmbH` / `Ltd` / `LLC` / `SRL` stripped. Domains additionally drop `www.` and `careers.` prefixes.

**The `ats_slug` is never normalized.** It is stored and sent byte-for-byte as seeded or resolved. Per **M2**, lowercasing breaks Ashby (`Coder`, `Deepnote`, `Mastra`) and stripping dots breaks `primer.io` and `ecosia.org`. Key normalization above applies to the company key and the website — not to the slug.

**Merge instead of insert.** Before inserting, look up by key, by `website`, and by `aliases`. On a hit, merge the new facts into the surviving row and record the other form in `aliases`; never create a second row. Without this, a company found by name on Monday and by domain on Tuesday becomes two companies — and both get polled.

**Fields:** name, `aliases`, website, `ats`, `ats_slug`, `board_url`, `board_etag`, `resolution_status`, `resolution_method`, `poll_state`, `next_resolve_at`, `first_seen_at`, `last_seen_at`, `last_polled_at`, `poll_fail_count`.

No `domains` and no `tech_tags` — see the decisions table above.

**Two independent statuses.** They answer different questions and must not share a column:

- `resolution_status`: `PENDING` → `RESOLVED` | `UNRESOLVED` | `UNSUPPORTED_ATS` — *did we find a board we can poll?*
- `poll_state`: `ACTIVE` | `INACTIVE` — *is that board producing vacancies?*

A company whose board goes quiet becomes `RESOLVED` + `INACTIVE`, keeping its verified slug. Collapsing these into one field throws the slug away and forces a full re-resolve when hiring resumes.

**`UNSUPPORTED_ATS`** is added on the strength of Phase 0: an ATS was recognised but is not one of the supported three. It stores the recognised `ats` value and is **not** retried on the 14-day `UNRESOLVED` cycle, because nothing about that company will change until a fetcher is written. Without this state those companies re-enter `PENDING` forever and burn resolve attempts. The size of the bucket is also the counter that says when a fourth fetcher pays: **2.3% Workday and 3.3% other ATS** in the current 1600-job corpus, **19%** of the 64-company seed list, and the redirect pass found 49 more per 157 URLs.

**Blocklist.** Staffing agencies, outsourcers and anonymous posts are never created as companies. Their vacancies are still stored, carrying whatever the aggregator gave. An agency's Greenhouse board is real and large, and polling it imports hundreds of other people's jobs. The list ships as **`config/companies/blocklist.yaml`**, drafted from the live DB: `exact` names (never fuzzy), `anonymous` placeholders, and `review` substring patterns that flag for manual review without blocking.

Per **M4** the list is **hand-written, ~15 entries** — token clusters (`talent*`, `*recruitment`, `people profilers*`) plus brands that carry no token (`lemon.io`, `proxify`). The token regex is a **review signal, not an auto-block**: after a poll, a company with an unusually high vacancy count and a wide stack spread goes to manual review rather than being blocked automatically.

Companies are never deleted by retention. The company row outlives its vacancies.

## Manual seed (`config/companies/companies_template.xlsx`)

One-shot via `cmd/companies-import`, then the file is archive only.

| Seed | Behavior |
| ---- | -------- |
| Name (+ website) | Insert; skip example / `Fit=SKIP` |
| ATS + slug (Greenhouse / Lever / Ashby) | Fetch board; `200` → `RESOLVED` (**an empty list is still `RESOLVED`** — the slug is verified, the company just has nothing open); `404` → `PENDING` without slug |
| Recognised but unsupported ATS (Workday, Teamtailor, Workable, Recruitee, Personio, Rippling…) | `UNSUPPORTED_ATS`, store the `ats` value, no slug verify, no 14-day retry |
| Custom / hand-rolled URL | No vacancy HTML parse; `PENDING` → normal resolve (ATS hunt only) |

The earlier row said "404/empty → `PENDING` without slug". **M3 corrected this**: `200` + empty list and `404` are different answers and must not be collapsed, or every seeded company that happens to be between hiring rounds loses its verified slug on import. Prismic and Ecosia both behave this way right now.

The file holds **64 companies, all `Fit=FIT`** — not the handful this section assumed. **M1 landed in the `>50%` band**, so the file stays a bootstrap rather than becoming the main intake; but 30 of its 64 companies are immediately pollable with verified slugs, which makes the import the shortest path to a working poll loop.

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

1. **Parse apply URL** — match Greenhouse / Lever / Ashby URL shapes on the raw host first; follow redirects only when the raw host misses → `resolution_method = APPLY_URL`. A recognised but unsupported host → `UNSUPPORTED_ATS`, stop.
2. **Aggregator detail page** — for the aggregators whose apply URL points back at themselves, extract the outbound ATS link from the listing page → `resolution_method = AGGREGATOR_PAGE`. Today this means `himalayas.app` and nothing else.
3. **Careers page** `{website}/careers`, `{website}/jobs` — ATS links / embeds only → `CAREERS_PAGE`.
4. **Guess + verify** — **deferred, not in the first build.** Slug candidates from the name, accepted only on evidence. Title overlap alone is too weak: aggregators rewrite titles, so it matches rarely, and where it does match (`Software Engineer`) it is generic enough to bind a different company with a similar name. If this step is revived, it needs two signals — board title overlap **and** the company name present on the board page. Until then, step 4 is "a human adds it to the seed file".
5. Else → `UNRESOLVED`, `next_resolve_at = now + 14d` (aggregator-only until then).

**`GAP-1` CLOSED** — **M1** landed in the `>50%` band for the intake sources that matter, so the order above stands with apply-URL first. Redirect-following moved from "first" to "only on a miss": it converted **1 of 157** sampled unknown URLs to a big-3 board, so it does not deserve a request on the happy path.

**`GAP-2` CLOSED, narrowed** — step 2 is new and step 3 shrank. `himalayas.app` carries 296 jobs (18% of the corpus) and **never redirects** (0 of 25 sampled), so its own detail page is the only place its ATS link exists; that is one parser against one known page, which is worth building. The generic `{website}/careers` crawler of step 3 is worth far less than the concept assumed and can wait — see Implementation order.

⇒ **Step 4's deferral is now evidence-backed, not caution:** 4 of 30 verified slugs (`541`, `primer.io`, `ecosia.org`, case-significant `Coder`/`Deepnote`/`Mastra`) cannot be derived from a company name at all.

**`PollCompaniesWorkflow`** — `RESOLVED` + `ACTIVE` companies that are due (no domain filter; the slice is single-domain):

- per-company interval (6–24h) via `last_polled_at` — the existing field finally gets a rule;
- hard cap on polls per run, oldest-first, so a growing table lengthens the cycle instead of the run. **M3** made this a memory limit too: payloads run 2.0 MB (`ashby`), 902 KB (`alpaca`), 895 KB (`n8n`), ≈9 MB for a 30-company cycle. Decode streaming, do not hold whole bodies;
- conditional requests via `board_etag` — **`GAP-5` CLOSED, all three send `ETag` and honour `If-None-Match` with `304`**, so this is unconditional, not best-effort;
- one request per company, `?content=true` on Greenhouse (without it the board returns titles only and nothing scores);
- check the HTTP status **before** decoding: Ashby answers an unknown slug with plain-text `Not Found`, not JSON;
- concurrency cap per ATS (~5);
- five consecutive empty-but-successful polls → `poll_state = INACTIVE`, re-checked weekly; a new aggregator card for that company also wakes it back to `ACTIVE`.

**Fetch vacancy text** — **`GAP-4` CLOSED: this step does not exist.** All three boards inline the full description in the list response (`content` with `?content=true` on Greenhouse, `descriptionPlain` on Ashby and Lever, 8–12k chars). The "dominant request cost of the slice" the concept feared is gone, and the per-company unit is one request.

**Structured fields come free.** The boards return `work_mode` and hiring geography as data (`isRemote` / `workplaceType` / `secondaryLocations[]` / `categories.allLocations[]` / `offices[]`), plus `applyUrl` and publish timestamps. Those are the two enrich facts that currently come back `unknown` most often. Mapping them is **not** in this slice, but do not throw them away in the fetcher's schema — a later slice will want them.

**Fallback rule — never go silent.** If a `RESOLVED` company's successful poll returns zero vacancies while the aggregator is still showing jobs for it, do **not** suppress the aggregator path for that company. Log the discrepancy (`board_jobs=0, aggregator_jobs=N`) — that is the signal a slug has rotted, and suppressing it loses vacancies with nothing left to recover them from. Same applies to companies posting on a second ATS outside the supported three.

**Closing vacancies.** This is the real prize of polling a board: boards list only open roles. For a `RESOLVED` company, a vacancy absent from a **complete** poll is closed immediately. The 30-day `last_seen_at` window stays only for aggregator-sourced vacancies, where absence proves nothing.

**"Complete", not merely "successful".** This is the one place where a wrong predicate destroys data instead of losing it. A poll that read 3 of 5 pages with `200` on every request looks successful and would close every live vacancy on the unread pages. A poll counts as complete only when the fetcher reached the documented end of the board — pagination exhausted, no truncation, and the parsed count consistent with whatever total the board reports. Anything short of that is a successful **read** that closes nothing. A `304` is complete by definition and also closes nothing, because nothing changed.

**A closed vacancy must survive the next aggregator pass.** `jobs` has **no status column today**, so closing needs one: `closed_at` plus `closed_by`. Without the second field the feature silently undoes itself, because the vacancy id is content-derived (`company_key` + normalized title) and aggregators lag:

1. Run N — the board no longer lists the vacancy, a complete poll closes it.
2. Run N+1 — the aggregator, still lagging, returns the same vacancy. Ingest upserts the same row and moves `last_seen_at`.
3. The close is reverted by a source that cannot know whether the role is open.

Rule: **an aggregator upsert never clears a close written by `ATS`.** It may still refresh `last_seen_at` and union `openings`, but `closed_at` stays. Only the vacancy reappearing in a *complete* poll of that company's board reopens it (`closed_at` back to null). Aggregator-sourced vacancies keep behaving as they do now — no close, just the 30-day `last_seen_at` window — and a close written for them, if ever, carries `closed_by = AGGREGATOR` and is not protected.

The API must also decide what a closed vacancy does to the job list. Default for this slice: closed vacancies are excluded from the default list the same way `HIDDEN` is, so the user is not shown roles that no longer exist. Scoring does not re-score them.

**Scoring** — existing gates / plus / penalties, unchanged in substance. What this slice adds, because it is what multiplies the volume:

- **reuse the existing text hash — do not add a second one.** `job_verdicts` is already keyed `(job_id, prompt_version, model, input_hash)` with NFC normalisation, so "an upsert whose text is unchanged does not re-score" is a matter of reading that hash, not introducing a parallel `description_hash` that can disagree with it;
- cheap deterministic phrase gates (stack / location) run **before** any LLM call;
- a ceiling on LLM calls per run, overflow deferred to the next run, oldest-first;
- **a minimum description length before a job is eligible for enrich at all.** Phase 0: 487 of 1600 jobs (30%) carry joined skill labels averaging 234 chars, and on those the model answers `unknown` for `hiring_scope` 100%, `work_authorization` 100%, `compensation` 100%, `work_mode` 84%. The threshold belongs on the enrich service, **not** in profile YAML — `ListMissing` is called with no profile id and `MissingIDs` reads every job, so a per-profile value would let one profile decide another's enrich and would desynchronise `facts_status`. This item is small enough to ship before the rest of the slice.

The "known company + matching stack" signal is **deferred** (see Phase 0 defaults).

**Run progress** — counters on the run row, not free-form logs: `companies_discovered`, `companies_merged`, `companies_blocked`, `resolved`, `polled`, `poll_failures`, `jobs_upserted`, `jobs_closed`, `scored`, `score_skipped_unchanged`. Without these there is no way to tell whether the flip is working.

## One company poll (core loop)

The unit of work. Sequential, **state committed only after the full chain succeeds**:

1. Load company; skip if not due.
2. Fetch board — conditional request via `board_etag`; `304` ends the unit with nothing changed. Greenhouse carries `?content=true`. Status checked before decoding.
3. Upsert jobs; ATS text and location win on dedup. Descriptions are already inline — there is no text-fetch step (**`GAP-4`** closed).
4. Close jobs present last time and absent now — **only if the poll was complete** (pagination exhausted).
5. Persist; `last_polled_at` and the new `board_etag` set, `poll_fail_count` reset, run counters incremented.

Tagging and scoring run after the poll pass, not inside the unit — they are batch steps over touched companies.

## Failure at any step

A failed poll must never look like an empty board, or timeouts will march healthy companies into `INACTIVE`.

- **Transport failure** (timeout, 5xx, connection reset): Temporal retry; on exhaustion `poll_fail_count++`, company otherwise untouched, **no vacancy closed**. Closing requires a *complete* poll.
- **404 / unknown slug**: slug is suspect — `poll_fail_count++`, and at 3 in a row `resolution_status = PENDING` for a re-resolve, slug kept in `resolution_method` history. Detect it by **status code**, not by a decode error: Ashby's 404 body is plain text, Greenhouse's and Lever's are JSON.
- **200 with an empty list**: success, not failure, and the slug stays verified. Feeds the `INACTIVE` counter and the fallback rule. **M3** confirmed all three distinguish this from a dead slug by status code alone.
- **Partial read** (pagination truncated, every request `200`): a successful read that is **not** complete. Jobs are upserted, **nothing is closed**, `last_polled_at` moves, `poll_fail_count` untouched, and the run counts it separately so a chronically truncating board is visible.
- **Parse failure on a changed board format**: terminal for that company this run, flagged loudly — it means a fetcher needs updating, not a retry.
- Business skips (blocklist hit, not due, no domain intersection, `UNSUPPORTED_ATS`) are never failures.

In logs: `failure_stage` = `DISCOVER` | `RESOLVE` | `POLL` | `UPSERT` | `TAG` | `SCORE`. (`FETCH_TEXT` dropped — **`GAP-4`** closed, the stage does not exist.)

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| Apply URL on big-3 host | `RESOLVED` without any extra request |
| Apply URL on a recognised non-big-3 host | `UNSUPPORTED_ATS`, store the `ats`, no 14-day retry |
| Apply URL on unknown host | follow redirects; still unknown → aggregator detail page, then careers |
| Apply URL points back at the aggregator itself | aggregator detail page parse (`himalayas.app` today); no redirect exists to follow |
| Company name on blocklist / anonymous | vacancy stored, **no company row** |
| Known company found again under another form | merge into existing row, record in `aliases`, never a second row |
| Two similar names, different companies | **no merge** — exact normalised name / domain / alias only, never fuzzy (`cloudbees` ≠ `cloudbeds`) |
| Seed slug returns `200` with vacancies | `RESOLVED` |
| Seed slug returns `200` with an empty list | **`RESOLVED`, slug kept** — verified board, nothing open |
| Seed slug returns `404` | `PENDING`, no slug |
| Slug casing or dots | stored and sent verbatim; **never normalised** |
| Custom careers | `PENDING` + ATS resolve only |
| Same job from ATS and aggregator | one row; **ATS text and location win** |
| Board empty five successful polls | `INACTIVE`, slug kept |
| `INACTIVE` company reappears on an aggregator | back to `ACTIVE` |
| `RESOLVED` board empty while aggregator still has jobs | keep aggregator path, log discrepancy |
| Job gone from a **complete** ATS poll | `closed_at` set, `closed_by = ATS` |
| Closed job returned by a lagging aggregator | **stays closed**; `last_seen_at` may move, `closed_at` does not clear |
| Closed job reappears in a complete ATS poll | `closed_at` cleared — the only way back |
| Closed job in the default job list | excluded, like `HIDDEN` |
| Job gone from a **partial** ATS poll | nothing closed |
| Job gone from an aggregator | 30-day `last_seen_at` window as before |
| All company jobs expired | company row kept; it outlives its vacancies |
| Retention | `jobs` only, never `companies` |
| Paywalled aggregator | not intake, not vacancy source |
| Applied name in YAML | UI highlight only |

**Hard rules:**

- A company key never changes after creation.
- The `ats_slug` is stored and sent verbatim — never lowercased, never stripped of dots.
- Slug only when verified: `APPLY_URL` — known ATS URL shape (raw host, or after redirects); seed — board fetch answers `200` (an empty list still counts); `CAREERS_PAGE` / `AGGREGATOR_PAGE` — board live.
- At most one active resolve per company (`workflow ID = resolve-{company_key}`); unique index on key, insert via upsert. Two profiles running at once must not race.
- **Vacancies are closed only on a complete poll** — successful is not sufficient.
- **An aggregator upsert never clears a close written by `ATS`.** Only a complete poll of that board reopens a vacancy.
- Status code decides the outcome, not the ability to decode the body.
- Company matching is exact (normalised name, domain, alias). No fuzzy matching.
- No LLM in this slice's own logic.

## Modules

| Module | Purpose |
| ------ | ------- |
| `internal/companies` | contract, resolve/poll use cases, merge + blocklist, storage, workflows |
| `internal/collectors/ats/{greenhouse,lever,ashby}` | board fetchers for this slice |
| `internal/jobs` | `company_id`, close-on-absent |
| `internal/scoring` | reuse the existing `input_hash`; pre-LLM gates; per-run LLM ceiling |
| `cmd/companies-import` | seed import + board verify |
| make `db-dump` / `db-restore` | **already in Makefile** — prerequisite, not work in this slice |

```go
type CompanyBoardFetcher interface {
	ATS() schema.ATS
	FetchBoard(ctx context.Context, slug string) ([]domain.Job, error)
}
```

## Layers

- **Saved** — company row: key, aliases, ats, slug, resolution status, poll state, `board_etag`. On a vacancy: `closed_at` / `closed_by`.
- **Run / session** — run counters, per-company poll outcome, `failure_stage`.
- **Wire** — board payloads and workflow inputs, in module `schema/`.

## Implementation order

Reordered: identity hygiene first. Everything downstream — resolve, polling, tags, scoring — builds on the company table, so merging and the blocklist cannot be bolted on later without a cleanup migration.

Revised after Phase 0. Two things moved: the **enrich description threshold is step 0** because it pays immediately and is independent of the whole slice, and **resolve step 1 moved up** because `GAP-1` is closed and apply-URL parsing alone resolves 33% of companies with zero extra requests. The vacancy-text step is **deleted**.

0. **Enrich description threshold** on the enrich service (not profile YAML). Independent of everything below, ~30% fewer LLM calls. Ship it first.
1. `companies` table with both status columns — including `UNSUPPORTED_ATS` — and the new fields; storage; `jobs.company_id`; **fixed key + merge-or-insert + aliases**, exact matching only.
2. **Blocklist** on the discover path (~15 hand-written entries per **M4**).
3. Confirm `make db-dump` / `db-restore` still works after the companies migration (do not re-build dump tooling).
4. Resolve step 1 (apply URL on the raw host; redirects only on a miss). Closed `GAP-1`, no extra requests, resolves 33% of existing companies.
5. Seed import with board verify — `200` is `RESOLVED` even with an empty list, `404` is not. Brings 30 verified slugs and 542 vacancies into reach.
7. Three ATS fetchers + `PollCompaniesWorkflow` with due-interval, per-run cap (memory!), `ETag` conditional requests, `?content=true` on Greenhouse, status-before-decode + wire the run so aggregators feed companies and ATS feeds vacancies.
8. Close-on-**complete** + the fallback rule + run counters (including the partial-read counter).
9. ~~Vacancy text fetch~~ — **deleted, `GAP-4` closed: boards inline the description.**
10. Scoring guards: reuse the existing `input_hash`, pre-LLM gates, per-run ceiling.
11. Resolve step 2 — **Himalayas detail-page parser** (296 jobs, 18% of corpus, no redirect). The generic careers-page hunt of step 3 stays unbuilt until something justifies it.

Early check: one company, one seeded slug, one poll → vacancies upserted, counters moved, second poll returns `304` and scores nothing new.

## Prerequisites

- ✅ Phase 0 findings written down — [`rnd-findings.md`](rnd-findings.md).
- `make db-dump` / `db-restore` already working before any destructive companies migration.
- ✅ Seed file and job-tracker names already under `config/companies/` (64 companies, 30 with verified big-3 slugs).

## Out of scope

- Workday, SAP SuccessFactors, and any ATS board fetcher beyond **Greenhouse, Lever, Ashby**. **M2 settled this:** zero Workday in the target list and 2.3% in the corpus, so the supported three do *not* miss most real targets (47% coverage). Workable + Teamtailor + Recruitee + Personio would reach 66% and are a second iteration — the `UNSUPPORTED_ATS` bucket is the counter that says when.
- Parsing hand-rolled careers HTML as a vacancy source. This is the real residue: **31% of the target list** is Custom/UNKNOWN and stays aggregator-only.
- Crawling a whole company site for a careers page.
- LLM extraction.
- Guess + verify resolve — deferred, and **M2 confirmed why**: 4 of 30 real slugs cannot be derived from the company name.
- Mapping the boards' structured `work_mode` / geography fields into `facts` to replace enrich answers. Phase 0 found the data is there and would displace the two most-unknown fact fields, but it is a separate slice.
- "Known company + matching stack" score signal — deferred, see Phase 0 defaults.
- Company admin UI beyond debug `GET /api/v1/companies`.
- Re-fetching a known, healthily-polled company from aggregators for vacancies.
- Paying aggregators for jobs that sit on a public ATS.
- `applied_companies`, the applied-company UI highlight, and any applied status — **removed from the slice**, not deferred. Handled by hand outside the product.
- `tech_tags` and `UpdateCompanyTags` — **removed from the slice**; they return with the deferred company/stack signal.
- A `domains` column and any per-profile company filter — the slice is single-domain.
- Extra company-intake sources (e.g. Hiring Cafe — **not implemented, no collector, no source id**; worth revisiting *after* this slice and only as intake, since it hands over an ATS link and would duplicate resolve step 1).
- Pruning the US-heavy fund boards, or adding European ones. Phase 0 showed 23 fund-board sources produce 0 TOP and 89% of their rejections are `CUT_LOCATION`, but changing the source list is a profile-config decision, not this slice.
- Contracts / tasks markdown — written when Phase 1 implementation starts.
