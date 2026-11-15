# Plan 33 — Users, profiles and permission sets

Issue: —

## Track and dependencies

- **Track:** S — Security model
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** Plan 26 (merged), Plan 27 (merged), Plan 29 (merged)
- **Roadmap:** [§3, Plan 33](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- User records.
- Object and field permissions enforced in REST, Composite, Bulk and SOQL.
- User language, locale and timezone settings.
- Permissions the candidate cannot evaluate deny access (fail closed).
- *E2E:* the access matrix, with a restricted user denied through real clients and no unexpected grants.

## Out of scope

- Sharing, roles and record-level access (Plan 34).
- Metadata deploy of profiles and permission sets (Plan 35).

## Code areas

Likely to touch:

- `internal/policy`
- `internal/platform` (OAuth identity, request context)
- `internal/soql` field security
- `internal/records`
- a forward-only migration

## Decisions needed

- The roadmap marks this plan Full: its own design settles the permission model before work starts.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-33-users-profiles-permissions-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-33-users-profiles-permissions.md`
5. OpenSpec change: `openspec/changes/plan-33-users-profiles-permissions/`
6. Branch on your fork: `plan-33-users-profiles-permissions`. The pull request says `Closes #<issue>`.
