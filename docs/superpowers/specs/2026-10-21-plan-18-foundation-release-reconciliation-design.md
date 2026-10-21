# Plan 18 — Foundation release-history reconciliation

Status: proposed for maintainer review; implementation is not authorized by this document.
Milestone: bounded M1R review supporting the M3 foundation; full M1R remains open.

## Intent and selection

Turn the foundation's broad missing-document blocker into a source-backed, bounded decision record: which release changes affect the selected REST/query contracts, what is documented, and what still requires observation or a maintainer decision. Deliver actual source review and contract implications rather than another collector or a new runtime assumption.

This is the executor's recommended next package after [Plan 17](../../compat/plan-17-target-inventory.md). The [remaining roadmap](../../plans/2026-10-20-progress-and-remaining-roadmap.md) puts foundation evidence and relevant release findings first. Plans [10](../../compat/plan-10-foundation-decision.md) and [14](../../compat/plan-14-external-id-normalization-contract.md) resume under their own numbers when their private prerequisites exist. Plan 17 enables a real inventory but does not supply one. Early customization feasibility remains valuable separate work; this package addresses a concrete existing foundation blocker first.

Success means a reviewer can trace each in-scope contract to reviewed release material, an explicit negative finding within a stated search boundary, or an owned source gap. Documentary reconciliation must never be presented as accepted live behavior, a frozen foundation contract or a milestone pass. This request produces a specification only; an implementation plan is a later deliverable.

## Existing assets and constraints

Read the [original M1R requirements](../../plans/2026-10-14-salesforce-compatibility-roadmap.md#m1r--salesforce-release-notes-and-changelog-review), `docs/compat/releases/index.yaml`, `docs/compat/releases/changes.yaml`, `docs/compat/sources.yaml`, the foundation profile and contracts, and Plans 5/10 before planning. The existing archive inventory declares a 2026-10-17 cutoff and 71 document links; these are historical repository statements, not a fresh claim about the current archive.

Follow [CLEAN_ROOM.md](../../../CLEAN_ROOM.md). Do not edit that policy, copy Salesforce prose/code, inspect client implementations, or inspect proprietary internals. Independently summarize findings from public official documentation. Cite exact source locations and review dates. Preserve required notices for any permitted reused artifacts; this package should not need imported source documents or implementation code.

Work directly on main under repository convention. Use existing Node/TypeScript lab dependencies if validation changes are necessary; do not add dependencies. No product Go/database/UI changes, new reference collectors, credential discovery, Salesforce requests using private sessions, source-org changes or automatic communications belong here.

## Scope and review boundary

The selected profile is `rest-query-foundation`, API `65.0`. Read its actual current bytes and contract IDs at execution time; record their hashes in the review record and do not modify them.

Cover the foundation concerns already identified by Plans 5/10: query response shape and continuation; LIMIT, requested batch and total size; locator retention, replay, session scope and mutation effects; identifier forms; object/field name handling and duplicate canonical fields; string comparison; null ordering and tied traversal; errors and unknown objects. Exclude external-ID normalization, missing-key races and wider Apex/Flow/UI behavior except when a source exposes a direct dependency on an in-scope contract. Record such dependencies without expanding the package.

Review the archive's Winter ’26 release documentation as the selected release candidate for API 65.0, and Winter ’23 material for the existing cursor finding. Verify both release/API associations from official material before relying on them; do not infer versions from a URL or season. If the association is wrong or unavailable, correct the proposed batch explicitly and retain the reason. Also follow official linked REST/SOQL/API changelogs, enforced updates, deprecation notices and current guides that directly affect these contracts. Trace older releases only where those sources identify a relevant change or supported-version dependency; retain any unreviewed historical interval as a gap.

For each release document inspect its table of contents/index and relevant API, query, security, limits and release-update sections, including links that could alter the selected semantics. Record what was searched and read, not just keyword hits. No requirement to read every unrelated product chapter is implied. A scoped section review is never a whole-document review.

Current documentation can corroborate a documented expectation but cannot substitute silently for a release-pinned source. An official dated article may supply historical context; distinguish it from release notes. A loading shell, search snippet, failed fetch or inaccessible PDF is an access result, not reviewed content. Do not repeatedly retry inaccessible sources without a materially different official retrieval path. If all paths fail, deliver an explicit blocked outcome with the attempts and next action.

This is a bounded batch, not completion of all archive releases or all changes between API versions 56.0 and 65.0. Do not refresh the archive cutoff or assert archive completeness without separately reviewing the archive and explaining the changed boundary.

## Deliverables

1. Extend `docs/compat/releases/index.yaml` with precise source/section coverage, access outcomes and provenance for this batch, preserving all existing document entries and unresolved history.
2. Extend `docs/compat/releases/changes.yaml` with independently written findings, contract implications, regression acceptance intent and unresolved decisions. Preserve prior findings and dated access checks; add explicit supersession links when an interpretation changes.
3. Write `docs/compat/plan-18-foundation-release-reconciliation.md` with the selected batch, contract-by-contract decision table, reviewed boundaries, remaining blockers and an operator handoff.
4. Update the relevant Plan 5/10 documentary status links only where the new review warrants it. Preserve their live-evidence gates, accepted assumptions and historical outcomes.

Reuse existing ledger structures where they convey the distinctions below. The builder may choose additive fields and a small offline validation extension when needed. Inspect the current validator before choosing the representation. Do not create a general document ingestion system, crawler, source mirror or new reporting CLI.

## Source and finding records

Each reviewed source or section records a stable ID, official URL, release and API applicability when established, access date, reviewer, exact section/page/heading locator, coverage boundary and outcome. Unknown publication dates/API versions remain explicitly unknown. Include a SHA-256 checksum when actual permitted source bytes were retrieved; state which representation was hashed. Do not invent a checksum from a URL, title, extracted fragment or error page. Keep large source files out of the repository.

Each finding records a stable ID, independent behavioral summary, source IDs/locators, introduction/effective versions when known, activation (`opt-in`, `enforced`, `default`, `unknown`), affected contract IDs, selected-profile applicability and rationale. Record deprecation/removal and migration implications when present. Separate facts the source states from the reviewer's inference and unresolved questions. Conflicting or version-ambiguous material remains unresolved with an owner and next action.

Each relevant finding maps to an existing regression test when one genuinely covers it, or a precise proposed test identified as unimplemented. A test proposal names the input/operation, required assertion and evidence prerequisite; it cannot require an outcome the sources do not settle. Out-of-profile decisions require evidence-backed applicability and an explicit rationale, not merely an inconvenient implementation cost.

A negative finding says only that no relevant change was located in the listed sections and search boundary. It cannot establish unchanged behavior across unreviewed releases. Distinguish source accessibility, section review completeness, documentary disposition, live reconciliation and runtime implementation in both ledgers and prose.

## Foundation decision rules

Produce one row per actual in-scope contract ID. Rows list source/finding IDs, documented expectation or unresolved question, review coverage, remaining documentary gaps, live evidence status, regression intent, owner/next action and whether contract freeze remains blocked. Shared findings may appear in several rows without being counted as several independent observations.

Handle `winter23-query-cursor-retention` explicitly. Plan 10 already records a maintainer-authorized two-day lifetime assumption and unknown exact expiry boundary, anchor, eviction and retrieval effects. Preserve that assumption and its scope. A newly readable historical source can resolve a documentary access gap; it cannot measure expiry or remove the live-reconciliation requirement. Do not reintroduce a long-duration expiry experiment or widen the existing waiver.

Narrow `release-content-unavailable` only for sources/sections actually reviewed. Split it into specific owned residual gaps when useful, retaining its historical record and a clear mapping to the replacement entries. Do not remove `linked-changelog-expansion` or declare full M1R complete after a foundation-only traversal. If the source review leaves all substantive blockers intact, report that result plainly; a fully recorded blocked review is preferable to a fabricated decision.

No changes to accepted profile bytes/hashes, readiness attestations, registered observations, captured evidence, runtime semantics or the existing maintainer authorization are permitted. Neither documentation nor passing offline tests can turn an assumed or unmeasured behavior into observed parity.

## Verification and acceptance

At execution, use public official sources and record actual retrieval results; repository statements about past access are not fresh verification. Follow the repository's PDF-reading workflow if inspecting PDF content. Verify each published source locator supports the associated summary and each contract/test reference resolves. Source review remains a human assessment; automated validation must not claim to establish its truth or completeness.

If ledger fields/validation change, use focused tests for unknown version/publication data, scoped versus whole-document review, dangling source/contract/test references, contradictory completion, unresolved required findings and explicit supersession. Reuse the ordinary offline repository validator rather than introducing a parallel acceptance path. Do not force a blocked or partial source into `reviewed` merely to satisfy a schema.

Run the existing full Node lab check and `git diff --check`. Confirm profiles, readiness files and historical evidence are byte-identical to execution base. No database, browser, candidate-client or live-reference rerun is required for documentary-only changes. Obtain fresh whole-change review before claiming the package implemented; publish actual checks and limitations in the guide.

The package is accepted as a completed review batch when its promised source attempts and bounded review are recorded, every in-scope contract has an actionable disposition, links and offline checks pass, and limitations are explicit. That acceptance can include owned unavailable sources. Foundation contract freeze remains blocked by every unresolved required finding and Plan 10's missing accepted live pair. Full M0, full M1R, Plan 13 visual acceptance and Plan 14 runtime prerequisites retain their separate status.

## Handoff and builder choices

End the guide with concrete next actions for each residual source gap, any evidence-dependent Plan 10 reconciliation, and separately scoped runtime decisions once their gates pass. Identify useful later M1R review batches without assigning them completed coverage. Do not change program estimates from this small batch alone.

The executor selected this package, its two initial release candidates and the documentation-first delivery as recommendations. The builder chooses additive ledger field names, narrowly necessary validator/test changes and retrieval tools. Those choices may not weaken historical coverage, omit required-profile gaps or expand product scope.

## Builder check

A fresh software-engineer reviewer read this design and relevant repository context and reported “nothing important left.” Package selection and review boundaries remain executor recommendations for maintainer review.
