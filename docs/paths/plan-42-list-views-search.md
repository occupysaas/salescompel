# Plan 42 — List views and search

Issue: [#32](https://github.com/occupysaas/salescompel/issues/32)

## Track and dependencies

- **Track:** A — Application
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** Plan 29 (merged), [Plan 36](plan-36-application-shell.md)
- **Roadmap:** [§3, Plan 42](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Saved list views with filters and sorting.
- Global search.
- A SOSL subset.
- PostgreSQL full-text search first. A dedicated search backend only if tests show the need.

## Out of scope

- Reports (Plan 43).

## Code areas

Likely to touch:

- `web/src`
- `internal/soql` (SOSL subset)
- search storage and a forward-only migration

## Decisions needed

- How closely relevance follows Salesforce.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-42-list-views-search-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-42-list-views-search.md`
5. OpenSpec change: `openspec/changes/plan-42-list-views-search/`
6. Branch on your fork: `plan-42-list-views-search`. The pull request says `Closes #<issue>`.
