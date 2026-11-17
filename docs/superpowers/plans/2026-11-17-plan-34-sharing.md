# Plan 34 - Sharing Implementation Plan

> **For agentic workers:** use superpowers:subagent-driven-development or
> superpowers:executing-plans, task by task. Steps use checkbox syntax.

**Status:** Tasks 0-1 done, 2026-11-17 ([reference record](../../compat/plan-34-sharing.md)). Waiting on the maintainer's review of the
spec and the `OwnerId` boundary with Plan 30 (#24).

**Goal:** Record ownership, org-wide defaults, the role hierarchy, groups,
manual shares, owner-based sharing rules and Account/Contact implicit
sharing, enforced on every read and write path; a multi-role access matrix
through unmodified clients and the browser.

**Spec:** [Plan 34 design](../specs/2026-11-17-plan-34-sharing-design.md).
Read it first. Every rule below comes from the spec; where they differ, the
spec wins and the plan is corrected.

**Reference record:** `docs/compat/plan-34-sharing.md` (created in Task 1).

## Global constraints

- Go 1.25.0 (`GOTOOLCHAIN=go1.25.0` or the `go1.25.0` wrapper), PostgreSQL 17,
  pgx v5, API v65.0. No new runtime dependency.
- Every record write through `records.Tx`; every record read through the
  SOQL engine (`Tx.Read` already is). One PostgreSQL transaction per HTTP
  request. Sharing data is read in that transaction.
- Plan 33's access matrix (`TestAccessMatrixNoUnexpectedGrants`), Plan 31's
  Composite and Collections tests, and every Plan 24–33 acceptance gate stay
  green without editing their assertions. Tenants start at Salesforce's new-org
  OWDs (Account and custom objects Public Read/Write, Contact Controlled by
  Parent, D34-18), which give today's access, so existing tests see no change.
- No source file over 500 lines: `sh scripts/check-file-size.sh` before every
  commit. `soql/bind.go` (494), `soql/compile.go` (434), `records/write.go`
  (424) and `metadata/admin.go` (443) are near the limit: new code goes into
  new files (`soql/sharing*.go`, `records/sharing*.go`,
  `records/owner.go`, `policy/sharing*.go`).
- Every commit `git commit -s`. One PR for the plan, a commit per task.
- The migration takes its number at merge time (0008 if nothing lands
  first). `scripts/check-migrations.sh` before the PR.

## Review focus

- **No unexpected grant.** `TestSharingMatrix` computes the expected answer
  independently of the predicate builder and checks every path for every
  user and record.
- **No existence leak.** A hidden record answers exactly as an unknown ID on
  REST, Composite, Collections, SOQL counts and aggregates.
  `TestSharingHiddenEqualsMissing`.
- **Closure consistency.** A role move or group change and a concurrent query
  never see a half-built `user_groups`. `TestSharingClosureConcurrent`.
- **Owner change cleanup.** Manual shares are gone after a transfer, in the
  same transaction. `TestOwnerTransferDropsManualShares`.
- **Parent joins.** A hidden parent is null in results and in filters, and
  never contributes a value. `TestSharingHiddenParentIsNull`.
- **Upgrade.** A Plan 33 database with data migrates with every user's
  visible records unchanged. `TestMigrationSharingUpgrade`.

## File map

| Files | Responsibility |
| --- | --- |
| `internal/migration/migrations/00NN_sharing.sql` (+ test) | `owner_id`, `sc_sharing` schema, `user_role_id`, defaults, grants |
| `internal/metadata/seed_sharing.go` | UserRole, Group, GroupMember, share objects, `OwnerId` field registration |
| `internal/metadata/admin.go` | `owner_id` column on new custom objects (one line; helper in a new file) |
| `internal/policy/sharing.go`, `sharing_load.go` (+ tests) | Sharing context: OWDs, group closure, visible owners, revision cache |
| `internal/soql/sharing.go` (+ test) | Predicate SQL for an alias, object and level |
| `internal/soql/compile.go`, `sqlexpr.go`, `paging.go` | Predicate at root, child, semi-join, parent join, locator pages |
| `internal/records/owner.go`, `sharing_write.go`, `setup_write_sharing.go` (+ tests) | Owner default and transfer, write checks, roles/groups/shares writes, closure rebuild |
| `cmd/salescompel/sharing.go` (+ test) | `sharing set-default`, `sharing rule add|list|remove` |
| `api/candidate-openapi.yaml`, `compat/clients/node/src/candidate-openapi-*.ts` | New objects, `OwnerId`, errors |
| `compat/clients/node/src/candidate-sharing*.ts`, `test/candidate-sharing.test.ts`, acceptance wiring | Acceptance |

---

### Task 0: OpenSpec change

**Files:** `openspec/changes/plan-34-sharing/`.

- [x] `proposal.md`, `design.md` (decisions from the spec's "Calls made"),
      `tasks.md` mirroring this plan, and `specs/sharing/spec.md` with one
      requirement per enforced rule (ownership, OWD, hierarchy, groups,
      manual shares, rules, implicit sharing, enforcement points, errors,
      migration). Requirement IDs `shr-NN`, referenced from code comments as
      `// spec: shr-NN`, as Plans 29 and 31 do.
- [x] Regenerate digests with `.git/sc-openspec-digest.py
      openspec/changes/plan-34-sharing`.
- [x] Commit `docs: add Plan 34 OpenSpec change`.

### Task 1: Observation and changelog review

**Files:** `docs/compat/plan-34-sharing.md`, `docs/compat/sources.yaml`.

- [x] Review Salesforce release notes since the Plan 33 review for sharing,
      roles, groups, share objects and `OwnerId`. Record relevant items.
- [x] If a trial-org session is available (CLEAN_ROOM.md), observe: REST GET,
      PATCH and DELETE on a hidden record and on a read-only record (status,
      `errorCode`, message); `AccountShare` insert by a non-owner;
      `RowCause` values returned by a share query; Account transfer effect on
      the old owner's Contacts; deactivation effect on manual shares and group
      membership; parent fields through a lookup to a hidden record; upsert
      matching a hidden record; `UserRole` delete with members.
- [x] Otherwise record each as `documented` with its source, and list it as
      a gap. Every "Task 1 confirms" item in the spec gets a row.
- [x] Append sources to `docs/compat/sources.yaml` (keep existing bytes).
- [x] Commit `docs: record Plan 34 observations and sources`.

### Task 2: Migration and upgrade test

**Files:** `00NN_sharing.sql`, `internal/migration/sharing_test.go`,
`runtime_privileges.go` if grants change.

- [ ] Failing test `TestMigrationSharingUpgrade`: build a Plan 33 database
      with two tenants, admin and standard users, a static principal,
      Accounts, Contacts and a custom object with rows; migrate; assert
      `owner_id` backfilled to the default owner (oldest active System
      Administrator, else oldest active user), OWD rows per D34-19,
      one Organization group per tenant with every active user, and each
      user's visible record set unchanged.
- [ ] `TestMigrationSharingRLS`: every `sc_sharing` table has RLS forced
      and the records login cannot read another tenant's rows.
- [ ] Write the migration: `owner_id` (+ index) on every record table found
      in `sc_catalog.objects`; `sc_sharing` tables `object_settings`,
      `roles`, `groups`, `group_members`, `user_groups`, `shares`,
      `owner_rules`, `revisions`; `sc_setup.users.user_role_id`.
- [ ] The no-users tenant case: decide from the test whether it exists; if
      it does, keep the column nullable for that tenant's tables and
      backfill on first user create (spec, Migration).
- [ ] Commit `feat: add sharing storage and record owners`.

### Task 3: `OwnerId`

**Files:** `internal/metadata/seed_sharing.go`, `internal/records/owner.go`,
`admin.go` (column on create), tests.

- [ ] Failing tests: `TestOwnerIdDescribe` (reference to User, relationship
      `Owner`, createable, updateable, not nillable, defaulted on create),
      `TestOwnerDefaultOnCreate`, `TestOwnerGivenMustBeActiveReader`,
      `TestOwnerQueueRefused`, `TestOwnerStaticPrincipalDefaultOwner`,
      `TestOwnerRelationshipSOQL` (`SELECT Owner.Name FROM Account`),
      `TestCustomObjectHasOwner`.
- [ ] Register `OwnerId` on Account, Contact and custom objects in the
      catalog seed and in object creation.
- [ ] Default and validate the owner in the create path, before
      `insertRecord`. Static principals resolve the default owner once per
      request.
- [ ] Commit `feat: add OwnerId to records`.

### Task 4: Roles, groups and the closure

**Files:** `seed_sharing.go`, `records/setup_write_sharing.go`,
`records/sharing_closure.go`, tests.

- [ ] Failing tests: `TestUserRoleCRUD` (Manage Users only; delete with users
      or children refused; cycle refused), `TestUserRoleIdOnUser`,
      `TestGroupRegularCRUD`, `TestGroupSystemTypesReadOnly`,
      `TestGroupMemberNestingAndCycle`, `TestClosureRebuild` (direct,
      nested, Role, both subordinate types, Organization; deactivation keeps
      role and GroupMember rows, drops the user from the closure and deletes
      manual shares to them, D34-25),
      `TestSharingRevisionBumps`.
- [ ] Register UserRole (00E), Group (00G), GroupMember (011) as setup
      objects with Plan 33's setup-write rules. Role, RoleAndSubordinates and
      RoleAndSubordinatesInternal groups are created and deleted with their
      role (D34-12).
- [ ] Rebuild `user_groups` for the tenant in the writing transaction after
      any change listed in the spec, and bump the sharing revision.
- [ ] `TestSharingClosureConcurrent`: concurrent role move and queries under
      the default isolation never observe a partial closure (rebuild in one
      statement, or delete-and-insert under a tenant advisory lock that
      readers don't need).
- [ ] Commit `feat: add roles, groups and the group closure`.

### Task 5: Sharing context

**Files:** `internal/policy/sharing.go`, `sharing_load.go`, tests.

- [ ] Failing tests: `TestSharingContextOWD`, `TestSharingContextGroups`,
      `TestVisibleOwnersHierarchy` (subordinates only, not peers or
      managers), `TestVisibleOwnersRules`, `TestSharingBypass` (View All
      Data, Modify All Data, object View All / Modify All),
      `TestSharingCacheRevision`, `TestSharingFailClosed` (unknown OWD value,
      unknown group type).
- [ ] Load once per (tenant, user, sharing revision); visible owners per
      object and level on first use. Carry it on the execution context
      (`ExecutionContext.SharingMode` stops being the `"tenant"`
      placeholder).
- [ ] Commit `feat: compute the running user's sharing context`.

### Task 6: Predicate builder

**Files:** `internal/soql/sharing.go`, `sharing_test.go` (pure Go).

- [ ] Failing tests, one per term of the spec's predicate (bypass, OWD,
      owner set, share principals, ControlledByParent, implicit parent) and
      per level (Read, Edit, Delete), checking the SQL text and argument
      numbering continues the caller's.
- [ ] Implement `sharingPredicate(alias, object, level, ctx, addArg)`. A
      bypass returns no clause; Public Read/Write at Read and Edit returns no
      clause; unknown returns `false`.
- [ ] Commit `feat: build the sharing predicate`.

### Task 7: SOQL enforcement

**Files:** `compile.go`, `sqlexpr.go`, `paging.go`, `execute.go` (child
subqueries), tests.

- [ ] Failing tests with a Private OWD tenant: `TestSharingSOQLRoot`,
      `TestSharingSOQLChildSubquery`, `TestSharingSOQLSemiJoin` (IN and NOT
      IN), `TestSharingHiddenParentIsNull` (select and WHERE on a parent
      path), `TestSharingCountAndAggregates`, `TestSharingLocatorRefilter`
      (record hidden between pages is dropped).
- [ ] Add the predicate at the root `WHERE`, the child `WHERE`, the
      semi-join `WHERE` and each parent join's `ON` (so a hidden parent
      joins as null). Locator pages re-apply the root predicate.
- [ ] Re-run Plan 29 and Plan 33 SOQL tests unchanged.
- [ ] Commit `feat: enforce sharing in SOQL`.

### Task 8: Write-path enforcement

**Files:** `records/sharing_write.go`, small hooks in `write.go`, tests.

- [ ] Failing tests: `TestSharingUpdateDelete` (hidden: unknown ID;
      readable not editable: `INSUFFICIENT_ACCESS_OR_READONLY`; delete needs
      owner-level access), `TestOwnerTransfer` (who may; Transfer Record
      permission stored, not enforced, D34-09),
      `TestOwnerTransferDropsManualShares` (and the Account's Contact shares),
      `TestAccountTransferMovesContacts` (per Task 1),
      `TestLookupToHiddenRecord`, `TestUpsertMatchesHidden`,
      `TestSharingHiddenEqualsMissing` (REST, Composite and Collections
      bodies byte-equal for hidden and unknown IDs).
- [ ] Check the record's access level before `UPDATE`/`DELETE` with one
      query using the predicate at Read and at the needed level, so the two
      errors can be told apart without a second round trip.
- [ ] Composite and Collections inherit through `records.Tx`; add their
      tests only.
- [ ] Commit `feat: enforce sharing on record writes`.

### Task 9: Share objects

**Files:** `seed_sharing.go`, `records/setup_write_sharing.go` (shares),
a SQL view per share object or a compiler source, tests.

- [ ] Failing tests: `TestShareObjectDescribe` (AccountShare, ContactShare,
      `X__Share`; absent while OWD is Public Read/Write),
      `TestManualShareInsert` (owner, manager above owner, Modify All allowed;
      others refused; RowCause other than Manual refused; level All
      refused), `TestShareQueryComputedRows` (Owner and Rule rows returned,
      implicit rows not), `TestManualShareDelete`, `TestShareGrantsAccess`
      (user, public group, role, role and subordinates).
- [ ] Present `sc_sharing.shares` plus computed Owner and Rule rows as each
      share object. The builder picks a view or a compiler source; the
      choice goes in the reference record.
- [ ] Commit `feat: add manual shares and share objects`.

### Task 10: OWD and sharing-rule commands

**Files:** `cmd/salescompel/sharing.go`, `store.go` helpers, tests.

- [ ] Failing tests: `TestSharingSetDefault` (valid values per object,
      ControlledByParent pairing with Account, revision bump),
      `TestSharingRuleCommands` (add, list, remove; source and target
      groups must exist; Read or Edit only), `TestOrganizationDefaultAccess`
      (`Organization.DefaultAccountAccess` and `DefaultContactAccess`
      reflect the setting).
- [ ] Commit `feat: add sharing operator commands`.

### Task 11: Implicit sharing

**Files:** `soql/sharing.go` terms, `records` tests.

- [ ] Failing tests: `TestContactControlledByParent` (private contacts
      with no Account: owner and bypass holders only, D34-23), `TestAccountOwnerContactAccess`
      (`ContactAccessForAccountOwner` Read and Edit),
      `TestImplicitParentRead`, `TestMasterDetailControlledByParent`.
- [ ] Commit `feat: add Account and Contact implicit sharing`.

### Task 12: OpenAPI and contract guards

- [ ] Add `OwnerId` to record schemas; UserRole, Group, GroupMember,
      AccountShare, ContactShare and the custom share pattern with
      `x-source`; the new errors. Mark stored-not-enforced fields
      (Opportunity and Case access levels) in their descriptions.
- [ ] Update the operation-count guard and traffic tests.
- [ ] `npm run check` in `compat/clients/node`.
- [ ] Commit `feat: describe sharing in OpenAPI`.

### Task 13: Matrix and size

**Files:** `internal/platform/sharing_matrix_test.go` (+ helpers file).

- [ ] `TestSharingMatrix`: a three-level role tree with two branches, a
      public group, users with and without roles, View All / Modify All
      holders, each OWD for Account, Contact and a custom object, manual
      shares, a rule, implicit parent. For every user and record compare SOQL
      visibility, count, REST GET, PATCH, DELETE, Composite and Collections
      retrieve with an expected answer computed by a separate, simple model
      in the test.
- [ ] `TestSharingSize` (skipped under `-short`): 1,000 users, 6 levels,
      100,000 Accounts, Private OWD; record the timing budget in the
      reference record. Decide arrays vs a temporary table (spec, Left to
      the builder).
- [ ] Commit `test: add the sharing access matrix`.

### Task 14: Acceptance

- [ ] `candidate-sharing-fixtures.ts`: journal every create before dispatch,
      as Plan 31 does.
- [ ] `test/candidate-sharing.test.ts`: `candidate/sharing/matrix` — JSforce
      and simple-salesforce as users in different roles under Private OWD:
      create as one, query as the other, manual share through
      `AccountShare`, transfer, check visibility after each step. Browser leg
      with the connected-browser harness, signed in as two users; if Plan 36
      hasn't landed, the existing preview pages, with the gap logged.
- [ ] Wire the name into `candidate-acceptance-run.ts`,
      `candidate-acceptance-contract.ts` (name and required Go tests),
      `candidate-acceptance-results.ts`, and
      `compat/schemas/candidate-acceptance.schema.json`.
- [ ] Full run in the Linux container:
      `CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <private-summary>`.
      Record the gate table in the reference record.
- [ ] Commit `test: add sharing acceptance`.

### Task 15: Records and docs

- [ ] Complete the reference record: final status per row, gaps, measured
      acceptance, tool versions, the size budget.
- [ ] Add the implementation-record line under Plan 34 in the first-release
      roadmap; mark this plan implemented; update `docs/paths/README.md`.
- [ ] `sh scripts/check-file-size.sh`, `scripts/check-dco.sh upstream/main HEAD`,
      `scripts/check-migrations.sh`.
- [ ] Commit `docs: record Plan 34 implementation`; open the PR with
      `Closes #24`, commands run and their output.
