# Plan 19 — Apex execution feasibility

Status: implemented, independently reviewed, corrected and verified, 2026-11-03.
Decision: **Continue** for the bounded synthetic subset only; full M7 remains open.

Execution base: `2aa326536f38768c24744079398966d325bb6589`, clean `main`.
Execution date: 2026-11-03. Commands use `GOTOOLCHAIN=go1.25.0`, matching
`go.mod`; the ambient executable reports Go 1.27.3. Baseline `go test ./...`
passed (three tested packages, one package without tests). A temporary SHA-256
manifest covers 28 tracked protected files: profiles, observations, readiness,
evidence/captures where tracked, dependency manifests/locks and `CLEAN_ROOM.md`.
No private captures or credentials were opened.

[Specification](../superpowers/specs/2026-10-21-plan-19-apex-execution-feasibility-design.md)
and [implementation plan](../superpowers/plans/2026-10-21-plan-19-apex-execution-feasibility.md).

## Official source ledger

All accesses below are 2026-11-03, public official HTML, independently summarized.
Only the named sections were reviewed; embedded examples/grammars were not copied.
These current pages do not pin a class API version for this synthetic experiment;
version applicability is **unknown** unless specifically noted. No reference-org
observations were collected. Source IDs are used by the scenario manifest.

| ID | URL and section read | Outcome and limited basis |
| --- | --- | --- |
| classes | [Apex Class Definition](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/apex-classes-defining.html), definition/access-modifier paragraphs | Read: top-level access modifier and class name; inheritance exists but is excluded here. Legacy atlas URL redirected successfully. |
| methods | [Class Methods](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/apex-classes-defining-methods.html), element list and primitive arguments | Read: return type, typed comma-separated parameters, braced body. Incorrect short `apex-methods.html` and legacy `apex_methods.htm` paths failed; the link from Primitive Data Types resolved. |
| primitives | [Primitive Data Types](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/langcon-apex-primitives.html), Boolean/Integer/String rows | Read: 32-bit Integer, single-quoted strings, broader null/escape/comparison behavior than this experiment. Initial `datatypes-primitives` path failed; legacy `langCon_apex_primitives.htm` redirected successfully. |
| lists | [Lists](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/langcon-apex-collections-lists.html), opening and declaration paragraphs | Read: ordered collection and angle-bracket element type. General list APIs excluded. |
| operators | [Expression Operators](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/langcon-apex-expressions-operators-understanding.html), operator table | Read: arithmetic, assignment, parentheses, Boolean short circuit, equality/null and unary operators. Short operator URLs failed; Primitive Data Types link resolved. No locale semantics imported. |
| loops | [Loops](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/langcon-apex-loops.html), procedural loop list; [List or Set Iteration](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/langcon-apex-loops-for-lists.html), opening paragraphs | Read: while and enhanced for constructs. Dedicated while detail page unavailable; overview supplies construct only. |
| basics | [Get Started with Apex](https://trailhead.salesforce.com/content/learn/modules/apex_database/apex_database_intro), Language Highlights, Apex Collections, Apex Classes / Call a Static Method | Read: conditional/loop constructs, typed iteration, public/static method form. Guide if/else URLs returned an error/shell; this different official tutorial shows braced if/else. |
| query | [SOQL and SOSL Queries](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/langcon-apex-soql.html), opening paragraphs | Read: square-bracket queries and list results; no general query behavior adopted. |
| binds | [Using Apex Variables](https://developer.salesforce.com/docs/platform/apex-dev/references/apex-dev/langcon-apex-soql-variables.html), opening and filter-literals bullet | Read: colon-prefixed binding in WHERE. Fixed query shape below is independently selected. |
| dml | [Manipulate Records with DML](https://trailhead.salesforce.com/content/learn/modules/apex_database/apex_database_dml), DML Statements and Bulk DML | Read: update statements accept record lists. Legacy update-example URL and alternative guide route failed; Trailhead resolved. Current page discusses API 67.0 security changes: no security mode/version inference made here. |

Failures on alternate URLs are retrieval history, not absent language features.
The dedicated while detail remains a documentary gap owned by `platform-implementer`:
retrieve it and a pinned class-version guide before promoting loop semantics.
Available overview/tutorial syntax suffices for the synthetic experiment.

## Exact independently authored subset

Terminals are case-sensitive here (unlike Apex). ASCII identifiers exclude all
keywords/types/fields/query words below and the recognized excluded keywords.
Integer literals are decimal digits 0..2147483647; direct unary `-2147483648`
is also accepted. Strings contain UTF-8 without raw CR/LF, supporting only
`\'` and `\\`. Whitespace and `//` line comments are ignored. No block comments.
The method return type is always Integer; scalar locals/parameters use the three
listed types, record/list locals stay private to the evaluator.

```ebnf
program = "public", "class", identifier, "{", "public", "static", "Integer",
          "run", "(", [parameter, {",", parameter}], ")", block, "}" ;
parameter = scalarType, identifier ;
scalarType = "Integer" | "Boolean" | "String" ;
type = scalarType | "Account" | "List", "<", "Account", ">" ;
block = "{", {statement}, "}" ;
statement = type, identifier, "=", expression, ";"
          | identifier, [".", "Name"], "=", expression, ";"
          | "if", "(", expression, ")", block, ["else", block]
          | "while", "(", expression, ")", block
          | "for", "(", "Account", identifier, ":", expression, ")", block
          | "update", expression, ";" | "return", expression, ";" | block ;
expression = or ;
or = and, {"||", and} ;
and = equality, {"&&", equality} ;
equality = relation, {("==" | "!="), relation} ;
relation = sum, {("<" | "<=" | ">" | ">="), sum} ;
sum = product, {("+" | "-"), product} ;
product = unary, {("*" | "/"), unary} ;
unary = ("!" | "-"), unary | primary ;
primary = integer | string | "true" | "false" | "null"
        | identifier, [".", ("Id" | "Name" | "Probe_Group__c")]
        | "(", expression, ")" | query ;
query = "[", "SELECT", "Id", ",", "Name", ",", "Probe_Group__c", "FROM",
        "Account", "WHERE", "Probe_Group__c", "=", ":", identifier, "]" ;
```

Parse the entire source before host begin, including unreachable statements.
Inheritance, platform/general calls, dynamic query construction, triggers, sharing,
exception handlers/savepoints, async/callouts, reflection, packaging and deployment
are unsupported. Malformed grammar is `syntax`; recognizable excluded syntax is
`unsupported`. Diagnostics expose only code and one-based line/column (0,0 for
pre-source inputs), never values/source. Type errors in supported expressions are
checked on the executed path; this is not a full Apex static type checker.

Synthetic decisions: checked int32 arithmetic; division truncates toward zero;
overflow/division by zero are `runtime`. Evaluate left-to-right; short circuit
Boolean operators. Strings compare by exact UTF-8 bytes, equality only; `+`
requires two Integers or two Strings. No implicit conversion. Null is assignable
to String/Account/list and comparable with equality; Integer/Boolean reject it.
Other null operations fail. Lexical blocks may shadow outer variables, but no
same-scope redeclarations or duplicate parameters. Missing executed return fails.
These choices deliberately differ from documented Apex nullability, locale-aware
string equality, case insensitivity and broader conversions; they are not parity.

Each invocation has tenant/principal, positive finite limits, private variables,
a synthetic store snapshot and staged overlay. The host alone queries/updates;
exact cohort matching, tenant filtering and ID sorting are synthetic. Handles
are invocation-bound; only Name changes, no implicit administrator bypass.
Queries see staged Names. Successful commit publishes a new complete snapshot;
every error/cancellation discards all staged changes. Separate stores are required
for concurrent invocations; shared-store concurrency is unsupported.

Defaults: SourceBytes=32768, ParseDepth=64, Steps=10000, CallDepth=2, Rows=100,
Writes=100, StringBytes=4096, CollectionItems=100. All must be positive. Counters
charge before bounded actions/allocation; steps include statements, expressions,
loop iterations/conditions and host scans/stages. Rows/writes accumulate per run;
a list has its own collection bound. Entry depth is one, host transitions two.
Strings use byte lengths, including arguments, literals and host-visible fields.
No evaluated source has filesystem/network/process/time/environment capabilities.
The trusted Go harness controls store size and configured limits; snapshot copies,
parser allocations and retained values are not proof of a production memory sandbox.

## Candidate integration boundary (read, not changed)

`internal/platform/http.go:NewHandler` calls `Store.Authenticate` for service
requests. `context.go:timeBound` applies a 15-second request bound. `Store.Create`,
`Update`, `Delete` and `Read` are separate operations through the pool; `Upsert`
begins/commits a local database transaction. None supplies an invocation-wide Apex
transaction. A later design must establish ownership/rollback across all calls.
`Validate`/`Describe`/`Select` need real metadata/type/security/query integration;
`writeError` requires an explicit runtime error mapping; interpreter checkpoints
need production metering/cancellation policy. No such integration is performed.
Plan 17 has an inventory format, no real target requirements or class API versions.

## Results and decision

Executed 2026-11-03: 25 manifest scenarios each passed twice on fresh state,
and 12 parallel invocations on separate stores passed. Expected outcomes in
`internal/compat/apex/testdata/scenarios.json` are independent synthetic inputs;
the actual outcomes below were observed from Go tests, not Salesforce.
Fresh whole-change review and final scope checks are recorded below.

Reproduce with `GOTOOLCHAIN=go1.25.0 go test ./internal/compat/apex -count=1 -v`.
The command ran and passed; each scenario logs its actual error/result and host
trace. All rows below use that command unless a focused test is named.

| Required experiment | Fixtures / tests | Actual result and assertion |
| --- | --- | --- |
| Source execution | `compute.cls`; `TestRunCompute`, `TestScenarios/compute-{0,1,4}` | Passed: same bytes return 0,0,6 from inputs 0,1,4; SHA-256 checked across variants. |
| Record workflow | `rename.cls`; `rename-empty`, `rename-one`, `rename-multiple-tenants` | Passed: empty, one and two matching rows; exactly one commit; every store field compared, other tenant and differently cased cohort unchanged. |
| Failed transaction | `fail-after-write.cls`, `budget-after-write.cls`, `bulk-failure.cls`; `staged-runtime`, `staged-budget`, `late-batch-denial` | Passed: traces show staged then discard, never commit; entire store unchanged after division-by-zero, infinite-loop budget failure, or later denied row after an earlier update. |
| Tenant/principal separation | `TestHostTenantAndPolicy`, `TestHostHandles`; manifest principal/tenant cases | Passed: same local IDs across tenants remain separate; denied query/write principals fail; unknown, cross-tenant and previous-invocation handles rejected. |
| Resource termination | `TestCompileBounds`, `TestMeterBoundaries`, `TestHostBudgets`, `TestRunBudgets`, `TestRunStringBounds`; manifest `limit-*`, `concat-*` | Passed: source, nesting/unary/flat-tree depth, steps, host call depth, cumulative rows/writes, collection size and UTF-8 byte size produce budget errors; exact boundaries pass; staged budget failure rolls back. |
| Cancellation | `TestRunCancellation/{before,staged,before-commit}` | Passed: deterministic synchronous event observer cancels at each point, entire store unchanged; no sleep or timing assumption. |
| Unsupported/malformed | `unsupported.cls`, `dynamic-query.cls`, `inheritance.cls`, `malformed.cls`; `TestCompileRejectsWholeProgram`, `TestRunUnsupportedBeforeHost`, `TestCompileRejectsQuotedGrammar`, `TestCompileRestrictsAssignmentAndFieldGrammar`, `TestRunRejectsOutOfSubsetBeforeHost` | Passed: whole program rejected including unreachable exclusions, structured locations, no host begin/query/stage. A regression initially accepted quoted keywords/fields; token-kind checks corrected it and the regression passed. |
| Repeatability/isolation | `TestScenarios`, `TestParallelInvocations`, `TestHostSnapshotIsolation` | Passed: every manifest case twice; 12 concurrent invocations have separate identities/stores/parameters/budgets; inputs and detached snapshots unchanged. Shared-store concurrency is unsupported and untested. |

`TestRunTypes` additionally verifies checked arithmetic (including minimum-int
negation/division), short circuit, exact string equality, null errors/equality,
lexical scope and missing returns. Argument/entry errors occur before host begin.
Trace labels contain no record IDs or source values. Scalar/null equality is
supported; two non-null records/lists cannot be compared. Null can inhabit a
String variable, but the synthetic `Account.Name` store field requires a String
value; assigning null to it fails with `type`. Parser columns count UTF-8 bytes.

## Architecture and follow-up gates

Decision after review and correction: **Continue**, limited to the tested synthetic
subset. A hand-written parser and tree interpreter made complete-source rejection,
per-node checkpoints, invocation-local scopes and one host boundary directly
inspectable. The observed rollback/cancellation tests support keeping this design
for the next discriminating experiment. Transpilation would need equivalent
instrumentation and host restrictions; bytecode could centralize instruction
accounting but adds lowering/verification machinery. Neither alternative was
implemented or benchmarked; no performance advantage is claimed.

Production blockers remain: unsupported language/types and platform calls;
full static checking; metadata and version resolution; actual sharing/CRUD/FLS
semantics; transaction/trigger ordering; heap accounting and hostile-program
sandboxing; database/concurrency integration; deployment and target relevance.
The harness trusts store size and configured limits. Parser token/AST storage,
all retained locals/issued handles and store snapshot copies have no aggregate
heap budget; finite source/steps/rows limits bound this experiment but do not
constitute production memory isolation. Cancellation is checked between steps
and immediately before publication, not during Go allocation/sort/compilation.
An external cancellation racing after that last check may occur after publication;
no atomic cancellation/commit guarantee is claimed. No database/live client or
reference-org run was performed.

| Owner | Next action / promotion prerequisite |
| --- | --- |
| maintainer | Select a real target and assign its Plan 17 inventory owner; collect required Apex classes, exact class/API versions and authorized reference access. |
| platform-implementer | Resolve the dedicated while documentation gap and pin language sources to selected versions. Compare independently authored fixtures through authorized public interfaces; synthetic results cannot settle parity. |
| platform-implementer | Design invocation-wide database ownership, metadata/type resolution, security context, query adapter, error mapping and aggregate metering at the candidate attachment points above. Require reference evidence on ordering/limits/security before wiring into product. |
| platform-implementer | Plan separate Flow execution, imported component hosting and package feasibility experiments with actual target requirements and package availability. |

Full M7, M0/M1R gates, program estimates and Plans 10/14 remain unchanged.
No compatibility certification or product promotion follows from this study.


## Verification record

On 2026-11-03, using the pinned Go 1.25.0 toolchain:

- `go test -race ./internal/compat/apex -count=1`: passed; 103 passing
  test/subtest events in the focused package, no skips or race reported.
- `go test -json -count=1 ./...`: exit 0; four tested packages passed,
  214 passing test/subtest events and 23 skipped database test/subtest events;
  the command package has no tests. No failures.
- `go vet ./...`: exit 0, no diagnostics.
- `git diff --check`: exit 0.
- All 28 protected-file SHA-256 hashes match execution base. Changed paths are
  limited to the new package, this report and truthful spec/plan status changes.
- `go list -deps ./cmd/... ./internal/platform`: no experiment import. Every
  direct package import is standard library. Local report links resolve.

Database integration was unavailable (`TEST_DATABASE_URL` unset), so these
existing tests/subtests were skipped; no database acceptance is claimed:

- `TestCatalogFreshV2`
- `TestCatalogUpgradeV1`
- `TestCatalogUpgradeOversizedRollback/compat_text`
- `TestCatalogUpgradeOversizedRollback/compat_key`
- `TestCatalogRejectsUnknown/UPDATE_platform_catalog_SET_version=99`
- `TestCatalogRejectsUnknown/UPDATE_platform_catalog_SET_definition='{}'`
- `TestCatalogRejectsUnknown/DELETE_FROM_platform_catalog`
- `TestCatalogV2PhysicalMismatch`
- `TestCatalogConcurrentOpen`
- `TestHTTPBoundary`
- `TestHTTPExternalIDPreservesEscapedSlashes`
- `TestHTTPRejectsUndeliverablePathsAndListings`
- `TestHTTPCustomFieldLengths`
- `TestHTTPUpsertBodyValidation`
- `TestHTTPUpsertBodyBoundaryControls`
- `TestHTTPOrdinaryExternalIDWrites`
- `TestHTTPQueryColumns`
- `TestHTTPQueryColumnsPolicy`
- `TestStoreIsolationAndPersistence`
- `TestStoreQueryPolicyAndRelationships`
- `TestStoreConcurrentUpdatesAndUpsert`
- `TestStoreBlankNillableStringsAreNull`
- `TestStoreUpsertBodyValidation`

## Fresh whole-change review and correction

A separate fresh-context reviewer inspected execution base through the matrix
commit plus the final report/status updates. Review covered complete-source
rejection, host bypass/handle ownership, aliases, late bulk denial, rollback,
pre-allocation budgets, cancellation at commit and evidence claims. It ran focused
and race tests and independently exercised 1,372,370 temporary-copy malformed-source
fuzz inputs without a panic; this was exploratory review, not a shipped fuzz target.

No critical issue was found. One important finding reproduced an out-of-subset
program reaching the host: parenthesized assignment targets and fields on arbitrary
primaries were accepted, even after an executed return. The fix restricts assignment
targets to identifiers with optional `.Name`, and field reads to identifiers.
`TestCompileRestrictsAssignmentAndFieldGrammar` and
`TestRunRejectsOutOfSubsetBeforeHost` both failed before correction and passed
afterward, including assertions of zero host events and full-store preservation.
The full Go suite, focused race suite and vet passed after this fix. No second
review was substituted for those regression checks.

One minor finding is deferred: errors during commit finalization return diagnostic
position `0,0` despite occurring after source execution. Error codes remain
`budget`/`cancelled`, and all staged effects are discarded. This location gap does
not affect parser diagnostics or rollback; it remains a diagnostic limitation for
a future slice rather than an undisclosed parity or isolation claim.

The review's excluded questions were explicitly accepted as scope boundaries:

| Ruling | Reason | Cost if this boundary is inappropriate |
| --- | --- | --- |
| Work directly on main | User authorized Plan 19's explicit repository convention | No branch isolation. |
| Do not judge Salesforce parity, security modes or governor limits | Synthetic experiment; no reference observations | Compatibility remains unmeasured. |
| Do not judge static type errors in unreachable supported syntax | Executed-path checking is documented | Dead type errors remain accepted. |
| Do not judge aggregate heap isolation or trusted harness sizes/limits | Documented production blockers | Production memory safety remains unproven. |
| Do not judge shared-store concurrency or overlapping transactions | Only separate-store concurrency is supported/tested | Shared-store lost updates remain possible. |
| Do not judge cancellation inside compilation/allocation/sorting or after publication checkpoint | Cancellation occurs between explicit checkpoints | Cancellation can arrive after effects publish. |
| Do not judge database integration, triggers, Flow, imported UI or packages | Separate follow-up studies with no positive claim | Real-target execution/integration remains unproven. |
