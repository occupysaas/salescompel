# Plan 19 — Apex Execution Feasibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Execute a bounded Apex-shaped source subset against a synthetic transactional host and publish a reproducible continue/revise/blocked feasibility decision.

**Architecture:** An isolated Go package parses complete source into an immutable syntax tree before opening a host transaction. A checkpointed interpreter accesses records only through an invocation-local host, stages changes, and commits only on successful completion. Tests exercise source fixtures, rollback, policy boundaries and deterministic resource limits without importing the production platform.

**Tech Stack:** Go 1.25.0 as declared in `go.mod`; standard library and ordinary Go tests only.

**Spec:** [Plan 19 design](../specs/2026-10-21-plan-19-apex-execution-feasibility-design.md).

## Global Constraints

- Use Go and the repository's pinned toolchain, standard library only, in a new isolated `internal/compat/apex` package. No product package may import it.
- Work directly on main under repository convention.
- Do not change dependencies, database/schema/catalog, product endpoints, UI, accepted profiles, readiness attestations, observations or historical captures.
- Do not change the clean-room policy or copy implementation code, grammars or Salesforce examples.
- No live org calls, credential discovery, customer source, managed-package inspection or external communications belong to this package.
- No evaluated program may access filesystem, network, process execution, wall-clock time or environment variables.
- Scope the harness to sequential commits; concurrent isolated invocations must use separate stores.
- Synthetic limits and error categories are not Salesforce governor limits or wire errors.
- A negative result is a valid completed study when the minimum experiments were attempted and the blocker is reproducible.

## Review Focus

- Unsupported code in an unreachable branch must fail before host activity, not escape detection until execution (Task 2).
- A detached record or returned snapshot must not mutate committed state through Go slice/pointer aliases (Task 3).
- A denied row late in a bulk update must discard earlier staged rows, including earlier update statements (Tasks 3 and 5).
- Cancellation after staging and just before commit must leave the entire store unchanged (Tasks 4 and 5).
- Resource checks must precede allocation/action, including string concatenation, query materialization and deep expressions (Tasks 2–4).

## Files and execution order

All Go files below are new under `internal/compat/apex/`; no existing Go file needs modification.

| Files | Responsibility |
| --- | --- |
| `types.go`, `limits.go`, `limits_test.go` | Public scalar inputs/results, diagnostics, limits and invocation-local accounting |
| `syntax.go`, `lexer.go`, `parser.go`, `parser_test.go` | Source locations, private syntax tree, lexer and complete-input parser |
| `host.go`, `host_test.go` | Synthetic store, policy, opaque handles, staged effects and atomic publication |
| `eval.go`, `run.go`, `eval_test.go` | Scoped values, evaluation, host calls, cancellation and transaction lifecycle |
| `scenarios_test.go`, `testdata/*.cls`, `testdata/scenarios.json` | Source-driven scenario matrix, expected results and provenance |
| `docs/compat/plan-19-apex-execution-feasibility.md` | Official-source ledger, exact subset, actual results, decision and handoff |

Tasks are sequential because they share types and lifecycle decisions. Commit only each task's named files. Tests may introduce small private helpers in their owning `_test.go` files. Do not create a command, generic VM framework or runtime registry.

## Shared implementation decisions

These are plan-selected experiment semantics, not measured Salesforce behavior. Task 1 checks syntax against official documentation and records discrepancies before implementing it. A source gap cannot silently become documented or observed parity.

**Public package boundary:**

- `type Identity struct { Tenant, Principal string }`
- `type Account struct { Tenant, ID, Cohort, Name string }` — `Cohort` represents the synthetic field `Probe_Group__c`, never a production catalog addition.
- `type Policy struct { Identity Identity; QueryAllowed, NameWriteAllowed bool; DeniedWriteIDs []string }` — exact IDs are tenant-local; no implicit administrator bypass.
- `type Limits struct { SourceBytes, ParseDepth, Steps, CallDepth, Rows, Writes, StringBytes, CollectionItems int }`
- `func DefaultLimits() Limits` — defaults respectively `32768, 64, 10000, 2, 100, 100, 4096, 100`. Caller-provided limits must all be positive; zero does not mean unlimited.
- `type Usage struct { Steps, Rows, Writes int }`
- `type Diagnostic struct { Code string; Line, Column int }`, implementing `error`. Codes: `invalid-input`, `syntax`, `unsupported`, `type`, `runtime`, `denied`, `budget`, `cancelled`. Source positions are one-based; pre-source input errors use `0,0`. Diagnostics do not echo source or values.
- `Value` is a tagged scalar with private storage; constructors `Integer(int32) Value`, `Boolean(bool) Value`, `String(string) Value`, `Null() Value`; accessors `Int() (int32, bool)`, `Bool() (bool, bool)`, `Text() (string, bool)`, `IsNull() bool`. Record/list values stay private to the interpreter.
- `type Request struct { Source, Entry string; Args []Value; Identity Identity; Limits Limits }`; Entry is `ClassName.run`.
- `type Result struct { Value Value; Usage Usage }`.
- `func NewStore(accounts []Account) (*Store, error)` validates nonempty tenant/ID and unique `(tenant, ID)`; identical IDs across tenants are valid. Copy inputs. `func (*Store) Snapshot() []Account` returns detached copies sorted by tenant then ID.
- `func Run(ctx context.Context, req Request, store *Store, policy Policy) (Result, error)` owns compile, argument checking, begin, evaluate, cancellation check and commit/discard. Error returns a zero Result; use private accounting in focused tests where failure usage matters. Store and policy are trusted Go harness inputs; evaluated source cannot construct them.

**Syntax contract:** One `public class Identifier { public static Integer run(parameters) { statements } }`, no other classes/methods. Parameters are comma-separated `Integer`, `Boolean` or `String` names. Locals additionally support `Account` and `List<Account>`. Statements: declaration with initializer, assignment, brace-delimited `if (...) {...} else {...}`, `while (...) {...}`, enhanced `for (Account name : list) {...}`, `update list;`, `return expression;`. No standalone expression statements or general calls. Account reads expose only `Id`, `Name`, `Probe_Group__c`; only Name is assignable. No field assignment to Id or the selection field.

Expressions: signed 32-bit integer values, single-quoted strings with only escaped quote/backslash, `true`, `false`, `null`, identifiers, parentheses, record-field access, unary `!` and `-`, binary `* / + - < <= > >= == != && ||` with conventional precedence. Integer arithmetic is checked; division by zero and overflow produce `runtime`. `+` additionally concatenates two strings, with length checked first; no implicit conversions. Evaluation is left-to-right with short-circuit Boolean operators. Strings compare by exact UTF-8 bytes for equality only. Null supports equality/inequality and assignment to String/Account/list; arithmetic, condition evaluation and dereference of null fail. Integer and Boolean reject null. Require a return on the executed path; reaching the end is `runtime`. Check argument arity/types before host begin. Reject duplicate declarations in a scope; lexical block scopes may shadow outer names.

The sole query expression is `[SELECT Id, Name, Probe_Group__c FROM Account WHERE Probe_Group__c = :identifier]`, with a String bind and fixed selected fields/order. Synthetic host matching is exact, tenant-filtered and sorted by ID. No general SOQL semantics are claimed. Whitespace and `//` line comments are accepted; other syntax is rejected. Publish the full independently written EBNF in the guide before evaluator work. Recognizable excluded constructs use `unsupported`; malformed grammar uses `syntax`; both must reject the whole program before host begin.

**Host boundary:** Private `transaction` exposes `query(ctx context.Context, cohort string) ([]record, error)`, `stage(ctx context.Context, records []record) error`, `commit(ctx context.Context) error`, `discard()`. `record` carries private invocation-bound handle metadata and detached fields. `Store.begin(identity Identity, policy Policy, meter *meter) (*transaction, error)` rejects missing identity and mismatched policy identity. Only host-issued handles may be staged; verify tenant, ID and invocation origin. Count each submitted row per update statement toward Writes, even repeated IDs; reject duplicate handles within one bulk statement. Queries see staged Names within the invocation; commit publishes one new snapshot. Discard is idempotent. No shared-store concurrent-use guarantee is offered.

**Accounting:** Private `newMeter(limits Limits) (*meter, error)`, `(*meter).checkpoint(ctx context.Context) error`, `(*meter).chargeRows(n int) error`, `(*meter).chargeWrites(n int) error`, `(*meter).enter() error`, `(*meter).leave()` centralize controls. One checkpoint per executed statement and expression, plus each loop condition/iteration and host-row scan/stage. It checks cancellation before incrementing Steps. Entry depth is one, synchronous host call depth two; recursive/general method calls are unsupported. Count returned rows cumulatively; collection size applies to each list, writes cumulatively. Use overflow-safe remaining-budget comparisons. Charge before append/mutation; scan query rows without collecting first. Bound caller strings and source before copying; bound parser recursion across blocks, unary chains and expressions. Input store size is harness-controlled; its snapshot copy is not represented as proof of a production memory sandbox.

### Task 1: Establish source provenance and the executable subset

**Files:** Create `docs/compat/plan-19-apex-execution-feasibility.md`; read the spec, `CLEAN_ROOM.md`, `go.mod`, Plan 17 inventory documentation and `internal/platform/{context,store,http}.go`.

**Interfaces:** Produces the reviewed syntax/subset ledger and execution-base manifest consumed by Tasks 2–6. No runtime implementation in this task.

- [ ] **Step 1: Capture execution scope.** Record `git rev-parse HEAD`, clean/dirty state and Go version. Save a temporary SHA-256 manifest of tracked profiles, readiness files, observations, historical evidence/captures, dependency manifests/lockfiles and `CLEAN_ROOM.md`; identify capture paths from repository tracking. Preserve user changes and record the base for final diff inspection.
- [ ] **Step 2: Review official documentation.** Browse public official Apex language documentation for class/method declarations, selected scalar/list types, expressions, control flow, inline query binds and update statements. Record exact URL, heading, access date, applicable version or unknown, read boundary and outcome. Follow the PDF skill if reading PDFs. Loading shells/snippets are not content. Use a materially different official retrieval path after a failure, then retain an owned source gap rather than repeated retries. Do not copy examples or grammar.
- [ ] **Step 3: Publish the subset and integration notes.** Write independently authored EBNF and the shared decisions above in the guide. Separate source-supported syntax from synthetic type/security/ordering/budget semantics. Record that `NewHandler` authenticates through `Store.Authenticate`, request contexts currently have a 15-second bound, basic store CRUD calls are separate operations and `Store.Upsert` has a local transaction; none supplies an invocation-wide Apex transaction. Verify these observations against execution-time source. Document unsupported features and API-version uncertainty.
- [ ] **Step 4: Review boundary and commit.** Verify each claimed source statement against retrieved content and all local links. If source review requires changing a planned syntax form, record the reason and update this plan before coding; material scope changes return to design review. Run `git diff --check`; commit the guide as `docs: define bounded Apex experiment syntax and provenance`.

### Task 2: Parse and reject complete source before execution

**Files:** Create `types.go`, `limits.go`, `syntax.go`, `lexer.go`, `parser.go`, `parser_test.go`, `limits_test.go` and `testdata/compute.cls` under the package.

**Interfaces:** Produces the public scalar/diagnostic/limit types, private meter, immutable `program`, and `compile(source string, limits Limits) (*program, error)`. Program holds the parsed class/entry, typed parameter declarations and statements with locations; it contains no host or mutable invocation state.

- [ ] **Step 1: Write failing parser and limits tests.** `TestCompileSubset` parses a fixture whose `run(Integer n)` accumulates `0..n-1` with a while loop and returns the sum if positive, otherwise zero. `TestCompileRejectsWholeProgram` places inheritance, a platform call (`System.debug`), dynamic query (`Database.query`) and malformed syntax both early and after a return; assert diagnostic code/location and no accepted partial tree. `TestCompileBounds` covers source size at/over limit, deeply nested parentheses/blocks/unary operators, unterminated strings, invalid UTF-8 and trailing tokens. `TestMeterBoundaries` asserts exact limit success, next charge failure, negative/zero limits rejection, overflow-safe charges and independent meters. Tests for duplicate parameters/declarations reject ambiguity.
- [ ] **Step 2: Confirm failure.** Run `go test ./internal/compat/apex -run 'Test(Compile|Meter)' -count=1`; expect missing definitions, then implementation-dependent assertion failures until complete.
- [ ] **Step 3: Implement types, accounting and parser.** Implement the signatures above in their named files. Validate UTF-8/size before token buffering; bound tokens by source size and parser descent before recursive calls. Consume the entire input and allow only the published grammar. Lex unknown tokens without guessing general call support. Keep per-compilation state local. Implement stable diagnostics without embedding arbitrary source text.
- [ ] **Step 4: Verify and commit.** Run `gofmt` on new Go files and the focused command; expect all passing. Inspect the EBNF against accepted/rejected fixtures. Commit as `feat: parse bounded Apex experiment source`.

### Task 3: Implement the synthetic transaction and policy host

**Files:** Create `host.go`, `host_test.go`; extend `limits_test.go` only for host metering cases.

**Interfaces:** Consumes `Identity`, `Policy`, `Limits`, `Diagnostic`, private `meter`. Produces `Account`, `Store`, `NewStore`, `Snapshot`, `Store.begin` and transaction methods defined above. Keep record handles private and invocation-bound.

- [ ] **Step 1: Write failing host tests.** `TestHostTenantAndPolicy` queries two tenants with identical local IDs, checks exact cohort membership/ID ordering, denies query/Name writes and rejects policy identity mismatch. `TestHostHandles` rejects unknown, cross-tenant and prior-invocation handles even for an existing ID. `TestHostSnapshotIsolation` mutates input rows, snapshots and queried records without staging and asserts committed state unchanged. `TestHostCommitDiscard` tests read-your-staged-writes, whole-store equality after discard, and success with only the intended Names changed. `TestHostBulkFailure` stages an earlier write, then sends a batch with an allowed row followed by a denied row; discard leaves the original full snapshot. `TestHostBudgets` proves exact/over row, write and collection budgets and duplicate-handle rejection before publication.
- [ ] **Step 2: Confirm failure.** Run `go test ./internal/compat/apex -run TestHost -count=1`; expect missing host definitions or failed assertions.
- [ ] **Step 3: Implement the host.** Use structural tenant/ID keys, copied policy denial lists and per-invocation handle ownership. Validate all submitted rows before staging that batch. Use a staged overlay and detached record results; stage never mutates the committed store. Commit checks cancellation and constructs/publishes the complete next snapshot once. Keep commit/discard inaccessible to evaluated source. A finalized transaction rejects further operations.
- [ ] **Step 4: Verify and commit.** Run `gofmt`, the host tests and `go test -race ./internal/compat/apex`; expect all passing. Commit as `feat: isolate synthetic Apex host transactions`.

### Task 4: Evaluate source with deterministic failure and rollback

**Files:** Create `eval.go`, `run.go`, `eval_test.go`; create `testdata/rename.cls`, `testdata/fail-after-write.cls`, `testdata/infinite.cls`.

**Interfaces:** Consumes `compile`, private syntax/value types, meter and host. Produces `Request`, `Result`, `Run`, and private `evaluate(ctx context.Context, p *program, args []Value, tx *transaction, meter *meter) (Value, error)`. Static program state stays immutable; locals/record values are invocation-local.

- [ ] **Step 1: Write failing scalar execution tests.** `TestRunCompute` runs unchanged compute source with `n=0,1,4`, expecting `0,0,6`. `TestRunTypes` covers argument count/type mismatch before begin, null equality versus null dereference, short circuit avoiding division by zero, overflow including minimum-int negation/division, lexical scope and missing return. Assert structured errors; no panic. `TestRunStringBounds` permits an exact-limit result and rejects over-limit concatenation before allocation.
- [ ] **Step 2: Write failing source-driven host tests.** Rename fixture has `run(String cohort, String newName)`, queries the fixed selection into `List<Account> rows`, iterates assigning `a.Name = newName`, performs `update rows;` and returns zero. Fail-after-write performs that update then `return 1 / 0;`. Assert success commits exactly matching tenant rows and failure leaves every store field unchanged. `TestRunUnsupportedBeforeHost` includes forbidden code after an otherwise valid update and verifies no begin/query/stage using a private test observer or equivalent lifecycle counters.
- [ ] **Step 3: Write failing termination tests.** Infinite source uses `while (true) {}`; a small Steps limit must produce `budget`. Test exhausted CallDepth on a query, row/write/string/list limits, and exhaustion after staging. `TestRunCancellation` uses an unexported invocation-local checkpoint observer to cancel deterministically after a staged write and immediately before commit, plus pre-cancelled context; every case preserves the full store. No sleeps or timing races; observer receives event labels only and is absent from the public Request.
- [ ] **Step 4: Confirm failure.** Run `go test ./internal/compat/apex -run TestRun -count=1`; expect missing runner/evaluator or failing semantics.
- [ ] **Step 5: Implement evaluation and lifecycle.** Implement the exact signatures above; use checked arithmetic, typed internal values and lexical scopes. Precheck source, entry, args, identity and policy before host activity. Defer discard after begin, evaluate, check context, commit, then return the scalar result and usage. Check limits/cancellation at the defined checkpoints, including empty loop iterations and host scans. Do not recover arbitrary programmer panics as successful execution or dispatch by fixture name.
- [ ] **Step 6: Verify and commit.** Run `gofmt`, `go test ./internal/compat/apex -count=1` and `go test -race ./internal/compat/apex`; expect passing. Commit as `feat: execute bounded Apex source with rollback and budgets`.

### Task 5: Establish the complete reproducible scenario matrix

**Files:** Create `scenarios_test.go`, `testdata/scenarios.json`; extend named source fixtures/tests only where a missing scenario needs them. Update the guide with actual results.

**Interfaces:** Consumes `Run`, constructors and store snapshots. JSON scenario entries contain `id`, `sourceFile`, `entry`, typed scalar `args`, `identity`, `policy`, `accounts`, positive `limits`, `expectedCode`, `expectedReturn` (typed scalar or null for errors), `expectedAccounts`, `basis` and `sourceRefs`. The decoder rejects unknown fields; source paths are filenames inside testdata. All scenario identities/data are synthetic. Actual outcomes are test output and the dated guide, not expected-data fields masquerading as measurements.

- [ ] **Step 1: Add matrix tests.** `TestScenarios` loads JSON, verifies unique IDs and source references, runs each scenario twice with fresh stores and compares error code, return and the entire sorted store to expectations. Assert source SHA-256 is identical across input variants. Include empty/one/multiple query matches; allowed/denied principal; same local IDs in two tenants; partial-batch denial after an earlier stage; staged-write runtime/budget failure; malformed/unsupported source; all resource bounds. Link deterministic cancellation and forged-handle cases to their dedicated tests in the guide rather than forcing private controls into JSON.
- [ ] **Step 2: Add isolation test.** `TestParallelInvocations` compiles/runs identical source concurrently on separate stores with distinct identities, parameters and budgets. Assert each independent result/store, unchanged inputs and no shared state; share immutable source only. Keep shared-store concurrency explicitly untested/unsupported.
- [ ] **Step 3: Run, resolve and record.** Run `go test ./internal/compat/apex -count=1 -v` and `go test -race ./internal/compat/apex`; all positive-decision scenarios must pass. Fix implementation gaps within scope with regression tests. If an architectural failure prevents a mandatory scenario, retain a reproducible failing test and publish revise/blocked rather than deleting/skipping it or declaring the suite green. Record command, actual assertion/result and source/basis for every required experiment.
- [ ] **Step 4: Commit the matrix.** Commit as `test: cover Apex feasibility scenarios and isolation`. For a negative-study commit, explicitly identify any retained failing tests and the decision; do not imply implementation acceptance.

### Task 6: Verify scope and publish the feasibility decision

**Files:** Complete `docs/compat/plan-19-apex-execution-feasibility.md`; update this plan's task checkboxes and the spec's status only to reflect actual outcome. Correct package files only for substantive verification/review findings.

**Interfaces:** Consumes all recorded source attempts and test results. Produces the reviewed continue/revise/blocked decision, actual limitations and follow-up owners/actions.

- [ ] **Step 1: Write the decision.** Cover all required scenarios with exact tests/fixtures, outcomes and provenance. Distinguish documented syntax, synthetic semantics, implemented behavior and verification; reference observations are none. Compare interpreter versus transpilation/bytecode based on the measured host/control needs, without claiming untested alternatives work. Record production blockers: full language/types, metadata, transaction/trigger ordering, actual security modes, resource sandboxing, concurrency, deployment and real target relevance. Map future integration needs to current candidate functions without importing/refactoring them. Assign `platform-implementer` to technical follow-ups and `maintainer` to selecting a real target/inventory owner. Identify separate Flow/imported-UI/package studies. Preserve Plans 10/14 and all milestone gates/estimates.
- [ ] **Step 2: Run final checks.** Run `go test -race ./internal/compat/apex`, `go test ./...`, `go vet ./...` and `git diff --check` on frozen source. Record counts, failures, skips and environmental limits; database-test skips are not database acceptance. Inspect `go list -deps ./cmd/... ./internal/platform` and require no `salescompel/internal/compat/apex`; inspect package imports to ensure standard library only. Compare protected hashes and `git diff --name-only <execution-base>`; only package files, guide and truthful plan/spec status updates are allowed. Verify all local links and source-claim locators.
- [ ] **Step 3: Obtain fresh whole-change review.** Review against the spec/plan with emphasis on parser bypasses, handle forgery/aliasing, rollback, budget checks before allocation, cancellation at commit, concurrency claims and source-evidence honesty. Correct substantive findings, add meaningful regressions and rerun affected checks. Record review findings and actual fixes/limitations. A negative result remains a negative study; review cannot turn an unattempted scenario into completion.
- [ ] **Step 4: Commit the handoff.** Commit as `docs: publish Apex execution feasibility decision`. Report actual decision, checks and remaining blockers. No product promotion, Salesforce compatibility, production sandbox or completed M7 claim follows.

## Plan self-review

Six sequential tasks cover the spec's source review, precise grammar, complete-source rejection, isolated host, invocation semantics, mandatory scenario matrix, deterministic budgets/cancellation, repeatability, preserved artifacts, fresh review and decision handoff. Shared signatures and type names above are the interface contract for every task. Review Focus cases map to explicit tests in Tasks 2–5. The parser/host/evaluator choices and synthetic defaults are plan decisions; actual source access and feasibility results remain execution outcomes. This document creates the implementation plan only.
