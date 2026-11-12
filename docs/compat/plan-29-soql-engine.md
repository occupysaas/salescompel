# Plan 29 - SOQL engine reference and compatibility record

Status: reference review recorded, 2026-11-12; Tasks 2-10 pending; no Salesforce compatibility certification.

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

| Item | Candidate rule | Status | Source / note |
| --- | --- | --- | --- |
| 1. Hand-written parser | New `internal/soql` lexer and parser; no parser generator or new dependency | gap | Spec default and a local implementation choice; no Salesforce source applies. |
| 2. Child subquery cap | Child subqueries capped at 2,000 rows per parent per page instead of nested paging | gap | Salesforce returns up to 2,000 results per request, including results from child objects, and pages larger sets with `nextRecordsUrl` and `queryMore()` ([limits cheatsheet][c1], [REST execute][c2], [queryMore][c3]). The per-parent cap and `QUERY_TOO_COMPLICATED` are recorded deviations. |
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
- **`FIELDS()` duplicate-field error.** The reference says a duplicate in the
  SELECT list is an error, but the exact message is not shown ([SELECT
  fields][c13]).
- **Winter '27 SOQL additions outside scope.** `FORMULA()` in WHERE (Beta, API
  68.0, not in production) and `SET OPTIONS` ([Winter '27 developer
  guide][c23]).
- **Aggregate queries return `QUERY_TOO_COMPLICATED`.** Aggregate functions
  and `COUNT()` are not yet supported by the engine. The binder rejects them
  with `QUERY_TOO_COMPLICATED` ("aggregate queries are not yet supported")
  rather than `MALFORMED_QUERY`. Task 7/9 should note this code choice:
  `QUERY_TOO_COMPLICATED` signals a capability gap that a later task may
  close, while `MALFORMED_QUERY` would imply permanently invalid syntax.

## Items reserved for Task 10

Task 10 completes the gaps list, the error-code table, the operator notes for
query limits and paging, and the acceptance evidence table. This document
records only the reference review and the candidate status table.

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
