# Plan 7 — Declared Metadata Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans under the recorded native sequential preference. Steps use checkbox syntax.

**Goal:** Represent observed synthetic metadata and declared grants without inferring effective access.

**Architecture:** Pure Go value lookups in a separate metadata model. Existing catalog persistence and HTTP behavior remain unchanged.

**Tech Stack:** Go standard library; no dependencies or migrations.

**Spec:** [Plan 7 design](../specs/2026-10-18-plan-7-declared-metadata-design.md).

## Global constraints

- Preserve all existing Salesforce routes, runtime catalog, database schema and authorization behavior.
- Use exact synthetic names and values from Plan 6; missing is unknown, not false.
- Keep raw captures and credentials outside git; no live requests or client implementation inspection.
- Existing native branch execution preference applies; no push requested.
- No compatibility promotion or effective-security parity claim.

## Review focus

1. Unknown permission set/standard field must not fabricate a denial — lookup tests.
2. Known caseSensitive false must differ from absent caseSensitive — field tests.
3. Callers mutating returned values must not modify later lookups — value isolation test.
4. Declared grants must not change principal authorization — boundary regression.
5. Metadata model must not change stored catalog serialization/startup checks — boundary regression and unchanged runtime files.

## Task 1: Declaration model and independent boundaries

**Files:** Add internal/platform/metadata.go and metadata_test.go; update docs/compat/implementation-log.md and add docs/compat/plan-7-declared-metadata.md.

**Interfaces:** Use the exact function and type names/fields in the spec. OptionalBool is a value struct with Known/Value bool fields; no maps, pointers or slices in public values. Unknown lookups return zero value, false. CaseSensitive unknown is OptionalBool{}; observed false is OptionalBool{Known:true,Value:false}.

- [ ] Verify synthetic property values in both private reviewed Plan 6 inventories; publish only synthetic scope and source references.
- [ ] Write table-driven tests for both fields, grant lookups, unknown and case-variant names, value isolation, existing restricted Validate/Describe and unchanged serialized catalog before/after reads.
- [ ] Run go test ./internal/platform -run 'TestDeclared'; observe failure before implementation.
- [ ] Implement the minimal pure lookups with documented unknown/effective-access distinction.
- [ ] Run gofmt, go test ./..., go vet ./..., git diff --check. Report PostgreSQL test skips explicitly if no test database is available.
- [ ] Record boundaries and evidence in docs; commit implementation and verification.
- [ ] Request one fresh read-only final code review, fix Important/Critical issues with regression tests and repeat relevant checks. Record declined-to-judge rulings.

## Integration boundary

A separate future plan must address catalog migration, describe/Metadata API
integration, external-ID case behavior and effective permissions. This model is
not yet activated in any client-facing endpoint. Plan 5 remains independently gated.
