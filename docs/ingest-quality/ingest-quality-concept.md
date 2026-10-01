# Ingest quality — concept

> Debug the nine live sources before adding boards. No new collectors in this slice.
> Pair with frontend only where stage-2 keyword rules are built: `jobhound_frontend` `stage2RulesFromKeywords`.

## Why

**Problem today:** ingest already writes jobs from nine boards. Two things are wrong on that path, and they block trusting the pipeline.

1. Stage 2 keyword **exclude** matches the title only. **Include** matches title or description. A phrase that sits in the company, tags, location text, or salary is invisible to both.
2. Hiring location is often missing or not the restriction the board stated. Stage 2 geo then treats the job as “region not stated” and does not reject it. The 2026-10-01 ingest names which sources. See Location.

**This slice:**

1. Include and exclude must match the **whole listing**: title, company, description, tags, hiring raw, salary. They do not do this today. Exclude is title only. Include is title or description. The rest of the stored card is skipped.
2. Location is a later pass, not this coding step. The checklist under Location is the script: walk one source at a time, fix that collector, then the next. Built In and Europe Remotely already carry a location because their search is scoped that way. Once the open sources store a country or a region, stage 2 geo can reject a listing that is outside the search. Empty scope stays “not stated” and the search lets it through.
3. New boards and ATS stay a written backlog. They are not built here.

Product context: solo operator running a slot on the current sources. Primary entity: **listing** (`Job` after ingest, then stage 2).

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| HTTP | **`net/http`** | product stage-2 run stays `internal/publicapi`; debug fetch stays `internal/collectors/handlers/debughttp` |
| Persistence | **PostgreSQL + GORM + migrations** | `internal/jobs/storage` — no new tables |
| Orchestration | **Temporal** | ingest and stage 2 stay on the existing workflows |
| Ingest | **HTTP, goquery, rod** | `internal/collectors` |
| Scoring | **Claude API** | unchanged; stage 3 is out of this slice |

Debug HTTP on `cmd/agent` stays the way to pull one source without a slot run.

## Listing

**Input:** one board record, normalized into title, company, listing URL, description, apply URL, and hiring scope.

**Hiring scope on the listing:**

- **Hiring countries** — ISO countries the board restricted to.
- **Hiring regions** — named regions (`EUROPE`, `WORLDWIDE`, …) when the board did not list countries.
- **Hiring raw** — the board text those two fields were parsed from.
- **Country code** — one alpha-2, usually the first resolved country. Stage 2 geo does **not** read this field. It reads hiring countries plus expanded hiring regions. Empty both means “not stated”.

**Lifecycle:** ingest writes the listing. Stage 2 reads the stored row and writes pass, reject, or unknown on the pipeline-run job. This slice does not change that lifecycle.

Statuses at stage 2 stay **REJECTED_STAGE_2**, **PASSED_STAGE_2**, **UNKNOWN_STAGE_2**.

**After stage 2:** the listing text is unchanged. A new stage-2 run re-evaluates the same stored fields. Fixing a collector does not rewrite old rows until the next ingest.

## Runtime

**Start:** existing ingest (one child per source and query) writes listings. Stage 2 starts from the product stage-2 run, with rules built in the frontend from include and exclude lines.

**Main flow:** collectors fill the listing. Stage 2 exclude and include then search it. Today that search is not the whole listing: exclude reads the title, include reads the title or the description. Company, tags, hiring raw, and salary are stored and ignored. Geo reject runs only when a hiring country or region was stored (`when: explicit`).

**Required:** include and exclude must search the whole listing. A phrase only in the company, the description, the tags, the hiring raw, or the salary has to fire the same rule as a phrase in the title. This is the point of the slice. Until the matcher does that, stage 2 is wrong even when ingest stored the text.

**UI-only fields:** the include and exclude textareas. The engine sees the rules those lines become, not the textareas.

**Outcomes:**

- **Passed** — a boost rule fired and no reject fired.
- **Rejected** — an exclude phrase or the geo rule fired.
- **Unknown** — no boost and no reject. Empty rules also end here.

**Adjacent modules:** stage 3 scores jobs that passed or are unknown. It does not re-parse the board.

**Progress:** stage status on the pipeline-run job. A bug is a phrase present on the listing that include or exclude missed, or a board location that never becomes hiring countries or hiring regions.

## How a listing is pulled

Nine sources. Query boards are searched with the fixed keyword list (10 children). Catalog boards are one fetch with no keyword.

| Source | Site | What we read | Location comes from |
| ------ | ---- | ------------ | ------------------- |
| `europe_remotely` | euremotejobs.com | listing HTML + detail page | card and detail location text, then country/region parse. If nothing resolves, region is forced to Europe |
| `working_nomads` | workingnomads.com | Elasticsearch JSON | `locations` and `location_base` |
| `builtin` | builtin.com | listing pages per country (rod), then detail JSON-LD | the **search filter country**, not the job’s own location text. Hiring raw is that alpha-2 |
| `himalayas` | himalayas.app | public JSON | `locationRestrictions` |
| `remotify_europe` | remotifyeurope.com | listing + detail | detail location strings. Remote with no location becomes worldwide. If still empty, region is forced to Europe |
| `we_work_remotely` | weworkremotely.com | RSS only | `<region>` first (`Anywhere in the World`, `USA Only`, `Europe Only`). Country and state are the company HQ and are used only when region says nothing |
| `wellfound` | wellfound.com | role listing HTML + detail JSON-LD | `applicantLocationRequirements` and `jobLocation`. Remote with no location becomes worldwide |
| `vue_jobs` | vuejobs.com | remote catalog in the page payload | workplace strings |
| `golang_cafe` | golang.cafe | JSON via rod | post location / country. Catalog already drops rows outside a Europe-shaped allowlist |

Description is taken on every source (detail HTML, JSON, or RSS). Stage 2 still does not search company, tags, hiring raw, or salary. That is the keyword gap, separate from whether a field was stored.

## One stage-2 pass (core loop)

1. Load stored listings for the slot run.
2. **Geo** — reject when hiring countries or regions are outside Europe-ok and the listing stated a region. No stated region: do not reject.
3. **Exclude** — phrase reject. Today this reads the **title only**.
4. **Include** — phrase boost, weight 1. Today this reads **title or description**.
5. Persist status, hits, and boost. Duplicate listings are rejected before these rules.

### Change

Include and exclude search one text, built from the stored listing:

- title
- company
- description
- tags
- hiring raw
- salary

A phrase matches if it appears in any of those, with the existing word boundary and negation window. Listing URL, apply URL, source id, and country or region codes are not part of that text.

The matcher today has title, description, and title-or-description. Whole-listing search is a new field on the same rule, used by both the include rule and the exclude rule. No second search path.

### Failure at any step

A collector skip (detail fetch, empty title) drops that listing and continues the source. It is not a stage-2 failure.

- Stage 2 does not rewrite the listing.
- A bad rule payload is rejected at the HTTP edge, same as today.
- Logs keep the source and the skip reason. Empty hiring scope is not a fetch error.

## Location

Stage 2 geo is blind when both hiring countries and hiring regions are empty. Country code alone does not count. Hiring raw can be filled while both structured fields stay empty: the text was stored and not recognized.

Parse resolves a country name or a fixed phrase (`europe`, `eu`, `worldwide`, `usa only`, …) on whole words. `European Union` does not contain the word `europe` or `eu`, so that string alone resolves to nothing. `EU` as its own fragment does resolve to Europe. Cities with no country (`Berlin`), timezone text (`USA - East`), and the word `remote` do not become a country or a region. `Bosnia and Herzegovina` is split on ` and ` before the country name is looked up. `Bolivia` does not match the catalog name `Bolivia (Plurinational State of)`.

**Already known, because the search is scoped.** Leave these collectors as they are:

- **Built In** — every row is the country filter it was scraped under. 2026-10-01 ingest: 80/80, one country each (DE 56, PL 14, NL 7, ES 2, IE 1).
- **Europe Remotely** — the board is Europe. Card and detail text are parsed; if nothing resolves, the region is Europe. Same ingest: 8/8 have a country or a region.

**Later pass — walk these by hand and fix them.** Empty hiring countries and empty hiring regions on that ingest (29 of 860 jobs). `wellfound` was 88/88 and is closed. One source at a time: look at how that collector extracts location, fix it, re-ingest, then move on. When a row has a country or a region, stage 2 geo can reject it. While both are empty, the search treats the listing as not stated and keeps it.

- [ ] **vue_jobs** — 8/8. `hiring_raw` is only `remote`.
- [ ] **himalayas** — 14/219. Twelve have no `locationRestrictions` and empty hiring raw. Two have raw text the parser dropped: `Bolivia`, `Bosnia and Herzegovina`.
- [ ] **working_nomads** — 4/387. Raw text stored and not resolved: `USA - East` (3), `Korea Republic of Korea, Republic of` (1).
- [ ] **we_work_remotely** — 3/70 have no `<region>` and empty hiring raw. The other 67 are stated regions from RSS (64 `WORLDWIDE`, 2 `EUROPE`, 1 `LATAM`) and stay as they are.
- [ ] **remotify_europe**, **golang_cafe** — no rows in this ingest. Walk them on the next run that stores listings. Remotify Europe still forces Europe when parse returns nothing; confirm that on real rows before changing it.

## Backlog (not this slice)

Run a separate pass later: can we ingest these boards, and is the yield worth a collector. LinkedIn stays out.

- Indeed
- Glassdoor
- Welcome to the Jungle (former Otta)
- Remote OK
- Remotive
- No Fluff Jobs
- Just Join IT
- Landing.jobs
- The Hub
- Arbeitnow
- Arc

ATS is the same backlog. There is no global feed. Each call is one company board and needs that company’s **board slug** (the token in the URL, not the legal name). The slug list is the operator’s xlsx of companies, outside this repo. Do not invent slugs in code.

| ATS | Board URL shape |
| --- | --- |
| Greenhouse | `https://boards-api.greenhouse.io/v1/boards/{company}/jobs?content=true` |
| Lever | `https://api.lever.co/v0/postings/{company}?mode=json` |
| Ashby | `https://api.ashbyhq.com/posting-api/job-board/{company}` |
| Workable | `https://apply.workable.com/api/v1/widget/accounts/{company}` |
| Recruitee | `https://{company}.recruitee.com/api/offers/` |
| SmartRecruiters | `https://api.smartrecruiters.com/v1/companies/{company}/postings` |
| Personio | `https://{company}.jobs.personio.de/xml` |

Recruitee today is only an apply-link check on Europe Remotely (normalize the URL, drop it if the public page says the job is gone). That is not a Recruitee collector.

## Rules

| Situation | Behavior |
| --------- | -------- |
| Exclude or include phrase in title, company, description, tags, hiring raw, or salary | That rule fires |
| Phrase only in a URL or a country code | Does not fire |
| No hiring country and no hiring region | Geo does not reject |
| Location empty on a source in the later Location pass | Fix that collector on the walk, then re-ingest. Built In and Europe Remotely stay as they are |
| New aggregator or ATS | Not implemented. Backlog only |
| Board slug missing from the xlsx | That ATS company is not fetched |

**Hard rules:** no new source id in ingest. No Temporal SDK in `impl/`. Stage 2 geo still does not read country code. Keyword search does not read URLs.

## Modules

| Module | Purpose |
| ------ | ------- |
| **`jobhound_frontend` stage-2 payload** | Include and exclude both use the whole-listing field |
| **`internal/pipeline`** | New rule field: match title, company, description, tags, hiring raw, and salary |
| **`internal/collectors`** | Location edits happen on the later source-by-source walk, only for sources still open under Location |
| **`internal/jobs/storage`** | Read model for that walk. No migration |

## Layers

- **Saved** — listing title, company, description, tags, salary, hiring countries, hiring regions, hiring raw. Unchanged by stage 2.
- **Run** — stage-2 status, hits, boost.
- **Wire** — existing stage-2 rule body. Include and exclude send the whole-listing field. No new route.

## Implementation order

1. Whole-listing field in the stage-2 matcher. Point include and exclude at it.
2. Later, walk the Location checklist by hand and fix each open source. Order: vue_jobs, himalayas, the unresolved working_nomads strings, the three empty We Work Remotely rows, then remotify_europe and golang_cafe once an ingest stores them. Built In and Europe Remotely stay as they are. After each fix, the next ingest fills hiring countries or regions, and stage 2 geo can reject listings the search should drop.
3. Leave the aggregator and ATS backlog in this file.

Early check: a phrase that appears only in the company, the description, the tags, the hiring raw, or the salary fires the same include or exclude rule as a phrase in the title. A phrase that appears in none of those does not.

## Out of scope

- New aggregators and ATS collectors, including the xlsx import.
- LinkedIn.
- Stage 3 scoring, slot model, auth, scheduled refresh.
- Rewriting historical rows in place. The next ingest picks up collector fixes.
