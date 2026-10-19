# Plan 9 — External-ID Contract

Plan 9 adds bounded reference probes, durable recovery and evidence comparison.
It is M1 observation work supporting M3 REST/upsert semantics. Product routes,
upsert implementation, catalog v2 and accepted pilot/foundation profiles are
unchanged. It does not complete M3, M4 or the separate Plan 5 expiry work.

## Collection boundary

The authorized reference administrator identity and synthetic field metadata were
rechecked before collection: API 65.0; both Text fields length 80;
Compat_Key__c externalId=true, unique=true, caseSensitive=false. Credentials,
identities, full response bodies, journals and record IDs remain in owner-only
storage outside the checkout. Existing Plan 5 sessions were preserved.

The fixed manifest exercises 51 operations, including five rounds each of two
concurrent exact/variant upserts and two native JSforce calls. Collection is
bounded to ten minutes, cleanup to five minutes, at most 80 records, 160
non-cleanup mutations and concurrency two. There are no mutation retries.
Returned and independently discovered IDs are retained durably. Cleanup requires
exact marker/key ownership, exact-ID deletion and independent absence checks.
An unresolved dispatched request remains blocked even after empty queries;
an identity mismatch is durably latched and prevents automatic cleanup.

## Interpretation and next implementation boundary

The measured cases determine a narrow contract, not a general Unicode algorithm.
Case-insensitive ASCII matching, composed/decomposed accent matching and sharp-s
matching cannot safely be reproduced by simply choosing lower(), citext or an
arbitrary database collation. Dotless-I is a counterexample to treating all
visually related forms as equal. Leading/trailing whitespace observations apply
to the tested lookup strings; broader whitespace and creation normalization need
separate observations.

The next product design must make lookup, uniqueness checks and concurrency
control use the same tenant-scoped equivalence relation, while preserving the
observed stored spelling. It must explicitly cover create/update duplicate errors,
upsert create versus update status/body, path-field rejection in the body, and
one-pass URL decoding. Native-client results must remain a separate projection
from wire status and body presence. Cross-tenant fixtures must prove isolation.

Before changing catalog v2, a migration design must scan for collisions under the
proposed equivalence rule and refuse ambiguous existing data without rewriting
or deleting it. Unicode beyond the tested pairs, other locales, other fields,
missing/null keys, whitespace variants and all possible race schedules remain
unknown. Five measured rounds do not establish a universal race guarantee.
Plan 5 authorization/session expiry evidence remains an independent dependency.
No equality algorithm or product migration is selected in this plan.

## Review and verification

A fresh whole-change review found no Critical issues and four Important issues.
All four were accepted and corrected with failing-then-passing regressions:
retain known IDs after a marker changes; reject missing observed-response
metadata and missing/corrupt disk artifacts; latch transient identity mismatch;
and compare race inputs, response multisets and final states independently of
which request won. Raw request/response associations remain in private reports.
There are no deferred review findings.

Reviewer exclusions were resolved by the implementer: actual reference identity,
metadata and cleanup were checked live; the final documentation was checked
against private reports; `.recomp.json` passed Ajv validation against the fetched
v1 schema; and the full Node check passed 106 tests with one optional candidate
skip, zero failures, plus typecheck, formatting and repository validation.
The reviewer did not independently access private reference evidence.

## Reference observations

Both sequential runs completed 51 operations, verified identity and cleaned all
owned records. Artifact bytes were digest-validated before comparison. The
comparison outcome is **drift**: rounds race-variant-2, -3 and -4 stored `AbC`
in run 1 and `abc` in run 2. Each retained one record and returned one 201 create
and one 400 DUPLICATE_VALUE. This reflects the winning create spelling, not
just request ordering. No stable whole-contract claim is made. All deterministic
cases matched. Exact-key race rounds 0 and 4 swapped request winners only;
the comparator correctly treats those response multisets and final states as equal.

- Run 1: 2026-10-19T01:50:58.767Z to 2026-10-19T01:51:45.365Z; report SHA-256 `55dc9835483036b62c3b16bab0f99818a04bd623082d38704b3a503bc580b64f`.
- Run 2: 2026-10-19T01:51:45.400Z to 2026-10-19T01:52:30.388Z; report SHA-256 `cb0eb6bbcfffd122822b04a1f1fd281edad2579296539c4efd630a67ab3b7b0f`.

Manifest SHA-256: `20e0d6fee4b486b881dcb7d0dc5e4d6d337da71cfd932c9577e582ac26a632f1`.
Foundation profile SHA-256: `99d90e50fc63573f08f5a48da878eb9c129b7d660eb9ed5d600306b9706bd8f8`.

Toolchain binding: jsforce 3.11.0, @salesforce/cli 2.156.0, @salesforce/plugin-data 5.1.16, @salesforce/plugin-deploy-retrieve 4.2.15, simple-salesforce 1.12.9. Only wire and native JSforce operations were exercised here.

The table lists operations in execution order, with a state snapshot after each
sequential operation (after both requests for races). `{p}` is the case-specific
random ownership prefix; JSON escapes preserve Unicode and whitespace exactly.
`r0`/`r1` are case-local identity aliases. Body-field sequences show only supplied
values. All wire responses had bodies; upsert 201/200 projected created=true/false,
and successful direct creates returned an ID with success=true. Native results
had success=true, created=true then false, and no HTTP status projection. Error
codes below are observed failures, not unavailable requests. Confidence is limited
to two runs in this one tenant/configuration.

| Case | Input keys; body field if supplied | Run 1 outcomes | Run 1 stored state sequence | Run 2 difference |
|---|---|---|---|---|
| exact | `["{p}key","{p}key"]` | 201; 200 | `[["r0={p}key"],["r0={p}key"]]` | same |
| ascii | `["{p}AbC","{p}abc"]` | 201; 200 | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| create-variant | `["{p}AbC","{p}abc"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| update-variant | `["{p}AbC","{p}other","{p}abc"]` | 201; 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r1={p}other","r0={p}AbC"],["r1={p}other","r0={p}AbC"]]` | same |
| body-identical | `["{p}AbC","{p}AbC"]`; body `["{p}AbC"]` | 201; 400 INVALID_FIELD | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| body-variant | `["{p}AbC","{p}AbC"]`; body `["{p}abc"]` | 201; 400 INVALID_FIELD | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| body-different | `["{p}AbC","{p}AbC"]`; body `["{p}different"]` | 201; 400 INVALID_FIELD | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| leading-space | `["{p}key"," {p}key"]` | 201; 200 | `[["r0={p}key"],["r0={p}key"]]` | same |
| trailing-space | `["{p}key","{p}key "]` | 201; 200 | `[["r0={p}key"],["r0={p}key"]]` | same |
| accent | `["{p}\u00e9","{p}e\u0301"]` | 201; 200 | `[["r0={p}\u00e9"],["r0={p}\u00e9"]]` | same |
| ascii-i | `["{p}I","{p}i"]` | 201; 200 | `[["r0={p}I"],["r0={p}I"]]` | same |
| dotless-i | `["{p}I","{p}\u0131"]` | 201; 201 | `[["r0={p}I"],["r1={p}\u0131","r0={p}I"]]` | same |
| sharp-s | `["{p}\u00df","{p}ss"]` | 201; 200 | `[["r0={p}\u00df"],["r0={p}\u00df"]]` | same |
| encoded | `["{p}/percent%\u00e9","{p}/percent%\u00e9"]` | 201; 200 | `[["r0={p}/percent%\u00e9"],["r0={p}/percent%\u00e9"]]` | same |
| race-exact-0 | `["{p}AbC","{p}AbC"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | 400 DUPLICATE_VALUE; 201; `[["r0={p}AbC"],["r0={p}AbC"]]` |
| race-exact-1 | `["{p}AbC","{p}AbC"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| race-exact-2 | `["{p}AbC","{p}AbC"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| race-exact-3 | `["{p}AbC","{p}AbC"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| race-exact-4 | `["{p}AbC","{p}AbC"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | 400 DUPLICATE_VALUE; 201; `[["r0={p}AbC"],["r0={p}AbC"]]` |
| race-variant-0 | `["{p}AbC","{p}abc"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| race-variant-1 | `["{p}AbC","{p}abc"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | same |
| race-variant-2 | `["{p}AbC","{p}abc"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | 400 DUPLICATE_VALUE; 201; `[["r0={p}abc"],["r0={p}abc"]]` |
| race-variant-3 | `["{p}AbC","{p}abc"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | 400 DUPLICATE_VALUE; 201; `[["r0={p}abc"],["r0={p}abc"]]` |
| race-variant-4 | `["{p}AbC","{p}abc"]` | 201; 400 DUPLICATE_VALUE | `[["r0={p}AbC"],["r0={p}AbC"]]` | 400 DUPLICATE_VALUE; 201; `[["r0={p}abc"],["r0={p}abc"]]` |
| native | `["{p}key","{p}key"]` | native created=true; native created=false | `[["r0={p}key"],["r0={p}key"]]` | same |
