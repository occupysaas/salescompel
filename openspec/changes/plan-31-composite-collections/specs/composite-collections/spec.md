# Spec: composite-collections

## Purpose

Run partial-success Composite requests with reads, queries, upserts,
references and nested Collections, and serve the six sObject Collections
operations, through the one record write path and one transaction per request.

## ADDED Requirements

### Requirement: Error classes
<!-- id: cmp-01 -->
Every Composite or Collections failure SHALL be a request error, an item error
or a storage error. A request error SHALL return outer HTTP 400 with the
standard error array before any transaction begins. An item error SHALL be an
`APIError` with status below 500 from the record layer, an unresolvable
reference or an unsupported route, and SHALL affect only its subrequest or item
subject to `allOrNone`. A storage error SHALL roll back the whole request and
return outer HTTP 500 `SERVER_UNAVAILABLE` with no success entries.

#### Scenario: mapped constraint error under partial success
- **WHEN** an `allOrNone: false` Composite request has a middle subrequest that violates a mapped constraint
- **THEN** that entry reports the error, later independent subrequests succeed and commit

#### Scenario: unmapped storage error
- **WHEN** any subrequest raises an unmapped PostgreSQL error
- **THEN** the outer response is 500 and nothing persists

### Requirement: Composite partial success
<!-- id: cmp-02 -->
Omitted `allOrNone` SHALL mean `false`. With `false`, each subrequest SHALL run
in its own savepoint; an item error SHALL roll back only that savepoint; a
subrequest that references a failed or unrun subrequest SHALL NOT run and SHALL
report 400 `PROCESSING_HALTED`. With `true`, behavior SHALL match Plan 24. Outer
status SHALL be 200 in both modes, and every result's `httpHeaders` SHALL be
`{}`.

#### Scenario: dependent of a failed subrequest
- **WHEN** subrequest B references failed subrequest A under `allOrNone: false`
- **THEN** B reports 400 `PROCESSING_HALTED` and is not executed

### Requirement: Composite subrequests and limits
<!-- id: cmp-03 -->
Composite SHALL accept record create, read with optional `fields`, update,
delete, `Account.Compat_Key__c` upsert, query, query-more and Collections
subrequests under `/services/data/v65.0/`, at most 25 in total and at most 5
query or Collections subrequests. A 26th subrequest or 6th query/Collections
subrequest SHALL be a request error. Other routes, versions and external ID
fields SHALL be item errors with 404 `NOT_FOUND`. Read, query and upsert
bodies SHALL equal the standalone route bodies.

#### Scenario: sixth query
- **WHEN** a request contains six query subrequests
- **THEN** the request fails with 400 `INVALID_COMPOSITE_REQUEST` and nothing runs

### Requirement: References
<!-- id: cmp-04 -->
`@{ref.path}` SHALL support `.name` and `[n]` steps, including a leading index.
In `url` it SHALL be allowed only as a whole record-ID segment, a whole
external-ID value segment, or inside a query-parameter value. In `body` it SHALL
be allowed in any string value; a whole-value reference SHALL keep the
referenced JSON type. Forward, self, unknown or malformed references SHALL be
request errors. After substitution, the URL SHALL be routed again and SHALL
fail as an item error if its route shape changed.

#### Scenario: value that would reroute
- **WHEN** a referenced value contains `/`, `?` or `..` and is used in a record-ID segment
- **THEN** the subrequest fails without reaching another route

#### Scenario: substituted query stays scoped
- **WHEN** a referenced value alters the SOQL text of a query subrequest
- **THEN** the query still returns only rows and fields the caller can read in its own tenant

### Requirement: Collections writes
<!-- id: col-01 -->
`POST`, `PATCH` and `DELETE /services/data/v65.0/composite/sobjects` and
`PATCH /services/data/v65.0/composite/sobjects/Account/Compat_Key__c` SHALL
create, update, delete and upsert up to 200 records through `records.Tx`,
returning outer 200 and one result per input in order. Failure items SHALL be
`{success: false, errors: [{statusCode, message, fields}]}`. With
`allOrNone: true`, the first item error SHALL roll back the batch, and every
other item SHALL report `ALL_OR_NONE_OPERATION_ROLLED_BACK`. Upserts SHALL take
their key locks in sorted order.

#### Scenario: all-or-none rollback
- **WHEN** the third of five creates fails with `allOrNone: true`
- **THEN** no record persists, the first two items keep their `id` with `success: false`

#### Scenario: concurrent upserts
- **WHEN** two upsert batches with the same keys in opposite order run concurrently
- **THEN** both complete without a deadlock error

### Requirement: Collections retrieve
<!-- id: col-02 -->
`GET /services/data/v65.0/composite/sobjects/{Object}?ids=&fields=` (at most
800 IDs) and `POST` with `{ids, fields}` (at most 2,000 IDs) SHALL return one
entry per requested ID in order, `null` for malformed, wrong-object, missing
or unreadable records, reading valid IDs in chunks of at most 1,000. An
unknown or unreadable field SHALL be a request error with `INVALID_FIELD`.

#### Scenario: 2,000 IDs
- **WHEN** a POST retrieve asks for 2,000 IDs
- **THEN** 2,000 entries return in request order

### Requirement: Collections inside Composite
<!-- id: col-03 -->
Outer Composite `allOrNone: true` SHALL override each Collections subrequest's
flag; any failed item SHALL fail the subrequest and roll back the Composite
request. With outer `false`, each Collections subrequest SHALL follow its own
flag inside its savepoint, and item failures SHALL NOT fail the subrequest.

#### Scenario: outer true, inner false
- **WHEN** an outer all-or-none request contains a Collections create with one failing item
- **THEN** nothing persists and every other Composite entry is `PROCESSING_HALTED`

### Requirement: Standalone record fields
<!-- id: rec-31 -->
`GET /services/data/v65.0/sobjects/{Object}/{id}` SHALL accept an optional
`fields` parameter. Without it, output SHALL be byte-identical to before.

#### Scenario: unknown field
- **WHEN** `fields` names a field that does not exist or is not readable
- **THEN** the response is 400 `INVALID_FIELD`
