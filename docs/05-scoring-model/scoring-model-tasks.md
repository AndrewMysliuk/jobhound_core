# Gated scoring model — tasks

> Agent implementation plan. One checkbox, one agent. Launch in dependency order.
> A checkbox is done when its **Done** command passes. Do not edit files owned by a later checkbox.
> Until **Q1**, `go test ./...` may fail to compile. That is expected. Do not fix it inside the current checkbox.
> Spec: [`scoring-model-concept.md`](./scoring-model-concept.md), [`scoring-model-schemas-contracts.md`](./scoring-model-schemas-contracts.md).
> Frontend paths are relative to `jobhound_frontend`. Backend paths are relative to `jobhound_core`.

---

## Scope

**In this slice**

1. Profile YAML and loader use five blocks: `gate_role`, `gate_hard`, `gate_location`, `signals_plus`, `penalties_soft`, plus `buckets.top`.
2. One scoring pass always runs all five steps, records every fired rule, and picks the bucket at the end.
3. Buckets are `TOP`, `REVIEW`, `REJECTED`. `PASSED` is deleted.
4. The jobs list defaults to `TOP` and no longer hides score `0` and below.
5. The browser client uses the new bucket and signal types, and three bucket tabs that default to `TOP`.

**Paused (do not implement here)**

- New HTTP routes, a new JSON Schema file, or changes to `registerRoutes()`.
- Changes to `internal/profiles/contract.go` or `internal/scoring/contract.go`.
- Temporal registration, payloads, or `cmd/worker` wiring. `ProfileRunWorkflow` stays as it is.
- `internal/ingest`, `internal/collectors`, `queries`, `sources`, `wellfound_roles`.
- Per-query attribution, recomputing stored matches, LLM scoring, notifications, numeric timezone offsets.
- Code constants for weights, caps, the `TOP` threshold, or the currency rate. `ScoreWindowDays` and `PostedAtRejectDays` stay.
- Tuning the numbers after a live run. The first YAML values are an educated guess.

---

## 0. Schema / contract

- [ ] **T1** — Profile model, loader, and `config/profiles/andrew.yaml`.
  - `internal/profiles/schema/profile.go`: replace `LocationCountries`, `LocationRegions`, `ExcludeTitle`, `ExcludeText`, `Penalties` with the structs in the schemas doc (`GateLocation`, `SignalsPlus`, `TextSignals`, `SalarySignals`, `FreshnessSignals`, `PenaltyGroup`, `Buckets`).
  - `internal/profiles/impl/store.go`: snake_case YAML tags, `KnownFields(true)` stays. Flatten `penalties_soft` into a slice sorted by `Name`. `MinOccurrences` of `0` or `1` is data, not a loader error.
  - `config/profiles/andrew.yaml`: reclassify existing phrases. Do not invent a new taxonomy and do not drop a phrase unless the concept names it (staff, principal, lead, architect, platform, developer-experience, forward-deployed are not rejects).
    - `gate_role`: title phrases that are not the user's trade.
    - `gate_hard`: US work authorization, clearance, no visa sponsorship, unpaid or deferred compensation.
    - `gate_location`: move `location_countries` / `location_regions`; timezone phrases go to `reject_timezones`. No numeric UTC offsets.
    - `signals_plus`: stack terms with explicit weights. `text.max_total` sits beside `terms`, not inside the map. `queries` stays a separate list.
    - `penalties_soft`: groups `foreign_stack`, `foreign_domain`, `office_pressure`, `agency`, `culture_smell`, `gambling_crypto`, `ambiguous_us`, `part_time`, `security_domain`, `ml_domain`. Each has `weight` (signed delta, penalties negative), `terms`, `min_occurrences`, `check_negation`. Set `check_negation: true` on `ambiguous_us` and `part_time`. Set `min_occurrences: 2` on `foreign_stack`, `1` on the others.
    - `buckets.top`: one explicit integer. `sources`, `queries`, `wellfound_roles` stay.
  - `internal/profiles/impl/store_test.go`: five blocks decode; `penalties_soft` is sorted by name; a leftover `exclude_title` key returns `profiles.ErrProfileInvalid`. Drop the old list-length assertions.
  - `internal/publicapi/handlers/get_profiles_test.go`: update the `Profile` literal only. Do not change handler behavior.
  - **Do not** edit `internal/scoring`.
  - **Done:** `go test ./internal/profiles/...`

- [ ] **T2** — Bucket, signal, and the two deleted point constants.
  - `internal/scoring/schema/bucket.go`: `TOP`, `REVIEW`, `REJECTED`. Delete `BucketPassed`. Keep the Canonical Enum methods.
  - `internal/scoring/schema/signal.go`: codes `GATE_ROLE`, `GATE_HARD`, `CUT_LOCATION`, `CUT_TIMEZONE`, `CUT_POSTED_AT`, `PLUS`, `PENALTY`. Delete `SignalQuery`, `SignalCutExcludeTitle`, `SignalCutExcludeText`. `Signal` is `{Code, Rule, Phrase, Delta, Occurrences}` with the JSON tags from the schemas doc. Delete `Points` and `Terms`.
  - `internal/scoring/schema/constants.go`: delete `QueryPoints` and `PenaltyPoints`. Keep `ScoreWindowDays` and `PostedAtRejectDays`.
  - **Do not** edit call sites. Compile errors outside `internal/scoring/schema` belong to **I1**, **M1**, **S1**, **H1**.
  - **Done:** `go test ./internal/scoring/schema/...`

---

## 1. Config / migration

No new `JOBHOUND_*` variable. Do not add one.

- [ ] **M1** — Bucket check constraint, edited in place. No `000002`.
  - `migrations/000001_initial_schema.up.sql`: `CHECK (bucket IN ('TOP', 'REVIEW', 'REJECTED'))`.
  - Same `CHECK` string in the sqlite setups:
    - `internal/scoring/storage/repository_test.go`
    - `internal/scoring/workflows/profile_run_test.go`
    - `internal/jobs/storage/repository_retention_test.go`
  - Replace inserted `'PASSED'` literals in those tests and in `internal/platform/pgsql/migrations_integration_test.go` with `'REVIEW'`. The integration assertion must require `TOP`, `REVIEW`, and `REJECTED`, and must not accept `PASSED`.
  - Where a Go literal still says `BucketPassed`, `SignalQuery`, `Points`, or `Terms`, update that literal so the file compiles. Do not change list-filter or scoring behavior; **S1** and **I1** own that.
  - **Done:** `go test ./internal/scoring/storage/ ./internal/jobs/storage/ ./internal/platform/pgsql/`

---

## 2. Storage

- [ ] **S1** — List defaults to `TOP` and returns non-positive scores.
  - `internal/scoring/storage/repository.go`: empty bucket means `TOP`. Delete the `score > 0` branch. Order stays `score DESC, jobs.first_seen_at DESC`.
  - `internal/scoring/impl/service.go`: same default. Update the comment.
  - `internal/scoring/storage/repository_test.go` and `internal/scoring/impl/service_test.go`: a `REVIEW` row with score `0` and a negative-score row are both listed. A `TOP` list does not hide them either.
  - **Done:** `go test ./internal/scoring/storage/ ./internal/scoring/impl/ -count=1 -run 'List|Service'`

---

## 3. Impl

`Evaluate(profile, job) Outcome` stays. The clock for a run is the `now` already passed into `evaluateAt` from `Score`. Do not import the Temporal SDK. New helpers stay unexported in `internal/scoring/impl`.

Phrase match stays case-insensitive and word-bounded. Signal deltas, including gate zeros, are the only score inputs: they sum to `Score`.

- [ ] **I1** — Gates, negation, and the bucket decision. Rewrite `internal/scoring/impl/evaluate.go` and replace `internal/scoring/impl/evaluate_test.go`.
  - All five steps run. Nothing returns on the first hit. Bucket is chosen only after the steps: any gate signal means `REJECTED`, otherwise `score >= profile.Buckets.Top` means `TOP`, otherwise `REVIEW`.
  - `gate_role`: title only. One `GATE_ROLE` signal per matched phrase. `Rule` is `gate_role`, `Delta` is `0`, `Occurrences` is the hit count in the title. No negation check.
  - `gate_hard`: title, description, and `location.raw`. Negation-checked. One `GATE_HARD` signal per phrase that still hits. `Rule` is `gate_hard`.
  - Negation window: the 50 characters before the phrase, clamped at the previous `.`, `!`, or `?`. A hit does not count when that window contains `not`, `no`, `n't`, or the phrase is prefixed by `non-`. `not part-time`, `no US authorization required`, `you won't need to relocate`, and `non-hybrid` do not count. `We are not a huge company. Security clearance required.` still rejects.
  - `gate_location`: empty allow lists, or empty job countries and regions, are not a reject. A known country or region that misses both allow lists emits one `CUT_LOCATION` per code. `Rule` is `gate_location.allow_countries` or `gate_location.allow_regions`. `Phrase` is that code.
  - Timezone phrases are matched against title, description, `location.raw`, and `location.timezone`, with the same negation check. One `CUT_TIMEZONE` per phrase. `Rule` is `gate_location.reject_timezones`.
  - A known `posted_at` older than `PostedAtRejectDays` emits `CUT_POSTED_AT`. `Rule` is `posted_at`, `Phrase` is RFC3339. A zero `posted_at` is not a reject.
  - Leave `signals_plus` and `penalties_soft` as no-ops that add nothing. **I2** and **I3** fill them.
  - Tests in this checkbox: the negation sentences above, empty location is not `REJECTED`, stale `posted_at` is `REJECTED` with `CUT_POSTED_AT`.
  - **Done:** `go test ./internal/scoring/impl/ -count=1 -run TestEvaluate`

- [ ] **I2** — Plus signals in `internal/scoring/impl/evaluate.go`. Add cases to `evaluate_test.go`; do not rewrite **I1** cases.
  - Title terms, then body terms, then salary, then freshness. Walk map keys in sorted order.
  - Body points stop at `signals_plus.text.max_total`. Emit one `PLUS` per matched term with the points actually awarded after the cap, including `0` when the cap is already spent. `Rule` is `signals_plus.title` or `signals_plus.text`. `Phrase` is the term. `Occurrences` is the hit count.
  - Salary: parse a range from the title, description, and raw salary text. Compare the upper bound with `MinEURYear` or `MinUSDYear` after `USDPerEUR`. Add `BonusIfAbove` or `PenaltyIfBelow` as stored (do not flip the sign again). No salary, or an unparseable string, adds nothing and emits no signal. One `PLUS`, `Rule` `signals_plus.salary`, empty `Phrase`.
  - Freshness: prefer `posted_at`, otherwise `first_seen_at`, against `evaluateAt`'s `now`. Within 3 days adds `Days3`, else within 7 days adds `Days7`. Neither timestamp usable adds nothing and emits no signal. `Rule` is `signals_plus.freshness`, empty `Phrase`.
  - Tests: body-cap deltas are the awarded points; a missing salary does not apply `PenaltyIfBelow`.
  - **Done:** `go test ./internal/scoring/impl/ -count=1 -run TestEvaluate`

- [ ] **I3** — Soft penalties in `internal/scoring/impl/evaluate.go`. Add cases; do not rewrite **I1** or **I2** cases.
  - Visit groups in slice order. One `PENALTY` per group, not per term. `Rule` is the group name. `Delta` is `Weight`. `Phrase` is the term that fired the group. `Occurrences` is the count used for the threshold.
  - A title hit fires the group once even when the body count is below `MinOccurrences`. Body-only hits fire when the count is at least `MinOccurrences` (`0` and `1` both mean one hit). Do not branch on the group name.
  - Run the negation check only when `CheckNegation` is true. Do not run it for plus signals.
  - Tests: one `ruby` in the body scores nothing when `min_occurrences` is 2; two apply the weight once; `ruby` in the title applies it once regardless of count.
  - **Done:** `go test ./internal/scoring/impl/ -count=1 -run TestEvaluate`

- [ ] **I4** — One more `TestEvaluate` case. A vacancy that trips `gate_role` and still scores high is `REJECTED`, the score is the sum of the signal deltas, and the list contains both the gate signal and the plus signal.
  - **Done:** `go test ./internal/scoring/impl/ -count=1 -run TestEvaluate`

---

## 4. Temporal

No workflow or activity changes. **Q1** only updates the existing test fixture.

---

## 5. HTTP

- [ ] **H1** — `internal/publicapi/schema/profiles.go`: `ProfileJobSignal` is `{Code, Rule, Phrase, Delta, Occurrences}` with `json:"code"`, `rule`, `phrase,omitempty`, `delta`, `occurrences,omitempty`. Delete `Points` and `Terms`.
  - **Done:** package `internal/publicapi/schema` builds. Call sites are **H2**.

- [ ] **H2** — `internal/publicapi/handlers/get_profile_jobs.go`: default bucket is `TOP`. Project `Rule`, `Phrase`, `Delta`, `Occurrences`. Do not add a route or a schema file.
  - **Done:** `go test ./internal/publicapi/handlers/ -count=1 -run 'ProfileJob'` compiles. Assertion updates are **H3** if this command still fails on them.

- [ ] **H3** — `get_profile_jobs_test.go` and `patch_profile_job_test.go`.
  - Default bucket sent to the service is `TOP`.
  - `bucket=REVIEW` is accepted. `bucket=PASSED` is `400`.
  - The JSON signal carries `code`, `rule`, `phrase`, `delta`, `occurrences` and has no `points` or `terms`.
  - **Done:** `go test ./internal/publicapi/handlers/ -count=1 -run 'ProfileJob'`

---

## 6. Wiring

No `cmd/api` or `cmd/worker` change.

---

## 7. Frontend

Paths below are in `jobhound_frontend`.

- [ ] **F1** — `app/types/IProfiles.ts`
  - `IProfileJobBucket` is `"TOP" | "REVIEW" | "REJECTED"`.
  - `IProfileSignalCode` is `"GATE_ROLE" | "GATE_HARD" | "CUT_LOCATION" | "CUT_TIMEZONE" | "CUT_POSTED_AT" | "PLUS" | "PENALTY"`.
  - `IProfileSignal` is `{ code, rule, phrase?, delta, occurrences? }`. Delete `points` and `terms`.
  - **Done:** no remaining `PASSED`, `points`, or `terms` on these types.

- [ ] **F2** — `app/pages/profiles/[id].vue`
  - Bucket tabs are Top, Review, and Rejected. The default tab sends `bucket=TOP` explicitly.
  - The Hidden control stays a `user_status=HIDDEN` view. It is not a bucket.
  - When `location.countries` and `location.regions` are both empty, the location cell renders a dash. No badge and no extra signal for that.
  - **Done:** the page type-checks against **F1**. No other route changes.

---

## 8. Tests (extra coverage)

- [ ] **Q1** — `internal/scoring/workflows/profile_run_test.go` fixture only. Do not change workflow code.
  - The inline profile uses the new fields: `GateRole: ["recruiter"]`, a `signals_plus.title` entry `vue: 1`, `Buckets.Top: 1`.
  - The Vue job is `TOP` with score `1` and a `PLUS` signal whose phrase is `vue`. The recruiter job is `REJECTED` with `GATE_ROLE` and phrase `recruiter`. Hidden status still survives the second run. Jobs scored and sources skipped stay the same.
  - **Done:** `go test ./internal/scoring/workflows/ -count=1 -run TestProfileRunWorkflow`

---

## Dependencies (brief)

```
T1 → T2 → I1 → I2 → I3 → I4
T2 → M1 → S1
T2 → H1 → H2 → H3 → F1 → F2
I4 + S1 + M1 → Q1
```

**M1:** T1, T2, I1, I2, I3, M1, S1
**M2:** I4, H1, H2, H3
**M3:** Q1, F1, F2
