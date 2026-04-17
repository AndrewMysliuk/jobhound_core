# Contract: Tier-3 browser document fetch (shared)

**Spec**: `005-job-collectors`  
**Last Updated**: 2026-04-17  
**Status**: Normative (implemented)

## Purpose

Provide a **single, source-agnostic** way to load a **final HTML document** for an absolute **HTTPS URL** using a **real browser engine** (headless Chromium via **go-rod**), when **`net/http`** is blocked (e.g. Cloudflare interstitial) or the origin requires a browser-like client.

This module is **not** a collector and **must not** contain:

- Site-specific selectors, JSON-LD parsing, or `domain.Job` mapping.
- LinkedIn-only or Built In-only navigation scripts (those stay in **`internal/collectors/<source>/`**).

**First consumer:** Built In (`builtin.com`) — see **`resources/builtin.md`**, **`tasks.md`** § **M**.  
**Planned reuse:** LinkedIn Jobs (inventory row 7) — same abstraction; session/cookies file and login flows are **LinkedIn-specific** adapters layered **on top of** or **beside** this contract, not inside the generic fetcher unless expressed as optional hooks documented here.

## Behavioral contract (normative intent)

Implementations expose **`browserfetch.HTMLDocumentFetcher`**:

- **Input:** `context.Context`, absolute URL string (`https://…` only; **http** and non-absolute URLs are rejected).
- **Output:** raw **HTML** bytes (encoding as returned by the page; UTF-8 expected for target sites).
- **Semantics:** Navigate to the URL, wait for the **`load`** lifecycle event (CDP), short **bounded settle** (~750ms, respects context) instead of Rod **`WaitStable`** (which can exceed the nav timeout on noisy SPAs), then read **`<html>`** outer HTML. Return an error if navigation fails, context cancelled, or timeout.

**No HTTP retries** at this layer (aligns with **`spec.md`** collector stance); one attempt per call unless a future spec explicitly adds policy.

**Built In follow-up (challenge HTML):** when a response is **`HTTP 200`** but the HTML matches the **builtin-local** Cloudflare interstitial heuristic, **`builtin`** sleeps **`5 s`** (**`context`‑aware**), then performs **one** same-URL refetch on the **same** transport (**T2** or **T3**), and feeds the **last** body to JSON-LD parsing (see **`resources/builtin.md`** — *Same-transport challenge refetch*). That is **not** waiting inside one browser tab until the interstitial clears (no Built In logic in **`browserfetch`** for that); each **`FetchHTMLDocument`** remains **one** navigate + **load** + short settle + read HTML, and the **`5 s`** gap is only between those invocations when **`builtin`** decides to retry.

## Implementation home

- Package: **`internal/collectors/browserfetch`**.
- **`HTMLDocumentFetcher`** — interface; **`RodFetcher`** — go-rod implementation (**`NewRodFetcher`**, **`Close`**).
- **Rod** is the reference backend per **`.specify/memory/constitution.md`**.
- **Configuration:** **`contracts/environment.md`** + **`internal/config/collectors_browser.go`**.

### Lifecycle / ops

- **One long-lived Chromium process** per agent/worker process by default (**`JOBHOUND_BROWSER_ENABLED`** defaults on; **`bootstrap.MVPCollectors`** calls **`NewRodFetcher`** once). Set **`JOBHOUND_BROWSER_ENABLED=0`** to skip the browser.
- **One new browser tab (page) per `FetchHTMLDocument` call**, closed after HTML is read — isolates failures and avoids navigation state leaks; startup cost is amortized across many calls on the shared browser.

## Testing

- Default **`go test ./...`**: **no** mandatory live network; **no** mandatory local Chrome — URL validation tests in **`browserfetch`**; Built In tests use a **fake** **`HTMLDocumentFetcher`**.
- Optional: **`go test -tags=integration ./internal/collectors/browserfetch`** with **`JOBHOUND_BROWSER_INTEGRATION=1`** for a live **`https://example.com/`** smoke test (needs Chromium on the machine).

## Related

- `../spec.md` — tiering, Follow-ups
- `../resources/builtin.md` — Built In T2 vs T3 delivery
- `../contracts/environment.md` — `JOBHOUND_*` knobs
- `../contracts/sources-inventory.md` — row 6 (Built In), row 7 (LinkedIn)
- `../tasks.md` — § **M**
- `collector.md` — collector layout; browser fetch is **infrastructure**, not `Collector`
