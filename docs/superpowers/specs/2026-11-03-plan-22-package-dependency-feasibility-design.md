# Plan 22 — Package dependency feasibility

Status: implemented, independently reviewed and verified, 2026-11-04. Decision: Continue for synthetic dependency resolution only; [results and execution gaps](../../compat/plan-22-package-dependency-feasibility.md). Real-package execution remains unproven.
Milestone: bounded M7 feasibility discovery with M0 inputs; all milestone gates remain open.

## Intent and selection

Determine whether a small, independently authored package manifest can represent exact dependencies, resolve namespace-qualified component references and expose the reasons a required package cannot yet be executed. Produce a reproducible dependency experiment and an owned package execution blocker matrix. A successful graph experiment must never become a claim that a Salesforce package installs or runs.

This is the executor's recommended next package after completed [Plan 21](../../compat/plan-21-imported-ui-hosting-feasibility.md). The [recommended sequence](../../plans/2026-10-20-progress-and-remaining-roadmap.md#recommended-sequence) calls for early Apex, Flow, imported UI and package feasibility. Plans 19 and 21 demonstrated tiny independent interpreter subsets; [Plan 20](2026-11-03-plan-20-flow-execution-feasibility-design.md) remains unimplemented. [Plan 17](../../compat/plan-17-target-inventory.md) has a synthetic inventory, with no selected real package or target. Package support is therefore still an unknown, rather than an optional requirement we can omit.

Alternatives are to wait for an actual package and perform authorized black-box tests, or expand the Apex/UI runtimes first. Waiting supplies no independent dependency experiment; expanding runtimes without a package inventory risks building the wrong surfaces. This bounded experiment is recommended now, with real-package execution explicitly gated on a named target, permitted evidence and a separately designed test. It does not replace unfinished foundation work or Plan 20.

## Constraints and exclusions

- Work directly on main under the repository convention.
- Use the existing compat Node >=22 toolchain and pinned dependencies; no new dependency or manifest/lockfile changes.
- Use independently authored synthetic manifests only; no live org, credentials, network calls, package installation, subprocess, customer source or vendor communication in the experiment.
- Preserve CLEAN_ROOM.md and all existing product code, accepted profiles, observations, readiness/evidence, historical reports and Plans 10/14/17/19/20/21 byte-for-byte.
- Source locators are inert text; never open, fetch, import or execute them.
- No package archive parsing, source-code parsing/execution, version-range solver, CLI command, deployment engine, license bypass or runtime bridge.
- No claim of Salesforce installation, dependency-discovery, visibility, security, upgrade or execution compatibility.

Read [CLEAN_ROOM.md](../../../CLEAN_ROOM.md) before implementation. No proprietary internals, CLI/plugin source, public examples or third-party implementation are copied. Recording declared source availability does not grant permission to read or reuse source. Any later source reuse remains subject to the exact policy and a separate provenance decision.

## Documentation basis

Official public documentation read on 2026-11-03 informs vocabulary only:

| Source | Limited basis |
| --- | --- |
| [Unlocked Packages](https://developer.salesforce.com/docs/platform/sfdx-dev/guide/sfdx-dev-unlocked-pkg-intro.html) | Packages organize metadata; package configuration includes version/dependency information. Org-dependent packages can also require unpackaged metadata. |
| [Understanding Namespaces](https://developer.salesforce.com/docs/platform/sfdx-dev/guide/sfdx-dev-unlocked-pkg-plan-namespaces.html) | Namespaces qualify components; multiple packages may share a namespace. Visibility requires its own semantics. |
| [First-generation managed package dependencies](https://developer.salesforce.com/docs/platform/pkg1-dev/guide/about-dependencies.html) | Dependencies can include other components, permissions and org features. This page does not establish 2GP or unlocked-package behavior. |

These current guides are not a pinned target version or reference observation. The manifest format, case-sensitive symbol resolver and graph rules below are independent synthetic choices. Do not infer real Apex visibility, package type eligibility or completeness from them. The execution report records URL/section/access date/read boundary/version uncertainty; inaccessible required sources remain owned gaps.

## Deliverables and file boundaries

1. An offline pure assessor in `compat/clients/node/src/package-feasibility-{model,parse,resolve,assess}.ts` and a strict schema at `compat/schemas/package-feasibility.schema.json`.
2. Synthetic YAML manifests in `compat/fixtures/package-feasibility/`, with Node tests in `compat/clients/node/test/package-feasibility*.test.ts` and an optional fixture helper without a `.test.ts` suffix.
3. `docs/compat/plan-22-package-dependency-feasibility.md`: measured scenario outcomes, source ledger, separate graph/real-package conclusions, alternatives and owned next actions.

No CLI/validator registration is needed. Existing test discovery includes new `.test.ts` files; typecheck covers `src` and `test`. The tests load fixed local fixtures; the assessor receives bytes and performs no I/O. Production modules never import the experiment. Its own modules may reuse `parseInventoryDocument` as a bounded YAML parser; do not modify that parser or treat these manifests as Plan 17 inventories.

## Exact synthetic input contract

One UTF-8 YAML document, maximum 1,048,576 bytes. Reuse `parseInventoryDocument`'s duplicate-key rejection, depth 32, node 10,000 and alias-expansion 50 bounds. Strict schema rejects unknown keys at every object; arrays have the cardinalities below. Invalid encoding, parsing or schema errors return bounded diagnostics without raw exceptions. Check byte size before decoding, hashing or retaining the input; oversized input has `inputDigest=null`. Otherwise SHA-256 covers original bytes, including invalid input. No implicit coercion or defaulting.

All properties below are required. `id` means ASCII `[a-z][a-z0-9-]{0,63}`. `version` is an opaque exact ASCII `[A-Za-z0-9][A-Za-z0-9._-]{0,63}` string; do not order, normalize, range-match or assume Salesforce ID/version syntax. IDs identify packages in this synthetic target, not Salesforce package IDs. `name` means `[A-Za-z_][A-Za-z0-9_]{0,63}`; namespace is null or `[A-Za-z][A-Za-z0-9]{0,14}`. Matching is case-sensitive. Namespaces/symbols are model labels, not a complete Salesforce naming grammar.

| Object | Properties and cardinalities |
| --- | --- |
| Document | `schemaVersion: 1`, `classification: synthetic`, `roots: id[1..32]`, `packages: Package[1..128]`. |
| Package | `id`, `version`, `ownerId: id`, `kind: unmanaged|unlocked|managed-1gp|managed-2gp|unknown`, `namespace`, `source: available|unavailable|unknown`, `dependencyKnowledge: complete|unknown`, `dependencies: Dependency[0..128]`, `components: Component[0..256]`, `requirements: Requirement[0..32]`. |
| Dependency | `packageId: id`, `version`. |
| Component | `kind: object|field|apex|flow|imported-ui`, `name`, `references: Reference[0..128]`. |
| Reference | `packageId: id`, `kind` from Component, `name`. The provider package supplies the namespace; no guessed prefix or fallback. |
| Requirement | `id`, `surface: metadata|apex|flow|imported-ui|platform-service|org-feature|installation|license-service`, `evidence: absent|subset-only|target-verified`. |

The scalar `source` is a declaration about availability, not source contents or eligibility. `target-verified` is deliberately accepted as an adversarial declaration to prove it cannot confer real verification on a synthetic manifest. It is reported as an untrusted claim, not converted to evidence. Package kinds do not imply source availability. Non-package org/platform dependencies belong in `requirements`; this experiment cannot prove those lists complete.

## Validation and resolution

Validate every package, including unreachable ones, before resolving roots. Duplicate roots/package IDs, repeated dependency IDs, duplicate per-package `(kind,name)` components, duplicate per-component reference triples, duplicate requirement IDs, missing roots, missing dependency providers, dependency version mismatches and dependency cycles are model-invalid. Self dependencies are cycles. One target cannot declare two installed versions of the same package ID. Use iterative graph traversal or explicitly bounded traversal; never recurse without enforcing depth bounds.

Across the complete manifest, two components with the same `(namespace,kind,name)` are a symbol collision even if owned by different packages. Two packages may share a namespace when their component keys differ. Null is its own namespace, distinct from any string. Same component names in different namespaces or of different kinds are distinct. Use structural keys or collision-free tuple serialization and Maps; user IDs such as `constructor` must never index prototypes. This collision rule is synthetic, not an installation eligibility determination.

A reference resolves only to its explicitly named provider and exact `(kind,name)`. A cross-package reference also requires a **direct** declared dependency; transitive reachability alone does not grant an import. A same-package reference needs no dependency. Missing provider, missing component or undeclared direct dependency is model-invalid even in an unreachable component. No fallback to another package, namespace or component kind; same namespace does not confer access. This rule tests deliberate binding, not actual Apex visibility.

If any issue exists, report `validation=invalid`, empty `closure`, `bindings` and `blockers`, and `execution=unproven`. Otherwise compute the transitive closure of roots. Output closure entries contain package ID/version, sorted by package ID; this is not an installation order. Bindings include only components in that closure and identify consumer package/kind/name and provider package/kind/name/namespace. Sort bindings ordinally by consumer package/kind/name then provider package/kind/name/namespace, with null before namespace strings. Unreachable valid packages do not add execution blockers.

## Report and blocker policy

`Assessment` contains exactly `schemaVersion:1`, `classification:synthetic`, `inputDigest:string|null`, `validation:valid|invalid`, `execution:unproven`, `closure`, `bindings`, `issues`, `blockers`. There is no `compatible`, `ready` or executable boolean. No timestamp, environment or random value enters the result. Return detached data; later assessment/caller mutations do not alter prior reports.

Issues contain `code` and optional validated `packageId`, `componentKind`, `componentName`. Codes: `input-too-large`, `encoding-invalid`, `parse-invalid`, `schema-invalid`, `duplicate-id`, `duplicate-dependency`, `duplicate-component`, `duplicate-reference`, `duplicate-requirement`, `missing-root`, `missing-package`, `version-mismatch`, `dependency-cycle`, `symbol-collision`, `missing-component`, `undeclared-dependency`. Syntax/schema failures expose only their one category, without input-derived paths, names or prose. Model diagnostics use only schema-validated identifiers. Deduplicate and sort issues by `(code,packageId-or-empty,componentKind-or-empty,componentName-or-empty)` using ordinal comparison, not locale sorting.

For each package in the root closure emit all applicable blockers:

| Condition | Code | Owner / next action |
| --- | --- | --- |
| Every reachable package, without exception | `real-target-unverified` | `maintainer` / `select-target-and-collect-evidence` |
| kind unknown | `package-kind-unknown` | package ownerId / `collect-package-details` |
| source unavailable | `source-unavailable` | package ownerId / `establish-permitted-execution-strategy` |
| source unknown | `source-unknown` | package ownerId / `collect-source-availability` |
| dependencyKnowledge unknown | `dependency-inventory-unknown` | package ownerId / `collect-dependency-inventory` |
| Each requirement with evidence absent or subset-only | `surface-unverified` | `platform-implementer` / `design-target-surface-test` |
| Each requirement with evidence target-verified | `synthetic-evidence-claim` | `maintainer` / `collect-real-evidence` |

Blockers contain `code`, `packageId`, `ownerId`, `nextAction` and, only for requirement blockers, `requirementId`, `surface`, `evidence`. Sort/deduplicate by `(packageId,code,requirementId-or-empty)`. Empty components/requirements or `source=available` never suppress the universal real-target blocker. An opaque package stays in the closure; never drop it or replace its behavior with a stub and claim execution. A root with an opaque dependency inherits the blocker through the included dependency; no duplicated ancestor blocker is needed.

## Required experiments and completion gate

Run every deterministic scenario twice with fresh buffers/objects and compare complete results, including original digests. Permutations must preserve semantic output after excluding the intentionally byte-sensitive digest.

| Scenario | Required result |
| --- | --- |
| Two-package synthetic root/provider, exact version and qualified object/field references | Correct closure/bindings, unchanged fixture hashes, execution unproven and both universal blockers. |
| Chain and diamond | Deduplicated complete closure; transitive package is included but direct-reference rule still enforced. |
| Namespace boundary | Same names under distinct namespaces resolve to explicit providers; shared namespace with distinct symbols passes; exact symbol collision fails; differing kinds stay distinct. |
| Broken graph | Missing root/provider, exact-version mismatch, self/multi-node cycle, duplicate identities/edges, missing component and undeclared direct import fail without partial closure/bindings. |
| Unreachable content | Bad unreachable graph/reference still invalidates; valid unreachable opaque package adds no blocker. |
| Opaque/unknown required dependency | Retained closure, source/inventory blockers with exact owners/actions; no substitute execution. |
| Optimistic claims | Available source, zero requirements and target-verified declarations all leave execution unproven; claimed verification yields explicit synthetic-evidence-claim. |
| Input bounds and rejection | Exact/over byte size, invalid UTF-8, duplicate YAML keys, parser depth/node/alias bounds, unknown keys, oversized arrays/strings, invalid enum/classification and trailing document reject safely. |
| Determinism and identity safety | Input permutations, constructor/prototype-like allowed names, caller/report mutation and repeated calls cannot change bindings or leak shared state. |

The report maps Plans 19/20/21 to subset-only/absent feasibility evidence in prose, never auto-detects their status from mutable files or labels any package target-verified. Compare three follow-up paths: independent implementation from authorized observations, eligible source-based execution after policy/provenance review, and vendor-supported execution where source is unavailable. No path is declared viable without actual evidence.

Acceptance: mandatory offline tests pass; `npm --prefix compat/clients/node run check` and `git diff --check` pass; protected existing-file hashes match. No database/browser/live test is required for this pure dependency experiment. Record actual commands/counts/failures/skips and get a fresh whole-change review of complete validation, structural identity, root closure, blocker ownership and truthful claims.

Separate conclusions: **Continue** may apply only to the synthetic declaration/resolution approach after mandatory assertions pass; **Revise** records an attempted approach failure and discriminating alternative; **Blocked** names a missing required experiment prerequisite/owner/action. Unattempted work is incomplete. **Real-package execution remains unproven** in every case. Maintainer owns real target/package inventory and evidence access; platform-implementer owns follow-up runtime/metadata/security/install/upgrade/service probes. No M0/M7 exit gate, Plan 20 completion, production readiness or program-budget change follows from this package.

## Builder check

A fresh software-engineer reviewer read the design and reported “nothing important left.” The reviewer found the contract, graph/namespace rules, blocker policy and acceptance boundary sufficient to plan implementation. All package format/resolver choices above remain executor recommendations for maintainer review, not collected target requirements.
