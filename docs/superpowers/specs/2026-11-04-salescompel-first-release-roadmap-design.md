# SalesCompel — Design: first release roadmap (Plans 25 onward)

- **Status:** Accepted by the maintainer on 2026-11-04: exclude Apex, Flow and custom UI from the first release and add them later, with new teams and simple orgs as the audience. The first release is a `0.x` early preview. Plan numbers and order may change by maintainer decision without breaking the "Needs" column.
- **Date:** 2026-11-04
- **Scope:** the shortest path from the current candidate to an early preview people can use, and the order of work after it.
- **Inputs:** the [compatibility roadmap](../../plans/2026-10-14-salesforce-compatibility-roadmap.md), the [progress roadmap](../../plans/2026-10-20-progress-and-remaining-roadmap.md), and zeroauthy's [roadmap](https://github.com/occupysaas/zeroauthy/blob/main/docs/superpowers/specs/2026-10-19-zeroauthy-management-roadmap-design.md) and [v1.0 scope](https://github.com/occupysaas/zeroauthy/blob/main/docs/superpowers/specs/2026-11-02-zeroauthy-v1-release-scope.md) as the pattern.

This roadmap sets the order of work and what the first release must do. It does not replace the architecture, clean-room, licensing, localization or OpenAPI policies in the compatibility roadmap. M0–M10 gates stay as the definition of certification. The first release does not pass them and does not claim to.

## 0. Where we are (2026-11-04)

Plans 1–24 are recorded in the [progress roadmap](../../plans/2026-10-20-progress-and-remaining-roadmap.md) and `docs/compat/`, except that Plan 20 has a design and plan but no execution.

| Area | Works today |
| --- | --- |
| API | 20 candidate operations for Account and Contact: describe, CRUD, limited query, external-ID upsert, atomic Composite writes, served OpenAPI |
| Auth | Provisioned bearer tokens and `/services/oauth2/userinfo`; no OAuth login flows |
| Data | PostgreSQL with a fixed catalog of two objects and synthetic custom fields; tenant isolation in application code only |
| UI | A connected Account/Contact editor and a separate exploratory preview with synthetic data |
| Tests | One acceptance script (`scripts/check-candidate-acceptance.sh`) covering real clients, restart, browser and cleanup |
| Migration | Contracts and planning only |

**Why this roadmap exists.** After 24 plans the candidate has about 3,000 lines of production Go. The existing budget of 185–350 further plans before any release assumes that every gate closes first. Several plans are blocked on live evidence (5, 10, 14), and the M0 inventory waits on a customer inventory that does not exist.

zeroauthy shipped sooner by making plans feature-sized, naming the client that proves each one, logging gaps rather than waiting on them, and cutting a release from what already worked.

## 1. Goal and ground rules

**Goal.** A `0.x` early preview that a new team or a simple Salesforce org can run for real sales work, through its own UI and through unmodified standard clients. It tells anyone migrating exactly what will not carry over before they commit.

**Audience.**

| Fit | Who | First release behavior |
| --- | --- | --- |
| Good | New teams; small orgs using standard objects and custom fields with little automation | Works |
| Partial | Orgs with formulas, validation rules or Flows | Data and records move; automation is stored inactive and listed as a blocker |
| Not yet | Orgs relying on Apex, packages or custom screens | The dry-run report says so before any import |

**Non-goals for the first release:** certification, a 1:1 claim, hosted or multi-user preview operations, other clouds.

**Anti-goals, which make the release a failure even if it technically works:**

- Imported automation or security configuration silently dropped, or reported as working when it is not.
- Any wording that suggests universal Salesforce compatibility.
- A dry-run report that passes an org that then behaves differently because of a feature the report knew about.
- Salesforce kept as a hidden dependency.
- Paid tiers or billing, which the project's no-fee policy already rules out.

### Rules that change

| Today | From Plan 25 |
| --- | --- |
| Live evidence must be accepted before behavior is implemented | **Follow the documentation and log the gap.** Observe on the trial org when a session is available. Otherwise the contract stays at `documented`, the implementation proceeds, and the gap is listed on the compatibility page. |
| Complete M0 inventory before estimating and building | **Order by what a typical Sales Cloud org uses.** A full inventory waits until there is a real target org. |
| M1R archive review before the M3 contract freeze | **Review only relevant changelogs in each plan's first task.** The full archive review moves to certification. |
| Plans often answer one compatibility question | **One plan ships one feature**, including its storage, API, runtime, UI and tests. |
| Unsupported configuration has no defined state | **Stored, not enforced**, as defined below. |
| Nothing is explicitly out of scope | **§5 lists what comes later and why.** Anything can move by maintainer decision. |

**Stored, not enforced.** Metadata the candidate cannot yet run is accepted, retained and round-tripped so tools and later releases can use it. Every such item is marked in the API documentation and UI, and named in the dry-run report.

Limits on this state:

- It is never used for a plan's own feature at that plan's end.
- Security settings that are not enforced fail closed. A permission the candidate cannot evaluate denies access rather than granting it.
- Imported active automation becomes inactive, and that is reported as a blocker.
- A formula field's definition is retained, but copied values are not served as if they were live results.

**Parity rule.** Endpoints alone do not count. A permission that never affects a query, or a field that never reaches the UI, is not done.

### Every plan keeps the same shape

1. Review relevant Salesforce changelogs since the last plan.
2. **Task 1: observation.** Use the trial org where available, under `CLEAN_ROOM.md`. Where it is not available, follow the documentation and log the gap. This task never blocks the plan.
3. Storage changes through a forward-only migration, with an upgrade test.
4. Endpoints added to `api/candidate-openapi.yaml` with `x-source` provenance and traffic validation.
5. Runtime behavior, with tenant isolation and permissions applied.
6. UI where the feature is user-facing, localized through the catalogs.
7. Black-box tests with unmodified clients added to the acceptance script, so the script grows with the roadmap.
8. Documentation, compatibility page, decisions and follow-ups.

**Design depth.** **Short** plans can be planned directly from their section here after Task 1. **Full** plans need their own design first because they carry product decisions.

## 2. Order

Order follows dependencies first, then how much a simple org relies on the feature. The maintainer may reorder anything that does not break the "Needs" column.

| Plan | Stage | Feature | Depth | Needs |
| --- | --- | --- | --- | --- |
| 25 | A | Foundation decisions: settle open REST and external-ID behavior | Short | — |
| 26 | A | OAuth login and identity | Full | 25 |
| 27 | B | Forward migrations, row-level security, dynamic objects and fields | Full | 25 |
| 28 | B | Field types I | Short | 27 |
| 29 | A | SOQL engine | Full | 27, 28 |
| 30 | A | Record lifecycle: recycle bin, audit fields, limits and headers | Short | 27, 29 |
| 31 | A | Composite II and sObject Collections | Short | 29 |
| 32 | A | Bulk API 2.0 | Full | 29, 30 |
| 33 | B | Users, profiles and permission sets | Full | 26, 27, 29 |
| 34 | B | Sharing: org-wide defaults, role hierarchy, manual shares | Full | 33 |
| 35 | B | Metadata deploy and retrieve subset | Full | 28, 33 |
| 36 | C | Live application shell | Full | 26, 28, 33 |
| 37 | C | Leads and conversion | Short | 34, 36 |
| 38 | C | Opportunities and stages | Short | 34, 36 |
| 39 | C | Products and price books | Short | 38 |
| 40 | C | Activities and notes | Short | 36 |
| 41 | C | Campaigns | Short | 37, 38 |
| 42 | C | List views and search | Full | 29, 36 |
| 43 | C | Basic reports and dashboards | Full | 29, 34, 42 |
| 44 | D | Bundle validation and dry-run report | Full | 35 |
| 45 | D | Data load: IDs, owners, relationships, audit values, files | Full | 32, 44 |
| 46 | D | Reconciliation and export | Short | 45 |
| 47 | D | Connected extraction | Short | 44 |
| 48 | E | Security review and hardening | Short | All above |
| 49 | E | Release pipeline | Short | 48 |
| 50 | E | First-run docs, compatibility page, stability promise | Short | 49 |

**Budget.** These are bottom-up judgments, not measurements. They count feature-sized plans, so they are not directly comparable with the 185–350 plan budget, which assumed many narrower plans and full M0–M9 scope.

| Stage | Plans | Budget |
| --- | --- | --- |
| A. Client-ready core | 25, 26, 29–32 | 5–7 |
| B. Customizable data | 27, 28, 33–35 | 5–8 |
| C. Sales CRM | 36–43 | 6–8 |
| D. Migration | 44–47 | 3–5 |
| E. Release | 48–50 | 2–3 |
| **Total** | **26 listed** | **21–31**, or about 26–42 with the existing 25–35% reserve |

Re-estimate after Plans 29 and 36, which will show how large a feature-sized plan really is here.

## 3. Plans

### Stage A and B: a platform clients can use

**Plan 25 — Foundation decisions.** Close Plans 5, 10, 12 and 14 by choosing documented defaults. Complete the pending Plan 5 semantics. Choose external-ID lookup, stored spelling, uniqueness and collision handling, and resolve the missing-key race. Plan 12's two whitespace disagreements stay listed as gaps rather than blocking. The maintainer's waiver for long expiry windows stands. *E2E:* existing acceptance clients.

Implementation record, 2026-11-04: Plan 25 implemented and independently reviewed. Catalog v3, ID/name normalization, bounded query semantics and a required real-client foundation gate pass full candidate acceptance. [Decisions, gaps and verification](../../compat/plan-25-foundation-decisions.md). The whitespace lookup choice follows observed behavior; creation and uniqueness gaps remain recorded. Plan 29 owns pagination.

**Plan 26 — OAuth login and identity.** Web-server flow with PKCE, refresh, JWT bearer, client credentials, revoke, introspect, discovery, a minimal Connected App record, and correct `instance_url`. Provisioned tokens remain for tests. *E2E:* unmodified `sf org login` (web and JWT) using `--instance-url`, JSforce, and simple-salesforce CRUD using an OAuth token via `instance_url`/`session_id`. simple-salesforce's own login remains deferred with the routing proxy (§5 item 12). *Full design decides:* the login UI, how secrets are stored, and which flows the first release needs. SAML, device flow and My Domain hosts are later.

Implementation record, 2026-11-04: Plan 26 provides OAuth login, scoped expiring/revocable access, server-rendered localized consent, identity and CLI administration. Full candidate acceptance passes CLI web/JWT/refresh, JSforce, Python token-session CRUD, policy/isolation and refresh after restart. [Decisions, operator guide, gaps and verification](../../compat/plan-26-oauth-login.md). simple-salesforce's own login remains outside this delivery; no live reference parity is claimed.

**Plan 27 — Migrations, row-level security, dynamic schema.** Forward-only versioned migrations replace the startup catalog check. PostgreSQL row-level security for tenants. Tenant-scoped object and field definitions replace the fixed catalog, with Account and Contact becoming seeded standard objects. Split `internal/platform` along the planned boundaries here. *Full design decides:* physical storage, benchmarked as the compatibility roadmap requires. It must keep the seams automation will need later (§6). *E2E:* create a custom object and field, then use them through JSforce, across a restart.

Implementation record, 2026-11-05: Plan 27 implements explicit forward migrations and legacy adoption, forced tenant RLS, separate record/identity credentials, persistent tenant metadata, additive Text schema commands and a metadata-driven connected editor. Full acceptance passes unmodified client CRUD/query/Composite, schema persistence across restart, isolation and browser gates. [Operator guide, decisions and evidence](../../compat/plan-27-dynamic-schema.md). Plan 28 is next.

**Plan 28 — Field types I.** Text, numeric, currency (single currency), percent, date and datetime, checkbox, picklist and multi-select, email, phone and URL, lookup and master-detail, auto-number, and compound Name and Address. Decimal arithmetic throughout. Formula, rollup, encrypted and geolocation fields are stored, not enforced. *E2E:* describe and CRUD for each type through two clients.

**Plan 29 — SOQL engine.** Parser and AST validated against the catalog. Field and object security applied. Relationship traversal, polymorphic fields where seeded, filters, ordering with Salesforce null behavior, date literals, aggregates and grouping, pagination. Replaces the fixed `ParseQuery`. SOSL is in Plan 42. *E2E:* `sf data query`, JSforce and simple-salesforce with relationship queries and paging.

**Plan 30 — Record lifecycle.** System and audit fields, `OwnerId`, recycle bin, undelete, `queryAll`, `getUpdated`/`getDeleted`, 15/18-character IDs, `/limits`, `Sforce-Limit-Info`, conditional headers. *E2E:* delete, restore and query-all flows through clients.

**Plan 31 — Composite II and Collections.** Extends Plan 24 with `allOrNone: false`, reads and query subrequests, upsert, and sObject Collections. Composite Graph and Batch are if ready. *E2E:* JSforce collection calls and raw HTTP.

**Plan 32 — Bulk API 2.0.** Durable job table, worker process, ingest and query job lifecycles, abort, retries, result downloads, malformed CSV handling. *E2E:* `sf data import bulk`, `sf data export bulk`, JSforce and simple-salesforce bulk. *Full design decides:* where files live, since this is the first use of S3-compatible storage, and how the worker runs.

**Plan 33 — Users, profiles, permission sets.** User records, object and field permissions enforced in REST, Composite, Bulk and SOQL, and user language, locale and timezone settings. *E2E:* the access matrix, with a restricted user denied through real clients and no unexpected grants.

**Plan 34 — Sharing.** Org-wide defaults, ownership, role hierarchy, manual shares, and recalculation. Teams, territories, restriction rules and sharing rules beyond owner-based ones come later and fail closed. *E2E:* multi-role access matrix through clients and browser.

**Plan 35 — Metadata subset.** Deploy, retrieve, validate and status for CustomObject, CustomField, Profile, PermissionSet, Layout, RecordType and Translations, plus the minimum Tooling calls the CLI needs. Everything else in a package is stored, not enforced and reported in the deploy result. *E2E:* `sf project deploy start`, `retrieve start` and a failed deployment leaving correct state. *Full design decides:* the type list, how unknown types are retained, and the deploy result wording for stored types. Source tracking, scratch orgs, sandboxes and packages are later.

### Stage C: a CRM people can use

**Plan 36 — Live application shell.** Turns the exploratory preview's navigation and visual direction into a connected, metadata-driven app: sign-in, lists, record detail and edit, related lists, layouts, RTL, keyboard access, and the Plan 13 visual acceptance left open. The preview's "Planned" destinations remain disabled until their plan ships. *E2E:* Playwright, with each language in the catalog.

**Plan 37 — Leads and conversion.** Lead object, statuses, and conversion into Account, Contact and Opportunity. *E2E:* browser and API by role.

**Plan 38 — Opportunities.** Opportunity, stages, sales path, contact roles and stage history. *E2E:* browser journey, and report totals once Plan 43 lands.

**Plan 39 — Products and price books.** Products, price books, entries and opportunity line items, in a single currency. *E2E:* browser and API.

**Plan 40 — Activities and notes.** Tasks, events, notes and polymorphic `WhoId`/`WhatId`. Email sending is if ready. *E2E:* browser and API.

**Plan 41 — Campaigns.** Campaigns, members and attribution to opportunities. *E2E:* browser and API.

**Plan 42 — List views and search.** Saved list views with filters and sorting, global search, and a SOSL subset. PostgreSQL full-text search first; a dedicated search backend only if tests show the need. *Full design decides:* how closely relevance follows Salesforce.

**Plan 43 — Basic reports and dashboards.** Tabular and summary reports, simple dashboards, and running-user security. The Reports API is later. *E2E:* report totals under each role match the records that role can see. *Full design decides:* the report builder scope, which is the main size risk in this stage.

### Stage D: a way in and a way out

**Plan 44 — Dry-run report.** The product's most important honesty feature, and the source of demand data for §5. Accepts a backup ZIP or CSV set plus optional metadata. Validates hashes, paths, encodings and schemas, and classifies each item as supported, stored not enforced, absent, inaccessible or unsupported. Produces a human-readable and machine-readable report with no changes to target data. *E2E:* fixtures with Apex, Flow, formulas and malformed files each produce the correct blocker, never false success.

**Plan 45 — Data load.** Loads in dependency order, resolves cycles and polymorphic references, preserves source IDs at the boundary, maps owners and inactive users, loads files and notes, and keeps source audit values as provenance. Resumable and idempotent. The load path suppresses side effects. *E2E:* crash, restart and reimport produce no duplicates.

**Plan 46 — Reconciliation and export.** Counts, field hashes, relationships, ownership and file hashes, plus a portable SalesCompel export so the project creates no new lock-in. *E2E:* zero unexplained differences on the fixture org; export then reimport to a clean tenant.

**Plan 47 — Connected extraction.** The third entry path: extract data, metadata and files from an org the user authorizes, into the same bundle. Deltas and final-window cutover remain later. *E2E:* against the trial org where available, otherwise against SalesCompel itself as the source, with the limit logged.

### Stage E: release

**Plan 48 — Security review and hardening.** Whole-repository review with findings fixed, general rate limiting, `SECURITY.md`, and an oldest-schema upgrade test.

**Plan 49 — Release pipeline.** Semantic versioning starting at `0.1.0`, reproducible builds with checksums, a container image, upgrade and rollback notes, a changelog, and a `RELEASING.md`. Builds publish only from a release branch, never from `main`, with maintainer approval. Direct work on `main` continues until this plan adds the release branch; §10 records the replacement rule.

**Plan 50 — First-run documentation.** A quickstart, a deployment guide, a compatibility page that lists gaps first, "migrating from Salesforce", and a stability promise. Under `0.x`, the API, configuration and upgrade path change only in a new minor version, with a changelog entry and an upgrade path. The UI and internal packages are not covered.

## 4. Release tiers and exit criteria

**Must ship:** Plans 25–50.

**If ready, never waited for:** Composite Graph and Batch; outbound email from activities; quotes; multi-currency; more complete translations in existing catalogs.

**Exit criteria:**

- All must-ship plans are merged.
- The full Go race suite, the web tests, the offline lab check and the whole acceptance script pass on the release commit, with actual results, skips and gaps recorded.
- The security review has no open critical or high findings.
- A clean-machine quickstart passes, run by someone other than the docs' author.
- The fixture migration reconciles with zero unexplained differences, and every blocker fixture is reported correctly.
- The compatibility page and every stored-not-enforced item are accurate on the release commit.

**Languages.** The first release advertises only languages that have catalogs, currently English, French, German, Brazilian Portuguese and Hebrew. Partial catalogs stay visibly partial.

## 5. After the first release

This is the starting order, adjusted using blocker counts that people share from their dry-run reports. The first release adds no telemetry. Numbers are assigned when each is scheduled.

| Order | Work | Why here |
| --- | --- | --- |
| 1 | Formulas, validation rules, rollups | Likely to unlock the most existing orgs for the least work. Also a dependency for Flow and Apex. |
| 2 | Common Flow: order of execution, record-triggered automation | Flow is available below Enterprise, so it affects small orgs. Resume Plan 20 first. |
| 3 | Remaining Flow, approvals, legacy Workflow Rules and Process Builder, assignment and escalation rules | The rest of declarative automation |
| 4 | Multi-currency, Person Accounts, email sending and deliverability, forecasting, territories | Sales features that some orgs need, if not already shipped |
| 5 | Deltas and cutover rehearsal | Needed for live migrations of busy orgs |
| 6 | Apex | Largest technical uncertainty. Plan 19 is the starting point. |
| 7 | SOAP and the Data Loader tool | Remaining legacy clients |
| 8 | Jobs, Pub/Sub, CDC, Streaming, outbound messages | Integration events |
| 9 | UI API, GraphQL, Reports API, Connect, External Services | Remaining integration surfaces |
| 10 | Development lifecycle: source tracking, scratch orgs, sandboxes, packages | CLI and team workflows |
| 11 | LWC, Aura, Visualforce, managed packages | Plans 21–22 are the starting point. Real-package execution remains unproven. |
| 12 | CLI matrix: installer and Docker channels, plugin runner, routing proxy | Build the proxy sooner if a pinned client hardcodes a host |
| 13 | Full M1R archive, full M0 profile, M9 certification | Certification against a named profile, with all original gates |
| 14 | M10 tracks: Service Cloud and others | Each with its own inventory |

**Parked now:** Plan 20 keeps its number, design and plan, and is not executed until item 2.

**Not planned:** a SalesCompel-specific CLI plugin as a substitute for compatibility; proxying to real Salesforce; paid tiers.

## 6. Keeping later work possible

Automation is deferred, not ignored. Plans 27 and 29–33 must keep these seams, without building the features behind them:

- **One write path.** Every REST, Composite, Bulk, UI and import write goes through one record service, so transaction-ordered hooks can be added at one point.
- **Explicit execution context.** User, tenant, API version, privilege mode and sharing mode are passed through the record and query services.
- **Transaction scope.** A single request's writes can share one database transaction across multiple objects.
- **Metadata retention.** Unknown or inactive metadata is retained exactly enough to activate later.
- **Transactional job outbox.** Plan 32's job table is designed so events can later be added without changing the write path.

## 7. How the old plan maps to this one

| Milestone | In the first release | Later |
| --- | --- | --- |
| M0 | Typical-org assumptions and the dry-run classifier | Full named profile |
| M1 | Existing laboratory; pinned npm CLI, JSforce and Python clients | Plugin runner, Docker and proxy |
| M1R | Per-plan changelog reviews | Full archive review |
| M2 | Remaining acceptance closed by Plans 27 and 36 | — |
| M3 | Plans 25, 26 and 29–32 | SOAP, SAML, device flow, remaining surfaces |
| M4 | Plans 27, 28 and 33–35 | Remaining field types, development lifecycle, org security settings |
| M5 | Plans 44–47 | Deltas, cutover, history and audit-trail mappings |
| M6 | Plans 36–43 | Forecasting, territories, multi-currency, Person Accounts, email |
| M7 | Seams only (§6); feasibility results retained | §5 items 1–3, 6 and 11 |
| M8 | None | §5 items 8–9 |
| M9 | Plans 48–50 as a release, not certification | Item 13 |
| M10 | None | Item 14 |

## 8. Evidence and calls made in this draft

**Edition availability, checked 2026-11-04.** The official Salesforce pages found were older versioned documents, and no current edition matrix was retrieved. Confirm each before publishing audience claims.

| Feature | Finding | Confidence |
| --- | --- | --- |
| Apex | Salesforce's Apex guide lists Enterprise, Performance, Unlimited, Developer and Database.com ([v244 guide](https://developer.salesforce.com/docs/atlas.en-us.244.0.apexcode.meta/pages/apex_intro_what_is_apex.htm)). | Official, older |
| Flow | Salesforce's documentation lists Essentials, Professional and above ([Visual Workflow guide, v220](https://resources.docs.salesforce.com/220/latest/en-us/sfdc/pdf/salesforce_vpm_implementation_guide.pdf)). Pro Suite includes flow automation per the [navigation inventory](../../compat/ui/navigation-inventory.md), from a third-party source. A reported limit on flows in lower editions is unconfirmed. | Official, older |
| LWC and Visualforce | **Unconfirmed.** Sources found conflict or are out of date. | Low |
| Formulas and validation rules | **Unconfirmed** by this search. Their position in §5 rests on judgment that they are common in small orgs. | Low |

**Calls made in this draft, for the maintainer to check:**

1. Plan list, numbering, order and budgets.
2. Using zeroauthy's label "early preview" for the release.
3. Formula fields retain their definition but do not serve copied values as live results.
4. Row-level security and migrations move to the start of stage B.
5. Reports and dashboards are required, in basic form, because a simple sales team expects them. This is the easiest item to cut if the budget slips.
6. Sharing is required rather than deferred, because otherwise the first release would expose records too widely for real team use.
7. No telemetry; demand data comes from reports people choose to share.
8. Plan 20 is parked rather than finished, although it is the last unrun feasibility probe.

**Left to each plan's builder:** details not named here, including the exact client commands, within the rule that a real unmodified client proves each feature.

## 9. Risks

- **Size.** Plans 27, 29, 35, 43 and 45 are large. Their designs should split them rather than let them grow. Salesforce is a much larger target than Auth0, so zeroauthy's pace is not a forecast here.
- **Documentation-only behavior.** Some contracts will ship unobserved and may be wrong. The compatibility page says which, and correcting one to match Salesforce counts as a fix rather than a break.
- **Later rework.** Plans 19, 21 and 22 passed only on synthetic subsets. They do not prove Apex, custom UI or real packages will fit. §6 reduces this risk but does not remove it.
- **Report accuracy.** A dry-run report that misses a blocker does more damage than a missing feature. Plan 44's fixtures must grow whenever a new metadata type is classified.
- **Moving target.** Salesforce ships three releases a year. Per-plan changelog reviews keep this visible.
- **Expectations.** A public preview may be read as a promise of parity. The first thing on the compatibility page must be what does not work.

## 10. Maintainer decisions (2026-11-04)

1. **Version label:** the first release is a `0.x` early preview, not `1.0`. Far more of the product is still missing than at zeroauthy's 1.0.
2. **Branch policy:** the rule that all work stays on `main` until M0–M10 finish is replaced. Direct work on `main` continues until Plan 49 adds the release branch. From then on, publishing happens only from that branch. How day-to-day work reaches `main` after that is settled in Plan 49.
3. **Older roadmaps:** both older roadmaps and the README point to this document for the current order of work.
