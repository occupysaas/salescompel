# Plan 27 — Dynamic schema decisions and execution record

Status: implementation in progress; no Salesforce compatibility certification.

## Reference review (2026-11-05)

No authorized trial-org session was supplied. Existing observed contracts remain
unchanged; no local candidate test is a Salesforce observation. Reviewed the
[Winter 27 developer overview](https://developer.salesforce.com/blogs/2026/10/developers-guide-to-the-winter-27-release)
and [release notes](https://help.salesforce.com/s/articleView?id=release-notes.salesforce_release_notes.htm&release=264&type=5).
The overview covers headless tooling and metadata generation, with no accessible
claim changing this bounded Text/describe contract. Dynamic Metadata API reference
pages yielded wrapper HTML during planning; their detailed semantics remain
unverified. Custom naming bounds, CLI schema control and current broad custom
object access are candidate defaults, not observed reference behavior.

## Storage measurement and tradeoff review

[Raw measurements](benchmarks/plan-27-storage.txt), command:

```sh
CANDIDATE_DISPOSABLE=yes TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/metadata -run '^$' -bench BenchmarkStorageLayout -benchtime=1x -count=3 -v
```

PostgreSQL 17 in a local disposable container; Go 1.25.0; linux amd64,
12th Gen Intel Core i7-12700H (as reported by Go). Seed 2701, 100,000 records, 10 tenants,
20/100 text fields, balanced/skewed distribution, 1/8 workers. Three repetitions
passed, 96 operation/configuration series, zero SQL errors. Workload source
records schema/index generation; raw output retains plans, size, p50/p95,
throughput, errors and field-add time. Warmup: 10 operations per worker before
100 measured operations. Throughput includes warmup and is descriptive only.

**Decision:** keep typed relational tables/columns. The initial run is retained in
[initial measurements](benchmarks/plan-27-storage-initial.txt). The balanced 20-field workload
showed repeatable write disadvantages that cross the design's review threshold:
median p95 create 27,233 us typed versus 4,226 us JSONB at one worker; update
45,269 versus 7,215 us at eight workers. The maximum median ratio is 6.44x.
In that initial run, other measured typed configurations were at most 1.28x JSONB p95, and the
100-field workloads did not repeat that large gap. These host-local results
are not an SLA or a general performance claim. Their cause is not yet established;
do not explain them as a proved storage-engine property.

This is the required written tradeoff review: accept that early-preview write
cost in exchange for native per-field bounds, explicit typed indexes and direct
relational constraints, without building expression-index/type coercion machinery
around JSONB before Plan 28. Both layouts retain database uniqueness and composite
foreign keys. This benchmark measures owner SQL, not RLS overhead or complete
API traffic. Direct restricted-role isolation correctness is a separate mandatory
Task 4 gate. Revisit at measured customer scale, especially standard-table column
growth and tenant table counts; do not advertise multi-tenant scale from this run.
No physical migration was committed before these measurements and this decision.

A subsequent complete rerun passed in 143.837 seconds. The latest raw report linked
above measures JSONB field addition as a metadata insert without DDL and typed
addition as DDL plus the same metadata insert, both transactional. The initial
field-add measurement used ALTER TABLE for both layouts and is superseded.
The latest maximum median typed/JSONB p95 ratio is 1.49x (20-field skewed query,
109 versus 73 us at one worker); the earlier large write gap did not recur.
Field-add medians range 3,335–3,535 us typed and 3,202–3,420 us JSONB. Host load
was not controlled; the discrepancy is further reason to avoid general latency
claims. The typed-layout decision is unchanged.

## Verification

Baseline Go suite passed against disposable PostgreSQL 17. Migration, catalog,
role, client and final acceptance evidence will be recorded as those tasks pass.

## Operator setup and upgrade

PostgreSQL 17 is the tested database. Stop old instances and take a complete
backup before upgrading. Do not run old and new writers together. A DBA creates
these NOLOGIN privilege groups once per cluster and grants the operator login
membership in `salescompel_owner`:

```sql
CREATE ROLE salescompel_owner NOLOGIN;
CREATE ROLE salescompel_records NOLOGIN;
CREATE ROLE salescompel_identity NOLOGIN;
GRANT salescompel_owner TO your_operator_login;
CREATE ROLE your_records_login LOGIN;
CREATE ROLE your_identity_login LOGIN;
GRANT salescompel_records TO your_records_login;
GRANT salescompel_identity TO your_identity_login;
```

Set LOGIN passwords privately through your database administration mechanism.
Do not grant owner membership, ownership, superuser, BYPASSRLS, CREATEROLE,
CREATEDB or replication to either runtime login. The operator needs CREATE on the
database/public schema for a fresh install and must own, or be able to alter,
legacy application tables during adoption. Tables transfer to the owner group
in migration 3. Runtime roles receive only the documented grants; extra direct
credential-table grants or disabled business/catalog RLS refuse server startup.

Use three private environment values:

- `MIGRATION_DATABASE_URL`: operator connection, used only by migration,
  provisioning, identity administration and schema commands.
- `DATABASE_URL`: record runtime login, used by serve.
- `AUTH_DATABASE_URL`: identity runtime login, used by serve.

Run `salescompel migrate status`, then `salescompel migrate up`. Status emits
versions, names, checksums and applied flags. Fresh databases and exact legacy
catalog v1/v2/v3 layouts are supported, with/without Plan 26 identity tables.
Partial/unknown layouts, invalid existing data, changed checksums, gapped history
and newer versions refuse migration. Existing passwords, auth_version, lockouts,
apps and token/code/replay families are preserved. Migration files and frozen
adoption code must never be edited once released; add a new forward version.

Start `salescompel serve` with only the two runtime URLs and existing public URL,
TLS and asset settings. Startup checks migration head and privileges and does
not run DDL. If a migration fails, its transaction is rolled back; previous
versions remain committed. Remediate the reported condition and rerun up.
Rollback of a completed upgrade means stopping the new binary and restoring the
complete pre-upgrade backup before starting old code; no down migrations exist.
The development databases used before checksum finalization are disposable,
not an additional supported historical release.

Provision a tenant/principal using the existing `salescompel provision` or user
administration commands with the operator URL. Provisioning seeds Account and
Contact atomically. For a provisioned tenant, with operator credentials:

```sh
salescompel schema object create --tenant example --name Project__c --label Project --plural-label Projects
salescompel schema field create --tenant example --object Project__c --name Notes__c --label Notes --type Text --length 80
salescompel schema field create --tenant example --object Account --name Project_Note__c --label 'Project note' --type Text --length 80
salescompel schema inspect --tenant example
```

Schema operations are additive and transactional. Names/labels/types/lengths are
validated before commit. Contended schema operations time out with no partial
DDL or definition. Refresh objects in the connected editor to see newly created
definitions; reconnecting always reloads the tenant catalog. Tenant labels are
escaped text and do not enter shipped translation catalogs.

## Boundaries and follow-ups

RLS covers business records and tenant metadata (including revisions and retained
artifacts). Identity tables, tenant registry and migration history are a separate
control plane; identity credentials cannot read records/catalog and record
credentials cannot read identity secrets. The application still authenticates
the principal and selects the transaction tenant. RLS is protection against an
omitted predicate, not against an administrator or a compromised process choosing
another tenant setting.

Custom objects/fields are currently visible to both provisioned roles; the
existing Account.Compat_Text__c denial remains enforced in writes, describe,
projection/filter/order and Composite. This is a candidate policy, not imported
Salesforce object/field permissions. Exact role matrices arrive in Plan 33 and
sharing in Plan 34. Other field types belong to Plan 28; SOQL grammar/paging to
29; Metadata API deployment to 35; setup UI and application shell to 36. Schema
rename/update/delete and new custom external-ID/unique fields are not supported.

Unknown REST object routes retain 404 NOT_FOUND for the established boundary and
identity-enumeration contract; query unknown types return 400 INVALID_TYPE.
This corrects the implementation plan's initial REST 400 expectation.
