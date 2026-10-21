# Plan 16 — CLI Query Column Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development if the maintainer selects it. Implement task by task; checkboxes track execution, not planning completion.

**Goal:** Resolve the pinned CLI's nonempty query failure through a justified column-metadata contract and verify the existing candidate with two complete acceptance passes.

**Architecture:** Freeze a small, provenance-backed response fixture set before changing Go behavior. Add catalog-derived projection metadata to the existing authorized query route, extend the existing native CLI journey and enforce the new coverage through Plan 15's unchanged report/lifecycle framework. Unresolved material contract rows block dependent runtime tasks.

**Tech Stack:** Existing Go/net/http/pgx/PostgreSQL, TypeScript/node:test, pinned Salesforce CLI and the existing acceptance runner; no new dependencies.

**Spec:** [Plan 16 design](../specs/2026-10-20-plan-16-cli-query-column-metadata-design.md).

**Status:** Implemented and reviewed on main. Two complete sequential full acceptance runs passed at code revision `807c937983311383f947f14bf636dd6eeba17e70`; see the [decision/evidence record](../../compat/plan-16-cli-query-column-metadata.md).

## Global Constraints

- Use API 65.0, catalog version 2, Go module baseline 1.25.0, PostgreSQL 17, Node >=22.12 and Python >=3.12.
- Preserve JSforce 3.11.0, Salesforce CLI 2.156.0, plugin-data 5.1.16, plugin-deploy-retrieve 4.2.15, simple-salesforce 1.12.9 and existing Vue/Playwright pins.
- No dependency upgrades, schema/catalog migration or new runtime dependencies.
- Work directly on `main` under the repository convention.
- Exact wire shapes belong in the contract decision record and its fixtures, rather than guesses in this plan. Client consumption requirements do not establish universal Salesforce behavior.
- No new external write authorization, reference credentials access, Plan 10/14 collection, baseline expansion, cloud provisioning or general query/metadata framework.
- Preserve existing query syntax, values, `attributes`, `totalSize`, `done`, policy and tenant isolation; do not redesign pagination, equality, authentication, external IDs or localization.
- Preserve Plan 15's nonempty `cli/query`, required identities, exit handling, source/result bindings, exact fixture cleanup, no-skip policy and private raw diagnostics.
- Acceptance requires two fresh complete sequential runs against the same explicitly disposable database, clean committed source, distinct run IDs/outputs and successful cleanup between runs. Setup/install remains separate.
- Report blocked contracts, implemented behavior and consolidated acceptance separately; no full M1/M2 or universal compatibility claims.

## Review Focus

- An empty result or all-null projected field still has the metadata specified by the frozen contract (Task 2).
- A duplicate/reordered projection or `Contact.Account.Name` must not lose order, crash, broaden syntax or regress nested records (Tasks 1–2).
- A denied field late in a projection, a forbidden filter/order field or another tenant's matching ID must not leak data or successful metadata (Task 2).
- Conflicting/repeated/empty/malformed `columns` values follow a deterministic documented rule, while ordinary queries remain unchanged (Tasks 1–2).
- A CLI subprocess or result omits one new required check: the consolidated report rejects acceptance independently of process exit zero (Task 3).

## File and interface map

| File | Responsibility |
| --- | --- |
| `docs/compat/plan-16-cli-query-column-metadata.md` | Reproducer, source provenance, contract decisions, blockers and final evidence |
| `internal/platform/testdata/query-columns-contract.json` | Frozen sanitized request/response cases; a test oracle independent of the implementation |
| `internal/platform/query_columns.go` (new) | Request option interpretation and authorized catalog-to-wire metadata mapping |
| `internal/platform/query_columns_test.go` (new) | Fixture-driven helper and database-backed HTTP behavior tests |
| `internal/platform/http.go` | Add optional metadata to the existing query response after successful selection |
| `internal/platform/select.go`, `catalog.go` | Read/reuse existing query/field policy; modify only if sharing authorization is needed to prevent drift |
| `compat/clients/node/src/candidate-acceptance-clients.ts` | Preserve native CLI journey and add real nonempty Contact/empty-result checks |
| `compat/clients/node/src/candidate-acceptance-contract.ts` | Require new Go test and CLI check identities |
| `compat/clients/node/test/candidate-acceptance-contract.test.ts` | Reject omitted/failed/skipped new identities |
| `docs/compat/plan-15-candidate-acceptance.md`, `docs/compat/plan-2-implementation.md` | Evidence-based acceptance status and supported-route/setup notes |
| `api/openapi.yaml` | Conditional update only if its query model can accurately express this bounded served behavior |

## Task 1: Reproduce the failure and freeze the bounded contract

**Files:** Create the Plan 16 decision record and `internal/platform/testdata/query-columns-contract.json`. Read the existing native CLI runner, `versions.ts`, lockfile, query/catalog code, and applicable official material.

**Interfaces:** Consumes the unchanged `runNativeCLI(c: GateContext): Promise<ParsedResult>` journey and API 65.0. Produces the contract record plus JSON `{ "schemaVersion": 1, "cases": [...] }`. Each case contains `id`, `principal` (`admin`, `restricted`, `other`), `query`, `querySuffix`, `expectedStatus`, `expectedBody`, and `basis` references. Use symbolic `$accountId`, `$contactId`, `$name` values expanded only by the test fixture loader. `expectedBody` is the complete decoded body, including errors, so absence differs from null. Each distinct authorization fixture context is documented beside the cases.

- [x] **Step 1: Reproduce against the current candidate.** Install locked prerequisites only through separate documented setup if needed; do not update lockfiles. Use an explicitly disposable local database and the existing candidate-only runner with a new private output. Verify its required nonempty `cli/query` fails, preserves owned-ID cleanup and matches the Plan 15 symptom. Record actual command, revision, versions, request parameters, response shape and outcome in the decision record. A missing prerequisite is blocked; do not invent a fresh reproduction.
- [x] **Step 2: Inspect the actual pinned consumer and official material.** Resolve plugin-data through the installed CLI dependency tree rather than assuming a top-level package path. Record file paths, package versions, source digests and the code path that consumes metadata; cite the exact official pages consulted. Distinguish consumer requirements, documentation, reference observations and candidate decisions. Do not make reference requests without independently established authorization/prerequisites.
- [x] **Step 3: Freeze every contract row from the spec.** Specify top-level metadata presence, member names/types/values, field-family mapping, labels/canonical spelling/order, duplicate selection, empty and null results, and absent/false/empty/invalid/repeated `columns`. Include every existing Account/Contact field and the already supported `Contact.Account.Name` nested projection. The current special case in `Select` is existing support, not permission to add other traversal. Include denied object/field/filter/order and invalid query cases. State whether each decision is specified, observed, deliberately bounded candidate behavior or unresolved, with its basis.
- [x] **Step 4: Write the fixture oracle.** Encode exact bodies and parameters from Step 3, covering ordinary and metadata-enabled queries for Account, Contact, custom Text(80), IDs/reference IDs, empty results, nulls, reordered/duplicate fields, existing nested projection and policy errors. Describe fixture seed data and substitutions in the record. Keep synthetic values only. Do not serialize output from the proposed implementation as expected output.
- [x] **Step 5: Enforce the gate and commit.** Verify JSON parsing and review every case against its cited decision. If any material wire row is unresolved, record the precise blocker and the minimal missing observation; leave Tasks 2–4 unchecked. Fixture cases may describe established requirements, but must not assert guesses as observations. If the gate passes, add an executor addendum to this plan naming the frozen metadata member/value mappings and `columns` parsing rules before writing product code. Commit the record, oracle and addendum as `docs: freeze bounded CLI query metadata contract`.

This task intentionally contains research and fixture work rather than a fabricated red/green cycle. Exact wire decisions cannot be safely predetermined from the recorded exception. No runtime code is authorized past an unresolved gate. If an offline probe is necessary, define its narrow input/output and meaningful assessment tests in the addendum before creating it; do not introduce a generic collector.

## Task 2: Implement catalog-derived metadata and HTTP regressions

**Files:** Create `internal/platform/query_columns.go`, `internal/platform/query_columns_test.go`; modify `internal/platform/http.go`. Reuse `field`, `Describe`, `Query`, `Field` and `Store.Select` without duplicating field declarations.

**Interfaces:** Consumes Task 1's frozen cases and existing `Query{Object, Fields, ...}`. Produces `queryColumnsRequested(values url.Values) (bool, error)` and `queryColumnMetadata(p Principal, q Query) ([]map[string]any, error)` in package `platform`. The map keys/JSON values come exclusively from the frozen decision table. The slice represents ordered projection metadata, never result-row inference. If the frozen contract requires a different top-level JSON type, update this interface in the Task 1 addendum before implementation rather than coercing it into an incorrect array.

- [x] **Step 1: Add fixture-driven failing tests.** In `TestQueryColumnsOptions`, read the frozen parameter cases and assert the exact enabled/error decision. In `TestHTTPQueryColumns`, provision admin/restricted/other principals with `testStore`, `principals` and `Provision`, seed the documented Account/Contact data, execute the actual handler through `httptest` and compare decoded complete bodies to expanded `expectedBody` with `reflect.DeepEqual`. Use exact case IDs as subtest names. Seed another tenant's matching fixture and prove it cannot be returned. Run both empty-database and populated-result cases.
- [x] **Step 2: Add boundary regressions to `TestHTTPQueryColumnsPolicy`.** Test a denied `Compat_Text__c` after an allowed Id, denied filter/order fields, unknown object/field, malformed SELECT and existing unsupported syntax. Assert existing exact status/error codes and no successful metadata projection. Verify an admin metadata query does not affect a later restricted request. For `Contact.Account.Name`, assert the frozen metadata plus unchanged nested `Account`/`attributes` shape, including a null relationship. Reordered/duplicate projections and all-null custom values use the corresponding Task 1 oracle cases.
- [x] **Step 3: Observe red.** With `DATABASE_URL` and `TEST_DATABASE_URL` privately bound to the same disposable database, run `go test -count=1 ./internal/platform -run 'Test(QueryColumnsOptions|HTTPQueryColumns|HTTPQueryColumnsPolicy)$' -v`. Expect failures from missing helpers/metadata on the original implementation; require database-backed cases to execute, not skip. Save a sanitized account of the failure; do not check secret-bearing logs into git.
- [x] **Step 4: Implement the two helper interfaces and route integration.** Parse options exactly as frozen. Resolve field metadata through current catalog/policy in projection order, including the existing Account.Name special case, without adding syntax. Execute `Store.Select` successfully before publishing metadata; denied WHERE/ORDER must still fail even if all SELECT fields are readable. Build the ordinary result map as today and add the frozen metadata property only under the frozen request rule. Return normal API errors rather than panics or partial success. Keep metadata definitions independent of returned row count and preserve all existing record serialization.
- [x] **Step 5: Observe green and regress existing behavior.** Run the focused command above, then `go test -race -count=1 ./...` and `go vet ./...` against the disposable database. Require applicable database tests to execute with zero skips; missing DB is blocked. Review that ordinary query fixture bodies, existing relationship tests and limit errors are unchanged.
- [x] **Step 6: Align applicable contract docs and commit.** Inspect the OpenAPI query parameter and shared QueryResult consumers. If changing it would imply support beyond the candidate, keep the bounded contract in the Plan 16 record and explain why; otherwise add the precise optional parameter/schema/example and validate with the existing lab validator. Run `git diff --check` and commit only the focused runtime/tests/contract changes as `fix: serve catalog-derived query column metadata`.

## Task 3: Require the real-client regression in consolidated acceptance

**Files:** Modify the acceptance clients/contract and contract tests listed in the map. Reuse `candidate-acceptance-fixtures.ts`; no new client framework or public report format.

**Interfaces:** Preserve `runNativeCLI(c: GateContext): Promise<ParsedResult>`, existing `cli/query` identity and all existing required IDs. Add `cli/query-contact` and `cli/query-empty` to `requiredCoverage.cli`. Add `TestHTTPQueryColumns` and `TestHTTPQueryColumnsPolicy` to required database coverage. All additional fixtures use `createFixture` or durable intent/known journaling before/after native writes.

- [x] **Step 1: Add a failing coverage regression.** Extend `candidate-acceptance-contract.test.ts` with `coverage requires query metadata regressions`: construct otherwise valid CLI/database results and individually omit, fail or skip each new identity; assert the existing coverage validator rejects every case. A zero exit status cannot override rejection. Keep the test behavioral, not a shell-source search.
- [x] **Step 2: Observe red.** Run `node --import tsx --test test/candidate-acceptance-contract.test.ts` from `compat/clients/node`. Expect the new missing-identity assertion to fail before updating required coverage.
- [x] **Step 3: Extend the real journey.** Keep the original owned Account query and its exact record Id/updated Name assertions. Using journaled owned fixtures, add a nonempty Contact query projecting `Id,LastName,AccountId` and assert all three exact values against the owned relationship. Add an independent query for a never-created run-owned Name and assert zero records. Do not replace the nonempty query or suppress its metadata request. Keep native auth state private, assert subprocess completion/JSON envelope as before and emit the new IDs only after their assertions pass. Preserve cleanup and a failed result if any query fails; a new failure must not bypass existing delete/absence checks or outer cleanup. HTTP fixture tests own exact wire metadata assertions because CLI JSON may transform that response.
- [x] **Step 4: Require coverage and verify.** Update `requiredCoverage` additively. Run the focused contract test followed by `npm --prefix compat/clients/node run check`; require the new coverage regression to pass. Execute `scripts/check-candidate.sh --out /absolute/new-plan16-candidate.json` with disposable opt-in and private DB configuration. Require real CLI checks and fixture/process cleanup to pass; candidate-only is diagnostic and never full acceptance. If unavailable, document the blocker without substituting mocks.
- [x] **Step 5: Commit.** Run `git diff --check` and commit as `test: require native CLI column metadata journeys`.

## Task 4: Review, run fresh full acceptance and publish the outcome

**Files:** Update Plan 16's decision/evidence record, Plan 15's status, Plan 2's guide and this plan's executed checkboxes. Update README/roadmap only if the recorded status changes; retain historical failures and exclusions.

**Interfaces:** Consumes the existing `scripts/check-candidate-acceptance.sh --out PATH`, report schema version 1 and all required gate identities. Produces two digest-identified sanitized full-run summaries outside the tracked tree and a truthful public evidence record.

- [x] **Step 1: Review the whole runtime/test change.** Obtain a fresh whole-change review under the chosen execution workflow. Resolve Important/Critical findings with meaningful regressions and rerun affected checks. Confirm no unresolved Task 1 wire decision was guessed into product code and no old coverage/gate was removed. Run `git diff --check` and commit all tested code before final acceptance.
- [x] **Step 2: Verify setup separately.** Follow Plan 15's documented locked setup; use preinstalled compatible clients/browser and dedicated disposable PostgreSQL 17. Bind both DB URL variables privately to that same database and set `CANDIDATE_DISPOSABLE=yes`. Choose two new absolute `.json` paths outside the repository with existing real parent directories. No dependencies/browser downloads during acceptance, foreign service reuse, cloud resources, automatic database drop or raw-log publication.
- [x] **Step 3: Run full A.** Execute `scripts/check-candidate-acceptance.sh --out /absolute/new-plan16-full-a.json`. Require exit 0, scope `full`, outcome `passed`, `acceptanceEligible=true`, every required gate passed, required coverage complete, verified exact fixture disposition, zero unknown intents and stopped owned processes. Record revision, actual versions, source/lock/catalog/binary/asset bindings, report digest and gate counts/skips from the validated report. A failed/blocked run remains preserved and does not count toward the required pair.
- [x] **Step 4: Run full B sequentially.** Only after A's cleanup/process disposition is verified, run the same command with `/absolute/new-plan16-full-b.json` against the same database and unchanged clean code. Require the same full conditions, distinct run identity and unchanged code bindings. If runtime code changes after either attempt, rerun the pair against the new committed code; do not combine revisions into acceptance.
- [x] **Step 5: Publish actual outcomes.** Append the Plan 16 gate matrix, commands, versions, digests, counts/skips, fixture disposition and both intervals/outcomes. Explain the bounded contract's evidence level separately from candidate interoperability. Update Plan 15 with the follow-up outcome and link while retaining its original failed attempts; update Plan 2's supported-route/setup notes. Describe CI as configured unless an actual run was observed. Missing prerequisites, unexecuted gates or unresolved assertions stay explicit. No product fix alone closes acceptance, and no passing pair closes full M1/M2 or Plan 13's visual review.
- [x] **Step 6: Commit the evidence.** Run `git diff --check`, verify linked files and report digests, ensure the tree contains no secrets/raw diagnostics, and commit as `docs: record plan 16 query and acceptance outcome`. Documentation-only commits cite the tested code revision rather than implying the newer revision was retested. Report exact completed work and remaining blockers.

## Self-review and handoff

Coverage: Task 1 owns provenance and every frozen wire/option decision; Task 2 owns catalog/policy/HTTP behavior; Task 3 owns unchanged-client interoperability and mandatory identities; Task 4 owns review, repeatability and truthful delivery. All five Review Focus items have explicit tests above. Interfaces use existing principal/query/catalog/result types; metadata JSON members remain deliberately gated rather than invented.

Execution used the approved native workflow. A fresh whole-change reviewer found one Important malformed-encoding gap; three HTTP regressions reproduced it, the fix passed the Go race suite/vet, and both final full runs passed. Reference collection was not performed.

## Executor addendum — frozen contract

Task 1 froze the minimal candidate contract in `docs/compat/plan-16-cli-query-column-metadata.md` and its JSON oracle. Enable metadata only for exactly one literal `columns=true`; absent/one false preserves ordinary responses. Reject all other/repeated values with 400 INVALID_QUERY_PARAMETER. Scalar metadata is `{columnName: catalog.Name}`; existing Account.Name traversal is `{columnName: "Account", joinColumns: [{columnName: "Name"}]}`. Return ordered `[]map[string]any`, preserving duplicate projections. No unused type/label/precision members are claimed or emitted. Query parsing precedes option validation; full Select policy precedes metadata publication. These are explicitly candidate interoperability decisions, not observed Salesforce wire semantics.

Execution addendum: malformed percent escapes and unescaped semicolons are rejected from raw query parsing after SELECT parsing with 400 INVALID_QUERY_PARAMETER / `Invalid query parameter encoding`. The oracle has 35 cases after the three review regressions. Private result publication uses `<gate>-result.json`, preserving the restart fixture.
