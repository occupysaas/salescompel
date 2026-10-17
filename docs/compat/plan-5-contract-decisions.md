# Plan 5 contract decision gate

Status: **not frozen**. The first focused reference capture is in progress; no complete capture pair exists. Candidate semantic
implementation (Tasks 4–8) remains pending the evidence gate in the approved plan.

## Current evidence

- Tasks 1–2 implement bound suite selection and focused wire probes. Existing pilot
  profile bytes and all eleven accepted pilot observations are unchanged.
- The saved administrator and restricted sessions passed public userinfo checks.
  Private credentials are retained outside the repository; no fixture writes ran.
- Two distinct same-administrator sessions now pass public userinfo checks in the
  same organization. Restricted-user identity and field exclusion were verified.
  Readiness is profile-bound; the first capture is in progress with private journals.
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
