# Gated scoring model — concept

> **Sequence 5.** After [`docs/02-architecture-profile/`](../02-architecture-profile/architecture-profile-concept.md).
> Contracts: [`scoring-model-schemas-contracts.md`](./scoring-model-schemas-contracts.md). Tasks: [`scoring-model-tasks.md`](./scoring-model-tasks.md).
> This slice adds no new HTTP surface; it reshapes an existing one.
> Pairs with a small frontend change (bucket tabs and the signal type), described under *Adjacent modules*. Those tasks are in the same tasks file.

## Why

**Problem today:** the profile carries three flat phrase lists — `exclude_title`, `exclude_text`, `penalties` — about 190 blacklist entries in total, and the scoring pass returns on the first hit. That produces three separate failures.

The pass is blind. A vacancy killed by one phrase in `exclude_title` never gets a score, so there is no way to tell a correct rejection from a mistake. Everything that was ever mildly annoying ended up in the blacklist, because a penalty that cannot be inspected is worthless, and rejection was the only outcome with visible effect.

The phrase match has no sense of negation. `exclude_text` contains `part time`, so "this is not a part-time role" rejects the vacancy. The same holds for `must be based in`, `relocate to` and `in office`, which is why they describe exactly the jobs the user wants.

And the bucket carries no information. Everything that survives is `PASSED`, including a score of zero, so the list is sorted noise rather than a short daily shortlist.

**New solution:**

1. The three flat lists become five blocks with distinct jobs: three **gates** that reject, one block of **plus signals**, one block of **soft penalties** that cost points but keep the vacancy alive. Roughly 45 of the old 190 phrases stay hard rejects; the rest turn into points.
2. **The pass no longer short-circuits.** Every fired rule is recorded, the score is computed even for a vacancy that tripped a gate, and the bucket is decided at the very end. A rejected vacancy with a high score is the signal that a gate is wrong.
3. Phrase matching gains a **negation check**, so the long location and employment phrases can come back as rules without inverting their own meaning.
4. The bucket becomes three-valued: **`TOP`** is the shortlist to apply to today, **`REVIEW`** is everything plausible but unconvincing, **`REJECTED`** is the audit list.
5. All weights and thresholds live in the profile YAML, so the model is tuned by editing one file and re-running, with no code change.

Product context: a single user triaging a daily vacancy feed by hand, whose main cost is not missing a good job rather than reading a bad one. Primary entity: **Match**.

## Stack

| Layer | Library / tool | Module |
| ----- | -------------- | ------ |
| Profile definition | **YAML on disk** (`config/profiles/*.yaml`) | `internal/profiles` |
| Scoring | **phrase rules, no LLM** | `internal/scoring/impl` |
| Persistence | **PostgreSQL + GORM + migrations** | `internal/scoring/storage` |
| Orchestration | **Temporal** (existing `ProfileRunWorkflow`) | `internal/scoring/workflows` |
| HTTP | **`net/http`** | `internal/publicapi` |

No new source, no new collector, no network call inside scoring. The ingest side is untouched: the profile's `queries` and `sources` keep feeding collectors exactly as they do now, and the plus signals are a separate block rather than a reuse of `queries`.

## Match

**Input:** one stored vacancy and one profile. The scoring pass reads only the vacancy's title, description, location, raw salary text, `posted_at` and `first_seen_at`. It never reads the network and never re-fetches the listing.

**Profile blocks that drive it:**

- **`gate_role`** — job titles that are not the user's trade. Title only, hard reject.
- **`gate_hard`** — text that makes the job legally or financially impossible: US work authorization, clearance, no visa sponsorship, unpaid or deferred compensation. Negation-checked, hard reject.
- **`gate_location`** — allowed countries and regions, plus a list of rejected timezone phrases. Hard reject.
- **`signals_plus`** — points for the user's stack in the title and in the body, plus salary and freshness.
- **`penalties_soft`** — named groups, each with one weight and a term list: foreign stack, foreign domain, office pressure, agency, culture smell, gambling and crypto, ambiguous US-only wording, part-time, security domain, ML domain.

**Lifecycle:** a match row is written by the scoring step of a profile run and overwritten by the next run. Nothing about it is edited by hand except the user status, which already exists and is out of this slice. Re-tuning the model means editing the YAML and starting a run; no backfill and no recompute path is needed.

Buckets: **`TOP`** | **`REVIEW`** | **`REJECTED`**. The old **`PASSED`** value is removed outright rather than kept for compatibility — the database is recreated from scratch, so there is no legacy row to read.

## Runtime

**Start:** unchanged. The scoring step of `ProfileRunWorkflow` loads the profile, takes the vacancies in the run's window, and evaluates each one.

**Score window:** unchanged — `first_seen_at` inside the run window, plus vacancies whose `posted_at` is already stale so they get re-bucketed.

**UI-only fields:** an unknown hiring location is **not** a signal and **not** a bucket. It simply renders as a dash in the client. The engine treats an empty country and region list as "no information", which is not a reason to reject and not a reason to score differently.

**Outcomes:**

- **`TOP`** — no gate fired and the score reached the profile's `buckets.top`;
- **`REVIEW`** — no gate fired and the score is anything below that, including zero and negative;
- **`REJECTED`** — at least one gate fired, whatever the score.

**Adjacent modules:** `internal/publicapi` keeps its existing list endpoint — no audit endpoint is added. Because the score is now computed for rejected vacancies too, and the list is already ordered by score descending, `bucket=REJECTED` returns the highest-scoring killed vacancies first, which is exactly the false-rejection audit. Two things there do change: the default bucket stops being `PASSED`, and the "hide score of zero and below" filter is dropped, since a score of zero is now a legitimate `REVIEW` row. The browser client moves from two tabs to three (`TOP`, `REVIEW`, `REJECTED`) and its signal type is updated in the same pass.

**Progress:** per-vacancy reasoning lives in the match's signal list, not in logs. Every fired rule contributes one signal carrying its code, the rule or group name, the matched phrase, the point delta and the occurrence count. A bucket that cannot be explained from its own signal list is a bug.

## One scoring pass (core loop)

One vacancy against one profile. Pure function of its inputs: no storage, no network, no clock beyond the run's start time. All five steps always run.

1. **`gate_role`** — the title against the role list, case-insensitive, on word boundaries.
2. **`gate_hard`** — title, description and the raw location line against the hard list, with the negation check.
3. **`gate_location`** — known countries and regions against the profile's allow lists; separately, the text against the rejected timezone phrases, negation-checked. A `posted_at` older than the staleness threshold is a gate at this level too.
4. **`signals_plus`** — title terms, body terms up to the body cap, salary, freshness.
5. **`penalties_soft`** — one subtraction per group, groups visited in a stable order so that two runs over the same vacancy produce the same signal list.

Then, and only then, the bucket: any gate hit means `REJECTED`, otherwise the score decides between `TOP` and `REVIEW`.

### Failure at any step

Scoring cannot fail on a per-vacancy basis — there is nothing to fail, every step degrades to "no match". A malformed profile is a load failure before the loop starts and fails the run; the existing Temporal retry applies. An unparseable salary string is not a failure and not a penalty: it contributes zero.

## Matching rules

| Situation | Behavior |
| --------- | -------- |
| A gate phrase is negated within the same sentence | The hit does not count. The window is the 50 characters before the phrase, clamped at the previous sentence boundary so that a negation in an unrelated sentence cannot disarm a gate. |
| A foreign-stack term appears once in the body | No penalty. One mention in a "nice to have" list is noise. |
| A foreign-stack term appears twice or more, or appears in the title | The group penalty applies, once. |
| Several terms from the same penalty group match | The group penalty applies once, not once per term. |
| A gate fires and the vacancy also scores well | `REJECTED`, but the score is computed and stored. This is the audit case. |
| Countries and regions are empty or unknown | Not a rejection. The bucket is decided by score alone. |
| A salary range is found | The upper bound is compared against the profile minimum; above it earns a bonus, below it takes a penalty. |
| No salary anywhere in the listing | Zero, not a penalty. |
| Neither `posted_at` nor `first_seen_at` is usable for freshness | Zero freshness points, no penalty. |
| A known `posted_at` is older than the staleness threshold | `REJECTED`. A four-year-old listing with a high score is still worthless. |

**Hard rules:** the negation check applies to `gate_hard`, the rejected timezone phrases and the ambiguous-US penalty group, and nowhere else — a negated plus signal is not worth the false negatives. No weight, cap, threshold or currency rate is a code constant; all of them come from the profile. Timezone handling is phrase-based only: the vacancy's timezone field is free text, so a numeric UTC offset range would be configuration nothing can evaluate. No Temporal SDK in `impl/`.

## Modules

| Module | Purpose |
| ------ | ------- |
| **`internal/profiles`** | The profile data model and the YAML loader. Gains the five blocks, loses the three flat lists. Unknown YAML keys stay a load error. |
| **`internal/scoring/impl`** | The pass itself, the negation check, phrase and occurrence counting, salary parsing, bucket selection. |
| **`internal/scoring/schema`** | Bucket and signal-code enums, the signal shape, the staleness threshold's home. |
| **`internal/scoring/storage`** | The match row and the list query. Loses the "score of zero and below is hidden" rule. |
| **`internal/publicapi`** | The list endpoint's default bucket and its signal projection. |
| **`migrations`** | The bucket check constraint, edited in place in `000001` rather than altered in a follow-up file. |

## Layers

- **Saved** — the profile YAML. Read on every run, never written by the engine.
- **Run** — the match rows of one profile run: bucket, score, signals.
- **Wire** — the list endpoint's job and signal projection, and the browser client's mirrored types.

## Implementation order

1. The profile YAML rewritten to the five blocks, with `sources` kept intact and the body cap split out from the body term weights.
2. The profile data model and loader against that file.
3. The bucket and signal enums, and the check constraint in `000001`.
4. The scoring pass, in the order of the five steps above.
5. The list query and the endpoint default.
6. The browser client's tabs and signal type.
7. Tests at the boundaries: the negation check against real phrasings, occurrence-threshold penalties, an empty location landing in a score-decided bucket, and a gated vacancy that still carries a computed score.

Early check: one gated, high-scoring vacancy through `Evaluate` — if its signal list explains both the gate and the score, the central change of this slice works.

## Out of scope (concept level)

- **Per-query attribution on vacancies.** Recording which search word produced a vacancy is not feasible at a sensible price: the `consider_*` and `getro_*` sources, which are most of the source list, receive all queries as a single union request and return a merged result that does not say which word matched. Attribution would mean one request per query per board. The `REJECTED` tab shows the same information by eye.
- **Tuning the numbers.** The threshold and the weights in the first version of the YAML are an educated guess. They are adjusted after the first run by reading the three tabs, not by changing code.
- **Recomputing existing matches.** The next run overwrites them.
- **Everything the old blacklist implied about seniority.** Staff, principal, lead and architect titles are deliberately no longer rejected, and neither are platform, developer-experience or forward-deployed roles.
- **LLM scoring, scheduled auto-refresh, notifications.** Unchanged and unrelated.
