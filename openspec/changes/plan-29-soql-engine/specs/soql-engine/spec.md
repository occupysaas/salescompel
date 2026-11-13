# Spec: soql-engine

## Purpose

Parse, bind, secure, compile, execute and page a documented SOQL subset
against the tenant catalog so unmodified Salesforce clients can read records
through one engine, with locked error codes, paging semantics and security
behavior.

## ADDED Requirements

### Requirement: One engine with parameterized SQL and tenant predicates
<!-- id: soql-01 -->
The query engine SHALL consist of a lexer, recursive-descent parser, binder,
compiler and executor/shaper in `internal/soql`. Every compiled statement
SHALL pass every query value as a SQL parameter. Only validated catalog
identifiers MAY be interpolated, and only when quoted with `pgx.Identifier`.
Every joined table and subquery SHALL carry an explicit tenant predicate;
RLS remains the backstop and tenant context is transaction-local. A unique
marker value placed in every literal of a bound query SHALL never appear in
`Statement.SQL` and SHALL appear in `Args`.

#### Scenario: no value text in SQL
- **WHEN** a query with string, numeric and date literals is compiled
- **THEN** the SQL text contains none of those literal values
- **AND** each value appears in the statement's argument list

#### Scenario: a forged cross-tenant ID never returns a row
- **WHEN** tenant B's parent ID is placed in a filter or path of a tenant A query
- **THEN** no tenant B row is returned through joins, subqueries, semi-joins or locators

### Requirement: SELECT supports fields, parent paths, children, aggregates and functions
<!-- id: soql-02 -->
The SELECT list SHALL accept field references, parent relationship paths up to
five levels, child subqueries, aggregates, `toLabel()`, the documented date
functions and `FIELDS(ALL|CUSTOM|STANDARD)`. Parent paths SHALL nest each level
in REST JSON with its own `attributes`, and a null reference SHALL produce a
null parent object rather than an error. Ordering by a parent field SHALL put
null parents first by default. Child subqueries SHALL support fields, parent
paths, WHERE, ORDER BY and LIMIT, and SHALL refuse aggregates, GROUP BY,
OFFSET and nested subqueries. Each parent row SHALL receive
`{totalSize, done: true, records}` or `null` when there are no child rows.
Aggregate and date-function result keys SHALL be the alias when present;
otherwise unaliased aggregate and function items SHALL be numbered `expr0`,
`expr1`, … in the order they appear among those items. Grouped fields keep
their names. `SUM` and `AVG` SHALL accept only numeric kinds; `MIN` and `MAX`
SHALL accept numeric, date, date-time, text-like and picklist kinds;
`COUNT(field)` and `COUNT_DISTINCT` SHALL accept any filterable non-long-text
kind. A mismatch SHALL be `MALFORMED_QUERY`. `COUNT()` alone
SHALL return `totalSize` equal to the count and `records: []`. Mixing
aggregates with plain fields without GROUP BY SHALL be `MALFORMED_QUERY`.
`FIELDS(ALL|CUSTOM)` SHALL require `LIMIT` ≤ 200 and SHALL silently omit
fields the principal cannot read. Exact same-path duplicate fields in one
SELECT SHALL be accepted; the record SHALL carry one JSON key for that path
while `columnMetadata` keeps each selected entry. Distinct paths that share a
leaf (for example `Name` and `Account.Name`) SHALL be accepted and shape to
distinct JSON keys. A result-key collision with an alias, a generated `exprN`,
a child key or a `FIELDS()` key SHALL be `MALFORMED_QUERY` with the message
"duplicate field selected". Functions outside the documented set SHALL be
`MALFORMED_QUERY` naming the function.

#### Scenario: null parent at level three
- **WHEN** `SELECT Id, Parent__r.Group__r.Region__r.Name FROM Item__c` runs and
  `Parent__c` is null
- **THEN** the response has `"Parent__r": null` and no error

#### Scenario: child subquery shape
- **WHEN** `SELECT Name, (SELECT LastName FROM Contacts ORDER BY LastName LIMIT 2) FROM Account` runs
- **THEN** each account carries a contacts object with `totalSize`, `done: true` and `records`

#### Scenario: COUNT() alone
- **WHEN** `SELECT COUNT() FROM Item__c` runs over n rows
- **THEN** `totalSize` is n, `done` is true and `records` is empty

#### Scenario: date function as a select item
- **WHEN** `SELECT CALENDAR_YEAR(CreatedDate) y, COUNT(Id) FROM Item__c GROUP BY CALENDAR_YEAR(CreatedDate)` runs
- **THEN** rows are `AggregateResult` with keys `y` and `expr0`

#### Scenario: unaliased aggregates number from expr0
- **WHEN** `SELECT Stage__c, COUNT(Id) total, SUM(Amount__c), AVG(Amount__c), MIN(Day__c), MAX(At__c) FROM Item__c GROUP BY Stage__c` runs
- **THEN** rows are `AggregateResult` with keys `Stage__c`, `total`, `expr0`, `expr1`, `expr2`, `expr3`

#### Scenario: SUM over a text field
- **WHEN** `SELECT SUM(Txt__c) FROM Item__c` is bound
- **THEN** the result is `MALFORMED_QUERY`

### Requirement: WHERE types literals and filters by documented operators
<!-- id: soql-03 -->
The WHERE clause SHALL accept `=`, `!=`, `<>`, `<`, `<=`, `>`, `>=`, `LIKE`,
`IN`, `NOT IN`, `INCLUDES` and `EXCLUDES` with typed literals, plus `AND`,
`OR` and `NOT`. Text-like comparisons SHALL be case-insensitive; Id and
reference values SHALL compare after 15/18-character normalization. `LIKE`
SHALL treat `%` and `_` as wildcards and `\%`/`\_` as escaped wildcards,
matching case-insensitively. `NOT IN` on a nullable field SHALL return rows
whose value is null (`(col IS NULL OR col NOT IN …)`). Semi-join and anti-join
subqueries SHALL select one Id or reference field and contain only WHERE. The
outer field SHALL be Id or a reference; nested semi-joins SHALL be
`MALFORMED_QUERY`; a semi-join under `OR` SHALL be `MALFORMED_QUERY`; a
semi-join selecting a non-Id field SHALL be `MALFORMED_QUERY`. `INCLUDES`/`EXCLUDES`
SHALL apply to multi-select fields using case-folded semicolon lists. A
literal of the wrong type for its field SHALL be `INVALID_FIELD` with the
documented "value of filter criterion … must be of type …" wording and SHALL
not echo the value. `LIKE` on a non-text kind SHALL be
`INVALID_QUERY_FILTER_OPERATOR`. `INCLUDES` on a non-multi-select SHALL be
`INVALID_QUERY_FILTER_OPERATOR`. `= null` and `!= null` SHALL be the only
null tests. `toLabel(field)` SHALL be allowed in WHERE and SHALL compare
against labels. Date-time literals SHALL require `Z` or an explicit offset.

#### Scenario: NOT IN includes null rows
- **WHEN** `SELECT Id FROM Item__c WHERE Parent__c NOT IN ('a','b')` runs over rows including null parents
- **THEN** rows with null `Parent__c` are included

#### Scenario: wrong-type literal never echoes the value
- **WHEN** `Num__c = '5'` is filtered on a numeric field
- **THEN** the error is `INVALID_FIELD` and its message contains neither `'5'` nor the raw query text

#### Scenario: toLabel filters by label
- **WHEN** `SELECT Id FROM Item__c WHERE toLabel(Stage__c) = 'Closed label'` runs
- **THEN** only rows whose stage label is `Closed label` are returned

#### Scenario: semi-join under OR is refused
- **WHEN** `SELECT Id FROM Item__c WHERE Name = 'a' OR Id IN (SELECT AccountId FROM Contact)` is bound
- **THEN** the result is `MALFORMED_QUERY`

### Requirement: Date literals and date functions are UTC with documented ranges
<!-- id: soql-04 -->
Date literals and date functions SHALL be evaluated in UTC. Weeks SHALL start
on Sunday. Fiscal periods SHALL equal calendar periods. Comparison operators
against date literals SHALL become half-open range predicates: `=` means
`[start, end)`, `<` means `< start`, `>` means `>= end`. `LAST_N_DAYS:n`
SHALL include today. Relative-period `N` values SHALL follow the documented
bounds (days 1–1,000). A `Clock` injected as `Options.Now` SHALL control
evaluation for tests. Out-of-range `N` SHALL be `MALFORMED_QUERY`.
`DateLiteralRange` SHALL return UTC times with a half-open end.

#### Scenario: leap-day TODAY
- **WHEN** `TODAY` is evaluated on 2028-02-29
- **THEN** the range is [2028-02-29, 2028-03-01)

#### Scenario: THIS_QUARTER at year end
- **WHEN** `THIS_QUARTER` is evaluated on 2026-12-31
- **THEN** the range is [2026-10-01, 2027-01-01)

#### Scenario: LAST_N_DAYS includes today
- **WHEN** `LAST_N_DAYS:30` is evaluated on 2026-11-11
- **THEN** the range is [2026-10-12, 2026-11-12)

#### Scenario: out-of-range N
- **WHEN** `LAST_N_DAYS:1001` is parsed
- **THEN** the result is `MALFORMED_QUERY`

### Requirement: Grouping, ordering and WITH follow documented rules
<!-- id: soql-05 -->
GROUP BY SHALL accept fields, parent paths and date functions; ungroupable
kinds SHALL be `MALFORMED_QUERY`. HAVING SHALL take aggregate and grouped-field
comparisons. ORDER BY SHALL accept multiple keys with ASC/DESC and NULLS
placement, up to 32 keys. Keys SHALL be fields, parent paths, aggregates,
aliases and date functions; unsortable kinds SHALL be `INVALID_FIELD`. LIMIT
SHALL allow values above 2,000 (paged).
OFFSET SHALL allow at most 2,000 and SHALL not combine with locator paging of
the same result (one page only). `WITH SECURITY_ENFORCED` and `WITH USER_MODE`
SHALL be accepted. `WITH SYSTEM_MODE` SHALL be `MALFORMED_QUERY`. Mixed `AND`
and `OR` at one parenthesis level without parentheses SHALL be
`MALFORMED_QUERY`.

#### Scenario: multi-key ORDER BY with NULLS LAST
- **WHEN** a query orders by two keys, the first with NULLS LAST
- **THEN** nulls sort last on that key and the second key breaks ties

#### Scenario: OFFSET cap
- **WHEN** a query uses `OFFSET 2001`
- **THEN** the result is `MALFORMED_QUERY`

### Requirement: Structural limits and a closed error vocabulary
<!-- id: soql-06 -->
The engine SHALL refuse queries longer than 100,000 characters, WHERE clauses
longer than 4,000 characters, relationship paths deeper than five levels,
more than 20 child subqueries, more than two semi- or anti-joins, more than 35
joined objects, IN lists longer than 1,000 elements, more than 2,000 child
rows for one parent in a child subquery without a LIMIT, more than 2,000 result
groups and locator result sets larger than 50,000 IDs. An explicit child
subquery `LIMIT` of 2,000 or less SHALL return that bounded set, including
exactly `LIMIT 2000`; a `LIMIT` above 2,000 SHALL NOT raise the cap and SHALL
fail with `QUERY_TOO_COMPLICATED` when the child set reaches 2,001. Child
subquery results SHALL NOT have an independent `nextRecordsUrl`; only the
outer result pages. Child overflow SHALL fail closed without truncation.
Over-limit refusals SHALL use `QUERY_TOO_COMPLICATED`
except where the plan documents `MALFORMED_QUERY` (FIELDS without LIMIT,
OFFSET 2001, out-of-range date N). The only query error codes SHALL be
`MALFORMED_QUERY`, `INVALID_FIELD`, `INVALID_TYPE`,
`INVALID_QUERY_FILTER_OPERATOR`, `QUERY_TOO_COMPLICATED` and
`INVALID_QUERY_LOCATOR`. Error messages SHALL name fields and constructs and
SHALL never echo query values. `TYPEOF` SHALL be `MALFORMED_QUERY` naming the
construct. An unknown object SHALL be `INVALID_TYPE`. An unknown field or
relationship SHALL be `INVALID_FIELD`.

#### Scenario: IN list cap
- **WHEN** an IN list has 1,001 literals
- **THEN** the result is `QUERY_TOO_COMPLICATED`

#### Scenario: child overflow is fail-closed
- **WHEN** one parent has 2,001 child rows in a subquery without LIMIT
- **THEN** the query fails with `QUERY_TOO_COMPLICATED` and no truncated child set is returned

#### Scenario: child LIMIT at the cap
- **WHEN** one parent has 2,001 child rows and the subquery uses `LIMIT 2000`
- **THEN** the result carries exactly 2,000 child rows with `done: true`, and the child result has no independent `nextRecordsUrl`

#### Scenario: child LIMIT above the cap
- **WHEN** one parent has 2,001 child rows and the subquery uses `LIMIT 2001`
- **THEN** the query fails with `QUERY_TOO_COMPLICATED` and no truncated child set is returned

### Requirement: Field policy and tenant isolation hold in every clause
<!-- id: soql-07 -->
The binder SHALL refuse fields the principal cannot read in SELECT, WHERE,
ORDER BY, GROUP BY, `COUNT()` arguments, parent-path leaves and child
subquery selects (`INVALID_FIELD`). `FIELDS()` SHALL omit denied fields
silently. Cross-tenant rows SHALL never appear through any construct. The
restricted-role denial of `Compat_Text__c` SHALL hold through
`Contact.Account.Compat_Text__c`, child selects and aggregates. Object access
stays the current two-role policy, applied per joined object.

#### Scenario: denied field in a parent path
- **WHEN** the restricted principal selects `Contact.Account.Compat_Text__c`
- **THEN** the result is `INVALID_FIELD` and no value is returned

#### Scenario: FIELDS(ALL) drops denied fields
- **WHEN** the restricted principal runs `FIELDS(ALL) … LIMIT 200`
- **THEN** the denied field is absent from the row and the query succeeds

### Requirement: Child relationships are named stored metadata
<!-- id: child-01 -->
Lookup and master-detail fields SHALL carry a `childRelationshipName`.
`DefaultChildRelationshipName` SHALL derive the plural label with
non-alphanumeric characters removed, plus `__r` on custom children
("Typed items" → `Typeditems__r`). An explicit name SHALL be kept; a
duplicate name on the same parent (case-insensitive) SHALL error; an invalid
name SHALL error. `Snapshot.ChildRelationships` and
`Snapshot.ChildRelationship(parent, name)` SHALL resolve names
case-insensitively. `records.Describe` SHALL surface `childRelationships`
with `childSObject`, `field`, `relationshipName`, `cascadeDelete`,
`restrictedDelete` and `deprecatedAndHidden`, applying field policy (a
restricted role does not see children through a denied field). Migration
0005 SHALL backfill existing tenants, deduplicating with a numeric suffix
("2"), without changing any value or ID. CLI flag
`--child-relationship-name` SHALL set an explicit name.

#### Scenario: default name from plural label
- **WHEN** a custom child object has plural label "Typed items"
- **THEN** the default relationship name is `Typeditems__r`

#### Scenario: describe lists standard children
- **WHEN** Account is described
- **THEN** `childRelationships` includes `{childSObject: Contact, field: AccountId, relationshipName: Contacts}`

### Requirement: Paging uses query locators with fixed membership
<!-- id: page-01 -->
Results larger than one page SHALL be paged with server-side locators in
`sc_query.locators(tenant_id, id, user_id, query_sha256, plan jsonb, ids text[],
created_at, expires_at)`, primary key `(tenant_id, id)`, with RLS and
records-role SELECT/INSERT/DELETE. Locator IDs SHALL be `01g` plus 15 random
base62 characters. Membership SHALL be fixed at the first page: later pages
re-read stored IDs in order (`array_position`), show current field values,
skip deleted rows and keep `totalSize` at its first value. `totalSize` SHALL
be the query's result size after its own `LIMIT`/`OFFSET` are applied. The stored plan
SHALL be the canonical re-serialized query text plus catalog revision and
SHALL be re-parsed and re-bound per page; if a selected field is removed or
denied after page one, the next page SHALL be `INVALID_QUERY_LOCATOR` and
SHALL not leak the field. Expiry SHALL be two days. At most 10 open locators
per user: opening an eleventh SHALL evict the oldest. More than 50,000 IDs
SHALL be `QUERY_TOO_COMPLICATED`. Aggregate, `COUNT()` and OFFSET queries
SHALL create no locator. `nextRecordsUrl` SHALL be
`/services/data/v65.0/query/{locator}-{offset}`. `Sforce-Query-Options:
batchSize=n` SHALL clamp to 200–2,000. Another user or tenant SHALL receive
`INVALID_QUERY_LOCATOR`. A malformed locator path SHALL be
`INVALID_QUERY_LOCATOR`. Exactly 2,000 rows SHALL return one page with
`done: true` and no `nextRecordsUrl`; 2,001 rows SHALL return two pages.

#### Scenario: exact page boundary
- **WHEN** a query returns exactly 2,000 rows at the default batch
- **THEN** the first page has `done: true` and no `nextRecordsUrl`

#### Scenario: totalSize honors the query LIMIT
- **WHEN** a query with `LIMIT 4500` matches more rows and is served at batch 500
- **THEN** `totalSize` is 4500 and there are nine pages

#### Scenario: deletions between pages
- **WHEN** rows from page 3 are deleted after page 1 is served
- **THEN** page 3 omits them, nothing repeats, and `totalSize` is unchanged

#### Scenario: catalog change invalidates the next page
- **WHEN** a selected field is denied or removed after page 1
- **THEN** page 2 returns `INVALID_QUERY_LOCATOR` and does not include that field

### Requirement: REST JSON shape stays compatible for queries that work today
<!-- id: soql-08 -->
Record rows SHALL carry `attributes` with `type` and `url`. Aggregate rows
SHALL be typed `AggregateResult`. Query results SHALL be
`{totalSize, done, records}` with `nextRecordsUrl` when pages remain.
Selected values SHALL use Plan 28's `selectExpression`/`wireValue` shapes
(exact decimals, Plan 28 REST date forms). Every query that works today SHALL
return byte-identical JSON. The 35 observed column cases SHALL stay green
except `unsupported-traversal` and `unsupported-aggregate`, which become
supported. Column metadata SHALL report `{columnName}` for plain fields,
nested `joinColumns` for parent paths, `{columnName: rel, joinColumns}` for
subqueries and `{columnName: key, aggregate: true}` for aggregates.

#### Scenario: existing queries are unchanged
- **WHEN** each query in the existing foundation/query suites and the 35-case
  column fixture (except the two that become supported) runs through the engine
- **THEN** the JSON is byte-identical to today's output

#### Scenario: aggregate row type
- **WHEN** `SELECT Stage__c, COUNT(Id) total FROM Item__c GROUP BY Stage__c` runs
- **THEN** each row has `attributes.type` equal to `AggregateResult`
