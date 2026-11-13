# Design: plan-29-soql-engine

## Context

The bounded query grammar in `internal/query` cannot express relationship
traversal, typed literals, aggregates, child subqueries, semi-joins or paging.
Plan 29 replaces it with one engine whose stages are independently testable.
The maintainer-authored Superpowers spec (`docs/superpowers/specs/2026-11-11-plan-29-soql-engine-design.md`)
is the narrative source; this design locks the decisions that the requirements
bind to. Superpowers plan tasks 1–10 are the implementation order.

## Goals / Non-Goals

**Goals:**

- One engine: `soql.Parse`/`Lex` → `Bind` → `Compile`/`CompileChild` →
  `Run`/`RunAST`/`NextPage` → REST JSON `Result`.
- Parameterized SQL only. Validated catalog identifiers quoted with
  `pgx.Identifier`; every value is a SQL parameter.
- Explicit tenant predicates on every joined table and subquery; RLS backstop.
- Exact decimals end to end (no float64 for field values).
- Byte-identical JSON for every query that works today (35 observed column
  cases stay green except `unsupported-traversal`/`unsupported-aggregate`).
- Locked error codes: `MALFORMED_QUERY`, `INVALID_FIELD`, `INVALID_TYPE`,
  `INVALID_QUERY_FILTER_OPERATOR`, `QUERY_TOO_COMPLICATED`,
  `INVALID_QUERY_LOCATOR`. Messages never echo query values.

**Non-Goals:**

- TYPEOF binding, USING SCOPE, FOR, ROLLUP/CUBE, FORMAT/convertCurrency,
  geolocation, nested child subqueries, independent paging of child results
  (over-2,000 child sets fail closed unless bounded by a subquery `LIMIT` of
  2,000 or less; child results have no `nextRecordsUrl`),
  user time zones / fiscal calendars other than calendar, multi-currency,
  Composite query subrequests (Plan 31), Bulk query (Plan 32).

## Decisions

1. **Hand-written lexer and parser** in `internal/soql`, no parser generator
   or new dependency (*candidate* call 1).
2. **Child cap is fail-closed**: more than 2,000 child rows for one parent
   gives `QUERY_TOO_COMPLICATED`; nothing is truncated (*candidate* call 2).
3. **Locators store ordered IDs** (max 50,000) and a re-validated plan
   (canonical query text + catalog revision). Membership is fixed at the first
   page; deletions are skipped; `totalSize` is frozen (*candidate* call 3).
4. **Batch size clamps** to 200–2,000, never rejects (*candidate* call 4).
5. **Date literals and date functions are UTC** with Sunday-start weeks and
   fiscal = calendar until Plan 33 (*candidate* call 5).
6. **`NOT IN` on nullable fields includes nulls** via
   `(col IS NULL OR col NOT IN …)` (*candidate* call 6).
7. **Default child relationship name** = plural label minus non-alphanumerics,
   plus `__r` on custom; backfill migration 0005 deduplicates with a numeric
   suffix (*candidate* call 7).
8. **`FIELDS()` drops denied fields** rather than failing (*candidate* call 8).
9. **`WITH SECURITY_ENFORCED`/`USER_MODE` accepted**; `SYSTEM_MODE` refused
   (*candidate* call 9).
10. **Wrong-type literal → `INVALID_FIELD`** with the documented wording;
    messages never echo query values (*candidate* call 10).
11. **`IN` lists cap at 1,000** (*candidate* call 11).

## Risks / Trade-offs

- The 50,000-ID locator cap and the 2,000-child cap are deliberate gaps
  versus Salesforce; they are recorded in the compat record, not hidden.
- UTC date literals differ from documented per-user time zones until Plan 33.
- Re-parsing and re-binding every page is slower than replaying stored SQL,
  and is what makes catalog-change invalidation safe.
- Deleting the old grammar (task 9) is gated on `grep` showing no callers;
  shims stay only where existing tests still need them until then.
