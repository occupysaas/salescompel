# Plan 23 — Served candidate OpenAPI contract

Status: proposed, 2026-11-04. Planning only; implementation and candidate acceptance have not run.
Milestone: bounded M3 API-contract delivery; no milestone exit gate or Salesforce parity claim.

## Intent and selection

Publish a machine-readable contract for the API the candidate actually implements, and make real candidate HTTP request/response checks depend on that document. An authenticated developer must be able to fetch the contract from a deployed binary without needing the source checkout. Unsupported Composite, collections, pagination, migration and metadata deployment must not appear as implemented operations.

The [roadmap](../../plans/2026-10-14-salesforce-compatibility-roadmap.md) requires a served YAML contract, operation provenance and HTTP traffic validation. The [candidate guide](../../compat/plan-2-implementation.md#implemented-surface) explicitly defers serving the broader proposed document. [Plan 22](../../compat/plan-22-package-dependency-feasibility.md) completed synthetic package dependency discovery; its missing real-package evidence cannot be filled by another declaration tool. This plan returns to an explicit candidate/M3 gap while Plans 10/14, real target inventory and unimplemented Plan 20 remain independent work.

Executor recommendation: add a separate truthful candidate document rather than serve the broader proposal or remove its future endpoints. A full Go OpenAPI middleware/library adoption is another option, but it requires its own version/eligibility decision. This package uses existing Node tooling for bounded test-time traffic validation; it does not select a general Go traffic-validation library or finish wider M3 validation.

## Global constraints

- Work directly on main under the repository convention.
- Use Go 1.25.0 and the existing compat Node >=22 toolchain with pinned dependencies; no new dependency, lockfile or manifest changes.
- Author the candidate document as OpenAPI 3.2.1 YAML with JSON Schema 2020-12-compatible component schemas; preserve api/openapi.yaml as the broader proposal.
- Preserve existing CRUD/query/authentication semantics, database/catalog/schema, UI and accepted Salesforce profiles/observations; only contract serving and its discovery link add product behavior.
- Preserve historical Plans 10/14/17/18/19/20/21/22 and CLEAN_ROOM.md; source-registry changes append new entries only.
- Use synthetic fixtures and the existing authorized disposable candidate acceptance workflow; no live Salesforce org, proprietary implementation, copied examples, new source reuse or external communication.
- Never log, persist or return bearer tokens, raw Authorization headers, record values or request/response bodies in traffic-validator diagnostics.

## Documentation and version basis

Public sources read on 2026-11-04:

| Source | Limited basis |
| --- | --- |
| [OpenAPI 3.2.1](https://spec.openapis.org/oas/v3.2.1.html), [published latest](https://spec.openapis.org/oas/latest.html) | Latest currently identifies 3.2.1, dated 2026-09-10; paths, media, security, reference and schema vocabulary. Existing 3.2 minor policy accepts this patch version. |
| [Ajv JSON Schema versions](https://ajv.js.org/json-schema.html) | Existing Ajv2020 entry point validates the selected 2020-12 schema subset; this is not a complete OpenAPI traffic validator. |
| [Go embed, version 1.25.0](https://pkg.go.dev/embed@go1.25.0) | Embed a repository YAML file in a binary without a runtime filesystem dependency. |

A read-only planning probe used existing `validateOpenApi`/Validator to accept a minimal 3.2.1 document and existing Ajv2020 to accept nullable String/reject Number. That establishes the narrow tooling prerequisite, not validation of the unwritten full contract. Recheck exact constructs during implementation and record URL/section/date/applicability. Copy no examples or implementation. Current candidate behavior comes from this repository and measured candidate tests, not Salesforce documentation or reference observations.

## Single source and serving behavior

`api/candidate-openapi.yaml` is the sole hand-written candidate contract. A new Go package `apicontract` in `api/candidate_contract.go` embeds it as a private String and exports `CandidateOpenAPI() string`, `CandidateOpenAPIPath = "/services/data/v65.0/openapi.yaml"` and `CandidateOpenAPIContentType = "application/yaml; charset=utf-8"`. There is no generated JSON twin, runtime YAML parser, filesystem lookup or Node runtime dependency in the service.

Add GET serving after the existing bearer authentication and normalized service-path selection in `internal/platform/http.go`. Return the exact embedded bytes, 200, the content type above and existing security/no-store headers. Missing/invalid bearer remains the existing JSON 401 response. Other methods return the existing JSON 405 with `Allow: GET`; HEAD is not newly supported and actual net/http HEAD responses suppress the body. Unsupported API versions remain 404. The contract is static and identical for admin, restricted and other-tenant principals; it contains no tenant/user/token/record values. Static synthetic field definitions are API documentation, not a grant to access those fields.

Add exactly `openapi: /services/data/v65.0/openapi.yaml` to the authenticated v65.0 resource map. Use `servers: [{url: "/"}]`; request Host/forwarding headers cannot rewrite the contract. No Accept negotiation, ETag/304, custom deployment URL, public schema route or new version alias. Canonical paths are documented; existing leading-slash/one-trailing-slash conveniences remain behavior tests rather than extra OpenAPI path entries.

## Exact operation inventory

Literal Account/Contact paths avoid implying arbitrary object support. Required operationIds:

| Path | Method / operationId |
| --- | --- |
| `/services/data` | GET `candidateVersions` |
| `/services/data/v65.0` | GET `candidateResources` |
| `/services/oauth2/userinfo` | GET `candidateUserInfo` |
| `/services/data/v51.0/sobjects/User/{userId}` | GET `candidateOwnUser51` |
| `/services/data/v65.0/sobjects/User/{userId}` | GET `candidateOwnUser65` |
| `/services/data/v65.0/sobjects` | GET `candidateDescribeGlobal` |
| `/services/data/v65.0/sobjects/Account/describe` | GET `candidateDescribeAccount` |
| `/services/data/v65.0/sobjects/Contact/describe` | GET `candidateDescribeContact` |
| `/services/data/v65.0/sobjects/Account` | POST `candidateCreateAccount` |
| `/services/data/v65.0/sobjects/Contact` | POST `candidateCreateContact` |
| `/services/data/v65.0/sobjects/Account/{recordId}` | GET `candidateReadAccount`, PATCH `candidateUpdateAccount`, DELETE `candidateDeleteAccount` |
| `/services/data/v65.0/sobjects/Contact/{recordId}` | GET `candidateReadContact`, PATCH `candidateUpdateContact`, DELETE `candidateDeleteContact` |
| `/services/data/v65.0/sobjects/Account/Compat_Key__c/{externalId}` | PATCH `candidateUpsertAccount` |
| `/services/data/v65.0/query` | GET `candidateQuery` |
| `/services/data/v65.0/openapi.yaml` | GET `candidateOpenAPI` |

Nineteen operations, no wildcard API version/object, `/healthz`, assets, Composite, Bulk, collections, query cursor, OAuth login/refresh or additional custom fields. The own-user paths are discovery for the authenticated user only, not general User CRUD. Bearer security is inherited by every operation; it is provisioned candidate authentication, not OAuth support.

## Schema and provenance requirements

Each operation has a nonempty `x-source` array of HTTPS URLs registered in `docs/compat/sources.yaml`. Append entries for exact repository revision URLs for current http/catalog/select/query-column sources, this Plan 23 design for the new serving behavior, and the newly consulted standards. Registry entries distinguish locally reviewed independent candidate sources from external specification vocabulary; a public locator is not a claim it was fetched successfully. Never replace old source extraction states or present candidate code as a Salesforce oracle. Offline validation checks URL registration without fetching.

Component schemas must describe concrete shapes from `catalog.go`, `http.go`, `select.go` and `query_columns.go`, including:

- Account create requires Name; Contact create requires LastName. PATCH permits omission and preserves explicit nulls for nillable fields; Id/unknown fields are not writable. Name/FirstName/LastName maxLength 255; Compat_Text__c/Compat_Key__c maxLength 80. AccountId is nullable String. Required name nonblank semantics and Go validation remain server checks, not a claimed complete JSON Schema business-rule encoding.
- Upsert accepts Name/Compat_Text__c and permits Compat_Key__c only as null, matching the existing body rule. The URL supplies the key. Preserve current 201-created/200-updated envelopes with id/created/success/errors. Create returns 201 with id/success/errors; PATCH/DELETE records return 204 with no body/content schema.
- Read shapes include `attributes`, object-specific selected fields and nullable values. Account Compat_Text__c is optional because restricted reads omit it. Ordinary Account/Contact reads require their remaining catalog fields. Attribute type is fixed to its object; attribute URL is optional when Id is not projected.
- Query requires q as a String and documents the existing equality/order/LIMIT 1..2000 subset in prose. Optional columns has wire enum `"true"|"false"`; repeated/malformed columns rejects. Unknown query options remain ignored. Query projections require attributes, allow only existing catalog fields/Contact.Account.Name, and do not require fields absent from a projection. Relationship Account may be null. done is true; totalSize is nonnegative Integer. columnMetadata is optional and has the exact columnName/one-level joinColumns shape; policy and projection order are separately asserted.
- Global/object describe uses the actual Object/Field structs, including booleans, field lengths and referenceTo arrays. Discovery, version/resource map, userinfo and own-user responses have required concrete properties, without 18-character assumptions for provisioned tenant/user labels.
- JSON errors are nonempty arrays of objects requiring errorCode/message and optional String[] fields. Document only current 400/401/404/405/500 outcomes where applicable, exact 200/201/204 success codes, and method-error Allow headers. No wildcard/default response that could hide an undocumented success/status. JSON object envelopes reject unexpected properties; record/projection schemas explicitly enumerate optional fields.

All refs are local and resolvable. No external fetch, legacy nullable keyword, discriminator-dependent validation, recursive schemas or formats needing new plugins. Use plain 2020-12 keywords plus annotations. No permissive `{}` schema may stand in for a record/envelope. The self-document response is application/yaml String data; validate its structure separately and compare served bytes exactly with the canonical file.

## Bounded test-time traffic validation

New Node modules `candidate-openapi-contract.ts` and `candidate-openapi-traffic.ts` load/compile the repository-controlled contract using existing validateOpenApi, yaml and Ajv2020. No runtime server middleware and no arbitrary uploaded spec. Compile failures for unsupported parameter serialization, media, refs or schema constructs fail tests; never silently skip an unhandled construct. Local component refs may be mapped into an Ajv $defs root; do not feed the entire OpenAPI object to a JSON Schema validator or downgrade the document to 3.0.

`createCandidateTrafficValidator(document: unknown): Promise<CandidateTrafficValidator>` first validates the full document. The returned `validate(exchange: CandidateExchange): CandidateTrafficResult` performs no I/O. Exchange contains method, relative path plus query, request/response contentType strings (or null), request body Uint8Array (or absent), response status/body Uint8Array. It contains no Authorization header. Result contains optional operationId plus separate requestIssues/responseIssues arrays of category codes only: route, method, parameters, request-media, request-json, request-schema, status, response-media, response-json, response-schema, response-body, budget. No raw exception, parameter/header/body/value or Ajv prose is exposed.

Scope is canonical path matching, scalar String parameters with form/simple serialization, declared JSON request schemas and exact status/media/response schemas. Literal paths take precedence over parameterized paths. Split raw segments before decoding template values exactly once; external IDs with %2F or %252F must not become extra route segments. Match own-user route shape only; the server enforces identity. Unknown query options remain ignored; validate the first q value as the handler does, and require any supplied columns to occur exactly once. Do not interpret SOQL, authenticate credentials or prove permission logic through schema validation.

Bounds: request bytes <=1,048,576, response bytes <=8,388,608; check before decode/copy. Decode UTF-8 fatally, parse JSON only for JSON media, preserve absent/empty/body-present distinctions. Normalize media type case/parameters. A 204 with any bytes fails response-body; malformed/missing content type, wrong schema, unknown status/route/method and unsupported spec features cannot be treated as a passing exchange. Valid JSON scalar types are not coerced and defaults are not inserted. Deterministic issue ordering follows the category list above, deduplicated; request/response failures remain separate so intentional invalid requests on a matched operation can still require a correct documented error response. An unmatched route/method returns no operationId and a route/method request issue; no response schema is selected, responseIssues stays empty and the exchange cannot count toward coverage. Unsupported-method/version errors are direct server assertions of status/Allow and the shared error shape, outside successful traffic validation; actual HEAD asserts status/Allow and an empty body instead. Authentication and policy are verified by real server assertions, not this helper.

## Candidate acceptance and completion

Add opt-in `test/candidate-openapi.test.ts` with parent identity `candidate/openapi/traffic`. Use the existing candidate environment/TLS trust, bounded requests, fixture journal and exact-ID cleanup; no bypass of certificate checks or new credentials/config system. The acceptance runner currently lists only candidate.test.ts, so explicitly include the new file in its Node invocation and add the new identity to candidateNames/required coverage. Update `candidate-acceptance-results.ts` to preserve exactly this new parent identity rather than prefixing it with the test filename; its current special case handles only candidate/jsforce/ names. Add a regression through reporter-event parsing and coverage assessment. Retain all three existing JSforce identities. The historical `jsforce` gate now includes this clearly named raw-HTTP supplement; do not call it another independent client implementation. Add database coverage for `TestHTTPServedOpenAPI`.

The live test fetches the served YAML, checks exact repository bytes, validates/compiles it and exercises a successful exchange for each of all nineteen operationIds using actual candidate fixtures. Collect observed operationIds and assert exact equality with the fixed inventory above; document-generated counts alone cannot hide a missing operation. Cover Account/Contact create/read/patch/delete, upsert create/update with one escaped-slash key, ordinary/empty/projected/relationship queries and columns true/false. Check both admin and restricted/other-tenant views: denied field write/filter/query is the current error status, denied field absent from reads, tenant B cannot read tenant A rows, and failures leave stored values unchanged. Negative cases include missing/invalid auth, malformed JSON, unknown field, wrong field type, invalid columns, missing record and unsupported contract methods/version. Request-schema failures are expected for intentional bad inputs; error-response validation must still pass. Preserve journaling before every potential create and known IDs before assertions; a validation failure cannot strand an untracked row. Cleanup Contact before Account, verify absence, and leave unknown dispatched outcomes visibly failed for existing recovery.

Offline tests pin document inventory/provenance/schemas and mutate synthetic traffic to prove the validator catches wrong required properties/types/status/media, missing/extra 204 bodies, null/projection distinctions, path ambiguity, encoded IDs, malformed JSON/UTF-8, bounds and secret-marker diagnostics. Extend required-coverage tests so a missing/skipped new identity or Go route test cannot pass acceptance.

Completion requires focused Go/Node checks, offline lab validation and one full `CANDIDATE_DISPOSABLE=yes scripts/check-candidate-acceptance.sh --out <new-private-summary>` on a clean committed revision with dedicated PostgreSQL 17 and existing prerequisites. This owns database/race-vet/client/restart/browser/build/cleanup checks; do not duplicate its full gates. Missing prerequisites produce a recorded incomplete/blocked acceptance result with owner/action, not a completed package based on skipped DB/live tests. Preserve private diagnostics and publish only safe counts/outcomes/digests. Verify base-to-final diff whitespace and intended-file boundaries; get a fresh whole-change review.

Update README/current candidate-guide wording to distinguish the served candidate contract from the unchanged broader proposal. Publish actual commands, counts, failures/skips, operation coverage, source ledger and limits in `docs/compat/plan-23-served-candidate-openapi.md`. No Salesforce reference request is required and no compatibility freeze, production certification, package/runtime feasibility upgrade or M2/M3 closure follows.

## Builder check

A fresh software-engineer reviewer found two consequential details: preserving the new acceptance result identity, and distinguishing unmatched operations/real HEAD errors from schema-validated matched traffic. Both are specified above and must have plan tasks/regressions. The remaining design was sufficient to plan implementation. Endpoint/version/test-helper choices remain executor recommendations for maintainer review, not collected customer requirements.
