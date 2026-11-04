# Plan 22 — Package Dependency Feasibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Demonstrate exact synthetic package dependency/symbol resolution and report owned execution blockers without claiming a package can install or run.

**Architecture:** A pure bytes-to-report pipeline validates a bounded synthetic YAML manifest, resolves an explicit package graph and qualified references, then assesses all packages in the root closure. Existing Node tooling runs independent fixtures twice. No product, CLI, network or interpreter integration is added.

**Tech Stack:** Existing compat Node >=22, TypeScript 7.0.2, tsx 4.23.15, Ajv 8.20.0, yaml 2.9.1, Node test/assert/crypto built-ins.

**Spec:** [Plan 22 design](../specs/2026-11-03-plan-22-package-dependency-feasibility-design.md). Implemented and independently reviewed, 2026-11-04; [results and execution gaps](../../compat/plan-22-package-dependency-feasibility.md). Real-package execution remains unproven.

## Global Constraints

- Work directly on main under the repository convention.
- Use the existing compat Node >=22 toolchain and pinned dependencies; no new dependency or manifest/lockfile changes.
- Use independently authored synthetic manifests only; no live org, credentials, network calls, package installation, subprocess, customer source or vendor communication in the experiment.
- Preserve CLEAN_ROOM.md and all existing product code, accepted profiles, observations, readiness/evidence, historical reports and Plans 10/14/17/19/20/21 byte-for-byte.
- Source locators are inert text; never open, fetch, import or execute them.
- No package archive parsing, source-code parsing/execution, version-range solver, CLI command, deployment engine, license bypass or runtime bridge.
- No claim of Salesforce installation, dependency-discovery, visibility, security, upgrade or execution compatibility.

## Review Focus

- A manifest claiming target verification must remain synthetic and execution-unproven (Task 3).
- A transitive or same-namespace provider must not satisfy an undeclared direct import (Task 2).
- A malformed unreachable package must invalidate the complete input before publishing any closure (Task 2).
- Shared namespaces must allow distinct symbols while rejecting exact tuple collisions, including null namespace (Task 2).
- Large or malformed YAML and prototype-like identifiers must return bounded deterministic outcomes without state leakage (Tasks 1–3).

## File Structure and interfaces

Create only these implementation paths, besides this plan/spec's truthful status updates:

| Path | Responsibility |
| --- | --- |
| `compat/schemas/package-feasibility.schema.json` | Strict input contract, no unknown keys/coercion/defaults. |
| `compat/clients/node/src/package-feasibility-model.ts` | Input, validated model, issue, binding, blocker and report types. |
| `compat/clients/node/src/package-feasibility-parse.ts` | Bounded bytes/hash/UTF-8/YAML/schema boundary. |
| `compat/clients/node/src/package-feasibility-resolve.ts` | Whole-model validation, symbols, root closure and bindings. |
| `compat/clients/node/src/package-feasibility-assess.ts` | Public assessment composition and blocker policy. |
| `compat/clients/node/test/package-feasibility-{parse,resolve,assess,scenarios}.test.ts` | Meaningful boundary, graph, policy and fixture regressions. |
| `compat/clients/node/test/package-feasibility-fixtures.ts` | Small independently authored manifest/byte helpers shared by tests. |
| `compat/fixtures/package-feasibility/{linked,opaque,optimistic,transitive-import,namespace-collision}.yaml` | Fixed synthetic scenario inputs, with expected results asserted in tests. |
| `docs/compat/plan-22-package-dependency-feasibility.md` | Actual source/experiment record, decision and owned follow-ups. |

Define these shared types in model.ts (complete property types/enum unions come directly from the spec): `PackageManifest`, `PackageDeclaration`, `ComponentKind`, `PackageIssue`, `PackageBinding`, `PackageBlocker`, `PackageClosureEntry`, `PackageAssessment`, `ParsedManifest`, `PackageResolution`. No generic extensible details map or supported-capabilities registry.

- `ParsedManifest = { ok:true; inputDigest:string; manifest:PackageManifest } | { ok:false; inputDigest:string|null; issues:PackageIssue[] }`.
- `PackageClosureEntry = { packageId:string; version:string }`.
- `PackageBinding = { consumer:{ packageId:string; kind:ComponentKind; name:string }; provider:{ packageId:string; kind:ComponentKind; name:string; namespace:string|null } }`.
- `PackageResolution = { validation:'valid'|'invalid'; closure:PackageClosureEntry[]; bindings:PackageBinding[]; issues:PackageIssue[] }`. Invalid resolutions have empty closure/bindings.
- `PackageAssessment` has the exact spec report keys, including literal `classification:'synthetic'` and `execution:'unproven'`; it includes no manifest/source copy or runtime readiness boolean.
- `MAX_PACKAGE_MANIFEST_BYTES = 1_048_576` in parse.ts. Schema array/string/depth bounds are the spec's exact values, not adjustable caller limits.

Four sequential tasks. All Node commands below run from `compat/clients/node/`; git commands run from repository root. Changes remain limited to named new paths; fix an earlier new module only when a regression requires it.

### Task 1: Implement the bounded synthetic manifest boundary

**Files:** Create model.ts, parse.ts, schema, fixture helper and parse.test.ts at the paths above. Read existing `src/target-inventory.ts` and `test/target-inventory.test.ts`; do not modify them.

**Interfaces:**
- Consumes: `parseInventoryDocument(bytes:Uint8Array):unknown` from target-inventory.ts and Ajv's existing API.
- Produces: shared model types and `parsePackageManifest(bytes:Uint8Array):ParsedManifest` from parse.ts.

- [x] **Step 1: Record the boundary and protect existing files.** Read CLEAN_ROOM.md, the spec and pinned package.json/tsconfig/test-runner configuration. Save a temporary SHA-256 manifest outside the checkout for all tracked files except this plan/spec; do not publish machine-local paths. Capture baseline git status and avoid unrelated changes. Add source URLs/read boundaries to a draft execution report; leave measurements and decision pending.
- [x] **Step 2: Write failing parser tests** named `parsesSyntheticManifest`, `rejectsMalformedManifest` and `boundsManifestInput`. Independently author a minimal byte helper, with assertions including:

```ts
assert.equal(parsePackageManifest(bytes).ok, true);
assert.deepEqual(parsePackageManifest(realClassification).issues, [{code:'schema-invalid'}]); // narrow failure union first
assert.equal(parsePackageManifest(overLimit).inputDigest, null);
assert.equal(parsePackageManifest(invalidUtf8).issues[0].code, 'encoding-invalid');
```

Use proper union narrowing in actual tests. Exercise exactly/over 1,048,576 bytes using YAML whitespace padding; duplicate keys, trailing YAML documents, root scalar, unknown nested key, enum and version/namespace/name bounds, all array maxima, depth/node/alias limits inherited from the existing parser. Below-limit rejections must retain the original SHA-256. Inject marker strings and assert no raw exception/marker enters diagnostics. Invalid inputs never return a partial manifest.
- [x] **Step 3: Run RED.** `node --import tsx --test test/package-feasibility-parse.test.ts`; expect failure because the module/functions do not exist.
- [x] **Step 4: Implement model/schema/parser.** Encode every required property/cardinality from the spec with Ajv additionalProperties=false at all object levels. Check size before hashing/decoding, decode UTF-8 fatally, reuse bounded YAML parsing, then validate without coercion/defaults. Map failures to the one canonical input issue; never expose YAML/Ajv exception text or paths. Parse bytes only, with no caller-object input overload.
- [x] **Step 5: Verify GREEN and types.** Run the focused test command and `npm run typecheck`; both exit 0, no test skips. Format only the new TS files with the existing Prettier binary.
- [x] **Step 6: Commit named files.** Commit `feat: define bounded synthetic package manifests`; include the draft execution report, no historical file edits.

### Task 2: Resolve the whole manifest and exact qualified references

**Files:** Create resolve.ts and resolve.test.ts; extend the fixture helper if needed.

**Interfaces:**
- Consumes: `PackageManifest` from a successful Task 1 parse; input is trusted parser-owned plain data, not forged/proxy objects.
- Produces: `resolvePackageManifest(manifest:PackageManifest):PackageResolution` from resolve.ts. It does not mutate the input and has no I/O.

- [x] **Step 1: Write failing graph tests** named `resolvesExactClosure`, `rejectsWholeInvalidModel`, `bindsExplicitProvider`, `preservesNamespaceIdentity` and `requiresDirectDependency`. Include the following checks, using test helpers to parse before resolving:

```ts
assert.deepEqual(linked.closure, [
  {packageId:'base', version:'1.0.0'}, {packageId:'sales', version:'2.0.0'},
]);
assert.equal(linked.bindings[0].provider.packageId, 'base');
assert.equal(transitiveOnly.issues[0].code, 'undeclared-dependency');
assert.deepEqual(badUnreachable.closure, []);
assert.deepEqual(badUnreachable.bindings, []);
```

Pin all model issue codes with separate cases: duplicate root/package/dependency/component/reference/requirement, missing root/provider/component, version mismatch, self/multi-node cycle, exact symbol collision and undeclared dependency. Diamond closure deduplicates. Same namespace/different symbols and same names/different namespaces or kinds pass; same `(namespace,kind,name)` fails, including null namespace. Explicit provider wins over another namespace's matching name. Shared namespace and transitive reachability confer no implicit import. Validate references in unused components and packages. Allowed `constructor` IDs/member names work via Maps/structural tuples. Invalid outputs have no partial results.
- [x] **Step 2: Run RED.** `node --import tsx --test test/package-feasibility-resolve.test.ts`; expect missing resolver failure.
- [x] **Step 3: Implement `resolvePackageManifest`.** Validate the complete manifest before publishing results. Use Maps and tuple keys, indexed exact versions, iterative cycle/closure traversal and explicit provider lookup. Repeated package IDs invalidate even when versions differ. Sort closure by packageId; sort bindings ordinally by consumer package/kind/name then provider package/kind/name/namespace (null before strings). Sort/deduplicate issues exactly as the spec requires. Never return an installation order or treat synthetic binding as real visibility.
- [x] **Step 4: Verify GREEN.** Run both parser and resolver test files, then `npm run typecheck`; expect exit 0/no skips. Format only touched new TS files.
- [x] **Step 5: Commit named files.** Commit `feat: resolve synthetic package graphs and qualified symbols`.

### Task 3: Compose deterministic execution blocker assessments

**Files:** Create assess.ts and assess.test.ts; use Task 1/2 types and helper.

**Interfaces:**
- Consumes: Task 1 parser, Task 2 resolver, the spec's exact blocker table.
- Produces: `assessPackageFeasibility(bytes:Uint8Array):PackageAssessment` from assess.ts, the only public bytes-to-report entry point. No clock/env/FS/network/CLI input.

- [x] **Step 1: Write failing policy tests** named `keepsExecutionUnproven`, `retainsOpaqueDependencies`, `ownsEveryBlocker`, `rejectsOptimisticEvidence`, `ignoresValidUnreachableBlockers` and `returnsDetachedDeterministicReports`. Example assertions:

```ts
assert.equal(report.execution, 'unproven');
assert.equal(report.classification, 'synthetic');
assert.ok(report.blockers.some(b => b.packageId === 'vendor' && b.code === 'source-unavailable'));
assert.ok(optimistic.blockers.some(b => b.code === 'synthetic-evidence-claim'));
assert.equal(emptyRequirements.blockers[0].code, 'real-target-unverified');
assert.deepEqual(invalid.blockers, []);
```

For all seven blocker rows assert complete object equality, owner/action and requirement fields, not only presence. Opaque dependencies remain in the closure and are not stubbed. Available source, empty components/requirements, and even target-verified claims never suppress universal blockers. Unknown source/kind/inventory produce separate blockers. A valid unreachable package yields no blockers. Invalid parsing/model returns no closure/bindings/blockers and keeps execution unproven. Run twice on fresh bytes; mutate old reports and source buffers after assessment to prove detachment. Permute roots/packages/dependencies/components/references/requirements: all semantic report fields stay equal after omitting digest.
- [x] **Step 2: Run RED.** `node --import tsx --test test/package-feasibility-assess.test.ts`; expect missing assessor failure.
- [x] **Step 3: Implement `assessPackageFeasibility`.** Compose parsing/resolution and derive blockers only from the validated root closure. Use exactly the literal codes/actions in the spec, ordinal sorting/deduplication and detached arrays/records. Never import runtime experiments, auto-read repository status or use declared evidence as measured target verification. No optional feature flag removes universal blockers.
- [x] **Step 4: Verify GREEN.** Run the three focused test files and `npm run typecheck`; exit 0/no skips. Format new TS files only.
- [x] **Step 5: Commit named files.** Commit `feat: report owned package execution blockers`.

### Task 4: Run repeatable fixtures and publish the bounded decision

**Files:** Create the five YAML fixtures and scenarios.test.ts listed above; complete `docs/compat/plan-22-package-dependency-feasibility.md`. Update only this plan/spec's status if justified.

**Interfaces:**
- Consumes: `assessPackageFeasibility(bytes)` and complete report types.
- Produces: fixed independent scenario matrix, measured commands/results and a synthetic-method decision separate from real-package execution status.

- [x] **Step 1: Author fixtures and failing integration assertions.** `runsPackageScenarioMatrixTwice` reads only fixed URLs under the fixture directory; no manifest controls file paths. linked is a valid exact-pin two-package reference, opaque adds a required unavailable-source package, optimistic claims target verification, transitive-import violates direct import, namespace-collision duplicates a qualified symbol. Include org-feature/installation/license-service requirements to demonstrate dependencies beyond source/runtime. Store expected codes/closure/binding/blocker assertions in the test, with no snapshot auto-acceptance. Each fixture runs twice with fresh buffers/reports and byte hashes checked before/after. Point report scenario rows to the focused tests for remaining required experiments.
- [x] **Step 2: Run RED.** `node --import tsx --test test/package-feasibility-scenarios.test.ts`; expect absent fixtures or mismatched expected outcomes before filling them in. Do not break existing passing resolver code to force a failure.
- [x] **Step 3: Complete fixtures and verify.** Run the scenario test and all focused Plan 22 tests; expect all pass/no skips. Then run repository checks once: `npm --prefix compat/clients/node run check` and `git diff --check` from root. Record actual failures/skips if any; correct substantive regressions and rerun affected checks. No browser/database/reference test is needed for these pure new modules.
- [x] **Step 4: Write the evidence-backed decision.** Record source ledger/access dates/version uncertainty, exact synthetic model, commands/counts and all mandatory experiment outcomes. Explain available source is not permission or implementation; symbol resolution is not Salesforce visibility/installation. Keep real-package execution unproven even if every offline test passes. Compare independent behavior implementation, eligible source execution and vendor-supported execution without claiming untested paths work. Assign maintainer to real inventory/access/policy decisions and platform-implementer to discriminating runtime/service/security/install/upgrade tests. Preserve historical Plan 19/21 evidence and unimplemented Plan 20 status; leave all milestones and estimates unchanged.
- [x] **Step 5: Verify isolation and review the whole change.** Compare all protected hashes, inspect imports/diff and verify no old file changed except permitted new-plan status text. Request a fresh whole-change review focused on root closure, unreachable rejection, direct dependencies, structural identity, input bounds, blocker ownership and evidence claims. Reproduce/fix substantive findings with focused regression tests, then rerun the checks affected by corrections. Record any unresolved material limitation; do not substitute reviewer opinion for measured test results.
- [x] **Step 6: Commit the measured report and fixtures.** Commit `test: record package dependency feasibility and execution gaps`; mark complete only after mandatory evidence and review are recorded. A positive decision covers the synthetic method alone.

## Plan self-review

Spec coverage: Task 1 owns strict bounded input/source policy; Task 2 owns complete model/namespace/graph validation; Task 3 owns reporting, closure blockers and claim limits; Task 4 owns repeatability, source ledger, evidence, review and isolation. Every Review Focus condition has an owning test above. Shared signatures/types originate in Task 1 and remain consistent across later tasks. Actual package execution is deliberately unproven, and no step depends on Plan 20 being implemented.
