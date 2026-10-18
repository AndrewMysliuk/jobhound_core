# Vacancy facts — tasks

> Agent implementation plan for [enrich-concept.md](enrich-concept.md).
> Frontend tasks are in this file because `jobhound_frontend` has no feature folder.

---

## Scope

**In this slice**

1. Fold the Latin diacritics the matcher still misses.
2. Store a verdict per `(job_id, prompt_version, model, input_hash)` and fill it from an Anthropic batch.
3. Score reads a ready verdict. Every field stays `scored: false`, so fact deltas are 0 and fact gates do not reject.
4. `make eval` prints per-field metrics offline.

**Paused (do not implement here)**

- Setting any `facts.*.scored` to true.
- Deleting or editing phrase lists because a field replaced them.
- `primary_stack` replacing `signals_plus` or `foreign_stack`.
- Passing collector location into the prompt or the hash.
- The one-off Haiku audit of `GATE_ROLE` rejects.
- Labeling the 100-vacancy gold set. The fixture is a handful of rows only.
- A new product HTTP route.

---

## 0. Schema / contract

- [ ] **T1** — `internal/enrich/schema`: `PromptVersion`, field enums (Canonical Enum), `QuotedValue`, `ModelReply`, `Verdict`, `EnrichBatchInput`
- [ ] **T2** — `internal/enrich/contract.go`: `API`, `ErrNotFound`. No helpers at the module root
- [ ] **T3** — `internal/enrich/schema/json_schema/model_reply.schema.json` for the model reply
- [ ] **T4** — `internal/profiles/schema`: `facts` block (`scored`, `allow`, `deltas`, `gates`)
- [ ] **T5** — `internal/scoring/schema.Signal`: optional `Value`. New codes `LLM` and `CONFLICT`
- [ ] **T6** — `internal/publicapi/schema.ProfileJob`: `facts_status`. `ProfileJobSignal`: optional `value`

---

## 1. Config / migration

- [ ] **G1** — `JOBHOUND_ANTHROPIC_API_KEY`, `JOBHOUND_ENRICH_MODEL` in `internal/config`. Empty key means enrich off
- [ ] **M1** — `migrations/000002_job_verdicts.{up,down}.sql` with unique `(job_id, prompt_version, model, input_hash)`

---

## 2. Storage

- [ ] **S1** — `internal/enrich/storage`: insert verdict, get by the unique key, list job ids in the given set that have no row
- [ ] **S2** — storage tests for the unique key and the missing-id query

---

## 3. Impl

- [ ] **I1** — normalize + hash (NFC, whitespace collapse). Location is not in the hash
- [ ] **I2** — validate and coerce `ModelReply`: unknown enum, quote substring, absence quotes, pool vs single-value, drop `none` / `english_only` when a real value is present
- [ ] **I3** — prompt text: field descriptions, few-shots, data delimiters, years title-word rule (junior / intern / entry → `0-1`, senior / staff / principal → `5+`)
- [ ] **I4** — batch submit and poll. Retryable errors return to Temporal. A body that fails schema after retries is stored as all fields `unknown`
- [ ] **I5** — phrase→field map. Seniority phrases map only to `years_experience_required`
- [ ] **I6** — `internal/scoring/impl.Evaluate` accepts a verdict or pending. Unscored field → `LLM` at delta 0. `gates` ignored. `CONFLICT` only for a mapped phrase whose allow-pool still hits. Pending → no fact signals
- [ ] **I7** — impl tests: coercion, eval metrics, evaluate cases above, diacritic fold through the scorer (`äöüß`)

---

## 4. Temporal

- [ ] **W1** — `EnrichBatchWorkflow` registered in `New`. Id `enrich-{promptVersion}-{model}-{hash of sorted job ids}`
- [ ] **W2** — activities: submit, poll, store, rescore. Mappers in `workflows/mappers.go`. Rescore does not write `user_status`
- [ ] **W3** — `ProfileRunWorkflow` starts the child with parent-close abandon and does not wait. Register from `cmd/worker`
- [ ] **W4** — backfill uses the same workflow. The job set is TOP, REVIEW, and matches whose signals include `GATE_ROLE`

---

## 5. HTTP

- [ ] **H1** — job JSON includes `facts_status` (`off` | `pending` | `ready`) and signal `value`
- [ ] **H2** — extend `get_profile_jobs_test.go` for both fields

---

## 6. Wiring

- [ ] **C1** — `cmd/worker` builds enrich from config. No business rules in `cmd`
- [ ] **C2** — `config/profiles/andrew.yaml` and `lada.yaml`: `facts` block, every field `scored: false`
- [ ] **C3** — `make eval` compares `testdata/enrich/gold.jsonl` and `predictions.jsonl`, prints the three per-field metrics, no network, exit 0

---

## 7. Frontend (`jobhound_frontend`)

- [ ] **F1** — `IProfileSignal` accepts `LLM` and `CONFLICT` and optional `value`. `IProfileJob` accepts `facts_status`
- [ ] **F2** — profile job table shows `facts_status` and, for fact signals, `value`, `phrase`, and `delta`

---

## Dependencies

```
T* → G* / M* → S* → I* → W* → H* → C*
F* after H1
I7 after I2 and I6
```

**M1:** T*, G1, M1, S*, I1–I3  
**M2:** I4–I7, W*, H*, C1–C2  
**M3:** C3, F*
