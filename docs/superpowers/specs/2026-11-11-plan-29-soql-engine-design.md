# Plan 29 — SOQL engine

**Status:** Draft for maintainer review, 2026-11-11.
**Date:** 2026-11-11
**Depends on:** Plan 27 (tenant catalog, RLS, record service), Plan 28 (typed fields, relationship metadata). Uses the paging decisions recorded in Plan 25.
**Roadmap:** [first release roadmap](2026-11-04-salescompel-first-release-roadmap-design.md), Plan 29 (Full depth); [Plan 25 decisions for Plan 29](2026-11-04-plan-25-foundation-decisions-design.md#decisions-recorded-for-plan-29).
**Implementation:** [Plan 29](../plans/2026-11-11-plan-29-soql-engine.md).

## Intent and outcome

Every Salesforce client, report and integration speaks SOQL. Today the candidate accepts one shape only: `SELECT fields FROM object [WHERE field = 'text'] [ORDER BY one field] [LIMIT ≤2000]`, plus one hard-coded `Contact.Account.Name` join. Real tools fail on anything beyond that:
- `sf data query` with a relationship;
- JSforce `autoFetch` over more than 2,000 rows;
- simple-salesforce `query_all`;
- any date, number or multi-condition filter;
- a parent-to-child subquery;
- a `COUNT()`.

Plan 28 also left typed WHERE literals to this plan.

After Plan 29, a single engine parses, validates, secures and executes the SOQL subset below against the tenant catalog. It serves the REST query endpoint, Composite, the connected editor and later plans (reports in 43, list views in 42, Bulk query in 32). Unmodified `sf data query`, JSforce and simple-salesforce can:
- run relationship, aggregate, date-literal and multi-condition queries;
- page through large results with `nextRecordsUrl`.

The maintainer handed scope decisions to this design ("write the spec from the roadmap and Salesforce's abilities"). Salesforce behavior below comes from the public SOQL and REST documentation. It was not observed on the trial org during drafting, and Task 1 confirms what it can. Choices that are not in the roadmap or the Plan 25 decision table are marked *candidate* and listed at the end.

## Scope and boundaries

Ships:

- **SELECT list:**
  - fields, including compound Address fields;
  - parent relationship paths up to 5 levels (`Contact.Account.Name`, `Item__c.Parent__r.Name`);
  - parent-to-child subqueries one level deep;
  - aggregates: `COUNT()`, `COUNT(field)`, `COUNT_DISTINCT`, `SUM`, `AVG`, `MIN`, `MAX`, with aliases;
  - `toLabel()` on picklists;
  - `FIELDS(STANDARD)`, `FIELDS(CUSTOM)` and `FIELDS(ALL)`.
- **FROM:** one object, with an optional alias.
- **WHERE:**
  - comparison operators `=`, `!=`, `<>`, `<`, `<=`, `>`, `>=`;
  - `LIKE`, `IN`, `NOT IN`, `INCLUDES`, `EXCLUDES`;
  - `AND`, `OR`, `NOT` and parentheses;
  - typed literals: strings, numbers, booleans, `null`, dates, date-times, and the documented date literals;
  - date functions;
  - semi-joins and anti-joins (`Id IN (SELECT …)`).
- **Grouping and paging:**
  - `GROUP BY` and `HAVING`;
  - `ORDER BY` with multiple keys, `ASC`/`DESC` and `NULLS FIRST`/`LAST`;
  - `LIMIT` and `OFFSET`;
  - `WITH SECURITY_ENFORCED` and `WITH USER_MODE`.
- **Paging** with server-side query locators, `nextRecordsUrl`, `GET /query/{locator}` and the `Sforce-Query-Options: batchSize` header, following Plan 25's decisions.
- **Child relationship metadata:** `childRelationships` in describe, the child relationship name on lookups and master-detail fields, and a migration that backfills existing ones.
- **Field security everywhere a field can appear:** select, filter, order, group, aggregate, relationship path and subquery.
- **Column metadata (`columns=true`)** extended to relationship, subquery and aggregate columns.
- **Contract and tests:** OpenAPI and traffic validation for the query, paging and new error shapes; acceptance through `sf data query`, JSforce and simple-salesforce.

Non-goals:
- SOSL and `FIND` (Plan 42);
- `queryAll`, deleted rows and `ALL ROWS` (Plan 30);
- `TYPEOF` and polymorphic relationships: the AST is designed for them, but no polymorphic field exists until Plans 30 and 40;
- `USING SCOPE`, which needs ownership (Plan 34);
- `FOR VIEW`, `FOR REFERENCE` and `FOR UPDATE`;
- `GROUP BY ROLLUP`/`CUBE` and `GROUPING()`;
- `FORMAT()` and `convertCurrency()` (no multi-currency);
- distance and geolocation functions;
- `WITH DATA CATEGORY`;
- `UPDATE TRACKING`/`VIEWSTAT`;
- Bulk query (Plan 32);
- Composite query subrequests (Plan 31);
- the Tooling API;
- query plans (`explain`);
- user time zones and locales for date literals (Plan 33);
- fiscal-year settings, where fiscal is calendar until configurable;
- relevance or search ranking.

Anti-goals:
- A query that silently returns a field, row or relationship the principal may not read, or a different tenant's row through a join or locator.
- Unsupported syntax accepted and quietly ignored, so a client gets the wrong rows. Every refusal must be an explicit error.
- Building SQL from user text. Only validated catalog identifiers are interpolated, quoted; every value is a parameter.
- Results whose types differ from the record read path: numbers must be exact, and dates and date-times use the REST forms from Plan 28.
- Paging that duplicates or drops rows, or lets one user resume another's locator.
- Any change to the observed Plan 25 behavior for queries that work today: canonical names, null placement, error codes and the 35 column cases. The only exceptions are cases that move from "refused" to "supported", each listed in §9.

## Global constraints

Plan 27 and 28 constraints carry over:
- Go 1.25.0 and PostgreSQL 17;
- API v65.0 and OpenAPI 3.2.1 with `x-source`;
- typed relational storage, RLS on every table and the transaction-local tenant context;
- owner-only DDL and forward-only migrations;
- exact decimals and no float64 for field values;
- product copy in the five catalogs;
- `CLEAN_ROOM.md`.

No new runtime dependency: the lexer and parser are hand-written Go (*candidate*), with no parser generator.

## 1. Engine architecture

A new package, `internal/soql`, replaces the bounded `internal/query` grammar. `internal/query` stays as a thin facade until callers move. The engine has five stages, each with its own tests:

1. **Lexer.** Builds tokens with source offsets: keywords (case-insensitive), identifiers, string literals with the documented escapes (`\'`, `\"`, `\\`, `\n`, `\r`, `\t`, `\b`, `\f`, `\u` and, in LIKE patterns, `\%` and `\_`), number, date, date-time and date literal tokens, operators and punctuation. It keeps today's UTF-8 and NUL rejection.
2. **Parser.** Produces an AST: `Query{Select []SelectItem; From; Alias; Where Expr; GroupBy; Having; OrderBy; Limit; Offset; With}`. `SelectItem` is a field path, subquery, aggregate, `toLabel` or `FIELDS()`. `Expr` is a boolean tree of comparisons, `IN`/`NOT IN` (with value lists or semi-join subqueries) and `INCLUDES`/`EXCLUDES`. `TYPEOF` parses into a node the binder refuses, so later plans only add binding.
3. **Binder.** Resolves every name against the tenant snapshot, case-insensitively with canonical spelling: the object, alias, fields, relationship paths, child relationship names and aggregate aliases. It types every literal against its field, applies field policy and enforces the structural rules and limits in §5. After this stage nothing refers to user text, only to catalog entries and typed values.
4. **Compiler.** Turns the bound AST into one parameterized PostgreSQL statement, plus one statement per child subquery.
   - Parent paths become `LEFT JOIN`s on `(tenant_id, id)` with an explicit tenant predicate on every joined table (RLS stays the backstop).
   - Child subqueries run as a second query per page over the page's parent IDs (`parent_col = ANY($ids)`), ordered and limited per parent with a window function.
   - Semi- and anti-joins become `IN`/`NOT IN (SELECT …)` with tenant predicates.
   - Aggregates become `GROUP BY`/`HAVING` with typed casts.
   - Text comparisons use the existing case-insensitive `lower(... COLLATE pg_c_utf8)` form.
5. **Executor and shaper.** Runs inside the caller's record transaction (`records.Tx`). It shapes rows into the REST JSON:
   - `attributes {type, url}`;
   - parent relationships as nested objects, or `null`;
   - child relationships as `{totalSize, done, records}`, or `null` when empty;
   - aggregate rows typed `AggregateResult` with `expr0…` or alias keys.

   Values use Plan 28's `wireValue` (exact numbers, REST dates).

`records.Tx.Select`, the HTTP query handler, the connected editor's queries and the `isOrganizationDiscovery`/currency bridges all go through the engine. The two Organization bridges stay exact-match shortcuts ahead of the engine.

## 2. SELECT

**Fields** follow today's rules: canonical names and field policy. Compound Address fields return the Plan 28 object. Duplicate fields in one SELECT are rejected with `MALFORMED_QUERY` ("duplicate field selected"). This is documented behavior and is today's observed `malformed-duplicate` case.

**Parent relationships** use the relationship name: `Account` for standard `AccountId`, `Parent__r` for custom `Parent__c`. Paths are up to 5 levels (documented limit). The JSON nests each level, with its own `attributes`. A null reference gives `"Account": null`. A field that is not a reference, or an unknown relationship name, gives `INVALID_FIELD` with the documented "Didn't understand relationship" wording (*candidate* message). `Id` is not implicitly added to parent objects unless selected, matching documented REST output.

**Child subqueries:** `(SELECT … FROM Contacts)` or `(SELECT … FROM Items__r)` inside the outer SELECT.
- The FROM names a child relationship of the outer object.
- The inner query supports fields, parent paths, WHERE, ORDER BY and LIMIT. It does not support aggregates, GROUP BY, OFFSET or nested subqueries (documented: one level).
- At most 20 subqueries per query (documented).
- Each parent row gets `{ "totalSize", "done": true, "records" }`, or `null` when there are no child rows (documented REST shape).
- A parent's child set is capped at 2,000 rows per page (*candidate*). If a parent has more, the query fails with `QUERY_TOO_COMPLICATED` ("child relationship exceeds candidate limit; add a LIMIT to the subquery"). Salesforce instead pages large child sets; that is recorded as a gap.

**Aggregates:**
- `COUNT()` with no other select items returns `totalSize` = the count and `records: []` (documented).
- `COUNT(field)`, `COUNT_DISTINCT(field)`, `SUM`, `AVG`, `MIN` and `MAX` return rows of type `AggregateResult`. Keys are the alias or `expr0`, `expr1`, … in select order, and grouped fields keep their names.
  - `SUM` and `AVG` apply to numeric kinds.
  - `MIN` and `MAX` apply to numeric, date, date-time, text-like and picklist kinds.
  - `COUNT_DISTINCT` and `COUNT(field)` apply to any filterable, non-long-text kind.
  - Each rule follows the documented field-type support. A mismatch gives `MALFORMED_QUERY`.
- Numeric results are exact (`AVG` keeps PostgreSQL `numeric` precision, written with `DecimalJSON`).
- Mixing aggregate and plain fields without GROUP BY gives `MALFORMED_QUERY` (documented).
- Aggregate queries are not paged. More than 2,000 result groups gives `QUERY_TOO_COMPLICATED` (documented as unsupported by queryMore).

**`toLabel(Picklist__c)`** returns the picklist label instead of the API value. Labels are tenant data with no translation (translation is a later plan). It is also allowed in WHERE, comparing against labels (documented). Other functions in SELECT give `MALFORMED_QUERY` naming the function as not supported.

**FIELDS():**
- `FIELDS(STANDARD)` selects the object's standard fields, which on custom objects means `Id` and `Name`.
- `FIELDS(CUSTOM)` selects custom fields; `FIELDS(ALL)` selects both.
- `CUSTOM` and `ALL` require `LIMIT` ≤ 200 (documented).
- Compound fields are expanded as they appear in describe.
- Fields the principal cannot read are left out (*candidate*; Salesforce applies field-level security the same way for `FIELDS()`).

## 3. WHERE

**Comparisons:** `=`, `!=`, `<>`, `<`, `<=`, `>`, `>=` on every filterable kind, with typed literals.
- Strings are quoted. Numbers, booleans, dates, date-times and date literals are not (documented).
- A literal of the wrong type for its field gives `INVALID_FIELD` with the documented "value of filter criterion for field 'X' must be of type … and should not be enclosed in quotes" wording (*candidate* code).
- Numbers parse with Plan 28's exact decimal rules and support exponents. Range comparisons on text use the stored text order.
- Comparisons with `null` (`= null`, `!= null`) are the only null tests, as documented.
- Text-like kinds compare case-insensitively: Text, TextArea, Email, Phone, Url, Picklist, multi-select element values, AutoNumber, PersonName and Name. This fixes the Plan 28 deferred note on picklist and auto-number equality. Id and reference values compare after 15/18-character normalization, exactly.

**LIKE** applies to text-like kinds. `%` and `_` are wildcards, `\%` and `\_` escape them, and matching is case-insensitive (documented). The compiler translates patterns to `ILIKE … ESCAPE` with parameter values only.

**IN / NOT IN** take a parenthesized literal list, or a semi-join or anti-join subquery.
- The subquery selects one Id or reference field from another object and has only WHERE.
- Documented restrictions apply:
  - the outer field must be `Id` or a reference;
  - no nesting;
  - at most two semi- or anti-joins per query;
  - no `OR` around them.
- `NOT IN` follows SQL three-valued logic. Salesforce returns rows whose value is null for `NOT IN` on a nullable field, so the compiler writes `(col IS NULL OR col NOT IN …)` (*candidate*, to observe).

**INCLUDES / EXCLUDES** apply to multi-select picklists only.
- Each element is a string. `'A;B'` inside one element means both values (documented).
- Matching is on canonical stored values, case-insensitively.
- Storage stays the Plan 28 canonical text. The compiler splits it with `string_to_array(col, ';')` and uses array operators, so no new index is required (*candidate*).
- `=` on multi-select compares the whole canonical value, which Salesforce documents.

**Boolean logic:** `AND`, `OR`, `NOT` and parentheses, with standard precedence (`NOT` > `AND` > `OR`). Mixing `AND` and `OR` without parentheses at the same level gives `MALFORMED_QUERY`, as Salesforce documents.

**Date-times and date literals:**
- Date-time literals must carry `Z` or an offset (documented).
- Date literals compare against date fields directly and against date-time fields as UTC ranges (*candidate* until user time zones exist; documented behavior uses the user's time zone):
  - `YESTERDAY`, `TODAY`, `TOMORROW`;
  - `LAST_WEEK`, `THIS_WEEK`, `NEXT_WEEK`;
  - `LAST_MONTH`, `THIS_MONTH`, `NEXT_MONTH`;
  - `LAST_90_DAYS`, `NEXT_90_DAYS`;
  - `LAST_N_DAYS:n`, `NEXT_N_DAYS:n`, `N_DAYS_AGO:n`;
  - the `WEEKS`, `MONTHS`, `QUARTERS` and `YEARS` equivalents (`LAST_N_*`, `NEXT_N_*`, `N_*_AGO`);
  - `THIS_QUARTER`, `LAST_QUARTER`, `NEXT_QUARTER`;
  - `THIS_YEAR`, `LAST_YEAR`, `NEXT_YEAR`;
  - the `FISCAL_*` forms, evaluated as calendar periods.
- Weeks start on Sunday (en_US default) (*candidate*).
- `n` is 1–1,000 for days and documented bounds for others.
- Equality means "within the range". `<` and `>` compare against the range start or end as documented.
- The engine evaluates literals with a clock injected per query, so tests are deterministic.

**Date functions** `CALENDAR_YEAR`, `CALENDAR_QUARTER`, `CALENDAR_MONTH`, `DAY_IN_MONTH`, `DAY_IN_WEEK`, `DAY_IN_YEAR`, `DAY_ONLY`, `HOUR_IN_DAY`, `WEEK_IN_MONTH`, `WEEK_IN_YEAR` and the `FISCAL_*` equivalents (evaluated as calendar) are allowed in WHERE, GROUP BY, HAVING and ORDER BY on date and date-time fields. They use UTC (*candidate*).

## 4. GROUP BY, HAVING, ORDER BY, LIMIT, OFFSET, WITH

**GROUP BY** takes fields, parent paths and date functions. Ungroupable kinds give `MALFORMED_QUERY`, following describe's `groupable`: long text, multi-select, address, double/currency/percent and date-time all refuse grouping, matching the documented rule that "fields of type double cannot be grouped". **HAVING** takes aggregate comparisons and grouped-field comparisons.

**ORDER BY:**
- Up to 32 keys (documented). Keys can be fields, parent paths, aggregates and aliases in aggregate queries, and date functions.
- Unsortable kinds give `INVALID_FIELD`, as Plan 28 already enforces.
- Null placement keeps Plan 25: nulls first in both directions by default, with `NULLS FIRST`/`LAST` honored.
- The stable final tie-break is `Id`; for aggregate queries it is the group keys.

**LIMIT** has no upper bound beyond paging: values above 2,000 are valid and paged (Plan 25). **OFFSET** goes up to 2,000 (documented). It is not allowed in subqueries, and not with locator paging of the result (documented); a query with OFFSET returns at most one page.

**WITH:**
- `WITH SECURITY_ENFORCED` and `WITH USER_MODE` are accepted. The candidate already enforces field policy for the user in every mode and refuses inaccessible fields with `INVALID_FIELD`, which is the documented SECURITY_ENFORCED behavior.
- `WITH SYSTEM_MODE` gives `MALFORMED_QUERY`, since there is no system mode for API callers.
- Other WITH clauses are refused.

## 5. Structural limits and errors

Documented limits, enforced in the binder with explicit errors:
- query text ≤ 100,000 characters;
- WHERE clause ≤ 4,000 characters, measured on the clause text;
- ≤ 5 relationship levels;
- ≤ 20 child subqueries;
- ≤ 2 semi- and anti-joins;
- ≤ 35 joined objects across the query;
- `IN` lists ≤ 1,000 elements (*candidate*; Salesforce's bound is the clause length).

Errors stay in today's REST error envelope:
- `MALFORMED_QUERY` for syntax and unsupported constructs, with a message that names the construct;
- `INVALID_FIELD` for unknown, denied, unfilterable, unsortable or wrongly typed fields;
- `INVALID_TYPE` for an unknown object;
- `INVALID_QUERY_FILTER_OPERATOR` for an operator invalid for the field's type or a malformed ID filter (existing);
- `QUERY_TOO_COMPLICATED` for the limits above;
- `INVALID_QUERY_LOCATOR` for an expired, unknown or foreign locator.

Messages never echo values from the query, only field names and constructs (*candidate*, to keep error logs free of data).

## 6. Paging and locators

This follows Plan 25's decision table.
- The default page is 2,000 rows. `totalSize` counts all rows, and `done` is false while pages remain.
- `nextRecordsUrl` is `/services/data/v65.0/query/{locator}-{offset}`, the documented shape.
- `Sforce-Query-Options: batchSize=n` is honored between 200 and 2,000 (documented bounds). Values outside are clamped to the nearest bound (*candidate*; documentation says the batch size is a hint).

**Membership is fixed at the first page.**
- The first request runs the query and stores the ordered result IDs in a locator row.
- Later pages re-read those IDs in stored order, so they show current field values, which is what Plan 25 decided.
- Rows deleted since the first page are skipped, and `totalSize` stays as first reported (Plan 25 default).
- Child subqueries and parent paths are evaluated per page.

**Locators are stored server-side** in a new tenant-scoped table `sc_query.locators(tenant_id, id, user_id, query_sha256, plan jsonb, ids text[], created_at, expires_at)`.
- The table has RLS, and the records role may SELECT, INSERT and DELETE.
- `plan` holds the bound query in a versioned, validated form. It never stores raw SQL to replay, and it is re-validated against the current catalog for each page. If a field was removed or denied since, the page fails with `INVALID_QUERY_LOCATOR` (*candidate*).
- Locators expire after two days (Plan 25).
- The same user may replay a page and use it from another session.
- Another user's locator returns 400 `INVALID_QUERY_LOCATOR`, as does one from another tenant (RLS returns nothing).
- At most 10 open locators per user: the oldest is deleted when an eleventh opens (documented limit). Expired rows are deleted when they are encountered and by a cleanup query run on each new locator.
- A locator stores at most 50,000 IDs. A larger result gives `QUERY_TOO_COMPLICATED` with a message to narrow the query (*candidate*, recorded as a gap; Salesforce has no such cap).
- Aggregate, `COUNT()` and OFFSET queries never create locators.

`GET /services/data/v65.0/query/{locator}-{offset}` serves later pages with the same response shape. A malformed locator path returns `INVALID_QUERY_LOCATOR`.

## 7. Child relationship metadata

Subqueries need child relationship names, which describe does not expose today.

- **Field definitions** for Lookup and MasterDetail gain a stored `childRelationshipName`.
  - The schema CLI takes `--child-relationship-name`.
  - The default is the child object's plural label with non-alphanumeric characters removed. Custom objects get `__r` appended (`Typed items` → `Typeditems__r`) (*candidate*; Salesforce derives it from the plural label and lets admins edit it).
  - The name must be a valid identifier and unique among the parent object's child relationships, case-insensitively. A collision with the default is an explicit CLI error asking for the flag.
- **Seeds:** `Contact.AccountId` gets `Contacts` (documented standard name).
- **Migration 0005** backfills existing tenant lookups and master-details with the default rule, deduplicating with a numeric suffix (*candidate*), and records it in the definition JSON.
- **Object describe gains `childRelationships`:** `childSObject`, `field`, `relationshipName`, `cascadeDelete` and `restrictedDelete`, plus the always-false `deprecatedAndHidden` flag documented in the DescribeSObjectResult. Children whose relationship field the principal cannot read are left out.

## 8. Security

The binder applies `policy.Field` to every field reference in any clause, at every relationship level and in every subquery. A denied field anywhere gives `INVALID_FIELD`, exactly like today's projection, filter and order denial.
- `FIELDS()` silently leaves denied fields out (see §2).
- Object access stays the current two-role candidate policy until Plan 33, applied per joined object.
- Joins and subqueries carry explicit tenant predicates under RLS.
- Locators are bound to tenant and user.
- `toLabel` and aggregates follow field policy for their argument.
- The restricted role's existing `Account.Compat_Text__c` denial must hold through `Contact.Account.Compat_Text__c`, `(SELECT Compat_Text__c FROM …)`, `COUNT(Compat_Text__c)`, `GROUP BY` and `FIELDS(ALL)`.

## 9. Compatibility with existing behavior

Queries that work today must return byte-identical JSON. The 35 observed column cases stay green, except these cases that move from refused to supported:
- `unsupported-traversal` becomes a supported parent path;
- `unsupported-aggregate` becomes `COUNT()`.

The fixture is updated with the new expected bodies. Observed basis tags are kept only where the observation still applies, and the change is recorded in the compat doc.

Other behavior changes:
- Today's "Result exceeds candidate limit" rejection for more than 2,000 rows is replaced by paging.
- `LIMIT` above 2,000 becomes valid.
- The editor's `LIMIT 2000` query keeps working.

**Column metadata (`columns=true`):**
- Plain fields keep `{columnName}`.
- Parent paths return nested `joinColumns`, extending today's `Account.Name` shape to any depth.
- Subqueries return `{columnName: relationshipName, joinColumns: [...]}`.
- Aggregates return `{columnName: alias|exprN, aggregate: true}`.

The richer CLI metadata (`apexType`, `displayName` and so on) remains a gap.

## 10. Evidence, acceptance and completion

**Task 1.** Review release notes since 2026-11-11 for SOQL or REST query changes, and fetch the SOQL reference pages; record any that cannot be retrieved. If a trial-org session exists, observe:
- the documented shapes for relationship, child, aggregate and `COUNT()` results;
- `nextRecordsUrl` and batch-size behavior;
- the wrong-type-literal error;
- `NOT IN` with nulls;
- `INCLUDES` case;
- date-literal boundaries.

Record every *candidate* as observed, documented or gap in `docs/compat/plan-29-soql-engine.md`.

**Unit tests:**
- Lexer and parser golden tests over a corpus of valid and invalid queries, each with an expected AST or error.
- Binder tests per rule and limit, field security in every clause, and typed literal errors.
- Compiler tests asserting parameterization: no value text in the SQL string.
- Date-literal evaluation against a fixed clock across month, quarter, year and leap-day boundaries.
- Fuzzing of the lexer and parser with Go's native fuzzing, so no panic is possible on any input.

**Database tests:**
- Every construct runs against a two-tenant fixture with every Plan 28 type.
- Cross-tenant rows never appear through joins, subqueries, semi-joins or locators.
- Restricted-role denial holds in every clause.
- Paging returns every row exactly once across pages, both with concurrent updates and with deletions between pages.
- Locator expiry, the 10-per-user eviction, foreign-user rejection and catalog-change invalidation behave as specified.
- Aggregates are exact on currency.

**Acceptance (extend the JSforce, Python, CLI and schema gates):**
- `sf data query` runs a relationship query and a child subquery and gets correct rows.
- JSforce `query().run({autoFetch: true, maxFetch})` over 4,500 fixture rows with `batchSize` 500 receives every row once.
- simple-salesforce `query_all` over the same set receives every row once, and `query` with a child subquery and `COUNT()` works.
- JSforce aggregate and date-literal queries against fixture dates relative to a seeded clock offset (*candidate*: fixtures are created with dates computed from the run's start time).
- The OpenAPI traffic validator covers every new response shape and error.
- The connected editor still lists records.

Every created fixture is journaled. Large fixture sets use Composite or a bulk-insert helper, following the existing journal rules.

**Complete when:**
- full acceptance passes with no required skips;
- Go race and vet, fuzzing for a fixed time budget, web and Node checks pass;
- the compat record lists every candidate and gap;
- the operator documentation covers query limits and paging;
- an independent review finds nothing outstanding.

## Sources

Not fetched during drafting (Salesforce documentation pages refused automated fetches during Plan 28). Task 1 verifies them.

- [SOQL and SOSL Reference](https://developer.salesforce.com/docs/atlas.en-us.soql_sosl.meta/soql_sosl/sforce_api_calls_soql.htm): SELECT syntax, relationship queries, aggregates, date literals and date functions, limits, `FIELDS()`, `WITH SECURITY_ENFORCED`, `toLabel()`.
- [REST API: Execute a SOQL query](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/dome_query.htm) and [query options header](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/headers_queryoptions.htm): `nextRecordsUrl`, batch size 200–2,000.
- [DescribeSObjectResult](https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-describesobjects-describesobjectresult.html): `childRelationships`.
- [Plan 25 decisions for Plan 29](2026-11-04-plan-25-foundation-decisions-design.md#decisions-recorded-for-plan-29): paging and locator behavior.

## Candidate calls for review

These are the author's calls, not maintainer decisions:

1. A hand-written lexer and parser in a new `internal/soql` package, with no parser generator or new dependency.
2. Child subqueries capped at 2,000 rows per parent per page instead of nested paging (recorded gap).
3. Locators store the ordered result IDs, at most 50,000 per locator (recorded gap), with the bound plan re-validated per page.
4. Batch sizes outside 200–2,000 are clamped, not rejected.
5. Date literals and date functions use UTC with Sunday-start weeks until user time zones and locales arrive (Plan 33). Fiscal periods are calendar.
6. `NOT IN` on nullable fields includes nulls; to observe.
7. Default child relationship name derived from the plural label, with a backfill migration.
8. `FIELDS()` leaves denied fields out rather than failing.
9. `WITH SECURITY_ENFORCED` and `USER_MODE` are accepted as the existing enforcement; `SYSTEM_MODE` is refused.
10. The wrong-type literal error uses `INVALID_FIELD`, and error messages never echo query values.
11. `IN` lists are capped at 1,000 elements.

The builder may choose AST type names, the compiler's SQL forms (within parameterization and tenant predicates), the locator ID format (within the documented URL shape) and test layout. Changing scope, security behavior, paging semantics or error codes requires updating this spec and the plan together.
