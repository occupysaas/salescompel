# Plan 31 - Composite II and sObject Collections reference record

Status: implemented and verified locally, 2026-11-15; no Salesforce
compatibility certification. Nothing here claims Salesforce parity. Each row in the status table says
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

## Client request shapes (2026-11-15)

Unmodified JSforce 3.11.0 (the pinned compat client) was pointed at a local
logging HTTP server that recorded only method, path, query-parameter names and
JSON property names, never values or tokens. This is candidate-side,
black-box observation of the client, not of Salesforce. No client source was
read.

| Call (arrays of 2) | Request sent | Body or query |
| --- | --- | --- |
| `create(records, {allOrNone})` | `POST composite/sobjects` | `allOrNone`, `records[]` with `attributes` and fields |
| `update(records, {allOrNone})` | `PATCH composite/sobjects` | `allOrNone`, `records[]` with lowercase `id`, `attributes` and fields |
| `upsert(records, 'Compat_Key__c', {allOrNone})` | `PATCH composite/sobjects/Account/Compat_Key__c` | `allOrNone`, `records[]` with the key field in each record |
| `destroy(ids, {allOrNone: false})` | `DELETE composite/sobjects?ids=` | no `allOrNone` parameter |
| `destroy(ids, {allOrNone: true})` | `DELETE composite/sobjects?ids=&allOrNone=true` | |
| `retrieve(ids)` | `GET sobjects/Account/describe`, then `POST composite/sobjects/Account` | `ids`, `fields` (every described field) |
| `retrieve(ids, {fields})` | `POST composite/sobjects/Account` | `ids`, `fields` |
| `retrieve(id)`, `create(record)` | Standalone `sobjects` routes | unchanged |

Result: JSforce always sends `fields` on retrieve, so the spec's required
`fields` holds and its fallback is not needed. JSforce never uses the GET form
of retrieve; it is implemented from the documentation and tested by raw HTTP.

## Candidate limits carried into the spec

- Rows-by-external-ID upsert exists only for `Account` keyed on
  `Compat_Key__c` (Plan 11). Composite and Collections upsert support exactly
  that pair. Other external ID fields return a clear unsupported error and are
  listed as a gap.
- Error messages never echo request values (project rule). Where the
  documentation's example messages include IDs or values, the candidate uses a
  static message with the same error code.

## Implementation record (2026-11-15)

Delivered on branch `plan-31-composite-collections`
([spec](../superpowers/specs/2026-11-15-plan-31-composite-collections-design.md),
[plan](../superpowers/plans/2026-11-15-plan-31-composite-collections.md),
OpenSpec `openspec/changes/plan-31-composite-collections`).

- Composite: optional `allOrNone` (default `false`) with a savepoint per
  subrequest; record GET with `fields`, query, query-more, Account
  `Compat_Key__c` upsert and Collections subrequests; `@{ref.path}`
  references with `.name` and `[n]` steps; 25/5 limits; request, item and
  storage error classes.
- sObject Collections: create, update, upsert, delete, GET and POST retrieve.
- Standalone `GET /sobjects/{object}/{id}` accepts `fields`.
- OpenAPI: six Collections operations, the `fields` parameter on record reads,
  and the broadened Composite request and result schemas.

### Decisions made during implementation

| Decision | Why |
| --- | --- |
| References are substituted into an already-parsed URL (a segment or a query value), never re-parsed | A referenced value such as `x/../../query` stays data inside one segment. It reaches `normalizeID` as a malformed ID (item 404) or is stored as a literal external ID (`TestCompositeRefReroute`). |
| Retrieve reads in chunks of 150 IDs | SOQL limits a WHERE clause to 4,000 characters (`soql.maxWhereChars`), which binds before the 1,000-value `IN` limit the spec first named. |
| An embedded reference to a null renders as `null` | Nothing is documented for this case; whole-value references keep JSON null. |
| `PUT` subrequests are request errors | No supported resource takes `PUT`. |
| A well-formed URL to an unknown object, version or route is an item error, not a request error | Matches the spec's item 404; only malformed URL syntax is a request error. |
| Concurrent Collections upserts take advisory locks in sorted key order | Proven by `TestCollectionsUpsertLockOrder` (two batches with reversed key order, four rounds). |

### Plan 24 assertions that changed

The spec intended Plan 24 tests to stay untouched apart from decisions 2 and 3.
Plan 31 lifts more restrictions than that, so these assertions now pin the new
behavior. Every Plan 24 scenario still runs.

| Test | Old expectation | New expectation |
| --- | --- | --- |
| `TestDecodeComposite` | Omitted or `false` `allOrNone`, `collateSubrequests: true` and `httpHeaders: {}` rejected | Accepted; `null` values and a forbidden header still rejected |
| `TestValidateComposite` | Unsupported but well-formed URLs, `@{a.Id}`, embedded references and references to non-POST subrequests rejected before the transaction | Unsupported routes classified as item errors; well-formed references accepted and resolved at run time; malformed or forward references and references outside value positions still rejected |
| `TestCompositeAtomic/prevalidation` | GET on a create URL rejected before the transaction | Item 405 that rolls back; a malformed reference keeps the prevalidation case |
| `candidate/composite/atomic` | `request-schema` issues for an unknown Account field and an empty Contact body; `allOrNone: false` as the prevalidation case | Field checks happen on the server (still `INVALID_FIELD` and rollback); the prevalidation case uses `allOrNone: "false"` |
| OpenAPI traffic and contract tests | Plan 24 closed request schema and `const` flags | Plan 31 schemas; result bodies stay tied to status codes and `httpHeaders` stays the closed empty object |

### Client note

JSforce's `sobject(X).create(records)` sets every record's `attributes.type`
to `X`, so a mixed-type create cannot be sent through one `sobject` call. The
JSforce acceptance creates Contacts separately; mixed-type chunks are covered
by raw HTTP and Go tests.

### Verification (local, 2026-11-15)

Tools: Go 1.25.0, PostgreSQL 17.12 (disposable container), Node 24.19.0,
JSforce 3.11.0, OpenSSL 3.6.3.

| Check | Result |
| --- | --- |
| `go vet ./...` and `go test ./... -count=1` against PostgreSQL 17 | All packages passed |
| Plan 31 Go tests (Composite, references, Collections, fields) | All passed; a mutation that removed the per-subrequest savepoint made `TestCompositePartialRecovery` fail as intended |
| `npm run check` in `compat/clients/node` | Typecheck and formatting passed. Tests: 461 run, 107 failed. The same 107 fail on unmodified upstream `main` on this machine (pinned CLI, Python and browser prerequisites are not installed), and no other test fails |
| Live client acceptance against an HTTPS candidate with the limited record and identity logins, fresh run ID | `candidate/jsforce/core`, `/policy`, `/isolation`, `candidate/openapi/traffic`, `candidate/composite/atomic`, `candidate/composite/partial`, `candidate/collections/jsforce` and `candidate/foundation/decisions` all passed |
| Fixture journal cleanup after that run | `verified: true`, 232 records, 0 unknown intents |
| `sh scripts/check-file-size.sh`, `scripts/check-dco.sh` | Passed |

Not run locally: the full `scripts/check-candidate-acceptance.sh` runner, which
requires pinned tool versions (Salesforce CLI, Python, Chromium) not installed
on this machine. The pull request's CI runs it.

### Independent code review (2026-11-15)

A separate model session (Claude Fable 5.2) reviewed the code against the spec.
It reported no Critical findings and confirmed savepoint handling, tenant
isolation, field permissions, injection boundaries and unchanged standalone
routes. Its two Important and five Minor findings were reproduced with failing
tests (`composite_review_test.go`) and fixed:

| Finding | Fix |
| --- | --- |
| A null, empty or non-string reference in a URL value position was used as a literal (an upsert with key `null`) | Such a value is an item error, `INVALID_COMPOSITE_REQUEST`, and nothing runs |
| A trailing slash (`query/?q=`) failed the whole request | One trailing slash is ignored, as the standalone handler does. This also accepts `sobjects/Account/`, which Plan 24 rejected |
| A path segment that is not a name (`ui-api/...`) failed the whole request | It is an unsupported route: item 404 |
| Collections GET and DELETE limits inside Composite were checked only at run time | Literal values are prevalidated; values with references are checked after substitution |
| Standalone Collections took the first of repeated parameters and ignored trailing JSON | Both are request errors |
| `account` in an upsert URL was case-sensitive | Matched case-insensitively, as standalone routes are |
| An empty or non-string `referenceId` returned `INVALID_COMPOSITE_REQUEST` | Returns `JSON_PARSER_ERROR`, as decision 2 intends |

After the fixes, the full Go suite and the live client acceptance set (eight
tests, cleanup verified with 0 unknown intents) passed again.

### Gaps

- Composite Graph and Composite Batch (R21).
- QueryAll subrequests, until Plan 30 (R22).
- Subrequest `httpHeaders` other than none or `{}`; `If-Match` waits for
  Plan 30.
- Upsert on any external ID other than `Account.Compat_Key__c`.
- The documented `Location` result header (spec decision 5).
- Collection-level error codes and messages are candidate choices (R20).
