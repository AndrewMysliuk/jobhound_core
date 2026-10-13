# Architecture profile — concept

> **Sequence 2.** Next: [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).
> **Profile file exists.** `config/profiles/lada.yaml` (`id: lada`, `domain: architecture`). `sources` and `queries` are empty. The phrase block in that file is still the software-profile copy. The scoring section below is the rewrite, and it is not applied yet.
> Checked 2026-10-13 for the six industry boards. Romanian and DACH boards are named only. They were not fetched, so this file claims no collector behavior for them.

## Why

**Problem today:** `lada.yaml` was copied from the remote software profile. It penalizes office wording, allows the US, and has no Romanian titles. The boards that were checked are UK and US category sites. Her market is office work in Romania.

**New solution:**

1. One profile, `lada`, read by the same profile run as `andrew`. The file does not share `andrew`'s `queries` or sources. A run ingests only the source ids in this file. An empty `sources` list starts no collector.
2. The score pass reads jobs already stored. It does not fetch. IT rows Andrew already stored get a Lada match and fall through `gate_role`. They are not re-fetched.
3. Category slugs live in a new `children` map on the profile, one list per source. A source with children does not fan `queries`.
4. Keyword boards share one `queries` list. The Romanian list and an English remote probe are never in the file together.
5. Collectors stay ordinary fetchers on the existing ingest lock. Static HTML uses `net/http` + goquery. `ejobs` is the rod board, and it comes after the static Romanian boards. No new module, no new HTTP API.
6. Phrase scoring stays the v2 rule scorer. This slice changes the profile file, the `children` field, and one identity rule. It does not change how a phrase is matched.
7. Near-term volume is the Romanian boards. The six industry boards stay as the second tier. Practice career pages stay in [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).

Primary entity: **Profile** (`lada`, domain `architecture`).

Roles: Architectural Designer, Architectural Assistant, Landscape Designer. Adjacent titles (`urbanist`, `desenator tehnic`, `peisagist`) score lower. Interior design stays in `gate_role`.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Ingest | **`net/http` + goquery**, rod only for `ejobs` | `internal/collectors` |
| Profile file | **YAML** | `config/profiles/lada.yaml`, `internal/profiles` |
| Identity | placeholder company key | `internal/domain/utils` |
| Orchestration | existing ingest + profile run | `internal/ingest/workflows`, `internal/scoring/workflows` |

No new tables. Debug HTTP stays on `cmd/agent`.

## Scoring

Applied in `lada.yaml` before the first run. The scorer is unchanged: title phrases stack, body phrases share one cap, matching is a literal case-folded substring with a word boundary. Diacritics are not stripped, so each Romanian spelling is stored twice.

**Gates.** `office_pressure` and `ambiguous_us` are deleted. An office job in an allowed country is a match.

`US` comes out of `allow_countries`. `GB` and the European list stay. A job whose parsed country or region misses both allow lists is `CUT_LOCATION`. An empty location is not cut that way. Unparsed "remote, US" text has no phrase gate after `ambiguous_us` is gone.

`gate_hard` drops the US citizenship, clearance, ITAR, and govcloud lines. Sponsorship refusals and the deferred-pay lines stay. These lines move here from `office_pressure`:

- `right to work in the uk`
- `right to work in germany`
- `eligible to work in france`
- `must reside in switzerland`
- `drept de semnatura`
- `drept de semnătură`

`work permit required` is deleted. It is too broad for Romanian posts.

`gate_role` gains the IT architect titles: `solution architect`, `solutions architect`, `enterprise architect`, `software architect`, `data architect`, `cloud architect`, `security architect`, `systems architect`, `system architect`, `infrastructure architect`, `network architect`, `information architect`, `integration architect`, `platform architect`. `technical architect` is not added. `intern` and `internship` are removed. `interior designer` stays.

**Titles.** Both spellings where the diacritic differs.

| Phrase | Weight |
| --- | --- |
| `architectural designer`, `architectural assistant`, `landscape designer`, `arhitect proiectant`, `arhitect junior`, `peisagist` | 6 |
| `landscape architect` | 5 |
| `architectural technologist`, `graduate architect`, `junior architect`, `arhitect` | 4 |
| `urbanist`, `desenator tehnic` | 3 |

A shorter title also hits inside a longer one. `Arhitect proiectant` scores 6 for the long phrase and 4 for `arhitect`. A bare `Arhitect` is 4, which is REVIEW at the cutoff below.

**Body.** The CV tool terms stay. `max_total` becomes 6. Added terms: `brasov` and `brașov` at 2, `bucuresti` and `bucurești` at 2, `cluj` at 2, `sibiu`, `hibrid`, and `proiectare` at 1.

**Cutoff and pay.** `buckets.top` is 5. Salary stays in the file with no floor and no penalty. A parsed salary does not move the score. `wellfound_roles` is removed. The other soft groups (`agency`, `culture_smell`, `part_time`) stay.

## Sources

Four tiers. A tier is an order, not a new ingest mechanism.

**Tier 0 — existing collectors, left off.** `working_nomads`, `himalayas`, `remotify_europe`, `we_work_remotely`. They are not on this profile. The first three are keyword boards and would receive whatever `queries` says. `we_work_remotely` ignores queries and fetches its whole catalog; Andrew already does that, on the same cooldown key. Listing it here does not search architecture. The remote check is the empty-source score of rows already stored.

**Tier 1 — Romania, the market.** Not fetched.

| Source id | Site |
| --- | --- |
| `bestjobs` | bestjobs.eu |
| `hipo` | hipo.ro |
| `ejobs` | ejobs.ro |
| `olx_ro` | olx.ro |
| `undelucram` | undelucram.ro |

`queries`, once a keyword board in this tier is wired: `arhitect`, `arhitect proiectant`, `peisagist`, `urbanist`, `desenator tehnic`. `designer de interior` is not a query.

**Tier 2 — industry boards already checked.** Children go in `children`, not in `queries`.

| Source id | Site | Children |
| --- | --- | --- |
| `dezeen_jobs` | [dezeenjobs.com](https://www.dezeenjobs.com/) | `architectural-designer`, `architectural-assistants` (plural), `landscape-designer`, `landscape-architect`, `part-1`, `part-2` |
| `riba_jobs` | [jobs.architecture.com](https://jobs.architecture.com/) | `architectural-assistant`, `architect`, `part-1`, `part-2`, `landscape-and-public-realm` |
| `landscape_institute` | [jobs.landscapeinstitute.org](https://jobs.landscapeinstitute.org/) | one catalog child |
| `archgee` | [archgee.com/jobs](https://archgee.com/jobs) | `architect`, `landscape-architect` |
| `archinect` | [archinect.com/jobs](https://archinect.com/jobs/) | one child: `/jobs/region/Remote/all/remote` |
| `think_architecture` | [thinkarchitecturejobs.com](https://thinkarchitecturejobs.com/) | `architectural-designer`, `landscape-architect`, `landscape` |

`dezeen_jobs` has EU cities (Berlin, Basel, Madrid, Paris) plus the UK and remote. `riba_jobs` and `landscape_institute` are UK and small (`architectural-assistant` was 4; the landscape catalog was 9). `archgee` has volume: the header said 1373, 20 per page, pager to page 69. `landscape-architect` plus `country=ro` was empty. The unfiltered first page is US agencies. `archinect` is a US board (667 jobs, 656 in the US, 41 remote). A 2026-10-13 fetch did not get past Cloudflare. `think_architecture` is US only. "2,120 opportunities" is the site total; one category page stopped at 50. A parsed `US` country on these boards is `CUT_LOCATION`.

**Tier 3 — DACH and a European board.** Not fetched. After Romania and the industry boards: `competitionline`, `baunetz`, `bdla`, `world_architects`.

## How a run uses the file

`queries` is one list. It is fanned to every source that has no `children` entry.

`children` is a map from source id to that board's slugs:

```yaml
sources:
  - dezeen_jobs
children:
  dezeen_jobs:
    - architectural-designer
    - architectural-assistants
```

A source named in `children` expands to one ingest child per slug and does not also fan `queries`. `wellfound_roles` stays on `andrew` only. This profile does not use it.

The Romanian query list is written when the first Tier 1 keyword collector exists. Until then `queries` stays empty. Tier 0 stays off for the whole of that state, because those keyword boards would receive the Romanian list.

The score pass loads jobs with `first_seen_at` inside 10 days of the run start, plus jobs whose `posted_at` is already older than 30 days. It evaluates each one against this profile and upserts a match, including rejects. It does not look at `sources` and it does not call a collector.

A locked, cooling-down, or unbuilt source is a skipped child. It is not a failed run. A collector error uses the existing ingest retry.

## Identity

`StableJobID` is `company_key` plus the normalized title. The listing URL is not part of the key. Two posts with company `Confidențial` and title `Arhitect` are one row; the second write replaces the first.

Before the first Romanian collector, a placeholder company key uses the normalized listing URL as well. The placeholder keys are `confidențial`, `confidential`, and `nedivulgat`, after the existing company-key normalization. A named firm still collapses across boards. The rule lives in `internal/domain/utils` and applies to every profile.

## Per-source constraints

Checked 2026-10-13. Detail pages were not. Each collector still needs a fixture from one real detail page. Salary on the card (Dezeen, RIBA, Archinect, ArchGee) maps to the existing salary field and is not scored.

**`dezeen_jobs`.** WordPress SSR. Walk `/job-category/{slug}/`, then `/page/N/`. Do not keyword-search. `/feed/` is the editorial blog, not the vacancy list. Confirm `/job-category/{slug}/feed/` only while building this collector; do not switch the crawl to it on an unchecked response. The card has city, title, `Salary: £…` or `Undisclosed`, and a short blurb. Listing `/job/{company}-{title}-{id}/`. An Algolia block on the same page (`{{data.post_salary}}`) is not the list. `JobHound/1.0` got Cloudflare 403; a browser User-Agent got 200. The apply link is on the detail page. Sitemap: `https://www.dezeenjobs.com/sitemap_index.xml`.

**`riba_jobs`.** Madgex SSR. Category `/jobs/{slug}/` is allowed. Do not append employment-type, salary, or recruiter facet segments — robots.txt disallows those paths. `/jobs/` with no category redirected too many times; start from a category page. The card has city, salary, one sentence, and a date. Listing `/job/{id}/{title-slug}/`. The same set is `/jobsrss/?Job_Function_=N&countrycode=GB` (`architectural-assistant` is `Job_Function_=3`). Confirm that RSS path still returns vacancies before relying on it. Strip `TrackID` and `utm_*`. That description is the card blurb, not the full text.

**`landscape_institute`.** WP Job Manager SSR. One catalog page, `/jobs/`. The card has title, firm, city, type, posted, and closes. Listing `/job/{slug}/`. Search is `GET` `search_keywords`, `search_location`, `search_region` (Remotely `1153`, Overseas `33`, Republic of Ireland `32`, London `34`). `/feed/` is an empty WordPress post feed. `sitemap.rss` is a job list of `link` and `title` only. Use the `/jobs/` catalog for the card.

**`archgee`.** Laravel SSR. Crawl HTML only — robots.txt disallows `/api/`. Cut with query params: `category` (`architect`, `landscape-architect`; the site also has `interior-designer`, and this profile does not send it), `country` (ISO2), `remote` (`remote` / `hybrid` / `onsite`), `seniority` (`intern` through `director`), `page`. The card has title, firm, city, a remote badge, and sometimes `Salary:`.

**`archinect`.** Not re-read on 2026-10-13: Cloudflare challenged `robots.txt` and the remote URL for both `JobHound/1.0` and a browser User-Agent. Prior note, unverified that day: SSR, pages of 30 at `/jobs/list/{offset}`. robots.txt has no `User-agent: *` block. Named crawlers including `ClaudeBot`, `GPTBot`, `anthropic-ai`, `CCBot`, and `Scrapy` are `Disallow: /`. Legitimate named bots get `Crawl-delay: 5`. The User-Agent must not contain any name from that block. Wait ≥5s between requests, with a site mutex, the same shape as `wellfound`, not the 1s `builtin` delay. Remote region only.

**`think_architecture`.** Astro SSR. Category is a root path (`/architectural-designer`, `/landscape`, `/architect`, and city paths such as `/new-york`). One page was 50 articles: title, firm, city, a relative date, a two-line excerpt, and `Full-Time`. No salary on the card. `?page=2` returned the same 50 links; the HTML has no next control. Do not crawl `/go/` (outbound redirect, disallowed). Listing slugs are cut to about 24 characters plus a numeric suffix, so the slug is a weak id. Use `StableJobID` from the normalized URL, and test near-duplicate slugs.

The other four of these six keep the existing 1s gap. All six are small.

**Cooldown.** A 3600s global cooldown is the wrong pace for these six. They need their own cooldown, 6–24 hours, not `JOBHOUND_INGEST_COOLDOWN_TTL_SEC`. Romanian board cooldowns wait until one of them has been fetched.

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| Lada run, `sources` empty | no collector starts; the score pass still reads stored jobs |
| Job already stored by another profile | scored for Lada; not fetched again |
| Source with `children` | one child per slug; `queries` is not fanned to it |
| Source with no `children` | one child per `queries` entry |
| Same vacancy, named firm, two boards | one `jobs` row; `sources` accumulates |
| Placeholder company name | id includes the normalized listing URL |
| Parsed country outside the allow lists | `CUT_LOCATION` |
| Empty location | no location cut |
| `archinect` | remote region only; ≥5s and a site mutex; User-Agent stays off the robots block list |
| `riba_jobs` | category URLs only; no facet segments |
| `archgee` | HTML pages; never `/api/`; no `interior-designer` |
| `think_architecture` | never `/go/` |
| UK or US category board has no Romania rows | do not add a Romania query to force one |
| Source on cooldown | skipped child; the run still succeeds |
| Salary on the posting | stored on the job; no floor and no penalty |

**Hard rule:** do not send a User-Agent that robots.txt names as disallowed. The gap is the site's own crawl-delay.

## Modules

| Module | Purpose |
| ------ | ------- |
| **`internal/collectors`** | one collector per source id that this slice builds |
| **`internal/profiles`** | reads `lada.yaml`; gains `children`; no new store |
| **`internal/domain/utils`** | placeholder company keys include the listing URL |
| **`internal/ingest`** | existing lock, cooldown, child workflow |
| **`internal/scoring`** | existing profile run; the phrase engine stays |

## Layers

- **Saved** — `lada.yaml`, `jobs` rows.
- **Run** — existing `profile_runs`, Redis lock and cooldown.
- **Wire** — no new HTTP. `children` is file content, not a request body.

## Implementation order

1. Rewrite the phrase block in `lada.yaml` as in Scoring. `sources` and `queries` stay empty. Run the profile. That run stores Lada matches for jobs already in the table and starts no collector.
2. Add `children` to the profile schema and to ingest child expansion. A source with children does not fan `queries`.
3. Placeholder company keys include the normalized listing URL. This lands before the first Romanian collector.
4. `bestjobs` and `hipo`. Static HTML. Collector notes are written after one real fetch, not before.
5. Confirm the Dezeen category feed, the RIBA `/jobsrss/` path, and the Landscape Institute catalog notes above. Then build those three collectors. `sitemap.rss` on Landscape Institute is only `link` and `title`; use `/jobs/` for the card.
6. `ejobs` with rod.
7. `olx_ro` and `undelucram`, then `archgee`, `archinect` (remote only), and `think_architecture`.
8. Tier 3 last: `competitionline`, `baunetz`, `bdla`, `world_architects`.

Early check on the first category board: one category fetch stores jobs, and a second fetch of the same category does not duplicate them.

## Out of scope

- Practice career pages, and the practice names already seen (Grimshaw, SOM, Herzog & de Meuron, Perkins&Will, Stantec, HDR, WSP, plus the WLA footer: Felixx, OMGEVING, SLA, LDA Design, LUC, Grant Associates, Studio Egret West, EPD Landscape, Arup, AtkinsRéalis, Benoy, WATG). That is [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).
- Turning on Tier 0. Those collectors stay off this profile.
- IT sources copied from `andrew`: `builtin`, `golang_cafe`, `vue_jobs`, `wellfound`, every `consider_*`, every `getro_*`, `europe_remotely`.
- `academic_positions` — university posts, not this profile. `find-jobs?keywords=` returned HTML once and Cloudflare 403 on the next request.
- `wla_jobs` — four posts on the archive. The collector is not worth it.
- `greenjobs.co.uk` — category indexes are open, job detail URLs are disallowed, so there is no description to score.
- `architecture.co.uk` — the list is client-rendered; the usual WP Job Manager `admin-ajax.php` action was not called.
- `my.asla.org/joblink` — `robots.txt` refused automated access, and the board is US landscape.
- `worldarchitecture.org` — the list comes from `/ajax_job/`, which robots.txt disallows. Romania returned an empty list. Revisit only if the list becomes server-rendered.
- `landezine.com` — robots.txt is fine; `/jobs/` and `/category/jobs/` were 404. The real path was not found.

Still unchecked, so not a decision: every detail page, Think pagination past 50, and Archinect past Cloudflare.
