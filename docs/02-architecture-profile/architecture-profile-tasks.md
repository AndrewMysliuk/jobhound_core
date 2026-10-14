# Architecture profile — tasks

> One agent, one checkbox. Read this file and `architecture-profile-schemas-contracts.md` before editing. If the concept disagrees with the contracts file, the contracts file wins.
> Do not add routes, tables, env keys, debughttp handlers, or boards that the contracts file does not name.
> Do not fetch live sites from tests.

---

## Scope

**In this slice**

1. `lada.yaml` phrase block from the concept Scoring section, with the contract overrides.
2. `children` on the profile and ingest fan-out.
3. Placeholder company ids include the listing URL.
4. Europe-office penalty and remote location exemption in the scorer.
5. Twelve-hour cooldown for the architecture source ids only.
6. The twelve collectors named in the contracts file, then the `sources` / `queries` / `children` block in `lada.yaml`.

**Paused (do not implement here)**

- `undelucram`, `olx_ro`, `think_architecture`, Tier 0 sources, practice career pages.
- Archinect regions other than remote.
- A rod fallback inside a `net/http` collector.
- Debug routes in `internal/collectors/handlers/debughttp/`.
- New product HTTP, new tables, new `JOBHOUND_*` keys.
- Changing `DefaultIngestCooldownTTLSec` (stays 3600).
- Decoding Cloudflare-obfuscated emails.

---

## 0. Schema / contract

- [x] **T1** — `internal/profiles/schema/profile.go`: add `Children map[string][]string`. `internal/profiles/impl/store.go`: read YAML `children`. A key whose source id is not in that profile's `sources` is `ErrProfileInvalid`. `store_test.go`: round-trip one map; reject an unknown source id.
- [x] **T2** — `internal/scoring/workflows/mappers.go`: `IngestChildren` follows the contracts rule (children slug → one child, `catalog` → empty `Query`, no `queries` copied onto a children source). `resourceLane`: `ejobs`, `dezeen_jobs`, and `world_architects` return `architecture-browser`. `mappers_test.go`: hipo with queries set still emits one child and does not copy queries; `catalog` emits `Query == ""`; ejobs, dezeen_jobs, and world_architects share a lane that is not `browser`.

---

## 1. Config / cooldown

- [x] **G1** — `internal/ingest/coordinator.go`: `RecordSuccessfulIngest` uses 43200 seconds for the twelve architecture source ids listed in the contracts file. Any other source id keeps `c.cooldownTTL`. `coordinator_test.go` covers both. Do not add an env key. Do not change `DefaultIngestCooldownTTLSec`.

---

## 2. Identity and score

- [x] **I1** — `internal/domain/utils/identity.go`: placeholder keys `""`, `confidential`, `confidențial`, `nedivulgat` (after `CompanyKey`) put the normalized listing URL into `ID`. A named company does not. Extend `internal/domain/utils/identity_test.go` with those two cases. Do not add a new test file.
- [x] **I2** — `internal/scoring/impl/evaluate.go`: location tier from the contracts table. Europe set and relocation phrases are constants in that file. Not a YAML penalty group. `evaluate_test.go`: remote + country `US` is not `CUT_LOCATION`; office `DE` scores −2 once; office `DE` with body `umzug` has no `europe_office` signal; office `RO` has no `europe_office` signal; office `US` is `CUT_LOCATION`; empty location is not cut.

---

## 3. Profile phrases

- [x] **Y1** — Rewrite the phrase block of `config/profiles/lada.yaml` as the concept Scoring section, plus the contracts overrides (drop `US` from `allow_countries`; do not add an Europe penalty group). Leave `sources: []` and `queries: []`. Do not register collectors in this task.

---

## 4. Collectors

Each task is one package. Follow the matching section in the contracts file. Register `SourceName` in `sourceids/ids.go` and construct it in `bootstrap/bootstrap.go`. User-Agent is the shared constant from **U1**. Fixture test, no live HTTP.

- [x] **U1** — `internal/collectors/utils`: export the User-Agent string from the contracts file. No other change.
- [x] **B1** — `internal/collectors/bestjobs`. Query fetches only the two suffixed URLs. API pagination must not drop the suffix filters and must not send `_lat` or `_lon`.
- [x] **B2** — `internal/collectors/hipo`. Category URL only. Drop id `103395`. Description from `div.alertbox2`. `posted_at` and `salary` empty.
- [x] **B3** — `internal/collectors/ejobs`. Rod. Both the keyword URL and `/remote/{words}`. Stop at `/pagina10`. List rows come from the Nuxt payload; detail pages are not opened. Sleep ≥ 4s is in the collector.
- [x] **B5** — `internal/collectors/dezeen`. Rod, lane `architecture-browser`. Stop on an empty `.main-jobs-list`. Ignore `rel=next`. Salary from `div.salary-range`, not JSON-LD `baseSalary`.
- [x] **B6** — `internal/collectors/riba`. Category path plus `/2/`. No facet segments. Do not read the GTM dataLayer.
- [x] **B7** — `internal/collectors/landscapeinstitute`. Source id `landscape_institute`. Single `/jobs/` document. Workplace from the list-card region class.
- [x] **B8** — `internal/collectors/archgee`. Three queries from the contracts table. Never `/api/`. Store the teaser as the description.
- [x] **B9** — `internal/collectors/archinect`. Remote region only. `net/http`, sleep ≥ 5s. Do not `json.Unmarshal` the JSON-LD. No email decoder.
- [x] **B10** — `internal/collectors/competitionline`. Unfiltered `/api/jobs/`. Never `/api/modgnikehtotsyek`. No `fs`.
- [x] **B11** — `internal/collectors/baunetz`. Only `/stellenmarkt/`. Never `/stellenmarkt/suche`.
- [x] **B12** — `internal/collectors/bdla`. Catalog pages, cards only in `div.listLine`. No `bl` or `toj`.
- [x] **B13** — `internal/collectors/worldarchitects`. Source id `world_architects`. Rod, lane `architecture-browser`. No `term`. Stop when the slug set repeats. `posted_at` from the list-card `time[datetime]`.

---

## 5. Wiring

- [x] **C1** — `config/profiles/lada.yaml`: set `queries`, `sources`, and `children` to the exact block in the contracts file. Only after **B1–B3** and **B5–B13** are in `KnownSourceIDs`. `internal/profiles/impl/store_test.go`: loading `lada.yaml` returns those sources and does not return an unknown-source error.

---

## Dependencies

```
U1 → B*
T1 → T2
I1, I2, Y1, G1 have no upstream task
C1 after B1–B3, B5–B13 and T1
```

**M1:** T1, I1, I2, Y1, G1, U1  
**M2:** T2, B1–B3 and B5–B13 (B* after U1; otherwise parallel)  
**M3:** C1

An agent on a **B** task does not edit `lada.yaml`, the scorer, or another collector package.
