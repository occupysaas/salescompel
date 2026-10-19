# Plan 11 external-ID body validation evidence decision

Status: **blocked**, recorded 2026-10-19. Product implementation remains gated.
The first reference probe stopped at administrator identity verification before
any fixture operation. There is no completed capture pair or assessment, and no
body-field behavior was measured by this attempt.

Existing named-tenant authorization in
[reference readiness](reference-readiness-foundation.yaml) and
[CLEAN_ROOM.md](../../CLEAN_ROOM.md) applied. The documented, unmodified Salesforce
CLI reported a connected session whose origin and organization exactly matched
the existing private configuration. API and profile pins validated. Before the
attempt, the shared recovery directory was empty and both historical Plan 9
reports recorded complete collection and clean cleanup. Those historical files
were preserved.

The probe's first identity check failed. A subsequent read-only diagnostic using
the same laboratory wire transport returned HTTP **403**, with a non-JSON body,
from `/services/oauth2/userinfo`. It provided neither required identity field.
The verifier records this condition as `IDENTITY_MISMATCH`; this does not prove
that a different administrator was authenticated. The response's underlying
cause remains unknown. Administrator privileges and Account field metadata could
not be checked because their preflight steps follow identity verification.

The private report records collection `blocked`, identity verification `false`,
zero observations, and cleanup `blocked` with `IDENTITY_LATCHED`. Its durable
journal is exactly zero bytes, with SHA-256
`e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855`.
There are zero dispatched operations, known fixture records and unresolved
operation IDs. No fixture creation, update or deletion was dispatched. This is
zero-dispatch evidence, not a successful cleanup or exact-ID absence result.
The permanent identity latch and shared active recovery lock remain intact.
No second probe, mutation retry, recovery bypass or pair assessment was run.

Each required case remains independently unmeasured:

| Body `Compat_Key__c` | Existing-record path | Missing-record path |
| --- | --- | --- |
| Omitted | Blocked before fixture setup | Blocked before fixture setup |
| Identical string | Blocked before fixture setup | Blocked before fixture setup |
| ASCII case-variant string | Blocked before fixture setup | Blocked before fixture setup |
| Different string | Blocked before fixture setup | Blocked before fixture setup |
| `null` | Blocked before fixture setup | Blocked before fixture setup |
| Empty string | Blocked before fixture setup | Blocked before fixture setup |
| Number | Blocked before fixture setup | Blocked before fixture setup |
| Object | Blocked before fixture setup | Blocked before fixture setup |

Private artifacts retain the following provenance. Digests are SHA-256 of exact
file bytes unless described otherwise.

| Binding | Value |
| --- | --- |
| Executed code revision | `52b064e6bfb1ed0e69a01fbe9e4438500ce1cda0` |
| API | `65.0` |
| Profile | `rest-query-foundation` |
| Profile file digest | `99d90e50fc63573f08f5a48da878eb9c129b7d660eb9ed5d600306b9706bd8f8` |
| Body manifest digest | `900e6691beaba174f49abcce4138c5a02c189155563835169c87abb242cc9f98` |
| First report digest | `7f0b0a4922e1f9f7b7e628649c3acaf9ca904ba507eafcca4c27d4e2ece7dcbf` |
| First cleanup proof digest | `e8993105beb249fe5325f5c26658d172a3702714cceaa701dc8956994a6ec0e0` |
| Client binding digest | `0a31f58791092398494ea58875635c4206b2af3dd007fbdf0c3d9d730134a11f` |
| Probe start | `2026-10-19T22:30:18.172Z` |
| Probe finish | `2026-10-19T22:30:18.432Z` |
| Cleanup attempt start | `2026-10-19T22:30:18.417Z` |
| Cleanup attempt finish | `2026-10-19T22:30:18.420Z` |
| Second report / assessment | Not produced |

Client binding is SHA-256 of compact JSON in the recorded report's connector
property order. Pinned versions are JSforce `3.11.0`, Salesforce CLI `2.156.0`,
plugin-data `5.1.16`, plugin-deploy-retrieve `4.2.15`, and simple-salesforce
`1.12.9`. Reports and artifacts remain outside the checkout with private file
and directory permissions. Authentication material and reference response prose
are excluded from this document.

Reproduction uses `probe-external-id-body --config <private-config> --out
<private-output.json>` through `npx tsx src/cli.ts` in `compat/clients/node`,
with the authorized reference URL/token, disposable authorization and shared
recovery directory supplied privately. This attempt exited **2**, the measured
blocked-result status. Its bounded sequential probe never reached the case loop.

The next action is to resolve access to the identity endpoint and verify the
pinned administrator through the authorized public interface. Review the retained
lock, latch and zero-dispatch journal before any approved recovery action; do not
delete the latch or weaken identity checks to force collection. Once recovery
state is legitimately resolved, use fresh exclusive private outputs for two
sequential captures, verify first-run cleanup and exact-ID absence before the
second starts, and run `assess-external-id-body`. All supplied-property cases in
both record states must repeatedly return HTTP 400 `INVALID_FIELD` without
mutation, and omitted controls must succeed, before enabling the product fix.

This decision does not revise the proposed presence rule into an observation,
promote Plan 9's broader drift result, or change the separate
[Plan 10 blocked decision](plan-10-foundation-decision.md).

## Offline verification and review status

Tasks 1–3 are implemented and received task-level review. The final bounded
outcome is **offline lab complete; reference evidence and product work pending**.
The user selected Sol medium subagent execution, overriding the plan's native
sequential recommendation. That execution choice does not relax the evidence gate.

Final verification on 2026-10-19 ran `npm run check` in
`compat/clients/node`: TypeScript typecheck, Prettier, Node tests and repository
contract/catalog/dependency/OpenAPI validation all passed (exit 0). Node reported
193 tests: 192 passed, zero failed, one skipped. The skip was the optional real
JSforce candidate CRUD/policy/relationship/upsert test; it provides no live
candidate verification. Repository-root `git diff --check` also passed (exit 0).
The local verification log is retained in
`.superpowers/sdd/2026-10-19-plan-11-external-id-body-validation/task-6-npm-check.log`.

Task 5 remains entirely unimplemented because no ready assessment exists. No Go
files changed; no Go tests or database-backed tests were expected or run for this
blocked path. Database behavior and real reference behavior remain unverified.
No reference calls or private-artifact/configuration changes were made during
this final verification. The retained identity latch and recovery lock remain
untouched. Historical evidence, accepted profiles and Plan 10 status are preserved.

A fresh whole-change review is **pending**. The controller will request it after
this documentation commit, supplying the baseline, changed files, verification
results and blocked gate outcome. Reviewer access to sanitized documentation and
offline tests does not establish independent verification of inaccessible private
reference artifacts. Any review findings still require resolution before claiming
the final whole-change review is complete. Progress is tracked in the
[implementation plan](../superpowers/plans/2026-10-19-plan-11-external-id-body-validation.md).
