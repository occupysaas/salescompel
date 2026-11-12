# Plan 28 — Field Types I Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** Implemented and verified 2026-11-11. See [decisions, gaps and evidence](../../compat/plan-28-field-types.md).

**Goal:** Make every Plan 28 field type definable, describable and usable through REST, Composite, the bounded query, unmodified JSforce and simple-salesforce, and the connected editor, with exact decimals and honest stored-not-enforced definitions.

**Architecture:** A new pure package, `internal/fieldtype`, owns value parsing, rounding and formatting for every type. `metadata.Field` gains typed options, persisted in a versioned definition shape, and schema administration generates the matching typed columns, constraints and foreign keys. The record service coerces through `fieldtype` before SQL, and the query layer selects typed expressions and converts them to wire values. Every adapter therefore inherits the types from one catalog snapshot.

**Tech Stack:** Go 1.25.0, PostgreSQL 17, pgx v5, Vue 3, TypeScript, pinned JSforce 3.11.0, simple-salesforce (existing Python gate), Playwright.

**Spec:** [Plan 28 design](../specs/2026-11-11-plan-28-field-types-design.md). Read both documents before execution; every *candidate* value below comes from it.

## Global Constraints

- Go 1.25.0; PostgreSQL 17 is the verification baseline; retain the pinned Vue/TypeScript/client toolchains.
- No new runtime dependency without a recorded reason; decimal math uses `math/big` or PostgreSQL `numeric` text, never float64.
- API version remains v65.0; OpenAPI remains 3.2.1 with `x-source` provenance.
- Typed relational storage through generated `f_<hex>` columns; owner-only DDL under advisory lock 739102 with `lock_timeout` 5s; forward-only migrations; RLS on every record and metadata table.
- Preserve existing Account/Contact IDs, values, external-ID behavior, Contact.AccountId restrict-on-delete and its error, and the restricted-role Compat_Text__c denial.
- Product copy uses the en_US, fr, de, pt_BR and iw catalogs; field and picklist labels are tenant data, escaped and never catalogued.
- Follow CLEAN_ROOM.md; every *candidate* behavior is recorded as observed, documented or a gap.

## Review Focus

- Eight parallel inserts on one auto-number field must yield eight distinct, increasing values; Task 5 `TestAutoNumberConcurrent` pins it with a pool of 8.
- Python sends `0.1 + 0.2` as `0.30000000000000004`; a Currency(18,2) field must store and return `0.3`, never a float artefact; Tasks 2 and 8 assert it.
- A DateTime with a negative offset that crosses midnight (`2026-03-08T23:30:00-05:00`) must return `2026-03-09T04:30:00.000+0000`; Task 2 `TestDateTimeOffsets` pins it.
- Composite that deletes a master whose detail is itself a master, then fails a later subrequest, must restore every level; Task 6 `TestCompositeCascadeRollback` pins it.
- A restricted picklist with `Open` must reject `open` (case-sensitive API value); Task 5 `TestRestrictedPicklistCase` pins it.

## File and interface map

| Files | Responsibility |
| --- | --- |
| `internal/fieldtype/{kind.go,decimal.go,temporal.go,text.go,picklist.go,autonumber.go}` + tests | Pure value rules per spec §1–§2; no database, no metadata import |
| `internal/metadata/{model.go,definition.go,fieldspec.go,stored.go,seed.go,admin.go,catalog.go}` | Typed options, persisted definition shape, DDL generation, stored-not-enforced records, standard seeds |
| `internal/migration/migrations/0004_field_types.sql`, `internal/migration/migrate.go` | Settings, auto-number counters, artifact columns, definition rewrite, standard columns and seeds |
| `internal/records/{values.go,validation.go,write.go,autonumber.go,errors.go}` | Coercion, defaults, auto-number allocation, constraint-to-error mapping |
| `internal/query/{select.go,wire.go,columns.go,normalize.go}` | Typed select expressions, wire conversion, compound reads, filter/sort rules |
| `internal/platform/{http.go,http_composite.go,oauth_http.go}` | `UseNumber` decoding, describe properties, currency Organization bridge |
| `cmd/salescompel/{schema.go,main.go}` | New type flags, `--definition`, object `--name-type`, `schema currency set`, `provision --currency` |
| `api/candidate-openapi.yaml`, `compat/clients/node/src/candidate-openapi-*.ts` | Field schema, wire forms, errors, traffic validation |
| `web/src/{api.ts,connected/ConnectedApp.vue,connected/fields.ts,connected/FieldInput.vue}`, `locales/messages/*.json` | Per-type controls and formatting |
| `compat/clients/node/src/candidate-field-types.ts`, `scripts/candidate-field-types-python.py`, `web/test/field-types.spec.ts` | Required acceptance |

Sequential execution: Tasks 1–9 in order. Task 3's migration must land before any task that reads the new definition shape.

---

### Task 1: Reference review and compatibility record

**Files:** Create `docs/compat/plan-28-field-types.md`.

**Interfaces:** Produces a table with one row per spec *candidate* (§2 value rules, §3 auto-number backfill and required-on-empty, §5 field selection, §6 exclusion from describe), each with the status `observed`, `documented` or `gap`, and its source. Later tasks link to it, and Task 9 completes it.

- [ ] **Step 1: Review Salesforce release notes and changelogs since 2026-11-05** for field-type, describe or REST value changes, and fetch each spec Sources URL. Record pages that return wrapper HTML or fail as unverified; do not inspect client or plugin implementation code.
- [ ] **Step 2: If an authorized trial-org session exists**, observe under CLEAN_ROOM.md one field of each active type: describe properties, numeric rounding and `NUMBER_OUTSIDE_VALID_RANGE`, string-for-number coercion, datetime precision and offsets, checkbox null, email rejection, multi-select normalization, restricted-picklist errors, lookup delete options and compound read shapes. Otherwise, record "no session" and keep each candidate at `documented` or `gap`. This step never blocks.
- [ ] **Step 3: If an observation contradicts a candidate**, update the spec and this plan together before Task 2 and note the change in the record.
- [ ] **Step 4: Commit** with `docs: record Plan 28 reference review`.

### Task 2: Field value rules

**Files:** Create `internal/fieldtype/{kind.go,decimal.go,temporal.go,text.go,picklist.go,autonumber.go}` and a `_test.go` for each.

**Interfaces (produces):**
- `type Kind string` with constants `Id, Text, TextArea, LongTextArea, Number, Currency, Percent, Date, DateTime, Checkbox, Picklist, MultiselectPicklist, Email, Phone, Url, Lookup, MasterDetail, AutoNumber, Address, PersonName`, and stored-only `Formula, RollupSummary, EncryptedText, Geolocation, RichTextArea`. `func (k Kind) Active() bool`; `func (k Kind) DescribeType() string`; `func (k Kind) SoapType() string`; `func (k Kind) Filterable() bool`; `func (k Kind) Sortable() bool`; `func ParseKind(cli string) (Kind, error)`.
- `var ErrSyntax, ErrRange, ErrEmail, ErrPicklist = errors.New(...)` sentinel errors; records maps them to API codes.
- `func Decimal(v any, precision, scale int) (string, error)`: accepts `json.Number` or a plain decimal string; returns the canonical decimal rounded half-up to `scale`; `ErrRange` when integer digits exceed `precision-scale`; `ErrSyntax` for exponent, NaN, Infinity, hex, blank or other kinds.
- `func DecimalJSON(text string) json.Number`: trims trailing fractional zeros and a trailing point (`12.50`→`12.5`, `3.00`→`3`, `-0.0`→`0`).
- `func Date(v any) (string, error)`: `YYYY-MM-DD` within 1700-01-01..4000-12-31.
- `func DateTime(v any) (time.Time, error)`: RFC 3339 with `Z`, `±hh:mm` or `±hhmm`, optional fraction, or no offset (UTC); returns UTC truncated to whole seconds; same range.
- `func FormatDateTime(t time.Time) string` → `2006-01-02T15:04:05.000+0000` in UTC.
- `func Email(s string) bool` per spec §2; `func Text(s string, max int) bool` (UTF-8, no NUL, rune count ≤ max).
- `func MultiPicklist(input string, defined []string) (canonical string, values []string)`: split on `;`, trim spaces, drop empties and duplicates, order by definition then input order; empty input gives `""`.
- `func ValidAutoNumberFormat(f string) bool`; `func FormatAutoNumber(f string, n int64, now time.Time) (string, error)`: one `{0…}` group of 1–10 zeros (zero-padded, longer numbers not truncated), optional `{YYYY}{YY}{MM}{DD}`, result ≤30 characters, otherwise an error.

- [ ] **Step 1: Write failing table tests.** `TestDecimal` cases (precision, scale → input ⇒ result): (18,2) `json.Number("0.30000000000000004")`⇒`0.30`; (18,2) `"12.345"`⇒`12.35`; (18,2) `"-12.345"`⇒`-12.35`; (5,2) `"999.995"`⇒ErrRange; (5,2) `"999.994"`⇒`999.99`; (18,0) `"1e3"`⇒ErrSyntax; `"NaN"`, `"0x10"`, `" "`, `true`⇒ErrSyntax; (18,18) `"0.1"`⇒`0.100000000000000000`. `TestDecimalJSON` covers the trimming examples above. `TestDateTimeOffsets`: `2026-03-08T23:30:00-05:00`⇒`2026-03-09T04:30:00.000+0000`; `+0530` and `+05:30` agree; `2026-01-01T00:00:00.999Z` truncates to `.000`; `1699-12-31T23:59:59Z`⇒ErrRange. `TestDate` rejects `2026-02-30`, `20260101`, `4001-01-01`. `TestEmail` accepts `a@b.co` and rejects `a@b`, `a b@c.d`, `a@@b.c`, `a@b..c`, 81 characters. `TestMultiPicklist` turns `" B ;A;;B;Z"` with defined `[A B C]` into `A;B;Z`. `TestAutoNumber` renders `PRJ-{0000}` at n=7 as `PRJ-0007` and at n=12345 as `PRJ-12345`, renders `{YYYY}-{00}` on 2026-11-11, and rejects two number groups, none, or a 31-character result. `TestKindTable` asserts DescribeType/SoapType for every active kind against spec §1 (`Number`→`double`/`xsd:double`, `Lookup`→`reference`/`tns:ID`, `Address`→`address`/`urn:address`, `AutoNumber`→`string`/`xsd:string`).
- [ ] **Step 2: Run `GOTOOLCHAIN=go1.25.0 go test ./internal/fieldtype -count=1`.** Expected: fail to compile (package absent).
- [ ] **Step 3: Implement the package.** Decimal rounding uses `math/big` on the digit string (no `big.Float`); add a test that greps the package for `float64` and `ParseFloat` and fails if they are found.
- [ ] **Step 4: Run Step 2 with `-race` and `go vet ./internal/fieldtype`.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: add exact field value rules`.

### Task 3: Typed metadata, migration 0004 and standard fields

**Files:** Create `internal/metadata/definition.go`, `internal/migration/migrations/0004_field_types.sql`. Modify `internal/metadata/{model.go,catalog.go,seed.go,catalog_test.go}`, `internal/migration/{migrate.go,migrate_test.go}`.

**Interfaces (produces):**
- `metadata.Field` keeps its current public JSON fields and adds describe properties: `SoapType string "soapType"`, `Precision, Scale, Digits int`, `PicklistValues []PicklistValue "picklistValues"`, `RestrictedPicklist, DependentPicklist, CascadeDelete, RestrictedDelete, AutoNumber, Calculated, DefaultedOnCreate, NameField, HTMLFormatted, Filterable, Sortable, Groupable, Unique bool`, `RelationshipName *string`, `RelationshipOrder *int`, `DefaultValue any`, `CompoundFieldName *string`. It also gains `Options Options` with `json:"-"`.
- `metadata.PicklistValue { Active bool "active"; DefaultValue bool "defaultValue"; Label string "label"; Value string "value"; ValidFor *string "validFor" }`.
- `metadata.Options { Kind fieldtype.Kind; Required bool; VisibleLines int; DeleteBehavior string /* SetNull|Restrict|Cascade */; Reparentable bool; DisplayFormat string; StartingNumber int64; Components []string /* compound */; Generated bool }`.
- `metadata.EncodeDefinition(f Field) ([]byte, error)` writes `{"v":2,"field":{public},"options":{...}}`; `metadata.DecodeDefinition(b []byte) (Field, error)` decodes strictly, rejects other versions, and rejects any inactive kind or kind/type mismatch.
- `metadata.Snapshot` gains `Currency string`; `Load` reads `sc_catalog.settings` and decodes through `DecodeDefinition`.
- `metadata.StandardObjects()` returns the existing fields plus spec §5 fields with these physical columns. Account: `phone, fax, website, type, industry, annual_revenue numeric(18,0), number_of_employees numeric(8,0), description text, billing_street, billing_city, billing_state, billing_postal_code, billing_country`, the `shipping_*` equivalents, and compound `BillingAddress`/`ShippingAddress` with no column. Contact: `salutation, name (generated), title, department, email, phone, mobile_phone, birthdate date, description, mailing_*`, and compound `MailingAddress`.
- Migration 0004 creates `sc_catalog.settings(tenant_id text PRIMARY KEY REFERENCES public.tenants(id) ON DELETE CASCADE, currency_iso_code char(3) NOT NULL DEFAULT 'USD' CHECK (currency_iso_code ~ '^[A-Z]{3}$'))` and `sc_catalog.auto_numbers(tenant_id text, field_id text, next_value bigint NOT NULL, PRIMARY KEY(tenant_id, field_id), FOREIGN KEY → sc_catalog.fields ON DELETE CASCADE)`, both with the closed RLS policy (settings read-only to records; auto_numbers SELECT/UPDATE). It adds `object_id, api_name, kind` to `sc_catalog.artifacts`, rewrites every existing definition to v2 (`id`→Id, `string`→Text, `reference`→Lookup with Restrict for Contact.AccountId), and adds the standard columns, Contact `name varchar(121) GENERATED ALWAYS AS (nullif(btrim(concat_ws(' ', first_name, last_name)), '')) STORED`, standard metadata rows (ordinals after the existing fields) and settings rows for every tenant. The manifest appends `{4, "field types", sha256}`.

- [ ] **Step 1: Write failing tests.** `TestDefinitionRoundTrip` covers every active kind and rejects `"v":1`, unknown keys and `Formula`. `TestCatalogSeedParity` (existing) now asserts that migration 0004 on an existing tenant and `SeedTenant` on a new tenant produce byte-identical `EncodeDefinition` output and ordinals for Account and Contact. `TestMigrationFieldTypesUpgrade` populates a Plan 27 database (two tenants, a custom object with a Text field, Accounts with Compat_Key__c, Contacts with AccountId, OAuth state), runs `up`, and asserts every prior value, ID, field ID and the revision are preserved. It also checks that new standard columns are null and that Contact `Name` equals `First Last`, or `Last` when FirstName is null. `TestFieldTypesRLS` uses the records role and no tenant context to see zero settings and auto_numbers rows and fail to update another tenant's counter. Extend the migration checksum and newer/gapped-history tests for version 4.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/metadata ./internal/migration -count=1`.** Expected: new tests fail.
- [ ] **Step 3: Implement the model, definition codec, seeds and 0004.** Standard picklist values for Type, Industry and Salutation are copied from the documentation cited in the spec, unrestricted, with `x-source` noted in a comment beside the seed. Describe flags derive from `Kind` (`Filterable`, `Sortable`, `Groupable` false for LongTextArea, MultiselectPicklist, Address; `NameField` true for Account.Name, Contact.Name and custom Name). Do not change behavior in records/query yet beyond compiling against the new `Load`.
- [ ] **Step 4: Run Step 2 plus `go test ./internal/records ./internal/query ./internal/platform -count=1`.** Expected: all pass. Existing describe output may gain properties only; update golden tests to the new properties, without changing existing values.
- [ ] **Step 5: Commit** with `feat: persist typed field definitions and seed standard fields`.

### Task 4: Schema administration for every type

**Files:** Create `internal/metadata/{fieldspec.go,stored.go,fieldspec_test.go}`. Modify `internal/metadata/{admin.go,admin_test.go}`, `cmd/salescompel/{schema.go,schema_test.go,main.go,admin.go}`.

**Interfaces:**
- Consumes: Task 2 `fieldtype` validators; Task 3 `Field`, `Options` and `EncodeDefinition`.
- Produces: `metadata.FieldInput` keeps `Object, Name, Label, Type string; Length int` and adds `Precision, Scale, VisibleLines int; Values []PicklistValue; Restricted, Required, Reparentable bool; Default *string; ReferenceTo, DeleteBehavior, DisplayFormat, RelationshipName string; StartingNumber int64`. Also `metadata.BuildField(in FieldInput, target *Object, snapshot Snapshot) (Field, ColumnDDL, error)` with `ColumnDDL { Type string; Check string; ForeignKey string }`. `metadata.ObjectInput` adds `NameType, DisplayFormat string; StartingNumber int64`. Further produces `metadata.CreateStoredField(ctx, owner, tenant string, object string, definition []byte) (StoredField, error)`, `metadata.SetCurrency(ctx, owner, tenant, code string) error`, and `Snapshot.Stored []StoredField { Object, Name, Label, Kind string }`, loaded from artifacts. `schema inspect` emits each field with `"status":"active"` and each stored definition with `"status":"stored-not-enforced"`.
- CLI flags exactly as spec §3: `--precision --scale --visible-lines --values --restricted --default --required --reference-to --delete-behavior --reparentable --display-format --starting-number --relationship-name --definition`. Also `schema object create … --name-type Text|AutoNumber --display-format --starting-number`, `schema currency set --tenant T --code EUR`, and `provision` reads `SALESCOMPEL_CURRENCY` (default `USD`).

- [ ] **Step 1: Write failing tests.** `TestBuildFieldRules` gives each kind a valid case plus every rejection: precision 0/19, scale > precision, LongTextArea length 255/131073, Picklist with 0 or 1,001 values or a 256-character value, duplicate values, a default not in values, Lookup to an unknown object, MasterDetail on Account/Contact as child or a third master-detail, a MasterDetail self-reference, an AutoNumber format without a number group, flags irrelevant to the type (e.g. `--precision` on Email), and `--required` on Checkbox/AutoNumber/MasterDetail (implicitly non-null) or on a non-empty object. `TestCreateFieldDDL` checks for each kind that `information_schema.columns` reports the expected type, nullability and default, and that the FK's ON DELETE matches SetNull→`SET NULL (col)`, Restrict→`RESTRICT`, MasterDetail→`CASCADE`. `TestMasterDetailRequiresEmpty` fails on an object with a record and leaves no column or metadata. `TestAutoNumberBackfill` adds an AutoNumber to an object with three records and finds them numbered from the starting number in ascending Id order, with the counter set to the next value. `TestAutoNumberObject` creates an object whose Name is AutoNumber; its column is unique per tenant and describe shows `autoNumber:true, createable:false`. `TestStoredFieldReservesName`: a stored Formula `Score__c` makes a later active `score__c` fail; inspect shows it, and describe and `information_schema` do not. `TestSchemaCurrency` accepts `EUR`, rejects `eur` and `EURO`, and shows the result in `Snapshot.Currency`. Extend the CLI tests for flag parsing and rejection of unknown flags.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/metadata ./cmd/salescompel -count=1`.** Expected: new tests fail.
- [ ] **Step 3: Implement.** Keep a single owner transaction per command (DDL, FK, grants, metadata, counter row, revision); FK constraint names are `fk_<field id>`, and auto-number unique indexes `uq_<field id>`. Stored definitions validate only that JSON is an object with `type` in the stored-only kinds and a valid API name and label, and retain the raw bytes. `relationshipName` defaults per spec §1 and must be unique per object.
- [ ] **Step 4: Run Step 2 and the Plan 27 concurrency tests (`-run 'Schema|Catalog'`).** Expected: pass with no partial DDL after injected failures.
- [ ] **Step 5: Commit** with `feat: administer typed, relationship and stored field definitions`.

### Task 5: Typed record writes

**Files:** Create `internal/records/{values.go,autonumber.go,errors.go,values_test.go}`. Modify `internal/records/{validation.go,write.go,references.go}`, `internal/platform/{http.go,http_composite.go,http_test.go}`.

**Interfaces:**
- Consumes: Task 2 and Task 3/4 metadata.
- Produces: `records.coerce(f Field, v any) (param any, cast string, err error)` returns the SQL parameter and its explicit cast (`::numeric`, `::date`, `::timestamptz`, `""`), and `records.applyDefaults(o Object, data map[string]any)`. Also `records.allocateAutoNumbers(ctx, db, p Principal, o Object, now time.Time) (map[string]string, error)` (one `UPDATE … RETURNING` per auto-number field), and `records.dbError(err error, op string) error`, which maps by constraint name: `fk_*` 23503 → `DELETE_FAILED` when op is delete, otherwise `INVALID_CROSS_REFERENCE_KEY`. The legacy contacts FK keeps today's mapping, the Compat_Key__c unique keeps `DUPLICATE_VALUE`, and 22003 maps to `NUMBER_OUTSIDE_VALID_RANGE`. HTTP and Composite body decoding call `UseNumber()`.

- [ ] **Step 1: Write failing tests** (database-backed, using a tenant with one field of every kind from Task 4). `TestWriteEveryKind` asserts create plus Read round trip values for each kind, including `0.30000000000000004`→`0.3`, `"12.5"` string accepted, `""` on Number → null, Checkbox omitted → default, null → false, Picklist default on create only, multi-select canonicalization, a 15-character Lookup ID normalized to 18, and DateTime offset conversion. `TestWriteErrors` covers each code with its field name: `NUMBER_OUTSIDE_VALID_RANGE`, `JSON_PARSER_ERROR` (object for number, `"true"` for checkbox, `"2026-02-30"` for date), `INVALID_EMAIL_ADDRESS`, `INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST`, `MALFORMED_ID`, `INVALID_CROSS_REFERENCE_KEY` (a well-formed but absent ID, and an ID that exists only in tenant B), `INVALID_FIELD_FOR_INSERT_UPDATE` (AutoNumber, Address, Contact.Name, non-reparentable MasterDetail update), `REQUIRED_FIELD_MISSING` (MasterDetail omitted). `TestRestrictedPicklistCase` covers `open` versus `Open`. `TestDeleteBehaviors`: SetNull clears the child, Restrict gives `DELETE_FAILED` and keeps both rows, MasterDetail cascades two levels, and Contact.AccountId is unchanged. `TestAutoNumberConcurrent` runs 8 goroutines on a pool of 8 and expects distinct, strictly increasing values in commit order with gaps allowed. `TestHTTPNumberPrecision` sends `{"Amount__c": 12345678901234567.89}` through HTTP to Number(18,2) and reads the exact value back.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/records ./internal/platform -count=1`.** Expected: new tests fail.
- [ ] **Step 3: Implement.** `Validate` dispatches on `Options.Kind` and keeps sorted-key error order. Text kinds keep today's rules. Generalize `normalizeReferences` to every reference field. `columns` emits `$n::cast` placeholders. Auto-number allocation happens inside the caller's transaction, after validation and before INSERT.
- [ ] **Step 4: Run Step 2 with `-race`, then the full existing suite `go test ./... -count=1`.** Expected: pass; Plan 25/27 regression tests unchanged.
- [ ] **Step 5: Commit** with `feat: validate and store every active field type`.

### Task 6: Typed reads, describe, query and Composite

**Files:** Create `internal/query/{wire.go,wire_test.go}`. Modify `internal/query/{select.go,columns.go,normalize.go}`, `internal/records/validation.go` (Describe), `internal/platform/{http.go,oauth_http.go,http_composite_test.go,http_test.go}`.

**Interfaces:**
- Consumes: Tasks 2–5.
- Produces: `query.selectExpression(f Field) string`. Numeric fields become `col::text`, Date `to_char(col,'YYYY-MM-DD')`, and DateTime the UTC `to_char` form from spec §1. An Address expands to its component columns. `query.wireValue(f Field, raw any) any` gives `fieldtype.DecimalJSON` for numerics and an address object or null. `records.Describe` emits the Task 3 properties with per-field `createable`/`updateable` from `Options`. The Organization bridge `isCurrencyDiscovery(q string) bool` matches exactly `SELECT DefaultCurrencyIsoCode FROM Organization` (whitespace-normalized like the existing bridge) and returns `{"DefaultCurrencyIsoCode": snapshot.Currency}`.

- [ ] **Step 1: Write failing tests.** `TestDescribeEveryKind` compares the full describe of the Task 4 object and of Account and Contact with golden JSON in `internal/platform/testdata/describe-field-types.json`. `TestQueryTypedProjection` checks SELECT of every kind, ORDER BY Number, Date, DateTime, Checkbox and Picklist (nulls per existing placement), and equality on Picklist, Email and Lookup. It expects `MALFORMED_QUERY` with message `Filter on this field type is not yet supported` for numeric, date, datetime and checkbox equality, and `INVALID_FIELD` for filtering or ordering by LongTextArea, MultiselectPicklist or Address. `TestAddressRead` returns all five keys with nulls when partly filled, and null when empty. `TestCompositeCascadeRollback` covers the Review Focus case, and also checks that an auto-number value allocated in a rolled-back Composite is never visible. `TestCurrencyBridge` returns the tenant's code and rejects near-miss queries through the normal query path. Restricted-role describe still omits Compat_Text__c.
- [ ] **Step 2: Run `TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test ./internal/query ./internal/records ./internal/platform -count=1`.** Expected: fail.
- [ ] **Step 3: Implement.** Column metadata reports each field's describe type. Equality on string-like kinds keeps today's case-insensitive comparison only for Text, TextArea, Email, Phone and Url; Picklist, Lookup, Id and AutoNumber compare exactly. Query parsing is unchanged.
- [ ] **Step 4: Run Step 2 with `-race`.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: serve typed values through describe, query and Composite`.

### Task 7: Served contract and traffic validation

**Files:** Modify `api/candidate-openapi.yaml`, `internal/platform/http_openapi_test.go`, `compat/clients/node/src/{candidate-openapi-model.ts,candidate-openapi-traffic.ts,candidate-openapi-contract.ts}`, `compat/clients/node/test/candidate-openapi*.test.ts`.

**Interfaces:** `Field` schema adds every Task 3 property (still `additionalProperties: false`), extends the `type` enum to the describe types of active kinds, and adds `PicklistValue` and `Address` schemas. Dynamic record maps document the per-type value forms. Error codes added: `NUMBER_OUTSIDE_VALID_RANGE`, `INVALID_EMAIL_ADDRESS`, `INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST`, `DELETE_FAILED`, `INVALID_FIELD_FOR_INSERT_UPDATE`. Each gets `x-source` and a status of documented or candidate.

- [ ] **Step 1: Write failing traffic tests** that send one valid request and one rejected request for every active kind, plus a describe, a cascade delete and the currency bridge. The validator must accept them all, and must reject a describe carrying an unknown type or property.
- [ ] **Step 2: Run `npm --prefix compat/clients/node run check` and `GOTOOLCHAIN=go1.25.0 go test ./internal/platform -run 'OpenAPI' -count=1`.** Expected: fail.
- [ ] **Step 3: Update the schema and validators;** state that formula, roll-up, encrypted, geolocation and rich text fields are never described (stored-not-enforced).
- [ ] **Step 4: Run Step 2.** Expected: pass with only the established offline skips.
- [ ] **Step 5: Commit** with `feat: document field types in the served contract`.

### Task 8: Connected editor controls

**Files:** Create `web/src/connected/{fields.ts,FieldInput.vue}`, `web/test/fields.test.ts`, `web/test/field-types.spec.ts`. Modify `web/src/api.ts`, `web/src/connected/ConnectedApp.vue`, `web/playwright.config.ts`, `locales/messages/{en_US,fr,de,pt_BR,iw}.json`.

**Interfaces:** `api.ts` `Field` adds the describe properties used: `precision, scale, picklistValues, restrictedPicklist, referenceTo, autoNumber, compoundFieldName, defaultValue`. `RecordData` values become `string | number | boolean | null | object`. `fields.ts` exports `validateDecimal(text: string, precision: number, scale: number): "ok" | "syntax" | "range"` (string digit logic; no `Number()`), `formatValue(field: Field, value: unknown, locale: string, currency: string): string`, `toApiValue(field: Field, input: string | boolean | string[]): unknown`, and `loadCurrency(signal): Promise<string>` (currency bridge). `FieldInput.vue` takes props `field`, `modelValue`, `disabled` and `lookupOptions` and renders the control for each type in spec §7.

- [ ] **Step 1: Write failing tests.** `fields.test.ts` covers `validateDecimal("12345678901234567.89", 18, 2)` as ok, `"1e3"` as syntax and `"1000"` at (5,2) as range. It checks `formatValue` for Currency in en_US/USD and de/EUR, Percent, DateTime in the browser time zone, and multi-select. For numeric fields, `toApiValue` returns a `RawNumber` wrapping the validated text, and `api()` serializes `RawNumber` as an unquoted JSON number literal. The test asserts that the request body contains `12345678901234567.89` exactly. Playwright `field-types.spec.ts` edits and saves each type on the Task 4 object, checks the read-only auto-number and address display, the lookup select, the stored-not-enforced note listing a stored Formula, the restricted picklist error message, currency formatting after switching to German, URL rendered as text, and Hebrew RTL.
- [ ] **Step 2: Run `npm --prefix web test` and `npm --prefix web run test:e2e -- field-types.spec.ts`** with the existing candidate fixture environment. Expected: fail.
- [ ] **Step 3: Implement.** Add every new product message to all five catalogs. The lookup select loads `SELECT Id,Name FROM <target> ORDER BY Name LIMIT 200` through the existing query path. Keep the Plan 27 stale-response generation guard for lookup and currency loads.
- [ ] **Step 4: Run Step 2, the existing connected-browser specs, and `npm --prefix web run build`.** Expected: pass.
- [ ] **Step 5: Commit** with `feat: edit every field type in the connected editor`.

### Task 9: Required acceptance and documentation

**Files:** Create `compat/clients/node/src/candidate-field-types.ts`, `compat/clients/node/test/candidate-field-types.test.ts`, `scripts/candidate-field-types-python.py`. Modify `compat/clients/node/src/candidate-acceptance-{contract,run,results}.ts`, `compat/clients/node/src/candidate-schema.ts` (restart fixtures), `compat/schemas/candidate-acceptance.schema.json`, `docs/compat/plan-28-field-types.md`, `README.md`, the first release roadmap and this plan's and the spec's Status lines.

**Interfaces:** `runFieldTypesGate(c: GateContext): Promise<ParsedResult>` runs after `schema` and before `restart`. Required IDs: `field-types/provision`, `field-types/jsforce-describe`, `field-types/jsforce-crud`, `field-types/jsforce-errors`, `field-types/relationships`, `field-types/composite-rollback`, `field-types/standard-fields`, `field-types/python-crud`, `field-types/http-contract`. Also `restart/field-types` and connected-browser IDs `browser/field-types-edit`, `browser/field-types-stored`, `browser/field-types-locale`. Every created resource is journaled intent-first, following `candidate-schema.ts`.

- [ ] **Step 1: Write failing harness tests:** a missing, skipped or partial field-types gate or `restart/field-types` makes full acceptance ineligible, and the Python script receives only the limited token and instance URL.
- [ ] **Step 2: Run `npm --prefix compat/clients/node run check`.** Expected: fail.
- [ ] **Step 3: Implement the gate.** Through owner subprocesses, create the spec §8 object set (an auto-number Name, one field per active kind, Lookup SetNull and Restrict, MasterDetail to a second object, and one stored definition per stored kind). JSforce asserts describe against the Task 6 golden subset, full CRUD with boundary values, every error, cascade/restrict deletes, Composite rollback, and Account/Contact standard fields with compound reads. The Python script uses `Decimal` and float inputs including `0.1 + 0.2`, dates and datetimes, and asserts exact returned values. It retains one record per kind for the restart gate and asserts identical values after restart. Cleanup removes everything in reverse dependency order and verifies that nothing remains.
- [ ] **Step 4: Run the final checks** against disposable PostgreSQL with the real restricted roles:

```bash
GOTOOLCHAIN=go1.25.0 go vet ./...
TEST_DATABASE_URL=<disposable-url> GOTOOLCHAIN=go1.25.0 go test -race ./... -count=1
npm --prefix compat/clients/node run check
npm --prefix web test
npm --prefix web run build
CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <exclusive-absolute-report-path>
```

Expected: every gate passes with all required field-types, restart and browser IDs present and zero required skips; cleanup is verified. Record the tested revision, report path and SHA-256.
- [ ] **Step 5: Finish documentation.** In `docs/compat/plan-28-field-types.md`, complete the candidate table, list the gaps (typed WHERE → 29, geocodes/state-country picklists, OtherAddress, name locale order → 33, picklist edits → 35, Time fields, account-delete cascade → 30), the operator guide for the new CLI flags, `--definition` examples, `schema currency set`, the upgrade note (stop, back up, `migrate up`, start), and the evidence table. Update the README and roadmap implementation record; mark this plan and the spec implemented only after evidence passes.
- [ ] **Step 6: Commit** with `test: require field type acceptance and document Plan 28`. Run full acceptance again if documentation changes source bindings, and record the tested revision separately from any evidence-only commit.

## Self-review

Coverage of the spec: §1 type catalog → Tasks 2–4; §2 values and errors → Tasks 2, 5; §3 storage, migration and CLI → Tasks 3–4; §4 runtime → Tasks 5–6; OpenAPI → Task 7; §5 standard fields → Task 3 (seed), 6 (describe), 9 (clients); §6 stored-not-enforced → Tasks 4, 7, 8; §7 editor → Task 8; §8 evidence → Tasks 1, 9. All five Review Focus conditions have owning tests. Names used across tasks: `fieldtype.Kind`, `metadata.Options`, `EncodeDefinition`/`DecodeDefinition`, `BuildField`, `Snapshot.Currency`, `Snapshot.Stored`, `records.coerce`, `query.wireValue`, `runFieldTypesGate`.
