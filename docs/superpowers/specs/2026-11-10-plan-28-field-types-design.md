# Plan 28 — Field types I

> **Superseded (2026-11-11):** This draft design predates upstream's completed
> Plan 28. The implemented design lives in
> [2026-11-11-plan-28-field-types-design](2026-11-11-plan-28-field-types-design.md),
> with the execution record in
> [docs/compat/plan-28-field-types.md](../../compat/plan-28-field-types.md).
> Kept for history only.

**Status:** Draft for review — superseded, see banner
**Date:** 2026-11-10
**Depends on:** Plan 27 (dynamic schema and tenant isolation)
**Roadmap:** First release roadmap, Stage B, item 28

## Summary

Broaden the platform's field type support from the current three active types
(`id`, `string`, `reference`) to the full first-release set: text, number,
currency (single currency), percent, date, datetime, checkbox, picklist,
multi-select picklist, email, phone, URL, lookup, master-detail, auto-number,
and compound Name and Address. Decimal arithmetic throughout. Formula, rollup,
encrypted and geolocation fields are stored, not enforced.

## Goals

1. Every field type above is creatable through the metadata admin API and
   usable in record CRUD.
2. Describe output carries type-appropriate metadata (precision, scale,
   picklist values, compound subfields) so clients can render correct editors.
3. Validation enforces structural correctness and format constraints where
   the format is unambiguous. Business rules (picklist membership, email RFC
   format) follow evidence from reference captures.
4. Decimal arithmetic is exact for number, currency and percent.
5. All types are exercised end-to-end through two independent clients
   (JSforce and simple-salesforce, or sf CLI and raw HTTP).

## Non-goals

- Formula, rollup, encrypted, geolocation and rich text field behavior
  beyond storage and describe.
- Multi-currency.
- Dependent picklists, controlling fields.
- Default value enforcement on create (stored in metadata, not applied).
- Unique constraint enforcement at the database level (declared in metadata,
  enforced later).
- Relationship queries (SOQL engine is Plan 29).
- Field-level security (Plan 33).

## Current state

The `Field` struct in `internal/metadata/model.go` has `Type` as a free
string. The catalog loader in `internal/metadata/catalog.go` rejects any
type other than `id`, `string`, `reference`. Validation in
`internal/records/validation.go` handles only string values with length and
nillable checks. Describe output omits precision, scale, picklist values and
compound structure.

## Design

### Data model

Extend `Field` in `internal/metadata/model.go`:

```go
type Field struct {
    Name        string   `json:"name"`
    Label       string   `json:"label"`
    Type        string   `json:"type"`
    Nillable    bool     `json:"nillable"`
    Createable  bool     `json:"createable"`
    Updateable  bool     `json:"updateable"`
    ExternalID  bool     `json:"externalId"`
    Length      int      `json:"length"`
    ReferenceTo []string `json:"referenceTo"`

    Precision      int      `json:"precision,omitempty"`
    Scale          int      `json:"scale,omitempty"`
    Unique         bool     `json:"unique,omitempty"`
    PicklistValues []string `json:"picklistValues,omitempty"`
    AutoNumber     bool     `json:"autoNumber,omitempty"`
    Compound       string   `json:"compound,omitempty"`

    Column string `json:"-"`
    ID     string `json:"-"`
}
```

New fields use `omitempty` so existing clients see no change for types that
do not set them. The wire type names map to Salesforce describe type names:
`string` (text), `double`/`currency`/`percent`, `date`, `datetime`,
`boolean`, `picklist`, `multipicklist`, `email`, `phone`, `url`,
`reference` (lookup/master-detail), `autonumber`, and compound types
`Name`/`Address` declared via the `Compound` field.

### Type table

A `typeMeta` table in `internal/records/validation.go` maps each simple type
to its storage column type and coercion facts:

| Wire type   | PG column              | Coerce input        | Strict checks                  |
|-------------|------------------------|---------------------|--------------------------------|
| `string`    | `TEXT`                 | string              | length, nillable, UTF-8        |
| `double`    | `NUMERIC(p,s)`         | number or string    | precision/scale range, overflow|
| `currency`  | `NUMERIC(p,s)`         | number or string    | same as double                 |
| `percent`   | `NUMERIC(p,s)`         | number or string    | same as double                 |
| `date`      | `DATE`                 | string              | ISO 8601 `YYYY-MM-DD`          |
| `datetime`  | `TIMESTAMPTZ`          | string              | ISO 8601 with timezone         |
| `boolean`   | `BOOLEAN`              | bool                | must be true/false/null        |
| `email`     | `TEXT`                 | string              | length, nillable               |
| `phone`     | `TEXT`                 | string              | length, nillable               |
| `url`       | `TEXT`                 | string              | length, nillable               |
| `autonumber`| `TEXT`                 | server-generated    | read-only on create/update     |
| `picklist`  | `TEXT`                 | string              | membership in PicklistValues   |

Compound and relationship types route to dedicated handlers:

| Wire type        | Handler                | Storage                     |
|------------------|------------------------|-----------------------------|
| `multipicklist`  | `validateMultiPicklist`| `TEXT` (semicolon-joined)    |
| `reference`      | existing path          | `TEXT` (record ID)           |
| `Name` compound  | `validateCompoundName` | per-subfield columns         |
| `Address` compound| `validateCompoundAddress`| per-subfield columns       |

Existing `id`, `string`, `reference` wire names are unchanged for backward
compatibility.

### Validation and coercion

Two-phase write path:

1. **Coerce** — raw JSON value to canonical storage value. Type mismatch
   (`string` to number, object to text) returns `INVALID_TYPE` or
   `JSON_PARSER_ERROR` immediately. Numbers accept `json.Number` or numeric
   strings and coerce to a canonical decimal string. Dates accept ISO 8601
   and normalize. Checkboxes accept `true`/`false`/`null`.

2. **Validate** — coerced value against field constraints:
   - `nillable=false` rejects null/missing on create
   - `length` rejects overlong strings (UTF-8 rune count)
   - `precision`/`scale` rejects decimal overflow
   - `PicklistValues` membership rejects unlisted values (single-select)
   - `autoNumber` rejects client-supplied values on create
   - `Unique` is metadata-only in this plan (enforcement deferred)

Shared helpers: `checkNillable`, `checkLength`, `checkPrecision`,
`checkPicklist`. The dispatcher is a single `coerceAndValidate(f Field, raw any) (any, error)`
called from `Validate()` in `validation.go`, replacing the string-only branch.

### Compound types

**Name** (Contact): subfields `FirstName`, `LastName`, `Salutation`. Each
subfield is a `string` with its own length. The compound `Name` describe
entry carries `compound: "Name"`. CRUD writes to subfield columns; the
compound value is assembled on read.

**Address** (Account): subfields `BillingStreet`, `BillingCity`,
`BillingState`, `BillingPostalCode`, `BillingCountry` (and shipping
counterparts). Same pattern: subfield columns on write, compound assembly
on read.

### Describe output

The describe response for each field includes the new metadata when set:
`precision`, `scale`, `picklistValues`, `autoNumber`, `compound`,
`unique`. The `type` value uses the Salesforce wire names above. Compound
fields appear as the compound type with a `compoundFields` array naming
their subfields.

### Storage

Field definitions are stored in `sc_catalog.fields.definition` (JSONB),
which already round-trips through `Load()`. The new Field struct fields
serialize into this JSONB automatically. Physical column types are set
when the admin API creates the field: the `CREATE TABLE`/`ALTER TABLE` DDL
maps each wire type to its PG column type via the type table.

### Acceptance

End-to-end: describe and CRUD for each type through two clients:

1. **JSforce** — create records with each field type, read back, verify
   values round-trip. Describe returns type-appropriate metadata.
2. **sf CLI** (`sf data create`, `sf data query`) — same operations.
3. **Raw HTTP** (candidate client) — POST/PATCH/GET with each type,
   verifying coercion and error messages match expectations.

Error cases tested per type:
- Wrong type (string to number) → `INVALID_TYPE`
- Overlong string → `STRING_TOO_LONG`
- Invalid date format → `INVALID_FIELD`
- Picklist value not in list → `INVALID_FIELD`
- Null on non-nillable → `REQUIRED_FIELD_MISSING`
- Client-supplied autoNumber → `AUTO_NUMBER_FIELD`

Decimal arithmetic: currency/number/percent values with exact decimal
representation (e.g., `0.1 + 0.2 = 0.3`) round-trip without floating-point
drift.

### Out of scope (stored, not enforced)

Formula, rollup, encrypted and geolocation field types accept metadata
definitions and store values, but do not compute formulas, aggregate
rollups, encrypt/decrypt, or validate coordinate ranges. Describe reports
their type; CRUD treats their values as opaque strings.
