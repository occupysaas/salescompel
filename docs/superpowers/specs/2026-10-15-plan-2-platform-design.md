# Plan 2 — Platform walking slice design

The user requested the next plan and implementation in the same session. Plan 1
is an offline laboratory; Plan 2 implements the M2 walking slice from the roadmap.
M1 reference evidence is still missing: this work is a candidate implementation,
not a declaration that the roadmap's dependency or certification gates passed.

## Chosen approach

Use a modular Go HTTP service and PostgreSQL 17 with typed tables for the two
pilot objects. An embedded database would simplify setup but diverge from the
roadmap. A generic dynamic-schema engine would prematurely pull M4 into M2.
Use pgx v5, a small catalog in Go, tenant-scoped relational keys and constraints,
and a Vue/TypeScript application built with Vite. The initial catalog is fixed;
metadata deployment and arbitrary custom schemas belong to M4.

## Contract

- Go 1.25 or newer; Node 22.12 or newer; PostgreSQL 17.
- API version 65.0 is a candidate contract only, not a claim about a reference org.
- Account: Id, Name (required), Compat_Text__c (nullable text), Compat_Key__c
  (nullable unique external ID). Contact: Id, FirstName, LastName (required),
  AccountId (nullable, same-tenant reference). All supplied values are strings or null.
- Stable generated 18-character IDs; CRUD persists across process restart.
- Persist metadata version and seed definitions in PostgreSQL; startup rejects a
  database schema/catalog version that differs from the executable.
- Bearer tokens map to provisioned tenant/user/role tuples. Store SHA-256 token
  digests, never plaintext. No default credentials. Admin and restricted roles:
  restricted users cannot read/write/filter Account.Compat_Text__c. Both roles
  access all other pilot records in their own tenant. This is not general sharing.
- Unknown token returns 401; absent/cross-tenant records return 404. Tenant
  identity never comes from request payload, query string or a tenant header.
- Discovery, identity, own User read (including v51.0 CLI discovery), object
  describe, record CRUD and Account external-ID upsert. Unsupported surfaces
  return Salesforce-shaped errors and never an empty success.
- SOQL subset: SELECT explicit comma-separated fields FROM Account|Contact;
  optional WHERE one direct field = quoted string or null; optional ORDER BY one
  direct field ASC|DESC; optional LIMIT 1..2000. Contact.Account.Name is the only
  parent traversal. No wildcard, aggregates, subqueries or arbitrary SQL.
  Parser must consume the full input. Bound values and allowlisted identifiers
  prevent SQL injection. At most 2000 records; overflow is rejected explicitly
  (pagination is M3), not silently presented as complete.
- Required-field, unknown-field, null, wrong-type, malformed JSON, foreign-key,
  duplicate external-ID and permission errors leave the database unchanged.
- Concurrent disjoint-field updates preserve both values. External-ID upsert
  uses the database unique key and a transaction with a per-tenant/key advisory lock.
- HTTPS supported via certificate/key flags; localhost HTTP allowed for local
  development. Credentials and database URLs never logged. Client tests use HTTPS.

## UI and localization

A native metadata-driven Account/Contact list and editor supports token sign-in,
create/read/update/delete, relationship entry and errors/loading/empty states.
Tokens live in memory only and are cleared on logout. Build assets are served
same-origin by Go. Language selection lazy-loads the existing separate catalogs;
registry drives BCP 47 and direction including iw→he and pt_BR→pt-BR. Product
strings live in catalogs, with English fallback explicitly marked as unreviewed
coverage where translations are absent. Canonical field names and values do not
change with language. This slice does not implement migrated UI or user-setting
persistence.

## Verification and limits

Unit tests cover catalog policy, query parsing and invalid input. Real PostgreSQL
HTTP tests cover isolation, rollback on validation failure, null versus omitted,
relationships, uniqueness, concurrency and reopening the store. Unmodified
JSforce and Python clients exercise HTTPS candidate CRUD; a browser test exercises
the Vue CRUD flow, language switching and RTL. Existing Plan 1 offline checks
must remain green. CI runs database integration and client/UI tests, with missing
database/client configuration failing rather than silently passing.

Metadata SOAP, Composite, Bulk, full SOQL, refresh, pagination, migration,
production identity provisioning and full Salesforce security are out of scope.
Document exact implemented routes alongside the broader proposed OpenAPI file.
