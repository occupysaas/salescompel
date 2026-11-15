# Proposal: plan-31-composite-collections

## Why

Plan 24 accepts only one Composite shape: explicit `allOrNone: true` with
Account/Contact POST, PATCH and DELETE. Unmodified clients send partial-success
Composite requests with reads, queries and upserts, and JSforce sends every
array-valued `create`, `update`, `upsert`, `destroy` and `retrieve` to sObject
Collections, which the candidate does not serve. The Plan 31 spec and plan
(`docs/superpowers/specs/2026-11-15-plan-31-composite-collections-design.md`,
`docs/superpowers/plans/2026-11-15-plan-31-composite-collections.md`) define the
extension. This change package locks it as requirements.

## What Changes

- Composite: optional `allOrNone` (default false), per-subrequest savepoints,
  record GET with `fields`, query and query-more, Account `Compat_Key__c`
  upsert, Collections subrequests, general `@{ref.path}` references with
  bounded URL positions, 25/5 limits, three error classes.
- sObject Collections: create, update, upsert, delete, GET and POST retrieve
  under `/services/data/v65.0/composite/sobjects`.
- Standalone record GET accepts `fields`.
- OpenAPI gains six operations; acceptance gains JSforce Collections and raw
  HTTP Composite cases.

## Capabilities

### New Capabilities

- `composite-collections`: Composite II and sObject Collections.

### Modified Capabilities

(none tracked in OpenSpec; Plan 24 predates OpenSpec packages. Its behavior is
preserved except spec decisions 2 and 3.)

## Product boundary

Composite Graph, Composite Batch, QueryAll subrequests, subrequest headers,
general external ID upsert and the `Location` result header stay out of
scope.

## Impact

- Code: `internal/platform/{http,http_sobject_read,query_handler,composite,
  composite_ref,composite_route,http_composite,collections,http_collections}.go`,
  `internal/records/write.go`, `internal/metadata`, `api/candidate-openapi.yaml`,
  `compat/clients/node/src/candidate-collections*.ts` and acceptance wiring.
- No migration and no new runtime dependency.
