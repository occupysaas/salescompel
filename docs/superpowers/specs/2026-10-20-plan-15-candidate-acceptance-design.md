# Plan 15 — Candidate walking-slice acceptance

Status: tooling implemented and reviewed, 2026-10-21 UTC. The original two full runs failed the native CLI query; [Plan 16](../../compat/plan-16-cli-query-column-metadata.md) subsequently resolved the blocker and records two full passes. Full M1/M2 milestones remain open. See the [execution and decision record](../../compat/plan-15-candidate-acceptance.md).
Milestone: bounded M2 acceptance work, with regression coverage for the existing M3/M4 slices and M6 preview.

## Intent and selection

Establish one reproducible answer to whether the current synthetic Account/Contact candidate still works end to end after Plans 7–14. The [Plan 2 guide](../../compat/plan-2-implementation.md) records a real-client/restart run from October 15; later plans added catalog upgrades, field validation, localized routing and preview behavior. Recent lab checks passed with the real-candidate test skipped, and [Plan 12 recorded skipped database tests](../../compat/plan-12-external-id-equality.md).

The executor selects this independent acceptance package while [Plan 14 reference collection](../../compat/plan-14-external-id-normalization-contract.md) and Plan 10 foundation collection remain blocked. Resume those existing plans when their prerequisites become available; their unfinished reference work does not need another plan number. This package exercises the implemented candidate and selects no new Salesforce semantics.

Deliver a strict local/CI acceptance entry point built on the existing checks, meaningful missing regressions, and a sanitized verification record. Passing establishes the tested candidate walking slice. It does not certify Salesforce parity, close M1, or close the full M2 milestone with its outstanding dependencies and exclusions.

## Existing implementation to build on

Reuse `scripts/check-candidate.sh`, `scripts/candidate-python.py`, `compat/clients/node/test/candidate.test.ts`, Go platform integration tests, `web/test/records.spec.ts`, the separate preview suite, and `.github/workflows/platform.yml`. Do not build another reference collector, general orchestration framework or parallel copy of these suites.

The current candidate script already provisions two tenants and three principals, serves HTTPS, invokes real JSforce/Python/CLI clients, checks an Account across a process restart and runs connected browser tests. It currently prints a success sentence rather than a consolidated versioned result. Its Python journey covers Account CRUD, CLI covers login/query, the restart probe retains only one Account, and connected browser coverage primarily uses the administrator. Extend these concrete gaps rather than restating existing tests as new work.

Use current lockfiles and Go module constraints: Go 1.25.0 module baseline, Node >=22.12 for web, Python >=3.12, PostgreSQL 17, API 65.0. Keep JSforce 3.11.0, Salesforce CLI 2.156.0, plugin-data 5.1.16, plugin-deploy-retrieve 4.2.15, simple-salesforce 1.12.9 and existing Vue/Playwright pins. Record actual tool versions. No framework/client upgrades or new runtime dependencies belong here.

## Acceptance entry point and outcomes

Extend the existing candidate script or add one thin wrapper that calls it. Provide a documented single command from the repository root which runs the full ordered gate below and writes a versioned JSON summary to an explicit output path. The exact wrapper name and schema layout are builder choices. Keep the existing documented candidate command usable.

Require `CANDIDATE_DISPOSABLE=yes`, a dedicated disposable PostgreSQL database and an output location before provisioning. Bind `DATABASE_URL` and `TEST_DATABASE_URL` to that same test database for this invocation; conflicting supplied values block before writes. Do not inspect unrelated databases, auto-create cloud resources or infer disposability from a database name. CI uses its existing PostgreSQL service; local execution uses an operator-provided disposable instance. No automatic drop of a supplied database.

Check dependencies, the configured database, TLS tooling, browser and compatible versions before starting fixture work. Do not install dependencies or download browsers within an acceptance run. Missing prerequisites produce **blocked**, never a skip-based pass. Setup commands remain separate and documented. Enforce an overall 20-minute test deadline and bounded owned-process shutdown; timeouts cannot produce a pass.

The runner starts its own Go child service on loopback with an ephemeral trusted test certificate. It chooses or validates an available port and refuses to reuse another running service. All candidate connector destinations come from that owned service URL; ignore or reject inherited endpoint overrides. Do not load reference credentials, contact Salesforce, reuse external sessions or run Plan 10/14 collection.

Overall outcomes are **passed**, **failed** and **blocked**, with exit codes 0, 1 and 2. Individual gates additionally support **not-run** after a prior failure/blocker. Assertion failures, process crashes, unexpected test skips, missing expected tests, failed cleanup and malformed result files prevent a pass. Missing setup is blocked; a broken assertion is failed. Preserve the original failure if cleanup also fails. A signal/deadline must stop owned children and retain an incomplete/blocked result when publication is possible; a crash with no completed summary cannot count as acceptance.

Use stable gate IDs and verify expected suite/test identity rather than freezing today's raw test count. A subprocess exiting zero after selecting zero tests or skipping the real-candidate gate is insufficient.

## Required gate sequence

| Gate | Required evidence |
| --- | --- |
| Source/tool binding | Revision, tracked-source state/digest, lockfile/catalog digests, actual tool versions, built executable and web asset digests |
| Database/platform | `go test -race ./...` against disposable PostgreSQL, with every applicable database test executed; `go vet ./...` |
| Offline lab | `npm --prefix compat/clients/node run check`; its optional real-candidate skip is permitted only here because the separate real-client gate below is mandatory |
| Web foundation | Existing web unit tests and production build |
| Candidate clients and policy | Real pinned JSforce, simple-salesforce and native CLI against the owned HTTPS service; matrix below |
| Restart | Retained Account/Contact/custom fields/principals checked across an actual service process replacement |
| Connected browser | Production assets served by Go, real CRUD/relationships, administrator and restricted/other-tenant boundaries, locale/token checks |
| Preview regression | Existing production preview suite, including zero service requests; isolated from connected browser session/data |
| Cleanup/publication | Owned service stopped, exact fixture cleanup verified, required results present, sanitized summary published |

The final acceptance record must bind a clean committed source revision. Development runs may record a dirty source state and actual file digests for diagnosis but cannot be published as the final accepted run. Generated reports stay outside the tracked tree. No success claim may rest solely on source revision while the tested executable or web bundle came from different bytes.

Run two complete fresh local acceptance invocations sequentially against the same disposable database to expose leftovers, cached credentials, occupied ports and stale-process reuse. Use distinct run markers, principals and outputs. The second invocation begins only after the first stopped its owned processes and verified its fixture disposition. A CI job need run one complete invocation per revision; document both local repeatability and CI availability separately. No claim that GitHub Actions ran without an actual observed run.

## Real-client and policy matrix

Use synthetic records and the existing roles: tenant A administrator, tenant A restricted principal, tenant B administrator. Prove actual response/committed-state relationships, not only successful client construction. All writes use currently implemented endpoints and candidate semantics.

- JSforce retains its existing describe, custom-field metadata, Account/Contact relationship, null/omission and exact-key upsert journey. Strengthen external tenant checks to include update/delete attempts and attempts to attach a Contact to the other tenant's Account; verify no mutation afterward. An identical external key is usable independently in the other tenant.
- Restricted access covers describe/read/write/select/filter/order for `Account.Compat_Text__c`. Assert that denied operations leave stored values unchanged and that allowed Name/relationship operations still work. This is the candidate's synthetic field policy, not a Salesforce sharing model.
- Extend the real Python client to create and read a linked Account/Contact pair, query the supported `Contact.Account.Name` projection, exercise one permitted update and one denied/cross-tenant operation, and delete children before parents. Preserve TLS validation; do not patch simple-salesforce or replace its transport with an HTTP mock.
- Keep native CLI login/query through isolated test state and add a real Account create/read/update/delete journey using commands already available in the pinned CLI. Derive exact flags and JSON projections from the locally installed command help during implementation. Keep command responses separate from wire semantics; do not invent unsupported CLI output contracts. Do not install plugins, change global CLI config or adopt installer/Docker/plugin certification scope.
- Preserve Plan 11's body-validation, Plan 8's catalog/length and current URL-decoding regression tests through the Go gate. Equality/spelling/race behavior remains the current candidate behavior unless an independently accepted contract already authorizes a change. No case/Unicode/whitespace matching rule is selected here.

Keep fixtures per run and track returned IDs immediately. Share retained restart fixtures deliberately; otherwise independent client fixtures must not accidentally satisfy each other's queries. Discovery/readbacks must identify exact fixture IDs and expected values, rather than pass on any nonempty Account table.

## Restart and browser coverage

Retain an Account with non-null custom fields and a linked Contact across shutdown. Capture their IDs, exact values and relationship before stopping the process. Prove the original PID exited; start a new process with the same binary/database/assets. Authenticate using the previously provisioned tokens, read the same IDs/values, query the relationship, and recheck a restricted-field denial and cross-tenant invisibility. Reopening a Store in one process or recreating equivalent records is insufficient. This proves service restart persistence, not host/database disaster recovery or catalog migration correctness beyond existing tests.

Run connected browser journeys against the Go-served production bundle at `/#/`. Preserve existing create/edit/delete, failed-refresh, language and RTL checks. Add restricted-session assertions that the forbidden field is unavailable and allowed writes persist, plus an other-tenant session that cannot see the first tenant's fixtures. Verify token cleanup on disconnect and switching to `/#/preview`; use fresh browser contexts where isolation is part of the assertion.

Exercise all five registered language choices in the connected journey at least for successful selection, committed document lang/dir and expected localized or explicit fallback labels. Existing asynchronous latest-selection-wins and storage-failure unit tests remain required. Draft translation coverage stays draft; English fallback is not translation completion. Preserve responsive containment and keyboard-operable forms/navigation using existing application behavior. No redesign, new screens, translation campaign or i18n-library migration belongs here.

Run the preview suite separately with no connected credentials or mounted connected session. Preserve its network-isolation assertions, theme/locale behavior and local reset semantics. It may continue using the established production preview server. Do not weaken its zero-service-request assertion to make the combined run pass.

Automated browser checks and controlled synthetic screenshots are regression evidence only. [Plan 13's interactive visual acceptance](../../compat/ui/preview-walkthrough.md) remains open under its existing denial; this plan does not authorize another browser path or screenshot workaround to claim that review completed. No human usability, accessibility-conformance or visual-polish certification is implied.

## Fixtures, shutdown and report privacy

Use a private per-run scratch directory and isolated CLI state. Do not inherit a user's CLI account store, write auth tokens to shared files, echo environment variables/DSNs, disable connector TLS verification or publish raw CLI login output. The existing browser-only ephemeral-certificate exception remains confined to that browser configuration.

Track Account/Contact IDs for cleanup, delete Contacts before Accounts and verify exact-ID absence through the API while the service is running. Failure paths must preserve enough private ownership information for diagnosis. Stop and reap only owned PIDs on success, failure and interruption; never kill by port, broad process name or unrelated PID. A failed cleanup is not masked by a passing test command.

The existing harness leaves provisioned test tenants/principal digests in the disposable database. Choose one explicit policy for this package: retain that behavior, record the run-owned principal/tenant disposition privately and require discarding the dedicated database after testing. Do not add a broad tenant purge, infer a safe delete prefix or claim that row cleanup returns the database to an empty state. Both sequential runs must work with these retained principals. Whole-instance database destruction remains an operator action or CI service teardown.

Write the JSON summary atomically to a new output name, with a schema version, stable gate IDs/outcomes, expected versus executed coverage, start/finish times, source/tool bindings, sanitized failure categories, cleanup disposition and links/digests for approved sanitized evidence. Reject an existing output path rather than overwrite earlier acceptance evidence. Individual raw logs, secrets, responses and browser artifacts stay private by default. Publish only allowlisted fields; never rely on token redaction alone to make arbitrary subprocess output public.

CI should retain the sanitized summary even on test failure when one exists. Do not upload private scratch directories, raw Playwright traces/DOM snapshots, token-entry screenshots, CLI auth state, raw HTTP headers or database connection strings. Failure to produce/store required evidence prevents a reported acceptance pass even if subprocesses exited zero.

## Verification and delivery

Harness tests must demonstrate prerequisite blocking before provisioning, endpoint ownership, zero selected tests/unexpected skips, subprocess failure, missing/malformed result files, occupied port, restart failure, shutdown on signals and cleanup failure. Use controlled local processes/transports to test orchestration failures; the accepted integration journeys themselves require real Go/PostgreSQL/clients/browser. Avoid tests that merely search shell source for an expected command.

Add failing-then-passing regressions for missing client/policy/browser coverage, using the existing frameworks. Changes to Go/database, native clients and browser suites must exercise actual behavior. Keep unrelated lab schemas and historical captures unchanged. If a failing assertion reveals an already specified candidate defect, fix it narrowly with its regression. If it requires deciding an unresolved reference contract, record a failed/blocked acceptance and a separate follow-up; do not choose new semantics to force a pass.

Publish `docs/compat/plan-15-candidate-acceptance.md` with the gate matrix, source/tool/binary/asset bindings, actual commands/counts/skips, both local runs, fixture disposition and sanitized report digests. Update the Plan 2 setup/check guide and existing platform CI to use the consolidated entry point without double-running expensive suites unnecessarily. Run `git diff --check` and obtain fresh whole-change review; resolve Important/Critical findings before declaring execution complete.

A missing disposable database/browser/tool is a precise blocker. Finish useful harness/test work and report unexecuted integration gates separately; do not manufacture an acceptance record from unit tests. The outcome is either a reproducibly verified candidate slice or a complete truthful account of the remaining blockers.

## Calls left to the builder

The executor selected this independent M2 slice, the two fresh local runs, the bounded client extensions and retained-principal disposition. Choose focused helper/module boundaries, report schema details, source-manifest hashing and CLI flags from existing local tooling. Reuse established shell/Node/Python/Playwright infrastructure. No new product APIs, schema/catalog version, broad CLI runner, hosting, live reference collection or migration engine is authorized by this design.

## Builder check

A fresh software-engineer reviewer read the design and relevant repository context and reported “nothing important left.” No material planning questions required a maintainer decision. Scope selection and the execution choices listed above are executor recommendations for review, not completed acceptance or authorization to implement.
