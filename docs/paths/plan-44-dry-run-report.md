# Plan 44 — Dry-run report

Issue: —

## Track and dependencies

- **Track:** D — Data in and out
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** [Plan 35](plan-35-metadata-subset.md)
- **Roadmap:** [§3, Plan 44](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Accepts a backup ZIP or CSV set plus optional metadata.
- Validates hashes, paths, encodings and schemas.
- Classifies each item as supported, stored not enforced, absent, inaccessible or unsupported.
- Produces a human-readable and a machine-readable report, with no changes to target data.
- *E2E:* fixtures with Apex, Flow, formulas and malformed files each produce the correct blocker, never false success.

## Out of scope

- Loading data (Plan 45).
- Connected extraction (Plan 47).

## Code areas

Likely to touch:

- new bundle reader and report
- `internal/metadata`

## Decisions needed

- The roadmap marks this plan Full: its own design settles the bundle format and report schema.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-44-dry-run-report-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-44-dry-run-report.md`
5. OpenSpec change: `openspec/changes/plan-44-dry-run-report/`
6. Branch on your fork: `plan-44-dry-run-report`. The pull request says `Closes #<issue>`.
