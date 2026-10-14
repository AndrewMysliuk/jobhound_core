# Architecture profile — schemas and contracts

> Implementation contract for `docs/02-architecture-profile/architecture-profile-concept.md`.
> If this file and the concept disagree, **this file wins**.
> Do not add HTTP routes, tables, env keys, or collectors that this file does not name.

No product HTTP. No migration. Debug routes under `internal/collectors/handlers/debughttp/` are out of this slice.

---

## Modules

| Module | HTTP | Persistence | Temporal |
|--------|------|-------------|----------|
| `internal/profiles` | — | YAML file only | — |
| `internal/scoring` | — | existing `profile_matches` | existing `ProfileRunWorkflow` |
| `internal/scoring/workflows` | — | — | child `IngestSourceWorkflow` via `IngestChildren` |
| `internal/domain/utils` | — | — | — |
| `internal/ingest` | — | existing Redis cooldown key | existing `IngestSourceWorkflow` |
| `internal/collectors/<board>` | — | — | — |

`internal/publicapi` is unchanged.

---

## Decisions locked here

These override the concept where it still says otherwise.

1. **bestjobs** does not fetch the bare keyword URL. One query fetches only `{words}-in-romania` and `{words}-in-remote`.
2. **competitionline, baunetz, bdla, world_architects** do not receive `queries`. Each is one catalog child.
3. The Europe-office penalty is **code in the scorer**, not a `penalties_soft` phrase group. Do not match the word `office`.
4. Relocation phrases that cancel the penalty include the German words below. They live in the scorer, not in YAML.
5. **archinect** in this slice fetches only the remote region. European region URLs are paused.
6. **ejobs**, **dezeen_jobs**, and **world_architects** share one browser lane (`architecture-browser`). They do not join the Built In / Golang Cafe lane.
7. Architecture-board cooldown is 12 hours. `JOBHOUND_INGEST_COOLDOWN_TTL_SEC` stays 3600 for every other source.
8. Architecture profiles use `PostedAtRejectDaysArchitecture` (90) instead of 30 for `posted_at` cuts and stale job selection.
9. TOP for `domain: architecture` requires at least one `signals_plus.title` hit; body-only score stays REVIEW.
10. `lada.yaml` uses `buckets.top: 6` and `signals_plus.text.max_total: 4` to limit ranked-search noise.

---

## Profile file

`config/profiles/lada.yaml`. Phrase task and source task are separate. The phrase task leaves `sources: []` and `queries: []`.

### Phrases

Apply the concept section **Scoring** literally, including title weights, body terms, `buckets.top: 5`, body `max_total: 6`, `gate_role` additions and removals, `gate_hard` edits, deletion of `office_pressure` and `ambiguous_us`, deletion of `wellfound_roles` entries, salary with no floor and no penalty.

Also:

- Remove `US` from `allow_countries`. Remote jobs are exempt from `CUT_LOCATION` in the scorer, so a parsed country `US` on a remote job is not a cut.
- Do not add a `penalties_soft` group for Europe, office, or relocation.

### Sources, written only after every source id below is in `sourceids.KnownSourceIDs`

Keyword fan-out uses three queries only. Bare `arhitect` and `desenator tehnic` pull ranked IT/CAD noise on bestjobs and ejobs; hipo and the EU `children` boards cover category/catalog fetch.

```yaml
queries:
  - arhitect proiectant
  - peisagist
  - urbanist

sources:
  - bestjobs
  - hipo
  - ejobs
  - dezeen_jobs
  - riba_jobs
  - landscape_institute
  - archgee
  - archinect
  - competitionline
  - baunetz
  - bdla
  - world_architects

children:
  hipo:
    - Arhitectura-Design-Proiectare
  dezeen_jobs:
    - architectural-designer
    - architectural-assistants
    - landscape-architect
    - part-1
    - part-2
  riba_jobs:
    - architectural-assistant
    - architect
    - part-1
    - part-2
    - landscape-and-public-realm
  landscape_institute:
    - catalog
  archgee:
    - architect
    - landscape-architect
    - remote
  archinect:
    - remote
  competitionline:
    - catalog
  baunetz:
    - catalog
  bdla:
    - catalog
  world_architects:
    - catalog
```

`designer de interior` is not a query. `interior-designer` is not an archgee child. `landscape-designer` is not a dezeen child.

`bestjobs` and `ejobs` are absent from `children`. They fan `queries`.

---

## Types

### `internal/profiles/schema/profile.go`

Add to `Profile`:

```go
Children map[string][]string
```

YAML key `children`. Keys are source ids. Values are the slugs in the file above. No other field.

### `internal/scoring/workflows/mappers.go`

`IngestChildren` rule, in order:

1. Source id empty: skip.
2. `wellfound`: unchanged (one child per `WellfoundRoles` entry).
3. Existing catalog sources (`we_work_remotely`, `vue_jobs`, `golang_cafe`): unchanged.
4. `consider_*`: unchanged (one child, `Queries` set, `Query` empty).
5. **New.** If `profile.Children[id]` is non-empty: one `IngestSourceInput` per slug. `Query` is the slug. Do not copy `profile.Queries`. The slug `catalog` is the exception: `Query` is empty.
6. Otherwise: one child per `profile.Queries` entry, as today.

`resourceLane`: `ejobs`, `dezeen_jobs`, and `world_architects` return the lane name `architecture-browser`. Built In and Golang Cafe stay on `browser`.

### `internal/domain/utils/identity.go`

`AssignStableID` still sets `CompanyKey` via `CompanyKey`.

Placeholder keys, compared after `CompanyKey`: `""`, `confidential`, `confidențial`, `nedivulgat`.

For a placeholder key, `ID` is `companyKey + sep + normalized title + sep + normalized listing URL`. `NormalizeListingURL` is already in this package. A named company stays `companyKey + sep + normalized title`. The listing URL is not part of that id.

### Location tier (`internal/scoring/impl/evaluate.go`)

Run this before the existing allow-list cut. Do not encode it as phrase penalties.

| Job | Result |
| --- | --- |
| `Location.Type == "remote"` | no `CUT_LOCATION`, no Europe penalty, even when a country is `US` |
| No countries and no regions | no location cut, no Europe penalty |
| Any country outside the Europe set below, and not remote | `CUT_LOCATION` |
| Every country is in the Europe set, one of them is `RO`, not remote | no Europe penalty |
| Every country is in the Europe set, none is `RO`, not remote | one penalty, delta **−2**, rule name `europe_office` |
| That same job whose title or description hits a relocation phrase | the −2 is not applied |

Europe set, fixed in code, not read from YAML: `AT BE BG CY CZ DE DK EE ES FI FR GR HR HU IE IT LT LU LV MT NL PL PT SE SI SK IS LI NO CH GB RO`.

Relocation phrases, fixed in code, existing case-folded word-boundary match, title and description: `relocation`, `relocation package`, `relocation assistance`, `relocate`, `relocare`, `umzug`, `umzugskosten`. Do not add phrases.

`hybrid` is not remote. It takes the office row.

---

## Ingest cooldown

`DefaultIngestCooldownTTLSec` stays **3600**. No new `JOBHOUND_*` key.

In `RecordSuccessfulIngest`, the TTL for these source ids is **43200** seconds: `bestjobs`, `hipo`, `ejobs`, `dezeen_jobs`, `riba_jobs`, `landscape_institute`, `archgee`, `archinect`, `competitionline`, `baunetz`, `bdla`, `world_architects`. Every other source id keeps the coordinator TTL.

---

## Collectors

One package per source under `internal/collectors/<name>/`. Export `SourceName` equal to the source id string. Implement `collectors.Collector` and `collectors.QueryFetcher`. `Fetch` with an empty query behaves like `FetchWithQuery` with an empty query.

Register the id in `internal/collectors/sourceids/ids.go` and construct it in `internal/collectors/bootstrap/bootstrap.go`. Do not add a debughttp route.

Shared User-Agent, one constant in `internal/collectors/utils`, used by every new collector:

```text
Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Safari/537.36
```

Do not send `JobHound/1.0`. The User-Agent must not contain `ClaudeBot`, `Claude-Web`, `anthropic-ai`, `GPTBot`, `CCBot`, `PerplexityBot`, `Scrapy`, `AhrefsBot`, `SemrushBot`, `PetalBot`.

Tests use `httptest` or a fixture file. No live HTTP.

`posted_at` stays zero when the site has no publication date. Salary is stored only as a raw offer string. A site estimate is left empty. Call `domain/utils.AssignStableID` on every job.

### `bestjobs` — `net/http`

`FetchWithQuery("arhitect proiectant")` requests only:

- `https://www.bestjobs.eu/locuri-de-munca/arhitect+proiectant-in-romania`
- `https://www.bestjobs.eu/locuri-de-munca/arhitect+proiectant-in-remote`

Spaces in the query become `+`. Do not request the URL without `-in-romania` or `-in-remote`.

First page: cards from `__NEXT_DATA__` → `props.pageProps.jobListCardsFromServer.items`. `?page=2` is the same page. Do not use it.

Later pages: `GET https://www.bestjobs.eu/api/proxy/v2/jobs` with `limit=24`, `cursor` from `nextCursor`, and the filter fields already present on that first page, **except** `_lat` and `_lon`. Stop when `nextCursor` is null. If the first page does not show those filter fields, do not call the API with `keyword` alone.

Detail URL: `https://www.bestjobs.eu/loc-de-munca/{slug}`. Fields from `__NEXT_DATA__` → `props.pageProps.job`.

| Field | Source |
| --- | --- |
| title | `job.title` |
| company | `job.employer.employerName` |
| description | `job.description` |
| posted_at | empty. The site has no publication date |
| salary | `job.salary` only. Ignore `estimatedSalary` |
| workplace | `job.remote` → remote, `job.partialRemote` → hybrid, else office |
| city / country | `job.locations[].name` and `job.locations[].tooltip` |
| apply | `ownApplyUrl` only when it is `http` or `https` |
| id input | numeric `jobId` is not the listing URL. `AssignStableID` still runs |

### `hipo` — `net/http`

`FetchWithQuery` ignores the query string and fetches:

`https://www.hipo.ro/locuri-de-munca/cautajob/Arhitectura-Design-Proiectare`

Then `/{page}` from the pager. 40 jobs per page. Stop when the next page has no job rows or there is no `a.page.page-last` and page 2 is empty. Drop listing id `103395`.

Do not GET `cuvintecheie`. Do not POST the search form. Use the one-hyphen slug. Do not follow triple-hyphen links.

Detail: `https://www.hipo.ro/locuri-de-munca/locuri_de_munca/{id}/...`. Description is the visible HTML in `div.alertbox2`, not JSON-LD. JSON-LD is invalid and its `jobLocationType` disagrees with the page. Workplace is the `Remote work:` row (`Remote`, `Hybrid`, `On-site`). `posted_at` is empty (`Actualizat la` is an update stamp). `salary` is empty. Apply URL is empty.

### `ejobs` — rod, lane `architecture-browser`

Sleep at least 4s between requests. curl gets Cloudflare 403.

`FetchWithQuery("arhitect proiectant")` fetches both:

- `https://www.ejobs.ro/locuri-de-munca/arhitect-proiectant`
- `https://www.ejobs.ro/locuri-de-munca/remote/arhitect-proiectant`

Spaces become `-`.

Pagination: `/pagina2` … `/pagina10` only. Stop when `_nextBtnVisible` is false, or at `/pagina10`. Do not request `sort-publish`, `sort-geo`, `workModels=OFFICE`, or `workModels=HYBRID`.

The list page no longer has an `ItemList`. Read each row from `#__NUXT_DATA__` `pinia.jobs._listItems`: `id`, `title`, `slug`, `company.name`, `salary`, `locations[0].address`, `creationDate`. The listing URL is `/user/locuri-de-munca/{slug}/{id}`. Do not open that detail page. Description and workplace stay empty.

Detail JSON-LD is valid for title, company, description, `datePosted`, city, and country `RO`. Salary is the visible summary `Salariu`, not JSON-LD. Workplace is the visible summary `Mod de lucru`: `Remote`, `Hibrid`, or anything else → office. Apply URL is empty.

### `dezeen_jobs` — rod, lane `architecture-browser`

curl gets Cloudflare 403.

`FetchWithQuery` receives one category slug. URL:

`https://www.dezeenjobs.com/job-category/{slug}/`

then `/page/2/`, `/page/3/`, … Stop when `.main-jobs-list` contains no `li.job_listing`. `rel=next` is present on empty pages. Do not use it. Do not read the Algolia block. Do not fetch `/feed/`.

Cards are only under `.main-jobs-list` (the nav repeats `ul.job_listings`). Description and title and company come from detail JSON-LD. Salary is the visible `div.salary-range`. Ignore JSON-LD `baseSalary`. Listing id is the trailing number in `/job/{slug}-{id}/`. There is no remote field; leave workplace empty unless the location term is `remote`. Apply URL is the `href` already in the first HTML. When the block says sign-in or a geo gate, leave apply empty.

### `riba_jobs` — `net/http`

`FetchWithQuery` receives one category slug. URL:

`https://jobs.architecture.com/jobs/{slug}/`

then `https://jobs.architecture.com/jobs/{slug}/2/`. Stop when `link rel=next` is absent. Do not append employment-type, salary, or recruiter segments. Do not request `/jobs/remote/` or `/jobs/{slug}/romania/`: those return the unfiltered list.

Description, title, company, `datePosted`, city, and country come from detail JSON-LD. Salary is the definition-list `Salary` row. Do not read the GTM dataLayer. Listing id is the number in `/job/{id}/{slug}/`. Strip `TrackID` and `utm_*`. Apply URL is the listing URL. There is no workplace field.

### `landscape_institute` — `net/http`

The child query is empty (`catalog`). Fetch only:

`https://jobs.landscapeinstitute.org/jobs/`

Do not fetch `/jobs/page/2/` (it repeats page 1). Do not send `search_keywords` or `search_region`.

Workplace comes from the list-card class `job_listing_region-*` (`remotely` → remote, `hybrid` and `homebased` → hybrid). The detail page does not carry that class. Description and `datePosted` come from detail JSON-LD. Salary is JSON-LD `baseSalary.value.value` when it is present (free text). Listing URL is `/job/{slug}/`. Stable id input for `AssignStableID` is still company + title; keep the post id on the job only if a field already exists. Do not add a field. Apply URL is the mailto in the first HTML, after HTML-entity decode.

### `archgee` — `net/http`

| Query | URL |
| --- | --- |
| `architect` | `https://archgee.com/jobs?category=architect` |
| `landscape-architect` | `https://archgee.com/jobs?category=landscape-architect` |
| `remote` | `https://archgee.com/jobs?remote=remote` |

Then `&page=2`. Stop when `rel=next` is absent. Never request `/api/` or `category=interior-designer`.

Description on the detail page is a short teaser. Store that teaser. Do not invent a second request for a full description. Workplace: JSON-LD `jobLocationType` `TELECOMMUTE` → remote, and the card badge Remote / Hybrid / On-site when the JSON-LD value is absent. Salary raw string is the one in `meta[name="description"]` when present. Listing id for dedup is `AssignStableID`; also keep the JSON-LD `identifier` UUID only by using it as `job.ID` **before** `AssignStableID` if the company is a placeholder. Named companies use `AssignStableID` only. Apply URL is empty (POST with CSRF).

### `archinect` — `net/http`

`FetchWithQuery("remote")` fetches only:

`https://archinect.com/jobs/region/Remote/all/remote`

then `/jobs/region/Remote/all/remote/30`, `/60`, … while the offset is below `#arc_tr`. Stop when `li.Entry` is empty. Sleep at least 5s between requests. Do not fetch `/jobs/` or any other region.

Detail URL `/jobs/entry/{id}`. Description is `div.JobDescription`. JSON-LD is not valid JSON. Do not parse it with `encoding/json`. Salary is the visible pay string. `datePosted` may be read from the visible posted date. Workplace is remote because the list URL is the remote region. Apply URL is empty when the only address is Cloudflare email obfuscation. Do not add a decoder in this slice.

### `competitionline` — `net/http`

The child query is empty. List:

`GET https://www.competitionline.com/api/jobs/`

Page size 20. `page` starts at 0. Stop when `next` is empty. Do not send `fs`. Do not request `/api/modgnikehtotsyek`.

Detail `https://www.competitionline.com/de/jobs/{id}` or `GET /api/jobs/{id}`. Title, company, description, `publication_date`, city, and country come from that JSON. Salary is empty. Workplace is empty. Apply URL is `directApply` only when it is `http` or `https`.

### `baunetz` — `net/http`

The child query is empty. Fetch only `https://www.baunetz.de/stellenmarkt/`. Take the job links in that HTML (31 on 2026-10-14). Do not request `/stellenmarkt/suche` or `/suche`. Do not paginate.

Detail JSON is `window.__INITIAL_STATE__.nonPersistentStore.jobDetailsPageContent` or JSON-LD. Title, company, description, `publicationDatetime`, and city come from there. Country is empty (the contact address country is the employer's country, not the job). Salary is empty when `baseSalary` is 0. Workplace is empty. Apply URL is `contactAddress.applicantPortal` when it is `http` or `https`, else the email is not stored as apply URL.

### `bdla` — `net/http`

The child query is empty. Fetch:

`https://www.bdla.de/de/jobs-praktika/jobangebote?view=ccbk`

then `&page=2`. Count cards only in `div.listLine`. Stop when that count is 0. Do not send `bl` or `toj`. Do not count `details=` inside the map script.

Detail `?details={id}`. There is no JSON-LD. Title is `h1`. Company is the `Firma` row. Description is the paragraph after `Stellenbeschreibung`. City is the `Einsatzort` row. Country is empty. `posted_at` may be parsed from `Angebot vom`. Salary is empty. Workplace is empty. Apply URL is an `http(s)` link in the contact block. A `mailto:` is not an apply URL.

### `world_architects` — rod, lane `architecture-browser`

curl gets Cloudflare 403. The child query is empty. Fetch `https://world-architects.com/en/jobs` and `?page=2`, `?page=3`, … Do not send `term`. Stop when the set of `/en/jobs/{slug}` links repeats the previous page. The footer pager is not the end marker.

Detail `https://world-architects.com/en/jobs/{slug}`. Title is `h1`. Company is the provider link. Description is `div.col-md-7`. City and country are the `Job Location` row, split on the last comma. `posted_at` is the `time[datetime]` on the **list card** (the detail date has no year). Salary is empty. Workplace is empty. Apply URL is empty. Normalize the listing URL to `https://world-architects.com/en/jobs/{slug}` with no query string.

---

## Errors

No new product API errors. Unknown source id in YAML stays `profiles.ErrProfileInvalid`. A `net/http` collector that cannot pass Cloudflare returns the HTTP error. It does not switch to rod inside the same function. `dezeen_jobs` and `world_architects` are rod, same as `ejobs`. Cooldown and lock stay `ingest.ErrCooldownActive` and `ingest.ErrLockHeld` and still skip the child.

---

## Tests (boundaries)

| Layer | Where | What |
| ----- | ----- | ---- |
| Profile load | `internal/profiles/impl/store_test.go` | `children` round-trip; unknown source id still fails |
| Fan-out | `internal/scoring/workflows/mappers_test.go` | children do not receive `queries`; `catalog` has empty `Query`; `ejobs`, `dezeen_jobs`, and `world_architects` share `architecture-browser` |
| Identity | exercised by a collector test or `internal/domain` caller already covered by collector save path | placeholder company id includes the normalized URL; a named company does not |
| Score | `internal/scoring/impl/evaluate_test.go` | remote US is not cut; DE office is −2; DE office plus `umzug` is not −2; RO office is not −2; US office is cut |
| Cooldown | `internal/ingest/coordinator_test.go` | architecture source TTL 43200; another source keeps the coordinator TTL |
| Collector | `internal/collectors/<name>/*_test.go` | fixture HTML, no live network |
