# Contract: Redis ingest coordination (lock & cooldown)

**Feature**: `006-cache-and-ingest`  
**Purpose**: Freeze **key shape**, **TTL defaults**, and **degraded** behaviour so implementation and tests stay aligned.

**Related**: `specs/006-cache-and-ingest/spec.md`; `specs/006-cache-and-ingest/contracts/environment.md`; `007` stage-1 column name in `specs/007-llm-policy-and-caps/contracts/pipeline-run-job-status.md`.

---

## 1. Key naming

| Purpose   | Pattern                    | Notes |
|-----------|----------------------------|--------|
| In-flight lock | `ingest:lock:{source_id}` | `source_id` is **normalized** (see §4). |
| Cooldown  | `ingest:cooldown:{source_id}` | Set **after** a successful ingest for that source (see `spec.md`). |

No other Redis keys are required for ingest in v1. **No** Redis-backed search-result cache in v1.

---

## 2. TTL defaults (code constants)

Values are **defaults** in code (e.g. `internal/config` or ingest package); **not** environment variables in v1 unless a later epic adds optional overrides under `JOBHOUND_*` via `internal/config` only.

| Constant (illustrative name) | Seconds | Role |
|------------------------------|---------|------|
| `IngestLockTTLSeconds`       | **600** | Prevents overlapping ingest for the same `source_id`; lock auto-expires if a worker dies mid-run. |
| `IngestCooldownTTLSeconds`   | **3600** | Minimum interval between **successful** ingests for the same `source_id`, unless **explicit refresh** bypasses cooldown (see `spec.md`). |

**Lock acquisition**: use **SET** with **NX** (or equivalent) and the lock TTL above. No heartbeat requirement in v1.

**Cooldown set**: after ingest completes **successfully** (Postgres commit of upserts for that run is acceptable as the boundary — exact hook is implementation detail documented next to the activity).

---

## 3. Explicit refresh vs cooldown

When **explicit refresh** is enabled (see `environment.md`), a new ingest may start **even if** `ingest:cooldown:{source_id}` is still present — implementation **deletes** or **overrides** that key as part of the refresh path, or skips cooldown check. **Lock** (`ingest:lock:{source_id}`) is **still** taken before work; refresh does **not** bypass the lock.

---

## 4. `source_id` normalization for keys

To avoid duplicate locks for the same logical source, normalize before interpolating into keys:

- Trim whitespace; **lowercase** ASCII.
- If the domain `source` string may contain characters unsafe for Redis key readability, use a **stable slug** already used elsewhere for that collector (e.g. `linkedin`, `greenhouse`) — must match whatever identifies the collector instance in code.

---

## 5. Redis unavailable

**Fail closed**: if Redis cannot be reached for **lock** (and cooldown check where applicable), **do not** start a new ingest for that source — return/log an error. This avoids hammering external sites when coordination is down.

No in-process single-flight fallback in v1 unless a later spec adds it.

---

## 6. Compose / ops

Local Docker Compose must include **Redis** when implementing this feature; URL documented via `JOBHOUND_REDIS_URL` in `environment.md`.
