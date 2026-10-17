# Plan 5 — REST/query Foundation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement sequentially using the recorded native execution preference. Steps use checkbox syntax. Read the approved spec; complete Task 3's evidence gate before product implementation.

**Goal:** Deliver selected Account/Contact REST/query contracts with repeatable evidence and identical client-facing routes for drop-in use.

**Architecture:** Extend the existing laboratory with a separately bound foundation suite. Extract query parsing/resolution, retain PostgreSQL execution, add persistent opaque query locators and preserve the Salesforce HTTP adapter. Serve accurate OpenAPI separately from Salesforce routes.

**Tech Stack:** Existing Go 1.25+, pgx/PostgreSQL 17, Node/TypeScript, pinned JSforce/Salesforce CLI/simple-salesforce and Vue smoke workflow. No dependency upgrades by default.

**Spec:** [Approved design](../specs/2026-10-17-plan-5-rest-query-foundation-design.md), including identical routes for one-to-one drop-in use.

**Status:** Execution authorized by the maintainer's “continue” after written-plan delivery. Restored from the conversation after the workspace filesystem recovered and the temporary copy disappeared. Task completion must be recorded with actual verification evidence.

## Global constraints

- Work on main, native sequential execution, under the recorded maintainer preference.
- Use public APIs, synthetic owned fixtures and unmodified pinned clients. Never inspect Salesforce CLI/plugin implementation or copy Salesforce prose.
- Keep credentials/raw captures/recovery journals private. Existing named-tenant authorization applies.
- Preserve Salesforce methods, versioned paths, parameter names, headers, escaping and response-link shapes. Clients change origin/credentials only; no SDK patches, route rewrites or pagination adapters.
- Continuation uses `/services/data/v65.0/query/{queryLocator}` and nextRecordsUrl; no replacement /cursor or pageToken interface. Preserve narrow v51.0 own-user discovery without implying general v51.0 support.
- Preserve pilot profile bytes and all eleven scenarios. Foundation acceptance is neither full-pilot nor full-M3 acceptance.
- Preserve stored IDs, tenant isolation and denied-field checks. Never hold transactions across HTTP requests or silently truncate results.
- Exclude SOAP, Bulk, Composite, OAuth lifecycle, recycle-bin behavior, dynamic metadata and full SOQL/SOSL.
- Follow CLEAN_ROOM.md for source reuse. No implementation copying is planned.
- Authenticated /openapi.json is additive SalesCompel documentation outside the Salesforce namespace, not a required client endpoint or new reference discovery key.

## Review focus

1. Encoded slashes/percent/Unicode external IDs survive routing without redirecting writes — Tasks 2, 4, 7.
2. Reused locators after role changes or across principals cannot expose cached fields — Task 6.
3. Ordering ties, mutation and replay do not cause unexplained omissions/duplicates — Tasks 2, 6.
4. Duplicate canonical JSON fields and denied aliases cannot bypass checks or depend on map order — Tasks 2, 4.
5. Forged continuation URLs never forward credentials off-origin; suite/profile mixups fail before transport — Tasks 1, 2.

## File and interface map

New files are proposed interfaces; tests accompany each production module.

| Files | Responsibility |
| --- | --- |
| compat/clients/node/src/suites.ts; existing capture/readiness/evidence/validate/cli | Trusted suite selection and bindings |
| compat/clients/node/src/foundation.ts, wire.ts | Focused scenarios and bounded same-origin capture |
| docs/compat/profiles/rest-query-foundation.yaml, reference-readiness-foundation.yaml | Focused profile and fixture attestation |
| docs/compat/contracts/rest-query-foundation.yaml | Supplemental dimensions |
| docs/compat/releases/index.yaml, changes.yaml | Release inventory/findings |
| docs/compat/observations/rest-query-foundation.yaml, plan-5-contract-decisions.md | Reviewed observations and semantic decisions |
| internal/query/{types,parse,resolve}.go | Pure query parsing and policy-based resolution |
| internal/platform/{ids,query_adapter,cursor,query_http}.go | ID normalization, SQL boundary, cursors and HTTP |
| Existing catalog/store/select/http/schema files | Record/catalog integration and persistence |
| api/implemented.json, api/embed.go | Embedded implemented OpenAPI |
| compat/clients/node/src/traffic.ts | HTTP exchange validation |
| docs/compat/plan-5-results.md | Acceptance evidence and limitations |

## Task 1: Bound suite selection with unchanged pilot defaults

**Files:** New src/suites.ts and test/suites.test.ts under compat/clients/node; existing src/{capture,readiness,evidence,validate,cli}.ts and tests; focused profile/readiness/contracts above. Update schemas only as needed with backwards-compatibility tests.

**Interfaces:** SuiteId = 'pilot' | 'rest-query-foundation'; suite(id: string): SuiteDefinition with constant profilePath/readinessPath/requiredScenarios/cleanupScenarios. capture(target, env, suiteId = 'pilot'): Promise<Report>. Add --suite to preflight/capture/compare/repeatability/verify-evidence. Retain Report v1 where possible; profile/hash/manifest bind scope.

- [ ] Write behavioral tests: default pilot remains valid; unknown suite, cross-suite comparison, stale readiness, missing scenarios and mismatched observations fail before transport.

```ts
assert.throws(() => suite('../../private'), /suite/i);
assert.notEqual(suite('pilot').profilePath, suite('rest-query-foundation').profilePath);
```

- [ ] Run `cd compat/clients/node && node --import tsx --test test/suites.test.ts test/preflight.test.ts test/evidence.test.ts`. Expected: new behavior fails before implementation.
- [ ] Implement trusted registry; focused IDs: foundation-routes, foundation-pagination, foundation-cursors, foundation-ids, foundation-names, foundation-text-null, foundation-errors. Require cleanup for every fixture-writing scenario. Parameterize existing validation; never bypass it.
- [ ] Create separate profile with reusable verified settings. Keep expanded fixture flags false until Task 3. Retain authorization attribution without claiming new setup completion. Add scoped header/locator/route/duplicate-key contracts; partial batch coverage never promotes all request-headers.
- [ ] Run `npm --prefix compat/clients/node run check` and `git diff --check`. Expected: pass. Commit `feat(compat): add bound REST query foundation suite`.

## Task 2: Wire probes and identical route fixtures

**Files:** New src/{wire,foundation}.ts, test/{wire,foundation}.test.ts, compat/fixtures/rest-query/routes.json; existing capture.ts.

**Interfaces:** wire(base: URL, token: string, path: string, init: RequestInit): Promise<{status: number; headers: Record<string,string>; body: unknown}>; foundationScenarios: Record<string,(c: Context) => Promise<void>>. Reuse Context, FixtureTracker and create helpers; raw messages stay private.

- [ ] Write local-server tests for off-origin/protocol-relative paths, redirects, encoded components, timeout/response bounds and no credential forwarding. Count requests to prove rejection before transport.

```ts
await assert.rejects(wire(new URL('https://reference.example.invalid'),
  'synthetic-token', '//other.example.invalid/x', {}), /origin|relative/i);
```

- [ ] Run wire/foundation tests. Expected: fail before implementation. Implement URL resolution, exact origin/API-path checks, redirect rejection and bounded reads. Never send credentials to unchecked continuation URLs.
- [ ] Implement focused scenarios for all spec table dimensions: default/requested/invalid batch, empty/exact/multiple pages, 2001+ rows, LIMIT 5000, omitted Id and relationships; replay/concurrency/expiry/unknown locators and mutation/ties; same-user new-session/different-user access; 15/18 ID failures; REST/SOQL/JSON names and denied aliases; text accents/case/null ordering; raw status/code/body errors and failed-write atomicity.
- [ ] Journal every write; shared setup uses supported public CRUD rather than candidate Bulk/Composite. Route fixtures substitute only origins, credentials and owned identities/locators. Preserve methods/path families/query/header names and encoded slash/percent/Unicode. Expiry uses a bounded recorded schedule; inconclusive results remain blocked. Cross-tenant checks use local tenants.
- [ ] Test mid-create failure cleanup and comparison sensitivity to changed status/casing/order/missing pages. Normalize only explicit identity/locator values while retaining URL shape/reuse relations. Run full offline check. Commit `feat(compat): probe REST query wire and route contracts`.

## Task 3: Reference evidence and release reconciliation

**Files:** Focused profile/readiness, releases/{index,changes}.yaml, observations/rest-query-foundation.yaml, plan-5-contract-decisions.md, rest-semantics/supplemental contracts, sources and implementation log.

**Interfaces:** Decisions supply exact expected status/body/route/ID/name/text/null rules, batch/LIMIT/totalSize, cursor representation/lifetime/replay/mutation and numeric resource bounds. Reference choices link evidence; local operational choices are labeled. No dependent task invents missing semantics.

- [ ] Inventory official release archive at recorded cutoff, URLs/versions/products/access dates/reviewer/states. Follow relevant changes and retirements; blocked sources have owner/reason/resolution path. Prioritize supported-version/selected-contract findings without claiming complete M1R.
- [ ] Verify private sessions/expanded fixtures; bind only focused readiness. Set FOUNDATION_RUN_DIR to an owner-only external directory. Never print tokens.
- [ ] Execute sequentially:

```sh
npm --prefix compat/clients/node run preflight -- --suite rest-query-foundation --target reference
npm --prefix compat/clients/node run capture -- --suite rest-query-foundation --target reference --out "$FOUNDATION_RUN_DIR/first.json"
npm --prefix compat/clients/node run capture -- --suite rest-query-foundation --target reference --out "$FOUNDATION_RUN_DIR/second.json"
npm --prefix compat/clients/node run repeatability -- --suite rest-query-foundation --first "$FOUNDATION_RUN_DIR/first.json" --second "$FOUNDATION_RUN_DIR/second.json"
npm --prefix compat/clients/node run verify-evidence -- --suite rest-query-foundation --first "$FOUNDATION_RUN_DIR/first.json" --second "$FOUNDATION_RUN_DIR/second.json" --out "$FOUNDATION_RUN_DIR/verification.json"
```

- [ ] Require all focused scenarios, repeatability, pair verification and cleanup; independently inspect owned-record absence. Publish actually reviewed sanitized notes with digests/times/profile/API/client binding and reciprocal links only for measured dimensions.
- [ ] Resolve relevant release findings. Choose materialized projection for stable values, ordered IDs/re-read for stable membership/live values, or another evidenced bounded representation. Specify exact expiry/replay/locking/resource limits. Choose text strategy from field settings and observed Unicode, not blanket lowercasing. No cross-request transaction.
- [ ] Add observed examples to offline expectations. Run full offline/diff checks. Commit frozen decisions only if gates pass; otherwise commit precise blockers and stop dependent product tasks. Mock captures cannot satisfy this gate.

## Task 4: ID and name normalization with route preservation

**Files:** New platform/ids.go, ids_test.go, routes_test.go; catalog/store/http and corresponding tests.

**Interfaces:** NormalizeRecordID(object,input string)(string,error), CanonicalObject(input string)(string,error), CanonicalFields(p Principal,object string,data map[string]any)(map[string]any,error). Preserve CRUD/NewHandler signatures and HTTP context for route-specific errors.

- [ ] Write evidence-backed ID length/checksum/case/prefix/reference/tenant tests and unchanged-state assertions after failed writes.

```go
func TestNormalizeGeneratedID(t *testing.T) {
    id, err := newID("Account"); if err != nil { t.Fatal(err) }
    got, err := NormalizeRecordID("Account", id[:15])
    if err != nil || got != id { t.Fatalf("got %q, %v", got, err) }
}
```

- [ ] Test canonical output, duplicate canonical fields and denied aliases on read/write/filter/order; test raw duplicate keys before map decoding if evidence requires. Include escaped external IDs/no redirects.
- [ ] Run `go test ./internal/platform -run 'Normalize|Canonical|Route'` with TEST_DATABASE_URL. Expected: new failures.
- [ ] Implement canonical resolution before policy/validation; do not lowercase ID/external-ID values merely to resolve names. Preserve stable IDs, RawPath splitting and discovery routes. Replay route fixtures; run race-enabled platform tests with PostgreSQL. Commit verified normalization.

## Task 5: Pure query parsing/resolution and selected semantics

**Files:** internal/query/{types,parse,resolve}.go/tests; platform/query_adapter.go; existing query/select/store tests.

**Interfaces:** Pure types and Parse(input string)(Parsed,error), Resolve(c Catalog,parsed Parsed)(Resolved,error); SQL identifiers remain private to platform.

```go
type NullPlacement uint8
const (NullDefault NullPlacement = iota; NullFirst; NullLast)
type Parsed struct {
    Object string; Fields []string; HasWhere bool; WhereField string
    WhereValue *string; Order string; Desc bool; Nulls NullPlacement; Limit *int64
}
type Resolved struct { Parsed }
type Field struct { Name string; Kind string }
type Catalog interface {
    Object(input string) (string,error)
    Field(object,input string) (Field,error)
}
```

- [ ] Move parser tests retaining unsupported grammar/injection/invalid UTF-8/NUL/full consumption. Replace only superseded LIMIT/null assertions. Test explicit null placement and LIMIT 5000 separately from page size; LIMIT 0 comes from evidence.
- [ ] Run query tests to establish failures; add real PostgreSQL Unicode/null/tie/denied-alias cases with observed expectations.
- [ ] Extract without HTTP/pgx. Canonicalize and authorize before allowlisted SQL/bound values. Keep existing grammar plus selected extensions. Implement field-aware comparisons/null rules. Retrieve identity for attributes.url without adding unrequested Id fields; preserve relationship shape. Keep truthful overflow until Task 6 delivers all pages.
- [ ] Run `go test -race ./internal/query ./internal/platform` with PostgreSQL and `go vet ./...`. Commit verified semantics.

## Task 6: Persistent query locators and matching continuation endpoint

**Files:** platform/{cursor,query_http}.go/tests; schema/select/http/store/startup tests.

**Interfaces:** QueryPage result and Store.QueryPage(ctx,p,q query.Resolved,batchSize int), ContinueQuery(ctx,p,locator string), both returning (QueryPage,error); ReapQueryCursors(ctx) error.

```go
type QueryPage struct {
    TotalSize int64 `json:"totalSize"`; Done bool `json:"done"`
    Records []map[string]any `json:"records"`
    NextRecordsURL string `json:"nextRecordsUrl,omitempty"`
}
```

- [ ] Test empty/page-boundary/2001+ results, LIMIT above batch, bad header/locator, totals and omitted-Id attributes. Inject small bounds for exhaustion tests.
- [ ] Test restart/replay/concurrent continuation/expiry/reaping/mutations/initial-save rollback; enforce Task 3 semantics. Test other tenant/user, revoked token, changed role and denied fields even for persisted projections.
- [ ] Run cursor/page/continuation tests with PostgreSQL. Expected: fail before implementation.
- [ ] Add tenant-scoped metadata/payload tables and expiry indexes; unpredictable crypto/rand tokens and digest lookup where feasible. Apply additive schema twice without altering records/IDs. Persist Task 3's chosen representation.
- [ ] Use bounded consistent initial transactions, atomic resource bounds/advancement, required locking and commit-before-response. Recheck principal/field access every request. Reap during normal operation, not only restart.
- [ ] Wire GET query and GET query/{queryLocator}, emitting matching nextRecordsUrl and observed Sforce-Query-Options. Never use a replacement custom route. Run all Go race tests and real HTTPS JSforce queryMore/queryAll. Commit verified pagination; never use done=true for resource exhaustion.

## Task 7: Wire errors and accurate served OpenAPI

**Files:** api/implemented.json/embed.go; platform/openapi_test.go/http/catalog/tests; node src/traffic.ts/test/traffic.test.ts/openapi module/tests; broader api/openapi.yaml description.

**Interfaces:** api.Implemented embedded bytes; authenticated GET /openapi.json before frontend fallback. validateTraffic(doc,exchange) checks method/path/request headers/body/status/response headers/body against the required dialect.

- [ ] Test all frozen HTTP status/code/fields rules, malformed/trailing JSON, REST/SOQL unknown objects, permissions/IDs/missing records and failed-write atomicity. Re-run escaping/no-redirect tests.
- [ ] Test exact implemented routes/methods/versions/auth, absence of unsupported surfaces, and traffic negatives (missing totalSize, wrong error shape, invalid input, undocumented method). Establish failures before implementation.
- [ ] Verify current official OpenAPI release and validator dialect support during execution. Record any required pinned tool change; never silently downgrade or ignore unsupported keywords.
- [ ] Implement route-specific mappings and embedded accurate document with per-operation x-source; independently word sanitized messages. Keep broader proposal separate and documentation additive, without changing reference discovery keys.
- [ ] Validate actual HTTP exchanges, prove valid traffic passes and mutations fail, run Go/Node suites and commit verified contract.

## Task 8: Drop-in workflows and acceptance record

**Files:** scripts/check-candidate.sh, node test/candidate.test.ts, docs/compat/plan-5-results.md; README/runbook/Plan 2 guide/log/roadmap links.

**Interfaces:** Existing CANDIDATE_DISPOSABLE=yes scripts/check-candidate.sh; no client patches or narrowed pilot manifest. Evidence names tested commit/profile/captures.

- [ ] Configure disposable DB, TEST_DATABASE_URL, HTTPS and private env. DB-skipped suites do not satisfy acceptance. Run:

```sh
npm --prefix compat/clients/node run capture -- --suite rest-query-foundation --target candidate --out "$FOUNDATION_RUN_DIR/candidate.json"
npm --prefix compat/clients/node run compare -- --suite rest-query-foundation --reference "$FOUNDATION_RUN_DIR/first.json" --candidate "$FOUNDATION_RUN_DIR/candidate.json"
go test -race ./...
go vet ./...
npm --prefix compat/clients/node run check
npm --prefix web test
npm --prefix web run build
CANDIDATE_DISPOSABLE=yes scripts/check-candidate.sh
git diff --check
```

- [ ] Verify unchanged JSforce pagination, Python CRUD, CLI query and identical route fixtures with only origin/credential changes. Retain browser/restart checks and investigate unexplained differences. Verify cleanup.
- [ ] Publish per-contract reference/candidate evidence, route checks, release findings, operational limits and unresolved differences. M1R/full M3 stay open. Preserve historical Plan 2 facts.
- [ ] Inspect all diffs/private artifacts; commit acceptance only if gates pass, otherwise record incomplete dimensions/checks. Unsupported full-pilot surfaces preclude full candidate pilot parity claims.

## Self-review and handoff

Tasks 1–3 preserve bindings and establish live/release evidence; 4–6 implement normalization/query/pagination; 7 covers HTTP/OpenAPI; 8 proves client routes and acceptance. All review-focus conditions have owning tests. Evidence-dependent values are gated, not guessed. The maintainer authorized continuation after the written plan; no new execution-method selection is needed.
