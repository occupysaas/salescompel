# Plan 19 — Apex execution feasibility

Status: execution in progress. No feasibility decision yet.

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

Pending execution and fresh whole-change review. Full M7, M0/M1R gates, estimates
and Plans 10/14 remain unchanged. Flow, imported UI hosting and required packages
need separate feasibility studies.
