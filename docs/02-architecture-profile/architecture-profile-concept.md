# Architecture profile — concept

> **Sequence 2.** Next: [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).
> **Profile file.** `config/profiles/lada.yaml` (`id: lada`, `domain: architecture`) carries the Scoring phrase block and the `queries`, `sources`, and `children` block from the contracts file.
> Opened 2026-10-14. Tier 1 is `bestjobs`, `hipo`, and `ejobs`. Boards evaluated and not shipped are under Out of scope. European office boards stay and are shown with a penalty.

## Why

**Problem today:** `lada.yaml` was copied from the remote software profile. It penalizes office wording, allows the US, and has no Romanian titles. The boards that were checked are UK and US category sites. Her market is office work in Romania.

**New solution:**

1. One profile, `lada`, read by the same profile run as `andrew`. The file does not share `andrew`'s `queries` or sources. A run ingests only the source ids in this file. An empty `sources` list starts no collector.
2. The score pass reads jobs already stored. It does not fetch. IT rows Andrew already stored get a Lada match and fall through `gate_role`. They are not re-fetched.
3. Category slugs live in a new `children` map on the profile, one list per source. A source with children does not fan `queries`.
4. Keyword boards share one `queries` list. The Romanian list and an English remote probe are never in the file together.
5. Collectors stay ordinary fetchers on the existing ingest lock. Static HTML uses `net/http` + goquery. `ejobs`, `dezeen_jobs`, and `world_architects` need a browser: curl gets 403. No new module, no new HTTP API.
6. Phrase scoring stays the v2 rule scorer. This slice changes the profile file, the `children` field, and one identity rule. It does not change how a phrase is matched.
7. Near-term volume is the Romanian boards. European office boards stay in the crawl. Practice career pages stay in [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).

Primary entity: **Profile** (`lada`, domain `architecture`).

Roles: Architectural Designer, Architectural Assistant, Landscape Designer. Adjacent titles (`urbanist`, `desenator tehnic`, `peisagist`) score lower. Interior design stays in `gate_role`.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Ingest | **`net/http` + goquery**, rod for `ejobs`, `dezeen_jobs`, and `world_architects` | `internal/collectors` |
| Profile file | **YAML** | `config/profiles/lada.yaml`, `internal/profiles` |
| Identity | placeholder company key | `internal/domain/utils` |
| Orchestration | existing ingest + profile run | `internal/ingest/workflows`, `internal/scoring/workflows` |

No new tables. Debug HTTP stays on `cmd/agent`.

## Scoring

Applied in `lada.yaml` before the first run. The scorer is unchanged: title phrases stack, body phrases share one cap, matching is a literal case-folded substring with a word boundary. Diacritics are not stripped, so each Romanian spelling is stored twice.

**Gates.** `office_pressure` and `ambiguous_us` are deleted. European office is not a reject.

Romania and any remote job, including remote in the US, carry no location penalty and can be TOP. An office job in Europe outside Romania is still shown, with one soft penalty of −2, so a normal title stays under `buckets.top`. A relocation offer cancels that penalty. The phrases are `relocation`, `relocation package`, `relocation assistance`, `relocate`, and `relocare`. An office job outside Europe is `CUT_LOCATION`. Remote is never that cut, even when the parsed country is `US`. An empty location is not cut.

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

**Body.** The CV tool terms stay. `max_total` is 4 (cap body stacking from ranked search). Added terms: `brasov` and `brașov` at 2, `bucuresti` and `bucurești` at 2, `cluj` at 2, `sibiu`, `hibrid`, and `proiectare` at 1.

**Cutoff and pay.** `buckets.top` is 6; TOP also requires a title phrase hit (`domain: architecture` rule in the scorer). Salary stays in the file with no floor and no penalty. A parsed salary does not move the score. `wellfound_roles` is removed. The other soft groups (`agency`, `culture_smell`, `part_time`) stay.

## Sources

Tiers are an order, not a new ingest mechanism.

**Tier 0 — existing collectors, left off.** `working_nomads`, `himalayas`, `remotify_europe`, `we_work_remotely`. They are not on this profile. The first three are keyword boards and would receive whatever `queries` says. `we_work_remotely` ignores queries and fetches its whole catalog; Andrew already does that, on the same cooldown key. Listing it here does not search architecture. The remote check is the empty-source score of rows already stored.

**Tier 1 — Romania.** `bestjobs`, `hipo`, and `ejobs`.

| Source id | Walk |
| --- | --- |
| `bestjobs` | Keyword. `https://www.bestjobs.eu/locuri-de-munca/{words}` with `+` between words, and the same slug with `-in-romania` and `-in-remote`. All five queries returned rows. The search is ranked, not filtered: `peisagist` also returns shop jobs, so the title still has to match. |
| `hipo` | Category, not keywords. `https://www.hipo.ro/locuri-de-munca/cautajob/Arhitectura-Design-Proiectare`. A GET `cuvintecheie` is ignored. On 2026-10-14 the category had 31 jobs, and its remote slice had 0. |
| `ejobs` | Keyword, hyphens. `https://www.ejobs.ro/locuri-de-munca/{words}` and remote `https://www.ejobs.ro/locuri-de-munca/remote/{words}`. `peisagist` and `urbanist` were 0. `desenator-tehnic` was 1476 and matches either word. |

`queries`, for `bestjobs` and `ejobs`: `arhitect proiectant`, `peisagist`, `urbanist`. Bare `arhitect` and `desenator tehnic` were dropped after ingest noise on ranked search. `designer de interior` is not a query. `hipo` is in `children` and does not receive this list.

**Tier 2 — opened 2026-10-14.** Children go in `children`, not in `queries`. These are European office boards plus a remote slice. Romania was empty on the industry sites. The office rows are the penalty case above, not a reason to skip the board.

| Source id | What to fetch |
| --- | --- |
| `dezeen_jobs` | `/job-category/{slug}/` for `architectural-designer`, `architectural-assistants`, `landscape-architect`, `part-1`, `part-2`. `landscape-designer` was empty and is not in the site nav. |
| `riba_jobs` | `/jobs/{slug}/` for `architectural-assistant`, `architect`, `part-1`, `part-2`, `landscape-and-public-realm`. |
| `landscape_institute` | `https://jobs.landscapeinstitute.org/jobs/`. One catalog, nine jobs, UK office. |
| `archgee` | `category=architect`, `category=landscape-architect`, and `remote=remote`. `country=ro` was 0. `interior-designer` is not sent. |
| `archinect` | `https://archinect.com/jobs/region/Remote/all/remote` (37). **Shipped in this slice:** remote region only. European region URLs were evaluated but stay paused. The unfiltered `/jobs/` list is the US catalog and is not fetched. |

`think_architecture` is dropped. See Out of scope.

**Tier 3 — European office, opened 2026-10-14.** `competitionline`, `baunetz`, `bdla`, `world_architects`. Germany, Austria, and Switzerland, so the office penalty applies. Romanian keywords returned nothing; that does not drop the board. Build these after the Romanian boards and Tier 2.

## How a run uses the file

`queries` is one list. It is fanned to every source that has no `children` entry.

`children` is a map from source id to that board's slugs:

```yaml
sources:
  - hipo
  - archinect
children:
  hipo:
    - Arhitectura-Design-Proiectare
  archinect:
    - remote
```

A source named in `children` expands to one ingest child per slug and does not also fan `queries`. `wellfound_roles` stays on `andrew` only. This profile does not use it.

`queries` holds the five Romanian keywords and fans to keyword sources (`bestjobs`, `ejobs`). Tier 0 stays off this profile so those boards never receive that list.

The score pass loads jobs with `first_seen_at` inside 10 days of the run start, plus jobs whose `posted_at` is already older than 30 days. It evaluates each one against this profile and upserts a match, including rejects. It does not look at `sources` and it does not call a collector.

A locked, cooling-down, or unbuilt source is a skipped child. It is not a failed run. A collector error uses the existing ingest retry.

## Identity

`StableJobID` is `company_key` plus the normalized title. The listing URL is not part of the key. Two posts with company `Confidențial` and title `Arhitect` are one row; the second write replaces the first.

Before the first Romanian collector, a placeholder company key uses the normalized listing URL as well. The placeholder keys are `confidențial`, `confidential`, `nedivulgat`, and an empty company name, after the existing company-key normalization. BestJobs leaves the name empty and sets `employer.confidential`. A named firm still collapses across boards. The rule lives in `internal/domain/utils` and applies to every profile.

## Per-source constraints

Opened 2026-10-14 in a browser. Salary on the posting maps to the existing salary field and is not scored.

**`bestjobs`.** The first page is in `__NEXT_DATA__` (`jobListCardsFromServer.items`, 24 cards). `?page=2` repeats that page. The next page is `GET /api/proxy/v2/jobs?limit=24&keyword={words}&cursor={nextCursor}`, which robots.txt allows. Stop when `nextCursor` is null. Do not send `_lat` or `_lon`; the page adds them from the client IP and they change the order. The detail page is the same blob at `props.pageProps.job`. Description is `job.description`. There is no posted date, so `posted_at` stays empty. Salary is `job.salary`. `estimatedSalary` is the board's own guess and is not stored. `job.remote` and `job.partialRemote` are the workplace. The stable id is `jobId`, not the slug. `ownApplyUrl` is stored only when it is an http(s) URL; it is sometimes an email or a bare host. curl got 200. The User-Agent must not contain a bot name robots.txt sets to `Disallow: /` (`AhrefsBot`, `SemrushBot`, `PetalBot`, and the rest of that list).

**`hipo`.** `/locuri-de-munca/` redirects to the homepage. The list is the category URL above, then `/{page}`, 40 jobs. The last page is `a.page.page-last`, or a page with no jobs. Drop promo id `103395`; it is on every page. Use the one-hyphen slug from `input[name="domeniis[]"]`. The triple-hyphen links in the HTML return 301. Remote, if used, is the third segment (`/Toate-Orasele/Remote`); in the city segment it is ignored. The card marker is one of Remote, Hybrid, On-site, or a city. The description is the visible text in `div.alertbox2`, not JSON-LD. That JSON-LD has raw control characters and broken encoding, and its `jobLocationType` disagrees with the page. The date is "Actualizat la", an update stamp, so it is not `posted_at`. The only salary on the job page is one number in JSON-LD, so `salary` stays empty. Apply is behind login. curl got 200. robots.txt closes `/public/prvfiles/`, `/objects/`, and `/angajator/`.

**`ejobs`.** curl gets Cloudflare 403, so this collector uses rod. About 15 fast requests produced a 429; wait ≥4s. Pages are `/pagina2` through `/pagina10`. Stop when `_nextBtnVisible` is false. The count in the title is larger than the list. The list page no longer has an ItemList. Read the row from `#__NUXT_DATA__` (`pinia.jobs._listItems`) and do not open the detail page. Company, salary, date, and city are on that row. Description and workplace stay empty. `sort-publish`, `workModels=OFFICE`, and `workModels=HYBRID` are disallowed. The remote path does not use `workModels`. The listing URL is `/user/locuri-de-munca/{slug}/{id}`. Apply is a button behind login. `desenator-tehnic` is about 1500 hits and is cut at 10 pages.

**`dezeen_jobs`.** WordPress SSR. curl gets Cloudflare 403, so this collector uses rod. Walk `/job-category/{slug}/page/N/` until `.main-jobs-list` has no job cards. `rel=next` is present on an empty page, so it is not the end marker. The Algolia block is empty in the HTML and is not the list. Description is JSON-LD. `baseSalary` in that JSON-LD is garbage (`USD` / `choose_my_own`); the salary string is `div.salary-range`. There is no remote field. `/location/remote/` exists and was empty. `/location/romania/` is 404.

**`riba_jobs`.** Madgex SSR. Category `/jobs/{slug}/`, then `/jobs/{slug}/2/`. robots.txt disallows employment-type, salary, and recruiter facet segments. `/jobs/remote/` and `/jobs/architect/romania/` return 200 and the unfiltered list, so those paths are not filters. Description is JSON-LD. The GTM dataLayer on the detail page described a different job; do not read it.

**`landscape_institute`.** One SSR catalog, `/jobs/`, nine jobs. `page/2/` repeats page 1. `search_region` is ignored, including `1153`. The region class on the list card (`job_listing_region-*`) is the only remote signal; the detail page does not have it. The five Romanian queries returned 0. Fetch the catalog anyway.

**`archgee`.** Laravel SSR. Crawl HTML only — robots.txt disallows `/api/`. Pages are `page=2`, 20 jobs, and the last page has no `rel=next`. Fetch the two categories and `remote=remote`. The card has title, firm, city, a Remote badge, and sometimes a salary string. JSON-LD `jobLocationType` is `TELECOMMUTE` when the job is remote. The description in JSON-LD and on the page is the same short teaser, a few hundred characters, not the full posting. The stable id is JSON-LD `identifier` (a UUID). The slug is not. Apply is a POST to `/jobs/{slug}/apply` with a CSRF token, so the stored URL is the listing. US office rows from the category pages are `CUT_LOCATION`.

**`archinect`.** SSR. Pages of 30. The remote list continues at `/jobs/region/Remote/all/remote/30`. European regions use `/jobs/region/{ISO2}/all/{name}`. Do not fetch the unfiltered `/jobs/` list. The total is the hidden `#arc_tr`, not the pager. An offset past the end is an empty list. The card has title, firm, a raw salary string, city, and a date. The detail description is in `div.JobDescription`. JSON-LD `description` contains raw newlines and does not parse. There is no remote field on the job; the region URL is the signal. The listing id is the number in `/jobs/entry/{id}`. The apply target is an email inside the description, obfuscated by Cloudflare. robots.txt has no `User-agent: *` block. Named crawlers including `ClaudeBot`, `Claude-Web`, `anthropic-ai`, `GPTBot`, `CCBot`, `PerplexityBot`, and `Scrapy` are `Disallow: /`. The User-Agent must not contain any name from that block. Named bots that are allowed get `Crawl-delay: 5`. Wait ≥5s between requests, with a site mutex. A browser got 200 on 2026-10-14. A non-browser client is untested.

**`competitionline`.** The list is `GET /api/jobs/`, which robots.txt allows. Page size 20, `page` starts at 0, stop when `next` is empty. Do not request `/api/modgnikehtotsyek`. The detail page has JSON-LD. There is no salary field and no remote field. The five Romanian queries returned 0; a German word such as `Architekt` returned rows. Cities were DE, AT, and CH.

**`baunetz`.** `/stellenmarkt/suche` is the full list and robots.txt disallows it, so it is not fetched. The allowed `/stellenmarkt/` page had 31 of 148 jobs in the HTML. Detail pages have JSON-LD. No remote field. No Romania rows.

**`bdla`.** SSR. `https://www.bdla.de/de/jobs-praktika/jobangebote`. Categories are `bl=` and `toj=`, one at a time; together they do not combine. Pages are `page=2`, 25 jobs in `div.listLine` only. The map markers on the same page are not the list. No JSON-LD. No country field and no remote field. `bl=international` was two jobs in Zürich.

**`world_architects`.** SSR. curl gets Cloudflare 403, so this collector uses rod. Fetch the unfiltered catalog `https://world-architects.com/en/jobs`, not `?term=`. Pages are `page=2`, 36 jobs, and the last page repeats the previous one. The country dropdown is Germany and Switzerland only. No remote field. The listing URL is a slug with no numeric id. The apply email is Cloudflare-obfuscated.

**Cooldown.** A 3600s global cooldown is the wrong pace. These boards need their own cooldown, 6–24 hours, not `JOBHOUND_INGEST_COOLDOWN_TTL_SEC`. `ejobs` also waits ≥4s between requests. `archinect` waits ≥5s.

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| Lada run, `sources` empty | no collector starts; the score pass still reads stored jobs |
| Job already stored by another profile | scored for Lada; not fetched again |
| Source with `children` | one child per slug; `queries` is not fanned to it |
| Source with no `children` | one child per `queries` entry |
| Same vacancy, named firm, two boards | one `jobs` row; `sources` accumulates |
| Placeholder company name | id includes the normalized listing URL |
| Empty location | no location cut |
| `bestjobs` | keyword plus `-in-romania` and `-in-remote`; cursor API for page 2+; no `_lat`/`_lon`; skip `estimatedSalary` |
| `hipo` | category slug only; drop id `103395`; description from the HTML, not JSON-LD |
| `ejobs` | rod; ≥4s; stop at `/pagina10`; rows come from the list payload, not the detail page |
| `archinect` | remote region only in this slice; never the unfiltered US list; ≥5s and a site mutex; User-Agent stays off the robots block list |
| `archgee` | HTML only; categories `architect` and `landscape-architect`, plus `remote=remote`; never `/api/`; no `interior-designer` |
| `dezeen_jobs` | rod; category pages until the list is empty; ignore `rel=next`; no Algolia |
| `riba_jobs` | category URLs only; no facet segments; `/romania/` and `/remote/` are not filters |
| `baunetz` | never `/stellenmarkt/suche` |
| `competitionline` | `/api/jobs/` only; never `/api/modgnikehtotsyek` |
| Office in Europe, not Romania, not remote | show, penalty −2; a relocation phrase cancels it |
| Office outside Europe | `CUT_LOCATION`; remote is exempt |
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
4. `bestjobs` (`net/http`, cursor API for later pages) and `hipo` (category HTML).
5. `ejobs` with rod.
6. `dezeen_jobs`, `riba_jobs`, and `landscape_institute`.
7. `archgee` (the two categories and `remote=remote`) and `archinect` (remote region only).
8. Tier 3: `competitionline`, `bdla`, `world_architects`, then `baunetz` within the pages robots.txt allows.

Early check on the first category board: one category fetch stores jobs, and a second fetch of the same category does not duplicate them.

## Out of scope

- Practice career pages, and the practice names already seen (Grimshaw, SOM, Herzog & de Meuron, Perkins&Will, Stantec, HDR, WSP, plus the WLA footer: Felixx, OMGEVING, SLA, LDA Design, LUC, Grant Associates, Studio Egret West, EPD Landscape, Arup, AtkinsRéalis, Benoy, WATG). That is [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).
- Turning on Tier 0. Those collectors stay off this profile.
- `undelucram`. Opened 2026-10-14. Four of the five queries returned 0. The fifth was one IT posting, Salesforce Solution Architect. No collector.
- `olx_ro`. Evaluated 2026-10-14; no collector. curl gets CloudFront 403. The list lived in `window.__PRERENDERED_STATE__` in pre-hydration HTML; rod could not reliably read that body on detail URLs (CDP `-32000`).
- `think_architecture`. Opened 2026-10-14. US only. `/remote` is 404. The first HTML is 50 cards and `?page=2` repeats them. The rest is Typesense on another host or a sitemap with no category split. Apply links go to `/go/`, which robots.txt disallows.
- IT sources copied from `andrew`: `builtin`, `golang_cafe`, `vue_jobs`, `wellfound`, every `consider_*`, every `getro_*`, `europe_remotely`.
- `academic_positions` — university posts, not this profile. `find-jobs?keywords=` returned HTML once and Cloudflare 403 on the next request.
- `wla_jobs` — four posts on the archive. The collector is not worth it.
- `greenjobs.co.uk` — category indexes are open, job detail URLs are disallowed, so there is no description to score.
- `architecture.co.uk` — the list is client-rendered; the usual WP Job Manager `admin-ajax.php` action was not called.
- `my.asla.org/joblink` — `robots.txt` refused automated access, and the board is US landscape.
- `worldarchitecture.org` — the list comes from `/ajax_job/`, which robots.txt disallows. Romania returned an empty list. Revisit only if the list becomes server-rendered.
- `landezine.com` — robots.txt is fine; `/jobs/` and `/category/jobs/` were 404. The real path was not found.

Still unchecked, so not a decision: Archinect from a non-browser User-Agent.
