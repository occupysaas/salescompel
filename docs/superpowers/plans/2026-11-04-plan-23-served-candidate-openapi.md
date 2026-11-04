# Plan 23 — Served Candidate OpenAPI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Serve a truthful candidate API contract and require actual candidate HTTP exchanges to conform to it in acceptance tests.

**Architecture:** A new canonical YAML contract is embedded in the Go binary and served through the existing authenticated v65.0 API. Existing Node OpenAPI validation and Ajv2020 support a narrowly scoped test-time request/response validator. A new raw-HTTP candidate test joins the current acceptance runner and required-result coverage without removing existing client checks.

**Tech Stack:** Go 1.25.0/embed/net/http; compat Node >=22, TypeScript 7.0.2, tsx 4.23.15, yaml 2.9.1, Ajv 8.20.0, @seriousme/openapi-schema-validator 2.11.0; existing PostgreSQL 17 candidate acceptance prerequisites.

**Spec:** [Plan 23 design](../specs/2026-11-04-plan-23-served-candidate-openapi-design.md). Implemented, independently reviewed and verified, 2026-11-04; [results and limits](../../compat/plan-23-served-candidate-openapi.md).

## Global Constraints

- Work directly on main under the repository convention.
- Use Go 1.25.0 and the existing compat Node >=22 toolchain with pinned dependencies; no new dependency, lockfile or manifest changes.
- Author the candidate document as OpenAPI 3.2.1 YAML with JSON Schema 2020-12-compatible component schemas; preserve api/openapi.yaml as the broader proposal.
- Preserve existing CRUD/query/authentication semantics, database/catalog/schema, UI and accepted Salesforce profiles/observations; only contract serving and its discovery link add product behavior.
- Preserve historical Plans 10/14/17/18/19/20/21/22 and CLEAN_ROOM.md; source-registry changes append new entries only.
- Use synthetic fixtures and the existing authorized disposable candidate acceptance workflow; no live Salesforce org, proprietary implementation, copied examples, new source reuse or external communication.
- Never log, persist or return bearer tokens, raw Authorization headers, record values or request/response bodies in traffic-validator diagnostics.

## Review Focus

- A literal describe path must outrank a record-ID template, and %2F/%252F external IDs must remain one raw segment (Task 3).
- Projection/null/restricted-field shapes must validate without accepting unknown response properties or treating optional fields as access grants (Tasks 1, 3, 4).
- A successful-looking undocumented status or a 204 body must fail validation; unmatched methods/versions and actual HEAD use explicit direct assertions (Tasks 2–4).
- The new raw-HTTP identity must survive result parsing and be required; omitted/skipped/renamed identities must fail acceptance (Task 4).
- A traffic-validation assertion must not strand a created record before its fixture intent/known ID is durably journaled (Task 4).

## File Structure

| Path | Responsibility |
| --- | --- |
| Create `api/candidate-openapi.yaml` | Sole source of the exact nineteen-operation contract. |
| Create `api/candidate_contract.go`, `api/candidate_contract_test.go` | Immutable embedded YAML accessor/path/media constants and source-byte checks. |
| Modify `internal/platform/http.go` | Authenticated contract GET and one resource-discovery link. |
| Create `internal/platform/http_openapi_test.go` | DB-backed route/auth/method/deployment behavior. |
| Create `compat/clients/node/src/candidate-openapi-contract.ts` | Fixed contract loading and candidate-specific document/provenance invariants. |
| Create `compat/clients/node/src/candidate-openapi-model.ts`, `candidate-openapi-traffic.ts` | Test-only exchange/result interfaces and bounded schema validation. |
| Create `compat/clients/node/test/candidate-openapi-contract.test.ts`, `candidate-openapi-traffic.test.ts` | Offline document/validator regressions. |
| Create `compat/clients/node/test/candidate-openapi.test.ts` | Opt-in live candidate HTTP inventory, behavior and schema assertions. |
| Modify `compat/schemas/candidate-acceptance.schema.json` | Permit the exact new parent identity among offline allowed skips; preserve strict publication. |
| Modify `compat/clients/node/src/validate.ts` | Add the candidate contract/invariants to normal offline validation. |
| Append `docs/compat/sources.yaml` | New source/version/read-boundary records; existing entries unchanged. |
| Modify `compat/clients/node/src/candidate-acceptance-{contract,results,run,preflight}.ts` | Required identities, identity preservation and explicit test-file invocation. |
| Modify `compat/clients/node/test/candidate-acceptance-{contract,results,run,preflight}.test.ts` | Prove new identity/Go test coverage and runner inclusion. |
| Modify `README.md`, `docs/compat/plan-2-implementation.md` | Current serving/setup statements only; preserve historical evidence. |
| Create `docs/compat/plan-23-served-candidate-openapi.md` | Actual commands, coverage/results, sources, limitations and review. |

All Node development commands below run from `compat/clients/node/`; Go/git/full-acceptance commands run from repository root. Four sequential tasks. Internal document/schema representations and small test helpers are builder choices; do not build a general OpenAPI router, proxy or capture service.

### Task 1: Author the canonical candidate contract and offline invariants

**Files:** Create candidate-openapi.yaml, candidate-openapi-contract.ts and its test at the paths above; modify validate.ts; append sources.yaml; begin the Plan 23 execution report.

**Interfaces:**
- Consumes: `validateOpenApi(document, 'salescompel')` from openapi.ts, existing yaml parser, ROOT and registered source URLs.
- Produces: `loadCandidateOpenAPI(): Promise<unknown>` loads only fixed canonical YAML and validates it against registered sources; `validateCandidateOpenAPI(document:unknown, sourceUrls:ReadonlySet<string>): Promise<void>` enforces full OpenAPI validity plus the spec's fixed inventory/provenance/candidate-shape invariants. Fail with bounded category-only errors, without input data.

- [x] **Step 1: Read policy and establish boundaries.** Read CLEAN_ROOM.md, spec, current HTTP/catalog/select/query-column code and validator/acceptance paths. Record base SHA/clean status and protected hashes except the explicit modification list. Re-read the spec-linked standards and record sections/dates/version uncertainty. Reproduce the minimal existing-validator 3.2.1/Ajv2020 compatibility probe before authoring the full contract; do not change dependency versions to force it through.
- [x] **Step 2: Write failing document tests.** `candidateContractInventory` independently lists the nineteen operationIds from the spec and exact method/path pairs. `candidateContractProvenance` requires every operation's nonempty x-source URLs to be registered and rejects remote refs/duplicate IDs/missing security. `candidateContractShapes` asserts maxLength 255/80, create-required fields, optional PATCH/null behavior, optional restricted field, projection/relationship nulls, the new discovery link and 204 without content schemas. Assert concrete record/envelope properties rather than accepting `{}`. Mutation tests must reject an added Composite operation, wildcard version, missing source, absolute servers URL, default/wildcard response, permissive replacement record schema and changed success status. Example fixed assertions:

```ts
assert.equal(document.openapi, '3.2.1');
assert.deepEqual(document.servers, [{url:'/'}]);
assert.equal(observedOperationIds.size, 19);
assert.deepEqual(observedOperationIds, new Set(expectedIds)); // literal expected list
```

Use test-local narrowing for parsed document fields; no product-code types yet.
- [x] **Step 3: Run RED.** `node --import tsx --test test/candidate-openapi-contract.test.ts`; expect missing module/document failure.
- [x] **Step 4: Implement the document and invariants.** Author the spec inventory and concrete schemas from current code. Use JSON Schema 2020-12 nullable types and local refs; no invented endpoints/403 behavior. Distinguish 201 create, 200/201 upsert and bodyless 204. Describe semantic/permission/query limits separately from structural schema validation. Append source entries for pinned current repository HTTP/catalog/select/query-column URLs, the new design and standards; do not alter historical extraction claims. Implement both helper signatures, then add their candidate-specific checks to validate.ts alongside the unchanged broader proposal's validation.
- [x] **Step 5: Verify GREEN.** Run the focused document test, `npm run typecheck`, and `npm run validate`; expect exit 0/no new skips, including full 3.2.1 document validation by the pinned validator. Format only touched TS files with the pinned formatter. If actual code contradicts the document, fix the new document or rule explicitly; do not change CRUD/query behavior.
- [x] **Step 6: Commit named files.** Commit `feat: define the implemented candidate OpenAPI contract`.

### Task 2: Embed and serve the contract through authenticated discovery

**Files:** Create api/candidate_contract.go and test, internal/platform/http_openapi_test.go; modify internal/platform/http.go only at route serving and v65.0 resource discovery.

**Interfaces:**
- Consumes: Task 1 canonical YAML; existing NewHandler/authentication/method/security-header behavior.
- Produces: package `apicontract` in `salescompel/api` with `CandidateOpenAPI() string`, `CandidateOpenAPIPath = "/services/data/v65.0/openapi.yaml"`, `CandidateOpenAPIContentType = "application/yaml; charset=utf-8"`; authenticated HTTP contract route and discovery key `openapi`.

- [x] **Step 1: Write failing Go tests.** `TestCandidateOpenAPIMatchesSource` compares embedded bytes to canonical source bytes. `TestHTTPServedOpenAPI` uses existing testStore/principals plus a restricted principal to assert GET=200/exact bytes/content type/security headers/no-store and identical content across tenants/roles. Assert missing/bad token=401 JSON, POST=405/Allow GET/shared error shape, unsupported version=404, discovery contains exactly the added link. A real httptest.Server HEAD request asserts 405/Allow GET/empty body; do not infer network HEAD body behavior from ResponseRecorder. Temporary working-directory/asset settings must not affect serving. Token/tenant marker strings must be absent from the response.
- [x] **Step 2: Run RED.** `GOTOOLCHAIN=go1.25.0 go test ./api ./internal/platform -run 'TestCandidateOpenAPIMatchesSource|TestHTTPServedOpenAPI' -count=1 -v`; expect missing package/accessor or route failure. DB test execution requires configured TEST_DATABASE_URL; a skipped DB test is not a RED/GREEN route measurement.
- [x] **Step 3: Implement the accessor and HTTP branch.** Embed the YAML as a private immutable String, return it directly, and write its exact bytes after normal authentication/path normalization. Reuse method checks; do not call ServeFile/ServeContent or serialize YAML as JSON. Add only the named discovery-map key. Preserve inherited headers and all existing API routes/error semantics.
- [x] **Step 4: Verify GREEN.** Run the focused command with a dedicated disposable PostgreSQL 17 database; expect actual test passes/no skips. Offline `go test ./api` and compilation may aid development but cannot replace route/auth DB tests. The complete acceptance run in Task 4 owns final broader Go coverage/vet.
- [x] **Step 5: Commit named files.** Commit `feat: serve the embedded candidate API contract`.

### Task 3: Validate bounded candidate request and response traffic

**Files:** Create candidate-openapi-model.ts, candidate-openapi-traffic.ts and test/candidate-openapi-traffic.test.ts.

**Interfaces:**
- Consumes: Task 1 validated document and existing Ajv2020 API. Runtime server code never imports these modules.
- Produces: `createCandidateTrafficValidator(document:unknown): Promise<CandidateTrafficValidator>` returning `{validate(exchange:CandidateExchange):CandidateTrafficResult}`.
- Model exports `CandidateExchange = {method:string; path:string; request:{contentType:string|null; body?:Uint8Array}; response:{status:number; contentType:string|null; body:Uint8Array}}` and `CandidateTrafficResult = {operationId?:string; requestIssues:CandidateTrafficIssueCode[]; responseIssues:CandidateTrafficIssueCode[]}`. Define CandidateTrafficIssueCode as exactly the spec's category union. No raw headers, authentication secrets or data enter results.

- [x] **Step 1: Write failing traffic tests.** `validatesCandidateExchanges` supplies independently authored concrete request/response bytes for create/read/query/describe/upsert/self-document and bodyless update/delete shapes. `rejectsBrokenCandidateTraffic` mutates required properties, Number/String/null types, unknown response fields, wrong status (including 299/418), media, malformed JSON/UTF-8, missing and extra bodies. Separate requestIssues from responseIssues for malformed requests with valid 400 envelopes. Explicitly test projected records without Id, restricted reads omitting Compat_Text__c, Contact.Account=null and ordered join columns. Ordinary q repeats use the first value; repeated columns fails parameters; ignored query options remain ignored.
- [x] **Step 2: Add route/boundary tests.** `matchesCandidatePaths` proves literal describe precedence, only explicit versions, own-user template matching, escaped %2F/%252F external keys as one segment and malformed percent escapes. Unmatched method/version returns no operationId and method/route issue, with no selected response validation; it cannot count toward operation coverage. `boundsCandidateTraffic` checks exactly/over 1,048,576 request and 8,388,608 response bytes before parsing/copying. `keepsTrafficDiagnosticsPrivate` injects token/record/URL marker text into wrong types/statuses/bodies and sees only sorted category codes/declared operationId. Tests also reject unsupported document serialization/schema constructs at construction; no silent unsupported feature. Example shape assertions:

```ts
assert.deepEqual(validated.requestIssues, []);
assert.deepEqual(validated.responseIssues, []);
assert.deepEqual(bodyOn204.responseIssues, ['response-body']);
assert.equal(unmatched.operationId, undefined);
assert.ok(unmatched.requestIssues.includes('method'));
```

- [x] **Step 3: Run RED.** `node --import tsx --test test/candidate-openapi-traffic.test.ts`; expect missing validator module/constructor.
- [x] **Step 4: Implement the bounded validator.** Validate the full OpenAPI document first; resolve local object refs and map schema refs into an Ajv2020-compatible schema root. Compile only the specified parameter/media/schema subset; reject unsupported constructs rather than downgrade/ignore them. Match raw literal segments before templates and decode template data once. Use exact response statuses/media, fatal UTF-8/JSON parsing, no coercion/defaults, and empty-body checks before schema selection for 204. Handle byte budgets independently per direction and return spec-ordered deduplicated categories. Do not interpret SOQL or enforce real authentication/authorization through schemas.
- [x] **Step 5: Verify GREEN.** Run both offline candidate-openapi test files, `npm run typecheck` and targeted pinned Prettier checks; expect exit 0/no skips. No network/database/browser is involved in these helper tests.
- [x] **Step 6: Commit named files.** Commit `test: validate candidate traffic against its served contract`.

### Task 4: Require live contract coverage in candidate acceptance and publish results

**Files:** Create candidate-openapi.test.ts; modify candidate-acceptance-{contract,results,run,preflight}.ts and the three corresponding test files. Update README/current candidate-guide text and complete the Plan 23 execution report; update this plan/spec status only from measured evidence.

**Implementation ruling:** Include api/ in collectSourceBindings and regress both changed embedded inputs and untracked API code. Also add the exact new parent to the report schema allowedSkipped enum, with a real publication regression. The existing source roots omitted the new binary inputs; this is required to bind acceptance to the delivered contract.

**Interfaces:**
- Consumes: Task 2 serving path/bytes, Task 3 validator, existing CANDIDATE_URL/TOKEN/RESTRICTED_TOKEN/OTHER_TOKEN/RUN_ID/FIXTURE_JOURNAL environment, recordFixtureEvent/trackFixtureAttempt and acceptance reporter/coverage APIs.
- Produces: parent test identity `candidate/openapi/traffic`, preserved unchanged through node result parsing; the existing `jsforce` gate requires it plus all three previous client identities. Database required coverage includes `TestHTTPServedOpenAPI`. Fixed successful operation coverage equals the spec's nineteen IDs.
- Runner helper: `candidateNodeTestArgs(reporterPath:string, reportPath:string): string[]` in candidate-acceptance-run.ts returns the actual command arguments for the existing owned Node process, including both test files. Extract only this argument construction; do not introduce a general process factory.

- [x] **Step 1: Write failing acceptance-wiring tests.** In results.test.ts feed real-format Node events for the new parent and assert its identity stays exactly `candidate/openapi/traffic`, while ordinary test names retain filename prefixes. In contract.test.ts prove missing/skipped/renamed new identity and omitted/skipped TestHTTPServedOpenAPI cannot pass coverage; preserve all previous required tests. In run.test.ts assert `candidateNodeTestArgs('reporter.mjs','report.json')` equals `['--import','tsx','--test','--test-reporter=reporter.mjs','--test-reporter-destination=report.json','test/candidate.test.ts','test/candidate-openapi.test.ts']`. Run the three modified test files with `node --import tsx --test test/candidate-acceptance-contract.test.ts test/candidate-acceptance-results.test.ts test/candidate-acceptance-run.test.ts`; expect failures against current prefixing/required lists/missing argument helper.
- [x] **Step 2: Implement acceptance wiring.** Add the exact new identity to candidateNames, preserve that identity explicitly in candidate-acceptance-results.ts, add the new Go test requirement, and implement/use candidateNodeTestArgs in the existing owned Node process with RUN_CANDIDATE=1 and unchanged reporter binding. Rerun the same focused command: expect all tests pass. Update only affected contract/results/runner fixtures. Do not count the raw-HTTP supplement as another JSforce implementation or skip existing client checks.
- [x] **Step 3: Write the failing live inventory test.** `candidate/openapi/traffic` is skipped in ordinary offline runs unless RUN_CANDIDATE=1; under acceptance it must run and cover all nineteen literal expected operationIds. Fetch exact served bytes and compile them; execute actual authenticated request/response exchanges for every operation and the spec's negative/policy/projection/upsert/escaped-key cases. Unsupported POST/version use direct error assertions because no operation response schema exists; real HEAD asserts empty body separately. Assert operationId plus both issue arrays before adding a success to coverage. Record fixture intent before every possible create and known ID before schema/behavior assertions; preserve unknown outcomes for existing recovery and clean Contact before Account by exact ID. Deliberately bad requests assert expected request categories but no matched error-response issues. Bound network reads/timeouts, use existing TLS trust and never write raw traffic/token data to logs/artifacts.
- [x] **Step 4: Run RED then complete the live journey.** On the owned candidate, run the new live test in a failing state (missing successful operation or assertion), then finish its specific missing journey/assertion. Do not manufacture a server failure or break another passing test to obtain RED. The full runner must eventually observe the parent passing, not skipped. Development commands use the existing acceptance environment; do not invent a parallel provisioning process.
- [x] **Step 5: Verify offline and commit the implementation.** `npm run check` must pass, with opt-in live skips labeled honestly. Run focused Go checks as appropriate and `git diff --check <execution-base>` to inspect committed additions too. Commit all code/tests plus current serving/setup wording before full acceptance, because the full preflight requires a clean committed revision. Use `feat: require candidate OpenAPI traffic acceptance`.
- [x] **Step 6: Run the existing full acceptance once.** With DATABASE_URL naming the dedicated PostgreSQL 17 database (TEST_DATABASE_URL, if set, must match) and prerequisites ready, run `CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <new-private-summary>`. It owns broader DB/vet/lab/web/real-client/restart/browser/cleanup checks; do not duplicate them. Verify both the new parent identity and Go route test are present/passed, all nineteen successful IDs were asserted, every gate passes and cleanup is verified. Record actual counts/skips/failures/tool versions/source bindings and safe summary digest. Missing DB/browser/client prerequisites remain a named blocker on package completion, with owner/action.
- [x] **Step 7: Review and publish the measured result.** Request a fresh whole-change review of schema truthfulness, embedding/auth, validator soundness, identity preservation, operation coverage, private diagnostics and fixture cleanup. Correct material findings with failing regression tests and only affected rechecks; rerun full acceptance only when changes affect its result. Verify intended-file boundaries, original source entries/proposal/historical reports and base-to-final whitespace. Publish source ledger, exact commands/results, acceptance digest and limitations in the new report; explain Node test-only validation does not select the later Go validator or certify Salesforce parity. Mark complete only when required acceptance and review have passed. Commit `docs: publish served candidate OpenAPI acceptance`.

## Self-review

Task 1 covers exact schemas, operation inventory, latest patch prerequisite, source registration and ordinary offline validation. Task 2 covers immutable binary delivery/auth/discovery/real HEAD. Task 3 covers bounded traffic schema/path/status validation, nulls/projections and diagnostics. Task 4 covers reporter identity, runner inclusion, required coverage, actual HTTP effects/policy/cleanup, deployment acceptance and truthful documentation. Every Review Focus line has an owning test. The builder review's two consequential details are explicitly covered by Tasks 2–4; no task assumes real Salesforce evidence, full Go OpenAPI middleware or Plan 20 implementation.
