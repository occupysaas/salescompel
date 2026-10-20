# Exploratory UI evidence matrix

Plan 13; Account/Contact prototype. Every preview interaction is local simulation, including interactions whose corresponding candidate route exists. API version 65.0. Candidate route existence was checked in `internal/platform/http.go`, with query limitations recorded in Plan 2. This is a UI design artifact, not fresh reference collection.

| Screen / action | API contract or candidate route | Observed behavior | Candidate implementation | Prototype behavior / limits |
| --- | --- | --- | --- | --- |
| Account/Contact lists | GET `/services/data/v65.0/query` | Plan 4 pilot query scenario only; Plans 5/10 foundation contract remains blocked | Explicit field list, one equality predicate, one ordering field, LIMIT 1..2000; no cursor | Fixed synthetic lists; no search/filter/pagination claims |
| Form field selection | GET `/services/data/v65.0/sobjects/{object}/describe` | Plan 4 pilot describe; Plan 6 incomplete inventories; Plans 7/8 bounded metadata evidence | Fixed Account/Contact catalog; Plan 8 custom key length 80 | Name/key or FirstName/LastName/AccountId only; no dynamic-schema/permission parity |
| Record detail | GET `/services/data/v65.0/sobjects/{object}/{id}` | Plan 4 pilot CRUD scenario | Account/Contact read implemented | Synthetic-ID lookup; proposed missing-record presentation |
| New record | POST `/services/data/v65.0/sobjects/{object}` | Plan 4 pilot CRUD scenario | Account/Contact create implemented | Local Pinia insertion; proposed required-field validation; no request |
| Edit record | PATCH `/services/data/v65.0/sobjects/{object}/{id}` | Plan 4 pilot CRUD scenario; not all field semantics measured | Candidate updates implemented | Local replacement of draft fields; Cancel discards; no request |
| Contact → Account | GET query with Contact.Account.Name, or independent Account read | Plan 4 pilot relationship scope, not every UI composition | Only Contact.Account.Name relationship projection; AccountId equality also supported | Local account picker/link; no lookup-search API claim |
| Account related Contacts | Possible Contact equality query by AccountId; no dedicated reverse endpoint claimed | No direct observation claimed for this UI composition | Candidate equality query can express the list; no reverse traversal implementation claimed | Locally computed sample contacts, explicitly proposed UI |
| Navigation, theme, language, reset, validation presentation | No Salesforce route required | No Salesforce UI observation claimed | Product UI | Proposed accessible navigation and state changes, locally simulated |

The candidate's external-ID upsert route is implemented, but has no editor in this slice. Plan 9 equality/race drift remains unresolved; Plan 11 body validation is complete within its measured bounds. Neither changes this prototype's local-only nature. Proposed routes in `api/openapi.yaml` are not automatically implemented.

Sources: [Plan 2](../plan-2-implementation.md), [Plan 4](../plan-4-reference-baseline.md), [Plan 5](../plan-5-contract-decisions.md), [Plan 6](../plan-6-metadata-discovery.md), [Plan 8](../plan-8-active-field-metadata.md), [Plan 9](../plan-9-external-id-contract.md), [Plan 10](../plan-10-foundation-decision.md), [Plan 11](../plan-11-external-id-body-validation.md).

User-facing summaries live in `web/src/preview/evidence.ts` and use translation keys. General UI strings live in the existing language catalogs. Non-English new keys intentionally fall back to English; their catalogs remain partial drafts. No new live reference calls or private-artifact reads were performed.
