# Tasks: plan-34-sharing

Implementation order matches `docs/superpowers/plans/2026-11-17-plan-34-sharing.md`
Tasks 0-15. Each step is TDD and commits with DCO sign-off.

## 0. OpenSpec change
- [x] 0.1 Proposal, design, tasks and the `sharing` spec

## 1. Observation
- [ ] 1.1 Release-note review and documented behaviour with gaps

## 2. Migration
- [ ] 2.1 `owner_id`, `sc_sharing`, `user_role_id`, defaults, upgrade test

## 3. OwnerId
- [ ] 3.1 Field, default, validation, `Owner` relationship

## 4. Roles, groups, closure
- [ ] 4.1 UserRole, Group, GroupMember, closure rebuild, revision

## 5. Sharing context
- [ ] 5.1 OWDs, groups, visible owners, cache

## 6. Predicate builder
- [ ] 6.1 Predicate SQL per term and level

## 7. SOQL enforcement
- [ ] 7.1 Root, child, semi-join, parent join, aggregates, locators

## 8. Write enforcement
- [ ] 8.1 Update, delete, transfer, lookups, upsert

## 9. Share objects
- [ ] 9.1 AccountShare, ContactShare, `__Share`, computed rows

## 10. Operator commands
- [ ] 10.1 `sharing set-default`, `sharing rule`

## 11. Implicit sharing
- [ ] 11.1 Controlled by Parent, owner child access, implicit parent

## 12. OpenAPI
- [ ] 12.1 Objects, `OwnerId`, errors, guards

## 13. Matrix and size
- [ ] 13.1 Sharing matrix and size budget

## 14. Acceptance
- [ ] 14.1 `candidate/sharing/matrix` and full run

## 15. Records
- [ ] 15.1 Reference record, roadmap, PR
