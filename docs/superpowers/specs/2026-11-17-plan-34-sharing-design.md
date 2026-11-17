# SalesCompel — Design: Plan 34, sharing

Status: draft for maintainer approval (2026-11-17). Issue #24. Brief:
`docs/paths/plan-34-sharing.md`.

## Intent

After Plan 33 a user's profile and permission sets decide which objects and
fields they can use. They still see every record of a readable object in
their tenant. Records have no owner: `OwnerId` exists nowhere in the code.

Plan 34 adds Salesforce's record-level model:
- every record has an owner
- each object has an org-wide default (OWD)
- users sit in a role hierarchy, and a manager sees what their subordinates
  own
- owners can share single records with users and groups (manual shares)
- owner-based sharing rules open records owned by one group to another

The same filter applies on every path that returns or changes a record:
SOQL (root, relationships, subqueries, semi-joins, aggregates, query
locators), REST record reads and writes, Composite, Collections, and through
them Bulk (Plan 32).

As in Plan 33, behaviour follows Salesforce's public documentation. Task 1
confirms it on the trial org when a session is available. Otherwise the gap
is logged on the compatibility page.

### Scope boundary with Plan 30

Plan 30 (Record lifecycle, claim requested, no code pushed as of
2026-11-17) lists "store and return `OwnerId`" in its scope. Sharing can't
work without it, so **Plan 34 adds `OwnerId`**: storage, default, the `Owner`
relationship, describe, and ownership transfer. Plan 30 keeps `CreatedById`,
`LastModifiedById`, the date audit fields, the recycle bin and the rest of its
scope. Whichever plan merges second rebases onto the other. Raised on #24 for
the maintainer and Plan 30's claimant to confirm.

## Goals

- `OwnerId` on Account, Contact and every custom object, defaulting to the
  creating user. It is queryable with the `Owner` relationship, describable,
  and settable on create and update.
- Org-wide defaults per object: Private, Public Read Only, Public Read/Write,
  and Controlled by Parent where Salesforce allows it.
- `UserRole` and `User.UserRoleId`. Access is granted up the hierarchy.
- `Group` and `GroupMember`: public groups, plus the system-maintained Role,
  Role and Subordinates, and Organization groups.
- Manual shares through `AccountShare`, `ContactShare` and `<Object>__Share`
  for custom objects. Share rows are readable, and creatable and deletable by
  the people Salesforce allows.
- Owner-based sharing rules, configured by the operator until Plan 35 deploys
  `SharingRules` metadata.
- Implicit sharing between Account and Contact: Contact access controlled by
  the parent, and read access to an Account through its Contacts.
- View All / Modify All (object), and View All Data / Modify All Data
  (system), now bypass sharing as on Salesforce. Plan 33 stored the object
  flags but didn't use them.
- Access changes take effect in the same transaction as the change. There is
  no recalculation lag.
- *E2E:* a multi-role access matrix through unmodified clients and the
  browser.

## Non-goals

- Account, opportunity and case teams. Territory management. Restriction
  rules. Criteria-based sharing rules. Guest, portal and community sharing,
  and external OWDs. Queues as owners. All fail closed (below).
- Apex managed sharing (`RowCause` values other than Owner, Manual,
  Rule and the implicit causes). There is no Apex.
- `Opportunity` and `Case` sharing columns beyond storing them on
  `AccountShare`. The objects arrive in Plans 38 and later, and use this
  design then.
- A setup UI for roles, groups, OWDs and sharing rules (Plan 36 and later).
- Metadata deploy and retrieve of `sharingModel`, `Role`, `Group` and
  `SharingRules` (Plan 35 writes into the storage defined here).
- Disabling "Grant Access Using Hierarchies" on custom objects. It is always
  on here.
- Asynchronous recalculation, deferred sharing calculation and
  the "sharing recalculation in progress" lock.

## Anti-goals

- **A record visible or changeable** by a user whom ownership, OWD, the role
  hierarchy, a share, a rule, implicit sharing or a View/Modify All
  permission does not admit.
- **A record's existence leaking.** Hidden records behave as if they don't
  exist in reads, counts, aggregates, relationship traversal and error
  messages.
- **A sharing setting returned as active that isn't enforced.** Anything
  stored but not enforced is marked so, and fails closed.
- **Today's users losing or gaining access** because of the migration.
- **Manual shares or rules surviving** a change that Salesforce says
  removes them (owner change, user deactivation), so access lingers.

## Constraints

- CLEAN_ROOM.md: public interfaces and documentation only. New objects and
  fields in `api/candidate-openapi.yaml` carry `x-source`.
- Roadmap plan shape (§1): Task 1 observation (documentation plus gap log if
  no trial org), forward-only migration with an upgrade test, black-box
  tests with unmodified clients added to the acceptance script.
- Plan 33's rules stay: one write path (`records.Tx`), one query engine, the
  effective-permission cache keyed by a tenant revision, fail closed.
- The migration number is taken at merge time.
- No source file over 500 lines.
- Sharing must not add a round trip per row. The filter is part of the SQL
  that already runs.

## Approaches considered

1. **Query-time evaluation with maintained group closure (chosen).**
   - Store what people set: owners, OWDs, roles, groups, members, manual
     shares and sharing rules. Maintain one derived table: each user's
     effective groups (direct, nested, role, role-and-subordinates,
     organization), rebuilt for the affected tenant in the same transaction
     as any role, group, membership or `UserRoleId` change.
   - At request start, compute the running user's **visible owners** and
     **share principals** for each object they touch, and pass them as array
     parameters. The filter is
     `owner_id = ANY(owners) OR EXISTS(share row for record and principals)`
     plus OWD and implicit-parent terms.
   - Pros: always consistent, no background job, no share-row explosion,
     OWD changes are instant. "Recalculation" is the closure rebuild and the
     owner-change cleanup, both transactional.
   - Cons: `Owner` rows in `AccountShare` are computed, not stored. Large
     role trees make large arrays (fine at first-release sizes, measured in
     tests).
2. **Materialized share rows, as Salesforce stores them.** A row per
   (record, user or group) for every cause, with `RowCause`. Reads are a
   join. Every owner, role, group, rule or OWD change rewrites share rows,
   which needs batching, a job queue and a consistency window the
   anti-goals forbid. Rejected for this release. The share objects still
   look the same to clients.
3. **PostgreSQL row-level security per user.** Rejected for the same reasons
   as in Plan 33: policies can't produce Salesforce's errors and would churn
   with dynamic objects.

## Design

### Ownership

- **Storage:** every record table gets `owner_id varchar(18) NOT NULL`, with
  an index on `(tenant_id, owner_id)`. New custom-object tables are created
  with it (`internal/metadata/admin.go`).
- **Field:** `OwnerId`, a reference to `User`, relationship name `Owner`,
  createable, updateable, not nillable, defaulted on create. `Owner.Name`
  and the other User fields traverse through the Plan 29 engine. A
  reference to a Group (queue) is refused (`INVALID_CROSS_REFERENCE_KEY`,
  Task 1 confirms), because queues are out of scope.
- **On create:** the running user unless `OwnerId` is given. A given owner
  must be an active user in the tenant with Read on the object, as Salesforce
  requires.
- **Transfer (an update that changes `OwnerId`):** allowed for the owner, a
  user above the owner in the role hierarchy, and holders of Modify All on
  the object or Modify All Data (D34-09). The "Transfer Record" system
  permission is stored and not enforced: Salesforce documents it for mass
  transfer. On transfer:
  - manual shares on the record are deleted, as Salesforce does; for an
    Account, the old owner's manual shares on its Contacts too (D34-10)
  - for Account, Contacts the old owner owned under it move to the new owner
    (D34-11, assumed from the Mass Transfer documentation)
- **Static-token principals** (`salescompel provision`) have no User row.
  Records they create are owned by the tenant's **default owner**: the
  oldest active user with System Administrator. A `restricted` static
  principal sees only what OWDs and Organization-group shares and rules
  grant. Recorded as a gap; Plan 33 already notes `provision` may retire.
- **Deactivating a user** keeps their records, role and GroupMember rows, as
  on Salesforce. They leave the effective closure, so they lose all record
  access at once, and the manual shares granted to them are deleted in the
  same transaction (D34-25).

### Org-wide defaults

- **Values** (internal access, as in the Object Reference's
  `Organization.Default*Access` fields): `None` (Private), `Read`, `Edit`,
  and `ControlledByParent`.
  - Account: Private, Read, Edit.
  - Contact: Private, Read, Edit, ControlledByParent.
  - Custom objects: Private, Read, Edit. A custom object whose record type
    is the detail of a master-detail is always ControlledByParent, as on
    Salesforce.
  - Read/Write/Transfer and Full Access values (Lead, Case, Campaign) wait
    for those objects.
- **Rule:** a child with ControlledByParent needs its parent's OWD to be at
  least as open as it would be; Salesforce enforces the pairing, e.g. Contact
  can't be more open than Account. The builder takes the exact pairings from
  the documentation.
- **Storage:** `sc_sharing.object_settings(tenant_id, object_id,
  internal_access)`. A missing row means the object default below.
- **API:** `Organization.DefaultAccountAccess` and
  `Organization.DefaultContactAccess` are returned (read-only, as on
  Salesforce). Custom-object OWD is returned in describe only where
  Salesforce returns it; otherwise it is reached through Plan 35 metadata.
- **Changing an OWD:** `salescompel sharing set-default <Object> <value>`
  until Plan 36 has a setup page. It bumps the tenant's sharing revision.
  Effective immediately.
- **Defaults:** Salesforce creates new orgs with Account at Public
  Read/Write, Contact Controlled by Parent and custom objects at Public
  Read/Write (D34-18). New tenants start the same way, and the migration sets
  existing tenants to the same values, which give exactly today's access
  (D34-19).

### Roles

- **UserRole** (prefix 00E): Name, DeveloperName, ParentRoleId → UserRole,
  RollupDescription, plus the access-for-account-owner fields
  (`ContactAccessForAccountOwner`, `OpportunityAccessForAccountOwner`,
  `CaseAccessForAccountOwner`). Only the Contact field is enforced now;
  the others are stored and marked not enforced.
- **User.UserRoleId** → UserRole. Optional, as on Salesforce. A user with no
  role sees no one else's records through the hierarchy.
- **API:** UserRole is createable, updateable and deletable over REST by
  Manage Users, as the Object Reference allows. Deleting a role with users or
  child roles fails (Task 1 confirms the code). A parent change that would
  form a cycle is refused.
- **Hierarchy access:** a user gets the same access to a record as any user
  below them in the hierarchy has through ownership, manual shares and
  rules. Implemented through the visible-owner set (owners in subordinate
  roles) and the role-and-subordinates groups in the share-principal set.

### Groups

- **Group** (00G): Name, DeveloperName, Type (`Regular`, `Role`,
  `RoleAndSubordinates`, `RoleAndSubordinatesInternal` (Winter '26),
  `Organization`), RelatedId, DoesIncludeBosses (stored, not enforced). Each
  role gets its Role and both subordinate groups; with no portal users the two
  subordinate groups have the same members (D34-12).
  Regular groups are createable, updateable and deletable by Manage Users.
  Role and Organization groups are system-maintained and read-only, as on
  Salesforce.
- **GroupMember** (011): GroupId, UserOrGroupId (user or group, so groups
  nest). Writable for Regular groups only.
- **Organization group:** one per tenant ("All Internal Users"); every active
  user is a member.
- **Closure:** `sc_sharing.user_groups(tenant_id, user_id, group_id)` lists
  every group a user is in, directly or through nesting and role groups.
  Rebuilt for the tenant in the same transaction as any change to UserRole,
  Group, GroupMember, User.UserRoleId or User.IsActive. A cycle of nested
  groups is refused on write.
- Other Group types (Queue, Territory, Manager groups) are refused on
  create and grant nothing if they ever appear (fail closed).

### Manual shares

- **Objects:** `AccountShare` (00r: AccountId, UserOrGroupId,
  AccountAccessLevel, ContactAccessLevel, OpportunityAccessLevel,
  CaseAccessLevel, RowCause), `ContactShare` (ContactId, UserOrGroupId,
  ContactAccessLevel, RowCause), and for each custom object `<Name>__Share`
  (ParentId, UserOrGroupId, AccessLevel, RowCause). Prefixes and fields
  follow the Object Reference.
- **Storage:** one table `sc_sharing.shares(tenant_id, id, object_id,
  record_id, user_or_group_id, access_level, row_cause, child_levels jsonb)`,
  presented as the per-object share objects through the catalog. A share
  object exists only for an object whose OWD is not Public Read/Write, as on
  Salesforce; for other objects it isn't describable (INVALID_TYPE).
- **Levels:** `Read` or `Edit` on create, and more open than the OWD. `All`
  appears only on computed Owner rows. Edit implies Read. Inserting a share
  for an existing (record, grantee) pair updates that row (D34-06).
- **Who can share:** the owner, a user above the owner in the hierarchy, and
  Modify All / Modify All Data holders, as documented. Others get
  `INSUFFICIENT_ACCESS_OR_READONLY` (Task 1 confirms).
- **RowCause:** `Manual` on API inserts; anything else is refused on insert.
  Reads also return computed `Owner` rows (level All) and `Rule` rows, so
  `SELECT … FROM AccountShare WHERE AccountId = …` looks as it does on
  Salesforce. Rows for implicit causes aren't returned (logged as a gap).
- **Update and delete:** Manual rows only. Delete of the computed rows is
  refused.

### Sharing rules (owner-based)

- A rule: object, source group, target group, access level (Read or Edit;
  for Account, plus child levels for Contact). Records owned by any member of
  the source group are shared with every member of the target group.
- **Storage:** `sc_sharing.owner_rules`. Managed with
  `salescompel sharing rule add|list|remove` until Plan 35 deploys
  `SharingRules` metadata into the same table. Salesforce doesn't expose
  sharing rules as sObjects, so there is no REST surface.
- **Evaluation:** folded into the visible-owner set per object and level:
  owners who are in a source group of a rule whose target group the user is
  in.
- **Criteria-based rules** are accepted by Plan 35 as stored-not-enforced and
  grant nothing (fail closed). Plan 34 has no way to create them.

### Implicit sharing (Account and Contact)

- **Contact ControlledByParent:** a contact is visible or editable exactly
  when its Account is. A contact with no Account is visible only to its
  owner and View/Modify All holders, not through the hierarchy, shares or
  rules, as on Salesforce (private contacts, D34-23).
- **Account owner's access to children:** the Account owner's access to
  Contacts under it follows the owner's role setting
  `ContactAccessForAccountOwner` (Read or Edit) when Contact OWD is Private.
- **Implicit parent:** a user who reaches a Contact through ownership, the
  hierarchy, a share or a rule gets Read on its Account. View All / Modify All
  on Contact don't count (D34-21).
- Opportunity and Case join this list in their own plans.
- Master-detail custom objects: the detail's access is the master's.
  Lookups give no implicit access.

### Enforcement

- **The access predicate.** For a user U, object O and level L (Read, Edit,
  Delete), a record R is admitted when any of these holds:
  1. U has View All Data (Read) or Modify All Data (any level), or View All
     / Modify All on O.
  2. O's OWD is at least L (Read or Edit; never Delete, which needs full
     access).
  3. R's owner is U or in U's visible-owner set for O and L (subordinates
     through roles, and owners matched by rules).
  4. A share row on R names U or a group in U's closure, at level L or
     above. Delete needs owner-level access, so only 1 and 3 apply to it.
  5. O is ControlledByParent and R's parent is admitted at L.
  6. Implicit parent: L is Read, O is Account, and a child Contact of R
     is admitted for U at Read.
  Plan 33's object and field permissions still gate first. Sharing never
  widens CRUD: a Read share on an object without Edit permission doesn't
  allow edits.
- **Sharing context:** at request start, alongside the effective
  permissions, the store loads the user's group closure and, per object on
  first use, the visible-owner arrays. Cached by (tenant, user, sharing
  revision). Every write to UserRole, Group, GroupMember, rules, OWDs,
  `User.UserRoleId` or `User.IsActive` bumps the sharing revision in the same
  transaction. Owner changes and share rows don't, because the predicate reads
  them live.
- **SOQL:** the compiler adds the predicate to the root `WHERE`, child
  subqueries, semi-join and anti-join subqueries, and each parent join.
  - A parent the user can't see behaves as null on that path: its fields are
    null in results, and a filter on them treats them as null (Task 1
    confirms; documented as Salesforce's behaviour for lookups the user
    can't follow).
  - `COUNT()` and aggregates count admitted rows only.
  - **Query locators** captured IDs are re-filtered when a later page is
    fetched, so a record that lost visibility since capture is dropped (fail
    closed). Logged as a possible difference in page sizes.
  - `WITH SECURITY_ENFORCED`, `WITH USER_MODE` and `WITH SYSTEM_MODE`:
    sharing applies in every mode over the API, as it does for API callers
    on Salesforce. `SYSTEM_MODE` grants nothing extra (stored, not
    enforced, as in Plan 33).
- **REST, Composite and Collections:**
  - Read of a hidden record: 404 `NOT_FOUND`, exactly as an unknown ID
    (anti-goal: no existence leak).
  - Update or delete of a readable but not editable record:
    `INSUFFICIENT_ACCESS_OR_READONLY` (Task 1 confirms status and message).
  - Update or delete of a hidden record: as an unknown ID.
  - Collections retrieve returns `null` in place for hidden IDs, as for
    missing ones.
  - Upsert by external ID matches only admitted records. A match on a hidden
    record is a duplicate-value error, not an update, as documented (Task 1
    confirms).
- **Lookups:** setting a lookup to a record the user can't read fails as an
  unknown ID (`INVALID_CROSS_REFERENCE_KEY`), as on Salesforce.
- **Setup objects** keep Plan 33's rules. User records stay readable by
  every user in the tenant (Salesforce's User sharing default, Public Read
  Only, with User OWD not configurable here).
- **Fail closed:** an OWD value, group type, row cause or rule the candidate
  can't evaluate grants nothing. A share to an inactive user or deleted
  group grants nothing.

### Migration

- Adds `owner_id` to every existing record table. Backfill: the tenant's
  default owner (oldest active System Administrator). A tenant with no such
  user takes its oldest active user. A tenant with no users at all keeps the
  column nullable until its first user is created, then it is backfilled
  (the builder decides if this case exists in practice; the upgrade test
  covers it either way).
- Creates `sc_sharing` with `object_settings`, `roles`, `groups`,
  `group_members`, `user_groups`, `shares`, `owner_rules` and `revisions`,
  all tenant-scoped with RLS forced, and grants for the records login.
- Adds `user_role_id` to `sc_setup.users`.
- Sets Account and custom objects to Public Read/Write and Contact to
  Controlled by Parent, and creates each tenant's Organization group.
- Registers the new setup objects and share objects in each tenant catalog.
- The upgrade test runs the migration on a Plan 33 database with data and
  shows every user's visible records are unchanged.

### Tests and evidence

- **Unit:** the predicate built for each OWD, level and object kind;
  closure rebuild for nested groups, role moves and deactivation; cycle
  refusal; transfer rules.
- **Matrix:** extend Plan 33's `TestAccessMatrixNoUnexpectedGrants` with a
  sharing matrix: users in a three-level role tree (two branches), public
  groups, each OWD for Account, Contact and a custom object, manual shares,
  rules, implicit parent, and the View/Modify All bypasses. For each user and
  record it checks SOQL, aggregate count, REST GET, PATCH, DELETE,
  Composite and Collections retrieve against the expected answer computed
  independently in the test. The test fails on any unexpected grant or
  denial.
- **Leak tests:** a hidden record isn't distinguishable from a missing one by
  status, code, message or count.
- **Concurrency:** a role move and a query in parallel never see a
  half-built closure.
- **Size:** a tenant with 1,000 users in a 6-level tree and 100,000
  Accounts: Private-OWD SOQL stays within a stated budget (the builder
  records it).
- **Acceptance (unmodified clients):** `candidate/sharing/matrix` with
  JSforce and simple-salesforce as users in different roles: create as one,
  query as another, share manually through `AccountShare`, transfer
  ownership, and check visibility after each step. The browser leg uses the
  connected-browser harness to sign in as two users and list records; if
  Plan 36 hasn't landed, it uses the existing preview pages, and the gap is
  logged.

## Left to the builder

- Exact table layouts and index choices, within the predicate's needs.
- Whether visible-owner arrays are parameters or a temporary table, chosen
  on the size test.
- Error codes and messages, from Task 1 or the documentation.
- The task split, as long as the migration, `OwnerId` and the predicate land
  before OWDs other than Public Read/Write can be set.

## Calls made in this design (check while reading)

- Plan 34 takes `OwnerId` from Plan 30's scope (Scope boundary above).
- Sharing is evaluated at query time over a maintained group closure, not
  with materialized share rows. Changes are effective immediately; Salesforce
  recalculates some changes asynchronously.
- Computed Owner and Rule rows appear in share-object queries; implicit
  rows don't.
- Tenants start with Salesforce's new-org defaults (Account and custom
  objects Public Read/Write, Contact Controlled by Parent), so the upgrade
  changes nobody's access.
- Static-token principals own nothing; their records go to the default owner.
- Owner-based sharing rules are managed by an operator command until Plan 35.
- OWDs are set by an operator command until a setup UI exists.
- "Grant Access Using Hierarchies" is always on.
- Account transfer moves the old owner's Contacts under it (pending Task 1).
- Query locator pages drop records that lost visibility after capture.
- A hidden parent behaves as null in relationship paths.
- Deactivation keeps the user's role and memberships but removes them from
  the closure and deletes manual shares to them.
- Transfer Record is stored, not enforced.
- Task 1 record: `docs/compat/plan-34-sharing.md` (D34-01 to D34-31).
