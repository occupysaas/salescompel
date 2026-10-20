# Plan 14 — External-ID Normalization Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Capture and compare a bounded external-ID normalization experiment without changing candidate behavior.

**Architecture:** A separate immutable manifest and strict report schema drive sequential wire probes. Adapt the established body-suite recovery and artifact validation pattern into suite-specific modules, reusing low-level transport, identity and private-file primitives. Compare observations without imposing a proposed normalization algorithm.

**Tech Stack:** Existing Node/TypeScript, Ajv, node:test, locked compatibility clients; no dependency changes.

**Spec:** `docs/superpowers/specs/2026-10-20-plan-14-external-id-normalization-contract-design.md`

## Global Constraints

- API 65.0, existing profile and client pins; work directly on `main`.
- 36 scenarios, 80 non-cleanup operations, at most 64 potentially created fixtures in the fixed suite.
- Outer ceilings: 80 records, 160 non-cleanup mutations, ten-minute collection, five-minute cleanup, sequential requests, 15-second wire deadlines, no mutation retries.
- No product, catalog, migration, UI or historical suite semantic changes.
- Private evidence remains outside checkout, files 0600/directories 0700, exclusive writes and durable journals.
- Unknown dispatched outcomes and latched identity mismatch block clean status and subsequent capture.

## Review Focus

- Unexpected normalized keys must remain measurable without making arbitrary discovered records eligible for deletion (Task 2).
- Failed prerequisite seeds must not cause dependent writes or fabricated complete coverage (Task 3).
- Error responses that commit writes must preserve the hidden state in comparison (Tasks 3/4).
- A delayed commit after empty reads must remain blocked across recovery (Task 2).
- Fresh runs with different synthetic IDs and row order must compare equally while spelling differences remain drift (Task 4).

### Task 1: Manifest and report contract

**Files:** Create `compat/clients/node/src/external-id-normalization-contract.ts`, `compat/schemas/external-id-normalization.schema.json`, `compat/clients/node/test/external-id-normalization-contract.test.ts`.

**Interfaces:** Produce `normalizationManifest`, `normalizationManifestHash`, `normalizationProbeHash()`, `normalizationScope(runId, scenario)`, `normalizationRequest(runId, scenario, step, ids)`, `validateNormalizationReport(value)`, and suite `NormalizationCase/Step/Report/Observation/Cleanup/Assessment` types. Operations carry method, target alias, submitted key and Name; observations retain per-operation before/after state and response/state artifact references. Cleanup uses the established body-cleanup shape. Bind probe hash to all suite modules and reused runtime primitives.

- [x] Write tests pinning 36 cases, 80 steps, 64 maximum creations, directional exact keys, absent upsert body key, immutable manifest, invalid report bindings/coverage and request construction.
- [x] Run `node_modules/.bin/tsx --test test/external-id-normalization-contract.test.ts` from the Node lab; expect missing module/export failures before implementation.
- [x] Implement manifest, short scopes, requests, strict schema and semantic validation including actual timestamps, operation uniqueness and complete coverage.
- [x] Run focused tests and typecheck; expect PASS. Commit `feat: define external ID normalization experiment`.

### Task 2: Durable recovery and ownership

**Files:** Create `compat/clients/node/src/external-id-normalization-recovery.ts`, `compat/clients/node/test/external-id-normalization-test-server.ts`, `compat/clients/node/test/external-id-normalization-recovery.test.ts`.

**Interfaces:** Consume Task 1 manifest/scopes/types. Produce suite-specific equivalents of `initializeNormalizationRun`, `readNormalizationRun`, `appendNormalizationEvent`, `normalizationArtifact`, `normalizationCondition`, `normalizationOwned`, `captureNormalizationState`, `recoverNormalizationRun`, `knownNormalizationRecords`, `normalizationRow`, and cleanup proof helpers, using existing `Transport` and config types.

- [x] Write tests for unknown dispatch and late commit, identity latch, changed key/Name ownership, cross-case ID reuse, corrupt journals, incomplete discovery, delete failure and exact-ID absence.
- [x] Run focused recovery tests; expect missing functionality failures.
- [x] Implement journal bindings (including probe hash), state snapshots and recovery. Discover by exact Names; preflight also checks submitted keys. Require a matching dispatched case and exact journal-derived Name for deletion; retain arbitrary stored key bytes. Unknown Names or duplicate case ownership block cleanup.
- [x] Run focused recovery tests; expect PASS. Commit `feat: recover normalization probe fixtures safely`.

### Task 3: Sequential probe and private command lifecycle

**Files:** Create `compat/clients/node/src/external-id-normalization-probe.ts`, `compat/clients/node/src/external-id-normalization-command.ts`, `compat/clients/node/test/external-id-normalization-probe.test.ts`, `compat/clients/node/test/external-id-normalization-command.test.ts`.

**Interfaces:** Consume Tasks 1/2. Produce `runNormalizationProbe(config, env, dir, injected?) : Promise<NormalizationReport>` and `normalizationCommand(mode, configPath, out, env, first?, injected?) : Promise<NormalizationReport | NormalizationCleanup>`.

- [x] Write tests pinning POST seeds, same/other-record targets, omitted upsert body key, stored normalized spelling, all 80 operations, rejection with hidden mutation, missing IDs, failed seeds, metadata/admin gates, response loss, output exclusivity and shared pending-run locking.
- [x] Run focused probe/command tests; expect missing functionality failures.
- [x] Implement identity/metadata preflight, per-operation journaling and independent before/after reads. Stop dependent operations on failed seeds; retain partial evidence. Commands share the existing external-ID active identity lock and revalidate previous clean evidence/absence before the next run.
- [x] Run focused tests and typecheck; expect PASS. Commit `feat: collect bounded normalization evidence`.

### Task 4: Evidence assessment and CLI

**Files:** Create `compat/clients/node/src/external-id-normalization-assess.ts`, `compat/clients/node/test/external-id-normalization-assess.test.ts`; modify `compat/clients/node/src/cli.ts`.

**Interfaces:** Consume Tasks 1–3. Produce `validateNormalizationCleanupEvidence(config, dir, cleanup, full?)` and `assessNormalizationPair(firstPath, secondPath) : Promise<NormalizationAssessment>`. Commands: `probe-external-id-normalization --config --out`, `recover-external-id-normalization --config --first --out`, `assess-external-id-normalization --first --second --out`. Assessment outcomes: repeatable/drift/blocked, exit 0/1/2; malformed invocations exit 1.

- [x] Write tests for equal runs with different IDs/order/prose, unexpected repeatable outcomes, spelling/hidden-state drift, corrupt artifacts, missing coverage, alias reuse, changed bindings, incomplete absence proofs, overlapping captures and CLI validation.
- [x] Run focused assessment tests; expect missing functionality failures.
- [x] Implement strict artifact/response/state/journal checks, exact coverage, chronological run binding and per-case deterministic alias projection. Omit only message prose and raw transport headers from comparison; retain raw artifacts. Compare normalized ownership prefixes, never normalize key values.
- [x] Run all normalization tests and `npm run check`; expect PASS. Commit `feat: assess normalization captures and expose commands`.

Tasks 3/4 were committed together because command publication depends on cleanup evidence validation. The baseline catalog-validator correction is a separate commit with a failing-then-passing regression. No product catalog or runtime behavior changed.

### Task 5: Decision, verification and whole-change review

**Files:** Create `docs/compat/plan-14-external-id-normalization-contract.md`; update spec/plan status and compatibility README.

**Interfaces:** Consume verified command surface and actual test results; no new runtime API.

- [x] Check presence of reference environment prerequisites without printing values. If unavailable, record exact missing prerequisites and do not make requests or search private files. Otherwise follow the reviewed spec's live preflight and two sequential captures.
- [x] Document actual observed results or blocked live evidence, command usage, bounds, privacy/recovery and deferred product gate.
- [x] Run `npm --prefix compat/clients/node run check` and `git diff --check`; expect PASS, recording any optional skips explicitly.
- [x] Obtain a fresh whole-change review against spec and plan, fix Important/Critical findings with failing-then-passing regressions and rerun the full check. Commit final report and status.

## Execution outcome — 2026-10-20

Offline implementation is complete. The final lab check passed TypeScript, formatting, repository validation and 234 tests, with one optional real-candidate integration test skipped and zero failures. `git diff --check` passed. The fresh review's three Important findings (uppercase prefix aliasing, malformed create-success errors and repeat recovery snapshots) were each reproduced and corrected with failing-then-passing regressions. Blocked assessment now explicitly lists every scenario. No Minor findings were deferred.

Live evidence remains blocked: reference URL/token, disposable opt-in and private recovery directory were unset. No reference requests or private reference artifact reads occurred. See the [decision and run guide](../../compat/plan-14-external-id-normalization-contract.md) for commands, evidence limits and the subsequent product gate.

Execution calls: work stayed natively on main under repository policy; future suite modules were initially included as placeholders in the fixed source-digest list and fully implemented before command delivery; command and cleanup assessment shipped together because their interfaces depend on each other. These choices affect execution/review grouping and invalidate only development fixtures when source bytes change; they do not alter product behavior. The separate partial-catalog validator correction permits declared fallback for explicitly incomplete translations, retaining strict fixture/complete-catalog checks. The live/runtime review exclusions remain explicit in the linked guide.
