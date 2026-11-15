# Plan 50 — First-run documentation

Issue: [#40](https://github.com/occupysaas/salescompel/issues/40)

**Maintainer-only.** This item is reserved for the maintainer.

## Track and dependencies

- **Track:** E — Release
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** [Plan 49](plan-49-release-pipeline.md)
- **Roadmap:** [§3, Plan 50](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- A quickstart and a deployment guide.
- A compatibility page that lists gaps first.
- "Migrating from Salesforce".
- A stability promise: under `0.x`, the API, configuration and upgrade path change only in a new minor version, with a changelog entry and an upgrade path. The UI and internal packages are not covered.

## Out of scope

- Versioned documentation for later releases.

## Code areas

Likely to touch:

- `docs/`
- `README.md`

## Decisions needed

- Reserved for the maintainer.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-50-first-run-docs-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-50-first-run-docs.md`
5. OpenSpec change: `openspec/changes/plan-50-first-run-docs/`
6. Branch on your fork: `plan-50-first-run-docs`. The pull request says `Closes #<issue>`.
