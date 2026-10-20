# Plan 12 external-ID equality feasibility decision

Status: **offline experiment complete; hypothesis cannot be promoted to runtime**
(2026-10-20). The two known whitespace disagreements are useful results, not
failed experiment tests. This decision does not fix candidate parity or approve
a migration.

## Hypothesis and evidence boundary

`nfc-full-fold-v1` applies NFC, locale-independent full Unicode case folding
through `cases.Fold`, then NFC again. The implementation uses Go **1.25.0** and
the existing **golang.org/x/text v0.29.0** dependency, now classified as direct.
It validates UTF-8 and creates folding state per call. It performs no trimming,
accent removal, compatibility normalization, locale tailoring or URL decoding.

The following finite vectors use synthetic prefixes. “Observed” summarizes the
existing [Plan 9 contract](plan-9-external-id-contract.md), not new reference
collection. Unicode escapes identify exact code points; spaces inside quoted
strings are significant. The expected column is the hypothesis prediction;
computed results are checked by `TestCanonicalKeyReferenceVectors`.

| Inputs | Basis | Observed relation | Expected hypothesis | Computed relation | Mismatch |
| --- | --- | --- | --- | --- | --- |
| `p-key`, `p-key` | Plan 9 exact | equal | equal | equal | no |
| `p-AbC`, `p-abc` | Plan 9 ASCII | equal | equal | equal | no |
| `p-é`, `p-e\u0301` | Plan 9 accent | equal | equal | equal | no |
| `p-I`, `p-i` | Plan 9 ASCII I | equal | equal | equal | no |
| `p-I`, `p-ı` | Plan 9 dotless I | distinct | distinct | distinct | no |
| `p-ß`, `p-ss` | Plan 9 sharp-s | equal | equal | equal | no |
| `p-/percent%é`, same input | Plan 9 decoded key | equal, literal retained | equal, literal retained | equal, literal retained | no |
| `"p-key"`, `" p-key"` | Plan 9 leading-space lookup | equal | distinct | distinct | **yes** |
| `"p-key"`, `"p-key "` | Plan 9 trailing-space lookup | equal | distinct | distinct | **yes** |

The experiment agrees with seven rows and disagrees with exactly the two space
rows. Lookup-only whitespace matching does not establish creation normalization
or uniqueness behavior, so trimming must not be added silently. Passing the
other finite vectors proves no broader Unicode or Salesforce collation parity.
Plan 9's whole-contract comparison remains **drift**: variant race winners stored
different spellings across runs, even though deterministic cases matched.

## Candidate robustness and synthetic collisions

These are **unobserved candidate properties**, not additional reference results:
empty strings map to empty; tabs and NBSP remain; `e` and `é` remain distinct;
`p-/percent%é` remains literal; `p-%2F` becomes `p-%2f`, distinct from `p-/`.
Invalid UTF-8 returns an error and an empty key. Valid vectors are idempotent.
Thirty-two workers each repeat the nine-row table 100 times without disagreement;
the race detector reports no races. Null is outside the string API and is
represented separately by `Record.Key == nil` in the diagnostic.

`FindCollisions` is an in-memory diagnostic for synthetic records. Tests confirm:

- Tenant `a`, IDs `2`/`1`, keys `p-AbC`/`p-abc` produce canonical `p-abc`
  with sorted IDs `["1", "2"]`.
- Composed/decomposed accent keys collide at `p-é`; sharp-s/`ss` collide at
  `p-ss`. Equivalent keys in tenant `b` form a separate group. `p-I` and
  `p-ı` do not collide.
- Two empty keys collide; null keys are skipped. Reusing an ID across tenants
  is valid. Empty tenant/ID, duplicate same-tenant IDs (including null keys)
  and invalid UTF-8 keys return an error with no partial result.
- Structural tenant/key and tenant/ID tuples withstand embedded `\x1f`
  delimiters. All 24 permutations of a four-record fixture produce the same
  report, sorted by tenant, canonical bytes and IDs.
- Deep snapshots, key pointer checks and result mutation confirm that input
  records, pointed-to values and ordering are preserved; result IDs are owned
  by the result.

No record is rewritten, merged, deleted or selected as a winner. The package has
no product importers, database access, startup hooks, CLI or export adapter.
Product code, schema/catalog v2, accepted profiles, historical captures, routes,
body validation, query equality and UI are unchanged. No live reference calls,
credentials or private artifact reads were needed.

## Verification

All commands used `GOTOOLCHAIN=go1.25.0`; the host default toolchain differs.
Executed on 2026-10-20:

| Command | Outcome |
| --- | --- |
| `go test ./internal/compat/externalid -run TestCanonicalKey -count=1` | PASS after expected missing-symbol RED; both reference disagreements asserted |
| `go test -race ./internal/compat/externalid -run TestCanonicalKey -count=1` | PASS, no race diagnostics |
| `go test ./internal/compat/externalid -run TestFindCollisions -count=1` | Expected missing-types/function RED before implementation |
| `go test ./internal/compat/externalid -count=1` | PASS |
| `go test -race ./internal/compat/externalid -count=1` | PASS, no race diagnostics |
| `go test ./... -count=1 -v` | PASS for executed tests; database skips below |
| `go vet ./...` | PASS |
| `git diff --check` | PASS |

`TEST_DATABASE_URL` was unset. Database verification is **not claimed**. Skipped
cases were `TestCatalogFreshV2`, `TestCatalogUpgradeV1`, both
`TestCatalogUpgradeOversizedRollback` subtests, all three `TestCatalogRejectsUnknown`
subtests, `TestCatalogV2PhysicalMismatch`, `TestCatalogConcurrentOpen`,
`TestHTTPBoundary`, `TestHTTPExternalIDPreservesEscapedSlashes`,
`TestHTTPRejectsUndeliverablePathsAndListings`, `TestHTTPCustomFieldLengths`,
`TestHTTPUpsertBodyValidation`, `TestHTTPUpsertBodyBoundaryControls`,
`TestHTTPOrdinaryExternalIDWrites`, `TestStoreIsolationAndPersistence`,
`TestStoreQueryPolicyAndRelationships`, `TestStoreConcurrentUpdatesAndUpsert`,
`TestStoreBlankNillableStringsAreNull` and `TestStoreUpsertBodyValidation`.
No Node capture commands or reference sessions ran.

## Subsequent product gate

The [Plan 12 design](../superpowers/specs/2026-10-20-plan-12-external-id-equality-design.md)
is a bounded prerequisite. A separately reviewed product design must resolve
lookup versus creation normalization, duplicate envelopes, preservation of stored
spelling, tenant-scoped database uniqueness and shared locking across every write
path. A transactional migration must refuse ambiguous collisions without choosing
winners; rollback and concurrent deployment behavior also require design.

Missing-key races remain unresolved. The current serialized candidate can update
after waiting, while Plan 9 measured a create and `DUPLICATE_VALUE`. Those finite
rounds establish neither a deterministic race guarantee nor a loser policy.
This experiment selects neither.

[Plan 10 remains blocked for live foundation evidence](plan-10-foundation-decision.md).
[Plan 11's completed body-field validation](plan-11-external-id-body-validation.md)
is independent of this equality decision. Further reference collection requires
its own reviewed scope and existing identity/recovery gates. This experiment
provides no universal compatibility, migration-readiness or Plan 10 completion
claim.

## Fresh whole-change review

An independent reviewer inspected `bf59002..f731265`, the plan/spec, tests and
this decision report before its documentation commit. Review found **no Critical,
Important or Minor issues** and required no corrections. It independently ran
package race tests, the verbose full Go suite and vet with Go 1.25.0; all passed
with database skips matching those above. Review checked UTF-8 rejection,
idempotence, concurrent folding, tuple isolation, null/empty separation, report
ordering, input preservation, mismatch reporting and absence of product wiring.
It did not collect fresh reference evidence.

The implementer retained three review exclusions as explicit scope decisions:

- Universal Salesforce/Unicode equality remains unestablished because finite
  fixtures cannot prove it. Treating these fixtures as universal would risk
  unsupported equality and collisions.
- Runtime normalization, uniqueness/migration and missing-key race outcomes stay
  deferred to a separate design. Promoting this diagnostic prematurely could
  change write behavior or merge ambiguous keys.
- Database integration remains unverified without `TEST_DATABASE_URL`. Any
  unrelated database regressions would therefore remain undetected by this run.

These limits do not prevent completion of the bounded offline experiment.
