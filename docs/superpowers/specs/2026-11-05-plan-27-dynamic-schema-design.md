# Plan 27 — Forward migrations, tenant RLS and dynamic schema

**Status:** Draft for maintainer review; implementation is not authorized by this document.
**Date:** 2026-11-05
**Depends on:** Plan 25; preserves the implemented Plan 26 identity system.
**Roadmap:** [first release roadmap](2026-11-04-salescompel-first-release-roadmap-design.md), Plan 27 and §6.
**Implementation:** [Plan 27](../plans/2026-11-05-plan-27-dynamic-schema.md).

## Intent and outcome

A tenant administrator can add a custom object and text fields, then use them through unmodified JSforce and the connected Vue editor, including after restart. PostgreSQL enforces tenant isolation for business records and metadata. Operators run explicit, forward-only migrations instead of letting every process mutate the database at startup.

Today `internal/platform` has a process-global Account/Contact catalog, object-specific SQL, and startup upgrades to catalog v3. Plan 26 adds eight identity/OAuth tables, including `users.auth_version`, without changing that version. Plan 27 replaces these assumptions while retaining IDs, values, relationship integrity, external-ID behavior, field denials and OAuth token families.

The design choices below are proposed defaults derived from the accepted roadmap, not new maintainer decisions. This plan is one data-platform delivery with sequential internal tasks; migration, catalog and record service interfaces must land before the client/UI slice.

## Scope and boundaries

Ships:

- Explicit migration history with checksums, fresh installation and legacy adoption.
- Tenant-scoped, persistent object and field definitions; Account/Contact seeded for every tenant.
- Additive custom objects with `Id` and required text `Name` (length 80), and optional Text fields of length 1–255 on standard or custom objects.
- PostgreSQL RLS on business records and tenant metadata, transaction-local tenant context and restricted database credentials.
- Dynamic discovery, describe, CRUD, existing bounded SOQL and atomic Composite writes through the shared services.
- Operator CLI schema creation and inspection, plus metadata-driven discovery and editing in the existing connected UI.
- Required real-client, upgrade, isolation, restart and browser acceptance.

Non-goals: numeric/date/lookup/picklist types (Plan 28); new SOQL grammar or pagination (29); general custom external IDs; metadata edits, renames, deletion or length changes; Metadata/Tooling API deployment (35); profiles/sharing (33/34); setup UI or replacement application shell (36); automation; online mixed-version upgrades; certification.

Anti-goals: silently dropping legacy data or credentials, serving a field that storage does not enforce, using a database owner for record traffic, trusting a tenant supplied in HTTP input, or publishing a performance claim before measurement.

## Global constraints

- Go 1.25.0; PostgreSQL 17 is the verification baseline; retain the pinned Vue/TypeScript/client toolchains.
- No new runtime dependency is required by this design.
- API version remains v65.0; OpenAPI remains 3.2.1 unless the repository policy advances it before execution.
- Preserve existing Account/Contact IDs, external-ID canonical keys, field lengths, null rules and restricted-field denial.
- Preserve all Plan 26 users, apps, hashes, auth_version values, pending requests, codes, refresh/access tokens and replay records.
- Unknown security behavior denies access; unsupported active automation is never reported as executable.
- Product copy uses the existing en_US, fr, de, pt_BR and iw catalogs; tenant labels remain tenant data.
- Follow CLEAN_ROOM.md; distinguish documented behavior, candidate defaults and live observations.

## 1. Physical storage and measurement gate

**Proposed choice:** typed relational tables and columns. Retain shared `accounts` and `contacts` and their existing constraints. Each tenant custom object receives a generated table `sc_records.o_<32 lowercase hex>` with `(tenant_id, id)` primary key and bounded text `name`. A new custom field receives a generated column `f_<32 lowercase hex>` on its object's physical table. A field added to a shared standard table occupies a separate physical column for that tenant's definition. Physical identifiers never come from API names; quote generated identifiers with pgx.Identifier. Values always use SQL parameters.

A tenant's field names may coincide with another tenant's, but have distinct metadata IDs and columns. Other tenants cannot address that column through metadata. Standard table column growth and per-object table growth are deliberate early-preview tradeoffs, not a multi-tenant scale claim.

Before implementing storage, benchmark this layout against a shared JSONB-record alternative in a disposable database: 10 tenants × 10,000 records with 20 populated text fields; repeat at 100 fields. Include 1 and 8 concurrent workers, a skewed tenant holding half the total rows, create/read/update, case-insensitive filtered ordered queries, existing Account/Contact joins, uniqueness checks, and field-add latency. Run 3 measured repetitions after warmup and retain workload seed, environment, schema, indexes, EXPLAIN plans, p50/p95, throughput, bytes and errors. No host-specific absolute latency is a release gate. Correctness, database-enforced uniqueness/relationships and isolation are mandatory; a repeatable >2× p95 disadvantage in a required operation triggers a written tradeoff review before committing the physical storage migration. JSONB is not selected merely because it is quicker to implement. Record the decision in `docs/compat/plan-27-dynamic-schema.md`; change this spec and plan together if measurement warrants a different layout. No benchmark has been run while drafting.

## 2. Migrations and operational roles

Create `sc_control.schema_migrations(version bigint primary key, name text, checksum text, applied_at timestamptz)`; checksums are SHA-256 over the exact embedded migration bytes; a versioned Go migration also includes its frozen source bytes in that checksum. An ordered immutable manifest applies each pending migration in its own transaction under session advisory lock 739102. Release the lock/connection on every exit. Reject unknown newer versions, gaps and changed checksums before executing pending changes. There are no down migrations.

`migrate status` reads state; `migrate up` applies it. `serve`, provisioning and identity commands stop running schema.sql/ensureCatalog; they verify the expected migration head and fail with actionable instructions if missing or different. Migration and schema creation use the same exclusive advisory lock; record/catalog transactions take its shared transaction form until completion. Set schema lock_timeout to 5 seconds so busy deployments fail with a retry instruction. Migration commands have a separate 30-minute deadline; schema commands have a 30-second deadline. Operators stop old servers, back up, migrate, then start the new server. Rollback restores the backup. A later migration failing rolls back itself; earlier committed migrations remain recorded and restart resumes safely.

Migration 0001 adopts an exact legacy v1/v2/v3 installation or installs the v3 baseline fresh. Freeze the existing legacy validators/upgrades as migration-only code, preserving their refusal of oversized values, canonical collisions, unknown catalog definitions and physical mismatches. Recognize both pre-26 and post-26 schemas explicitly; add missing identity structures without rewriting existing values. Reject partially present identity layouts except the documented optional `auth_version` addition. Only an empty application database qualifies as fresh; do not overwrite unexplained tables.

Migration 0002 adds tenant metadata, seeds existing tenants and allocates custom prefixes. Migration 0003 installs roles/grants/RLS. Each is atomic; adoption tests snapshot all legacy business and identity state. `platform_catalog` becomes a historical legacy marker, unread by runtime catalog resolution. New tenant provisioning atomically inserts the tenant, its seeded standard metadata and its principal.

Three credentials separate privileges:

| Environment | Role and use |
| --- | --- |
| `MIGRATION_DATABASE_URL` | Operator-only owner connection for migrations, provisioning, identity administration and schema CLI; never loaded by serve |
| `DATABASE_URL` | Record runtime; SELECT metadata and business-record DML only; no DDL, identity-table access, ownership, superuser, BYPASSRLS or privileged role membership |
| `AUTH_DATABASE_URL` | Identity runtime; existing OAuth/login/principal operations only; no business-record/tenant-metadata access, DDL or privileged role membership |

Roles are `salescompel_owner` (NOLOGIN owner), `salescompel_records` and `salescompel_identity` (NOLOGIN privilege groups); deployments supply LOGIN members and credentials. The migration login must be able to SET ROLE owner; DBA bootstrap of groups and memberships is documented and tested by the harness. Migration SQL never creates passwords. `serve` verifies effective role attributes, ownership and reachable memberships, not just current_user, and rejects unsafe combinations or the same login for both pools. No owner credential fallback exists.

**RLS boundary:** enable and force RLS on accounts, contacts, custom tables, object definitions, field definitions, tenant catalog revisions and retained metadata artifacts. Apply both USING and WITH CHECK with `tenant_id = nullif(current_setting('salescompel.tenant_id', true), '')`. No context sees no rows and cannot insert. Explicit application tenant predicates remain. Each record/query/metadata read transaction sets tenant from authenticated execution context with `set_config(..., true)`; never use session SET. Pool reuse after success, rollback, cancellation and panic cannot retain a tenant. Composite establishes context once for its whole transaction.

Identity tables, the tenant registry and migration history are control-plane storage, explicitly outside this RLS claim. The identity role may resolve a globally unique username or credential before a tenant is known; it cannot read records or metadata. Existing identity authorization and same-tenant app/user checks remain mandatory, with regression coverage. RLS protects against omitted tenant predicates; it does not protect against a compromised application choosing a different tenant setting or a database administrator. Expanding control-plane RLS requires a separate credential-resolution design.

## 3. Tenant metadata and schema commands

Object definitions store stable internal ID, tenant, canonical API name and lowercase lookup key, label/plural label, standard flag, physical schema/table, ID prefix and catalog revision. Field definitions store stable ID, object/tenant, canonical name/lookup key, label, logical type, length, required/read-only flags, physical column, external-ID properties and reference target where seeded. Unique constraints bind names to tenant/object case-insensitively; foreign keys include tenant. Add per-tenant `catalog_revision` and retained source-artifact storage for future import. Runtime rejects any unrecognized effective type or security declaration; raw retained artifacts are never executable definitions. Plan 27 commands accept only this active subset, reject unsupported properties, and create no inactive artifacts themselves.

`schema object create --tenant T --name Project__c --label Project --plural-label Projects` creates one object; `schema field create --tenant T --object Project__c --name Notes__c --label Notes --type Text --length 80` creates an optional field; `schema inspect --tenant T` emits definitions and revision as JSON. Also support field creation on Account and Contact. These are operator commands using MIGRATION_DATABASE_URL; no HTTP schema-write API is introduced. Tenant must already exist. Default labels are not inferred from user text.

Custom API names: ASCII letter followed by letters/digits/single underscores, ending `__c`; stem length 1–40; no consecutive underscores in the stem or trailing underscore. Reject standard-name collisions and case-only duplicates. Labels/plural labels are 1–80 Unicode scalar values, valid UTF-8, no NUL and not whitespace-only. Reject duplicate flags, unknown properties, unsupported types, required/unique/external-ID new fields and invalid lengths. Errors are explicit CLI failures; no partial DDL or metadata survives.

DDL, metadata insertion, grants, RLS and revision increment commit in one owner transaction under the schema advisory lock. No HTTP runtime process holds DDL credentials. Read metadata in a fresh transaction for each operation; no process-global cache in this delivery. Concurrent operations see a complete pre-change or post-change snapshot, never a half-definition. Serialize schema changes against each other and use database locking against in-flight record transactions; lock_timeout produces an actionable retry without partial changes.

Seed Account/Contact exactly as today, including Contact.AccountId and the two synthetic Account fields. Reuse existing physical columns for seeds. Allocate a persistent tenant-unique custom-object prefix from `a00` through `aZZ` using base62 order (3,844 slots); exhaustion fails explicitly. Prefixes may repeat across tenants. Standard and identity prefixes are reserved. Preserve 15/18-character normalization/checksum rules and validate prefixes against the tenant's resolved object.

## 4. Service boundaries and runtime behavior

Extract focused packages as behavior moves, retaining `internal/platform` as HTTP/OAuth adapter and compatibility facade:

- `internal/policy`: explicit execution context and current role/field policy. Context contains principal, API version, privilege mode (`user` only) and sharing mode (`tenant` only); unsupported modes fail closed.
- `internal/metadata`: tenant catalog snapshots, definitions, validation, seeding and schema administration.
- `internal/records`: transaction runner and the sole create/update/delete/upsert write service; existing SQL external-ID implementation moves here without changing its behavior.
- `internal/query`: existing parser/normalizer/compiler over a resolved catalog. Preserve the bounded grammar and `Account.Name` relationship; Plan 29 owns the replacement engine.
- `internal/migration`: immutable migration manifest, legacy adoption and status/up.

Shared error/ID/value contracts live with their owning domain; prevent package cycles. Metadata and policy do not import platform. OAuth can remain in platform for this plan. New protocols later consume record transactions rather than copying writes.

Object/field discovery and normalization use a tenant snapshot, including canonical spelling. New custom objects and fields are visible to both existing roles, consistent with today's broad object access; restricted users still cannot see/write/filter/order by Account.Compat_Text__c. This is a documented candidate policy pending Plan 33, not imported Salesforce permission parity. Only admin-equivalent operators holding the owner credential create definitions.

Additive fields are nullable; existing records return null. Enforce text bounds, UTF-8, NUL rejection, case-insensitive names, optional-empty-to-null and required Name semantics through the shared record service. Dynamic SOQL supports the existing direct-field equality/order/limit behavior, column metadata and over-limit error. Keep external-ID upsert restricted to Account.Compat_Key__c. Composite accepts dynamic object paths for the existing POST/PATCH/DELETE subset; earlier ID references in paths continue to work; AccountId body references remain the only relationship reference. Preserve rollback/error semantics. All adapters share the same execution context, catalog snapshot and transaction for each operation.

Expose dynamic discovery/describe through existing GET `/services/data/v65.0/sobjects` and `/sobjects/{object}/describe`; extend CRUD path schemas to dynamic names. Retain the narrow OAuth User/Organization bridges before general object resolution. Candidate OpenAPI documents dynamic maps plus runtime validation, current permissions and provenance (`x-source`); it does not advertise a static universal field union or Metadata API. Traffic tests validate new dynamic paths and errors.

## 5. Connected editor

Discover the tenant's objects at connection time and provide a localized refresh action for catalog changes. Replace the fixed Account/Contact choices with returned object labels. Describe drives fields, lengths and required markers; custom record titles use Name. Preserve Contact's LastName title behavior. Product messages remain localized; custom labels are displayed as tenant text with API-name fallback and escaped by Vue. Use known translations for seeded fields and metadata labels for dynamic fields. Editing and creating remain on the existing token-based connected editor until Plan 36.

Clear selections, drafts and records when disconnecting or changing principals; refetch discovery/describe after reconnect. No stale result from the prior token/object may commit after an async switch. Failed refresh leaves a recoverable state with no stale save/delete action. Browser acceptance proves create/edit/read custom text, catalog refresh, two tenants with the same API names but different definitions, denied standard field, restart persistence and Hebrew RTL. No setup UI is included.

## 6. Evidence, acceptance and completion

Task 1 reviews relevant official changelogs since 2026-11-04 and public object/field/describe documentation, and offers the existing observation path if an authorized trial-org session exists. Missing live access never blocks implementation. Record unavailable sources honestly; dynamic Metadata API pages yielded only wrapper HTML during drafting, so their detailed semantics have not been verified. Do not inspect CLI/plugin implementation code.

Add required `schema` gate after oauth and before restart. The harness bootstraps disposable roles, migrates with the owner credential, then launches the service with record/identity credentials only. Through operator CLI create Project__c and Notes__c, and a field on Account; through JSforce discover, describe, create, read, update, query and delete records, assert cross-tenant invisibility and Composite rollback. Retain one custom record plus definitions for restart, then prove exact values, ID and metadata revision survive. Raw HTTP traffic measures dynamic routes. Every generated resource is journaled before creation; cleanup uses an explicitly owner-authorized test helper for metadata/DDL (no public delete command) and verifies no owned resources remain.

Database tests directly query using the actual restricted role, deliberately omit tenant predicates, and prove no-context read denial, wrong-tenant INSERT/UPDATE denial, pooled context cleanup and absence of owner/identity privileges. Upgrade tests cover fresh, v1/v2/v3, pre/post-26, repeated/concurrent migration, checksum drift, newer/gapped history, data-invalid rollback, injected failure and subsequent resume. Identity tests prove refresh/replay/lockout/auth_version behavior is retained. No database skip counts as evidence.

Complete only when the full acceptance command passes with required schema/restart/browser coverage, Go race suite and vet, web tests/build and offline lab check pass, benchmark decision is recorded, operator backup/upgrade/rollback guidance exists, and decisions/gaps/actual revision are recorded. Document any existing permitted offline skips; never mark synthetic tests as Salesforce observations.

## Sources and proposed calls

- [PostgreSQL 17 row security](https://www.postgresql.org/docs/17/ddl-rowsecurity.html): owner/superuser bypass is why runtime roles must be restricted and FORCE RLS is required.
- [PostgreSQL 17 configuration functions](https://www.postgresql.org/docs/17/functions-admin.html): transaction-local set_config is the basis for pooled tenant context.
- [Salesforce describe result](https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-describesobjects-describesobjectresult.html): public field/object names and labels inform discovery; exact candidate behavior stays bounded above.
- [Salesforce API names](https://help.salesforce.com/s/articleView?id=000385500&language=en_US&type=1): custom __c naming; the precise validation bounds here are candidate defaults.

Calls for review: typed storage subject to benchmark; operator CLI rather than premature Metadata API; separate identity and records credentials with an explicit control-plane RLS exclusion; additive optional Text only; per-request catalog reads; broad custom-object access under the current two-role policy. Builder may choose SQL helper names and visual arrangement within the existing editor. Changing scope, privilege boundaries or storage strategy requires updating both documents before implementation.
