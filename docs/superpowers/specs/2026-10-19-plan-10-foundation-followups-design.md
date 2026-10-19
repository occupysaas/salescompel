# Plan 10 — Foundation evidence follow-ups

Status: proposed design for review. No product implementation is authorized by
this document. Milestone: M1/M1R follow-ups supporting the M3 foundation.

## Intent and decisions

The maintainer wants outstanding follow-ups prioritized before new features and
explicitly accepts assumptions without testing for expiry windows longer than
one hour. Preserve the same routes for one-to-one drop-in clients.

The executor recommends closing the available Plan 5 evidence gaps first, then
returning to the narrow external-ID request-validation fix identified by Plan 9.
This design covers the former. No product code, catalog migration, normalization
algorithm, route changes, dependency upgrades or new client adapters are included.
Use the existing TypeScript lab and pinned, unmodified clients on main, with
native sequential execution. These implementation choices are the executor's.

Deliver an auditable development decision separating observed behavior,
authorized assumptions, unresolved differences and unavailable evidence. Success
is either a bounded decision ready for the next product design or an exact list
of remaining blockers; it is not a requirement to force a passing comparison.

## Expiry assumption and testing policy

For reference expiry windows longer than 3,600 seconds, do not wait for live
expiry, schedule background polling or require old late checkpoints to proceed.
Use a named assumption with scope, duration, source, authorization and unknowns.
For this plan the only waiver is `foundation-query-result-lifetime`: documented
two-day availability, API 65.0, authorized by the maintainer on 2026-10-19.
One hour caps waiting for an expiry observation; it does not shorten the intended
product lifetime or waive unrelated timeouts, safety or identity checks.

The official API Query Cursor Limits page was readable on 2026-10-19 and states
two-day availability. The old Winter 23 release-pinned Help link returned a shell
on recheck; keep its prior recorded evidence and the new access limitation.
Source: https://developer.salesforce.com/docs/platform/salesforce-app-limits-cheatsheet/guide/salesforce-app-limits-platform-apicursors.html

Exact boundary, eviction details and whether later retrieval affects retention
remain unobserved. This plan does not select a candidate clock anchor or claim
an exact 48-hour reference boundary. A subsequent cursor implementation design
must state its own anchor explicitly. Fast local clock-controlled tests remain
appropriate for candidate behavior, without claiming live reference verification.

Old captures and prepared late-checkpoint artifacts remain immutable and private.
Do not run the late checkpoint helper as part of this plan. Its absence cannot
block development under the accepted assumption. Historical blocked outcomes and
accepted pilot/foundation profile bytes remain unchanged.

## Foundation collection and separate assessment

Add an explicit, fixed foundation collection policy for the authorized assumption.
Under that policy omit the sixteen-minute expiry-only delay and its two dedicated
observations, since continued validity at sixteen minutes cannot verify the
assumed two-day window. Keep replay, mutation, session/identity, unknown/malformed
locator and complete traversal checks. Default collection behavior stays intact;
the policy must be explicitly selected and recorded in an immutable sidecar.
Do not offer arbitrary skipped-scenario or waiver flags.

Retain the existing capture format and observed-only acceptance behavior. A
capture using the policy must not be labeled fully observed/passing: the cursor
scenario records its unmeasured expiry dimension explicitly. Add a separate
strictly validated assessment artifact and command/function. Do not rewrite a
capture, forge assertions or weaken existing compare/verify-evidence semantics.

The policy sidecar binds capture SHA-256, current foundation profile/API/client
versions, actual probe source/manifest digest, fixed policy version, assumption
identifier and collection times. Publish it only once the capture bytes exist;
an interruption before publication leaves an incomplete run, not eligible evidence.
Private files/directories remain 0600/0700 outside the checkout, create-exclusive,
with validated ownership and no symlink escape. No tokens enter artifacts.

The assessment consumes two distinct sequential fresh captures, their sidecars,
current binding and recorded cleanup/absence evidence. It checks fixed expected
scenario, observation and assertion coverage from the current probe version,
not an arbitrary reason string, assertion count or user-supplied skip list.
Only the two explicitly omitted expiry observations are exempted. Every other
required observation must exist with its expected shape, and every assertion
must pass. Authentication/session failures, unknown outcomes, malformed reports,
missing operations or incomplete cleanup remain blockers. Old corrected-away
captures may be examined diagnostically but cannot satisfy the fresh-pair gate.

Assessment outcomes are `ready-with-assumptions`, `drift`, or `blocked`:

- ready-with-assumptions: all required measured dimensions agree under the narrow
  comparison rules, cleanup is verified, and the named expiry assumption is present;
- drift: complete measured evidence contains unexplained cross-run differences;
- blocked: evidence, authorization, binding, coverage or cleanup is incomplete.

Any relevant unresolved release finding also blocks the development decision.
The assessment reports those findings separately from pair-comparison results.
A ready decision permits the next product design to consume the measured contract
plus the named assumption; it is not an observed-only evidence registration,
full Plan 5 completion, contract certification or permission to ignore other gaps.
No automatic promotion of existing evidence links or profiles is allowed.

## Tied ordering: narrow comparison rule

The known `ties` fixture intentionally orders 201 records by one identical value.
Preserve raw order, record-to-page associations, statuses and continuation routes.
Within each capture, require complete traversal, successful pages, valid done/link
transitions, expected totalSize, all 201 expected owned IDs exactly once, correct
projected values and no foreign, missing or duplicated identities. Keep existing
batch constraints and terminal-page checks. Same-locator replay/session checks
are separate and remain order-sensitive; do not apply this rule to them.

Across captures, compare the tied traversal by the full multiset of aliased
record payloads after checking those invariants. Different tied row order/page
membership is a reported diagnostic, not a semantic drift by itself. This is an
explicit development comparison policy for this one fixed fixture, not evidence
of arbitrary reference ordering guarantees. Do not sort arbitrary arrays or
relax comparisons for unique ordering, non-tied groups, other query scenarios,
Plan 9 races, statuses, spelling, missing fields or error properties.

Tests must prove that a tied permutation can be accepted while loss, duplicates,
changed values, failed pages, invalid continuation, truncated traversal and
changes to unrelated ordered queries still fail. Keep raw reports untouched.

## Focused release reconciliation and live work

Limit release review to the existing Plan 5 selected contracts/API 65.0 and
known relevant release findings. Retry accessible official HTML/PDF sources and
record exact URL/version/access result and independent summaries. Do not attempt
the entire 71-document historical archive or claim M1R completion. Resolve a
finding only when source content supports that resolution; unavailable sources
retain their owner, reason and next action. Keep the two-day documentation fact
separate from the maintainer's decision to proceed without a live expiry test.

Before fresh captures, recheck existing named-tenant authorization, all required
administrator/fresh-session/restricted identities, metadata prerequisites and
recovery state. Existing authorization applies; never inspect CLI/plugin source.
Do not overwrite saved Plan 5 session artifacts or silently replace an identity.
If credentials are unavailable, record the exact blocker rather than collecting
partial evidence or manufacturing a second session.

Run two corrected captures sequentially, starting the second only after verified
cleanup of the first. Use existing bounded fixture journaling and exact-ID
ownership checks. Preserve limits, credentials and raw evidence privately. Do
not retry potentially committed mutations automatically. Do not run a new capture
when unresolved fixture/recovery state exists. No cleanup or identity waiver is
introduced by the expiry policy.

Review the new assessment and actual outcomes, then publish a sanitized decision
record bound to capture/policy/probe/profile/client digests and times. Every
selected dimension is labeled observed, assumed, differing or blocked. Record
precise next product boundaries; no equality, cursor storage or migration choice
is invented to close a checklist. In particular, do not merge Plan 5 query-text
comparison observations with Plan 9 external-ID equality observations.

## Verification and completion

Regression tests cover the assumption's exact allowlist, missing sidecars,
source/profile/client drift, malformed/missing operations, stale captures,
immutable old reports, cleanup failures and tied-order invariants. Show relevant
regressions fail before implementation. Run the full Node check and diff checks;
confirm product routes/code/catalog and accepted profiles remain unchanged.
Request one fresh whole-change implementation review and resolve Important or
Critical findings. Report live results honestly, including any remaining gate.

Plan 10 ends at reviewed evidence and the assumption-aware development decision.
If follow-ups are blocked externally, document the blocker and identify whether
the independent Plan 9 body-field validation follow-up can proceed next. Its
missing-record reference cases and product fix require a separate scoped design;
this plan does not expand itself into that work or the full Plan 5 product build.

## Builder check

A fresh software-engineer review found no important unanswered planning questions.
Artifact schemas, command names and digest construction remain implementation
choices within the boundaries above. No product code was changed for this design.
