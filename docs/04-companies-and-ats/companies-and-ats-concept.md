# Company database and direct ATS polling — concept

> **Sequence 4.** After [`docs/03-vc-portfolio-boards/`](../03-vc-portfolio-boards/vc-portfolio-boards-concept.md). Depends on [`docs/01-profiles-and-scoring/`](../01-profiles-and-scoring/profiles-and-scoring-concept.md) shipping first.
> Contracts and tasks files are intentionally absent until this slice is picked up.

## Why

**Problem today:** every vacancy comes from an aggregator. That caps quality in three ways. Aggregators lag — a job appears on the company's own board days earlier. They truncate — the description is a teaser, the real requirements and location rules live on the ATS page, which is exactly the text scoring needs. And they are incomplete — plenty of companies never post to them at all.

Meanwhile the aggregators already hand us the one thing needed to bypass them: an apply link, which usually points straight at Greenhouse, Lever, Ashby or Workable.

**New solution:**

1. Companies are written into the database **once**, then checked. Re-discovering a known company from an aggregator on every run is wasted work: the ATS board is public, and the slug does not change.
2. Discovery only adds a row that is not already stored. Inputs, cheapest first: the manual company table the user already keeps, VC portfolio boards ([`docs/03-vc-portfolio-boards/`](../03-vc-portfolio-boards/vc-portfolio-boards-concept.md)), and aggregators that already return an apply URL for free.
3. A resolver works out each company's **ATS and board slug**, primarily by parsing the apply URL. It runs for `PENDING` rows only.
4. Every later run polls the stored `RESOLVED` companies directly on their ATS board. That is the vacancy source. The aggregator drops out of the path for that company.
5. Company rows accumulate **tech tags** from their recent vacancies, which feeds the "known company with matching stack" scoring signal left unimplemented in v2.

Aggregators that charge the job seeker are not an ongoing source. The same vacancy is already posted in the open on Greenhouse, Lever, Ashby or Workable.

Primary entity: **Company**.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Persistence | **PostgreSQL + GORM + migrations** | `internal/companies/storage` |
| Orchestration | **Temporal** | `internal/companies/workflows` |
| ATS fetch | **`net/http`** (JSON boards) + **goquery** (careers pages) | `internal/collectors/ats/*` |
| Tagging | **dictionary `Tagger`** from the v2 slice | `internal/dictionary` |

No LLM, no headless browser: the target ATS platforms all expose either JSON or plain server-rendered HTML.

## Company

**Input:** a company name plus whatever the aggregator gave us — website, apply URL. Companies also arrive from a **manual import file** the user already maintains.

**Key:** the website domain when known, otherwise the normalized name (lowercased, `Inc` / `GmbH` / `Ltd` / `LLC` / `SRL` stripped). The v2 `jobs.company_key` column is written with exactly this rule, so the join target already exists.

**Fields:** name, website, `ats` + `ats_slug` (null until found), `resolution_status`, `resolution_method`, `domains` (`software` / `architecture`), `tech_tags` (`{"vue": 4, "go": 2}` counted over vacancies from the last 30 days), `first_seen_at`, `last_seen_at`, `last_polled_at`, `poll_fail_count`.

Statuses: **`PENDING`** → **`RESOLVED`** | **`UNRESOLVED`** | **`INACTIVE`**.

**Companies are never deleted by retention.** Their vacancies expire on the 30-day `last_seen_at` window, the company row does not — resolution is expensive and worth keeping.

## Steady state

Each run does three things, and only the first one touches companies already known:

1. **Check** every `RESOLVED` company in the table: fetch its ATS board.
2. **Resolve** `PENDING` rows that still have no slug.
3. **Discover** rarely, and only to notice a company that is not in the table yet. A hit on a company already stored writes nothing and does not go back to the aggregator.

Discovery itself is not part of the every-run button. The manual table is a one-shot import. Portfolio boards and free aggregators run on their own cooldown, as company intake, not as the vacancy feed.

## Runtime

Poll and resolve run as steps of the existing `ProfileRunWorkflow` so there is still one button. Discovery does not.

**`ResolveCompanyWorkflow`** — only for `PENDING` rows, in four escalating attempts:

1. **Parse the apply URL.** Known shapes: `boards.greenhouse.io/{slug}`, `job-boards.greenhouse.io/{slug}`, `jobs.lever.co/{slug}`, `jobs.ashbyhq.com/{slug}`, `apply.workable.com/{slug}`, `{slug}.recruitee.com`, `{slug}.jobs.personio.de`. Cheapest and most reliable; `resolution_method = APPLY_URL`.
2. **Fetch the careers page.** `{website}/careers` and `{website}/jobs`, looking for ATS links and embed scripts. `resolution_method = CAREERS_PAGE`.
3. **Guess and verify.** Derive slug candidates from the name, fetch the candidate board, and accept it **only if** it lists a vacancy whose normalized title matches one we already saw on the aggregator. No match means nothing is written — a wrong slug is worse than no slug. `resolution_method = GUESS_VERIFIED`.
4. Nothing worked → `UNRESOLVED`. That company stays aggregator-only and is retried after **14 days**.

**`PollCompaniesWorkflow`** — all `RESOLVED` companies whose `domains` intersect the running profile's domain. Concurrency is capped per ATS (around 5 at a time) to stay polite. A board that 404s or returns empty **five times in a row** becomes `INACTIVE` and drops out of the rotation.

**`UpdateCompanyTags`** — recounts `tech_tags` from the company's vacancies inside the 30-day window.

## One resolution attempt (core loop)

1. Load the `PENDING` company and the apply URLs of its known vacancies.
2. Try the four steps above in order; stop at the first that yields a verified slug.
3. Write `ats`, `ats_slug`, `resolution_status`, `resolution_method`, `last_polled_at` in one update.

**Failure:** a network error is a Temporal retry, not an `UNRESOLVED`. Only an exhausted search sets `UNRESOLVED`, so a flaky evening does not permanently mark a company unresolvable.

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| Apply URL on an unknown host | fall through to the careers page step |
| Guessed slug with no title overlap | write nothing, keep `PENDING` until the 14-day retry |
| Same vacancy from ATS and aggregator | one row by `dedup_key`; the ATS text and location win |
| ATS board empty five polls running | `INACTIVE` |
| Company vacancies all expired | row kept, `tech_tags` decay to empty |
| Retention sweep | touches `jobs` only, never `companies` |
| Company already stored | do not re-fetch it from aggregators; poll its ATS |
| Paywalled aggregator | not a discovery input and not a vacancy source |

**Hard rule:** a slug is only persisted when it is verified against a known title, in every path except `APPLY_URL`.

## Modules

| Module | Purpose |
| ------ | ------- |
| **`internal/companies`** | contract, resolution use cases, `companies` storage, both workflows |
| **`internal/collectors/ats/{greenhouse,lever,ashby}`** | first three boards; later Workable, Recruitee, Personio, SmartRecruiters |
| **`internal/dictionary`** | reused for `tech_tags` |
| **`internal/jobs`** | gains `company_id`, deferred from v2 on purpose |
| **`cmd/companies-import`** | one-shot import of the manual company file as `PENDING` |

ATS collectors need their own contract, because an ATS board is fetched **per company**, not per keyword:

```go
type CompanyBoardFetcher interface {
	ATS() schema.ATS
	FetchBoard(ctx context.Context, slug string) ([]domain.Job, error)
}
```

## Implementation order

1. `companies` table, enums, storage, `cmd/companies-import`, `jobs.company_id`.
2. Resolution step 1 only (apply URL), since it covers most cases for the least code.
3. Greenhouse, Lever, Ashby fetchers + `PollCompaniesWorkflow` + `UpdateCompanyTags`.
4. Resolution steps 2–3 (careers page, verified guessing).
5. Wire the "known company with matching stack" signal into scoring.

Early check: run the apply-URL parser over the `apply_url` column already collected in v2 and count how many companies resolve for free. That number decides whether steps 2–4 are worth writing at all.

## Prerequisites

- **Database must survive restarts.** `make docker-down` currently runs `down -v` and drops the `jobhound_pgdata` volume. Resolution costs dozens of HTTP requests per company, and `tech_tags` are counted over 30 days — both are pointless if the database lives for one evening. Separating "stop" from "wipe" is a prerequisite, not a nicety.
- **The v2 collector audit.** Which of the nine aggregators actually report an apply URL and a company website. Sources that report neither cannot feed resolution.
- **The manual company file** from the user, in whatever format it currently has. A small draft already exists. Whether it carries ATS links or only name and website decides if import lands rows as `RESOLVED` or `PENDING`.

## Portfolio board candidates

Discovery input, not the vacancy feed. Europe first — closer geography — then the large US funds. Both shortlists are taken from a third-party sheet, [Venture Funds & Job Boards](https://docs.google.com/spreadsheets/d/1eOV2B4nfjJ-4MSRYeyuifc8R4nIyMT8mKhF29YmEKIk/edit?gid=0#gid=0), which also holds smaller funds not copied here. Links were not opened one by one, so some may be dead. The platform behind each board was not checked either.

**Europe**

| Fund | URL |
| ---- | --- |
| Balderton | https://careers.balderton.com |
| Cherry Ventures | https://talent.cherry.vc |
| Creandum | https://careers.creandum.com |
| Dawn Capital | https://jobs.dawncapital.com |
| Earlybird | https://jobs.earlybird.com |
| Northzone | https://opportunities.northzone.com |
| Point Nine | https://jobs.pointnine.com |
| Speedinvest | https://careers.speedinvest.com |
| Seedcamp | https://talent.seedcamp.com |
| Moonfire | https://positions.moonfire.com |
| Headline | https://talent.headline.com |
| firstminute | https://jobs.firstminute.capital |
| Credo Ventures | https://jobs.credoventures.com |
| Molten | https://www.moltenventures.com/opportunities |
| Index Ventures | https://www.indexventures.com/startup-jobs |

**United States**

| Fund | URL |
| ---- | --- |
| a16z | https://jobs.a16z.com/ |
| Sequoia | https://jobs.sequoiacap.com/jobs |
| Accel | https://jobs.accel.com |
| General Catalyst | https://jobs.generalcatalyst.com |
| Greylock | https://jobs.greylock.com |
| Kleiner Perkins | https://jobs.kleinerperkins.com |
| Khosla | https://jobs.khoslaventures.com |
| Lightspeed | https://jobs.lsvp.com |
| Bessemer | https://jobs.bvp.com |
| Battery | https://jobs.battery.com |
| NEA | https://careers.nea.com |
| GV | https://jobs.gv.com |
| IVP | https://careers.ivp.com |
| Insight Partners | https://jobs.insightpartners.com |

Before parsing:

- Check `robots.txt` on every board.
- Sort the live ones by platform with a script that looks in the footer for "Powered by Getro" or "Powered by Consider". That split is two groups for two parsers.
- Index in the table is a page on the fund's main site. Its `jobs.` host previously refused automated access.
- Atomico is not on the list. If it is wanted, the URL has to be found separately.

## Hiring Cafe

Discovery candidate, same role as the portfolio boards: company intake via apply URLs, not the ongoing vacancy feed.

| Board | URL |
| ----- | --- |
| Hiring Cafe | https://hiringcafe.com/ |

Not opened yet. The same research is due before it is treated as an input:

- Check `robots.txt`.
- Is there a public JSON search, or only a rendered catalogue?
- Does a listing expose the apply URL and the company website? Without both it cannot feed resolution.
- Whether search is per keyword or catalogue-only.

A headless-only page stays out. Rod is already expensive on `builtin` and `golang_cafe`, and this source is intake, not the vacancy feed.

## Out of scope

- Workday, SAP SuccessFactors and hand-rolled careers pages — high effort per site, bad yield.
- Crawling a company's whole website looking for a careers page.
- Any LLM-assisted extraction.
- Company-level UI beyond a debug list (`GET /api/v1/companies` filtered by `resolution_status`, `domain`, `tech`).
- Re-running aggregator discovery for a company already in the table.
- Paying an aggregator to read vacancies the company already posts on a public ATS.
