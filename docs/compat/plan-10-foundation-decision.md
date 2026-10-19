# Plan 10 foundation evidence decision

Status: **blocked for live evidence**, recorded 2026-10-19 16:43 UTC. This is a
development decision, not observed-only evidence registration, a frozen Plan 5
contract, or compatibility certification.

The compatibility lab now has an explicitly selected collection policy for the
maintainer-authorized `foundation-query-result-lifetime` assumption. It removes
only the `expiry-start` and `after-16-minutes` observations and the associated
sixteen-minute wait. The raw cursor scenario remains blocked with expiry
unmeasured. A private sidecar binds capture bytes and the executed probe, manifest,
profile, API, clients and administrator identity digest. A separate assessor
requires two fresh sequential runs, exact coverage, valid cleanup/absence
attestations and the narrow invariant-checked `ties` comparison. Existing
observed-only capture, comparison and evidence registration remain unchanged.
The post-implementation review tightened this gate: every owned fixture ID
must appear under its scenario in cleanup and exact-ID absence evidence;
authentication failures and malformed positive reads cannot count as measured
responses. The first absence check must precede the second capture.

The official API Query Cursor Limits guide and current Developer Limits PDF
document two-day cursor/result availability at API 65.0. The maintainer authorized
using that fact as an **assumption**, without a live expiry wait longer than one
hour. The exact expiry boundary, clock anchor, eviction behavior and effect of
later retrieval remain unknown. The release-pinned Winter 23 Help page still
exposes no readable content; focused source results and unresolved findings are
in [release changes](releases/changes.yaml).

| Selected dimension | Status | Basis or blocker |
| --- | --- | --- |
| Query response shape, LIMIT/batch/totalSize, routes | blocked | No fresh corrected capture pair |
| ID and name behavior, duplicate canonical fields | blocked | No fresh corrected capture pair |
| Query text equality and null ordering | blocked | No fresh corrected capture pair; these observations are separate from Plan 9 external-ID equality |
| Error and unknown-object responses | blocked | No fresh corrected capture pair |
| Cursor replay, mutation, session and malformed locator behavior | blocked | No fresh corrected capture pair |
| Complete tied traversal and narrow order comparison | blocked | No fresh corrected capture pair |
| Cursor result lifetime | assumed | Documented two-day availability; named maintainer waiver; no exact live boundary measured |
| Relevant release reconciliation | blocked | `winter23-query-cursor-retention` awaits live reconciliation and `release-content-unavailable` retains its owner/next action |

The current foundation profile is `rest-query-foundation`, SHA-256
`99d90e50fc63573f08f5a48da878eb9c129b7d660eb9ed5d600306b9706bd8f8`,
API `65.0`. Pinned clients are JSforce `3.11.0`, Salesforce CLI `2.156.0`,
plugin-data `5.1.16`, plugin-deploy-retrieve `4.2.15`, and simple-salesforce
`1.12.9`. The probe and coverage-manifest digests must be recomputed from the
final reviewed code before any live run; the sidecar and assessor enforce that
binding. There are **no new capture, policy, cleanup or assessment digests or
times** to cite because no Plan 10 live run started.

The offline foundation preflight exited 2 without making reference requests.
The process lacks `COMPAT_REFERENCE_URL`, `COMPAT_REFERENCE_TOKEN`,
`COMPAT_REFERENCE_FRESH_TOKEN`, `COMPAT_REFERENCE_RESTRICTED_TOKEN`,
`COMPAT_REFERENCE_EXPECTED_ORG_ID`,
`COMPAT_REFERENCE_EXPECTED_ADMIN_USER_ID`,
`COMPAT_REFERENCE_EXPECTED_RESTRICTED_USER_ID`, `COMPAT_API_VERSION`,
`COMPAT_DISPOSABLE=yes` and `COMPAT_RECOVERY_DIR`. The expected IDs are private
operator pins for the authorized tenant and personas; the live preflight compares
all three before fixture writes. Named-tenant
authorization remains recorded in the foundation readiness file, but live
administrator/fresh/restricted identities, deployed metadata and recovery state
could not be rechecked in this process. No fixture writes, cleanup attempts,
second session fabrication or late expiry checkpoints occurred.

When the private sessions and recovery directory are made available, re-run the
reference preflight; inspect unresolved recovery state; collect the first fixed
policy capture; run the read-only `verify-foundation-cleanup` command and review
its journal and exact-ID absence evidence. That command writes a private,
capture-bound absence marker in the recovery run directory; the next capture
rechecks every foundation recovery journal digest, the identity pins and exact-ID
absence before it writes fixtures. Then collect and verify the second
run sequentially. Run `assess-foundation` with both captures, sidecars and cleanup
attestations, and publish only a sanitized reviewed outcome. Any relevant
unresolved release finding remains a separate development-decision blocker.

The independent Plan 9 body-field validation follow-up can be designed next,
subject to its own missing-record reference cases and scoped product fix. It does
not inherit Plan 5 query-text observations or change this blocked decision.
