# Plan 8 — Active custom-field lengths

Account.Compat_Text__c and Account.Compat_Key__c now report length 80 through
GET /services/data/v65.0/sobjects/Account/describe. The active catalog takes these
lengths from Plan 7's declarations; candidate write validation and PostgreSQL
column bounds agree. All existing routes and methods are preserved.

Type remains string, both fields remain nullable, and externalId remains false
for Compat_Text__c and true for Compat_Key__c. No new uniqueness/caseSensitive
properties or effective permission rules were introduced. Restricted visibility
remains the existing candidate behavior, not a reference parity assertion.

## Upgrade procedure

1. Stop all old application instances. Mixed-version rolling upgrades are unsupported.
2. Back up the target database and verify your restore procedure before upgrading.
3. Start the new binary with the intended DATABASE_URL. Startup serializes with
   advisory lock 739102, recognizes the exact legacy v1 catalog, checks existing
   data under table locks, and commits column bounds and catalog version 2 together.
4. Verify both custom fields report length 80 on the existing authenticated describe
   route, then resume service. Subsequent starts verify catalog and physical bounds.

Existing oversized values in either field, in any tenant, stop the upgrade with
`oversized custom-field values require operator remediation before catalog upgrade`.
The transaction rolls back; records, column bounds and the version 1 catalog remain
unchanged. Nothing is truncated, deleted or rewritten. The error exposes no record
values or tenant identifiers. Decide how to remediate the data explicitly; the old
binary remains usable on the unchanged v1 database after a refused upgrade.

Unknown versions, mismatched definitions, populated databases missing the catalog,
and version 2 physical-bound mismatches also fail closed. Fresh empty databases
initialize directly to version 2. The bootstrap SQL retains legacy column bounds;
the transactional migration establishes 80 before Open succeeds.

After successful upgrade, the old binary rejects version 2. There is no automatic
downgrade. To roll back, stop the service and restore the verified pre-upgrade
backup using the old binary; account for any writes since the backup. No production
database was upgraded during this implementation.

## Evidence and limits

Both reviewed Plan 6 administrator describe responses independently reported
length 80 for these fields, agreeing with the Metadata read/retrieve declarations.
See [source intervals and digests](plan-6-metadata-discovery.md). Source inventories
remain incomplete; no profile or accepted compatibility observation was promoted.

Validation continues to count Unicode code points, using the existing
STRING_TOO_LONG response for invalid/oversized values. This is candidate behavior,
not a claim about Salesforce UTF-16 counting or exact error parity. Null/empty
semantics and unrelated fields remain unchanged. Plan 5 cursor, canonicalization,
permission and error evidence remain separate; no M4 completion claim is made.

## Verification

- Go tests and vet passed with disposable PostgreSQL 17. Migration tests executed,
  including fresh v2, populated v1 upgrade, both oversized-column rollback cases,
  unknown/missing catalog, physical drift, reopen and concurrent startup.
- HTTP tests cover both fields at 80/81, unchanged rejected-update records, and
  exact existing describe output. Unicode boundary tests cover ASCII, BMP and
  supplementary code points.
- The new native JSforce assertion failed against the e9709d5 binary with
  `255 !== 80`, then passed against the updated candidate. The old binary was
  built in a separate temporary checkout; HEAD was never moved.
- Node check: 88 passed, one optional candidate skip, zero failed. The separate
  live candidate gate ran without that skip, exercising JSforce before/after a
  process restart, Python, Salesforce CLI and four passing browser tests.
- Candidate checks used an isolated local PostgreSQL container and synthetic data.
  No Salesforce reference requests, deployment, or profile changes occurred.

Execution used the approved native branch workflow. Catalog construction uses a
narrow internal lookup seam to test invalid declarations. Schema-isolated migration
tests keep unrelated databases untouched. No substantive plan deviations were needed.

## Final review and disposition

Fresh read-only review of `57184e5..97c6236` found no Critical or Important issues.
One Minor test improvement is deferred: add simultaneous startup against a wholly
empty schema; current concurrency coverage exercises populated v1 upgrade. The
review confirmed the advisory lock precedes bootstrap DDL, so no defect was observed.
The race-enabled Go suite also passed with PostgreSQL, and the actual old binary
refused the upgraded database with `database catalog version mismatch`.

Review exclusions were ruled on explicitly:

- Salesforce Unicode/error parity, effective permissions and uniqueness/case
  behavior remain outside this length-only slice. Existing candidate semantics
  are preserved; the cost of extending the claim would be unsupported parity.
- Mixed-version rolling operation and automatic downgrade remain unsupported.
  Stop old instances and use verified backup restoration; ignoring this boundary
  risks rejected operations or loss of post-backup writes during rollback.
- General schema reconciliation remains outside this bounded migration. Only
  the two custom-column bounds are verified at v2; other schema drift is not
  automatically repaired and requires separate operator diagnosis.

No production changes or push were performed. The approved native local workflow
was retained, and the implementation's disposable PostgreSQL container was removed
at completion. The deferred Minor does not block this reviewed release slice.
