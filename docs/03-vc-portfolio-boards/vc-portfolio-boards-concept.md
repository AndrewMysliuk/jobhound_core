# VC portfolio job boards — concept

> **Sequence 3.** After [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md), before [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).
> **Backlog.** Two parts: research (how these boards work, and which other funds have the same thing), then wire the ones worth taking into the software profile next to the existing IT aggregators.

## Why

**Problem today:** the nine current aggregators are general remote-work boards. Venture funds run their own portfolio job boards, which are a different and denser pool: only funded companies, often posted before they reach a general board.

Two examples, both Consider tenants:

- [portfoliojobs.a16z.com](https://portfoliojobs.a16z.com/jobs) — board `andreessen-horowitz`, 17 870 vacancies.
- [jobs.sequoiacap.com](https://jobs.sequoiacap.com/jobs) — board `sequoia-capital`, 10 554 vacancies.

The interesting part: both pages carry a "Powered by Consider" mark. They are not two bespoke sites but two tenants of one platform. **One collector covers every fund using Consider.** The same applies to the other common platform in this space, Getro.

Second payoff: each listing has an Apply button pointing straight at the company's own ATS. That makes these boards a ready-made source of company-to-ATS pairs for [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md) — the two slices reinforce each other.

Primary entity: unchanged **Job**; this slice only adds sources.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Transport | **`net/http`** | `internal/collectors/consider`, `internal/collectors/getro` |
| Profiles | existing `sources` list | `internal/profiles` |

No new module. Each fund is one source id and one `collectors.Collector` instance. Consider is one search per fund; Getro is one search per profile query. Contracts: [`vc-portfolio-boards-schemas-contracts.md`](./vc-portfolio-boards-schemas-contracts.md).

## Research

Checked 2026-10-11 against the live boards. No login. The official key APIs (`boards.considerapi.com`, `api.getro.com/v2`) are not the path: each board already calls a public search endpoint. Opening that URL in a browser returns 404 because it is POST-only.

### Boards

Consider. Same search body on every host; the tenant is `board.id` with `isParent: true`. Sizes are the unfiltered catalogue where measured.

| Fund | URL | `board.id` | Jobs |
| ---- | --- | ---------- | ---- |
| a16z | https://portfoliojobs.a16z.com/jobs | `andreessen-horowitz` | 17 870 |
| Lightspeed | https://jobs.lsvp.com/jobs | `lightspeed` | 16 769 |
| Sequoia | https://jobs.sequoiacap.com/jobs | `sequoia-capital` | 10 554 |
| GV | https://jobs.gv.com/jobs | `gv` | 8 124 |
| Bessemer | https://jobs.bvp.com/jobs | `bessemer-ventures` | 7 764 |
| Balderton | https://careers.balderton.com/jobs | `balderton-capital` | 2 542 |
| Notion Capital | https://jobs.notion.vc/jobs | `notion-capital` | 624 |
| Atlantic Labs | https://jobs.atlantic.vc/jobs | `atlantic-labs` | 544 |
| Battery | https://jobs.battery.com/jobs | `battery-ventures` | live, size not measured |
| BIP Ventures | https://jobs.panoramic.vc/jobs | `panoramic-ventures` | live, size not measured |
| IVP | https://careers.ivp.com/jobs | `ivp` | live, size not measured |
| Kleiner Perkins | https://jobs.kleinerperkins.com/jobs | `kleiner-perkins` | live, size not measured |
| NEA | https://careers.nea.com/jobs | `nea` | live, size not measured |
| Peak XV | https://careers.peakxv.com/jobs | `sequoia-capital-india` | live, size not measured |
| Union Square Ventures | https://jobs.usv.com/jobs | `union-square-ventures` | live, size not measured |

Getro. Tenant is the numeric collection id. Search is `POST https://api.getro.com/api/v2/collections/{id}/search/jobs`. Counts below are the unfiltered catalogue where measured; the other rows answered 200 and the page is Getro.

| Fund | URL | Collection | Jobs |
| ---- | --- | ---------- | ---- |
| Accel | https://jobs.accel.com/jobs | 8672 | 39 486 |
| General Catalyst | https://jobs.generalcatalyst.com/jobs | 222 | 20 857 |
| Atomico | https://careers.atomico.com/jobs | 36986 | 2 412 |
| HV Capital | https://jobs.hvcapital.com/jobs | 234 | 2 162 |
| Point Nine | https://jobs.pointnine.com/jobs | 1680 | 1 759 |
| Earlybird | https://jobs.earlybird.com/jobs | 617 | 598 |
| Firstminute | https://jobs.firstminute.capital/jobs | 178 | 557 |
| Craft Ventures | https://jobs.craftventures.com/jobs | 340 | 1 768 |
| Emergence | https://talent.emcap.com/jobs | — | live |
| Foundry | https://jobs.foundry.vc/jobs | — | live |
| Greycroft | https://jobs.greycroft.com/companies | — | live |
| Insight Partners | https://jobs.insightpartners.com/jobs | — | live |
| Kapor Capital | https://jobs.kaporcapital.com/jobs | — | live |
| Khosla Ventures | https://jobs.khoslaventures.com/jobs | — | live |
| Kindred Ventures | https://kindredventures.getro.com/jobs | — | live |
| Lux Capital | https://jobs.luxcapital.com/jobs | — | live |
| Madrona | https://jobs.madrona.com/jobs | — | live |
| Menlo Ventures | https://jobs.menlovc.com/jobs | — | live |
| NFX | https://jobs.nfx.com/jobs | — | live |
| Primary | https://jobs.primary.vc/jobs | — | live |
| Redpoint | https://careers.redpoint.com/jobs | — | live |
| Sapphire | https://jobs.sapphireventures.com/jobs | — | live |
| Summit Partners | https://jobs.summitpartners.com/companies | — | live |
| SV Angel | https://jobs.svangel.com/jobs | — | live |
| Thrive | https://jobs.thrivecap.com/jobs | — | live |
| True Ventures | https://jobs.trueventures.com/companies | — | live |
| Upfront | https://jobs.upfront.com/jobs | — | live |
| Wing | https://careers.wing.vc/jobs | — | live |

Dropped. Own sites, not Consider or Getro:

- https://www.indexventures.com/startup-jobs/
- https://greylock.com/jobs/portfolio-jobs/
- https://jobs.a16z.com/ — not [portfoliojobs.a16z.com/jobs](https://portfoliojobs.a16z.com/jobs), which is the Consider board `andreessen-horowitz`

Blume (`jobs.blume.vc`) and GSV (`gsv.getro.com`) returned 404.

### Answers

1. **Public JSON, Tier 1.** Consider is `POST {host}/api-boards/search-jobs`. The call needs the session from `GET {host}/jobs`: that page sets the cookie and embeds `csrfToken`, sent back as `x-csrf-token`. Getro needs no cookie. Neither needs a headless browser.

2. **One collector per platform.** On Consider the host changes and `board.id` changes; the body does not. The same Vue.js + Europe/United States query returned 40 jobs on a16z and 10 on Sequoia. On Getro the only tenant key is the collection id. A fund is configuration. A dead tenant skips that source only.

3. **Keyword search, not a full catalogue.** Consider is faceted:

   - Values inside one field are a union. `Europe` + `United States` equals the sum minus jobs that match both. On a16z, Vue.js in Europe is 7, in the United States is 33, together 40.
   - Skills are the same union, not an intersection. Vue.js is 40, Go is 80, both together are 119 (40 + 80 − 1). All five profile skills together are 897, not the smaller intersection. One request carries every skill. A separate request per skill repeats the same jobs.
   - Different fields intersect. Skill and location together are smaller than either field alone. `remoteOnly: true` cuts the slice again: Vue.js + Europe + United States drops from 40 to 10. The five-skill slice drops from 897 to 211.
   - A skill hits both `requiredSkills` and `preferredSkills`. `scores.matchScore` is the share of requested skills present on the job (0.8 is 4 of 5). Go therefore also returns security and SRE roles where Golang is only a nice-to-have. `exclude_title` / `exclude_text` still drop those after ingest.
   - Skill ids come from `POST {host}/api-boards/autocomplete/skills` with `{"q","board","skipCompanyExcludes":false}`. They are not the profile words. For this profile: `vue` → `resume:Vue.js`, `golang` → `resume:Go` (the query is `golang`; `go` returns Google Cloud and Governance), `typescript` → `resume:TypeScript`, `react` → `resume:React.js` (leave out `resume:React Native`), `node` → `resume:Node.js` (leave out `resume:Node`).
   - The title box is `titlePrefix`. `vue` returns 0; `Software Engineer` returns 1 858. Skills are the filter that matches the profile queries. A bare `title` field is ignored.
   - Pagination is the cursor in `meta.sequence`, passed back on the next `meta`. Page size 50 works. The next page does not repeat job ids. The cursor disappears when the slice is exhausted. `promoteFeatured` does not change `total`.

   Fetch shape for one fund, one run:

   ```json
   {
     "meta": { "size": 50 },
     "board": { "id": "andreessen-horowitz", "isParent": true },
     "query": {
       "skills": ["resume:Vue.js", "resume:Go", "resume:TypeScript", "resume:React.js", "resume:Node.js"],
       "locations": ["Europe", "United States"]
     },
     "grouped": false
   }
   ```

   Getro is `POST https://api.getro.com/api/v2/collections/{id}/search/jobs`. The collection id is the tenant. No cookie and no `Origin`. `Accept` must be `application/json`; a missing header or `*/*` is HTTP 406. With that header the same call returns 78 for Accel `8672` and 18 for Craft `340`, matching each board's `?q=vue` page. `filters.q` does not filter. `filters.locations` does not filter. The working body is:

   ```json
   {
     "hitsPerPage": 20,
     "page": 0,
     "query": "vue",
     "filters": {
       "searchable_locations": ["Europe", "United States"],
       "searchable_location_option": ["remote"]
     }
   }
   ```

   - `query` is one profile word. Several words are an intersection: `node vue` is 1 job, `vue OR golang` is 0. Skills cannot share one request the way they do on Consider. One request per profile query, then dedup on job id.
   - `vue` is 78 and `vue.js` is 36, because `VueJS` in a title matches `vue` and misses `vue.js`. `golang` is 363. `go` is 11 107, because it matches "go-to-market". `node` is 594, `Node.js` is 461, `nodejs` is 66. Use the profile word, except `golang` instead of `go`.
   - `searchable_locations` is a union. On Accel, `vue` in Europe is 12, in the United States is 26, together 37 (12 + 26 − 1 job tagged with both).
   - `searchable_location_option: ["remote"]` intersects that slice. It returns the same jobs as `filters.work_mode: ["remote"]`. Accel `vue` + remote is 15. Accel `vue` + remote + Europe + United States is 8. Accel `golang` + the same pair of filters is 27 of 363.
   - `hitsPerPage` above 20 still returns 20. `page` is zero-based. Page 3 of 78 is the last 18, page 4 is empty. Pages do not overlap.
   - The hit is full text, not a skill id. `vue` also returns a retail-manager title and an account-executive title. `react` includes React Native.
   - The job object has `url` (apply link, 20/20 on the pages checked) and `organization.name`. It has no `organization.domain` and no description. `GET /api/v2/collections/{id}/jobs/{jobId}` is 401. Company domain is a separate `POST /api/v2/collections/{id}/search/companies` (`n8n` → `n8n.io`). Phrase scoring reads title plus description, so a Getro row scored from this payload only sees the title and the `skills` list.

4. **Apply URL and company site are on the Consider job.** `applyUrl` is the ATS or company apply link (`utm_source` may be appended). `url` is the same link without that parameter. `companyDomain` is the company site (`databricks.com`, not the board). On the pages checked, both were present. Getro's job `url` is the apply link. The company domain is not on that object.

Consider is one search per fund per run, all profile skills in that one `query.skills`. Getro is one search per profile query per fund, because a multi-word `query` intersects instead of uniting. Neither path pages the unfiltered catalogue.

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| The same vacancy on a portfolio board and a general aggregator | one `jobs` row by `dedup_key`; `sources` lists both |
| Portfolio board also carries the company's website | feeds `company_website`, which helps ATS resolution |
| Catalogue-only board | one child per run, no keyword matrix, same as `we_work_remotely` |
| Consider fund | one search per run: all profile skills in one `query.skills` (union), locations in one `query.locations` (union). Not one child per skill |
| Getro fund | one search per profile query: `query` is a single word (an extra word intersects). Locations in one `searchable_locations` (union). `remote` intersects |
| Tenant dies or renames | that one source is skipped; other tenants unaffected |

**Hard rule:** these sources go through the ordinary Redis lock/cooldown. A board with 18 000 listings must not be re-fetched per keyword without a cooldown.

## Implementation order

1. Research: list similar boards, answer the four questions, write both here. Done for Consider and Getro.
2. Both collectors, with every tenant whose key is known registered together. Parse fixtures for one response of each platform.
3. Add those source ids to `config/profiles/andrew.yaml` `sources` and to both collector allow-lists. They run in the same stage-1 ingest, with the same lock, cooldown and dedup.
4. Optional: feed the discovered apply URLs into company resolution.

Early check: count how many of the fetched listings carry a usable apply URL. That is the number that makes the companies slice cheaper.

## Out of scope

- Fund-specific scraping if the shared-platform assumption fails for a given fund. Dropped on that rule: Index Ventures, Greylock portfolio jobs, and `jobs.a16z.com`. The a16z Consider board stays `portfoliojobs.a16z.com`.
- Login-gated boards, "Interested"-style tracking features, talent-network signup flows.
- Architecture-domain sources — [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md).
