# Plan 28 — Field types I Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Broaden field types from `id`/`string`/`reference` to the full first-release set with decimal arithmetic and graduated validation.

**Architecture:** Extend `Field` with type-specific metadata. A `typeMeta` table maps simple types to PG column types and coercion facts. A `coerceAndValidate` dispatcher routes simple types through the table and compound types through dedicated handlers. Validation coerces and checks constraints inside `Validate()`, mutating the data map in place.

**Tech Stack:** Go 1.25, PostgreSQL 17, pgx v5.

**Spec:** [Plan 28 design](../specs/2026-11-10-plan-28-field-types-design.md). Read both documents before execution.

## Global Constraints

- Preserve backward compatibility: existing `id`/`string`/`reference` wire names unchanged.
- Decimal arithmetic exact via `big.Rat` — no float64 for number/currency/percent.
- Graduated validation: structural always, format where unambiguous, picklist membership enforced, email/phone/URL content not enforced (deferred for evidence).
- `omitempty` on all new Field JSON tags — existing clients see no describe change.
- Follow CLEAN_ROOM.md.
- No new runtime dependencies.

## Review Focus

- Coercion must mutate the data map so downstream `columns()` picks up typed values (e.g., `bool` for boolean, canonical decimal string for numbers).
- The catalog loader type check (`catalog.go:36`) must accept every new active type or dynamic fields fail at load time.
- `CreateField` DDL must emit the correct PG column type from `typeMeta`.
- Auto-number fields must reject client-supplied values on create.
- Compound Name/Address describe entries must carry `compound` and `compoundFields` keys.

## File map

| File | Action | Responsibility |
|------|--------|---------------|
| `internal/metadata/model.go` | Modify | Extend `Field` struct |
| `internal/metadata/admin.go` | Modify | Extend `FieldInput`, wire metadata through create |
| `internal/metadata/catalog.go` | Modify | Accept new active types in loader |
| `internal/records/fieldtypes.go` | Create | Type table, coerceAndValidate, per-type validators |
| `internal/records/fieldtypes_test.go` | Create | Unit tests for coercion and validation |
| `internal/records/validation.go` | Modify | Wire coerceAndValidate into Validate(), update Describe() |
| `internal/records/write.go` | Modify | Compound read assembly if needed |
| `internal/platform/http_dynamic.go` | Modify | Accept new field metadata in admin REST |
| `internal/platform/http_fieldtypes_test.go` | Create | Integration CRUD tests per type |

---

### Task 1: Field struct and type table

**Files:** Modify `internal/metadata/model.go`, create `internal/records/fieldtypes.go` and `internal/records/fieldtypes_test.go`.

- [ ] **Step 1:** Add failing tests for type table coverage and Field defaults. Assert every active type has a `typeMeta` entry and that new Field fields default to zero values.
- [ ] **Step 2:** Run `go test ./internal/records/ -run "TestField|TestTypeMeta" -v`. Expected: FAIL.
- [ ] **Step 3:** Extend `Field` struct with `Precision int`, `Scale int`, `Unique bool`, `PicklistValues []string`, `AutoNumber bool`, `Compound string`, `CompoundFields []string` — all with `json:"...,omitempty"`. Create `fieldtypes.go` with `typeInfo{pgType string}` and `typeMeta` map covering all simple types (see spec type table).
- [ ] **Step 4:** Run tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat: extend Field struct and add field type table for Plan 28`.

### Task 2: Numeric coercion (double, currency, percent)

**Files:** Modify `internal/records/fieldtypes.go`, `internal/records/fieldtypes_test.go`.

- [ ] **Step 1:** Add failing tests: `coerceAndValidate` with `json.Number` and string inputs for double/currency/percent. Cases: valid values, nil on nillable, nil on non-nillable (error), scale overflow (error), precision overflow (error), non-numeric string (error), wrong type (error).
- [ ] **Step 2:** Run `go test ./internal/records/ -run "TestCoerceNumber" -v`. Expected: FAIL.
- [ ] **Step 3:** Implement `coerceAndValidate(f Field, raw any) (any, error)` dispatcher and `coerceNumber`. Use `big.Rat` for validity. Check `Scale` (decimal places in input string) and `Precision` (significant digits). Handle `json.Number` and `string` inputs. Return canonical decimal string. Add `fieldProblem` helper wrapping `contract.FieldProblem`. Add `checkNillable` at top of `coerceAndValidate` for all types.
- [ ] **Step 4:** Run tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat: add numeric type coercion with precision/scale validation`.

### Task 3: Date, datetime, boolean coercion

**Files:** Modify `internal/records/fieldtypes.go`, `internal/records/fieldtypes_test.go`.

- [ ] **Step 1:** Add failing tests for date (accept `YYYY-MM-DD`, reject invalid month/format/datetime-formatted input), datetime (accept RFC 3339, reject date-only/non-ISO/numeric), boolean (accept true/false/null, reject strings).
- [ ] **Step 2:** Run `go test ./internal/records/ -run "TestCoerceDate|TestCoerceDatetime|TestCoerceBoolean" -v`. Expected: FAIL.
- [ ] **Step 3:** Implement `coerceDate` (parse `2006-01-02`), `coerceDatetime` (parse `time.RFC3339`), `coerceBoolean` (type-assert `bool`).
- [ ] **Step 4:** Run tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat: add date, datetime, and boolean type coercion`.

### Task 4: Picklist, multi-select picklist, and string-format types

**Files:** Modify `internal/records/fieldtypes.go`, `internal/records/fieldtypes_test.go`.

- [ ] **Step 1:** Add failing tests for picklist (membership check against `PicklistValues`), multipicklist (semicolon-separated, each part checked, empty string OK), and string types email/phone/url (length + nillable + UTF-8 via shared `coerceString`).
- [ ] **Step 2:** Run `go test ./internal/records/ -run "TestCoercePicklist|TestCoerceMultiPicklist|TestCoerceString" -v`. Expected: FAIL.
- [ ] **Step 3:** Implement `coercePicklist`, `coerceMultiPicklist`, `coerceString` (shared by string/email/phone/url). The dispatcher routes `string`/`email`/`phone`/`url` to `coerceString`.
- [ ] **Step 4:** Run tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat: add picklist, multi-select, and string-format type validation`.

### Task 5: Reference and auto-number

**Files:** Modify `internal/records/fieldtypes.go`, `internal/records/fieldtypes_test.go`.

- [ ] **Step 1:** Add failing tests: `coerceReference` (valid 15/18-char ID, reject non-string and wrong length), `autonumber` (reject any non-nil client value).
- [ ] **Step 2:** Run tests. Expected: FAIL.
- [ ] **Step 3:** Implement `coerceReference` with `isValidRecordID` format check. Auto-number case in dispatcher returns `AUTO_NUMBER_FIELD` error for non-nil.
- [ ] **Step 4:** Run tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat: add reference ID validation and auto-number read-only enforcement`.

### Task 6: Compound types (Name, Address)

**Files:** Modify `internal/metadata/model.go`, `internal/metadata/admin.go`, `internal/records/write.go`.

- [ ] **Step 1:** Add failing test: compound Field carries `Compound` marker and `CompoundFields` array. Assert Name compound has subfields `["FirstName","LastName","Salutation"]`, Address has `["Street","City","State","PostalCode","Country"]`.
- [ ] **Step 2:** Run tests. Expected: FAIL (CompoundFields not yet populated).
- [ ] **Step 3:** In `admin.go`, when creating fields with compound type, populate `Compound` and `CompoundFields`. In `write.go` `Read()`, assemble compound `Name` from subfields if not stored as a column (check existing Contact schema first — `Name` may already be a concatenated column).
- [ ] **Step 4:** Run tests. Expected: PASS.
- [ ] **Step 5:** Commit `feat: add compound Name and Address type support`.

### Task 7: Wire coercion into Validate and Describe

**Files:** Modify `internal/records/validation.go`.

- [ ] **Step 1:** Add failing test: `Validate` mutates data map with coerced values (bool for boolean, canonical string for number).
- [ ] **Step 2:** Run tests. Expected: FAIL.
- [ ] **Step 3:** In `Validate()`, replace the string-only validation block with a loop that calls `coerceAndValidate(f, data[k])` and writes the result back to `data[k]`. Remove old length/nillable checks (now in coerceAndValidate). Keep permission and createable/updateable checks. `Describe()` serializes new Field fields automatically via `omitempty`.
- [ ] **Step 4:** Run `go test ./internal/records/ ./internal/platform/ -v`. Expected: PASS.
- [ ] **Step 5:** Commit `feat: wire coerceAndValidate into Validate and update describe output`.

### Task 8: Catalog loader and admin API

**Files:** Modify `internal/metadata/catalog.go`, `internal/metadata/admin.go`, `internal/platform/http_dynamic.go`.

- [ ] **Step 1:** Add failing test: `FieldInput` with new fields (Precision, Scale, PicklistValues, etc.) round-trips.
- [ ] **Step 2:** Run tests. Expected: FAIL.
- [ ] **Step 3:** Extend `FieldInput` with `Precision`, `Scale`, `Unique`, `PicklistValues`, `AutoNumber` (JSON tags). In `catalog.go`, replace the 3-type check with an `activeTypes` map covering all types. In `admin.go` `CreateField`, map new FieldInput fields to Field and emit PG column type from `typeMeta` in DDL. In `http_dynamic.go`, verify JSON decoding picks up new fields.
- [ ] **Step 4:** Run `go test ./internal/metadata/ ./internal/platform/ -v`. Expected: PASS.
- [ ] **Step 5:** Commit `feat: accept new field types in catalog loader and admin API`.

### Task 9: Integration test — CRUD per type through HTTP

**Files:** Create `internal/platform/http_fieldtypes_test.go`.

- [ ] **Step 1:** Write integration test that creates one custom field of each type on Account via the admin REST API, creates a record with valid values for each, reads back and verifies round-trip. Test error cases: wrong type, overlong string, invalid date, picklist violation, null on non-nillable, client-set auto-number.
- [ ] **Step 2:** Run `go test ./internal/platform/ -run "TestFieldTypes" -v`. Expected: FAIL (gaps in DDL, read path, or describe).
- [ ] **Step 3:** Fix gaps exposed by tests (column type mapping in DDL, boolean cast on read, describe metadata). Run full suite: `go test ./... -v`.
- [ ] **Step 4:** Commit `test: add field types CRUD and validation integration tests`.

### Task 10: Run gates

- [ ] **Step 1:** Run full gate suite (charset, encoding, corruption, manifest-join, pattern-ratchet, semantic-ratchet, silent-success, exec-bits, regression). Expected: all green, 0 new findings.
- [ ] **Step 2:** Fix any new findings. Commit if needed.
