# Plan 30 — Record lifecycle

Issue: —

## Track and dependencies

- **Track:** R — Records API
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** Plan 27 (merged), Plan 29 (merged)
- **Roadmap:** [§3, Plan 30](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- System and audit fields (`CreatedDate`, `CreatedById`, `LastModifiedDate`, `LastModifiedById`, `SystemModstamp`) and `OwnerId`.
- Recycle bin, delete and undelete.
- `queryAll`, `getUpdated` and `getDeleted`.
- 15- and 18-character IDs accepted everywhere.
- `/limits` and the `Sforce-Limit-Info` header.
- Conditional request headers.
- *E2E:* delete, restore and query-all flows through real clients.

Existing groundwork: PR #19 (offline checks for `/limits` and `Sforce-Limit-Info` in the compatibility lab). It is not a claim.

## Out of scope

- Bulk API (Plan 32) and Composite changes (Plan 31).
- Sharing and owner-based access (Plan 34). `OwnerId` is stored and returned here but not yet enforced.

## Code areas

Likely to touch:

- `internal/records` (the one write path, roadmap §6)
- `internal/platform` REST handlers and headers
- `internal/soql` for `queryAll` and deleted rows
- a forward-only migration for audit and delete columns

## Decisions needed

- None known. Follow the roadmap's "follow the documentation and log the gap" rule for limit values.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-30-record-lifecycle-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-30-record-lifecycle.md`
5. OpenSpec change: `openspec/changes/plan-30-record-lifecycle/`
6. Branch on your fork: `plan-30-record-lifecycle`. The pull request says `Closes #<issue>`.
