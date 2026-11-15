# Plan 47 — Connected extraction

Issue: [#37](https://github.com/occupysaas/salescompel/issues/37)

## Track and dependencies

- **Track:** D — Data in and out (independent). It ignores the track's one-at-a-time rule and doesn't count toward it.
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** [Plan 44](plan-44-dry-run-report.md)
- **Roadmap:** [§3, Plan 47](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Extracts data, metadata and files from an org the user authorizes, into the same bundle as Plan 44.
- *E2E:* against the trial org where available, otherwise against SalesCompel itself as the source, with the limit logged.

## Out of scope

- Deltas and final-window cutover (roadmap §5).

## Code areas

Likely to touch:

- new extraction client
- the Plan 44 bundle format

## Decisions needed

- None known. Follow CLEAN_ROOM.md for any reference-org access.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-47-connected-extraction-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-47-connected-extraction.md`
5. OpenSpec change: `openspec/changes/plan-47-connected-extraction/`
6. Branch on your fork: `plan-47-connected-extraction`. The pull request says `Closes #<issue>`.
