# Plan 12 External-ID Equality Feasibility Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Status:** Complete (2026-10-20). All three tasks were implemented after the
initial move to Plan 13. Implementation commits: `025f2f3` (hypothesis and tests),
`f731265` (collision analysis), and `f910ca4` (decision report). Fresh whole-change
review found no Critical, Important or Minor issues. See the
[decision and verification record](../../compat/plan-12-external-id-equality.md).
Database integration tests were skipped because `TEST_DATABASE_URL` was unset;
this offline experiment requires no database. The two whitespace mismatches
block runtime promotion but do not leave Plan 12 incomplete. Runtime equality,
migration and race semantics require a separate design.

**Goal:** Evaluate a named Unicode equality hypothesis and identify synthetic tenant-scoped collisions before designing a storage migration.

**Architecture:** A pure Go laboratory package computes canonical keys and groups collisions, without imports from product runtime. Tests distinguish observed relations from hypothesis results, including the known whitespace mismatch. A decision report bounds what later runtime work may infer.

**Tech Stack:** Go 1.25.0; existing golang.org/x/text v0.29.0; standard Go testing.

**Spec:** `docs/superpowers/specs/2026-10-20-plan-12-external-id-equality-design.md` (read in full before execution).

## Global Constraints

- Use Go 1.25.0 and the already pinned `golang.org/x/text v0.29.0`; add no dependency or version update.
- No trimming, accent removal, compatibility normalization or locale-specific tailoring.
- Never URL-decode inside this package.
- No changes to `internal/platform`, schema, catalog v2, accepted profiles, historical captures, API routes, body validation, query equality or UI.
- No new live calls, credentials or private artifact reads.
- Work directly on main under the existing roadmap convention; creating this spec and plan does not execute them.
- A mismatch is a useful completed experiment and blocks promotion to runtime; it is not a failed task to conceal.

## Review Focus

- Invalid UTF-8 must return an error, not silently replace bytes (Task 1).
- Transform state must be safe under concurrent calls and canonicalization idempotent (Task 1).
- Null keys must remain distinct from empty keys (Task 2).
- Tenant/ID delimiters and input permutations must not merge groups or change reports (Task 2).
- Known whitespace disagreement must remain visible in passing experiment tests and the decision (Tasks 1 and 3).

## File map

Create `internal/compat/externalid/key.go` and `key_test.go` for the hypothesis; `collisions.go` and `collisions_test.go` for pure diagnostic grouping. Modify only go.mod's x/text dependency classification if necessary. Create `docs/compat/plan-12-external-id-equality.md` for results. Existing product code and Node laboratory are reference material only.

### Task 1: Characterize the named canonical-key hypothesis

**Files:** Create `internal/compat/externalid/key.go`, `internal/compat/externalid/key_test.go`; modify `go.mod` only to classify x/text as direct.

**Interfaces:** Produces `const HypothesisVersion = "nfc-full-fold-v1"` and `func CanonicalKey(value string) (string, error)` in package `externalid`.

- [x] **Step 1: Write failing table-driven tests.** Name them `TestCanonicalKeyReferenceVectors`, `TestCanonicalKeyRobustness` and `TestCanonicalKeyConcurrent`. Encode all nine spec rows with separate `observedEqual` and `hypothesisEqual` values. The first seven agree; both whitespace rows have observedEqual=true and hypothesisEqual=false. Assert calculated equality equals hypothesisEqual and that exactly those two rows disagree with observedEqual. For literal decoded input assert canonical output `p-/percent%é`; assert `p-%2F` yields `p-%2f`, distinct from `p-/`, showing no decoding. Assert `e` differs from `é`, null is not a string input, empty maps to empty, tabs/NBSP are preserved, invalid `string([]byte{0xff})` returns an error, every valid vector is idempotent, and 32 concurrent workers repeat the vector table without disagreement. Use subtests and worker error channels rather than calling Fatal in worker goroutines.

- [x] **Step 2: Verify the tests fail.** Run `go test ./internal/compat/externalid -run TestCanonicalKey -count=1`; expect undefined CanonicalKey/HypothesisVersion, not an unrelated environment failure.

- [x] **Step 3: Implement the hypothesis.** Validate `utf8.ValidString`, apply `norm.NFC.String`, a per-call `cases.Fold().String`, then `norm.NFC.String`. Return an error and empty result for invalid UTF-8. Do not introduce product length or null validation. Pin the version constant exactly. Move x/text from indirect to direct without changing its version or unrelated requirements.

- [x] **Step 4: Verify determinism and race safety.** Run `go test ./internal/compat/externalid -run TestCanonicalKey -count=1` and `go test -race ./internal/compat/externalid -run TestCanonicalKey -count=1`; expect PASS with the two reference disagreements asserted as findings.

- [x] **Step 5: Commit the hypothesis and tests.** Stage only Task 1 files and commit `test: characterize external ID equality hypothesis`.

### Task 2: Detect synthetic collisions without mutating records

**Files:** Create `internal/compat/externalid/collisions.go` and `internal/compat/externalid/collisions_test.go`.

**Interfaces:** Consumes Task 1 `CanonicalKey`. Produces `type Record struct { Tenant string; ID string; Key *string }`, `type Collision struct { Tenant string; Canonical string; IDs []string }`, and `func FindCollisions(records []Record) ([]Collision, error)`.

- [x] **Step 1: Write failing tests.** Add `TestFindCollisions`, `TestFindCollisionsInvalidInput`, `TestFindCollisionsDeterministic`, `TestFindCollisionsPreservesInput`. Assert tenant `a`, IDs `2`/`1`, keys `p-AbC`/`p-abc` yield exactly `{Tenant:"a", Canonical:"p-abc", IDs:["1","2"]}`. Add accent and sharp-s groups, a separate tenant with equal canonical keys, and distinct `p-I`/`p-ı` without collision. Two empty keys collide; two null keys do not. Empty tenant/ID, repeated same-tenant ID (even if null), or invalid UTF-8 must return error and no partial collisions. Same ID across tenants is valid. Use tuple-adversarial tenants/keys containing `\x1f` to prove no concatenation collision. Compare all fixed permutations of a small dataset and deep snapshots before/after, including pointed-to keys. Require deterministic tenant/canonical/ID sorting.

- [x] **Step 2: Verify tests fail.** Run `go test ./internal/compat/externalid -run TestFindCollisions -count=1`; expect missing types/function.

- [x] **Step 3: Implement collision grouping.** Validate all records, including duplicate identity detection, before returning results. Use structural map keys for `(tenant, canonical)` and `(tenant, ID)`, skip null keys only after identity validation, retain groups of two or more IDs and sort output per spec. Copy IDs into result-owned slices; never sort or modify the input. Return an empty slice for no collisions and nil result on error. No database or CLI adapters.

- [x] **Step 4: Verify package behavior.** Run `go test ./internal/compat/externalid -count=1` and `go test -race ./internal/compat/externalid -count=1`; expect PASS, no race diagnostics.

- [x] **Step 5: Commit the diagnostic.** Stage only Task 2 files and commit `feat: add offline external ID collision analysis`.

### Task 3: Publish the bounded feasibility decision

**Files:** Create `docs/compat/plan-12-external-id-equality.md`.

**Interfaces:** Consumes the hypothesis version, tested comparison table and diagnostic results. Produces a human-readable decision; no machine-consumed runtime approval flag.

- [x] **Step 1: Run final verification.** Run `go test ./internal/compat/externalid -count=1`, `go test -race ./internal/compat/externalid -count=1`, `go test ./... -count=1`, `go vet ./...` and `git diff --check`. Inspect results and explicitly record database skips. A failing command must be resolved or reported; no passing claim based only on intended output. Do not provision reference sessions or run Node capture commands.

- [x] **Step 2: Write the decision from actual results.** Include hypothesis and version pins, all nine rows with observed/computed relations, the two whitespace mismatches, robustness results labeled unobserved, synthetic collision examples and verification commands/outcomes. Explain why no runtime promotion follows: lookup/create normalization, shared uniqueness/locking, stored spelling, transactional collision refusal, rollback/deployment and missing-key race semantics require the next design. Link Plans 9, 10, 11 and this spec. Preserve Plan 10's blocked status and Plan 9's drift qualification. No credentials, actual tenant IDs or private artifacts are needed.

- [x] **Step 3: Obtain fresh whole-change review.** Review against the spec, focusing on truthful mismatch reporting, tuple-safe tenant isolation, immutable input and lack of product wiring. Resolve Important/Critical findings; rerun only checks affected by changes. State review scope and remaining limitations without implying fresh reference evidence.

- [x] **Step 4: Commit the decision.** Run `git diff --check`, stage the decision and commit `docs: record external ID equality feasibility decision`.

## Completion boundary

This plan is complete when the offline experiment, collision diagnostic, report and review meet the spec. Runtime lookup, schema migration and race behavior remain follow-up work. A product fix requires a separately reviewed design; this plan provides evidence for that design without selecting unsupported Salesforce behavior.
