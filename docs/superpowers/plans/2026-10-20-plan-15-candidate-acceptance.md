# Plan 15 — Candidate Walking-Slice Acceptance Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution, or superpowers:subagent-driven-development if the maintainer explicitly selects it. Implement task by task; checkboxes track executed steps, not planning completion.

**Goal:** Produce a reproducible, evidence-bound acceptance result for the existing Account/Contact candidate using its real database, clients and browser journeys.

**Architecture:** Add a thin full-acceptance shell entry point backed by focused TypeScript helpers in the existing Node lab. Refactor the existing candidate script to share those lifecycle/fixture helpers, extend the current client/browser suites, and interpret machine-readable results before publishing an allowlisted summary. No product API or normalization changes are planned.

**Tech Stack:** Existing Bash, Node/TypeScript/Ajv/node:test, Go/PostgreSQL, simple-salesforce, Salesforce CLI and Playwright; existing dependency pins.

**Spec:** [Plan 15 design](../specs/2026-10-20-plan-15-candidate-acceptance-design.md).

**Status:** Tooling implemented and reviewed on `main`. The original native CLI blocker was resolved by [Plan 16](../../compat/plan-16-cli-query-column-metadata.md), which records two fresh full acceptance passes. Full M1/M2 milestones and reference parity remain open. See [execution and decision record](../../compat/plan-15-candidate-acceptance.md) for tested revision, reports, commands and exclusions.

## Global Constraints

- Work directly on `main` under repository policy. Do not create a feature branch or assume this plan authorizes implementation.
- Go 1.25.0 module baseline, Node >=22.12, Python >=3.12, PostgreSQL 17, API 65.0.
- Preserve JSforce 3.11.0, Salesforce CLI 2.156.0, plugin-data 5.1.16, plugin-deploy-retrieve 4.2.15, simple-salesforce 1.12.9 and existing Vue/Playwright pins. No runtime dependencies or upgrades.
- Require `CANDIDATE_DISPOSABLE=yes`, one dedicated disposable database, compatible preinstalled tools/browser and explicit full-run output before provisioning. Reject conflicting database URLs.
- Full runs use an owned loopback HTTPS child service, private scratch/CLI state and a 20-minute deadline. No service reuse, reference requests, automatic setup/install or database destruction.
- Outcomes: passed/failed/blocked, exits 0/1/2. Later unexecuted gates are not-run. Missing coverage, unexpected skips, malformed results, cleanup failures and unpublished required evidence prevent a pass.
- Account/Contact fixtures are removed by proven exact ID, children first. Provisioned tenant/principal digests remain in the disposable database; retain their disposition privately and document operator/CI database teardown.
- Publish an allowlisted atomic JSON summary to a new output path. Raw logs, responses, auth state and browser artifacts remain private. Never print arbitrary subprocess stderr or secret-bearing argv/environment.
- Final acceptance requires two fresh sequential local full runs of a clean committed code revision; CI performs one. Preserve M1/M2 dependencies, unresolved reference semantics and Plan 13's unverified interactive visual acceptance.

## Review Focus

- A child exits zero with no required tests or a skipped real-client test: coverage must fail independently of exit status (Tasks 1 and 6).
- A port is occupied or a child/grandchild survives a signal: refuse foreign-service reuse and stop/reap only owned processes (Tasks 2 and 3).
- A create commits before its client crashes: preserve its intent/known IDs and never report verified cleanup from an incomplete journal (Tasks 3 and 4).
- A report exists but belongs to another run, source build or browser suite: reject stale/cross-bound evidence, including a stale preview server (Tasks 1, 5 and 6).
- Failure output contains tokens, database URLs or CLI auth JSON: keep raw material private and publish only fixed categories/counts/bindings (Tasks 1, 2 and 6).

## File and interface map

Use the prefix `candidate-acceptance-` only for this bounded harness; do not change reference-suite schemas or `cli.ts`.

| Files | Responsibility |
| --- | --- |
| `compat/clients/node/src/candidate-acceptance-contract.ts`, `compat/schemas/candidate-acceptance.schema.json` | Gate IDs, required coverage, typed versioned report, aggregation and validation |
| `compat/clients/node/src/candidate-acceptance-results.ts`, `candidate-acceptance-node-reporter.mjs` | Private Go/Node/Playwright/step result adapters |
| `compat/clients/node/src/candidate-acceptance-process.ts`, `candidate-acceptance-preflight.ts`; `compat/candidatecheck/main.go` | Bounded argv-based subprocess execution, owned processes, prerequisite/source/tool checks; read-only PostgreSQL connectivity/version helper using existing pgx |
| `compat/clients/node/src/candidate-acceptance-fixtures.ts`, `candidate-acceptance-service.ts` | Private fixture journal, exact-ID cleanup, provision/start/stop/restart |
| `compat/clients/node/src/candidate-acceptance-clients.ts` | Native CLI journey and retained restart fixture checks |
| `compat/clients/node/src/candidate-acceptance-run.ts`, `candidate-acceptance-command.ts` | Ordered full/candidate-only gate selection and invocation/publication |
| `scripts/check-candidate-acceptance.sh`, existing `scripts/check-candidate.sh` | Thin full and compatible candidate-only entry points |
| Existing JSforce, Python, connected/preview browser tests/configs and platform workflow | Extend and integrate actual journeys |

Tests live in matching `compat/clients/node/test/candidate-acceptance-{contract,results,process,preflight,service,clients,run}.test.ts` files. Add `test/helpers/candidate-acceptance-fixture.ts` for controlled local child processes/HTTPS fixtures used only by harness tests. Browser and real-client journeys remain in their current suites.

Shared types belong to the contract module:

```ts
type GateID = 'bindings' | 'database' | 'lab' | 'web' | 'jsforce' |
  'python' | 'cli' | 'restart' | 'connected-browser' | 'preview' | 'cleanup';
type Outcome = 'passed' | 'failed' | 'blocked';
type GateOutcome = Outcome | 'not-run';
type Scope = 'full' | 'candidate-only';
interface Coverage { expected: string[]; observedRequired: string[];
  passed: number; failed: number; skipped: number; allowedSkipped: string[] }
interface GateResult { id: GateID; outcome: GateOutcome; coverage: Coverage;
  categories: string[]; startedAt: string | null; finishedAt: string | null }
interface ParsedResult { schemaVersion: 1; runId: string; suite: GateID;
  complete: boolean; exitCode: number | null;
  tests: { id: string; outcome: 'passed'|'failed'|'skipped' }[] }
interface FixtureEvent { schemaVersion: 1; runId: string; operationId: string;
  principal: 'admin'|'restricted'|'other'; object: 'Account'|'Contact';
  names: string[]; kind: 'intent'|'known'|'deleted'|'absent'|'rejected'; id?: string }
```

`AcceptanceReport` contains schemaVersion=1, kind=`candidate-acceptance`, runId, scope, outcome, acceptanceEligible, UTC times, bindings, all gate results, cleanup disposition and sanitized evidence references. `acceptanceEligible` is derived: full scope, clean source, every required gate passed and successful publication. Candidate-only success is never full acceptance. Bindings contain revision/source-manifest/lockfile/catalog hashes, actual tool versions and executable/asset hashes; never DSNs, tokens, org IDs or raw command output.

`GateContext` is private: runId, scratch, deadline, database URL, owned service URL, principal tokens, fixtureJournal and per-gate result paths. Never serialize it into the public report. `CommandSpec` uses executable, argv array, cwd and explicit environment; no shell interpolation of values.

---

### Task 1: Define report, coverage manifest and result adapters

**Files:** Create contract/schema/results/reporter modules and contract/results tests from the map. Add no execution entry point yet.

**Interfaces:** Produce `requiredCoverage: Record<GateID, readonly string[]>`, `assessCoverage(gate: GateID, result: ParsedResult): GateResult`, `validateAcceptanceReport(value: unknown): asserts value is AcceptanceReport`, `aggregateReport(report: AcceptanceReport): AcceptanceReport`, `readGateResult(kind: 'go'|'node'|'playwright'|'steps', path: string, runId: string): Promise<ParsedResult>`, `mergeGateResults(gate: GateID, results: ParsedResult[]): ParsedResult` and `publishAcceptanceReport(path: string, report: AcceptanceReport): Promise<void>`. `ParsedResult` is the shared private protocol above; adapters never publish raw error objects. `mergeGateResults` requires the same run/suite and preserves all failures/skips across component commands such as Go tests/vet and web units/build. For native output without run identity (Go/Node/Playwright), `runCommand` writes a sibling `<result>.binding.json` after flushing result bytes, recording schemaVersion=1, runId, gate ID, command/source digests, result digest, exit code and completion state. `readGateResult` verifies that descriptor and native terminal events; a stale or truncated native file cannot acquire the current run's identity merely from its filename. Step-result writers emit the same run/suite fields directly.

- [x] Write contract/results tests with hand-authored events and reports. Include these behavior assertions:

  ```ts
  assert.equal(assessCoverage('jsforce', zeroSelected).outcome, 'failed');
  assert.equal(assessCoverage('jsforce', skippedRequired).outcome, 'failed');
  assert.equal(assessCoverage('lab', onlyAllowedCandidateSkips).outcome, 'passed');
  assert.equal(aggregateReport(candidateOnlySuccess).acceptanceEligible, false);
  assert.throws(() => validateAcceptanceReport(forgedFullPass));
  ```

  Construct the named fixtures locally in the test from literal event/report shapes. Also test truncated/empty/malformed results, another run/suite, duplicate terminal events, suite failure despite passing children, invalid UTC times, failed cleanup, missing build hashes and an existing output file. Feed token/DSN/auth-JSON canaries through raw failure records and assert none appear in serialized public output.
- [x] Run from the Node lab: `node_modules/.bin/tsx --test test/candidate-acceptance-contract.test.ts test/candidate-acceptance-results.test.ts`. Expected: missing module/export failures before implementation.
- [x] Implement the schema, report reducer and bounded parsers. Go uses `go test -json`; Node uses a small custom reporter writing completion/test events as private NDJSON; Playwright uses its built-in JSON reporter; Python/CLI/restart use private versioned step results. Validate completion/run/suite binding before assessing coverage. Publish by exclusive output reservation plus atomic replacement, with publication errors propagated to the command exit.
- [x] Populate required identities from the current Go/web/lab tests and the stable new client/browser identities defined in Tasks 4/5. Protect applicable database tests, including `TestStoreIsolationAndPersistence`, catalog fresh/upgrade/rollback tests and HTTP upsert/body/length/path tests. Require all applicable Go tests to execute without skips. Permit only the explicitly named real-candidate test skips in the offline lab phase; the separate JSforce gate permits none. New tests may increase counts without changing acceptance thresholds.
- [x] Run the focused tests and `npm run typecheck` in the Node lab. Expected: PASS. Commit `feat: define candidate acceptance coverage and reports`.

### Task 2: Preflight, source binding and bounded process execution

**Files:** Create process/preflight modules, matching tests and the controlled test fixture helper; create `compat/candidatecheck/main.go` and `compat/candidatecheck/main_test.go` for the read-only database prerequisite command.

**Interfaces:** Consume Task 1 types. Produce `preflightAcceptance(options: AcceptanceOptions, env: NodeJS.ProcessEnv): Promise<PreparedRun>`, `collectSourceBindings(root: string): Promise<SourceBindings>`, `runCommand(spec: CommandSpec, context: ProcessContext): Promise<CommandResult>`, `stopOwnedProcesses(context: ProcessContext): Promise<void>`. `PreparedRun` carries the private GateContext plus initial public bindings; ProcessContext owns registered spawned children, log paths and the absolute deadline. Define `AcceptanceOptions { scope: Scope; out: string; port?: number }` and `CommandResult { exitCode: number | null; complete: boolean; resultPath: string; bindingPath: string; categories: string[] }`. The command layer allocates `out` privately for legacy candidate-only mode before calling preflight; full mode requires user-supplied output. Options never accept an arbitrary endpoint. Process execution writes Task 1's digest-bound result descriptor only after the child and output streams finish.

- [x] Write executable harness tests for absent opt-in/database/tools/browser, conflicting URLs, unsafe/existing output, incompatible versions, inherited external candidate/reference endpoints, occupied ports, stale source/bundle and dirty full-run source. Every preflight rejection must assert zero provision commands. Use controlled child scripts to test nonzero exits, hanging grandchildren, SIGINT/SIGTERM, and a 20-minute absolute deadline via an injectable clock/deadline in the process helper. Assert unrelated PIDs/listeners survive and secret canaries never reach public console/summary.
- [x] Run `node_modules/.bin/tsx --test test/candidate-acceptance-process.test.ts test/candidate-acceptance-preflight.test.ts`. Expected: missing exports before implementation. Run `go test ./compat/candidatecheck` for the new Go prerequisite helper tests; expected missing implementation first.
- [x] Implement explicit environment allowlisting. Pass one matching database URL to database tests and the owned service; strip inherited `COMPAT_REFERENCE_*`, `CANDIDATE_*` endpoint/session values and user CLI state before supplying generated values. Preserve required tool paths/locale and documented local Python/Chromium overrides. Use private 0700 scratch/0600 files, argv-based spawning, bounded logs and owned process groups. On termination allow at most ten seconds for graceful shutdown, then terminate/reap the owned group; never target a port or a broad process name.
- [x] Preflight the installed lock versions through existing version helpers, Go toolchain, Python import/version, PostgreSQL server major 17, TLS tooling and Chromium launchability without provisioning or schema writes. Check the configured candidate and preview ports. Full scope requires clean tracked source before fixture work; candidate-only mode can diagnose a dirty source but is never acceptance-eligible. Use a small standalone Go harness utility at `compat/candidatecheck` to connect using the environment's `DATABASE_URL` and existing pgx dependency, execute only connectivity/server-version reads, and emit allowlisted `{reachable, major}` JSON. It must not call `platform.Open`, apply schema or print connection errors containing a DSN. Include tests for invalid DSNs, sanitized failures and version parsing; final PostgreSQL integration proves actual connectivity. No new psql or database-client package requirement. Run Go subprocesses with local toolchain selection, read-only modules and network module retrieval disabled; preflight must block on uncached modules rather than download them. Dependency cache preparation stays in setup. Never install/download during the run.
- [x] Hash sorted `(relative path, file bytes)` for tracked execution inputs: `go.mod`, `go.sum`, `cmd`, `internal`, `scripts`, `compat`, `web` source/config/locks, `locales`, and the platform workflow. Exclude ignored outputs and documentation-only files; record revision and tracked dirty state independently. Reject untracked non-ignored execution inputs under these roots, so an uncommitted Go/Vue/test file cannot affect a build outside its source manifest. Build fresh artifacts during their gates and hash the actual executable and sorted web asset bytes. Verify source inputs have not changed again before publication; changed source blocks acceptance.
- [x] Run focused tests and typecheck. Expected: PASS. Commit `feat: bound candidate acceptance processes and preflight`.

### Task 3: Share the owned service lifecycle, fixture journal and restart proof

**Files:** Create fixtures/service modules, service tests and the restart functions in clients module. Refactor the lifecycle currently embedded in `scripts/check-candidate.sh`; the final thin-wrapper switch waits for Task 6.

**Interfaces:** Consume Tasks 1/2. Produce `startCandidate(context: GateContext): Promise<OwnedService>`, `stopCandidate(service: OwnedService): Promise<void>`, `restartCandidate(context: GateContext, service: OwnedService): Promise<OwnedService>`, `recordFixtureEvent(path: string, event: FixtureEvent): Promise<void>`, `cleanupFixtures(context: GateContext): Promise<CleanupResult>`, `seedRestartFixture(context: GateContext): Promise<void>` and `verifyRestartFixture(context: GateContext): Promise<ParsedResult>`.

`OwnedService` records its spawned process handle/PID and generated URL. The private JSONL FixtureEvent contract records runId, unique operation ID, principal alias (`admin`, `restricted`, `other`), object, exact allowed Names and create intent/returned ID/cleanup state. It never contains tokens. Expose journal path and runId to external clients as `CANDIDATE_FIXTURE_JOURNAL` and `CANDIDATE_RUN_ID`. Step outputs use `CANDIDATE_STEP_REPORT`, always a new private file.

- [x] Write tests using controlled local HTTPS processes: foreign listener is refused; readiness cannot outlive a failed child; restart must observe old-process exit and a new PID; changed fixture values, lost principals, wrong relationship or missing record fail restart; failed delete/non-absence blocks cleanup. Simulate a client dying after create intent and after recording an ID; retain incomplete intent/known IDs and never report fully verified cleanup from missing ownership information.
- [x] Run `node_modules/.bin/tsx --test test/candidate-acceptance-service.test.ts`. Expected: missing functionality failures.
- [x] Move existing TLS/provision/start/stop behavior into shared helpers using two fresh tenants and three principals per invocation. Generate a fresh local certificate, pass it through Node/Python/CLI trust and use only the generated loopback service URL. Verify child liveness and trusted readiness; do not reuse an already running service. Isolate CLI state under this run's scratch and suppress interactive/telemetry/update behavior through supported installed-CLI settings.
- [x] Implement durable fixture intent/ID recording before subsequent client operations and exact-ID cleanup while the service is still alive. Verify ownership against the operation's recorded Names, use the right tenant token, delete Contacts before Accounts, and independently verify absence. An incomplete or ambiguous create remains a failed/blocked fixture disposition for this run; retain private diagnosis and require operator database disposal rather than adding a general reference-recovery subsystem. Report retained tenant/principal disposition separately from record cleanup.
- [x] Retain one Account with both custom fields non-null and a linked Contact. Verify exact IDs/values and Contact.Account.Name before and after actual process replacement; reuse the original tokens, verify restricted denial and other-tenant invisibility after restart. Do not reseed after restarting. Existing Go catalog upgrade tests remain in the database gate.
- [x] Run focused tests and typecheck. Expected: PASS. Commit `feat: share candidate fixture lifecycle and restart checks`.

### Task 4: Complete the real JSforce, Python and native CLI journeys

**Files:** Modify `compat/clients/node/test/candidate.test.ts`, `scripts/candidate-python.py`; complete `candidate-acceptance-clients.ts`; create clients tests. Update required coverage identities in Task 1's manifest in the same commit.

**Interfaces:** Consume Task 3 GateContext/journal/step report contracts. Produce JSforce identities `candidate/jsforce/core`, `candidate/jsforce/policy`, `candidate/jsforce/isolation`; Python step IDs `python/relationship`, `python/update`, `python/denied`, `python/cleanup`; CLI step IDs `cli/login`, `cli/create`, `cli/read`, `cli/update`, `cli/query`, `cli/delete`, `cli/absence`. Produce `runNativeCLI(context: GateContext): Promise<ParsedResult>`. The existing `RUN_CANDIDATE=1` opt-in remains; ordinary offline lab runs explicitly skip these real-client identities.

- [x] Add real JSforce assertions covering describe/read/write/select/filter/order for restricted `Compat_Text__c`, allowed Name/Contact operations, other-tenant update/delete/Contact-link denial, unchanged records after denials, and equal external keys in different tenants. Keep existing metadata/null/omission/exact-key upsert behavior. Split the current broad test into the three stable identities above, with independent run-owned fixtures and immediate journal records.
- [x] Extend Python with real Account/Contact creation, `SELECT Id,LastName,Account.Name FROM Contact WHERE AccountId = '<owned ID>'`, an allowed update and denied cross-tenant mutation with admin readback. Preserve native exception/TLS behavior. Implement its small journal/step-result writer using standard-library JSON/file/fsync support; delete children first and assert exact absence. Add local transport-based harness regressions for corrupt step output/cleanup errors, keeping accepted runs on unmodified simple-salesforce against the real candidate.
- [x] Run the new real-client tests against the owned disposable candidate before completing their helpers and inspect expected missing coverage/failed assertions; prove new failure-detection behavior with controlled local harness fixtures where the existing policy assertions already pass. Separately run `node_modules/.bin/tsx --test test/candidate-acceptance-clients.test.ts` for result failure cases. Expected: new missing harness/coverage behavior fails before implementation; existing correct policy assertions may pass and must not be represented as newly fixed product bugs.
- [x] Implement the native CLI journey with the locally inspected installed flags (help checked during planning): `data create record --sobject Account --values <one argv value>`, `data get record --record-id <id>`, `data update record --record-id <id> --values <one argv value>`, and `data delete record --record-id <id>`, each with `--target-org <run alias> --api-version 65.0 --json`. Login remains `org login access-token --instance-url <owned URL> --alias <run alias> --no-prompt --json` with `SF_ACCESS_TOKEN` in the child's environment only. No shell assembly of Name/ID/values.
- [x] Validate CLI JSON status and create-returned ID, then prove actual values through get/query after update and exact-ID absence after delete. Keep raw CLI result shapes private; determine projections from the installed plugin implementation/native results rather than infer wire status. A nonempty result or exit zero alone cannot satisfy an operation. Ensure failure at any stage preserves the fixture journal for Task 3 cleanup.
- [x] Run focused clients tests, typecheck and all three real clients against the owned service. Expected: every required identity executed, zero skips, exact cleanup; otherwise record a genuine blocker. Commit `test: complete candidate client and policy acceptance`.

### Task 5: Cover connected roles, all locales and preview separation

**Files:** Modify `web/test/records.spec.ts`, `web/playwright.config.ts`, `web/playwright.preview.config.ts` and, only where needed for boundary coverage, `web/test/preview.spec.ts`. Add a Node-side browser fixture-journal helper under `web/test/helpers/candidate-fixtures.ts`. Update required browser identities in the acceptance manifest.

**Interfaces:** Consume generated candidate environment/journal from Task 3. Produce connected coverage IDs `connected/admin`, `connected/restricted`, `connected/other-tenant`, `connected/locales`, `connected/token-boundary`, plus the existing failed-refresh and preview identities. Playwright results are private JSON files selected by the orchestrator; report paths/output directories come from the run, with no raw public artifact upload.

- [x] Write browser assertions against the Go-served production bundle: restricted field has no editable/displayed value while an allowed Name edit persists; the other tenant cannot see the admin fixture; switching to preview and disconnecting clear the connected token/session; all original failed-refresh scenarios remain. Journal API/browser creates with exact allowed Names and returned IDs so cleanup survives test failure. For UI creates, write the intent before clicking Save and observe the actual owned POST response with Playwright response events before recording its returned ID; do not add a product-only test endpoint or replace real writes. Use separate browser contexts for different identities.
- [x] Add one language-selection journey covering `(en_US,en-US,ltr)`, `(fr,fr,ltr)`, `(de,de,ltr)`, `(pt_BR,pt-BR,ltr)`, `(iw,he,rtl)`. Assert document lang/dir after committed selection and `action.new` labels inspected locally: `New record`, `Nouvel enregistrement`, `Neuer Datensatz`, `Novo registro`, `רשומה חדשה`, respectively. Retain unit tests for latest-selection-wins/storage failures and responsive/keyboard assertions; do not assert translation completeness.
- [x] Run `npm --prefix web run test:e2e` against the owned service before completing missing test helpers. Expected: new helper/coverage failures first; if an assertion reveals a real documented candidate defect, reproduce it before a bounded fix. Do not change unresolved reference semantics to pass tests.
- [x] Add `CANDIDATE_ACCEPTANCE=1` handling to browser configs: private output directory/result reporter, one worker, no retries and no preview server reuse. In acceptance mode, preview serves the production bundle already built/hashed by the web gate instead of rebuilding it; preserve the standalone preview suite's usual build behavior. Retain existing developer defaults outside acceptance mode. Strip connected tokens/URL from the preview subprocess, retain zero-service-request assertions and refuse an occupied preview port; do not let a stale developer server supply the preview result. Leave raw traces/DOM/token-entry screenshots disabled and controlled existing screenshots private.
- [x] Run `npm --prefix web test`, production build, connected e2e and preview tests in separate contexts. Expected: required identities executed without skips, production assets tested, forbidden preview requests fail, locale/role/token assertions pass. No interactive visual-acceptance claim. Commit `test: verify candidate browser roles and locale boundaries`.

### Task 6: Assemble the full entry point and CI evidence handling

**Files:** Create `candidate-acceptance-run.ts`, `candidate-acceptance-command.ts`, `scripts/check-candidate-acceptance.sh` and run tests. Replace `scripts/check-candidate.sh` internals with the compatible thin candidate-only entry point. Modify `.github/workflows/platform.yml`; add workflow behavior tests in the Node lab run test file.

**Interfaces:** Consume Tasks 1–5. Produce `runAcceptance(options: AcceptanceOptions, env: NodeJS.ProcessEnv): Promise<AcceptanceReport>` and `candidateAcceptanceMain(argv: string[], env: NodeJS.ProcessEnv): Promise<0|1|2>`.

Full entry point: `CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out /absolute/new-summary.json`. Existing `scripts/check-candidate.sh` stays usable as candidate-only mode backed by the same lifecycle/client helpers; it can allocate its own private diagnostic output when none is supplied, reports scope explicitly and cannot be final acceptance. Full mode requires explicit `--out`. Never let arbitrary gate selection/skip flags create a full passed report.

- [x] Write run tests using controlled executable fixtures: gate ordering, zero selected tests, an allowed lab skip versus a forbidden client skip, exit-zero malformed/missing/stale output, final source/bundle changes, foreign listener, restart failure, deadline/signal, cleanup after an earlier failure and report-publication failure. Assert all later gates are not-run when appropriate, original error categories survive cleanup failure, unrelated processes survive and raw secret canaries never enter public output. Verify full versus candidate-only scope cannot be forged.
- [x] Run `node_modules/.bin/tsx --test test/candidate-acceptance-run.test.ts`. Expected: missing orchestration exports/entry point failures.
- [x] Implement the ordered full run: initial bindings/preflight → `go test -race -count=1 -json ./...`/vet → existing offline lab check → web units/production build → owned candidate with JSforce/Python/CLI → retained-fixture process restart verification → connected browser → credential-free preview → exact fixture cleanup/owned-process shutdown → final bindings/report. Capture native structured results privately and evaluate Task 1 coverage. Finalize executable/asset hashes after fresh builds and verify unchanged source/artifacts again before publication. Reuse Go build outputs and the production bundle where applicable.
- [x] Preserve `npm --prefix compat/clients/node run check` as the lab gate: add an acceptance-mode reporter output hook to its existing test command through a small npm script/argv forwarding change if needed, keeping normal `npm test/check` behavior intact. Do not silently replace its typecheck/format/tests/validate components with a weaker command. Web units likewise use the Node result adapter; Python/CLI/restart step outputs bind to this run. All subprocess logs stay private, including failures.
- [x] Update platform CI setup to install locked Node/Python dependencies and Chromium first, then call the full entry point once. Remove duplicated standalone Go/lab/web/client/preview checks now owned by that command. Reserve setup/upload time beyond the runner's 20-minute deadline (raise the outer job timeout to 30 minutes). Use an `if: always()` artifact step restricted to the sanitized summary path outside the private scratch; never upload logs, CLI state or Playwright directories. If no summary exists, retain the failed job rather than manufacture a pass.
- [x] Test CI shell behavior with controlled npm/process commands and sentinel files: acceptance executes exactly once; its failure remains nonzero; only the sanitized summary is selected for upload; setup failure never provisions. Validate workflow structure through existing YAML tooling as a secondary check, not source-text matching as the sole test. Run focused run tests plus the full lab check. Expected: PASS with only declared offline real-client skips. Commit `feat: integrate strict candidate acceptance locally and in CI`.

### Task 7: Execute repeatable acceptance, review and publish the decision

**Files:** Create `docs/compat/plan-15-candidate-acceptance.md`; update `docs/compat/plan-2-implementation.md`, the Plan 15 spec status and this plan's checkboxes/results. No accepted reference profile changes.

**Interfaces:** Consume final run command and actual sanitized summaries. Publish the exact accepted code revision/source/tool/binary/asset bindings, gate/coverage results, cleanup disposition and two report digests. Documentation-only follow-up commits may cite the tested code revision explicitly; never relabel an older report as testing a newer code change.

- [x] Verify all focused task tests, full lab check and whitespace checks, and commit the runtime/test changes before strict acceptance. Run one fresh whole-change review against the spec/plan and resolve Important/Critical issues using failing-then-passing regressions. Commit corrections before the two final acceptance invocations. Do not weaken required coverage to force acceptance.
- [x] Prepare the documented dedicated disposable PostgreSQL 17 instance, locked clients/Python environment, cached Go modules (`go mod download` during setup) and installed Chromium. Use existing setup commands; do not read reference private files or change reference environment. If anything is unavailable, record exact missing prerequisites and unexecuted gates while preserving offline implementation/test evidence.
- [ ] With an otherwise clean tracked source tree and external output directory, execute the full command twice sequentially against the same disposable database with different output names. Expected: both summaries passed/full/acceptanceEligible, every required gate executed, zero unexpected skips, distinct run fixtures, first process/record cleanup verified before run two and retained-principal disposition explicit. A blocked or failed run does not satisfy this step.
- [x] Verify both summaries against the schema and required coverage; compute SHA-256 digests and inspect the published field allowlist. Confirm no secrets/private artifacts enter the documentation or CI artifact selection. Record an actual GitHub Actions run only if observed; configured CI is a separate fact from executed CI.
- [x] Write the decision/setup documentation with commands, actual versions/counts/skips, both local results/digests, tested source revision, catalog v2, retained principals and cleanup, review findings/disposition and all remaining M1/M2/reference/visual exclusions. If live candidate execution was blocked, label the package's tooling and its acceptance status separately; no fabricated accepted report.
- [x] Run `git diff --check` and check documentation links. Expected: clean. Commit `docs: record candidate acceptance verification and remaining gates`. Leave all implementation steps unchecked until their actual commands/results exist.

## Self-review record

Plan self-review checked every spec section against Tasks 1–7. Source/build binding, strict coverage and privacy map to Tasks 1/2/6; ownership/deadline/restart to Tasks 2/3; real-client matrix to Task 4; browser/locales/preview to Task 5; two clean sequential runs, CI and truthful blockers to Tasks 6/7. Shared GateContext/report/result/journal names and dependency order were checked for consistency. All five Review Focus conditions have named behavioral tests. No product implementation or acceptance execution occurred while this plan was originally written; the later execution record follows.

## Execution record — 2026-10-21 UTC

Implementation ran natively on the required `main` branch; no feature branch or worktree was created. One fresh whole-change reviewer requested corrections for four Important issues, all fixed with regressions before the final report pair. Minor findings and explicit catalog-input hashes were also addressed. The private execution ledger recorded RED/GREEN commands and commit boundaries.

Task implementations are committed and focused/full lab checks pass. Controlled child scripts and injectable requester tests live directly in the matching test files rather than a separate helper module. Native structured parser/coverage negatives, live disposable-database runs and separate real browser diagnostics provide complementary evidence; a separate fake HTTPS reproduction was not created for every proposed scenario. Existing correct candidate behavior was not represented as a product bug fixed by this work.

The two final invocations were executed, with clean source and verified cleanup between them, but both failed the native nonempty CLI query. The successful-acceptance checkbox remains open intentionally. Downstream restart/connected/preview gates remain `not-run` in those full reports; separate diagnostics do not convert them into passing full acceptance. CI is configured and unobserved. The detailed decision record binds each result to its actual tested revision; documentation-only commits do not relabel that revision.
