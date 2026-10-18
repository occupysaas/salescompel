# Plan 7 — Synthetic metadata declaration model

Implement the independent next slice identified by Plan 6: an internal, read-only
Go representation of observed synthetic field metadata and declared grants. The
maintainer requested implementation of the next plan and previously delegated
selection. This bounded scope is the executor's choice.

## Contract

Expose `DeclaredCustomField(fullName string) (CustomFieldDeclaration, bool)` and
`DeclaredFieldGrant(permissionSet, field string) (FieldGrantDeclaration, bool)`
in internal/platform/metadata.go. Exact full names are required. These are
internal functions, not new HTTP routes. Both return values, not mutable shared
maps or pointers. False means unknown/not captured, never denied or nonexistent.

CustomFieldDeclaration has FullName, Type, Length, Required, ExternalID, Unique,
and CaseSensitive. Type is Metadata API `Text`, not describe `string`.
CaseSensitive uses an explicit optional boolean value struct with Known and Value
fields, so missing is distinct from false. The other properties were observed
for both fixtures. Compat_Text__c: Text, 80, required false, externalId false,
unique false, caseSensitive unknown. Compat_Key__c: Text, 80, required false,
externalId true, unique true, caseSensitive known false. Full names include Account.

FieldGrantDeclaration contains PermissionSet, Field, Readable, Editable. Only
Compat_Admin declarations for the two custom fields are captured, both readable
and editable true. Other permission sets, standard fields and object grants are
unknown. This model contains no principal lookup, assignment or effective-access
calculation. Its existence must never grant a principal access.

## Boundaries and evidence

Use Plan 6's two matching incomplete inventories as property-level evidence;
verify these exact synthetic values privately before implementing. Do not promote
the inventories to accepted compatibility evidence or claim full security parity.
No live requests are necessary. No raw captures, tokens or identity IDs in git.

No changes to existing Field, catalog serialization, database schema, startup
version checks, Describe, Validate, query/write authorization, SQL uniqueness,
error behavior or Salesforce HTTP routes. Existing runtime behavior stays intact.
These boundaries avoid introducing a catalog migration or unsupported effective
permission semantics while Plan 5 remains gated. A later plan must define migration
and route integration and resolve enforcement evidence before activating the model.

## Acceptance

Table-driven tests cover all observed properties, explicit false versus unknown,
unknown names/types/case variants, and returned-value mutation isolation. A
regression ensures reading declarations does not affect Describe or Validate for
the existing restricted principal. The existing catalog stays byte-equivalent
in serialization. Run go test ./... and go vet ./...; disclose database integration
skips if TEST_DATABASE_URL is unavailable. No new dependencies.

Rejected alternatives: changing live catalog lengths now entails migration and
write semantics; adding a Metadata SOAP route entails a substantially wider API
contract. This plan deliberately builds only the reusable declaration model.
