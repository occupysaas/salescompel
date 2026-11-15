# Plan 46 — Reconciliation and export

Issue: [#36](https://github.com/occupysaas/salescompel/issues/36)

## Track and dependencies

- **Track:** D — Data in and out
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** [Plan 45](plan-45-data-load.md)
- **Roadmap:** [§3, Plan 46](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Counts, field hashes, relationships, ownership and file hashes.
- A portable SalesCompel export, so the project creates no new lock-in.
- *E2E:* zero unexplained differences on the fixture org, and export then reimport to a clean tenant.

## Out of scope

- Deltas and cutover (roadmap §5).

## Code areas

Likely to touch:

- the Plan 44 report and the Plan 45 loader

## Decisions needed

- None known.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-46-reconciliation-export-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-46-reconciliation-export.md`
5. OpenSpec change: `openspec/changes/plan-46-reconciliation-export/`
6. Branch on your fork: `plan-46-reconciliation-export`. The pull request says `Closes #<issue>`.
