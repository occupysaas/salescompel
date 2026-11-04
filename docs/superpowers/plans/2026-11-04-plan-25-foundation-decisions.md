# Plan 25 — Foundation Decisions Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Implement the chosen REST, query and external-ID defaults so Plans 5, 10, 12 and 14 no longer block work, proven through a required client gate.

**Architecture:** Add small normalization helpers in `internal/platform`, applied through the existing `create`, `update`, `Read`, `Select` and HTTP paths. Add a canonical external-ID column and catalog version 3 in the existing startup transaction. Extend the candidate OpenAPI contract and acceptance workflow, and record every decision and gap in one document.

**Tech Stack:** Go 1.25.0, pgx/v5, PostgreSQL 17 (`pg_c_utf8`), `golang.org/x/text` v0.29.0, Node >=22/TypeScript, JSforce 3.11.0.

**Spec:** [Plan 25 design](../specs/2026-11-04-plan-25-foundation-decisions-design.md). Read it before each task; its decision tables are the source of exact behavior.

## Global Constraints

- Work directly on `main`.
- No new dependency, lockfile or manifest change.
- Use `GOTOOLCHAIN=go1.25.0` and a disposable PostgreSQL 17 database through `TEST_DATABASE_URL`. A skipped database test does not count as passing.
- No live Salesforce org, private capture, credentials or external communication.
- Do not change `docs/compat/contracts/`, `docs/compat/profiles/`, `docs/compat/observations/`, `api/openapi.yaml`, `CLEAN_ROOM.md`, `internal/compat/externalid`, or historical captures and reports.
- Error messages and diagnostics never include record values, tokens, SQL or database error details.
- Pagination, `LIMIT` above 2,000 and Composite URL forms do not change.
- Existing tests change only where they assert behavior this spec replaces. Note each such change in the commit message.

Shared test command, referred to below as **DB-TEST**:

```sh
docker run -d --rm --name plan25-pg -e POSTGRES_PASSWORD=local-test-only -p 55432:5432 postgres:17
export TEST_DATABASE_URL='postgres://postgres:local-test-only@127.0.0.1:55432/postgres?sslmode=disable'
GOTOOLCHAIN=go1.25.0 go test -race ./internal/platform -count=1
```

Stop the container once all tasks finish: `docker stop plan25-pg`.

## Review Focus

- An existing database with keys that now collide must refuse to upgrade and leave every row unchanged; Task 4 pins this.
- Two 15-character IDs differing only by case are different records, so a lowercased ID must not resolve to the original; Task 1 pins this.
- A restricted user must not reach a denied field through a differently cased name, in a query or a write, including one duplicated beside an allowed spelling; Task 2 and Task 3 pin this.
- A key near the 80-character limit whose folded form is longer, such as 80 `ß`, must store and match without truncation; Task 4 pins this.
- An upsert racing with an ordinary POST using an equivalent key must leave one record, and the POST gets `DUPLICATE_VALUE` if it loses; Task 5 pins this.

---

### Task 1: Record IDs

**Files:**
- Create: `internal/platform/ids.go`, `internal/platform/ids_test.go`
- Modify: `internal/platform/store.go` (`newID`, `Read`, `update`, `deleteRecord`), `internal/platform/catalog.go`
- Test: `internal/platform/http_test.go`

**Interfaces:**
- Produces: `NormalizeRecordID(object, input string) (string, error)`, returning `errMalformedID` for any malformed value; `idChecksum(base string) string`.

- [ ] **Step 1: Write failing tests in `ids_test.go`**

`TestNormalizeRecordID`: a `newID("Account")` value resolves from its first 15 characters and from 18 characters with an upper-, lower- or mixed-case suffix, always to the original; the wrong prefix (`Contact` for a `001` ID), length 14, 16 and 19, non-alphanumeric characters, and a wrong suffix return `errMalformedID`. `TestNormalizeRecordIDCaseSensitiveBase`: changing the case of one letter in the first 15 characters returns a value different from the original, or `errMalformedID` when given 18 characters with the old suffix.

- [ ] **Step 2: Write failing DB tests in `http_test.go`**

`TestHTTPRecordIDForms`: GET, PATCH and DELETE work with a 15-character ID, and GET returns the 18-character `Id`. GET with a malformed or wrong-prefix ID, and a lowercased 15-character ID, return 404 `NOT_FOUND`. POST Contact with a malformed `AccountId` returns 400 `MALFORMED_ID` and `fields: ["AccountId"]`, and with a 15-character `AccountId` it stores and returns the 18-character form.

- [ ] **Step 3: Run tests to verify they fail**

Run: DB-TEST with `-run 'NormalizeRecordID|HTTPRecordIDForms'`. Expected: FAIL, undefined `NormalizeRecordID`.

- [ ] **Step 4: Implement `ids.go` and apply it**

Extract the existing checksum loop from `newID` into `idChecksum`, unchanged in output. Object prefixes stay `001` and `003`. Normalize in `Read`, `update` and `deleteRecord`, mapping `errMalformedID` to `dbError(pgx.ErrNoRows)`. Normalize `reference`-type string values at the start of `create` and `update`, on a copy of the caller's map, before `Validate`.

- [ ] **Step 5: Run tests to verify they pass**

Run: DB-TEST. Expected: PASS.

- [ ] **Step 6: Commit**

```bash
git add internal/platform
git commit -m "feat: accept 15-character record IDs and return 18"
```

### Task 2: API names, duplicate fields, errors and unknown objects

**Files:**
- Create: `internal/platform/names.go`, `internal/platform/names_test.go`
- Modify: `internal/platform/http.go` (`body`, object routing, upsert field routing), `internal/platform/catalog.go` (`field`, `Validate`), `internal/platform/store.go` (`create`, `update`, `Upsert`, `dbError`)
- Test: `internal/platform/http_test.go`, `internal/platform/composite_test.go`

**Interfaces:**
- Produces: `CanonicalObject(input string) (string, bool)`; `canonicalFields(p Principal, object string, data map[string]any) (map[string]any, error)`; `fieldProblem(code, message, field string) error`.
- Consumes: Task 1's normalization of reference values, which runs on the canonical names.

- [ ] **Step 1: Write failing tests**

`TestCanonicalFields`: `{"name": "x"}` becomes `{"Name": "x"}` without changing the caller's map; `{"Name": 1, "name": 2}` in one map returns 400 `INVALID_FIELD`; unknown names return `INVALID_FIELD`.

`TestHTTPCaseInsensitiveRoutes`: `/sobjects/account/describe`, POST `/sobjects/ACCOUNT`, GET `/sobjects/account/{id}` and PATCH `/sobjects/account/compat_key__c/{key}` succeed and return canonical names.

`TestHTTPDuplicateBodyFields`: raw body `{"Name":"first","Name":"second"}` stores `second`; `{"name":"first","Name":"second"}` stores `second`, and `{"Name":"first","name":"second"}` also stores `second`. A restricted principal sending `{"Name":"x","compat_text__c":"y"}` gets `INVALID_FIELD` and the record is unchanged.

`TestHTTPUnknownObjects`: GET `/sobjects/Nope/describe` and POST `/sobjects/Nope` return 404 `NOT_FOUND`; a query on `Nope` returns 400 `INVALID_TYPE`.

`TestHTTPFieldErrorEnvelope`: missing `Name`, over-long `Name`, writing `Id`, and a duplicate external ID each return an array with `errorCode`, `message` and the exact `fields` value (`["Name"]`, `["Name"]`, `["Id"]`, `["Compat_Key__c"]`).

`TestCompositeRejectsCaseDifferingFields`: one Composite body with `Name` and `name` returns 400 `INVALID_FIELD` for that subrequest and rolls back.

- [ ] **Step 2: Run tests to verify they fail**

Run: DB-TEST with `-run 'CanonicalFields|HTTPCaseInsensitive|HTTPDuplicateBody|HTTPUnknownObjects|HTTPFieldError|CompositeRejectsCase'`. Expected: FAIL.

- [ ] **Step 3: Implement**

`body` decodes with `json.Decoder` tokens to retain order, and collapses keys case-insensitively so the last key and value win; trailing-data and non-object errors keep their `JSON_PARSER_ERROR` behavior. `field` matches with `strings.EqualFold` and still applies `allowed` against the canonical name. `create`, `update` and `Upsert` call `canonicalFields` first, so Upsert's existing body-key check sees the canonical name. REST routes resolve the object with `CanonicalObject`, returning 404 `NOT_FOUND` if unknown. `Describe` and SOQL keep `INVALID_TYPE`.

- [ ] **Step 4: Run tests to verify they pass**

Run: DB-TEST. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform
git commit -m "feat: resolve API names case-insensitively with last-write-wins fields"
```

### Task 3: Query semantics in the existing parser

**Files:**
- Modify: `internal/platform/query.go` (`Query`, `ParseQuery`), `internal/platform/select.go`, `internal/platform/query_columns.go`, `internal/platform/http.go` (query route)
- Test: `internal/platform/query_test.go`, `internal/platform/store_test.go`, `internal/platform/query_columns_test.go`

**Interfaces:**
- Consumes: `CanonicalObject` and `field` from Task 2; `NormalizeRecordID` from Task 1.
- Produces: `canonicalQuery(p Principal, q Query) (Query, error)`; `Query.Nulls` of type `NullPlacement` with `NullDefault`, `NullFirst`, `NullLast`.

- [ ] **Step 1: Write failing tests**

`TestParseQueryNullPlacement`: `ORDER BY Name NULLS FIRST`, `ORDER BY Name DESC NULLS LAST` and lowercase forms parse; `NULLS` with no value and `NULLS MIDDLE` return `MALFORMED_QUERY`.

`TestStoreQueryCaseInsensitiveNames`: `select id, name from account` returns canonical `Id`, `Name` and `attributes.type: "Account"`; `select account.name from contact` returns the canonical nested shape; `columnMetadata` uses canonical names. A restricted principal querying, filtering or ordering by `compat_text__c` gets `INVALID_FIELD`.

`TestStoreQueryTextEquality`: on real PostgreSQL, `WHERE Name = 'mixedcase'` finds `MixedCase`; `é` and `É` match; composed `é` does not match decomposed `é`; `I` does not match `ı`.

`TestStoreQueryIDFilters`: `WHERE Id = '<15 chars>'` finds the record; `WHERE AccountId = '<15 chars>'` finds its contacts; a malformed value returns 400 `INVALID_QUERY_FILTER_OPERATOR`; a same-letters ID with different case does not match.

`TestStoreQueryNullOrdering`: default ASC and default DESC both list nulls first; explicit `NULLS LAST` lists them last in both directions; ties remain ordered by `Id`.

- [ ] **Step 2: Run tests to verify they fail**

Run: DB-TEST with `-run 'ParseQueryNull|StoreQuery'`. Expected: FAIL.

- [ ] **Step 3: Implement**

`Select` and the HTTP column-metadata path both use `canonicalQuery`. For `string` fields, compare with this exact form, because it case-maps without normalizing:

```sql
lower(r.<column> COLLATE pg_c_utf8) = lower($n COLLATE pg_c_utf8)
```

`id` and `reference` fields compare exactly after `NormalizeRecordID`. Order with `NULLS FIRST` unless `NULLS LAST` was requested, and keep the `,r.id` tie-break.

- [ ] **Step 4: Run tests to verify they pass**

Run: DB-TEST and `GOTOOLCHAIN=go1.25.0 go vet ./...`. Expected: PASS.

- [ ] **Step 5: Commit**

```bash
git add internal/platform
git commit -m "feat: apply case-insensitive names, text equality and null ordering in queries"
```

### Task 4: External-ID equivalence and catalog version 3

**Files:**
- Create: `internal/platform/externalid.go`, `internal/platform/externalid_test.go`
- Modify: `internal/platform/catalog_migration.go`, `internal/platform/store.go` (`columns`, `dbError`)
- Test: `internal/platform/catalog_migration_test.go`, `internal/platform/http_test.go`

**Interfaces:**
- Produces: `const externalIDRuleVersion = "trim-nfc-full-fold-v1"`; `canonicalExternalID(value string) (string, error)`; column `accounts.compat_key_canonical`; `const catalogVersion = 3`.

- [ ] **Step 1: Write failing vector tests**

`TestCanonicalExternalIDVectors` checks every row of the Plan 12 table with the relation the reference **observed**: `key`=`key`; `AbC`=`abc`; `é`=`é`; `I`=`i`; `I`≠`ı`; `ß`=`ss`; `/percent%é` retained literally; `key`=` key`; `key`=`key `. Also: tab, NBSP and interior space stay significant, so `key\t`≠`key` and `a b`≠`ab`; `%2F` stays literal and ≠`/`; `"   "` maps to `""`; invalid UTF-8 returns an error. The result is idempotent for every vector.

- [ ] **Step 2: Write failing migration tests**

Update the existing catalog tests to version 3 and add:

- `TestCatalogFreshV3`: fresh install has the column, the check and the new unique constraint, and not the old one.
- `TestCatalogUpgradeV2ToV3` and the existing `TestCatalogUpgradeV1`: rows keep their `compat_key` bytes and gain correct canonical values; a null key stays null; a repeat `Open` is a no-op.
- `TestCatalogV3RefusesCollisions`: seeded `AbC` and `abc` in one tenant makes `Open` fail, with an error containing neither value, and leave version, schema and rows unchanged. The same keys in two tenants succeed.
- `TestCatalogV3RefusesWhitespaceOnlyKey`: as above, for a key of `"   "`.
- `TestCatalogV3FoldedLengthBeyond80`: a seeded key of 80 `ß` upgrades, with a canonical value of 160 characters.

Add `TestHTTPOrdinaryExternalIDUniqueness`: POST or PATCH of `abc` while `AbC` exists on another record returns 400 `DUPLICATE_VALUE` with `fields: ["Compat_Key__c"]`, and both records are unchanged; PATCH of the same record to `abc` succeeds and stores `abc`; POST with `"   "` stores null; 80 `ß` round-trips, and an equivalent key of 160 `s` is too long and returns `STRING_TOO_LONG`.

- [ ] **Step 3: Run tests to verify they fail**

Run: DB-TEST with `-run 'CanonicalExternalID|CatalogFreshV3|CatalogUpgrade|CatalogV3|HTTPOrdinaryExternalID'`. Expected: FAIL.

- [ ] **Step 4: Implement**

`canonicalExternalID` follows the spec's rule, using `strings.Trim(value, " ")`, `norm.NFC`, and a new `cases.Fold()` per call. In `columns`, when the field is `Compat_Key__c`, write both columns, or null for both when the canonical form is empty. The migration runs v1→v2 when needed, then v2→v3 inside the same transaction and table lock: add the column, compute canonical values in Go from `SELECT tenant_id, id, compat_key`, refuse on the two spec conditions, write the values, add the check and new unique constraint, drop `accounts_tenant_id_compat_key_key`, and set version 3. The catalog definition JSON does not change. Extend the physical schema check to require the new column and constraint. Map a 23505 on the new constraint to `DUPLICATE_VALUE` with `fields`.

- [ ] **Step 5: Run tests to verify they pass**

Run: DB-TEST. Expected: PASS, including `TestCatalogConcurrentOpen`.

- [ ] **Step 6: Commit**

```bash
git add internal/platform
git commit -m "feat: enforce one external-ID equivalence with catalog version 3"
```

### Task 5: Upsert behavior and races

**Files:**
- Modify: `internal/platform/store.go` (`Upsert`), `internal/platform/http.go` (upsert route)
- Test: `internal/platform/store_test.go`, `internal/platform/http_test.go`

**Interfaces:**
- Consumes: `canonicalExternalID`, `compat_key_canonical` from Task 4.

- [ ] **Step 1: Write failing tests**

`TestUpsertPlan9Sequences` replays the Plan 9 table cases against the candidate, checking status and stored spelling after each step: `ascii`, `leading-space`, `trailing-space`, `accent`, `ascii-i`, `dotless-i` (two records), `sharp-s`, `encoded`, and `body-identical`, `body-variant`, `body-different` (Plan 11's 400 `INVALID_FIELD`).

`TestUpsertUpdateKeepsSpelling`: upsert of `abc` over `AbC` returns 200 with `created: false`, updates other fields and keeps `AbC`.

`TestUpsertWhitespaceOnlyKey`: `PATCH .../Compat_Key__c/%20%20` returns 400 `INVALID_FIELD`, creating nothing.

`TestUpsertConcurrentVariants`: 16 concurrent upserts mixing `AbC`, `abc` and ` abc ` give exactly one record, exactly one `created: true`, no errors, and a stored spelling matching the creating request. `TestUpsertTenantIsolation`: the same variants in two tenants create one record each.

`TestUpsertRacingPost`: repeated concurrent upsert and POST with equivalent keys always leave one record per round; the POST succeeds or returns `DUPLICATE_VALUE`, and any upsert result is a success or `DUPLICATE_VALUE`.

- [ ] **Step 2: Run tests to verify they fail**

Run: DB-TEST with `-run 'Upsert'`. Expected: FAIL in the new tests.

- [ ] **Step 3: Implement**

Compute the canonical key after the body checks and return `INVALID_FIELD` if it is empty. Take the advisory lock on tenant plus canonical key, and find the row with `compat_key_canonical=$2 ... FOR UPDATE`. Create with the supplied spelling, and on update remove `Compat_Key__c` from the map before calling `update`. If an ordinary POST inserts between the lookup and the create, the database constraint returns `DUPLICATE_VALUE`, which is the behavior the race test allows.

- [ ] **Step 4: Run tests to verify they pass**

Run: DB-TEST with `GOTOOLCHAIN=go1.25.0 go vet ./...`. Expected: PASS, including existing Plan 11 tests.

- [ ] **Step 5: Commit**

```bash
git add internal/platform
git commit -m "feat: serialize upserts on the canonical external ID and keep stored spelling"
```

### Task 6: Served contract and required acceptance gate

**Files:**
- Create: `compat/clients/node/test/candidate-foundation.test.ts`
- Modify: `api/candidate-openapi.yaml`; `compat/clients/node/src/candidate-acceptance-contract.ts`, `candidate-acceptance-results.ts`, `candidate-acceptance-run.ts`; `compat/schemas/candidate-acceptance.schema.json`; `compat/clients/node/test/candidate-acceptance-contract.test.ts`, `candidate-acceptance-results.test.ts`; offline tests `candidate-openapi-contract.test.ts` and `candidate-openapi-traffic.test.ts` if the document change requires them
- Reference: `compat/clients/node/test/candidate-composite.test.ts` for the owned-fixture pattern

**Interfaces:**
- Consumes: HTTP behavior from Tasks 1–5; `candidateRequest`, `createFixture` and `recordFixtureEvent` from `candidate-acceptance-fixtures.ts`.
- Produces: required gate `candidate/foundation/decisions`.

- [ ] **Step 1: Write failing offline tests**

Add `candidate/foundation/decisions` to the expected identities in `candidate-acceptance-contract.test.ts` and `candidate-acceptance-results.test.ts`, alongside `candidate/composite/atomic`. Add a negative: a result missing this identity fails.

Add contract tests asserting that `recordId` and external-ID descriptions state the accepted forms, and that the record and describe routes declare 404 with the `Errors` schema.

- [ ] **Step 2: Run tests to verify they fail**

Run: `npm --prefix compat/clients/node test`. Expected: FAIL for the new identity and contract assertions.

- [ ] **Step 3: Implement the registration and contract**

Add the name to `candidateNames`, the results list that treats it like `candidate/composite/atomic`, the test argv, and the schema's permitted skip identities. Update the contract's descriptions and any missing 404 responses. Keep `additionalProperties: false` on `Error`.

- [ ] **Step 4: Write the live gate**

`candidate/foundation/decisions`, skipped unless `RUN_CANDIDATE=1`, journals every created ID and validates each exchange with `createCandidateTrafficValidator`. Using real JSforce where the operation exists and raw HTTP otherwise, it asserts:

- `retrieve` and `update` with a 15-character ID, returning 18 characters.
- `select id, name from account where name = '<lowercased>'` returns canonical keys and the record.
- Default ordering lists null `Compat_Text__c` first, with `ORDER BY compat_text__c DESC` as well; `NULLS LAST` lists them last.
- JSforce `upsert` with `AbC`, then `abc`, then ` abc ` reports created, then not created twice, and the record keeps `AbC`.
- POST with an equivalent key returns 400 `DUPLICATE_VALUE`.
- The restricted principal is denied `compat_text__c` by query and by write.
- The other-tenant principal cannot see the record and can create its own with an equivalent key.
- An unknown REST object returns 404 `NOT_FOUND`, and an unknown SOQL object returns 400 `INVALID_TYPE`.

- [ ] **Step 5: Run offline checks**

Run: `npm --prefix compat/clients/node run check`. Expected: PASS, with the new gate listed among the explicit opt-in skips.

- [ ] **Step 6: Run full acceptance**

Run, with the setup in `docs/compat/plan-15-candidate-acceptance.md` and a dedicated disposable database:

```sh
scripts/check-candidate-acceptance.sh --out "$SCRATCH/plan25-acceptance.json"
```

Expected: exit 0, all gates passed, `candidate/foundation/decisions` passed, and verified cleanup with zero unknown intents. Fix any existing client gate this plan's behavior changes break by correcting the candidate, not the client.

- [ ] **Step 7: Commit**

```bash
git add api compat
git commit -m "feat: require foundation decision acceptance through real clients"
```

### Task 7: Decision record and closure

**Files:**
- Create: `docs/compat/plan-25-foundation-decisions.md`
- Modify: `docs/compat/plan-5-contract-decisions.md`, `plan-10-foundation-decision.md`, `plan-12-external-id-equality.md`, `plan-14-external-id-normalization-contract.md` (dated note only); `docs/compat/implementation-log.md`; `docs/superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md` (Plan 25 record); `README.md` (catalog upgrade section and next priority); this plan's status line

- [ ] **Step 1: Write the decision record**

Include:

- One row for each of the twelve contract IDs in `suite("rest-query-foundation").contractIds` (`compat/clients/node/src/suites.ts`), and for `upsert-outcomes` if it is not among them, with decision, label from the spec, code location, test, and whether Plan 25 or Plan 29 implements it.
- The Plan 14 dimensions, each marked as a gap with the default used.
- The spec's gap list, and the Plan 29 decision table by reference.
- Catalog v3 upgrade and rollback notes: stop old instances, back up first, how to find and resolve refused rows without exposing values, and that rollback means restoring the backup.
- Actual results of the commands in Step 3, including skips.

- [ ] **Step 2: Add closure notes and links**

Each of the four historical documents gets one dated section stating that Plan 25 chose defaults, linking to the decision record, and stating that earlier evidence and outcomes are unchanged. Update the README's catalog section from version 2 to 3, and replace its next priority with Plan 26. Add the implementation record to Plan 25 in the roadmap. Add a log entry.

- [ ] **Step 3: Run final verification**

Run:

```sh
GOTOOLCHAIN=go1.25.0 go test -race ./... -count=1
GOTOOLCHAIN=go1.25.0 go vet ./...
npm --prefix compat/clients/node run check
npm --prefix web test
npm --prefix web run build
scripts/check-candidate-acceptance.sh --out "$SCRATCH/plan25-final.json"
git diff --check
```

Expected: all exit 0. `git diff --stat 7151727 -- docs/compat/contracts docs/compat/profiles docs/compat/observations api/openapi.yaml CLEAN_ROOM.md internal/compat/externalid` prints nothing.

- [ ] **Step 4: Commit**

```bash
git add README.md docs
git commit -m "docs: record Plan 25 foundation decisions and close the evidence waits"
```
