# Plan 4 reference baseline

Date: 2026-10-16. Status: pilot baseline accepted; broader roadmap milestones and
candidate Salesforce parity remain unverified.

The authorized disposable trial was exercised through public APIs and unmodified
pinned clients. Two sequential reference captures passed all eleven pilot
scenarios, repeatability and pair verification. The reviewed evidence is
[pilot-black-box-baseline](observations/pilot-black-box-baseline.yaml).

## Setup and reproduction

[The profile](profiles/pilot.yaml) pins API 65.0, client versions, installed package
versions and available test settings. [Readiness](reference-readiness.yaml) binds
its exact byte hash to the existing authorization. All eleven scenarios remain
required. The profile's historical `referenceStatus: pending` is intentionally
unchanged: validated contract links, rather than that field, report observations.
Do not edit profile bytes without rebinding readiness and recapturing evidence.

Three dedicated synthetic users provide administrator, restricted and locale
personas. Independent CLI web sessions verified the administrator and restricted
users belong to the expected tenant and have different identities. The restricted
user has API access and Account read without the synthetic text-field grant or
Account write/view-all/modify-all permissions. Positive Name access and negative
text-field access were exercised through its own session.

Persistent fixtures are the two synthetic Account fields, Compat_Admin permission
set, a dedicated restricted permission set and the three test users. They are
retained for future authorized runs; their IDs and setup ownership journal remain
private. Runtime record fixtures and per-run metadata fields were cleaned up.
No primary account was repurposed or reset.

Use the private environment and sequential commands in the
[reference runbook](README.md#plan-3-reference-authorization-and-evidence).
Keep raw reports and recovery journals outside the checkout with owner-only
permissions. Refresh saved CLI sessions when needed, then export access tokens
privately using `sf org auth show-access-token --json`; `sf org display` redacts
its token field in the pinned CLI. No tokens belong in command arguments or chat.

## Verification

| Check | Result |
| --- | --- |
| Full offline compatibility check after profile binding | 48 passed; 1 optional candidate test skipped; TypeScript, formatting and repository validation passed |
| Reference preflight | Exit 0 with verified profile and private runtime configuration |
| First reference capture | 11/11 passed; all reported cleanup passed |
| Second reference capture | 11/11 passed; all reported cleanup passed |
| Repeatability and pair verification | Both passed; no new normalization added |
| Independent cleanup reads | 414 owned Account/Contact records absent from normal queries; both locale settings restored; both run-specific fields absent from metadata read |
| Candidate smoke | JSforce, Python HTTPS CRUD, service restart, CLI query and 4 browser tests passed |

Normal-query absence establishes deletion, not permanent purge from the recycle
bin. Deleted custom fields may likewise remain recoverable in Salesforce setup.
Persistent setup fixtures are excluded from the runtime cleanup count. The
rollback requests separately verified that their owned key had no committed row.
Capture times, digests and exact connector bindings are in the evidence record.
No full candidate-versus-reference comparison was claimed: the candidate still
lacks several mandatory pilot surfaces.

## M3 handoff

Candidate behavior below is the documented Plan 2 scope, corroborated by the
current smoke run where applicable. The pilot does not measure every deferred
contract. Those contracts remain `not_run` until focused reference probes exist.

| Contract ID / work item | Evidence reference | Observed reference behavior | Current candidate behavior | Required change | Remaining evidence gap |
| --- | --- | --- | --- | --- | --- |
| `query-response-shape`, `query-limit-above-batch` | Baseline `relationships` (partial only) | 201 related Contacts traverse two pages when batch size 200 is requested | No cursor support; explicit LIMIT is bounded at 2000 | Add tenant-bound query continuation and client pagination | Complete response shape, default batch size, LIMIT above 2000, cursor expiry/replay, concurrent changes and cross-user access |
| `id-15-18` | None | Not measured | Accepts 18-character IDs only | Add validated 15/18-character handling where reference requires it | Accepted forms, casing/checksum failures, wrong-object IDs and route-specific errors |
| `case-insensitive-names` | None | Not measured | Object/field names are case-sensitive | Resolve names to catalog canonical names as observed | REST versus SOQL behavior, canonical output casing, unknown names and denied-field aliases |
| `case-insensitive-comparison` | None | Not measured | Text equality is case-sensitive | Implement observed text comparison semantics | Case, accents, locale, external-ID uniqueness and null behavior |
| `null-ordering` | None | Not measured | NULLS LAST in both directions | Implement observed defaults and explicit null-placement syntax | ASC/DESC defaults, explicit options and stable tie ordering |
| `error-shape`, `unknown-sobject` | Baseline `errors`, `crud`, `permissions` (codes only) | INVALID_FIELD, MALFORMED_QUERY and NOT_FOUND; captured JSforce statuses are null | Structured candidate errors; documented unknown-object behavior awaits parity evidence | Match observed wire status/code/body per route | Raw HTTP statuses and body fields; malformed JSON, unknown REST object versus SOQL object, size limits |
| `upsert-outcomes` | Baseline `upsert` (partial only) | First call creates; second updates; one final record | Unique external-ID upsert exists | Preserve existing behavior and adjust only observed differences | HTTP status/body, duplicate matches and body Id handling |
| `describe-properties` | Baseline `describe` (partial only) | Fixture field type and external-ID flags observed | Fixed Account/Contact catalog | Expand tested properties as required | Full createable/updateable/nillable/unique/length/URL contracts |
| Served OpenAPI | Repository requirement; not reference behavior | Not applicable | Proposed broader document exists but is not served | Serve an accurate implemented-surface document, retain broader proposals explicitly, add provenance and request/response checks | Select and verify validation support for the repository's latest-version policy before contract freeze |
| `rollback` | Baseline `rollback` | Composite subrequests report 400 with PROCESSING_HALTED/INVALID_FIELD; no committed row | Composite unsupported | Add transactional composite behavior in a subsequent M3 slice | Partial success, locking and broader rollback cases |
| `metadata`, `locale` | Baseline records with these IDs | Owned field deploy/read/retrieve/delete; independent locale update/restore | Metadata and User-setting writes unsupported | Deliver in appropriate M4/localization work packages | General metadata/security and wider language/currency behavior |
| `cli-query`, `python-crud`, `permissions` | Baseline records with these IDs; candidate smoke | Existing pinned clients and restricted field test succeed | Walking-slice clients and synthetic restricted-role behavior pass smoke | Retain these as regression gates while widening protocols | Broader CLI channels/plugins, OAuth lifecycle and full security matrix |

Start the next product work package with focused REST/query evidence, then implement
pagination, IDs, name/text/null semantics, error shapes and served OpenAPI in
reviewable slices. Reconcile M1R release-history findings before freezing those
contracts. SOAP, Bulk, OAuth, recycle-bin behavior and the remaining M3 scope need
later work packages; the REST/query foundation alone cannot close M3.

## Limits and retained work

- This is a synthetic pilot, not full M0/M1 completion or production certification.
- `iw` was a synthetic profile value unavailable in this tenant's User describe.
  It was removed before capture; its offline legacy-code catalog/contract remains
  intact and unobserved. The eleven-scenario manifest was not reduced.
- Installed packages and active platform/package flows are environment context;
  their internal behavior, customization coverage and migration are not certified.
- Currency, broader locale behavior, scale/downtime objectives, CLI distributions,
  plugin runners/routing and the M1R archive review retain their roadmap gates.
- Error codes observed through JSforce do not establish missing HTTP statuses.
  Deferred REST/identity contracts were not promoted to observed.
