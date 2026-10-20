# Plan 11 — External-ID body-field validation

Status: evidence-driven revision approved and executed (2026-10-20). The reviewed
v2 assessment is ready for sixteen cases; product implementation `4d57177` and
Go verification are complete. Task 5 and final whole-change spec/quality reviews
approved the bounded implementation with no findings. The final review covered
`28ca619..465fba2`; inaccessible private evidence relies on recorded controller
verification, as detailed in the linked decision.
Milestone: bounded M1 evidence follow-up and M3 candidate fix.

## Evidence-driven revision — 2026-10-20

This execution revision supersedes the original universal presence rule and its
acceptance criteria below. It is the executor's correction within the authorized
validation work, based on two complete reference captures documented in
[the decision](../../compat/plan-11-external-id-body-validation.md). The user
explicitly authorized either test administrator and necessary test setup on
2026-10-20; the new private binding preserves the original configuration.

The captures agree in both existing-record and missing-record states:

| Body value | Required behavior |
| --- | --- |
| Omitted or null | Use path key; update with 200 or create with 201; success=true, created=false/true, errors=[] |
| Identical, case-variant, different or empty string; number 42 | 400 INVALID_FIELD; no mutation |
| Object | 400 JSON_PARSER_ERROR; no mutation |

Each rejection is a one-element error array containing message:string and
errorCode:string, with fields absent. Author explanation text independently.
Successful null update retains the existing ID and external key while applying
Name; null create stores the path key. Null must not erase the lookup field.
The supplied non-null rejection check runs before transactions/locks/writes;
null and omitted follow the existing copy-and-inject path, without mutating the
caller's map. Objects and arrays return JSON_PARSER_ERROR; array behavior is a
candidate robustness decision consistent with existing structured-value
validation, not an additional reference observation. Other non-null scalar values
return INVALID_FIELD; only the five scalar cases above are reference-tested.

Preserve the first assessment's `different` result and all capture bytes.
Version revised assessments with explicit `contractVersion` equal to
`plan11-observed-body-validation-v2`. Reassess the already complete pair into a
new exclusive private output; the same fixture matrix, identity, integrity,
state, cleanup and repeatability requirements apply. The new version changes
only expected case semantics and pins the observed rejection envelope. Historical
unversioned assessments represent the original hypothesis, never this revision.
Do not modify or relabel them. No new capture is required solely to evaluate
these existing measured responses under the corrected expectations.

Before product implementation, a fresh reviewer must check this revision and
its assessor, and the versioned assessment must be ready for all sixteen cases.
No query, equality, catalog, permissions or route expansion is introduced.
The old blocked attempt and the metadata-preflight-only attempt remain recorded;
temporary fixture permissions for the successful pair were restored.

## Historical original design — 2026-10-19

The remaining sections preserve the original planning rationale and provisional
universal rejection hypothesis. Their present-tense candidate descriptions,
missing-evidence statements and acceptance instructions describe the pre-execution
state; they are superseded by the approved 2026-10-20 revision above wherever
semantics differ. The two complete captures measured missing-record behavior,
and the reviewed v2 ready assessment satisfied the revised evidence gate before
product commit `4d57177`. Historical observations remain as originally recorded.

### Intent and basis

Close the request-validation defect identified by
[Plan 9](../../compat/plan-9-external-id-contract.md), the next independent work
identified in [Plan 10's decision](../../compat/plan-10-foundation-decision.md).
An external-ID upsert currently accepts `Compat_Key__c` in the JSON body when
its value exactly equals the path key. Plan 9 observed HTTP 400 `INVALID_FIELD`
for identical, case-variant and different body values against an existing
record. The candidate's equal-value exception therefore contradicts the recorded
existing-record observations. Missing-record behavior has not been observed.

Deliver a focused reference decision and, when its evidence gate passes, reject
the path field in the upsert body without creating or changing records. Keep
the existing public route:
`PATCH /services/data/v65.0/sobjects/Account/Compat_Key__c/{key}`.

The executor selects this scope from the recorded follow-up order. Use the
existing Go/PostgreSQL candidate and TypeScript compatibility laboratory, pinned
clients and API 65.0. Work directly on `main`, per the roadmap. Native sequential
execution is the recommendation inherited from Plan 10; implementation method
can be settled when the implementation plan is reviewed.

### Scope and exclusions

The supported request in this design uses the exact field spelling
`Compat_Key__c` and the existing Account external-ID route. Validate presence
of that own JSON property, independently of its value and independently of
whether the path key matches a record. This is the proposed candidate rule,
conditional on the reference gate below, not an observation of every JSON type.

Do not select an external-ID equality algorithm, trim or normalize keys, change
uniqueness or locking, migrate catalog v2, or alter successful upsert responses.
Unicode equivalence, creation normalization, races, field-name case aliases,
duplicate JSON member handling and general error precedence remain separate
contracts. Ordinary create and record-ID update may still supply this field
under their existing validation rules. No UI, dependencies or new routes are
required. Plan 5 query evidence and the blocked Plan 10 foundation decision
remain independent.

### Reference evidence required before the product fix

Add a separately identified, fixed body-validation probe suite using Plan 9's
identity verification, private artifacts, durable journaling and recovery.
Preserve historical Plan 9 manifests, report bytes and comparison meaning;
new coverage must have its own manifest binding. Reuse supporting machinery
where practical without silently expanding the historical 51-operation suite.

For each of existing-record and missing-record paths, collect these eight body
cases with valid `Name` and otherwise valid data:

| Body `Compat_Key__c` | Purpose |
| --- | --- |
| omitted | Successful update/create control |
| identical string | Equal-value exception |
| ASCII case-variant string | Value-independent rejection |
| different string | Conflicting value |
| `null` | Presence distinct from omission |
| empty string | Presence distinct from a usable key |
| number | Type-invalid supplied value |
| object | Structured supplied value and panic regression |

Use a fresh case-specific path key and ownership marker for every case, with
existing records seeded through ordinary create. The requested Name differs
from the seed Name but remains within the same provable ownership scope, so a
rejected update cannot silently mutate Name. Missing cases have no seed: verify
absence before dispatch. Retain ownership tracking if a supplied different key
is unexpectedly stored. After every request, capture response status, body
presence, error code and fields shape, plus independently read persisted state.
Verify existing records retain ID, Name and key after rejection; missing cases
must leave no owned record under either path or body key. Controls must create
or update the intended record. Record messages privately and summarize their
meaning independently; do not copy Salesforce product prose into the candidate.

Run two fresh sequential captures, with cleanup and exact-ID absence verified
before the second begins. Require matching deterministic observations for this
suite and current profile/API/client/manifest bindings. Keep private 0600 files
and 0700 directories outside the checkout, validate artifact digests, and publish
only sanitized summaries. Do not treat authentication failures or unavailable
state reads as measured validation responses.

Recheck the named authorized tenant, administrator and field metadata using the
existing Plan 9 preflight. Keep its ceilings: collection ten minutes, cleanup
five minutes, at most 80 created records and 160 non-cleanup mutations; this
suite is sequential, with no mutation retries. Unresolved dispatched requests,
identity mismatch or incomplete recovery block subsequent collection. Existing
authorization applies; credentials must be available through the private setup.

The intended gate is repeatable HTTP 400 `INVALID_FIELD` and no mutation for
all supplied-property cases in both record states, with successful omitted-field
controls. If any case differs, record the actual behavior and revise this design
before implementing a universal presence rule. If credentials or evidence are
unavailable, finish the offline probe work and report the precise blocker; do
not implement the unverified missing-record rule or claim reference completion.

### Candidate behavior after the evidence gate

Put the validation in `Store.Upsert` in `internal/platform/store.go`, shared by
HTTP and direct store callers. Reject a supplied `Compat_Key__c` before injecting
the path key and before opening a transaction, taking a lock or performing a
write. Check property presence rather than comparing interface values, which
can panic for maps or arrays. Leave the caller's map untouched.

Return the existing API error machinery's HTTP 400 `INVALID_FIELD` envelope,
with independently written explanation text. Match any additional measured
error-field shape only when the evidence supports it. Authentication, route
resolution and JSON parsing retain their current order. For valid requests
without that body property, inject the already decoded path key and retain the
current validation, tenant boundary, lookup and create/update behavior.

Do not infer broad validation precedence from a body containing multiple faults.
The reference fixtures isolate this one fault. Existing malformed-body and
unauthorized-request handling must continue to work.

### Verification and acceptance

Laboratory tests must demonstrate fixed coverage, omitted versus null distinction,
state-change detection, incomplete observations, incorrect bindings, corrupt
artifacts and cleanup/recovery failures. Missing-record rejection is not proven
by an error alone: a hidden insert must fail assessment. An unexpected response
must be captured faithfully and block the gate rather than be rewritten to the
expected outcome. Historical Plan 9 reports retain their validation semantics.

After the gate passes, add failing-then-passing Go regressions for existing and
missing records, all supplied-property values above, and an array value as a
candidate robustness test. The array test is not live reference evidence.
Assert API error/status, unchanged caller data, unchanged stored rows and no
cross-tenant effects. Exercise both HTTP and direct store calls. Include omitted
property create/update controls, ordinary create/record-ID update controls, and
an encoded-key regression to preserve one-pass decoding. Direct validation tests
should prove rejection does not need a usable database connection.

Run `npm run check` in `compat/clients/node`, `go test ./...` with
`TEST_DATABASE_URL` pointing at a disposable PostgreSQL database, and
`git diff --check`. Integration tests skipped for lack of a database are an
explicit verification gap. Obtain one fresh whole-change review and resolve
Important or Critical findings before declaring the implementation complete.

Publish `docs/compat/plan-11-external-id-body-validation.md` during execution,
recording case-level observations, capture times/digests, manifest and toolchain
bindings, cleanup outcomes, candidate verification and unresolved limitations.
Do not promote the whole Plan 9 drift result or Plan 10 blocked result to pass.
Completion means this narrow validation contract is evidenced and implemented,
or the evidence work ends with a precise blocker and product work remains pending.

### Planning boundaries

The implementation plan should separate the independently testable probe and
assessment work from the evidence-gated candidate fix. Likely product touchpoints
are `internal/platform/store.go`, `store_test.go` and `http_test.go`; reuse the
existing Node external-ID recovery helpers and add focused suite/schema/test
files as needed. Exact helper signatures and artifact schema layout are left to
the builder, provided historical evidence remains interpretable and all gates
above are enforced. No product code or live reference mutation is part of
creating this spec.

### Builder check

A fresh software-engineer review found no important unanswered planning
questions. The case matrix and evidence gate are executor recommendations for
review; no new reference observations or product changes are claimed.
