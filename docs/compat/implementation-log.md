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
