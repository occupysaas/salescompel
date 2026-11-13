# Plan 28 — Field types decisions and execution record

Status: implemented, 2026-11-11; no Salesforce compatibility certification.

## Reference review (2026-11-11)

No authorized trial-org session was supplied, so nothing here is a live
observation. Automated fetches of developer.salesforce.com reference pages
returned HTTP 403 during this review. The pages cited in the
[Plan 28 spec](../superpowers/specs/2026-11-11-plan-28-field-types-design.md)
remain unverified unless a row below names a source reached through search
results. No client or plugin implementation code was inspected.

Release-note review: the Winter '27 notes reviewed for Plan 27 remain the
latest general release. No field-type, describe or REST value change after
2026-11-05 was found in accessible sources.

## Candidate behavior status

`documented` means a public Salesforce source states the behavior.
`candidate` means the spec's default, with no confirming source.
`gap` means a known or likely difference that is recorded and not implemented.

| Item | Candidate rule | Status | Source / note |
| --- | --- | --- | --- |
| Date range | 1700-01-01 to 4000-12-31 | documented | [Data Loader date formats](https://developer.salesforce.com/docs/atlas.en-us.264.0.dataLoader.meta/dataLoader/data_loader_dates.htm) (via search): earliest 1700-01-01T00:00:00Z, latest 4000-12-31T00:00:00Z GMT |
| DateTime input | RFC 3339 with `Z` or offset, optional milliseconds | documented | [Valid date and dateTime formats](https://developer.salesforce.com/docs/atlas.en-us.api_rest.meta/api_rest/intro_valid_date_formats.htm) (via search): `yyyy-MM-ddTHH:mm:ss.SSS+/-HH:mm` or `…SSSZ`; older docs show `+/-HHmm` |
| DateTime without offset read as UTC | accept | candidate | not documented |
| DateTime output `…ss.000+0000` | fixed format | candidate | matches commonly reported REST output; not confirmed |
| DateTime precision | whole seconds | candidate | storage precision is not documented in accessible sources |
| Numeric rounding | round half-up to the field scale | **disputed** | Community reports conflict: some say API writes keep extra decimals beyond the declared scale ([Fivetran](https://support.fivetran.com/hc/en-us/community/posts/21755243528727-Connector-Improvement-Cast-Salesforce-Decimals-to-Doubles-to-Maintain-Precision), [Nintex](https://community.nintex.com/skuid-nlx-sfx-exp-89/users-can-enter-more-decimal-places-than-the-ui-displays-in-read-mode-68704)). Apex `Decimal.round()` defaults to half-even. The candidate keeps exact storage at declared scale; observe on the trial org before claiming parity. |
| Numeric range error | `NUMBER_OUTSIDE_VALID_RANGE` | candidate | error name in common use; not confirmed here |
| Numeric JSON string accepted | `"12.5"` accepted | candidate | not documented |
| Checkbox explicit null | stored false | candidate | not documented |
| Email grammar | one `@`, dotted domain, no spaces, ≤80 | candidate | exact Salesforce grammar not documented |
| Multi-select normalization | split, trim, dedupe, definition order | candidate | not documented |
| Restricted picklist error | `INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST` | candidate | error name in common use |
| Lookup delete options | clear value or prevent deletion | candidate | help article unverified (403) |
| Address REST shape | `street, city, state, postalCode, country` (+ geocodes omitted) | candidate / gap | geocode keys omitted by design |
| Contact.Name | `FirstName LastName` | candidate / gap | locale-dependent order waits for Plan 33 |
| Auto-number backfill | ascending Id order | candidate | Salesforce offers backfill as an option; order undocumented |
| Required on new field | only for empty objects | candidate | candidate restriction |
| Stored-not-enforced fields absent from describe | yes | candidate | design choice (spec Sec. 6) |

## Gaps carried by this plan

- Typed WHERE literals for numeric, date, datetime and checkbox filters (Plan 29).
- Account deletion cascading to Contacts (Plan 30); Contact.AccountId keeps restrict.
- Picklist value edits, field edits and deletion (Plan 35).
- State and country picklists, address geocodes, `OtherAddress`.
- Name order by user locale (Plan 33).
- Time fields; multi-currency.

## Error codes

Each code below is returned with the offending field name in `fields`.

| Code | When | Status |
| --- | --- | --- |
| `NUMBER_OUTSIDE_VALID_RANGE` | Rounded value has more integer digits than precision − scale | candidate (name in common use) |
| `JSON_PARSER_ERROR` | Wrong JSON kind, unparsable or out-of-range date/date-time, exponent or non-decimal number | candidate |
| `INVALID_EMAIL_ADDRESS` | Email fails the candidate grammar | candidate |
| `INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST` | Value not an active API value of a restricted picklist (case-sensitive) | candidate |
| `INVALID_FIELD_FOR_INSERT_UPDATE` | Write to auto-number, compound, Contact.Name or non-reparentable master-detail | candidate |
| `REQUIRED_FIELD_MISSING` | Master-detail or required field omitted or null | existing |
| `MALFORMED_ID` / `INVALID_CROSS_REFERENCE_KEY` | Malformed reference, or a well-formed ID that does not exist in this tenant | existing |
| `DELETE_FAILED` | Deleting a record still referenced by a `Restrict` lookup | candidate |
| `FIELD_INTEGRITY_EXCEPTION` | More than 100 multi-select values | candidate |
| `MALFORMED_QUERY` | Equality on numeric, date, date/time or checkbox fields (typed SOQL literals arrive in Plan 29) | gap |

The closed OpenAPI schema subset cannot carry per-code `x-source`; this table
is the provenance record.

## Operator guide

Upgrade: stop old servers, take a complete backup, run `salescompel migrate up`
(migration 4, "field types"), then start the new server. Migration 4 adds
standard Account/Contact columns and catalog rows, tenant settings with `USD`,
auto-number counters and stored-definition columns. It rewrites existing field
definitions to the typed shape without changing any record value. Rollback is
restoring the backup.

Create typed fields with the owner credential (`MIGRATION_DATABASE_URL`):

```sh
salescompel schema field create --tenant T --object Deal__c --name Amount__c --label Amount --type Currency --precision 16 --scale 2
salescompel schema field create --tenant T --object Deal__c --name Close__c --label 'Close date' --type Date
salescompel schema field create --tenant T --object Deal__c --name Stage__c --label Stage --type Picklist --values New,Won,Lost --restricted --default New
salescompel schema field create --tenant T --object Deal__c --name Account__c --label Account --type Lookup --reference-to Account --delete-behavior Restrict
salescompel schema field create --tenant T --object Line__c --name Deal__c --label Deal --type MasterDetail --reference-to Deal__c
salescompel schema object create --tenant T --name Ticket__c --label Ticket --plural-label Tickets --name-type AutoNumber --display-format 'T-{0000}'
salescompel schema currency set --tenant T --code EUR
```

Types: `Text` (`--length` 1–255), `TextArea`, `LongTextArea` (`--length`
256–131072, default 32768, `--visible-lines`), `Number`/`Currency`/`Percent`
(`--precision` 1–18, `--scale`; defaults 18,0 and Currency 18,2), `Date`,
`DateTime`, `Checkbox` (`--default true|false`), `Picklist` and
`MultiselectPicklist` (`--values`, `--restricted`, `--default`), `Email`,
`Phone`, `Url`, `Lookup` (`--reference-to`, `--delete-behavior SetNull|Restrict`,
`--relationship-name`), `MasterDetail` (`--reference-to`, `--reparentable`;
custom child objects without records only, at most two) and `AutoNumber`
(`--display-format`, `--starting-number`, default 1; existing records are
numbered in Id order). `--required` applies only to custom objects without records; required lookups use `Restrict`. Required and AutoNumber fields are refused on Account and Contact.
Flags that do not apply to a type are rejected. `provision` accepts
`SALESCOMPEL_CURRENCY` for the tenant's display currency.

`--definition FILE` takes the same parameters as JSON, including picklist
labels:

```json
{"name": "Stage__c", "label": "Stage", "type": "Picklist", "restricted": true,
 "values": [{"value": "new", "label": "New", "default": true}, {"value": "won", "label": "Won"}]}
```

Formula, RollupSummary, EncryptedText, Geolocation and RichTextArea definitions
are accepted only through `--definition`. They are retained exactly as given,
reserve their API name, create no column and are never described or served.
`schema inspect` lists them with `"status": "stored-not-enforced"`, the
connected editor shows them under "Not active in SalesCompel", and the
candidate-only `GET /services/data/v65.0/candidate/sobjects/{object}/storedFields`
lists them.

Field definitions cannot yet be edited, renamed or deleted, and picklist values
cannot be added after creation (Plan 35).

## Decisions made during execution

- `Contact.Name` describes length 511 and is an unbounded generated column,
  because FirstName/LastName keep Plan 27's 255-character limits; Salesforce's
  121 would truncate names.
- Migration 4 writes catalog rows per tenant with that tenant's context, so
  forced row security holds when the operator is not a superuser.
- Exponent-form JSON number literals are expanded exactly (Python/JavaScript
  floats such as `1e+16`); exponents inside decimal strings stay rejected.
- Required and AutoNumber fields are refused on Account and Contact, whose
  table is shared by every tenant; a required Lookup always uses `Restrict`.
- The connected editor sends only changed fields on update.
- Query column metadata is unchanged (column name only); IS NULL filters remain
  allowed on every filterable type.
- Lookups in the connected editor are text inputs with record suggestions, so
  an ID can always be typed; numeric inputs show a formatted locale/currency
  preview.
- Field-type acceptance runs inside the `schema` gate as `schema/field-types-*`
  IDs plus `restart/field-types` and three browser IDs, instead of a new gate.
  Records in the Plan 28 fixture objects are removed with their journaled
  tables, because auto-numbered names cannot be journaled before creation.

## Verification

Full acceptance passed against disposable PostgreSQL 17 with Go 1.25.0,
pinned JSforce 3.11.0, Salesforce CLI 2.156.0 and simple-salesforce 1.12.9.
Revision `9d00f08d523328fcc88fd4b67478d984dbde7106` (including the review
fixes) passed the complete entry point from 2026-11-12T03:27:13.912Z to
2026-11-12T03:30:37.842Z. Report SHA-256
`772371b68df9a091311bab86ab600f98451a92911dc13f5ea978e9271adbd5f1`.

Deviation: the pinned Playwright 1.56.1 browser build (Chromium 1194) was not
installed on the verification host, so `CHROMIUM_PATH` pointed at the installed
Chrome for Testing 156.0.8078.4 (Playwright build 1248). All three Playwright
configurations honour that variable.

| Gate | Passed | Failed | Skipped |
| --- | ---: | ---: | ---: |
| bindings | 3 | 0 | 0 |
| database | 612 | 0 | 0 |
| lab | 468 | 0 | 6 |
| web | 11 | 0 | 0 |
| jsforce | 8 | 0 | 0 |
| python | 4 | 0 | 0 |
| cli | 9 | 0 | 0 |
| oauth | 16 | 0 | 0 |
| schema | 16 | 0 | 0 |
| restart | 6 | 0 | 0 |
| connected-browser | 14 | 0 | 0 |
| preview | 11 | 0 | 0 |
| cleanup | 2 | 0 | 0 |

The six lab skips are the established offline parents covered by the live
client gates. Cleanup verified 46 journaled records, zero unknown intents, the
owned schema resources removed and all owned processes stopped. Standalone
`go vet ./...`, `go test -race ./...`, `npm --prefix compat/clients/node run
check`, web unit tests and the web build also passed.

## Review

An independent whole-change review found one critical and three important
issues, all fixed with failing tests first: required and auto-number fields on
the shared Account/Contact table, required lookups defaulting to clear-on-delete,
the editor rewriting untouched values, and exponent-form JSON numbers. Deferred
minor items: exact editor numbers rely on `JSON.parse` source text (older
Safari/Firefox fall back to doubles); migration 4 ordinals may tie with custom
fields already on Account/Contact; Composite creates on two auto-numbered
objects in opposite order can deadlock (500); SOQL equality on picklist and
auto-number is case-sensitive; a required number or date given `""` reports
`JSON_PARSER_ERROR`; `auto_numbers.field_id` is not a composite tenant key.
This evidence-only commit does not change the tested source.
