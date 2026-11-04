# Plan 24 — Atomic Composite writes execution

Status: implementation in progress, 2026-11-04.

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

Full acceptance and independent review remain pending.
