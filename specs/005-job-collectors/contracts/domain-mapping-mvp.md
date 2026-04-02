# Contract: MVP mapping → normalized `Job`

**Spec**: `005-job-collectors`  
**Status**: Draft

Target shape: **`domain.Job`** in `internal/domain/job.go`, plus **planned fields** below (add to domain and persistence when implementing this feature).

**`Job.ID`**: always from **`domain.StableJobID(Job.Source, listingURL)`** with **`listingURL` = `Job.URL`**, per `001-agent-skeleton-and-domain`.

**`UserID`**: MVP **collectors do not** populate this from site data — leave **`nil`**. **Orchestration / ingest** (`006` and callers) may set it when persisting **slot-scoped** rows per [`product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md) (single-tenant MVP, multi-user later).

- **`URL`** — canonical **job listing page** on the board (stable identity).
- **`ApplyURL`** — link where the candidate **applies** (ATS, external form, or `mailto:`). Empty if unknown or same-as-listing is not asserted.

---

## Planned `domain.Job` extensions

| Field | Type (MVP) | Meaning |
| ----- | ---------- | ------- |
| `SalaryRaw` | `string` | Opaque compensation text from the board; `""` if none |
| `Tags` | `[]string` | Skill/topic labels |
| `Position` | `*string` | **Nil** = no MVP keyword match; non-nil = inferred label (see below) |

**`Remote` (MVP rule):** collectors set **`Job.Remote`** to non-nil **`true`** or **`false`**: **`true`** if the substring **`remote`** appears (case-insensitive) in any of: **`Title`**, normalized **plain-text `Description`**, or **tag strings** from the board (see per-source columns). Otherwise **`false`**. **`nil`** remains valid for legacy or unknown rows (see `004` broad filter); MVP sources always populate **`true`**/**`false`** per this rule.

When these land, **`jobs`** storage needs matching columns — see **`jobs-table-extension.md`**.

---

## Cross-cutting normalization

### `Description`

- Source HTML (listing/detail or `_source.description`) is **stripped to plain text** for **`Job.Description`** (no HTML stored). Whitespace normalized reasonably (trim, collapse inner spaces/newlines as implementation chooses — document in code if non-obvious).

### `PostedAt`

- **Working Nomads:** parse **`_source.pub_date`** (ISO-8601). Parse failure for a hit → **entire `Fetch` error** (WN dates are authoritative structured data).
- **Europe Remotely:** prefer **absolute** detail line **`li.date-posted`** when it parses as a calendar date. Otherwise parse **relative** phrases from listing or detail **`posted_display`** (e.g. “Posted 12 hours ago”) using an **anchor** of **`time.Now().UTC()`** at parse time. Implementation may use a small **regex/table** for English phrases and/or a library such as **[github.com/olebedev/when](https://github.com/olebedev/when)**; **new site phrases** are added over time (spec/tests updated when discovered).
- **Europe — soft date failure:** if a **relative** (or unrecognized) **`posted_display`** cannot be parsed, set **`PostedAt` to zero**, **log a warning** with the raw string, and **do not** fail **`Fetch`** for that reason alone (see **`collector.md`**). Unparseable **absolute** detail date attempts should follow the same soft rule if the rest of the job is valid.

### `Position` (`*string`)

- **Nil** if no match.
- **Non-nil** only via **keyword** inference over a single searchable string: **`Title` + space + plain `Description` + space + space-joined tag strings** (tags from board: Europe `p.job_tags` tokens, Working Nomads `_source.tags` / `all_tags`). Matching is **case-insensitive**.
- **Keyword groups** (first matching group wins; order of groups below):

| Canonical value (`*string` points at) | Match if text contains any substring |
| ------------------------------------- | ------------------------------------ |
| `full-stack` | `full-stack`, `full stack`, `fullstack` |
| `frontend` | `frontend`, `front-end`, `front end` |
| `backend` | `backend`, `back-end`, `back end` |

- **Order of evaluation:** test **full-stack** group first, then **frontend**, then **backend** (so e.g. “full stack” does not lose to “frontend” in the same title).

---

## Country code

Resolve **`CountryCode`** (ISO 3166-1 alpha-2) using **`data/countries.json`** (`name` → `alpha_2`) plus aliases (e.g. UK → GB, USA → US). Unknown / ambiguous → **`""`**.

---

## Europe Remotely → `Job`

| `Job` field | Source (see `../resources/europe-remotely.md`) |
| ----------- | ---------------------------------------------- |
| `Source` | `europe_remotely` |
| `Title` | Detail `h1.page-title` (fallback: listing `h2.job-title`) |
| `Company` | Detail `li.job-company` (fallback: listing `div.company-name`) |
| `URL` | Absolute job listing page from card link |
| `ApplyURL` | Detail `a.application_button_link` `@href` if non-empty; else `""` |
| `Description` | Plain text from `div.job_listing-description` |
| `PostedAt` | Detail `li.date-posted` when parseable; else listing/detail `posted_display` relative parse; else zero + warn |
| `Remote` | Bool rule (title + description + tags from `p.job_tags`) |
| `CountryCode` | From location fields + countries dictionary |
| `SalaryRaw` | Listing `compensation_raw` and/or detail `compensation_meta_raw` |
| `Tags` | Tokens from `p.job_tags` (strip “Tagged as:” style prefixes in parser) |
| `Position` | Keyword inference only (`*string`); nil if no group matches |

---

## Working Nomads → `Job`

| `Job` field | Source (see `../resources/working-nomads.md`) |
| ----------- | ---------------------------------------------- |
| `Source` | `working_nomads` |
| `Title` | `_source.title` |
| `Company` | `_source.company` |
| `URL` | `https://www.workingnomads.com/jobs/{_source.slug}` |
| `ApplyURL` | See **ApplyURL matrix** below |
| `Description` | Plain text from `_source.description` (HTML input) |
| `PostedAt` | `_source.pub_date` (ISO-8601); parse error → **`Fetch` error** |
| `Remote` | Bool rule (title + description + `_source.tags` / `all_tags`) |
| `CountryCode` | `_source.locations` / `location_base` + dictionary |
| `SalaryRaw` | `_source.salary_range` or `_source.salary_range_short` when set |
| `Tags` | `_source.tags` or `all_tags` |
| `Position` | Keyword inference only (`*string`); nil if no group matches |

Omit jobs with `_source.expired == true`.

### ApplyURL matrix (Working Nomads)

| `apply_option` | `ApplyURL` |
| -------------- | ---------- |
| `with_your_ats` | `_source.apply_url` if non-empty; else `""` |
| `with_email` | If `_source.apply_email` non-empty: `mailto:` + email (single RFC 6068–style `mailto:addr`); else `""` |
| `with_our_ats` | `_source.apply_url` if non-empty (site-hosted flow); else `""` |

If a new `apply_option` value appears at runtime, **`Fetch` must error** (do not silently drop apply semantics).

---

## Related

- `collector.md`
- [`specs/000-epic-overview/product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md)
- `jobs-table-extension.md`
- `../resources/europe-remotely.md`, `../resources/working-nomads.md`
