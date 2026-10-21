# Plan 18 — Foundation release-history reconciliation

Status: implemented, independently reviewed, corrected and verified, 2026-10-21. Historical source reconciliation remains blocked. This is a completed bounded set of review attempts and current-document context, not completed M1R or a foundation contract freeze.

See the [design](../superpowers/specs/2026-10-21-plan-18-foundation-release-reconciliation-design.md), [implementation plan](../superpowers/plans/2026-10-21-plan-18-foundation-release-reconciliation.md), [source batch](releases/index.yaml) (`reviewBatches`, `plan-18-foundation`) and [findings](releases/changes.yaml).

## Boundary and provenance

Execution base: `6cd02e388d01e05e0c62536068f59bff10e08153`. Work stayed directly on main under the approved convention. Review date: 2026-10-21. Accountable reviewer/next-action owner: `platform-implementer`.

| Protected input | SHA-256 |
| --- | --- |
| `docs/compat/profiles/rest-query-foundation.yaml` | `99d90e50fc63573f08f5a48da878eb9c129b7d660eb9ed5d600306b9706bd8f8` |
| `docs/compat/contracts/rest-query-foundation.yaml` | `c4d71ee2235025bbe786d97a8b2a262dc104b2447e03323435410da055a85ddb` |

The twelve contract IDs come from `suite("rest-query-foundation").contractIds`. Eight definitions live in [REST semantics](contracts/rest-semantics.yaml), and four in [foundation contracts](contracts/rest-query-foundation.yaml); neither file changed. The review covers selected foundation sections, not whole documents, all releases between API 56.0 and 65.0, or all product/API changelogs. The existing 71-link archive inventory, 2026-10-17 cutoff and incomplete full-review status are preserved. No new archive-completeness claim is made.

The official [edition/API version table](https://help.salesforce.com/s/articleView?id=000199268&language=en_US&type=1), Examples section, confirms the initial Winter ’26/API 65.0 and Winter ’23/API 56.0 associations. This table is current corroboration; neither release candidate needed correction. Its publication date is 2026-06-14; other publication dates remain null where not established. The dated article's month appears in its URL, but an exact publication day was not independently established and remains null.

## What could be read

| Source IDs | Attempt/result and actual coverage |
| --- | --- |
| `p18-winter26-pdf`, `p18-winter23-pdf` | Archive PDFs inaccessible through web; direct verified-TLS retrieval failed certificate-chain validation. No PDF bytes/pages were available to inspect, hash or render. TLS verification was retained. |
| `p18-winter26-toc`, `p18-winter26-api`, `p18-winter26-soql`, `p18-winter26-changelog`, `p18-winter26-security`, `p18-winter26-updates` | Pinned English TOC/API/SOQL/changelog/security/update URLs returned loading/CSS shells or inaccessible content. Alternate Italian API URL also returned a shell. No historical section or outbound-link review is claimed. |
| `p18-winter23-api`, `p18-winter23-cursor` | Pinned API index and cursor change page returned shells. Earlier observations of readable content remain historical records. |
| `p18-current-limits-pdf` | Current limits PDF fetch returned 502. Its earlier successful access remains recorded separately. |
| `p18-version-table` | Read the release/API mapping table, not an org's version or settings. |
| `p18-cursor-guide`, `p18-api-article` | Read current cursor limits and the dated article's REST/locator/pagination subsections. Corroboration of the existing two-day assumption; no exact expiry measurement. Bulk sections excluded. |
| `p18-query`, `p18-query-more`, `p18-batch-header` | Read resource descriptions, fields, URIs, method/parameter tables and complete short header section. Historical changes remain unread. |
| `p18-order-by`, `p18-comparisons` | Read syntax/default/tie/locale/restriction paragraphs and comparison introduction/operator table. Semi-join examples excluded. |
| `p18-errors`, `p18-core-types` | Read REST status table/missing-ID/object context and SOAP error-handling introduction plus relevant invalid-ID/type/locator entries. No complete REST error matrix derived from SOAP. |

Source records hold official URLs, exact heading/table locators, kinds, access outcomes, attempts and gaps. Search snippets were discovery aids only. Sources could not supply their historical TOC or linked sections, so those traversals remain explicit gaps. No conclusion of “no relevant historical changes” is supported.

All source checksums are null: readable bodies were web-extracted representations, and exact document bytes were unavailable. No digest was fabricated from a URL, extraction or error page. Large source files and private notes are not checked in. Summaries are independently written; no Salesforce code or product prose is imported.

## Contract decisions

All rows retain an owned documentary gap: retrieve the relevant pinned release sections and linked updates, then reconcile their API 65.0 applicability. All rows also need the fresh accepted Plan 10 pair and appropriate downstream runtime decisions. `partial` means current-source context exists; it does not mean historical reconciliation passed. `blocked` means even the selected semantic question has no adequate source in this batch. Every row keeps `blocksContractFreeze: true`.

Finding shorthand below maps to exact IDs in `changes.yaml`: `query` = `p18-query-continuation-context`; `batch` = `p18-batch-size-context`; `comparison` = `p18-locale-comparison-context`; `ordering` = `p18-null-and-tie-context`; `errors` = `p18-error-id-context`; `cursor` = existing `winter23-query-cursor-retention`.

| Contract ID | Documentary disposition and source/finding IDs | Live state and regression intent |
| --- | --- | --- |
| `query-response-shape` | partial; `p18-query`, `p18-query-more`; query | blocked; traverse >2000 owned rows, compare envelope/attributes and complete ID membership |
| `query-limit-above-batch` | partial; `p18-query`, `p18-api-article`; query; exact LIMIT 5000 still unsettled | blocked; compare LIMIT 5000 pages, totals and all IDs against accepted evidence |
| `id-15-18` | partial; `p18-core-types`; errors; SOAP ID vocabulary does not establish every REST outcome | blocked; test 15-character and valid/invalid 18-character operations, canonical IDs and committed state |
| `case-insensitive-names` | blocked; no adequate reviewed source; explicit owned gap | blocked; compare route/name variants, canonical response keys and selected fields |
| `case-insensitive-comparison` | partial; `p18-comparisons`; comparison; locale-dependent string rules, no universal Unicode conclusion | blocked; compare ASCII/sharp-s/accent/Turkish membership under pinned locale |
| `null-ordering` | partial; `p18-order-by`; ordering; defaults, ties and relationship caveats require scoped interpretation | blocked; compare ASC/DESC/null options and complete tied traversal under accepted policy |
| `error-shape` | partial; `p18-errors`, `p18-core-types`; errors; exact REST matrix unresolved | blocked; compare malformed input/missing-record statuses and error properties; verify no failed-write effects |
| `unknown-sobject` | partial; `p18-errors`, `p18-core-types`; errors; path/query outcomes independent | blocked; compare unknown REST object versus SOQL FROM status/code pairs |
| `query-batch-header` | partial; `p18-batch-header`, `p18-query`; batch; requested size is not guaranteed actual size | blocked; traverse requested 200, then boundary/invalid cases without asserting equal-sized pages |
| `query-locator-lifecycle` | partial; `p18-cursor-guide`, `p18-api-article`, unavailable `p18-winter23-cursor`; cursor | assumed lifetime only; replay/session/mutation and exact expiry remain blocked/unmeasured; retain existing waiver |
| `rest-route-parity` | partial; `p18-query`, `p18-query-more`; query; query resource templates alone do not settle all routes/hosts | blocked; use returned links and encoded text through clients; compare routes and traversal |
| `duplicate-canonical-fields` | blocked; no adequate reviewed source; explicit owned gap | blocked; compare raw duplicate keys/aliases, committed state and denied-field behavior |

Every regression has exact operation/assertion/evidence prerequisites in the batch and an ID `p18-<contractId>-regression`. All twelve are **proposed**, with null test path/name. Existing collection probes and offline assessor tests are not presented as implemented product parity regressions. No newly accepted behavior or product test is claimed.

The current [Query resource](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-query.html) supports the page/envelope context; the [Query Options Header](https://developer.salesforce.com/docs/platform/api-rest/guide/headers-queryoptions.html) makes clear that actual pages can differ from the request. The Query More Results paragraph's QueryAll wording is recorded as source ambiguity, not silently generalized. The [comparison guide](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-comparisonoperators.html) and [ORDER BY guide](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-select-orderby.html) inform locale and tie handling; they do not certify arbitrary Unicode or relationship behavior at the selected version.

## Gates preserved

`winter23-query-cursor-retention` remains unresolved. Its original resolution text about a long-duration experiment is retained as history; the dated reviewDetail explicitly says [Plan 10's waiver](plan-10-foundation-decision.md) controls. The existing two-day assumption is supported by the [current cursor guide](https://developer.salesforce.com/docs/platform/salesforce-app-limits-cheatsheet/guide/salesforce-app-limits-platform-apicursors.html) and [dated API article](https://developer.salesforce.com/blogs/2022/12/processing-large-amounts-of-data-with-apis-part-1-of-2). Exact expiry boundary, clock anchor, eviction and effects of later retrieval remain unmeasured. No wait longer than one hour or expanded waiver was introduced.

`release-content-unavailable` retains its full historical record and affected contracts; no newly readable pinned body justifies narrowing it. `linked-changelog-expansion` remains full-M1R blocking. Five additive current-context findings remain `reviewed-source-awaiting-live-reconciliation`; none has `resolved` or `reviewed-and-reconciled` status. No supersession was needed for this batch. The validator tests legitimate supersession references/cycles, and characterization tests prove supersession does not resolve live obligations.

## Validation and verification

The ordinary lab `validate` entry point now checks batch structure, exact contract coverage, dates/enums/hashes, source/finding/regression links and supersession cycles. It compares recorded protected-file digests with repository bytes and checks paths for regressions labeled existing. It never fetches source URLs or opens provenance locators. Existing-test paths must stay in the approved test directories without traversal or symlink components. A reviewer still verifies a test's semantics and the truth/adequacy of source summaries; automation does neither.

The synthetic unit fixtures explicitly model unknown versions/dates and owned unavailable sources. A complete batch can contain blocked/partial dispositions. Source section completion, documentary reconciliation, live observation and implementation are separate facts. Offline tests cannot establish source truth or Salesforce parity.

Final focused verification passed **59 tests**, with no failures or skips, including repository integration, foundation assessor characterization and the review corrections. The frozen-source post-review `npm --prefix compat/clients/node run check` passed typechecking, formatting, **369 tests / 366 passed / zero failed / three skipped**, and repository validation. The three skips are the existing optional `candidate/jsforce/core`, `candidate/jsforce/policy` and `candidate/jsforce/isolation` live-candidate tests. All release-review tests ran. The earlier pre-review full check passed 358 of 361 tests with the same three skips; the post-review result is the final verification.

Fresh whole-change review reported **one Important issue, no Critical or Minor issues**: finding-level `status`, `blocksContractFreeze` and `affectedContracts` could contradict unresolved review details and silently stop blocking the existing assessor. Seven independent mutation tests reproduced the bypasses before correction and passed afterward. Validation now rejects those contradictions for unresolved in-profile/unknown-applicability records; legitimate legacy resolution semantics remain unchanged. The fix received focused and full verification; no second reviewer pass was dispatched.

The reviewer checked official current-source summaries, actual regression mappings and historical preservation. Historical unavailable bodies/access-attempt reproducibility, runtime parity/private readiness, full archive completeness and final executor completion were explicitly outside that review's judgment; the gaps and independent acceptance gates remain recorded here.

`git diff --check` and local guide links passed. All six manifest-protected profile/readiness/observation files, both relevant contract files, dependency manifests/lockfile and `CLEAN_ROOM.md` were byte-identical to execution base. Parsed comparisons confirmed every original archive entry/metadata value and original finding field, blocker and access-check record was retained. No product code, database, UI, accepted observation or profile behavior changed. The review batch comprises ten reviewed current/context sources, eleven unavailable sources, ten partial contract dispositions and two blocked dispositions.

## Operator handoff

1. `platform-implementer`: obtain readable official Winter ’26 foundation/API/SOQL/security/update/changelog sections and Winter ’23 cursor material through a new viable retrieval path. Record pages read, actual bytes/checksums if available, remaining gaps and linked updates. Do not retry shells as evidence or bypass TLS verification to force a result.
2. Reconcile API applicability and any conflicts with the current context. Keep unreviewed older intervals and absent name/duplicate-field semantics owned and explicit. Review migration/activation/deprecation implications when source content supplies them.
3. Resume Plan 10 only with its authorized private administrator/fresh/restricted sessions, identity pins and recovery prerequisites. Collect its accepted sequential pair and independent cleanup proofs; maintain the existing lifetime assumption and unknown exact expiry dimensions.
4. Propose runtime behavior and concrete regression implementation only after their evidence/decision gates. Plan 14's external-ID normalization, missing-key races and broader security remain separate work.
5. Continue M1R with readable release sections for other supported versions, linked enforced updates/retirements and remaining archive history. This batch does not retire those obligations or change program estimates.

Full M0, full M1R, Plan 13 visual acceptance, Plan 14 reference prerequisites and certification remain open independently. No database, browser, candidate-client, private-reference or production acceptance is claimed.
