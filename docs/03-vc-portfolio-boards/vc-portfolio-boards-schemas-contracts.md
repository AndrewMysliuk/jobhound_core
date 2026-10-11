# VC portfolio job boards — schemas and contracts

> Implementation contracts: types, ingest, storage, debug HTTP, errors.
> Concept: [`vc-portfolio-boards-concept.md`](./vc-portfolio-boards-concept.md).

Two collector packages. Every fund with a known tenant key is a source id, registered together. A fund is a configured instance, so a dead tenant skips that source only.

---

## Modules

| Module | HTTP | Persistence | Temporal |
|--------|------|-------------|----------|
| `internal/collectors/consider` | debug `POST /debug/collectors/consider` | — | — |
| `internal/collectors/getro` | debug `POST /debug/collectors/getro` | — | — |
| `internal/collectors` | — | — | — (`UnionQueryFetcher`, `ErrTenantUnavailable`) |
| `internal/ingest` | — | `ingest_watermarks` (unchanged) | `IngestSourceWorkflow` (input gains `Queries`) |
| `internal/scoring/workflows` | — | — | `IngestChildren` gains a union branch; `ChildSkipped` learns the tenant sentinel |
| `internal/jobs/storage` | — | `jobs.company_website` fill when empty | — |
| `internal/profiles` | — | YAML `sources` | — |

`internal/publicapi` has no new route. Company resolution stays in [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md). This slice writes `apply_url` and `company_website` and stops there.

No migration. No new env key. No headless browser. Skill autocomplete and the Getro job-detail endpoint are not called.

---

## Source ids

`Name()` is the fund source id. The worker map and both allow-lists (`cmd/api/main.go`, `cmd/worker/main.go`, the `knownCollectorSourceIDs` literals) include every id below. `config/profiles/andrew.yaml` `sources` lists the same ids next to the current aggregators.

Consider id: `consider_` plus the board id, hyphens replaced with underscores. Host is the origin; the session page is `{host}/jobs` and the search is `POST {host}/api-boards/search-jobs`.

| Source id | Host | `board.id` |
| --------- | ---- | ---------- |
| `consider_andreessen_horowitz` | `https://portfoliojobs.a16z.com` | `andreessen-horowitz` |
| `consider_lightspeed` | `https://jobs.lsvp.com` | `lightspeed` |
| `consider_sequoia_capital` | `https://jobs.sequoiacap.com` | `sequoia-capital` |
| `consider_gv` | `https://jobs.gv.com` | `gv` |
| `consider_bessemer_ventures` | `https://jobs.bvp.com` | `bessemer-ventures` |
| `consider_balderton_capital` | `https://careers.balderton.com` | `balderton-capital` |
| `consider_notion_capital` | `https://jobs.notion.vc` | `notion-capital` |
| `consider_atlantic_labs` | `https://jobs.atlantic.vc` | `atlantic-labs` |
| `consider_battery_ventures` | `https://jobs.battery.com` | `battery-ventures` |
| `consider_panoramic_ventures` | `https://jobs.panoramic.vc` | `panoramic-ventures` |
| `consider_ivp` | `https://careers.ivp.com` | `ivp` |
| `consider_kleiner_perkins` | `https://jobs.kleinerperkins.com` | `kleiner-perkins` |
| `consider_nea` | `https://careers.nea.com` | `nea` |
| `consider_sequoia_capital_india` | `https://careers.peakxv.com` | `sequoia-capital-india` |
| `consider_union_square_ventures` | `https://jobs.usv.com` | `union-square-ventures` |

Getro id: `getro_` plus the fund slug. Search is `POST https://api.getro.com/api/v2/collections/{collection_id}/search/jobs`. The fund site is not requested.

| Source id | Collection | Board |
| --------- | ---------- | ----- |
| `getro_accel` | 8672 | `https://jobs.accel.com/jobs` |
| `getro_general_catalyst` | 222 | `https://jobs.generalcatalyst.com/jobs` |
| `getro_atomico` | 36986 | `https://careers.atomico.com/jobs` |
| `getro_hv_capital` | 234 | `https://jobs.hvcapital.com/jobs` |
| `getro_point_nine` | 1680 | `https://jobs.pointnine.com/jobs` |
| `getro_earlybird` | 617 | `https://jobs.earlybird.com/jobs` |
| `getro_firstminute` | 178 | `https://jobs.firstminute.capital/jobs` |
| `getro_craft_ventures` | 340 | `https://jobs.craftventures.com/jobs` |

Live Getro boards whose collection id is still unknown stay off both allow-lists: Emergence, Foundry, Greycroft, Insight Partners, Kapor Capital, Khosla Ventures, Kindred Ventures, Lux Capital, Madrona, Menlo Ventures, NFX, Primary, Redpoint, Sapphire, Summit Partners, SV Angel, Thrive, True Ventures, Upfront, Wing. Adding one is a new row in the same table (`getro_` + slug, numeric collection id). It is not a new collector.

---

## Types

Tenant tables are Go values in the collector package. They are not YAML and not env.

### `internal/collectors/consider`

```go
type Tenant struct {
	SourceID string // consider_andreessen_horowitz
	Host     string // https://portfoliojobs.a16z.com
	BoardID  string // andreessen-horowitz
}

type Board struct {
	HTTPClient *http.Client
	Countries  *utils.CountryResolver
	Tenant     Tenant
}
```

`Name()` returns `Tenant.SourceID`.

Each `FetchUnion` builds its own cookie jar on a client that shares the process transport. The jar is not the shared bootstrap client: the session cookie belongs to that fund's fetch.

### `internal/collectors/getro`

```go
type Tenant struct {
	SourceID     string
	CollectionID int
}

type Board struct {
	HTTPClient *http.Client
	Countries  *utils.CountryResolver
	Tenant     Tenant
}
```

`Name()` returns `Tenant.SourceID`. The shared HTTP client is enough. No cookie and no `Origin`.

### `internal/collectors`

```go
// UnionQueryFetcher searches once with every profile query.
// Consider maps those words onto skill ids and sends them as one union.
type UnionQueryFetcher interface {
	FetchUnion(ctx context.Context, queries []string) ([]schema.Job, error)
}

// ErrTenantUnavailable means this fund's host or collection is gone.
// The profile run skips that source. Other tenants still run.
var ErrTenantUnavailable = errors.New("collector tenant unavailable")
```

Consider implements `UnionQueryFetcher`. `Fetch` returns an empty slice. Getro implements `QueryFetcher`. `Fetch` returns an empty slice. An empty Getro query returns an empty slice and does not call the API.

### `internal/ingest/schema`

```go
type IngestSourceInput struct {
	SourceID        string
	ExplicitRefresh bool
	// Query is one search string. Empty is the catalog lock segment.
	Query string
	// Queries is the full profile query list for a union search.
	// Set only for Consider. Query stays empty, so the lock segment is catalog.
	Queries []string
}
```

---

## Search

### Consider

One request carries every resolved skill. Locations are one union. `remoteOnly` is absent. Page size is 50. `grouped` is false. `board.isParent` is true.

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

Session, once per fetch, before the first search: `GET {host}/jobs`. Read `csrfToken` from the HTML (`"csrfToken":"..."`). Send it as `x-csrf-token` on every search POST. The cookie jar keeps `session`.

The next page copies `meta` from the response, including `sequence`, and sends the same `board` and `query`. Stop when `jobs` is empty, when `meta.sequence` is missing, or when every `jobId` on the page was already seen in this fetch. `maxPages` is 40. Hitting the cap returns the jobs already mapped.

Profile word → skill id. The map is code in the consider package. Unknown words are dropped. Zero resolved skills returns an empty slice and does not call the API.

| Profile query | Skill id |
| ------------- | -------- |
| `vue` | `resume:Vue.js` |
| `golang` | `resume:Go` |
| `typescript` | `resume:TypeScript` |
| `react` | `resume:React.js` |
| `node` | `resume:Node.js` |

`go` is not a skill query. `resume:React Native` and `resume:Node` are not sent.

### Getro

One profile word per request. `filters.q` and `filters.locations` are not used. `hitsPerPage` above 20 still returns 20, so the body sends 20. `page` is zero-based. `Accept` is `application/json`. Without it Getro returns 406.

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

The HTTP query is the trimmed profile word. The word `go` is sent as `golang`. Any other word is sent unchanged (`vue`, not `vue.js`). A multi-word query is sent as one string; profile queries are single words.

Read `results.jobs`. Stop when the page is empty or every job URL on the page was already seen. `maxPages` is 25. Hitting the cap returns the jobs already mapped.

---

## Job mapping

`Source` is the fund source id. `AssignStableID` runs after the fields below. Identity stays company key + title + `Location.Raw`. The apply URL is not part of the id.

A job with an empty title, an empty company, or both URL fields empty is dropped. The rest of the page is kept. The fetch log includes how many mapped jobs have a non-empty `ApplyURL`.

### Consider wire → `domain/schema.Job`

The search hit has no description. `requiredSkills` and `preferredSkills` are either a JSON array of `{id,label}` or a string containing that JSON.

| Job field | Wire |
| --------- | ---- |
| `Title` | `title` |
| `Company` | `companyName` |
| `CompanyWebsite` | `https://` plus `companyDomain` when the domain has no scheme. Empty domain stays empty |
| `URL` | `url` |
| `ApplyURL` | `applyUrl` when set, otherwise `url` |
| `Description` | skill `label`s from `requiredSkills`, then `preferredSkills`, de-duplicated, one per line. Skill id `resume:Go` also contributes the word `golang` |
| `Tags` | the same labels |
| `PostedAt` | `timeStamp` (RFC3339). Unparseable stays zero |
| `SalaryRaw` | `minValue-maxValue currency` when either bound is set |
| `Location` | below |

`URL` and `ApplyURL` keep their query string (`gh_jid` and the rest). They are not passed through `NormalizeListingURL`, which strips the query.

Location: `utils.ParseHiringScope` over `locations`, `normalizedLocations[].label`, and `regions[].label`. `Location.Raw` is `locations` joined with `, `. `remote: true` sets `Location.Type` to `remote`. Otherwise `hybrid: true` sets `hybrid`. Otherwise a non-empty `locations` list sets `office`.

### Getro wire → `domain/schema.Job`

The hit has `url`, `organization.name`, and `skills`. It has no description and no company domain. `GET /api/v2/collections/{id}/jobs/{jobId}` is not called. Company search is not called.

| Job field | Wire |
| --------- | ---- |
| `Title` | `title` |
| `Company` | `organization.name` |
| `CompanyWebsite` | empty |
| `URL` | `url` |
| `ApplyURL` | `url` |
| `Description` | `skills` joined, one label per line |
| `Tags` | `skills` |
| `PostedAt` | `created_at` as unix seconds. Zero or missing stays zero |
| `Location` | below |

`URL` keeps its query string.

Location: `utils.ParseHiringScope` over `locations`. `Location.Raw` is that list joined with `, `. `work_mode` `remote` sets `Location.Type` to `remote`.

Scoring reads title plus description, so the skill lines are the text a portfolio-only row can match. `react` matches a `React.js` label on a word boundary. `golang` matches the extra token on a Consider `resume:Go` skill; the label `Go` alone does not contain `golang`.

---

## Ingest children

`IngestChildren` in `internal/scoring/workflows`:

| Source | Children |
| ------ | -------- |
| `consider_*` | one child. `Queries` is the profile query list. `Query` is empty |
| `getro_*` | one child per profile query, same as the other keyword boards |
| catalog (`we_work_remotely`, `vue_jobs`, `golang_cafe`) | unchanged |
| `wellfound` | unchanged |

A Consider source id is the prefix `consider_`. Each fund is its own lane. Getro queries for one fund stay in order on that fund's lane. Different funds run together.

Workflow id for a Consider child is `ingest-{source_id}-catalog`, because `Query` is empty. The Redis lock and cooldown are `ingest:lock:{source_id}:catalog` and `ingest:cooldown:{source_id}:catalog`. One fund is one fetch per cooldown, not one fetch per skill.

`RunIngestSource`: when `Queries` is non-empty, call `UnionQueryFetcher.FetchUnion` and skip `FetchWithQuery` / `Fetch`. A collector that does not implement the interface is an error. The lock still uses `Query`, which is empty for that child.

---

## Storage

No new table and no new column.

`jobs.company_website` already exists. `mergeIngestJob` keeps the stored row and today does not copy `CompanyWebsite`. This slice adds one fill: when the stored website is empty and the incoming website is not, write the incoming value. A non-empty website stays. `URL` and `ApplyURL` stay on `preferURL`. Consider and Getro source ids are not ATS sources, so they do not win `preferURL` over an existing link.

Description, title, and location stay as first written.

Dedup is the existing stable id. A second board adds its source id to `jobs.sources`.

---

## HTTP — debug only

Product API: no new route.

`CollectorsPOSTBody` gains `source_id`. Two dev routes, registered in `registerRoutes()`:

### `POST /debug/collectors/consider`

Runs `FetchUnion` for one tenant.

**Body**

```json
{
  "source_id": "consider_andreessen_horowitz",
  "queries": ["vue", "golang"],
  "max_pages": 1
}
```

`queries` defaults to `vue`, `typescript`, `react`, `golang`, `node`. `max_pages` overrides the package cap for this call.

**Response `200`** — the existing debug job list. **Errors:** `400` when `source_id` is missing, `404` when it is not a Consider tenant.

### `POST /debug/collectors/getro`

Runs `FetchWithQuery` for one tenant.

**Body**

```json
{
  "source_id": "getro_accel",
  "q": "vue",
  "max_pages": 1
}
```

`q` defaults to `vue`.

**Response `200`** — the existing debug job list. **Errors:** `400` when `source_id` is missing, `404` when it is not a Getro tenant.

---

## Temporal

No new workflow. `IngestSourceWorkflow` is unchanged apart from the `Queries` field on its input.

| Kind | Name | Module |
|------|------|--------|
| Workflow | `IngestSourceWorkflow` | `internal/ingest/workflows` |
| Activity | `RunIngestSourceActivity` | `internal/ingest/workflows/activities` |

`ProfileRunWorkflow` starts the children from `IngestChildren`. A skipped tenant is a skipped child, counted on the run row, and is not a failed run.

---

## Config

No new `JOBHOUND_*` key. Hosts, board ids, and collection ids live in the two tenant tables.

---

## Errors

| Sentinel / HTTP | When |
| --------------- | ---- |
| `collectors.ErrTenantUnavailable` | `GET {host}/jobs` or the search POST returns 404 or 410. `ChildSkipped` is true (`errors.Is`). The other funds continue |
| fetch error | any other non-200, a missing `csrfToken`, or a 200 body with no `jobs` / `results.jobs` array. Temporal retries. This is not a skip |
| `400` debug | `source_id` missing |
| `404` debug | `source_id` is not in that platform's table |

A 429 stays a skip through the existing rate-limit check in `ChildSkipped`.

---

## Tests

| Layer | Where | What |
| ----- | ----- | ---- |
| Collector | `internal/collectors/consider/collector_test.go` | fixture: CSRF header, one body with every skill, `sequence` passed to page 2, `golang` on a `resume:Go` skill, query string kept on `applyUrl`, 404 → `ErrTenantUnavailable`, unknown query dropped |
| Collector | `internal/collectors/getro/collector_test.go` | fixture: `query` is the single word, `go` sent as `golang`, remote + Europe/United States filters, empty page stops, skills copied into `Description`, no company website |
| Workflow mapping | `internal/scoring/workflows` test next to `IngestChildren` | one `consider_*` child with `Queries` set and `Query` empty; one `getro_*` child per query; workflow id `ingest-consider_andreessen_horowitz-catalog` |
| Ingest activity | `internal/ingest/workflows/activities` | `Queries` set calls `FetchUnion` and does not call `Fetch` |
| Storage | `internal/jobs/storage` ingest merge test | empty `company_website` is filled; a non-empty one stays |

Fixtures live in each package's `testdata/`. One real response page per platform is enough. Do not add a test whose only job is the source-id string prefix.
