# Plan 31 — Composite II and sObject Collections

Issue: —

## Track and dependencies

- **Track:** W — Composite (independent). It ignores the track's one-at-a-time rule and doesn't count toward it.
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** Plan 29 (merged)
- **Roadmap:** [§3, Plan 31](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Extends Plan 24 with `allOrNone: false`.
- Read and query subrequests.
- Upsert subrequests.
- sObject Collections.
- Composite Graph and Batch only if ready.
- *E2E:* JSforce collection calls and raw HTTP.

## Out of scope

- Bulk API (Plan 32).
- Record lifecycle fields and recycle bin (Plan 30). If Plan 30 lands first, rebase onto its write-path changes.

## Code areas

Likely to touch:

- `internal/platform/composite.go`, `http_composite.go`
- `internal/records` write path (shared with Plan 30)
- `api/candidate-openapi.yaml`

## Decisions needed

- None known.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-31-composite-collections-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-31-composite-collections.md`
5. OpenSpec change: `openspec/changes/plan-31-composite-collections/`
6. Branch on your fork: `plan-31-composite-collections`. The pull request says `Closes #<issue>`.
