# Plan 31 - Composite II and sObject Collections reference record

Status: Task 1 (observation) complete, 2026-11-15; implementation not started.
Nothing here claims Salesforce parity. Each row in the status table says
whether its behavior is `observed`, `documented` or a `gap`.

## Reference review (2026-11-15)

No authorized trial-org session was available, so nothing here is a live
observation. Following the roadmap rule "follow the documentation and log the
gap", every row below is `documented` or `gap`. The only `observed` Composite
behavior is still the Plan 24 pilot rollback in
[pilot-black-box-baseline.yaml](observations/pilot-black-box-baseline.yaml).

Direct WebFetch requests to developer.salesforce.com returned HTTP 403. The
public pages were read in an ordinary browser session, using the site's own
"View as Markdown" form of each page. No client, plugin or Salesforce
implementation code was inspected. Notes below are independent summaries; no
documentation prose or examples are copied.

Changelog review: the Composite and sObject Collections pages read today
describe the API at v68.0. Plans 24 and 29 left no Composite rule that a later
release changed. The documented version splits (reference-ID syntax errors and
null-field references both changed at v52.0) are below the candidate's v65.0,
so only the v52.0-and-later behavior applies.

## Sources retrieved

| Id | Page | Used for |
| --- | --- | --- |
| c1 | [Composite request body](https://developer.salesforce.com/docs/platform/api-rest/guide/requests-composite.html) | Envelope properties, allOrNone, collation, httpHeaders, reference operators, 25/5 limits |
| c2 | [Send multiple requests using Composite](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-composite-post.html) | Allowed subrequest resources |
| c3 | [Composite subrequest result](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-subrequest-result.html) | Result envelope, reference-ID errors, null and unselected field references |
| c4 | [allOrNone in Composite and Collections](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-allornone.html) | Outer/inner allOrNone interaction |
| c5 | [Create records (Collections)](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-sobjects-collections-create.html) | POST collections |
| c6 | [Get records (Collections)](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-sobjects-collections-retrieve.html) | GET collections |
| c7 | [Get records with a body (Collections)](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-sobjects-collections-retrieve-post.html) | POST retrieve collections |
| c8 | [Update records (Collections)](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-sobjects-collections-update.html) | PATCH collections |
| c9 | [Upsert records (Collections)](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-sobjects-collections-upsert.html) | PATCH upsert collections |
| c10 | [Delete records (Collections)](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-sobjects-collections-delete.html) | DELETE collections |

## Status table

| # | Behavior | Status | Source |
| --- | --- | --- | --- |
| 1 | `allOrNone` is optional on Composite; when false, a failed subrequest does not stop independent ones, and subrequests that depend on it are not run. Outer status is 200 either way. | documented | c1 |
| 2 | Default for omitted `allOrNone` on Composite | gap: c1 calls it optional but names no default. The candidate treats omitted as `false`, matching Collections (c5) | c1, c5 |
| 3 | `collateSubrequests` may reorder independent subrequests; with it false, array order is kept. Default true from v49.0. | documented | c1 |
| 4 | Error body of a dependent subrequest whose parent failed under `allOrNone: false` | gap: c3 shows `PROCESSING_HALTED` with a message naming the missing reference for one such case | c3 |
| 5 | Allowed subrequests: every `/sobjects/` resource (including rows by external ID, excluding blob get), Query, QueryAll, and sObject Collections | documented | c2 |
| 6 | At most 25 subrequests, of which at most 5 are Collections or query operations | documented | c1, c2 |
| 7 | Reference syntax `@{ref.path}` with `.` for fields and `[n]` for array indexes, usable recursively, in a subrequest's URL or body; case-sensitive | documented | c1 |
| 8 | Reference IDs must start with a letter or digit and contain only letters, digits and `_`; a bad one fails the whole request with HTTP 400 `JSON_PARSER_ERROR` (v52.0+) | documented | c3 |
| 9 | A reference to a field that is present but null resolves (v52.0+) | documented | c3 |
| 10 | A reference to a field the parent did not select fails the dependent subrequest with `PROCESSING_HALTED` (v52.0+) | documented | c3 |
| 11 | Subrequest `httpHeaders` may carry any header the target supports except `Accept`, `Authorization` and `Content-Type`; those fail the whole request with 400 | documented | c1 |
| 12 | Under `allOrNone: true`, a failure sets every other subrequest to 400 | documented, and observed for the pilot case | c3, pilot |
| 13 | Collections create, update and delete: up to 200 records, ordered results, `allOrNone` default false, outer 200 for a well-formed request with per-item `{id?, success, errors[{statusCode, message, fields}]}` | documented | c5, c8, c10 |
| 14 | Collections create and update accept mixed object types, processed as at most 10 consecutive same-type chunks | documented | c5, c8 |
| 15 | Collections `allOrNone: true` failure: the failing item keeps its error and every other item gets `ALL_OR_NONE_OPERATION_ROLLED_BACK` with `success: false` | documented | c5, c8, c9, c10 |
| 16 | Collections delete accepts IDs of different object types | documented | c10 |
| 17 | Collections upsert: one object type named in the URL, matched on the external ID field named in the URL, never on record ID; adds `created` to each successful result | documented | c9 |
| 18 | Collections retrieve: GET with `ids` and `fields` (about 800 IDs before the URL is too long), or POST with a body (up to 2,000 IDs); one object type; `null` in place of an unreadable or invalid ID; an invalid or unreadable field fails the request with 400 | documented | c6, c7 |
| 19 | Outer `allOrNone: true` overrides each nested Collections `allOrNone`; outer false leaves each Collections request to its own flag | documented | c4 |
| 20 | Exact error codes and messages for malformed Collections bodies (missing `attributes.type`, `id` on create, more than 200 records, more than 10 chunks) | gap: only "400 Bad Request" is documented | c5, c8 |
| 21 | Composite Graph and Composite Batch | out of scope for this plan's first pass ("if ready" in the roadmap) | roadmap |
| 22 | QueryAll subrequests | deferred to after Plan 30, which adds `queryAll` | brief |

## Candidate limits carried into the spec

- Rows-by-external-ID upsert exists only for `Account` keyed on
  `Compat_Key__c` (Plan 11). Composite and Collections upsert support exactly
  that pair. Other external ID fields return a clear unsupported error and are
  listed as a gap.
- Error messages never echo request values (project rule). Where the
  documentation's example messages include IDs or values, the candidate uses a
  static message with the same error code.
