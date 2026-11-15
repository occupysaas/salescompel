# Plan 35 — Metadata deploy and retrieve subset

Issue: —

## Track and dependencies

- **Track:** D — Data in and out
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** Plan 28 (merged), [Plan 33](plan-33-users-profiles-permissions.md)
- **Roadmap:** [§3, Plan 35](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Deploy, retrieve, validate and status for CustomObject, CustomField, Profile, PermissionSet, Layout, RecordType and Translations.
- The minimum Tooling calls the CLI needs.
- Everything else in a package is stored, not enforced, and reported in the deploy result.
- *E2E:* `sf project deploy start`, `retrieve start`, and a failed deployment leaving correct state.

## Out of scope

- Source tracking, scratch orgs, sandboxes and packages (roadmap §5).

## Code areas

Likely to touch:

- new Metadata API endpoints
- `internal/metadata`
- `internal/migration` (dynamic schema)
- `api/candidate-openapi.yaml`

## Decisions needed

- The type list.
- How unknown types are retained.
- The deploy result wording for stored types.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-35-metadata-subset-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-35-metadata-subset.md`
5. OpenSpec change: `openspec/changes/plan-35-metadata-subset/`
6. Branch on your fork: `plan-35-metadata-subset`. The pull request says `Closes #<issue>`.
