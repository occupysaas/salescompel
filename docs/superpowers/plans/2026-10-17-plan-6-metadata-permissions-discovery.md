# Plan 6 — Metadata and Permissions Discovery Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans for the preserved native sequential execution preference. Steps use checkbox syntax. This is a plan, not completed evidence.

**Goal:** Produce a bounded read-only metadata/persona inventory and a reviewed next implementation slice while Plan 5 waits for evidence.

**Architecture:** Add an explicitly separate discovery command to the existing lab. Public describe/Metadata reads and isolated CLI retrieval feed a private inventory; a deterministic compiler emits a capability matrix without inferring effective grants. Existing compatibility suites and product code remain unchanged.

**Tech Stack:** Existing TypeScript/Node, pinned JSforce and Salesforce CLI, existing Ajv/YAML utilities; future product remains Go/PostgreSQL.

**Spec:** [Metadata and permissions discovery design](../specs/2026-10-17-plan-6-metadata-permissions-discovery-design.md).

## Global constraints

- Preserve Salesforce HTTP paths, methods, versions and native client commands; no custom routes, rewriting proxies or client patches.
- Pin discovery to API 65.0 and the existing locked clients; no dependency upgrades.
- Follow CLEAN_ROOM.md: public interfaces only; never inspect CLI/plugin source.
- No reference writes: no deploy, create/update/delete, assignments, settings changes, record extraction or queries. Metadata transport may legitimately use HTTP POST; enforce read-only operations at the API/CLI operation level.
- Existing named-tenant authorization applies. Verify both persona identities and same organization before collection; no fresh same-admin session is required.
- Keep tokens, raw metadata, component fullNames beyond synthetic fixtures, and user/org identifiers outside the checkout in owner-only files/directories.
- Preserve pilot and foundation profile bytes, accepted observations and report semantics. This inventory is not a new passing compatibility suite.
- Absence, explicit false, unavailable, unsupported, unrequested and conflicting evidence must remain distinguishable. Never turn failed reads into empty success.
- Keep every product file under internal/platform and all product routes unchanged.
- Work on the existing branch under the recorded maintainer policy. Do not start product implementation as part of this plan.

## Review focus

1. Metadata read uses POST: reject mutation operations without accidentally rejecting legitimate reads — Task 2.
2. Describe omits a field: preserve unknown visibility without declaring effective denial — Tasks 1, 4.
3. A profile name contains path/shell characters: pass it as data, reject filesystem escapes, never interpolate shell commands — Tasks 1, 3.
4. A retrieval partially succeeds or times out: retain component-level gaps and clean isolated auth files — Task 3.
5. A persona changes organizations or credentials during a run: refuse mixed-identity evidence and preserve Plan 5's saved sessions — Tasks 2, 5.

## File and interface map

All src/test paths below are beneath `compat/clients/node/`.

| Files | Responsibility |
| --- | --- |
| src/metadata-inventory.ts, test/metadata-inventory.test.ts | Strict configuration/inventory types and validation |
| compat/schemas/metadata-inventory.schema.json | Versioned artifact schema |
| src/metadata-discovery.ts, test/metadata-discovery.test.ts | Identity verification, read-only describe/Metadata collection |
| src/metadata-retrieve.ts, test/metadata-retrieve.test.ts | Exact synthetic CLI retrieval and private manifest |
| src/metadata-matrix.ts, test/metadata-matrix.test.ts | Lossless state classification and stability comparison |
| src/cli.ts, src/validate.ts, test/metadata-cli.test.ts | Separate command, schema registration and safe output handling |
| docs/compat/plan-6-metadata-discovery.md | Sanitized results, gaps and next-slice recommendation |
| docs/compat/README.md, implementation-log.md | Usage and actual verification evidence |

## Task 1: Strict scope and private inventory format

**Interfaces:** Export `DiscoveryConfig`, `Inventory`, `Fact`, `Gap` from metadata-inventory.ts. `parseDiscoveryConfig(value: unknown): DiscoveryConfig`; `validateInventory(value: unknown): asserts value is Inventory`.

Config: schemaVersion 1, apiVersion '65.0', profile 'rest-query-foundation', profileHash, HTTPS origin, and optional exact `profiles: string[]` / `permissionSets: string[]` (defaults []). No tokens or arbitrary operations. Fixed objects/fields and synthetic CustomField/PermissionSet members come from code, not wildcard input. Optional names must be nonempty, at most 255 characters, contain no control characters, and be bounded to 20 entries per type. Never interpret them as paths.

Inventory: schemaVersion 1, kind 'metadata-discovery', scope, profileHash, connectors, startedAt/finishedAt, identityVerified booleans, operations, facts, gaps and outcome 'complete'|'incomplete'|'blocked'. Facts retain source operation ID, persona alias, component identity, property and JSON scalar/array value. Gaps retain reason codes, affected scope and owner. Do not embed credentials or identity IDs; raw files remain separately private.

- [ ] Add tests `rejectSecretsWildcardsAndUnknownKeys`, `rejectProfileHashAndApiMismatch`, `preserveFalseVersusMissing`, and `rejectMalformedInventory`. Assert false remains an observed value; missing property creates a gap; arbitrary commands and wildcard members are rejected.
- [ ] Run `cd compat/clients/node && node --import tsx --test test/metadata-inventory.test.ts`; expect new tests to fail before implementation.
- [ ] Implement strict validation with existing Ajv and profile/toolchain loaders. Add `additionalProperties: false` throughout the schema and finite input/output collection bounds (20 optional components per type, 8 MiB raw operation response). Include Task 1 schema validation in repository validation without adding compatibility contracts or promoting observations.
- [ ] Run the targeted tests and `npm --prefix compat/clients/node run check`; expect pass. Commit the inventory format and tests.

## Task 2: Read-only persona and declared-metadata collection

**Interfaces:** `collectMetadata(config: DiscoveryConfig, env: NodeJS.ProcessEnv): Promise<Inventory>` in metadata-discovery.ts. Consumes Task 1 types. Credentials are `COMPAT_REFERENCE_TOKEN` and `COMPAT_REFERENCE_RESTRICTED_TOKEN`; origin must match `COMPAT_REFERENCE_URL`. Use the existing API version/profile bindings.

- [ ] Add local transport tests `verifyBothIdentitiesBeforeReads`, `rejectCrossOrgAndSamePersona`, `collectOnlyFixedObjectsAndMembers`, `metadataPostIsReadOnly`, `preserveUnavailableMetadata`, `redactAuthAndProse`. Prove zero discovery reads after identity failure, zero mutation operations, and preserved missing/false distinctions.
- [ ] Run metadata-discovery tests; expect failures before implementation.
- [ ] Implement public userinfo identity checks with existing bounded wire transport, then JSforce describe for Account/Contact per persona and administrator Metadata read for the exact synthetic components plus configured optional components. Verify documented public client signatures at execution time; do not inspect client implementation. Allow only these read operations. Project describe into the fixed field/property scope, never persist unrelated record data. Distinguish missing component from access/transport failure when the response supports that distinction; otherwise record unavailable.
- [ ] Bound JSforce collection with a 60-second operation deadline; do not retry a failed operation indefinitely or turn it into success. Verify identities again before finalizing; any changed identity blocks the whole inventory. Raw error prose stays private; public inventory uses stable codes and source references.
- [ ] Run targeted tests and full Node check; expect pass. Commit collector and tests. No live collection until Tasks 3–4 can preserve and interpret the whole result.

## Task 3: Isolated native CLI retrieval

**Interfaces:** `retrieveSyntheticMetadata(config: DiscoveryConfig, env: NodeJS.ProcessEnv): Promise<RetrievalResult>` in metadata-retrieve.ts. Export `RetrievalResult` with outcome, member results, relative artifact paths, byte counts, SHA-256 values and gaps. Do not return credential material. Caller merges component facts into Task 1 Inventory.

- [ ] Add tests `exactRetrieveManifest`, `rejectTraversalAndSymlinkOutputs`, `partialRetrieveIsIncomplete`, `timeoutRemovesAuthHome`, `neverMutateSharedAliases`. Verify only the two synthetic fields and Compat_Admin are requested; optional real profiles are not included in the CLI retrieval.
- [ ] Run metadata-retrieve tests; expect failures before implementation.
- [ ] Reuse the existing isolated `withCli` authentication helper where its Context requirements can be satisfied without fixtures. Use a fresh temporary project and exact package manifest at API 65.0, then documented `project retrieve start` flags. Verify the pinned CLI help before choosing flags. Do not invoke the existing `metadata()` scenario: it deploys/deletes fixtures. No changes to the user's persistent CLI home or saved session files.
- [ ] Verify retrieved members and retain only allowlisted relative files under a validated external artifact directory. Refuse symlinks, traversal, extra members and files over 8 MiB; cap aggregate retained bytes at 32 MiB. Treat missing/partial retrieval as incomplete. Remove temporary project/auth state in finally. Existing helper's six-minute process timeout is the bound, not an API compatibility claim.
- [ ] Run targeted tests and full Node check; expect pass. Commit retrieval and tests.

## Task 4: Matrix compiler, stability comparison and command

**Interfaces:** `compileMetadataMatrix(inventory: Inventory): Matrix`; `compareMetadataInventories(a: Inventory, b: Inventory): { outcome: 'stable'|'drift'|'blocked'; differences: string[] }` in metadata-matrix.ts. Matrix rows have area, componentAlias, persona, property, state, value?, sourceRefs and nextAction. State is observed|unavailable|unsupported|unrequested|conflict.

Add `discover-metadata --config <private.json> --out <private.json>` to cli.ts. Use existing credential environment variables; do not add it to SuiteId, capture or compatibility compare. Add `--config` option without changing existing commands. Inventory and raw artifact directory are create-exclusive, outside checkout, mode 0600/0700; check resolved parents and symlink escapes before network access.

- [ ] Add tests `falseIsNotDeniedOrMissing`, `missingDescribeDoesNotProveEnforcement`, `declaredGrantDoesNotProveEffectiveAccess`, `differentFactsProduceConflict`, `enumerationOrderDoesNotProduceDrift`, `scopeOrToolchainMismatchBlocks`, and CLI `unsafeOutputFailsBeforeTransport`. Assert conflicting sources survive as separate source-backed facts. Conflict requires the same fact scope; a declared grant and persona visibility are different fact scopes, not inherently a conflict.
- [ ] Run matrix/CLI tests; expect failures before implementation.
- [ ] Implement matrix and comparison. Sort component/property identities only; never normalize values or erase gaps. Reject different scope/profile/API/toolchain and incomplete required operations as a successful stability comparison. Optional unsupplied profile names remain unrequested; they do not masquerade as full security coverage.
- [ ] Implement command wiring, merging read and CLI evidence with explicit sources. CLI exit: 0 complete, 2 incomplete/blocked, 1 internal failure. Console output contains only counts, status and private output path. Produce no automatic repository summary containing tenant metadata.
- [ ] Run full Node check and `git diff --check`; expect pass. Commit command/matrix and tests.

## Task 5: Two collections, review and next-slice recommendation

**Files:** Private artifacts outside checkout; docs/compat/plan-6-metadata-discovery.md, README.md and implementation-log.md.

- [ ] Read existing authorization and recheck credentials privately. Record current profile/toolchain binding and optional scope; do not refresh or overwrite Plan 5's preserved session files. No new same-admin login is required for this plan.
- [ ] Prepare private config from the registered profile hash and named reference origin. Run sequentially:

```sh
cd compat/clients/node
node --import tsx src/cli.ts discover-metadata --config "$DISCOVERY_CONFIG" --out "$DISCOVERY_RUN_DIR/first.json"
node --import tsx src/cli.ts discover-metadata --config "$DISCOVERY_CONFIG" --out "$DISCOVERY_RUN_DIR/second.json"
```

Expected: bounded inventories, no reference mutations, truthful completion or per-operation blockers. Do not invent settings or fabricate evidence to force exit 0.

- [ ] Run compileMetadataMatrix and compareMetadataInventories through a private local driver using the exported interfaces. Require identical bindings and explain any drift before accepting inventory stability. Verify temporary CLI projects/auth homes were removed. This is not the pilot/foundation capture-pair verifier and cannot promote either suite.
- [ ] Review facts against the actual candidate catalog and existing synthetic fixture XML. Publish synthetic-name/persona-alias summary, source/digest/time bindings, capability gaps and unsupported items. Keep raw customer metadata and prose private. Record relevant official source provenance only for claims actually reviewed; do not imply complete M1R coverage.
- [ ] Recommend the smallest evidenced metadata/grant representation slice, with acceptance examples and explicit Plan 5 dependencies: canonical resolution, query permission checks, ID handling and HTTP errors. Product implementation requires its own design/plan; no speculative product code in this task.
- [ ] Run `npm --prefix compat/clients/node run check` and `git diff --check`; expect pass. Confirm pilot/foundation profile hashes unchanged. Commit reviewed documentation, or precise blockers if live discovery is incomplete. Do not claim M4 completion.

## Self-review and handoff

Tasks 1–4 deliver independently testable read-only lab tooling; Task 5 supplies the
actual bounded inventory and reviewed recommendation. All five review-focus cases
have named tests. No task consumes Plan 5 cursor semantics or edits product routes.
Native sequential execution is preserved. This document completes planning only;
Plan 5's pending evidence and prepared expiry follow-up remain separate work.
