# Plan 6 — Metadata and permissions discovery

Date: 2026-10-17. Status: planning deliverable; the maintainer delegated the choice
of next work and opted out of further brainstorming questions. This scope is the
assistant's choice, not a claim that the maintainer selected its individual details.

## Intent

Prepare the next metadata/security implementation slice while Plan 5's query
foundation evidence remains incomplete. Establish what the dedicated reference
personas can discover, what the existing synthetic metadata contains, and where
the fixed candidate catalog cannot represent it. Deliver a reproducible read-only
inventory, a capability matrix and a bounded follow-on recommendation.

Use the existing Node/TypeScript compatibility lab and pinned JSforce/Salesforce
CLI clients. The future product remains Go/PostgreSQL. This plan implements lab
instrumentation only; it does not implement dynamic schema or permission grants.

## Scope

Default object scope is Account and Contact, with the existing field inventory:
Account.Id, Account.Name, Account.Compat_Text__c, Account.Compat_Key__c;
Contact.Id, Contact.FirstName, Contact.LastName, Contact.AccountId.
Read object capabilities and these fields' type, length, nullability, external-ID,
reference targets, createable and updateable properties under the administrator
and restricted persona. Record missing properties explicitly as unknown.

Read existing CustomField metadata for Account.Compat_Text__c and
Account.Compat_Key__c and the synthetic Compat_Admin PermissionSet through public
Metadata API read operations. Optional Profile and additional PermissionSet reads
require exact fullNames in private configuration; do not enumerate unrelated
profiles, users or assignments. Record whether the optional scope was supplied.
An omitted scope is a visible gap, not a successful inventory of all permissions.

Retrieve exactly the two synthetic CustomFields and Compat_Admin PermissionSet
using the pinned, unmodified CLI in an isolated temporary project/auth home.
This checks retrieval shape and cross-client agreement for those components;
it does not run deploy, change field-level security or assign permission sets.

Compare declared component settings with persona-visible describe results, but
never compute effective authorization by treating a retrieved profile or permission
set as exhaustive. Assignment state, groups, muting, sharing and inherited grants
are outside this slice. Describe omission alone does not prove a denied record
operation. Existing pilot enforcement evidence remains separately attributed.

## Constraints

- Preserve Salesforce HTTP paths, methods, versions and native client commands;
  no custom routes, rewriting proxies or client patches.
- Pin discovery to API 65.0 and the existing locked clients; no dependency upgrades.
- Follow CLEAN_ROOM.md: public interfaces only; never inspect CLI/plugin source.
- No reference writes: no deploy, create/update/delete, assignments, settings changes,
  record extraction or queries. Metadata transport may legitimately use HTTP POST;
  enforce read-only operations at the API/CLI operation level.
- Existing named-tenant authorization applies. Verify both persona identities and
  same organization before collection; no fresh same-admin session is required.
- Keep tokens, raw metadata, component fullNames beyond synthetic fixtures, and
  user/org identifiers outside the checkout in owner-only files/directories.
- Preserve pilot and foundation profile bytes, accepted observations and report
  semantics. This inventory is not a new passing compatibility suite.
- Absence, explicit false, unavailable, unsupported, unrequested and conflicting
  evidence must remain distinguishable. Never turn failed reads into empty success.
- Keep every product file under internal/platform and all product routes unchanged.

## Deliverables and boundaries

A new `discover-metadata` lab command writes a private versioned inventory. Its
input is a strict private configuration with the target origin, API version,
existing profile hash and allowlisted component
names; fixed administrator/restricted aliases provide persona attribution.
Credentials come from environment variables, never configuration or args.
Use reference administrator/restricted tokens already available. No new login is
necessary if those tokens remain valid.

The inventory contains scope, timestamps, profile/toolchain binding, per-operation
outcomes, persona-visible schema, declared metadata properties, CLI retrieval
manifest/digests and explicit gaps. Raw payloads are private companion artifacts.
Component names are data, never shell fragments or filesystem paths. Output paths
must be external to the checkout; generated files are create-exclusive.

A separate deterministic compiler turns inventory into capability-matrix rows:
`area`, `componentAlias`, `persona`, `property`, `state`, `value`, `sourceRefs` and
`nextAction`. States are observed, unavailable, unsupported, unrequested or
conflict. An explicit false is an observed value, not an unavailable row. Failure or missing properties in required operations keep the result incomplete
even when a useful matrix can be built. Optional unrequested components remain
visible scope gaps but do not prevent completion of the bounded default inventory.
Conflicts require matching fact scope: declared grants and persona visibility are
different facts and their difference alone is not a contradiction.

Two sequential collections must agree on normalized supported facts, or explain
and preserve drift. Compare component sets by canonical identity, not enumeration
order; property values and missing/false distinctions remain exact. Authentication
failure, scope mismatch or toolchain drift blocks a comparison. This is inventory
stability, not compatibility parity or completion of M4.

Publish only a reviewed summary using synthetic names and persona aliases, report
digests and gaps. Never automatically copy raw labels, descriptions, XML, usernames
or org identifiers into repository documentation. Recommend a subsequent bounded
implementation slice: represent the existing text/external-ID field properties and
explicit object/field grants before considering arbitrary field types or sharing.
That recommendation remains conditional on actual inventory findings.

## Dependencies

Plan 6 collection, matrix compilation and gap analysis can execute now. They do
not query records, use cursors, depend on ID normalization or edit the Plan 5 suite.
Do not run simultaneous CLI auth/export operations against shared aliases; isolated
CLI homes and captured in-memory credentials prevent overwriting Plan 5 sessions.

Product implementation is a separate future plan. Its catalog and permission
checks must integrate with Plan 5 canonical object/field resolution and preserve
tenant isolation. Query authorization, pagination access checks and wire-error
mapping wait for the corresponding Plan 5 contracts; this plan cannot waive them.

## Acceptance

Acceptance means bounded inventory tooling works, fails closed, preserves unknowns,
produces two explainable collections, leaves the reference configuration untouched,
and yields a reviewed recommendation. Unavailable required metadata makes discovery
incomplete, with an owner and concrete next action. No broad security, deployment,
metadata round-trip or Salesforce parity certification is implied.

Excluded: dynamic schema, permission mutations, grant computation across groups,
record sharing, custom objects/field-type expansion, layouts/record types, Tooling,
Apex, source tracking, packages/scratch orgs, migrations, OAuth lifecycle and new UI.
