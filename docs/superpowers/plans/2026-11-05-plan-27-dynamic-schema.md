# Plan 27 — Dynamic Schema and Tenant Isolation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** Implemented and verified 2026-11-05. See [evidence and operator guide](../../compat/plan-27-dynamic-schema.md).

**Goal:** Replace startup catalog mutation with explicit migrations and make tenant-defined objects/text fields usable through JSforce and the connected editor under PostgreSQL RLS.

**Architecture:** Keep standard tables, move definitions into a tenant catalog, and provision typed custom tables/columns through an operator-only connection. Extract policy, metadata, records, query and migration packages while retaining platform as the HTTP/OAuth adapter. Record operations use transaction-local tenant settings and a shared catalog snapshot; the service has separate record and identity connections and no schema-owner credential.

**Tech Stack:** Go 1.25.0, PostgreSQL 17, pgx v5, Vue 3, TypeScript, pinned JSforce 3.11.0/Salesforce CLI 2.156.0, Playwright.

**Spec:** [Plan 27 design](../specs/2026-11-05-plan-27-dynamic-schema-design.md). Read both documents before execution.

## Global Constraints

- Go 1.25.0; PostgreSQL 17 is the verification baseline; retain the pinned Vue/TypeScript/client toolchains.
- No new runtime dependency is required by this design.
- API version remains v65.0; OpenAPI remains 3.2.1 unless the repository policy advances it before execution.
- Preserve existing Account/Contact IDs, external-ID canonical keys, field lengths, null rules and restricted-field denial.
- Preserve all Plan 26 users, apps, hashes, auth_version values, pending requests, codes, refresh/access tokens and replay records.
- Unknown security behavior denies access; unsupported active automation is never reported as executable.
- Product copy uses the existing en_US, fr, de, pt_BR and iw catalogs; tenant labels remain tenant data.
- Follow CLEAN_ROOM.md; distinguish documented behavior, candidate defaults and live observations.

## Review Focus

- A reused pooled connection after rollback/cancellation must never retain the prior tenant; Task 4 tests it with a pool size of one.
- Legacy v3 includes optional Plan 26 identity state beyond the catalog marker; Task 2 compares values and verifies live refresh/replay behavior after adoption.
- Two tenants can define the same API names with different lengths/labels; Tasks 3/5 prove independent catalog and storage resolution.
- Metadata must not become visible before its DDL/grants/RLS commit, even under concurrent writes; Tasks 3/5 test a blocked and rolled-back schema transaction.
- An async UI response from a previous object or credential must not restore stale actions; Task 7 tests delayed responses and disconnect/reconnect.

## File and interface map

Create focused packages rather than another large platform file:

| Package/files | Responsibility |
| --- | --- |
| `internal/migration/{migrate.go,manifest.go,legacy.go,migrations/*}` | Version/checksum runner, frozen legacy adoption and immutable migration files |
| `internal/policy/{context.go,access.go}` | Principal, execution context, current field policy |
| `internal/contract/error.go`, `internal/recordid/id.go` | Cycle-free API errors and checksum/prefix helpers shared by domain packages |
| `internal/metadata/{model.go,catalog.go,seed.go,admin.go}` | Persistent tenant definitions, immutable snapshot, seeding and owner schema commands |
| `internal/records/{service.go,transaction.go,write.go,externalid.go}` | One transaction/write path, existing Account external-ID behavior |
| `internal/query/{query.go,normalize.go,select.go,columns.go}` | Existing bounded grammar/compiler against snapshot, no new grammar |
| `internal/platform/{store.go,http.go,composite.go,http_composite.go,oauth_*.go}` | Facade, HTTP and existing identity runtime using the proper pool |
| `cmd/salescompel/{main.go,migrate.go,schema.go}` | Dispatch by command/credential before opening a database |
| `web/src/connected/{ConnectedApp.vue,catalog.ts}` | Dynamic object selection and metadata labels in existing editor |
| `compat/clients/node/src/candidate-schema.ts` | Required real-client schema gate and restart fixtures |

Keep existing tests as regression oracles. When moving helpers, preserve platform aliases/wrappers used by existing callers until all references migrate; runtime wrappers must delegate to the tenant catalog. The frozen legacy catalog is migration-only. Do not leave two active catalogs.

---

### Task 1: Reference review and storage decision

**Files:** Create `docs/compat/plan-27-dynamic-schema.md`, `internal/metadata/benchmark_test.go`, `internal/metadata/testdata/storage-benchmark.sql`.

**Interfaces:** Produces the measured physical-layout decision and benchmark command; subsequent migrations depend on it. This task does not modify the running candidate.

- [x] **Step 1: Record source review and observation availability.** Review official relevant changelogs since 2026-11-04, object/field naming and describe docs. Use an authorized trial-org session only if available. Record source URLs, dates, versions and exact accessible content; wrapper-only pages and missing sessions stay explicit gaps. Preserve historical observation statuses.
- [x] **Step 2: Add the disposable storage benchmark.** `BenchmarkStorageLayout` compares typed columns with shared JSONB using the spec's 10×10,000 rows, 20/100 fields, 1/8 workers, balanced/skewed distributions, workload seed and three repetitions. Pin query/index/uniqueness/relationship semantics equally; record p50/p95, throughput, bytes, field-add timing, plans and correctness errors. Guard against use outside a database explicitly designated disposable.
- [x] **Step 3: Run and record the decision.** Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/metadata -run '^$' -bench BenchmarkStorageLayout -benchtime=1x -count=3`. Expected: complete measurements with zero correctness errors. A repeatable >2× required-operation p95 disadvantage requires a written tradeoff review before Task 2 locks storage. Do not invent measurements or make live parity claims. If layout changes, update both design and plan before continuing.
- [x] **Step 4: Commit the benchmark and decision.** `git add docs/compat/plan-27-dynamic-schema.md internal/metadata; git commit -m "docs: measure and select Plan 27 record storage"` (run as separate shell commands).

### Task 2: Explicit migrations and legacy adoption

**Files:** Create `internal/migration/{migrate.go,manifest.go,legacy.go,migrate_test.go,legacy_test.go}`, `internal/migration/migrations/0001_legacy_baseline.sql`, `cmd/salescompel/{migrate.go,migrate_test.go}`. Modify `cmd/salescompel/main.go`, `internal/platform/{store.go,catalog_migration.go,catalog_migration_test.go,schema.sql}`.

**Interfaces:**
- `migration.Entry { Version int64; Name, Checksum string; Applied bool }`.
- `migration.Status(ctx context.Context, pool *pgxpool.Pool) ([]Entry, error)`.
- `migration.Up(ctx context.Context, pool *pgxpool.Pool) error`.
- `migration.CheckHead(ctx context.Context, pool *pgxpool.Pool) error`.
- CLI: `salescompel migrate status|up` uses only `MIGRATION_DATABASE_URL`; status emits JSON without connection secrets. Give migration up a command-specific 30-minute timeout (configurable downward for tests), schema operations 30 seconds, and preserve the existing 15-second HTTP bounds; do not inherit main.go's current 20-second context for long upgrades.

- [x] **Step 1: Add failing migration tests.** Bootstrap the spec's NOLOGIN groups and disposable LOGIN memberships with a DBA connection; migrations use a login allowed to SET ROLE salescompel_owner. Use isolated databases with explicit fixture versions, including v3 pre-26 and post-26. Assert these named cases:

```text
TestFreshMigration: history == [(1, expectedSHA256)]; current schema exists
TestLegacyAdoption: IDs, data, canonical keys and every identity column equal before/after
TestLegacyIdentityOptionalAuthVersion: missing column added as 0; existing values unchanged
TestMigrationRejectsPartialIdentity: incomplete unknown layout -> error; no history written
TestMigrationRejectsInvalidLegacy: oversize/collision/catalog or physical mismatch -> unchanged
TestMigrationHistory: checksum drift, gap or newer version -> error before any mutation
TestMigrationResume: injected migration failure -> its DDL/history absent; prior version retained
TestConcurrentMigrate: both calls finish; each version recorded exactly once
TestServeDoesNotMigrate: unmigrated database -> actionable error; zero schema changes
```

Include currently valid v1/v2 fixtures and their existing refusal cases. Populate users.auth_version, lockout, used codes and descendant refresh/access tokens, pending requests and JWT IDs; after adoption exercise password reset and replay with existing Plan 26 tests. Keep existing catalog regression test names required by acceptance, adapting their implementation rather than removing their coverage.

- [x] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/migration ./internal/platform ./cmd/salescompel -run 'Migration|Legacy|Catalog|ServeDoesNotMigrate' -count=1`.** Expected: failures for missing explicit runner, not skipped DB tests.
- [x] **Step 3: Implement the runner and adoption.** Hash embedded bytes, validate the complete known history first, use one acquired connection and session advisory lock 739102, and transact each version independently. Freeze the current catalog v1/v2/v3 validators under migration ownership; the manifest includes their versioned source checksum as well as SQL, so edits to a Go migration body are detected. Install/adopt the exact v3 baseline without replacing identity data. Remove startup schema mutation; dispatch migration commands before runtime Open. During the intermediate commit the existing runtime connection may remain, but missing/stale history must refuse startup.
- [x] **Step 4: Run Step 2 and full migration tests; expected all pass with no DB skips.** Check migration lock release after cancellation and rollback, and that a retry resumes without reapplying a committed version.
- [x] **Step 5: Commit** the migration package, frozen fixtures and command changes with `feat: introduce explicit forward database migrations`.

### Task 3: Tenant catalog and atomic schema administration

**Files:** Create `internal/metadata/{model.go,catalog.go,seed.go,admin.go,catalog_test.go,admin_test.go}`, `internal/migration/migrations/0002_tenant_catalog.sql`, `cmd/salescompel/{schema.go,schema_test.go}`. Modify migration manifest, operator provisioning and tests.

**Interfaces:**
- `metadata.Field`: existing Field wire properties plus internal ID, physical column, logical type and seeded policy flags; `metadata.Object`: existing Object wire properties plus internal ID, tenant, physical schema/table, prefix and name-field marker.
- `metadata.Snapshot { Tenant string; Revision int64; Objects []Object }`; `Load(ctx context.Context, tx pgx.Tx, tenant string) (Snapshot,error)` returns a complete immutable snapshot; methods `Object(name string) (Object,error)`, `Field(object,name string) (Field,error)` resolve canonical case-insensitive names.
- `metadata.ObjectInput { Name, Label, PluralLabel string }`; `metadata.FieldInput { Object, Name, Label, Type string; Length int }`.
- `metadata.CreateObject(ctx context.Context, owner *pgxpool.Pool, tenant string, in ObjectInput) (Object,error)`; `CreateField(ctx context.Context, owner *pgxpool.Pool, tenant string, in FieldInput) (Field,error)`.
- `metadata.SeedTenant(ctx context.Context, tx pgx.Tx, tenant string) error` idempotently seeds exact Account/Contact metadata; operator `schema inspect` returns Snapshot JSON with no secrets or physical credentials.

- [x] **Step 1: Add failing tests.** `TestCatalogSeedParity` compares every seed field, length, role denial, reference and external-ID property to the current catalog. `TestTenantCatalogNames` creates same names in A/B with different labels/lengths, permits case-insensitive lookup but rejects case-only duplicates within one tenant/object. `TestSchemaInputRules` covers stem lengths 0/1/40/41, underscores, SQL punctuation, Unicode/NUL/blank labels, lengths 0/1/255/256 and unsupported flags/types. `TestSchemaAtomicDDL` injects failure after table/column creation and before metadata insert: table/column/grants/revision all unchanged. `TestSchemaPrefixAllocation` covers concurrent allocation, exhaustion and restart stability. Metadata isolation at this stage uses explicit tenant predicates; direct RLS assertions belong to Task 4.
- [x] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/metadata ./internal/migration ./cmd/salescompel -run 'Catalog|Schema|Prefix|Migration' -count=1`.** Expected: fail for absent definitions/commands.
- [x] **Step 3: Implement 0002, model and commands.** Add object/field/revision/artifact tables, composite tenant foreign keys and folded-name constraints; seed existing tenants. Use generated hex physical names, qualified identifiers, owner transactions, exclusive advisory lock 739102 and lock_timeout 5s. Under the same transaction create table/column, apply bounded-text constraints, install the table RLS policy, grant minimum record permissions, insert definition and increment revision. New fields are optional only. Prefix allocation is tenant-unique in base62 order a00–aZZ. Provision tenant+seeds+principal atomically. Keep retained artifacts isolated and inactive; command inputs reject unsupported metadata.
- [x] **Step 4: Run Step 2 and the migration suite.** Add `TestCatalogConcurrentVisibility`: hold owner DDL transaction open, attempt runtime load/write, then commit or roll back; observer sees a complete old/new catalog with matching schema, never partial DDL. Hold the shared record advisory lock while reading the snapshot; schema changes serialize through the exclusive owner lock. Expected all pass, including idempotent seed and transactional retry on lock timeout.
- [x] **Step 5: Commit** 0002 and CLI with `feat: persist tenant catalogs and provision typed custom schema`.

### Task 4: Execution context, role separation and tenant transactions

**Files:** Create `internal/policy/{context.go,access.go,context_test.go}`, `internal/contract/error.go`, `internal/recordid/{id.go,id_test.go}`, `internal/records/{transaction.go,transaction_test.go}`, `internal/migration/migrations/0003_tenant_rls.sql` (manifest after Task 3's 0002), `internal/platform/roles_test.go`. Modify `internal/platform/{store.go,oauth_store.go,oauth_tokens.go,oauth_page.go,http.go,composite.go}`, `cmd/salescompel/{main.go,admin.go}`.

**Interfaces:**
- `policy.Principal`: existing Tenant/User/Role/Username/Scope fields; `platform.Principal` remains a type alias.
- `policy.ExecutionContext { Principal Principal; APIVersion, PrivilegeMode, SharingMode string }`; constructor `policy.ForAPI(p Principal, version string) (ExecutionContext,error)` accepts v65.0, user privilege and tenant sharing only.
- `records.WithTenantTx(ctx context.Context, pool *pgxpool.Pool, execution policy.ExecutionContext, fn func(pgx.Tx) error) error` sets transaction-local tenant, takes shared transaction advisory lock 739102 and always closes transaction; reject empty tenant/unknown modes.
- `platform.StoreConfig { RecordsURL, IdentityURL string }`; `platform.OpenRuntime(ctx context.Context, config StoreConfig) (*Store,error)` verifies head and effective privileges for both pools.
- `platform.OpenOperator(ctx context.Context, url string) (*Store,error)` verifies head and owner capability; used only for operator commands.

- [x] **Step 1: Add failing tests** for record role attributes, ownership and inherited/SET ROLE paths to privileged roles. Prove the identity role cannot SELECT records/catalog and the records role cannot SELECT secrets, create/alter tables, TRUNCATE or disable RLS. Direct SQL tests omit tenant predicates and assert tenant A reads only A, absent/empty context reads zero, inserting B under A and changing tenant_id fail WITH CHECK. Test success, error, cancellation and panic through a one-connection pool, then run without context and as B; assert no A data. OAuth regressions must still resolve global usernames and hashed credentials across tenants correctly.
- [x] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/policy ./internal/records ./internal/platform -run 'TenantTx|RuntimeRole|RoleSeparation|OAuth|Refresh|Replay' -count=1`.** Expected: new privilege/isolation assertions fail before grants/RLS exist.
- [x] **Step 3: Implement transaction runner, split connections and owner command dispatch.** `DATABASE_URL` is records; `AUTH_DATABASE_URL` is identity; owner commands use `MIGRATION_DATABASE_URL`. NOLOGIN groups and DBA-created LOGIN memberships follow the spec. No credential fallback. Restrict public schema creation and unintended PUBLIC grants; fully qualify application tables. FORCE RLS and USING/WITH CHECK cover standard tables and later metadata/custom tables. Preserve explicit application predicates. Add 0003 to the manifest after 0002. Cover all metadata tables, tenant revision rows and retained artifacts as well as records; the identity and control-plane exclusions stay explicit.
- [x] **Step 4: Run Step 2 and existing identity tests against actual limited logins.** Route existing facade record/query/Composite calls through WithTenantTx before enabling standard-table RLS; extraction into the final domain services follows in Task 5. Expected: all pass, unsafe startup rejects before listening, schema remains unchanged on startup. Scope/CSRF/refresh/replay/auth_version regressions remain green.
- [x] **Step 5: Commit** with `feat: isolate record transactions and identity database privileges` after the complete ordered manifest passes on fresh and legacy databases.

### Task 5: One metadata-driven record/query path

**Files:** Create `internal/records/{service.go,write.go,externalid.go,service_test.go}`, `internal/query/{query.go,normalize.go,select.go,columns.go,query_test.go}`. Move policy/ID/error helpers into Task 4 packages. Modify platform facade and existing `catalog.go`, `store.go`, `select.go`, `names.go`, `ids.go`, `query*.go`, `composite.go`, `http_composite.go` and their tests.

**Interfaces:**
- `records.Service { Pool *pgxpool.Pool }`; `Within(ctx context.Context, execution policy.ExecutionContext, fn func(*Tx) error) error` wraps WithTenantTx, loads one snapshot, and invokes callback.
- `records.Tx` owns `pgx.Tx`, execution context and snapshot; methods `Create(ctx,object string,data map[string]any)(string,error)`, `Update(ctx,object,id string,data map[string]any)error`, `Delete(ctx,object,id string)error`, `Read(ctx,object,id string)(map[string]any,error)`, `UpsertAccount(ctx,key string,data map[string]any)(string,bool,error)`.
- `query.Query`: preserve existing Query fields; `query.Parse(input string)(Query,error)` preserves grammar; `query.Select(ctx context.Context, tx pgx.Tx, execution policy.ExecutionContext, catalog metadata.Snapshot, q Query)([]map[string]any,error)` performs existing normalization/security/SQL/serialization. `records.Tx.Select(ctx,q)` delegates to it without starting another transaction.
- `recordid.Normalize(prefix,input string)(string,error)` and `recordid.New(prefix string)(string,error)` preserve current checksum/randomness behavior.

- [x] **Step 1: Add failing dynamic service tests.** Create Project__c/Notes__c in A and differently bounded definitions in B. Assert CRUD/query canonical spelling, optional missing/empty->null, Unicode length bounds and required Name length 80, bad/wrong-object IDs, field policy in projections/filters/order, attributes.type/url and query columns. Add Account field CRUD without affecting B's same name. Query limits/null order/errors remain exactly the foundation contract. Use direct SELECT and standard Contact.Account.Name to prove seeded constraints. Test Composite custom-object earlier-ID PATCH/DELETE, AccountId references and full rollback on an oversized dynamic field.
- [x] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/records ./internal/query ./internal/platform -count=1`.** Expected: new dynamic paths fail; existing behavior is the regression baseline.
- [x] **Step 3: Extract and route services.** Remove active global catalog/table fallback and object-specific prefix assumptions. Compile only validated metadata into quoted identifiers; parameterize values. Preserve Plan 25 Account upsert implementation and retry/concurrency contract, restricted field enforcement, normalization and same-tenant foreign keys. Composite gets one records.Tx and snapshot and reuses Create/Update/Delete. Its route parser accepts validated dynamic API names without enabling unsupported reads/upserts/partial success. Keep OAuth bridges separate and existing HTTP wire shapes unchanged.
- [x] **Step 4: Run Step 2 with the race detector:** `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test -race ./internal/records ./internal/query ./internal/platform -count=1`. Expected all pass, no skipped database proof. Add malicious API-name and filter-value tests proving SQL injection is rejected or treated as data. Existing catalog tests now target seeded snapshots; frozen legacy tests remain migration-only.
- [x] **Step 5: Commit** with `refactor: route dynamic records and queries through tenant services`.

### Task 6: Dynamic HTTP discovery and served contract

**Files:** Modify `internal/platform/{http.go,http_test.go,http_openapi_test.go,http_composite_test.go}`, `api/candidate-openapi.yaml`, `compat/clients/node/src/{candidate-openapi-model.ts,candidate-openapi-traffic.ts,candidate-openapi-contract.ts}` and related tests.

**Interfaces:** Existing public URLs, request/response formats and OAuth scope classification remain; object path parameters resolve a tenant snapshot and dynamic text maps are validated at runtime. This adds no schema-write HTTP endpoint.

- [x] **Step 1: Write failing HTTP/traffic tests.** `TestHTTPDynamicCatalog` asserts A discovery/describe includes A's custom definitions only; B sees B's; unknown REST object -> 404 NOT_FOUND (preserve the existing boundary; query INVALID_TYPE remains 400); absent field -> 400 INVALID_FIELD; another tenant's record -> 404 NOT_FOUND. Validate dynamic POST 201, GET/PATCH/DELETE, JSON/error shapes, restricted field denial, unsupported type failure and Composite rollback. Assert id-only OAuth token receives 403 for dynamic data/discovery too. Exercise canonical case behavior through the same routes as seeds.
- [x] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/platform -run 'HTTPDynamic|HTTPComposite|HTTPServedOpenAPI|Scope' -count=1` and `npm --prefix compat/clients/node run typecheck`.** Expected: new dynamic traffic assertions fail until handlers/schema support them.
- [x] **Step 3: Implement dynamic discovery/describe and contract changes.** Populate labels/flags/urls from catalog, retaining User/Organization bridge precedence. Generalize OpenAPI path/map schemas without allowing arbitrary unchecked properties in effective record writes; describe runtime field validation and operator-only schema control. Add x-source URLs and source status; preserve OpenAPI 3.2.1. Remove stale candidate-contract prose claiming Composite is unavailable. Extend traffic validators to dynamically resolved object names rather than adding tenant fixtures to the static schema.
- [x] **Step 4: Run Step 2 and `npm --prefix compat/clients/node run check`.** Expected: Go tests, schema validation, typecheck, formatting and offline tests pass with only the established documented offline skips. Dynamic success and failure traffic must both be measured.
- [x] **Step 5: Commit** with `feat: expose tenant-defined objects through the candidate API`.

### Task 7: Metadata-driven connected editor

**Files:** Create `web/src/connected/catalog.ts`, `web/test/dynamic-schema.spec.ts`. Modify `web/src/{api.ts,connected/ConnectedApp.vue}`, `web/test/records.spec.ts`, `web/playwright.config.ts`, `locales/messages/{en_US,fr,de,pt_BR,iw}.json`.

**Interfaces:** Add `ObjectSummary { name:string; label:string; createable:boolean; queryable:boolean }` and describe field label/name-field properties to existing API types; `loadCatalog(signal:AbortSignal):Promise<ObjectSummary[]>` calls existing sobjects discovery. Component keeps token scoped to its lifetime and treats labels as escaped tenant text.

- [x] **Step 1: Add failing Playwright cases.** `dynamic schema supports custom record editing` creates an object/field through the owner fixture helper, refreshes discovery, fills Name/Notes, saves and reads exact values. `dynamic schema keeps tenant definitions separate` reconnects A->B with same API names/different labels/lengths and asserts no A values/drafts/options survive. `delayed catalog responses cannot restore a disconnected editor` holds responses while switching token/object. Also assert recoverable refresh failure leaves no stale save/delete, required/length controls, Contact title, standard restricted field denial, escaped HTML-like labels, and iw RTL. Use seeded backend definitions, not a browser-only synthetic catalog.
- [x] **Step 2: Run `npm --prefix web test` and `npm --prefix web run test:e2e -- dynamic-schema.spec.ts` with the existing candidate fixture environment.** Update playwright.config.ts to include dynamic-schema.spec.ts alongside records.spec.ts when no OAuth-only URL is set; preserve the OAuth-only selection. The new tests must fail due to fixed navigation, not missing fixtures.
- [x] **Step 3: Implement discovery, labels and refresh.** Replace fixed object choices, keep existing translations for seeds and metadata label/API-name fallback for custom fields, mark required fields and maxlength from describe. Custom objects use Name; Contact keeps LastName. Use request generations/abort on switches to prevent stale commits. Refresh/disconnect clears actionable state. Add every new product message to all five catalogs; do not translate API names or tenant labels.
- [x] **Step 4: Run web tests, the new browser cases, existing connected-browser tests and `npm --prefix web run build`.** Expected all pass, including Hebrew layout and asynchronous boundary tests.
- [x] **Step 5: Commit** with `feat: discover and edit custom objects in the connected UI`.

### Task 8: Required acceptance, restart and operator handoff

**Files:** Create `compat/clients/node/src/candidate-schema.ts`, `compat/clients/node/test/candidate-schema.test.ts`. Modify `compat/clients/node/src/candidate-acceptance-{contract,preflight,service,fixtures,run,results}.ts`, their tests, `compat/schemas/candidate-acceptance.schema.json`, acceptance source bindings and `web/test/dynamic-schema.spec.ts` fixture wiring. Update `README.md`, `docs/compat/plan-27-dynamic-schema.md`, `docs/compat/plan-2-implementation.md`, `docs/plans/2026-10-20-progress-and-remaining-roadmap.md` and first release roadmap.

**Interfaces:** `runSchemaGate(c:GateContext):Promise<ParsedResult>` follows the existing OAuth gate/result conventions; extend gate ID and evidence enums with `schema`. Exact required IDs: `schema/provision`, `schema/jsforce-discover`, `schema/jsforce-describe`, `schema/jsforce-crud`, `schema/jsforce-query`, `schema/account-field`, `schema/tenant-isolation`, `schema/composite-rollback`, `schema/http-contract`. Add restart ID `restart/schema` and connected-browser IDs for dynamic CRUD, catalog isolation and stale response protection. Reuse existing fixture journal and secure evidence path.

- [x] **Step 1: Add failing harness tests.** Required schema gate absent/skipped/partial must make full acceptance ineligible; restart/schema absent fails too. Bootstrap roles and migrations once per run; service subprocess environment contains only its two limited URLs and never MIGRATION_DATABASE_URL. Test subprocess env redaction, fixture journal intent-before-action, cleanup recovery after schema creation failure and no raw credentials in evidence. The no-live-org path remains runnable.
- [x] **Step 2: Run `npm --prefix compat/clients/node run check`.** Expected new schema coverage assertions fail before gate wiring.
- [x] **Step 3: Implement acceptance integration.** Preflight verifies PostgreSQL/role-creation prerequisites and required credentials without logging them. Migrate and provision via owner subprocesses; launch serve with limited record+identity logins. Run schema after oauth, keep a custom record/definitions for the existing restart gate, assert same ID/value/catalog revision after restart, then clean up through journaled owner-only fixture code in reverse dependency order. No public destructive schema command is added. Include migrations, new packages and fixtures in source hashes. Add custom browser coverage to existing browser gate; unmodified JSforce proves all dynamic data behavior, CLI schema commands handle definitions.
- [x] **Step 4: Run final checks against disposable PostgreSQL with actual restricted roles.**

```bash
GOTOOLCHAIN=go1.25.0 go vet ./...
TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test -race ./... -count=1
npm --prefix compat/clients/node run check
npm --prefix web test
npm --prefix web run build
CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <exclusive-absolute-report-path>
```

Set TEST_DATABASE_URL to the dedicated disposable owner database; the harness derives restricted role connections without publishing passwords. Run the full command above with an exclusive report path and all prerequisites. Expected full acceptance eligible/passed, required schema/restart/browser IDs present with zero skips, old gates green and cleanup verified. Allowed legacy offline skips remain named. Report actual revision/source/lock bindings and report hash; do not claim a final-commit pass before the final revision is tested.

- [x] **Step 5: Finish operator and compatibility documentation.** Include DBA NOLOGIN-group/LOGIN setup, all three environment URLs without secrets, fresh install and v1/v2/v3 upgrade, separate status/up, stop/back-up/migrate/start, backup restoration rollback, head/checksum failures, owner-only schema examples and control-plane RLS exclusions. Record actual benchmark/environment, observations or missing live session, default naming/access policy, limitations and follow-up owners (28 types, 29 query, 33 permissions, 35 deployment, 36 setup/shell). Mark Plan 27 implemented only after evidence passes; change next priority to Plan 28 at that point. Link the spec/plan and decisions from README.
- [x] **Step 6: Commit** with `test: require dynamic schema acceptance and document Plan 27 operations`. Run full acceptance once more if final documentation changes source bindings; record the tested revision separately from any later evidence-only commit.

## Self-review and execution handoff

Coverage: migration/upgrade -> Task 2; dynamic definitions/DDL -> Task 3; runtime roles/RLS/context -> Task 4; one write path, query and Composite -> Task 5; HTTP/provenance -> Task 6; localized UI -> Task 7; actual clients/restart/cleanup/docs -> Task 8. Task 1 owns source availability and the physical-storage decision before schema commitment. All five Review Focus conditions have owning tests.

Execution order is sequential, Tasks 1–8. Metadata migration 0002 precedes role/RLS migration 0003; do not run those manifest edits in parallel. Native execution is a reasonable default for this tightly coupled package extraction; any delegation must follow the user's chosen execution approach. The maintainer authorized implementation after the planning commit was pushed. Execute to completion using the preserved native approach.
