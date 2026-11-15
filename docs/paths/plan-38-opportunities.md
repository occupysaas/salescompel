# Plan 38 — Opportunities

Issue: —

## Track and dependencies

- **Track:** C — CRM objects (independent). It ignores the track's one-at-a-time rule and doesn't count toward it.
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** [Plan 34](plan-34-sharing.md), [Plan 36](plan-36-application-shell.md)
- **Roadmap:** [§3, Plan 38](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Opportunity and stages.
- Sales path.
- Contact roles.
- Stage history.
- *E2E:* a browser journey, and report totals once Plan 43 lands.

## Out of scope

- Forecasting and multi-currency (roadmap §5).
- Products and line items (Plan 39).

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
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-38-opportunities-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-38-opportunities.md`
5. OpenSpec change: `openspec/changes/plan-38-opportunities/`
6. Branch on your fork: `plan-38-opportunities`. The pull request says `Closes #<issue>`.
