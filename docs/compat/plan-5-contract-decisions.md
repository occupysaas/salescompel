# Plan 5 contract decision gate

Status: **not frozen**. The first focused capture finished with six passing scenarios and one blocked expiry dimension; the second capture has the same outcome. Pair verification is blocked; no accepted capture pair exists. Candidate semantic
implementation (Tasks 4–8) remains pending the remaining evidence decisions.
The maintainer has waived live verification of expiry windows longer than one
hour; the documented two-day cursor lifetime may be used as an explicit assumption.
This exception does not turn old reports into passing observations.

## Current evidence

- Tasks 1–2 implement bound suite selection and focused wire probes. Existing pilot
  profile bytes and all eleven accepted pilot observations are unchanged.
- The saved administrator and restricted sessions passed public userinfo checks.
  Private credentials are retained outside the repository; no fixture writes ran.
- Two distinct same-administrator sessions now pass public userinfo checks in the
  same organization. Restricted-user identity and field exclusion were verified.
  Readiness is profile-bound; captures and recovery journals remain private.
- The archive's 71 direct document links are inventoried in releases/index.yaml.
  PDF bodies remain unavailable; archive content review is incomplete. A focused
  official Winter 23 HTML release note was retrieved and reviewed separately:
  releases/changes.yaml records the two-day query-result retention change.
- The current sixteen-minute cursor probe cannot establish the documented expiry
  boundary. Continued validity must remain partial evidence, not an expiry pass.

## Decisions that remain evidence-dependent

Wire status/code/body fields; exact ID/name rules; text comparison and null-order
semantics; batch/LIMIT/totalSize behavior; cursor mutation/replay/expiry rules;
cursor storage representation and operational limits. None are inferred from
passing offline tests. Obtain two sequential focused captures with cleanup,
repeatability and reviewed reciprocal evidence links before freezing these rules.

## Review corrections before live use

A fresh code review identified duplicate normalization identities, discarded
resource-discovery links, and overwritten intermediate PATCH outcomes. Regression
tests reproduce each issue. Equivalent input variants now share one identity,
discovery links remain comparable, and each name/duplicate-key PATCH is followed
by an independent state read. These changes improve probe fidelity; they are not
reference observations or candidate compatibility certification.

## First capture review (provisional)

This is a single-run review, not registered compatibility evidence or a frozen
contract. Both captures used the same suite/profile; later probe corrections require new captures.

- Capture interval: 2026-10-17T21:09:27.561Z to 2026-10-17T21:49:43.303Z.
- Raw report SHA-256: `83e5529e5ec9bd67154a548960eb877d24f8c00e289cf004d88b476df3817ca9`.
- Six scenarios passed. All 28 cursor assertions passed, but cursor expiry stayed
  blocked because the locator remained valid at the sixteen-minute boundary.
- All 2,418 cleanup actions passed. A separate read-only check queried every
  owned ID in batches and confirmed zero remaining active Account/Contact records.
- Default pagination returned 2,000 then 1 row with totalSize 2,001. LIMIT 5000
  retained both pages. Requested batch 200 traversed eleven pages without loss.
- Cursor replay, concurrent retrieval and a distinct same-user session succeeded;
  the restricted user received INVALID_QUERY_LOCATOR. A pending record's updated
  field value appeared in continuation. A snapshot of original field values alone
  would not reproduce that observation; membership/deletion rules need care.
- Both duplicate spelling aliases and duplicate raw JSON keys were accepted in
  these probes, with the second value visible afterward. Denied aliases still
  failed and the denied write left the record unchanged.
- Default null placement was first for both ascending and descending probes.
  Empty-string input read back as null. Unicode equality differed by input:
  decomposed accent and Turkish lowercase variants did not match in this profile.
- Exact expiry, repeatability, release reconciliation and candidate implementation
  remain incomplete. No existing contract has been promoted from these notes.

## Second capture and repeatability diagnosis

- Second interval: 2026-10-17T22:39:06.054Z to 2026-10-17T23:16:25.386Z.
  SHA-256: `8b49348df5498539908e9a837ff729a2e452adb54e78dee7768b5d73a4d4b33b`.
- Again six scenarios passed; cursor expiry remained blocked. All 2,418 cleanup
  actions passed and independent exact-ID queries found zero remaining records.
- Official repeatability and pair verification both returned blocked because
  cursor evidence is incomplete. A separate diagnostic comparison, without
  altering the reports or their outcome, found additional ordering differences.
- The omitted-Id query had LIMIT without ORDER BY and selected different 201-row
  subsets. The name-equality query returned the same four rows in different order.
  Both probes now order by their unique fixture text values; regression checks
  reproduced the missing ordering before correction. These edits require fresh
  captures and do not retroactively validate the old pair.
- The deliberate equal-key cursor probe returned all 201 unique rows in both
  runs, but in different order and page partitions. Raw order is preserved.
  A comparison rule for ties remains unresolved; exact equality across these
  runs is not established. Do not silently sort away this discrepancy.
- Read-only expiry follow-up checkpoints are retained privately with report
  digests and same-user verification. The conservative first checkpoint is
  2026-10-19T21:54:43Z; both are due by 2026-10-19T23:21:26Z. These are capture
  completion plus two days and five minutes, not precise locator creation times.
  They are prepared commands, not a scheduled background job. No fixtures remain
  active for these checks. A later invalid-locator response supplies an upper
  bound, not an exact lifetime measurement.

Task 3 remains incomplete: expiry evidence, tie-order comparison semantics,
new captures after probe corrections, and relevant release reconciliation are
outstanding. Tasks 4–8 remain gated. No capture is currently running.

## Follow-up readiness check — 2026-10-19 UTC

At 02:24 UTC, the retained-checkpoint helper reported that neither expiry check
was due and exited before authentication or reference requests. The first remains
due at 21:54:43 UTC (17:54:43 America/New_York), and both are due by 23:21:26 UTC
(19:21:26 America/New_York). No background job was started or capture modified.

The official [API Query Cursor Limits](https://developer.salesforce.com/docs/platform/salesforce-app-limits-cheatsheet/guide/salesforce-app-limits-platform-apicursors.html)
page was successfully re-read: it still describes two-day result availability and
removal of the former cursor-count/inactivity limits across API versions. This
corroborates the documentary expectation only; it does not establish live expiry.
The release-pinned Winter 23 Help link returned a loading/CSS-error shell on this
attempt. Earlier recorded source evidence is retained; this attempt does not
resolve the broader release-content or linked-changelog blockers.

Priority for the next design is to close available evidence follow-ups before
product changes: define explicit tied-order diagnostic comparisons, prepare
immutable expiry checkpoint verification, and scope fresh corrected captures.
An expiry checkpoint on the old reports cannot make those reports acceptable after
probe corrections, and must not silently change their blocked outcomes.

## Maintainer expiry policy — 2026-10-19

The maintainer explicitly accepts assumptions without live testing for expiry
windows longer than one hour. Do not wait for those windows or require their
checkpoints to proceed. For the foundation cursor lifetime, use documented
two-day availability as a named, unverified assumption. One hour limits test
waiting, not the intended product lifetime. The exact expiration boundary and
its lifecycle anchor remain unobserved; any candidate choice must be labeled.

The previously prepared late checkpoints are optional and are not scheduled.
Preserve original captures and their blocked status. Record assumed versus
observed dimensions separately; this waiver does not cover identity, cleanup,
missing responses, ordering loss/duplication or unrelated compatibility gaps.
Local candidate expiry behavior may be tested with a controllable clock without
wall-clock waiting; those tests verify our implementation, not reference parity.

## Plan 10 focused release recheck — 2026-10-19

The official API Query Cursor Limits HTML guide and current official Developer
Limits PDF both state two-day cursor/result availability and removal of the old
fifteen-minute inactivity and per-user cursor-count limits at API 56.0, applicable
to REST API 65.0. An official December 2022 developer article independently
describes the Winter 23 change. The release-pinned Winter 23 Help page again
returned only a loading/CSS-error shell. Exact URLs, versions, access results
and separate summaries are recorded in `releases/changes.yaml`.

Current official REST/SOQL guides were readable for continuation, batch headers,
ties, string comparisons, error codes and ID shape. They are current contract
context, not a complete historical release review. The two-day documentation
fact does not establish the precise live boundary, clock anchor, eviction or
whether later retrieval changes retention. The maintainer's authorization to
proceed with this one named assumption is a separate decision. The existing
`release-content-unavailable` and `linked-changelog-expansion` blockers retain
their owners and next actions; the focused recheck does not complete M1R.

## Plan 10 implementation gate — 2026-10-19

The assumption-aware lab path is implemented, but no corrected foundation
capture pair was collected. The current process has no reference sessions,
API/disposable runtime setting or private recovery directory; offline preflight
exited 2 before any reference request. Therefore the assumption-aware assessment
has no capture, sidecar or absence evidence to consume, and the development
decision is blocked. See [Plan 10 decision](plan-10-foundation-decision.md) for
the exact evidence gate and next product boundary. Historical blocked captures
and accepted profiles remain unchanged.

This policy supersedes the earlier live-expiry acceptance requirement in Plan 5.
The next design must provide a distinct assumption-aware development decision;
existing observed-evidence validators must not silently treat assumptions as passes.

## Documentary follow-up — 2026-10-21

[Plan 18](plan-18-foundation-release-reconciliation.md) records fresh official-source attempts and current-guide context for all twelve foundation contracts. Release-pinned bodies remain unavailable; historical reconciliation and all existing live gates remain blocked. This dated follow-up qualifies the earlier source-access record without replacing its observations or the existing cursor-lifetime waiver.
