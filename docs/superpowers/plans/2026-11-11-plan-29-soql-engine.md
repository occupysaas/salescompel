# Plan 29 — SOQL Engine Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** Draft for maintainer review, 2026-11-11.

**Goal:** Replace the bounded query grammar with one SOQL engine that parses, binds, secures, compiles and pages the spec's subset for REST, the connected editor and internal record reads.

**Architecture:** A new `internal/soql` package has five stages: lexer, parser to AST, binder against the tenant snapshot and field policy, compiler to parameterized SQL, and executor/shaper. Each stage has its own tests. Paging stores bound plans and ordered IDs in a tenant-scoped locator table. Child relationship names become stored metadata, surfaced in describe. `internal/query` and `platform.ParseQuery` become thin facades over the engine, then the old grammar is deleted.

**Tech Stack:** Go 1.25.0, PostgreSQL 17, pgx v5, Go native fuzzing, Vue 3 (unchanged queries), pinned JSforce 3.11.0, Salesforce CLI 2.156.0, simple-salesforce 1.12.9.

**Spec:** [Plan 29 design](../specs/2026-11-11-plan-29-soql-engine-design.md). Read both documents before execution; every *candidate* value comes from the spec.

## Global Constraints

- Go 1.25.0 and PostgreSQL 17.
- API v65.0; OpenAPI 3.2.1 with `x-source`.
- No new runtime dependency. Hand-written lexer and parser, no parser generator.
- Only validated catalog identifiers are interpolated (quoted with `pgx.Identifier`). Every value is a SQL parameter.
- Explicit tenant predicates on every joined table and subquery, with RLS as the backstop. Transaction-local tenant context.
- Exact decimals end to end: no float64 for field values. Dates and date-times use the Plan 28 REST forms.
- Queries that work today return byte-identical JSON. The 35 observed column cases stay green, except `unsupported-traversal` and `unsupported-aggregate`, which move to supported.
- Error codes: `MALFORMED_QUERY`, `INVALID_FIELD`, `INVALID_TYPE`, `INVALID_QUERY_FILTER_OPERATOR`, `QUERY_TOO_COMPLICATED`, `INVALID_QUERY_LOCATOR`. Messages never echo query values.
- Product copy uses the five catalogs. Follow CLEAN_ROOM.md, and record every *candidate* as observed, documented or gap.

## Review Focus

- **Exact page boundaries.** A result of exactly 2,000 rows returns `done: true` and no `nextRecordsUrl`. A result of 2,001 returns two pages. Task 8 `TestPagingBoundaries` pins both.
- **Rows deleted between pages.** They are skipped. Other rows never repeat or vanish, and `totalSize` keeps its first value. Task 8 `TestPagingDeletion`.
- **Locator after a catalog change.** If a selected field is denied or removed after page one, the next page returns `INVALID_QUERY_LOCATOR` and never leaks the field. Task 8 `TestLocatorCatalogChange`.
- **Date literals across boundaries.** `THIS_QUARTER` evaluated on 2026-12-31, `LAST_N_MONTHS:2` in January, and `TODAY` on 2028-02-29 all give the documented ranges. Task 4 `TestDateLiteralRanges`.
- **Null parents.** A path through a null parent at level 3 returns `null` at that level, not an error. Ordering by a parent field puts null parents first by default. Task 5 `TestParentPathNulls`.

## File and interface map

| Files | Responsibility |
| --- | --- |
| `internal/soql/{token.go,lexer.go}` | Tokens with offsets, escapes, literals, keywords |
| `internal/soql/{ast.go,parser.go}` | AST and recursive-descent parser, including `TYPEOF` nodes the binder refuses |
| `internal/soql/{bind.go,types.go,limits.go,dates.go}` | Name resolution, literal typing, field policy, structural limits, date literal and function evaluation |
| `internal/soql/{compile.go,sqlexpr.go}` | Bound AST to parameterized SQL (main, child and semi-join statements) |
| `internal/soql/{execute.go,shape.go,columns.go}` | Execution in the caller's transaction, REST JSON shaping, column metadata |
| `internal/soql/locator.go` + migration `0006_query_locators.sql` | Locator storage, page retrieval, expiry, eviction |
| `internal/metadata/{model.go,fieldspec.go,seed.go,admin.go}` + migration `0005_child_relationships.sql` | Child relationship names, describe `childRelationships` |
| `internal/records/{write.go,service.go,validation.go}` | Record reads through the engine; `Tx.Query`, `Tx.QueryMore`; describe child relationships |
| `internal/query/*`, `internal/platform/{query.go,http.go,scoped.go,runtime_privileges.go}` | Facade, then removal of the old grammar; HTTP query and `/query/{locator}` routes |
| `cmd/salescompel/schema.go` | `--child-relationship-name` |
| `api/candidate-openapi.yaml`, `compat/clients/node/src/candidate-openapi-*.ts` | Query, paging, aggregate and child shapes; locator route; errors |
| `compat/clients/node/src/candidate-soql.ts`, `scripts/candidate-soql-python.py` | Acceptance (inside the schema gate, following the Plan 28 pattern) |

Execute Tasks 1–10 in order. Migrations 0005 and 0006 are separate files and land in Tasks 3 and 8 respectively.

---

### Task 1: Reference review and compatibility record

**Files:** Create `docs/compat/plan-29-soql-engine.md`.

**Interfaces:** Produces a status table with one row per spec *candidate* (calls 1–11, and the `NOT IN` null rule, `INCLUDES` case and wrong-type error), each marked `observed`, `documented` or `gap`, with its source. Task 10 completes it.

- [ ] **Step 1: Review release notes since 2026-11-11** for SOQL and REST query changes. Attempt each spec Sources URL, and use search results when direct fetch is refused, as in Plan 28. Record pages that cannot be retrieved.
- [ ] **Step 2: If an authorized trial-org session exists,** observe each item listed in spec Sec. 10 Task 1. Otherwise record "no session". This step never blocks.
- [ ] **Step 3: If an observation contradicts a candidate,** update the spec and this plan before Task 2.
- [ ] **Step 4: Commit** with `docs: record Plan 29 reference review`.

### Task 2: Lexer, AST and parser

**Files:** Create `internal/soql/{token.go,lexer.go,ast.go,parser.go,lexer_test.go,parser_test.go,fuzz_test.go}`, `internal/soql/testdata/parse/*.soql` with matching `*.json` golden files.

**Interfaces (produces):**
- `type Pos struct{ Offset int }`. `type Token struct{ Kind TokenKind; Text string; Pos Pos }`. Kinds: `Ident`, `String`, `Number`, `Date`, `DateTime`, `DateLiteral` (with optional `:n`), `Op`, `LParen`, `RParen`, `Comma`, `Keyword`, `EOF`.
- `func Lex(text string) ([]Token, error)`. Rejects invalid UTF-8, NUL, unclosed strings and unknown escapes with `MALFORMED_QUERY`. Decodes the escapes in spec Sec. 1. Inside strings it keeps `\%` and `\_` as escaped wildcards (`Token.Text` stores them as `\%`/`\_` for LIKE).
- AST:
  - `type Query struct{ Select []SelectItem; From Name; Alias string; Where Expr; GroupBy []ValueExpr; Having Expr; OrderBy []OrderItem; Limit *int; Offset *int; With string; Pos Pos }`.
  - `SelectItem` is an interface implemented by `*FieldRef{Path []string}`, `*SubQuery{Query *Query}`, `*Aggregate{Func string; Arg *FieldRef; Alias string}`, `*Call{Func string; Args []ValueExpr; Alias string}` (`toLabel` and date functions), `*FieldsAll{Kind string}` and `*TypeOf{...}`.
  - `Expr` is implemented by `*And`, `*Or`, `*Not`, `*Compare{Left ValueExpr; Op string; Right Literal}`, `*In{Left ValueExpr; Not bool; Values []Literal; Sub *Query}`, `*Includes{Left *FieldRef; Exclude bool; Values []Literal}`.
  - `Literal{Kind LiteralKind; Text string; N int}`.
  - `OrderItem{Value ValueExpr; Desc bool; Nulls NullPlacement}`.
- `func Parse(text string) (*Query, error)`. Recursive descent. Keywords are case-insensitive. Mixed `AND`/`OR` at one level without parentheses gives `MALFORMED_QUERY`.

- [ ] **Step 1: Write failing tests.**
  - `TestLexEscapes`: `'O\'Brien'` → `O'Brien`; `'é'` → `é`; `'50\%'` keeps the escaped wildcard; `'bad\q'` and `'open` give MALFORMED_QUERY.
  - `TestParseGolden` walks `testdata/parse`. Each `.soql` file has a `.json` holding the expected AST or `{"error":"MALFORMED_QUERY"}`. The corpus includes:
    - every existing `internal/query` test query;
    - parent paths 1–6 levels deep, child subqueries and nested subqueries (parse OK; the binder rejects them later);
    - all aggregates with aliases, `COUNT()`, `toLabel`, `FIELDS(ALL)` and `TYPEOF`;
    - every WHERE operator, `NOT`, parentheses, and mixed `AND`/`OR` (error);
    - every date literal form, including `LAST_N_DAYS:30`;
    - GROUP BY/HAVING, multi-key ORDER BY with NULLS, LIMIT/OFFSET and `WITH SECURITY_ENFORCED`;
    - a trailing semicolon, a duplicate keyword and an unterminated subquery (errors).
  - `FuzzParse`: arbitrary input never panics, and either returns an AST or an `*contract.APIError` with code `MALFORMED_QUERY`.
- [ ] **Step 2: Run `GOTOOLCHAIN=go1.25.0 go test ./internal/soql -count=1`.** Expected: fail to compile.
- [ ] **Step 3: Implement the lexer, AST and parser.** No catalog access in this task. Golden files are written by `UPDATE_GOLDEN=1` and then reviewed by hand before commit.
- [ ] **Step 4: Run Step 2, then `go test ./internal/soql -run '^$' -fuzz FuzzParse -fuzztime 60s`.** Expected: pass, no crashers.
- [ ] **Step 5: Commit** with `feat: add SOQL lexer and parser`.

### Task 3: Child relationship metadata

**Files:** Create `internal/migration/migrations/0005_child_relationships.sql`. Modify `internal/metadata/{model.go,fieldspec.go,seed.go,admin.go,fieldspec_test.go}`, `internal/migration/{migrate.go,field_types_test.go}`, `internal/records/validation.go` (Describe), `cmd/salescompel/schema.go`, `cmd/salescompel/schema_test.go`, `internal/platform/testdata/describe-field-types.json`.

**Interfaces (produces):**
- `metadata.Options` gains `ChildRelationshipName string` (JSON `childRelationshipName`).
- `metadata.FieldInput` gains `ChildRelationshipName string`.
- `metadata.ChildRelationship{ChildSObject string "childSObject"; Field string "field"; RelationshipName string "relationshipName"; CascadeDelete bool "cascadeDelete"; RestrictedDelete bool "restrictedDelete"; DeprecatedAndHidden bool "deprecatedAndHidden"}`.
- `func (s Snapshot) ChildRelationships(object string) []ChildRelationship`.
- `func (s Snapshot) ChildRelationship(parent, name string) (Object, Field, error)`, which resolves case-insensitively.
- `metadata.Object` gains `ChildRelationships []ChildRelationship` (JSON `childRelationships`), filled by `records.Describe` using field policy.
- `func DefaultChildRelationshipName(child Object) string`: the plural label with non-alphanumeric characters removed, plus `__r` on custom children.
- CLI flag `--child-relationship-name`.

- [ ] **Step 1: Write failing tests.**
  - `TestChildRelationshipDefaults`: the child plural label "Typed items" gives `Typeditems__r`. An explicit name is kept. A duplicate name on the same parent (case-insensitive) errors. An invalid name (`"bad name"`) errors.
  - `TestDescribeChildRelationships`: Account describe lists `{childSObject: "Contact", field: "AccountId", relationshipName: "Contacts", restrictedDelete: true}`. A custom master-detail lists `cascadeDelete: true`. The restricted role does not see children through a denied field.
  - `TestMigrationChildRelationshipBackfill`: a populated Plan 28 database with two lookups to one parent from objects with the same plural label gets unique names (second one suffixed `2`), and every value and ID is unchanged.
  - The seed parity test covers the `Contacts` seed.
  - CLI parse test for the new flag.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/metadata ./internal/migration ./cmd/salescompel ./internal/platform -count=1`.** Expected: new tests fail.
- [ ] **Step 3: Implement.** Migration 0005 rewrites definitions per tenant with `set_config`, following the 0004 pattern, and appends manifest version 5. Regenerate the describe golden file and review the diff: only `childRelationships` may change.
- [ ] **Step 4: Run Step 2 plus the full `go test ./...`.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: name and describe child relationships`.

### Task 4: Binder, literal typing, limits and date literals

**Files:** Create `internal/soql/{bind.go,types.go,limits.go,dates.go,bind_test.go,dates_test.go}`.

**Interfaces:**
- Consumes: the Task 2 AST; the Task 3 snapshot relationship helpers; `policy.Field`, `fieldtype` validators and `metadata.Field.Kind()`.
- Produces:
  - `type Bound struct{ Object metadata.Object; Columns []BoundColumn; Children []BoundChild; Where BoundExpr; GroupBy []BoundValue; Having BoundExpr; Order []BoundOrder; Limit, Offset *int; Aggregate, CountOnly bool }`.
  - `BoundColumn{Key string; Path []PathStep; Field metadata.Field; Agg string; Label bool; Func string}`, where `PathStep{Object metadata.Object; Via metadata.Field}`.
  - `BoundValue` and `BoundExpr` carry typed parameters (`Param{Value any; Cast string}`), never text.
  - `type Clock func() time.Time`.
  - `func Bind(q *Query, snap metadata.Snapshot, p policy.Principal, now Clock) (*Bound, error)`.
  - `func DateLiteralRange(lit Literal, now time.Time) (start, end time.Time, err error)`. Returns UTC with a half-open end and Sunday-start weeks; fiscal equals calendar.

- [ ] **Step 1: Write failing tests.**
  - `TestBindNames`: case-insensitive object, field, relationship and child names resolve to canonical names. An unknown object gives INVALID_TYPE. An unknown field or relationship gives INVALID_FIELD. A 6-level path gives `QUERY_TOO_COMPLICATED`. A nested child subquery gives MALFORMED_QUERY.
  - `TestBindLiteralTypes`, one case per kind:
    - `Num__c = 5` OK; `Num__c = '5'` gives INVALID_FIELD with the spec message (no value echoed);
    - `Day__c = 2026-11-11` OK; `At__c = 2026-01-01T00:00:00` (no offset) gives MALFORMED_QUERY;
    - `Flag__c = true` OK;
    - `Id = 'not-an-id'` gives INVALID_QUERY_FILTER_OPERATOR;
    - `Multi__c INCLUDES ('A;B')` OK; `Txt__c INCLUDES ('A')` gives INVALID_QUERY_FILTER_OPERATOR;
    - `LIKE` on a number gives INVALID_QUERY_FILTER_OPERATOR.
  - `TestBindSecurity`: for the restricted principal, `Compat_Text__c` gives INVALID_FIELD in select, where, order, group, `COUNT()` argument, `Contact.Account.Compat_Text__c` and a child subquery select. `FIELDS(ALL)` silently leaves it out.
  - `TestBindLimits`:
    - 100,001-character query, 4,001-character WHERE clause, 21 subqueries, 3 semi-joins and a 1,001-element IN list each give `QUERY_TOO_COMPLICATED`;
    - `FIELDS(ALL)` without `LIMIT` ≤ 200 gives MALFORMED_QUERY;
    - `OFFSET 2001` gives MALFORMED_QUERY;
    - `WITH SYSTEM_MODE` gives MALFORMED_QUERY;
    - `TYPEOF` gives MALFORMED_QUERY naming the construct.
  - `TestBindAggregateRules`:
    - `SUM(Txt__c)` gives MALFORMED_QUERY;
    - an aggregate mixed with a plain field without GROUP BY gives MALFORMED_QUERY;
    - `GROUP BY At__c` (date-time) gives MALFORMED_QUERY, while `GROUP BY CALENDAR_YEAR(At__c)` is OK;
    - aggregate keys are `expr0`, `expr1`, or the alias.
  - `TestDateLiteralRanges` against a fixed clock:
    - `TODAY` on 2028-02-29 is [2028-02-29, 2028-03-01);
    - `THIS_QUARTER` on 2026-12-31 is [2026-10-01, 2027-01-01);
    - `LAST_N_MONTHS:2` on 2027-01-15 is [2026-11-01, 2027-01-01);
    - `THIS_WEEK` on Wednesday 2026-11-11 is [2026-11-08, 2026-11-15);
    - `LAST_N_DAYS:30` on 2026-11-11 is [2026-10-12, 2026-11-12) (documented as including today);
    - `N_DAYS_AGO:3` is the single day 2026-11-08;
    - `LAST_N_DAYS:1001` gives MALFORMED_QUERY.
- [ ] **Step 2: Run `GOTOOLCHAIN=go1.25.0 go test ./internal/soql -run 'Bind|DateLiteral' -count=1`.** Expected: fail.
- [ ] **Step 3: Implement.** Literals are typed with `fieldtype.Decimal` (JSON-number exponent rules), `ParseDate` and `ParseDateTime` (offset required). Comparison operators against date literals become range predicates: `=` means `[start,end)`, `<` means `< start`, `>` means `>= end`, as documented. Relative-period `N` bounds follow the documented limits; days are 1–1,000.
- [ ] **Step 4: Run Step 2.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: bind SOQL against the tenant catalog`.

### Task 5: Compiler and executor for record queries

**Files:** Create `internal/soql/{compile.go,sqlexpr.go,execute.go,shape.go,compile_test.go}`. Modify `internal/records/{service.go,write.go}` (Read through the engine; `Tx.Query`), `internal/query/{query.go,select.go}` (facade), `internal/platform/{query.go,scoped.go,http.go}`. Test: `internal/platform/soql_test.go`.

**Interfaces:**
- Consumes: `soql.Bind` and `Bound`.
- Produces:
  - `type Statement struct{ SQL string; Args []any }`.
  - `func Compile(b *Bound) (Statement, error)`.
  - `type Result struct{ TotalSize int; Done bool; Records []map[string]any; NextLocator string }`.
  - `type Options struct{ BatchSize int; Now Clock }`.
  - `func Run(ctx context.Context, tx pgx.Tx, exec policy.ExecutionContext, snap metadata.Snapshot, text string, opt Options) (Result, error)`.
  - `func RunAST(ctx, tx, exec, snap, q *Query, opt Options) (Result, error)` for programmatic queries (record reads).
  - `func (t *records.Tx) Query(ctx context.Context, text string, opt soql.Options) (soql.Result, error)`.
  - `Store.Query(ctx, p Principal, text string, batch int) (soql.Result, error)`.
  - The HTTP `GET /query` handler calls `Store.Query`. `Store.Select` and `query.Select` stay only as deprecated wrappers for existing tests until Task 9 removes them.

- [ ] **Step 1: Write failing tests.**
  - `TestCompileParameterized`: for a corpus of bound queries, a unique marker value placed in every literal never appears in `Statement.SQL`, and appears in `Args`.
  - `TestSOQLRecordQueries`, database-backed with the Plan 28 typed fixture extended by Contacts and Accounts:
    - every comparison operator on every filterable kind;
    - `LIKE 'ab\%%'`;
    - `IN`/`NOT IN` lists, where `NOT IN` returns null rows;
    - `INCLUDES ('A;B','C')` and `EXCLUDES`;
    - `AND`/`OR`/`NOT` precedence;
    - `= null` and `!= null`;
    - case-insensitive `Stage__c = 'closed'` and AutoNumber equality;
    - date literals against a fixed clock, through `Options.Now`;
    - multi-key ORDER BY with NULLS LAST, LIMIT and OFFSET.
  - `TestParentPathNulls`: the test adds a three-level custom chain (`Item__c.Parent__c` → `Parent__c.Group__c` → `Group__c.Region__c` → `Region__c`). `SELECT Id, Parent__r.Group__r.Region__r.Name FROM Item__c` returns `null` at the first null level, and ORDER BY `Parent__r.Name` puts null parents first. `SELECT LastName, Account.Name FROM Contact ORDER BY Account.Name` does the same on the standard path.
  - `TestSOQLExistingBehavior`: every query in the existing `foundation_query_test.go`, `query_test.go` and the 35-case column fixture (except the two that become supported) returns byte-identical JSON through `Store.Query`.
  - Cross-tenant: tenant B's parent never appears through a path, even when IDs are forged in a filter.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/soql ./internal/records ./internal/platform -count=1`.** Expected: new tests fail.
- [ ] **Step 3: Implement.**
  - Joins are `LEFT JOIN <table> pN ON pN.tenant_id = r.tenant_id AND pN.id = <prev>.<ref_col>`, with `pN.tenant_id = $1` in the WHERE clause.
  - Selected values use Plan 28's `selectExpression`/`wireValue`, moved to `internal/soql/shape.go` and imported back by `internal/query`.
  - Text comparisons use `lower(x COLLATE pg_c_utf8)`, LIKE uses `ILIKE $n ESCAPE '\'`, and INCLUDES uses `string_to_array(col,';')` with case-folded arrays.
  - At this task, results beyond one page return `QUERY_TOO_COMPLICATED` (paging lands in Task 8).
- [ ] **Step 4: Run Step 2 with `-race`.** Expected: pass; existing suites unchanged.
- [ ] **Step 5: Commit** with `feat: execute SOQL record queries through one engine`.

### Task 6: Child subqueries and semi-joins

**Files:** Modify `internal/soql/{bind.go,compile.go,execute.go,shape.go}`. Test: `internal/platform/soql_children_test.go`.

**Interfaces:**
- Consumes: Task 3 `Snapshot.ChildRelationship` and Task 5 `RunAST`.
- Produces:
  - `BoundChild{Key string; Relationship metadata.ChildRelationship; Query *Bound}`.
  - The child statement is `func CompileChild(c BoundChild, parentIDs []string) (Statement, error)`, using `ROW_NUMBER() OVER (PARTITION BY parent ORDER BY …)` for the per-parent LIMIT.

- [ ] **Step 1: Write failing tests.**
  - `TestChildSubqueries`:
    - `SELECT Name, (SELECT LastName FROM Contacts ORDER BY LastName LIMIT 2) FROM Account` returns `{totalSize, done: true, records}` per account, or `null` when there are none;
    - a custom `(SELECT Name FROM Typeditems__r WHERE Stage__c = 'Open')` works;
    - the child selects a parent path;
    - more than 2,000 children for one parent gives `QUERY_TOO_COMPLICATED` with the spec message;
    - child rows from another tenant never appear.
  - `TestSemiJoins`:
    - `SELECT Id FROM Account WHERE Id IN (SELECT AccountId FROM Contact WHERE LastName LIKE 'A%')`, and the `NOT IN` anti-join;
    - a semi-join under OR gives MALFORMED_QUERY;
    - a nested semi-join gives MALFORMED_QUERY;
    - a semi-join selecting a non-ID field gives MALFORMED_QUERY.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/soql ./internal/platform -run 'Child|Semi' -count=1`.** Expected: fail.
- [ ] **Step 3: Implement.** One child statement per subquery per page, bound to the page's parent IDs as a `text[]` parameter.
- [ ] **Step 4: Run Step 2 with `-race`.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: add child subqueries and semi-joins`.

### Task 7: Aggregates, grouping, toLabel, FIELDS() and date functions

**Files:** Modify `internal/soql/{bind.go,compile.go,shape.go,dates.go}`. Test: `internal/platform/soql_aggregate_test.go`.

**Interfaces:** Produces `AggregateResult` rows. `Result.TotalSize` equals the count, with empty `Records`, for `COUNT()`.

- [ ] **Step 1: Write failing tests.**
  - `TestAggregates`:
    - `SELECT COUNT() FROM Item__c` returns `{totalSize: n, done: true, records: []}`;
    - `SELECT Stage__c, COUNT(Id) total, SUM(Amount__c), AVG(Amount__c), MIN(Day__c), MAX(At__c) FROM Item__c GROUP BY Stage__c HAVING COUNT(Id) > 1 ORDER BY Stage__c` returns rows typed `AggregateResult` with keys `Stage__c`, `total`, `expr0`, `expr1`, `expr2`, `expr3`;
    - SUM over 0.10 + 0.20 returns exactly 0.3, and AVG keeps exact decimals;
    - `GROUP BY CALENDAR_MONTH(Day__c)` works;
    - `COUNT_DISTINCT`;
    - more than 2,000 groups gives `QUERY_TOO_COMPLICATED`.
  - `TestToLabelAndFields`:
    - `SELECT toLabel(Stage__c) FROM Item__c` returns labels;
    - `WHERE toLabel(Stage__c) = 'Closed label'` filters by label;
    - `FIELDS(STANDARD)` on a custom object selects `Id` and `Name`;
    - `FIELDS(ALL) … LIMIT 200` matches the describe order and leaves out denied fields for the restricted role.
  - `TestDateFunctions`: `WHERE CALENDAR_YEAR(At__c) = 2026`, `DAY_ONLY(At__c) = 2026-03-09` and `HOUR_IN_DAY(At__c) = 4`, evaluated in UTC.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/platform -run 'Aggregate|ToLabel|DateFunction' -count=1`.** Expected: fail.
- [ ] **Step 3: Implement.** Numeric aggregates are selected as `::text` and written with `fieldtype.DecimalJSON`. Date functions use `extract(... from x AT TIME ZONE 'UTC')`, and `DAY_ONLY` uses `(x AT TIME ZONE 'UTC')::date`.
- [ ] **Step 4: Run Step 2 with `-race`.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: add SOQL aggregates, grouping and field functions`.

### Task 8: Paging and query locators

**Files:** Create `internal/soql/locator.go`, `internal/migration/migrations/0006_query_locators.sql`. Modify `internal/migration/migrate.go`, `internal/platform/{http.go,scoped.go,runtime_privileges.go}`, `internal/records/service.go`. Test: `internal/platform/soql_paging_test.go`, `internal/migration/query_locators_test.go`.

**Interfaces:**
- Consumes: Tasks 5–7.
- Produces:
  - The table `sc_query.locators(tenant_id text, id text, user_id text, query_sha256 text, plan jsonb, ids text[], created_at timestamptz, expires_at timestamptz, PRIMARY KEY(tenant_id,id))`, with the closed RLS policy and records-role SELECT, INSERT, DELETE (added to `runtimeTablesSafe`).
  - `func NextPage(ctx, tx, exec, snap, locator string, offset int, batch int) (Result, error)`.
  - `Result.NextLocator` is formatted `<id>-<offset>`.
  - `Tx.QueryMore`.
  - `Store.QueryMore(ctx, p, locatorPath string, batch int)`.
  - Route `GET /services/data/v65.0/query/{locator}`.
  - Header parsing: `Sforce-Query-Options: batchSize=n`, clamped to 200–2,000.
  - Locator IDs are `01g` + 15 random base62 characters (*candidate* format following the documented locator shape).
  - The stored `plan` is the canonical re-serialized query text plus the catalog revision. It is re-parsed and re-bound per page, which is how "re-validated against the current catalog" is met.

- [ ] **Step 1: Write failing tests.**
  - `TestPagingBoundaries`: 2,000 rows give one page with `done: true` and no `nextRecordsUrl`. 2,001 rows give two pages. `LIMIT 4500` with batch 500 gives 9 pages, each row exactly once in order, and `totalSize` 4500 on every page.
  - `TestPagingDeletion`: delete 10 rows from page 3 after page 1. Page 3 omits them, nothing repeats, and `totalSize` is unchanged. Field updates made between pages appear on later pages.
  - `TestLocatorRules`:
    - replaying page 2 twice returns the same rows;
    - another user in the same tenant gets 400 `INVALID_QUERY_LOCATOR`;
    - another tenant gets 400 `INVALID_QUERY_LOCATOR`;
    - expired locators (clock advanced 2 days + 1s) give `INVALID_QUERY_LOCATOR`;
    - opening an 11th locator evicts the oldest;
    - more than 50,000 IDs gives `QUERY_TOO_COMPLICATED`;
    - aggregate, `COUNT()` and OFFSET queries create no locator;
    - a malformed locator path gives `INVALID_QUERY_LOCATOR`.
  - `TestLocatorCatalogChange`: deny or remove a selected field after page 1, and page 2 gives `INVALID_QUERY_LOCATOR`.
  - `TestQueryLocatorsRLS`: the records role without context sees zero locators, and cannot read another tenant's.
  - HTTP test: `batchSize=100` is clamped to 200, `batchSize=5000` to 2,000, and `nextRecordsUrl` has the documented shape.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/soql ./internal/platform ./internal/migration -run 'Paging|Locator' -count=1`.** Expected: fail.
- [ ] **Step 3: Implement.**
  - The first page runs an ID-only ordered statement for membership. It stores the IDs (when more than one page) and loads the page rows with `id = ANY($ids)`, preserving stored order with `array_position`.
  - Eviction and expired-row cleanup run in the same transaction as the insert.
  - Remove the old `QUERY_TOO_COMPLICATED` over-limit rejection.
- [ ] **Step 4: Run Step 2 with `-race`, plus full `go test ./...`.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: page SOQL results with query locators`.

### Task 9: Column metadata, contract, and removing the old grammar

**Files:** Create `internal/soql/columns.go`. Delete the bounded grammar in `internal/query/{query.go,normalize.go,select.go,columns.go}`, keeping only shims that callers still need, or deleting the package. Modify `internal/platform/{query.go,http.go,query_columns_test.go}`, `internal/platform/testdata/query-columns-contract.json`, `api/candidate-openapi.yaml`, `compat/clients/node/src/{candidate-openapi-contract.ts,candidate-openapi-traffic.ts}`, `compat/clients/node/test/candidate-openapi*.test.ts`, `docs/compat/sources.yaml`.

**Interfaces:** `func ColumnMetadata(b *Bound) []map[string]any`.
- Plain fields give `{columnName}`.
- Parent paths give nested `joinColumns`.
- Subqueries give `{columnName: rel, joinColumns: [...]}`.
- Aggregates give `{columnName: key, aggregate: true}`.

OpenAPI adds:
- `QueryResult` extended with `nextRecordsUrl` and `done: boolean`;
- an `AggregateResult` projection;
- nested parent objects at any depth, as recursive `$ref` (check that the traffic validator's cycle guard permits a named recursive schema; if not, bound to 5 levels explicitly);
- child relationship results;
- the operation `candidateQueryMore` for `GET /services/data/v65.0/query/{locator}`;
- `ChildRelationship` in `ObjectDescribe`;
- the error codes in Global Constraints.

`x-source` points to the Plan 29 spec at its committed SHA, registered in `docs/compat/sources.yaml`.

- [ ] **Step 1: Write failing tests.**
  - The column fixture gains the expected bodies for `unsupported-traversal` (now a parent path) and `unsupported-aggregate` (now `COUNT()`), with basis tags updated. All other 33 cases are unchanged.
  - Node traffic tests: one valid and one invalid exchange each for a parent path, child relationship, aggregate, `COUNT()`, a paged first page plus `queryMore`, describe with `childRelationships`, and each error code.
  - The contract inventory test includes `candidateQueryMore`.
- [ ] **Step 2: Run `npm --prefix compat/clients/node run check` and `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/platform -run 'Columns|OpenAPI' -count=1`.** Expected: fail.
- [ ] **Step 3: Implement.** Delete the old grammar once `grep -rn "query.Parse\|query.Select" internal` shows no callers.
- [ ] **Step 4: Run Step 2 and full `go test -race ./...`.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: document the SOQL engine contract and retire the bounded grammar`.

### Task 10: Acceptance and documentation

**Files:** Create `compat/clients/node/src/candidate-soql.ts`, `scripts/candidate-soql-python.py`. Modify `compat/clients/node/src/{candidate-schema.ts,candidate-acceptance-contract.ts,candidate-cli.ts or the existing CLI gate module}`, `compat/clients/node/test/candidate-acceptance-contract.test.ts`, `docs/compat/plan-29-soql-engine.md`, `README.md`, the first release roadmap, and the Status lines in the spec and this plan.

**Interfaces:**
- Required IDs, added to the `schema` gate following the Plan 28 pattern: `schema/soql-relationships`, `schema/soql-children`, `schema/soql-filters`, `schema/soql-aggregates`, `schema/soql-paging`, `schema/soql-python`.
- CLI gate gains `cli/query-relationship` and `cli/query-children`.
- Fixtures: 4,500 records in a P29-prefixed custom object, created through Composite batches of 25 (journaled as one intent per batch with deterministic names `P29 <run> <n>`). Twelve Accounts with Contacts. Dates computed from the run's start time.

- [ ] **Step 1: Write failing harness tests:** a missing or skipped SOQL ID makes the gate ineligible, and the restart required list is unchanged.
- [ ] **Step 2: Run `npm --prefix compat/clients/node run check`.** Expected: fail.
- [ ] **Step 3: Implement the gate clients.**
  - JSforce: `query(...).run({ autoFetch: true, maxFetch: 5000 })` with `Sforce-Query-Options: batchSize=500` receives 4,500 unique IDs. It also runs relationship, child, filter, date-literal and aggregate queries with exact expected values.
  - simple-salesforce: `query_all` receives 4,500 unique IDs, `query` handles a child subquery and `COUNT()`.
  - `sf data query --query "SELECT Name, Account.Name FROM Contact WHERE …"` and a child subquery, with `--json` output asserted.
  - Records in the P29 object are removed with its journaled table, following the Plan 28 ruling.
- [ ] **Step 4: Run the final checks** with the real restricted roles:

```bash
GOTOOLCHAIN=go1.25.0 go vet ./...
TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test -race ./... -count=1
GOTOOLCHAIN=go1.25.0 go test ./internal/soql -run '^$' -fuzz FuzzParse -fuzztime 120s
npm --prefix compat/clients/node run check
npm --prefix web test && npm --prefix web run build
CANDIDATE_DISPOSABLE=yes DATABASE_URL=<owner-url> scripts/check-candidate-acceptance.sh --out <exclusive-absolute-report-path>
```

Expected: every gate passes with all required IDs and zero required skips; cleanup is verified. Record the revision, report path and SHA-256.

- [ ] **Step 5: Finish documentation.**
  - In the compat record: complete the candidate table, list the gaps (child paging beyond 2,000, the 50,000-ID cap, UTC date literals, calendar fiscal periods, `TYPEOF`, `USING SCOPE`, `FOR` clauses, `ROLLUP`/`CUBE`, `FORMAT`/`convertCurrency`, richer column metadata), add query limits and paging to the operator notes, and add the evidence table.
  - Update the README and roadmap records.
  - Mark this plan and the spec implemented only after evidence passes.
- [ ] **Step 6: Commit** with `test: require SOQL engine acceptance and document Plan 29`. Run full acceptance again if documentation changes source bindings.

## Self-review

Spec coverage:
- Sec. 1 architecture → Tasks 2, 4, 5;
- Sec. 2 SELECT → Tasks 5 (fields, paths), 6 (children), 7 (aggregates, `toLabel`, `FIELDS`);
- Sec. 3 WHERE → Tasks 4 (typing, dates) and 5–6 (operators, semi-joins);
- Sec. 4 grouping, ordering and WITH → Tasks 4, 5, 7;
- Sec. 5 limits and errors → Task 4;
- Sec. 6 paging → Task 8;
- Sec. 7 child metadata → Task 3;
- Sec. 8 security → Tasks 4 (binder) and 5, 6, 8 (tenant predicates, locators);
- Sec. 9 compatibility and columns → Tasks 5 and 9;
- Sec. 10 evidence → Tasks 1 and 10.

All five Review Focus items have owning tests. Names used across tasks: `soql.Parse`, `Bind`, `Bound`, `Compile`, `CompileChild`, `Run`, `RunAST`, `NextPage`, `Result`, `Options`, `Clock`, `DateLiteralRange`, `ColumnMetadata`, `Snapshot.ChildRelationship(s)`, `DefaultChildRelationshipName`, `records.Tx.Query`/`QueryMore`, `Store.Query`/`QueryMore`.
