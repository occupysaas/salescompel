# Plan 5 contract decision gate

Status: **not frozen**. The first focused capture finished with six passing scenarios and one blocked expiry dimension; the second capture has the same outcome. Pair verification is blocked; no accepted capture pair exists. Candidate semantic
implementation (Tasks 4–8) remains pending the evidence gate in the approved plan.

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
