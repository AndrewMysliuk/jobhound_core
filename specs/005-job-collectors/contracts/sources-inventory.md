# Sources inventory (job collectors)

**Spec**: `005-job-collectors`  
**Last Updated**: 2026-04-09  
**Status**: Draft

## Purpose

Single place for **which sites** we ingest from, **MVP vs later**, and **expected fetch tier** (hypothesis until spike). Implementation order follows MVP first; remaining sources are **planned**, not cancelled.

## Planned implementation order

1. **MVP (shipped):** Europe Remotely, Working Nomads, DOU.ua — rows 1–3 below.
2. **Then:** Himalayas, Djinni, Wellfound — exact sequence can move after per-site spikes (API vs HTML, rate limits).
3. **Built In** — **before LinkedIn**: still usually browsable without login, but often heavy client-side rendering; confirm tier and wire in a spike.
4. **LinkedIn Jobs** — **last**: session/cookies, fragile selectors, and the strictest operational constraints — treat as the final integration.

Row numbers in the inventory table match this priority for planned sources (4–8).

## Exclusions

- **Remote OK** is **out of scope** for this product inventory (deliberately not listed).

## Tier legend (theory)


| Tier   | Mechanism                                                         | When                                                         |
| ------ | ----------------------------------------------------------------- | ------------------------------------------------------------ |
| **T2** | `net/http` fetch + **goquery** HTML parse                         | Listing and detail HTML available without a headless browser |
| **T3** | **go-rod** (+ optional **session/cookies** file per constitution) | Client-rendered content, or login-required flows             |


There is **no separate “public API tier”** in requirements: delivery is **HTML pages** unless a later spike proves otherwise for a given source (document fact in the Notes column).

## Inventory


| #   | Source                                           | Status  | Tier (theory)                        | Notes                                         |
| --- | ------------------------------------------------ | ------- | ------------------------------------ | --------------------------------------------- |
| 1   | Europe Remotely ([euremotejobs.com](https://euremotejobs.com/)) | **MVP** | **T2** (fact) | `specs/005-job-collectors/resources/europe-remotely.md` — WP `admin-ajax.php` (`has_more` + HTML fragment), detail GET |
| 2   | [Working Nomads](https://www.workingnomads.com/) | **MVP** | **T2** (fact) | `specs/005-job-collectors/resources/working-nomads.md` — `POST` `jobsapi/_search` (Elasticsearch JSON); listing + description from `_source`; canonical URL `https://www.workingnomads.com/jobs/{slug}` (`?job=` alias) |
| 3   | [DOU.ua vacancies](https://jobs.dou.ua/vacancies/?descr=1) | **MVP** | **T2** (fact) | `specs/005-job-collectors/resources/dou.md` — `GET` listing (`search` + `descr=1`), `POST` `xhr-load` (CSRF + `count`) JSON `html` / `last` / `num`, detail `GET`; cookie jar + goquery |
| 4   | [Himalayas](https://himalayas.app/jobs)          | Planned | T2 / T3                              | Public job browse without login; site also advertises a Remote jobs API — prefer documented API in spike if it fits product |
| 5   | [Djinni](https://djinni.co/jobs/)                | Planned | T2 + session or T3                   | Auth / rate limits stronger than plain boards |
| 6   | [Wellfound](https://wellfound.com/jobs)          | Planned | T3 + session                         | Dynamic UI + account flows for some actions |
| 7   | [Built In](https://builtin.com/jobs)           | Planned | T2 / T3 (regional subsites)          | **Before LinkedIn** in rollout order; often heavy front-end |
| 8   | [LinkedIn Jobs](https://www.linkedin.com/jobs/)  | Planned | T3 + session                         | **Last** planned among this set — login/session, fragile selectors, highest operational risk |


After each **spike**, update **Tier (theory)** → **Tier (fact)** and **Notes** with concrete URLs, pagination, and stable vacancy identity (e.g. path segment vs query).

## Related

- [`specs/000-epic-overview/product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) — MVP phasing (“more sources” extends **`005`**)
- `specs/005-job-collectors/spec.md` — feature goal and scope
- `specs/005-job-collectors/contracts/collector.md` — module boundary + `Job.Source` values
- `specs/005-job-collectors/contracts/domain-mapping-mvp.md` — → `domain.Job` (MVP normalization + errors)
- `specs/005-job-collectors/contracts/jobs-table-extension.md` — optional DB columns
- `specs/005-job-collectors/contracts/test-fixtures.md` — fenced offline samples
- `specs/005-job-collectors/resources/europe-remotely.md` — MVP source 1
- `specs/005-job-collectors/resources/working-nomads.md` — MVP source 2 (`_search` JSON)
- `specs/005-job-collectors/resources/dou.md` — MVP source 3 (HTML + xhr-load)
- `.specify/memory/constitution.md` — `Collector` contract, `session.Provider` for headless

