# Plan 17 — Target Inventory Contract Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Deliver an offline, versioned requirements inventory with a synthetic worked example, deterministic gap reporting and validation distinct from readiness.

**Architecture:** Keep the schema and typed inventory model separate from pure semantic assessment and local file/CLI handling. Reuse pinned Ajv, strict YAML parsing and the existing laboratory CLI; repository validation invokes the same assessment against one explicit checked-in synthetic example.

**Tech Stack:** Existing Node/TypeScript lab, Node >=22, Ajv 8.20.0, yaml 2.9.1, Node test runner through tsx; no dependency changes.

**Spec:** [Plan 17 design](../specs/2026-10-21-plan-17-target-inventory-contract-design.md). The maintainer authorized implementation; execution is complete. See the [verification record](../../compat/plan-17-target-inventory.md).

## Global Constraints

- Work directly on main under repository convention.
- Keep accepted profile bytes, hashes, reference-readiness attestations and historical evidence unchanged.
- No new runtime dependencies, Go/database/catalog changes, product APIs, UI screens, network collection, credential discovery or package installation belong here.
- Public example data must be independently authored and synthetic.
- Exit 0 means structurally and semantically valid, even when the inventory openly records gaps.
- Exit 1 means malformed/invalid input or a failed validation invariant.
- Exit 2 means input/output could not be safely read/written.
- A synthetic inventory always reports `synthetic-only`, even if internally complete.
- A complete real inventory reports `ready-for-review`, never M0 passed or certified.
- Reports do not attest evidence truth, authorization or sanitization of arbitrary exports. Plans 10/14 evidence and Plan 13 visual acceptance remain independent.

## Review Focus

- YAML/JSON duplicate keys, expansion bombs and malformed UTF-8 must produce bounded, value-free diagnostics (Task 2).
- Symlinks and traversal through an apparently permitted repository path must never open private files (Tasks 2–3).
- A valid document with unknown numeric/version values or explicit false features must retain those distinctions (Tasks 1–2).
- User-controlled keys, locators and parser errors must not leak into terminal or report output (Tasks 2–3).
- An existing output, missing parent or input/output alias must fail without changing input or prior evidence (Task 3).

## File structure and interfaces

Create:

- `compat/schemas/target-inventory.schema.json`: closed draft-07 schema, local definitions only.
- `compat/clients/node/src/target-inventory-model.ts`: exported types, section/category registries, trusted issue/action codes.
- `compat/clients/node/src/target-inventory.ts`: bounded parsing, schema/semantic validation and deterministic report construction.
- `compat/clients/node/src/target-inventory-command.ts`: safe local input/output handling and command result.
- `compat/clients/node/test/target-inventory-fixtures.ts`: typed synthetic factories; no `.test.ts` suffix.
- `compat/clients/node/test/target-inventory.test.ts`: schema, semantic, parsing and privacy tests.
- `compat/clients/node/test/target-inventory-command.test.ts`: filesystem and real CLI subprocess tests.
- `docs/compat/inventories/synthetic-account-contact.yaml`: public incomplete example.
- `docs/compat/plan-17-target-inventory.md`: format, limits, command, privacy and handoff guide.

Modify `compat/clients/node/src/cli.ts` to dispatch the command before suite selection; `compat/clients/node/src/validate.ts` to validate schema/example; `docs/compat/README.md` to link the guide. Avoid restructuring the existing large CLI or changing general command semantics.

Use the following shared interfaces; do not invent parallel report or validation models in later tasks:

```ts
type SectionId = 'sourceOrg' | 'dataMetadata' | 'workflowsUsers' |
  'clientsIntegrations' | 'cliPlugins' | 'automationCustomization' |
  'security' | 'localization' | 'migrationOperations';
type Knowledge<T> = { status: 'known'; value: T } |
  { status: 'unknown'; question: string; nextAction: ActionCode };
type ActionCode = 'collect' | 'clarify' | 'assign-owner' |
  'define-acceptance' | 'reconcile' | 'review-scope';
type InventoryIssue = { code: IssueCode; section?: SectionId;
  itemId?: string; ownerId?: string; nextAction?: ActionCode };
type InventoryReport = { schemaVersion: 1; inputDigest: string;
  inventoryId?: string; revision?: number; classification?: 'synthetic' | 'real';
  validation: 'valid' | 'invalid';
  readiness: 'incomplete' | 'synthetic-only' | 'ready-for-review';
  sections: { id: SectionId; status: 'unknown' | 'partial' | 'complete' }[];
  issues: InventoryIssue[]; gaps: InventoryIssue[] };
```

`IssueCode` is a closed exported union defined in Task 2; messages are trusted static templates, never Ajv/parser prose. Reports contain no free-text questions, rationale, locators, labels or unknown property names. Owner/item/inventory IDs are public diagnostic identifiers, restricted to lowercase `[a-z][a-z0-9-]{0,63}`; authors must never place secrets in identifiers. Their deliberate inclusion is documented, not advertised as general anonymization.

### Task 1: Define the inventory contract and synthetic example

**Files:** model, schema, fixture factory, example and `target-inventory.test.ts` listed above.

**Interfaces:** Produces `TargetInventory`, `SectionId`, `Knowledge<T>`, `ActionCode`, `SECTION_IDS` and `CATEGORY_SECTION` from the model; `makeInventory(): TargetInventory` and `makeCompleteInventory(classification): TargetInventory` from the test factory. All required fields described below are reflected in both schema and types.

- [x] **Step 1: Write failing schema tests.** Use Ajv to load the proposed schema. Assert `makeInventory()` and both complete factories validate; removing any section, adding an unknown property, using schemaVersion 2, unknown disposition or malformed date/ID fails. Assert known false, known zero volume and unknown volume/version are distinct valid representations; negative volume and invented unknown sentinel strings fail.
- [x] **Step 2: Run the focused test and confirm a missing-schema/module failure.** From `compat/clients/node`: `node --import tsx --test test/target-inventory.test.ts`.
- [x] **Step 3: Define the model/schema.** Top-level fields: `schemaVersion:1`, `inventoryId`, positive integer `revision`, calendar-valid `date` (`YYYY-MM-DD`, checked semantically), `classification`, `targetLabel`, `ownerId`, `sources`, and all nine `sections`. Source records: `id`, `kind` (repository, operator, source-export, pipeline, observation), `date`, `locator`, `description`. Sections contain `status`, `ownerId`, `evidenceRefs`, `questions`, `absence` (null or owner/evidence/rationale attestation), `items`. Questions contain `id`, `question`, `ownerId`, `nextAction`.

  Items share `id`, `category`, `disposition`, `rationale`, `ownerId`, `provenanceRefs`, `behavior: Knowledge<string>`, `acceptance: Knowledge<string>`, `milestone: Knowledge<MilestoneId>`, `dependencies` (item IDs), `repositoryRefs` (`{path,id?:string}`), `decision` (null or question/nextAction), `facts`, and `details`. `MilestoneId` is M0, M1, M1R, M2 through M10. `facts` has independent observation/implementation/verification entries, each status `unknown|documented` and evidenceRefs; absence of implementation is never parity evidence.

  Define category-specific closed detail objects using `Knowledge<T>` on unknownable fields. Source categories: `source` (edition/release/API versions), `org-feature` (name, enabled). Data: `object`, `field`, `relationship`, `record-type`, `layout`, `data-volume`, `export-area` with named identity and typed kind/relationship/count/unit/exportability as relevant. Workflow: `workflow`, `persona` with journey/outcomes/permission boundaries. Client: `client`, `integration` with exact version, protocol, endpoint description, authentication method and dependencies. CLI: `cli`, `plugin`, `cli-command` with version/channel or source/signature/license, command/flags/output formats and org/non-org target. Automation: `apex`, `flow`, `formula`, `validation`, `legacy-rule`, `imported-ui`, `package` with name/version/class API where relevant and dependencies. Security: `security-policy` with policy kind, grants and denied behavior. Localization: `locale-requirement` with wire language/locale/currency/timezone/support category/fallback. Operations: `migration-path`, `operating-objective` with path/identity/reconciliation or objective/value/unit. Keep exact property lists in model/schema synchronized; no arbitrary `details` map. Versions are nonempty exact source strings, not an assumed latest value; require API syntax `digits.0` when applicable.

- [x] **Step 4: Author the synthetic Account/Contact example.** Use executor-selected demonstration requirements with honest rationale, references to candidate/Plan 16 docs as implementation facts, explicit unknown wider requirements and no real-org claims. Include a plugin/command, permission denial, locale requirement and unknown operating objective to demonstrate typed sections. Empty unknown sections have owner/questions. Preserve laboratory profile bytes.
- [x] **Step 5: Run focused tests and typecheck.** Both must pass: focused command above, then `npm run typecheck`.
- [x] **Step 6: Commit the contract/example/tests.** `git add` only this task's files; commit `feat: define target inventory schema and synthetic example`.

### Task 2: Implement bounded assessment and gap reporting

**Files:** `target-inventory.ts`, model issue types, existing Task 1 tests.

**Interfaces:** Consumes Task 1 types/schema; exports `assessTargetInventory(bytes: Uint8Array): InventoryReport` (synchronous, no filesystem/network); `repositoryReferences(inventory: TargetInventory): {path:string,id?:string}[]` (used only after valid assessment). Schema may be loaded once from its trusted module-relative path; no user-supplied schema loading. Report digest is SHA-256 of original bytes, including malformed input.

- [x] **Step 1: Add failing semantic/parser tests.** Assert incomplete valid real => validation valid/readiness incomplete; complete real => ready-for-review; synthetic => synthetic-only even if invalid or complete. Assert complete+questions, complete+unknown item/detail, complete-empty without evidence, missing provenance, duplicate item/source/question IDs, missing dependency/source reference, self/cyclic dependencies, missing required behavior/acceptance/milestone, unknown disposition without decision and out-of-scope without rationale invalidate. A complete empty section with evidence-backed absence is valid; a nonempty section with an absence attestation is invalid. Known zero and false cannot be mistaken for unknown.
- [x] **Step 2: Add failing bounded-input/privacy tests.** Limit original bytes to 1 MiB, YAML aliases to 50 (matching `core.ts`); reject malformed UTF-8 with fatal decoding, multiple documents, duplicate YAML/JSON keys, cyclic aliases, malformed documents and invalid dates. Inject marker secrets in free-text values, locator, unexpected keys and parser-error lines; no marker appears in JSON report. Repeat assessment produces byte-identical reports. Assert input is not mutated. Stub network entry points to fail if called.
- [x] **Step 3: Run focused tests and observe the new failures.** Use Task 1 command; retain the failure output privately for the execution record.
- [x] **Step 4: Implement assessment.** Reuse `core.ts` parsing only if errors can be caught without publishing prose and cyclic/depth expansion stays bounded; otherwise use the same pinned YAML APIs locally. Enforce maximum resulting nesting depth 32 and 10,000 collection nodes using cycle detection before schema validation. Compile trusted local schema with Ajv, no remote resolver, defaults/coercion/removal disabled. Invalid structural input yields only trusted `parse-invalid`, `schema-invalid`, `input-too-large` or `encoding-invalid` issues and safe metadata that independently validates; never spread input into reports. Semantic codes cover completion contradiction, absence invalid, duplicate ID, dangling reference, dependency cycle, required information missing, decision missing, date invalid and repository-reference invalid. Gap codes cover unknown section/detail/disposition, missing evidence and pending question. Sort/deduplicate by fixed section order, itemId, code, ownerId, nextAction; omit free text. Invalid always prevents ready-for-review; valid synthetic stays synthetic-only.
- [x] **Step 5: Verify focused tests and typecheck pass.** Inspect exact report assertions, not only process exit.
- [x] **Step 6: Commit.** Commit task files as `feat: assess target inventory readiness and gaps offline`.

### Task 3: Add safe local command and repository-reference checks

**Files:** command module/test, `cli.ts`.

**Interfaces:** Exports `validateTargetInventoryFile(inputPath: string, outPath?: string): Promise<{report:InventoryReport;exitCode:0|1|2}>` and `checkInventoryRepositoryRefs(refs: {path:string,id?:string}[], root:string): Promise<InventoryIssue[]>`. On I/O failure return a trusted report with validation invalid/readiness incomplete and `input-io`/`output-io`; never return paths/error prose. The file command only checks reference syntax; actual existence/ID resolution occurs in repository validation (Task 4).

- [x] **Step 1: Write failing filesystem/subprocess tests.** Invoke real `src/cli.ts validate-target-inventory --input <file> [--out <new.json>]`; test exit 0/1/2 mapping, missing input/unknown flag as 1, safe JSON stdout without output and summary-only stdout with output. Use private temporary directories. Cover missing/unreadable file, FIFO/directory/symlink input, >1 MiB file, existing/missing-parent/symlink output, input=output and output write/sync failure; originals remain byte-identical. Check output mode 0600 and no report copied to stderr. Run without COMPAT env/auth state and with unused `--suite` rejected before suite loading.
- [x] **Step 2: Write failing repository-reference tests.** Reject absolute paths, URL/drive forms, backslashes, dot segments, encoded traversal, NUL and symlink escapes. Approved prefixes are `docs/compat/`, `docs/plans/`, `docs/superpowers/`, `locales/`, `api/`; reject missing files and unknown explicit IDs. Source locators resembling URLs/files are inert and never opened. Resolve contract IDs in YAML `contracts[].id`, profile IDs via YAML top-level `id`, Markdown IDs as exact headings normalized by lowercase/space-to-hyphen; reject an ID on other formats. Allow paths without IDs; do not require individual free-text evidence locators to exist.
- [x] **Step 3: Run `node --import tsx --test test/target-inventory-command.test.ts` and confirm new behavior fails.**
- [x] **Step 4: Implement file/CLI behavior.** Read regular input via no-follow handle and bounded read, preventing FIFO hangs; reject symlink components. Real inputs must be mode 0600 with owner-only parent directory outside checkout using existing `privateDirectory`; classification detection occurs privately after bounded read, with no publication before safety checks. Synthetic input can reside in the checkout. Optional output requires an existing owner-only non-symlink parent, exclusive create (`wx`, 0600), write/sync/close; no directory creation or overwrite. Failure removes only a file created by this invocation. Add `input` to CLI options and dispatch before `suite(...)`; catch all command errors locally and use trusted JSON/static messages so the existing global catch cannot print parser/OS prose. Restrict flags to input/out and require exactly one positional command.

  Reference resolution uses lexical allowlist plus realpath containment and regular-file checks before reading. References cannot resolve through symlinks. Read referenced files boundedly with no remote fetch. Return stable issue codes only. Keep provenance source locators entirely outside resolution.

- [x] **Step 5: Run both focused suites and typecheck; verify pass.**
- [x] **Step 6: Commit.** `feat: add private offline target inventory validation command`.

### Task 4: Integrate repository checks and document handoff

**Files:** `validate.ts`, both test suites, guide, compatibility README.

**Interfaces:** `validateRepository()` retains `Promise<string[]>`; add trusted, deterministic issue summaries from the new assessment/reference helper. It validates the schema and exact `docs/compat/inventories/synthetic-account-contact.yaml` path, not arbitrary directory contents. The command report model does not change.

- [x] **Step 1: Add failing integration tests.** A temporary fixture repository with malformed example/schema, missing approved reference or dangling contract ID produces a validation issue; valid incomplete synthetic data passes repository validation. Factor a narrow helper accepting root if necessary; do not mutate tracked fixtures in tests. Assert repository validation actually invokes the new assessor and resolver. Ensure sources are never fetched, the sample cannot be reclassified real silently, and independent requirement/implementation facts survive round-trip validation.
- [x] **Step 2: Run focused tests and confirm the missing integration failure.**
- [x] **Step 3: Integrate validation.** Reuse Task 2/3 interfaces, require sample classification synthetic, and validate every repository reference from the example. Keep existing profile validation and evidence logic intact. Prefix failures with a static target-inventory label; no user-controlled diagnostics.
- [x] **Step 4: Write the guide and README link.** Show input/out invocation from `compat/clients/node`, exit/readiness table, all sections and typed unknown/false/zero examples, safe diagnostic identifier policy, inert source references, private real-input/output rules, limits, explicit absence and source-of-fact distinctions. Include operator checklist: nominate target/owners, collect authorized information privately, reconcile questions, review customization feasibility/export exclusions, propose separate target profile/contracts. Record synthetic example as incomplete and M0 still open. Do not publish any real inventory or claim reference collection/production readiness.
- [x] **Step 5: Run final verification from repository root.** Run `npm --prefix compat/clients/node run check`, `git diff --check`, then execute the new CLI against the public example without `--out` and confirm valid/synthetic-only plus actionable gaps. Record counts/skips honestly; the existing optional real-candidate skip is allowed for this offline change. Compare `git diff --name-only <execution-base>` and byte digests of both profiles/readiness/history against execution base; they must be unchanged. No database/browser/live acceptance is required.
- [x] **Step 6: Record actual results/limits in the guide and commit.** Commit `docs: integrate target inventory validation and operator handoff`. Obtain fresh whole-change review under the normal execution workflow, correct material findings, and rerun affected checks before reporting implementation complete.

## Self-review and handoff

Coverage: Task 1 owns the nine-section closed contract/example; Task 2 owns validity/readiness, unknowns, provenance and deterministic safe reports; Task 3 owns input/output privacy, reference safety and CLI exits; Task 4 owns normal validation, operator handoff and regression evidence. All five Review Focus conditions have explicit tests. This is one subsystem with sequential shared interfaces; native execution is recommended for its four dependent tasks.

This plan delivers no customer inventory, profile expansion, product behavior, M0 sign-off or program re-estimate. Native execution completed, including independent whole-change review and a regression-tested correction pass. Final full lab verification: 321 tests, 318 passed, three existing optional live-candidate skips; no failures.
