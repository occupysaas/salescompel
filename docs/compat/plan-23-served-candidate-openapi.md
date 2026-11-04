# Plan 23 — Served candidate OpenAPI contract

Status: implementation in progress, 2026-11-04. No acceptance/parity decision yet.

[Design](../superpowers/specs/2026-11-04-plan-23-served-candidate-openapi-design.md)
and [implementation plan](../superpowers/plans/2026-11-04-plan-23-served-candidate-openapi.md).

## Source ledger

Public OpenAPI 3.2.1 paths/security/media/reference/schema sections, Ajv JSON Schema
2020-12 version guidance and Go 1.25.0 embed documentation were re-read on
2026-11-04. URLs and read boundaries are appended to sources.yaml. Candidate HTTP,
catalog, select and column metadata source were reviewed locally at base 272883b;
the public repository locators were not fetched as an independent oracle. No
Salesforce observations, examples or third-party implementation were copied.

The canonical YAML was independently authored; its bytes are the single checked-in
source. There is no generated JSON twin or regeneration dependency. Existing
OpenAPI 3.2 minor tooling accepted the minimal 3.2.1 planning probe and the full
candidate document. Request business rules and authorization remain server checks.

## Implementation verification so far

The authenticated route and immutable embedded bytes pass two real Go tests against
an owned disposable PostgreSQL 17 database, with zero skips. Nine offline document
and traffic tests pass, along with typecheck. The acceptance-wiring tests pass;
the new candidate/openapi/traffic parent also passes against a real owned HTTPS
candidate and asserts exact coverage of all nineteen fixed operationIds. This
focused development run reused the existing preflight/service/fixture helpers.

The offline lab check passed: 422 test/subtest events, 418 passed, four opt-in
candidate tests skipped, zero failed. Typecheck, formatting and offline validation
passed, including eleven reviewed historical pilot observations/not_run=0. These
are not new Salesforce observations or parity certification. Full candidate
acceptance and independent review remain pending.

Implementation ruling: existing acceptance source roots excluded api/. Add api/
so the newly embedded YAML and Go code affect the source digest and untracked API
code prevents clean acceptance. A regression failed before the fix and passed
afterward, checking both byte changes and untracked inputs. This adds only the
preflight module/test to the planned file boundary. All 389 other protected
pre-existing file hashes match; original source registry content is unchanged
and new entries are append-only.

## Review correction checkpoint

The fresh review found one Important malformed-media issue and one constructor
subset guard gap graded Minor. The executor treated the latter as a required
subset-boundary correction because the constructor explicitly promises to reject
recursive/unsupported schemas, independently of the canonical-document guard.
Both reproductions failed before correction and now pass: full media parameter
grammar is checked before essence normalization; additionalProperties must be
false in the supported subset. The corrected focused suite has 35 passes, no
skips, and typecheck passes. No second reviewer substitutes for these regressions.

The first full acceptance attempt passed the database/race/vet stage and stopped
in the lab at the existing metadata-discovery test “read process deadline and
response bound fail closed.” Its isolated rerun passed, and that historical test
and implementation remain unchanged. This attempt produced no public summary:
the report schema still enumerated only the three legacy offline skip names.
A new real publication regression failed with RESULT, then passed after adding
exactly candidate/openapi/traffic to the allowedSkipped enum; unknown names still
reject. This adds the report schema to the explicit modified-file boundary.
Private first-attempt diagnostics are retained; full acceptance on the corrected
clean revision is still pending.
