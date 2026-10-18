# Plan 7 — Synthetic declaration model

Implemented an internal read-only Go model in `internal/platform/metadata.go`.
This is a representation foundation, not a new client-facing feature: existing
routes, describe responses, database catalog and permission enforcement are
unchanged. There is no new Metadata API endpoint.

`DeclaredCustomField` accepts exact full names; `DeclaredFieldGrant` accepts an
exact permission-set and field name. Both return value copies and a known flag.
A false known flag means not captured, never a denial or proof of nonexistence.
`OptionalBool` preserves unobserved versus explicit false properties.

| Full name | Type | Length | Required | External ID | Unique | Case sensitive |
| --- | --- | ---: | --- | --- | --- | --- |
| Account.Compat_Text__c | Text | 80 | false | false | false | unknown |
| Account.Compat_Key__c | Text | 80 | false | true | true | false |

`Compat_Admin` declares readable and editable true for both fields. No other
field or object grants, permission sets, assignments or effective access are
inferred. These declarations do not authorize any user.

## Evidence and limits

Values were checked against both private reviewed Plan 6 inventories, in both
Metadata read and CLI retrieval facts. The source operations are
`metadata:administrator:CustomField:Account.Compat_Text__c`,
`metadata:administrator:CustomField:Account.Compat_Key__c`,
`metadata:administrator:PermissionSet:Compat_Admin` and
`retrieve:administrator:synthetic`. Inventory digests and collection intervals
are recorded in [Plan 6 results](plan-6-metadata-discovery.md).

Both source inventories remain incomplete. This property-level implementation
does not promote a compatibility contract or claim effective security parity.
No reference requests or mutations were made for Plan 7.

The runtime catalog still has its previous lengths and role-based behavior.
Its persisted definition is checked at startup, so activating different values
requires a deliberate migration and integration plan. External-ID uniqueness,
case behavior, query permissions and errors require additional evidence and
Plan 5 dependency review before changing enforcement.

## Verification

Tests failed first because the new lookup interfaces did not exist. After
implementation, `go test ./...` and `go vet ./...` passed. PostgreSQL integration
tests were skipped because TEST_DATABASE_URL was not configured; the new tests
require no database. Tests cover values, unknown names, case variants, explicit
false, returned-value isolation, and the existing restricted-principal boundary.
Reading declarations leaves the serialized catalog and describe results intact.

Execution choices: use the existing Go package and native branch workflow; keep
declarations separate from the persisted catalog; use immutable value copies
rather than shared maps; preserve unknown data. These are bounded implementation
choices under the maintainer's request to implement the next plan.
