# Product concept (global draft)

**Status**: Draft  
**Created**: 2026-04-02  
**Last Updated**: 2026-04-04  

**Purpose**: Single end-to-end description of how JobHound is meant to behave for users and how the backend fits together. Numbered epics (`001`–`010`) remain the place for technical contracts; this document is the **product/source-of-truth narrative** until it is promoted or split.

**Docs vs epics**: This file is the **global product truth** (behavior, reset rules, MVP boundaries). Folders **`001`–`010`** are **slices** for concrete technical contracts (schema, env, Redis keys, workflows). If an epic contradicts this draft on **user-visible or data-lifecycle behavior**, align the epic to **this** document—or change this draft first if the product decision changed. **HTTP request/response details** are normative in **`009`** where specified.

**MVP scope**: One real user (“single-tenant” operationally), but **data model and APIs reserve** `user_id`, registration, and multi-user isolation later (PhantomBuster-style mental model: limited **search slots** per user).

---

## 1. Core ideas

- The user does not need the entire open web of listings—only **a few focused hunts** (**3** slots in the current MVP constant; may increase later, e.g. up to 5 per user). Each slot is one **broad** theme (stage 1).
- A **profile** (free-text CV-style: skills, experience, stack, timezone, domain, etc.) feeds **stage 3 (LLM)**. Richer profile → better validation.
- **Three stages**:
  1. **Broad (global / external)**: hit configured job **sources** (HTTP collectors), normalize, persist. Uses a **single string** of broad keywords (e.g. frontend / backend / fullstack) for MVP.
  2. **Narrow (local)**: filter **only** rows already stored for that slot—include/exclude keyword lists; optional date rules (TBD). Splits into **passed / not passed** for stage 2.
  3. **LLM**: run on rows that **passed stage 2**; splits into **passed / not passed** for stage 3. Outcome quality depends on profile and prompts.
- The user may **manually mark** a vacancy as “fits me” / “does not fit” to correct mistakes. **Do not introduce a large matrix of statuses** for MVP: keep the same coarse passed/failed buckets the pipeline already uses, plus a **small** manual correction mechanism. **Whenever stage-2 or stage-3 filter configuration is reset** (see §5), **all** dependent outcomes—including manual marks for that scope—are **cleared** (same reset semantics as automated results).

---

## 2. Search slot

- A **search slot** is the unit of “one hunt”: one **stage-1 broad keyword string** (the only string the client sends—**`009`**), **all backend-configured sources** (no per-slot source picker in MVP API), persisted vacancy rows for that slot, stage-2/3 parameters, and computed results.
- **Slot limit** per user: **3** in the current MVP (**`009`**); the narrative range **3–5** remains the long-term product constant when multi-user enforcement lands.
- **Stage-1 keyword string** is **immutable after the first successful stage-1 ingest** for that slot. To search with different broad keywords, the user **creates another slot**.
- **Deleting a slot** **hard-deletes** all data tied to it (vacancies in slot context, run rows, marks, filters—**no orphan rows**).
- **Slot-scoped data**: persisted vacancies and pipeline outcomes for a hunt are keyed by **`slot_id`** (and future **`user_id`**). Several slots may use the same broad keyword string; each slot still has its **own** stage-1 pool and downstream results—no requirement to dedupe across slots in MVP.

---

## 3. Stage 1: ingest and “refresh”

- **First run** (slot created with broad keywords—**`009`** **`POST /slots`** starts ingest): collectors run **in parallel** (per source), vacancies are **upserted** and associated with the slot; user sees the **stage-1 pool** (subject to limits—sites may not expose reliable dates, so “medium” fetch limits and **`006`** watermarks apply).
- **Repeat ingest on the same slot** is **not** in the **public HTTP MVP** (**`009`**): stage 1 runs **once** at slot creation; a different broad string requires a **new slot**. **Incremental “pull new”** on an existing slot remains a **backlog** item (would reuse **`006`** delta semantics when an API or schedule adds it).
- Changing **stage-2 or stage-3 filters** does **not** imply re-hitting external sites by itself (see §5).
- **Ingest coordination (`006`)**: Redis **lock + cooldown** applies on **every** ingest for a source—including the **first** successful run for a new slot. Same code path end-to-end. The lock is keyed by **`source_id`** (normalized collector identity), **not** by slot: parallel ingests for different slots that share a source **serialize** at the source, by design.

---

## 4. Stages 2 and 3 (local computation)

- **Stage 2** reads the **stage-1 pool** for the slot only. **Include** / **exclude** lists (and optional date—TBD). **Passed** and **not passed** lists are **derived**; no second full crawl for this step.
- **Stage 3** reads rows that **passed stage 2**. Output is again **passed / not passed** (plus rationale where the LLM contract defines it—see `004` / `007`).
- **Stage 3 (LLM) policy** (epics **`004` / `007`** spell out types and storage; **`009`** passes **`max_jobs`** per run; behavior must match below):
  - **Cap**: each run scores up to **`max_jobs`** from the HTTP API (**1–100**, further limited by **`007`**—effective batch = **minimum** of engine policy and request). Jobs beyond the batch stay **eligible** until a later **`POST …/stages/3/run`**—avoid silent drops without a defined queue.
  - **Deterministic ordering**: eligible jobs are ordered by **`posted_at`** descending, then **`job_id`** ascending (**`009`** lists use the same order); same inputs → same selection—so retries and UI are not flaky.
  - **Eligible pool**: jobs that **passed stage 2** and do not have a **current** stage-3 result for this slot (including after a §5 stage-3 reset or profile-driven invalidation).
  - **Idempotency**: LLM work units must be safe under **Temporal retries** (no double-consuming cap, no duplicate inconsistent rows for the same `(slot_id, job_id)` outcome).
- **UI** drives stages as separate actions in MVP (**`009`**): slot create runs stage 1; then **`POST` stage 2** (include/exclude), then **`POST` stage 3** with **`max_jobs`**. After **profile** text is saved, the client should **call** stage-3-only recompute (see §5)—otherwise the UI will show stale LLM outcomes.

---

## 5. Reset rules when filters change (no re-fetch for filter edits)

**Important**: “Restart from stage N” means **recompute from data already in PostgreSQL**, not re-running collectors. **MVP HTTP** does not offer stage-1 refresh on an existing slot (§3).


| User action                                                                           | What is wiped                                                                                                                    | Recompute from                                                    |
| ------------------------------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------- | ----------------------------------------------------------------- |
| **Stage-2 filters** change                                                            | All **intermediate and final** results that depend on stage 2 and 3 for this slot (including manual marks tied to those results) | **Stage-1 pool** for the slot → run **stage 2** (**`009`**: client may run **stage 3** in a **separate** call when ready) |
| **Only stage-3** inputs change (**profile** text and/or stage-3 **LLM filter** parameters—anything that does not alter stage-2 matching) | Stage-3 outputs and anything that only exists after stage 3 (including manual marks on those)                                    | Rows that **passed stage 2** → **stage 3 only**                   |


**No explicit versioning** of filter snapshots for MVP: the system stores **current** stage-2 and stage-3 parameters and **current** derived lists; history of “what the filters were last Tuesday” is **out of scope** until a later epic.

---

## 6. Users and auth (MVP vs later)

- **MVP**: effectively **one user**; APIs may omit auth but **schema includes `user_id`** (nullable or fixed) so registration and isolation can land without rewriting slot ownership.
- **Later**: registration; each user owns slots and profile; enforcement in API and workflows.

---

## 7. UI and operators

- **Backend** is **API-first** (`009`). Any UI is a separate deliverable (separate repo or folder).
- **Grafana** (or similar) may be used for **ops / metrics / read-only dashboards** (health, counts, ad-hoc SQL)—**not** required to be the primary product UI.
- For a **Tailwind-friendly** product UI, prefer a thin **web app** (e.g. Vite + React + Tailwind / shadcn, or **htmx** + server-rendered HTML + Tailwind) calling the same API—faster to customize than Grafana for CRUD and workflows.

---

## 8. Out of scope after the core (explicit backlog ideas)

Implement **after** the core works end-to-end for one user and the **source pool** has grown:

- **Scheduled auto-refresh** (hourly / few hours): same slot, same filters, delta ingest + re-run stages as defined—**no numbered epic yet**; add when we pick Temporal Schedule vs external cron and persistence for tick history.
- **Applications / outcomes table** (where the user applied, interview, reject)—**idea only** for now; product value for an aggregator, **not** part of MVP.

**Not planned as MVP** (no numbered epic): third-party push notifications (e.g. Telegram)—revisit after the core vertical if needed; stage-3 **caps** in `007` stay independent of any future delivery channel.

---

## 9. Implementation phasing (product order)

1. **Core**: slots (cap **3**), profile, stage-1 ingest at slot create, stage-2/3 recompute rules (**`009`**), persistence, minimal API + minimal UI to drive it. Same-slot delta ingest is **backlog** (§3).
2. **More sources**: extend collectors (`005`).
3. **Extensions**: scheduled auto-refresh (§8), applications tracking, richer observability (`010`).

---

## 10. Alignment with numbered epics

- **004**: pure stage logic (unchanged semantics; slot is orchestration + storage).
- **006**: ingest, watermarks, Redis coordination—slot-scoped ingest must not collide with **global** broad-key assumptions; **scope broad reuse by `(user_id, slot_id)`** when reconciling with `006`/`007`.
- **007**: caps and `pipeline_run` / per-job statuses—map **pipeline run** to **slot** (and user) when implementing; **caps, ordering, eligible pool, and idempotency** must match §4 stage-3 policy.
- **008**–**009**: manual workflow and HTTP API—**slot id**, §5 reset rules, and HTTP shape (**`009`** normative for public routes).
- This draft **supersedes** informal contradictions for **product** behavior until epics are updated to match.

---

## Related

- [spec.md](./spec.md) — epic index and stack summary.
- `.specify/memory/constitution.md` — engineering principles.

