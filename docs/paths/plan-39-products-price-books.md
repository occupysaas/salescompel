# Plan 39 — Products and price books

Issue: [#29](https://github.com/occupysaas/salescompel/issues/29)

## Track and dependencies

- **Track:** C — CRM objects (independent). It ignores the track's one-at-a-time rule and doesn't count toward it.
- **Size:** Short (from the roadmap). It can be planned straight from this brief after Task 1.
- **Needs:** [Plan 38](plan-38-opportunities.md)
- **Roadmap:** [§3, Plan 39](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Products and price books.
- Price book entries.
- Opportunity line items.
- Single currency.
- *E2E:* browser and API.

## Out of scope

- Multi-currency (roadmap §5).

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
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-39-products-price-books-design.md` (optional for a Short plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-39-products-price-books.md`
5. OpenSpec change: `openspec/changes/plan-39-products-price-books/`
6. Branch on your fork: `plan-39-products-price-books`. The pull request says `Closes #<issue>`.
