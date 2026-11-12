# Proposal: plan-29-soql-engine

## Why

Every Salesforce client, report and integration speaks SOQL. SalesCompel
today accepts one shape only: `SELECT fields FROM object [WHERE field = 'text']
[ORDER BY one field] [LIMIT ≤2000]`, plus one hard-coded `Contact.Account.Name`
join. Unmodified JSforce, Salesforce CLI and simple-salesforce fail on anything
beyond that. The maintainer's Plan 29 spec and plan (docs/superpowers, commits
`339b929` and `e8567c3`) define the replacement engine. This OpenSpec change
package locks that design as normative requirements before implementation.

## What Changes

- New `internal/soql` package: lexer → recursive-descent parser → AST → binder
  (tenant snapshot + field policy + structural limits) → compiler to
  parameterized PostgreSQL → executor/shaper (REST JSON).
- Child relationship names become stored metadata (migration 0005), surfaced
  in describe as `childRelationships`, with `DefaultChildRelationshipName`.
- Query locators (migration 0006): `sc_query.locators` stores ordered result
  IDs and a re-validated plan; `GET /services/data/v65.0/query/{locator}`
  pages results (`nextRecordsUrl`, `Sforce-Query-Options: batchSize`).
- `internal/query` and `platform.ParseQuery` become thin facades, then the old
  bounded grammar is deleted once no callers remain.
- OpenAPI (`api/candidate-openapi.yaml`) gains query/paging/aggregate/child
  shapes, `candidateQueryMore`, `ChildRelationship` in describe, and the
  Plan 29 error codes. Compat acceptance IDs extend the schema and CLI gates.

## Capabilities

### New Capabilities

- `soql-engine`: Parse, bind, secure, compile, execute and page the Plan 29
  SOQL subset for REST, the connected editor and internal record reads.

### Modified Capabilities

(none — `soql-engine` is new; prior query behavior is a subset that must stay
byte-identical for queries that work today)

## Product boundary

This adds the query language surface. Record writes, identity, multi-currency,
user time zones, TYPEOF, USING SCOPE, FOR clauses, ROLLUP/CUBE,
FORMAT/convertCurrency and Composite query subrequests stay out of scope.
Queries that work today return byte-identical JSON. Security behavior,
paging semantics and error codes are locked by the requirements below;
changing any of them requires a new change package.

## Impact

- Code: `internal/soql/*` (new), `internal/metadata/*`, `internal/records/*`,
  `internal/query/*`, `internal/platform/{query,http,scoped,runtime_privileges}.go`,
  `internal/migration/migrations/0005_*.sql` and `0006_*.sql`,
  `cmd/salescompel/schema.go`, `api/candidate-openapi.yaml`,
  `compat/clients/node/src/candidate-soql.ts`, `scripts/candidate-soql-python.py`.
- Narrative: `docs/compat/plan-29-soql-engine.md`, README, roadmap, the
  Superpowers spec/plan status lines.
- No new runtime dependency. Hand-written lexer and parser only.
- Verification on the ai03 self-hosted verify host (PostgreSQL 17).
