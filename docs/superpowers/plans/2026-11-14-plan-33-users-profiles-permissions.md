# Plan 33: Users, profiles and permission sets Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Replace the two hard-coded roles with Salesforce's user, profile and permission-set model:
- managed through standard objects over REST and SOQL
- enforced in every API path
- with org-default and per-user time zone, locale and language

**Architecture:**
- **Storage:** setup objects live in dedicated tables in a new `sc_setup` schema, registered in the tenant catalog as standard objects. SOQL, describe, REST and Composite reach them through the existing engine.
- **Enforcement:** a pure `policy.Permissions` value (the union of a user's profile-owned set and their assigned sets) is loaded per request, cached by a tenant permission revision, and consulted wherever `policy.Allowed` is consulted today.
- **Writes:** setup-object writes go through the one record write path with extra rules. User identity rows are reached only through two `SECURITY DEFINER` functions.

**Tech Stack:** Go, PostgreSQL 17 (RLS), pgx v5, Node/TypeScript compatibility lab, JSforce, simple-salesforce, `sf` CLI.

**Spec:** `docs/superpowers/specs/2026-11-14-plan-33-users-profiles-permissions-design.md` (issue #23, brief `docs/paths/plan-33-users-profiles-permissions.md`)

## Global Constraints

- Work on `main` (maintainer rule until Plan 49). Every commit is signed off (`git commit -s`) with the session's Co-Authored-By trailer. Each task's final commit message ends with `Refs #23`. The last task's ends with `Closes #23`.
- CLEAN_ROOM.md: public documentation and trial-org observation only. Every new OpenAPI path and field carries `x-source`.
- One forward-only migration, `internal/migration/migrations/0007_users_permissions.sql`, registered in `manifest()` in `internal/migration/migrate.go` as `Entry{7, "users and permissions", …}`. Renumber at merge time if `main` has moved (`scripts/check-migrations.sh`).
- The records login never reads or writes `public.users`. Identity rows are reached only through `sc_setup.create_identity` and `sc_setup.rename_identity` (`SECURITY DEFINER`, owned by `salescompel_owner`, `EXECUTE` granted to `salescompel_records`).
- ID prefixes: User `005`, Profile `00e`, PermissionSet `0PS`, PermissionSetAssignment `0Pa`, ObjectPermissions `110`, FieldPermissions `01k`, Organization `00D`.
- Enforced system permissions: exactly `PermissionsModifyAllData`, `PermissionsViewAllData`, `PermissionsApiEnabled` and `PermissionsManageUsers`. Any other `Permissions*` field is stored, marked not enforced, and grants nothing.
- Seeded profiles per tenant: `System Administrator`, `Standard User`, `Read Only`, `Minimum Access`, with grants exactly as in the spec's "Seeded profiles and migration".
- Fail closed: an unknown object or field, a missing profile, an inactive user or an unevaluable permission means no access.
- No source file over 500 lines (`sh scripts/check-file-size.sh`).
- Test database: `TEST_DATABASE_URL`, `TEST_RECORDS_URL` and `TEST_IDENTITY_URL` against PostgreSQL 17, with the `salescompel_owner`, `salescompel_records` and `salescompel_identity` group roles.

## Review Focus

- **A user deactivated while holding a valid token:** the token's next request is refused at once, with no restart and no cache delay (Task 6 test `TestDeactivatedUserTokenRefused`).
- **A permission granted or revoked while the server runs:** the next request reflects it, because the revision cache never serves a stale set (Task 4 test `TestPermissionRevisionInvalidatesCache`).
- **A field or custom object deleted after it was granted:** its FieldPermissions and ObjectPermissions rows disappear, and describe and SOQL show no trace of it (Task 6 test `TestDeletedFieldGrantRemoved`).
- **A parent traversal or child subquery to an object the user can't read:** `SELECT Account.Name FROM Contact` and `SELECT (SELECT Id FROM Contacts) FROM Account` fail exactly like a missing relationship (Task 5 test `TestHiddenObjectTraversal`).
- **A Composite request mixing allowed and denied subrequests:** with `allOrNone: true` the whole request rolls back. The denied subrequest reports the permission error, never a server error (Task 6 test `TestCompositeDeniedSubrequest`).

---

### Task 1: Observation and compatibility doc

**Files:**
- Create: `docs/compat/plan-33-users-profiles-permissions.md`

**Interfaces:**
- Produces: a "Decisions" table that later tasks cite by row ID (`D33-01`…), covering:
  - the error code and message for each denial (hidden object in SOQL and REST, hidden field in SOQL, hidden field on write, missing object permission on write, API not enabled, missing Manage Users)
  - the ObjectPermissions and FieldPermissions dependency errors
  - the required User fields on insert
  - the extra `Permissions*` fields to expose
  - whether Minimum Access has API Enabled
  - the TimeZoneSidKey, LocaleSidKey, LanguageLocaleKey and EmailEncodingKey value lists
  - the date-literal week start for `en_US`
  - last-admin behaviour

- [ ] **Step 1: Review changelogs.** Read Salesforce release notes since Plan 29's review for User, Profile, PermissionSet, ObjectPermissions, FieldPermissions, PermissionSetAssignment and date literals. Record the relevant items and their URLs.
- [ ] **Step 2: Observe or document each `D33-*` row.** Use the trial org where a session is available, under CLEAN_ROOM.md. Otherwise cite the public documentation and mark the row `documented` with the gap logged. The task never blocks the plan.
- [ ] **Step 3: Write the doc's sections:** Decisions, Gaps (welcome email, unenforced permissions, groups and muting, locale formatting, provisioned principals, password-less REST users), Evidence and Sources.
- [ ] **Step 4: Commit.** `git add docs/compat/plan-33-users-profiles-permissions.md && git commit -s -m "docs: record Plan 33 observations and decisions"` (add the trailer and `Refs #23`).

### Task 2: Permission model

**Files:**
- Create: `internal/policy/permissions.go`, `internal/policy/permissions_test.go`
- Create: `internal/policy/grants.go`, `internal/policy/grants_test.go`

**Interfaces:**
- Produces:
  - `type ObjectAccess struct{ Read, Create, Edit, Delete, ViewAll, ModifyAll bool }`
  - `type FieldAccess struct{ Read, Edit bool }`
  - `type SystemPerms struct{ ModifyAllData, ViewAllData, APIEnabled, ManageUsers bool }`
  - `type PermissionSet struct{ ID string; System SystemPerms; Objects map[string]ObjectAccess; Fields map[string]FieldAccess }`. Map keys are canonical API names: `"Account"` and `"Account.Phone"`.
  - `type Permissions struct{ … }` (unexported fields), built only by `Union(sets ...PermissionSet) Permissions`
  - `func (p Permissions) System() SystemPerms`
  - `func (p Permissions) Object(object string) ObjectAccess`. This applies Modify All Data (all true) and View All Data (Read and ViewAll).
  - `func (p Permissions) FieldRead(object string, f metadata.Field) bool` and `func (p Permissions) FieldEdit(object string, f metadata.Field) bool`. A non-permissionable field is readable when the object is readable, and editable when the object is creatable or editable and `f.Createable || f.Updateable`.
  - `func Permissionable(f metadata.Field) bool`. False for Id, non-nillable fields, name fields, generated fields and system fields.
  - `func ValidateObjectGrant(a ObjectAccess) error` and `func ValidateFieldGrant(a FieldAccess) error`. These return `contract.Invalid` with the `D33-*` code.

- [ ] **Step 1: Write the failing tests.**
  - `TestUnionGrantsAreAdditive`: union of `{Account: Read}` and `{Account: Edit, Read}` gives `Object("Account") == {Read: true, Edit: true}`.
  - `TestModifyAllDataGrantsEverything`: `Object("Anything__c")` is all true, and `FieldEdit` is true on any permissionable field.
  - `TestViewAllDataGrantsReadOnly`: Read and ViewAll are true, and Create, Edit and Delete are false.
  - `TestEmptyIsDenied`: a zero `Permissions` denies every object and every permissionable field.
  - `TestNonPermissionableFollowsObject`: Id and `Account.Name` are readable exactly when Account is readable.
  - `TestObjectGrantDependencies`: table cases Create without Read, Delete without Edit, ViewAll without Read, and ModifyAll without the others all fail. A full grant passes.
  - `TestFieldGrantDependencies`: Edit without Read fails.
- [ ] **Step 2: Run them.** `go test ./internal/policy/ -run 'Union|ModifyAll|ViewAll|Empty|NonPermissionable|GrantDependencies'` fails because the types are undefined.
- [ ] **Step 3: Implement the types and functions above.** Field keys compare case-insensitively.
- [ ] **Step 4: Run them.** `go test ./internal/policy/` passes.
- [ ] **Step 5: Commit.** `git commit -s -m "feat: add the Plan 33 permission model"`

### Task 3: Storage, migration and privileges

**Files:**
- Create: `internal/migration/migrations/0007_users_permissions.sql`, `internal/migration/users_permissions_test.go`
- Modify: `internal/migration/migrate.go` (manifest entry 7), `internal/platform/runtime_privileges.go`, `internal/migration/operator_setup_test.go` (privilege assertions)

**Interfaces:**
- Produces these tables, all with `tenant_id` and forced RLS by tenant like `sc_records`:
  - `sc_setup.users` (`id`, `tenant_id`, `username`, `username_canonical` unique globally, `first_name`, `last_name`, `alias`, `email`, `is_active`, `profile_id`, `time_zone_sid_key`, `locale_sid_key`, `language_locale_key`, `email_encoding_key`)
  - `sc_setup.profiles` (`id`, `tenant_id`, `name`, `description`, `permission_set_id`)
  - `sc_setup.permission_sets` (`id`, `tenant_id`, `name`, `label`, `description`, `is_owned_by_profile`, `profile_id`, the four enforced booleans, `extra_permissions jsonb`)
  - `sc_setup.object_permissions` (unique `tenant_id, parent_id, sobject_type`)
  - `sc_setup.field_permissions` (unique `tenant_id, parent_id, field`)
  - `sc_setup.permission_set_assignments` (unique `tenant_id, assignee_id, permission_set_id`)
  - `sc_setup.revisions(tenant_id PK, revision bigint)`
- Produces new columns `time_zone_sid_key` (default `'America/Los_Angeles'`), `default_locale_sid_key` (`'en_US'`) and `language_locale_key` (`'en_US'`) on `sc_catalog.settings`. These defaults hold unless `D33-*` gives the observed org defaults.
- Produces the functions:
  - `sc_setup.create_identity(p_id varchar, p_tenant text, p_username text) RETURNS void`. It inserts a `public.users` row with `password_hash = ''` (login refused) and `role = 'restricted'`, and raises SQLSTATE `23505` with constraint name `users_username_canonical_key` on a duplicate.
  - `sc_setup.rename_identity(p_id varchar, p_tenant text, p_username text) RETURNS void`.
  - Both are `SECURITY DEFINER`, use `SET search_path = pg_catalog, public`, and have `EXECUTE` revoked from `PUBLIC` and granted to `salescompel_records`.
- Produces grants:
  - `salescompel_identity`: `SELECT (id, tenant_id, is_active, profile_id)` on `sc_setup.users`.
  - `salescompel_records`: `SELECT, INSERT, UPDATE, DELETE` on the setup tables, with RLS.

- [ ] **Step 1: Write the failing tests** in `users_permissions_test.go`.
  - `TestMigration7SeedsProfiles`: after `Up`, each tenant has the four profiles, each with one owned permission set, and grants exactly matching the spec.
  - `TestMigration7MapsRoles`: a legacy database with an `admin` and a `restricted` user ends with System Administrator and Standard User assignments. The restricted user's union has no grant on `Account.Compat_Text__c` and has Read and Edit on `Account.Phone`. Existing custom objects get Standard User CRUD.
  - `TestRecordsLoginCannotReadCredentials`: as `TEST_RECORDS_URL`, `SELECT 1 FROM public.users` fails with SQLSTATE `42501`, and `SELECT sc_setup.create_identity(...)` succeeds.
  - `TestCreateIdentityRejectsDuplicateAcrossTenants`: a second tenant creating `Same@Example.com` raises `23505`.
  - `TestSetupTablesForceRLS`: every `sc_setup` table has `relrowsecurity` and `relforcerowsecurity`.
  - Extend the `TestFreshOperatorSetup` privilege queries: the identity group has `SELECT` on exactly the four `sc_setup.users` columns, and the records group still has no `SELECT` on `public.users`.
- [ ] **Step 2: Run them.** `go test ./internal/migration/ -run 'Migration7|RecordsLogin|CreateIdentity|SetupTables|FreshOperator'` fails.
- [ ] **Step 3: Write `0007_users_permissions.sql` and register manifest entry 7.**
  - Seed the four profiles and their owned sets for every existing tenant.
  - Copy every `public.users` row into `sc_setup.users`, backfilling `last_name`, `alias` and `email` from the username, and time zone, locale and language from the org defaults.
  - Create one profile-owned assignment row per user.
  - Extend `metadata.SeedTenant` so new tenants get the same.
- [ ] **Step 4: Update `runtimeTablesSafe`** in `runtime_privileges.go` so the exact matrix now includes the `sc_setup` tables and the identity column grant, and nothing more.
- [ ] **Step 5: Run the tests.** `go test -p 1 ./internal/migration/ ./internal/platform/ -run 'Migration|Privilege|Runtime|Operator'` passes.
- [ ] **Step 6: Commit.** `git commit -s -m "feat: add Plan 33 storage and migration"`

### Task 4: Catalog objects and loading permissions

**Files:**
- Create: `internal/metadata/seed_setup.go`, `internal/policy/load.go`, `internal/policy/load_test.go`
- Modify: `internal/metadata/seed.go` (`StandardObjects` appends `SetupObjects()`), `0007_users_permissions.sql` (catalog rows for existing tenants), `internal/policy/context.go`, `internal/platform/oauth_tokens.go`, `internal/platform/store.go`, `internal/platform/runtime.go`, `internal/platform/http.go`

**Interfaces:**
- Consumes: `Union` and `PermissionSet` (Task 2), and the tables (Task 3).
- Produces:
  - `func SetupObjects() []Object` in `internal/metadata`, with the seven objects:
    - fields, relationship names and child relationship names as in the spec's Objects table
    - schema `sc_setup`
    - `Createable`, `Updateable` and `Deletable` exactly as in the spec's "API access" column
  - `Principal` gains `Perms policy.Permissions`, `Location *time.Location` and `ProfileID string`, and keeps `Role` for provisioned tokens only.
  - `func LoadPermissions(ctx context.Context, q Querier, tenant, user string) (Permissions, int64, error)` returns the permissions and the revision. `Querier` is `interface{ Query(...) ; QueryRow(...) }` over a pgx tx.
  - `func RoleProfilePermissions(ctx context.Context, q Querier, tenant, role string) (Permissions, int64, error)` gives a provisioned token's role mapping: `admin` → System Administrator, `restricted` → Standard User.
  - `type Cache` with `Get(tenant, user string, revision int64) (Permissions, bool)` and `Put(...)`, held on `*platform.Store`.
  - `ExecutionContext.Validate` stops checking `Role`.
  - `policy.Allowed(p, object, field)` becomes `p.Perms.FieldRead(object, f)` after field lookup. `records/validation.go`'s duplicate `allowed` delegates to it.

- [ ] **Step 1: Write the failing tests** in `load_test.go` (database-backed, as in `internal/platform` tests).
  - `TestLoadPermissionsUnionsProfileAndAssigned`: Standard User plus a custom set granting Read on `Account.Compat_Text__c` makes `FieldRead` true.
  - `TestPermissionRevisionInvalidatesCache`: load, insert a FieldPermissions row, and bump the revision in the same transaction. Then `Cache.Get` at the old revision misses, and reload shows the grant.
  - `TestInactiveUserHasNoPermissions`: `is_active = false` gives an empty `Permissions` and no error.
  - `TestProvisionedRoleKeepsAccess`: `RoleProfilePermissions("restricted")` equals Standard User's union.
- [ ] **Step 2: Run them.** `go test ./internal/policy/ -run Load` fails.
- [ ] **Step 3: Implement `SetupObjects`, `LoadPermissions`, `RoleProfilePermissions` and `Cache`.**
  - Add the catalog rows to migration 7.
  - Wire authentication: after the token query, load the user row, permissions and location, consulting the cache by `sc_setup.revisions`. A provisioned token uses `RoleProfilePermissions` and the org default location.
- [ ] **Step 4: Run the full suite.** `go test -race -p 1 ./...` passes. Today's restricted-user tests, such as the `Compat_Text__c` cases, still pass unchanged.
- [ ] **Step 5: Commit.** `git commit -s -m "feat: register setup objects and load effective permissions"`

### Task 5: Read-path enforcement

**Files:**
- Modify: `internal/soql/bind.go` (object read in `FROM`, traversal and child subquery, plus field read), `internal/records/validation.go` (describe and global describe), `internal/records/write.go` (`Read`), `internal/platform/http.go` and `internal/platform/composite.go` (the API Enabled gate)
- Create: `internal/platform/permissions_read_test.go`, `internal/platform/api_enabled.go`

**Interfaces:**
- Consumes: `Principal.Perms` (Task 4).
- Produces:
  - `func requireAPIEnabled(p Principal, r *http.Request) error` in `api_enabled.go`. It returns the `D33-*` "API not enabled" error unless `p.Perms.System().APIEnabled`, or the request carries the first-party UI session marker: context key `uiSessionKey`, set only by the UI session middleware, which Plan 36 replaces. Tests set it directly.
  - **Setup-object read rule:** reading Profile, PermissionSet, ObjectPermissions, FieldPermissions or PermissionSetAssignment needs ManageUsers or ViewAllData. User is readable by every API-enabled user.

- [ ] **Step 1: Write the failing tests** in `permissions_read_test.go`.
  - `TestHiddenObjectTraversal`: a permission set without Account read. `SELECT Account.Name FROM Contact` and `SELECT (SELECT Id FROM Contacts) FROM Account` fail with the `D33-*` codes, matching an unknown relationship.
  - `TestHiddenObjectDescribe`: the global describe omits the object, and describing it gives `NOT_FOUND`.
  - `TestHiddenFieldEverywhere`: Standard User gets `Compat_Text__c` absent from describe, and `INVALID_FIELD` from SOQL `SELECT`, `WHERE` and `ORDER BY` and from REST `?fields=`.
  - `TestAPIDisabledRefused`: Minimum Access gets the `D33-*` error on REST and Composite.
  - `TestUISessionExempt`: the same user with `uiSessionKey` set succeeds.
  - `TestSetupObjectsNeedManageUsers`: Standard User gets `SELECT Id FROM PermissionSet` refused and `SELECT Id FROM User` allowed.
  - `TestSecurityEnforcedUsesPermissions`: `WITH SECURITY_ENFORCED` on a hidden field fails as documented in `D33-*`.
- [ ] **Step 2: Run them.** `go test -p 1 ./internal/platform/ -run 'Hidden|APIDisabled|UISession|SetupObjects|SecurityEnforced'` fails.
- [ ] **Step 3: Implement the checks** at the files listed. Error codes come from Task 1's doc.
- [ ] **Step 4: Run them, and then the full suite.** `go test -race -p 1 ./...` passes.
- [ ] **Step 5: Commit.** `git commit -s -m "feat: enforce object and field read permissions"`

### Task 6: Write-path enforcement and setup-object rules

**Files:**
- Create: `internal/records/setup_write.go`, `internal/records/setup_write_test.go`, `internal/platform/permissions_write_test.go`
- Modify: `internal/records/write.go` (`create`, `update`, `deleteRecord` call the object and field edit checks, and dispatch setup objects to `setup_write.go`), `internal/metadata/admin.go` (field and object deletion removes grants and bumps the revision), `internal/platform/oauth_tokens.go` (the token query joins `sc_setup.users.is_active`)

**Interfaces:**
- Consumes: `ValidateObjectGrant` and `ValidateFieldGrant` (Task 2), the identity functions (Task 3), and `Principal.Perms` (Task 4).
- Produces: `func setupWrite(ctx context.Context, db database, p Principal, op, object, id string, data map[string]any) (string, error)`. Every successful setup write bumps `sc_setup.revisions` in the same transaction. It enforces:
  - Manage Users or Modify All Data
  - the grant dependency rules, and no grant may target a setup object (`TestGrantOnSetupObjectRefused`)
  - uniqueness
  - for profile-owned sets: updatable only, never created or deleted
  - system-maintained PermissionSetAssignment rows: created with the user, moved on a ProfileId change, refused on direct create or delete
  - the last active Manage Users or Modify All Data holder can't lose it
  - for User: create calls `create_identity`, a Username change calls `rename_identity`, delete is refused, and the required fields come from `D33-*`

- [ ] **Step 1: Write the failing tests.**
  - `TestStandardUserCannotEditHiddenField`: `PATCH Account` with `Compat_Text__c` gives the `D33-*` write error.
  - `TestReadOnlyCannotCreate`: Read Only `POST Contact` is refused.
  - `TestObjectGrantRejectsCreateWithoutRead`: an ObjectPermissions insert with `PermissionsCreate=true` and `PermissionsRead=false` is refused with the `D33-*` code.
  - `TestAssignPermsetGrantsField`: an admin inserts a PermissionSetAssignment for a custom set granting `Compat_Text__c`, and the Standard User's next request can query it.
  - `TestProfileAssignmentRowsSystemMaintained`: changing a user's ProfileId moves their profile-owned assignment, and deleting that row directly is refused.
  - `TestLastAdminCannotBeDemoted`: changing the only System Administrator to Standard User is refused.
  - `TestRESTUserCreateHasNoPassword`: `POST User` succeeds and login with any password fails. A duplicate username in another tenant gives `DUPLICATE_USERNAME` or the `D33-*` code.
  - `TestDeactivatedUserTokenRefused`: PATCH `IsActive=false`, and the user's existing token gets `INVALID_SESSION_ID` on its next call.
  - `TestDeletedFieldGrantRemoved`: delete a granted custom field, and its FieldPermissions rows are gone.
  - `TestCompositeDeniedSubrequest`: `allOrNone: true` with one allowed and one denied subrequest rolls back both, and the denied one carries the permission error.
  - `TestStandardUserCannotWriteSetupObjects`: Standard User inserting a PermissionSet is refused.
- [ ] **Step 2: Run them.** `go test -p 1 ./internal/records/ ./internal/platform/ -run '<the names above>'` fails.
- [ ] **Step 3: Implement `setupWrite` and the checks.**
- [ ] **Step 4: Run the full suite.** `go test -race -p 1 ./...` passes.
- [ ] **Step 5: Commit.** `git commit -s -m "feat: enforce write permissions and setup-object rules"`

### Task 7: Time zone, Organization and CLI

**Files:**
- Modify: `internal/soql/dates.go` (`DateLiteralRange(lit Literal, now time.Time, loc *time.Location)`), `internal/soql/bind_expr.go:118,222` (pass `b.loc`), the `soql.Options` and binder (a `Location *time.Location` field, defaulting to UTC when nil), `internal/platform/http.go` (remove the string-matched Organization discovery; Organization is served from the catalog), `cmd/salescompel/admin.go` (`user create` reads `SALESCOMPEL_PROFILE` and falls back to `SALESCOMPEL_ROLE` with a deprecation line on stderr, plus the new `org set` subcommand reading `SALESCOMPEL_TIME_ZONE`, `SALESCOMPEL_LOCALE` and `SALESCOMPEL_LANGUAGE`)
- Test: `internal/soql/dates_test.go`, `internal/platform/organization_test.go`, `cmd/salescompel/admin_test.go`

**Interfaces:**
- Consumes: `Principal.Location` (Task 4).

- [ ] **Step 1: Write the failing tests.**
  - `TestTodayInUserZone`: with `now = 2026-11-14T03:00Z` and `loc = America/Los_Angeles`, `TODAY` gives the range `[2026-11-13T08:00Z, 2026-11-14T08:00Z)`.
  - `TestDateLiteralAcrossDST`: `LAST_N_DAYS:3` spanning `2026-11-01` in `America/New_York` has a 25-hour day boundary.
  - `TestDateFieldUsesLocalDate`: a date field compared to `TODAY` uses the local calendar date.
  - `TestDayOnlyStaysUTC`: `DAY_ONLY(CreatedDate)` is unchanged.
  - `TestOrganizationQueryable`: `SELECT TimeZoneSidKey, DefaultLocaleSidKey, LanguageLocaleKey FROM Organization` returns the settings, and `PATCH` gives the `D33-*` read-only error.
  - `TestOrgSetChangesDefaultsNotUsers`: after `org set`, a new CLI user gets the new zone and an existing user keeps theirs.
  - `TestUserCreateRoleAlias`: `SALESCOMPEL_ROLE=restricted` creates a Standard User and prints the deprecation notice.
- [ ] **Step 2: Run them.** `go test -p 1 ./internal/soql/ ./internal/platform/ ./cmd/salescompel/ -run 'Today|DST|DateField|DayOnly|Organization|OrgSet|RoleAlias'` fails.
- [ ] **Step 3: Implement.** Every existing `DateLiteralRange` caller passes a location (UTC where no principal exists).
- [ ] **Step 4: Run the full suite.** `go test -race -p 1 ./...` passes.
- [ ] **Step 5: Commit.** `git commit -s -m "feat: user time zones, Organization object and org defaults"`

### Task 8: Access matrix, real clients, OpenAPI and docs

**Files:**
- Create: `internal/platform/access_matrix_test.go`, `compat/clients/node/src/candidate-permissions.ts`, `scripts/candidate-permissions-python.py`
- Modify: `compat/clients/node/src/candidate-acceptance-clients.ts` and `candidate-acceptance-contract.ts` (register the `permissions/*` steps), `scripts/check-candidate-acceptance.sh`, `api/candidate-openapi.yaml` (the seven objects' paths and schemas with `x-source`), `docs/compat/plan-33-users-profiles-permissions.md` (evidence), `docs/compat/plan-29-soql-engine.md` (strike the user-time-zone gap), the roadmap §3 Plan 33 implementation record, and `docs/paths/README.md` "Free today"

- [ ] **Step 1: Write the failing access-matrix test.** `TestAccessMatrixNoUnexpectedGrants` covers every catalog object × permissionable field × {System Administrator, Standard User, Read Only, Minimum Access, Standard User + custom set}. For each, it asserts the exact `{read, create, edit, delete}` and `{fieldRead, fieldEdit}` against a table derived from the spec's seeded grants. Any extra allowed cell fails the test and names the cell.
- [ ] **Step 2: Run it** (fails until the table and the harness exist), then implement the harness and make it pass.
- [ ] **Step 3: Add the real-client steps.**
  - **`sf` CLI:** `permissions/cli-assign-permset` runs `sf org assign permset --name P33_<run> --on-behalf-of <standard user>`, after which `sf data query` of `Compat_Text__c` as that user succeeds. Before the assignment it fails with `INVALID_FIELD`.
  - **JSforce:** `permissions/jsforce-permset` creates a set with ObjectPermissions and FieldPermissions, then describes as the restricted user.
  - **simple-salesforce:** `permissions/python-readonly` gets a Read Only create refused.
  - **Time zone:** `permissions/timezone-today`.
- [ ] **Step 4: Update the OpenAPI document** and run `npm --prefix compat/clients/node run check`. It passes.
- [ ] **Step 5: Full verification.** `go vet ./... && go test -race -p 1 ./...`, `sh scripts/check-file-size.sh`, `bash scripts/check-migrations_test.sh`, and `scripts/check-candidate-acceptance.sh --out /tmp/p33.json` all pass with no required skips.
- [ ] **Step 6: Write up the docs.** Finish the compatibility doc's evidence. Add the roadmap implementation record ("Plan 34 and Plan 35 are now unblocked; Plan 36 is next for the maintainer"). Update "Free today" in `docs/paths/README.md`.
- [ ] **Step 7: Commit and push.** `git commit -s -m "test: Plan 33 access matrix and real-client acceptance"` with `Closes #23`, then `git push origin main`.
