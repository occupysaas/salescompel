# Plan 31 - Composite II and sObject Collections

Status: implemented, 2026-11-15; decisions below still await maintainer review.
Reviewed once by a separate model session; its findings are folded in.
[Implementation record](../../compat/plan-31-composite-collections.md#implementation-record-2026-11-15).
Issue: [#21](https://github.com/occupysaas/salescompel/issues/21). Brief: [plan-31](../../paths/plan-31-composite-collections.md).
Roadmap: [first release, Plan 31](2026-11-04-salescompel-first-release-roadmap-design.md#3-plans).
Reference record: [plan-31-composite-collections.md](../../compat/plan-31-composite-collections.md) (rows cited below as R1-R22).

## Intent

Plan 24 shipped one narrow Composite shape: explicit `allOrNone: true`,
Account/Contact POST, PATCH and DELETE, and `@{ref.id}` only in a record-ID
segment or `Contact.AccountId`. Real clients send more. This plan extends
Composite to partial success, reads, queries, upserts and general references,
and adds the sObject Collections resource that JSforce uses for array-valued
`create`, `update`, `upsert`, `destroy` and `retrieve`.

Composite Graph and Composite Batch are "if ready" in the roadmap. They are out
of this plan and recorded as gaps (R21). QueryAll subrequests wait for Plan 30
(R22).

## Global constraints

- Go 1.25.0, PostgreSQL 17, API v65.0, no new runtime dependency.
- One write path (roadmap Sec. 6): every write goes through `records.Tx`
  (`Create`, `Update`, `Delete`, `UpsertAccount`). No HTTP loopback, no
  recursive `NewHandler` call, no second connection.
- One request is one PostgreSQL transaction. Partial success uses pgx nested
  transactions (SAVEPOINTs), the mechanism `UpsertAccount` and `Store.begin`
  already use. `UpsertAccount`'s own nested `Begin` nests inside an outer
  savepoint without change.
- Existing standalone behavior stays byte-identical, except the one addition
  in "Standalone change".
- Plan 24 behavior stays: `httpHeaders` in every Composite result is `{}`
  (no `Location` header), and `candidate/composite/atomic` stays green. Its
  assertions that pinned rejections Plan 31 lifts were updated; the
  reference record lists each one.
- Error messages are static and never echo request values, IDs or SQL.
- Follow CLEAN_ROOM.md. Behavior beyond the documentation is a *candidate*
  decision and is named as one.

## Error classes

Every failure is one of three kinds. This applies to Composite subrequests
and Collections items alike.

| Kind | Examples | Effect |
| --- | --- | --- |
| Request error | Bad JSON, bad envelope, limits exceeded, bad reference syntax, forbidden header | Whole request fails before the transaction begins. Outer 400, standard error array `[{errorCode, message}]`. Nothing is written. |
| Item error | Any `APIError` with status below 500 from `records.Tx`, including mapped PostgreSQL constraint errors; unresolvable reference; unsupported route | That subrequest or item fails. Handling depends on `allOrNone`. |
| Storage error | `APIError` with status 500 or above (unmapped PostgreSQL errors become 500 in `records.dbError`), context cancellation, begin, savepoint, rollback-to-savepoint or commit failure | Whole request rolls back. Outer 500 `SERVER_UNAVAILABLE`, no success entries. |

## Composite (POST /services/data/v65.0/composite)

### Envelope

| Property | Rule |
| --- | --- |
| `allOrNone` | Optional boolean. Omitted means `false` (candidate default, R2). |
| `collateSubrequests` | Optional boolean. Subrequests always run in array order. Sequential order is one valid outcome of collation (R3), so `true` is accepted and has no other effect. |
| `compositeRequest` | Required array, 1-25 entries (R6). |

At most 5 entries may be query subrequests (`query?q=`, `query/{locator}`)
or Collections subrequests (R6). Record GETs do not count. A 6th is a request
error, `INVALID_COMPOSITE_REQUEST`.

Each subrequest has required `method`, `url`, `referenceId`, optional
`body`, and optional `httpHeaders`. `body` is required for POST and PATCH and
must be absent on GET and DELETE. `referenceId` keeps the Plan 24 pattern and
must be unique. A bad `referenceId` is a request error with code
`JSON_PARSER_ERROR` (R8; decision 2). `httpHeaders` may be omitted or `{}`.
Naming `Accept`, `Authorization` or `Content-Type` is a request error (R11).
Any other header is an item error, `INVALID_COMPOSITE_REQUEST` (candidate: no
subrequest header is supported yet; `If-Match` waits for Plan 30).

### Supported subrequests

All URLs start with `/services/data/v65.0/`. Any other version is an item
error, 404 `NOT_FOUND`.

| Method | URL after the version | Executes |
| --- | --- | --- |
| POST | `sobjects/{Object}` | `records.Tx.Create`; 201 `{id, success, errors}` |
| GET | `sobjects/{Object}/{id}` with optional `?fields=a,b` | `records.Tx.Read` with fields (Plan 33, D33-03); 200 record |
| PATCH | `sobjects/{Object}/{id}` | `records.Tx.Update`; 204 |
| DELETE | `sobjects/{Object}/{id}` | `records.Tx.Delete`; 204 |
| PATCH | `sobjects/Account/Compat_Key__c/{value}` | `records.Tx.UpsertAccount`; 201 created or 200 updated, body as the standalone route |
| GET | `query?q=...` | shared query function below; 200 |
| GET | `query/{locator}` | `records.Tx.QueryMore`; 200 |
| POST, PATCH, GET, DELETE | `composite/sobjects...` | Collections executor, inside the Composite transaction |

The query function is the one the standalone `/query` handler uses, moved out
of `http.go` so both call it, including the Organization and currency
discovery cases.

Objects are any object in the tenant catalog. Validation, field permissions
and tenant predicates are the existing ones. Upsert on any external ID field
other than `Account.Compat_Key__c` is an item error, 404 `NOT_FOUND`, the same
as the standalone route today (gap; decision 4). Any other route is an item
error, 404 `NOT_FOUND`. A supported path with a method it doesn't take is an
item error, 405 `METHOD_NOT_ALLOWED`, as the standalone route answers
(decision 6). `PUT`, or any method no supported route takes, is a request
error (decision 8). Objects hidden by Plan 33 permissions answer as unknown
ones, item 404 (D33-02).

### References

Syntax: `@{ref` then one or more steps then `}`. A step is `.name` (a JSON
object property, case-sensitive) or `[n]` (a zero-based array index). The
first step may be an index, for Collections results: `@{coll[0].id}`. Other
examples: `@{newAcct.id}`, `@{q.records[0].Id}`, `@{acct.BillingAddress.city}`.

Where references may appear (R7):

- **In `url`**, only as the whole of a record-ID segment, the whole of an
  external-ID value segment, or inside a query-parameter value. Anywhere else
  in a URL, including the `query/{locator}` segment, is a request error.
- **In `body`**, in any string value at any depth. A string that is exactly
  one reference takes the referenced value with its JSON type (string,
  number, boolean or null, R9). A reference inside a longer string is
  replaced by the scalar as text, `null` as the text `null` (decision 11); an
  object or array there, or as a whole value, is an item error.

Prevalidation (before the transaction) parses each URL with every reference
treated as an opaque token, checks the route shape against the table above,
and checks each reference names an earlier subrequest. Forward, self or
unknown references and malformed `@{` syntax are request errors,
`INVALID_COMPOSITE_REQUEST`.

At run time, the value is substituted into the segment or query value that
prevalidation already parsed; the URL is never parsed again, so a value
holding `/`, `?` or `..` stays data in that one position and cannot reach
another route (decision 7). A value in a URL position that is null, empty or
not a string is an item error, 400 `INVALID_COMPOSITE_REQUEST`, and nothing
runs (decision 9). A path that
does not exist in the earlier response, or a reference to a subrequest that
failed or did not run, is an item error, 400 `PROCESSING_HALTED`, static
message "Invalid reference specified" (R4, R10). This moves Plan 24's
pre-transaction rejection of `@{x.Id}` to this run-time error.

Substitution into `q` is raw text, as documented: a referenced value
containing a quote changes the SOQL. The query still runs as the caller with
the caller's field permissions and tenant predicates, so it cannot reach data
the caller could not query directly. A test pins that boundary.

### Execution

Begin one transaction. Run subrequests in order.

- `allOrNone: true`: as Plan 24. The first item error rolls back everything;
  the failing entry keeps its error and every other entry becomes 400
  `PROCESSING_HALTED`, message "Composite transaction rolled back" (R12).
- `allOrNone: false`: each subrequest runs in its own savepoint. On an item
  error, roll back to the savepoint, record the error and continue. Dependents
  of a failed or unrun subrequest get the reference error (R1). Commit at the
  end. Successes report their real bodies.

Outer status is 200 in both modes. A storage error at any point is a whole
request 500.

### Collections inside Composite

A Collections subrequest is decoded and prevalidated with the rest of the
Composite request; its request errors are Composite request errors. Checks
that depend on a reference (an `ids` or `fields` value, or a limit counted
from one) can only run after substitution, so they are item errors
(decision 13). Retrieve fields that hold no reference are checked against the
caller's describe before the transaction. Its
result is an entry with `httpStatusCode` 200 and the item array as `body`.

- Outer `allOrNone: true` wins (R19): the Collections executor runs with
  `allOrNone: true`, and any failed item makes the subrequest fail. Then the
  whole Composite rolls back; the Collections entry becomes 400 with its item
  array as `body`, failed item kept and other items
  `ALL_OR_NONE_OPERATION_ROLLED_BACK`. Every other entry is
  `PROCESSING_HALTED` as above.
- Outer `allOrNone: false`: the Collections subrequest runs inside its
  savepoint with its own flag. Item failures are reported inside its body and
  do not fail the subrequest.

## sObject Collections

All routes are under `/services/data/v65.0/composite/sobjects`, standalone
and as Composite subrequests.

| Method | Path and parameters | Body | Executes per item |
| --- | --- | --- | --- |
| POST | `composite/sobjects` | `{allOrNone?, records: [...]}` | `Create` |
| PATCH | `composite/sobjects` | `{allOrNone?, records: [...]}` | `Update` |
| PATCH | `composite/sobjects/Account/Compat_Key__c` | `{allOrNone?, records: [...]}` | `UpsertAccount` |
| DELETE | `composite/sobjects?ids=a,b&allOrNone=true` | none | `Delete` |
| GET | `composite/sobjects/{Object}?ids=a,b&fields=x,y` | none | SOQL read |
| POST | `composite/sobjects/{Object}` | `{ids: [...], fields: [...]}` | SOQL read |

### Request errors

All of these are request errors, outer 400, nothing written (codes are
candidate decisions, R20): more than 200 records or IDs on a write or delete;
more than 2,000 IDs on POST retrieve or 800 on GET retrieve (800 stands in for
the URL length limit, R18); more than 10 type chunks on create or update
(R14); a record without `attributes.type`; an `Id` on create; no `id` on
update; an upsert record whose type is not `Account`, or that carries an `Id`
(decision 10); missing `fields` on retrieve. Also, so that no input is
silently ignored (decision 14): an unknown or repeated query parameter, a
body key other than the documented ones, a non-string `ids` or `fields`
entry, and trailing data after the JSON body. All use
`INVALID_COMPOSITE_REQUEST` except a malformed body, which uses
`JSON_PARSER_ERROR`.

### Record body handling

Before calling `records.Tx`, the executor removes `attributes` from each
record. On update it takes `id` or `Id` (case-insensitive, exactly one) out of
the record and passes it as the ID. On upsert it takes `Compat_Key__c` out and
passes it as the key; a missing, null or non-string key is an item error,
`REQUIRED_FIELD_MISSING`. On delete, each ID is normalized with
`recordid.Normalize`, and its object is found from its key prefix through the
tenant catalog (a small lookup by prefix is added to the snapshot); an unknown
prefix or malformed ID is an item error, `MALFORMED_ID`.

Duplicates are not merged: two items with the same ID or key run in order, as
two operations. Upserts take their advisory locks in sorted key order before
any write, so two concurrent upsert batches cannot deadlock.

### Write results

Outer status 200 with an array in input order (R13). Success items are
`{id, success: true, errors: []}`; upsert adds `created` (R17). Failure items
are `{success: false, errors: [{statusCode, message, fields}]}`, built from
the item's `APIError` (`Code` becomes `statusCode`, `Fields` becomes `fields`,
always present).

- `allOrNone` false (default): each item runs in its own savepoint.
- `allOrNone` true: the executor opens one savepoint for the whole batch. At
  the first item error it rolls back to that savepoint. The failing item keeps
  its error; every other item becomes `success: false` with
  `ALL_OR_NONE_OPERATION_ROLLED_BACK` and the static message "Record rolled
  back because not all records were valid"; items already written keep their
  `id` (R15). Standalone, the request then commits an empty transaction,
  because `Store.within` commits whenever its callback returns nil.

### Retrieve results

Outer 200 with one entry per requested ID, in order (R18). Every ID is
normalized with `recordid.Normalize` first; a malformed ID or one of another
object becomes `null` and never reaches SOQL. Valid IDs are read in chunks of
at most 150, so each WHERE clause stays inside the SOQL 4,000-character limit
(`soql.maxWhereChars`, which binds before the 1,000-value `IN` limit), each
chunk with an explicit batch size large enough for one page. A missing or unreadable
record gives `null`. An unknown or unreadable field is a request error,
400 `INVALID_FIELD`.

Unmodified JSforce always sends `fields` on retrieve; without caller fields it
sends every described field (reference record, client request shapes).

## Standalone change

Superseded by Plan 33, which merged first: its decision D33-03 adds the same
`fields` parameter to `GET /services/data/v65.0/sobjects/{Object}/{id}`
through `records.Tx.Read(ctx, object, id, fields...)`. Plan 31 uses that path
unchanged. Composite read subrequests call it, so their bodies still equal the
standalone route's. Collections retrieve resolves fields the same way and adds
`Id`, which it needs to put each row back at its requested position
(decision 15). Under D33-03 an empty `fields` value reads every field, and
`Id` is returned only when requested.

`GET /services/data/v65.0/sobjects` keeps `maxBatchSize: 1`.

## Code layout

`internal/platform/http.go` is at 480 of its 500 lines, so new routing moves
out of it rather than into it. Every file stays under 500 lines.

| File | Responsibility |
| --- | --- |
| `internal/platform/http.go` | Delegates `sobjects/{obj}/{id}` GET, `/query` and `composite/...` to the files below; net shrink |
| `internal/platform/http_sobject_read.go` | Standalone record GET with `fields` |
| `internal/platform/query_handler.go` | Shared `/query` function, including discovery cases |
| `internal/platform/composite.go` | Wire types, Composite executor, savepoints, allOrNone modes |
| `internal/platform/composite_route.go` | Subrequest URL parsing, re-routing after substitution, dispatch |
| `internal/platform/composite_ref.go` | Reference parser, path resolution, URL and body substitution |
| `internal/platform/http_composite.go` | Strict envelope decoding and prevalidation |
| `internal/platform/collections.go` | Collections executor and result shaping |
| `internal/platform/http_collections.go` | Collections routes, decoding and limits |
| `internal/records/write.go`, `internal/metadata` | `SelectFields` for retrieve; snapshot lookup by key prefix |
| `api/candidate-openapi.yaml` | Composite operation updated; six Collections operations with `x-source` |
| `test/candidate-collections.test.ts` | JSforce and raw HTTP acceptance |

## Tests and acceptance

- Go unit tests: reference grammar (leading index, nested fields, null,
  missing, embedded, forbidden URL positions), URL re-routing after
  substitution, envelope validation, the 25/5 limits, the Collections limits.
- Go PostgreSQL tests: partial success with a failing middle subrequest and
  its dependents; all-or-none rollback across reads, upserts and Collections;
  recovery after a mapped constraint error inside a savepoint, and whole
  request 500 after an unmapped one; Collections partial and all-or-none for
  create, update, upsert and delete; retrieve across 1,000 and 2,000 IDs with
  nulls; sorted lock order for concurrent upserts; a referenced value that
  alters SOQL still cannot cross tenants or read a denied field; tenant
  isolation and restricted-user denial through each new path.
- HTTP tests for each route, method, version, body limit and `?fields=`.
- OpenAPI: update the Plan 24 operation-count guard (twenty plus six) and add
  an explicit schema exception for references in any body value.
- Acceptance, through unmodified JSforce 3.11.0: `create`, `update`,
  `upsert` (Account, `Compat_Key__c`), `destroy` and `retrieve` with arrays,
  each with `allOrNone` true and false; raw HTTP Composite with partial
  success, a query subrequest feeding a later reference, and a Collections
  subrequest. Each case asserts persisted state. Following Plan 24's fixture
  rules, every possible create is journaled as an intent before dispatch and
  every returned ID is recorded before any assertion, so partial-success rows
  are always cleaned up.
- Existing Composite acceptance (`candidate/composite/atomic`, the pilot
  rollback) stays green, with the assertion changes the reference record
  lists.

## Decisions needed

1. ~~Accept the `GET sobjects/{Object}/{id}?fields=` addition to the
   standalone route.~~ Settled by Plan 33's D33-03, which Plan 31 now uses.
2. A bad `referenceId` changes from `INVALID_COMPOSITE_REQUEST` to
   `JSON_PARSER_ERROR` to match R8. It only affects requests that already fail.
3. Omitted Composite `allOrNone` becomes `false` (R2). Plan 24 rejected an
   omitted flag, so no working client changes behavior.
4. Upsert stays limited to `Account.Compat_Key__c` until a plan adds general
   external ID fields.
5. Keep `httpHeaders: {}` on every Composite result rather than adding the
   documented `Location` header, so Plan 24's contract and acceptance hold.
   Adding it later is a small, separate change.

Made during implementation and review, also awaiting maintainer review:

6. A supported path with the wrong method inside Composite is item 405
   `METHOD_NOT_ALLOWED`, matching the standalone route, not item 404.
7. References are substituted into the already-parsed segment or query value,
   never re-parsed and re-routed. A value cannot change the route, so the
   spec's "route again and compare shapes" step is not needed.
8. `PUT`, or any method no supported route takes, fails the whole request
   instead of being an item 404.
9. A reference in a URL position that resolves to null, `""` or a non-string
   is item 400 `INVALID_COMPOSITE_REQUEST`, so it is never used as a literal
   (e.g. an upsert with key `null`).
10. An upsert record that carries `Id` is a request error; upsert matches on
    the external ID only.
11. An embedded reference to null renders as the text `null`, in body strings
    and in query values, `q` included.
12. Query subrequests accept `columns`, as the standalone query route does.
13. Collections checks that depend on a reference run after substitution and
    are item errors.
14. Standalone Collections rejects unknown or repeated parameters, extra body
    keys, non-string `ids`/`fields` and trailing JSON rather than ignoring them.
15. Collections retrieve returns `Id` on every row even when not requested.
16. A duplicate `referenceId` is `INVALID_COMPOSITE_REQUEST`; a malformed one is
    `JSON_PARSER_ERROR` (decision 2).
