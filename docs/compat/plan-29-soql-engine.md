# Plan 29 - SOQL engine reference and compatibility record

Status: implemented, 2026-11-13; no Salesforce compatibility certification.
"Implemented" describes the candidate's behavior recorded here. It is not a
claim of parity with every Salesforce SOQL detail; the status column and the
gaps section name each known difference.

## Reference review (2026-11-12)

No authorized trial-org session was supplied, so nothing here is a live
observation. Spec Sec. 10 Task 1 Step 2 is recorded as "no session", and every
row below is `documented` or `gap`, never `observed`.

Direct WebFetch requests to developer.salesforce.com returned HTTP 403 for
every spec Sources URL. A plain curl request returned the site's HTML error
shell with HTTP 200. The pages were reached through the public `r.jina.ai`
reader proxy, which followed each guide redirect to the current docs platform
(`developer.salesforce.com/docs/platform/...`). The local Plan 25 decision
record was read directly. No client or plugin implementation code was
inspected; this review used public reference pages only.

| Spec source | Direct WebFetch | Reached |
| --- | --- | --- |
| [SOQL and SOSL Reference][c30] | HTTP 403 | Reader proxy; read the SELECT syntax and its linked topics |
| [REST API: Execute a SOQL query][c2] | HTTP 403 | Reader proxy |
| [Query options header][c4] | HTTP 403 | Reader proxy |
| [DescribeSObjectResult][c12] | HTTP 403 | Reader proxy |
| [Plan 25 decisions](../superpowers/specs/2026-11-04-plan-25-foundation-decisions-design.md#decisions-recorded-for-plan-29) | n/a | Local file, read directly |

Retrieved and read: SOQL SELECT syntax and SELECT/FIELDS(); WHERE; comparison
operators; aggregate functions and COUNT(); toLabel(); date formats and date
literals; date functions; ORDER BY, LIMIT, OFFSET and WITH; multi-select
picklist queries; relationship queries, relationship naming, custom-object
naming, query results, lookup nulls and relationship limits; SOQL object
limits and the SOQL/SOSL limits cheatsheet; changing the batch size; REST
execute-a-query, the Query resource and the Sforce-Query-Options header;
DescribeSObjectResult and ChildRelationship; SOAP query(), QueryResult,
AggregateResult and queryMore(). The current platform URLs are listed under
[Sources retrieved](#sources-retrieved).

Could not be retrieved: the help.salesforce.com release-notes index (the page
returned a JavaScript/CAPTCHA loading shell), and direct developer.salesforce.com
HTML (HTTP 403). The exact statements below were not found in any retrieved
page: the `INVALID_FIELD` error code for a wrong-type literal, the REST JSON
shape of `COUNT()` with an empty `records` array, the `null` value of a child
relationship with no rows, and any element cap on `IN` lists.

Release-note review: Winter '27 (API version 68.0) remains the latest general
release, as Plan 28 recorded on 2026-11-11. Search results dated around this
review say Spring '27 release notes are expected in mid-December 2026, and no
Spring '27 notes existed at review time. No SOQL or REST query change after
2026-11-11 was found. The Winter '27 developer guide blog documents two SOQL
changes outside this plan's API v65.0 scope: `FORMULA()` in a WHERE clause
(Beta, API 68.0, not available in production orgs) and the `SET OPTIONS`
clause. Both are recorded as gaps for Task 10 rather than added to Plan 29.

## Candidate behavior status

`observed` means confirmed against an authorized trial org; no row uses it
because no session was supplied.

`documented` means a public Salesforce source states the behavior named in the
row.

`gap` means a known or likely difference recorded and not implemented. It also
covers a spec default that no accessible public source confirms. Each `gap`
row says which part is documented and which part is the default.

`documented` records what a Salesforce page states; it does not mean the
candidate reproduces every detail. `gap` records what the candidate does
instead. Neither status is a compatibility certification.

| Item | Candidate rule | Status | Source / note |
| --- | --- | --- | --- |
| 1. Hand-written parser | New `internal/soql` lexer and parser; no parser generator or new dependency | gap | Spec default and a local implementation choice; no Salesforce source applies. |
| 2. Child subquery cap | Child subquery results have no independent paging. Without a `LIMIT`, a parent with more than 2,000 matching child rows fails with `QUERY_TOO_COMPLICATED`; nothing is truncated. An explicit `LIMIT` of 2,000 or less returns that bounded set, including exactly 2,000. A `LIMIT` above 2,000 does not raise the cap. | gap | Salesforce returns up to 2,000 results per request, including results from child objects, and pages larger sets with `nextRecordsUrl` and `queryMore()` ([limits cheatsheet][c1], [REST execute][c2], [queryMore][c3]). The fail-closed per-parent cap and the absence of child-result paging are recorded deviations. |
| 3. Locators | Locators store ordered result IDs, at most 50,000 per locator, with the bound plan re-validated per page | gap | The REST example documents the locator shape `01g...-2000` in `nextRecordsUrl` ([REST execute][c2]); query results are available for 2 days, including nested results ([limits cheatsheet][c1]). The 50,000-ID cap, ID-array storage, per-page re-validation and oldest-first eviction are spec defaults; Salesforce documents no 50,000-ID cap. |
| 4. Batch clamp | Batch sizes outside 200-2,000 are clamped, not rejected | gap | Default and maximum are 2,000 and the minimum is 200; the requested size is a hint and can change to maximize performance ([Query Options Header][c4], [Change the Batch Size][c5]). Clamping out-of-range values, rather than rejecting them, is a spec default. |
| 5. Date time base | Date literals and date functions use UTC with Sunday-start weeks until Plan 33; fiscal periods are calendar | gap | Date-literal ranges depend on the literal and the user's locale, or the org locale when the user has none ([Date Formats and Date Literals][c6]). Client queries return date-time field values in UTC ([Date Functions][c7]), and `DAY_IN_WEEK()` is 1 for Sunday and 7 for Saturday. The UTC, Sunday-start and calendar-fiscal choices are recorded deviations until Plan 33. |
| 6. `NOT IN` includes nulls | `NOT IN` on nullable fields includes nulls; to observe | gap | Not stated in any retrieved Salesforce page. Secondary articles report that SOQL, unlike SQL, keeps rows where the field is null for `NOT IN` and `!=` ([sfdcdevelopers][c8], [LevelUpSalesforce][c9]). Not observed because there is no session; candidate default. |
| 7. Child relationship name | Default child relationship name derived from the plural label, with a backfill migration | gap | Parent-to-child names are the plural of the child object name, an admin sets a custom child relationship name, and custom relationship names use the `__r` suffix ([Understanding Relationship Names][c10], [Custom Objects and Custom Fields][c11], [DescribeSObjectResult][c12]). Removing non-alphanumerics, numeric-suffix de-duplication and the migration 0005 backfill are spec defaults. |
| 8. `FIELDS()` and denials | `FIELDS()` leaves denied fields out rather than failing | documented | `FIELDS()` respects field-level security and returns only the fields the user can access ([SELECT fields][c13]). |
| 9. `WITH` clauses | `WITH SECURITY_ENFORCED` and `USER_MODE` are accepted as the existing enforcement; `SYSTEM_MODE` is refused | gap | `WITH SECURITY_ENFORCED` and `WITH USER_MODE` are documented clauses, and user mode applies the running user's field-level security and object permissions ([SOQL SELECT Syntax][c14], [WITH][c15]). The reference describes them for Apex, not for REST callers. Refusing `SYSTEM_MODE` for API callers is a spec default; no documented REST behavior was found. |
| 10. Wrong-type code | The wrong-type literal error uses `INVALID_FIELD`, and error messages never echo query values | gap | WHERE values must match the field type, dates and numbers take no quotes, and strings use single quotes ([WHERE][c16]). The error code is not stated in the retrieved pages. Community threads report the message `value of filter criterion for field 'X' must be of type ... and should not be enclosed in quotes` ([Stack Exchange][c17]); `INVALID_FIELD` is the candidate code. Never echoing query values is a local logging choice. |
| 11. `IN` element cap | `IN` lists are capped at 1,000 elements | gap | Salesforce documents a 4,000-character limit on strings in a WHERE clause, not an element cap ([WHERE][c16], [limits cheatsheet][c1]). The 1,000-element cap is a recorded deviation. |
| 12. `NOT IN` null rule | SQL three-valued logic is rewritten so null rows come back for `NOT IN` on a nullable field | gap | Same evidence as item 6. Secondary sources report null rows are kept; no Salesforce page and no live observation confirms it. Candidate default. |
| 13. `INCLUDES` case | Multi-select matching folds case; storage stays Plan 28 canonical text and the compiler splits with `string_to_array` | gap | String comparisons are case-insensitive except on unique case-sensitive fields and follow the user's locale ([Comparison Operators][c18]). The multi-select page documents `INCLUDES`, `EXCLUDES` and `;` as AND but does not restate case handling ([Query Multi-Select Picklists][c19]). Case-folding with `string_to_array(col, ';')` is an internal choice; locale-specific casing waits for Plan 33. |
| 14. Wrong-type error | Wrong-type literal gives `INVALID_FIELD` with the documented "value of filter criterion for field 'X' must be of type ... and should not be enclosed in quotes" wording | gap | Same evidence as item 10, with which this row overlaps. The wording is reported in community threads; the `INVALID_FIELD` code and the no-value-echo rule remain candidates. |

## Findings for later tasks

These are not candidate rows. Task 10 completes the record, and the builder
should reconcile these before or during the named task.

- **OFFSET above 2,000.** The reference says an offset greater than 2,000
  returns `NUMBER_OUTSIDE_VALID_RANGE` ([OFFSET][c20]), while the plan's Task 4
  test expects `MALFORMED_QUERY` for `OFFSET 2001`. **Decision (lead,
  2026-11-12):** keep `MALFORMED_QUERY`. The design locks a closed error
  vocabulary of six codes (`soql-06`); `NUMBER_OUTSIDE_VALID_RANGE` is outside
  that set. The deviation is recorded here rather than expanding the vocabulary
  mid-plan. Revisit only if unmodified clients are shown to switch on this
  exact code.
- **`COUNT()` records.** SOAP `QueryResult` documents `size` as the count and
  `records` as null ([COUNT()][c21], [QueryResult][c22]). The REST empty-array
  shape in the spec is not stated in the retrieved REST pages; treat it as a
  candidate until observed.
- **Null child relationship.** The retrieved query-results page shows only
  parents that have children ([Understanding Query Results][c28]). The spec's
  `null` for an empty child set is a candidate.
- **Concurrent locators.** The 10-open-locators-per-user eviction is described
  as documented in the spec, but the retrieved pages do not state it. Treat the
  number as a candidate.
- **Confirmed documented limits:** 100,000-character query text;
  4,000-character WHERE strings; 55 child-to-parent relationships, 20
  parent-to-child relationships and 5 levels
  ([Relationship Query Limitations][c27], [SOQL SELECT Syntax][c14]);
  2,000 results per request including child rows; 2-day result availability;
  OFFSET maximum 2,000; multiple ORDER BY keys with nulls first by default
  ([ORDER BY][c29]).
- **Not confirmed:** the 35 joined objects and 32 ORDER BY keys that the spec
  states as documented were not found in the retrieved pages.
- **ORDER BY key cap.** The binder caps ORDER BY at 32 keys (`soql-05`) and
  returns `MALFORMED_QUERY` above the cap. The reference pages do not specify
  an error code for an over-cap ORDER BY, so `MALFORMED_QUERY` is a local
  choice.
- **`FIELDS()` duplicate-field error.** Salesforce's reference says a duplicate
  field in the SELECT list is an error, but the exact message is not shown
  ([SELECT fields][c13]). The candidate accepts exact same-path duplicates in a
  plain SELECT as described above; a collision with an expanded `FIELDS()` key
  is `MALFORMED_QUERY` with "duplicate field selected".
- **Winter '27 SOQL additions outside scope.** `FORMULA()` in WHERE (Beta, API
  68.0, not in production) and `SET OPTIONS` ([Winter '27 developer
  guide][c23]).
- **Aggregate queries return `QUERY_TOO_COMPLICATED`.** Superseded by Task 7:
  aggregate functions and `COUNT()` are supported, and this finding applied only
  to the interim binder before that task. It is retained here as history.
- **Contract cases that moved from refused to supported (Task 9).**
  `unsupported-traversal` is now a parent-path response and
  `unsupported-aggregate` is now a `COUNT(Id)` `AggregateResult` response. Their
  fixture basis tags moved from `existing-policy` to `plan-29-engine`; the other
  33 observed cases are unchanged. The column contract now reports plain root
  fields as `{columnName}`, nested `joinColumns` for parent paths, relationship
  `{columnName, joinColumns}` for child subqueries and
  `{columnName: key, aggregate: true}` for aggregates. Every entry nested under
  `joinColumns`, including a leaf, carries an explicit `joinColumns` array
  (empty at a leaf). The pinned `sf data query` reads that array while walking
  the chain. Same-path duplicate fields collapse in the record: for example,
  `SELECT Name, Id, Name FROM Account` returns one `Name` key and one `Id`
  key, while `columnMetadata` keeps each selected entry (`Name`, `Id`, `Name`).
  Distinct parent paths that share a leaf name, such as
  `SELECT Name, Account.Name FROM Contact`, are supported and shape to
  distinct JSON keys. A result-key collision with an alias, a generated
  `exprN`, a child key or a `FIELDS()` key is `MALFORMED_QUERY` with the
  message "duplicate field selected"; for example,
  `SELECT SUM(Amount__c) expr0, COUNT(Id) expr0` and two child subqueries with
  the same relationship key. The richer CLI metadata (`apexType`,
  `displayName`, `entityName`, `length` and so on) remains a gap.

## Gaps carried by this plan

These are known differences from Salesforce that the engine records and does
not implement. The candidate table above covers the behavioral choices; this
list covers the missing constructs and limits. It names the Plan 29 boundary,
not every Salesforce feature. A gap means the candidate refuses the construct
or uses the stated candidate limit; it never means the construct is silently
ignored.

- **Child paging beyond 2,000.** Child subquery results have no independent
  `nextRecordsUrl` paging; only the outer result pages. Without a `LIMIT`, a
  parent with more than 2,000 matching child rows fails closed with
  `QUERY_TOO_COMPLICATED`; nothing is truncated. An explicit `LIMIT` of 2,000
  or less returns the requested bounded set, including exactly `LIMIT 2000`.
  A `LIMIT` above 2,000 does not raise the cap and fails the same way when the
  child set reaches 2,001. Salesforce pages larger child sets with nested
  `nextRecordsUrl` and `queryMore()`; the fail-closed cap is the recorded gap.
- **50,000-ID locator cap.** A result whose membership exceeds 50,000 rows gives
  `QUERY_TOO_COMPLICATED` instead of paging. The cap is a candidate default;
  Salesforce publishes no such limit.
- **UTC date literals.** Date literals and date functions use UTC with
  Sunday-start weeks. Salesforce evaluates them in the running user's time zone
  or the org time zone; user time zones and locales arrive in Plan 33.
- **Calendar fiscal periods.** `FISCAL_*` literals and functions equal their
  calendar equivalents until fiscal-year settings exist (Plan 33).
- **`TYPEOF`.** The AST carries the node, and the binder refuses it with
  `MALFORMED_QUERY`; no polymorphic field exists until Plans 30 and 40.
- **`USING SCOPE`.** Needs ownership and sharing (Plan 34).
- **`FOR VIEW`, `FOR REFERENCE`, `FOR UPDATE`.** Refused; no lock or view
  tracking semantics yet.
- **`GROUP BY ROLLUP`/`CUBE` and `GROUPING()`.** Refused; grouped rows are plain
  groups.
- **`FORMAT()` and `convertCurrency()`.** Refused; the candidate has one display
  currency per tenant.
- **Richer column metadata.** `columns=true` reports `columnName`,
  `joinColumns` and `aggregate: true`. Salesforce CLI metadata such as
  `apexType`, `displayName`, `entityName` and `length` remains a gap.
- **Winter '27 additions outside scope.** `FORMULA()` in WHERE (Beta, API 68.0)
  and the `SET OPTIONS` clause are not parsed.
- **SOSL and `FIND`.** Out of scope here; SOSL is Plan 42.
- **Bulk query.** Bulk API 2.0 query jobs are Plan 32.
- **Composite query subrequests.** Composite save requests use the record read
  path, but Composite query subrequests are Plan 31.

## Operator notes: actual candidate behavior, limits and paging

The engine serves `GET /services/data/v65.0/query?q=...` and
`GET /services/data/v65.0/query/{locator}-{offset}`. These notes describe what
the candidate actually does. They are not a statement that Salesforce behaves
the same way; the candidate status table and gaps section record the
differences.

### Errors and limits

- **Query text and filters:** query text is at most 100,000 characters and a
  WHERE clause is at most 4,000 characters. Longer input returns
  `QUERY_TOO_COMPLICATED`.
- **Relationships:** at most 5 relationship levels, 20 child subqueries, 2
  semi- or anti-joins and 35 joined objects. An over-limit query returns
  `QUERY_TOO_COMPLICATED`; the construct is not partially executed.
- **`IN` lists and `OFFSET`:** `IN` lists are capped at 1,000 elements and
  `OFFSET` at 2,000. An `IN` list above 1,000 returns
  `QUERY_TOO_COMPLICATED`; `OFFSET 2001` returns `MALFORMED_QUERY`. An OFFSET
  query returns at most one page and creates no locator, per page-01. Its
  `totalSize` is the full post-`LIMIT`/`OFFSET` result size even though the
  single page carries at most `batchSize` rows and has no `nextRecordsUrl`.
  Without an explicit `LIMIT`, the count is not capped by the compiler's
  implicit 2,000-row page default; it counts every row after the `OFFSET`.
  Rows beyond that page are not retrievable because a locator is forbidden.
  Salesforce documents `NUMBER_OUTSIDE_VALID_RANGE` for `OFFSET` above 2,000;
  the candidate error code is a recorded candidate choice, not parity.
- **Aggregates:** at most 2,000 grouped rows; more returns
  `QUERY_TOO_COMPLICATED`. Aggregate, `COUNT()` and OFFSET queries never create
  locators.
- **Error vocabulary:** only `MALFORMED_QUERY`, `INVALID_FIELD`,
  `INVALID_TYPE`, `INVALID_QUERY_FILTER_OPERATOR`, `QUERY_TOO_COMPLICATED` and
  `INVALID_QUERY_LOCATOR`. Messages name fields and constructs but never echo
  query values.

### Paging

- **Page size:** the default is 2,000 rows. `Sforce-Query-Options:
  batchSize=n` is honored between 200 and 2,000; values outside that range are
  clamped to the nearest bound. No header means 2,000 rows per page.
- **Concrete example:** over 4,500 matching rows, `SELECT Id FROM P29__c LIMIT
  5000` with `Sforce-Query-Options: batchSize=500` returns nine pages of 500
  rows. Every page reports `totalSize: 4500`; pages one to eight report
  `done: false` and carry `nextRecordsUrl`; page nine reports `done: true` and
  no `nextRecordsUrl`.
- **`nextRecordsUrl` shape:**
  `/services/data/v65.0/query/{locator}-{offset}`, for example
  `/services/data/v65.0/query/01gAbCdEfGhIjKlMn-500`. GET that path to
  continue.
- **Membership is fixed on page one:** later pages read current field values,
  rows deleted after page one are skipped, and `totalSize` keeps its first
  value. Later pages apply only the tenant predicate and the captured IDs, so
  a row that stops matching the original WHERE (or a date literal re-evaluated
  after the day boundary) still appears with its current values. The query is
  still re-parsed and re-bound each page for policy and catalog validation.
  Child subqueries and parent paths are evaluated again for each page.
- **Child subqueries:** child results have no independent `nextRecordsUrl`
  paging; only the outer result pages. Without a `LIMIT`, a parent with 2,001
  or more matching child rows returns `QUERY_TOO_COMPLICATED` with the message
  "child relationship exceeds candidate limit; add a LIMIT to the subquery";
  nothing is truncated. `LIMIT 2000` returns exactly 2,000 rows with
  `done: true`; `LIMIT 1999` returns 1,999. A `LIMIT` above 2,000 does not
  raise the cap and returns `QUERY_TOO_COMPLICATED` when the child set reaches
  2,001. This fail-closed behavior is the child-paging gap, not Salesforce's
  nested pagination.
- **Locators:** the candidate stores at most 50,000 ordered IDs per locator for
  two days, bound to the tenant and user. At most 10 open locators per user are
  kept; opening an eleventh evicts the oldest. Expired, unknown, cross-user and
  cross-tenant locators return `INVALID_QUERY_LOCATOR`. Each page re-validates
  the plan against the current catalog; a field removed or denied after page
  one also returns `INVALID_QUERY_LOCATOR`.
- **Result-size cap:** a query whose membership exceeds 50,000 rows returns
  `QUERY_TOO_COMPLICATED` instead of paging. Salesforce publishes no such cap;
  this is a candidate limit.

### Date literals and query behavior

- **UTC:** date literals and date functions use UTC with Sunday-start weeks.
  `Due__c = TODAY` means the UTC calendar day, not the user's local day. User
  time zones and locales arrive in Plan 33.
- **Fiscal periods:** `FISCAL_*` literals and functions equal their calendar
  equivalents until fiscal-year settings exist. This is a candidate choice, not
  parity with a configured fiscal year.
- **Duplicate fields and result keys:** `SELECT Name, Id, Name FROM Account`
  is accepted. The record carries one `Name` key and one `Id` key;
  `columnMetadata` keeps each selected entry (`Name`, `Id`, `Name`).
  `SELECT Name, Name FROM Account` is also accepted and returns one `Name`
  key. `SELECT Name, Account.Name FROM Contact` is accepted and returns the
  contact `Name` plus `{Account: {Name: ...}}`. A result-key collision with an
  alias, a generated `exprN`, a child key or a `FIELDS()` key is
  `MALFORMED_QUERY` with "duplicate field selected"; examples include
  `SELECT SUM(Amount__c) expr0, COUNT(Id) expr0` and two child subqueries with
  the same relationship key.
- **Column metadata:** with `columns=true`, parent paths and child subqueries
  return nested `joinColumns`. Every nested entry, including a leaf, includes a
  `joinColumns` array; it is empty at a leaf. For example,
  `SELECT Name, Account.Name FROM Contact` returns
  `[{columnName:"Name"}, {columnName:"Account", joinColumns:[{columnName:"Name", joinColumns:[]}]}]`.
  The empty array is required by the pinned `sf data query`. Richer Salesforce
  CLI metadata (`apexType`, `displayName`, `entityName`, `length`) is not
  returned.

## Acceptance evidence

Task 10 adds `schema/soql-relationships`, `schema/soql-children`,
`schema/soql-filters`, `schema/soql-aggregates`, `schema/soql-paging` and
`schema/soql-python` to the `schema` gate, and `cli/query-relationship` and
`cli/query-children` to the CLI gate. The schema client creates 4,500 records in
a `P29_<run>__c` object through Composite batches of 25 and creates twelve
Accounts with one Contact each. The P29 records are removed with their journaled
table, following the Plan 28 ruling; the standard Accounts and Contacts are
journaled and deleted individually. The JSforce checks include
`query(...).run({autoFetch: true, maxFetch: 5000})` with
`Sforce-Query-Options: batchSize=500`, a parent relationship, a child subquery,
a numeric filter, date literals, `COUNT()` and `COUNT(Id)`, `SUM()`, and
grouped counts. simple-salesforce runs `query_all` over the same 4,500 rows and
runs a child subquery and `COUNT()`.

Verification scope and result:

- **Scope:** full candidate acceptance (`scope: full`), all 13 gates.
- **Source revision:** `e8130a5a77b8f1514e3c23bde0ea1bf42ab8586a`.
  This final cross-task fix commit corrects locator membership under changed
  filters, aggregate binder allowlists, aggregate relationship
  OpenAPI/metadata, LIKE escaping, timezone independence, child FK policy, and
  exact OFFSET totals/no-locator gap. The later documentation-only closeout
  commit does not change the source bindings.
- **Report:** `/home/user001/plan29-acceptance/plan29-final-e8130a5-r1.json`,
  SHA-256 `3633b3914d7f91a5789a8407f258175777704879177fa7e9b14b79f2a86e5dbe`,
  outcome `passed`, `acceptanceEligible: true`, tree clean at the run. This
  supersedes the earlier `9d506f7` report
  (`/home/user001/plan29-acceptance/plan29-quality-9d506f7-r4.json`, SHA-256
  `4dc72acd172fa226e8b4ae4a077e08a4f0559b74f5f86fb1f26f5b90d05b08c1`), which
  is retained rather than rewritten.
- **Required skips:** 0. **Total skips:** 6, all in the lab gate and all the
  allowed offline parents: `candidate/composite/atomic`,
  `candidate/foundation/decisions`, `candidate/openapi/traffic`,
  `candidate/jsforce/core`, `candidate/jsforce/policy` and
  `candidate/jsforce/isolation`. The live jsforce gate runs the corresponding
  live tests.
- **Cleanup:** verified; 455 journaled records removed, 0 unknown intents, and
  all owned processes stopped. The disposable database and temporary restricted
  roles were dropped after the run.
- **Tools:** PostgreSQL 17.12; Go 1.25.14 with `GOTOOLCHAIN=go1.25.0`; Node
  22.23.2; JSforce 3.11.0; Salesforce CLI 2.156.0; plugin-data 5.1.16;
  simple-salesforce 1.12.9; Python 3.12.13; OpenSSL 3.5.5; Chrome for Testing
  153.0.8010.
- **Node concurrency:** the candidate Node test command runs with
  `--test-concurrency=min(4, availableParallelism())` (1–4 workers). A
  two-CPU acceptance host otherwise serializes every test file and misses the
  documented 20-minute deadline; the cap bounds memory and fixed-port pressure.
- **Run time:** 2026-11-13T09:51:26.384Z to 2026-11-13T10:07:33.470Z
  (16 minutes 7 seconds, inside the documented 20-minute deadline).

| Gate | Passed | Failed | Skipped |
| --- | ---: | ---: | ---: |
| bindings | 3 | 0 | 0 |
| database | 829 | 0 | 0 |
| lab | 474 | 0 | 6 |
| web | 11 | 0 | 0 |
| jsforce | 6 | 0 | 0 |
| python | 4 | 0 | 0 |
| cli | 11 | 0 | 0 |
| oauth | 16 | 0 | 0 |
| schema | 22 | 0 | 0 |
| restart | 6 | 0 | 0 |
| connected-browser | 14 | 0 | 0 |
| preview | 11 | 0 | 0 |
| cleanup | 2 | 0 | 0 |

The lab row shows the gate's reporter coverage counts, which include parent
events; the Node test command itself reported 474 top-level tests: 468 passed,
0 failed and 6 skipped. The six skips are the allowed offline parents listed
above; required skips are 0.

Acceptance at the new source revision also ran `go test -race ./... -count=1`
(12 packages, exit 0), `go vet ./...` (exit 0), the Node check (474 tests: 468
passed, 0 failed, 6 skipped), the web unit tests (10 passed) and the web build.
A standalone `npm --prefix compat/clients/node run check` at the same revision
exited 0 with the same Node counts, and `openspec validate --all --strict`
passed its one item. A 120-second `FuzzParse` run at `e8130a5` completed
1,500,967 executions with no crashers and exited 0. The web gate's 11 passes
are the 10 unit tests plus the build step. This closeout records verification
of the existing Plan 29 implementation; it adds no new feature.

Residual gaps not implemented by Plan 29: independent child-subquery paging
(child results have no `nextRecordsUrl`; over-2,000 child sets fail closed
unless bounded by a `LIMIT` of 2,000 or less); result membership above 50,000
IDs in one locator;
UTC date literals and Sunday-start weeks with calendar fiscal periods (Plan 33);
`TYPEOF`; `USING SCOPE` (Plan 34); `FOR VIEW`, `FOR REFERENCE` and `FOR UPDATE`;
`GROUP BY ROLLUP`/`CUBE` and `GROUPING()`; `FORMAT()` and `convertCurrency()`;
richer column metadata (`apexType`, `displayName`, `entityName`, `length`); and
SOSL/`FIND` (Plan 42), Bulk query (Plan 32) and Composite query subrequests
(Plan 31). No Salesforce compatibility certification is claimed.

## Sources retrieved

All developer.salesforce.com pages cited in this record were reached through
the `r.jina.ai` reader proxy after direct WebFetch returned HTTP 403. The
reference definitions used by the table follow; secondary sources were reached
through search results. Pages read without a table citation are listed at the
end of this section.

[c1]: https://developer.salesforce.com/docs/platform/salesforce-app-limits-cheatsheet/guide/salesforce-app-limits-platform-soslsoql.html "SOQL and SOSL Limits for Search Queries"
[c2]: https://developer.salesforce.com/docs/platform/api-rest/guide/dome-query.html "Execute a SOQL Query"
[c3]: https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-querymore.html "queryMore()"
[c4]: https://developer.salesforce.com/docs/platform/api-rest/guide/headers-queryoptions.html "Query Options Header"
[c5]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-changing-batch-size.html "Change the Batch Size in Queries"
[c6]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-dateformats.html "Date Formats and Date Literals in WHERE"
[c7]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-date-functions.html "Date Functions"
[c8]: https://sfdcdevelopers.com/2026/05/06/soql-not-in-not-equal-exclusion/ "SOQL NOT IN and !=: Syntax, Subqueries and Null Rows (secondary source)"
[c9]: https://levelupsalesforce.com/soql-query-not-in "SOQL query NOT IN (secondary source)"
[c10]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-relationships-understanding.html "Understanding Relationship Names"
[c11]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-relationships-and-custom-objects.html "Understanding Relationship Names, Custom Objects, and Custom Fields"
[c12]: https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-describesobjects-describesobjectresult.html "DescribeSObjectResult"
[c13]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-fields.html "SELECT"
[c14]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select.html "SOQL SELECT Syntax"
[c15]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-with.html "WITH"
[c16]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-conditionexpression.html "WHERE"
[c17]: https://salesforce.stackexchange.com/questions/203903/value-of-filter-criterion-for-field-price-c-must-be-of-type-string-and-should "Value of filter criterion for field 'Price__c' must be of type string (secondary source)"
[c18]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-comparisonoperators.html "Comparison Operators"
[c19]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-querying-multiselect-picklists.html "Query Multi-Select Picklists"
[c20]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-offset.html "OFFSET"
[c21]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-count.html "COUNT() and COUNT(fieldName)"
[c22]: https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-query-queryresult.html "QueryResult"
[c23]: https://developer.salesforce.com/blogs/2026/10/developers-guide-to-the-winter-27-release "The Salesforce Developer's Guide to the Winter '27 Release"
[c27]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-relationships-query-limits.html "Understanding Relationship Query Limitations"
[c28]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-relationships-query-results.html "Understanding Query Results"
[c29]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-orderby.html "ORDER BY"
[c30]: https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql.html "SOQL and SOSL Reference"

Other pages read during the review, not cited in the table:

- [SOQL Object Limits and Limitations](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-limits.html)
- [Aggregate Functions](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-agg-functions.html)
- [toLabel()](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-tolabel.html)
- [LIMIT](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-limit.html)
- [Relationship Queries](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-relationships.html)
- [null Values in Lookup Relationships and Outer Joins](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-relationships-lookup.html)
- [Query](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-query.html)
- [query()](https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-query.html)
- [AggregateResult](https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-query-aggregateresult.html)
