# SalesCompel Salesforce compatibility roadmap

Date: 2026-10-14. Status: proposed architecture and milestone plan; Plan 1 offline laboratory and the Plan 2 M2 candidate walking slice implemented on 2026-10-15. Plan 2 proceeded without M0/M1 reference evidence, so no milestone exit gate has passed and no live parity certification has occurred. Revised after a plan review on 2026-10-15 (see [Coverage assignments](#coverage-assignments-from-the-2026-10-15-review)).

## Objective and planning assumptions

Build an independently implemented Salesforce replacement that accepts Salesforce exports and preserves the data, configuration, integrations, permissions, automation, and user workflows required by a migrating organization. Prefer Go for services and Vue/TypeScript for authenticated application pages without SSR.

SalesCompel is open source under the Apache License, Version 2.0 (SPDX: Apache-2.0), and free to use. Do not plan paid tiers, subscriptions, or billing features. This no-fee product policy does not add restrictions to the Apache-2.0 license. Review third-party dependency licenses and preserve required attribution and redistribution notices, including for API clients used in compatibility testing. See the repository's root LICENSE file.

The long-term objective is 1:1 replacement. The proposed first certification target is Sales Cloud plus the core customization platform. That sequencing is an assumption, not a reduction of the ultimate goal. Service Cloud, Experience Cloud, Revenue/CPQ, Marketing, Commerce, Data Cloud, industry products, analytics extensions, and AI products require separate inventories and certification tracks. Do not advertise universal Salesforce compatibility on the strength of a Sales Cloud pilot.

The repository contains the Plan 1 laboratory and the Plan 2 candidate service (`cmd/salescompel`, `internal/platform`, `web`). Team size, target org, data volumes, hosting constraints, downtime allowance, and required Salesforce versions are not yet established. Milestones below are dependency and acceptance driven, not delivery-date promises.

**Branch policy:** all work is committed directly on `main` until every milestone below is complete. Only then does the project become usable and testable by humans; until that point, plans must not require feature branches or merge-based review.

## What “drop-in” means

For a named compatibility profile, existing supported clients and customer customizations must work without business-logic rewrites. Changing the service endpoint, configuring the identity provider, and issuing new credentials are explicit migration operations. Salesforce-owned hostnames, sessions, credentials, and third-party allowlists cannot be assumed to transfer.

A profile records source edition/release, API versions, class versions, enabled features, packages, object and metadata inventory, client versions, locale, currency, timezone, security settings, and representative workloads. Pin versions from the actual source org; do not choose a presumed “latest” version from a search result.

Certification has separate dimensions:

| Dimension | Required evidence |
| --- | --- |
| Wire compatibility | Methods, paths, headers, payload shapes, status codes, errors, paging, authentication and async states match the profile |
| Data compatibility | Identifiers, relationships, values, files, ownership and audit provenance reconcile |
| Behavioral compatibility | Transactions, validation, formulas, automation, security and query semantics match |
| Customization compatibility | Required metadata and customer code deploy and execute with equivalent outcomes |
| User experience | Required workflows, layouts, navigation, keyboard behavior, localization and accessible states pass acceptance scenarios |
| Operational compatibility | Agreed throughput, latency, recovery, migration window and integration continuity pass rehearsal |

Every discovered feature has a status: unknown, documented, observed, specified, implemented, verified, or blocked. An unsupported feature used by an organization blocks that organization's certification. An explicitly accepted workaround is a migration exception, not 1:1 parity.

## Approaches considered

1. **Compatibility engine with organization certification — recommended.** Build a metadata-driven platform, Salesforce protocol adapters, an independent behavior suite, and a migration analyzer. Deliver complete vertical slices and widen the supported profiles. Highest alignment with drop-in replacement; requires substantial platform work before broad adoption.
2. **CRM UI and data import first.** Quickly useful for simple sales workflows, but code, automation, APIs, and customizations must often be rewritten. Appropriate only if the goal changes to a functional alternative.
3. **Salesforce-backed proxy or coexistence layer.** Useful temporarily for shadow reads and staged migration, but retains Salesforce as a dependency and does not satisfy the final standalone objective. Never hide proxying inside a certified replacement.

## Research findings that shape the design

- Metadata API handles customization separately from record data. A CSV import alone is not an org migration. Inventory metadata coverage by type and version. [Metadata API purpose](https://help.salesforce.com/s/articleView?id=platform.code_tools_websvcAPI.htm&language=en_US&type=5), [coverage guidance](https://developer.salesforce.com/docs/platform/sfdx-dev/guide/sfdx-dev-supported-mdapi-types.html).
- Salesforce's backup export has explicit options for attachments and Files/content versions. Import must inspect what is actually present and report omissions. [Backup export](https://help.salesforce.com/s/articleView?id=admin_exportdata.htm&language=en_US&type=0).
- Composite requests have dependency references and rollback behavior. A successful outer HTTP response does not imply every subrequest succeeded. [Composite requests](https://developer.salesforce.com/docs/platform/api-rest/guide/requests-composite.html).
- SOQL relationship traversal is schema constrained; it is not arbitrary SQL. Bulk query supports a different query subset from ordinary query. [Relationships](https://developer.salesforce.com/docs/platform/salesforce-soql-sosl/guide/sforce-api-calls-soql-relationships.html), [Bulk query](https://developer.salesforce.com/docs/platform/api-asynch/guide/queries.html).
- Current security documentation distinguishes Apex defaults before and after API version 67.0. Treat version-sensitive behavior as a contract requiring source-org verification, rather than assuming one universal security context. [Apex security](https://developer.salesforce.com/docs/platform/lwc/guide/apex-security). This claim was not re-verified during the 2026-10-15 review (the page refused automated fetches); the Plan 2 candidate's API version 65.0 predates 67.0.
- UI API and Lightning Data Service expose metadata-aware, permission-aware behavior. Recreating a few record forms does not recreate the customization runtime. [Lightning Data Service](https://developer.salesforce.com/docs/platform/lwc/guide/data-ui-api).
- Pub/Sub replay IDs are opaque, and retained events have a documented recovery window. Migration cannot treat event replay as an unlimited backup or calculate its own Salesforce replay positions. [Event durability](https://developer.salesforce.com/docs/platform/pub-sub-api/guide/event-message-durability.html).

These are initial findings, not an exhaustive reading of all Salesforce documentation. Dynamic documentation pages sometimes returned only an iframe shell; those pages are indexed below as follow-up resources, not treated as extracted evidence.

## Clean-room research and specification process

Maintain a provenance register for public documentation, official release notes, help articles, Trailhead exercises, public reference applications, API descriptions/WSDLs, and authorized observations from disposable test organizations. Public examples generate test scenarios; they are not proof of undocumented behavior.

Keep behavioral specifications separate from implementation. Researchers record inputs, preconditions, visible results, and reproducible observations; implementers work from those specifications. A strict organizational clean-room process additionally requires assigning separate research and implementation personnel and controlling their access; simply naming folders “clean-room” does not establish that separation.

Do not use proprietary Salesforce implementation source or copy product branding/assets into SalesCompel. Record licenses and provenance for any deliberately reused open-source client, protocol definition, framework, or test fixture. Customer-owned code imported for execution remains an input artifact, not a source for copying Salesforce internals.

For each contract record: stable ID, source URL and access date, document/API version, org configuration, preconditions, input, response, committed state, emitted side effects, normalization rules, client/version, confidence, conflicts, and associated test. Redact credentials and personal data from evidence. Prefer synthetic datasets.

When sources conflict, preserve both claims and reproduce against the selected profile. For example, the CLI bulk guide and Bulk query guide currently describe query restrictions differently; the endpoint/version observation must resolve the contract rather than silently picking one statement.

## Proposed architecture

Start with a modular Go application plus separate worker and untrusted-code execution processes. Use PostgreSQL for authoritative transactional data and metadata, and S3-compatible storage for binaries and migration bundles. Add a dedicated search backend only after SOSL/relevance and scale tests show the need. Avoid distributing core record transactions across services initially.

| Component | Responsibility and boundary |
| --- | --- |
| Protocol adapters | REST, Composite, SOAP, Bulk, Metadata, Tooling, UI API, GraphQL, Reports and event protocols; translate wire contracts into shared services |
| Metadata catalog | Versioned object/field definitions, relationships, record types, layouts, validation, permissions, namespaces and dependency graphs |
| Record engine | Tenant-scoped storage, stable external IDs, typed values, locking, transactions, delete/undelete/merge and audit provenance |
| Query engine | Parse SOQL/SOSL into an AST; validate against metadata; apply security; compile to storage/search plans; serialize Salesforce-shaped results |
| Policy engine | Object/field access, sharing, roles, groups, teams, territories, restriction rules and explicit execution context |
| Automation runtime | Formula evaluator, ordered transaction hooks, Flow, approvals, Apex compatibility, limits, scheduled and asynchronous execution |
| Jobs and events | Durable jobs, transaction-aware outbox, retries, event schemas, subscriber state and protocol-specific delivery behavior |
| Migration service | Discovery, bundle validation, staging, resumable import, reconciliation, deltas and cutover report |
| Vue application | Metadata-driven navigation, lists, record pages, workflows, reporting, setup and migration UI |

Use typed relational storage with metadata-defined fields and indexes; evaluate JSONB only where it does not sacrifice query, index, uniqueness or relationship semantics. Benchmark alternatives before locking the physical schema. Use decimal arithmetic for currency and numeric precision contracts, not binary floating point.

Tenant identity must participate in every key, query, job, cache and blob authorization path. Database row isolation is defense in depth; it does not replace Salesforce sharing semantics. PostgreSQL row-level security for tenant isolation is an M4 deliverable (the M2 candidate relies on application-level binding only). SalesCompel's own database needs versioned, forward-only schema migrations with upgrade tests before M4 introduces dynamic schema; the M2 startup check that rejects a mismatched catalog version is not a migration mechanism. Execution context explicitly carries user, tenant, API/class version, privilege mode, sharing mode and transaction limits.

Vue is the native SalesCompel UI. Imported LWC, Aura and Visualforce are distinct compatibility targets: Vue cannot execute them automatically. An isolated component/runtime bridge or independent compatible runtime must supply their required services. Any permitted OSS LWC reuse needs provenance tracking and still does not supply the entire Salesforce runtime.

## Internationalization, languages and locales

Internationalization is a foundation requirement starting in M0–M2, not a later UI retrofit. Match the selected Salesforce release's language inventory, support categories, org enablement and user selection behavior. Salesforce distinguishes fully supported, end-user and platform-only languages; these have different standard-UI translation coverage and fallback behavior. Preserve this distinction rather than claiming every selectable language has fully translated Setup. Maintain a versioned inventory and extend it as Salesforce adds languages. [Salesforce supported languages](https://help.salesforce.com/s/articleView?id=faq_getstart_what_languages_does.htm&language=en_US&lwr.isSeoBot=true).

**One file per language or regional language variant.** Store independently authored product translations in `locales/messages/<salesforce-language-code>.json`, with stable message keys shared by Vue and Go. Examples are `en_US.json`, `fr.json`, `de.json` and `pt_BR.json`; validate actual codes against the target inventory. Do not combine multiple languages into one translation file or hardcode user-facing strings in components. Build tooling can emit frontend and backend bundles from this single source. Lazy-load Vue catalogs; backend jobs resolve messages for their intended recipient, not the worker's machine locale.

Use `locales/languages.json` as the registry for code, native/display name, Salesforce support category, availability by version, text direction, explicit fallback and translation coverage. A registry is not a translation catalog. Preserve Salesforce wire codes, including documented legacy aliases such as `iw` and `in`; map them explicitly to BCP 47 tags for formatting libraries. Do not blindly replace underscores or rewrite stored/API values. Each enabled platform-only language also gets its own catalog, even when intentionally sparse and relying on a documented fallback.

Adding a language requires a registry entry, its own JSON catalog, reviewed translations for its support category, fallback/formatting fixtures and a passing catalog check. Existing UI components must not need editing. Validate duplicate/missing keys, interpolation parameters, plural/select forms and permitted markup; allow intentional missing standard labels only where the language's documented coverage requires fallback. Keep untranslated coverage visible and never label an English-filled catalog as a completed translation.

Language, locale, timezone and currency remain separate settings. Preserve org defaults and user `LanguageLocaleKey`, `LocaleSidKey`, `TimeZoneSidKey` and applicable currency settings during migration. Match Salesforce's allowed-language lists, permissions and restrictions on disabling languages still used by an org or user. Record fallback per surface and support category rather than assuming one global English fallback. [Language selection](https://help.salesforce.com/s/articleView?id=sf.admin_select_languages_for_your_org.htm&language=en_US&type=5), [language/locale settings](https://help.salesforce.com/s/articleView?id=sf.admin_language_locale_currency.htm&language=en_US&type=5).

Use locale-aware formatters with pinned runtime/data versions and reference fixtures for dates, times, decimals, currency, names, addresses, week starts and sorting where applicable. Preserve canonical API values independently of localized presentation. Test a French-language user with a US locale and a separate timezone; do not infer locale from language. Cover daylight-saving changes, ambiguous date input, non-Latin text, pluralization, long labels and right-to-left layout. Browser Intl defaults alone are not evidence of Salesforce equivalence.

Implement a Translation Workbench-compatible metadata path for required custom labels, object/field labels, picklist display labels and package overrides. Keep tenant translations separate from shipped product catalogs, isolated by tenant and namespace, and exported by language. Preserve API names and picklist stored values while translating display labels. Import/export the relevant `Translations`, `CustomObjectTranslation` and `CustomLabels` metadata representations for the pinned API version, and retain source artifacts for round-trip comparison. Missing translations use observed fallback behavior; missing required translation metadata blocks migration certification. [Metadata translation](https://help.salesforce.com/s/articleView?id=platform.workbench_metadata_translation.htm&language=en_US&type=5), [DX translation representations](https://developer.salesforce.com/docs/platform/sfdx-dev/guide/sfdx-dev-ws-decomposed-md-types.html).

Localization milestone gates:

- **M0/M1:** enumerate every supported language and locale for the target version; capture category, enablement, fallback and formatting contracts. Include separate language/locale combinations in the reference suite.
- **M2:** establish the shared catalogs, registry, resolver and validation checks before expanding the Vue UI. Prove switching languages, regional variants and right-to-left layout through a vertical slice.
- **M4/M5:** support org/user language settings and translation metadata; migrate and round-trip translations without changing API identifiers or leaking tenant overrides.
- **M6:** complete the category-appropriate translations and localized workflows for every language advertised by the release; review pluralization, accessibility and visual layout.
- **M9/M10:** require locale-specific acceptance for the certified profile and run regression checks when Salesforce's inventory or formatting behavior changes. A required language gap blocks that profile's certification.

## OpenAPI contract and External Services

SalesCompel must publish its HTTP API contract in OpenAPI **3.0** YAML (initial revision 3.0.3). The hand-written document is the source of truth: the service serves it, each operation cites its public sources (`x-source` URLs registered in `docs/compat/sources.yaml`), and HTTP tests validate requests and responses against it. Plan 1 adds validation and initial REST contract coverage; M2 serves the versioned document and later API changes update it alongside connector tests. SOAP and gRPC retain their own WSDL/proto contracts.

External Services compatibility must import OpenAPI **3.0 only**, in JSON and YAML, with Salesforce-compatible schema/operation handling, Named/External Credentials, permission checks and Flow/Apex invocation. OpenAPI 2.0 is excluded by user direction: SalesCompel uses OpenAPI 3.0 everywhere, and wherever Salesforce uses an older description format, SalesCompel replaces it with a 3.0 equivalent (the same policy as the sibling zeroauthy clean-room project). Salesforce accepts 2.0 External Service registrations, so the migration analyzer inventories registrations by OpenAPI version and **converts each 2.0 document to an equivalent OpenAPI 3.0 document** during import. The original 2.0 document is retained as a customer input artifact for provenance and round-trip comparison. Conversion must preserve registered operation names, schema-derived Flow/Apex types, parameters, media types and Named Credential binding, verified by equivalence tests against the source registration. A construct that cannot be converted without changing behavior blocks that registration's certification rather than being silently dropped. Callouts take their base endpoint from the Named Credential; the registration metadata resolves a relative `schemaUrl` against it. How a spec's `servers` entries interact with the Named Credential endpoint is unspecified in the reviewed sources and must be observed (`external-servers-vs-named-credential`). M1 supplies schema fixtures and behavioral contracts; M4 handles registration metadata; M7/M8 implement execution and migration. Distinguish standards validity from Salesforce's supported subset and observed runtime semantics. Use official Salesforce specs as pinned test fixtures when provenance and reuse terms permit; an org-specific generated spec remains pending without a reference org.

Sources: [ExternalServiceRegistration metadata](https://developer.salesforce.com/docs/atlas.en-us.api_meta.meta/api_meta/meta_externalserviceregistration.htm), [Salesforce support matrix](https://help.salesforce.com/s/articleView?id=platform.external_services_intro_openapi_2_3_support.htm&language=en_US&type=5), [Salesforce 3.0 examples](https://help.salesforce.com/s/articleView?id=platform.external_services_examples_openapi_3_0.htm&type=5), [OpenAPI 3.0.3](https://spec.openapis.org/oas/v3.0.3).

Implementation package: [Plan 1](2026-10-15-plan-1-compatibility-laboratory.md).

## Export/import contract

Support three entry paths: Salesforce backup ZIP/CSV plus binaries; Data Loader/API CSV exports; and connected extraction combining data, metadata and files. Accept a partial export for inspection, but never describe it as a complete org clone.

The proposed portable SalesCompel bundle contains:

```text
manifest.json             # schema version, source org/profile, extraction windows, hashes
schema/                   # describe results and object/relationship catalog
metadata/                 # retrieved configuration and customer customizations
data/                     # per-object streams with explicit encoding/null conventions
files/                    # content-addressed binaries and version/link manifests
identity/                 # user/group/role mappings, never reusable Salesforce sessions
deltas/                   # checkpoints, updates, deletions and reconciliation windows
reports/                  # completeness, unsupported items and reconciliation results
```

Import procedure:

1. Inventory source access and export contents. Classify inaccessible, absent, unsupported and unexportable items independently. Detect metadata/package dependencies and estimate storage/time.
2. Validate checksums, archive paths, sizes, encodings and schemas. Stage into an isolated target tenant; produce a dry-run dependency and incompatibility report.
3. Import metadata in dependency order, initially inactive where execution would cause side effects. Unsupported active automation blocks activation; never silently discard it.
4. Establish users, inactive-user representations, ownership and permission mappings. Reconfigure authentication and secrets through a separate secure process.
5. Load records in dependency order, then resolve cycles and polymorphic relationships. Preserve source IDs at the compatibility boundary, including valid 15/18-character equivalence, while using separate internal keys. Track source org plus object plus ID to prevent collisions across migrations.
6. Load files, versions, links, notes, activities and accessible history. Preserve source audit values as provenance without falsely claiming SalesCompel generated those historical events.
7. Reconcile counts, canonical field hashes, relationships, ownership, permissions, file hashes and representative reports. Derived values are evaluated and compared, not assumed correct from copied CSV columns.
8. Apply deltas idempotently with update and delete capture. Establish capture before bulk extraction where possible; reconcile extraction races. If event retention or object coverage leaves a gap, perform a bounded rescan or a write freeze and final extract. Do not assume a globally atomic Salesforce export.
9. Activate tested automation only after baseline reconciliation. The migration load path suppresses external emails, webhooks and business actions while preserving declared import semantics.
10. Freeze source writes for the agreed final window, apply final deltas, reconcile, move endpoints and identity settings, and run smoke workflows. Journal every target write after cutover.

Every job has a bundle hash, mapping version, checkpoints and row-level outcomes. A repeated import must not duplicate records or side effects. Reject/quarantine rows visibly; any unresolved required row or relationship blocks certification.

Rollback before new target writes can restore routing to the frozen source. Rollback after target writes requires tested reverse replay, conflict handling and reconciliation; it is not simply a DNS change. If reverse replay cannot cover a profile, state the recovery procedure and point of no return before cutover. Retain a portable SalesCompel export to prevent new lock-in.

## Compatibility test strategy and open-source tools

Run identical synthetic scenarios against a disposable Salesforce reference org and SalesCompel. Compare protocol results, final records, access decisions, transaction boundaries and side effects. Normalize only explicitly nondeterministic fields such as generated job IDs and timestamps; preserve significant ordering, nulls, precision, errors and permission failures. The reference oracle must be independent of the SalesCompel implementation.

Use these real clients in the test harness, pinned by release and digest:

Open-source connector tests against actual SalesCompel endpoints are a mandatory release gate, not optional examples. Build a Node/TypeScript harness under `compat/clients/node/` with its own `package.json` and lockfile. Use the published `jsforce` npm package and the Salesforce CLI's `@salesforce/plugin-data` and `@salesforce/plugin-deploy-retrieve` packages through supported CLI commands. Verify package names, Node requirements and versions when installing; use reproducible `npm ci` installs. The Go server does not acquire a Node runtime dependency merely because its interoperability tests use npm clients.

Run the same scenario code in two modes: against a disposable Salesforce reference org, and against a running SalesCompel instance with an isolated tenant and real database. Only endpoint, credentials, profile version and fixture IDs may vary. Do not mock the HTTP transport, stub the connector, or substitute a SalesCompel-specific SDK in these interoperability tests. Raw HTTP tests supplement connector coverage for endpoints the selected clients do not expose.

Require connector-driven authentication/refresh where supported, describe, CRUD, external-ID upsert, relationship query/pagination, composite rollback, Bulk job lifecycle/results, metadata deploy/retrieve and structured error handling. Include changing user language/locale, retrieving localized metadata and confirming canonical stored/API values remain stable. Record which package exercises each endpoint and behavior; JSforce and CLI tests alone are not independent implementations where the CLI shares JSforce, so retain the Python client as a separate check.

CI must run target connector tests on relevant changes and the complete supported-client matrix before a release. Run live reference comparisons in a credentialed test environment and retain sanitized, versioned reference evidence for ordinary offline regression runs. Cached evidence is not a claim of a fresh live Salesforce test. Release reports identify package/version, API version, profile, endpoint, scenario, reference evidence date and outcome. Fail the release gate for required-client failures; do not silently skip unavailable reference jobs or hardcoded-host blockers.

| Client/tool | Planned use | Constraint |
| --- | --- | --- |
| [JSforce](https://github.com/jsforce/jsforce) | Node client workflows for supported API families and async results | Exercise normal client calls; do not patch client logic to conceal incompatibility |
| [simple-salesforce](https://github.com/simple-salesforce/simple-salesforce) | Independent Python REST/query/bulk scenarios | A second client can expose assumptions hidden by a shared client stack |
| [Salesforce CLI data plugin](https://github.com/salesforcecli/plugin-data) | Query/export/import/upsert command workflows | Verify custom endpoint/auth support with the pinned CLI; record adapters and additional discovery calls |
| [CLI deploy/retrieve plugin](https://github.com/salesforcecli/plugin-deploy-retrieve) | Metadata round trips, validation, deployment status | Require dependency and error fidelity, not just XML parsing |
| [Data Loader](https://github.com/forcedotcom/dataloader) | Real CSV import/export and relevant SOAP/Bulk interoperability | Run the supported GUI/CLI mode for the chosen platform |
| [Salesforce Pub/Sub samples and proto](https://github.com/forcedotcom/pub-sub-api) | Go gRPC/Avro interoperability and replay scenarios | Repository archived (read-only) on 2026-08-08 with no relocation notice. Pin the proto from the [Pub/Sub API developer guide](https://developer.salesforce.com/docs/platform/pub-sub-api/overview) or a verified successor, recording commit/checksum and license. Samples are not production runtime code |

A configurable endpoint and new credentials are allowed. Rewriting a client to bypass missing discovery, authentication or API behavior does not count as unmodified-client compatibility. Where a client hardcodes Salesforce infrastructure, report the blocker separately.

Reference scenario families include:

- CRUD, external-ID upsert, invalid and duplicate IDs, omitted/null/empty fields, unicode and CSV newlines.
- Parent/child and polymorphic queries, aggregates, date literals, currency, timezone/DST, pagination, deleted records and query errors.
- Composite partial failure and rollback; Bulk partial success, abort, retries, result downloads and malformed CSV.
- Field/object/record permission combinations, inherited roles, sharing recalculation, cross-tenant attempts and user/system execution modes.
- Validation, formula null handling, rollups, duplicate rules, Flow/trigger ordering, recursion, transaction rollback, asynchronous work and limit failures.
- Metadata deploy/retrieve, namespaces, missing dependencies, unsupported types and rollback on deployment failure.
- Concurrent updates, locking, crashes between commit and delivery, duplicate events, stale cursors and replay gaps.
- Lead conversion, opportunity progression, campaign attribution, activity linkage, report totals and file access.

Use browser observation when documentation leaves UI behavior ambiguous: create repeatable fixtures, record role/configuration, perform the same task in both systems, and compare visible outcomes, keyboard/focus behavior and screenshots at fixed viewport sizes. Use an authorized disposable org; do not mutate production to discover behavior. Playwright can later automate accepted scenarios. Screenshots alone do not prove data or permission parity.

No authenticated Salesforce API or browser feature tests were run during this planning pass. The missing reference org and absence of an implementation prevent a live comparison. Open-source tooling was researched for the test plan, not installed or represented as executed.

## Milestones and acceptance gates

Milestones are complete only when their evidence is stored and reproducible. Scope growth changes estimates, not the acceptance definition.

| Milestone | Deliverables | Exit gate | Dependencies |
| --- | --- | --- | --- |
| M0 — Target and evidence charter | First org/profile inventory (including the org-level features in the profile's `orgFeatures`), source register re-verified against extracted content, clean-room responsibilities and recorded terms-of-service decision, feature matrix, scale and downtime objectives | Every required integration/customization assigned a contract owner; reference org identified; unsupported export areas explicit | None |
| M1 — Behavior laboratory | Disposable reference fixtures, pinned clients, API capture/comparator, browser scenario format, CI evidence reports | Repeated reference runs distinguish deterministic behavior from normalization; initial CRUD, denied-access, rollback and query scenarios reproducible | M0 |
| M1R — Complete release-history and changelog review | Inventory and review every release linked by the Salesforce previous-release-notes archive, plus linked product/API changelogs, release updates, deprecations and removals; maintain a versioned coverage ledger and trace findings into contracts | Every inventoried release/document is reviewed or explicitly blocked with owner and reason; all compatibility-relevant changes map to affected profiles/contracts/tests; unresolved required-profile gaps block certification | Starts alongside M1; initial review before M3 contract freeze, ongoing before every supported release certification |
| M2 — Platform walking slice | Go service, tenant/auth boundary, metadata catalog, record store, first REST describe/CRUD/query endpoints and minimal Vue record workflow | A synthetic Account/Contact/custom-field slice passes client, UI, isolation and restart tests end to end | M1 |
| M3 — Core protocol and query fidelity | Required REST/Composite/SOAP/Bulk surfaces, SOQL/SOSL, IDs, transactions, paging, error contracts and limits; OAuth 2.0/OpenID Connect identity surface (see [identity contracts](../compat/contracts/identity.yaml)); `/limits` and `Sforce-Limit-Info`; REST request headers; recycle bin, `queryAll`, undelete, `getUpdated`/`getDeleted`; system/audit fields; served OpenAPI document | Pinned client workflows pass; rollback, null/precision, locking and partial-result cases match reference | M2 |
| M4 — Metadata and security | Custom schema and the full field-type inventory, record types, layouts, permission sets/profiles, sharing, Metadata deployment and required Tooling calls; development lifecycle (sandboxes, scratch orgs/Dev Hub, unlocked/2GP packages, change sets, CLI source tracking, Apex test-coverage deployment gate); org security settings (CORS, Remote Site Settings, CSP trusted sites, login IP ranges, session settings, MFA); PostgreSQL row-level security and schema migrations | Metadata round trip preserves required semantics; access matrix has no unexpected grants; deployment failures leave correct state | M2; M3 for full certification |
| M5 — Migration rehearsal | All three import entry paths, files/history mappings, audit-field import ("Set Audit Fields upon Record Creation" semantics), field history and Setup Audit Trail provenance, dependency staging, resumability, delta capture, reconciliation and export | Crash/restart/reimport is idempotent; all required rows/files reconcile; malformed/incomplete exports produce blockers rather than false success | M3, M4 |
| M6 — Sales Cloud workflows | Leads/accounts/contacts/opportunities (including Person Accounts when profiled), products/price books, campaigns, activities, multi-currency, required forecasting/territory behavior, email templates/sending/deliverability, Vue lists/forms/search/reporting; Salesforce mobile app and Outlook/Gmail integrations declared as certification exceptions unless a compatible path exists | Named business workflows and report results match under each tested role; customer tasks pass UI acceptance | M3, M4; M5 supplies migrated fixtures |
| M7 — Automation and extensibility | Required formula/validation/rollup behavior, Flow/approvals, legacy Workflow Rules and Process Builder still active in source orgs, assignment/auto-response/escalation rules, Apex execution, `Messaging.sendEmail` and async jobs, LWC/Aura/Visualforce compatibility tracks | Target-org customizations execute unchanged with correct order, limits, security and failure behavior; unavailable package behavior remains a blocker | M3, M4; feasibility probes start during M1 |
| M8 — Integration and event completeness | Required UI API/GraphQL/Reports/Connect surfaces, Pub/Sub/CDC/Streaming, outbound messages and external-service adapters | Every inventoried integration passes reconnect, retry and failure tests with only declared endpoint/auth changes | M3, M4, M7 as used |
| M9 — Pilot production certification | Full migration rehearsals, load/recovery/security testing, operator runbooks, cutover and rollback rehearsal | All required contracts pass, zero unexplained reconciliation differences, zero unresolved critical/high security defects, agreed performance and recovery targets met | M5–M8 for the pilot profile |
| M10 — Wider Salesforce coverage | Separate specs and suites for each additional cloud, package ecosystem and release | Each profile independently satisfies M9-level evidence; regressions checked against already certified profiles | M9 foundation; independent discovery can start earlier |

### M1R — Salesforce release notes and changelog review

Added at the user's request on 2026-10-15. Start from the official
[previous Salesforce release notes archive](https://help.salesforce.com/s/articleView?id=release-notes.rn_previous_release_notes.htm).
Review **all releases available through that archive**, not only the current
release or pilot API version, and follow its relevant product/API changelog,
release-update, retirement and known-issue links. Record the archive inventory
and review cutoff date so “all” has an auditable boundary; unavailable historic
material remains an explicit evidence gap, never a silently omitted release.
DO NOT copy text or code blocks from salesforce!
Only use salesforce material to determine which features to update or change.

Deliver `docs/compat/releases/index.yaml` with one entry per release/source:
release/API versions, URL, publication/access dates, content checksum where
permitted, product area, reviewer, status, coverage and blockers. Deliver
`docs/compat/releases/changes.yaml` with behavioral changes, effective versions,
opt-in versus enforced activation, deprecations/removals, affected API/UI/security/
metadata/automation/localization/migration contracts, required regression tests
and compatibility decisions. Summarize sources independently; do not copy whole
release documents. Separate documentary expectations from reference observations.

Acceptance requires a reviewed entry for every inventoried document, or an
explicit unavailable/blocked entry with an owner and resolution path; every
relevant finding must map to a contract/test or a justified out-of-profile
classification. Any unresolved finding required by a certified profile blocks
that profile. Check legacy behavior still required by supported API versions,
upgrade defaults, changed permissions/limits and export/import implications.
Reconcile the findings before M3 contract freeze and repeat the delta review for
each new Salesforce release before certification.

The archive spans roughly two decades, so the inventory stays complete but the
review is ordered: first the releases covering every API version a profile must
support (retired versions, such as 21.0–30.0, are inventoried and classified
out-of-profile rather than reviewed in depth), then retirements and enforced
release updates that affect migrated orgs, then the remaining history. This milestone is **planned,
not completed**. The archive page could not be extracted by the web tool on
2026-10-15; no historical-release review is claimed by adding this milestone.

### Coverage assignments from the 2026-10-15 review

A plan review found Salesforce surfaces that no milestone owned. Each is now
assigned; deferred contracts live in `docs/compat/contracts/` and are validated
offline. Every item is `not_run` until reference observation.

| Area | Milestone | Contract / evidence |
| --- | --- | --- |
| OAuth 2.0/OIDC: token, authorize, revoke, introspect, discovery document, JWT bearer, client credentials, web-server, device and refresh flows; Connected Apps and External Client Apps; My Domain and sandbox login hosts; SOAP `login()`; SAML SSO | M3 (protocol), M4 (app/policy metadata), M5 (identity reconfiguration) | `identity.yaml` |
| Person Accounts (org-wide, irreversible) | M0 profile; M3/M6 behavior | profile `orgFeatures.personAccounts` |
| System/audit fields, `OwnerId` (User or Queue), audit-field import permission | M3, M5 | `rest-semantics.yaml` |
| Recycle bin: delete, undelete, `queryAll`, `getDeleted`/`getUpdated`, Bulk hard delete, retention | M3 | `rest-semantics.yaml` |
| Multi-currency and dated exchange rates | M0 profile; M3/M6 | profile `orgFeatures.multiCurrency` |
| Field types: compound Address/Name/Geolocation, dependent and global picklists, multi-select, auto-number, encrypted, lookup vs. master-detail, polymorphic `WhoId`/`WhatId` | M4 | catalog inventory at M0 |
| Legacy Workflow Rules/Process Builder, assignment/auto-response/escalation rules | M0 inventory; M7 | profile `orgFeatures.legacyAutomation` |
| Development lifecycle and Apex coverage gate | M4 | M0 inventory of customer pipelines |
| Email sending, templates, deliverability, Outlook/Gmail | M6/M7/M8 | M0 inventory |
| Org security settings (CORS, Remote Site Settings, CSP, IP ranges, sessions, MFA) | M4 | M0 inventory |
| `/limits`, `Sforce-Limit-Info`, API quotas; REST headers (`Sforce-Query-Options`, `If-Modified-Since`, ETags, `Sforce-Auto-Assign`, duplicate-rule header) | M3 | `rest-semantics.yaml` |
| Response and error shapes, upsert outcomes, describe properties, 15/18-character IDs, case-insensitive names and comparisons, null ordering, unknown-object errors, LIMIT above 2000 | M3 (M1 observation) | `rest-semantics.yaml` |
| Salesforce mobile app | M6 | declared certification exception |
| Field history, Setup Audit Trail, login history | M5/M9 | migration provenance |
| OpenAPI 2.0 External Service registrations, converted to OpenAPI 3.0 on import | M0 inventory; M5 conversion; M8 execution | profile `orgFeatures.externalServicesOpenApi2`; `external-services.yaml` (`external-openapi2-conversion`) |
| Served OpenAPI 3.0 contract with per-operation `x-source` provenance and traffic validation in HTTP tests | M3 | `api/openapi.yaml` |

M7 is likely the largest technical uncertainty. Prototype Apex language/runtime feasibility, Flow execution semantics, and customer component hosting during M1; do not wait until the rest of the product is built to discover that a required customization cannot run. Managed packages with unavailable source or platform services need a supported execution strategy, vendor cooperation, or an explicit blocker.

A pilot with no custom Apex or UI components may finish its required profile before full M7 breadth. That does not certify those omitted features. A pilot using them must wait for their verified implementation.

## First work package after roadmap review

This is a program roadmap, not an approved task-by-task implementation specification. Decompose it into independent subsystem specs before coding. The first detailed plan should cover M0/M1 and produce the following reviewable artifacts:

- `docs/compat/profiles/pilot.yaml`: exact source profile, mandatory features and client versions.
- `docs/compat/sources.yaml`: primary URLs, versions, access dates and provenance.
- `docs/compat/contracts/`: behavior contracts grouped by subsystem.
- `docs/compat/observations/`: sanitized reference outputs and reproduction instructions.
- `compat/clients/`: pinned JSforce, Python, CLI, Data Loader and Go probe definitions.
- `compat/scenarios/`: synthetic data setup and protocol/behavior/UI scenarios.
- `compat/reports/`: machine-readable differences and human-readable certification reports.
- `docs/migration/`: bundle schema, completeness rules and rehearsal runbook.

First vertical scenario: create a custom field through metadata, create an account/contact pair through a normal client, query the relationship, deny a restricted user's field access, prove a failed composite rolls back correctly, export data plus metadata, import into SalesCompel, and repeat the same client/UI assertions. This exposes dependencies between schema, records, security and migration early.

Proposed product boundaries once implementation is authorized: `cmd/salescompel`, `cmd/salescompel-worker`, `cmd/salescompel-migrate`, `internal/compat`, `internal/metadata`, `internal/records`, `internal/query`, `internal/policy`, `internal/automation`, `internal/events`, `internal/migration`, and `web`. These are planning boundaries, not scaffolding created in this pass. Ruling (2026-10-15): the M2 candidate deliberately keeps catalog, records, query and HTTP adapter in one `internal/platform` package while the surface is two fixed objects; split along these boundaries when M3/M4 add protocols or dynamic schema.

## Capacity, risks and decisions

Treat full ecosystem parity as a multi-year platform program until measured work proves otherwise. There is no defensible fixed completion date before the target inventory and runtime probes. Staff accountable ownership for protocol/query semantics, migration/storage, security/metadata, automation/runtime, UI/workflows, and verification/operations; roles may overlap but critical security and migration acceptance need independent review.

Estimate M2 onward after M1 using measured contract complexity and scenario throughput, separately for each subsystem. Budget permanent capacity for release drift and regression testing. Do not use endpoint count alone as a completion metric.

Key decisions for the next review are the first target organization/cloud, permitted migration reconfiguration, required API/class versions, mandatory managed packages, clean-room personnel separation, UI visual-fidelity expectations, hosting and scale objectives, and downtime/recovery targets. Current defaults are a Sales Cloud/core-platform pilot, endpoint/auth changes allowed, source IDs preserved externally, Vue native UI, and compatibility exceptions blocking a 1:1 claim.

## Additional source register

The linked research above supplies initial design evidence. These primary references expand the inventory and require version-specific extraction during M0/M1:

- [Salesforce documentation catalog](https://developer.salesforce.com/docs).
- [SOAP quick start and WSDL workflow](https://developer.salesforce.com/docs/platform/api/guide/sforce-api-quickstart-steps.html).
- [Metadata API guide](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/api_meta.pdf) — mutable latest URL; pin the downloaded release in the research register.
- [Apex developer guide](https://resources.docs.salesforce.com/latest/latest/en-us/sfdc/pdf/salesforce_apex_developer_guide.pdf) — resolve version differences against the target profile.
- [Trigger order of execution](https://developer.salesforce.com/docs/atlas.en-us.apexcode.meta/apexcode/apex_triggers_order_of_execution.htm) — dynamic page body not extracted in this pass.
- [Object reference](https://developer.salesforce.com/docs/atlas.en-us.object_reference.meta/object_reference/sforce_api_objects_list.htm) — dynamic page body not extracted in this pass.
- [Reports API](https://developer.salesforce.com/docs/analytics/salesforce-analytics-rest-api/guide/sforce-analytics-rest-api-resource-reference.html) and [dashboard running-user behavior](https://developer.salesforce.com/docs/analytics/salesforce-analytics-rest-api/guide/analytics-api-dashboard-results-resource.html).
- [GraphQL guide](https://developer.salesforce.com/docs/platform/graphql/guide).
- [LWC supported APIs](https://developer.salesforce.com/docs/platform/lwc/guide/get-started-supported-APIs.html) and [component behavior reference](https://developer.salesforce.com/docs/platform/lwc/guide/base-components-patterns.html).
- [Salesforce CLI large-data workflows](https://developer.salesforce.com/docs/platform/sfdx-dev/guide/sfdx-dev-data-bulk.html).

Review outcome required before implementation: agree the initial profile and architecture, then write and review the first subsystem specification and executable task plan. This roadmap itself does not claim that any compatibility milestone has shipped.

**Critical path (2026-10-15 review):** M2 work proceeded ahead of its M1 dependency. The candidate's list of behaviors awaiting reference evidence will keep growing until a disposable Developer Edition org exists, the maintainer records the terms-of-service risk decision, and observer/implementer roles are assigned (`CLEAN_ROOM.md`). Obtain these before expanding M3 implementation.
