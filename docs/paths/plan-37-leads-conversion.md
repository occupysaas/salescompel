# Plan 37 — Leads and conversion

Issue: [#27](https://github.com/occupysaas/salescompel/issues/27)

## Track and dependencies

- **Track:** C — CRM objects (independent). It ignores the track's one-at-a-time rule and doesn't count toward it.
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** [Plan 34](plan-34-sharing.md), [Plan 36](plan-36-application-shell.md)
- **Roadmap:** [§3, Plan 37](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Lead object and statuses.
- Conversion into Account, Contact and Opportunity.
- *E2E:* browser and API, by role.

## Out of scope

- Assignment rules and automation (roadmap §5).

## Code areas

Likely to touch:

- `internal/metadata` catalog
- `internal/records`
- `web/src`

## Decisions needed

- None known.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-37-leads-conversion-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-37-leads-conversion.md`
5. OpenSpec change: `openspec/changes/plan-37-leads-conversion/`
6. Branch on your fork: `plan-37-leads-conversion`. The pull request says `Closes #<issue>`.
