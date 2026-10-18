# Plan 9 — External-ID upsert contract

## Intent

The maintainer approved prioritizing reliable external-ID upserts while preserving
Salesforce routes for drop-in clients. The practical goal is that a retry resolves
to the expected Account rather than silently creating a duplicate. Plan 8 activated
field lengths; Plan 7's caseSensitive=false declaration does not establish runtime
matching, error or race behavior.

This plan's deliverable is executable, recoverable black-box probes and a reviewed
contract. Product implementation and collision migration are a subsequent plan,
written from the observations. This split is the executor's choice: selecting a
normalization algorithm before collecting evidence would make the implementation
plan speculative. No product code or schema change belongs to Plan 9.

## Current implementation and desired next boundary

Store.Upsert currently checks a literal body/path key match, locks a tenant plus
literal key, and queries compat_key with SQL equality. The database has a literal
tenant/key unique constraint. Any future equality rule must govern upsert lookup,
concurrent serialization, direct-create/update uniqueness and collision detection
together. Do not change one path independently.

Use the existing PATCH /services/data/v65.0/sobjects/Account/Compat_Key__c/{encodedKey}
route, direct Account POST/PATCH and exact-ID GET for observation. Preserve encoded
slashes, percent signs and Unicode as data; do not introduce custom routes or
patch clients. Use bounded wire transport for raw status/body and pinned unmodified
JSforce for one exact-key create/retry case. Raw HTTP results and client projections
are distinct evidence, not assumed identical.

## Fixed scenario matrix

All keys are unique run-owned values, at most 80 code points. Each case has an
independent ownership marker in Name; no shared records across cases.

1. Exact-key first upsert and identical replay: status/body, record identity and final fields.
2. Existing key with ASCII upper/lower suffix variants: which ID is updated and stored spelling.
3. Direct create with a variant of an existing key: uniqueness result and committed state.
4. Direct update of a second owned record to a variant of the first key: result and both records afterward.
5. Path key versus body key: identical, ASCII case variant and genuinely different values, each in isolated subcases.
6. Leading/trailing spaces: compare an owned base key to one prefixed and one suffixed with a space; do not trim observations.
7. Unicode suffix pairs: composed/decomposed accent, I/i, I/dotless-i, sharp-s/ss. These are distinct subcases; do not extrapolate a universal collation.
8. Encoded slash, percent and non-ASCII suffix: first create and replay with encodeURIComponent exactly once.
9. Two simultaneous first upserts with the exact same key, then ASCII variant keys: five independent two-request rounds for each kind. Preserve each response and final owned record set.
10. Pinned JSforce first upsert and replay on an exact key: client result, identity and final state.

Repeated upserts use the same Name marker and payload so winner ordering cannot
change requested business fields. Concurrency observations may include conflicts;
never demand both operations succeed or label measured races globally deterministic.
Use no load testing, extra tenants, metadata writes or permission assignments.
Do not broaden into null/empty-key routing, restricted-role policy, general SOQL
collation, or arbitrary Unicode equality in this plan.

## Authorization, ownership and recovery

Existing named disposable-reference authorization applies. Require the existing
disposable opt-in and private recovery directory plus endpoint/token variables.
Before mutations, verify current administrator identity via public userinfo and
compare it with a separately saved binding in private config (org/user IDs are
never printed or committed). Check identities again before cleanup and at finish;
identity mismatch stops writes/deletes and leaves recoverable ownership records.
Do not refresh, overwrite or consume Plan 5's preserved session artifacts.

Use fresh cryptographically random run/case markers. Preflight all intended key
variants and markers for existing records; any collision blocks that case before
mutation. Before each potentially creating request, durably journal its exact
marker, intended key variants and request identity. Record returned IDs immediately.
Never automatically replay a failed mutation as a transport retry.

After every mutation batch, independently query by the exact Name marker, validate
returned Name and keys against journaled values, and read discovered IDs directly.
Keep malformed, missing or ambiguous responses as unavailable, not success. A
lost response can still have committed a record: recovery must find it by the
journaled marker. Unexpected/unproven ownership blocks deletion; never delete by
prefix or bulk criteria. Retain private evidence for manual investigation.

A transport/native deadline is not cancellation at the reference server. Journal
request states separately as intent, dispatched, definitive-response-observed and
outcome-unknown. Any dispatched request without a durable definitive response
remains unresolved, even if subsequent marker/ID reads return nothing. It prevents
verified-clean status and run two. Recovery may remove proven owned records, but
finding/deleting one record does not prove an unresolved request cannot commit
later. Exhausting the reconciliation budget returns a blocked report with those
intents retained, never a time-based assumption of quiescence. Resolving such a
block requires separately reviewed terminal-outcome evidence; no arbitrary CLI
flag or elapsed waiting period may override it. This may intentionally leave a
run blocked after response loss.

Finally delete only exact IDs whose ownership is proven, and independently verify
absence by exact IDs and every case marker. Keep journal intents across crashes;
a separate recovery command must support reconciliation and cleanup after process
termination. A run is incomplete until cleanup and absence verification succeed.
Start run two only after verified cleanup of run one. Cleanup cannot use a fresh
identity to silently continue under a different organization.

Bounds: at most 80 potentially created fixture records, 160 non-cleanup mutation
requests and two concurrent requests; 15-second wire requests, 60-second native
JSforce operation deadline, ten-minute collection deadline. Reserve up to five
minutes for cleanup/reconciliation after collection stops. Recovery is independently
runnable and bounded. Use no automatic retry loops for mutations; bounded read
reconciliation may retry at most three times. Failed cleanup remains blocked.

## Evidence artifact and comparison

Separate diagnostic command/report, not a new passing pilot/foundation suite.
Private files 0600 and directories 0700 outside checkout, create-exclusive, no
symlink escape. Strict versioned schema binds API65.0, foundation profile digest,
locked connector versions, scenario manifest digest, run ID and times. Config also
binds the exact HTTPS origin and administrator identity; reject unknown keys.

Each operation retains scenario/round/request IDs, exact input spelling, raw
status/error code, body presence, response artifact digest, source references,
record identity aliases, persisted field observations and explicit unavailable
states. Raw auth headers are never retained. Private raw responses may include
IDs and prose; published summaries contain only synthetic case labels and reviewed
behavior. Reports record collection and cleanup outcomes separately.

Compare two sequential clean runs with equal bindings. Deterministic cases require
matching statuses, presence/absence, exact key spellings and identity relationships
under one-to-one run-owned ID aliasing. Never sort values, lowercase keys or erase
missing fields. Race cases preserve all raw outcomes and use only order-independent
per-round response multisets and final record-count/identity invariants for diagnosis;
matching five rounds is not proof of every possible schedule. Any mismatch or
unavailable required evidence blocks a stable contract claim. No report promotion
to existing compatibility profiles.

## Contract and implementation handoff

Publish verified inputs, outcomes, cleanup status, source digests/time bindings,
unknowns and contradictions. Separate observed behavior from candidate requirements.
For each equality question, say what was tested and what remains unknown. Do not
select PostgreSQL lower(), citext, locale collation or a Unicode folding algorithm
unless the supported contract justifies it. A small observed subset does not prove
an algorithm for every string; unsupported scope must remain explicit.

The next product plan must specify a single matching/locking/uniqueness strategy,
external-ID error/response behavior, cross-tenant tests and a versioned migration
from catalog v2. That migration must preflight collisions under the chosen rule,
abort atomically on any collision and never silently merge, rename or delete rows.
If evidence does not support a safe implementation, deliver precise blockers instead.

Plan 5's unresolved canonicalization, error and ordering evidence is not waived.
Only explicit external-ID observations may support this separate contract; conflicting
or overlapping Plan 5 claims need reconciliation before product implementation.
No cursor-dependent probes or expiry wait are needed for Plan 9 collection.

## Acceptance

Local HTTP transport tests must reproduce a commit after timeout and initial
absence queries (still blocked), response-loss-after-commit, concurrent
creation, missing IDs, changed identity, unknown ownership and cleanup failure.
Schema/comparator tests must reject altered bindings, omitted operations, alias
collisions and incomplete cleanup; exact values and nondeterministic responses
survive comparison. Real reference evidence requires two complete sequential runs
or a truthful blocker report. Final review checks safety and claim boundaries.

## Builder check disposition

A fresh reviewer identified late commits after mutation deadlines as an important
recovery gap. The design now retains unresolved dispatched intents and forbids
verified cleanup/run two until terminal-outcome evidence is established. This is
an executor safety choice, not a new claim about reference cancellation. Other
planning details were sufficient; no further user preference is required.
