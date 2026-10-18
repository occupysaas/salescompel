# Plan 8 — Active Field Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans under the preserved native sequential preference. Steps use checkbox syntax.

**Goal:** Return the observed custom-field lengths through the existing Account describe route, with consistent writes and a data-preserving versioned upgrade.

**Architecture:** Build the active catalog's two custom lengths from Plan 7 declarations. A bounded transactional v1-to-v2 startup migration validates the legacy definition and existing data before changing PostgreSQL bounds. Existing HTTP routing and clients stay intact.

**Tech Stack:** Existing Go, pgx/PostgreSQL, pinned Node/JSforce and candidate gate; no new runtime dependencies.

**Spec:** [Active field metadata design](../specs/2026-10-18-plan-8-active-field-metadata-design.md).

## Global constraints

- Preserve GET /services/data/v65.0/sobjects/Account/describe and all existing Salesforce paths/methods.
- Change only the two synthetic custom-field lengths to 80 in active Field JSON; keep type string and existing permission behavior.
- No automatic truncation, deletion or rewriting of existing data; upgrade must be atomic and fail closed.
- Preserve explicit declaration unknowns; do not emit Metadata API Text as describe type or activate declared grants.
- Preserve pilot/foundation profiles, accepted observations and Plan 5 sessions; no live reference collection required.
- Existing native branch preference applies. Local disposable databases only; no production upgrade, push or deployment.
- Real PostgreSQL integration is mandatory; skipped database tests do not satisfy acceptance.
- No new runtime dependencies, Metadata SOAP route or broad compatibility claim.

## Review focus

1. Oversized values in either column or another tenant must leave all data/schema/catalog unchanged — Task 1 rollback tests.
2. Old/unknown catalog definitions or v2 physical-schema drift must fail closed — Task 1 recognition tests.
3. Concurrent startups and repeated opens must produce one atomic transition without deadlock — Task 1 concurrency tests.
4. Supplementary Unicode, nulls and existing field permissions must retain candidate semantics at the new bound — Task 1 boundary tests.
5. Native describe and post-restart behavior must use the unchanged route and JSON representation — Task 2 client tests.

## Task 1: Activate lengths with transactional catalog migration

**Files:**
- Add internal/platform/catalog_migration.go, catalog_migration_test.go, catalog_v1.json.
- Modify internal/platform/catalog.go, store.go, schema.sql, catalog_test.go, metadata_test.go.

**Interfaces:**
- Consume `DeclaredCustomField(fullName string) (CustomFieldDeclaration, bool)` from metadata.go.
- Add `buildCatalog() (map[string][]Field, error)` in catalog.go; initialization must fail loudly on missing/invalid required declarations, without a 255 fallback.
- Add `ensureCatalog(ctx context.Context, tx pgx.Tx) error` in catalog_migration.go. Called by Open after base schema execution, within its existing advisory-locked transaction. It never commits or rolls back itself; Open owns both.
- Embed the frozen legacy definition from catalog_v1.json; catalogVersion is 2. Current Field struct/JSON keys remain unchanged.

- [ ] Obtain the exact v1 JSON definition from the repository's own e9709d5 catalog, using a temporary test/helper against that revision if needed; validate its decoded lengths are 255 and all other fields match the legacy serialization. Commit the static JSON fixture; do not derive it at runtime from the new catalog.
- [ ] Add `TestCatalogDeclaredLengths`: both custom fields have Length 80; Describe emits type string, nillable true, externalId false/true, referenceTo []; standard fields remain 255 and existing restricted visibility stays unchanged. A missing/invalid required declaration path in the constructor must error rather than fall back (use a narrow internal lookup parameter if needed for this test).
- [ ] Add `TestCustomLengthBoundaries`: Validate accepts 80 and rejects 81 code points for each writable custom field for ASCII, BMP and supplementary Unicode; null/empty remain accepted; restricted Compat_Text remains denied. Existing STRING_TOO_LONG error code/shape is retained without claiming reference overflow parity.
- [ ] Update Plan 7's boundary test: declarations remain passive, but the active catalog now intentionally rejects 81 for Compat_Key. Keep before/after lookup serialization and permission isolation assertions.
- [ ] Add PostgreSQL tests with isolated databases/schemas: `TestCatalogFreshV2`, `TestCatalogUpgradeV1`, `TestCatalogUpgradeOversizedRollback`, `TestCatalogRejectsUnknown`, `TestCatalogV2PhysicalMismatch`, `TestCatalogConcurrentOpen`. Fixture setup must reproduce v1 schema and exact frozen catalog, not call the new Open to create v1.
- [ ] Assert populated v1 upgrade preserves IDs, nulls, length-80 data, tenants and Contact relationships; both columns become varchar(80), catalog is version 2 and reopening succeeds. Test direct SQL rejects length 81 after upgrade. Assert failures for oversized text and key independently and another tenant; compare complete synthetic rows, column lengths and catalog before/after refusal. Unknown versions, altered definitions and absent catalog with existing Account/Contact rows fail without modification. V2 with wrong physical lengths fails. Concurrent Open calls with context deadlines both finish successfully and leave one v2 state.
- [ ] Run `go test ./internal/platform -run 'TestCatalog|TestCustomLength|TestDeclared' -count=1` with TEST_DATABASE_URL set to an explicitly disposable local database; observe failures before implementation. If no PostgreSQL is available, provision a local disposable instance using existing tooling; do not report skipped tests as passing acceptance.
- [ ] Implement buildCatalog using the declaration lengths; keep existing fields/properties and filtering. Keep base schema.sql's legacy 255 column definitions for recognition/bootstrap, then ensureCatalog applies the 80 bounds for both fresh and upgraded databases. Explain this ordering in a code comment.
- [ ] Implement ensureCatalog: recognize exact v1/v2 JSONB definitions, reject populated catalog-less databases, lock accounts ACCESS EXCLUSIVE, precheck char_length of both columns across all tenants, ALTER both to varchar(80) without truncating casts, atomically insert/update version 2/current definition. Verify v2 column lengths on reopen. Use a generic remediation error without row values or tenant identifiers. Preserve advisory lock 739102 and Open's transaction cleanup.
- [ ] Run `go test ./... -count=1` and `go vet ./...` with real PostgreSQL. Confirm all database tests actually execute and pass. Run gofmt and git diff --check.
- [ ] Commit the catalog/migration implementation and tests.

## Task 2: Existing-route client verification and upgrade documentation

**Files:**
- Modify internal/platform/http_test.go and compat/clients/node/test/candidate.test.ts.
- Modify scripts/check-candidate.sh only where needed to assert post-restart describe.
- Add docs/compat/plan-8-active-field-metadata.md; update docs/compat/implementation-log.md and README.md deployment section as appropriate.

**Interfaces:**
- Consume Task 1's unchanged Open/Describe interfaces and v2 database behavior.
- Use pinned JSforce's existing `conn.sobject("Account").describe()` and existing candidate HTTPS harness/auth; no client patch or new transport.

- [ ] Add HTTP regression assertions on GET /services/data/v65.0/sobjects/Account/describe: status 200 for the administrator, both custom lengths 80, string type, nillable true, externalId false/true, referenceTo empty; existing authentication failures and unsupported methods unchanged. Test HTTP create/update at 80/81, confirming rejected writes leave records unchanged. Preserve cleanup.
- [ ] Add assertions to the native JSforce candidate test for the same fields through the same API version. Run against the prior binary/fresh disposable database to demonstrate the length assertions fail at 255 before running against the new implementation; do not move HEAD or reuse a production database for this comparison.
- [ ] Extend the existing process-restart gate to describe Account after restart and assert length 80 persists. Keep authentication, TLS verification and owned fixture cleanup intact. Reuse existing native call conventions.
- [ ] Run `go test ./... -count=1`, `go vet ./...`, `npm --prefix compat/clients/node run check`, and `CANDIDATE_DISPOSABLE=yes scripts/check-candidate.sh` with DATABASE_URL/TEST_DATABASE_URL pointing to disposable PostgreSQL and existing Python/browser prerequisites installed. Every required gate must pass, not skip; document actual commands and outcomes without credentials.
- [ ] Document the maintenance workflow: stop old instances, back up, start new binary, verify v2/describe, restart service. Explain oversized-data refusal and operator-controlled remediation, old-binary rejection after successful upgrade, no automatic downgrade, backup/restore rollback, and unsupported mixed-version rolling upgrades. Explain Unicode counting as candidate behavior and keep uniqueness/case-sensitive enforcement, permission parity and Plan 5 completion out of claims.
- [ ] Verify profile bytes unchanged and git diff --check; commit documentation and client verification.
- [ ] Request a fresh read-only whole-change review against this spec/plan. Fix Critical/Important findings with failing regression tests and rerun affected gates; record all deferred/declined rulings truthfully before closing execution.

## Self-review and handoff

Task 1 owns all catalog/schema/write transitions; Task 2 verifies the existing HTTP
and native-client surface. Every review-focus case has a named test. No task
requires Plan 5 cursor completion or invents effective grants. Length activation
is the only intended client-visible change. Planning is complete only after the
design builder check; implementation requires the maintainer's review of this plan.
