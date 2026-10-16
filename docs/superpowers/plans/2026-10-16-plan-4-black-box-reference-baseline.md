# Plan 4 — Black-Box Reference Baseline Implementation Plan

> **For agentic workers:** Use superpowers:executing-plans to implement this plan sequentially. Native execution is the existing preference. Steps use checkbox syntax for tracking. Do not perform live writes merely because this plan exists; use the maintainer's already recorded authorization for the named tenant and the scope below.

**Goal:** Complete a reproducible pilot behavior baseline against the maintainer's authorized Salesforce tenant and produce concrete evidence for the next M3 REST/query work package.

**Architecture:** Reuse the Plan 1 capture/comparison harness and Plan 3 readiness/evidence validation. Align their remaining role checks with the current black-box policy, finish tenant/persona setup, then run two sequential reference captures and register sanitized observations. No new service or general-purpose testing framework is needed.

**Tech Stack:** Existing pinned Node/TypeScript laboratory, JSforce, Salesforce CLI, simple-salesforce, JSON/YAML contracts and node:test. No dependency upgrades are planned.

**Spec:** [Plan 3 reference-evidence design](../specs/2026-10-15-plan-3-reference-evidence-design.md), as superseded by [the black-box testing policy](../../../CLEAN_ROOM.md) and the maintainer's 2026-10-16 instructions: observation and behavior testing against their own tenant are authorized; do not reverse-engineer Salesforce CLI code. The superseded design's mandatory separated roles, Developer Edition/sandbox restriction and extra terms-review approval gates do not apply.

**Status:** Written for review. No implementation or live tests have been performed by writing this plan.

## Current state and intended outcome

Plan 2 provides the Account/Contact candidate. Plan 3 provides readiness and
reviewed-evidence tooling. The local CLI alias `velocity-innovation-8559` is
logged in, and the maintainer has authorized the pilot's synthetic tests and
necessary test setup. Read-only inventory reported API 65.0 availability and
spare user licenses; recheck operational details at execution time. A previously
recorded trial-edition blocker is superseded by the current policy.

The remaining technical gap is profile binding, prepared fixtures/personas,
restricted-persona authentication and real repeatable captures. The repository
currently has zero reviewed pilot observations and eleven not_run contracts.
The executable readiness and evidence modules still require different observer
and implementer identities, contrary to the updated policy.

Success is a reproducible, sanitized baseline for all eleven existing pilot
scenarios, with cleanup verified and contracts linked to evidence. Tooling
completion alone is not baseline completion. This plan does not certify M3,
finish every M0/M1 obligation, or perform the M1R release-history review.

## Approaches considered

1. **Finish the existing baseline — selected.** Small policy-alignment patch,
   then use the authenticated tenant and existing harness. Produces real evidence
   before expanding candidate semantics.
2. Start M3 REST/query implementation immediately. Moves product code forward,
   but carries unresolved behavior assumptions into another implementation.
3. Replace the laboratory. Duplicates functioning fixture journals, pinned
   clients, comparators and observation validators without solving missing access.

## Global constraints

- Work and commit on main, following the recorded repository policy.
- Observation and behavior testing against the maintainer's own authorized tenant
  are allowed. A disposable trial is valid; edition alone is not a blocker.
- **Do not reverse-engineer Salesforce CLI code.** Do not read, trace, instrument,
  decompile or patch its implementation or plugin implementation to derive
  behavior. Invoke documented commands unmodified and inspect their external
  outputs. Published command documentation, `--help`, `--version` and package
  version/license metadata may establish invocation and provenance only.
- Prefer behavior observations and independent implementation over reading or
  copying third-party implementations. Other open-source material may be used
  only under the source-selection rules below; the specific CLI/plugin restriction
  remains in force.
- Use public documented APIs and synthetic fixtures. Do not change unrelated
  business records, existing users' permissions or organization-wide features.
- Keep credentials and raw reports private, outside the checkout where practical,
  with owner-only permissions. Never put tokens in command arguments or output.
- Use the recorded authorization; do not ask again for terms review, a separate
  observer, or permission based solely on the trial edition.
- The same person/session may test, review sanitized notes and implement behavior.
  Review means actually checking the evidence; it need not mean a second person.
- Preserve the eleven-scenario manifest, fixture readiness, profile/API/client
  binding, cleanup, repeatability and sensitive-data checks.
- A missing authentication factor or login that only the user can complete is a
  real dependency. Ask for that concrete action, without requesting token strings
  in chat or manufacturing a broader approval requirement.

## Source selection and reuse requirements

The maintainer permits consideration of sources licensed for copying,
modification and commercial use/distribution, including projects discovered at
https://github.com/orgs/salesforce/repositories. Prefer behavior observations;
avoid code reuse when independent implementation is feasible. A repository's
public visibility or organization membership does not establish permission.

Before any new reuse, inspect the exact license and file-level exceptions at a
pinned revision. Record the upstream URL/revision, source and destination paths,
license, purpose, modifications and required notices in THIRD_PARTY_NOTICES.md.
Keep required legal notices and preserve existing dependency/fixture notices.
Do not reverse-engineer CLI/plugin implementation code under this permission.

Clarification requested: “without keeping authors” may mean either no obligation
to involve the original authors, with required notices acceptable, or a strict
no-attribution requirement. Do not select or copy code that relies on an answer
until the maintainer clarifies. This does not block behavioral tests or independent
implementation. Ask the maintainer whenever source eligibility or usage rights
are unclear. No third-party code reuse is planned for this work package.

## Review focus

1. Same-person testing must pass while unauthorized or stale-bound runs still
   block before transport — Task 1 regression tests.
2. Account access without the restricted field must be demonstrated; a broadly
   privileged user cannot stand in for the restricted persona — Task 2 live checks.
3. Tests must modify only their synthetic fixtures and restore dedicated-user
   locale settings — Tasks 2–3 ownership and cleanup checks.
4. Repeatability differences must be investigated, not erased with blanket
   normalization or reduced scenario coverage — Task 3.
5. Notes must bind to the exact captured profile and preserve useful failure
   codes without publishing credentials, IDs or copied messages — Task 4.

## Task 1: Align executable checks and current documentation with black-box testing

**Files:**
- Modify `compat/clients/node/src/readiness.ts` and `src/evidence.ts`.
- Modify `compat/clients/node/test/readiness.test.ts` and `test/evidence.test.ts`.
- Modify `docs/compat/README.md`, `docs/compat/observations/README.md`,
  `docs/compat/implementation-log.md`, `docs/plans/plan-1-followups.md`,
  `docs/plans/2026-10-14-salesforce-compatibility-roadmap.md` and `CLEAN_ROOM.md`.
- Update `docs/compat/reference-readiness.yaml` only with truthful attribution;
  profile hash/API and fixture attestations remain unset until Task 2 verifies them.

**Interfaces:** Retain the existing `Readiness`, `EvidenceContext`,
`readinessDecisionIssues`, `referencePreflight` and `observationIssues` interfaces.
The existing `observer` field identifies the test operator. `implementer` remains
an optional attribution in practice: its existing nullable schema field can stay
present and null, or name the same actor. No schema-version migration is needed.
`decision` records the maintainer's ordinary testing authorization, not a new
terms-review decision. Preserve the recorded authorization date and log reference.

- [ ] Add the following regressions using existing test-local helpers and fixtures:

```ts
// readiness.test.ts: existing approved(), schema, context and env fixtures.
const sameActor = approved();
sameActor.implementer = sameActor.observer;
assert.deepEqual(readinessIssues(sameActor, schema, context, env), []);
sameActor.implementer = null;
assert.deepEqual(readinessIssues(sameActor, schema, context, env), []);
sameActor.decision.observationAuthorized = false;
assert.ok(readinessIssues(sameActor, schema, context, env).length);
```

```ts
// evidence.test.ts: existing note(), schema, context and readiness fixtures.
const observed = note();
assert.deepEqual(observationIssues(observed, schema, context, {
  ...readiness, implementer: readiness.observer,
}), []);
assert.deepEqual(observationIssues(observed, schema, context, {
  ...readiness, implementer: null,
}), []);
const wrongOperator = {...observed, observer: 'unrecorded-operator'};
assert.ok(observationIssues(wrongOperator, schema, context, readiness).length);
```

- [ ] Run `cd compat/clients/node && node --import tsx --test test/readiness.test.ts test/evidence.test.ts`. Expected: same-actor/null-implementer acceptance fails against current code.
- [ ] Remove the mandatory implementer presence and differing-role conditions in
  readinessDecisionIssues; retain operator identity and all actual authorization,
  date, profile, API, manifest and fixture checks. Replace the evidence role check
  with operator attribution only:

```ts
if (!r || n.observer !== r.observer) {
  issues.push('Observation operator differs from the readiness record');
}
```

- [ ] Replace the old negative readiness test that rejects identical roles with a
  negative test for a missing operator. Keep pending/rejected authorization,
  malformed date, stale profile, missing credentials, unknown-field, no-transport,
  chronology, reciprocal-link and sanitizer tests unchanged.
- [ ] Add the maintainer's prohibition on reverse-engineering CLI/plugin code to
  CLEAN_ROOM.md if not already present. Preserve the new source-selection policy:
  behavior observations first, exact-license review and documented reuse only.
  Resolve attribution ambiguity with the maintainer before any affected reuse.
  Update current runbook/follow-up/roadmap instructions to refer to
  authorized tenant testing, optional role separation and the already recorded
  authorization. Mark the old trial-type blocker as superseded in a new log entry;
  preserve historical records rather than pretending the blocker never happened.
  Historical Plan 3 documents need only an explicit supersession notice, not a
  rewrite of the implementation history.
- [ ] Run `npm --prefix compat/clients/node run check` and `git diff --check`.
  Expected: all offline checks pass; real preflight may still block on unprepared
  fixtures. Commit `fix(compat): align reference checks with black-box policy`.

## Task 2: Bind the tenant and prepare the pilot fixtures/personas

**Files/artifacts:**
- Update `docs/compat/profiles/pilot.yaml` and `docs/compat/reference-readiness.yaml`.
- Reuse `compat/fixtures/salesforce/` and its existing Compat_Admin permission set.
- Record a sanitized setup summary in `docs/compat/implementation-log.md`.
- Keep runtime credentials, exact user IDs, setup results and recovery records in
  a private owner-only directory outside the checkout.
- If reproducibility needs a new test permission-set/profile artifact, place only
  its independent synthetic definition in `compat/fixtures/salesforce/`; do not
  commit exported unrelated org configuration.

**Interfaces:** Existing environment variables remain the boundary:
`COMPAT_REFERENCE_URL`, `COMPAT_REFERENCE_TOKEN`,
`COMPAT_REFERENCE_RESTRICTED_TOKEN`, `COMPAT_REFERENCE_LOCALE_USER_ID`,
`COMPAT_API_VERSION`, `COMPAT_DISPOSABLE=yes`, and an absolute owner-only
`COMPAT_RECOVERY_DIR` outside the checkout. Do not introduce another auth service.

- [ ] Inspect existing private recovery journals before making writes. Resolve
  only known owned fixtures; no broad cleanup query or mass deletion.
- [ ] Use the saved CLI alias through documented CLI commands, capturing any
  credential-bearing output in memory/private files without echoing it. Inspect
  installed CLI `--help` and published documentation if invocation details are
  uncertain; never inspect its implementation. Confirm session usability and
  API 65.0 support using documented service discovery. If expired, ask only for
  reauthentication to this alias.
- [ ] Finish bounded profile discovery: relevant features, objects, installed
  package dependencies, required languages/locales and current pinned clients.
  Record actual findings. Do not infer readiness from the edition string or
  enable Person Accounts/multi-currency to fit the synthetic profile. If an
  unexpected org feature affects a required test, record its actual impact and
  resolve that setup requirement without silently dropping coverage.
- [ ] Check the two fixed custom fields for collisions. If unrelated fields/data
  already use those names, preserve them and report the collision. Otherwise
  deploy the inspected synthetic fixtures with the pinned local CLI. From the
  repository root, after privately setting the verified COMPAT_API_VERSION:

```sh
cd compat/fixtures/salesforce
../../clients/node/node_modules/.bin/sf project deploy start \
  --source-dir force-app --target-org velocity-innovation-8559 \
  --api-version "$COMPAT_API_VERSION" --json
```

  Capture this command's result privately; do not publish raw output. Verify
  deployment completion before setting the deployed attestation.
- [ ] Prepare a dedicated test administrator, an Account-readable restricted
  persona without Compat_Text__c access, and a dedicated locale fixture user.
  Reuse only users clearly designated for these tests. Spare license counts are
  not proof that the required profiles/credentials exist. Do not alter unrelated
  users or grant administrative permissions to the restricted persona.
- [ ] Use documented user/permission setup paths or ask the maintainer for the
  specific setup action if it cannot be completed by documented APIs. Obtain
  the restricted persona's CLI/web authorization privately. Do not derive its
  token from the administrator token or substitute the administrator session.
- [ ] Verify the restricted token's identity belongs to the same tenant and a
  different user. Verify positive Account Name access and negative
  Compat_Text__c describe/query access. Verify the administrator sees the field.
  For the locale fixture, verify the specific language/locale/timezone settings
  needed by the existing locale scenario are available; journal original values
  before any temporary change and restore them.
- [ ] Pin the complete profile before either capture. Retain requiredScenarios and
  connector pins; keep profile.referenceStatus stable while registering notes.
  Compute its raw-byte hash:

```sh
sha256sum docs/compat/profiles/pilot.yaml
```

- [ ] Bind readiness to that hash/API and the existing authorization. Set each
  fixture flag only after verifying its condition. The observer label may name
  the same operator as the implementer; no separate-role approval is required.
- [ ] Run `npm --prefix compat/clients/node run check` and then
  `npm --prefix compat/clients/node run preflight -- --target reference` with the
  private runtime environment. Expected: offline checks pass; preflight exits 0
  only after every prerequisite is complete. Commit sanitized profile/readiness
  and setup records as `docs(compat): bind authorized tenant and pilot fixtures`.

**Real blockers:** If a restricted login, MFA step, user license or permission
cannot be supplied, retain completed setup and name the exact missing action.
Do not mark this task or the baseline complete. Trial edition and same-person
execution are not blockers under the current policy.

## Task 3: Obtain two repeatable full reference captures

**Files/artifacts:** Private reference reports and journals; independently phrased
findings under `docs/compat/observations/` only after Task 4 sanitization. Update
`docs/compat/contracts/pilot.yaml`/harness code only if an actual discrepancy
requires a verified correction; do not preemptively change expected behavior.

**Interfaces:** Reuse `capture`, `repeatability`, `verify-evidence`, the eleven
SCENARIOS and existing cleanup journals. `$REFERENCE_RUN_DIR` below is an absolute
owner-only directory outside the checkout, selected by the executor and retained
for recovery; it contains no credentials in its path.

- [ ] Create a fresh private run directory with mode 0700 and preserve the prepared
  environment. Run full preflight immediately before the first capture.
- [ ] Run the first reference capture:

```sh
npm --prefix compat/clients/node run capture -- \
  --target reference --out "$REFERENCE_RUN_DIR/reference-1.json"
```

  Expected: exit 0, all eleven scenarios passed, complete successful cleanup.
  Inspect the private report; do not infer completion from exit status alone.
- [ ] If a test fails, first distinguish setup/client invocation, harness defect,
  documented behavior mismatch and incomplete cleanup. Investigate only through
  public documentation and observable inputs/results. **Do not read or trace
  Salesforce CLI/plugin source to diagnose the difference.** Record the result
  independently and use the public command/API boundary to narrow it.
- [ ] For a harness defect, add a minimal sanitized synthetic reproduction in
  the existing `test/connectors.test.ts`, `test/cli.test.ts` or `test/core.test.ts`
  as appropriate, observe RED, fix only the demonstrated defect, and run the full
  offline check. Exact test content depends on the observed failure and must be
  written from the recorded input/output, not guessed in advance. Commit fixes
  with their regression tests; keep captures from before and after the correction
  distinct. If a contract/profile changes, rebind readiness and restart the pair.
- [ ] Resolve failed cleanup from the private ownership journals before another
  capture. For known owned IDs, verify absence after deletion and restored locale
  settings. Unresolved orphans remain a failed cleanup outcome.
- [ ] Run the second full capture only after the first run and cleanup finish:

```sh
npm --prefix compat/clients/node run capture -- \
  --target reference --out "$REFERENCE_RUN_DIR/reference-2.json"
npm --prefix compat/clients/node run repeatability -- \
  --first "$REFERENCE_RUN_DIR/reference-1.json" \
  --second "$REFERENCE_RUN_DIR/reference-2.json" \
  --out "$REFERENCE_RUN_DIR/repeatability.json"
npm --prefix compat/clients/node run verify-evidence -- \
  --first "$REFERENCE_RUN_DIR/reference-1.json" \
  --second "$REFERENCE_RUN_DIR/reference-2.json" \
  --out "$REFERENCE_RUN_DIR/pair-verification.json"
```

- [ ] Require two distinct sequential captures with identical profile/API/client
  bindings and passing repeatability/cleanup. Investigate every difference.
  Add normalization only for specifically demonstrated nondeterministic fixture
  values, with regression coverage that still detects a real behavioral change.
  No global suppression of dates, IDs, nulls, error codes or permission results.
- [ ] Retain failed attempts privately for diagnosis, but register only a complete
  successful pair. If no passing pair can be obtained, report the precise failing
  contract and leave observationStatus unchanged; do not manufacture a pass.

## Task 4: Register sanitized evidence and produce the M3 handoff

**Files:**
- Create `docs/compat/observations/pilot-black-box-baseline.yaml` after successful
  captures, conforming to `compat/schemas/observation.schema.json`.
- Update `docs/compat/contracts/pilot.yaml` reciprocal links.
- Create `docs/compat/plan-4-reference-baseline.md` with setup, validation results,
  evidence coverage and a concrete next-work table.
- Update `docs/compat/implementation-log.md`, `docs/compat/README.md` and README.md
  to link the result with accurate completion status.

**Interfaces:** Observation records carry exact captured profile hash/API/client
pins, two raw-report digests/times, operator, completed sanitization review,
contract IDs and independent behavioral summaries. Existing
`observationLinkIssues` and `validateRepository` remain the acceptance checks.
The operator and reviewer may be the same person/session, but review must actually
occur. No raw response or CLI source is needed in a published note.

- [ ] Independently summarize the eleven pilot behaviors: describe, CRUD, upsert,
  relationship pagination, rollback, permissions, error codes, locale settings,
  CLI query, metadata and Python CRUD. Preserve relevant status/error codes and
  resulting state; omit verbatim messages, credentials and real IDs.
- [ ] Check every note for secret material, personal data, identifiers and copied
  reference prose. Validate timestamps, profile/API/client pins and digests against
  the private successful pair. Record review identity/date only after this check.
- [ ] Create the observation record using the schema's exact required fields;
  set its id to `pilot-black-box-baseline`, contracts to the eleven pilot IDs,
  and notes to one independent summary per claimed contract. Fill capture
  metadata from the actual successful pair, never from synthetic fixtures.
- [ ] For each supported pilot contract, set observationStatus to observed and
  observationIds to `[pilot-black-box-baseline]`. Do not edit the captured profile
  bytes or mark unrelated deferred REST/identity/CLI contracts observed.
- [ ] Produce the M3 handoff table with columns: contract ID, evidence reference,
  observed behavior, current candidate behavior, required change and remaining
  evidence gap. Use the existing Plan 2 behavior guide for candidate scope; add
  tests before asserting an undocumented candidate behavior. Cover the first
  REST/query slice's known issues: paging, 15/18-character IDs, field/object name
  handling, text comparison, null ordering, error shapes and served OpenAPI.
  Explicitly mark dimensions not exercised by the eleven scenarios as unobserved;
  this plan does not expand the live manifest to certify them incidentally.
- [ ] Run the existing local candidate smoke workflow only if a disposable local
  test DB is available; otherwise keep its prior verification dated and explicit.
  The full pilot candidate capture is not expected to pass until missing M3/M4
  surfaces exist. Do not remove metadata, rollback or other scenarios to produce
  a misleading parity result.
- [ ] Run `npm --prefix compat/clients/node run check` and `git diff --check`.
  Expected: all offline checks pass and all registered links validate. Inspect
  staged filenames/diffs for private artifacts before committing. Commit
  `docs(compat): register repeatable black-box pilot observations` only when
  actual evidence supports that title; a blocked result gets a blocker record.

## Completion and next milestone

Report separately:

1. Policy/code alignment and offline test results.
2. Tenant/persona readiness and any concrete remaining user action.
3. Real capture, repeatability and cleanup results.
4. Reviewed evidence coverage and candidate parity gaps.

The next product implementation plan is the M3 REST/query foundation: pagination,
record IDs, query/error semantics and served OpenAPI. It must use this baseline
and obtain focused reference evidence for contracts not covered here. M1R must
still be reconciled before M3 contract freeze; expanded CLI distribution/plugin
runners, routing proxy and the remaining M0/M1 scope remain separately tracked.
No full milestone or Salesforce-compatibility claim follows from a passing pilot
baseline alone.

## Self-review

This plan continues the approved evidence design under the maintainer's explicit
policy override. Task 1 removes the contradictory runtime role restriction;
Tasks 2–3 complete the existing live work rather than replacing the harness;
Task 4 registers evidence and defines the next M3 slice. All five review-focus
conditions have concrete offline or live verification steps. Authorization is
already supplied; only real credential/setup dependencies may require user input.
No live commands were executed during plan creation.
