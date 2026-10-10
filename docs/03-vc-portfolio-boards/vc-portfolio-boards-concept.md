# VC portfolio job boards — concept

> **Sequence 3.** After [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md), before [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md).
> **Backlog.** Two parts: research (how these boards work, and which other funds have the same thing), then wire the ones worth taking into the software profile next to the existing IT aggregators.

## Why

**Problem today:** the nine current aggregators are general remote-work boards. Venture funds run their own portfolio job boards, which are a different and denser pool: only funded companies, often posted before they reach a general board.

Two examples, both large:

- [jobs.a16z.com](https://jobs.a16z.com/) — 689 companies, 18 444 vacancies.
- [jobs.sequoiacap.com](https://jobs.sequoiacap.com/jobs) — the same thing for Sequoia's portfolio.

The interesting part: both pages carry a "Powered by Consider" mark. They are not two bespoke sites but two tenants of one platform. If that holds, **one collector covers every fund using Consider**, which is a large share of the category. The same applies to the other common platform in this space, Getro.

Second payoff: each listing has an Apply button pointing straight at the company's own ATS. That makes these boards a ready-made source of company-to-ATS pairs for [`docs/04-companies-and-ats/`](../04-companies-and-ats/companies-and-ats-concept.md) — the two slices reinforce each other.

Primary entity: unchanged **Job**; this slice only adds sources.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Transport | **`net/http`** if a JSON endpoint exists, **goquery** if server-rendered, **go-rod** only as a last resort | `internal/collectors/consider` (working name) |
| Profiles | existing `sources` list | `internal/profiles` |

No new module layout: a portfolio board is an ordinary `collectors.Collector`, so it plugs into stage 1 unchanged.

## Research step (do this first)

Two jobs, both without production code.

**Find more boards like these.** a16z and Sequoia are the examples, not the list. Look for other fund portfolio boards on the same platforms (Consider, Getro, and whatever else turns up) and write the candidates here: fund, URL, platform, rough size. Skip boards that need a login.

**Then answer four questions** on the shortlisted ones. A few requests and the network tab are enough.

1. **Is there a public JSON API?** Platforms like this usually back the filter UI with a search endpoint. If one exists and takes a keyword, these become Tier-1 sources: cheap, paginated, no HTML parsing.
2. **Is one collector enough for many funds?** Confirm that a16z and Sequoia really serve the same platform, and whether the tenant is addressed by host or by a path/id. If so, a fund is a configuration line, not new code.
3. **Does search work per keyword, or is it catalogue-only?** This decides whether the collector implements `SlotSearchFetcher` (profile `queries` apply) or only `Fetch` (one catalogue pass, like `we_work_remotely` today).
4. **Does the listing expose the apply URL and the company website?** Both matter for dedup and for ATS resolution. A board that only links back to itself is much less valuable.

Record the answers in this file before writing code. If question 1 fails and the HTML needs a headless browser, the yield probably does not justify it — `builtin` and `golang_cafe` already show what rod costs in runtime and flakiness.

## Domain rules

| Situation | Behavior |
| --------- | -------- |
| The same vacancy on a portfolio board and a general aggregator | one `jobs` row by `dedup_key`; `sources` lists both |
| Portfolio board also carries the company's website | feeds `company_website`, which helps ATS resolution |
| Catalogue-only board | one child per run, no keyword matrix, same as `we_work_remotely` |
| Tenant dies or renames | that one source is skipped; other tenants unaffected |

**Hard rule:** these sources go through the ordinary Redis lock/cooldown. A board with 18 000 listings must not be re-fetched per keyword without a cooldown.

## Implementation order

1. Research: list similar boards, answer the four questions, write both here.
2. One collector against one tenant (a16z), with parse fixtures.
3. Add the other shortlisted tenants by configuration, not new collectors.
4. Add the source ids to `config/profiles/andrew.yaml` `sources`, next to the nine IT aggregators. They run in the same stage-1 ingest, with the same lock, cooldown and dedup.
5. Optional: feed the discovered apply URLs into company resolution.

Early check: count how many of the fetched listings carry a usable apply URL. That is the number that makes the companies slice cheaper.

## Out of scope

- Fund-specific scraping if the shared-platform assumption fails for a given fund.
- Login-gated boards, "Interested"-style tracking features, talent-network signup flows.
- Architecture-domain sources — [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md).
