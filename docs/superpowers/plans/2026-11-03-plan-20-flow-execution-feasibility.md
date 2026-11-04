# Plan 20 — Flow Execution Feasibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

Status: proposed for maintainer review, 2026-11-03. Implementation not started.

**Goal:** Execute a strict bounded Flow XML subset unchanged against a synthetic transactional host and publish a reproducible continue/revise/blocked decision.

**Architecture:** A bounded streaming XML reader produces private source-located elements. Complete allowlist, type and reference validation produces an immutable graph before any host begin. An iterative executor follows graph connectors, owns invocation-local resources and publishes the host's staged snapshot only after successful termination and final checks.

**Tech Stack:** Go 1.25.0, standard library (`encoding/xml`, `context` and ordinary Go tests); no dependency changes.

**Spec:** [Plan 20 design](../specs/2026-11-03-plan-20-flow-execution-feasibility-design.md).

## Global Constraints

- Use the repository's Go 1.25.0 toolchain explicitly. Work directly on main under the repository convention.
- No product package may import the experiment.
- Do not modify Plan 19's package or results, dependencies/locks, database/schema/catalog, product endpoints, UI, accepted profiles, readiness attestations, observations, historical captures or `CLEAN_ROOM.md`.
- No new CLI, server, generic workflow framework, shared Apex/Flow runtime or public plugin API is needed.
- No live org access, credentials, customer exports, managed-package inspection or external communications belong to this package.
- Independently author every XML fixture and expected outcome.
- No source may invoke filesystem, network, process, wall-clock, environment, Apex or external actions.
- Concurrent invocations require separate stores; shared-store isolation/concurrency is unsupported.
- These are experiment diagnostics, not Salesforce wire errors.

## Review Focus

- Unknown elements/attributes or wrong namespaces in disconnected nodes must reject the whole XML before any host activity, including apparently harmless decoration (Tasks 2–3 and 5).
- Node labels, reordered root children, duplicate singleton children and similarly named variables/nodes must not change dispatch or hide unsupported content (Tasks 2–3 and 6).
- A Null filter or Name value must fail before selection/staging even for empty matches, and must discard any earlier staged update (Tasks 4–6).
- A denied later row or cancellation at publication must not leak earlier writes through aliases, returned outputs or an attempted second commit (Tasks 4–5).
- XML token/string accumulation, cyclic graphs and update selection must enforce limits before retention/materialization/action, with controlled diagnostics at exact boundaries (Tasks 2–6).

## Files and execution order

All Go files are new under `internal/compat/flow/`. No existing Go file needs modification.

| Files | Responsibility |
| --- | --- |
| `types.go`, `limits.go`, `limits_test.go` | Scalar API, diagnostics, positive finite limits and invocation accounting |
| `xml.go`, `xml_test.go` | Bounded XML tokens, namespace/attribute policy and source offsets |
| `graph.go`, `compile.go`, `compile_test.go` | Exact metadata allowlist, cardinality/type/reference validation and immutable graph |
| `host.go`, `host_test.go` | Synthetic store, capabilities, tenant/policy enforcement, staged overlay and atomic publication |
| `eval.go`, `run.go`, `eval_test.go` | Scalar/lookup resources, graph dispatch and invocation lifecycle |
| `scenarios_test.go`, `testdata/*.flow-meta.xml`, `testdata/scenarios.json` | Independent source fixtures, strict scenario matrix and isolation |
| `docs/compat/plan-20-flow-execution-feasibility.md` | Sources, exact subset, actual results, decision and handoff |

Seven tasks are sequential: the source contract precedes parsing; graph and host consume shared types; execution consumes both; the matrix and decision consume the runner. Keep private test helpers in their owning `_test.go` files. Commit only named task files; substantive regressions may correct earlier package files. The report/spec/plan may be updated truthfully at final handoff.

## Shared implementation contract

These are selected synthetic experiment interfaces and rules, not Salesforce observations.

**Public API:**

- `type Identity struct { Tenant, Principal string }`
- `type Account struct { Tenant, ID, Cohort, Name string }`; Cohort is synthetic `Probe_Group__c`.
- `type Policy struct { Identity Identity; QueryAllowed, NameWriteAllowed bool; DeniedWriteIDs []string }`
- `type Limits struct { XMLBytes, XMLDepth, XMLTokens, Nodes, Variables, Steps, Rows, Writes, StringBytes int }`
- `func DefaultLimits() Limits`: respective defaults `65536, 32, 16384, 64, 32, 2000, 100, 100, 4096`; every caller limit must be positive.
- `type Usage struct { Steps, Rows, Writes int }`
- `Value` stores a private tag/scalar; `String(string) Value`, `Boolean(bool) Value`, `Null() Value`, and accessors `Text() (string, bool)`, `Bool() (bool, bool)`, `IsNull() bool`. Its zero value is Null. No public record value or executable callback.
- `type Diagnostic struct { Code, Phase string; Offset int64 }`, implementing `error`; error text includes those fields only. Codes/phases/offset rules are exactly those in the spec. Preserve original-source byte offsets; never return raw decoder errors.
- `type Request struct { Source string; Inputs map[string]Value; Identity Identity; Limits Limits }`
- `type Result struct { Outputs map[string]Value; Usage Usage }`; all error paths return zero Result.
- `func NewStore(accounts []Account) (*Store, error)` validates nonempty tenant/ID, valid UTF-8 fields, and unique `(Tenant,ID)`, copying/sorting inputs by tenant then ID. `func (*Store) Snapshot() []Account` returns detached sorted copies.
- `func Run(ctx context.Context, req Request, store *Store, policy Policy) (Result, error)` compiles, checks inputs, begins, evaluates, validates/copies outputs, then commits/discards. Nil context/store, empty/invalid identity, nonpositive limits are input errors; policy-identity mismatch is denied before host begin.

**Private boundaries:**

- `decodeXML(ctx context.Context, source string, limits Limits) (*element, error)` and `compile(ctx context.Context, source string, limits Limits) (*program, error)`. `element` and `program` are private, source-located and immutable after creation; exact fields/representation are builder choices.
- Private `newMeter(limits Limits) (*meter, error)`, `(*meter).checkpoint(ctx context.Context) error`, `chargeRows(n int) error`, `chargeWrites(n int) error`; private helpers for compilation/token/node/variable limits and error location are builder choices. Remaining-budget checks must resist int overflow. The checkpoint charges one execution step only after checking cancellation.
- `Store.begin(identity Identity, policy Policy, meter *meter) (*transaction, error)`; `transaction.query(ctx context.Context, cohort string, firstOnly bool) ([]record, error)`, `stage(ctx context.Context, rows []record, name string) error`, `commit(ctx context.Context) error`, `discard()`. Handles/records are private and invocation-bound. Host errors get source/phase attached by the runner/evaluator; source location is not stored in shared Store.
- `evaluate(ctx context.Context, p *program, inputs map[string]Value, tx *transaction, meter *meter) (map[string]Value, error)` returns only declared outputs on successful graph termination. Entry data and lookup resources are per invocation; no mutable program fields.
- `run(ctx context.Context, req Request, store *Store, policy Policy, observer func(string)) (Result, error)` is private; `Run` supplies nil observer. Implement private observer-aware compile/decoder wrappers as needed, with public-private entry signatures above unchanged. Event labels only: `xml-token`, `graph-item`, `begin`, `assignment`, `decision`, `lookup`, `update`, `query`, `stage`, `staged`, `before-commit`, `commit`, `discard`. No source values/IDs in labels; no global observer. Observers exist only for deterministic tests and lifecycle assertions.

**Source/semantics:** Implement the complete spec table, including optional node decoration, exact required root mode/version, ordered rules/items, per-decision unique rule names, full-name namespace checks and strict reference syntax. `start` is not an executable connector target. Variables/executable nodes are globally unique; node labels do not identify nodes. Whole lookup resources are only valid as IsNull's left reference. Root order is irrelevant; typed literal strings preserve decoded whitespace. Disconnected nodes receive the same validation as connected nodes. Cycles are accepted and budgeted iteratively; no FlowLoop is accepted.

All declared scalar inputs are required iff `isInput=true`; extra/non-input keys fail. Reject input maps larger than Variables and invalid/oversized scalar Strings before copying; capture the bounded map and policy denial list before any observer callback. After compile, validate exact keys/types before begin. Inputs are copied, including before observer callbacks. String Null is allowed; Boolean Null is a type error. Defaults are Null String/false Boolean unless one matching literal default is supplied; map outputs include every `isOutput=true` scalar. EqualTo checks matching scalar types, exact strings and String Null equality; IsNull requires Boolean right value and String/lookup left resource. An unexecuted lookup is runtime error, distinct from an executed empty lookup. Null cohort/Name values are runtime errors before the node's host query/stage even on an empty store; a prior staged update rolls back. Host update selection needs both query and write permissions, exact tenant/cohort match and Name-only updates. Permission is checked even for zero rows. All error results have no outputs/usage.

### Task 1: Establish provenance, scope and the complete source allowlist

**Files:** Create `docs/compat/plan-20-flow-execution-feasibility.md`. Read the spec, `CLEAN_ROOM.md`, `go.mod`, Plan 19 report/package, Plan 17 inventory model and candidate platform boundaries; do not modify them.

**Interfaces:** Produces the source/subset ledger, execution-base hashes and exact accepted XML/cardinality/enum/reference contract consumed by Tasks 2–7.

- [ ] **Step 1: Capture the base.** Record `git rev-parse HEAD`, clean/dirty state and `GOTOOLCHAIN=go1.25.0 go version`. Save a temporary SHA-256 manifest of tracked protected profiles/readiness/observations/historical captures, dependencies/locks, `CLEAN_ROOM.md` and the entire Plan 19 package/report. Preserve user changes. Run `GOTOOLCHAIN=go1.25.0 go test ./...` and record failures/skips before any implementation; missing database evidence does not imply database acceptance.
- [ ] **Step 2: Review official source sections.** Retrieve public official Flow root, FlowNode/FlowElement, FlowStart/FlowConnector, FlowVariable/FlowElementReferenceOrValue, FlowAssignment/FlowAssignmentItem/Assign operator, FlowDecision/FlowRule/FlowCondition, FlowRecordLookup/FlowRecordFilter and FlowRecordUpdate/FlowInputFieldAssignment definitions. Record exact URL, heading/page, access date, actual version/read boundary and outcome. Follow the PDF skill for PDF reading. After a failure use a materially different official retrieval path, then record an owned gap rather than repeated retries; no search-snippet or current-version parity inference.
- [ ] **Step 3: Publish the exact subset and integration gap.** Turn every spec table row into an independently written element/cardinality/type allowlist, including inherited node decoration, namespace declarations, literal/reference unions, whitespace handling, booleans and terminal connectors. Record any source disagreement before coding and adjust this plan/spec field mapping only if it preserves scope; material expansion returns to design review. Record current authentication, request timeout, separate CRUD calls and Upsert's local transaction, without importing/refactoring product code. Distinguish source-informed syntax from synthetic API65 boundary, graph/null/order/security semantics, budgets and required future reference observations.
- [ ] **Step 4: Verify and commit.** Check claimed statements against retrieved sections, local links and protected hashes; run `git diff --check` (expect exit 0). Commit only the guide as `docs: define bounded Flow XML experiment contract`.

### Task 2: Bound XML decoding and scalar/resource types

**Files:** Create `types.go`, `limits.go`, `limits_test.go`, `xml.go`, `xml_test.go` and a minimal independently authored `testdata/scalars.flow-meta.xml`.

**Interfaces:** Produces the scalar/diagnostic/limit types, meter and `decodeXML` contract above, retaining original token offsets for Task 3. No host or graph execution.

- [ ] **Step 1: Write failing tests.** `TestDecodeXML` accepts default/prefixed root namespace, comments, one initial XML declaration, decoded built-in entities and preserved String whitespace. `TestDecodeRejectsXML` rejects wrong root/child namespace, non-root namespace declarations, all non-namespace attributes (including `xsi:type`), DTD/custom entity/directive, extra processing instruction, extra root, invalid UTF-8/encoding, malformed nesting and mixed container text. Diagnostic code/phase/offset must be structured and text must not contain an injected marker. `TestDecodeBounds` tests XMLBytes/XMLDepth/XMLTokens/StringBytes exactly at and one beyond configured limits, including accumulated character-data chunks/entities/comments. Use helper-generated repeated inert metadata only to exercise limits; no oversized allocation before rejection. `TestMeterBoundaries` tests each nonpositive limit, exact/over steps/rows/writes, negative charges, MaxInt overflow, independent meters and pre-cancelled contexts without consuming a step.
- [ ] **Step 2: Confirm RED.** Run `GOTOOLCHAIN=go1.25.0 go test ./internal/compat/flow -run 'Test(Decode|Meter)' -count=1`. Expect missing definitions, then failing contract assertions until implemented; fix test syntax/setup errors before treating the failure as evidence.
- [ ] **Step 3: Implement the types, accounting and reader.** Use `encoding/xml.Decoder.Token`, strict UTF-8 with no external entity/charset fetcher, expanded xml.Name checks and bounded private element retention. Validate byte size before decoder creation, tokens/depth before retention and accumulated decoded leaf text before append. Track token-start offsets with `InputOffset`; canonicalize parser errors without raw source/value text. Check context between tokens; private observer fires label `xml-token` only. Never silently skip unsupported XML attributes/directives.
- [ ] **Step 4: Verify and commit.** Run `gofmt` on new Go files, the focused command and `git diff --check` (expect passing/exit 0). Commit as `feat: decode bounded Flow XML with structured diagnostics`.

### Task 3: Validate the complete metadata graph before host access

**Files:** Create `graph.go`, `compile.go`, `compile_test.go`; extend `scalars.flow-meta.xml` and add `testdata/rename.flow-meta.xml`, `testdata/cycle.flow-meta.xml`.

**Interfaces:** Consumes Task 2 types/reader; produces `compile`, immutable `program`, typed variable definitions/node references and source offsets consumed by Task 5. Do not begin a host or execute a graph here. The `scalars.flow-meta.xml` fixture declares required inputs `chooseFirst` (Boolean) and `candidate` (String), local `intermediate` (String, default Null), local/output `selected` (String, default `'default'`) and local/output `confirmed` (Boolean, default false). Its decision has two ordered rules both testing `chooseFirst EqualTo true`: the first assigns `intermediate=candidate`, then `selected=intermediate`, then `confirmed=true`; the second assigns literal `'second'` to selected. The default terminates without assignment. With `chooseFirst=true,candidate='picked'`, outputs must be `selected='picked',confirmed=true`; with false, outputs must be `'default',false`. The first rule's ordered items prove sequential visibility.

- [ ] **Step 1: Write failing graph tests.** `TestCompileSubset` accepts the scalar graph, the record graph and a supported-node cycle; root element reordering or changing labels/locations preserves semantic node/reference identity. `TestCompileWholeSource` rejects unknown root/node children, duplicated apiVersion/connector/value singleton, missing/wrong required root mode/version, duplicate variable/node names, variable/node collision, duplicate rule names in one decision, dangling connector/resource, start/variable/rule connector targets, malformed property chains, unsupported operator/type, assignment to record fields and two alternatives in one literal/reference union. Run those mutations inside connected, untaken-branch and disconnected nodes. Also reject FlowLoop, action/Apex calls, formula, screen, record trigger, faultConnector, unsupported decoration and Boolean lexical variants. `TestCompileTypes` rejects mismatched assignment/default/EqualTo/IsNull/filter/Name values even in disconnected nodes. `TestCompileGraphBounds` proves exact/over Nodes and Variables; `TestCompileCancellation` cancels at deterministic token/graph-item observer counts and checks xml/graph phase with no host dependency. Each source-bound offset must point inside original bytes.
- [ ] **Step 2: Confirm RED.** Run `GOTOOLCHAIN=go1.25.0 go test ./internal/compat/flow -run TestCompile -count=1`; expect missing compiler or failing assertions.
- [ ] **Step 3: Implement schema/reference validation.** Implement the complete Task 1 allowlist, reject unknown/duplicate children before conversion and validate every node regardless of reachability. Build name/type tables before resolving references to allow forward references; do not recursively traverse graph edges. Bound node/variable insertion before allocation. Retain rule/item XML order, graph node positions and root position; validate all type/shape restrictions without evaluating inputs. Observer label `graph-item` and context checks apply to bounded validation loops. Do not add an expression parser or fixture-name branches.
- [ ] **Step 4: Verify and commit.** Run `gofmt`, the compile tests, complete package tests and `git diff --check` (expect all passing). Commit as `feat: validate strict Flow metadata graphs before execution`.

### Task 4: Isolate synthetic record access and transaction effects

**Files:** Create `host.go`, `host_test.go`; extend `limits_test.go` only for host accounting regressions.

**Interfaces:** Consumes Task 2 identity/policy/meter; produces Account/Store and transaction/query/stage/commit/discard signatures above. Does not import Apex or platform code.

- [ ] **Step 1: Write failing host tests.** `TestHostStoreValidation` rejects empty/duplicate tenant-ID keys and invalid UTF-8, accepts the same ID across tenants and proves detached input/snapshots. `TestHostPolicyAndSelection` proves exact tenant/cohort matching, ascending ID, firstOnly versus all rows, query/write denials including an empty update and policy identity mismatch. `TestHostCapabilities` rejects fabricated, cross-tenant, changed-ID, previous-transaction and duplicate-row handles; repeated IDs across separate update statements are valid and cumulatively charged. `TestHostStagedVisibility` proves read-your-writes without early committed changes. `TestHostAtomicRollback` stages one update then denies a later row; failed/finalized transactions cannot commit/reuse and discard is idempotent. `TestHostBudgetAndCancellation` proves exact/over rows/writes before materialization/staging and cancellation after stage/before publication preserves the full snapshot. Policy denial lists are copied before observer activity. Use private helpers to compare complete stores, not only selected Names.
- [ ] **Step 2: Confirm RED.** Run `GOTOOLCHAIN=go1.25.0 go test ./internal/compat/flow -run TestHost -count=1`; expect missing host definitions or failing assertions.
- [ ] **Step 3: Implement the synthetic host.** Use structural tenant-ID keys, a detached sorted snapshot, issued invocation-bound capabilities and a staged Name overlay. Charge row scan/selection and writes before retaining/mutating; validate the full batch before staging. Poison transactions after any error. Check context/steps while constructing a new complete commit snapshot and immediately before publication. Emit only lifecycle labels; expose no commit/store capability to metadata execution. Trusted harness snapshot allocation remains an explicit production-memory gap.
- [ ] **Step 4: Verify and commit.** Run `gofmt`, focused host tests, `GOTOOLCHAIN=go1.25.0 go test -race ./internal/compat/flow` and `git diff --check` (expect passing). Commit as `feat: isolate synthetic Flow host transactions`.

### Task 5: Execute validated graphs with owned resources and rollback

**Files:** Create `eval.go`, `run.go`, `eval_test.go`; create `testdata/read-after-write.flow-meta.xml`, `testdata/fail-after-write.flow-meta.xml`, `testdata/deny-after-write.flow-meta.xml`, `testdata/budget-after-write.flow-meta.xml`, `testdata/empty-update.flow-meta.xml`. Extend existing fixtures only for missing source-driven cases.

**Interfaces:** Consumes compile/program and private host; produces Request/Result/Run/evaluate/run signatures above. Private observer-aware wrappers must preserve compile's no-host boundary. No mutable invocation state in program or package globals. The record fixture accepts String `cohort`, `expectedName`, `newName`, exposes String output `selectedName`, and has a local String `replacement`. Get-one exposes `FindAccount`; the first decision rule terminates on `FindAccount IsNull true`, a second rule matches `FindAccount.Name EqualTo expectedName` and assigns replacement from newName before filtered update; default terminates. A local/output selectedName is assigned from FindAccount.Name on the matching path, making the read meaningful. Use synthetic identity `a/writer` and rows `(a,1,g,one)`, `(a,2,g,two)`, `(b,1,g,other)`. Inputs `g,one,new` change exactly the two a/g Names to new; `g,miss,new` changes none; empty rows yield default Null selectedName. Use the same XML bytes for every data/input variant. The empty-update fixture has required String inputs cohort/newName and a filtered update followed by termination, without a preceding lookup/decision; it tests permission and Null-operand behavior with zero matched rows.

- [ ] **Step 1: Write failing scalar tests.** `TestRunScalars` runs the same XML with literal inputs for true/default/first-of-two-true outcomes and verifies ordered assignment items, String/Boolean outputs and input-map preservation. `TestRunInputsBeforeHost` rejects nil context/store, missing/extra/non-input/mistyped inputs, Null Boolean, invalid UTF-8/oversized String, empty identity and mismatched policy; observer sees no begin. `TestRunNulls` distinguishes empty String from Null, Null equality/IsNull, default outputs, unexecuted lookup errors and executed-empty lookup IsNull. `TestRunWholeSourceBeforeHost` applies Task 3 invalid metadata mutations to a valid writer graph and asserts zero host events and whole-store equality. `TestRunDiagnosticLocations` asserts source-bound offsets and exact phase classes, including commit budget/cancellation using root offset; injected source/value markers never appear in errors. Every failure returns zero Result.
- [ ] **Step 2: Write failing record and control tests.** `TestRunRecords` covers empty/one/multiple matches, same local IDs across tenants, Name-based branching, allowed/denied query/write principals, Name-only publication and exactly one success commit. `TestRunReadAfterWrite` exposes a staged Name before commit. `TestRunRollback` proves a staged write followed by Null field dereference, a later denied batch row or exhausted steps discards the entire store. `TestRunNullHostOperands` uses Null filters/Name values with populated/empty stores and after an earlier update; runtime error must precede that node's query/stage. `TestRunCancellation` synchronizes cancellation before work, during token/graph processing, after stage and before commit via invocation-private labels, with no sleeps and no committed/output leakage. `TestRunTermination` proves cycle budget failure, including a cycle after staging; cumulative row/write and String/step boundaries also produce controlled failure.
- [ ] **Step 3: Confirm RED.** Run `GOTOOLCHAIN=go1.25.0 go test ./internal/compat/flow -run TestRun -count=1`; expect missing runner/evaluator or failing assertions.
- [ ] **Step 4: Implement execution and lifecycle.** Compile all source and validate/copy the exact input map before host begin. The private observer cannot mutate caller-owned inputs after they are captured; copy policy denial lists too. Iteratively dispatch validated nodes by kind and connector, meter each visit/item/condition, keep an executed-lookup presence flag distinct from an empty result and follow ordered rules/items. Resolve both update operands and reject Null before selection. Use host.query for lookup/filtered update, then host.stage for Name-only writes. Own begin/deferred discard in Run, detach/validate declared outputs before commit, attach execution-node/root offsets to errors and return outputs only after successful publication.
- [ ] **Step 5: Verify and commit.** Run `gofmt`, all package tests, `GOTOOLCHAIN=go1.25.0 go test -race ./internal/compat/flow` and `git diff --check` (expect passing). Commit as `feat: execute bounded Flow XML with rollback and budgets`.

### Task 6: Establish the reproducible source-driven scenario matrix

**Files:** Create `scenarios_test.go`, `testdata/scenarios.json`; add `testdata/unsupported.flow-meta.xml` and `testdata/malformed.flow-meta.xml`; extend named fixtures/tests for genuinely missing mandatory cases. Update guide with actual results.

**Interfaces:** Consumes Run/store/scalars. Strict JSON scenario fields: `id`, `sourceFile`, `inputs` (map of tagged String/Boolean/Null scalar objects), `identity`, `policy`, `accounts`, positive `limits`, `expectedCode`, `expectedPhase`, `expectedOutputs` (typed map or null for errors), `expectedAccounts`, `basis`, `sourceRefs`. The tagged scalar objects have `type` plus exactly the matching `string`/`boolean` field, or type `null` alone. Actual measurements are test output and the dated guide, never expected JSON fields.

- [ ] **Step 1: Add the matrix test.** `TestScenarios` rejects unknown/trailing JSON, duplicate IDs, unsafe source paths, missing/unknown sourceRefs and malformed tagged values. Source files are basenames under testdata. Run each case twice on fresh state; compare exact output map/error code/phase and entire sorted store. Re-read and SHA-256-check XML bytes across variants; no source rewriting per input. Include scalar branch/order/default/null cases; empty/one/multirow success; tenant/principal separation; read-your-writes; staged runtime/late denial/budget failure; malformed/unsupported/disconnected input; every configured bound. Assert exactly one success commit and staged/discard traces on staged failures. Link cancellation/capability tests from the guide rather than forcing private synchronization into JSON.
- [ ] **Step 2: Add separate-store isolation tests.** `TestParallelInvocations` runs the identical XML concurrently on separate stores with distinct identities, inputs and budgets; compare each exact output/store and unchanged caller inputs. `TestCompiledGraphIsolation` compiles once and evaluates that immutable program against two independent host/meter/input sets, proving no shared variable/lookup state; compilation reuse is private test coverage, not a new public API. Shared-store concurrency remains unsupported.
- [ ] **Step 3: Run and record.** Run `GOTOOLCHAIN=go1.25.0 go test ./internal/compat/flow -count=1 -v` and `GOTOOLCHAIN=go1.25.0 go test -race ./internal/compat/flow` (expect all mandatory assertions passing for Continue). Resolve in-scope gaps with regressions; preserve reproducible architectural failures for Revise/Blocked, never skip/delete them to produce a green verdict. Populate each mandatory experiment's exact fixture/test, command, actual outcome, reason, source-informed shape and synthetic basis.
- [ ] **Step 4: Commit the matrix.** Commit as `test: cover Flow feasibility scenarios and invocation isolation`. If a negative study retains failing tests, name them and the decision explicitly in the commit/handoff; do not imply implementation acceptance.

### Task 7: Verify scope and publish the reviewed feasibility decision

**Files:** Complete `docs/compat/plan-20-flow-execution-feasibility.md`; update this plan's checkboxes/status and the spec's status to actual outcomes only. Correct package code only for substantive verification/review findings.

**Interfaces:** Consumes sources, implementation, scenario outcomes and execution-base hashes. Produces the reviewed Continue/Revise/Blocked decision and owned follow-up gates.

- [ ] **Step 1: Write the decision.** Address every mandatory scenario with actual results and limitations; distinguish documentary shape, synthetic semantics, implementation, verification and absent reference observations. Compare graph interpretation to a handwritten DSL and Flow-to-Apex translation without implementing or claiming those alternatives work. Document integration gaps at transaction ownership, metadata/version/type resolution, query/security context, order/triggers, error mapping, metering and development lifecycle. Assign platform-implementer to technical/source follow-ups and maintainer to real-target/inventory selection. Keep separate FlowLoop/screens/formulas/fault/subflow/Apex/wait/async/imported-UI/package slices, all milestone gates, program estimates and Plans 10/14 unchanged. Plan 19's existing results/deferred minor stay historical and byte-identical.
- [ ] **Step 2: Run final checks on frozen source.** Run `GOTOOLCHAIN=go1.25.0 go test -race ./internal/compat/flow`, `GOTOOLCHAIN=go1.25.0 go test ./...`, `GOTOOLCHAIN=go1.25.0 go vet ./...` and `git diff --check`; record actual counts/failures/skips/environment limits. Inspect `go list -deps ./cmd/... ./internal/platform` for no Flow dependency, Flow's imports for standard library only/no Apex dependency, protected hashes and `git diff --name-only <execution-base>` for allowed scope. Verify local links and source-claim locators; no database/live-client claim follows from skips.
- [ ] **Step 3: Obtain fresh whole-change review.** Review complete-source namespace/allowlist bypasses, ordered connector dispatch, resource ownership/aliases, late batch denial, rollback, limits before retention/action, deterministic cancellation, diagnostic locations and evidence honesty. Reproduce substantive findings with failing tests, fix within scope and rerun affected/full checks. Record findings, fixes and remaining minor limitations. Unattempted work cannot become a completed negative study.
- [ ] **Step 4: Commit the handoff.** Commit as `docs: publish Flow execution feasibility decision`. Report actual decision, checks, remaining blockers and deferred findings without certifying Flow compatibility, production sandboxing or M7 completion.

## Plan self-review

Seven tasks cover provenance and exact source contract, bounded XML retention, whole-graph validation, independent transactional host, source execution, reproducible mandatory scenarios/isolation and reviewed decision handoff. Nodes counts only executable nodes, Variables counts scalar declarations, and StringBytes includes decoded leaf text; diagnostic taxonomy and pre-copy input bounds follow the spec. Shared API/signatures and defaults are defined once above; all Review Focus cases map to owning tests. Source corrections precede coding, full graph rejection precedes host begin, input/output ownership precedes publication, and final diagnostics have an explicit commit phase/root location. No product code is authorized by this planning document. Execution begins only on a separate implementation request.


## Execution handoff

Recommended execution method: inline with `superpowers:executing-plans`, followed by one fresh whole-change review. Tasks share the XML contract, graph types, meter and transaction lifecycle, so keeping those decisions in one execution context avoids repeated interface reconstruction. This is a recommendation, not a selected execution method or authorization to implement. The present request delivers these two planning documents only.
