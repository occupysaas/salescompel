# Plan 2 — Platform Walking Slice Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Goal:** Run a persistent, tenant-isolated Account/Contact candidate service and localized Vue workflow against real clients.

**Architecture:** Go HTTP adapter, fixed metadata catalog, validated query AST and typed PostgreSQL storage. Bearer principals scope every operation. Vue consumes the same REST surface as the pinned connector clients.

**Tech Stack:** Go/pgx v5, PostgreSQL 17, Vue 3, TypeScript, Vite, Playwright, JSforce and simple-salesforce.

**Spec:** `docs/superpowers/specs/2026-10-15-plan-2-platform-design.md`

## Global Constraints

- Go 1.25 or newer; Node 22.12 or newer; PostgreSQL 17.
- API version 65.0 is a candidate contract only, not a claim about a reference org.
- Tokens live in memory only and are cleared on logout.
- Tenant identity never comes from request payload, query string or a tenant header.
- Product strings live in catalogs; canonical field names and values do not change with language.
- No live Salesforce observation or certification; M1 evidence remains pending.

## Review Focus

1. Cross-tenant IDs in read, write, relationship and external-ID paths must never expose or mutate another tenant.
2. Null, omitted, wrong-type and unknown fields must remain distinct and failed writes atomic.
3. SQL-like input, denied fields and unsupported query clauses must fail closed.
4. Concurrent updates and repeated upserts must not lose fields or duplicate rows.
5. Failed HTTP calls and language switches must not leave stale UI state or persist credentials.

## File and interface map

`internal/platform/catalog.go`: Object/Field descriptors and write validation.
`query.go`: ParseQuery(string) (Query,error), fully consumed restricted grammar.
`store.go`, `schema.sql`: Open(ctx,url), Provision(ctx,token,tenant,user,role),
CRUD and Query methods receiving Principal explicitly. All storage is PostgreSQL.
`http.go`: NewHandler(*Store,assetDir) http.Handler, authenticated REST adapter.
`cmd/salescompel/main.go`: configuration, provisioning subcommand, TLS and shutdown.
`web/src/`: Vue editor, typed API and lazy catalog resolver.
`compat/clients/node/test/candidate.test.ts`: opt-in actual endpoint integration.
`scripts/check-candidate.sh`: database/TLS/server/client/browser verification.

### Task 1: Catalog and query boundary

**Files:** create `go.mod`, `internal/platform/catalog.go`, `query.go`, `catalog_test.go`, `query_test.go`.
**Interfaces:** produces `Principal{Tenant,User,Role string}`, catalog field validation,
`Query{Object string,Fields []string,WhereField string,WhereValue *string,HasWhere bool,Order string,Desc bool,Limit int}`.

- [x] Write table-driven tests that reject unknown fields, required nulls, numeric values,
  restricted custom-field access, trailing SQL, OR, invalid LIMIT and unclosed literals.
  Assert a valid Contact query yields Object Contact, fields Id/Account.Name and literal O'Brien.
- [x] Run `go test ./internal/platform` and verify missing implementation fails.
- [x] Implement a fixed catalog and a full-consumption token parser; never interpolate a user identifier into SQL before catalog validation.
- [x] Run `go test ./internal/platform`; expected all unit cases pass.

Example executable contract:
```go
q, err := ParseQuery("SELECT Id,Account.Name FROM Contact WHERE LastName = 'O\\'Brien' LIMIT 2")
if err != nil || q.Object != "Contact" || *q.WhereValue != "O'Brien" { t.Fatal(q, err) }
```

### Task 2: Persistent tenant-scoped record engine

**Files:** create `internal/platform/schema.sql`, `store.go`, `store_test.go`.
**Interfaces:** consumes Principal/catalog/Query; produces Open, Close, Provision,
Authenticate, Create, Read, Update, Delete, Upsert and Select methods with context.

- [x] Write database tests with two tenants, same external key, cross-tenant reads and
  relationship attempts, omitted/null updates, duplicates, reopen persistence and concurrent disjoint updates.
- [x] Run `TEST_DATABASE_URL=... go test ./internal/platform -run Store -count=1`;
  expected missing store failure, then implement typed tables and composite foreign keys.
- [x] Implement each write in one statement or transaction; use bound parameters,
  random IDs, token digests, unique tenant/external keys and catalog version validation.
- [x] Repeat the database tests; expected all pass with no skipped integration tests.

Example storage acceptance:
```go
id, err := s.Create(ctx, adminA, "Account", map[string]any{"Name":"Acme"})
if err != nil { t.Fatal(err) }
if _, err = s.Read(ctx, adminB, "Account", id); err == nil { t.Fatal("tenant leak") }
```

### Task 3: REST service and executable

**Files:** create `internal/platform/http.go`, `http_test.go`, `cmd/salescompel/main.go`.
**Interfaces:** consumes store; produces NewHandler and CLI `serve` / `provision`.

- [x] Write HTTP tests for authentication, describe filtering, create/update/delete,
  malformed/trailing JSON, method errors, unknown version and cross-tenant spoofing.
- [x] Run HTTP tests red, then route only the documented subset, serialize errors
  as arrays, cap request size and never expose database errors or credentials.
- [x] Add provisioning from environment, TLS certificate/key options, timeouts and
  graceful shutdown. Serve built assets without exposing repository files.
- [x] Run `TEST_DATABASE_URL=... go test -race ./...` and `go vet ./...`; expected pass.

### Task 4: Real connector candidate checks

**Files:** create `compat/clients/node/test/candidate.test.ts`, `scripts/check-candidate.sh`;
modify compatibility package scripts and `docs/compat/README.md`.
**Interfaces:** consumes HTTPS REST; uses `CANDIDATE_URL`, `CANDIDATE_TOKEN`,
`CANDIDATE_RESTRICTED_TOKEN`, `CANDIDATE_OTHER_TOKEN`.

- [x] Write an opt-in Node test using actual JSforce Connection describe, create,
  retrieve, update, query, upsert and destroy calls; require configuration in candidate mode.
- [x] Run against the real server; add Python simple-salesforce HTTPS CRUD and
  CLI query smoke through the existing pinned clients; assert persisted values.
- [x] Script creates temporary trusted TLS material, provisions synthetic principals,
  starts server, runs clients, restarts server and verifies retained records; trap cleanup.
- [x] Run candidate checks plus `npm --prefix compat/clients/node run check`; expected pass.

### Task 5: Localized Vue record workflow

**Files:** create `web/package.json`, lockfile, index, Vite config, src and browser tests;
modify `locales/messages/*.json`, registry coverage and schema only as needed.
**Interfaces:** consumes REST describe/CRUD/query, registry and language catalogs.

- [x] Write Playwright tests for token login, Account creation/edit/delete, Contact
  association, error rendering, French and pt_BR selection, RTL and logout.
- [x] Run browser tests red before UI implementation.
- [x] Build a metadata-driven list and form; use explicit field values, accessible
  labels, loading states, same-origin fetch and in-memory token only.
- [x] Lazy-load catalogs via Vite; use registry fallbacks and direction, preserving
  canonical record data. Test resolver independently with unsupported language fallback.
- [x] Run typecheck, production build and browser tests; expected pass.

### Task 6: Release checks and handoff

**Files:** create `.github/workflows/platform.yml`, `docs/compat/plan-2-implementation.md`;
modify README, `.gitignore`, OpenAPI description and this checklist.
**Interfaces:** CI consumes PostgreSQL service and the same candidate script as local checks.

- [x] Add CI Go unit/integration/race checks, locked frontend build, real connector and UI gates.
- [x] Document exact route/query limits, provisioning, TLS trust, startup, restart,
  test commands and remaining certification blockers.
- [x] Run all checks and one fresh whole-change review, fix important findings with
  regression tests, record results and mark only verified tasks complete.
- [x] Leave changes on the local feature branch for review; do not push or merge.
  Superseded 2026-10-15: the user directed that all work happens on `main` until
  every roadmap milestone is complete; Plan 2 is committed on `main`.

## User steering — release-history milestone

The roadmap now includes M1R: review all releases and linked changelogs from
the official previous-release archive, with a coverage ledger, contract/test
traceability, explicit access blockers and recurring release-delta reviews.
This is a future milestone; adding it does not claim the archive was reviewed.
