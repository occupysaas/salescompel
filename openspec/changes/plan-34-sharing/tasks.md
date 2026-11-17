# Tasks: plan-34-sharing

Implementation order matches `docs/superpowers/plans/2026-11-17-plan-34-sharing.md`
Tasks 0-15. Each step is TDD and commits with DCO sign-off.

## 0. OpenSpec change
- [x] 0.1 Proposal, design, tasks and the `sharing` spec

## 1. Observation
- [x] 1.1 Release-note review and documented behaviour with gaps

## 2. Migration
- [x] 2.1 `owner_id`, `sc_sharing`, `user_role_id`, defaults, upgrade test

## 3. OwnerId
- [x] 3.1 Field, default, validation, `Owner` relationship

## 4. Roles, groups, closure
- [x] 4.1 UserRole, Group, GroupMember, closure rebuild, revision

## 5. Sharing context
- [x] 5.1 OWDs, groups, visible owners, cache

## 6. Predicate builder
- [x] 6.1 Predicate SQL per term and level

## 7. SOQL enforcement
- [x] 7.1 Root, child, semi-join, parent join, aggregates, locators

## 8. Write enforcement
- [x] 8.1 Update, delete, transfer, lookups, upsert

## 9. Share objects
- [x] 9.1 AccountShare, ContactShare, computed rows (`__Share` deferred, G34-15)

## 10. Operator commands
- [x] 10.1 `sharing set-default`, `sharing rule`

## 11. Implicit sharing
- [x] 11.1 Controlled by Parent, owner child access, implicit parent

## 12. OpenAPI
- [x] 12.1 Objects, `OwnerId`, errors, guards

## 13. Matrix and size
- [x] 13.1 Sharing matrix and size budget

## 14. Acceptance
- [x] 14.1 `sharing/*` acceptance steps and full run (browser leg deferred, G34-17)

## 15. Records
- [x] 15.1 Reference record, roadmap, PR
