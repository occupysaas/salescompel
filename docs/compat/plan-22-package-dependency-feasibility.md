# Plan 22 — Package dependency feasibility

Status: implemented, independently reviewed and verified, 2026-11-04.
Decision: **Continue** for the synthetic declaration/resolution method only.
Real-package execution remains **unproven**.

[Design](../superpowers/specs/2026-11-03-plan-22-package-dependency-feasibility-design.md)
and [implementation plan](../superpowers/plans/2026-11-03-plan-22-package-dependency-feasibility.md).

## Source ledger

Official pages re-read on 2026-11-04, as vocabulary only. Current documentation has
unknown applicability to any real target package/version. No source examples,
third-party implementation, package internals or reference observations are copied.

| Source / section | Read boundary |
| --- | --- |
| [Unlocked Packages](https://developer.salesforce.com/docs/platform/sfdx-dev/guide/sfdx-dev-unlocked-pkg-intro.html) | Introductory metadata/source-driven paragraphs, org-dependent overview and configuration overview; package/version/dependency vocabulary only. |
| [Understanding Namespaces](https://developer.salesforce.com/docs/platform/sfdx-dev/guide/sfdx-dev-unlocked-pkg-plan-namespaces.html) | Definition, shared-namespace considerations and visibility paragraph; qualification is distinct from visibility. No examples copied. |
| [1GP dependencies](https://developer.salesforce.com/docs/platform/pkg1-dev/guide/about-dependencies.html) | Opening definition and component/org-feature dependency paragraphs; no inference about other package types. |

## Implemented boundary

`assessPackageFeasibility(bytes: Uint8Array): PackageAssessment` in
`compat/clients/node/src/package-feasibility-assess.ts` is the public entry point.
It accepts the design's synthetic-only, 1 MiB bounded YAML declaration, validates
all packages and qualified references, computes the root closure, then emits
owned blockers. It performs no invocation-time I/O, network, subprocess or
execution. Module initialization reads fixed repository schemas, including the
existing target-inventory schema through the shared parser import. No caller
locator is opened, and this experiment registers no CLI command.

The schema pins the complete property/enum/cardinality allowlist. IDs/versions
and component names are exact case-sensitive labels. Versions are not sorted or
range-matched. Package identity, namespace identity and component identity are
separate: shared namespaces with different symbols are accepted, while exact
qualified collisions are rejected. Explicit direct dependencies are required for
cross-package references; same-namespace or transitive access supplies no grant.
These are synthetic rules, not Salesforce package eligibility or Apex visibility.

The parser reuses the existing depth/node/alias limits. During RED→GREEN testing,
a valid first YAML document followed by another was accepted by the shared parser.
A local `parseAllDocuments` cardinality check now rejects that input before the
shared bounded parser is used. The historical parser stays byte-identical. Input
size is checked before hashing/decoding; below-limit failures retain the original
SHA-256; oversized input returns a null digest. Parse/schema errors contain only
canonical categories, without raw exceptions or marker text. Module initialization,
trusted parser-owned models, native YAML parser allocation and aggregate heap
isolation are distinct from the bounded assessment contract; this is not a
hostile-code sandbox or a resource-isolation certification.

The resolver uses structural tuple keys and Maps. Complete validation precedes
publication; invalid input has empty closure, bindings and blockers. If package
IDs are ambiguous, identity diagnostics are returned before semantic resolution
rather than selecting one version/namespace arbitrarily. All cyclic members are
identified iteratively. Valid unreachable packages add no execution blockers,
but malformed unreachable references/graphs invalidate the whole model.

Every reachable package receives `real-target-unverified`, even with available
source and zero declared requirements. Opaque packages remain in the closure;
source/inventory/service blockers retain their specified owners and actions.
A synthetic `target-verified` declaration produces `synthetic-evidence-claim`;
it never becomes actual evidence. Returned reports contain detached primitives
and arrays, deterministic sorted diagnostics, no clock/env data and no runtime
readiness boolean. Original byte digests intentionally change with serialization.

## Measured scenarios

All 39 Plan 22 test/subtest events passed, with zero failures/skips. Each of the
14 focused cases runs twice with fresh inputs; the five checked-in scenarios
also each run twice, under one parent test. Assertions compare complete reports,
not an automatically accepted snapshot.

| Required experiment | Measured assertion / result |
| --- | --- |
| Exact dependency and qualified references | linked.yaml twice: closure base@1.0.0/sales@2.0.0, two explicit object/field bindings, both real-target blockers; fixture bytes/hashes unchanged. |
| Chain/diamond | resolvesExactClosure twice: diamond deduplicates; 128-package chain resolves without recursive graph execution. |
| Namespace boundary | preservesNamespaceIdentity/bindsExplicitProvider twice: distinct namespaces/kinds, shared namespace with distinct symbols, null namespace and provider-specific lookup; exact collision rejects both owners, independent of input order. |
| Broken graph | rejectsWholeInvalidModel/requiresDirectDependency twice: duplicate identities/edges/components/references/requirements, missing roots/providers/members, exact-version mismatch, self/multi-node cycles and undeclared direct/transitive imports reject without partial results. |
| Unreachable content | Malformed unused references/cycles invalidate; valid unused opaque package contributes no blockers until rooted. |
| Opaque dependency | opaque.yaml and retainsOpaqueDependencies: unavailable source remains in closure with exact owner/actions; org-feature, installation and license-service requirements are reported separately. No substitute implementation. |
| Optimistic claims | optimistic.yaml, keepsExecutionUnproven/rejectsOptimisticEvidence/ownsEveryBlocker: all package kinds and empty requirements retain universal blockers; target-verified becomes a synthetic claim; all seven blocker rows have exact object assertions. |
| Bounded input | parsesSyntheticManifest/rejectsMalformedManifest/boundsManifestInput: exact/over 1,048,576 bytes, UTF-8, duplicate keys/trailing documents, depth/node/alias failures, every required property and unknown nested key, all array maxima and identifier/version bounds. Diagnostics exclude injected marker text. |
| Repeatability and identity | returnsDetachedDeterministicReports and fixture matrix: repeated original digests/results, semantic equality under input permutations, prototype-like allowed IDs/member names, detached parsed models/reports and complete invalid outputs. |

The five fixtures and model are independently authored; they contain declarations
only. No real package contents, actual org inventory, package source or Salesforce
reference observations were collected.

## Reproduction and verification

From repository root, using the existing installed/pinned dependencies and
Node 26.8.1 (satisfies >=22):

```sh
cd compat/clients/node
node --import tsx --test test/package-feasibility-*.test.ts
npm run typecheck
cd ../../..
npm --prefix compat/clients/node run check
git diff --check
```

| Check | Actual result |
| --- | --- |
| Baseline laboratory npm test | 369 events: 366 passed, 3 skipped, 0 failed. |
| Focused Plan 22 tests | 39 passed, 0 failed/skipped. |
| Typecheck through Task 3 | Exit 0. |
| Full laboratory check | Exit 0: typecheck and formatting passed; 408 test/subtest events, 405 passed, 3 skipped, 0 failed; offline validation passed with 11 reviewed pilot observations, not_run=0. No parity certification. |
| Protected file hashes | All 385 pre-existing tracked files match. |
| git diff --check (complete base-to-final change) | Exit 0 against base f0b30fb89907ac6d210d489f272b65213fcd8a64; includes committed additions. |

No browser, database or live reference run is needed or implied for this pure
new experiment. The three unchanged baseline skips are `candidate/jsforce/core`,
`candidate/jsforce/policy` and `candidate/jsforce/isolation`, gated by
`RUN_CANDIDATE !== "1"`. They are not passing live candidate or compatibility
scenarios. The first full-check attempt stopped on Prettier formatting in the new
resolver, before tests; applying the pinned formatter resolved it and the full
check above passed. No existing file was changed to make a check pass.

## Decision and follow-up gates

Decision: **Continue** for this bounded synthetic declaration/resolution method.
All mandatory offline assertions and the full lab check passed, with independent
review finding no implementation issue. The experiment shows that exact declared
packages/references and their execution blockers can be represented without
pretending missing runtime/source/service evidence exists.
**Real-package execution remains unproven** regardless of that decision.

Plan 19's tiny Apex interpreter and Plan 21's tiny imported UI interpreter provide
subset-only feasibility evidence, not target-package coverage. Plan 20 remains
unimplemented and supplies no measured Flow execution evidence. No report imports
or reads these historical documents at runtime to upgrade a declared capability.
Plan 17 remains a synthetic inventory; no required package can silently be deemed
absent or optional. Plans 10/14, all milestone gates and budget estimates are
unchanged; this package does not close M0 or M7.

| Follow-up path | Evidence needed before a viability decision | Owner / next action |
| --- | --- | --- |
| Independent behavior implementation | Named target, exact versions, permitted black-box package workflows, dependency/service/visibility/security and failure observations | maintainer selects inventory/access; platform-implementer designs discriminating execution tests. |
| Eligible source-based execution | Actual source availability plus exact policy/provenance eligibility, runtime/service inventory and measured target execution | maintainer resolves eligibility before any reuse; platform-implementer designs the bounded runtime test. Available source alone supplies neither permission nor working execution. |
| Vendor-supported execution for opaque packages | Supported execution strategy, permitted interfaces and vendor-backed availability, then observed end-to-end behavior | maintainer owns the decision and any future explicitly authorized contact; platform-implementer designs service/install/upgrade probes. No contact was made. |

The universal next gate is a real package inventory with package/version/namespace,
source availability, full org-feature/service dependencies and workflow acceptance.
No source or vendor access is required to finish this synthetic experiment; those
are owned blockers on real-package execution, rather than unfinished tests here.

## Independent review

A fresh-context reviewer inspected base f0b30fb through the complete implementation,
independently ran all 39 focused tests (no failures/skips), and reported no Critical
or Important findings. Its additional read-only checks passed 39 graph/identity
cases and 20 trailing-line-terminator schema probes. These supplemental probes
are separate from the checked-in suite and do not increase its test count.

The sole Minor finding was report EOF whitespace missed by a clean-tree-only
`git diff --check`. The base-to-final-change command reproduced it; removing the
blank line during documentation finalization resolved it. No runtime code fix or
new implementation test was needed. The full lab check was measured by the
executor, not inferred from the reviewer's opinion.

The executor accepted the review's explicit boundaries:

| Ruling | Reason / remaining cost |
| --- | --- |
| Salesforce package execution/install/visibility/upgrade/eligibility remain unjudged | Synthetic declarations cannot supply target behavior; real-package execution remains unproven. |
| Direct-main work and fixed-schema initialization reads are accepted | Approved plan follows repository convention; there is no branch isolation and module loading needs the fixed schema files. Assessment calls perform no I/O. |
| Duplicate package identity errors precede further semantic resolution | Deterministic rejection avoids choosing an arbitrary version/namespace; diagnostics need not enumerate every other error on that invalid model. |
| Forged/proxy resolver models and concurrent shared-memory input mutation are outside the contract | Resolver models must be parser-owned plain data and byte inputs must remain stable during synchronous assessment; arbitrary hostile in-process callers are not certified. |
| Native parser peak heap and hard wall-clock isolation remain unjudged | The reviewed byte/AST/depth/alias limits do not establish a sandbox or aggregate allocation/time guarantees. |
| Public documentation establishes vocabulary, not target compatibility or reuse permission | No source is reused and actual package eligibility/version evidence remains a future gate under CLEAN_ROOM.md. |
| Final prose/checklists depend on measured checks and this review | Completed at handoff; future status claims must preserve these limits and actual commands. |
