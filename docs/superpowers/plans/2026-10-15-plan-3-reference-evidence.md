# Plan 3 Reference Evidence Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for native execution or superpowers:subagent-driven-development if the user selects delegation. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Make reference capture fail closed until authorized, and accept reviewed, reproducible observations without confusing them with candidate parity.

**Architecture:** Extend the existing TypeScript laboratory with focused readiness and evidence modules. Share the reference preflight between CLI and capture, and validate reviewed evidence through the existing repository validator. Keep live observation a separate authorized phase.

**Tech Stack:** Existing Node.js 22+, TypeScript, node:test, Ajv, YAML, pinned JSforce/Salesforce CLI/Python clients. No new dependencies.

**Spec:** [Approved reference-evidence design](../specs/2026-10-15-plan-3-reference-evidence-design.md).

## Global Constraints

- All changes stay on main under the roadmap's branch policy.
- Actual org identifiers, endpoints and credentials remain in private runtime configuration.
- The maintainer must supply the terms-review decision and explicitly authorize observation.
- Existing deferred contracts remain not_run until their own scenarios and observations exist.
- Observed does not mean the candidate implements or passes that behavior.
- No Go service or Vue product behavior changes belong to this work package.
- This implementation session does not become the observer for the same area.
- Offline tests and readiness never satisfy live acceptance. No live calls during implementation.

## Review Focus

1. Malformed YAML and unknown fields must block preflight without exposing source contents (Tasks 1–2).
2. A profile edit, changed API version, or reduced scenario scope must invalidate readiness (Task 1).
3. Direct capture calls must enforce the gate even when CLI preflight is bypassed (Task 2).
4. Copied captures and partial or failed cleanup must not satisfy repeatability acceptance (Task 3).
5. Evidence registration must not mutate the profile and invalidate its own binding (Task 4).

## File and interface map

| File | Responsibility |
| --- | --- |
| `compat/clients/node/src/readiness.ts` | Typed readiness context, schema validation, reference configuration gate |
| `compat/clients/node/src/evidence.ts` | Observation validation and private capture-pair verification |
| `compat/schemas/reference-readiness.schema.json` | Closed draft-07 readiness schema |
| `compat/schemas/observation.schema.json` | Closed draft-07 observation schema |
| `docs/compat/reference-readiness.yaml` | Pending, non-authorizing maintainer record |
| `compat/clients/node/src/capture.ts` | Enforce preflight before any reference transport |
| `compat/clients/node/src/cli.ts` | `preflight` and `verify-evidence` commands |
| `compat/clients/node/src/validate.ts` | Validate readiness structure and contract/evidence links |
| `compat/clients/node/test/readiness.test.ts` | Readiness and no-client tests |
| `compat/clients/node/test/evidence.test.ts` | Pair validation, observation and linkage tests |
| `.github/workflows/compat.yml` | Shared reference preflight in existing protected live job |
| `docs/compat/README.md`, `observations/README.md`, `implementation-log.md` | Execution, review and status documentation |

Keep readiness/evidence independent of validate.ts to avoid cycles: callers pass profile and registry data explicitly. Existing `profile()` hashes raw profile bytes; retain that behavior. Keep `referenceStatus: pending` unchanged during evidence acceptance and document that reviewed contract records are authoritative for this phase. A later profile revision requires new captures; do not silently rebind old evidence.

## Task 1: Typed readiness record and offline checks

**Files:** Create readiness.ts, reference-readiness.schema.json, reference-readiness.yaml and readiness.test.ts.

**Interfaces:** Export these types and functions from readiness.ts:

```ts
export interface EvidenceContext {
  profile: string;
  profileHash: string;
  apiVersion: string | null;
  requiredScenarios: string[];
  connectors: Record<string, string>;
  cleanupScenarios: string[];
}
export interface Readiness {
  schemaVersion: 1;
  referenceAlias: string | null;
  profile: string;
  profileHash: string | null;
  apiVersion: string | null;
  decision: {
    status: 'pending' | 'approved' | 'rejected';
    maintainer: string | null;
    recordedAt: string | null;
    logAnchor: string | null;
    observationAuthorized: boolean;
  };
  observer: string | null;
  implementer: string | null;
  scenarios: string[];
  fixtures: { disposable: boolean; administrator: boolean;
    restricted: boolean; locale: boolean; deployed: boolean };
}
export function readinessIssues(value: unknown, schema: unknown,
  context: EvidenceContext, env: NodeJS.ProcessEnv): string[];
export function readinessStructureIssues(value: unknown, schema: unknown): string[];
export async function referencePreflight(root: string, context: EvidenceContext,
  env: NodeJS.ProcessEnv): Promise<string[]>;
```

- [ ] Write tests with a fully approved synthetic Readiness record and context. Use a 64-character SHA-256 string, separate observer/implementer aliases, exact eleven-scenario manifest and synthetic environment credentials. Keep fixtures test-local, never in the real readiness record. Include the following regression assertions:

```ts
assert.equal(readinessIssues(record, schema, context, env).length, 0);
assert.ok(readinessIssues({...record, observer: record.implementer}, schema, context, env).length);
assert.ok(readinessIssues(record, schema, {...context, profileHash: 'b'.repeat(64)}, env).length);
assert.ok(readinessIssues({...record, scenarios: []}, schema, context, env).length);
assert.ok(readinessStructureIssues({...record, token: 'never-print-me'}, schema).length);
```

- [ ] Run `cd compat/clients/node && node --import tsx --test test/readiness.test.ts`; expect failure because readiness.ts does not exist.
- [ ] Implement closed draft-07 schemas with required keys, additionalProperties false at every object, unique scenario items, SHA-256 patterns, explicit API version patterns and nonblank identity aliases. Nullable pending fields remain structurally valid. Reject invalid calendar dates and future decision dates in semantic checks; compare normalized, trimmed role aliases for separation.
- [ ] Implement schema validation with existing Ajv and return fixed error messages or schema paths, never input values. `referencePreflight` reads only the fixed repository record/schema and the decision log; require logAnchor to name an existing Markdown anchor in implementation-log.md. Reject missing files and malformed YAML as safe blocked diagnostics. Require approved decision, explicit authorization, all fixtures, exact profile/API/scenario binding, and nonempty reference restricted token and locale user ID in addition to existing `checkConfig('reference', env)`. Never fetch URLs, instantiate clients or write files.
- [ ] Add the real record with status pending, observationAuthorized false, null identities/hash/API/decision references and all fixture flags false. Keep profile `pilot` and the existing eleven scenario IDs. It must pass structural checks but fail live preflight. No fictitious decision entry.
- [ ] Add tests for pending/rejected decisions, missing personas, API mismatch, malformed YAML, missing log anchor and whitespace-equivalent identities. Run the targeted tests to green, then `npm --prefix compat/clients/node run check`.
- [ ] Commit only this task's files on main: `feat(compat): add reference readiness validation`.

## Task 2: Gate CLI and direct capture before transport

**Files:** Modify capture.ts, cli.ts, package.json, readiness.test.ts; add test/preflight.test.ts.

**Interfaces:** Consume referencePreflight and EvidenceContext. Preserve public `capture(target, env): Promise<Report>` and existing candidate behavior. Add npm script `preflight: tsx src/cli.ts preflight`.

- [ ] Add a direct reference capture test using valid-looking configuration while the repository readiness record remains pending. Instrument Connection.prototype.request and fetch with test mocks that count and throw if called; run tests serially for this case and restore mocks in finally. Assert zero calls, provenance none, all eleven results blocked and no cleanup. Add a CLI subprocess test requiring exit 2 for `preflight --target reference` and no credential text in stdout/stderr.

```ts
const report = await capture('reference', {
  COMPAT_REFERENCE_URL: 'https://reference.invalid',
  COMPAT_REFERENCE_TOKEN: 'synthetic-secret',
  COMPAT_API_VERSION: '65.0', COMPAT_DISPOSABLE: 'yes',
});
assert.equal(report.provenance, 'none');
assert.ok(report.results.every(result => result.outcome === 'blocked'));
assert.equal(report.cleanup.length, 0);
assert.ok(!JSON.stringify(report).includes('synthetic-secret'));
```

- [ ] Run `cd compat/clients/node && node --import tsx --test test/preflight.test.ts`; expect the new CLI command/no-transport assertions to fail. Tests must use rejecting mocks, never actual outbound calls.
- [ ] Insert the reference gate before `new Connection`, private journal directory creation and scenario dispatch. Build EvidenceContext from profile data/hash, SCENARIOS and CLEANUP_SCENARIOS. Append returned issues to existing blocked-report handling; do not throw away the manifest.

```ts
if (target === 'reference') {
  issues.push(...await referencePreflight(ROOT, {
    profile: p.data.id, profileHash: p.hash, apiVersion: p.data.apiVersion,
    requiredScenarios: SCENARIOS, connectors: p.data.connectors,
    cleanupScenarios: CLEANUP_SCENARIOS,
  }, env));
}
```

- [ ] Add `preflight --target reference|candidate`. Reference uses referencePreflight, candidate uses checkConfig and existing recovery-directory validation only. Both return 2 for blocked configuration and 0 for readiness, explicitly print “offline readiness; no observation performed,” and sanitize output. Update both CLI usage strings. Do not describe preflight as testing network access or persona permissions.
- [ ] Add regressions for malformed readiness returning blocked rather than uncaught exceptions, missing reference record, and candidate configuration remaining independent of reference decisions. Existing candidate tests must continue to pass.
- [ ] Run targeted tests and full `npm --prefix compat/clients/node run check`; commit `feat(compat): enforce reference authorization before capture`.

## Task 3: Private capture-pair verification and observation schema

**Files:** Create evidence.ts, observation.schema.json, evidence.test.ts; modify cli.ts and package.json.

**Interfaces:** Consume EvidenceContext. Export from evidence.ts:

```ts
export interface Observation {
  schemaVersion: 1;
  id: string;
  profile: string;
  profileHash: string;
  apiVersion: string;
  connectors: Record<string, string>;
  contracts: string[];
  captures: { sha256: string; startedAt: string; finishedAt: string }[];
  observer: string;
  review: { reviewer: string; reviewedAt: string; sanitized: boolean };
  repeatability: 'passed';
  cleanup: 'passed';
  notes: { contract: string; summary: string }[];
}
export function verifyCapturePair(first: string, second: string,
  context: EvidenceContext): { outcome: import("./core.ts").Outcome; issues: string[];
    captures: Observation['captures'] };
export function observationIssues(value: unknown, schema: unknown,
  context: EvidenceContext, readiness: unknown): string[];
```

- [ ] Write synthetic report fixtures locally in evidence.test.ts using the Report shape from core.ts and full manifest/cleanup expectations. Two reports must have different nonoverlapping times, actual equivalent fixture observations and distinct raw-byte digests. Assert duplicate input, overlapping captures, synthetic provenance, failed or missing cleanup, missing scenarios, unequal API versions and mismatched connector pins all yield issues.

```ts
assert.ok(verifyCapturePair(firstJson, firstJson, context).issues.length);
assert.equal(verifyCapturePair(firstJson, secondJson, context).issues.length, 0);
assert.ok(observationIssues({...note, review: {...note.review, sanitized: false}},
  schema, context, readiness).length);
```

- [ ] Run `cd compat/clients/node && node --import tsx --test test/evidence.test.ts`; expect missing-module failure.
- [ ] Implement verifyCapturePair: parse raw JSON inside safe error handling; use validateReport and `compare(first, second, expected, 'repeatability')`; require two reference reports, explicit context API match, valid ordered timestamps, first finished no later than second started, different start times and digests. Return SHA-256 of original bytes and timestamps only on success; otherwise return an empty captures array. Structural/prerequisite failures have outcome blocked; comparison differences retain failed. Never return raw captures, IDs or messages. Require cleanup coverage via context.cleanupScenarios.
- [ ] Implement the closed observation schema: exactly two captures, nonempty unique known contract IDs, complete connector map, bounded nonempty note summaries (1–8000 characters), valid UTC date-times and SHA-256 patterns. Reject blank identities, unknown note contracts, missing notes for claimed contracts, duplicate digests and unreviewed/unsanitized notes. Require readiness observer match, distinct implementer, profile/API/client equality and review time after both captures. Reject obvious credentials and Salesforce ID-shaped tokens in notes, but document that human sanitization review remains mandatory.
- [ ] Add `verify-evidence --first path --second path --out path.json`. Read private reports, call verifyCapturePair and write only digests/timestamps/outcome using private file permissions. Exit 2 for invalid or blocked evidence, 1 for comparison differences, 0 for passing pair; preserve the compare outcome in the helper result’s `outcome` field. Never synthesize an approved Observation or change a contract. Add npm script and usage strings.
- [ ] Add subprocess tests for malformed JSON and secret-bearing errors remaining sanitized; tests for unknown fields, invalid timestamps, stale hashes and unknown contracts. Run targeted/full checks; commit `feat(compat): validate reference capture pairs and reviewed notes`.

## Task 4: Link observed contracts to reviewed evidence

**Files:** Modify validate.ts, evidence.ts, evidence.test.ts, docs/compat/observations/README.md.

**Interfaces:** Add `observationLinkIssues(contracts: {id: string; observationStatus: string; observationIds?: string[]}[], notes: unknown[], schema: unknown, context: EvidenceContext, readiness: unknown): string[]` to evidence.ts. It calls observationIssues and enforces cross-record links.

- [ ] Test a not_run contract without evidence passes; observed without notes fails; an observed contract with reviewed matching note passes. Test duplicate IDs, dangling links, partial notes, unreviewed records, stale bindings and note claims with no matching contract link.

```ts
assert.deepEqual(observationLinkIssues([
  {id: 'crud', observationStatus: 'not_run'},
], [], schema, context, readiness), []);
assert.ok(observationLinkIssues([
  {id: 'crud', observationStatus: 'observed', observationIds: []},
], [], schema, context, readiness).length);
```

- [ ] Run targeted evidence tests and observe failure for missing linkage function.
- [ ] Replace the pilot-only `c.observationStatus !== 'not_run'` check with the two supported states and linkage validation. Read observation `.yaml` files only; README is not evidence. Require stable filename `<id>.yaml`, unique IDs and observationIds. `not_run` must have no observationIds; observed must have at least one. Reject any note claiming an unknown or not_run contract. Deferred-contract checks remain unchanged.
- [ ] Structurally validate the pending readiness record in ordinary repository validation without requiring authorization. When observed records exist, additionally require the approved decision, profile binding and roles, but not live credentials. Export `readinessDecisionIssues(value: unknown, schema: unknown, context: EvidenceContext): string[]` from readiness.ts for this shared check; readinessIssues calls it before runtime checks. Keep runtime environment validation in referencePreflight. Refactor common decision checks inside readiness.ts rather than fabricating credentials for offline validation.
- [ ] Add a byte-preservation test around repository validation proving pilot.yaml remains unchanged. Do not alter profile.referenceStatus, contracts, readiness or observation files automatically. Add a derived summary listing counts observed and not_run; update CLI's unconditional “Live Salesforce verification: pending” message so it reports reviewed observation coverage without implying parity or full milestone completion.
- [ ] Document observation structure, private pair-verification procedure and manual review/linking. Store no synthetic observation YAML in the real observations directory. Run full checks; commit `feat(compat): bind observed contracts to reviewed evidence`.

## Task 5: CI, operator handoff and final offline verification

**Files:** Modify .github/workflows/compat.yml, docs/compat/README.md, docs/compat/implementation-log.md, docs/compat/observations/README.md; extend test/preflight.test.ts.

**Interfaces:** Use the same preflight CLI and capture gate. No new secrets or public artifact types.

- [ ] Add a CLI regression asserting a real pending record yields exit 2 with a clear reason and no credentials. Assert `validate` still succeeds for the pending record and no observations.
- [ ] Run targeted tests; implement any missing CLI behavior before documentation.
- [ ] In the protected live job's existing shell step, run the shared preflight before capture with the same environment. Keep concurrency, protected environment and persistent recovery checks. For reference runs, remove automatic capture-report upload: store reports in the private persistent recovery directory and publish only separately reviewed notes manually. Keep candidate artifact behavior scoped to candidate runs. This enforces the design's private-reference-report boundary.

```sh
npm --prefix compat/clients/node run preflight -- --target "$CAPTURE_TARGET"
```

- [ ] Document offline setup and `npm --prefix compat/clients/node run preflight -- --target reference`; a blocked result is expected until maintainer inputs exist. Add maintainer instructions to pin profile values first, compute its raw-byte SHA-256, record the actual decision/log anchor and separate roles, then complete fixture attestations. Emphasize that configuration attestations do not verify org permissions.
- [ ] Document the separate observer sequence: prepare authorized fixtures/personas; verify no outstanding recovery; capture twice sequentially; run repeatability and verify-evidence; independently write notes; obtain sanitization review; link contracts; run repository validation. Include exact existing capture/repeatability commands and new verify-evidence command. Never run these against Salesforce during implementation.
- [ ] Record actual tooling results in implementation-log.md, with live evidence pending and no fabricated risk decision. Retain M1R, expanded CLI runners/proxy, M0 inventory and future M3 work as separate unresolved gates.
- [ ] Run `npm --prefix compat/clients/node run check` and `git diff --check`. Run the real pending preflight and confirm exit 2 without transport. Inspect CI diff for reference raw-report exposure. No Go/UI tests are required because those components are unchanged.
- [ ] Commit `docs(compat): wire reference preflight and observer handoff`. Report offline readiness separately from live evidence. If maintainer inputs are absent, stop live work with a concrete list of missing inputs; never convert pending to approved on the user's behalf.

## Execution order and acceptance

Execute Tasks 1–5 sequentially: each consumes the preceding interfaces. Native execution is recommended because these are tightly coupled extensions to one harness. Review the full change after the full offline suite passes. Commit documentation/design alongside the plan on main before implementation, subject to repository write permissions.

Tooling acceptance: offline checks pass; pending/invalid authorization blocks before transport; candidate behavior is preserved; evidence links are validated; private reference reports stay private. Live acceptance is a later observer action requiring real captures and review, not a test fixture or an automatic continuation by the implementer.

## Self-review record

Spec coverage maps readiness to Tasks 1–2, evidence lifecycle to Tasks 3–4, errors/testing to all five tasks, and operational boundaries to Task 5. All five Review Focus conditions have assigned regression tests. No product changes, live authorization or M1R completion are inferred. Decision status remains pending until the maintainer supplies it.
