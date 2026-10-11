# VC portfolio job boards — tasks

> Agent implementation plan. One checkbox is one agent. Do that checkbox only, then stop.
> Behavior that is already written in [`vc-portfolio-boards-concept.md`](./vc-portfolio-boards-concept.md) and [`vc-portfolio-boards-schemas-contracts.md`](./vc-portfolio-boards-schemas-contracts.md) is fixed. Do not invent tenants, skill ids, filters, routes, or columns.
> Each task leaves the tree compiling: `make test` for the packages it touches, and `make build` when it changes `cmd/`. Mark only your own checkbox `[x]` when its **Done** line is true.
> No manual QA, DoD, E2E, or browser walkthroughs in this file. No frontend tasks. Do not create `jobhound_frontend/docs/03-vc-portfolio-boards/`.

---

## Scope

**In this slice**

1. One Consider collector and one Getro collector. Every fund in the contract source-id tables is a configured instance (`Name()` is that source id).
2. Consider searches once per run with every resolved skill. Getro searches once per profile query. Both stay on the existing Redis lock, cooldown, and dedup path.
3. A dead tenant skips that source only. `jobs.company_website` is filled when the stored value is empty.
4. Two dev-only debug routes. No product HTTP route.

**Paused (do not implement here)**

- Getro boards whose collection id is unknown (Emergence, Foundry, Greycroft, Insight Partners, Kapor Capital, Khosla, Kindred, Lux, Madrona, Menlo, NFX, Primary, Redpoint, Sapphire, Summit, SV Angel, Thrive, True, Upfront, Wing). A new fund is a new tenant row later, not a new collector.
- Dropped sites: Index Ventures, Greylock portfolio jobs, `jobs.a16z.com`, Blume, GSV.
- Login-gated boards, talent-network signup, skill autocomplete (`/api-boards/autocomplete/skills`), Getro `GET /jobs/{jobId}`, Getro company search.
- Company and ATS resolution — [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md). This slice writes `apply_url` and `company_website` and stops.
- Architecture-domain sources — [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md).
- A new `JOBHOUND_*` key, a migration, a public `/api/v1` route, a headless browser, or a new Temporal workflow.

---

## 0. Schema / contract

- [x] **T1** — `internal/collectors/contract.go`: add `UnionQueryFetcher` (`FetchUnion(ctx, queries []string) ([]schema.Job, error)`) and sentinel `ErrTenantUnavailable` (`errors.New("collector tenant unavailable")`). Do not add implementations.
  **Done:** package builds. No new test file.

- [x] **T2** — `internal/ingest/schema/ingest_source.go`: add `Queries []string` on `IngestSourceInput`. `Query` stays the lock segment (empty means catalog). `Queries` is the full profile query list and is set only for a Consider child. Do not change workflow registration.
  **Done:** package builds. Existing ingest callers still compile.

---

## 1. Collectors

Tenant tables are Go values in the collector package. They are not YAML and not env. Export the full table as `Tenants() []Tenant` so **C1** can range it without copying hosts or collection ids. `Fetch` returns an empty slice and does not call the network. Tests use `httptest` and `testdata/`. Do not call the live boards. Do not add a test whose only job is the source-id string prefix.

- [x] **K1** — `internal/collectors/consider`: the whole package. `Board` is `HTTPClient`, `Countries`, `Tenant`, and `MaxPages` (`0` means 40). `Name()` returns `Tenant.SourceID`. Register every Consider row in the contracts (15 funds).

  `FetchUnion` builds its own `cookiejar` on a client that shares the process transport. The jar is not the bootstrap client. Before the first search, `GET {host}/jobs`, read `"csrfToken":"..."` from the HTML, and send that value as `x-csrf-token` on every search POST. The jar keeps `session`.

  One POST `{host}/api-boards/search-jobs` carries every resolved skill. Body: `meta.size` 50, `board.id` + `isParent: true`, `grouped: false`, `query.locations` `Europe` and `United States`, no `remoteOnly`. The next page copies `meta` from the response, including `sequence`, and sends the same `board` and `query`. Stop when `jobs` is empty, when `meta.sequence` is missing, or when every `jobId` on the page was already seen in this fetch. Do not append that page. Hitting `MaxPages` returns the jobs already mapped and is not an error.

  Skill map, exact match on the trimmed profile word: `vue` → `resume:Vue.js`, `golang` → `resume:Go`, `typescript` → `resume:TypeScript`, `react` → `resume:React.js`, `node` → `resume:Node.js`. Unknown words are dropped. `go` is not a key. Do not send `resume:React Native` or `resume:Node`. Zero resolved skills returns an empty slice and does not call the API.

  Map a hit to `domain/schema.Job`, then `AssignStableID`. `Source` is the fund source id. Drop a hit with an empty title, an empty company, or both URL fields empty; keep the rest of the page. `Title` ← `title`, `Company` ← `companyName`, `URL` ← `url`, `ApplyURL` ← `applyUrl` when set else `url`. Keep the query string (`gh_jid` and the rest). Do not pass either URL through `NormalizeListingURL`. `CompanyWebsite` is `https://` plus `companyDomain` when the domain has no scheme; empty domain stays empty; a domain that already has a scheme stays as-is. `Description` is skill `label`s from `requiredSkills`, then `preferredSkills`, de-duplicated, one per line. Those two fields are either a JSON array of `{id,label}` or a string containing that JSON. Skill id `resume:Go` also contributes the line `golang`. `Tags` are the same labels. `PostedAt` is `timeStamp` (RFC3339); unparseable stays zero. `SalaryRaw` is `minValue-maxValue currency` when either bound is set. `Location.Raw` is `locations` joined with `, `. `ParseHiringScope` over `locations`, `normalizedLocations[].label`, and `regions[].label`. `remote: true` sets `Location.Type` to `remote`; else `hybrid: true` sets `hybrid`; else a non-empty `locations` list sets `office`.

  `GET {host}/jobs` or the search POST returning 404 or 410 is `collectors.ErrTenantUnavailable`. Any other non-200, a missing `csrfToken`, or a 200 body with no `jobs` array is a fetch error (not the sentinel). A 429 is that fetch error and the error text contains `429`. An empty `jobs` array stops pagination; it is not a fetch error. Log the source id, how many jobs were mapped, and how many of those have a non-empty `ApplyURL`.

  `collector_test.go`: CSRF header is sent, one body contains every resolved skill, `sequence` is passed to page 2, a `resume:Go` skill adds `golang` to the description, the `applyUrl` query string is kept, 404 returns `ErrTenantUnavailable`, an unknown query is dropped and does not add a skill. Fixture under `testdata/`. A synthetic page that carries these wire fields is enough.
  **Done:** `go test ./internal/collectors/consider` passes those cases. `Fetch` does not dial.

- [x] **G1** — `internal/collectors/getro`: the whole package. `Board` is `HTTPClient`, `Countries`, `Tenant`, and `MaxPages` (`0` means 25). `Name()` returns `Tenant.SourceID`. Register every Getro row in the contracts (8 funds: Accel 8672, General Catalyst 222, Atomico 36986, HV Capital 234, Point Nine 1680, Earlybird 617, Firstminute 178, Craft Ventures 340). The shared HTTP client is enough. No cookie and no `Origin`. `Accept` is `application/json` (without it Getro returns 406). The fund site is not requested.

  `FetchWithQuery` POSTs `https://api.getro.com/api/v2/collections/{collection_id}/search/jobs`. Body: `hitsPerPage` 20, `page` zero-based, `query` the trimmed profile word, `filters.searchable_locations` `Europe` and `United States`, `filters.searchable_location_option` `["remote"]`. Do not send `filters.q` or `filters.locations`. The word `go` is sent as `golang`. Any other word is sent unchanged (`vue`, not `vue.js`). A multi-word query is sent as one string. An empty query returns an empty slice and does not call the API.

  Read `results.jobs`. Stop when the page is empty or every job URL on the page was already seen. Do not append that page. Hitting `MaxPages` returns the jobs already mapped and is not an error. 404 or 410 is `collectors.ErrTenantUnavailable`. Any other non-200, or a 200 body with no `results.jobs` array, is a fetch error. A 429 is that fetch error and the error text contains `429`. An empty jobs array stops pagination; it is not a fetch error.

  Map a hit, then `AssignStableID`. `Source` is the fund source id. Drop a hit with an empty title, an empty company, or an empty `url`; keep the rest of the page. `Title` ← `title`, `Company` ← `organization.name`, `CompanyWebsite` stays empty, `URL` and `ApplyURL` ← `url` (keep the query string; do not `NormalizeListingURL`). `Description` is `skills` joined, one label per line. `Tags` are `skills`. `PostedAt` is `created_at` as unix seconds; zero or missing stays zero. `Location.Raw` is `locations` joined with `, `. `ParseHiringScope` over `locations`. `work_mode` `remote` sets `Location.Type` to `remote`. Do not call `GET /api/v2/collections/{id}/jobs/{jobId}` or company search. Log the source id, how many jobs were mapped, and how many of those have a non-empty `ApplyURL`.

  `collector_test.go`: `query` is the single word, `go` is sent as `golang`, the body has remote plus Europe and United States, an empty page stops, skills are copied into `Description`, `CompanyWebsite` is empty. Fixture under `testdata/`. A synthetic page that carries these wire fields is enough.
  **Done:** `go test ./internal/collectors/getro` passes those cases. `Fetch` does not dial.

---

## 2. Storage

No new table and no new column. Description, title, and location stay as first written.

- [x] **S1** — `internal/jobs/storage/merge.go`: when the stored `company_website` is empty and the incoming website is not, write the incoming value. A non-empty website stays. Do not add `consider_*` or `getro_*` to `atsSource`. `URL` and `ApplyURL` stay on `preferURL`, so a portfolio row does not replace an existing link. Extend `repository_ingest_test.go`: an empty website is filled; a non-empty website stays; a later `consider_*` row does not replace the stored URL or apply URL.
  **Done:** those storage cases pass.

---

## 3. Ingest

No new workflow. `IngestSourceWorkflow` is unchanged apart from the `Queries` field **T2** added. Redis lock and cooldown still use `Query`.

- [x] **A1** — `internal/ingest/workflows/activities/ingest.go`: when `Queries` is non-empty, call `UnionQueryFetcher.FetchUnion` with that slice and do not call `FetchWithQuery`, `Fetch`, or `FetchIncremental`. A collector that does not implement `UnionQueryFetcher` returns an error. That error must not contain `unknown source_id`, `429`, or `collector tenant unavailable`. The lock still uses `Query` (`Redis.Begin` unchanged). An empty `Queries` slice keeps today's query / incremental / catalog branches. In `ingest_test.go`: `Queries` set calls `FetchUnion` and does not call `Fetch`; `Queries` set on a plain `Collector` returns the error above and does not call `Fetch`.
  **Done:** those activity tests pass. `impl/` is not involved. This package does not import the consider or getro packages.

- [x] **W1** — `internal/scoring/workflows/mappers.go`. `IngestChildren`: a source id with prefix `consider_` is one child, `Queries` is the profile query list, `Query` is empty. A source id with prefix `getro_` stays on the keyword path: one child per profile query, `Queries` left empty. `wellfound`, `we_work_remotely`, `vue_jobs`, and `golang_cafe` stay as they are. Do not put every Consider fund on one lane. `resourceLane` already keys by source id; leave that. `IngestChildWorkflowID` already turns an empty query into `catalog`; leave it. `ChildSkipped` is true for `errors.Is(err, collectors.ErrTenantUnavailable)` and, same as the lock-held fallback, when the error text contains `collector tenant unavailable`. A 429 still skips through the existing rate-limit check. A missing `FetchUnion` does not skip.

  Test in the same package: one `consider_andreessen_horowitz` child with `Queries` set and `Query` empty; workflow id `ingest-consider_andreessen_horowitz-catalog`; one `getro_accel` child per query; two Consider funds are two lanes; two Getro queries for one fund are one lane, in order; `ChildSkipped` is true for the sentinel and for an error whose text is only that sentence; catalog and Wellfound children are unchanged.
  **Done:** that test passes. No new workflow type.

---

## 4. Debug HTTP

Product API: no new route. Do not import `internal/publicapi`. Do not put a package-level function in a route file; a shared lookup goes in `helpers.go`. Validation failures use `http.Error`, same as the other debug routes. Success is the existing `runCollectorResponse` JSON (`ok`, `collector`, `count`, `jobs`). These two routes do not go through `runCollectorDebug`. Copy the board before applying `max_pages`, so the bootstrap value is not mutated.

- [x] **H1** — `internal/collectors/schema/debug_http.go`: add `source_id` and `queries` to `CollectorsPOSTBody`. `q` and `max_pages` already exist. `parseCollectorsPOSTBody` uses `DisallowUnknownFields`, so these keys have to be on the struct before **H2**. Do not change route behavior in this task.
  **Done:** `go test ./internal/collectors/handlers/debughttp` still passes.

- [x] **H2** — `POST /debug/collectors/consider` in its own route file, registered in `registerRoutes()`. Add `considerBoards map[string]*consider.Board` as the last argument of `NewHTTPHandler`. Update `cmd/agent` and `newTestHandler` to pass `nil` so the tree compiles. Do not build the boards here (**C1**).

  Body `source_id` missing or blank is 400. An id that is not in the map is 404. Otherwise call `FetchUnion` on a copy of that board. Omitted `queries` defaults to `vue`, `typescript`, `react`, `golang`, `node`. `max_pages` overrides `MaxPages` on the copy only. Handler test: 400, 404, and a 200 that uses the default query list when `queries` is omitted (the board's HTTP client is an `httptest` server; assert the search body contains the five default skill ids).
  **Done:** those handler tests pass and `make build` passes.

- [x] **H3** — `POST /debug/collectors/getro` in its own route file, registered in `registerRoutes()`. Add `getroBoards map[string]*getro.Board` after the consider map on `NewHTTPHandler`. Update `cmd/agent` and `newTestHandler` to pass `nil`.

  Body `source_id` missing or blank is 400. An id that is not in the map is 404. Otherwise call `FetchWithQuery` on a copy of that board. Omitted `q` defaults to `vue`. `max_pages` overrides `MaxPages` on the copy only. Handler test: 400, 404, and a 200 whose request query is `vue` when `q` is omitted.
  **Done:** those handler tests pass and `make build` passes.

---

## 5. Wiring

Composition only. No fetch rules in `main`. Do not add these collectors to `MVPMulti` (`Fetch` is empty, and ingest does not use that wrapper).

- [x] **C1** — Build every `Tenants()` row. Consider and Getro boards share the process HTTP client and the country resolver. `bootstrap.MVPCollectors` gains two return slices, `[]collectors.Collector`, one per platform. Update `MVPCollector` to ignore those slices. `cmd/worker` `ingestCollectorMap` takes the two slices and keys each non-nil collector by `Name()`. `cmd/api` and `cmd/worker` `knownCollectorSourceIDs` gain the same ids. `config/profiles/andrew.yaml` `sources` lists them after the current nine, Consider table order then Getro table order. `internal/profiles/impl/store_test.go` `contractSourceIDs` matches that list exactly. `cmd/worker/ingest_collectors_test.go` expects every new id when the slices are set, and expects them absent when the slices are nil. Himalayas-nil and golang_cafe-nil behavior stays. `cmd/agent` passes the built boards into the debug handler maps from **H3**, keyed by source id.
  **Done:** `make test` and `make build` pass. `andrew.yaml` loads. No new env key.

- [x] **C2** — Rewrite the current-behavior lines in `docs/REPO_SNAPSHOT.md` that still say the product polls only the nine aggregators. Stage 1 also starts one catalog child per `consider_*` source (`Queries` set, lock segment `catalog`) and one keyword child per `getro_*` query. A 404 or 410 tenant is `sources_skipped`, not a failed run. Re-ingest fills an empty `company_website` and does not let a portfolio URL replace a stored URL. Known source ids are the nine aggregators plus the Consider and Getro ids from the contracts. Do not list the unknown-collection Getro funds as live sources. Leave slots, stage 3, and the profile HTTP sections as they are.
  **Done:** those snapshot sections match this slice. No other doc file changes.

---

## Dependencies

```
T1 → K1
T1 → G1
T1 → A1
T1 → W1
T2 → A1
T2 → W1
H1 → H2
K1 → H2
H2 → H3
G1 → H3
K1 → C1
G1 → C1
H3 → C1
S1 → C1
A1 → C1
W1 → C1
C1 → C2
```

K1, G1, S1, A1, and W1 do not depend on each other. H2 and H3 are sequential because both change `NewHTTPHandler`.

**Milestone 1:** T1, T2, H1. Contracts compile; debug JSON accepts the new keys.
**Milestone 2:** K1, G1, S1, A1, W1. Collectors, website fill, and the ingest branch exist. The worker does not run the new sources yet.
**Milestone 3:** H2, H3, C1, C2. Debug routes, allow-lists, `andrew.yaml`, and the snapshot match the contracts.
