# Plan 34 — Sharing

Issue: [#24](https://github.com/occupysaas/salescompel/issues/24)

## Track and dependencies

- **Track:** S — Security model
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** [Plan 33](plan-33-users-profiles-permissions.md)
- **Roadmap:** [§3, Plan 34](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Org-wide defaults and ownership.
- Role hierarchy.
- Manual shares and recalculation.
- Teams, territories, restriction rules and sharing rules beyond owner-based ones fail closed.
- *E2E:* a multi-role access matrix through clients and the browser.

## Out of scope

- Teams, territories, restriction rules and non-owner sharing rules (later, and fail closed until then).

## Code areas

Likely to touch:

- `internal/policy`
- `internal/soql` and `internal/records` access filters
- a forward-only migration

## Decisions needed

- The roadmap marks this plan Full: its own design settles recalculation and storage before work starts.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-34-sharing-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-34-sharing.md`
5. OpenSpec change: `openspec/changes/plan-34-sharing/`
6. Branch on your fork: `plan-34-sharing`. The pull request says `Closes #<issue>`.
