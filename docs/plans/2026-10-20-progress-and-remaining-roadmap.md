# SalesCompel progress and remaining roadmap

As of 2026-10-20. Brainstorming draft for review, based on repository decisions through completed Plan 11 and drafted Plan 12. This supplements the [original roadmap](2026-10-14-salesforce-compatibility-roadmap.md); it does not reduce its acceptance gates or authorize implementation.

## What is complete

Completed work packages are real progress, but do not equal completed program milestones. No full M0–M10 exit-gate pass is recorded in the reviewed repository documents.

| Work package | State | What it delivered |
| --- | --- | --- |
| Plan 1 | Offline laboratory complete | Contracts, pinned clients, capture/comparison scaffolding and validation |
| Plan 2 | Candidate walking slice implemented | Go/PostgreSQL Account/Contact service, tenant/auth boundary, initial REST/query and Vue workflow; full milestone acceptance remains open |
| Plan 3 | Readiness tooling complete | Authorization/readiness and evidence workflow; superseded in part by later live baseline work |
| Plan 4 | Pilot reference baseline accepted | Two repeatable captures covering eleven pilot scenarios; broader compatibility remains open |
| Plan 5 | Partial; contract gate open | Focused REST/query probes; product Tasks 4–8 pending evidence decisions |
| Plan 6 | Discovery tooling complete; inventories incomplete | Metadata observations; missing facts block formal stability/security conclusions |
| Plan 7 | Bounded implementation complete | Synthetic field/grant declarations; no general metadata deployment engine |
| Plan 8 | Bounded implementation complete | Active field lengths, catalog upgrade and validation alignment |
| Plan 9 | Probe work complete; contract drift retained | External-ID observations and recovery; equality/race product work remains |
| Plan 10 | Offline follow-up tooling complete; live decision blocked | Corrected foundation collection/assessment policy; no fresh accepted pair |
| Plan 11 | Evidence, implementation and review complete | Measured external-ID body validation implemented |
| Plan 12 | Spec/plan drafted; not executed | Proposed offline equality feasibility and collision analysis |
| Plan 13 | UI prototype implemented; automated checks pass; visual acceptance open | Routed local Account/Contact exploration, Zeroauthy visual family, Pinia, Router and i18n |

Sources: [Plan 2](../compat/plan-2-implementation.md), [Plan 4](../compat/plan-4-reference-baseline.md), [Plan 5](../compat/plan-5-contract-decisions.md), [Plan 6](../compat/plan-6-metadata-discovery.md), [Plan 7](../compat/plan-7-declared-metadata.md), [Plan 8](../compat/plan-8-active-field-metadata.md), [Plan 9](../compat/plan-9-external-id-contract.md), [Plan 10](../compat/plan-10-foundation-decision.md), [Plan 11](../compat/plan-11-external-id-body-validation.md), [Plan 12](../superpowers/plans/2026-10-20-plan-12-external-id-equality.md).

## How to read the estimates

A plan means one bounded subsystem deliverable with its evidence/design, implementation, meaningful tests and review, similar to recent numbered plans. One plan is not one task, endpoint, day or milestone. Research-only plans count when they produce a decision that unlocks implementation. Use the upper half if plans remain as narrow as Plans 7, 8 and 11.

These are bottom-up planning judgments, not measured delivery forecasts or confidence intervals. Count remaining work only, including Plan 12 and unfinished portions of existing plans. Resuming an old plan consumes work within the estimate; it does not require inventing a new plan number. Assign each shared deliverable to one primary milestone to avoid counting it twice. Client testing, accessibility, localization and security belong inside feature plans; their specialized infrastructure/final acceptance remains separately budgeted below.

Assume the existing Sales Cloud/core-platform target, one pinned initial profile and required in-profile customizations. Do not assume Apex, packages or custom UI are absent. Exact customer inventory can increase or decrease scope. Full M1R archive obligations remain included. Additional clouds belong to M10 and are excluded from the M0–M9 subtotal.

## Remaining milestones and plan budgets

| Milestone | Current position | Main remaining deliverables | Additional plan equivalents |
| --- | --- | --- | ---: |
| M0 — Target/inventory | Partial | Complete object/feature/package/client inventories; required workflows; supported versions/languages; scale, downtime and ownership decisions | 4–7 |
| M1 — Behavior laboratory | Partial; pilot baseline accepted | CLI/plugin isolation and runners, routing proxy, broader connectors, evidence coverage and repeatability, recovery hardening | 6–10 |
| M1R — Release history | Partial; required findings unresolved | Complete archive inventory, review batches, mapped regressions, unresolved-source decisions and profile reconciliation | 6–12 |
| M2 — Platform walking slice | Implemented slice; gate open | Consolidated client/UI/isolation/restart acceptance; close localization foundation gaps and document remaining exclusions | 2–4 |
| M3 — Protocol/query | Early implementation; foundation gate open | REST/query fidelity, external-ID equality/migration/races, transactions/Composite, SOAP, Bulk, OAuth/OIDC, recycle bin/audit semantics, limits/headers, served OpenAPI and traffic validation | 20–35 |
| M4 — Metadata/security | Small declaration/catalog slices implemented | Dynamic schema and field families, relationships, metadata deploy/retrieve, Tooling/development lifecycle, permissions/sharing, RLS, security settings and migrations | 24–40 |
| M5 — Migration | Planning/contracts | Three import paths, bundle validation, identities/relationships, files/history/audit/translation metadata, resumability, deltas, reconciliation and export | 12–20 |
| M6 — Sales workflows/UI | Minimal record UI | Sales object families, lifecycle workflows, products/pricing/campaigns/activities, search/reporting, forecasting/territories as profiled, email, localization and accessible UI acceptance | 18–30 |
| M7 — Automation/customization | Contract outlines; major feasibility work remains | Formula/validation/rollups, transaction order, Flow/approvals/legacy rules, Apex/runtime/security/limits, async execution, LWC/Aura/Visualforce and required packages | 30–60 |
| M8 — Integrations/events | Contract outlines | UI API/GraphQL/Reports/Connect, jobs/outbox, Pub/Sub/CDC/Streaming, replay/reconnect, outbound messages and External Services | 16–28 |
| M9 — Production certification | Not reached | Integrated rehearsals, performance/recovery/security, operator runbooks, cutover/rollback, full profile/client/language certification | 8–14 |
| **M0–M9 remaining subtotal** | **All gates still open** | **Initial Sales Cloud/core-platform program** | **146–260** |
| M10 — Wider coverage | Discovery/scoping remains | Independent inventory, implementation and certification for each additional cloud/package/release profile | **Unbounded until tracks are inventoried** |

Arithmetic: 146–260 is the sum of the eleven M0–M9 rows, including M1R. Add a 25–35% program reserve for discovered contracts and redesign: approximately **185–350 remaining plan equivalents through M9**. Reserve is not another set of already identified features. The range does not include indefinite future release maintenance or a guarantee that every proprietary package can be supported.

M7 can exceed its allocation if runtime feasibility fails or package behavior requires a different strategy. M4's development lifecycle and broad M1R source review can also grow substantially. A larger count may reflect useful discovery rather than poor execution.

## What “all milestones” could cost

M10 has no fixed completion boundary in the original roadmap: new clouds, packages and releases create new profiles. A finite total requires an explicit coverage cutoff and inventory. Keep that visible rather than treating M9 as universal parity.

For scenario budgeting only, allocate **30–80 additional plans per substantial cloud track** after reusable core infrastructure exists; difficult products may exceed this. If the chosen finish line contained eight such tracks, that would add **240–640 plans before a separate M10 reserve**. Combined with the reserved M0–M9 budget, the illustrative total becomes **425–990 remaining plans**, excluding M10 contingency and ongoing maintenance. This is multiplication of assumptions, not an estimate of eight inventoried products. Do not use it as a completion promise.

A defensible working budget today is therefore **roughly 200–350 plans to the first fully certified Sales Cloud/core-platform target**, with wider ecosystem completion budgeted separately. The lower end must not be achieved by silently dropping required customizations, languages, security or integrations.

## Recommended sequence

1. **Close the current evidence loop.** Execute proposed Plan 12 after review; resume Plan 10's actual identity/session/recovery prerequisites and fresh evidence pair; resolve required release findings. Plan 11's working administrator credential handoff does not prove the distinct-session/restricted prerequisites are satisfied. Reuse existing plans where possible.
2. **Choose runtime behavior deliberately.** Design and implement external-ID lookup/create normalization, stored spelling, shared uniqueness/locking and collision-blocking migration. Resolve missing-key races independently. Complete the remaining Plan 5 semantics against accepted evidence.
3. **Finish the target inventory and test the hard feasibility questions early.** Run bounded Apex, Flow, imported UI and package probes before investing in the whole CRM surface. These are M7-owned plans started early, already included in its budget. Complete M0 and the remaining M2 acceptance evidence alongside this work.
4. **Build protocol, metadata and policy together through small vertical slices.** Establish M3/M4 foundations, then import one meaningful data/metadata slice through M5. Every slice includes client tests, permissions, restart behavior and required locale handling.
5. **Expand business workflows, automation and integrations.** Develop M6–M8 against the same migrated fixtures and versioned profile. Avoid postponing transaction/security integration until certification.
6. **Certify the named profile.** Complete M9 after all required dependencies pass; preserve explicit blockers and exceptions. Then repeat inventory-to-certification for each bounded M10 track.

The first 8–12 plan equivalents should buy decisions and closed foundation contracts, not merely more probe infrastructure. This is an allocation within the subtotal, not an additional budget. Their exact numbering depends on which existing plans can be resumed.

## Progress reporting and re-estimation

Track each contract as unknown, observed, specified, implemented, verified or blocked; track milestone exit gates separately. Publish accepted work, remaining blockers and plan-budget changes after each five completed plans. Re-estimate M3/M4 after the foundation contract closes, M7 after feasibility probes, and the whole program after complete M0 inventory.

Do not calculate percent complete as 11 divided by a projected plan total: plans differ greatly in size and some existing plans are partial. Calendar estimates need measured plan throughput, staffing and actual blocker time. Preserve the no-fee Apache-2.0 product policy and existing direct-on-main branch policy.


## Early UI/UX track — maintainer direction, 2026-10-20

The maintainer accepted the recommended roadmap sequence and requested an early UI/UX for interested people. Bring a bounded part of M2/M6 forward alongside foundation work. Early human evaluation is now an explicit goal; this supersedes the original roadmap's statement that human use/testing must wait until all milestones are complete. It does not change the direct-on-main branch policy or certify the preview for production.

Proposed outcome: a coherent SalesCompel experience that interested people can understand, try and discuss while the compatibility platform develops. Build on the existing Vue application and Go/PostgreSQL candidate. Current real functionality covers Account/Contact record workflows; additional workflows need an explicit choice between demonstrated concepts and working persistence. The maintainer selected exploration of proposed screens grounded in observed behavior, known API routes and implemented API routes. The first deliverable is an exploratory prototype for feedback. It need not persist edits or connect to a live backend. Delivery format and the first screen set remain to be settled.

Suggested sequence, subject to that choice:

| Early package | Outcome | Budget/source |
| --- | --- | --- |
| Experience definition and prototype | Agree audience, primary task, navigation and visual direction using a small clickable prototype | 1 plan from M6 |
| Application shell and record workflow | Establish reusable navigation, lists, record detail/edit, responsive layout, keyboard access and existing language support | 1–2 plans from M2/M6 |
| Coherent demonstration journey | Seed synthetic data and complete one agreed journey with clear empty/loading/error/success states; make working versus proposed behavior clear | 1 plan from M6 |
| Preview delivery and feedback | Package a repeatable preview, define data reset/isolation, gather structured feedback and map findings into the backlog | 1 plan; local packaging overlaps M9, hosted multi-user delivery may add work |

Budget approximately **4–5 plans** for a bounded first preview, mainly pulled forward from existing allocations. Public hosted access, self-service accounts or multi-user preview operations could require **1–2 additional plans** after scope is known; these are provisional additions, not part of the existing subtotal. Do not build a general hosting/account platform merely to show screens.

Keep foundation/evidence work moving alongside the preview. Feed validated UI components into the eventual product; treat throwaway prototypes as exploratory artifacts. Expand the preview only when the next user journey has a clear purpose. Avoid advertising screens backed by illustrative data as completed platform capabilities.


### Selected preview direction — exploratory screens

Use three separate facts when proposing a screen or interaction: what reference behavior has actually been observed, what API contract/route is known, and what the candidate implements. A known route alone does not establish observed behavior or implementation. Record unknowns and blocked evidence explicitly; Plans 5/10 do not become accepted contracts because a prototype illustrates them.

Maintain a design evidence matrix alongside the prototype: screen/task, evidence source, relevant route/method, observation status, candidate implementation status, simulated behavior and unanswered design questions. Keep detailed provenance in reviewer notes or an optional evidence view. Show participants a concise prototype label and explain simulation where it affects their interpretation, without filling ordinary screens with engineering details.

Recommended first scope uses the existing Account/Contact slice: application navigation, record lists, record detail, new/edit forms, Account–Contact relationships where supported, and representative empty/loading/validation/access-denied states. Verify the relevant evidence and routes before including each interaction. List search, sorting, filters, pagination, relationship edits and permission-dependent controls are proposals until their specific support is mapped. Do not invent working dashboards, pipelines or automation merely to populate navigation.

Use synthetic data and simulated transitions for exploration. Live API integration, authentication provisioning, hosted multi-user operation and persistent writes are separate follow-ups rather than prerequisites for this preview. Retain reusable Vue components only when they suit the approved product design. Initial feedback should test navigation comprehension, finding a record, understanding relationships and completing a proposed create/edit journey.

Refine the earlier preview budget to **3–4 plans**: (1) route/evidence inventory plus screen map; (2) visual direction and navigable prototype; (3) representative interaction states and feedback walkthrough; optionally (4) iteration after feedback. These replace the earlier working-preview allocation for this selected scope and are primarily M6 work brought forward. No additional hosting budget is assumed.


### UI reference and architecture — maintainer direction, 2026-10-20

The maintainer identified sibling `../zeroauthy` and its [console-frame design](https://github.com/occupysaas/zeroauthy/blob/main/docs/superpowers/specs/2026-10-19-zeroauthy-console-frame-design.md) as the reference for a similar palette and fonts, and explicitly requires Pinia, i18n and Vue Router. Local `console/src/ui/app.css`, `console/package.json`, font assets/license and the cited design were inspected. The sibling repository is reference material; do not change it as part of SalesCompel work.

Carry forward the visual family: white surfaces, ink `#111111`, red `#d7141a` with hover `#a50e13`, black header, strong sidebar rule, bold readable navigation, red active indicator and uppercase letter-spaced headings. Use locally bundled Glacial Indifference regular/bold, retaining its SIL OFL license and adding appropriate SalesCompel attribution. Use a system monospace stack for technical values. Proposed light/dark support follows Zeroauthy's dark surface `#121212` and accent `#ff3b40`, subject to contrast and browser review. Keep SalesCompel naming and CRM-specific information architecture.

Build within the existing Vue 3/TypeScript/Vite application. Pinia owns shared preferences and preview state; Vue Router owns navigation and selected record identity. Keep transient form state local unless it must survive route changes. No access tokens belong in Pinia or browser persistence. Proposed implementation choices: use vue-i18n Composition API and token-based styling consistent with the reference; adoption of Tailwind is left to the dedicated UI design rather than inferred from the request. Existing SalesCompel package versions remain in place unless a separately justified change is needed.

Preserve SalesCompel's `locales/languages.json` registry, one JSON file per language/variant under `locales/messages`, Salesforce wire codes, explicit fallback and BCP 47 mappings. Do not replace its existing English/French/German/Brazilian Portuguese/Hebrew catalogs with Zeroauthy's English-only arrangement. All new headings, navigation, labels, empty/error states, prototype notices and accessibility text are translatable. Set document lang/dir, support RTL, use logical layout properties, and make the language menu usable with keyboard and assistive technology. Existing partial catalogs remain visibly partial; fallback is not translation completion. Any i18n-library migration preserves the existing language-selection race protection and tests.

Use routed prototype screens under a distinct preview entry such as `/preview`, keeping the current connected Account/Contact workflow available. Recommend hash-based prototype navigation initially, because the current Go static-file handler must not be assumed to provide history-route fallback. The dedicated design must verify asset serving and direct-link/reload behavior before choosing router history. A preview badge and optional evidence view distinguish simulated UI from connected functionality. Navigation, record selection, tabs and state demonstrations may work locally; no preview interaction dispatches a record mutation or requests credentials.

Zeroauthy's disabled planned-page controls and empty-only tables are reference-project decisions, not requirements inherited by SalesCompel. This maintainer selected exploratory screens, so synthetic rows and local simulated interactions are permitted when clearly identified. Do not import Auth0-specific sections, API paths or tenant/session assumptions. Route/evidence mapping still governs which CRM fields and actions are proposed.

Plan 13 execution: the first early UI package is implemented with automated preview and existing connected browser verification. Fresh code-review findings were corrected with regressions. Interactive visual review was denied and remains an explicit acceptance gap; see [preview walkthrough](../compat/ui/preview-walkthrough.md). This does not close the full M2/M6 gates or change the remaining platform budgets.
