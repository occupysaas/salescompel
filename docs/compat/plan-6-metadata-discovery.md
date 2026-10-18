# Plan 6 metadata discovery — 2026-10-18

The read-only discovery tooling is implemented. Two sequential collections produced
matching capability matrices and artifact hashes, but both inventories are
**incomplete**. Formal stability comparison is **blocked**, not accepted parity.
No product routes or behavior changed, and no compatibility observations were
promoted. Plan 5's cursor-expiry evidence remains separate.

## Scope and evidence

API 65.0, the unchanged `rest-query-foundation` profile, and the existing locked
JSforce and Salesforce CLI versions bind each inventory. Both administrator and
restricted identities were verified as distinct users in the same organization,
with verification repeated around collection and CLI retrieval. Optional Profile
and additional PermissionSet names were not requested.

Describe covers Account and Contact and their fixed scoped fields. Administrator
Metadata reads and native CLI retrieval cover only Account.Compat_Text__c,
Account.Compat_Key__c and Compat_Admin. There were no reference writes or record
extraction. Credentials, identities and raw metadata remain outside the checkout
in owner-only Plan 6 storage; Plan 5's saved sessions were not overwritten.

| Private inventory | Collection interval (UTC) | SHA-256 |
| --- | --- | --- |
| first-reviewed.json | 2026-10-18 05:45:16.490–05:45:28.673 | ff58e7985b832136ee3904ae39d85bdbeb2fab70616b82de50f451eba5de5f50 |
| second-reviewed.json | 2026-10-18 05:45:42.110–05:45:53.749 | 1bbe3ea49dab7be801bbb5795a2572495235377274a8947cd75d88ec03bde6e9 |

Each inventory contains 112 facts, 21 gaps and nine retained artifacts. The private
comparison found equal matrix rows, zero conflicts and equal artifact hashes.
`compareMetadataInventories` nevertheless returned blocked because complete
inventories are required. Matching incomplete observations do not establish
complete security coverage or compatibility.

An earlier collection rejected an unexpected native CLI Account container file.
The parser now accepts that file only when it is an empty namespaced CustomObject
container; a regression test rejects nonempty containers. The original inventory
was retained. Both reviewed retrievals succeeded with all three requested members.
Temporary CLI authentication/project directories were removed after collection.

## Recorded gaps

| Scope | Count per inventory | Interpretation |
| --- | ---: | --- |
| Restricted Account custom fields | 2 | Both scoped custom fields were absent from describe; visibility is unknown, not proof of effective denial. |
| Restricted Contact describe | 1 | Read unavailable; no missing-object or effective-access conclusion. |
| Metadata read declarations | 8 | Six scoped standard-field grants and two object grants were omitted. |
| CLI retrieved declarations | 8 | The same declarations were omitted; no false/default grants fabricated. |
| Optional Profile/additional PermissionSet | 2 | Explicitly unrequested, not required failures. |

Declared permissions and persona describe capabilities remain separate matrix
scopes. A declaration alone does not prove effective authorization. No access
rule is inferred from missing declarations, failed reads or an absent field.

## Smallest follow-on slice

Both public clients report the synthetic fields as Text with length 80 and
required false. Compat_Text__c declares externalId false and unique false;
Compat_Key__c declares externalId true, unique true and caseSensitive false.
The candidate's fixed catalog currently uses length 255 and does not represent
uniqueness/case sensitivity. Its role-based visibility also differs from the
observed restricted describe scope.

The next product design should therefore cover explicit field metadata and
explicit synthetic grants, starting with these fixtures. Acceptance examples:

- Preserve the observed field lengths and explicit false values in describe and
  metadata representations.
- Represent external-ID uniqueness and case sensitivity explicitly, with
  behavior tested before claiming enforcement parity.
- Represent declared grants separately from effective access; retain unknown
  states where discovery cannot establish a rule.
- Keep existing Salesforce paths, methods and API versions for drop-in clients.

Query permission enforcement, canonical field/object resolution, ID handling and
HTTP error behavior depend on Plan 5's reviewed evidence. Those dependencies must
be resolved before expanding the product slice; this document does not authorize
speculative enforcement rules or claim M4 completion. A separate design and plan
are required for that implementation.

## Provenance and validation

Collection uses the documented public JSforce describe and Metadata read APIs
([JSforce documentation](https://jsforce.github.io/document/#metadata-api)) and
flags verified against the installed pinned CLI's public `project retrieve start`
help. No Salesforce CLI/plugin implementation was inspected.

The final Node check after review corrections passed: 88 tests passed, one
optional candidate test skipped, zero failed; typecheck, formatting and repository
validation passed. Pilot and foundation profile bytes match the planning baseline
`b84e999`. This is lab-tool verification, not product parity certification.

## Review disposition

The fresh read-only review found two Important issues and no Critical or Minor
issues. Both were reproduced with failing tests and corrected: unsuccessful CLI
retrieval now scans its fresh isolated output and preserves validated components
and missing-member gaps; completion now requires all scoped operations and
required projected properties. A truncated inventory cannot compare as stable.
Successful live collection semantics did not change; the existing incomplete
inventories remain incomplete, so no new live collection was needed.

Review exclusions were resolved explicitly: effective authorization remains out
of scope; live artifacts were verified by the executor rather than shared with
the reviewer; the results/log documentation was checked by the executor after
the review; CLI/plugin implementation remains excluded by the clean-room rule.
There are no deferred review findings.

Execution rulings: native sequential work on the existing branch follows the
recorded maintainer preference. Bounded child processes allow terminating JSforce
reads; credentials are passed on stdin. CLI retrieval reuses isolated auth with
the existing six-minute default timeout and locked XML parser. The command
performs additional identity verification around the entire sequence. Only the
strictly empty native Account container is accepted beyond requested files.
Missing declarations remain gaps. Task 5 closes with precise live blockers,
as allowed by the plan; it does not close discovery acceptance.
