# Plan 13 Exploratory CRM UI Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans or superpowers:subagent-driven-development to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Deliver an evidence-led Account/Contact prototype in the Zeroauthy visual family without changing backend behavior.

**Architecture:** Hash-routed preview pages use local synthetic fixtures in Pinia, shared i18n and preference primitives, and token-based styling. The original connected application remains a separate route. An evidence matrix records observed, known, implemented and simulated behavior independently.

**Tech Stack:** Existing Vue 3, TypeScript, Vite, Node test runner and Playwright; add pinned compatible Pinia, Vue Router and vue-i18n.

**Spec:** `docs/superpowers/specs/2026-10-20-plan-13-exploratory-ui-design.md`.

## Global Constraints

- No backend, API parity, production readiness or milestone gate completion claims.
- No new private evidence reads or external reference calls are needed.
- No tokens enter Pinia, URLs or storage.
- Persist only locale/theme with storage-failure fallback.
- No theme/font CDN.
- Keep existing Vue/TypeScript/Vite versions.
- Preview tests must run without tokens/database.
- Work directly on main per the existing roadmap convention; do not deploy.

## Review Focus

- Moving from connected UI to preview must clear the existing in-memory token (Task 2).
- Preview create/edit must never invoke API or auth code, including accidental connected mounts (Tasks 2–3).
- Deep links, unknown synthetic IDs, reset and browser history must remain coherent (Task 3).
- Failed browser storage or rapid language choices must not break rendering or commit a stale locale (Task 1).
- Mobile RTL navigation/forms must preserve focus, readable contrast and containment (Tasks 2–4).

## File map

Modify `web/package.json`, its lockfile, `web/src/main.ts`, `web/src/App.vue`, `web/src/i18n.ts`, `web/src/style.css`, `web/src/api.ts` only if needed for explicit session cleanup, `locales/messages/en_US.json` and `NOTICE`. Move the existing App implementation into `web/src/connected/ConnectedApp.vue`, adjusting imports without changing API behavior. Add `web/src/router.ts`, `web/src/stores/preferences.ts`, `web/src/stores/preview.ts`, `web/src/preview/{PreviewLayout,RecordList,RecordDetail,RecordForm,EvidencePage,NotFound}.vue`, `web/src/preview/{model,fixtures,evidence}.ts`, `web/src/preview/preview.css`, and locally licensed font files under `web/src/assets/fonts/`. Tests live in `web/test/preview.spec.ts` and `web/test/preferences.test.ts`; preserve existing tests. Documentation: `docs/compat/ui/preview-evidence.md` and `docs/compat/ui/preview-walkthrough.md`.

### Task 1: Shared preferences, translation and typography foundations

**Interfaces:** Export `usePreferencesStore()` with `theme: 'system'|'light'|'dark'`, `locale: string`, `setTheme(theme): void`, `setLocale(code): Promise<void>`, `initialize(): Promise<void>`. Preserve existing `t(key: string): string`, `setLanguage(code: string): Promise<void>`, `language` ref and `languages` exports for connected callers; expose the vue-i18n instance for app installation.

**Files:** preferences/i18n/main/styles/fonts/catalog/NOTICE and package files above; `web/test/preferences.test.ts`, existing `web/test/i18n.test.ts`.

- [x] **Step 1: Write failing tests.** Cover saved valid/invalid preferences, unavailable/throwing localStorage, en_US fallback, iw→he/rtl, pt_BR mapping, missing translated keys, and two deferred language loads resolving in reverse order where only the latest choice changes messages/lang/dir. Retain the existing core alias/fallback test. Add dependency-free preference parsing/storage helpers in a focused module if needed so the existing Node runner can exercise them; browser effects are tested in Task 2.
- [x] **Step 2: Run `npm --prefix web test`.** Expect assertion failure for the new behavior, not a test-discovery failure. Expand the script to include all Node unit tests without matching Playwright specs.
- [x] **Step 3: Implement shared foundations.** Add compatible pinned dependencies without incidental upgrades. Use vue-i18n `legacy:false`, existing JSON catalog loading and committed-locale sequence guard. Add Pinia preferences and defensive persistence. Copy regular/bold woff2 plus OFL from the sibling reference with NOTICE attribution. Define spec token colors, self-hosted @font-face and system/light/dark behavior; avoid globally restyling the connected layout. Add English keys; leave other catalogs explicitly partial with fallback.
- [x] **Step 4: Run `npm --prefix web test` and `npm --prefix web run build`.** Expect passing tests and typechecked production build. Inspect lockfile changes for unrelated upgrades.
- [x] **Step 5: Commit Task 1 files** with `feat(web): add shared preview preferences and translations`.

### Task 2: Isolated routed frame and evidence map

**Interfaces:** `router.ts` exports a hash-history router using the spec's exact paths. `PreviewLayout` supplies RouterView and navigation. `evidence.ts` exports `EvidenceEntry[]`, with `{id, screen, action, api: {method,path}[], reference: {source,status}[], candidate: {source,status}, simulation, limitations}`; statuses are descriptive strings with no inferred certification flag.

**Files:** router/App/ConnectedApp/main, preview layout/CSS/evidence page/data and `docs/compat/ui/preview-evidence.md`; `web/test/preview.spec.ts`.

- [x] **Step 1: Write failing Playwright tests.** Require `/#/preview/accounts` to render prototype heading without any `/services/` request; intercept/count those paths and fail after navigation/interactions. Test preview/evidence/connected links, unknown-route state, desktop sidebar, mobile drawer Escape/focus return, theme choice, Hebrew lang/dir and translated/fallback labels. Test connected unmount explicitly clears the API module token, verified by a later connected request requiring a newly supplied token; do not expose token getters to production for tests.
- [x] **Step 2: Run the focused preview tests.** Use the existing Playwright configuration and a preview-only project if the current config forces database setup. Expect route/heading failure. Preview server must have no required token/database variables.
- [x] **Step 3: Implement the shell and map.** Move connected component and repair relative imports; clear its private API token on unmount. Register fixed `new` routes before `:id` and a catch-all. Add localized Preview link to connected and Return link to preview. Build header/sidebar/drawer, theme/language controls and sample-data notice. Map each action to the spec's exact API routes and scoped Plans 2/4/8/9/11 evidence. Keep blocked Plan 5/10 statuses intact and Account reverse-related-list composition explicitly proposed. Evidence renders escaped text, never raw HTML.
- [x] **Step 4: Run focused preview tests, existing Node tests and web build.** Require zero service calls on preview; re-run existing connected e2e with its normal setup when available and record any setup gap.
- [x] **Step 5: Commit Task 2 files** with `feat(web): add isolated exploratory CRM frame`.

### Task 3: Synthetic record journey

**Interfaces:** `type PreviewObject = 'accounts'|'contacts'`; `interface PreviewRecord {id:string; Name?:string; FirstName?:string; LastName?:string; AccountId?:string|null; Compat_Key__c?:string|null}`. `usePreviewStore()` exposes `accounts`, `contacts`, `find(object,id): PreviewRecord|undefined`, `save(object, draft, id?): string` and `reset(): void`. Seed IDs are `sample-account-1`, `sample-account-2`, `sample-contact-1`, `sample-contact-2`; new local IDs use a monotonic per-store counter. Save returns a new/updated record ID; records remain in memory only. Form component validates before save; store rejects unknown update IDs and invalid AccountId references.

**Files:** preview store/model/fixtures/list/detail/form/not-found; `web/test/preview.spec.ts` and English catalog.

- [x] **Step 1: Add failing journey tests.** Account list→detail→edit→save shows changed local Name and simulation notice, Cancel preserves prior value, new Account appears, Contact picker links correctly, related Contacts show only matching fixtures. Assert required Name/LastName and >80-character key errors, unknown IDs show not-found, reload restores seed data and new-record URLs become not-found, reset returns to Accounts, back/forward selects the correct page. Count service requests across every journey and require zero. No Compat_Text__c/search/pagination/delete/upsert controls.
- [x] **Step 2: Run focused preview tests.** Expect missing record UI or state behavior.
- [x] **Step 3: Implement the journey.** Clone fixtures per Pinia instance; keep local drafts separate until valid save. Follow spec columns/fields only. Use router params for selection; link Contact to Account and compute Account contacts locally. Include accessible form errors, success announcements, safe text rendering and reset notification. Locale/theme survive reset; synthetic records do not survive reload.
- [x] **Step 4: Run preview tests, Node tests and build.** Expect PASS with no record API requests. Include a fresh Pinia instance per isolated scenario and a route to missing Account references without a crash.
- [x] **Step 5: Commit Task 3 files** with `feat(web): add local Account and Contact prototype journey`.

### Task 4: Visual review and preview handoff

**Files:** preview styles/components/tests as needed; `docs/compat/ui/preview-walkthrough.md`.

**Interfaces:** Consumes all preceding UI/evidence exports; produces verified walkthrough and bounded status report.

- [ ] **Step 1: Verify interactions and layout in a real browser.** Inspect desktop 1440px and mobile 390px, light/dark, English/Hebrew, lists/detail/forms/evidence and open drawer. Check visible keyboard focus, focus return, readable contrast, no page-level horizontal overflow, wrap for long IDs/names and input labels. Add targeted browser assertions for regressions discovered rather than screenshot-only claims.
- [x] **Step 2: Fix material findings and run final checks.** Run `npm --prefix web test`, `npm --prefix web run build`, the preview Playwright project, existing connected e2e with its documented environment, and `git diff --check`. Require no preview service calls. Explicitly record skipped connected verification; it does not count as passing. No Go behavior changes are planned; broaden checks only if scope changes.
- [x] **Step 3: Obtain a fresh whole-change review.** Check spec/evidence honesty, isolated connected/preview modes, i18n/RTL, route reload, local state and font license. Resolve Important/Critical findings before completion.
- [x] **Step 4: Publish walkthrough and commit.** Include local start command and `/#/preview`, sample-data reset/reload behavior, implemented screen scope, evidence link, browser checks and actual test outcomes. Mark it exploratory, not backend parity. Commit `docs: record exploratory UI verification and walkthrough`.

## Handoff

Recommend native execution for this bounded, tightly connected frontend slice, with a fresh whole-change reviewer. Review the spec and plan before implementation; the maintainer approved native execution for this package. This plan preserves the independent Plan 12 equality work and does not spend the remaining UI feedback/expansion plans in advance.

Execution note: Tasks 1–3 implemented and verified. Fresh final code review found request-lifetime and hover-contrast defects plus reset-ID reuse; all were corrected with failing-then-passing regressions. Node suites, production build, nine preview browser tests and four connected e2e tests pass. Interactive visual review was denied by browser approval and remains unverified; no alternative-browser or screenshot workaround was attempted. The visual acceptance gate in Task 4 remains open. See docs/compat/ui/preview-walkthrough.md for verification and limitations.
