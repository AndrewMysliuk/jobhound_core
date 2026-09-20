# Feature documentation templates

Three files — one set. Fill them, then implement from `tasks.md` without extra questions.

## How to use

1. Copy the folder or individual files into `docs/<feature_slug>/`.
2. Rename by feature slug, for example:
   - `concept.md` → `my-feature-concept.md`
   - `schemas-contracts.md` → `my-feature-schemas-contracts.md`
   - `tasks.md` → `my-feature-tasks.md`
3. Replace all `{{...}}` placeholders and `[...]` blocks with concrete details.
4. Link sibling UI work if the slice changes the browser client (`jobhound_frontend/docs/...`).
5. In `tasks.md`, keep checkboxes up to date as you implement.

## Writing order

| # | File | Purpose |
|---|------|---------|
| 1 | `concept.md` | **Why** and **how** at product and architecture level. Behavior, boundaries, modules, work order. No handler signatures or JSON bodies unless needed for clarity. |
| 2 | `schemas-contracts.md` | **Contracts**: types, enums, HTTP, Temporal payloads, storage, env keys, errors. Verifiable in code. Optional for a pure refactor with no new surface. |
| 3 | `tasks.md` | **Implementation plan**: atomic tasks with IDs, checkboxes, dependencies, milestones. |

## Reference

- Project standards: [`.cursor/rules/specify-rules.mdc`](../rules/specify-rules.mdc)
- Audit a slice: [`.cursor/skills/audit-feature-tasks`](../skills/audit-feature-tasks/SKILL.md)

## Tips

- **Concept** — user/runtime flows and module boundaries (what happens on start / fail / empty).
- **Contracts** — one type per entity; map `internal/<module>/` layers this slice touches.
- **Tasks** — prefixes adapted to the feature (e.g. `T*` schema, `S*` storage, `I*` impl, `H*` handlers, `W*` workflows, `Q*` tests). Every checkbox must be agent-implementable in code; put manual QA / DoD / E2E outside this file.
