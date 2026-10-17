# Plan 5 contract decision gate

Status: **not frozen**. The first focused capture finished with six passing scenarios and one blocked expiry dimension; the repeat capture is running. No accepted capture pair exists. Candidate semantic
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
contract. The second capture is running against the unchanged suite/profile.

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
