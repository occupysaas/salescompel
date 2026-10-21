# Foundation Release-History Reconciliation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Produce a source-backed, bounded M1R review of the REST/query foundation, with actionable documentary and live-evidence dispositions for every registered foundation contract.

**Architecture:** Add one scoped review batch to the existing release index, additive finding details to the existing changes ledger, and a small offline validator integrated with ordinary repository validation. Perform the actual official-source review before filling those records. Preserve the existing foundation assessor's resolution semantics and distinguish batch completion from contract freeze.

**Tech Stack:** Existing Node >=22, TypeScript, `node:test`, YAML 2.9.1 and existing lab tooling; public official documentation retrieval, with the PDF skill when needed.

**Spec:** `docs/superpowers/specs/2026-10-21-plan-18-foundation-release-reconciliation-design.md`

## Global Constraints

- Work directly on main under repository convention.
- Use existing Node/TypeScript lab dependencies if validation changes are necessary; do not add dependencies.
- No changes to accepted profile bytes/hashes, readiness attestations, registered observations, captured evidence, runtime semantics or the existing maintainer authorization are permitted.
- The selected profile is `rest-query-foundation`, API `65.0`.
- Follow `CLEAN_ROOM.md`; do not edit it or copy Salesforce prose/code.
- No product Go/database/UI changes, new reference collectors, credential discovery, Salesforce requests using private sessions, source-org changes or automatic communications belong here.
- A scoped section review is never a whole-document review.
- Current documentation can corroborate a documented expectation but cannot substitute silently for a release-pinned source.
- Do not refresh the archive cutoff or assert archive completeness without separately reviewing the archive and explaining the changed boundary.
- Do not reintroduce a long-duration expiry experiment or widen the existing waiver.
- Foundation contract freeze remains blocked by every unresolved required finding and Plan 10's missing accepted live pair.

## Review Focus

- A source exposing only a loading shell remains unavailable; validation rejects a reviewed record without a locator and explicit read coverage (Task 2).
- Unknown release/API/publication information stays null and does not prevent recording an owned blocked review (Task 2).
- A successful scoped review cannot imply whole-document, archive or live completion (Tasks 2 and 4).
- A historical finding superseded by a documentary update cannot silently stop blocking live reconciliation (Task 3).
- A proposed regression has no existing test file; validation distinguishes proposals from existing tests and rejects dangling existing-test references (Task 2).

## File Structure and Order

| File | Responsibility |
| --- | --- |
| `docs/compat/releases/index.yaml` | Preserve all existing entries; add the Plan 18 batch and scoped source/access records |
| `docs/compat/releases/changes.yaml` | Preserve history; add source-backed findings, review details and owned residual gaps |
| `compat/clients/node/src/release-review.ts` (new) | Review-batch types, pure structural/semantic checks and bounded local reference checks |
| `compat/clients/node/src/validate.ts` | Load the ledgers and invoke release-review validation in `validateRepository()` |
| `compat/clients/node/test/release-review.test.ts` (new) | Synthetic valid/invalid batch tests and repository integration |
| `compat/clients/node/test/foundation-assessment.test.ts` | Prove documentary additions retain existing live blockers |
| `docs/compat/plan-18-foundation-release-reconciliation.md` (new) | Review method, hashes, source boundary, twelve-row decision table, verification and handoff |
| `docs/compat/plan-5-contract-decisions.md`, `docs/compat/plan-10-foundation-decision.md` | Narrow documentary follow-up links without changing evidence gates |

Tasks run sequentially. Task 1 produces review notes outside the checkout; Task 2 establishes the format and validator; Task 3 publishes the real records; Task 4 verifies and documents the final package. No new CLI, source cache or general-purpose ingestion framework.

### Task 1: Review the bounded official-source batch

**Files:** Read the spec, `CLEAN_ROOM.md`, `docs/compat/sources.yaml`, both release ledgers, `docs/compat/contracts/rest-query-foundation.yaml`, `docs/compat/profiles/rest-query-foundation.yaml`, Plans 5/10 and `compat/clients/node/src/suites.ts`. Create private temporary review notes only; publish through Tasks 3/4.

**Interfaces:**
- Consumes: `suite("rest-query-foundation").contractIds` (the authoritative twelve-contract set).
- Produces: source attempts, independently written scoped findings, explicit gaps and proposed regression intent sufficient to populate Task 2's format.

- [x] **Step 1: Record execution base and protected bytes.** Record `git rev-parse HEAD` and SHA-256 of the foundation profile and contract file. Save a temporary hash manifest for all tracked files under `docs/compat/profiles/`, `docs/compat/observations/`, `compat/evidence/` if present, and `docs/compat/reference-readiness*.yaml`. Include any other tracked capture/evidence paths identified from the repository. Do not put private paths or source bytes in the guide. The comparison in Task 4 uses this manifest, not remembered hashes.

- [x] **Step 2: Verify the initial release candidates.** Attempt the Winter ’26 and Winter ’23 official archive entries, starting with URLs already recorded in `index.yaml` and the cursor source in `changes.yaml`. Verify release/API associations from retrieved official content; record unknown associations as null, and explain any corrected batch candidate. Use browsing for fresh source verification. If reading PDFs, invoke the PDF skill and inspect relevant pages. Never treat a loading shell or search result as source content.

- [x] **Step 3: Review relevant sections and linked sources.** Inspect table of contents/index plus API, query, security, limits and release-update sections. Follow official relevant REST/SOQL/API changelogs, enforced updates, deprecations and direct historical dependencies. Revisit the official guide/article URLs in the existing `accessChecks` where they support these concerns. Stop retrying a failed source unless a materially different official retrieval path is available. Record exact section/page/heading, access date, attempted paths, what was read/searched and what remains unread. Hash actual retrieved source bytes only when available and permitted; otherwise keep checksum null.

- [x] **Step 4: Prepare twelve contract dispositions.** Use the registered IDs: `query-response-shape`, `query-limit-above-batch`, `id-15-18`, `case-insensitive-names`, `case-insensitive-comparison`, `null-ordering`, `error-shape`, `unknown-sobject`, `query-batch-header`, `query-locator-lifecycle`, `rest-route-parity`, `duplicate-canonical-fields`. Each needs sources/findings or an owned gap, explicit documentary scope, live status and regression intent. For negative findings state the searched boundary, never universal unchanged behavior. Keep external-ID and customization discoveries as separate dependencies.

### Task 2: Define and validate additive review records

**Files:** Create `compat/clients/node/src/release-review.ts` and `compat/clients/node/test/release-review.test.ts`; modify `compat/clients/node/src/validate.ts`.

**Interfaces:**
- Produces `releaseReviewIssues(index: unknown, changes: unknown, contractIds: readonly string[]): string[]` (pure, deterministic sorted diagnostic strings).
- Produces `validateReleaseReviewRepository(root: string): Promise<string[]>` (reads both repository ledgers and the registered contract set; checks existing-test paths; no network/source-locator reads).
- `validateRepository()` appends these issues. Load through a dynamic import inside the validation function so readiness/capture imports and minimal fixtures do not eagerly require release assets.

Use the following additive format, exporting its TypeScript interfaces from `release-review.ts`. Keep existing ledger `schemaVersion: 1`, archive metadata, document entries, findings, blockers and dated access checks intact. Do not migrate old records to manufacture new review coverage.

- `index.reviewBatches`: array containing batch `id: plan-18-foundation`, `profile: rest-query-foundation`, `apiVersion: "65.0"`, `profileHash`, `contractHash` (64 lowercase hex), `completedAt` (calendar date or null), `status` (`partial` or `complete`), `scope: foundation-sections`, `owner`, `sources`, `contracts`.
- Each source: `id`, `url` (official HTTPS Salesforce URL), `kind` (`release-notes`, `current-guide`, `dated-article`, `changelog`, `release-update`), `release`, `apiVersion`, `publicationDate` (each string or null), `accessedAt`, `reviewer`, `outcome` (`reviewed`, `partial`, `unavailable`), `locator` (string or null), `coverage` (nonempty string), `attempts` (nonempty string array), `checksum` (null or `{sha256, representation}`), `gap` (null or `{owner, reason, nextAction}`). API versions when known use `digits.0`; dates are calendar-valid `YYYY-MM-DD`. A reviewed source requires a locator; partial/unavailable sources require a gap. Unavailable sources cannot have a checksum or imply content read in their coverage.
- Each contract disposition: `contractId`, `sourceIds`, `findingIds`, `documentaryStatus` (`reviewed`, `partial`, `blocked`), `expectation` (independent summary or specific unresolved question), `coverage`, `gaps` (owned reason/nextAction records), `liveStatus` (`blocked` or `assumed`), `blocksContractFreeze: true`, `owner`, `nextAction`, `regressions`.
- Each regression: `id`, `status` (`existing` or `proposed`), `path` (repository-relative file or null), `testName` (string or null), `operation`, `assertion`, `evidencePrerequisite`. Existing tests need path and testName; proposed tests use null path/testName and remain unimplemented. Verify existing test files resolve under `compat/clients/node/test/`, `internal/` or `web/test/`, without traversal, absolute paths or symlink components. A reviewer verifies the named test's meaning; file existence alone is insufficient.
- New findings/blockers in `changes.yaml` keep the assessor-consumed fields `id`, `status`, `affectedContracts`, `blocksContractFreeze` and add `reviewDetail`: `{batchId, sourceIds, introducedApiVersion, effectiveApiVersion, activation, applicability, rationale, sourceStatement, inference, unresolvedQuestions, deprecation, migrationImplications, regressionIds, supersedes}`. Unknown versions/deprecation/migration effects may be null; activation is `opt-in|enforced|default|unknown`, applicability is `in-profile|out-of-profile|unknown`; unresolved questions carry owner/nextAction. `supersedes` references existing finding/blocker IDs, preserves originals, rejects self/cycles, and does not itself change assessor resolution. Attach reviewDetail to an old finding only when actually revisited; preserve its earlier fields/history.

- [x] **Step 1: Write failing synthetic tests.** Tests call the exact pure signature above. A complete twelve-contract batch with owned unavailable sources and unknown versions/dates returns `[]`; complete means review work recorded, not all sources accessible. A valid reviewed section also returns `[]` while retaining `scope: foundation-sections` and live blockers. Each malformed mutation must yield at least one issue: duplicate source/finding/batch IDs; missing/extra/duplicate contract row; dangling source/finding/regression/supersession reference; missing locator; reviewed source with empty coverage; unknown enum; malformed date/hash; partial source without owner; unavailable source with fabricated checksum; `status: complete` without completion date; a documentary reviewed row with unresolved documentary gaps; `liveStatus: observed`; `blocksContractFreeze: false`; self/cyclic supersession. An owned blocked row remains valid in a completed batch. Test unknown null values separately from malformed strings.

- [x] **Step 2: Verify failure.** From `compat/clients/node`, run `node --import tsx --test test/release-review.test.ts`. Expect failure for the missing module/exports.

- [x] **Step 3: Implement the pure validator and filesystem wrapper.** Guard unknown inputs before accessing properties; validate the batch-specific additions without retroactively requiring new fields on historical entries. Cross-reference IDs against combined findings/blockers and source IDs within the referenced batch. Require each contract row to cite a source/finding or have an owned gap, and at least one precise regression intent. A reviewed row needs reviewed sources and no documentary gaps. Full-M1R blockers outside this batch remain legitimate. No source URL is fetched, nor is a locator opened. Reject official-lookalike hosts; allow `salesforce.com` and its actual subdomains only. Return sorted issues rather than throwing parser/OS errors through repository validation.

- [x] **Step 4: Integrate and exercise local references.** Add temporary-root tests for valid proposed regressions with no file; existing test with a real file; missing file; traversal/absolute/symlink path; unavailable/malformed ledgers. Prove a source locator naming a nonexistent local file causes no open and a URL causes no fetch. Use temporary-root ledgers for this task’s integration tests. In Task 3 add a repository integration test that loads the real ledgers and requires the batch to exist; the wrapper itself must already report an absent reviewBatches as an issue (never silently skip it). The ordinary repository validate command may therefore report that missing batch until Task 3 publishes it; do not claim the intermediate commit passes the full lab check. Keep ordinary validation as the sole command entry point.

- [x] **Step 5: Run focused checks and commit.** Run the focused command, `npm run typecheck` and Prettier on changed TS files, then `npm run format:check`. Expect all passing. Commit only the validator, its tests and integration as `feat: validate scoped foundation release review records`.

### Task 3: Publish actual review results without releasing live gates

**Files:** Modify `docs/compat/releases/index.yaml`, `docs/compat/releases/changes.yaml`, `compat/clients/node/test/foundation-assessment.test.ts`.

**Interfaces:** Consumes Task 1 notes and Task 2 format. Preserves `foundationReleaseFindings(document: unknown): ReleaseFinding[]` in `src/foundation-assessment-command.ts`; no new resolution status or production assessor change.

- [x] **Step 1: Pin assessor compatibility with tests.** Add a test named `documentary review details and supersession preserve live release blockers`. Feed synthetic findings with reviewDetail, documentary completion and supersession into `foundationReleaseFindings`. Assert a relevant `reviewed-source-awaiting-live-reconciliation` finding remains `{relevant: true, resolved: false}`; `resolved` and `reviewed-and-reconciled` retain existing behavior; a full-M1R-only blocker remains irrelevant to this bounded assessor. Assert documentary completion alone supplies neither resolving status. These are characterization tests and may pass immediately; do not change production code merely to produce a red test.

- [x] **Step 2: Populate the batch and findings from actual reviewed content.** Record all attempts, source locators, correct associations/unknowns, exact-byte hash representations, source kind and search boundaries. Populate all twelve dispositions; preserve blocked sources and unreviewed historical intervals. Record activation/version ambiguity and negative findings conservatively. Add reviewDetail only for findings actually reviewed. Every in-profile finding gets precise regression IDs; out-of-profile classifications need an evidence-backed rationale. Do not invent documentary findings to fill rows: a specific owned unknown is valid.

- [x] **Step 3: Reconcile the named blockers explicitly.** Preserve `winter23-query-cursor-retention` and its history. Add a dated clarification in reviewDetail explaining that its earlier long-duration resolution text is historical and Plan 10's existing waiver controls; retain unknown expiry boundary/anchor/eviction/retrieval effects and live blocking. Retain `release-content-unavailable` unchanged unless actual coverage justifies narrowing it. If narrowed, retain its original record and add explicit residual records/supersession links covering every unreviewed affected contract; no prior live obligation disappears. Preserve `linked-changelog-expansion` as full-M1R blocking. Do not set findings to `resolved` or `reviewed-and-reconciled` on documentary review alone.

- [x] **Step 4: Verify and commit the ledger result.** Run `node --import tsx --test test/release-review.test.ts test/foundation-assessment.test.ts` from the Node lab and `npm run validate`. Expect passing offline checks, all twelve rows, and unresolved live release issues in assessor tests. Manually check every summary against its actual source and every existing-test mapping against its assertions. Commit as `docs: reconcile bounded foundation release sources`.

### Task 4: Publish the decision guide and verify the complete package

**Files:** Create `docs/compat/plan-18-foundation-release-reconciliation.md`; modify Plans 5/10 only for justified dated documentary follow-ups. Update new review tests only if final integration reveals a real gap.

**Interfaces:** Consumes validated batch and final reviewed findings. Produces the operator-facing decision/handoff, actual verification evidence and preserved-byte comparison.

- [x] **Step 1: Write the guide and minimal cross-links.** Include execution base, profile/contract hashes, actual source dates and corrected release associations, method/search scope, retrieved/unavailable source inventory, all twelve contract rows, source/finding IDs, documentary gaps, live blockers, proposed versus existing regressions and owner/next action. State archive cutoff/history remains unchanged and this is not full M1R. Explain the two-day assumption and all unmeasured dimensions without claiming an observation. Record meaningful negative results and the wholly blocked outcome if appropriate. Add dated links from Plans 5/10 that qualify their historical missing-document statements without rewriting old evidence. End with residual source actions, Plan 10 evidence prerequisites, later M1R batches and separate runtime decisions. Do not change estimates.

- [x] **Step 2: Check scope and evidence manually.** Compare the guide's twelve rows to `suite("rest-query-foundation").contractIds`, and each cited regression to its actual test or clearly labeled proposal. Verify every local link resolves and every official locator supports its summary. Recheck preserved archive entries, original finding fields/access history and all named unresolved blockers. Ensure no source bytes, credentials, private paths or copied Salesforce prose were committed.

- [x] **Step 3: Run final verification on frozen source.** Run `npm --prefix compat/clients/node run check` and `git diff --check`; record exact totals/skips and limitations rather than assumed counts. Compare the protected-byte manifest from Task 1 and confirm no profile, readiness or historical evidence changes. Inspect `git diff --name-only <execution-base>` for unexpected product/dependency/policy changes. No database/browser/live-reference rerun is required. After any further code/ledger change rerun the affected checks before publishing success.

- [x] **Step 4: Obtain fresh whole-change review.** Ask a fresh reviewer to inspect implementation against this plan/spec, especially accidental documentary-to-live promotion, historical coverage loss, malformed/contradictory records, ignored residual gaps and weak source/regression mappings. Correct substantive findings and rerun affected checks. Record the review result and actual verification in the guide; do not mark implementation complete while substantive findings remain.

- [x] **Step 5: Commit the final handoff.** Commit the guide and justified status links as `docs: publish foundation release review decisions and handoff`. Report the bounded batch outcome, actual remaining blockers, checks and any source-access limits. No contract freeze, certification or completed full milestone follows.

## Plan Self-Review

The four tasks cover source selection/retrieval, provenance and bounded coverage, additive ledger validation, all twelve contract dispositions, historical/supersession handling, regression intent, protected evidence, fresh review and operator handoff. Review Focus cases map to Task 2's malformed-record/path tests, Task 3's assessor characterization, and Task 4's source/coverage checks. Actual documentary conclusions remain execution results, not values fabricated in this plan. No runtime implementation or live experiment is authorized here.

## Execution Record

Implemented under maintainer-approved native execution on main, 2026-10-21. The [guide](../../compat/plan-18-foundation-release-reconciliation.md) records actual source-access limits, twelve owned dispositions, review correction and final verification. Historical release reconciliation remains blocked; the bounded review batch is complete.
