# Proposal: plan-34-sharing

## Why

After Plan 33 a user's profile and permission sets decide which objects and
fields they can use, but every user still sees every record of a readable
object in their tenant, and records have no owner. Sales teams need records
private to their owners and managers. The Plan 34 spec and plan
(`docs/superpowers/specs/2026-11-17-plan-34-sharing-design.md`,
`docs/superpowers/plans/2026-11-17-plan-34-sharing.md`) define Salesforce's
record-level sharing model for the candidate. This change package locks it as
requirements.

## What Changes

- `OwnerId` on Account, Contact and every custom object, with the `Owner`
  relationship, a create default and ownership transfer.
- Org-wide defaults per object: Private, Public Read Only, Public Read/Write,
  Controlled by Parent.
- `UserRole`, `User.UserRoleId`, `Group` and `GroupMember`, with a maintained
  group-membership closure.
- Manual shares through `AccountShare`, `ContactShare` and `<Object>__Share`.
- Owner-based sharing rules and OWDs set by operator commands until Plans 35
  and 36.
- Account/Contact implicit sharing and master-detail inheritance.
- One access predicate enforced in SOQL (root, child, semi-join, parent join,
  aggregates, locator pages), REST, Composite and Collections.
- A forward-only migration that leaves every existing user's access unchanged.
- OpenAPI additions and a `candidate/sharing/matrix` acceptance case.

## Capabilities

### New Capabilities

- `sharing`: record ownership and record-level access.

### Modified Capabilities

- `soql-engine`: every compiled query carries the sharing predicate.
- Records write path: update, delete and lookup targets check record access.

## Impact

- New migration (number taken at merge), new `sc_sharing` schema,
  `sc_setup.users.user_role_id`, `owner_id` on every record table.
- `internal/policy`, `internal/soql`, `internal/records`, `internal/metadata`,
  `cmd/salescompel`, `api/candidate-openapi.yaml`, the Node acceptance suite.
- Plan 30 no longer adds `OwnerId` (pending maintainer confirmation on #24).
