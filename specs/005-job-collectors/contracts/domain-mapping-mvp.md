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
| `TimezoneOffsets` | `[]float64` | Optional **UTC offset hours** from the board (e.g. Himalayas `timezoneRestrictions`); empty or `nil` = none / unknown |

**`Remote` (MVP rule):** collectors set **`Job.Remote`** to non-nil **`true`** or **`false`**: **`true`** if the substring **`remote`**, Ukrainian **`віддалено`**, or Ukrainian phrase **`віддалена робота`** appears (case-insensitive) in any of: **`Title`**, normalized **plain-text `Description`**, **tag strings** from the board, or **extra strings** documented per source (implemented via **`collectors/utils.RemoteMVPRule`** — e.g. DOU listing/detail location text; Himalayas **`excerpt`** and comma-joined **`locationRestrictions`**). Otherwise **`false`**. **`nil`** remains valid for legacy or unknown rows (see `004` broad filter); MVP sources always populate **`true`**/**`false`** per this rule.

When these land, **`jobs`** storage needs matching columns — see **`jobs-table-extension.md`** (including **`timezone_offsets`** when **`TimezoneOffsets`** is implemented).

---

## Cross-cutting normalization

### `Description`

- Source HTML (listing/detail or `_source.description`) is **stripped to plain text** for **`Job.Description`** (no HTML stored). Whitespace normalized reasonably (trim, collapse inner spaces/newlines as implementation chooses — document in code if non-obvious).

### `PostedAt`

- **Working Nomads:** parse **`_source.pub_date`** (ISO-8601). Parse failure for a hit → **entire `Fetch` error** (WN dates are authoritative structured data).
- **Europe Remotely:** prefer **absolute** detail line **`li.date-posted`** when it parses as a calendar date. Otherwise parse **relative** phrases from listing or detail **`posted_display`** (e.g. “Posted 12 hours ago”) using an **anchor** of **`time.Now().UTC()`** at parse time. Implementation may use a small **regex/table** for English phrases and/or a library such as **[github.com/olebedev/when](https://github.com/olebedev/when)**; **new site phrases** are added over time (spec/tests updated when discovered).
- **Europe — soft date failure:** if a **relative** (or unrecognized) **`posted_display`** cannot be parsed, set **`PostedAt` to zero**, **log a warning** with the raw string, and **do not** fail **`Fetch`** for that reason alone (see **`collector.md`**). Unparseable **absolute** detail date attempts should follow the same soft rule if the rest of the job is valid.
- **DOU.ua:** parse Ukrainian calendar phrases from listing/detail **`div.date`** (e.g. `9 квітня`, `8 квітня 2026`) with month-name table + optional year; anchor **`time.Now().UTC()`** when year is omitted (if parsed date is implausibly in the future vs anchor, treat as previous calendar year). **Soft failure** matches Europe: unparseable display → **`PostedAt` zero** + warning, continue (**`collector.md`**).
- **Himalayas:** parse **`pubDate`** as **Unix epoch seconds** (integer) → **`time.Unix(sec, 0).UTC()`**. Missing, zero, or non-numeric → **`PostedAt` zero** + warning, continue (**soft failure**, same spirit as Europe/DOU).
- **Djinni:** parse **`datePosted`** from detail **`JobPosting`** JSON-LD (ISO-8601 datetime string). Missing, empty, or parse error → **`PostedAt` zero** + warning, continue (**soft failure**, same spirit as Himalayas).
- **Built In:** parse **`datePosted`** from detail **`JobPosting`** JSON-LD when present (ISO-8601 datetime or date string). Missing, empty, or parse error → **`PostedAt` zero** + warning, continue (**soft failure**, same spirit as Djinni).

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

## DOU.ua → `Job`

| `Job` field | Source (see `../resources/dou.md`) |
| ----------- | ---------------------------------- |
| `Source` | `dou_ua` |
| `Title` | Detail `h1.g-h2` (fallback: listing `div.title a.vt`) |
| `Company` | Listing `div.title strong a.company` (no separate detail company selector in wire doc) |
| `URL` | Canonical vacancy URL from listing card |
| `ApplyURL` | `""` when not exposed on detail (future: map site apply control if product agrees) |
| `Description` | Plain text from detail `div.b-typo.vacancy-section` |
| `PostedAt` | Detail `div.date` when Ukrainian phrase parses; else listing `div.date`; else zero + warn |
| `Remote` | **`RemoteMVPRule`**: title, description, tags from `a.badge`, plus listing **`span.cities`** and detail **`span.place`** as extra string hints (variadic tail on the helper — same pattern as Himalayas **`excerpt`** + **`locationRestrictions`**) |
| `CountryCode` | From listing `span.cities` / detail `div.sh-info span.place` + countries dictionary |
| `SalaryRaw` | Detail `div.sh-info span.salary` when present |
| `Tags` | Text from detail `a.badge` links (optional; empty if none) |
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

## Himalayas → `Job`

Wire: **`../resources/himalayas.md`** (`GET` JSON only).


| `Job` field | Source |
| ----------- | ------ |
| `Source` | `himalayas` |
| `Title` | `title` |
| `Company` | `companyName` |
| `URL` | Canonical absolute URL from **`guid`**, else **`applicationLink`**; normalize (strip query/fragment) per shared URL helper |
| `ApplyURL` | **`""`** when the only apply/listing link is the Himalayas job page (same host/path family as **`URL`**). If the API later exposes a distinct external apply URL, set **`ApplyURL`** to that and keep **`URL`** as the board listing page. |
| `Description` | Plain text from **`description`** (HTML input) |
| `PostedAt` | From **`pubDate`** (Unix seconds); soft-fail to zero + warn per **PostedAt** rules above |
| `Remote` | **`collectors/utils.RemoteMVPRule`** with the same MVP substring rule as other sources: pass **`title`**, plain **`Description`**, **tag strings** (see **`Tags`** below), then **additional hints** like **DOU.ua** uses for `locationRaw`: include **`excerpt`** and a single comma-joined string of **`locationRestrictions`** (order preserved). Do not treat **`timezoneRestrictions`** as text hints unless stringified for debugging only — numeric offsets are not substring-matched for `remote`. |
| `CountryCode` | First successful **`data/countries.json`** match when iterating **`locationRestrictions`** (name/slug-style strings); unknown → **`""`** |
| `SalaryRaw` | If both **`minSalary`** and **`maxSalary`** are null/absent → **`""`**. Otherwise opaque string from min/max + **`currency`** (e.g. `90k-120k USD` or `min-max CURRENCY` per implementation consistency). |
| `Tags` | Concatenate non-empty trimmed strings from **`categories`**, then **`seniority`**, then **`parentCategories`** (dedupe while preserving first-seen order if practical). |
| `TimezoneOffsets` | Copy **`timezoneRestrictions`** floats when present; **`nil`** or empty slice when absent |
| `Position` | Keyword inference only (`*string`) over **title + description + tags** |

---

## Djinni → `Job`

Wire: **`../resources/djinni.md`** — listing **`GET`** + detail **`GET`**; primary fields from detail **`application/ld+json`** **`JobPosting`**.

| `Job` field | Source |
| ----------- | ------ |
| `Source` | `djinni` |
| `Title` | JSON-LD **`title`** |
| `Company` | JSON-LD **`hiringOrganization.name`** |
| `URL` | Absolute URL from JSON-LD **`url`**, normalized (strip query/fragment); must match canonical **`/jobs/{id}-{slug}/`** pattern |
| `ApplyURL` | **`""`** (apply flows are Djinni-login-gated in normal UX — revisit only if a stable external apply URL is documented) |
| `Description` | Plain text from JSON-LD **`description`** (escape sequences / newlines as delivered) |
| `PostedAt` | JSON-LD **`datePosted`** per **PostedAt** rules above |
| `Remote` | If JSON-LD **`jobLocationType`** equals **`TELECOMMUTE`** (case-insensitive), set **`true`**. Otherwise **`collectors/utils.RemoteMVPRule`** over **title**, plain **description**, **tag** strings (see **`Tags`**), plus **extra hints**: join the listing meta line fragments documented in **`resources/djinni.md`** (remote-only phrase, **`location-text`**, English/Ukrainian experience and language chips) as separate strings passed to the same helper pattern as DOU/Himalayas location hints. |
| `CountryCode` | Prefer ISO **alpha-2** from **`applicantLocationRequirements`** / nested **`address.addressCountry`** when present; else map **`jobLocation.address`** country strings through **`data/countries.json`**; **`addressRegion`** text like **Europe** alone → **`""`** if no ISO match |
| `SalaryRaw` | Format opaque string from JSON-LD **`baseSalary`** (`MonetaryAmount` → min/max + currency + unit, e.g. month); if absent, optional fallback to listing salary preview text (**`strong.text-success`**) then **`""`** |
| `Tags` | JSON-LD **`category`** as first tag when non-empty; append trimmed text from listing/detail **`div.job-item__tags span.badge`** when available (dedupe, preserve order) |
| `Position` | Keyword inference only (`*string`) over **title + description + tags** |

---

## Built In → `Job`

Wire: **`../resources/builtin.md`** — listing **`GET`** (JSON-LD **`ItemList`**, URLs only) + detail **`GET`** (JSON-LD **`JobPosting`**). **No fetch** when slot search is empty.

| `Job` field | Source |
| ----------- | ------ |
| `Source` | `builtin` |
| `Title` | JSON-LD **`JobPosting.title`** |
| `Company` | JSON-LD **`hiringOrganization.name`** when present; else **`""`** |
| `URL` | Absolute canonical URL from **`JobPosting.url`** when present; else the listing **`ListItem.url`** used for the detail **`GET`**, normalized (strip query/fragment) per shared URL helper |
| `ApplyURL` | Distinct external apply URL from JSON-LD when the payload exposes one (**`sameAs`**, application URL fields, or documented extension); else **`""`** (Easy Apply is often login-gated — revisit only if a stable external URL is documented) |
| `Description` | Plain text from JSON-LD **`description`** (HTML in payload → strip per cross-cutting rules) |
| `PostedAt` | JSON-LD **`datePosted`** per **PostedAt** rules above |
| `Remote` | **`collectors/utils.RemoteMVPRule`** over **title**, plain **description**, and **tag** strings (see **`Tags`**); include **extra hints** from JSON-LD location / employment type text when stringified fragments are available (same helper pattern as DOU/Himalayas) |
| `CountryCode` | **Normative:** ISO **alpha-2** from the **listing request’s `country` query** (alpha-3 → alpha-2 per **`resources/builtin.md`** table) for the run that produced this job’s URL — do **not** override from free-text location in JSON-LD for this source |
| `SalaryRaw` | Opaque string from JSON-LD **`baseSalary`** / **`salaryCurrency`** / textual compensation fields when present; else **`""`** |
| `Tags` | Non-empty strings from JSON-LD **`skills`**, **`qualifications`**, **`occupationalCategory`**, or similar array/string fields when present (implementation normalizes to `[]string`); else **`nil`** / empty |
| `Position` | Keyword inference only (`*string`) over **title + description + tags** |

---

## Related

- `collector.md`
- [`specs/000-epic-overview/product-concept-draft.md`](../../000-epic-overview/product-concept-draft.md)
- `jobs-table-extension.md`
- `../resources/europe-remotely.md`, `../resources/working-nomads.md`, `../resources/dou.md`, `../resources/himalayas.md`, `../resources/djinni.md`, `../resources/builtin.md`
