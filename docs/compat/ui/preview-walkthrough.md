# Exploratory CRM UI walkthrough

Plan 13 implementation, 2026-10-20. This is an Account/Contact design prototype using local synthetic data, with separate documentation of observed behavior, known routes and candidate implementation. It is not compatibility certification or a production CRM.

## Open it locally

From the repository root:

```sh
npm ci --prefix web --ignore-scripts
npm --prefix web run dev -- --host 127.0.0.1
```

Open the printed local address followed by `/#/preview/accounts` (normally `http://127.0.0.1:5173/#/preview/accounts`). No database or token is needed for preview. The connected candidate remains at `/#/` and requires its existing operator setup. Leaving connected mode cancels outstanding browser requests and clears its in-memory token. Already-dispatched server mutations are not undone by aborting a browser request.

For the production bundle, use `npm --prefix web run build`, then `npm --prefix web run preview -- --host 127.0.0.1`. The browser suite exercises this production delivery path. Development-only third-party devtools can fail if a browser forcibly denies all storage access; the production preview's defensive preference fallback is tested.

## Walk through the first journey

1. Open Accounts and choose Northstar Studio. Review its detail fields and related sample Contact.
2. Follow Alex Rivera to Contact detail, then follow the Account link back.
3. Edit a sample or create a new Account/Contact. Saving changes only the in-memory sample workspace; Cancel discards the draft.
4. Leave a required name blank to explore validation. External keys are limited to 80 JavaScript string units as a candidate-aligned prototype constraint, without claiming Salesforce Unicode length behavior.
5. Switch system/light/dark theme or language. Hebrew changes document language to `he` and layout direction to RTL; other supported wire codes remain unchanged. Newly added non-English strings fall back to English and are not labeled completed translations.
6. Open Evidence for the separate reference/candidate/simulation mapping. API details are kept there rather than in the normal record workflow.
7. Reset preview to restore seed data, or reload. Local edits and new records disappear on reload. Unknown or removed sample URLs show not-found. Local IDs stay monotonic across resets within a running preview so an old link cannot select a replacement record.

The mobile menu expands in page flow, closes on Escape or navigation and returns focus to its toggle. All preview interactions work without calling service routes. No search, pagination, bulk actions, delete, upsert editor, real authentication or server persistence is implied.

## Verification and review

Verified during implementation:

- Node unit tests: language aliases/fallback, preference failure handling, latest-selection-wins locale loading, isolated preview stores, invalid references and monotonic IDs across reset.
- Production web build: Vue/TypeScript check and Vite bundling pass.
- Preview Playwright suite: nine tests pass against the production bundle, without database or credentials. Covers route/history/unknown IDs, synthetic CRUD subset and reset/reload, Account/Contact links, zero service requests, storage fallback, language/theme persistence, Hebrew RTL, 390px/1440px containment, connected-mode isolation, delayed-request cancellation and primary-button hover contrast >=4.5 in both themes.
- Existing connected Playwright suite: four tests pass against an actual Go server and owned disposable PostgreSQL 17. The server's existing CSP remains unchanged. Connected record writes, language switching, RTL, logout and stale-refresh handling remain covered.
- `git diff --check` passes. No backend behavior or reference evidence was changed.

Fresh code review found two Important issues (pending connected requests and primary-button hover contrast) and one reset-ID issue. Regressions were observed failing before correction and passed afterward. The reset-ID issue was treated as a correctness fix for navigation, rather than deferred. No outstanding code-review findings remain. The reviewer did not rerun tests; the executor ran the checks above.

**Interactive visual inspection remains unverified:** browser approval denied access to the local review server. No alternate-browser or screenshot workaround was attempted. Automated layout/contrast assertions do not establish complete visual polish or accessibility conformance. Do not mark this package's visual acceptance gate complete until a person or authorized browser review has checked desktop/mobile, light/dark, English/Hebrew, forms and open navigation.

Reproduction commands:

```sh
npm --prefix web test
npm --prefix web run build
npm --prefix web run test:preview
# Set CHROMIUM_PATH=/usr/bin/chromium when using a system browser.
# Connected suite additionally needs its documented disposable candidate setup:
npm --prefix web run test:e2e
git diff --check
```

CI runs preview and connected suites separately. Font assets are bundled locally with their OFL license and NOTICE attribution. Pinia 3.0.4, Vue Router 4.6.4 and vue-i18n 11.1.12 are pinned; existing Vue 3.5.44/Vite 7.3.7 were retained. The sibling's newer i18n release was type-incompatible with this Vue version, so matching compatible i18n declarations were chosen rather than suppressing type checking.

## Feedback to collect next

Can people find the right record, understand Account/Contact relationships, distinguish sample actions from connected functionality, and finish create/edit without explanation? Record confusing labels, navigation dead ends and missing information before expanding the object inventory. The [evidence matrix](preview-evidence.md) bounds what this first slice demonstrates. Full M2/M6 acceptance and the independent Plan 12 equality experiment remain open.


Implementation choices: work stayed on main under the existing approved policy (shared-checkout changes); compatible i18n and matching declaration pins preserved the existing Vue toolchain (later upstream fixes require a future dependency review); frame and record pages were committed together because their imports are coupled (less granular rollback); automated preview tests use the production bundle (hostile-storage development devtools are outside that supported delivery path); reset ID reuse was treated as navigation correctness and fixed in the same review pass (no persisted-data impact). These choices do not waive the unverified visual acceptance gate. No minor code findings were deferred.
