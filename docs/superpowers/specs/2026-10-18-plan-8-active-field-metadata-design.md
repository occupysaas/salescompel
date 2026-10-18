# Plan 8 — Activate observed custom-field lengths

## Intent and approved direction

The maintainer approved prioritizing accurate describe responses on the existing
Salesforce route after brainstorming Plan 8. Existing native sequential execution
and identical routes for drop-in clients remain requirements. This design chooses
the smallest supported activation: both synthetic Account fields have length 80,
consistently across describe, candidate write validation and PostgreSQL storage.
No implementation is performed by this design document.

## Evidence and scope

Both private Plan 6 reviewed inventories contain administrator describe facts for
Account.Compat_Text__c and Account.Compat_Key__c: type string, length 80, nillable
true, referenceTo [], createable/updateable true, externalId false/true respectively.
These agree with Metadata read/retrieve declarations used by Plan 7. Use the time
and digest bindings in docs/compat/plan-6-metadata-discovery.md; source operation
is describe:administrator:Account. No raw identity/credential data enters git.

Only length changes in the existing Field JSON. Metadata API Text is not emitted
as describe type; no new unique or caseSensitive properties are added. Declared
grants are not assignments. Role filtering, object capabilities, standard fields,
Contact and API error shapes remain unchanged. Restricted behavior remains the
existing candidate behavior and is not claimed to match the reference.

## Active catalog and writes

Use DeclaredCustomField for the two existing custom field lengths when constructing
the current catalog; leave the independent declaration model intact. Missing or
invalid required declaration data is an initialization error, never a silent
fallback to 255. Keep existing type, nillable and externalId mapping explicit.
The existing Validate length check consequently enforces 80 Unicode code points
for both fields on create/update, including external-ID paths that reach it.
This is the candidate's existing counting/error policy, not a new claim about
Salesforce UTF-16 limits or exact overflow errors. Test ASCII, BMP and supplementary
Unicode boundaries as local consistency tests. Null and empty-string behavior
remain unchanged. Unrelated updates do not change other fields.

Expose via GET /services/data/v65.0/sobjects/Account/describe with unchanged auth,
status and JSON wrapper. No custom route, proxy, client patch or Metadata SOAP
endpoint. Successful existing routes and supported methods retain their shape.

## Versioned database transition

Move startup catalog checking into a focused migration module, keeping Open's
transaction and advisory lock 739102. Version 1 is the exact legacy catalog
serialized by e9709d5 (two lengths 255); keep a frozen fixture, not a dynamically
reconstructed copy of the new catalog. Version 2 is the new active catalog.

Classify under the lock: empty fresh database initializes version 2; recognized
version 1 definition upgrades; matching version 2 reopens idempotently; unknown
versions or mismatched definitions fail without modification. An absent catalog
row is fresh only if Account/Contact contain no rows; existing data without a
catalog fails closed. Do not treat arbitrary version 1 definitions as migratable.
JSONB structural equality is used, not textual key ordering.

For fresh creation and upgrade, both compat_text and compat_key end as varchar(80).
Before reducing lengths, hold an ACCESS EXCLUSIVE accounts table lock and check
both columns across every tenant with char_length > 80. If any row violates the
bound, abort the entire transaction with a stable actionable error saying that
oversized custom-field values require operator remediation. No automatic truncation,
deletion or rewriting, and no tenant IDs, record IDs or values in the error.
Use ALTER COLUMN TYPE varchar(80) without an explicit truncating cast/USING.
Preserve all rows, IDs, nulls, relationships, indexes and uniqueness constraints.
Commit schema and catalog version/definition atomically. Rollback on every failure.

Matching version 2 must also verify the two physical column bounds are 80; a
mismatched physical schema fails rather than silently repairing an arbitrary
schema. This is bounded validation, not a general schema reconciliation engine.
Concurrent new starts serialize and succeed once; a failed upgrade releases locks
and leaves version 1 and its column bounds intact.

Operator workflow: stop old application instances, back up the database, run the
new binary startup against the intended database, and restart service after success.
Rolling mixed-version operation is not supported. If oversized values block the
upgrade, the old binary can still open the unchanged version 1 database; remediation
is an explicit operator decision. After a successful upgrade, the old binary must
reject version 2. No automatic downgrade; rollback requires a tested backup/restore
procedure. Do not run a production upgrade during implementation.

## Verification and acceptance

Use disposable PostgreSQL databases/schemas isolated per migration test. Tests
must exercise fresh startup, populated recognized v1 upgrade, null and length-80
preservation, both oversized columns (including a different tenant), unchanged
bytes/catalog/schema after refusal, unknown catalog versions/definitions, missing
catalog with existing rows, v2 physical mismatch, reopen and concurrent starts.
Run real PostgreSQL tests; a skipped database suite does not satisfy this plan.
Existing integration tests must still pass with their cleanup guarantees.

Use unmodified pinned JSforce to describe Account through the existing candidate
HTTPS route and assert both lengths 80 and unchanged supported attributes; include
a restart check. This is candidate testing, not a new reference capture. Keep
pilot/foundation profile bytes and accepted observations unchanged. Run Go tests,
vet, Node checks and the existing candidate gate in a disposable local environment.
Document exact checks and limitations. No full parity, M4 or Plan 5 completion claim.

## Executor choices and dependencies

Executor choices: automatic transactional startup migration for the one known v1
schema; fail closed on oversized data; no destructive remediation; mandatory local
PostgreSQL verification. Reuse current dependencies and deployment conventions.
This length-only slice is independent of pending Plan 5 cursor semantics; broader
canonicalization, permission checks, ID handling and error parity remain gated.

## Design check

A fresh read-only builder check found no important unanswered questions. Catalog
initialization, migration organization and harness details are executor choices;
the companion implementation plan pins their interfaces and acceptance checks.
