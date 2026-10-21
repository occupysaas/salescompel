# Plan 16 — CLI query column metadata contract and evidence

Status: implemented, reviewed and verified with two full acceptance passes, 2026-10-21 UTC.

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

Implementation, behavioral tests, fresh review corrections and two full acceptance runs are complete; see the final record below. Reproduction raw logs, CLI auth, tokens and connection strings remain private. Provisioned acceptance principals intentionally remain only in the disposable database until its owned teardown.

Development verification: the original HTTP route failed the new complete-body oracle on missing metadata; after integration the focused cases passed. `go test -race -count=1 ./...` passed with disposable PostgreSQL and `go vet ./...` passed. The shared OpenAPI QueryResult also describes broader query/cursor surfaces; it remains unchanged because adding this undocumented bounded extension there would imply support outside the selected candidate path. The exact candidate wire contract lives here and in the oracle.

## Final implementation and acceptance outcome

The bounded candidate query contract is implemented and verified. Two sequential full runs passed every required gate against the same agent-owned disposable PostgreSQL 17 database. Tested clean code revision: `807c937983311383f947f14bf636dd6eeba17e70`. Documentation-only commits cite that revision without claiming to retest newer runtime code. No Salesforce reference requests were made.

The native Account query remains unchanged. Nonempty Contact Id/LastName/AccountId and an independent empty query are now required. Metadata is resolved from the authorized catalog projection after successful Select. Ordinary responses and existing nested relationship records retain their prior shapes.

A candidate-only diagnostic first exposed a preexisting restart publication collision: the seed fixture and step result both used restart.json. Separate `<gate>-result.json` names preserve the seed, exclusive publication and bound-result validation. A regression failed with EEXIST on the old path and passed after the correction. A fresh whole-change review found one Important issue: URL.Query silently dropped malformed pairs. Three HTTP regressions failed with 200 instead of 400, then passed after raw query parsing errors were handled. No Critical or Minor findings remained. The final Go race suite and vet passed; the full runs below also execute the complete lab checks.

The original 32-case oracle was expanded to 35 cases for malformed escaping, malformed duplicate and semicolon options. Those errors return the frozen candidate INVALID_QUERY_PARAMETER response. No dependency pins or catalog/schema version changed.

Commands (DB URLs supplied privately and identically):

```sh
CANDIDATE_DISPOSABLE=yes CHROMIUM_PATH=/usr/bin/chromium \
  scripts/check-candidate-acceptance.sh --out /tmp/salescompel-plan16-full-a.json
# Only after A verified fixture cleanup and stopped its owned processes:
CANDIDATE_DISPOSABLE=yes CHROMIUM_PATH=/usr/bin/chromium \
  scripts/check-candidate-acceptance.sh --out /tmp/salescompel-plan16-full-b.json
```

| Run | UTC interval | Outcome / exit | Run ID | Summary SHA-256 |
| --- | --- | --- | --- | --- |
| A | 2026-10-21T01:45:34.322Z → 2026-10-21T01:47:33.567Z | passed / 0; eligible | `750e3d9a38d575682ee5c753` | `276365e6fd745510744315344221cfea877045c1927bf8d1d8ea331ca0c5d449` |
| B | 2026-10-21T03:20:18.125Z → 2026-10-21T03:23:15.887Z | passed / 0; eligible | `e544cacf5905bca8cb287b54` | `5de5c667f7d14eb4b4eb6c5344e9bd3390fa824d093b73a4ee8b045a5ba33172` |

Both reports validate against the existing strict version-1 acceptance schema. Source, tool, lock/catalog, executable and web asset bindings match across the pair. Each has verified fixture disposition, zero unknown intents and stopped owned processes. The 21 journaled operations per run are not a distinct-row count. Provisioned principal digests intentionally remain in the disposable database until its owned teardown.

| Gate | A passed / failed / skipped | B passed / failed / skipped |
| --- | --- | --- |
| bindings | 3 / 0 / 0 | 3 / 0 / 0 |
| database | 247 / 0 / 0 | 247 / 0 / 0 |
| lab | 261 / 0 / 3 | 261 / 0 / 3 |
| web | 6 / 0 / 0 | 6 / 0 / 0 |
| jsforce | 3 / 0 / 0 | 3 / 0 / 0 |
| python | 4 / 0 / 0 | 4 / 0 / 0 |
| cli | 9 / 0 / 0 | 9 / 0 / 0 |
| restart | 3 / 0 / 0 | 3 / 0 / 0 |
| connected-browser | 8 / 0 / 0 | 8 / 0 / 0 |
| preview | 11 / 0 / 0 | 11 / 0 / 0 |
| cleanup | 2 / 0 / 0 | 2 / 0 / 0 |

The three lab skips are the permitted offline candidate/jsforce/core, candidate/jsforce/policy and candidate/jsforce/isolation tests; all three execute in the mandatory real JSforce gate. All applicable database tests ran with zero skips. Web counts include five unit tests and the build. The separate preview suite executes with no backend requests.

Actual local versions: jsforce `3.11.0`, @salesforce/cli `2.156.0`, @salesforce/plugin-data `5.1.16`, @salesforce/plugin-deploy-retrieve `4.2.15`, simple-salesforce `1.12.9`, go `1.27.1`, node `26.8.1`, python `3.14.8`, openssl `3.6.5`, chromium `154.0.8037`, postgres `17`.

| Binding | SHA-256 in both reports |
| --- | --- |
| source | `04cc6d3ce677f15065e148e24238575447dfd088ee54f3f38dc56b896042d9ca` |
| binary | `f37396645045746f247a5db6b0ccac0e27efe025c94a5323114a4e9fb002e245` |
| assets | `826598e273f8ae6c98fc83498a3549d2885774c5dc862daf7581d4007e90f360` |
| `go.mod` | `21febed5445c488750c478d48a4071e6f585ad23229e69ba34771d989ee0dd79` |
| `go.sum` | `a5e7a8db07eba28dbde48bf9a54c4e9084a5a95ded581c0ff44c08b74e3dfe69` |
| `web/package-lock.json` | `5ebbc26f2edc88604d838d4b208a1b10e9344d3c512f0159fd24a4d1c32e8ad1` |
| `compat/clients/node/package-lock.json` | `28dbb3a1056e02fcc64db82ac3719d96a185ad8aa2a1448170bf9639ac277a00` |
| `compat/clients/python/requirements.lock` | `1f8a4ab240070fd51587800067da66ef45a035ea2f5a3e34880a4649ac274327` |
| `internal/platform/catalog.go` | `d91624bc8ba2d5912a49732926636cc7b6e814a238d3e8e9b5ba2a546ab6d01b` |
| `internal/platform/metadata.go` | `a5b3c3aacc9788c225d52354be2c4572060528e1457f22b75afb137ae08f5fa6` |
| `internal/platform/catalog_migration.go` | `0b601b25731031eb1280b6edb48cb1eda71b4d0212bfa6b78d49531f7076eb01` |
| `internal/platform/catalog_v1.json` | `c4d7424d2ee74f09842ff0bdbc8028c021f5c2acf4d4b909ede5c532d71682ad` |
| `internal/platform/schema.sql` | `77afadf3c34eecf3b83a5c07e19d70d2e5961bbc1f4e4c4315d6bcec87b9318c` |

Sanitized summaries remain at the two /tmp paths above; raw logs, fixture journals, connection strings, auth state and browser artifacts remain private. CI is configured to run one full invocation and upload the sanitized report; no actual GitHub Actions run was observed.

## Calls and remaining limits

Execution used main as approved by the plan/repository convention; there is no isolated feature branch. The metadata contract intentionally emits only the pinned nonaggregate consumer requirements; extra reference properties and extension parity remain unmeasured. Private CLI/restart step result paths now end in -result.json, so local diagnostic readers must use those names. Broader query syntax, pagination and authorization redesign were excluded to preserve the existing bounded surface. The review did not waive acceptance or final evidence: both required full passes are now recorded above.

These passes establish the tested candidate walking slice and close Plan 15’s consolidated local-run blocker. They do not close the full M1/M2 exit gates, Plan 10/14 reference work or Plan 13 interactive visual acceptance. Salesforce compatibility certification remains unclaimed.

Local teardown: the agent-owned `salescompel-plan16-db` PostgreSQL container was stopped and removed after both passing runs and independent report/pair validation. No operator-supplied database or unrelated container was changed. Earlier blocked/failed diagnostics remain separate and do not count toward the final pair.
