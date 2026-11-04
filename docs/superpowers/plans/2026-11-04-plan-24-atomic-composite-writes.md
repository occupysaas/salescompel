# Plan 24 — Atomic Composite Writes Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Deliver a bounded atomic Account/Contact Composite write endpoint with measured rollback and required candidate acceptance.

**Architecture:** Strictly prevalidate the entire request, then execute ordered writes through existing validation/storage helpers in one pgx transaction. Extend the embedded candidate OpenAPI contract and existing owned acceptance workflow; never dispatch subrequests over HTTP.

**Tech Stack:** Go 1.25.0, pgx/v5, PostgreSQL 17, existing Node >=22/TypeScript, Ajv2020 and OpenAPI 3.2.1 tooling.

**Spec:** [Plan 24 design](../specs/2026-11-04-plan-24-atomic-composite-writes-design.md). Status: proposed, not executed. Read both documents before execution; topic selection is an author recommendation for review.

## Global Constraints

- Work directly on main under the repository convention.
- Use Go 1.25.0, PostgreSQL 17 and the existing compat Node >=22 toolchain with pinned dependencies; no new dependency, lockfile or manifest changes.
- Preserve existing standalone CRUD/query/upsert/authentication behavior, catalog/schema and UI; only the bounded Composite route and its discovery/contract entries add product behavior.
- Preserve api/openapi.yaml, CLEAN_ROOM.md and historical plans, profiles, observations and reports; append new source-registry entries only.
- Use synthetic fixtures and the existing disposable candidate acceptance workflow; no live Salesforce org, proprietary implementation, copied examples or external communication.
- Never publish tokens, Authorization headers, record values, raw request/response bodies, SQL or database error details in diagnostics or acceptance reports.
- Atomic success is reported only after PostgreSQL commit succeeds; any failure before commit must leave no partial writes.

## Review Focus

- A structurally invalid final subrequest must prevent all earlier writes; Task 1 tests full-request prevalidation and Task 2 checks the database.
- A SQL constraint failure or canceled context must undo earlier writes using the same transaction; Task 2 tests actual persistence and cleanup-context behavior.
- A commit failure must never expose generated IDs or success results; Task 2 injects the failure through a private transaction seam.
- An unknown Composite response may hide committed creates; Task 4 leaves intents unresolved and proves cleanup/publication failure.
- Empty httpHeaders must not weaken record/envelope schemas; Task 3 tests the exact empty-map exception and rejects permissive replacement records.

---

## File map and execution boundaries

| Action/path | Responsibility |
| --- | --- |
| Create `internal/platform/composite.go`, `composite_test.go` | Wire types, typed validation entry, transactional executor and DB regressions. |
| Create `internal/platform/http_composite.go`, `http_composite_test.go` | Strict decoder, full structural/route/reference validation, HTTP behavior. |
| Modify `internal/platform/store.go` | Extract deleteRecord(database); preserve standalone Delete. |
| Modify `internal/platform/http.go` | Authenticated Composite branch and one discovery link. |
| Modify `api/candidate-openapi.yaml` | Twentieth operation, discovery and bounded Composite schemas. |
| Modify `compat/clients/node/src/candidate-openapi-{contract,traffic}.ts` | Exact inventory and maxProperties keyword support. |
| Modify `compat/clients/node/test/candidate-openapi-{contract,traffic}.test.ts`, `candidate-openapi.test.ts` | Offline contract/traffic checks and twenty-operation live coverage. |
| Create `compat/clients/node/test/candidate-composite.test.ts` | Live atomic write/rollback/policy journey. |
| Create `compat/clients/node/src/candidate-composite-fixtures.ts`, `test/candidate-composite-fixtures.test.ts` | Private per-create journal reconciliation for Composite acceptance. |
| Modify `compat/clients/node/src/candidate-acceptance-{contract,results,run}.ts` and their corresponding tests | Required identities, identity preservation and explicit test invocation. |
| Modify `compat/schemas/candidate-acceptance.schema.json` | Exact permitted offline skip identity; preserve full acceptance strictness. |
| Append `docs/compat/sources.yaml` | Dated public documentation and independent repository provenance. |
| Modify `README.md`, `docs/compat/plan-2-implementation.md` | Current restricted Composite support and limitations. |
| Create `docs/compat/plan-24-atomic-composite-writes.md` | Measured execution report, never planned passes. |

Four sequential tasks. Go/git/full-acceptance commands run at repository root. Node commands run at `compat/clients/node/`. Existing testStore provisions real test principals; skipped database tests do not prove RED/GREEN. Use the current acceptance prerequisites and disposable database, not a new provisioning system. Do not execute product implementation until this proposed design/plan has been reviewed under the session's instructions.

### Task 1: Define and prevalidate the bounded request

**Files:** Create composite.go for wire types; create http_composite.go and http_composite_test.go for decoder/validation. No HTTP route yet.

**Interfaces:**
- Produces the four exported wire structs in the spec, with exact JSON tags.
- Produces `decodeComposite(r io.Reader) (CompositeRequest, error)` and `validateComposite(request CompositeRequest) error`. The decoder retains raw presence to reject DELETE body:null and unknown fields before conversion; typed nil represents absent.
- Invalid input returns APIError status 400, code JSON_PARSER_ERROR for JSON/byte-limit failure or INVALID_COMPOSITE_REQUEST for structural/subset failure, with static nonreflecting messages.
- Private route/reference representations are local builder choices. Store.Composite will reuse validateComposite, not trust decoder-only checks.

- [ ] **Step 1: Establish execution baseline.** Read CLEAN_ROOM.md, spec, current HTTP/store/catalog/schema, fixture journal and candidate-contract/acceptance code. Record base revision, clean/dirty status and protected-path hashes in the new execution report. Re-read the three linked public sources and append exact source/version/access/applicability entries only; distinguish the existing observed rollback from new candidate decisions.
- [ ] **Step 2: Write failing pure tests.** Add `TestDecodeComposite` and `TestValidateComposite` table cases. Assert 1 and 25 operations accepted, 0/26 rejected; allOrNone missing/false/null, collate true/null, unknown fields, null arrays/elements, duplicate IDs, empty/81-character/illegal IDs, lowercase methods and trailing JSON rejected. Test exact 80-character boundary and case-sensitive distinct IDs. Assert POST/PATCH body required/non-null, DELETE body absent versus explicit null, and invalid-field map accepted for execution-time validation.
- [ ] **Step 3: Add route/reference/privacy cases.** Reject absolute/protocol-relative URLs, query/fragment, escapes, traversal, repeated/trailing slash, nested Composite, other versions/objects, GET/upsert and extra segments. Accept literal 18-character IDs, earlier `.id` in a complete row segment and Contact.AccountId referencing Account POST. Reject forward/self/unknown/non-POST/wrong-case/indexed/embedded references, reference property keys and placeholders in ordinary text fields. Inject a private marker into every invalid input and assert APIError messages never contain it.
- [ ] **Step 4: Run RED.** `GOTOOLCHAIN=go1.25.0 go test ./internal/platform -run 'TestDecodeComposite|TestValidateComposite' -count=1 -v`; expect missing symbols or assertion failures, not a DB skip.
- [ ] **Step 5: Implement types and validation.** Use private raw wire structs plus json.Decoder unknown-field/trailing-value checks. Enforce the complete request before returning it. Keep data-field validation in existing Validate for Task 2; do not validate unknown writable fields as structural errors. Handle http.MaxBytesError as JSON_PARSER_ERROR without exposing decoder prose.
- [ ] **Step 6: Verify and commit.** Rerun focused pure tests, gofmt touched Go files and `git diff --check`. Commit named files and source entries with `feat: validate the atomic Composite request subset`.

### Task 2: Execute one database transaction and expose authenticated HTTP

**Files:** Complete composite.go; create composite_test.go; modify store.go/http.go; extend http_composite.go/test.go.

**Interfaces:**
- Consumes Task 1 request/validator/types and existing `create(ctx, db, p, object, data)`, `update(ctx, db, p, object, id, data)`.
- Produces `deleteRecord(ctx context.Context, db database, p Principal, object, id string) error`, with standalone Delete delegating through s.pool.
- Produces `func (s *Store) Composite(ctx context.Context, p Principal, request CompositeRequest) (CompositeResponse, error)`; returns ordered committed results or rewritten rollback results, with outer errors reserved for infrastructure failures.
- Produces POST `/services/data/v65.0/composite` and discovery key composite. Private transaction begin/commit seam is allowed only for deterministic failure tests; production uses s.pool.Begin and one pgx.Tx.

- [ ] **Step 1: Write failing transaction tests.** `TestCompositeAtomic` covers Account POST followed by Contact POST with AccountId reference, create/PATCH/DELETE ordering, 25 successful operations and all IDs visible after commit. Assert no changes on typed-call prevalidation failure. Assert the observed create+invalid-field request returns statuses `[400,400]`, codes `[PROCESSING_HALTED,INVALID_FIELD]` and exact-key query count 0. Verify rollback restores an earlier PATCH and DELETE to their original data; later operations never execute.
- [ ] **Step 2: Add failure/policy regressions.** Use a real FK/unique violation after a valid first write to prove SQL errors roll back. Restricted forbidden-field write and cross-tenant update/delete return existing API errors with zero partial effects. Force context cancellation after the first write and prove a second connection sees no committed row. Through a private transaction seam inject begin, commit and rollback failure; assert error results have no response successes/IDs and rollback receives a bounded uncanceled cleanup context. Do not manufacture a real ambiguous commit and then claim certainty about its effects.
- [ ] **Step 3: Write failing HTTP tests.** `TestHTTPCompositeAtomic` checks outer POST 200, exact per-entry envelope/body/status/header shapes, omitted collate support, invalid envelope 400 and auth 401. Check GET/HEAD 405 with Allow POST (real HEAD has empty body), unsupported version 404, inherited security/no-store headers, discovery and >1,048,576 body bytes rejected before writes. Use private marker inputs to prove static parser/infrastructure diagnostics. Compare standalone CRUD errors/effects before and after the delete extraction.
- [ ] **Step 4: Run RED on the disposable PostgreSQL 17 database.** `GOTOOLCHAIN=go1.25.0 go test ./internal/platform -run 'TestCompositeAtomic|TestHTTPCompositeAtomic' -count=1 -v`; require real failures/no skipped DB tests.
- [ ] **Step 5: Implement the executor and route.** Revalidate before Begin, execute sequentially on one transaction using the original Principal, resolve only compiled earlier-create references, and stop at the first write error. After successful rollback rewrite every nonfailing result to the spec's PROCESSING_HALTED envelope. Return successes only after commit. Infrastructure failure returns the existing generic outer 500, including rollback failure. Use a bounded independent cleanup context for rollback. Authenticate once at the existing HTTP boundary; no loopback request, savepoint or per-write pool call.
- [ ] **Step 6: Verify GREEN and commit.** Run the same focused tests with actual DB coverage, then `GOTOOLCHAIN=go1.25.0 go test ./internal/platform -run 'TestHTTPBoundary|TestHTTPUpsert|TestStore|TestCompositeAtomic|TestHTTPCompositeAtomic' -count=1`. Inspect selected tests actually present in output; existing full acceptance owns broader race/vet/client verification. gofmt and commit `feat: execute atomic Account and Contact Composite writes`.

### Task 3: Extend the served contract without weakening validation

**Files:** Modify candidate-openapi.yaml; candidate-openapi-contract.ts/traffic.ts and their two offline tests. Live candidate-openapi.test.ts updates belong to Task 4 so the operation inventory change is delivered with its journey.

**Interfaces:**
- Consumes Task 2 HTTP behavior and Task 1 exact accepted wire format.
- Produces exact new operation tuple `['/services/data/v65.0/composite','post','candidateCompositeAtomic',['200']]` in candidateOperations; total inventory twenty.
- Existing `validateCandidateOpenAPI` and `createCandidateTrafficValidator` signatures remain unchanged. Add maxProperties support and one explicit closed-empty-map schema exception, not arbitrary schema support.

- [ ] **Step 1: Write failing document tests.** Extend the independent expected operation list; assert Composite allOrNone const true, maxItems 25/minItems 1, optional false collation, concrete request/result branches, required referenceId pattern/maxLength and new resource-discovery key. Reject allOrNone false, 26 writes, unknown envelope/subrequest fields, arbitrary success bodies and nonempty httpHeaders. Replace the old unsupported-Composite mutation with `/services/data/v65.0/composite/batch` and retain unknown-operation rejection.
- [ ] **Step 2: Write failing traffic tests.** Supply valid linked-write, PATCH/DELETE null-body and rollback envelopes. Assert valid exchanges have empty requestIssues/responseIssues; invalid-field request has request-schema while its rollback response has no response issues. Mutate maxProperties, result required fields, method/body pairing and wrong status/media. Prove the permitted empty map validates only `{}`, and permissive `{}` record replacement still fails construction. Preserve the existing 1 MiB/8 MiB budgets and private diagnostics tests.
- [ ] **Step 3: Run RED.** `node --import tsx --test test/candidate-openapi-contract.test.ts test/candidate-openapi-traffic.test.ts`; expect inventory/schema/support failures against the nineteen-operation contract.
- [ ] **Step 4: Implement YAML and narrow validator changes.** Use concrete closed components, method/path oneOf branches and existing writable schemas with Contact.AccountId reference support. Add the operation/source URLs and discovery entry. Encode deterministic field shapes and document transaction/reference semantics separately. Permit a typed empty object only with additionalProperties false and maxProperties 0; add maxProperties to the schema keyword allowlist and validate through existing Ajv. Do not weaken record/envelope guards or change api/openapi.yaml.
- [ ] **Step 5: Verify and commit.** Run both focused tests, `npm run typecheck`, `npm run validate` and targeted pinned Prettier checks; expect all pass. `GOTOOLCHAIN=go1.25.0 go test ./api -count=1` verifies embedded-source behavior. Commit `feat: document the served atomic Composite subset`.

### Task 4: Require live atomicity and publish measured acceptance

**Files:** Create candidate-composite-fixtures.ts/test, candidate-composite.test.ts; modify candidate-openapi.test.ts, the three acceptance modules/tests, acceptance report schema and current README/guide; complete the new execution report.

**Interfaces:**
- Consumes existing FixtureEvent/recordFixtureEvent, Requester/Response and cleanupFixtures without changing their event schema or global semantics.
- Produces `runCompositeFixtureAttempt(journalPath:string, intents:FixtureEvent[], expected:readonly {referenceId:string; method:"POST"|"PATCH"|"DELETE"}[], outcome:"commit"|"rollback"|"prevalidation", perform:()=>Promise<Response>, reconcileAbsent:(intent:FixtureEvent)=>Promise<void>):Promise<Response>` in candidate-composite-fixtures.ts. Before any dispatch, check expected identities are unique and create intents correspond exactly to its POST entries. Persist every intent before perform; callback must reconcile exact synthetic markers, journal discovered IDs as specified below, and throw unless absence is independently established. Return raw data only privately to the caller; exceptions are bounded categories.
- Journal every independently attributable POST success ID before whole-envelope validation or assertions, even when another entry is malformed; duplicate response identities never resolve an intent. Use expected to require exactly one response per subrequest in order, with POST 201 and PATCH/DELETE 204 on commit, or exactly one original API error plus PROCESSING_HALTED for all other entries on rollback. Match the caller's expected outcome; an expected prevalidation response must be outer 400 with the declared static error envelope. A recognized complete rollback or expected prevalidation 400 requires reconcileAbsent for every intent before recording rejected. An invalid/ambiguous/500/transport result leaves unresolved intents, even if HTTP transport succeeded. Derive POST response associations by expected referenceId supplied in each intent's operationId; use that exact referenceId as operationId, unique within the run.
- Produces parent `candidate/composite/atomic`, preserved unprefixed; adds `TestCompositeAtomic` and `TestHTTPCompositeAtomic` required DB identities. Existing candidate/openapi/traffic remains independently required with twenty successful operation IDs.

- [ ] **Step 1: Write failing fixture tests.** `candidate-composite-fixtures.test.ts` asserts all intents durably recorded before dispatch, zero dispatch on a failed intent append, both create IDs recorded before a later caller assertion, and exact-ID cleanup order Contact then Account. Test complete rollback requires independent absence checks; a discovered row is journaled before failure. Omit a non-create response and assert that rollback is not considered complete. Return one valid success plus a malformed neighboring entry and assert the attributable ID is recorded before failure. Duplicate marker results must produce distinct recovery intent/known pairs, retain the original unresolved intent and still permit exact-ID cleanup of discovered rows. Transport/invalid JSON/duplicate or missing response identity/outer 500/invalid success ID leaves unknownIntents >0 and cleanup.verified=false. Existing report aggregation must reject this outcome; do not mark unknown outcomes rejected.
- [ ] **Step 2: Run RED and implement fixture bookkeeping.** `node --import tsx --test test/candidate-composite-fixtures.test.ts`; expect missing module/helper, then implement the signature above and rerun to pass. Keep callbacks/request content private; never include response data in thrown errors. Reuse recordFixtureEvent and existing cleanup rather than inventing blind deletion.
- [ ] **Step 3: Write failing acceptance-wiring tests.** In contract/results/run tests, assert missing/skipped/new identity fails required client coverage; omitted/skipped new Go parents fail database coverage; native reporter keeps the parent exactly. Assert candidateNodeTestArgs now includes candidate-composite.test.ts after its two existing test files. Extend the report schema's exact offline allowedSkipped enum and real publication regression. Run `node --import tsx --test test/candidate-acceptance-contract.test.ts test/candidate-acceptance-results.test.ts test/candidate-acceptance-run.test.ts`; expect missing identity/file/schema failures.
- [ ] **Step 4: Implement the wiring and live tests.** Add identity preservation, required lists, explicit file inclusion and offline allowed skip. Create the opt-in parent using RUN_CANDIDATE=1, existing HTTPS candidate/token/run-ID/journal environment and bounded requests. Validate actual traffic against served YAML. Implement independent persistence assertions for linked create, update/delete success, invalid-field rollback, update/delete restoration, denied restricted field and cross-tenant writes. Journal all possible creates through Step 2 before dispatch. Build reconcileAbsent with exact run-owned Name/LastName or Compat_Key__c queries and LIMIT 26. Record a unique match against the original intent. For duplicates, record each returned ID under a fresh intent/known pair with operationId `<original-operationId>__recovered_<recordId>`, retain the original unresolved intent and throw. A full 26-row page signals possible overflow and never establishes complete recovery. No raw logs. Add one tracked one-write Composite exchange to candidate/openapi/traffic and update its independent successful inventory to twenty.
- [ ] **Step 5: Observe live RED then GREEN.** Run the new opt-in test against the owned candidate with an initially missing assertion/journey, then complete that journey and verify it passes with no skip. Use the existing acceptance environment and reporter; do not break product code to manufacture RED. Rerun the focused fixture and wiring tests after live changes. Format only touched TS files with the pinned formatter.
- [ ] **Step 6: Verify offline and commit runnable changes.** Run `npm run check` and `git diff --check <execution-base>`. Offline candidate parents may skip only by exact registered identity. Update current documentation with the explicit restricted subset and deferred modes. Commit all intended implementation files before full acceptance, which requires a clean source-bound revision, using `feat: require atomic Composite candidate acceptance`.
- [ ] **Step 7: Run full acceptance.** With the dedicated PostgreSQL 17 DATABASE_URL (matching TEST_DATABASE_URL if set), pinned Go 1.25.0 on PATH and existing browser/client prerequisites, run `CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <new-private-summary>`. Require all eleven gates pass, the new parent and both Go identities present/passed, twenty operation IDs asserted and fixture/process cleanup verified with zero unknown intents. Record actual tool versions/counts/skips/digests. Missing prerequisites are owned blockers on execution completion, never evidence of success.
- [ ] **Step 8: Review, correct and publish.** Obtain a fresh whole-change review focused on transaction-handle use, rollback/commit errors, strict parsing/reference confinement, policy/tenant propagation, truthful schemas and fixture uncertainty. Reproduce material findings with regressions and recheck affected areas; rerun full acceptance when corrections change its result. Verify protected hashes and append-only source provenance. Complete the new execution report with measured evidence and limitations, update this plan/spec status only then, and commit `docs: publish atomic Composite acceptance`.

## Self-review and handoff

Task 1 covers full structural validation and privacy; Task 2 covers the observed rollback plus new candidate transaction/policy/error semantics; Task 3 covers truthful serving and the closed schema subset; Task 4 covers durable fixtures, required live coverage, review and publication. Each Review Focus condition has an owning regression. The plan changes neither historical nineteen-operation evidence nor existing milestone decisions. No implementation, DB test or acceptance pass is claimed by writing these documents.

Review the recommended topic and full spec before execution. Native execution fits four tightly connected tasks; subagent-driven execution is also supported by the required execution skills if selected later. No execution method has been selected in this documentation task. The fresh builder check identified two fixture-contract gaps; both were corrected by providing expected subrequest metadata and assigning distinct recovery identities without resolving ambiguous original intents.
