# Tasks: plan-29-soql-engine

Implementation order matches `docs/superpowers/plans/2026-11-11-plan-29-soql-engine.md`
Tasks 1–10. Each step is TDD (failing tests → implement → pass) and commits
with DCO sign-off.

## 1. Reference review record

- [x] 1.1 Create `docs/compat/plan-29-soql-engine.md`; review release notes since 2026-11-11 and each spec Sources URL (search when direct fetch is refused, as in Plan 28); record "no session" for trial-org observations
- [x] 1.2 Status table: one row per candidate (calls 1–11, NOT IN null rule, INCLUDES case, wrong-type error) marked observed/documented/gap
- [x] 1.3 Commit `docs: record Plan 29 reference review`

## 2. Lexer, AST and parser

- [x] 2.1 Failing tests: `TestLexEscapes`, `TestParseGolden` (testdata/parse corpus), `FuzzParse`
- [x] 2.2 Implement `internal/soql/{token,lexer,ast,parser}.go`; goldens via `UPDATE_GOLDEN=1` reviewed by hand
- [x] 2.3 `GOTOOLCHAIN=go1.25.0 go test ./internal/soql -count=1` and `go test ./internal/soql -run '^$' -fuzz FuzzParse -fuzztime 60s` pass
- [x] 2.4 Commit `feat: add SOQL lexer and parser`

## 3. Child relationship metadata

- [x] 3.1 Failing tests: `TestChildRelationshipDefaults`, `TestDescribeChildRelationships`, `TestMigrationChildRelationshipBackfill`, seed parity, CLI flag parse
- [x] 3.2 Migration 0005 + `Snapshot.ChildRelationship(s)` + `DefaultChildRelationshipName` + CLI `--child-relationship-name` + describe `childRelationships`
- [x] 3.3 Regenerate describe golden; only `childRelationships` may change
- [x] 3.4 Commit `feat: name and describe child relationships`

## 4. Binder, limits and date literals

- [x] 4.1 Failing tests: `TestBindNames`, `TestBindLiteralTypes`, `TestBindSecurity`, `TestBindLimits`, `TestBindAggregateRules`, `TestDateLiteralRanges`
- [x] 4.2 Implement `internal/soql/{bind,types,limits,dates}.go`; range-predicate semantics for date literals
- [x] 4.3 Commit `feat: bind SOQL against the tenant catalog`

## 5. Compiler and executor

- [x] 5.1 Failing tests: `TestCompileParameterized`, `TestSOQLRecordQueries`, `TestParentPathNulls`, `TestSOQLExistingBehavior`, cross-tenant
- [x] 5.2 Implement `internal/soql/{compile,sqlexpr,execute,shape}.go` + `records.Tx.Query` + `Store.Query` facade; tenant-prefixed LEFT JOINs
- [x] 5.3 Commit `feat: execute SOQL record queries through one engine`

## 6. Child subqueries and semi-joins

- [x] 6.1 Failing tests: `TestChildSubqueries`, `TestSemiJoins`
- [x] 6.2 `BoundChild` + `CompileChild` (`ROW_NUMBER() OVER (PARTITION BY parent …)`); semi/anti-joins
- [x] 6.3 Commit `feat: add child subqueries and semi-joins`

## 7. Aggregates, grouping and field functions

- [x] 7.1 Failing tests: `TestAggregates`, `TestToLabelAndFields`, `TestDateFunctions`
- [x] 7.2 AggregateResult rows; COUNT() totalSize with empty Records; toLabel; FIELDS(); date functions in UTC
- [x] 7.3 Commit `feat: add SOQL aggregates, grouping and field functions`

## 8. Paging and query locators

- [x] 8.1 Failing tests: `TestPagingBoundaries`, `TestPagingDeletion`, `TestLocatorRules`, `TestLocatorCatalogChange`, `TestQueryLocatorsRLS`, HTTP batchSize clamp
- [x] 8.2 Migration 0006 + `locator.go` + `NextPage` + `Store.QueryMore` + `GET /query/{locator}`
- [x] 8.3 Commit `feat: page SOQL results with query locators`

## 9. Column metadata and retire old grammar

- [ ] 9.1 Update 35-case column fixture (two cases become supported); OpenAPI shapes and `candidateQueryMore`; `x-source` in sources.yaml
- [ ] 9.2 Delete bounded grammar once `grep -rn "query.Parse\|query.Select" internal` shows no callers
- [ ] 9.3 Commit `feat: document the SOQL engine contract and retire the bounded grammar`

## 10. Acceptance and documentation

- [ ] 10.1 `candidate-soql.ts` / `candidate-soql-python.py` gate IDs; 4,500-record fixtures
- [ ] 10.2 Full verification on ai03: `go vet`, `go test -race ./...`, fuzz 120s, `npm run check`, web test+build, `scripts/check-candidate-acceptance.sh`
- [ ] 10.3 Complete compat record gaps and operator notes; README/roadmap; mark plan+spec implemented only after evidence
- [ ] 10.4 Commit `test: require SOQL engine acceptance and document Plan 29`

## Verification

- `openspec validate --all --strict` green on this package.
- Every requirement scenario in `specs/soql-engine/spec.md` has an owning test
  named in the Superpowers plan (Review Focus items included).
- Full acceptance passes with no required skips; report SHA-256 recorded.
- Queries that work today return byte-identical JSON (task 5 `TestSOQLExistingBehavior`).
