# Plan 9 — External-ID Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans under the preserved native sequential preference. Steps use checkbox syntax.

**Goal:** Produce recoverable public-API upsert probes and a reviewed external-ID contract that can support a safe product implementation plan.

**Architecture:** Separate private diagnostic command with a fixed scenario manifest, durable ownership journal and explicit recovery command. Bounded wire/native-client observations feed strict reports and a comparison that preserves spelling, response shape and race outcomes.

**Tech Stack:** Existing TypeScript/Node, Ajv, pinned JSforce and public API65.0; no new runtime dependencies.

**Spec:** [External-ID contract design](../specs/2026-10-18-plan-9-external-id-contract-design.md).

## Global constraints

- Preserve existing Salesforce HTTP paths and native clients; no product/schema changes in Plan 9.
- Require existing named-reference authorization, disposable opt-in and private administrator identity binding before mutations.
- Never overwrite Plan 5 sessions or change pilot/foundation profiles, suites or accepted observations.
- No mutation transport retries, bulk/prefix deletion, permission assignments or metadata writes.
- Private owner-only artifacts outside checkout; no auth headers or sensitive data in published summaries.
- Maximum 80 fixture records, 160 non-cleanup mutations, two concurrent requests, 15-second wire requests, 60-second native operation, ten-minute collection and separate five-minute cleanup budget.
- Unresolved dispatched mutations block verified cleanup and run two, regardless of empty reconciliation reads.
- Unknown/failed/missing evidence is not success. No speculative lower()/citext/collation rule or parity promotion.
- Native existing branch execution remains the preference; no push/deployment requested.

## Review focus

1. A mutation commits but its response is lost: journal-first reconciliation must recover ownership without blind replay — Task 2.
2. A case variant unexpectedly creates another record: discover every owned record and clean exact IDs only — Task 2.
3. Identity changes or ownership cannot be proven: stop mutations/deletes and retain recovery state — Tasks 1–2.
4. Missing responses, changed spelling or identity-alias collisions must not compare as stable — Task 3.
5. Concurrent outcomes differ by winner order: preserve raw outcomes and distinguish diagnostics from universal guarantees — Tasks 2–4.

## Task 1: Fixed scope, private config and report validation

**Files:** Add compat/clients/node/src/external-id-contract.ts, compat/schemas/external-id-contract.schema.json, compat/clients/node/test/external-id-contract.test.ts; register schema validation in src/validate.ts without changing SuiteId.

**Interfaces:**
- `parseExternalIdConfig(value: unknown): ExternalIdConfig`: schemaVersion1, API65.0, foundation profileHash, exact HTTPS origin, administrator user/org binding. No token or arbitrary operation input.
- Export a fixed scenario manifest with stable case/subcase/round IDs implementing all ten design cases. Define manifest hash from canonical serialized manifest; runtime flags cannot change cases.
- `validateExternalIdReport(value: unknown): asserts value is ExternalIdReport`.
- Report types cover bindings, timestamps, operations, state reads, raw artifact references, aliases, identity checks, collection outcome and cleanup outcome. Completion requires all expected manifest operations/evidence and verified cleanup, not merely a nonempty array.

- [ ] Write tests rejecting wrong API/profile digest, unknown keys, unsafe origins, missing identity binding, duplicate operation IDs, omitted required cases, duplicate identity aliases and false completion with pending cleanup. Assert absent body differs from {}, literal spelling is retained, and unavailable differs from observed failure responses.
- [ ] Run `node --import tsx --test test/external-id-contract.test.ts` from compat/clients/node; observe failures before implementation.
- [ ] Implement strict schemas/types with bounded arrays/strings and source binding. Reuse existing profile/toolchain loading; keep identity IDs private. Make identity changes render the report blocked.
- [ ] Run targeted tests and `npm --prefix compat/clients/node run check`; commit.

## Task 2: Bounded probe engine and crash-safe recovery

**Files:** Add src/external-id-probe.ts, src/external-id-recovery.ts and test/external-id-probe.test.ts, test/external-id-recovery.test.ts under compat/clients/node. Reuse wire.ts, private-directory validation and suitable FixtureTracker concepts; do not weaken existing recovery logic.

**Interfaces:**
- `runExternalIdProbe(config: ExternalIdConfig, env: NodeJS.ProcessEnv, runDir: string): Promise<ExternalIdReport>`.
- `recoverExternalIdRun(config: ExternalIdConfig, env: NodeJS.ProcessEnv, runDir: string): Promise<RecoveryResult>`; result distinguishes verified clean, pending, and blocked with stable reason codes.
- Journal is append-only owner-only JSONL with fsync before potentially creating requests. Entries bind run/manifest/origin/identity, exact Name ownership marker, allowed literal key variants, dispatched request and returned/discovered IDs. Reject malformed/truncated journal entries as blocked; persist intent/dispatched/definitive-response/outcome-unknown states; never silently ignore them.
- Native JSforce exact-key case executes in an isolated bounded child process with credentials on stdin, reusing the process-boundary pattern from metadata reads; no CLI source inspection.

- [ ] Build local HTTP transport fixtures for each manifest case. Assert methods/paths/body spellings, one encoding pass, independent state reads, 80/160/two-request bounds and no automatic mutation retry.
- [ ] Add response-loss-after-commit tests with missing ID and process interruption: durable intent survives, recovery discovers the committed owned record, deletes only exact proven IDs and verifies absence. Include no-created-record failures and case variants creating two records. A lost-response intent remains unresolved even after owned records are removed; absence alone cannot establish verified-clean status.
- [ ] Add a delayed-commit-after-timeout transport test: initial marker reads return no rows, the server commits later, and cleanup/run two remain blocked throughout until terminal-outcome evidence is separately established. No timer or override flag may treat absence as cancellation.
- [ ] Add failures for changed initial/final identities, wrong origin, unproven Name/key ownership, malformed query result, pending pagination, interrupted journal writes, cleanup refusal and native-client deadline. Assert no deletion after identity mismatch and no second run while cleanup is incomplete. Use local child/transport boundaries rather than mocking the reconciliation algorithm.
- [ ] Run targeted tests; observe failures. Implement all fixed manifest cases from the spec. Every create/update/upsert request includes the stable case Name marker. Preflight key variants/markers before writing; check all returned rows and pagination, never interpret a partial page as absence.
- [ ] Implement separate collection/cleanup deadlines, read-only reconciliation retries capped at three, exact-ID cleanup and independent ID/marker absence checks. Use static manifest-generated requests only, parameterized/escaped SOQL literals, same-origin routes and no arbitrary URL from report content.
- [ ] Implement recovery that validates private path, journal/config binding and current identity before any network mutation. Preserve original journal/report; append recovery records and emit a separate result. An invalid/incomplete journal blocks automatic deletion with actionable private recovery state.
- [ ] Run targeted tests and full Node check; commit engine and recovery.

## Task 3: Command, deterministic comparison and diagnostic race review

**Files:** Add src/external-id-command.ts, src/external-id-compare.ts, test/external-id-command.test.ts, test/external-id-compare.test.ts; modify src/cli.ts. All paths beneath compat/clients/node.

**Interfaces:**
- `probe-external-id --config <private.json> --out <private.json>` creates an exclusive report and sibling .artifacts run directory. Initial run binding/journal must exist before network mutation; report writes use atomic replacement within that new private run only.
- `recover-external-id --config <private.json> --first <private-run-directory> --out <private-recovery.json>` works even when the final probe report was never written. It reads the durable run binding/journal, not an assumed completed report.
- `compareExternalIdReports(a: ExternalIdReport, b: ExternalIdReport): {outcome: 'stable'|'drift'|'blocked'; differences: string[]}`.
- Exit 0 only for complete collection plus verified cleanup (or verified-clean recovery), 2 for incomplete/blocked, 1 for internal persistence failure. Console output is status/counts/private paths only.

- [ ] Add tests for unsafe/symlink/nonprivate output, output collision, invalid config and absent disposable opt-in; all fail before mutation. Simulate process termination with only initial run binding/journal and verify recovery command can reconcile it.
- [ ] Add comparison tests: equal sequential complete runs with fresh aliases compare stable; key case, spaces, Unicode spelling, status or absent body changes yield drift; binding mismatch, omitted operations, alias collision and incomplete cleanup block. No raw IDs or prose in diagnostic differences.
- [ ] Add race tests preserving responses per request/round while comparing only response multisets and final identity/count invariants for diagnostics. Swapping request winners must not erase a meaningful final-key spelling difference. Any unobserved required outcome or cross-run discrepancy blocks a stable contract claim; stable means these observations matched, not that all schedules are covered.
- [ ] Run targeted tests; observe failures. Implement commands and comparison using Task 1 validation and Task 2 engine/recovery. Do not add these commands to existing capture/compare suite semantics.
- [ ] Run full Node check and git diff --check; commit.

## Task 4: Two reference collections, contract review and next implementation boundary

**Files:** Private artifacts outside checkout; add docs/compat/plan-9-external-id-contract.md, update docs/compat/README.md and implementation-log.md.

- [ ] Recheck existing named-tenant authorization and private administrator binding without touching Plan 5 session files. Verify synthetic field metadata prerequisite still matches the declared scope; a changed unique/externalId/caseSensitive setting blocks collection rather than silently changing the contract.
- [ ] Prepare private config and durable recovery directory. Run the new probe twice sequentially; do not start the second until first-run cleanup/absence verification is complete. No fabricated successes or automatic mutation replays. If interrupted, use the documented recovery command and retain originals.
- [ ] Validate and compare both reports through the exported function. Review raw status/body and native-client projections separately. Bind the sanitized summary to report/manifest/profile/toolchain digests and collection times. Publish only synthetic case labels and behavior, with cleanup evidence and limitations.
- [ ] For every matrix case record exact tested input relationships, outputs, identity relationships, persisted spelling, failures and confidence. Treat five race rounds as measured schedules only. Explain drift; do not rewrite reports or normalize strings to force agreement.
- [ ] Produce a concrete handoff for the next product design: supported equality scope, required lookup/lock/uniqueness consistency, error/response rules, cross-tenant validation and catalog-v2 migration collision checks. Identify unknown Unicode behavior and Plan 5 dependencies explicitly. If the evidence does not determine a safe rule, record precise blockers; do not choose an algorithm just to finish.
- [ ] Run full Node check and git diff --check; confirm product files and pilot/foundation profile bytes unchanged. Commit tooling documentation and reviewed findings or precise live blockers. Do not claim product upsert behavior was fixed by this plan.
- [ ] Request fresh whole-change review. Address Important/Critical findings with failing regression tests, preserve deferred findings and report verification limits. Native sequential execution remains the handoff preference.

## Self-review

Each task consumes explicit types/functions from its predecessor. Required mutation
safety, recovery, aliasing and race cases are named in tests. This is a complete
lab-tooling/contract plan with a deliberate product implementation gate; no task
needs guessed reference outcomes or unbounded load. Existing Plan 5 work is preserved.
