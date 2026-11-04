# Plan 23 — Served candidate OpenAPI contract

Status: implemented, independently reviewed, corrected and verified, 2026-11-04.
All eleven full candidate acceptance gates passed on the pinned Go 1.25.0 toolchain.
No Salesforce parity certification or milestone closure is claimed.

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

## Implementation and initial verification

The authenticated route and immutable embedded bytes pass two real Go tests against
an owned disposable PostgreSQL 17 database, with zero skips. Nine offline document
and traffic tests pass, along with typecheck. The acceptance-wiring tests pass;
the new candidate/openapi/traffic parent also passes against a real owned HTTPS
candidate and asserts exact coverage of all nineteen fixed operationIds. This
focused development run reused the existing preflight/service/fixture helpers.

The offline lab check passed: 422 test/subtest events, 418 passed, four opt-in
candidate tests skipped, zero failed. Typecheck, formatting and offline validation
passed, including eleven reviewed historical pilot observations/not_run=0. These
are not new Salesforce observations or parity certification. Final acceptance and
review corrections are recorded below.

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
Private first-attempt diagnostics are retained; subsequent complete acceptance
results are recorded below.

The next corrected full run passed all lab tests but correctly failed the strict
coverage gate: the old result adapter flattened repeated nested Plan 22 leaf names
into duplicate identities. Its published summary records lab failure and verified
cleanup, not acceptance. A new adapter regression failed before correction;
lineage-aware IDs now preserve parent names and ordinals while duplicate results
still fail coverage. Current Node parentId/testId fields handle interleaved groups;
older event streams use declaration-ordered parent closure, verified with the
existing local Node 22.23.3 image (34 real terminal events, all passing/unique).
The actual previously rejected 431-event lab output now has 431 unique IDs and
passes unchanged coverage rules. No historical Plan 22 file or required test was
changed. Full acceptance on the corrected revision is recorded below.

## Final verification and acceptance

The full acceptance command ran on clean committed revision
`9dbc0f7761848a2fe8a1ef29ec8a2710721d977f`, with the installed Go 1.25.0 bin directory
first on PATH. The acceptance helper intentionally forces GOTOOLCHAIN=local; simply
exporting GOTOOLCHAIN=go1.25.0 was insufficient. An earlier complete run also
passed every gate using PATH Go 1.27.3; it is retained as additional evidence,
not substituted for the required pinned-toolchain run. No runtime code changed
between these passing runs.

```sh
# DATABASE_URL identifies the owned disposable PostgreSQL 17 database;
# TEST_DATABASE_URL matches it. Credentials are never recorded here.
# PATH starts with the installed Go 1.25.0 bin directory.
CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <new-private-summary>
git diff --check 272883beffc37b7597e02ef6e7ff54d3d55688d4 HEAD
```

Safe final summary SHA-256: `ef51f65669f676718c5dd1d3675ad15c1021139de0601586a615035c34f6bcfd`.
The full private summary and diagnostics are retained outside the checkout;
no tokens, raw traffic, auth files or private paths are published.

| Gate | Result | Passed test/result events | Failed | Skipped |
| --- | --- | ---: | ---: | ---: |
| bindings | passed | 3 | 0 | 0 |
| database | passed | 352 | 0 | 0 |
| lab | passed | 428 | 0 | 4 |
| web | passed | 6 | 0 | 0 |
| jsforce | passed | 4 | 0 | 0 |
| python | passed | 4 | 0 | 0 |
| cli | passed | 9 | 0 | 0 |
| restart | passed | 3 | 0 | 0 |
| connected-browser | passed | 8 | 0 | 0 |
| preview | passed | 11 | 0 | 0 |
| cleanup | passed | 2 | 0 | 0 |

Counts above are native reporter events/result identities, including Go subtests
and Node suite terminal events; do not sum them as independent compatibility
scenarios. The four lab skips are the three existing candidate/jsforce parents
plus candidate/openapi/traffic; all four run and pass in the subsequent owned
candidate gate. TestHTTPServedOpenAPI is present/passed in required database
coverage. candidate/openapi/traffic is present/passed in required client coverage
and its fixed nineteen-operation assertion passed, including tenant/field-policy,
upsert escaping, projections, negative errors, auth and real HEAD behavior.
The raw-HTTP supplement is not another independent JSforce implementation.

Cleanup verified 26 owned records, zero unknown intents and all owned
processes stopped; provisioned principals are explicitly retained until the
operator disposes of the dedicated test database. Tool versions: jsforce 3.11.0, @salesforce/cli 2.156.0, @salesforce/plugin-data 5.1.16, @salesforce/plugin-deploy-retrieve 4.2.15, simple-salesforce 1.12.9, go 1.25.0, node 26.8.1, python 3.14.8, openssl 3.6.5, chromium 156.0.8078, postgres 17. All 389 protected baseline files match;
original source-registry content is preserved byte-for-byte with appended entries.
The broader api/openapi.yaml and historical profiles/reports remain unchanged.

## Independent review and limits

A fresh-context reviewer inspected the complete implementation through e164755,
independently ran 27 focused checks with zero failures/skips, and found one
Important media-grammar issue and one schema-subset gap. Both were reproduced
with failing tests and corrected, followed by focused checks and the complete
final acceptance above. Acceptance-only publication and nested-lineage gaps
were also reproduced and corrected with regressions. No second review was used
as a substitute for verified corrections. There are no deferred review findings.

Explicit review boundaries and executor rulings:

| Boundary / ruling | Reason and practical limit |
| --- | --- |
| Work directly on main, using inline execution | Approved plan convention; no branch isolation. No push or merge was performed. |
| Extend source binding to api/ | Embedded YAML/Go must affect acceptance source hash; preflight module/test are additional justified paths. |
| Correct schema-valued additionalProperties despite the initial Minor grade | Constructor promises a closed nonrecursive subset; reject unsupported forms rather than silently admit them. |
| Extend report schema by the exact new offline skip name | Report publication must accept the intentional lab skip while still rejecting unknown skip identities. |
| Preserve nested result ancestry instead of allowing duplicates | Plan 22 repeat suites are distinct tests; the adapter must preserve their native lineage. Modern parent IDs and legacy declaration-order reconstruction were tested; ambiguous missing ancestry fails closed. |
| Verify the pinned Go version through PATH | Full acceptance forces local toolchain; the final evidence uses 1.25.0 and the earlier 1.27.3 pass remains separately identified. |
| Full acceptance was not independently rerun by the reviewer | Executor measured the final gates; review is additional code inspection, not certification of unseen output. |
| Salesforce parity, general OpenAPI compatibility and runtime security remain unjudged | Test-only controlled subset; no Go middleware/library selection, arbitrary uploaded specs or hostile-code sandbox. |
| Authentication, SOQL and permissions remain server responsibilities | Schema tests do not replace the measured policy/tenant/identity assertions or broader M3/M4 work. |
| Unknown create outcomes without an ID remain visibly unrecovered | Durable intent is retained, never replaced by guessed cleanup; no transport-failure injection or recovery certification added. |
| Noncanonical route aliases and remote provenance availability remain outside inventory evidence | Existing convenience paths are not extra operations; registered locally reviewed URLs are not claims of successful remote retrieval. |
| Final report/checklists depend on actual verification | Updated after acceptance; no program milestone, production readiness or prior-plan status is upgraded. |

The candidate now publishes its implemented contract at authenticated GET
/services/data/v65.0/openapi.yaml with a discovery link, exact embedded YAML and
bounded request/response acceptance coverage. Native UI, persistence, CRUD/query
semantics and the historical wider proposed contract retain their prior scope.
Plans 10/14/17/18/19/20/21/22, M2/M3 exit gates, real-target/package execution
and program estimates remain unchanged.
