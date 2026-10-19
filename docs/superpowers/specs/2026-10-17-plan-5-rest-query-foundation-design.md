# Plan 5 — REST/query foundation design

> Maintainer update, 2026-10-19: expiry windows longer than one hour may use
> explicit assumptions without live testing or waiting. The foundation's
> documented two-day lifetime is such an assumption. See the
> [current decision gate](../../compat/plan-5-contract-decisions.md). This
> overrides live-expiry requirements below only; other evidence and cleanup
> requirements remain. Historical captures are not reclassified as passing.

Date: 2026-10-17. Status: written spec approved by the maintainer, with the
explicit requirement to retain the same routes for one-to-one drop-in use.
This is not an implementation or evidence report.

## Intent and success criteria

The maintainer selected the full REST/query foundation as the next work package
and approved its proposed design. Deliver an evidence-backed improvement to the
existing Account/Contact candidate: pagination, record-ID handling, name and text
semantics, null ordering, HTTP errors and an accurate served OpenAPI document.
Success requires repeatable focused reference observations, candidate comparisons,
tenant/access isolation and retention of existing client and browser workflows.

This is one M3 foundation slice. It does not complete M3, the wider M0/M1 gates,
the full M1R archive review or production certification. The existing provisioned
bearer authentication and fixed catalog remain the supported platform scope.

## Starting point and constraints

[Plan 4's handoff](../../compat/plan-4-reference-baseline.md) records two successful
eleven-scenario reference captures. It also identifies dimensions that those
captures did not measure; pilot evidence must not be promoted into broader claims.

The candidate currently limits queries to 2000 records, has no continuation,
accepts only 18-character record IDs, resolves catalog names case-sensitively,
compares text with PostgreSQL equality and always orders nulls last. Its HTTP
adapter returns query results as a single completed page. The proposed
`api/openapi.yaml` is broader than the implementation and is not served.

Follow [the current black-box policy](../../../CLEAN_ROOM.md). Use public APIs,
synthetic owned fixtures and unmodified pinned clients. Do not inspect Salesforce
CLI/plugin implementations or copy Salesforce prose. Keep credentials, raw
captures and recovery journals private. Existing named-tenant authorization
continues to apply; missing login factors are concrete setup dependencies only.
Work stays on main under the recorded maintainer preference.

## Scope and alternatives

### One-to-one route compatibility

The maintainer explicitly requires the same routes for drop-in use. For every
supported operation preserve the Salesforce HTTP method, versioned path, path
parameter placement, query parameter names, header semantics and response-link
shape. Clients may change instance origin and credentials; they must not need
SalesCompel-specific paths, parameter rewrites, SDK patches or pagination adapters.
Retain existing discovery and own-user routes, including the narrow v51.0 CLI
discovery compatibility path. Do not imply general v51.0 support from that alias.

Continuation uses the observed `/services/data/v65.0/query/{queryLocator}` route
family through `nextRecordsUrl`, not a new `/cursor` route or a custom page token
parameter. Locator values are opaque and independently generated. Preserve URL
escaping, including encoded external-ID slashes; do not redirect writes or rewrite
resource names merely to fit internal package boundaries. Capture route-level
request fixtures and replay them against both origins to verify this requirement.
Unsupported operations remain explicit gaps, not substitute custom endpoints.

The selected approach combines focused evidence with implementation in reviewable
slices. Pagination-only would deliver earlier but leave the handoff's related
semantic gaps. Evidence-only would defer useful candidate improvements into a
separate work package. Neither is the selected scope.

Included contracts are `query-response-shape`, `query-limit-above-batch`,
`id-15-18`, `case-insensitive-names`, `case-insensitive-comparison`,
`null-ordering`, `error-shape` and `unknown-sobject`. Batch-size header behavior
is included as a scoped dimension of `request-headers`, not completion of that
entire contract. Upsert and describe change only where these selected contracts
require it; their broader unobserved dimensions stay deferred.

Exclude SOAP, Bulk, Composite, OAuth lifecycle, recycle-bin behavior, general
sharing, dynamic metadata, full SOQL/SOSL and unrelated UI features. Keep the
existing field-list projection, single equality predicate, single ordering field
and Contact.Account.Name traversal. Extend that grammar only for the selected
LIMIT and explicit null-placement behavior. Do not add SQL passthrough.

## Evidence before semantic decisions

Extend the existing Node laboratory rather than replace its readiness, capture,
recovery, repeatability or observation-validation mechanisms. Register a separate
focused suite/manifest and explicit scope so that neither its success nor a
candidate run can be confused with the mandatory eleven-scenario pilot suite.
Preserve the pilot profile bytes and historical evidence bindings. If focused
testing needs a different profile, bind that profile and its readiness separately.

Each probe records inputs, wire status, response structure, selected values,
resulting state, API/profile/client bindings and cleanup. Use direct HTTP capture
where client wrappers hide statuses or body properties. Publish independently
written sanitized observations with reproducible provenance and reciprocal
contract links. Require two sequential successful captures and repeatability;
normalization must not hide order, casing, status or missing/duplicate records.

| Area | Required observations |
| --- | --- |
| Pagination | Empty, exact-boundary and multi-page results; default/requested batch sizes and invalid headers; LIMIT above 2000; totalSize/done/nextRecordsUrl; attributes when Id is not projected; relationship projection |
| Continuation | Replay, malformed/unknown/expired locators, same-user fresh session, different user, concurrent inserts/updates/deletes and ordering ties |
| IDs | 15/18-character forms in supported CRUD paths, relationship values and ID predicates; altered casing/checksum, malformed values and wrong-object prefixes |
| Names | REST object/field paths, JSON field names and SOQL identifiers; canonical output; unknown names, duplicate canonical keys and denied-field casing variants |
| Text/null | Mixed case, accents and locale-sensitive examples for supported text fields; null equality; ASC/DESC defaults and explicit NULLS FIRST/LAST |
| Errors | Raw status/code/body fields for malformed JSON/query, unknown object/field, denied field, missing record and invalid IDs; failed writes leave state unchanged |

Observe only within the authorized fixture scope. Candidate cross-tenant tests
use two local tenants; do not assume access to a second reference organization.
If expiry or another required dimension cannot be measured reliably, record the
gap and block acceptance of that dimension rather than infer a reference rule.
Existing field uniqueness/collation settings must inform comparisons; do not
apply a universal lowercase transformation to IDs, text and external IDs.

## Release-history reconciliation

Create the roadmap's `docs/compat/releases/index.yaml` and `changes.yaml`. Inventory
the official archive at a recorded cutoff, retain every discovered source's
review state and prioritize supported-version and selected-contract findings.
Record independently summarized effects, effective versions, activation rules,
affected contracts/tests and compatibility decisions. Unavailable sources need
an owner, reason and resolution path; they are not reviewed evidence.

Reconcile findings relevant to this slice before freezing its implementation
contracts. The remaining archive review stays open under M1R. A required-profile
uncertainty blocks the affected contract's acceptance; recording a blocker does
not waive it. Documentary expectations and live observations remain distinct.

## Architecture and interfaces

Keep one Go service and PostgreSQL. Extract parsing and query semantic resolution
into `internal/query`, with a parsed query, canonical resolved query and explicit
ordering/null-placement representation. The package must not depend on HTTP or
pgx. A narrow catalog/policy interface supplied by the platform resolves allowed
objects, fields and relationships. Resolve canonical names before checking access;
selecting, filtering or ordering by a denied field must still fail closed.

Keep PostgreSQL query execution, record access and cursor persistence in focused
platform files behind explicit interfaces. Avoid moving unrelated CRUD solely to
reorganize packages. The HTTP adapter authenticates, decodes transport inputs,
invokes the query service and maps typed results/errors to the wire contract.

The query service exposes initial query and continuation operations returning a
page with total size, records, completion state and an optional opaque locator.
Both operations receive an authenticated principal and request context. Page size
and query LIMIT are separate values. SQL identifiers come only from the resolved
catalog and values remain parameters. Retrieve internal record identity as needed
for attributes.url without adding unrequested Id fields to the output.

Centralize ID parsing/normalization for supported record routes, references and
predicates. Keep existing stored IDs stable. Preserve original input context long
enough to emit route-specific observed errors; normalization must not turn an
invalid or cross-object value into a successful lookup.

Text comparison and sorting use an explicit field-aware strategy selected from
the evidence. Add database integration tests for the observed Unicode examples;
PostgreSQL defaults or ASCII-only folding are not compatibility evidence.

## Cursor lifecycle

Use PostgreSQL-backed cursor state. Process-local state loses continuity at
restart; stateless offsets alone do not reliably preserve the observed result
behavior during mutations. Persistent state permits ownership checks, expiry,
bounded resource accounting and atomic page operations.

Use unpredictable opaque locators with no credentials or client-supplied SQL.
Bind state to tenant, initiating user, API version and resolved query. Authenticate
and recheck access on every continuation, including field permission changes;
persisted results must never bypass current access policy. A different user or
tenant cannot consume the locator. If reference behavior permits broader access,
retain the conservative boundary and report that difference explicitly.

Do not hold a database transaction or connection open across HTTP requests.
Reference mutation/replay observations decide whether state stores a materialized
projection, ordered identities or another bounded representation. Record the
selected representation, expiry, replay/concurrency rules and resource limits in
the evidence reconciliation before implementing the cursor store. These are
evidence-dependent decisions, not permission to choose arbitrary semantics.

Cursor creation and page advancement are atomic. Persist state needed for process
restart; test restart between pages. Enforce per-principal and service-wide bounds
on active state and stored bytes, expire and reclaim state without a new service,
and report exhaustion explicitly. Candidate resource limits are documented as
candidate limits unless observed equivalent to the reference. Never silently
truncate a query or claim completion after dropping rows.

## HTTP errors and OpenAPI

Keep typed internal failures separate from wire mappings. Match observed status,
errorCode and presence/type of fields for the selected routes. Error messages are
independently authored and sanitized; database details and secrets never escape.
Preserve failed-write atomicity, body/time bounds and unsupported-route failures.
Do not equate an operational candidate bound with a measured Salesforce limit.

Serve a reproducible, embedded implemented-surface document at
`/openapi.json`, using the existing bearer authentication boundary. This additive
SalesCompel documentation endpoint stays outside the Salesforce resource namespace
and is never required by an existing client workflow. Do not add a discovery key
that alters the reference contract. Cover implemented methods,
parameters, authentication, success and error bodies, continuation and selected
headers. Mark the document as SalesCompel's contract, not a reference endpoint.
Retain the broader proposal separately and label it clearly.

Attach per-operation source/evidence provenance. Verify the repository's
latest-OpenAPI-version policy and tooling support during execution. Validate both
the document and representative real request/response traffic; schema syntax
validation alone is insufficient. If tooling cannot validate the required dialect,
record and resolve that gap before accepting the served contract. Do not silently
downgrade the document or treat unsupported keywords as validated.

## Delivery sequence and verification

1. Extend focused evidence capture and release ledgers; preserve pilot regressions.
2. Capture/repeat/review the focused reference suite and reconcile applicable
   release findings. Freeze selected semantic decisions with evidence links.
3. Extract query boundaries and implement canonical names, IDs, comparisons and
   ordering with unit and real-PostgreSQL regression tests.
4. Implement persistent pagination, lifecycle bounds and continuation endpoints;
   exercise restart, concurrent requests, mutation cases and access denial.
5. Align selected HTTP errors and serve/validate the implemented OpenAPI contract.
6. Compare focused candidate/reference runs, run existing client/browser smoke,
   and publish an acceptance and remaining-gap report.

The implementation plan must translate these stages into concrete files, tasks
and commands. Include parser and normalization unit tests, real PostgreSQL HTTP
tests, tenant/user/field-access negatives and wire-contract comparisons. Run Go
race tests/vet, the full offline compatibility check and existing candidate smoke
workflow using a disposable local database. Retain JSforce pagination, Python CRUD,
CLI query, restart persistence and browser workflow checks.

Acceptance requires all selected contract dimensions to have reviewed evidence
and passing candidate comparisons, cleanup on both reference runs, no unexplained
normalization, no cross-tenant/user cursor access, no field-policy bypass, and an
accurate validated served document. Explicitly report blocked dimensions and
candidate deviations; they prevent a full-slice compatibility claim. Do not narrow
the pilot manifest or claim the unsupported full pilot candidate suite passes.

## Review and next step

Self-review: scope maps to the approved foundation; historical pilot evidence is
preserved; reference-dependent cursor and semantic decisions have explicit gates;
release-history work retains its broader obligations; security and resource bounds
cannot be waived by parity testing. No product code or live reference operations
were performed while writing this design.

The maintainer approved this written spec and clarified route compatibility. Use
the writing-plans skill to produce the implementation plan with that requirement.
The recorded native sequential execution
preference remains applicable; writing this spec does not start implementation.
