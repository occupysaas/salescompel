# Design: plan-34-sharing

## Context

The narrative source is the Plan 34 Superpowers spec
(`docs/superpowers/specs/2026-11-17-plan-34-sharing-design.md`). This design
records the decisions the requirements bind to. Superpowers plan Tasks 0-15
are the implementation order.

## Goals / Non-Goals

**Goals:**

- Every record has an owner; access is decided by Salesforce's documented
  model of OWDs, the role hierarchy, groups, manual shares, owner-based rules
  and implicit sharing, with View All / Modify All bypasses.
- One predicate on every read and write path; hidden records are
  indistinguishable from missing ones.
- Changes take effect in the same transaction; no recalculation lag.
- The upgrade changes no existing user's access.

**Non-Goals:**

- Teams, territories, restriction rules, criteria-based rules, queues as
  owners, guest/portal/external sharing, Apex managed sharing, a setup UI,
  metadata deploy of sharing settings (Plan 35), disabling Grant Access Using
  Hierarchies, asynchronous recalculation.

## Decisions

1. Plan 34 adds `OwnerId`; Plan 30 keeps the other audit fields (pending #24).
2. Sharing is evaluated at query time over a maintained group closure, not
   materialized share rows.
3. Share-object queries return stored Manual rows plus computed Owner and Rule
   rows; implicit rows are not returned.
4. Existing and new tenants start at Public Read/Write on every object.
5. Static-token principals own nothing; records they create go to the
   tenant's default owner (oldest active System Administrator).
6. OWDs and owner-based sharing rules are set with `salescompel sharing`
   until Plans 35 and 36.
7. Grant Access Using Hierarchies is always on.
8. Account transfer moves the old owner's Contacts under it, pending Task 1.
9. Query-locator pages re-apply the sharing predicate and drop records that
   lost visibility after capture.
10. A hidden parent in a relationship path behaves as null in results and
    filters.
11. Deactivating a user removes their group memberships and the manual shares
    granted to them.

## Risks / Trade-offs

- Large role trees produce large visible-owner arrays. Task 13 measures a
  1,000-user, 6-level tree and may switch to a temporary table.
- Documentation-only behaviour (error codes, transfer cascade, deactivation)
  may differ from Salesforce; Task 1 logs each as a gap until observed.
- Plan 30 overlap on `OwnerId` needs maintainer confirmation.
