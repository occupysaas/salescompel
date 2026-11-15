# SalesCompel — Design: Plan 33, users, profiles and permission sets

Status: draft for maintainer approval (2026-11-14). Issue #23. Brief:
`docs/paths/plan-33-users-profiles-permissions.md`.

## Intent

SalesCompel's access control is a placeholder. Every user is `admin` or
`restricted` (`public.users.role`, Plan 26). The entire permission check is one
line that hides `Account.Compat_Text__c` from restricted users
(`internal/policy/context.go:29`, duplicated in
`internal/records/validation.go:21`). Plan 33 replaces it with the Salesforce
model:
- users, each with one profile, plus any number of permission sets
- object and field permissions, and a small set of system permissions
- the same checks in every API path
- per-user and org-default time zone, locale and language

Plans 34 (sharing), 35 (metadata) and 36 (application shell) build on it.

The maintainer chose to manage permissions **through the standard API, as on
Salesforce**. Profiles, permission sets, grants and assignments are records
that unmodified clients read and write, so `sf org assign permset` and JSforce
work. Plan 35 later deploys Profile and PermissionSet metadata into the same
storage. Where Salesforce has a behaviour, SalesCompel follows it ("keep it
similar to what Salesforce does"). Where it isn't observable, the roadmap rule
applies: follow the public documentation, log the gap, and confirm on the
trial org in Task 1.

## Goals

- `User`, `Profile`, `PermissionSet`, `ObjectPermissions`,
  `FieldPermissions`, `PermissionSetAssignment` and `Organization` are
  standard objects. They are queryable through SOQL, including relationships,
  describable, and readable over REST and Composite.
- Users with Manage Users can create and edit users, permission sets, grants
  and assignments over REST and Composite.
- A user's effective access is the union of their profile's permissions and
  every assigned permission set. It is enforced in describe, SOQL, REST reads
  and writes, and Composite. Bulk (Plan 32) inherits it through the one write
  path and query engine.
- Anything that can't be evaluated denies access.
- The org has default time zone, locale and language. Each user has their own.
  SOQL date literals use the running user's time zone.
- Existing deployments keep today's behaviour after the migration.

## Non-goals

- Sharing, roles, record ownership rules and record-level access (Plan 34).
  "View all" and "modify all" object permissions are stored and returned, but
  record visibility stays tenant-wide until Plan 34.
- Metadata deploy and retrieve of Profile and PermissionSet (Plan 35).
- A setup UI (Plan 36).
- Permission set groups, muting permission sets, permission set licenses, user
  licenses, login hours, login IP ranges, delegated administration and
  session-based permission sets.
- Using locale and language for formatting. They are stored and returned, and
  the UI uses them in Plan 36.
- `convertTimezone()` and time-zone-aware date functions. `DAY_ONLY()` and the
  other date functions stay in UTC, as on Salesforce.

## Anti-goals

- **An unexpected grant:** any user seeing or changing an object or field that
  their profile and permission sets don't allow.
- **A permission returned as granted that isn't enforced,** without being
  marked as stored-not-enforced.
- **Security data writable** by a user without Manage Users or Modify All Data.
- **Credentials reachable** through the `User` object or the records database
  login.
- **Today's users losing access** they have now, or gaining access they don't
  have, because of the migration.

## Constraints

- CLEAN_ROOM.md: public interfaces and documentation only. Each new endpoint
  and field in `api/candidate-openapi.yaml` carries `x-source`.
- Roadmap plan shape (§1). Task 1 is observation on the trial org, or
  documentation with the gap logged. Storage changes go through a
  forward-only migration with an upgrade test. Black-box tests use
  unmodified clients and go into the acceptance script.
- **Plan 26's split of database logins stays.** The records login must never
  read `public.users`, which holds password hashes. The #18 test and
  `runtime_privileges.go` assert this. The identity login holds credentials.
- The migration number is taken at merge time (`docs/paths/README.md`).
- No source file over 500 lines.

## Approaches considered

1. **Catalog-registered standard objects (chosen).**
   - The permission objects get dedicated tables in a protected schema and
     are registered in the tenant catalog like Account. SOQL, describe, REST,
     Composite and relationship queries reach them through Plan 29's engine.
   - Writes go through the one record write path, with setup-object rules
     (below).
   - Enforcement reads a cached effective-permission set carried in the
     execution context.
2. **Hand-written endpoints per setup object.** This isolates security data
   and gives full control. But SOQL over these objects, relationship queries,
   describe and Composite would all be re-implemented, and clients query these
   objects with SOQL. Rejected as a duplicate of Plan 29.
3. **PostgreSQL row and column grants per user.** Rejected. Grants would churn
   with dynamic objects and fields, and it can't produce Salesforce's errors
   (a hidden field gives `INVALID_FIELD`).

## Design

### Objects

All objects are tenant-scoped, with RLS forced like other business tables.
ID prefixes follow Salesforce.

| Object | Prefix | Fields (beyond Id) | API access |
|---|---|---|---|
| Organization | 00D | Name, TimeZoneSidKey, DefaultLocaleSidKey, LanguageLocaleKey, DefaultCurrencyIsoCode, plus today's discovery fields | read-only, one row per tenant |
| User | 005 | Username, FirstName, LastName, Name, Alias, Email, IsActive, ProfileId → Profile, TimeZoneSidKey, LocaleSidKey, LanguageLocaleKey, EmailEncodingKey | create and update by Manage Users. No delete (deactivate with IsActive = false, as on Salesforce). |
| Profile | 00e | Name, Description, PermissionsModifyAllData, PermissionsViewAllData, PermissionsApiEnabled, PermissionsManageUsers | read-only (create waits for Plan 35, as Salesforce doesn't create profiles over REST) |
| PermissionSet | 0PS | Name, Label, Description, IsOwnedByProfile, ProfileId → Profile, the system permission booleans | create, update and delete by Manage Users. Profile-owned sets can be updated but not created or deleted. |
| ObjectPermissions | 110 | ParentId → PermissionSet, SobjectType, PermissionsRead, PermissionsCreate, PermissionsEdit, PermissionsDelete, PermissionsViewAllRecords, PermissionsModifyAllRecords | create, update and delete by Manage Users |
| FieldPermissions | 01k | ParentId → PermissionSet, SobjectType, Field (e.g. `Account.Phone`), PermissionsRead, PermissionsEdit | create, update and delete by Manage Users |
| PermissionSetAssignment | 0Pa | AssigneeId → User, PermissionSetId → PermissionSet | create and delete by Manage Users |

These objects and their relationship names follow the public field
reference. Child relationships: `User.PermissionSetAssignments`,
`PermissionSet.Assignments`, `PermissionSet.ObjectPerms` and
`PermissionSet.FieldPerms`, as named in the reference.

**Each profile owns exactly one permission set** (`IsOwnedByProfile = true`).
Editing a profile's grants means writing ObjectPermissions and
FieldPermissions against that set, as on Salesforce. Each user has one
system-maintained assignment to their profile's set (see Consistency rules).
It can't be created, moved or deleted through the API.

**System permissions:**
- Four are enforced: Modify All Data, View All Data, API Enabled and Manage
  Users.
- Further `Permissions*` booleans that the field reference lists for
  PermissionSet and Profile may be exposed, so that clients that send them
  don't fail. They are stored, marked not enforced in describe and the
  OpenAPI document, and **grant nothing**.
- Task 1 decides which extra ones to expose.

### Consistency rules (write path, setup objects)

These follow Salesforce's documented dependencies. Task 1 confirms the exact
error codes.
- **Object permissions:**
  - Create, Edit or Delete requires Read.
  - Delete requires Edit.
  - View All Records requires Read.
  - Modify All Records requires all of the others.
  - A write that breaks these is rejected. It is never silently corrected.
- **Field permissions:**
  - Edit requires Read.
  - The field must exist and be permissionable.
- **Fields that can't be permissioned:** Id, required (non-nillable) fields,
  name fields and system fields. They are always readable when their object
  is readable. They can't get a FieldPermissions row, matching Salesforce,
  which won't let required fields be hidden.
- **Uniqueness:** one ObjectPermissions row per (ParentId, SobjectType), one
  FieldPermissions row per (ParentId, Field), and one assignment per
  (AssigneeId, PermissionSetId).
- **Unknown names:** SobjectType and Field must name objects and fields in the
  tenant catalog. Rows for objects or fields that are later deleted grant
  nothing and are removed with them.
- **Every user must keep a profile.** ProfileId is required and must exist.
- **Each user's profile-owned permission set has a PermissionSetAssignment
  row,** as on Salesforce. Clients filter these out with
  `PermissionSet.IsOwnedByProfile`.
  - The system maintains these rows. It creates the row with the user and
    moves it when ProfileId changes.
  - The API can't create or delete them directly.
  - The effective-permission union reads assignments uniformly, so profile
    permissions arrive through the same row.
- **Users can't lock themselves out:** a Manage Users user can't remove Manage
  Users or Modify All Data from the last active user who has it. Task 1 checks
  the Salesforce behaviour. Without evidence, this is a recorded SalesCompel
  safety rule.

### Enforcement

- **Effective permissions:**
  - The union of the profile-owned set and every assigned set: object
    permissions per object, field permissions per field, and system
    permission booleans.
  - Modify All Data grants every object permission and edit on every
    permissionable field.
  - View All Data grants read on every object and field.
- **Caching:** computed once per (tenant, user, permission revision) and
  carried in the execution context. Each tenant has a permission revision
  that every setup-object write, and every change to a user's ProfileId or
  IsActive, bumps in the same transaction. Cached sets for older revisions are
  never used.
- **Fail closed:** a user with no valid profile, an inactive user, a missing
  permission set or an unknown object name all mean no access.
- **API Enabled** is required for every REST and Composite call made with a
  token issued to a connected app or the `PlatformCLI` client. Without it, the
  call fails with Salesforce's error for API access not enabled (Task 1
  confirms the code). As on Salesforce, it doesn't restrict the product's own
  UI:
  - Requests from the first-party web UI session are exempt.
  - Plan 36's sign-in creates that session. This plan defines the exemption
    point and tests it with a stand-in session marker.
- **Objects:**
  - Read is required to describe an object, list it in the global describe,
    use it in SOQL `FROM`, traverse a relationship to it, run a child subquery
    on it, and read records over REST.
  - Create, Edit and Delete gate the matching writes.
  - A hidden object behaves as if it doesn't exist (Salesforce's
    `INVALID_TYPE` / not found).
- **Fields:**
  - Read is required to appear in describe, SOQL `SELECT`, `WHERE`,
    `ORDER BY` and `GROUP BY`, and REST record reads.
  - Edit is required to set the field on create or update.
  - A hidden field behaves as if it doesn't exist (`INVALID_FIELD`, or
    Salesforce's insert/update equivalent).
  - `WITH SECURITY_ENFORCED` and `WITH USER_MODE` (parsed since Plan 29) now
    act on real permissions.
- **Setup objects:** reading Profile, PermissionSet, ObjectPermissions,
  FieldPermissions and PermissionSetAssignment needs Manage Users or View All
  Data. Writing needs Manage Users or Modify All Data. Every user can read
  User records in their tenant, as on Salesforce before sharing.
- **One place to check:** `policy.Allowed` becomes a lookup in the effective
  set, and the duplicate in `internal/records/validation.go` is removed. The
  hard-coded `Role` checks in `policy.ExecutionContext.Validate` and the
  platform store give way to the profile model. The runtime decides nothing
  from `Role`.

### User records and the identity split

- **Two tables per user:**
  - Non-secret User fields live in a new setup table readable by the records
    login.
  - Credentials stay in `public.users` under the identity login.
  - Both rows share one ID, and the username is identical in both. Usernames
    stay unique across tenants, ignoring case (Plan 26).
- **IsActive and ProfileId are owned by the setup table.**
  - The identity login gets `SELECT` on exactly the columns that token and
    login checks need (ID, tenant, IsActive), and `runtime_privileges.go` is
    updated to allow precisely that.
  - Token validation and login join on it, so deactivating a user over REST
    stops their tokens at once.
- **Identity rows are written through narrow database functions.** The
  records login never reads or writes `public.users` directly.
  - Two `SECURITY DEFINER` functions are owned by the owner group, and the
    records login gets `EXECUTE` on them:
    - one creates a credential row with no usable password (login refused)
    - one renames a username
  - The functions are the only path from a REST User write to `public.users`.
  - They enforce cross-tenant, case-insensitive username uniqueness and
    return Salesforce's duplicate-username error. They run in the same
    transaction as the setup-row write, so both commit or neither does.
  - Username is updatable over REST, as on Salesforce, through the rename
    function.
  - `runtime_privileges.go` and the #18 privilege test allow exactly these
    `EXECUTE` grants and nothing more.
- **A user created over REST has no password.** They can't log in until the
  operator runs `salescompel user set-password`. Salesforce sends a welcome
  email at this point, and email sending isn't available. Recorded as a gap.
- **Static-token principals** (`salescompel provision`, `public.principals`)
  have no User row. Their stored `role` maps to a seeded profile's effective
  permissions when the token is checked (`admin` → System Administrator,
  `restricted` → Standard User), and they use the org-default time zone. So
  existing provisioned tokens keep exactly today's access. They can't be
  managed over the API. A later plan may retire `provision`. Recorded in the
  compatibility doc.
- **CLI changes:**
  - `salescompel user create` takes `SALESCOMPEL_PROFILE` (a profile name).
    It fills time zone, locale and language from the org defaults.
  - `SALESCOMPEL_ROLE` stays accepted for one release, mapping `admin` to
    System Administrator and `restricted` to Standard User, with a
    deprecation notice.
- **Required fields on REST create** follow Salesforce: Username, LastName,
  Alias, Email, ProfileId, TimeZoneSidKey, LocaleSidKey, LanguageLocaleKey and
  EmailEncodingKey. There's no org-default fill over the API, as on
  Salesforce, where the default applies to users made in the UI.

### Time zone, locale and language

- **Org defaults:**
  - New columns next to `currency_iso_code` in `sc_catalog.settings`:
    `time_zone_sid_key`, `default_locale_sid_key` and `language_locale_key`.
  - Returned on `Organization`, which becomes a real catalog object and
    replaces the string-matched discovery special case in `http.go`.
  - Organization is read-only over the API, as on Salesforce, where these are
    set on the Company Information page. Until Plan 36 has a setup page, the
    operator changes them with `salescompel org set` (environment values, like
    `user create`).
  - Changing the defaults doesn't change existing users.
- **Picklists:**
  - TimeZoneSidKey values are the IANA zone IDs Salesforce accepts. Task 1 or
    the public reference gives the list. Every value must load with Go's
    `time.LoadLocation`.
  - LocaleSidKey, LanguageLocaleKey and EmailEncodingKey values come from the
    same sources.
- **SOQL date literals:**
  - `TODAY`, `LAST_N_DAYS:n`, `THIS_MONTH` and the rest resolve in the running
    user's time zone. For a date-time field, the range runs from local
    midnight to local midnight, compared as instants. For a date field, the
    local calendar date is used.
  - `DateLiteralRange` (`internal/soql/dates.go`) takes a location instead of
    assuming UTC. The user's location travels in the execution context and
    `soql.Options`.
  - Results stay in UTC.
  - This closes the "user time zones" gap in the Plan 29 compatibility doc.

### Seeded profiles and migration

- **Every tenant gets four profiles:**
  - **System Administrator:** Modify All Data, View All Data, API Enabled,
    Manage Users.
  - **Standard User:** API Enabled. Read, create, edit and delete on Account,
    Contact and every custom object existing at migration time. Read and edit
    on every permissionable field, except that `Account.Compat_Text__c` gets
    no grant.
  - **Read Only:** API Enabled. Read on Account, Contact and existing custom
    objects, and read on their permissionable fields.
  - **Minimum Access:** no permissions. Task 1 checks whether Salesforce
    grants API Enabled here. If it does, follow it.
- **Users:** `admin` becomes System Administrator and `restricted` becomes
  Standard User. That keeps today's behaviour exactly: restricted users see
  everything except `Account.Compat_Text__c`.
- **Objects and fields created after migration** (Plan 27 dynamic schema) get
  no grant on any profile or permission set. Only Modify All Data and View
  All Data reach them, as with Salesforce's metadata API default. Admins grant
  access through ObjectPermissions and FieldPermissions.
- **Rollback means restoring a backup.** The previous binary refuses a
  database at a newer migration head (`migration.checked`) and rejects the new
  tables (`runtimeTablesSafe`), so nothing is kept for running old binaries.
  The `users.role` column stays for `provision` principals' sake only until a
  later plan. It isn't synchronised for new users.
- **The upgrade test** migrates a database with both roles and existing custom
  objects, then checks the access matrix is unchanged.

### Tests and evidence

- **Unit tests:** effective-permission union, the consistency rules,
  fail-closed cases, cache invalidation on revision, and date literals in
  several zones, including a DST boundary.
- **An access-matrix test** over every object and permissionable field ×
  each seeded profile plus a custom permission set. It asserts the exact
  allowed set, so there are no unexpected grants.
- **Real clients, added to `scripts/check-candidate-acceptance.sh`:**
  - **`sf` CLI:** `sf org assign permset` grants a custom permission set to a
    Standard User, after which a previously hidden field is queryable.
    `sf data query` as Standard User gets `INVALID_FIELD` on
    `Account.Compat_Text__c`.
  - **JSforce:** creates a permission set with object and field permissions,
    then describes as the restricted user.
  - **simple-salesforce:** a Read Only user is refused on create.
- **A time zone check:** a user in a non-UTC zone gets the right `TODAY`
  boundary.
- **Compatibility doc:** `docs/compat/plan-33-users-profiles-permissions.md`
  with decisions, gaps and evidence. Gaps include the welcome email,
  unenforced permissions, groups and muting, and locale formatting.

## Left to the builder

- Table and schema names for the setup objects, and how they register in the
  catalog seed (`internal/metadata/seed.go`).
- The cache's shape and where it lives, within the revision rule.
- Exact error codes and messages, from Task 1 or the documentation.
- The picklist value lists, from Task 1 or the public reference.
- The task split and order, as long as the migration and upgrade test land
  before enforcement is switched on.

## Calls made in this design (check while reading)

- Organization is read-only over the API, and org defaults are changed with an
  operator command until Plan 36. This follows Salesforce, where they're set
  in Setup, not over the API.
- REST user creation requires time zone, locale and language, as Salesforce
  does. The org-default fill applies only to the CLI, which stands in for
  Salesforce's UI.
- Profiles are read-only over REST, and their grants are edited through their
  owned permission set.
- Users are never deleted, only deactivated.
- New objects and fields after migration are granted to nobody but Modify All
  Data and View All Data holders.
- Last-admin lockout protection is added unless Task 1 shows Salesforce
  behaves differently.
- `SALESCOMPEL_ROLE` stays accepted for one release. Rollback is a backup
  restore, not an old binary.
- Profile-owned permission sets get system-maintained PermissionSetAssignment
  rows, as on Salesforce.
- API Enabled gates connected-app and CLI tokens, not the first-party UI
  session, as on Salesforce.
- REST User writes reach `public.users` only through two `SECURITY DEFINER`
  functions (create without a password, rename).
- Provisioned static tokens map their role to a seeded profile, keeping
  today's access.
- Date-literal weeks start on Sunday unless Task 1 shows locale-dependent
  behaviour for the reference locale (the builder logs it).
- Migrated users get name, alias and email backfilled from their username.
- Setup objects can't be targets of ObjectPermissions or FieldPermissions:
  access to them comes only from Manage Users, View All Data and Modify All
  Data.
