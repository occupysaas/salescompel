# Plan 10 — Foundation Evidence Follow-ups Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task by task. Steps use checkbox (`- [ ]`) syntax for tracking. The maintainer requested native sequential work in the design; do not dispatch parallel implementers.

**Goal:** Produce two fresh, safely collected foundation captures and an auditable development decision that separates measured behavior from the authorized two-day cursor assumption, or records exact external blockers.

**Architecture:** An explicitly selected, fixed collection policy changes only the foundation cursor expiry probe. An immutable sidecar binds each raw capture to that policy and the current probe/toolchain. A separate strict assessor validates two sequential captures, full measured coverage, cleanup and narrow tied-row equivalence without changing the existing observed-only capture or evidence validators.

**Tech Stack:** Existing TypeScript/Node compatibility lab, pinned JSforce/CLI, public REST API 65.0, YAML release records; no new runtime dependencies.

**Spec:** [Plan 10 foundation follow-ups design](../specs/2026-10-19-plan-10-foundation-followups-design.md).

## Global Constraints

- Keep product code, routes, catalog, profiles, accepted pilot/foundation observations, old captures and prepared expiry checkpoints unchanged.
- Use the existing TypeScript lab and pinned, unmodified public clients; never inspect CLI/plugin source.
- The sole expiry waiver is `foundation-query-result-lifetime`: documented two-day availability, API 65.0, maintainer authorization on 2026-10-19. Exact boundary, eviction and retention anchor remain unknown.
- Do not run the late-checkpoint helper, wait longer than one hour for expiry, poll in the background, forge assertions or call an assumed capture observed-only passing.
- Preserve default collection, raw order, report format, `compare` and `verify-evidence` semantics. No arbitrary waiver or skipped-scenario flag.
- Private raw artifacts stay outside the checkout, owner-only (directories 0700, files 0600), exclusive and symlink safe. No tokens enter artifacts or published notes.
- Recheck existing reference authorization, administrator/fresh/restricted identities, metadata and recovery before fixture writes. Run two fresh captures sequentially, with verified exact-ID cleanup and absence before run two. Never blindly retry potentially committed mutations.
- Release review is limited to selected Plan 5 contracts/API 65.0; inaccessible material retains owner, reason and next action. A relevant unresolved release finding blocks a ready decision.
- A ready development decision is neither full Plan 5 completion nor observed-only evidence registration. Plan 9 body-field validation and product changes need a separate design.

## Review Focus

1. A process stops after writing the capture but before publishing its sidecar: the assessor must reject the orphan capture — Task 2.
2. A capture is edited after sidecar publication, or a source/profile/client version changes: binding mismatch must block assessment — Tasks 2 and 4.
3. One required observation or operation is absent while scenario assertions still say passed: fixed coverage must block assessment — Tasks 1 and 4.
4. A tied traversal permutes records across pages while losing or duplicating one identity: invariants must reject it — Task 3.
5. Cleanup says passed but exact-ID absence or first-run recovery state is missing: run two and ready assessment must remain blocked — Tasks 4 and 6.

---

### Task 1: Fixed collection policy and cursor coverage manifest

**Files:** Create `compat/clients/node/src/foundation-policy.ts`, `compat/clients/node/test/foundation-policy.test.ts`; modify `src/foundation.ts`, `src/capture.ts`, `src/cli.ts`, `test/foundation.test.ts`.

**Interfaces:**
- `FOUNDATION_ASSUMPTION_ID = "foundation-query-result-lifetime"` and `FOUNDATION_POLICY_VERSION = 1`.
- `type FoundationCollectionPolicy = "observed-only" | "documented-two-day-assumption"`.
- `foundationCoverage(): Readonly<Record<scenarioId, {observations: readonly string[]; assertions: readonly string[]}>>` is a fixed, reviewed manifest for the current probe source; dynamic page labels have explicit cardinality/shape rules, not a caller-supplied allowlist.
- `capture(target, env, suiteId, policy = "observed-only")` passes the policy only to the foundation cursor handler. Reject assumption policy for other suites/targets if they are outside the fixed reference scope.

- [ ] Write tests showing default collection still reaches `expiry-start` and `after-16-minutes`, while the selected foundation policy omits exactly those two observations and the delay. Assert replay, mutation, identity/session, unknown/malformed locator and complete `ties` traversal still execute. Assert an unknown policy or non-foundation use fails before fixture writes.
- [ ] Run `node --import tsx --test test/foundation-policy.test.ts test/foundation.test.ts` from `compat/clients/node`; observe the new tests fail for the missing policy.
- [ ] Implement the fixed policy and coverage manifest. Under the assumption policy, record the cursor expiry dimension as unmeasured in the result reason/outcome; do not mark it passed or add a passing expiry assertion. Keep default behavior unchanged.
- [ ] Run the targeted tests and `npm run check`; commit this unit.

### Task 2: Immutable capture-policy sidecar and private command

**Files:** Create `compat/clients/node/src/foundation-sidecar.ts`, `compat/clients/node/test/foundation-sidecar.test.ts`; modify `src/cli.ts`, `src/reports.ts` only where the policy command needs safe output handling.

**Interfaces:**
- `capture-foundation-assumed --target reference --out <private.json>` explicitly selects the Task 1 policy; ordinary `capture` retains its current behavior. No generic skip/waiver option.
- `FoundationSidecar` schema version 1 binds raw capture SHA-256, profile ID/hash, API 65.0, exact connector versions, SHA-256 of the executed `foundation.ts` and fixed coverage/policy manifest, assumption ID, policy version, start/finish/publication times and the authorized documentary source URL.
- `writeFoundationSidecar(capturePath, report, binding): Promise<string>` writes `<capture>.policy.json` exclusively after the raw capture exists, with validated private ownership and no symlink traversal. A failed sidecar write leaves the run ineligible.

- [ ] Write tests for byte-level digest binding, exclusive creation, 0600/0700 permissions, wrong owner/mode, symlinked directory/file, checkout paths, interrupted publication, missing capture and no secrets in the sidecar. Show old report bytes remain unchanged.
- [ ] Run `node --import tsx --test test/foundation-sidecar.test.ts`; observe failures.
- [ ] Implement sidecar validation/writing using existing private-directory helpers where safe. Ensure the command validates destination/recovery state before collection, writes the immutable capture first, then publishes its sidecar; reserve an exclusive destination before network work so existing files cannot be overwritten.
- [ ] Run targeted tests and `npm run check`; commit this unit.

### Task 3: Narrow, invariant-checked tie comparison

**Files:** Create `compat/clients/node/src/foundation-ties.ts`, `compat/clients/node/test/foundation-ties.test.ts`.

**Interfaces:**
- `compareFoundationTies(a: Result, b: Result): {issues: string[]; diagnostics: string[]}` accepts only the fixed `foundation-cursors` fixture. It reads raw `ties:0...n` observations and identity maps, then compares normalized full record-payload multisets after validating each raw traversal.
- Validate 201 expected owned IDs (`tie-0` through `tie-200`) exactly once, status 200, valid totalSize/done/nextRecordsUrl transitions, batch bounds, terminal page, projected values and absence of foreign IDs. Preserve raw order and page association in reports; order/page movement becomes a diagnostic only.

- [ ] Write tests where different tied order and page partition pass with diagnostics. Separately fail dropped/duplicated/foreign IDs, changed projected value, failed page, invalid route, bad done/link transition, wrong totalSize, oversized page and truncated traversal. Show unrelated ordered query changes still fail in Task 4 comparison.
- [ ] Run `node --import tsx --test test/foundation-ties.test.ts`; observe failures.
- [ ] Implement the narrow validator/comparator without sorting arbitrary observations or changing replay/session comparison.
- [ ] Run targeted tests and `npm run check`; commit this unit.

### Task 4: Strict assumption-aware pair assessment

**Files:** Create `compat/clients/node/src/foundation-assessment.ts`, `compat/clients/node/test/foundation-assessment.test.ts`; modify `src/cli.ts`, `package.json`.

**Interfaces:**
- `assessFoundationPair(firstCaptureBytes, firstSidecar, secondCaptureBytes, secondSidecar, currentBinding, cleanupEvidence, releaseFindings): FoundationAssessment` returns `ready-with-assumptions | drift | blocked`, distinct pair issues, release issues, tie diagnostics, bindings/times/digests and observed/assumed/differing/blocked dimensions.
- `assess-foundation --first <private.json> --second <private.json> --first-policy <private.json> --second-policy <private.json> --first-cleanup <private.json> --second-cleanup <private.json> --out <private.json>` writes one exclusive private assessment. Cleanup evidence must bind each capture digest and exact owned-ID absence; the command never accepts a skip list or freeform waiver.
- Fixed coverage from Task 1 supplies exact scenario IDs, observation labels/shape and assertion names/counts. Only `expiry-start` and `after-16-minutes` are absent by policy; no other operation is exempt.

- [ ] Write tests rejecting missing/malformed sidecars, stale or same/overlapping captures, different profile/API/client/probe source, wrong policy/assumption, synthetic reports, omitted/malformed operations, failed assertions, session/auth errors, unknown outcome, absent/failed cleanup or absence attestation. Verify old corrected-away captures fail the fresh-policy/probe gate.
- [ ] Write comparison tests showing a valid tie permutation is ready with diagnostics, but changed unrelated ordered observations, replay/session ordering, status, spelling, fields or error properties become `drift`. A relevant unresolved release finding blocks the development decision and is listed separately.
- [ ] Run `node --import tsx --test test/foundation-assessment.test.ts`; observe failures.
- [ ] Implement strict input schemas and comparison. Reuse `validateReport` only as a baseline; do not change `compare`, `verifyCapturePair` or `observationIssues`. Assessment output must not copy private record IDs, token values or raw response prose.
- [ ] Run targeted tests and `npm run check`; commit this unit.

### Task 5: Focused official-source reconciliation

**Files:** Modify `docs/compat/releases/changes.yaml`, `docs/compat/plan-5-contract-decisions.md`; add a concise source-access record under `docs/compat/releases/` if the current schema cannot carry each attempted URL, version, access result and independent summary.

**Interfaces:** The assessor reads the current selected-contract finding statuses from a fixed release input derived from `changes.yaml`; it reports unresolved relevant findings separately. Documentary two-day availability and maintainer permission to assume it remain distinct facts.

- [ ] Recheck the official API Query Cursor Limits page and accessible official release-pinned HTML/PDF sources for selected Plan 5 contracts/API 65.0. Record exact URL/version, UTC access time, access outcome and independent summary. Do not expand the 71-document archive or infer content from an inaccessible shell.
- [ ] Update finding status only when source text supports it; otherwise retain owner, reason and next action. Test or validate the release-input parser against resolved and unresolved relevant findings, with unresolved findings blocking Task 4 readiness.
- [ ] Run the relevant parser test and `npm run check`; commit this unit.

### Task 6: Sequential live collection, review and decision record

**Files:** Add `docs/compat/plan-10-foundation-decision.md`; update `docs/compat/implementation-log.md` and `docs/compat/plan-5-contract-decisions.md` with bounded outcomes. Raw reports, sidecars, absence evidence and assessment stay in private owner-only storage outside the checkout.

**Interfaces:** The decision record cites capture, sidecar, probe, profile and connector digests/times without raw identities, and labels every selected dimension `observed`, `assumed`, `differing` or `blocked`. It states that query-text observations and Plan 9 external-ID equality observations are separate.

- [ ] Recheck named-tenant authorization, administrator/fresh/restricted userinfo, metadata prerequisites and unresolved recovery journals. If credentials or identity proof are unavailable, record the exact blocker and do not collect partial evidence.
- [ ] Run the first new fixed-policy reference capture. Validate sidecar binding, journal cleanup and read-only exact-ID absence for every owned fixture; persist a capture-bound private absence attestation. Do not automatically retry ambiguous mutations.
- [ ] Only after first-run cleanup/absence is verified, repeat for a distinct second session/run. If any recovery state remains unresolved, stop before run two and record the gate.
- [ ] Run the assessor, inspect its private diagnostics and any unresolved release findings. Publish a sanitized decision with exact remaining blockers or `ready-with-assumptions`, plus the bounded next product design boundary. Do not promote observations/profiles or label Plan 5 complete.
- [ ] Run `npm --prefix compat/clients/node run check`, `git diff --check` and diff checks for `api/`, `internal/`, `cmd/`, `web/`, `docs/compat/profiles/` and accepted observations. Request one fresh whole-change implementation review and fix Important/Critical findings with regression tests. Record actual live results and any external gate; commit the decision/documentation.
