# Plan 11 — External-ID Body-Field Validation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Establish the missing external-ID body-field observations and, only after the evidence gate passes, implement the observed validation contract without mutation on rejection.

**Architecture:** Add a separately bound TypeScript probe, recovery path and assessor for sixteen fixed cases. Reuse Plan 9's transport, identity and private-file primitives, preserving its manifest and report semantics. After two clean, sequential reference runs support the rule, replace the equal-value exception in Go `Store.Upsert` with the revised non-null body-value validation.

**Tech Stack:** Existing Go/PostgreSQL service; Node >=22, TypeScript, node:test, Ajv and pinned compatibility clients. No new dependencies.

**Spec:** [Plan 11 design](../specs/2026-10-19-plan-11-external-id-body-validation-design.md). Tasks 1–4 and 3R are complete; Task 3R review approved the v2 contract and the immutable-pair assessment is ready for all sixteen cases. Task 5 implementation and Go verification are complete. Task 5 spec/quality review passed with no findings; fresh whole-change review remains pending.

## Execution revision — 2026-10-20

The design's evidence-driven revision supersedes the universal presence rule in
this historical plan. Two fresh captures agree: null behaves like omission,
five tested non-null scalar values reject INVALID_FIELD, and object rejects
JSON_PARSER_ERROR. Original assessment `different` is preserved. Task 3R was
implemented and reviewed at `67c0c37`; the versioned assessment is ready for all
sixteen cases, satisfying the prerequisite for Task 5 (`4d57177`).
The user authorized the current test administrator and necessary fixture setup.
See the decision document for immutable evidence and restored permissions.

## Global Constraints

- Work directly on `main`, per the roadmap; no feature worktree or merge step.
- API `65.0`; preserve `PATCH /services/data/v65.0/sobjects/Account/Compat_Key__c/{key}`.
- Exact body field spelling `Compat_Key__c`; no equality, normalization, uniqueness, locking, catalog v2 or successful-response changes.
- JSforce `3.11.0`, Salesforce CLI `2.156.0`, plugin-data `5.1.16`, plugin-deploy-retrieve `4.2.15`, simple-salesforce `1.12.9`; retain existing version sources and lockfiles.
- Collection ten minutes, cleanup five minutes, at most 80 created records and 160 non-cleanup mutations; sequential requests, no mutation retries.
- Private 0600 files and 0700 directories outside the checkout; no credentials or raw reference captures in Git.
- Two fresh sequential captures; verified cleanup and exact-ID absence before the second starts. Missing credentials, unresolved dispatch or failed recovery block collection/product work.
- No product fix until the reviewed `plan11-observed-body-validation-v2` assessment is ready: omitted/null succeed with path key; tested non-null scalars reject `INVALID_FIELD`; object rejects `JSON_PARSER_ERROR`; rejection does not mutate state.
- Preserve historical Plan 9 evidence and the separate Plan 10 blocked decision. Follow `CLEAN_ROOM.md`; independently author explanation text.

## Review Focus

- A 400 response hides an insert or Name mutation: Task 2 captures state and Task 3 rejects the result.
- Null is mistaken for omission or a structured value is coerced to text: Task 1 pins request representation; Tasks 2 and 5 exercise it.
- An unexpected stored key defeats cleanup: Task 2 discovers by both allowed Name markers and keys, retains exact IDs and blocks deletion without proven ownership.
- A stale/corrupt report or unclean first run is accepted as a pair: Tasks 3 and 4 verify bytes, coverage, bindings, timestamps and cleanup before proceeding.
- Shared validation breaks other writes, tenants or URL decoding: Task 5 tests ordinary create/update, tenant isolation and encoded-key controls.

## File responsibilities and sequencing

New Node modules use the prefix `external-id-body-`: `contract.ts` owns the fixed cases/types/schema validation; `recovery.ts` owns suite-specific binding/journal/ownership/cleanup; `probe.ts` collects; `assess.ts` checks evidence; `command.ts` handles private files and locks. Corresponding tests live under `compat/clients/node/test/`. Add `compat/schemas/external-id-body.schema.json` and CLI dispatch in the existing `src/cli.ts`.

Do not parameterize Plan 9's public manifest or make its report validator accept Plan 11. Its recovery functions `initializeRun`, `readRun`, `scopedRecords` and `recoverExternalIdRun` embed Plan 9 cases and are unsuitable for direct reuse here. Reuse only generic primitives where their existing contracts fit. A small separate recovery module is preferable to a broad refactor.

Tasks 1–3 deliver offline lab functionality. Task 4 is the live decision gate. Task 5 is conditional on that gate; Task 6 records verification or the exact blocked outcome. All Node commands below run in `compat/clients/node` unless explicitly marked repository root.

### Historical Task 1–3 contract wording

Tasks 1–3 below retain their original implementation instructions and completion
record. Universal supplied-property rejection and unversioned assessor semantics
were the original hypothesis, not the current acceptance criteria. Revised
Task 3R and Task 5 govern scalar/null/object behavior and versioned acceptance;
Task 4 completed collection first returned `different` under the old hypothesis,
then the separately reviewed v2 assessment returned ready from unchanged inputs.

### Task 1: Define a distinct sixteen-case evidence contract

**Files:** Create `compat/clients/node/src/external-id-body-contract.ts`, `compat/schemas/external-id-body.schema.json`, `compat/clients/node/test/external-id-body-contract.test.ts`.

**Interfaces:** Export `BodyValueKind = "omitted" | "identical" | "variant" | "different" | "null" | "empty" | "number" | "object"`; `BodyCase { id: string; recordState: "existing" | "missing"; valueKind: BodyValueKind }`; readonly `bodyManifest: BodyCase[]` and `bodyManifestHash: string`. Export `bodyScope(runId: string, c: BodyCase): BodyScope` and `bodyRequest(scope: BodyScope, kind: BodyValueKind): Record<string, unknown>`.

`BodyScope` has `pathKey`, `variantKey`, `differentKey`, `seedName`, `requestName` (all strings). Use prefix `p11${runId}-${caseId}-`, keys ending `AbC`, `abc`, `different`; Names end `seed` and `request`. The numeric value is `42`, the object is `{ value: "invalid" }`, empty is `""`; omitted has no own property. All requests have `Name: scope.requestName`.

Export `BodyReport`, `BodyObservation`, `BodyCleanup`, `BodyAssessment` types and `validateBodyReport(value: unknown): asserts value is BodyReport`. Schema version 1, kind `external-id-body-validation`. Reuse `ExternalIdConfig`, `RecordState` and `connectorVersions` types/values from existing modules.

- `BodyReport`: config, connectors, manifestHash, runId, startedAt, finishedAt, identityVerified, collection (`complete|incomplete|blocked`), observations, cleanup, reasons. Times are UTC ISO strings; runId is 24 lowercase hex characters.
- `BodyObservation`: caseId, exact request object, pathKey, before/after `RecordState[]`, stateVerified, status (`number|null`), bodyPresent, body (`unknown`), state (`observed|unavailable`), response artifact basename/SHA-256 and state artifact basename/SHA-256. Include seed response artifact/digest for existing cases. Artifacts retain actual responses; schema accepts unexpected measured outcomes.
- `BodyCleanup`: outcome (`clean|pending|blocked`), startedAt, finishedAt, journal digest, exact IDs with case associations and absence-artifact digests, unresolved operation IDs, reasons. No `clean` result with missing absence verification or unresolved dispatch.
- `BodyAssessment`: outcome (`ready|different|blocked`), reasons, case-level findings and input report digests. `different` means complete evidence contradicts the proposed rule or differs across runs; `blocked` means evidence is insufficient.

- [x] **Step 1:** Write tests `manifest has sixteen unique cases`, `request preserves omission and supplied JSON types`, and `report requires suite binding and exact coverage`. Assert 8 cases per record state, omitted `Object.hasOwn(...) === false`, null `=== null`, number `=== 42`, and deep equality for the object. Reject duplicate/missing cases, unsafe artifact paths and a Plan 9 report. Preserve an unexpected HTTP 200 response as valid measured data.
- [x] **Step 2:** Run `npx tsx --test test/external-id-body-contract.test.ts`; expect failure because the new module does not exist.
- [x] **Step 3:** Implement the interfaces/schema above. Hash the fixed manifest serialization, validate complete coverage only for complete reports, and allow partial blocked reports to retain real observations. Keep Plan 9's manifest hash unchanged.
- [x] **Step 4:** Run the new test plus `npx tsx --test test/external-id-contract.test.ts`; expect pass. Run `npm run typecheck`.
- [x] **Step 5:** Format only touched TypeScript files and commit this contract and its tests: `feat: define external-ID body validation evidence`.

### Task 2: Collect stateful observations with durable recovery

**Files:** Create `src/external-id-body-recovery.ts`, `src/external-id-body-probe.ts`, `test/external-id-body-probe.test.ts`, `test/external-id-body-recovery.test.ts`, `test/external-id-body-test-server.ts` under `compat/clients/node`.

**Interfaces:** Consume Task 1 types and the existing `Transport`, `boundedTransport`, `transport`, `verifyIdentity`, `privateJSON`, `durableJSON`, `query`, `recordPath`, `latchIdentity` primitives from `external-id-recovery.ts`. Export:

- `initializeBodyRun(config: ExternalIdConfig, dir: string, runId: string): Promise<void>`.
- `readBodyRun(config: ExternalIdConfig, dir: string): Promise<BodyRun>` where `BodyRun` contains runId, validated events and operation stages.
- `recoverBodyRun(config: ExternalIdConfig, env: NodeJS.ProcessEnv, dir: string, injected?: Transport): Promise<BodyCleanup>`.
- `runBodyProbe(config: ExternalIdConfig, env: NodeJS.ProcessEnv, dir: string, injected?: Transport): Promise<BodyReport>`.

Journal uses Plan 9's intent/dispatched/response/unknown progression, with suite-bound IDs `${caseId}:seed`, `${caseId}:request`, and `delete:${id}`. Existing cases have a seed and a request; missing cases have only a request: 24 non-cleanup mutations on a successful complete run. Persist suite kind and manifest hash in its own validated binding, not Plan 9's binding format.

- [x] **Step 1:** Build a synthetic transport fixture and tests named `collects all cases without coercion`, `records hidden insert and rejected Name mutation`, `retains different-key fixtures`, `unknown dispatch stays blocked`, `identity mismatch latches`, `cleanup checks every exact ID`, `bounds and ownership fail closed`. Assert 16 observations, 24 mutation dispatches, sequential maximum concurrency 1, valid omitted controls and unchanged rejected state. Inject 400 plus hidden mutation, response loss, malformed state reads, deletion failure and deadline expiry; assert actual evidence survives and unsafe cleanup/collection cannot pass.
- [x] **Step 2:** Run `npx tsx --test test/external-id-body-{probe,recovery}.test.ts`; expect missing-module failures.
- [x] **Step 3:** Implement suite-specific recovery. Validate private paths/ownership, fixed operation IDs and transitions before requests; retain discovered/returned IDs durably. Discover records using both exact allowed Names and all string path/body keys. Read known IDs even after they disappear from discovery queries. Delete only an exact ID with a permitted Name and a permitted key; permitted keys include path/variant/different, empty and null, accommodating unexpected writes. Do not guess coercions for 42 or the object: retain their IDs and block cleanup if ownership cannot be proved. Never delete a foreign row merely matching a key. After deletes, independently read every known ID for absence and repeat scoped discovery; persist these results and journal digest. Unresolved dispatch remains blocked even if queries return no rows. Identity mismatch permanently latches the run.
- [x] **Step 4:** Implement the probe. Recheck administrator and Text field metadata (length 80, externalId=true, unique=true, caseSensitive=false) before writes. Check collision absence per case, seed existing cases, persist pre-state, journal request intent/dispatch, save the actual transport response representation before recording response completion, then independently capture post-state. On failure preserve partial evidence and attempt only safe recovery. Count every mutation, enforce all ceilings, and perform cleanup before setting report finishedAt. Absence of a response is `unavailable`, never an observed API rejection.
- [x] **Step 5:** Run the Task 2 tests and existing `test/external-id-recovery.test.ts`, `test/external-id-probe.test.ts`; expect pass. Typecheck and format touched files.
- [x] **Step 6:** Commit: `feat: collect and recover external-ID body validation cases`.

### Task 3: Assess private evidence and expose bounded CLI commands

**Files:** Create `src/external-id-body-assess.ts`, `src/external-id-body-command.ts`, `test/external-id-body-assess.test.ts`, `test/external-id-body-command.test.ts`; modify `src/cli.ts` (all under `compat/clients/node`).

**Interfaces:** Export `assessBodyPair(firstPath: string, secondPath: string): Promise<BodyAssessment>`. Derive artifact directories as `<report basename>.artifacts` alongside each report. Export `bodyCommand(mode: "probe" | "recover", configPath: string, out: string, env: NodeJS.ProcessEnv, first?: string, injected?: Transport): Promise<BodyReport | BodyCleanup>`. CLI commands:

- `probe-external-id-body --config <private JSON> --out <new report.json>`.
- `recover-external-id-body --config <private JSON> --first <run artifacts directory> --out <new cleanup.json>`.
- `assess-external-id-body --first <report1.json> --second <report2.json> --out <new assessment.json>`.

Unknown flags fail. Exit 0 only for complete clean collection, clean recovery, or ready assessment; exit 2 for measured different/blocked/incomplete; exit 1 for invocation/persistence failures. Console output contains only a summary and private output path.

- [x] **Step 1:** Write assessor tests `ready requires exact expected behavior`, `equal wrong responses are different`, `hidden writes cannot pass`, `missing corrupt or stale evidence blocks`, `pair requires distinct clean sequential runs`. Test missing/null distinction, all 16 cases, duplicate cases, same run twice, overlapping times, first cleanup finishing after second start, wrong profile/client/manifest, missing/tampered raw/state/absence artifacts and 401/403 or malformed positive responses. Expect `ready` only for the valid fixture pair; complete repeatable HTTP 200 on a supplied-property request is `different`, not ready.
- [x] **Step 2:** Write command tests `private outputs are exclusive`, `pending run blocks both suites`, `second run rechecks first absence`, `failed publication retains recovery lock`, `CLI rejects unknown flags`. Unsafe permissions, symlinks, missing disposable opt-in and existing output must fail before fixture writes.
- [x] **Step 3:** Run `npx tsx --test test/external-id-body-{assess,command}.test.ts`; expect missing-module failures.
- [x] **Step 4:** Implement assessment from private disk bytes. Check schema, every digest and artifact/content correspondence, current config/profile/clients/manifest, exact request matrix and state read shapes. Require valid timestamps, different run IDs and first cleanup completion before second start. Normalize only recorded run prefixes and exact record-ID aliases for comparison; retain response arrays/order, error fields and state values. Existing controls update the seeded ID; missing controls create exactly one record, with existing success bodies/statuses 200/201 checked. Reject hidden changes or missing evidence. Do not copy reference messages into product code.
- [x] **Step 5:** Implement commands using private create-exclusive output and the same identity-derived `external-id-<digest>.active.json` lock namespace as Plan 9, so unresolved work in either suite blocks new probes. Preserve Plan 9 lock shape `{dir}`. In addition retain a Plan 11 last-clean-run pointer under the recovery root: before another Plan 11 run, validate its binding/journal/absence artifacts and recheck all its exact IDs with the current identity. Refuse invalid pointers or reappearing records. Persist pointer before releasing the active lock. Recovery commands only accept their own suite binding. Publication failure must retain recoverable state and cannot authorize a second capture. Add CLI dispatch without changing existing command behavior.
- [x] **Step 6:** Run `npm run check`; expect typecheck, formatting, Node tests and repository validation to pass. Existing optional candidate skips remain explicit; no live evidence is claimed.
- [x] **Step 7:** Commit: `feat: assess and run external-ID body validation evidence`.

### Task 4: Collect the reference pair and record the product decision

**Files:** Create `docs/compat/plan-11-external-id-body-validation.md`. Private reports, artifacts and configuration stay outside the checkout.

**Progress:** Complete after authorized resume: current administrator binding and temporary Compat_Admin setup verified, two clean sequential captures collected, temporary assignment restored. Original blocked attempt and original `different` assessment are preserved. The reviewed v2 reassessment is ready for all sixteen cases; see [decision](../../compat/plan-11-external-id-body-validation.md).

**Consumes:** Task 3 CLI and existing authorized private reference configuration. **Produces:** Sanitized case-level decision with `ready`, `different` or `blocked` outcome and exact provenance. This is the prerequisite for Task 5.

- [x] **Step 1:** Verify available private configuration without printing secrets. Confirm the named tenant authorization, administrator identity, API/client pins, metadata prerequisites and unresolved recovery state. Required environment includes `COMPAT_REFERENCE_URL`, `COMPAT_REFERENCE_TOKEN`, `COMPAT_DISPOSABLE=yes` and `COMPAT_RECOVERY_DIR`. Use private config with origin, administrator user/org, API and profile hash as required by `parseExternalIdConfig`. If unavailable, record which prerequisite is missing and go to Task 6's blocked path; do not fabricate a configuration or run partial collection.
- [x] **Step 2:** With `P11_CONFIG` and `P11_PRIVATE` set to existing private paths, run `npx tsx src/cli.ts probe-external-id-body --config "$P11_CONFIG" --out "$P11_PRIVATE/run1.json"`. Expected: complete collection, verified clean recovery. Inspect private findings and absence evidence. If incomplete, recover using `recover-external-id-body --first "$P11_PRIVATE/run1.artifacts"` with a new output; unresolved outcomes stop collection. Never overwrite or retry a possibly committed mutation.
- [x] **Step 3:** Only after first-run cleanup/absence succeeds, run the same probe to `$P11_PRIVATE/run2.json`; the command must recheck first-run absence before writes. Then run `npx tsx src/cli.ts assess-external-id-body --first "$P11_PRIVATE/run1.json" --second "$P11_PRIVATE/run2.json" --out "$P11_PRIVATE/assessment.json"`.
- [x] **Step 4:** Publish independent summaries of all cases, report/manifest/profile/client digests, capture and cleanup times, gate outcome and limitations. If outcome is `different`, document actual results and revise the design before product implementation. If `blocked`, identify missing evidence and the next action. Only `ready` enables Task 5; no broad Plan 9/10 promotion is allowed.
- [x] **Step 5:** Run repository-root `git diff --check`, inspect the document for credentials/identifiers/raw prose, and commit only the sanitized decision: `docs: record Plan 11 body validation evidence decision`.

The run/output filenames in Steps 2–3 are illustrative original instructions:
actual resumed outputs are `capture-a.json`, `capture-b.json` and the retained
original `assessment.json`. Universal supplied-value rejection was the historical
hypothesis; the actual revised acceptance criteria are omission/null success,
five scalar INVALID_FIELD rejections and object JSON_PARSER_ERROR rejection.
Task 3R reviewed and reassessed those same inputs into `assessment-v2.json`;
no additional collection or unresolved evidence prerequisite remains.

### Task 3R: Version the observed validation contract

**Files:** Modify `compat/clients/node/src/external-id-body-contract.ts`,
`src/external-id-body-assess.ts` and `test/external-id-body-assess.test.ts` (the
last two paths relative to `compat/clients/node`). Modify other suite tests only
where their synthetic fixtures must represent this revised contract.

- [x] Write regressions proving observed null success/object parser errors pass,
  original universal rejections do not pass v2, malformed/error-field envelopes
  and hidden writes remain rejected, and output includes exact contractVersion.
- [x] Observe failures, then add required `contractVersion:
  "plan11-observed-body-validation-v2"` to new BodyAssessment outputs. Keep
  BodyReport schema/manifest and all historical capture bytes unchanged.
- [x] Treat omitted/null as success cases with existing state and 200/201 checks;
  object requires JSON_PARSER_ERROR and all other supplied cases INVALID_FIELD.
  Require a single rejection object, string message/errorCode, absent fields,
  and unchanged state. Keep all integrity/cleanup/pair checks intact.
- [x] Run focused tests, typecheck and `npm run check`; commit.
- [x] Fresh review this amendment and assessor. Reassess the existing immutable
  complete pair to a new private exclusive output, never overwrite v1. Require
  ready and contractVersion v2 before Task 5; record digests and outcome.

### Task 5: Implement the observed body-field behavior

**Prerequisite:** Reviewed revision Task 3R and ready versioned assessment for
all sixteen cases. **Files:** `internal/platform/store.go`, `store_test.go`,
`http_test.go`. Preserve `Store.Upsert` signature and public route.

- [x] Add `TestUpsertRejectsBodyKeyWithoutDatabase` using `&Store{}`: identical,
  variant, different, empty strings and number42 reject400 INVALID_FIELD;
  map/array reject400 JSON_PARSER_ERROR. Assert no panic, empty ID, created=false,
  unchanged input. Null is excluded from this rejection-only test.
- [x] Add database-backed direct-store and HTTP tests for existing/missing keys
  in two tenants. Omitted/null create201 then update200 with same ID/path key and
  requested Name; five scalar and object rejections leave all stored rows
  unchanged. Array is local robustness coverage only. All inputs remain unchanged.
  Assert one error element, absent fields, independently written message.
  Ordinary create/record-ID update, malformed JSON, unauthorized calls and exact
  encoded-key values `key/`, `a/b`, `key%2F` retain their current behavior.
- [x] Run focused tests against disposable PostgreSQL and observe intended
  equal-string acceptance/null rejection failures before product edits.
- [x] In `Store.Upsert`, inspect the exact body key before copying/injecting.
  If non-null, use a type switch: map[string]any or []any returns
  `invalid("JSON_PARSER_ERROR", "External ID requires a scalar value or null")`;
  all other non-null values return
  `invalid("INVALID_FIELD", "External ID value must be supplied in the upsert URL")`.
  Omitted/null use the existing copy and path-key injection. Remove old equality
  exception. Preserve remaining validation/transaction/tenant logic.
- [x] Run focused tests and `go test ./... -count=1` with TEST_DATABASE_URL;
  satisfy existing CI Go gates `go test -race ./...` and `go vet ./...` on the
  disposable database. No database skips. Run gofmt/diff checks.
- [x] Commit tested product changes and report exact evidence/limitations.

### Task 6: Review and publish the final bounded outcome

**Progress:** Final unchanged-JavaScript checks at `67c0c37` and Go verification at `4d57177` passed. Current documentation is prepared. Task 5 spec/quality review passed with no findings; fresh whole-change review of the resumed evidence-driven implementation remains pending; the earlier blocked-outcome review is historical. Private raw evidence is outside reviewer access; the controller verified immutable-pair and assessment digests.

**Files:** Update `docs/compat/plan-11-external-id-body-validation.md` and this plan's task checkboxes to reflect actual progress.

- [x] **Step 1:** Run `npm run check` in the Node lab and repository-root `git diff --check`. If Task 5 ran, retain its full Go verification result; rerun Go only if subsequent fixes changed Go code. Report database skips as gaps. Do not run unrelated UI/client suites merely to expand the checklist.
- [ ] **Step 2:** Request one fresh whole-change review against this plan and its spec. Include the baseline commit, changed files, actual verification, gate outcome and private-evidence access limits. Resolve Important/Critical findings with relevant regressions and checks. Do not give the reviewer credentials or claim it independently verified inaccessible raw evidence.
- [ ] **Step 3:** Publish final status: either evidenced and implemented with review/verification, or offline lab complete but reference/product work pending with exact blockers. Update case findings and links without changing historical evidence, accepted profiles or Plan 10 status. Mark only completed checkboxes.
- [ ] **Step 4:** Run `git diff --check`, commit documentation as `docs: finalize Plan 11 validation status`, and verify `git status --short`. Report remaining unrelated changes accurately; no push is required by this plan.

## Self-review and handoff

Spec coverage: matrix/representation → Task 1; bounded collection, pre/post state, ownership and recovery → Task 2; binding/integrity/pair gate and CLI → Task 3; live evidence/blocker decision → Task 4; pre-transaction property rejection and regressions → Task 5; review and honest completion → Task 6. Offline Review Focus items have tests in their owning tasks; Task 5 product regressions and implementation are complete; current final review remains pending. Product routes, schema/catalog, equality and foundation evidence remain outside the implementation boundary.

Recommended execution: native sequential implementation. These tasks share one evidence contract and a live prerequisite; a single implementer with one final independent review avoids repeated recovery-context setup. Review this plan before implementation; the evidence gate remains mandatory regardless of execution method.

Actual execution: the user selected Sol medium subagent execution, overriding the native sequential recommendation above. Tasks 1–3 received task-level review; the first whole-change review approved the historical blocked outcome. After authorized resume, Task 3R received clean spec/quality review, the v2 gate passed, and Task 5 implementation and database-backed verification completed. Task 5 spec/quality review passed with no findings; fresh whole-change review remains pending.
