# Architecture profile — concept

> **Sequence 2.** Next after [`docs/01-profiles-and-scoring/`](../01-profiles-and-scoring/profiles-and-scoring-concept.md). Then [`docs/03-vc-portfolio-boards/`](../03-vc-portfolio-boards/vc-portfolio-boards-concept.md), then [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).
> **Backlog.** Separate slice. Profiles-and-scoring only reserves `domain: architecture`. It does not create `architecture.yaml` or these collectors. Contracts and tasks stay absent until this slice is picked up.
> Checked 2026-10-10. Roles: Architectural Designer, Architectural Assistant, Landscape Designer.

## Why

**Problem today:** the software profile is one query list fanned across IT aggregators. Architecture boards are not that. Each one has its own category slugs, they are small, and they sit on the UK and the US. There is no architecture equivalent of that aggregator layer. None of the boards checked that day had a Romania listing.

**New solution:**

1. One `architecture` YAML, read by the same profile run as `andrew`. It does not share `andrew`'s `queries`.
2. Each source has its own child list: category slugs, one catalog, or one remote region. Those strings address that board. They are not a shared q-list, and they do not add score points (same idea as `wellfound_roles`).
3. Collectors are ordinary Tier-2 fetchers (`net/http` + goquery) on the existing ingest lock. No new module, no new HTTP API.
4. Scoring stays the v2 rule scorer. Phrase lists for this profile are written in the YAML when the slice is built. They are not copied from `andrew` and they are not invented here.
5. The volume is not these boards. It is practice career pages, which belongs to [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).

Primary entity: **Profile** (`architecture`).

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Ingest | **`net/http` + goquery** | `internal/collectors` |
| Profile file | **YAML** | `config/profiles/architecture.yaml`, `internal/profiles` |
| Orchestration | existing ingest + profile run | `internal/ingest/workflows`, `internal/scoring/workflows` |

No rod in the first cut. No new tables. Debug HTTP stays on `cmd/agent`.

## Sources

Six sources, in this order. Each row is its own child list.

| Source id | Site | Children |
| --- | --- | --- |
| `dezeen_jobs` | [dezeenjobs.com](https://www.dezeenjobs.com/) | `architectural-designer`, `architectural-assistants` (plural), `landscape-designer`, `landscape-architect`, `part-1`, `part-2` |
| `riba_jobs` | [jobs.architecture.com](https://jobs.architecture.com/) | `architectural-assistant`, `architect`, `part-1`, `part-2`, `landscape-and-public-realm` |
| `landscape_institute` | [jobs.landscapeinstitute.org](https://jobs.landscapeinstitute.org/) | one catalog child |
| `archgee` | [archgee.com/jobs](https://archgee.com/jobs) | category queries: Architect, Landscape Architect, Interior Designer |
| `archinect` | [archinect.com/jobs](https://archinect.com/jobs/) | one child: `/jobs/region/Remote/all/remote` |
| `think_architecture` | [thinkarchitecturejobs.com](https://thinkarchitecturejobs.com/) | `architectural-designer`, `landscape-architect`, `landscape` |

`dezeen_jobs` first. Its EU cities are Berlin, Basel, Madrid, and Paris, plus the UK and remote. `riba_jobs` and `landscape_institute` are UK and tiny (about 70 and 6 live posts). `archgee` has volume and a country filter, but the first page is US and full of agencies. `archinect` is a US board (667 jobs, 656 in the US, 41 remote) — the remote region only, not the catalog. `think_architecture` last: US only, and the published counts do not add up.

Two more, not in the six:

- `academic_positions` ([academicpositions.com](https://academicpositions.com/)) — the best EU country coverage found, but the posts are university jobs. In only if academic roles are in scope. If taken, wait ≥5s between requests.
- `wla_jobs` ([worldlandscapearchitect.com/job/](https://worldlandscapearchitect.com/job/)) — three paid posts. Titles are `Role | City, Country | Firm`, so a catalog parse needs no detail page. The useful part is the sponsor footer (practice names), not the board.

## Per-source constraints

**`dezeen_jobs`.** WordPress SSR. Listing `/job/{company}-{title}-{id}/`, category `/job-category/{slug}/`. Salary is on the card. The apply link is on the detail page. Sitemap: `https://www.dezeenjobs.com/sitemap_index.xml`.

**`riba_jobs`.** Madgex SSR. Category `/jobs/{slug}/` is allowed. Do not append employment-type, salary, or recruiter facet segments — robots.txt disallows those paths. `/jobs/` with no category redirected too many times; start from a category page. Listing `/job/{id}/{title-slug}/`.

**`landscape_institute`.** WP Job Manager SSR. Listing `/job/{slug}/`. Regions include the UK, Ireland, London, Remotely, and Overseas. robots.txt also names `/sitemap.rss`. If that feed is a real job list, this collector is RSS and needs no HTML parse. That check is still open.

**`archgee`.** Laravel SSR. Crawl HTML only — robots.txt disallows `/api/`. Pagination is `/jobs?page=N`. "1000 jobs found" is not the true total. EU density under the country filter is unverified.

**`archinect`.** SSR, pages of 30 at `/jobs/list/{offset}`. robots.txt has no `User-agent: *` block. Named crawlers including `ClaudeBot`, `GPTBot`, `anthropic-ai`, `CCBot`, and `Scrapy` are `Disallow: /`. Legitimate named bots get `Crawl-delay: 5`. The User-Agent must not contain any name from that block. Wait ≥5s between requests, with a site mutex, the same shape as `wellfound`, not the 1s `builtin` delay.

**`think_architecture`.** Astro SSR. Category pages live at the site root (`/architectural-designer`). Do not crawl `/go/` (outbound redirect, disallowed). Listing slugs are cut to about 24 characters plus a numeric suffix, so the slug is a weak id. Use `StableJobID` from the normalized URL, and test near-duplicate slugs.

The other four keep the existing 1s gap. All six are small.

**Cooldown.** A 3600s global cooldown is the wrong pace for boards this small. These sources need their own cooldown, 6–24 hours, not `JOBHOUND_INGEST_COOLDOWN_TTL_SEC`.

## Runtime

A profile run expands each architecture source into the children in the table. It does not cross those slugs with `andrew`'s queries, and it does not fan one phrase across every board.

A locked, cooling-down, or unbuilt source is a skipped child, same as v2. It is not a failed run. A collector error uses the existing ingest retry.

List pages and robots.txt were checked. Detail pages were not. Each collector still needs a fixture from one real detail page. Salary on the card (Dezeen, RIBA, Archinect, ArchGee) maps to the existing salary field.

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| Same vacancy on two of these boards | one `jobs` row; `sources` accumulates |
| Architecture child list | that source only; never the software `queries` |
| `archinect` | remote region only; ≥5s and a site mutex; User-Agent stays off the robots block list |
| `riba_jobs` | category URLs only; no facet segments |
| `archgee` | HTML pages; never `/api/` |
| `think_architecture` | never `/go/` |
| Board has no Romania rows | do not add a Romania query to force one |
| Source on cooldown | skipped child; the run still succeeds |

**Hard rule:** do not send a User-Agent that robots.txt names as disallowed. The gap is the site's own crawl-delay.

## Modules

| Module | Purpose |
| ------ | ------- |
| **`internal/collectors`** | one collector per source id above |
| **`internal/profiles`** | reads `architecture.yaml`; no new store |
| **`internal/ingest`** | existing lock, cooldown, child workflow |
| **`internal/scoring`** | existing profile run; this slice does not change the scorer |

## Layers

- **Saved** — `architecture.yaml`, `jobs` rows.
- **Run** — existing `profile_runs`, Redis lock and cooldown.
- **Wire** — no new HTTP. The child list is file content, not a request body.

## Implementation order

1. Wait until the v2 profile run exists. This slice does not change `andrew.yaml`.
2. `dezeen_jobs`: list parse and one detail-page fixture.
3. `riba_jobs`, then `landscape_institute`. Check `sitemap.rss` first and use it if it is a job feed.
4. `archgee`, then `archinect` (remote only), then `think_architecture`.
5. `config/profiles/architecture.yaml` with the child lists above. The profile run reads them per source. Scoring phrases are filled in that file, not in Go.

Early check: one Dezeen category fetch stores jobs, and a second fetch of the same category does not duplicate them.

## Out of scope

- Building any of this inside the profiles-and-scoring tasks.
- Practice career pages, and the practice names already seen (Grimshaw, SOM, Herzog & de Meuron, Perkins&Will, Stantec, HDR, WSP, plus the WLA footer: Felixx, OMGEVING, SLA, LDA Design, LUC, Grant Associates, Studio Egret West, EPD Landscape, Arup, AtkinsRéalis, Benoy, WATG). That is [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).
- National boards that were not opened, so nothing is claimed about them: competitionline, BauNetz, eJobs.ro, BestJobs.eu, Hipo.ro.
- `architecture.co.uk` — the list is client-rendered; the usual WP Job Manager `admin-ajax.php` action was not called.
- `my.asla.org/joblink` — `robots.txt` refused automated access, and the board is US landscape.
- `worldarchitecture.org` — the list comes from `/ajax_job/`, which robots.txt disallows. Romania returned an empty list. The job-type taxonomy is the richest checked (`Architectural designer`, `Architectural assistant`, `Landscape designer` as separate types). Revisit only if the list becomes server-rendered.
- `greenjobs.co.uk` — category indexes are open, job detail URLs are disallowed, so there is no description to score.
- `landezine.com` — robots.txt is fine; `/jobs/` and `/category/jobs/` were 404. The real path was not found.

Still unchecked, so not a decision: whether `sitemap.rss` is a job feed, ArchGee's real total and its EU density, whether academicpositions.com cards are server-rendered, and every detail page.
