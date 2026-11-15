# Plan 43 — Basic reports and dashboards

Issue: —

## Track and dependencies

- **Track:** A — Application
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** Plan 29 (merged), [Plan 34](plan-34-sharing.md), [Plan 42](plan-42-list-views-search.md)
- **Roadmap:** [§3, Plan 43](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Tabular and summary reports.
- Simple dashboards.
- Running-user security.
- *E2E:* report totals under each role match the records that role can see.

## Out of scope

- The Reports API (roadmap §5).

## Code areas

Likely to touch:

- `web/src`
- `internal/soql`
- report storage and a forward-only migration

## Decisions needed

- The report builder scope, which is the main size risk in Stage C.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-43-reports-dashboards-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-43-reports-dashboards.md`
5. OpenSpec change: `openspec/changes/plan-43-reports-dashboards/`
6. Branch on your fork: `plan-43-reports-dashboards`. The pull request says `Closes #<issue>`.
