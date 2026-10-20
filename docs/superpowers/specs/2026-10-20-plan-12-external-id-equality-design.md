# Plan 12 — External-ID equality feasibility

Status: proposed for review; no runtime change authorized by this document.
Milestone: bounded M3 design prerequisite following completed Plan 11.

## Intent and basis

Resolve the equality decision left open by [Plan 9](../../compat/plan-9-external-id-contract.md) before changing persisted keys. Plan 11 closed body-field validation; Plan 10 foundation evidence remains separately blocked. The executor selects this bounded preparatory slice because lookup, uniqueness and concurrency must eventually share one equivalence relation, and existing exact-distinct keys can collide after normalization.

Current `Store.Upsert` in `internal/platform/store.go` locks and looks up the raw tenant/key and injects the path spelling into updates. Plan 9 instead observed ASCII case-insensitive lookup, composed/decomposed accent equality, sharp-s equality, distinct dotless-I, and preservation of the original spelling. These facts do not identify a universal Salesforce collation or justify changing query-text equality.

Deliver an offline Go experiment and deterministic collision analysis, plus a reviewed decision stating exactly what a subsequent product implementation may use. This plan does not claim to fix candidate parity.

## Proposed experiment

Create a separate `internal/compat/externalid` package, unreferenced by product runtime. Use Go 1.25.0 and the already pinned `golang.org/x/text v0.29.0`; add no dependency or version update. Promote x/text to a direct go.mod requirement if imported directly.

Evaluate one explicitly named hypothesis, `nfc-full-fold-v1`: NFC normalization, locale-independent full Unicode case folding through `cases.Fold`, then NFC normalization again. No trimming, accent removal, compatibility normalization or locale-specific tailoring. Expose `CanonicalKey(value string) (string, error)`; invalid UTF-8 is an error. Empty and whitespace strings are valid experiment inputs, not a decision about product validation. Instantiate folding state within the call so concurrent callers do not share mutable transformer state.

Pin the following comparison vectors using synthetic `p-` prefixes:

| Inputs | Expected relation | Basis |
| --- | --- | --- |
| `p-key`, `p-key` | equal | Plan 9 exact |
| `p-AbC`, `p-abc` | equal | Plan 9 ASCII |
| `p-é`, `p-e\u0301` | equal | Plan 9 accent |
| `p-I`, `p-i` | equal | Plan 9 ASCII I |
| `p-I`, `p-ı` | distinct | Plan 9 dotless I |
| `p-ß`, `p-ss` | equal | Plan 9 sharp-s |
| `p-/percent%é`, same input | equal, literal characters retained | Plan 9 decoded key |
| `p-key`, ` p-key` | distinct under hypothesis | Plan 9 lookup matched; hypothesis mismatch |
| `p-key`, `p-key ` | distinct under hypothesis | Plan 9 lookup matched; hypothesis mismatch |

The whitespace mismatch is an intentional decision finding, not an excuse to mark all vectors compatible or add trimming silently. Lookup-only trimming need not imply that stored/create uniqueness uses trimming. Historical Plan 9 observations remain unchanged. Record observed expectations separately from the hypothesis result; a test passes when the experiment reports the mismatch accurately.

Additional robustness vectors cover idempotence, concurrent repeatability, empty strings, tabs, NBSP, accent distinctions (`e` versus `é`), literal percent escapes and invalid UTF-8. Label these candidate properties, not reference observations. Never URL-decode inside this package.

## Collision analysis

Expose `FindCollisions(records []Record) ([]Collision, error)` with `Record{Tenant string, ID string, Key *string}` and `Collision{Tenant string, Canonical string, IDs []string}`. Group non-null keys by exact tenant plus canonical key. Report groups with at least two distinct record IDs; preserve all input values and order. Null is ignored; empty strings participate. Reject empty tenant/ID, duplicate tenant/ID entries and invalid UTF-8 keys with an error and no partial result. The same ID in different tenants is permitted. Sort groups by tenant then canonical bytes and IDs lexicographically so input permutations produce identical output. Use structural tuple keys, never delimiter concatenation.

This is an in-memory, pure diagnostic for synthetic fixtures. It has no database connection, startup hook, command-line export or production-data access. Fixture collision tests must include raw-distinct ASCII/accent/sharp-s keys, cross-tenant equivalents, dotless-I noncollision, nulls, empty strings, duplicate IDs and delimiter-containing components. No key is rewritten, merged, deleted or selected as a winner.

## Decision and subsequent gate

Publish `docs/compat/plan-12-external-id-equality.md` with the hypothesis identifier, Go/x/text pins, every vector's basis, expected relation, computed relation and mismatch status, synthetic collision examples and verification results. State that the hypothesis does not cover whitespace lookup and that passing the other finite vectors proves no broader Unicode parity.

The next product design must resolve lookup versus creation normalization, stored spelling, duplicate envelopes, tenant-scoped database uniqueness, shared locking across all write paths, collision-blocking transactional migration, rollback and concurrent deployment behavior. Race behavior is a separate unresolved contract: the current serialized candidate may update after waiting, while Plan 9's measured missing-key races returned a create and DUPLICATE_VALUE. Do not infer a deterministic race guarantee or select a loser policy in this experiment.

Any further reference collection gets a separately reviewed scope and existing identity/recovery gates. No live reference calls are needed here. A mismatch is a useful completed experiment and blocks promotion to runtime; it is not a failed task to conceal.

## Boundaries and acceptance

No changes to `internal/platform`, schema, catalog v2, accepted profiles, historical captures, API routes, body validation, query equality or UI. No new live calls, credentials or private artifact reads. Work directly on main under the existing roadmap convention; creating this spec and plan does not execute them.

Acceptance requires meaningful Go unit/race tests, deterministic nonmutating collision analysis and a complete honest decision report. Run focused package tests, `go test -race ./internal/compat/externalid`, `go test ./...`, `go vet ./...` and `git diff --check`. Record any database-test skips rather than claiming database verification; this pure experiment needs no database. Obtain a fresh whole-change review before declaring execution complete. No universal compatibility, migration readiness or Plan 10 completion claim is permitted.
