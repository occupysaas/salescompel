# Plan 13 — Evidence-led exploratory CRM UI

Status: approved for native implementation, 2026-10-20; implementation verification in progress. First early UI package, independent of Plan 12 execution. The maintainer selected exploratory proposed screens, Zeroauthy's visual family, Pinia, Vue Router and i18n. This document settles the first bounded deliverable; it does not certify backend parity.

## Intent

Let interested people explore Account/Contact screens and give useful feedback while foundation work continues. Start with the actual candidate surface and reference observations, rather than inventing a complete CRM. Deliver a navigable prototype within the existing Vue application, using synthetic data and local interactions. Preserve the existing connected workflow.

The first package combines the route/evidence inventory, shared visual frame and Account/Contact prototype. It is one of the roadmap's proposed 3–4 UI plans; later packages can expand states and incorporate feedback. No deployment or live reference collection is required.

## Visual and technical direction

Reference: sibling `../zeroauthy/console/src/ui/app.css` and [console-frame design](https://github.com/occupysaas/zeroauthy/blob/main/docs/superpowers/specs/2026-10-19-zeroauthy-console-frame-design.md), inspected locally. Do not modify that repository.

Use white surfaces, ink #111111, red #d7141a (hover #a50e13), black header with white text, a strong sidebar rule, bold readable navigation and a red active indicator. Headings are uppercase, bold and letter-spaced. Bundle Glacial Indifference regular/bold from the sibling assets with their SIL OFL license and SalesCompel NOTICE attribution. Use system monospace for IDs and paths. Dark mode uses #121212 surfaces, light text and #ff3b40 accent, with black header. Provide system/light/dark choice; verify contrast, visible focus and WCAG 2.2 AA behavior. Retain SalesCompel branding and CRM information architecture.

Keep existing Vue/TypeScript/Vite versions. Add Pinia and Vue Router; use vue-i18n Composition API for the common translation runtime. Resolve compatible versions at implementation and pin them in the existing lockfile; do not upgrade the existing framework/toolchain incidentally. Use CSS custom-property tokens and scoped preview component styles; Tailwind is not required by this design. Keep connected-page styling intact apart from shared font/preference primitives where compatible. No theme/font CDN.

Pinia owns shared preferences and preview scenarios/data. Router owns object/record selection. Form drafts stay local. No tokens enter Pinia, URLs or storage. Persist only locale/theme with storage-failure fallback. Synthetic edits exist only in memory and reset on reload or Reset preview. No real tenant or session model is introduced.

## Routes and boundaries

Use Vue Router hash history: the Go asset handler serves index only at `/` and does not provide SPA history fallback. Do not change the server to support preview routing.

| Hash route | Page |
| --- | --- |
| `#/` | Existing connected application, preserved as ConnectedApp.vue |
| `#/preview` | Redirect to preview Accounts |
| `#/preview/accounts` | Account list |
| `#/preview/accounts/new` | Simulated new Account form |
| `#/preview/accounts/:id` | Account detail |
| `#/preview/accounts/:id/edit` | Simulated Account edit |
| `#/preview/contacts` | Contact list |
| `#/preview/contacts/new` | Simulated new Contact form |
| `#/preview/contacts/:id` | Contact detail |
| `#/preview/contacts/:id/edit` | Simulated Contact edit |
| `#/preview/evidence` | Screen/interaction support notes |

Resolve new before dynamic ID. Unknown IDs and unknown routes show an accessible not-found state with links to lists. Detail/edit URLs survive reload using fixed synthetic fixture IDs; newly created records disappear on reload and their URLs then show not-found. Reset returns to Accounts and announces completion. Browser back/forward and mobile navigation work. A connected-page Preview link and preview Return to connected app link make both modes discoverable. Leaving the connected route unmounts it and clears its in-memory token; no network or connected component mounts under preview.

## Screens and interactions

Use a shared header with brand, prototype badge, language and theme controls; sidebar Accounts, Contacts and Evidence; responsive drawer on narrow screens. Drawer supports keyboard close, focus return and no focus trap after closing. Avoid dashboard/pipeline/settings navigation without a design basis.

Lists show clearly synthetic rows, a translatable title and New action. Columns: Account Name and Compat_Key__c; Contact FirstName, LastName and linked Account.Name. IDs are secondary technical values. Do not expose Compat_Text__c in this first preview: its access semantics are deliberately narrow and incomplete. No global search, arbitrary filters, pagination, bulk actions or sortable columns are implied.

Details show the corresponding fields, Edit and list navigation. A Contact links to its Account. Account detail can show matching synthetic Contacts as a locally computed related list, explicitly classified as a proposed UI composition, not an implemented reverse-relationship endpoint. Forms use only these same fields; Contact AccountId is a synthetic account picker, not evidence of a native lookup-search API. Name/LastName are required in the proposed UI. Saving changes local fixtures and returns to detail with a simulation notice; Cancel discards the local draft. No delete, upsert editor or production auth screen in preview. Restrict local keys to 80 characters as a candidate-aligned UI rule; do not claim reference Unicode length semantics.

A persistent concise notice says interactions use sample data and do not save to a server. Simulation is a prototype property even when the related candidate API exists. The evidence page carries detailed provenance rather than placing API details in normal workflows. No fake success response represents a real request.

## Evidence matrix

Create `docs/compat/ui/preview-evidence.md` before building screens. For each screen/action record route/method, reference evidence source and status, candidate implementation source/status, proposed UI composition and simulation behavior separately.

Minimum mapping:
- Lists: GET `/services/data/v65.0/query`; actual candidate supports explicit fields, one equality predicate, one ordering field, LIMIT 1..2000, no cursor. Sources: Plan 2 and `internal/platform/http.go`/query code. Do not promote blocked Plan 5/10 contracts.
- Field definitions: GET `/services/data/v65.0/sobjects/{Account|Contact}/describe`; distinguish Plan 8 implemented length metadata from general metadata parity.
- Detail: GET `/services/data/v65.0/sobjects/{object}/{id}`.
- New: POST `/services/data/v65.0/sobjects/{object}`; edit: PATCH `/services/data/v65.0/sobjects/{object}/{id}`. They exist but preview sends neither.
- Contact Account.Name projection is candidate-supported; Account-related Contacts use a proposed list composed from local fixtures, with possible equality-query support documented separately.
- Empty, validation and not-found presentation are design proposals. Use references only for actual measured underlying conditions. Prototype reset, theme, language and navigation are product UX, not Salesforce parity claims.

Use Plans 2/4/8/9/11 only within their stated limits. Known API paths from OpenAPI may remain proposed; an entry in api/openapi.yaml alone does not establish implementation or live observation. No new private evidence reads or external reference calls are needed.

## Internationalization

Keep `locales/languages.json`, Salesforce codes en_US/fr/de/pt_BR/iw, explicit fallback, and one JSON message file per language/variant. Preserve BCP 47 lang and RTL dir mapping, draft coverage labels and latest-selection-wins asynchronous loading. All new labels, notices, accessible names, validation and empty states use keys. English has complete new keys; other catalogs may use explicit English fallback without being marked translated. Avoid copying English into other files as fabricated translation coverage. Existing connected translations remain available through the compatible t/setLanguage exports.

Use saved valid locale, otherwise en_US for this package; browser-language negotiation is deferred. Set lang/dir on committed locale changes. Use logical CSS for RTL. Synthetic proper names and API identifiers are data, not translated UI strings. Locale-aware formatting must use explicit selected locale if introduced; no dates/currencies are required initially.

## Verification and completion

Meaningful tests prove route refresh/back/forward/unknown IDs, preview network isolation, local-only edits/reset, connected workflow regression, theme/locale persistence failure handling, language race safety, fallback and Hebrew RTL. Use existing Playwright infrastructure and Node unit tests; do not add a second test framework solely for this slice.

Run web test/build/e2e and git diff --check. Existing connected e2e setup may need PostgreSQL/Go; report skipped connected verification explicitly. Preview tests must run without tokens/database. Inspect browser screenshots at desktop and 390px mobile, light/dark, English and Hebrew, including open drawer and forms; resolve overflow/focus/contrast problems. Obtain fresh whole-change review. Publish evidence matrix and preview walkthrough with actual verification results. No backend, API parity, production readiness or milestone gate completion claims.

## Calls left to the builder

Component boundaries, secondary token shades, spacing and icon choices are implementation decisions within this design. Verify font attribution from the local OFL before copying assets. Exact dependency versions are chosen and locked during execution. No hosting, broad new CRM sections, backend changes or live API integration belongs in this package.

## Builder check

Fresh design review found no important unanswered planning questions. Token cleanup can use existing setToken("") on connected-component unmount; no API redesign is needed.
