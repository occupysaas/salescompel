# Plan 24 — Atomic Composite writes

Status: implemented, independently reviewed, corrected and verified, 2026-11-04. [Measured acceptance and limits](../../compat/plan-24-atomic-composite-writes.md).
Milestone: bounded M3 transaction slice; no M3 exit-gate or Salesforce parity claim.

## Intent and selection

Allow a client to create an Account and its Contact in one atomic request, and prove that a later failure undoes earlier creates, updates and deletes. This closes the missing rollback portion of the roadmap's first vertical scenario while reusing the candidate's existing validation, permissions and PostgreSQL storage.

Plan 24 has no preassigned topic in the repository. Atomic Composite writes are the author's recommended next package after [Plan 23](../../compat/plan-23-served-candidate-openapi.md), chosen from the [M3 roadmap](../../plans/2026-10-14-salesforce-compatibility-roadmap.md). This was an author recommendation in the draft; the maintainer subsequently requested implementation of the reviewed specification and plan. Plans 10/14, real target inventory and unimplemented Plan 20 retain their existing scope and status.

The [accepted pilot observation](../../compat/observations/pilot-black-box-baseline.yaml) already records a create followed by an invalid-field write: both subresponses were 400, with PROCESSING_HALTED and INVALID_FIELD respectively, and zero Accounts remained. [The scenario](../../../compat/clients/node/src/scenarios.ts) supplies the precise request. Only that bounded observation supports a measured reference claim; it does not establish every behavior below.

## Global constraints

- Work directly on main under the repository convention.
- Use Go 1.25.0, PostgreSQL 17 and the existing compat Node >=22 toolchain with pinned dependencies; no new dependency, lockfile or manifest changes.
- Preserve existing standalone CRUD/query/upsert/authentication behavior, catalog/schema and UI; only the bounded Composite route and its discovery/contract entries add product behavior.
- Preserve api/openapi.yaml, CLEAN_ROOM.md and historical plans, profiles, observations and reports; append new source-registry entries only.
- Use synthetic fixtures and the existing disposable candidate acceptance workflow; no live Salesforce org, proprietary implementation, copied examples or external communication.
- Never publish tokens, Authorization headers, record values, raw request/response bodies, SQL or database error details in diagnostics or acceptance reports.
- Atomic success is reported only after PostgreSQL commit succeeds; any failure before commit must leave no partial writes.

## Alternatives and boundary

The selected design adds a typed transaction executor using existing storage helpers. Routing subrequests through an HTTP loopback would use separate connections and fail atomicity. Implementing full Composite, partial success and savepoints together would add independent contracts before the existing rollback scenario works. Neither approach is selected.

Non-goals: allOrNone=false/default behavior; collation; GET/query/describe/upsert subrequests; collections/batch/graph/tree; nested Composite; arbitrary headers or URLs; expression evaluation; new objects/fields; idempotency/retry guarantees; new locking/isolation policy; automation; reference-org captures; migration or milestone certification.

## Source and evidence basis

Public primary documentation read on 2026-11-04, independently summarized:

| Source | Limited use |
| --- | --- |
| [Composite endpoint](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-composite-post.html) | Ordered dependent API work, common user context and maximum 25 subrequests. |
| [Request properties](https://developer.salesforce.com/docs/platform/api-rest/guide/requests-composite.html) | Field names, reference notation and explicit all-or-none rollback. |
| [Result properties](https://developer.salesforce.com/docs/platform/api-rest/guide/resources-composite-subrequest-result.html) | Response envelope fields; failure under all-or-none changes other subresponse statuses to 400. |

These mutable documentation pages describe a wider API than this plan. No current release/version parity is inferred. The candidate intentionally requires explicit allOrNone=true, permits only absent/false collateSubrequests, and implements only earlier-create ID references. Documentation says modern Salesforce defaults collation to true; deterministic sequential execution when omitted is an explicit candidate limitation. Exact unsupported-input errors below are candidate decisions. During implementation, append dated source records for these pages, this design and pinned repository source/observation locators; preserve existing provenance states. Copy no Salesforce prose, examples or implementation.

## HTTP and request contract

Add authenticated POST `/services/data/v65.0/composite` and `composite: /services/data/v65.0/composite` to v65.0 resource discovery. Reuse the existing principal, 15-second context, security/no-store headers and 1,048,576-byte request limit. Never authenticate subrequests independently. Unsupported outer methods use existing 405/Allow: POST; unsupported versions remain 404; missing/bad bearer remains 401.

The sole accepted top-level object has required `allOrNone: true` and `compositeRequest` array with 1–25 elements; optional `collateSubrequests` must be false. Reject unknown properties, nulls and wrong types. Each subrequest has exactly required `method`, `url`, `referenceId`, plus `body` only for POST/PATCH. Reject any `httpHeaders`, including an empty map; callers cannot override the principal. Decoder rejects trailing JSON and unknown envelope/subrequest fields. Preserve existing duplicate-JSON-key handling; changing the global JSON parser is outside scope.

`referenceId` is unique, case-sensitive ASCII matching `[A-Za-z0-9][A-Za-z0-9_]{0,79}`. The 80-character bound is candidate-specific. Methods are uppercase and limited to:

| Method | Exact URL shape | Body |
| --- | --- | --- |
| POST | `/services/data/v65.0/sobjects/Account` or `/services/data/v65.0/sobjects/Contact` | Non-null JSON object, validated by existing create rules. |
| PATCH | The same object paths plus one literal record ID or `@{earlier.id}` | Non-null JSON object, validated by existing update rules. |
| DELETE | The same object paths plus one literal record ID or `@{earlier.id}` | Absent, including no explicit null. |

URLs are service-relative raw strings, never network destinations. Reject scheme/authority, query, fragment, backslash, percent escapes, repeated/trailing slashes, dot segments, other versions/resources/objects and extra path segments. Literal record IDs are exactly 18 ASCII alphanumeric characters; this is a Composite subset limit, not a change to standalone paths.

Only these references are supported: a complete record-ID segment in PATCH/DELETE, or the entire string value of Contact `body.AccountId`, written `@{referenceId.id}` and targeting an earlier POST. Contact.AccountId references must target an Account POST. Any placeholder marker `@{` elsewhere, malformed/embedded expression, reference in a property name, unknown/self/forward reference, wrong-case `.Id`, non-POST target or unsupported selector is rejected before transaction start. AccountId may still be a literal ID or null under existing rules. Ordinary field names/values, including unknown fields, go through existing Validate during execution; do not reject the observed invalid-field scenario as envelope syntax.

Prevalidate every envelope, route, body shape and reference before any write. Invalid JSON/body size uses HTTP 400 `JSON_PARSER_ERROR`; structural/subset/reference errors use HTTP 400 `INVALID_COMPOSITE_REQUEST`. Responses are the existing error array with a bounded static message; do not echo supplied URL, reference, body or decoder errors. No transaction starts on these errors. These exact error mappings are author decisions for the candidate.

## Execution and response contract

After prevalidation, begin one pgx transaction at the existing PostgreSQL default isolation. Execute in array order with the same Principal and the same transaction handle. Reuse `create`/`update`; extract a database-parameterized `deleteRecord` helper from Store.Delete. Standalone methods keep using s.pool. Do not change Select/Read/Upsert, use per-subrequest transactions/savepoints, call NewHandler recursively, or perform HTTP requests.

Resolve references from successful earlier create results immediately before a write. Generated IDs stay private until commit. Existing validation, field permissions, tenant predicates and composite relationship foreign keys apply to every write. SQL failure aborts execution; do not attempt further SQL in an aborted transaction. Request cancellation, execution error and commit error invoke rollback with a bounded cleanup context independent of the canceled request. No background retry or reuse of a canceled connection.

On committed success, outer HTTP 200 returns exactly `{compositeResponse: [...]}` in request order. Every entry has exactly `body`, `httpHeaders`, `httpStatusCode`, `referenceId`. `httpHeaders` is `{}`. POST entries have 201 and the existing `{id,success:true,errors:[]}` body; PATCH/DELETE have 204 and body null. No generated success ID is returned before commit.

On a normal subrequest API error, rollback everything, then return outer 200 and one entry per input: the failing entry retains its existing error status/code/message/optional fields; every other entry, including earlier successes and unexecuted later entries, becomes 400 with a one-element error array, code `PROCESSING_HALTED`, message `Composite transaction rolled back`. Headers remain `{}`; no generated IDs or success bodies survive. Stop at the first failure. This generalizes the observed rollback case as candidate behavior.

Unexpected storage, begin, rollback or commit failure returns the existing outer 500 SERVER_UNAVAILABLE envelope, with no Composite success entries. Cancellation must not report a committed success when commit failed. A lost connection during commit can leave the client uncertain about the outcome; no replay safety is claimed. A success lost during response delivery is likewise an unknown client outcome.

## Code boundaries and interfaces

Create `internal/platform/composite.go` for exported wire structs and typed storage execution, and `internal/platform/http_composite.go` for strict parsing/route validation/reference compilation. Names/signatures are fixed for the implementation plan:

- `CompositeRequest { AllOrNone bool; CollateSubrequests *bool; CompositeRequest []CompositeSubrequest }`.
- `CompositeSubrequest { Method string; URL string; ReferenceID string; Body map[string]any }`.
- `CompositeResponse { CompositeResponse []CompositeSubresponse }`.
- `CompositeSubresponse { Body any; HTTPHeaders map[string]string; HTTPStatusCode int; ReferenceID string }`.
- JSON tags are exactly the field names in the HTTP contract, including `referenceId`, `httpStatusCode` and `httpHeaders`.
- `decodeComposite(r io.Reader) (CompositeRequest, error)` preserves field presence using private raw wire types, then calls `validateComposite(request CompositeRequest) error`.
- `validateComposite` also checks typed input; `Store.Composite(ctx context.Context, p Principal, request CompositeRequest) (CompositeResponse, error)` calls it before Begin, so callers cannot bypass route/reference validation.
- `deleteRecord(ctx context.Context, db database, p Principal, object, id string) error` preserves Store.Delete semantics.

Private parsed route/reference types and rollback helper names are builder choices. No new general HTTP router or expression engine. Missing/explicit-null DELETE bodies must remain distinguishable in the decoder even though typed nil means absent.

## Served contract and traffic validation

Extend `api/candidate-openapi.yaml` with exactly one operation, `candidateCompositeAtomic`; total inventory becomes twenty. Keep OpenAPI 3.2.1, local references and the existing schema subset. Add concrete closed envelope/subrequest/result components, method-specific oneOf branches, minItems 1/maxItems 25, allOrNone const true, optional collateSubrequests const false, referenceId pattern/maxLength 80, fixed URL patterns, and body schemas based on existing Account/Contact writable fields. Contact.AccountId also accepts the supported reference string. Describe ordering/reference resolution/policy/transaction semantics in prose and behavior tests.

Write bodies remain syntactically open to unknown field names at the Go execution boundary, but the contract enumerates valid fields; the observed invalid-field request deliberately has request-schema issues and a valid error response. Result body alternatives are concrete create-result, Error[] or null, with 201/400/404/204 statuses as applicable; envelope and successful field-policy semantics require separate assertions. Do not broaden a schema to accept arbitrary JSON. Keep `httpHeaders` a closed empty object with maxProperties 0: this is the only new empty-object allowance in the contract guard, not permission for `{}` record schemas. Add maxProperties to the traffic validator's supported keyword set with a regression. Reuse pattern/oneOf support; do not add general OpenAPI machinery.

Update offline inventory guards/tests, the current discovery schema and live inventory from nineteen to twenty. The old rejection mutation must now add an unsupported `/composite/batch` operation; it must not reject the newly supported endpoint. Historical Plan 23 documents remain nineteen-operation evidence for their measured revision.

## Acceptance and fixture ownership

Add parent identity `candidate/composite/atomic` to the existing owned candidate Node process and required jsforce-gate identities. This raw HTTP supplement is not another independent JSforce client. Preserve reporter identity, explicit runner file inclusion, offline allowed-skip schema and strict full-run publication. Database required identities add `TestCompositeAtomic` and `TestHTTPCompositeAtomic`; all previous required coverage stays required.

The live suite validates Composite exchanges against the served document and independently reads/queries affected records. Cover linked creation, PATCH/DELETE success, invalid-field rollback, undo of updates/deletes, restricted field denial with total rollback and cross-tenant failures. The ordinary OpenAPI live inventory also makes one valid one-write Composite exchange so twenty successful operation IDs remain independently required.

Before sending any Composite POST, durably journal one distinct existing-format intent per possible create. Immediately persist every independently attributable, structurally valid returned success ID before whole-envelope, schema or behavior assertions, even if another entry is malformed. Only an exact expected create reference with a unique response identity may resolve an intent; ambiguous associations stay unresolved. Then validate the complete response. A complete, expected rollback envelope or expected outer 400 prevalidation response may resolve its create intents as rejected only after independent exact synthetic-marker queries verify no fixture rows remain. If a regression left a row, record its exact ID as known before failing, so existing cleanup can delete it. Never infer absence from PROCESSING_HALTED alone. Transport, parse, commit/outer-500 or ambiguous response leaves unknown intents unresolved and makes cleanup/acceptance fail. Do not add marker-based blind deletion or a new recovery tool.

Only synthetic run-owned markers are queried with LIMIT 26. One match is recorded against its original intent. For multiple matches, journal each returned ID under a fresh intent/known pair using `<original-operationId>__recovered_<recordId>`, preserving the original unresolved intent; fail the suite and cleanup eligibility for manual resolution. The 26-row boundary is an overflow signal, never proof that all matches were recovered. This preserves the existing one-ID-per-operation journal invariant while permitting exact-ID cleanup of discovered rows. The fixture helper receives every expected subrequest identity/method and an expected outcome (commit, rollback or prevalidation); create intents alone cannot establish response completeness. Caller reference IDs/fixture operation IDs must be unique across the whole run. Reject unexpected duplicates. Reuse the journal and exact-ID Contact-before-Account cleanup. Preserve preexisting rows used by rollback tests; seed them through existing tracked standalone fixtures. Restricted fixtures use permitted marker fields. A failure while journaling any intent must prevent dispatch of the whole request.

Required automated evidence:

1. Pure validation tests for all shape/count/route/reference boundaries and zero writes on prevalidation failure.
2. Real PostgreSQL tests for commit, linked IDs, ordered writes, all three rollback kinds, SQL constraint failure, policy/tenant isolation, cancellation and no leaked success on failed commit. Use a package-private transaction seam to force commit/rollback errors deterministically; real DB tests still prove persistence effects.
3. HTTP tests for auth/method/version/limit/security/discovery, exact envelopes and no reflected private markers.
4. Offline document/traffic and acceptance-wiring regressions, including omitted/skipped new identities and ambiguous fixture outcomes.
5. Existing full candidate acceptance with all eleven gates passing, zero new full-run skips, cleanup verified and committed source bindings. Prerequisite failures remain named blockers, never substituted with offline passes.

## Completion and publication

Create `docs/compat/plan-24-atomic-composite-writes.md` only during execution, recording actual commands, source/evidence distinctions, coverage, safe acceptance digest, review findings and limitations. Update current README/candidate-guide serving statements to name the restricted atomic subset; do not rewrite historical evidence. A fresh whole-change review must examine atomicity, principal propagation, parser boundaries, commit uncertainty, contract truthfulness and fixture handling. Completion requires corrected findings and measured acceptance. This specification and its implementation plan alone authorize no claim that Composite is implemented or that any milestone is complete.

## Builder check

A fresh builder reviewed both drafts on 2026-11-04. Its two material fixture questions are resolved in the acceptance contract: full expected subrequest metadata is required, and duplicate marker matches use distinct recovery identities while the original intent remains unresolved. Individually attributable IDs are recorded before global response validation. These were author decisions at design review. Subsequent execution and acceptance are recorded in the linked report.
