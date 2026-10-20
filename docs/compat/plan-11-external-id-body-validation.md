# Plan 11 external-ID body validation evidence decision

Status: **different**, measured 2026-10-20. Two fresh complete captures repeat
the same behavior, with clean cleanup. The proposed universal presence rule is
contradicted by null acceptance and the object error code. Task 5 remains gated
pending an evidence-driven design revision; no Go implementation changed.

## Current reference decision

The maintainer explicitly authorized both administrators as test accounts in the
same organization and authorized necessary test setup, excluding production.
A new private configuration binds the currently connected administrator. Its
origin, organization, API and profile match the original configuration; the user
pin changed with authorization. Public userinfo and current-user queries verified
same-organization identity, active status and ModifyAllData. The original private
configuration and historical evidence were preserved.

The original authentication failure now has a causal diagnosis: `sf org display
--json` returns a redacted token placeholder. The supported documented
`sf org auth show-access-token --target-org <authorized-alias> --json` supplies
the existing session token. Replacing only credential acquisition made the exact
laboratory wire identity request succeed. Credentials were captured in subprocess
memory and never printed or persisted. A real user mismatch also existed against
the original pin; the authorized new binding resolves that separate issue.

Before fresh collection, the original blocked run's exact report digest, binding,
byte-empty journal, zero observations, zero known IDs, zero unresolved operations
and permanent identity latch were checked. Only expected artifacts existed.
An immutable private resolution record was written and fsynced with authorization
and exact hashes. After checking unchanged bytes and lock inode, the old active
lock was archived unchanged under a distinct resolved filename. The latch remains.
This operator resolution proves zero dispatch; it is not successful cleanup of
the original blocked run.

A fresh preflight then stopped with `FIELD_METADATA_MISMATCH`: Account describe
returned HTTP 200 but omitted the fixture field. Public Tooling metadata confirmed
the existing field is Text, length 80, externalId true, unique true, caseSensitive
false. Existing Compat_Admin grants read/edit access, but the new administrator
had no assignment. The authorized temporary assignment returned HTTP 201 and was
tracked by a separate fsynced intent/dispatch/response journal and exact ID.
The unchanged real preflight then passed. Both captures used those same permissions.
After assessment, deletion of that exact assignment returned HTTP 204; an
independent query confirmed zero assignments, restoring the original state.
No field metadata or evidence expectations were changed.

Each cell below was independently observed in both captures. Each run contains
16 cases: two record states times omission plus seven supplied values.

| Body `Compat_Key__c` | Existing-record path | Missing-record path |
| --- | --- | --- |
| Omitted | HTTP 200; update intended row | HTTP 201; create one intended row |
| Identical string | HTTP 400 `INVALID_FIELD`; unchanged | HTTP 400 `INVALID_FIELD`; absent |
| ASCII case-variant string | HTTP 400 `INVALID_FIELD`; unchanged | HTTP 400 `INVALID_FIELD`; absent |
| Different string | HTTP 400 `INVALID_FIELD`; unchanged | HTTP 400 `INVALID_FIELD`; absent |
| `null` | HTTP 200; update intended row | HTTP 201; create one intended row |
| Empty string | HTTP 400 `INVALID_FIELD`; unchanged | HTTP 400 `INVALID_FIELD`; absent |
| Number | HTTP 400 `INVALID_FIELD`; unchanged | HTTP 400 `INVALID_FIELD`; absent |
| Object | HTTP 400 `JSON_PARSER_ERROR`; unchanged | HTTP 400 `JSON_PARSER_ERROR`; absent |

For omitted and null updates, the original ID and path key remain and Name
changes to the requested Name. For omitted and null creates, exactly one row
contains the path key and requested Name. Success objects contain `id`,
`success: true`, `errors: []`, and `created` (false for updates, true for creates);
the returned ID equals the independently read row ID. Null is accepted without
clearing the key. Rejected existing cases preserve ID, Name and key; rejected
missing cases leave no owned row under either path or body key.

Every rejection body is a one-element JSON array. Its object contains
`message` (string) and `errorCode` (string); the `fields` property is **absent**,
not null or an empty array. Raw messages remain private. The object response
indicates parsing/type rejection; the scalar response indicates the external-ID
body field is disallowed. These isolated fixtures establish no broader error
precedence or behavior for arrays, booleans, aliases or duplicate JSON members.

`assess-external-id-body` exited 2 with outcome `different`, no top-level blocking
reasons, twelve ready findings and four different findings: existing/missing null
and object, each labeled `UNEXPECTED_OR_UNREPEATABLE_BEHAVIOR`. Independent case
inspection shows these four results repeat; they differ from the proposed rule.
The assessor was not weakened or forced to ready.

Both captures completed with verified identity, clean cleanup, ten exact-ID
absence proofs and zero unresolved operations each. First cleanup completed
strictly before the second began. The standard command held its shared recovery
lock while validating prior cleanup and rechecking prior exact-ID absence before
second-run writes. No active recovery lock remains. The shared last-clean pointer
retains the second capture's private evidence. Historical Plan 9 evidence and the
separate Plan 10 blocked decision remain independent.

## Fresh provenance

Digests below are SHA-256 of exact file bytes. Parsed wire responses, state,
discovery, journal and cleanup proof artifacts remain private; original HTTP
response bytes are not available and are not claimed.

| Binding | Value |
| --- | --- |
| Executed code revision | `28ca619a069fe4ef102787bfb3b5bae546e2946f` |
| API / profile | `65.0` / `rest-query-foundation` |
| Profile file digest | `99d90e50fc63573f08f5a48da878eb9c129b7d660eb9ed5d600306b9706bd8f8` |
| Body manifest digest | `900e6691beaba174f49abcce4138c5a02c189155563835169c87abb242cc9f98` |
| Client binding digest | `0a31f58791092398494ea58875635c4206b2af3dd007fbdf0c3d9d730134a11f` |
| New config digest | `c3191cb77ab8f5f983ca49d2a1f09d7c53a266f0f85dd00e42b60aa7a01f117e` |
| Capture A report digest | `47254f0abcd997fb313a0bdd992d7a0e8e5fa3ff2e935c322d93de6772ab942d` |
| Capture B report digest | `698a7733c3b9fa6de718fc5a54614b034b8073d4b0d9fb7b0927afbfdf338a1a` |
| Assessment digest | `af5ff0f505216e2d7f91f98525c1f9496f2daf50b438e1356dcb3c268447eb15` |
| A capture start | `2026-10-20T04:05:39.906Z` |
| A capture finish | `2026-10-20T04:06:07.963Z` |
| A cleanup start | `2026-10-20T04:05:55.242Z` |
| A cleanup finish | `2026-10-20T04:06:07.951Z` |
| A cleanup proof digest | `e64f07eca01ee2c8b6332faa797ed98b8964e58e5d27f2abb5745f2187127627` |
| B capture start | `2026-10-20T04:06:19.669Z` |
| B capture finish | `2026-10-20T04:06:48.860Z` |
| B cleanup start | `2026-10-20T04:06:36.131Z` |
| B cleanup finish | `2026-10-20T04:06:48.846Z` |
| B cleanup proof digest | `09de05b69e3ba631a6cdf377899ed48f3e217775b540d1911922b57ceeb97202` |

Pins remain JSforce `3.11.0`, Salesforce CLI `2.156.0`, plugin-data `5.1.16`,
plugin-deploy-retrieve `4.2.15`, simple-salesforce `1.12.9`. Reproduction uses the
existing probe twice with fresh private outputs and the standard assessor,
with authorized reference URL/token, disposable flag and shared recovery directory.
Private files are 0600 and directories 0700; the immutable resolution record is
0400. No credentials, organization/user identifiers or raw response prose are
published. This documentation update requires `git diff --check` and a sanitized
content review; it introduces no code requiring a new test run.

## Historical blocked decision (2026-10-19)

The retained text below describes the original attempt and its review status at
that time. Its diagnosis and next-action statements are superseded by the fresh
measured decision above. Its original latch and captures remain intact.

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

A fresh whole-change review is **complete** for the authorized blocked outcome.
It reviewed baseline `e5e3960` through `856f335` (seven commits and fifteen changed
files), the design/plan and recorded verification. Spec and quality review approved
with no Critical, Important or outstanding Minor findings. The reviewer inspected
existing verification records; no new test run was needed for this documentation
update. The local review is retained in
`.superpowers/sdd/2026-10-19-plan-11-external-id-body-validation/final-review.md`.

The reviewer did not access private raw reference evidence. Separately, the
controller verified the blocked report digest, zero observations, empty journal
and retained identity latch. These checks establish the recorded blocked state;
live body-field behavior, repeatability, successful cleanup/absence and the
identity-endpoint denial's cause remain unverified. Review approval does not
authorize Task 5. Progress is tracked in the
[implementation plan](../superpowers/plans/2026-10-19-plan-11-external-id-body-validation.md).
