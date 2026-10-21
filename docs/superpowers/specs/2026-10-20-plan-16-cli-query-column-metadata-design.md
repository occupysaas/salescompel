# Plan 16 — CLI query column metadata

Status: approved for implementation by the maintainer; implemented, reviewed and verified with two full acceptance passes, 2026-10-21 UTC. See the [contract and evidence record](../../compat/plan-16-cli-query-column-metadata.md).
Milestones: bounded M3 query-response contract supporting the open M2 candidate acceptance gate.

## Intent and selection

Unblock the specific native CLI failure recorded by [Plan 15](../../compat/plan-15-candidate-acceptance.md). Both final full acceptance runs failed the required nonempty CLI query: pinned plugin-data 5.1.16 requests `columns=true`, but the candidate returns ordinary query data without `columnMetadata`. The client reports `UnexpectedValueTypeError: Value is not a JsonArray`. Empty results bypass the failing path and are not an acceptable substitute.

The executor selects this next package because it has a concrete reproducer and directly blocks consolidated acceptance. Establish the bounded response contract before implementing it. Reuse Plan 15's harness for acceptance; this package does not replace that harness or renumber its unfinished gates. Plan 10/14 reference work and Plan 13 visual acceptance remain independent.

Success is a documented, justified column-metadata contract for the existing Account/Contact query subset, a narrow implementation with real-client regressions, and two fresh full acceptance passes. If required contract evidence or execution prerequisites are unavailable, deliver the completed useful work and an explicit blocked decision instead of claiming acceptance.

## Existing implementation and constraints

Start from `internal/platform/http.go` (query route), `query.go` (parser), `select.go` (authorized query execution), `catalog.go`/`metadata.go` (field declarations), their existing Go tests, and the native CLI stage under `compat/clients/node/src/candidate-acceptance-*`. Inspect the actual pinned dependency installed from the existing lockfile before selecting response properties. Plan 15's diagnostic is the starting hypothesis; reproduce it before changing runtime behavior.

Use API 65.0, catalog version 2, Go module baseline 1.25.0, PostgreSQL 17, Node >=22.12 and Python >=3.12. Preserve JSforce 3.11.0, Salesforce CLI 2.156.0, plugin-data 5.1.16, plugin-deploy-retrieve 4.2.15, simple-salesforce 1.12.9 and existing Vue/Playwright pins. No dependency upgrades, schema/catalog migration or new runtime dependencies.

Work directly on `main` under the repository convention. This document authorizes no implementation by itself; the implementation plan follows spec review.

## Contract decision gate

First reproduce the current nonempty query failure against an owned disposable candidate service using the existing pinned CLI invocation. Record the selected query, relevant request parameters, exit/error classification and sanitized response shape. Keep tokens, connection strings, CLI auth state and raw diagnostic logs private.

Inspect the pinned client's consuming code and available official API material. Record exact source/version references and distinguish three facts: what the client needs, what authoritative material specifies, and what has actually been observed on the reference service. Installed client code can establish a consumption requirement, but cannot alone establish universal Salesforce behavior.

Before product changes, publish a decision table in `docs/compat/plan-16-cli-query-column-metadata.md` covering:

- Top-level property presence and type for ordinary queries and `columns=true`.
- Each required metadata member, exact JSON type and value mapping for the supported field families.
- Projection ordering, canonical field/object spelling and whether metadata depends on result-row presence.
- Empty results, null field values, selected fields in a different order and a supported field selected more than once.
- Account and Contact fields already supported by the parser/catalog, including IDs, relationship IDs and custom text fields.
- The treatment of absent, explicit false, malformed and repeated `columns` parameters.
- Unsupported projections and denied objects/fields, preserving the existing authorization boundary.

For each row, cite its basis and mark it specified, observed, a deliberately bounded candidate decision, or unresolved. Do not infer field labels, type names, precision, nested structures or unsupported query features merely from the exception. Exact wire shapes belong in this decision record and its fixtures, rather than guesses in this spec.

Official material plus pinned-client requirements may justify an explicitly labeled candidate interoperability contract without live reference access. If a material wire decision remains ambiguous, implement offline probe/assessment support and report that row blocked; do not silently choose reference semantics. Any new live reference collection requires existing explicit disposable-tenant authorization, identity binding, recovery prerequisites and exact fixture cleanup. This spec grants no new external write authorization. Do not resume Plan 10/14 or broaden the accepted eleven-scenario baseline to obtain this narrow answer.

## Required candidate behavior after the gate

For the supported query subset, the metadata-enabled response must satisfy the frozen decision table and let the unchanged pinned CLI return the actual owned records. Preserve ordinary query response behavior when metadata was not requested, unless the decision record establishes and explicitly justifies a change.

Derive metadata from the validated, authorized projection and existing catalog, not from the first returned row or a separate hand-maintained field dictionary. Empty results and null values must have the contractually correct metadata. Preserve selected-column ordering and supported spelling behavior as frozen by the decision. Do not expand SELECT syntax to accommodate unrelated aggregates, aliases, relationship traversal, subqueries or tooling queries.

Apply existing object/field authorization before emitting records or metadata. A denied selection must not expose field declarations through metadata, return a partial successful projection or bypass tenant isolation. Invalid query behavior must remain consistent with existing candidate rules; metadata processing must not turn invalid selections into server errors.

Keep existing record values, `attributes`, `totalSize` and `done` semantics. No pagination, cursor, equality, null ordering, external-ID normalization, authentication or localization redesign belongs here. Update any applicable machine-readable candidate contract/example only after checking that it accurately represents the served route; do not advertise a broader supported protocol.

## Verification and acceptance

Add a failing-then-passing regression for the actual missing-metadata failure. Use focused Go HTTP/database tests for the frozen response shape and existing real native-client infrastructure for interoperability. A unit fixture accepted by a mocked client cannot substitute for the actual pinned CLI.

The test matrix covers Account and Contact; nonempty and empty results; null and custom text values; projection ordering and the decided duplicate-selection behavior; absent/false/invalid/repeated parameters; unauthorized objects/fields and another tenant's records. Ordinary queries retain their existing response contract. Compare meaningful metadata values and returned record identity/value, not just exit status or the presence of an array.

Keep Plan 15's required owned-record query, expected test identities, exit handling, source/result bindings, cleanup and no-skip policy intact. Do not switch to an empty query, remove `columns=true`, upgrade the client, inject a fake result or continue failed gates to manufacture a pass. Add an empty-query CLI regression as a separate case if needed; it never replaces the nonempty one.

Run focused regressions, the existing offline lab check and applicable Go checks while developing. Then use `scripts/check-candidate-acceptance.sh --out /absolute/new-summary.json` for two complete sequential fresh runs against the same explicitly disposable PostgreSQL database, following Plan 15's setup and lifecycle rules. Setup/install steps remain separate from acceptance. Require clean committed source, distinct run IDs/outputs, successful fixture disposition and stopped owned processes between runs. Each passing invocation must execute all required database, lab, web, real-client, restart, connected-browser, preview and cleanup gates. Preserve failed results and do not overwrite historical Plan 15 reports.

If a run fails, classify the actual assertion or prerequisite and fix only defects within this contract or already specified candidate behavior. Do not decide unrelated reference semantics to force acceptance. Missing browsers/tools/database are blockers; unexecuted later gates stay explicitly unexecuted. CI availability and an observed CI run are separate claims.

## Deliverables and reporting

Deliver the contract decision record with provenance, bounded implementation and behavior regressions if the gate permits them, applicable setup/contract documentation updates, and sanitized verification evidence. Record actual commands, versions, source/binary/asset/report digests, test/skips, fixture disposition and both full-run outcomes in the Plan 16 record; refer to Plan 15's report schema rather than inventing another format.

Update Plan 15's decision/status and the Plan 2 setup guide only to reflect evidence actually produced. Preserve its historical failed attempts. Two full passes establish the tested candidate slice, not universal Salesforce parity or full M1/M2 milestone completion. Keep wider roadmap edits limited to this changed status.

Run `git diff --check`, obtain fresh whole-change review and resolve Important/Critical findings before declaring implementation complete. A reference blocker can leave runtime work pending; a product fix without two complete passing runs leaves consolidated acceptance pending. State these separately.

## Calls left to the builder

The executor chose this blocker as the next package and recommends extending the existing HTTP/query/catalog and acceptance modules. The builder chooses focused helper boundaries, test names and the smallest evidence/probe mechanism. Exact metadata wire properties must come from the contract decision gate. No general CLI framework, new metadata engine, UI redesign, cloud resources or unrelated query features are selected.

## Builder check

A fresh software-engineer reviewer read the specification and reported “nothing important left.” Wire-shape decisions remain explicitly assigned to the evidence-based contract gate. The subsequent contract decision, implementation and acceptance evidence are recorded in the linked Plan 16 execution record.
