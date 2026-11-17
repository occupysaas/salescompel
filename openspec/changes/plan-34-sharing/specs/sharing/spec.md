# Spec: sharing

## Purpose

Give every record an owner and decide record-level access by Salesforce's
documented sharing model, enforced on every read and write path through the
one query engine and the one record write path.

## ADDED Requirements

### Requirement: Record owner
<!-- id: shr-01 -->
Account, Contact and every custom object SHALL have `OwnerId`, a reference to
`User` with relationship name `Owner`, createable, updateable and not
nillable. On create it SHALL default to the running user. A given owner SHALL
be an active user in the tenant with Read on the object; a reference to a
Group or queue SHALL be refused. A static-token principal SHALL create records
owned by the tenant's default owner, the oldest active System Administrator.

#### Scenario: default owner
- **WHEN** a user creates an Account without `OwnerId`
- **THEN** the record's `OwnerId` is that user and `SELECT Owner.Name` returns their name

#### Scenario: invalid owner
- **WHEN** a create names an inactive user, a user without Read on the object, or a Group as owner
- **THEN** the create fails and nothing is written

### Requirement: Ownership transfer
<!-- id: shr-02 -->
An update that changes `OwnerId` SHALL be allowed only for the owner, a user
above the owner in the role hierarchy, holders of Modify All on the object or
Modify All Data. The new owner SHALL have Read on the object. Transfer Record
SHALL be stored and not enforced. A transfer SHALL delete the record's manual shares in the same
transaction. An Account transfer SHALL move Contacts under it that the old
owner owned, unless Task 1 shows Salesforce does not.

#### Scenario: manual shares removed
- **WHEN** a shared Account is transferred
- **THEN** its Manual share rows are gone and their grantees lose access in the same transaction

### Requirement: Org-wide defaults
<!-- id: shr-03 -->
Each object SHALL have an internal OWD: `None` (Private), `Read`, `Edit`, or
`ControlledByParent` where allowed (Contact; master-detail details always). A
child's OWD SHALL NOT be more open than its parent's where Salesforce pairs
them. New and migrated tenants SHALL start with Account and custom objects at
`Edit` and Contact at `ControlledByParent`. OWDs
SHALL be set with `salescompel sharing set-default`, take effect immediately,
and be returned read-only as `Organization.DefaultAccountAccess` and
`Organization.DefaultContactAccess`.

#### Scenario: Private OWD
- **WHEN** Account OWD is Private and user B has no role, share or rule reaching user A's Account
- **THEN** B cannot see it in SOQL, REST or Composite

### Requirement: Role hierarchy
<!-- id: shr-04 -->
`UserRole` (prefix 00E) with `ParentRoleId` and `User.UserRoleId` SHALL be
available through the API, writable by Manage Users. A parent change forming a
cycle, and deleting a role with users or child roles, SHALL be refused. A user
SHALL have the same access to a record as any user below them in the
hierarchy has through ownership, manual shares and rules. A user with no role
SHALL gain nothing through the hierarchy.

#### Scenario: manager sees subordinate's records
- **WHEN** A's role is the parent of B's role and Account OWD is Private
- **THEN** A sees and can edit Accounts B owns, and B does not see A's

### Requirement: Groups and closure
<!-- id: shr-05 -->
`Group` (00G) of type `Regular`, `Role`, `RoleAndSubordinates`,
`RoleAndSubordinatesInternal` and `Organization`, and `GroupMember` (011), SHALL be available through the API.
Regular groups and their members SHALL be writable by Manage Users; the other
types SHALL be system-maintained and read-only. Nested group cycles SHALL be
refused. Each tenant SHALL have one Organization group containing every active
user. The tenant's user-to-group closure SHALL be rebuilt in the same
transaction as any change to roles, groups, members, `User.UserRoleId` or
`User.IsActive`, and no query SHALL observe a partial closure. Other group
types SHALL be refused on create and SHALL grant nothing.

#### Scenario: nested public group
- **WHEN** group G1 contains group G2 and user U is in G2
- **THEN** a share to G1 grants U access

### Requirement: Manual shares
<!-- id: shr-06 -->
`AccountShare`, `ContactShare` and `<Object>__Share` SHALL exist for objects
whose OWD is not `Edit`, and SHALL be unknown objects otherwise. Inserts SHALL
take `RowCause` `Manual` and level `Read` or `Edit`, and SHALL be allowed only
for the owner, a user above the owner, and Modify All / Modify All Data
holders. Queries SHALL return stored Manual rows and computed `Owner` (level
`All`) and `Rule` rows. Only Manual rows SHALL be updatable or deletable.

#### Scenario: non-owner share refused
- **WHEN** a user who is not the owner, above the owner, or a Modify All holder inserts an AccountShare
- **THEN** the insert fails with `INSUFFICIENT_ACCESS_OR_READONLY` (Task 1 confirms) and nothing is written

### Requirement: Owner-based sharing rules
<!-- id: shr-07 -->
An owner-based rule SHALL share records of one object owned by members of a
source group with members of a target group at `Read` or `Edit`. Rules SHALL be
managed with `salescompel sharing rule add|list|remove` and stored where
Plan 35 can deploy `SharingRules`. Criteria-based rules SHALL grant nothing.

#### Scenario: rule grants read
- **WHEN** a rule shares Accounts owned by role group R1 with public group G at Read
- **THEN** members of G see those Accounts but cannot edit them

### Requirement: Implicit sharing
<!-- id: shr-08 -->
A Contact with `ControlledByParent` SHALL be visible or editable exactly when
its Account is; a Contact with no Account SHALL be visible only to its owner
and View All / Modify All holders. With Contact OWD Private, an Account
owner's access to Contacts under it SHALL follow their role's
`ContactAccessForAccountOwner`. A user who reaches a Contact through
ownership, the hierarchy, a share or a rule SHALL have Read on its Account;
View All / Modify All on Contact SHALL NOT count. A master-detail detail SHALL take its master's access; lookups
SHALL give no implicit access.

#### Scenario: implicit parent read
- **WHEN** user U sees a Contact through a manual share but has no access to its Account otherwise
- **THEN** U can read the Account and cannot edit it

### Requirement: Bypasses and permission order
<!-- id: shr-09 -->
View All Data and object View All SHALL grant Read on every record of the
object; Modify All Data and object Modify All SHALL grant every level. Plan 33
object and field permissions SHALL be checked first; sharing SHALL never widen
CRUD.

#### Scenario: Edit share without Edit permission
- **WHEN** a user has an Edit share on an Account but no Edit permission on Account
- **THEN** an update fails as it did under Plan 33

### Requirement: SOQL enforcement
<!-- id: shr-10 -->
Every compiled query SHALL apply the access predicate to the root object,
child subqueries, semi-join and anti-join subqueries and each parent join.
A parent the user cannot read SHALL be null in results and in filters.
`COUNT()` and aggregates SHALL count admitted rows only. Query-locator pages
SHALL re-apply the predicate. Sharing SHALL apply in every `WITH` mode.

#### Scenario: hidden parent
- **WHEN** a user queries `SELECT Name, Account.Name FROM Contact` and can see a Contact but not its Account
- **THEN** `Account` is null for that row and `WHERE Account.Name = 'X'` does not match it

### Requirement: Write and read enforcement
<!-- id: shr-11 -->
REST, Composite and Collections SHALL answer a hidden record exactly as an
unknown ID (status, code, message, Collections retrieve `null`). Update or
delete of a readable record without the needed level SHALL fail with
`INSUFFICIENT_ACCESS_OR_READONLY` (Task 1 confirms). Delete SHALL need
owner-level access (owner, hierarchy, Modify All). A lookup to a record the
user cannot read SHALL fail as an unknown ID. Upsert by external ID SHALL match
only admitted records.

#### Scenario: hidden equals missing
- **WHEN** a user GETs, PATCHes or DELETEs a hidden record and an unknown ID with the same prefix
- **THEN** the responses differ only in the ID

### Requirement: Fail closed
<!-- id: shr-12 -->
An OWD value, group type, row cause or rule the candidate cannot evaluate
SHALL grant nothing. A share to an inactive user or deleted group SHALL grant
nothing. Deactivating a user SHALL keep their role and GroupMember rows, remove
them from the closure, and delete the manual shares granted to them.

#### Scenario: deactivated grantee
- **WHEN** a user with a manual share is deactivated and reactivated
- **THEN** the share is gone and they have only what their role and groups give

### Requirement: Migration
<!-- id: shr-13 -->
A forward-only migration SHALL add `owner_id` to every record table, backfilled
to the tenant's default owner; create the sharing storage with RLS forced;
add `user_role_id` to users; set Account and custom objects to `Edit` and
Contact to `ControlledByParent`; and create each tenant's Organization group. After it, every user's visible record set SHALL
be unchanged.

#### Scenario: upgrade keeps access
- **WHEN** a Plan 33 database with users and records is migrated
- **THEN** each user sees and can change exactly the records they could before
