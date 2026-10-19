# Plan 1 implementation log

Plan: docs/plans/2026-10-15-plan-1-compatibility-laboratory.md

- Approved plan saved with OpenAPI 3.0-only additions.
- Ruling: use a feature branch in the shared checkout; keep changes reviewable without moving the user into another directory. Superseded 2026-10-15: all work is committed on `main` until every roadmap milestone is complete.
- Ruling: OpenAPI API documentation and External Services import are separate surfaces. Schema validation is not runtime certification.
- Ruling: local tests can exercise synthetic observations; live connectors must never silently substitute fixtures.
- Pre-flight: scenario IDs, profile requirements, result format, comparator and validator share one registry.

## Delivered

- Task 1: complete — profile, provenance register, behavior contracts and source references.
- Task 2: complete — locked Node/TypeScript and Python connector harnesses; explicit blocked outcomes.
- Task 3: complete — eleven executable scenarios, bounded fixture ownership, private recovery journals and cleanup reports. Live observations remain pending.
- Task 4: complete — per-language fixture catalogs, schemas, ICU validation, fallback/legacy-code contracts.
- Task 5: complete — JSON/Markdown reports, strict profile-bound comparator, hosted offline CI and configured self-hosted live CI.
- Task 6: complete — OpenAPI 3.0-only validation, proposed REST contract, synthetic External Services fixtures, official Salesforce Einstein Bots YAML with unchanged bytes, commit/checksum and BSD license.
- Review: an independent reviewer identified and checked fixes for incomplete comparison manifests, plugin package resolution, cleanup evidence, positive permission controls and crash recovery. Remaining recovery-path hardening was implemented and regression-tested.
- Verification: clean npm ci succeeded; 24 offline tests pass on Node 22 and the local Node 26 runtime, TypeScript and formatting checks pass, repository/OpenAPI validation passes, Python pip check and byte compilation pass. Live Salesforce/API/CLI scenarios have not run against a real org or a SalesCompel server.
- Dependency risk: remaining Salesforce CLI toolchain advisories are recorded in dependency-audit.md; no audit-clean claim.

## Review pass — 2026-10-15

A plan/code review found live-run defects and roadmap gaps. Fixed: Metadata API verification for the `metadata` scenario, HTTPS blocking and journal-based cleanup adoption for the Python probe, a shared deleted-record error-code rule, a `repeatability` command for the M1 gate, profile-pinned connector versions, validated deferred contracts, missing OpenAPI discovery/userinfo/collections paths and `CLEAN_ROOM.md`. Offline tests: 29 passing. Deferred items are in [plan-1-followups.md](../plans/plan-1-followups.md). Live verification remains pending.

## Plan and documentation review — 2026-10-15

A review of the roadmap, Plans 1–2 and the contracts against Salesforce documentation (much of this material was originally drafted with another AI assistant) recorded these rulings:

- Ruling (user): all work stays on `main` until every milestone is complete; only then is the project usable and testable by humans.
- Ruling (user): SalesCompel uses the latest OpenAPI version (initially stated as 3.0, then revised the same day to "whatever the latest version is": 3.2.0). Wherever Salesforce uses an older version, SalesCompel replaces it with a latest-version equivalent (as in the zeroauthy clean-room project); Salesforce 2.0/3.0 External Service documents are accepted as input and converted (`external-openapi-conversion`). `api/openapi.yaml` moved to 3.2.0, and the validator moved from `@apidevtools/swagger-parser` 13.1.0 (3.1.2 at most) to `@seriousme/openapi-schema-validator` 2.11.0 (MIT; 2.0–3.2), with `salescompel` and `salesforce-source` roles. Earlier "3.0-only" entries in this log are historical.
- Ruling: the M2 candidate keeps one `internal/platform` package; split along the roadmap boundaries when M3/M4 widen the surface.
- Unowned Salesforce areas were assigned to milestones in the roadmap's "Coverage assignments" table; new deferred contracts are in `contracts/identity.yaml` and `contracts/rest-semantics.yaml`, and `orgFeatures` was added to the profile.
- Source register: entries previously marked `documentation-reviewed` without recorded extracted content are now `reverify`. Automated fetches of developer.salesforce.com returned HTTP 403 during the review, so those claims (including the Apex API 67.0 security change and SOQL null ordering) remain unconfirmed. The Pub/Sub repository was confirmed archived on 2026-08-08.
- Process: M2 ran ahead of M1. A disposable reference org, the terms-of-service decision and observer/implementer assignment are now the critical path.

## Plan 3 reference-evidence tooling — 2026-10-15

- Implemented readiness schema/record, reference preflight and a direct capture
  gate before client creation. The real record remains pending and unauthorized.
- Added private capture-pair verification, closed observation schema and reciprocal
  contract links. Offline validation reports 0 observed / 11 not_run; no notes or
  risk decision were fabricated. Candidate authentication behavior is unchanged.
- Protected live CI runs shared preflight. Reference reports stay in unique
  owner-only directories under the persistent recovery directory; only candidate
  reports are uploaded. The observer runbook explains setup, repeatability,
  sanitization review and contract registration.
- Verification: Node compatibility check passes (43 tests passed, 1 optional
  candidate test skipped), TypeScript/formatting/repository validation pass,
  git diff --check clean. Real pending reference preflight exits 2 as intended.
- Ruling: keep raw-byte profile hashes stable when registering observations;
  contract coverage is authoritative for this phase. Editing the profile requires
  newly bound authorization and captures.
- Ruling: pair-verification output uses a new filename with exclusive creation,
  so it cannot overwrite a private input or follow an existing output symlink.
- Live observation not performed. Required inputs remain the authorized disposable
  org/profile, actual maintainer terms decision and explicit authorization,
  separately assigned observer, prepared personas and private credentials.
- M0 completion, M1R, expanded CLI distribution/plugin runners and routing proxy,
  and M3 protocol expansion remain outstanding; tooling completion certifies none
  of those milestones.

### Plan 3 final independent review

The independent reviewer found three important issues: capture dates could
precede authorization, ordinary 15-letter prose could be mistaken for IDs, and
credential detection was case-sensitive. Each was reproduced with a failing
regression test, fixed, and verified. Final full compatibility check: **46 passed,
1 optional candidate integration test skipped**, with TypeScript, formatting and
repository validation passing. No live Salesforce observation was performed.

Execution decisions and limitations:

- Work stayed on main per the recorded user policy; no branch or worktree was
  created. A different integration preference would require relocating commits.
- Profile bytes remain unchanged when notes are registered; downstream readers
  must use validated contract coverage rather than profile.referenceStatus.
- Pair-verification output requires a new filename; callers cannot reuse an old
  output path. This prevents overwriting input evidence and existing symlinks.
- Private capture authenticity and role identities remain human attestations,
  as designed. Automated validation cannot prove those attestations true.
- Live behavior, permissions and actual cleanup remain unverified until the
  authorized observer supplies evidence. Offline tests cannot close that gate.
- The existing comparator's scenario-level cleanup coverage is retained; it does
  not prove resource-level completeness beyond the recorded cleanup evidence.
- Authorization chronology is checked at UTC date precision. The observer must
  ensure approval precedes capture within that day. Secret/identifier scanning
  is heuristic and does not replace human sanitization review.

No minor findings were deferred by the final reviewer.


## Reference observation authorization 2026-10-16

The project maintainer confirmed in this session that they reviewed the
applicable Salesforce terms and authorized a separate observer session to run
the planned synthetic reference tests in the newly created disposable org.
The maintainer supplied the local CLI alias `velocity-innovation-8559`; local
CLI authorization is saved. No credentials or org/user identifiers are recorded
here.

Assigned roles: `plan3-observer` performs documented-API reference setup and
observations; `platform-implementer` (the existing implementation session)
consumes only reviewed, sanitized notes. Raw captures and auth material stay
private to the observer workflow and are not returned to the implementation
session. Scope is the existing eleven pilot scenarios, their synthetic fixtures,
necessary test-persona preparation and cleanup. No production testing,
undocumented API access, load tests or Salesforce UI capture is authorized.

This records authorization, not fixture readiness or compatibility evidence.
The observer must verify and bind the actual profile/API version, prepare the
required personas and pass preflight before full captures. Missing prerequisites
remain explicit blockers; no gate or required scenario may be removed to pass.

### Authorization transition regression check

Recording actual authorization exposed a test that assumed the committed
readiness record would always remain pending. Negative capture and CLI tests
now use an isolated temporary repository with a synthetic pending record;
readiness tests no longer depend on the real decision, and the offline CLI
coverage assertion allows observations to progress. No runtime gate was relaxed.
The failure was reproduced before the test-fixture change. Full check after
fix: 46 passed, 1 optional candidate test skipped; TypeScript, formatting and
repository validation passed.


## Reference setup scope check 2026-10-16

The separately assigned `plan3-observer` used the saved CLI authorization and
bounded, documented API inventory requests. The supplied disposable reference
reports Base Edition, is not a sandbox, and has a trial expiration date. It does
not satisfy the Developer Edition/sandbox restriction in CLEAN_ROOM.md and the
approved reference-evidence design. Observation authorization does not implicitly
amend that restriction. No fixture deployment, user creation, settings mutation
or full scenario capture was performed.

Setup is blocked pending either a disposable Developer Edition/sandbox alias or
an explicit maintainer decision extending the authorized environment scope to
this disposable trial. The profile remains unbound and fixture attestations
remain false. Inventory confirms API 65.0 availability and spare Salesforce user
licenses, but does not establish restricted-persona authentication or complete
profile readiness. Private inventory/auth files remain outside the checkout in
an owner-only observer directory. No pilot contract is marked observed and no
sanitization review is claimed.

## Plan 4 policy alignment — 2026-10-16

- Current black-box policy supersedes the earlier disposable-trial blocker and
  mandatory separated roles. Existing named-tenant testing authorization stands;
  no additional terms-review approval is required.
- The executing operator is `platform-implementer`, also implementing this work.
  Readiness remains unbound and fixture flags false until verified live.
- Added regressions for same-operator/null-implementer acceptance and continued
  rejection of unauthorized captures and mismatched observation attribution.
  Both new tests failed against the old role restrictions before the fix.
- Historical Plan 3 documents retain their history with a supersession notice.
- Verification: full compatibility check passes (48 passed, 1 optional candidate
  integration test skipped), including TypeScript, formatting and repository
  validation; no live observations registered.

### Plan 4 partial tenant setup — 2026-10-16

- Authenticated documented API inventory confirmed API 65.0, a disposable trial,
  Account/Contact/User availability, no Person Accounts or multi-currency, and
  nine unused Salesforce licenses. Required French language, US locale and
  America/New_York timezone picklist values are available.
- Checked both custom-field names and Compat_Admin for collisions: none existed.
  Deployed the two existing synthetic fields and permission set through the
  pinned CLI. Deployment completed successfully with three components and zero
  component errors. These persistent test fixtures are retained for the baseline;
  their ownership/setup journal is private. No business records or user settings
  were changed, and no new users have been created.
- Discovery found two installed packages and active flows. The synthetic profile
  is not yet a verified representation of this tenant; package dependencies and
  relevant flow effects must be resolved before binding it. No WorkflowRule or
  ExternalServiceRegistration records were returned by the inventory queries.
- No dedicated administrator, restricted or locale test users were present in
  the active-user inventory. Requested a contact email for creating these users;
  restricted-user authentication/MFA will also need a private login. Do not
  substitute the current administrator token for restricted-user credentials.
- Readiness attests only disposable=true and deployed=true. Profile/API binding
  and the remaining persona flags remain unset. Reference preflight exits 2,
  correctly identifying incomplete bindings/personas/runtime configuration.
- CLI org display returns a redacted access-token placeholder in this pinned
  version. Its documented org auth show-access-token command supplies the actual
  token privately. Initial direct REST failures were INVALID_AUTH_HEADER from
  that placeholder, not an expired session. No CLI source was inspected.
- Task 1 independent review found no actionable issues and reran all thirteen
  readiness/evidence tests successfully. Tasks 2–4 remain incomplete; no captures,
  repeatability result or reviewed pilot observations are claimed.

### Plan 4 dedicated personas — 2026-10-16

- Created three dedicated synthetic users using maintainer-supplied private
  inboxes: administrator, restricted API reader and locale fixture. Each has a
  unique synthetic username; no existing user account was repurposed or changed.
- Administrator uses System Administrator plus the existing Compat_Admin fixture
  permission set. Restricted and locale users use Minimum Access - Salesforce;
  the restricted user receives a dedicated permission set with API access and
  Account read, without a custom-field grant or write/admin permissions.
- Salesforce accepted password-reset/activation requests for the two new users
  requiring independent sessions. User completion of password setup and private
  CLI web login remains pending. Email addresses, usernames, user IDs and
  activation responses remain outside the repository.
- One supplied inbox may also belong to the maintainer's primary account.
  Provisioning and activation targeted only the newly created synthetic IDs;
  the primary account and its credentials/permissions were not changed.
- Read-back checks confirmed all three users active with the configured inboxes.
  Restricted profile/permission-set inspection confirms API access and Account
  read, zero Compat_Text__c grants, and no Account write/view-all/modify-all
  grants. These are configuration checks, not restricted-session observations;
  readiness flags remain conservative until independent authentication is tested.

### Plan 4 verified reference readiness — 2026-10-16

- Both dedicated users completed independent CLI web authorization. Live identity
  checks confirm the expected distinct users in the authorized tenant. Admin
  Account describe includes Compat_Text__c; restricted describe omits it, its
  explicit field query returns INVALID_FIELD, and Account Name query succeeds.
- The dedicated locale fixture is active and readable by the test administrator;
  its original language/locale/timezone are journaled privately. Required French,
  US locale and America/New_York values are available.
- Bound API 65.0 and the exact updated pilot profile hash. Installed package names
  and complete versions are recorded as environment context, without claiming
  package parity. Active Flow inventory has no Workflow/InvocableProcess types;
  earlier queries found no WorkflowRule or ExternalServiceRegistration records.
- Ruling: remove synthetic `iw` from the live profile because User describe does
  not offer it. The legacy-code offline catalog/contract remains unchanged and
  unobserved; all eleven live scenarios and connector pins are retained. This
  profile records supported test settings rather than inventing tenant support.
- Readiness flags now reflect verified setup. Profile currency remains the pilot's
  synthetic USD fixture setting; multi-currency is absent and currency semantics
  are outside these eleven scenarios. Active packaged/platform flows and installed
  packages are not certified by this baseline.
- Verification: full offline check passes (48 passed, 1 optional candidate test
  skipped); bound reference preflight exits 0 with private runtime configuration.

### Plan 4 repeatable pilot evidence accepted — 2026-10-16 (America/New_York)

- Both sequential reference captures passed all eleven scenarios and all reported
  cleanup. Repeatability and capture-pair verification passed without adding any
  normalization or reducing coverage. Exact UTC times, digests and bindings are
  recorded in observations/pilot-black-box-baseline.yaml.
- Independently verified 414 owned Account/Contact records absent from normal
  queries, both locale restorations and both run-specific metadata removals.
  Persistent setup fixtures remain intentionally available; recycle-bin purge
  is not claimed. Private reports and ownership journals remain outside git.
- Reviewed independently authored summaries for credentials, identities, personal
  data and copied messages, then registered reciprocal links for eleven pilot
  contracts. No deferred REST/identity contract was marked observed. JSforce's
  null statusCode observations do not establish HTTP status parity.
- Existing candidate smoke passed: JSforce, Python HTTPS CRUD, restart persistence,
  CLI query and four browser tests. This does not compare the full pilot against
  the candidate, which still lacks mandatory Composite/Metadata/locale surfaces.
- Handoff: plan-4-reference-baseline.md lists concrete M3 gaps, evidence limits and
  remaining focused probes. M0/M1 runner obligations and M1R remain outstanding.
- During independent cleanup inspection an expired API session was refreshed
  through its saved CLI authorization; the same verified personas were retained.
  A transient read connection reset was retried without repeating any writes.
- Final verification: full compatibility check passes (48 passed, 1 optional
  candidate integration test skipped); repository validation reports eleven
  observed pilot contracts and zero pending pilot contracts. Independent final
  review found no actionable issues, verified profile/readiness/evidence hashes
  and reciprocal links, and confirmed the documented limits. No minor findings
  were deferred.
- Execution decisions: used main and optional role separation under recorded
  policy; kept the profile unbound until actual persona/discovery verification;
  used the documented token-export command rather than the redacted display;
  removed only unavailable synthetic iw from the live profile while retaining
  its offline contract; retained the shared harness's broader deleted-record
  code allowance while recording NOT_FOUND for this profile. A different profile
  or broader claim requires new evidence, not reinterpretation of these captures.

## Plan 5 foundation suite setup — 2026-10-17

- The maintainer approved the design and implementation plan, explicitly requiring
  identical Salesforce routes for drop-in clients, then authorized continuation.
- Restored the implementation plan after workspace I/O errors; the temporary copy
  disappeared, while the route amendment survived. Work continues on main.
- Added a separate foundation suite/profile/readiness path. Existing pilot defaults,
  profile bytes, eleven-scenario manifest and reviewed observations are retained.
- Foundation readiness carries existing authorization but has no bound profile hash
  or verified expanded fixture flags. It cannot authorize a reference capture yet.
- Focused scenario implementations are pending; capture fails closed until registered.
  Suite selection is applied to preflight, capture, comparison and pair verification.
- Observation contract IDs are distinct from execution scenario IDs for the focused
  suite; registered contract scope and reciprocal links remain required. Broader
  request-header contracts are not promoted by scoped batch-size coverage.

## Plan 5 focused probe implementation — 2026-10-17

- Implemented seven focused handlers for routes, paging, cursor lifecycle, IDs,
  name handling, text/null semantics and HTTP errors, plus same-origin wire capture.
- Offline verification: 63 passed, one optional candidate test skipped; typecheck,
  formatting and repository validation passed. Local HTTP tests verify redirect
  rejection, escaped paths, response/time bounds and cleanup after a failed create.
- Reference administrator and restricted sessions both passed read-only userinfo
  checks. Tokens and identities remain in an owner-only external directory.
- An independent same-administrator session is needed for the cursor probe; the
  maintainer was asked to log in privately with alias salescompel-reference-admin-fresh.
  No focused live capture, fixture write or candidate semantic change occurred yet.
- Discovery/describe measurement is limited to supported-version and fixture-field
  dimensions. Null equality retains only owned-record membership. Fixed expiry and
  traversal bounds produce explicit gaps rather than successful partial captures.
- Foundation readiness remains unbound/unverified, and focused contracts remain
  not_run. Existing pilot profile and accepted baseline are unchanged.

## Plan 5 evidence gate and probe review — 2026-10-17

- Corrected three independently reviewed probe defects: repeated text variants no
  longer create duplicate identities, discovery query/sobjects links survive
  normalization, and each name/duplicate-key PATCH has its own state readback.
  Each correction has a regression test that failed before the correction.
- Fresh offline verification: 66 passed, one optional candidate test skipped;
  typecheck, formatting and repository validation passed. YAML parsing and archive
  URL uniqueness checks passed. These checks do not establish reference parity.
- Inventoried 71 distinct direct official release-archive document links. Source
  bodies were unavailable, so content review and linked-changelog expansion remain
  incomplete. The inventory and changes ledger explicitly record these blockers.
- The fresh same-administrator alias remains absent. No focused live captures or
  fixture writes have run. Contract decisions remain unfrozen; Tasks 4–8 await
  the approved Task 3 evidence gate. Existing product routes remain unchanged.

## Plan 5 foundation session and fixture readiness — 2026-10-17

- Verified two distinct live sessions for the exact same reference administrator
  and organization using public userinfo. Preserved the primary session separately
  before exporting the fresh session. Restricted identity remains distinct.
- Verified administrator Account create/delete, writable Compat_Text__c, external
  Compat_Key__c and Contact create permissions; restricted Account query access
  excludes Compat_Text__c. Existing disposable-tenant authorization and pilot
  locale attestation apply; the focused suite does not mutate locale settings.
- Bound foundation readiness to its unchanged profile hash. This records setup
  readiness only; live observation and release-review gates remain separate.

## Plan 5 first focused capture — 2026-10-17

- First live run finished: six scenarios passed; the cursor scenario has 28 passing
  assertions but remains blocked on unmeasured expiry after sixteen minutes.
- All 2,418 cleanup actions passed. Independent exact-ID REST queries confirmed
  zero active owned records remain. Private report and verification files retain
  the full evidence; the decision note records the report digest and provisional
  findings without publishing tenant/user/record identifiers.
- Started the second capture only after independent cleanup verification. The
  suite/profile are unchanged. No repeatability or complete evidence-pair claim
  has been made, and product routes/behavior remain unchanged.
- Tried release-pinned Winter 26 API/REST/SOQL HTML pages; responses were a loading
  shell or unavailable. This does not close the release-content review blocker.

## Plan 5 second capture and ordering corrections — 2026-10-17

- Second capture repeated six passes and the expiry gap. Independent queries
  confirmed all 2,418 journaled records absent after successful cleanup.
- Repeatability and pair verification returned blocked. Diagnostic comparison
  found unordered limited-query membership differences, unordered name-equality
  row-order differences, and equal-key cursor order/page-partition differences.
- Added explicit unique fixture-text ordering to the first two probes, preserving
  projection and supported grammar. A local HTTP regression failed for each
  missing ORDER BY before both corrections passed. Raw captures stay untouched;
  tied-row comparisons remain unresolved and require an explicit evidence rule.
- Updated the CLI test to check a missing fresh session rather than assume that
  real readiness will stay unbound forever. Full check: 67 passed, one optional
  candidate skip; typecheck, formatting and repository validation passed.
- Prepared a private read-only expiry follow-up with digest, identity, route and
  not-before checks. Earliest checkpoint is October 19 at 21:54 UTC; both are due
  after 23:21 UTC. No background job is scheduled. Product behavior/routes remain
  unchanged and Task 3 does not yet meet its completion gate.

## Plan 6 read-only metadata discovery — 2026-10-18

- Implemented strict inventory/config validation, bounded JSforce reads, repeated
  distinct-persona identity checks, isolated native CLI retrieval, private output,
  capability matrix compilation and guarded stability comparison.
- A live CLI retrieve emitted an empty Account object container; added a failing
  regression before allowing only that exact empty container. Extra metadata,
  symlinks, DTD/entities and size violations remain rejected.
- Two sequential reviewed collections each retained 112 facts, 21 gaps and nine
  artifacts. Matrices and artifact hashes matched without conflicts. Both runs
  remain incomplete, so formal stability comparison correctly returned blocked.
- Recorded omitted grants and unavailable restricted describe evidence without
  interpreting them as effective access. Optional scopes remain unrequested.
- Final Node check: 88 passed, one optional candidate skip, zero failed; typecheck,
  formatting and repository validation passed. CLI temporary homes were removed.
  Existing profile bytes and product routes/code remain unchanged.
- See [Plan 6 results](plan-6-metadata-discovery.md) for time/digest bindings,
  blockers and the narrow proposed field-metadata/grants slice. No M4 or Plan 5
  completion claim is made; no expiry background job is scheduled.
- Fresh review found two Important issues, both fixed with failing-then-passing
  regressions: retain partial retrieval members on execution failure, and require
  full operation/property coverage before accepting completion. No deferred
  findings remain. Successful collection semantics and live blockers are unchanged.

## Plan 7 synthetic declaration model — 2026-10-18

- Added internal immutable-value lookups for the two observed custom fields and
  their Compat_Admin declared grants. Unknown names/properties remain unknown;
  declared grants do not activate effective access.
- Verified synthetic values in both Plan 6 inventories and both read/retrieve
  sources; no live requests, raw capture publication or compatibility promotion.
- Regression tests failed before implementation and then passed. `go test ./...`
  and `go vet ./...` passed; database integration tests skipped without
  TEST_DATABASE_URL. No database/schema or existing runtime files changed.
- This is an internal representation slice. Existing routes and describe/write
  behavior remain unchanged. [Plan 7 details](plan-7-declared-metadata.md) explain
  evidence and the migration/enforcement dependencies for future activation.

## Plan 8 active field metadata — 2026-10-18

- Activated observed length 80 for both synthetic Account custom fields using the
  declaration model; existing describe route and Field JSON properties preserved.
- Added exact-v1 recognition and transactional catalog v2 migration, with table
  locks, oversized-data refusal, rollback, idempotent reopen and physical checks.
- PostgreSQL-backed Go tests and vet passed; Node check passed (88 plus one optional
  candidate skip). The separate native candidate gate passed JSforce before/after
  restart, Python, CLI and four browser tests. Old binary failed the new describe
  assertion at 255, confirming the regression test detects the change.
- See [Plan 8 upgrade guide](plan-8-active-field-metadata.md) for operator actions,
  non-destructive refusal and rollback limits. No production/reference mutations
  or compatibility promotion; Plan 5 remains separate.

## Plan 9 external-ID contract — 2026-10-18

- Added fixed 51-operation probes, journal-first recovery, durable known-ID and
  identity-mismatch handling, private artifact validation and diagnostic comparison.
- Two authorized sequential reference runs completed with verified cleanup.
  Deterministic cases matched; variant race rounds 2–4 differed in winner/stored
  spelling. Overall comparison is drift, so no stable contract promotion.
- Fresh review identified four Important findings, all fixed with regression
  evidence. Full Node check passed: 106 tests, one optional skip, zero failures.
- Added root `.recomp.json` with applicable project information and validated it
  against the published v1 schema. No game/decompilation claims were inferred.
- Product routes, catalog and accepted profiles remain unchanged. This is M1
  evidence supporting M3; product upsert semantics are not fixed by this plan.
  See [Plan 9 evidence and implementation boundary](plan-9-external-id-contract.md).

## Plan 10 foundation evidence follow-ups — 2026-10-19

- Added a fixed assumption-aware cursor collection policy, immutable private
  capture sidecar, exact coverage and tied-traversal comparison, separate pair
  assessment, live identity preflight and read-only exact-ID cleanup verifier.
- Focused official-source recheck corroborated documented two-day cursor/result
  availability at API 65.0; the release-pinned Winter 23 Help page remained
  unreadable. Relevant release findings retain their blocker status.
- No Plan 10 live capture ran: the process lacks all required reference session
  settings and the private recovery directory. Offline foundation preflight exited
  2 before reference requests or fixture writes. The decision remains blocked;
  see [Plan 10 decision](plan-10-foundation-decision.md).
