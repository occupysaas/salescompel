# Plan 25 — Foundation decisions

Implemented 2026-11-04. This is the current decision record for candidate behavior. It closes the evidence waits in Plans 5, 10, 12 and 14 by selecting defaults. It does not promote their historical captures, change `not_run` contract status, or claim Salesforce parity. The [design](../superpowers/specs/2026-11-04-plan-25-foundation-decisions-design.md) is the source of exact rules and evidence labels.

| Contract ID | Decision and basis | Implementation / verification | Owner |
| --- | --- | --- | --- |
| `query-response-shape` | Existing `totalSize`, `done`, `records`; default page size 2,000 and all-page total are Provisional. Candidate rejects unpaged overflow until pagination ships. | `select.go`; `TestHTTPQueryColumns`; future paging gate | 25 bounded response; 29 paging |
| `query-limit-above-batch` | LIMIT above 2,000 is valid and paged (Provisional); keep truthful rejection until implemented. | `query.go`; `TestRejectUnsupportedQuery`; future paging gate | 29 |
| `id-15-18` | Accept case-sensitive 15-character bases and 18-character IDs (Documented); suffix case-insensitive (Default). Responses use 18. Malformed record IDs: 404 NOT_FOUND; malformed filter IDs: INVALID_QUERY_FILTER_OPERATOR; malformed references: MALFORMED_ID with fields (Default). | `ids.go`, `store.go`, `query_normalize.go`; `TestNormalizeRecordID`, `TestHTTPRecordIDForms`, `TestStoreQueryIDFilters` | 25 |
| `case-insensitive-names` | REST paths, SOQL and write fields resolve case-insensitively, with canonical response keys (Documented). | `names.go`, `http.go`, `query_normalize.go`; `TestHTTPCaseInsensitiveRoutes`, `TestStoreQueryCaseInsensitiveNames` | 25 |
| `case-insensitive-comparison` | Simple Unicode case-insensitive text equality without normalization (Provisional). Composed and decomposed accents stay distinct. | `select.go`, `pg_c_utf8`; `TestStoreQueryTextEquality` | 25 |
| `null-ordering` | Nulls first for ASC and DESC (Provisional); explicit NULLS FIRST/LAST (Documented). Stable database Id tie-break (Default). Blank optional text reads null (Provisional). | `query.go`, `select.go`; `TestParseQueryNullPlacement`, `TestStoreQueryNullOrdering`, `TestStoreBlankNillableStringsAreNull` | 25 |
| `error-shape` | Array of errorCode/message objects; field-specific errors include fields (Documented). Messages omit values and database details. | `catalog.go`, `http.go`, `store.go`; `TestHTTPFieldErrorEnvelope` | 25 |
| `unknown-sobject` | REST: 404 NOT_FOUND (Default); SOQL FROM: 400 INVALID_TYPE (Documented). | `http.go`, `query_normalize.go`; `TestHTTPUnknownObjects` | 25 |
| `query-batch-header` | Honor Sforce-Query-Options batchSize (Provisional); document bounds in Plan 29 (Default). Current bounded candidate has no pagination. | Future parser/paging; full behavior deferred | 29 |
| `query-locator-lifecycle` | Two-day lifetime under existing named maintainer assumption (Documented; boundary unknown). Same-user replay/concurrent retrieval across sessions allowed; another user's locator: 400 INVALID_QUERY_LOCATOR. Membership fixed at first page, later values current (Provisional; deletion rules Default). | Future locators and paging gate; no fabricated runtime support | 29 |
| `rest-route-parity` | Existing bounded discovery, CRUD, query and upsert routes retained; object names case-insensitive (Documented), unknown objects 404 (Default). Composite URL forms unchanged. | `http.go`, `api/candidate-openapi.yaml`; `TestHTTPBoundary`, `TestHTTPCaseInsensitiveRoutes`, `candidate/openapi/traffic` | 25 bounded routes; 29 locators |
| `duplicate-canonical-fields` | REST JSON duplicates, exact or case variants: last value wins (Provisional). Decoded Composite case variants reject with INVALID_FIELD (Default). Authorization applies after canonical resolution. | `http.go`, `names.go`; `TestHTTPDuplicateBodyFields`, `TestCompositeRejectsCaseDifferingFields` | 25 |
| `upsert-outcomes` | Tenant-scoped lookup and uniqueness use trim-nfc-full-fold-v1. Preserve supplied spelling on create and original spelling on upsert update (Observed; spaces on creation Default). Body-key rules remain Plan 11 (Observed). Serialize missing-key races (Default), giving one creation and subsequent successes. POST races may lose with DUPLICATE_VALUE. | `externalid.go`, `store.go`, catalog v3; `TestCanonicalExternalIDVectors`, `TestUpsertPlan9Sequences`, `TestUpsertConcurrentVariants`, `TestUpsertRacingPost`, `candidate/foundation/decisions` | 25 |

Code paths above are under `internal/platform/`, unless stated otherwise. External-ID equivalence removes only leading/trailing U+0020 spaces, applies NFC, locale-independent full case folding, then NFC again. Tabs, NBSP and interior spaces remain significant. URL decoding happens once; percent signs remain literal. Lookup follows the nine observed Plan 12 relationships. Uniqueness extends that rule by Default beyond the observed ASCII cases. POST/PATCH whitespace-only keys become null; whitespace-only upsert URLs reject with INVALID_FIELD (Default). Ordinary PATCH may change the same record's spelling (Default). Supplied spelling still has an 80-character limit even though its canonical value may exceed 80 characters.

## Plan 14 gaps

All six dimensions remain unobserved in Plan 14. Selecting a default does not fill an evidence gap.

| Dimension | Gap and selected default |
| --- | --- |
| Whitespace lookup in both directions | Gap: trim U+0020 on lookup in both directions; extends Plan 9's base-to-spaced observation. |
| POST creation spelling | Gap: keep supplied spelling, including spaces; blank canonical keys become null. |
| Missing-key upsert creation/replay spelling | Gap: first writer's spelling stays; equivalent replays update without changing it. |
| Ordinary POST uniqueness for all finite pairs | Gap: one canonical equivalence for all pairs; conflicting insert returns DUPLICATE_VALUE. |
| Other-record update uniqueness | Gap: same canonical uniqueness; conflicting update returns DUPLICATE_VALUE without changing records. |
| Same-record spelling edits | Gap: ordinary PATCH may change spelling; upsert update cannot. |

Other open gaps are Unicode beyond the finite vectors and other locales; dynamic external-ID fields (Plan 27); HTTP statuses where only client codes were observed; exact reference malformed-ID behavior; text ordering collation and reference tie order; text equality where simple mapping differs from Salesforce; Composite URLs with short IDs or noncanonical names; and release-history reconciliation of `winter23-query-cursor-retention` and `release-content-unavailable`. The latter retain their recorded status and move to certification under roadmap rule M1R.

The [Plan 29 decision table](../superpowers/specs/2026-11-04-plan-25-foundation-decisions-design.md#decisions-recorded-for-plan-29) records paging defaults. This delivery does not add locators, batch headers, or LIMIT above 2,000. The experimental `internal/compat/externalid` package remains an experiment.

## Catalog v3 operation

Stop all old instances and back up the database before upgrading. Fresh installs and catalog v1/v2 upgrade inside the existing advisory-locked startup transaction and exclusive table lock. The catalog definition JSON is unchanged. A nullable text column `accounts.compat_key_canonical`, a matching-null check, and UNIQUE(tenant_id, compat_key_canonical) replace the original raw-key uniqueness. Describe remains unchanged.

An upgrade refuses colliding canonical keys within a tenant or whitespace-only keys, leaving the catalog version, schema, and every original row unchanged. It never rewrites or deletes supplied keys. Errors do not print row values. Existing oversized custom fields still require remediation before upgrading.

To investigate a refusal, use a private, access-controlled operator session against the backup or stopped database. Read tenant_id, id and compat_key privately, calculate the documented rule with the same Go normalization helper, and group by tenant plus canonical key. Report only tenant/record identifiers and reason codes to the operator, never raw or canonical values. Find blank groups the same way. PostgreSQL lower/trim is insufficient for NFC and full folding. Resolve rows explicitly using the authorized business key: clear or change the selected duplicate/blank key, record the affected IDs privately, and rerun startup. Do not auto-delete rows or silently select a winner. The existing physical schema checker also refuses drift rather than repairing it.

Rollback means stopping the new instances and restoring the pre-upgrade backup before starting old code. Dropping the new column or decrementing platform_catalog.version is not a supported rollback. These instructions extend the [Plan 8 upgrade guide](plan-8-active-field-metadata.md).

## Verification

Verified runtime revision `dae71c5fd0e5878f872e70799031f2713dfab529` with Go 1.25.0 and disposable PostgreSQL 17. No database tests skipped. The following commands passed:

- `GOTOOLCHAIN=go1.25.0 go test -race ./... -count=1` with TEST_DATABASE_URL set.
- `GOTOOLCHAIN=go1.25.0 go vet ./...`.
- `npm --prefix compat/clients/node run check`: TypeScript, Prettier, tests and offline validation passed. The clean full acceptance reran the final Node suite: 445 passed / 0 failed / 6 skipped. The earlier standalone check had 438 passed / 6 skipped; the final complete suite includes the additional skip-publication regression.
- `npm --prefix web test`: 5 passed / 0 failed / 0 skipped.
- `npm --prefix web run build`: passed.
- `scripts/check-candidate-acceptance.sh --out /tmp/salescompel-plan25/acceptance-browser.json`: exit 0, full, acceptance eligible. `CHROMIUM_PATH=/usr/bin/chromium` and PATH selecting Go 1.25.0 were supplied. The first invocation stopped at the default Chromium prerequisite and did not run product gates; the explicit installed browser resolved it.
- `git diff --check`: passed. The protected-path diff against `7151727` was empty.

The six permitted offline skips are `candidate/openapi/traffic`, `candidate/composite/atomic`, `candidate/foundation/decisions`, `candidate/jsforce/core`, `candidate/jsforce/policy`, and `candidate/jsforce/isolation`. All six passed without skips in the live JSforce gate.

| Gate | Outcome | Passed | Skipped |
| --- | --- | --- | --- |
| bindings | passed | 3 | 0 |
| database | passed | 423 | 0 |
| lab | passed | 445 | 6 |
| web | passed | 6 | 0 |
| jsforce | passed | 6 | 0 |
| python | passed | 4 | 0 |
| cli | passed | 9 | 0 |
| restart | passed | 3 | 0 |
| connected-browser | passed | 8 | 0 |
| preview | passed | 11 | 0 |
| cleanup | passed | 2 | 0 |

Full run `6540bd0ade3b4ed453a723e8`: 2026-11-05T00:35:56.080Z → 2026-11-05T00:38:10.250Z. Report SHA-256: `0c6b97c13a644b03a350db0bd60bd43cd853e7ca16b326ee0ab941edbb2e7dbc`. Cleanup verified 37 journaled operations (not distinct rows), zero unknown intents, and stopped processes; provisioned principals are retained per the harness contract. Private journals, tokens and wire logs are excluded from this record.

A fresh whole-change review found no actionable issues. The specific Plan 11 body-key rejection envelope remains unchanged, including omission of fields, taking precedence over the general field-error default. Documentation-only closure follows the tested runtime revision; the complete final commit is checked again through the same full acceptance entry point, with its local report at `/tmp/salescompel-plan25/final.json`.


The serialization choice deliberately differs from Plan 9's timing-dependent losing-upsert error: equivalent concurrent upserts succeed in sequence while preserving one record. Ordinary POST contention still uses the database uniqueness constraint. No live Salesforce org, credentials, private capture or external communication was used. Historical contracts, profiles, observations, reference OpenAPI, CLEAN_ROOM.md and the experimental external-ID package remain byte-identical to the protected baseline.
