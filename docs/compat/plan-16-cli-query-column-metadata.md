# Plan 16 — CLI query column metadata contract and evidence

Status: candidate interoperability contract frozen; runtime work pending.

## Evidence and decision boundary

Fresh candidate-only reproduction at revision `5b4d35b05305331650e1430157d8c7732890450f`, run `b0ec0326dd425aac6dcb45b2`, failed the existing nonempty `cli/query` (6 other CLI checks passed). JSforce 3/3 and Python 4/4 passed; exact cleanup passed, zero unknown intents, owned processes stopped. This confirms Plan 15's blocker, not a full acceptance run. Summary: `/tmp/salescompel-plan16-repro-2.json`. An earlier attempt was blocked before fixture work because the default Playwright browser path was unavailable; the installed `/usr/bin/chromium` was selected explicitly on retry.

Command: `CANDIDATE_DISPOSABLE=yes CHROMIUM_PATH=/usr/bin/chromium scripts/check-candidate.sh --out /tmp/salescompel-plan16-repro-2.json`, with DATABASE_URL/TEST_DATABASE_URL privately set to the same agent-owned disposable PostgreSQL 17 database. No reference requests or credentials were used. Actual tool pins match Plan 15; Go 1.27.1, Node 26.8.1, Python 3.14.8, Chromium 154.0.8037, OpenSSL 3.6.5.

`pinned-consumer`: installed `compat/clients/node/node_modules/@salesforce/cli/node_modules/@salesforce/plugin-data/lib/commands/data/query.js`, version 5.1.16, SHA-256 `f4555d517b01053ca12016b39aa194fc78055e61f13885c9c011e9f1a5248fea`. Its `runSoqlQuery` requests columns only for nonzero totalSize. `retrieveColumns` makes a second GET of the same query with `columns=true`; it requires `columnMetadata` to be an array. Leaf entries require a string `columnName`. A nonempty `joinColumns` array recursively describes nested names; omitted/false `aggregate` follows ordinary-field handling. No label, type, length, precision or displayName is consumed on this nonaggregate path. This is inspection of the pinned client, not observation of reference wire semantics.

`official-query`: [Salesforce Query reference](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-query.html) and [query example](https://developer.salesforce.com/docs/platform/api-rest/guide/dome-query.html), consulted 2026-10-21 UTC, document the ordinary envelope, q parameter and invalid-query behavior. They do not specify `columns` or a columnMetadata schema. They are current documentation, not an API 65.0 capture. No claim about undocumented Salesforce metadata values follows from them.

`candidate-contract`: the following deliberately bounded candidate decisions satisfy the pinned nonaggregate consumption path. `existing-policy` refers to current parser, `Store.Select`, `Describe` and `field` behavior. No material candidate wire decision remains unresolved; reference parity for this extension remains unmeasured.

## Frozen decision table

| Input/surface | Exact candidate behavior | Basis/status |
| --- | --- | --- |
| Absent columns / exactly `columns=false` | Ordinary `totalSize`, `done`, `records` body, no columnMetadata property | Existing behavior; candidate decision; ordinary envelope documented |
| Exactly `columns=true` | Same envelope and records plus `columnMetadata`: array, including for empty results | Pinned consumer requirement + candidate decision |
| Malformed query-parameter encoding (including percent escapes and unescaped semicolons), after valid SELECT parsing | HTTP 400, `INVALID_QUERY_PARAMETER`; message `Invalid query parameter encoding` | Candidate decision; reject the raw parse error rather than using silently dropped values |
| Empty, other spelling/value, repeated columns (even identical values) | HTTP 400, `INVALID_QUERY_PARAMETER`; message `columns must be specified once as true or false` | Candidate decision; not reference error semantics |
| Scalar selected field | `{ "columnName": "<exact catalog field Name>" }` | Minimal pinned consumption requirement + candidate decision |
| ID, string, reference and custom text fields | Same scalar shape; no type/label/length/precision/aggregate/displayName or empty joinColumns properties | Candidate decision; omitted unused values avoid invented Salesforce mappings |
| Field family/catalog | Account Id, Name, Compat_Text__c, Compat_Key__c; Contact Id, FirstName, LastName, AccountId | Existing catalog/policy, custom storage lengths remain 80 |
| Ordering and duplicates | One metadata entry per projection in input order, including repeated selected fields; ordinary records retain existing map serialization | Candidate decision preserving existing parser/Select behavior |
| Canonical spelling | Existing exact-case object/field resolution; no new case normalization | Existing-policy |
| Contact.Account.Name | `{ "columnName": "Account", "joinColumns": [{ "columnName": "Name" }] }` at that projection position; existing nested Account record shape unchanged | Pinned recursive join consumer + candidate decision preserving existing traversal |
| Null or absent relationship, empty result, all-null field | Metadata derived from authorized catalog projection, independent of rows/values | Candidate decision |
| Restricted Account.Compat_Text__c in SELECT, WHERE or ORDER | Existing HTTP 400 INVALID_FIELD, no successful envelope or metadata | Existing-policy |
| Unsupported object/field/traversal or syntax | Existing INVALID_TYPE/INVALID_FIELD/MALFORMED_QUERY response; no syntax expansion | Existing-policy |
| Another tenant | No matching record, even when an Id from the first tenant is known; readable metadata is schema only | Existing-policy |
| Invalid query and invalid option together | Parse query first; valid SELECT then option validation then Select; preserve query parse error precedence | Candidate decision |
| Endpoint beyond query | No columns interpretation or new support | Candidate scope |

## Oracle and seed context

The committed `internal/platform/testdata/query-columns-contract.json` is a hand-authored synthetic oracle; expected values were constructed before the metadata implementation. It contains complete decoded bodies so absence/null/empty array differ. Test-only placeholders are `$accountId`, `$contactId`, `$nullContactId`, `$name`.

The admin tenant contains one Account named `$name`, Compat_Key__c `owned-key`, Compat_Text__c null; one Contact LastName `Contact`, FirstName null linked to it; one Contact LastName `Unlinked` without an Account. Restricted principal shares this tenant. The other tenant has an Account with the same Name but a distinct Id. Test lookup by `$accountId` must not return it. Each test context starts with fresh principals/tenant rows and is removed by the existing test cleanup. `nested`/`nested-null` cases retain the current `attributes` shape and use Account.Name explicitly. Error prose is pinned to candidate behavior, never advertised as vendor prose.

## Verification and acceptance

Pending implementation, behavioral tests, fresh review and two full acceptance runs. Reproduction raw logs, CLI auth, tokens and connection strings remain private. Provisioned acceptance principals intentionally remain only in the disposable database until its owned teardown.

Development verification: the original HTTP route failed the new complete-body oracle on missing metadata; after integration the focused cases passed. `go test -race -count=1 ./...` passed with disposable PostgreSQL and `go vet ./...` passed. The shared OpenAPI QueryResult also describes broader query/cursor surfaces; it remains unchanged because adding this undocumented bounded extension there would imply support outside the selected candidate path. The exact candidate wire contract lives here and in the oracle.
