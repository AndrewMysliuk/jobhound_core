---
name: audit-feature-tasks
description: >-
  Audits a feature `*-tasks.md` slice against the codebase: checkbox truth,
  missing work, duplicates, overengineering, dead code. Use when the user asks
  to check tasks, review a slice, verify task completion, audit docs tasks,
  or whether a feature plan is actually done.
---

# Audit feature tasks

Read this skill, then audit. **Do not implement fixes** unless the user asks after the review.

Task files follow `.cursor/templates/tasks.md`. Live under `docs/<feature_slug>/*-tasks.md`.

## Locate the plan

1. Use the path the user named. Else search `docs/**/*-tasks.md` in this repo.
2. If several files match, pick the one they mentioned; if still ambiguous, ask.
3. Also read, when present in the same folder: `*-concept.md`, `*-schemas-contracts.md`. Product snapshot: `docs/REPO_SNAPSHOT.md` if linked.
4. Follow `.cursor/rules/specify-rules.mdc`. If the slice includes UI, also search `jobhound_frontend/docs/**/*-tasks.md`.

## Parse the tasks file

Treat the markdown as the contract, not git status.

- **Scope / In this slice** — required work.
- **Paused / out of scope** — must stay unimplemented (or clearly leftover, not new).
- **IDs** — `T1`, `S2`, `I1`, `H1`, `W1`, `Q1`, … (prefixes vary per feature).
- **Checkboxes** — `[x]` claimed done, `[ ]` open. **Do not trust them.**
- **File paths** in a task — verify those files (and nearby call sites).
- **Dependencies / M1–Mn** — use for leftover order, not as a substitute for code checks.

A task is **done** only if the behavior and files match the bullet, including “drop / remove / only X” wording.

**Skip as work items** (do not verify by running the product, do not put in Plan): `RV*`, “manual”, “QA script”, “DoD checklist”, “E2E”, “smoke”, “walkthrough”, “open the app and click”. The user runs those. Infer from code only if needed for a one-line leftover note.

## Verify each task

For every **agent-implementable** ID (code, types, unit tests, migrations in repo):

1. Find the code (and tests) the task names.
2. Confirm the change exists **and** the old path is gone when the task says drop/remove.
3. Confirm paused items were **not** built.
4. Classify:
   - **Done** — matches the task.
   - **False done** — `[x]` but missing, partial, or reverted.
   - **Open** — `[ ]` and still missing.
   - **Extra** — code beyond slice / paused list.
   - **Checkbox drift** — code done but still `[ ]`.

If the tasks file names UI work, also check `jobhound_frontend`.

Do not run the product, browser QA, or live Temporal/LLM against production keys.

## Quality pass (touched code only)

Inspect files the slice changed (and obvious leftovers of the same feature). Look for:

| Smell | What to flag |
|-------|----------------|
| **Dead / garbage** | Unused exports, leftover debug routes, commented-out blocks, orphan migrations, constants nobody reads |
| **Duplicates** | Same helper/type in two modules; parallel “v2” packages; copy-pasted mapper that should be one function |
| **Overengineering** | Extra abstraction, wrapper, or endpoint the task did not ask for; speculative flags; unused configurability |
| **Layout leak** | Helpers at module root; Temporal SDK inside `impl/`; `os.Getenv` outside `internal/config`; package-level funcs in a route file; enum with only `Valid()` |
| **Scope leak** | Paused bullets implemented; drive-by refactors; public API/contract changes the task did not list |

Ignore pre-existing mess **outside** the slice unless it blocks the feature or was supposed to be deleted by a “drop” task.

Project defaults: minimal, explicit, no new abstractions, no public API changes unless the task says so. Follow `specify-rules.mdc` (module layout, Canonical Enum, schema-first JSON, Temporal separation).

Flag only smells the agent can delete or rewrite in code.

## Output (user language)

Keep it short. No motivational text. Write in the same language as the user.

**Plan = only what this agent can patch** (files, tests, types, checkbox sync). No “run through the UI”, no “you should QA”.

```markdown
## Verdict
[one line: slice done / not done / done with leftovers]

## Checkboxes vs code
- **False done:** ID — what is missing (path)
- **Open:** ID — still required
- **Drift:** ID — code is done, box still empty
- **Paused but present:** what leaked

(omit empty bullets; omit RV* / manual IDs)

## Quality
- **Must fix:** duplicate / dead / overengineered — path + why
- **Should fix:** smaller leftovers

(omit if clean)

## Plan
1. [ID or smell] — concrete action (file + what to change)
2. ...

Ready to apply this plan on request. Say which items (or “all”).
```

Order the plan: correctness (false done / open in-slice) → dead code from “drop” tasks → duplicates / overengineering / layout leaks → checkbox sync last.

If everything agent-fixable is clean: one-line verdict + “no plan.” Do not invent work. Do not pad with QA.

Optional, one line only if the tasks file still lists them: **Human leftover:** `RV1` (you run this). Never number it in Plan.

## After the review

- Do **not** edit code, and do **not** retick `*-tasks.md`, until the user asks.
- When they ask to fix: follow **Plan** only, smallest diffs, no extra refactors. Skip Human leftover. Then re-check only the items you touched.
- Do not add E2E, DoD docs, or browser walkthroughs “just in case.”
