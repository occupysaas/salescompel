# Plan 31 - Composite II and sObject Collections Implementation Plan

> **For agentic workers:** use superpowers:subagent-driven-development or
> superpowers:executing-plans, task by task. Steps use checkbox syntax.

**Status:** Implemented, 2026-11-15. [Implementation record](../../compat/plan-31-composite-collections.md#implementation-record-2026-11-15).

**Goal:** Composite partial success, read/query/upsert subrequests and general
references; the six sObject Collections operations; JSforce and raw HTTP
acceptance.

**Spec:** [Plan 31 design](../specs/2026-11-15-plan-31-composite-collections-design.md).
Read it first. Every rule below comes from the spec; where they differ, the
spec wins and the plan is corrected.

**Reference record:** [plan-31-composite-collections.md](../../compat/plan-31-composite-collections.md).

## Global constraints

- Go 1.25.0 (`GOTOOLCHAIN=go1.25.0` or the `go1.25.0` wrapper), PostgreSQL 17,
  pgx v5, API v65.0. No new runtime dependency.
- Every write through `records.Tx`. Partial success through pgx nested
  `Begin` (savepoints). One PostgreSQL transaction per HTTP request.
- Plan 24's `candidate/composite/atomic`, `TestCompositeAtomic` and
  `TestHTTPCompositeAtomic` stay green without editing their assertions,
  except the two decided changes (spec decisions 2 and 3), which update the
  tests that pinned the old rejection.
- No source file over 500 lines: `sh scripts/check-file-size.sh` before every
  commit. `http.go` must shrink.
- Every commit `git commit -s`. One PR for the plan, a commit per task.

## Review focus

- **Savepoint recovery.** After a mapped constraint error inside a savepoint
  the transaction keeps working; after an unmapped one the request is a 500
  with no success entries. `TestCompositePartialRecovery`.
- **Reference re-routing.** A referenced value containing `/`, `?` or `..`
  never reaches a different route. `TestCompositeRefReroute`.
- **SOQL substitution boundary.** A referenced value that alters `q` still
  cannot cross tenants or read a denied field. `TestCompositeRefQueryScope`.
- **Collections all-or-none.** Already-written items keep their `id`, report
  `ALL_OR_NONE_OPERATION_ROLLED_BACK`, and nothing persists.
  `TestCollectionsAllOrNone`.
- **Retrieve at 1,000 and 2,000 IDs.** Chunked, ordered, nulls in place.
  `TestCollectionsRetrieveChunks`.
- **Upsert lock order.** Two concurrent upsert batches with overlapping keys
  in opposite order both finish. `TestCollectionsUpsertLockOrder`.

## File map

| Files | Responsibility |
| --- | --- |
| `internal/platform/http_sobject_read.go`, `query_handler.go` | Moved out of `http.go`: record GET with `fields`, shared `/query` function |
| `internal/records/write.go`, `internal/metadata/*` | `ReadFields`; snapshot `ObjectByPrefix` |
| `internal/platform/composite_ref.go` (+ test) | Reference grammar, resolution, substitution |
| `internal/platform/composite_route.go` (+ test) | Subrequest URL routing, re-routing, dispatch |
| `internal/platform/http_composite.go` (+ test) | Envelope decoding, prevalidation, limits |
| `internal/platform/composite.go` (+ test) | Executor, savepoints, error classes |
| `internal/platform/collections.go`, `http_collections.go` (+ tests) | Collections executor, routes, limits |
| `api/candidate-openapi.yaml`, `compat/clients/node/src/candidate-openapi-*.ts` | Operations and guards |
| `compat/clients/node/src/candidate-collections*.ts`, `test/candidate-collections.test.ts`, acceptance wiring files | Acceptance |

---

### Task 1: Black-box client probe

**Files:** `docs/compat/plan-31-composite-collections.md`.

- [ ] Start the candidate with the existing disposable workflow. Point
      unmodified JSforce 3.11.0 at it with request logging on the candidate
      side only (method, path, query keys, body keys; no values, no tokens).
- [ ] Call `create`, `update`, `upsert(…, 'Compat_Key__c')`, `destroy` and
      `retrieve` with arrays of 2 and with `allOrNone` true and false. Record
      which path and method each call sends, and whether `retrieve` sends
      `fields`. The calls will fail today; only the request shape matters.
- [ ] Add the results as `observed (candidate-side, client black-box)` rows.
      If `retrieve` omits `fields`, record the spec's fallback (all readable
      fields) as a candidate decision.
- [ ] Commit `docs: record Plan 31 client request shapes`.

### Task 2: Move routes out of http.go; `ReadFields`

**Files:** `http.go`, new `http_sobject_read.go`, `query_handler.go`,
`records/write.go`, tests.

- [ ] Pure move first (one commit, reviewable with `git diff --color-moved`):
      record GET and `/query` (including the Organization and currency cases)
      into the two new files. All existing HTTP tests pass unchanged.
- [ ] Failing tests: `TestReadFieldsSubset`, `TestReadFieldsUnknown`,
      `TestHTTPReadFieldsParam` (unknown field 400 `INVALID_FIELD`; absent
      param byte-identical to today).
- [ ] Implement `Tx.ReadFields(ctx, object, id string, fields []string)`;
      `Read` calls it with all fields.
- [ ] `go1.25.0 test ./internal/records ./internal/platform -count=1`.
- [ ] Commits `refactor: move record read and query routes out of http.go`,
      `feat: accept fields on record GET`.

### Task 3: Reference grammar

**Files:** `composite_ref.go`, `composite_ref_test.go` (pure Go, no database).

- [ ] Failing tests: `TestRefParse` (steps, leading index, malformed `@{`),
      `TestRefResolve` (nested, index, null, missing path, object in
      embedded position), `TestRefSubstituteBody` (whole-value keeps JSON
      type; embedded renders text), `TestRefSubstituteURL` (path-segment
      percent-encoding, query-value encoding, forbidden positions).
- [ ] Implement. Resolution reads the earlier entry's JSON body as
      `any` decoded with `UseNumber`.
- [ ] Commit `feat: parse and resolve Composite references`.

### Task 4: Envelope and routing

**Files:** `http_composite.go`, `composite_route.go`, tests.

- [ ] Failing tests: `TestDecodeCompositeEnvelope` (optional allOrNone,
      collate true accepted, body rules per method, httpHeaders rules),
      `TestCompositeLimits` (25 total; 5 query/Collections; record GETs not
      counted), `TestCompositeRoutes` (each table row, other versions and
      routes as item 404s, upsert on another field 404),
      `TestCompositeRefPrevalidate` (forward, self, unknown; bad referenceId
      gives `JSON_PARSER_ERROR`), `TestCompositeRefReroute`.
- [ ] Update the Plan 24 decoder tests that pinned rejection of omitted
      `allOrNone` and the old referenceId code (decisions 2 and 3); every
      other Plan 24 assertion untouched.
- [ ] Implement routing as a small table of route kinds, each with a
      template matcher that treats references as opaque tokens.
- [ ] Commit `feat: decode and route Composite II subrequests`.

### Task 5: Executor

**Files:** `composite.go`, `composite_test.go` (PostgreSQL).

- [ ] Failing tests: `TestCompositePartialSuccess` (middle failure,
      dependents get `PROCESSING_HALTED`, independents commit),
      `TestCompositePartialRecovery`, `TestCompositeAllOrNoneMixed` (read,
      query, upsert, then failure; nothing persists; Plan 24 response shape),
      `TestCompositeReadQueryUpsert` (bodies equal standalone route bodies),
      `TestCompositeRefQueryScope`, restricted-user and cross-tenant cases.
- [ ] Implement the three error classes, per-subrequest savepoints for
      `allOrNone: false`, and dispatch through `composite_route.go`.
- [ ] Commit `feat: run Composite subrequests with partial success`.

### Task 6: Collections writes

**Files:** `collections.go`, `http_collections.go`, `internal/metadata`
(`ObjectByPrefix`), tests.

- [ ] Failing tests: `TestCollectionsDecode` (every request error in the
      spec, 200/10-chunk limits), `TestCollectionsCreateUpdateDelete`
      (partial and all-or-none, mixed types, delete across objects by
      prefix, `MALFORMED_ID`), `TestCollectionsUpsert` (`created` flag,
      missing key `REQUIRED_FIELD_MISSING`, duplicate keys run in order),
      `TestCollectionsAllOrNone`, `TestCollectionsUpsertLockOrder`,
      HTTP tests for the three write routes and DELETE query parsing.
- [ ] Implement body handling (strip `attributes`, extract `id` and key),
      per-item savepoints, batch savepoint for all-or-none, sorted advisory
      locks for upsert.
- [ ] Commit `feat: add sObject Collections writes`.

### Task 7: Collections retrieve

- [ ] Failing tests: `TestCollectionsRetrieve` (order, nulls for malformed,
      wrong object, missing and unreadable), `TestCollectionsRetrieveChunks`
      (1,000 and 2,000 IDs), field errors, GET 800-ID cap, POST body form.
- [ ] Implement with `ReadFields`' field resolution and chunked SOQL with an
      explicit batch size; chunks of 150 because of the SOQL WHERE length limit.
- [ ] Commit `feat: add sObject Collections retrieve`.

### Task 8: Collections inside Composite

- [ ] Failing tests: `TestCompositeCollectionsOuterAllOrNone` (outer wins,
      entry 400 with item array, others `PROCESSING_HALTED`),
      `TestCompositeCollectionsOuterPartial` (inner flag respected, item
      failures do not fail the entry), reference into a Collections result
      (`@{coll[0].id}`).
- [ ] Implement by calling the Collections executor from the dispatcher
      inside the subrequest savepoint.
- [ ] Commit `feat: run Collections as Composite subrequests`.

### Task 9: OpenAPI and contract guards

- [ ] Update `candidateCompositeAtomic` description and schemas; add six
      Collections operations with `x-source` pointing at this spec and the
      reference record sources; add the body-reference schema exception.
- [ ] Update the operation-count guard (twenty plus six) and traffic tests.
- [ ] `npm run check` in `compat/clients/node`.
- [ ] Commit `feat: describe Composite II and Collections in OpenAPI`.

### Task 10: Acceptance

- [ ] `candidate-collections-fixtures.ts`: journal every possible create
      before dispatch; record returned IDs before assertions.
- [ ] `test/candidate-collections.test.ts`: `candidate/collections/jsforce`
      (five calls, both flags, persisted state checked by query) and
      `candidate/composite/partial` (raw HTTP: partial success, query feeding
      a reference, nested Collections).
- [ ] Wire both names into `candidate-acceptance-run.ts`,
      `candidate-acceptance-contract.ts` (names and required Go tests),
      `candidate-acceptance-results.ts`, and
      `compat/schemas/candidate-acceptance.schema.json`.
- [ ] Full run: `CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <private-summary>`.
      Record the gate table in the reference record.
- [ ] Commit `test: add Composite II and Collections acceptance`.

### Task 11: Records and docs

- [ ] Complete the reference record: final status per row, gaps, measured
      acceptance, tool versions.
- [ ] Append source entries to `docs/compat/sources.yaml` (keep existing
      bytes).
- [ ] Add the implementation-record line under Plan 31 in the first-release
      roadmap; mark this plan implemented.
- [ ] `sh scripts/check-file-size.sh`, `scripts/check-dco.sh upstream/main HEAD`,
      `scripts/check-migrations.sh` (no migration expected).
- [ ] Commit `docs: record Plan 31 implementation`; open the PR with
      `Closes #21`, commands run and their output.
