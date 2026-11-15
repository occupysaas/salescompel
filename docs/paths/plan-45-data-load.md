# Plan 45 — Data load

Issue: —

## Track and dependencies

- **Track:** D — Data in and out
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** [Plan 32](plan-32-bulk-api.md), [Plan 44](plan-44-dry-run-report.md)
- **Roadmap:** [§3, Plan 45](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Loads in dependency order and resolves cycles and polymorphic references.
- Preserves source IDs at the boundary.
- Maps owners and inactive users.
- Loads files and notes.
- Keeps source audit values as provenance.
- Resumable and idempotent. The load path suppresses side effects.
- *E2E:* crash, restart and reimport produce no duplicates.

## Out of scope

- Deltas and cutover (roadmap §5).

## Code areas

Likely to touch:

- new loader
- `internal/records` write path
- Bulk job machinery from Plan 32

## Decisions needed

- The roadmap marks this plan Full: its own design settles resume and ID mapping.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-45-data-load-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-45-data-load.md`
5. OpenSpec change: `openspec/changes/plan-45-data-load/`
6. Branch on your fork: `plan-45-data-load`. The pull request says `Closes #<issue>`.
