# Code Health Remediation Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Fix all gate findings — SQL injection, ignored errors, goroutine safety, `any` types, console.log, JSON.parse safety, and oversize files — before any new feature code.

**Architecture:** File-by-file fixes following the spec's fix strategy. Each task addresses one finding class across all affected files, runs the gates, and commits.

**Tech Stack:** Go 1.25, TypeScript 5, PostgreSQL 17.

**Spec:** [Code health remediation design](../specs/2026-11-10-code-health-remediation-design.md).

## Global Constraints

- Zero new gate findings — the ratchet must show 0 new AND 0 remaining for fixed rules.
- No behavior changes except where fixing a real bug (SQL injection, error handling).
- Tests must pass after each task. Run `go test ./...` and `npm --prefix compat/clients/node run check` per task.
- DCO sign-off on every commit.

---

### Task 0: Journal queue poisoning fix (already implemented)

**Files:** `compat/clients/node/src/external-id-recovery.ts:106-131`.

- [x] Fix: resolve the journal queue promise unconditionally so a rejection cannot poison the queue for concurrent callers. The settled promise is stored in the map; errors re-throw to the awaiting caller.
- [ ] Include this fix in the cleanup branch. Rebase or cherry-pick from the existing fix commit if not already on the branch.
- [ ] Run `npm --prefix compat/clients/node run check`. Expected: PASS.
- [ ] Commit `fix: prevent journal queue poisoning on write failure in appendJournal` (already signed-off).

### Task 1: SQL injection fixes (PREVENT-002, 3 findings) — DONE

**Files:** Modify `compat/clients/node/src/foundation.ts:335,339,363`.

- [x] Read the three SQL concatenation sites. Replace string interpolation with template literals.
- [x] Split foundation.ts into modules (foundation-helpers.ts, foundation-field-scenarios.ts) to stay under 500 lines.
- [x] Run gates. Expected: PASS, no PREVENT-002.
- [ ] Commit `fix: parameterize SQL queries in foundation.ts to prevent injection`.

### Task 2: Go ignored errors (PREVENT-009, 22 findings) — DONE

**Files:** `internal/metadata/admin.go`, `seed.go`, `internal/migration/migrate.go`, `internal/platform/composite.go`, `http.go`, `oauth_page.go`, `runtime.go`, `internal/query/select.go`, `internal/records/validation.go`, `write.go`.

- [x] For each of the 22 sites: identify the pattern and apply proper error handling.
- [x] Run `go build ./...`. Expected: PASS, no PREVENT-009.
- [ ] Commit `fix: handle all ignored error return values in Go code`.

### Task 3: Goroutine context (PREVENT-010, 13 findings) — DONE

**Files:** `cmd/salescompel/main.go`, test files in `internal/platform/`, `internal/migration/`, `internal/metadata/`, `internal/compat/externalid/`.

- [x] For production code (`main.go:89`): pass `context.Context` to goroutine.
- [x] For test code: pass `ctx` or `context.Background()` to each goroutine.
- [x] Run `go build ./...`. Expected: PASS, no PREVENT-010.
- [ ] Commit `fix: add context propagation to all goroutine launches`.

### Task 4: JSON.parse safety (PREVENT-001, 5 findings) — DONE

**Files:** `candidate-acceptance-preflight.ts:263`, `candidate-acceptance-results.ts:163`, `external-id-native.ts:52`, `metadata-read-process.ts:92`, `process-clients.ts:67`.

- [x] Wrap each `JSON.parse` result with a null/undefined check or type guard before property access.
- [ ] Run `npm --prefix compat/clients/node run check`. Expected: PASS, no PREVENT-001.
- [ ] Commit `fix: add null checks after JSON.parse in compat clients`.

### Task 5: console.log review (PREVENT-012, 21 findings)

**Files:** Various `compat/clients/node/src/*.ts`.

- [ ] For each `console.log`: determine if it's intentional CLI output (stdout for user) or debug logging.
- [ ] Intentional CLI output in `cli.ts`/command files: keep and add allowlist entry in `.guardrails/silent-success-allowlist.json` with reason.
- [ ] Debug logging: replace with `process.stderr.write` or the project's logger pattern.
- [ ] Run `npm --prefix compat/clients/node run check`. Expected: PASS, no new PREVENT-012.
- [ ] Commit `fix: replace debug console.log calls with proper logging`.

### Task 6: TypeScript `any` elimination (PREVENT-011, 201 findings)

**Files:** All `compat/clients/node/src/*.ts` with `any` usage. Priority order: `foundation.ts`, `candidate-oauth-worker.ts`, `candidate-schema.ts`, `candidate-openapi-traffic.ts`, then remaining files.

- [ ] For each `any`: replace with `Record<string, unknown>`, specific interface, or `unknown` + type guard. Tests may use `as unknown as T` where mocking requires it.
- [ ] Work file-by-file. After each file, run `npm --prefix compat/clients/node run check` to verify no regressions.
- [ ] Run final `npm --prefix compat/clients/node run check`. Expected: PASS, no PREVENT-011.
- [ ] Commit per file or per batch: `fix: replace any types with proper type annotations in <file>`.

### Task 7: False positive allowlisting (PREVENT-003, PREVENT-026)

**Files:** `.guardrails/` allowlist files.

- [ ] Verify PREVENT-003 (`candidate-oauth-worker.ts:207`): read the code, confirm it's a placeholder/test fixture, not a real credential. If real, fix it. If false positive, add allowlist entry with reason.
- [ ] Verify PREVENT-026 (`oauth_page.go:48`): confirm URL fetch target is the configured OAuth issuer. Add allowlist entry with reason.
- [ ] Commit `fix: disposition false-positive security findings with documented allowlist entries`.

### Task 8: File splits — hard-limit files (15 remaining)

**Files:** See spec's file split table. `foundation.ts` already split into `foundation-helpers.ts` + `foundation-field-scenarios.ts`.

- [x] Split `foundation.ts` (752→334+231+214).
- [ ] For each remaining file: extract cohesive sub-concerns into sibling modules. Preserve public interface via re-exports where needed. Update imports in consumers.
- [ ] After splitting each file, remove its `size_overrides` entry from `.guardrails/scope.json`.
- [ ] Run tests after each split. Expected: PASS.
- [ ] Commit per file: `refactor: split <file> to stay within size limits`.

### Task 9: File splits — soft-limit files (over 300 lines)

**Files:** Files over 300 lines not already handled in Task 8.

- [ ] Same approach as Task 8. Extract sub-concerns. Each result under 500 lines.
- [ ] Run tests after each split. Expected: PASS.
- [ ] Commit per file or batch.

### Task 10: Final gate run and verification

- [ ] Run full gate suite. Expected: all green, 0 findings for fixed rules, no size overrides remaining.
- [ ] Run `go test ./...` and `npm --prefix compat/clients/node run check`. Expected: PASS.
- [ ] Commit any final fixes.
