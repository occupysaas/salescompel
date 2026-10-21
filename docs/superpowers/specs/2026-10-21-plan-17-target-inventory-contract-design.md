# Plan 17 — Target inventory contract

Status: draft for maintainer review, 2026-10-21. No implementation authorized.
Milestone: bounded M0 inventory foundation; the full M0 exit gate remains open.

## Intent and selection

Establish a reviewable answer to what a named migration target requires, where that requirement came from and who must resolve each unknown. The roadmap requires a target inventory before the program can make defensible coverage and certification commitments. Existing pilot/foundation profiles describe bounded laboratory fixtures; neither is a customer requirements inventory.

The executor selects this package after [Plan 16's candidate acceptance](../../compat/plan-16-cli-query-column-metadata.md). [Plan 10](../../compat/plan-10-foundation-decision.md) and [Plan 14](../../compat/plan-14-external-id-normalization-contract.md) retain their reference prerequisites and should resume under their existing numbers. Another collector would not resolve those prerequisites. Runtime external-ID normalization remains gated by Plan 14. This independent package follows the roadmap's target-inventory priority without selecting unsupported runtime semantics.

Success is a versioned inventory format, offline validation, a clearly synthetic worked example and an actionable gap report. The result enables a later operator-led inventory; it does not invent a real customer's requirements or complete M0. Scope selection and the format choices below are executor recommendations for review.

## Existing assets and boundaries

Build on the [original roadmap](../../plans/2026-10-14-salesforce-compatibility-roadmap.md), [remaining roadmap](../../plans/2026-10-20-progress-and-remaining-roadmap.md), `docs/compat/profiles/pilot.yaml`, `docs/compat/profiles/rest-query-foundation.yaml`, `compat/schemas/profile.schema.json`, existing contracts and `locales/languages.json`. Inspect their actual structures during planning. Keep accepted profile bytes, hashes, reference-readiness attestations and historical evidence unchanged.

Reuse the Node/TypeScript laboratory, its pinned Ajv/YAML dependencies and existing CLI conventions. No new runtime dependencies, Go/database/catalog changes, product APIs, UI screens, network collection, credential discovery or package installation belong here. Work directly on main under repository convention. Public example data must be independently authored and synthetic.

## Deliverables

Provide a JSON Schema under `compat/schemas/target-inventory.schema.json`, a synthetic YAML example under `docs/compat/inventories/`, an offline `validate-target-inventory` command in the existing lab CLI, and `docs/compat/plan-17-target-inventory.md` explaining authoring, validation, gaps and the later handoff. Integrate validation of the checked-in example into the ordinary lab validation entry point. This specification does not require creating an implementation plan in the same change.

The example describes the existing synthetic Account/Contact slice and explicitly unknown wider target dimensions. It is labeled synthetic in machine-readable metadata and prose. Current candidate support and laboratory observations can be cited as those facts only; they cannot establish customer demand, source-org feature absence or full compatibility. Do not mark an empty list as verified absence.

## Inventory model

Use schema version 1, a stable inventory ID, a synthetic/real classification, revision/date, target label and accountable inventory owner. Real names, tenant identities, endpoints and source documents remain private; the public fixture uses synthetic labels. A source register contains stable IDs, source kind, dated provenance and a nonsecret locator. References are local register IDs, not instructions to fetch URLs or files.

Every required inventory section must be present. Represent unknown values explicitly rather than inventing defaults. For each section record collection status (`unknown`, `partial`, `complete`), responsible owner, evidence references and unresolved questions with owner and next action. A complete empty section requires an explicit evidence-backed absence attestation. A complete section with unknown items or unresolved questions is contradictory and rejected.

Every item has a stable ID, category, requirement disposition (`required`, `optional`, `out-of-scope`, `unknown`), rationale, accountable contract owner, provenance references and acceptance intent. Unknown requirement disposition has a question and next action; out-of-scope has an explicit decision rationale. Required items identify the behavior/workflow to preserve and the intended acceptance check. They need not have an implemented test yet. Keep requirement, observation, implementation and verification as separate facts; an implementation link never establishes observed parity.

Cover these sections:

| Section | Required content when known |
| --- | --- |
| Source and org features | Edition/release, exact API versions, enabled org-wide features including Person Accounts, currencies and legacy automation; explicit unknowns |
| Data and metadata | Objects, field families, relationships, record types, layouts, volumes, history/files/audit needs, and unsupported export areas |
| Workflows and users | Named user journeys, personas, required outcomes and permission boundaries |
| Clients and integrations | Exact client versions, endpoints/protocol families, authentication methods and downstream dependencies; no secrets |
| CLI and plugins | CLI version/channel, exact plugin version/source/signature status/license, commands/flags/output formats actually required, including non-org tools classified explicitly |
| Automation and customization | Apex/class API versions, Flow, formulas, validation, legacy rules, imported UI and package/version/dependency requirements |
| Security | Profiles/permission sets, sharing and org security settings, with required denied-access behavior |
| Localization | Wire language, locale, currency, timezone, support category and fallback requirements; selectable language does not imply complete translation |
| Migration and operations | Import/export paths, identity mapping, reconciliation expectations, scale/workload, downtime, recovery and cutover objectives |

Section details may use typed category-specific records, but not an unrestricted arbitrary payload that bypasses meaningful validation. Unknown numbers/versions must remain unknown, not zero or a fabricated current release. Operating objectives record value/unit or a specific unresolved decision. The builder chooses reusable schema definitions and field names while preserving these distinctions.

## References and coverage mapping

Inventory IDs, source IDs and item IDs are unique within their respective namespaces. Reject dangling local references. A required item maps to one primary roadmap milestone/contract owner and may list dependencies; this is accountability, not a milestone pass. Existing contract/profile references use repository-relative paths and explicit IDs where applicable. Resolve only approved repository references during repository validation; reject traversal and never follow arbitrary external paths or network locations.

Do not copy laboratory profiles into a new presumed production profile or expand the accepted compatibility surface. A subsequent reviewed decision translates a completed real inventory into target profiles, new contracts and bounded feasibility work. This package may identify missing coverage but cannot authorize it.

## Validation and reporting

The new command takes an explicit input file and operates offline. Reuse existing bounded YAML parsing where available; reject duplicate mapping keys, unsupported schema versions and malformed input. Apply schema validation followed by semantic checks for contradictions, reference integrity and category requirements. Avoid executing interpolations, dereferencing remote schemas or reading provenance documents. Choose and document input-size/alias limits consistent with the lab.

Emit an allowlisted report with schema version, inventory ID/revision, input digest, classification, validation outcome, readiness outcome, per-section status, stable issue codes, item IDs and owner/next-action gaps. Do not copy arbitrary input values, source locators or document contents into diagnostics. A human-readable view may accompany the structured result. Deterministic issue ordering is required; report generation time is metadata, not part of semantic equality.

Exit 0 means structurally and semantically valid, even when the inventory openly records gaps. Exit 1 means malformed/invalid input or a failed validation invariant. Exit 2 means input/output could not be safely read/written. Validity and readiness are distinct: readiness is `incomplete` until all sections are complete, all requirement dispositions are resolved and required items have owners, provenance and acceptance intent. A synthetic inventory always reports `synthetic-only`, even if internally complete. A complete real inventory reports `ready-for-review`, never M0 passed or certified.

Keep reports private by default for real inventories. If an output path is requested, create it exclusively and do not overwrite prior reports. The checked-in fixture and documentation are public and synthetic. This command does not attest authorization, truth of evidence or sanitization of arbitrary customer exports.

## Meaningful verification

Use the current Node test framework. Exercise valid incomplete and internally complete synthetic inventories, a real fixture with synthetic identities reaching ready-for-review, contradictory completion, missing required sections, duplicate IDs/keys, dangling references, invalid dispositions, missing owners/acceptance intent, unknown values, explicit absence with and without evidence, unsafe paths, oversized/alias-heavy input, unsupported schema versions and output failure/overwrite behavior. The real-classification fixture remains test data and is never presented as a collected real inventory.

Prove the command makes no network requests and does not open provenance locators. Assert stable issue ordering, digest binding and absence of injected secret-like input values from reports/errors. Test the distinction between successful validation and incomplete/synthetic readiness. Test actual parsing and semantic behavior rather than snapshots of implementation text.

Run focused tests, the existing full lab check and `git diff --check`; confirm existing profile hashes and historical evidence are unchanged. Database, live clients and browser acceptance need no rerun for this offline-only package. Publish actual results and limitations in the implementation guide and obtain fresh whole-change review before claiming implementation complete.

## Handoff and exclusions

The guide ends with a concrete operator checklist: nominate target and owners; collect authorized source/pipeline/workflow information privately; reconcile missing and contradictory items; review required customization feasibility and export exclusions; then propose profile/contract changes. Missing customer input is a recorded gap, not an invitation to search private credentials or contact anyone.

No real target inventory, M0 sign-off, M1R archive completion, Salesforce behavior claim, runtime feasibility conclusion, revised program estimate or product implementation is delivered by this package. Plan 13 visual acceptance and Plans 10/14 reference collection remain separate. The builder may choose module boundaries, schema composition and report presentation within this scope.

## Builder check

A fresh software-engineer reviewer read this design and repository context and reported “nothing important left.” No material planning questions remained. The inventory package selection, readiness rules and offline-only scope are executor recommendations pending maintainer review.
