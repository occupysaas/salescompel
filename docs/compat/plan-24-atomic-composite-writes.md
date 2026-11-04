# Plan 24 — Atomic Composite writes execution

Status: implemented, independently reviewed, corrected and verified, 2026-11-04. All eleven full candidate acceptance gates passed on Go 1.25.0 and PostgreSQL 17.

Execution base: `5df1d0ae9c65d3efabf46da1fb464c65997fd1eb`. Initial tracked tree clean; only the two approved Plan 24 draft documents were untracked. Native execution directly on main follows the approved plan.

The accepted pilot observation supports only create followed by invalid-field rollback. Other semantics are candidate decisions. No live Salesforce org is used. The three public sources linked in the spec were reread on 2026-11-04; no source prose or code was copied.

## Development verification

- Pure Go decoder/reference tests: missing symbols RED, then passing.
- PostgreSQL 17 transaction tests: linked commit, ordered writes, create/update/delete rollback, real foreign-key failure, restricted/tenant policy, 25 writes, prevalidation, and injected begin/commit/rollback/cancellation failures passed.
- HTTP: disabled-route RED then passing route/auth/method/version/body-budget/header/discovery checks. Existing unsupported-resource/discovery assertions were updated for the new endpoint.
- Offline candidate contract/traffic tests: nineteen-operation RED, then 13 tests passed with twenty operations and the bounded closed-empty-header schema exception. Typecheck, validate and embedded-source checks passed.
- Fixture/wiring checks: missing helper/identity RED then passing. Ambiguous outcomes retain unresolved intents; known IDs are recorded before whole-envelope assertions.
- Owned live development: missing cross-tenant journey RED, then both candidate/composite/atomic and candidate/openapi/traffic passed, with verified exact-ID cleanup and zero unknown intents.
- Full offline npm run check passed with five explicit opt-in candidate skips; these must all execute in the subsequent owned client gate.

A development preflight needed CHROMIUM_PATH set to the installed browser instead of the pinned Playwright package's absent default browser path. No browser package/version or manifest was changed.

Full acceptance and independent review are recorded below.

## Final measured acceptance

The first full run passed every gate at `88f5cb2`; the final run repeated every gate after the independent review correction. This final evidence binds revision `938f3404d5cc6fad723e2345d4ba5d30e010641b` and source digest `5caf390b2d5af600068055f24953dff9e20c3249ac814f9db0e48ef20c8ee35e`.
Safe summary SHA-256: `ef77677128cca856b63d2945246cf3be6e61c6b1abfe205b82fde3d101c00401`. The private summary and diagnostics contain no published token/body captures.

The final command was the existing owned runner, with DATABASE_URL identifying the dedicated disposable PostgreSQL 17 test database, the installed Go 1.25.0 bin first on PATH, and CHROMIUM_PATH selecting installed Chromium 156:

```sh
CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <new-private-summary.json>
```

| Gate | Outcome | Passed events | Failed | Skipped |
| --- | --- | ---: | ---: | ---: |
| bindings | passed | 3 | 0 | 0 |
| database | passed | 386 | 0 | 0 |
| lab | passed | 443 | 0 | 5 |
| web | passed | 6 | 0 | 0 |
| jsforce | passed | 5 | 0 | 0 |
| python | passed | 4 | 0 | 0 |
| cli | passed | 9 | 0 | 0 |
| restart | passed | 3 | 0 | 0 |
| connected-browser | passed | 8 | 0 | 0 |
| preview | passed | 11 | 0 | 0 |
| cleanup | passed | 2 | 0 | 0 |

Counts are native reporter events including subtests/suite events, not independent compatibility scenarios. The five offline lab skips are the three existing JSforce parents, candidate/openapi/traffic and candidate/composite/atomic; all five execute and pass in the owned client gate. Required database coverage includes both TestCompositeAtomic and TestHTTPCompositeAtomic. The independent OpenAPI journey asserts exactly twenty successful operation IDs. Composite traffic validates against the served document, with separate persistence, policy, rollback and tenant assertions.

Final cleanup verified 30 owned fixture records, zero unknown intents and all owned candidate processes stopped. Provisioned principals were retained by the runner under its existing policy, then the task-owned disposable PostgreSQL container was removed after verification. No other database or container was changed.

Tool versions: jsforce 3.11.0, @salesforce/cli 2.156.0, @salesforce/plugin-data 5.1.16, @salesforce/plugin-deploy-retrieve 4.2.15, simple-salesforce 1.12.9, go 1.25.0, node 26.8.1, python 3.14.8, openssl 3.6.5, chromium 156.0.8078, postgres 17. Final offline `npm run check`: 437 passed, five deliberate skips, zero failures; typecheck, formatting and document validation also passed. Final full acceptance repeats the broader Go race/vet, web, real JSforce/Python/CLI, restart and connected/preview browser checks; those are not inferred from the focused tests.

## Independent review and correction

A fresh reviewer inspected the complete change from `5df1d0a` through `88f5cb2`, ran 44 focused offline Node tests plus the pure Go decoder/validator tests, and found one Important issue: repeated create intent IDs across helper calls could invalidate the entire cleanup journal. No Critical or Minor findings were reported. Review did not independently measure the full database or acceptance run.

The issue was reproduced with failing sequential-call and concurrent reservation tests. The correction rejects existing create identities before appending or dispatching, preserving the original cleanable journal. A short private sidecar lock serializes reservation across the owned Node processes; stale locks fail closed after a bounded wait. No fixture event format or global cleanup semantics changed. All eleven focused fixture tests passed after the correction, then the entire offline suite and final full acceptance above passed. No material review finding remains deferred.

## Rulings and review boundaries

- Native inline execution directly on main follows the approved plan and repository convention; no separate integration step is needed. This does not publish a remote revision.
- Existing HTTP unsupported-resource and discovery tests now target composite/batch and four resources, because the newly implemented endpoint invalidates their old assertions. Retaining the old expected values would contradict delivered behavior.
- Sidecar reservation is a private acceptance-helper concern; a stale lock blocks further create dispatch rather than guessing recovery. A stale test lock may need operator resolution, but cannot silently corrupt a journal.

The review examined the following conditions and found no additional defect. These are retained judgments with their practical limits:

| Condition | Retained judgment / limit |
| --- | --- |
| Invalid final route/body/reference | Whole-request structural validation precedes Begin; business errors remain transactional. A prevalidation regression could otherwise leave partial effects. |
| SQL failure and cancellation | One transaction and a bounded independent rollback context undo writes. No broader locking/isolation policy is certified. |
| Commit failure | Outer failure exposes no generated success IDs; a lost connection can still leave client outcome uncertain. |
| Lost response/uncertain commit | Unknown intents block verified cleanup; retry/idempotency is not promised. |
| Malformed neighboring response | Independently attributable IDs are journaled before global assertions; ambiguous identities remain unresolved. |
| Rollback/prevalidation absence | Exact-marker queries must establish absence before rejection is recorded; duplicate recovery leaves the original intent unresolved. |
| Permissions and tenants | Shared existing validation and tenant predicates remain authoritative; this is not a wider security certification. |
| URL/reference/null/case boundaries | Strict parser matches the bounded design; arbitrary expressions and URL destinations remain unsupported. |
| Duplicate JSON keys | Existing last-value parser behavior is preserved; no new canonical JSON contract is claimed. |
| Cross-object row references | An allowed earlier-create ID may produce a transactional NOT_FOUND against the other object; it does not bypass tenant or object constraints. |
| Empty httpHeaders | Only the named closed empty-map schema gets the guard exception; permissive record schemas remain rejected. |
| Inventory and acceptance wiring | Twenty operations and exact required identities/file inclusion are enforced; raw HTTP adds coverage, not another independent JSforce implementation. |
| Updated old HTTP assertions | They track the newly implemented current surface while historical evidence remains unchanged. |
| Main execution/publication timing | Direct main was approved; final completion text follows measured execution and corrected review. |
| Reference parity/full acceptance review | Reviewer did not measure Salesforce parity or rerun full acceptance; executor measured the final candidate run above. |

## Preserved boundaries and remaining work

All 396 untouched baseline files match their original hashes. The source register preserves its original bytes with appended entries; dependency manifests/locks, catalog/schema, CLEAN_ROOM.md, broader api/openapi.yaml, historical plans/reports, profiles and accepted observations retain their previous content. The intentional current-guide and HTTP-test adjustments are identified above.

This delivers only POST /services/data/v65.0/composite for explicit allOrNone=true Account/Contact POST/PATCH/DELETE and restricted earlier-create ID references, with omitted/false collation executed sequentially. Partial-success mode, collation, GET/query/upsert subrequests, arbitrary headers, nested Composite, general expression evaluation and other Composite resources remain deferred. The pilot reference supports only its recorded create+invalid-field rollback; added behavior is measured candidate behavior, not newly observed Salesforce behavior. No program milestone, production-readiness or general Salesforce compatibility claim is upgraded; Plans 10/14/17/18/19/20/21/22 retain their prior scope/status.
