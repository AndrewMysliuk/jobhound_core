# Sources inventory (job collectors)

**Spec**: `005-job-collectors`  
**Last Updated**: 2026-04-11  
**Status**: Draft

## Purpose

Single place for **which sites** we ingest from, **MVP vs later**, and **expected fetch tier** (hypothesis until spike). Implementation order follows MVP first; remaining sources are **planned**, not cancelled.

## Planned implementation order

1. **MVP (shipped):** Europe Remotely, Working Nomads, DOU.ua, Himalayas, Djinni — rows 1–5 below.
2. **Built In** — **before LinkedIn**: still usually browsable without login, but often heavy client-side rendering; confirm tier and wire in a spike.
3. **LinkedIn Jobs** — **last**: session/cookies, fragile selectors, and the strictest operational constraints — treat as the final integration.

Row numbers in the inventory table match this priority for planned sources (6–7).

## Exclusions

- **Remote OK** is **out of scope** for this product inventory (deliberately not listed).
- **Wellfound** ([wellfound.com/jobs](https://wellfound.com/jobs)) is **out of scope**: search is tied to curated typeahead entities and dynamic UI; cost vs maintainability is too high for this product line.

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
| 4   | [Himalayas](https://himalayas.app/jobs)          | **MVP** | **T2 (fact)**                        | Public JSON API only (no RSC/HTML crawl): `GET` `https://himalayas.app/jobs/api` + `.../jobs/api/search` — see `specs/005-job-collectors/resources/himalayas.md` and [Remote Jobs API](https://himalayas.app/api); max **20** jobs per request on browse; rate limit **429** |
| 5   | [Djinni](https://djinni.co/jobs/)                | **MVP** | **T2 (fact)**                        | `specs/005-job-collectors/resources/djinni.md` — `GET` listing `?all_keywords=&search_type=full-text&page=` (~**15**/page); detail `GET`; **`application/ld+json`** (`JobPosting`, optional **`baseSalary`**); listing may embed **array** of job JSON-LD; **delay** between requests (env); no login required for read path observed |
| 6   | [Built In](https://builtin.com/jobs)           | Planned | T2 / T3 (regional subsites)          | **Before LinkedIn** in rollout order; often heavy front-end |
| 7   | [LinkedIn Jobs](https://www.linkedin.com/jobs/)  | Planned | T3 + session                         | **Last** planned among this set — login/session, fragile selectors, highest operational risk |


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
- `specs/005-job-collectors/resources/himalayas.md` — MVP source 4 (public JSON API)
- `specs/005-job-collectors/resources/djinni.md` — planned source 5 (HTML + JSON-LD)
- `.specify/memory/constitution.md` — `Collector` contract, `session.Provider` for headless

