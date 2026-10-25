# Phase 0 findings — company database and direct ATS polling

> Measured 2026-10-25 against the live local stack (`make docker-up`), 1600 `jobs` rows / 857 distinct `company_key` / 1600 `job_verdicts`, and against live Greenhouse / Ashby / Lever boards by hand.
> Numbers only. Decisions they trigger are marked **⇒**. The concept file carries the resolved gap markers.

**Status: M1 M2 M3 M4 M5 done. M1 partially — the redirect pass is a 110-host sample, not the full 538.**

Caveat on every per-source number below: a job can carry several source ids, so per-source rows double-count multi-source jobs (1881 job-source pairs over 1600 jobs). Per-group numbers are per job and do not.

---

## M1 — aggregator payload coverage

### Raw host of `jobs.apply_url`, no redirects (1600 jobs)

| Host class | Jobs | Share |
|---|---:|---:|
| greenhouse (incl. `grnh.se`) | 378 | 23.6% |
| ashby | 213 | 13.3% |
| lever | 74 | 4.6% |
| **big-3 total** | **665** | **41.6%** |
| workday | 37 | 2.3% |
| other ATS (smartrecruiters, rippling, comeet, gem, applytojob…) | 53 | 3.3% |
| unknown host | 538 | 33.6% |
| no `apply_url` at all | 307 | 19.2% |

Company level: **287 of 857 companies (33%)** resolve to big-3 from the raw host alone. Job level (41.6%) is higher than company level (33%) because big-3 companies post more vacancies each — this favours polling.

### Per source

| Source | Jobs | % with `apply_url` | % with `company_website` | % big-3 raw | avg description chars |
|---|---:|---:|---:|---:|---:|
| working_nomads | 463 | 99 | 2 | 67 | 6047 |
| himalayas | 303 | 99 | 1 | 0 | 2984 |
| consider_andreessen_horowitz | 146 | 99 | 100 | 84 | 463 |
| consider_lightspeed | 130 | 100 | 100 | 76 | 342 |
| wellfound | 108 | 1 | 3 | 1 | 4838 |
| we_work_remotely | 90 | 0 | 26 | 0 | 6262 |
| consider_sequoia_capital | 82 | 100 | 100 | 83 | 516 |
| consider_gv | 76 | 99 | 100 | 58 | 296 |
| dou_ua | 76 | 39 | 0 | 0 | 3545 |
| builtin | 66 | 0 | 2 | 0 | 4884 |
| getro_accel | 58 | 98 | 16 | 64 | 181 |
| consider_bessemer_ventures | 48 | 100 | 100 | 73 | 224 |
| consider_ivp | 48 | 98 | 100 | 75 | 231 |
| consider_nea | 38 | 100 | 100 | 50 | 239 |

Consider boards are the best intake source in the corpus: ~100% carry an apply URL, **100% carry a company domain**, and 50–84% point straight at a big-3 board.

### Redirect pass

Sample: 157 URLs covering 110 distinct unknown hosts (2 per host, random), plain `GET`, redirects followed.

| Outcome | URLs |
|---|---:|
| became big-3 | **1** |
| became another ATS (jobvite, smartrecruiters, applytojob, gem, join.com, paycom…) | 49 |
| still unknown | 107 |

⇒ **Following redirects buys nothing for big-3** (1 of 157). The gap between raw-host and post-redirect match — the thing this measurement existed to price — is effectively zero. Redirect-following is still worth having, but as a cheap normalisation step, not as a resolve strategy.

⇒ The 49 other-ATS hits are only useful as an `UNSUPPORTED_ATS` counter (see M2).

### himalayas.app is not a redirect problem

`himalayas.app` is the single largest unknown host (296 jobs, 18% of the corpus). Dedicated sample of 25 of its apply URLs: **25 of 25 do not redirect anywhere.** The URL is Himalayas' own landing page and the outbound ATS link sits behind on-page markup, not a 3xx.

⇒ Those 296 jobs need the outbound link extracted from the Himalayas detail page. That is resolve step 2, but **not** the generic `{website}/careers` crawler the concept describes — it is one narrow parser against one aggregator's own page.

**Closes GAP-1 and GAP-2.** Decision gate from the concept: Consider/Getro sit in the `>50%` band ⇒ `APPLY_URL` is the primary resolve path, step order 1→2 as written.

---

## M2 — ATS reachability of real targets

Source: `config/companies/companies_template.xlsx`, 64 hand-picked companies, every one marked `Fit=FIT`.

| ATS | Companies | Share |
|---|---:|---:|
| Ashby | 20 | 31% |
| Greenhouse | 7 | 11% |
| Lever | 3 | 5% |
| **big-3 total** | **30** | **47%** |
| Custom / UNKNOWN / NONE | 20 | 31% |
| Teamtailor | 4 | 6% |
| Workable | 3 | 5% |
| Recruitee | 2 | 3% |
| Personio | 2 | 3% |
| Rippling / Kula / Notion | 3 | 5% |
| **Workday** | **0** | **0%** |

⇒ The supported three cover **47%** of the real target list. The ceiling fear this measurement existed to test — "half of them are on Workday, so ATS-as-vacancy-feed is false" — **is not confirmed**: zero Workday, and only 2.3% Workday in the whole 1600-job corpus.

⇒ Workable + Teamtailor + Recruitee + Personio would add 19% → 66%. Four more fetchers for 12 companies: second iteration, not this slice. `Out of scope` stands as written.

⇒ Custom/UNKNOWN (31%) stay aggregator-only. That is the real residue, and it is bigger than any single ATS.

**Closes GAP-3.** No fourth fetcher in this slice.

### All 30 big-3 slugs verified live

30 of 30 returned `200`. Total 542 open vacancies in 30 requests.

Slug shapes that break any name-derived guess:

| Company | Slug | Trap |
|---|---|---|
| Form3 | `541` | numeric Greenhouse board id, unrelated to the name |
| Primer | `primer.io` | dot inside the slug |
| Ecosia | `ecosia.org` | dot inside the slug |
| Coder, Deepnote, Mastra | `Coder`, `Deepnote`, `Mastra` | capital letter is significant |

⇒ 4 of 30 slugs are not derivable from the company name. **The concept's decision to defer "guess + verify" (resolve step 3) is confirmed by data.**

⇒ Hard constraint: **do not normalise the slug.** Lowercasing breaks Ashby (`Coder`, `Deepnote`, `Mastra`), stripping dots breaks `primer.io` and `ecosia.org`. Store and send the slug exactly as seeded.

---

## M3 — board response shape

One live slug per ATS, by hand.

| Question | Greenhouse | Ashby | Lever |
|---|---|---|---|
| Endpoint | `boards-api.greenhouse.io/v1/boards/{slug}/jobs` | `api.ashbyhq.com/posting-api/job-board/{slug}` | `api.lever.co/v0/postings/{slug}?mode=json` |
| Full description in the list response | **only with `?content=true`** | yes, `descriptionPlain` | yes, `descriptionPlain` |
| Description field / size | `content`, 8.6k chars | `descriptionPlain`, 11.5k chars | `descriptionPlain` |
| Per-vacancy fetch needed | **no** | **no** | **no** |
| `ETag` sent | yes | yes | yes |
| `If-None-Match` → `304` | **yes** | **yes** | **yes** |
| Unknown slug | `404` JSON `{"status":404,...}` | `404` **plain text** `Not Found` | `404` JSON `{"ok":false,...}` |
| Zero open vacancies | `200` `{"jobs":[]}` | `200` `{"jobs":[],"apiVersion":"1"}` | `200` `[]` |

Without `?content=true` Greenhouse returns titles only — 8.2 KB vs 185 KB for the same 14 jobs. Easy to wire up wrong and never notice until scoring produces nothing.

⇒ **Empty board and dead slug are unambiguously distinguishable: `200` vs `404`.** This was the question the concept flagged as the one that matters. Live examples turned up unprompted during slug verification: Prismic (Lever) and Ecosia (Ashby) both answered `200` with an empty list.

⇒ Ashby's `404` body is **not JSON**. A fetcher that unmarshals before checking the status code will report a decode error instead of "slug is dead", and `poll_fail_count` will blame the wrong thing.

**Closes GAP-4 (negatively — no `FETCH_TEXT` step exists) and GAP-5 (positively — conditional polling on all three).**

### Structured fields the boards return

| Field | Ashby | Lever | Greenhouse |
|---|---|---|---|
| work mode | `isRemote`, `workplaceType` | `workplaceType` | `location.name`, `offices[]` |
| geography | `secondaryLocations[].addressCountry` | `categories.allLocations[]`, `country` | `offices[]` |
| apply url | `applyUrl` | `applyUrl`, `hostedUrl` | `absolute_url` |
| timestamps | `publishedAt` | `createdAt` | `updated_at`, `first_published` |

⇒ Boards return `work_mode` and hiring geography **as data**. Those are exactly the two enrich facts that cost the most and come back `unknown` most often (see "Enrich cost" below). The ATS path does not only improve description quality — it removes part of the model's job. Not a task in this slice, but it changes what the slice is worth.

### Payload sizes

`ashby` 2.0 MB · `supabase` 834 KB · `n8n` 895 KB · `alpaca` 902 KB. A full cycle over 30 companies is ≈ 9 MB.

⇒ The per-run poll cap is a memory constraint on the worker, not only politeness to the boards. Decode streaming, not whole-body-into-memory.

---

## M4 — recruiting agencies in the current data

Name-token heuristic over 857 distinct `company_key` (`talent|recruit|staffing|outsourc|headhunt|hr solution|people profiler|resourcing|manpower|workforce|placement|consultanc`):

- **13 companies, 20 jobs** (1.5% of companies, 1.25% of jobs).
- Largest: `aida recruitment` 5 jobs. Everything else 1–2.
- Adding bare `consulting` catches `pradeepit consulting services pvt` (18 jobs) — clearly an outsourcer the token list missed.
- Known staff-augmentation brands carry no token and need naming by hand: `lemon.io` (9 jobs), `proxify` (seen as `career.proxify.io` in apply URLs).

Token clusters are visible and sortable by shared prefix: `talent hackers / talentwelove / talentcross / talentuch / talent insider / talent hunts indonesia`, `people profilers / people profilers vietnam / people's growth hr solutions`.

⇒ **A hand-written blocklist is enough.** The entry list is ~15 names, not hundreds. No heuristic is needed as the mechanism.

⇒ But the footprint is small *because aggregators show only a slice of an agency's board*. The concept's risk — resolve succeeds on an agency, the poller imports hundreds of other people's vacancies — is real and unmeasured here. Keep the token list as a **quarantine signal for review**, not an auto-block: after a poll, a company with an unusually high vacancy count and a wide spread of stacks goes to manual review.

---

## M5 — key collision check

Over 857 distinct `company_key`:

| | Count |
|---|---:|
| name-form keys (no dot) | 828 |
| domain-form keys (contains a dot) | 29 |
| **name-form / domain-form pairs for one company** | **1** (`yld` vs `yld.com`) |

Domain-form keys come almost entirely from Consider (`companyDomain`) plus a few aggregator rows: `horizon3.ai`, `eightfold.ai`, `lemon.io`, `pitchup.com`, `eon.io`, `modash.io`.

Near-duplicate name pairs that are *not* the same company and must not be merged: `cloudbees` / `cloudbeds`, `insider` / `insider one`, `lemon.io` / `lemon.markets`, `betterme` / `betterment`, `creatio` / `creative chaos`.

⇒ **Merge logic needs almost nothing today — 1 collision in 857.** But that is a consequence of 97% of keys being name-form, because only Consider supplies a domain. Once the ATS path lands, every polled company has a verified board and usually a domain, so domain-form share rises and collisions rise with it.

⇒ Keep merge-or-insert and `aliases` as the concept specifies — the cost now is near zero and retrofitting it after the table fills requires a cleanup migration. But fuzzy name matching is **not** justified: the near-duplicate pairs above show it would merge distinct companies. Match on exact normalised name, exact domain, and `aliases` only.

---

## Enrich cost — not an M, but it decides the order of work

Per job (1600 jobs, all carry a verdict; `fund_boards` = every source id on that job is `consider_*` or `getro_*`).

| | Jobs | avg description | < 800 chars | `hiring_scope` unknown | `work_mode` unknown | `work_authorization` unknown | `compensation` unknown |
|---|---:|---:|---:|---:|---:|---:|---:|
| aggregators | 1113 | 4857 | 14 | 67% | 34% | 91% | 67% |
| fund_boards | 487 | 234 | **487** | **100%** | **84%** | **100%** | **100%** |

Fund-board descriptions are joined skill labels, not prose ([`consider/map.go:121`](../../internal/collectors/consider/map.go), [`getro/map.go:38`](../../internal/collectors/getro/map.go)).

⇒ **487 of 1600 enrich calls (30%) go into text that cannot contain the answer** — 100% unknown on three of four fields.

⇒ A `< 800` description threshold catches 487 of 487 fund-board jobs and 14 of 1113 aggregator jobs. Those 14 were checked individually: **0 TOP**, 5 REVIEW, and 12 of 14 returned `hiring_scope: unknown` anyway.

⇒ The threshold belongs on the enrich service, **not in profile YAML**: `ListMissing` is called with no profile id ([`profile_run.go:152`](../../internal/scoring/workflows/profile_run.go)) and `MissingIDs` reads every job via `ListFirstSeenSince(ctx, time.Time{})` ([`service.go:65`](../../internal/enrich/impl/service.go)). The verdict key `(job_id, prompt_version, model, input_hash)` has no profile either. A profile-level threshold would let `andrew.yaml` decide whether jobs needed only by `lada` get enriched, and would desynchronise `Statuses` (api, profile known) from `MissingIDs` (worker, no profile) so `facts_status` reports `pending` forever.

⇒ `work_mode` is extracted even from 137-char text, so the threshold is not uniformly right. The sharper fix is dropping the three hopeless fields from the prompt for short text. Threshold first, that second.

---

## Scoring yield — why the flip is worth doing

Per job, profile `andrew`:

| | Jobs | TOP | REVIEW | REJECTED | of which `CUT_LOCATION` |
|---|---:|---:|---:|---:|---:|
| aggregators (9 sources) | 1113 | **13** | 178 | 922 | 612 |
| fund_boards (23 sources) | 487 | **0** | 53 | 434 | 386 (89%) |

23 of the 32 configured sources produced **zero TOP** between them.

Two independent causes, both structural:

1. **No body text** — `signals_plus.text`, `penalties_soft`, and salary parsing cannot fire on a 234-char skill list, and `facts` come back unknown (above).
2. **Wrong geography** — 386 of 434 fund-board rejections (89%) are `CUT_LOCATION`. The US funds (a16z, Sequoia, Lightspeed, GV, Bessemer, IVP, NEA, Kleiner Perkins, USV, Battery) are structurally outside the profile's allowed countries.

By contrast the EU-leaning boards have poor volume but far better density: `consider_balderton_capital` 10 jobs → 6 REVIEW / 4 REJECTED; `consider_notion_capital` 3 → 3 REVIEW / 0 REJECTED; `consider_atlantic_labs` 4 → 3 REVIEW / 1 REJECTED. Against `consider_andreessen_horowitz` 146 → 14 REVIEW / 132 REJECTED.

⇒ **Fund boards are a company-intake source, not a vacancy feed.** This is the bet the concept opens with, and the data supports it on both axes.

⇒ Adding more US fund boards is negative work: more rows in `REJECTED`, more empty enrich calls, no TOP.

⇒ If the fund list grows, grow it with European funds only.

### Comparison of scale

| Path | Requests | Vacancies | Hand-vetted companies |
|---|---:|---:|---:|
| 32 aggregator sources with query fan-out | many (fan-out per query/slug) | 1600 | no |
| 30 big-3 boards from the seed file | 30 | 542 | yes, all `FIT` |

---

## Four configured sources returned nothing

`consider_panoramic_ventures`, `getro_point_nine`, `getro_earlybird`, `getro_firstminute` are listed in `config/profiles/andrew.yaml` and have **zero rows** in `jobs`. Dead tenant, moved host, or a permanent skip — not diagnosed here.

⇒ Check and either fix or remove. Four sources silently contributing nothing is also four `sources_skipped` increments per run that look like normal cooldown.

---

## Decision summary

| Gap | Status | Value |
|---|---|---|
| GAP-1 resolve step order | **closed** | `APPLY_URL` primary, order 1→2 as written. Consider/Getro in the `>50%` band |
| GAP-2 careers parser needed | **closed, narrowed** | Not a generic `{website}/careers` crawler. One parser for the Himalayas detail page (296 jobs, 18% of corpus, 0% redirect) |
| GAP-3 ceiling / fourth fetcher | **closed** | big-3 = 47% of real targets, Workday 0%. No fourth fetcher in this slice |
| GAP-4 vacancy-text fetch step | **closed, removed** | All three boards inline the full description. Greenhouse needs `?content=true` |
| GAP-5 conditional polling | **closed** | `ETag` + `304` on all three |

### New constraints the data added

1. Never normalise a slug — case and dots are significant (4 of 30 slugs break).
2. Check the HTTP status before decoding — Ashby's `404` body is plain text.
3. Greenhouse list request must carry `?content=true`.
4. Per-run poll cap is a worker memory limit (≈9 MB per 30 companies), not only board politeness.
5. Need a fourth resolution state for recognised-but-unsupported ATS, so those companies stop re-entering `PENDING` every 14 days — 2.3% Workday and 3.3% other ATS in the corpus, 19% of the seed list, and the redirect pass found 49 more per 157 URLs. The state doubles as the counter that says when a fourth fetcher pays.
6. Closing vacancies requires a *complete* poll, not merely a successful one: Greenhouse and Lever paginate, and a partial read with `200` on every page would close live vacancies.

### Open, not measured here

- M4's real magnitude — what an agency's own board actually returns when polled. Unmeasurable before the poller exists.
- The full 538-URL redirect pass (sampled at 110 hosts here).
- Whether dropping `hiring_scope` / `work_authorization` / `compensation` from the prompt for short text beats a flat description threshold.
