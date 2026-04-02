# Sources inventory (job collectors)

**Spec**: `005-job-collectors`  
**Last Updated**: 2026-03-30  
**Status**: Draft

## Purpose

Single place for **which sites** we ingest from, **MVP vs later**, and **expected fetch tier** (hypothesis until spike). Implementation order follows MVP first; remaining sources are **planned**, not cancelled.

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
| 3   | Himalayas                                        | Planned | T2 / T3                              |                                               |
| 4   | Built In                                         | Planned | T2 / T3 (regional subsites)          | Often heavy front-end                         |
| 5   | Dou.ua                                           | Planned | T2 / T3                              |                                               |
| 6   | Djinni                                           | Planned | T2 + session or T3                   | Auth / rate limits stronger than plain boards |
| 7   | Wellfound                                        | Planned | T3 + session                         | Dynamic UI + account                          |
| 8   | LinkedIn Jobs                                    | Planned | T3 + session                         | Cookie/session; fragile selectors             |


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
- `.specify/memory/constitution.md` — `Collector` contract, `session.Provider` for headless

