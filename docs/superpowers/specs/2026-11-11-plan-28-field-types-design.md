# Plan 28 — Field types I

**Status:** Draft for maintainer review, 2026-11-11.
**Date:** 2026-11-11
**Depends on:** Plan 27 (typed tenant metadata, generated physical columns, RLS, schema CLI).
**Roadmap:** [first release roadmap](2026-11-04-salescompel-first-release-roadmap-design.md), Plan 28; "Stored, not enforced" in §1.
**Implementation:** [Plan 28](../plans/2026-11-11-plan-28-field-types.md).

## Intent and outcome

Today every field a tenant can define is text. Real Salesforce orgs, even simple ones, depend on numbers, money, dates, checkboxes, picklists, email/phone/URL fields, relationships, auto-numbered names and addresses. Without them, no org could move its data in, and no client that reads describe could treat SalesCompel as a normal org.

After Plan 28, an operator can define fields of each supported type on standard or custom objects. Unmodified JSforce and simple-salesforce can describe them, then create, read, update and delete records using them, with the value formats and errors Salesforce documents. The connected editor shows and edits each type with a suitable control. Account and Contact gain the common standard fields that use these types, including the compound Name and Address fields. Types the candidate cannot yet run (formula, roll-up summary, encrypted, geolocation, rich text) can be recorded and retained, but they are never served as working fields.

The maintainer handed scope decisions to this design ("write the spec from the roadmap and Salesforce's abilities"). Every choice below that is not in the roadmap is a **candidate call** by the author, listed at the end for review. Salesforce behavior here is from public documentation and was not observed on the trial org during drafting. Task 1 confirms what it can, and anything left unconfirmed is recorded as a gap. It never blocks the plan.

## Scope and boundaries

Ships:

- Active field types: Text, Text Area, Long Text Area, Number, Currency (single currency), Percent, Date, Date/Time, Checkbox, Picklist, Multi-Select Picklist, Email, Phone, URL, Lookup, Master-Detail and Auto Number. Auto Number is also offered as the Name type of a new custom object.
- Compound Name on Contact and compound Billing/Shipping/Mailing Address on Account and Contact, read through REST as Salesforce documents them and written through their component fields.
- A bounded set of new standard fields on Account and Contact (§5), seeded for every existing and new tenant.
- Exact decimal handling from the HTTP body to PostgreSQL and back. Binary floating point is never used for field values.
- A tenant corporate currency setting, used for display only.
- Stored-not-enforced definitions for Formula, Roll-Up Summary, Encrypted Text, Geolocation and Rich Text Area (§6).
- Describe, REST CRUD, Composite and the existing bounded SOQL all working with every active type. OpenAPI and traffic validation are updated to match.
- Connected editor controls for every active type.
- Acceptance coverage through JSforce and simple-salesforce, plus database, upgrade and browser tests.

Non-goals: SOQL grammar beyond today's (typed literals in WHERE, relationship traversal, `INCLUDES`/`EXCLUDES`, date literals, aggregates; all Plan 29); child relationships in describe and relationship queries (29); formula or roll-up evaluation; encryption at rest; geolocation distance queries; validation rules; field dependencies (dependent picklists, controlling fields); global value sets; picklist value translation; record types; multi-currency and `CurrencyIsoCode`; Time fields; editing, renaming or deleting field definitions, including adding picklist values after creation (Plan 35 metadata path); new unique or external-ID fields; field history; audit fields such as CreatedDate (30); per-field permissions (33); master-detail sharing inheritance (34; tenant-wide access continues until then); setup UI (36).

Anti-goals:

- Any numeric value passing through float64, or being rounded differently from the documented rule.
- A formula, roll-up, encrypted or geolocation field appearing in describe or record JSON as if it were live.
- Encrypted-field values stored in plaintext under a field that claims encryption.
- A picklist marked restricted that accepts an unlisted value.
- Master-detail children left orphaned, or a lookup that silently points at another tenant's record.
- Changing existing Account/Contact behavior, IDs or values beyond the documented additions.

## Global constraints

Plan 27's constraints carry over unchanged: Go 1.25.0, PostgreSQL 17 baseline, API v65.0, OpenAPI 3.2.1 with `x-source`, no new runtime dependency without a recorded reason, typed relational storage through generated columns, owner-only DDL under the schema advisory lock, forward-only migrations, RLS on every record and metadata table, localized product copy in the five existing catalogs, and `CLEAN_ROOM.md`. Field labels and picklist labels are tenant data: Vue escapes them, and they never go into shipped catalogs.

## 1. Type catalog

Each active type has a logical type in metadata, a describe `type` and `soapType`, a physical column type, a JSON wire form and validation. Limits are documented Salesforce limits unless marked *candidate*.

| Type (CLI `--type`) | describe `type` | Physical column | JSON value | Parameters and limits |
| --- | --- | --- | --- | --- |
| `Text` | string | varchar(n) | string | length 1–255 (as Plan 27) |
| `TextArea` | textarea | varchar(255) | string | fixed length 255; line breaks kept |
| `LongTextArea` | textarea | text with CHECK length ≤ n | string | length 256–131,072, default 32,768; visible lines are display-only metadata |
| `Number` | double | numeric(p,s) | number | precision 1–18, scale 0–precision, integer digits = p−s |
| `Currency` | currency | numeric(p,s) | number | as Number; default 18,2 |
| `Percent` | percent | numeric(p,s) | number | as Number; the value 12.5 means 12.5% |
| `Date` | date | date | `"YYYY-MM-DD"` | 1700-01-01 to 4000-12-31 |
| `DateTime` | datetime | timestamptz | `"YYYY-MM-DDThh:mm:ss.000+0000"` | same range, stored UTC to whole seconds (*candidate*; see §2) |
| `Checkbox` | boolean | boolean NOT NULL DEFAULT d | true/false | default false or true; never null |
| `Picklist` | picklist | varchar(255) | string | 1–1,000 values (*candidate*), each value ≤255; restricted or not; optional default value |
| `MultiselectPicklist` | multipicklist | varchar(4099) | `"A;B"` | as Picklist; visible lines display-only; ≤100 selected (*candidate*) |
| `Email` | email | varchar(80) | string | format check (§2) |
| `Phone` | phone | varchar(40) | string | no format check |
| `Url` | url | varchar(255) | string | no scheme added or required |
| `Lookup` | reference | varchar(18) + composite FK | 18-char ID | target: any object in the tenant (self-lookup allowed); on parent delete: `SetNull` (default) or `Restrict` |
| `MasterDetail` | reference | varchar(18) NOT NULL + composite FK ON DELETE CASCADE | 18-char ID | target: any object except the child itself; child must be a custom object; at most two per object; `--reparentable` optional |
| `AutoNumber` | string (`autoNumber: true`) | varchar(30) NOT NULL, unique per tenant | string | display format with `{0}`–`{0000000000}` and optional `{YYYY}`/`{YY}`/`{MM}`/`{DD}`; starting number ≥0 |

Describe adds the type-specific properties Salesforce documents, which all fields carry so the OpenAPI schema stays closed: `soapType`, `precision`, `scale`, `digits`, `picklistValues` (`active`, `defaultValue`, `label`, `value`, `validFor: null`), `restrictedPicklist`, `dependentPicklist: false`, `relationshipName`, `relationshipOrder` (0 or 1 for master-detail, otherwise null), `cascadeDelete`, `restrictedDelete`, `autoNumber`, `calculated: false`, `defaultValue`, `defaultedOnCreate`, `nameField`, `compoundFieldName`, `htmlFormatted: false`, `filterable`, `sortable`, `groupable` and `unique`. `createable` and `updateable` become per-field values instead of "everything except Id": false for Id, auto-number, compound fields and Contact.Name, and `updateable` false for non-reparentable master-detail. `nillable` is false for checkbox, master-detail, auto-number and required Name fields.

`relationshipName` defaults to the field name with `__c` replaced by `__r`, and `Id` dropped for standard references (Contact.AccountId → `Account`). It is described now so Plan 29 can rely on it; traversal itself is Plan 29.

## 2. Values, validation and errors

**Decoding.** REST and Composite bodies decode with `json.Decoder.UseNumber` (or an equivalent that keeps the literal text). Numeric values reach PostgreSQL as decimal text parameters and come back as `numeric` text. The response encoder writes them as JSON number literals without trailing fractional zeros (12.50 → `12.5`), matching documented REST output (*to confirm in Task 1*). A numeric field also accepts a JSON string holding a plain decimal literal (*candidate*: Salesforce REST coerces strings for numeric fields; not observed). Exponent notation, NaN, Infinity, hex and empty strings are rejected; an empty string on a nillable numeric field becomes null, as for text.

**Rounding and range.** A value with more fractional digits than the scale is rounded to the scale using round-half-up (*candidate* reading of the documentation; unverified). If the rounded value has more integer digits than p−s, the write fails with `NUMBER_OUTSIDE_VALID_RANGE` naming the field. All arithmetic in Go uses decimal values (`math/big` or the PostgreSQL value as text); a helper with exhaustive tests owns parsing, rounding and formatting.

**Dates and times.** Date accepts `YYYY-MM-DD` only. DateTime accepts RFC 3339 with `Z` or a numeric offset (`+0000` and `+00:00` both accepted), optional fractional seconds, and a bare date-time without an offset read as UTC (*candidate*). Values are stored in UTC and returned as `YYYY-MM-DDThh:mm:ss.000+0000`. Fractional seconds are truncated (*candidate*; record whether Salesforce keeps milliseconds). Out-of-range or unparsable values fail with `JSON_PARSER_ERROR` naming the field.

**Checkbox.** Accepts JSON true/false. On create, an omitted value takes the field default. An explicit null stores false (*candidate*). Strings such as `"true"` are rejected with `JSON_PARSER_ERROR` (*candidate*).

**Picklists.** A value is matched against active values case-sensitively by API value. A restricted picklist rejects an unlisted or inactive value with `INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST`. An unrestricted picklist stores any value up to 255 characters. Multi-select splits on `;`, trims spaces around each value, drops empty entries and duplicates, orders selections by definition order (unlisted values last, in input order), and stores the canonical `A;B` text. An empty selection becomes null. On create, an omitted picklist takes its default value if one is defined.

**Email** requires exactly one `@`, a non-empty local part, a domain containing a dot with no empty labels, no whitespace and no control characters, length ≤80; otherwise `INVALID_EMAIL_ADDRESS` (*candidate* rule; Salesforce's exact grammar is unverified). Phone and URL check only length and text validity.

**References.** Lookup and master-detail values accept 15- or 18-character IDs and are normalized against the target object's prefix (existing `normalizeReferences`, generalized from Contact.AccountId). A malformed ID returns `MALFORMED_ID`. A well-formed ID that does not exist in the tenant returns `INVALID_CROSS_REFERENCE_KEY`. Composite foreign keys `(tenant_id, column) → target(tenant_id, id)` enforce this in the database. `SetNull` uses PostgreSQL 17's `ON DELETE SET NULL (column)` so `tenant_id` is never cleared. Deleting a parent that a `Restrict` lookup still references returns `DELETE_FAILED`, and master-detail deletion cascades inside the same transaction. The existing Contact.AccountId restrict behavior and its error stay unchanged (*candidate*; documented Salesforce behavior is account deletion cascading to contacts, but changing it belongs with Plan 30's recycle bin). Changing a non-reparentable master-detail value fails with `INVALID_FIELD_FOR_INSERT_UPDATE`.

**Read-only fields.** Writing to an auto-number, compound or other non-createable/non-updateable field returns `INVALID_FIELD_FOR_INSERT_UPDATE` naming the field.

**Wrong JSON kinds.** A JSON value of the wrong kind for the field (for example an object for a number) returns `JSON_PARSER_ERROR` naming the field, consistent with today's text handling.

All validation runs in `internal/records` before SQL. Database constraints are the backstop, and `dbError` maps each constraint failure to the same error codes (generalizing today's hard-coded `Compat_Key__c` duplicate mapping). Error ordering stays deterministic: sorted field names, as today.

## 3. Metadata, storage and migration

The field definition JSON in `sc_catalog.fields.definition` gains the type parameters (precision, scale, picklist values with active/default flags, restricted flag, delete behavior, relationship order, reparentable, auto-number format and start, default value, compound parent, visible lines). `metadata.Load` keeps strict decoding and its closed type list. It now admits every active type in §1 and rejects any other value. Physical column names stay generated, never derived from API names.

Auto-number values come from a per-field counter row in a new `sc_catalog.auto_numbers(tenant_id, field_id, next_value)` table, under RLS, incremented in the record transaction. Values are unique and increase per field. Gaps after rollback are allowed, as in Salesforce. The builder may use another mechanism with the same guarantees. When an auto-number field is added to an object with existing records, they are numbered in ascending Id order inside the schema transaction (*candidate*; Salesforce offers this as an option, and Id order stands in for creation order until Plan 30 adds CreatedDate).

Migration 0004 adds the auto-number table, rewrites existing field definitions to the extended JSON shape with explicit defaults, adds the §5 standard columns to the shared `accounts` and `contacts` tables, seeds their metadata for every existing tenant, adds the tenant settings table with a `USD` row per tenant, and adds `object_id`, `api_name` and `kind` columns to `sc_catalog.artifacts`, with a tenant-scoped case-insensitive uniqueness check shared with active field names (§6). `SeedTenant` seeds the same set for new tenants. A test asserts that migrated and freshly seeded catalogs are identical. Upgrade tests cover a populated Plan 27 database and preserve every existing value, ID, definition ID and revision ordering.

**Schema CLI.** `schema field create` keeps its Plan 27 flags and adds type parameters: `--precision`, `--scale`, `--visible-lines`, `--values` (comma-separated API values; labels default to values), `--restricted`, `--default`, `--reference-to`, `--delete-behavior SetNull|Restrict`, `--reparentable`, `--display-format`, `--starting-number` and `--relationship-name`. Unknown or irrelevant flags for a type are rejected. `--definition FILE` accepts the same parameters as a JSON object, including picklist labels, and is the only way to create stored-not-enforced definitions. `schema object create --name-type AutoNumber --display-format F [--starting-number N]` creates a custom object whose Name is auto-numbered. Creating a master-detail field on an object that already has records fails with an explicit message (Salesforce requires a lookup to be filled and converted, and conversion is out of scope). `--required` is accepted for Text, numeric, Date/DateTime, Picklist, Email, Phone, URL and Lookup only on objects with no records (*candidate*). `schema inspect` reports every definition with `"status": "active"` or `"status": "stored-not-enforced"`.

**Tenant currency.** A tenant-scoped `sc_catalog.settings(tenant_id, currency_iso_code)` row (ISO 4217, three uppercase letters, default `USD`), under RLS and readable by the records role, is set by `provision --currency` and changeable with `schema currency set`. It is loaded with the catalog snapshot. It affects only UI formatting. API currency values are bare numbers, as in a single-currency org, and no `CurrencyIsoCode` field is described. The editor reads it through one more exact-match Organization bridge, `SELECT DefaultCurrencyIsoCode FROM Organization`, using Salesforce's documented Organization field name. The control-plane `tenants` table is not used, because the records role cannot read it.

## 4. Runtime behavior

The record service, describe, Composite and query all read the same catalog snapshot, so every type works in every adapter at once. Specifically:

- **Describe** returns the §1 properties. Restricted-role field hiding (Account.Compat_Text__c) is unchanged.
- **Create/update/upsert/delete** apply §2. Defaults (checkbox, picklist, auto-number) apply only on create.
- **Read and query projection** serialize each type in its wire form. Compound fields are computed from components on read: an Address returns `{"street","city","state","postalCode","country"}` with nulls for empty components, or null when every component is empty. Contact.Name is a stored generated column (`FirstName` and `LastName` joined by one space, trimmed), so it can be filtered and ordered.
- **Bounded SOQL** (today's grammar) can project every active type, `ORDER BY` any sortable type with typed ordering and current null placement, and filter by equality on string-like types (Text, TextArea, Picklist, Email, Phone, URL, reference, Id, auto-number). Long text area, multi-select and compound fields are not filterable or sortable, matching describe. Equality on numeric, date, datetime and checkbox fields needs typed SOQL literals, which the current lexer does not support. It returns `MALFORMED_QUERY` with a message that the filter is not yet supported, recorded as a gap closed by Plan 29. Column metadata reports each field's type.
- **Composite** inherits everything through the shared record service. Cascade deletes and auto-number allocation take part in its single transaction and roll back with it.
- **External-ID upsert** remains restricted to Account.Compat_Key__c.

OpenAPI: extend `Field` with the new properties and type enum, document value wire forms per type, document compound read-only fields and the new error codes with `x-source` provenance, and extend traffic tests to cover a request and response for every active type and every new error.

## 5. Standard fields added to Account and Contact

Fields already present stay exactly as they are. Added (*candidate selection* of fields common in Sales Cloud orgs, using documented API names, types and lengths):

- **Account:** `Phone` (Phone), `Fax` (Phone), `Website` (Url), `Type` (Picklist), `Industry` (Picklist), `AnnualRevenue` (Currency 18,0), `NumberOfEmployees` (Number 8,0, describe type `int`), `Description` (LongTextArea 32,000), `BillingStreet` (TextArea), `BillingCity` (40), `BillingState` (80), `BillingPostalCode` (20), `BillingCountry` (80), `BillingAddress` (compound), and the same five components plus compound for `Shipping`.
- **Contact:** `Salutation` (Picklist), `Name` (compound, read-only, 121), `Title` (128), `Department` (80), `Email` (Email), `Phone`, `MobilePhone` (Phone), `Birthdate` (Date), `Description` (LongTextArea 32,000), and `Mailing` address components plus `MailingAddress`.

Standard picklists are unrestricted and seeded with the documented default values (Type, Industry, Salutation), cited with `x-source`. State and country picklists, geocode components (latitude, longitude, accuracy) and `OtherAddress` are left out and recorded as gaps. Name formatting by locale (family name first) waits for user locale in Plan 33. The existing restricted-role policy covers only Compat_Text__c, so the new standard fields are visible to both roles.

## 6. Stored, not enforced

Formula, Roll-Up Summary, Encrypted Text, Geolocation and Rich Text Area definitions are accepted only through `--definition`. The complete source definition (including formula text, roll-up spec, mask settings) is retained in `sc_catalog.artifacts` with `active=false`, linked to its object and its intended API name. Such definitions:

- create no physical column and do not appear in describe, record JSON, query, Composite or OpenAPI field lists;
- reserve their API name, so an active field cannot later take it silently;
- appear in `schema inspect` with `"status": "stored-not-enforced"` and their type;
- appear in the connected editor under a localized "Not active in SalesCompel" note on the object, listing label, API name and type;
- are documented on the compatibility page and, later, named by the Plan 44 dry-run report.

This follows the roadmap rule that a formula's definition is retained but copied values are never served as live results. It goes one step further and keeps these fields out of describe entirely, so no client mistakes a null for a computed value (*candidate*). Rich Text Area is in this group because it needs an HTML sanitizer and a safe rendering policy that this plan does not design (*candidate*).

## 7. Connected editor

Describe drives one control per type, with localized product copy and tenant-supplied labels:

- Text, Text Area and Long Text Area: input or textarea with maxlength.
- Number, Currency and Percent: a text input with `inputmode="decimal"` that validates against precision and scale as decimal text, never `Number()`. Display uses `Intl.NumberFormat` with the UI locale, the tenant currency for Currency, and a % suffix for Percent; the edited value keeps the canonical API form.
- Date and Date/Time: native date and datetime-local inputs. Date/Time is shown in the browser's time zone and sent as UTC. User time zones arrive with Plan 33, and the screen says which zone it shows.
- Checkbox: checkbox.
- Picklist: select with a "none" option unless required.
- Multi-select: a checkbox group in definition order.
- Email, Phone and URL: typed inputs. URLs are shown as text, never as active links (*candidate*, to avoid `javascript:` URLs).
- Lookup and master-detail: a select filled from a bounded query of the target object (first 200 records by Name), showing Name and ID, with a manual ID entry fallback.
- Auto number and compound fields: read-only display. Address components are grouped under their compound label.

Contact titles use Name. Validation messages mirror the API errors and come from the catalogs. RTL layout and Hebrew are covered as in Plan 27. List columns keep their current behavior; each new type is displayed in its formatted form there.

## 8. Evidence, acceptance and completion

**Task 1.** Review Salesforce release notes since 2026-11-05 for field-type or describe changes. If an authorized trial-org session exists, observe describe and REST write/read for one field of each active type, including: numeric rounding and range errors, the string-for-number coercion, datetime precision and offset handling, checkbox null, email validation, multi-select normalization, restricted-picklist errors, lookup delete behaviors and compound read shapes. Record each *candidate* above as observed, documented or still a gap in `docs/compat/plan-28-field-types.md`. Missing access never blocks implementation.

**Unit and database tests:** a decimal helper (parse, round-half-up, range, formatting, no float path); per-type validation tables; every error code; RLS on new tables; FK tenant scoping, including a cross-tenant reference rejected at the database with the restricted role; SetNull/Restrict/cascade; auto-number concurrency (parallel inserts produce unique, increasing values); upgrade from a populated Plan 27 database; migrated equals seeded catalog.

**Acceptance (extend the `schema` gate and the JSforce/Python gates):** through the operator CLI, create a custom object with an auto-number Name, one field of every active type on it, a Lookup and a Master-Detail to a second custom object, and one field of each stored type via `--definition`. Then:

- **JSforce:** describe matches the expected properties. Create, read, update and delete records using every type, including boundary values (max precision, rounding, min/max date, multi-select ordering, restricted rejection) and each error. Delete a master record and verify the cascade, and a restricted lookup parent and verify DELETE_FAILED. Composite rolls back an auto-number allocation and a cascade. The new Account/Contact standard fields and compound reads work.
- **simple-salesforce:** describe, create, read, update and delete on the same object, including Decimal values sent as JSON numbers and dates, verifying exact returned values.
- **Restart:** a record of every type and the stored definitions survive with identical values.
- **Browser:** edit each type in the connected editor, the stored-not-enforced note, currency formatting in two locales, and Hebrew RTL.
- **Raw HTTP traffic validation:** covers every new wire form and error.

Every resource is journaled and cleaned up as in Plan 27.

**Complete when:** full acceptance passes with no required skips, Go race and vet, web unit tests and build, and the offline lab check pass; the compatibility record lists every candidate and gap; the operator guide documents the new CLI flags, currency setting and upgrade; and an independent review finds nothing outstanding.

## Sources

Not fetched during drafting; Task 1 verifies them and records any page that cannot be retrieved.

- [Field types (Object Reference)](https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/field_types.htm) and [primitive data types (SOAP API)](https://developer.salesforce.com/docs/atlas.en-us.api.meta/api/primitive_data_types.htm): describe types, value forms, date range.
- [Compound fields](https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/compound_fields.htm): Name and Address read-only shape.
- [Custom field types](https://help.salesforce.com/s/articleView?id=sf.custom_field_types.htm&type=5): lengths, precision, picklist and auto-number limits.
- [Object relationships](https://help.salesforce.com/s/articleView?id=sf.overview_of_custom_object_relationships.htm&type=5): lookup delete options and master-detail rules.
- [DescribeSObjectResult Field](https://developer.salesforce.com/docs/platform/api/guide/sforce-api-calls-describesobjects-describesobjectresult.html): describe properties.
- [PostgreSQL 17 CREATE TABLE](https://www.postgresql.org/docs/17/sql-createtable.html): `ON DELETE SET NULL (column)`.

## Candidate calls for review

These are the author's calls, not maintainer decisions:

1. Adding the §5 standard fields to Account/Contact in this plan, since compound Name and Address only exist on standard objects.
2. Text Area and Long Text Area included as active, Rich Text Area as stored-not-enforced.
3. Stored-not-enforced fields kept out of describe entirely rather than described with null values.
4. Contact.AccountId keeps today's restrict-on-delete instead of Salesforce's cascade until Plan 30.
5. No typed WHERE literals until Plan 29; projection and ordering work now.
6. No picklist value edits after creation; master-detail only on empty child objects; required new fields only on empty objects.
7. Auto-number backfill in Id order.
8. The value rules marked *candidate* in §2 (string-to-number coercion, round-half-up, seconds precision, checkbox null, email grammar), each to be observed or recorded as a gap.
9. A tenant corporate currency for display only.

The builder may choose helper and package names, the auto-number mechanism (within §3's guarantees), visual arrangement of controls, and test structure. Changing scope, storage strategy, error codes or the stored-not-enforced boundary requires updating this spec and the plan together.
