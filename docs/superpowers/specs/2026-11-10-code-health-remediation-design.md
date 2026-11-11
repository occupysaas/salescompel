# Code Health Remediation

**Status:** Draft for review
**Date:** 2026-11-10
**Depends on:** None (gates all Plan 28+ work)

## Summary

Fix every gate finding in existing code before adding new features. The gate
ratchet run against `upstream/main` found 267 pre-existing pattern findings,
22 oversize files, and structural debt. This spec defines what to fix, what
to defer, and what is a confirmed false positive.

## Problem inventory

### Already fixed (folded into this effort)

| ID | Location | Problem | Status |
|----|----------|---------|--------|
| Journal queue poisoning | `compat/clients/node/src/external-id-recovery.ts:106-131` | `queues.set(dir, next)` stored a potentially-rejected promise; concurrent callers chaining onto it would fail without reaching disk. Fixed by resolving unconditionally and re-throwing to the awaiting caller. | Fixed — include in this PR |

### Critical — fix immediately

| ID | Count | Location | Problem |
|----|-------|----------|---------|
| PREVENT-002 | 3 | `foundation.ts:335,339,363` | SQL string concatenation (potential injection) |
| PREVENT-003 | 1 | `candidate-oauth-worker.ts:207` | Hardcoded credential (verify: likely false positive) |

### Error severity — fix in this effort

| ID | Count | Location | Problem |
|----|-------|----------|---------|
| PREVENT-011 | 201 | `compat/clients/node/src/*.ts` | TypeScript `any` type defeats type safety |
| PREVENT-009 | 22 | `internal/**/*.go` | Ignored Go error return values |
| PREVENT-001 | 5 | `compat/clients/node/src/*.ts` | JSON.parse property access without null check |

### Warning severity — fix in this effort

| ID | Count | Location | Problem |
|----|-------|----------|---------|
| PREVENT-012 | 21 | `compat/clients/node/src/*.ts` | `console.log` in production code |
| PREVENT-010 | 13 | `internal/**/*.go` + `cmd/` | Goroutine launched without context |

### Warning severity — fix in this effort (file sizes)

| ID | Count | Location | Problem |
|----|-------|----------|---------|
| Size hard | 11 | `compat/clients/node/src/*.ts`, `internal/platform/*.go` | Over hard limit (500/600 lines) — split files |
| Size soft | 22 | Various | Over soft limit (300 lines) — split files |

The 11 hard-limit files carry reasoned overrides in `scope.json` that
currently grandfather them. This pass splits each one so the overrides can
be removed and the ratchet reaches zero.

### Deferred / false positive

| ID | Count | Decision | Reason |
|----|-------|----------|--------|
| PREVENT-026 | 1 | **False positive** | `oauth_page.go:48` — URL fetch target is the configured OAuth issuer, not arbitrary user input. Dismiss with allowlist entry. |
| PREVENT-003 | 1 | **Verify then dismiss** | `candidate-oauth-worker.ts:207` — check if the "credential" is a test fixture default or placeholder. If so, add allowlist entry. If real, rotate and remove. |

## Fix strategy

### 1. SQL injection (PREVENT-002) — `foundation.ts`

Replace string concatenation with parameterized queries. The three
occurrences at lines 335, 339, 363 build SQL with interpolated values.
Use `$1`/`$2` placeholders and pass values as query arguments.

### 2. Ignored Go errors (PREVENT-009) — 22 occurrences

Each ignored `error` return gets explicit handling:
- **Deferred close/rollback errors** (`defer rows.Close()`, `defer tx.Rollback()`):
  use `defer func() { _ = rows.Close() }()` pattern to mark intentional discard.
- **Ignored scan/assign errors** in loops: propagate or wrap.
- **Ignored errors in cleanup paths**: log or wrap.

Files: `admin.go:125`, `seed.go:28`, `migrate.go:35,37,41,43,47`,
`composite.go:69`, `http.go:192,317,331`, `oauth_page.go:81,82,84,138`,
`runtime.go:123`, `select.go:103,121`, `validation.go:30,101`,
`write.go:66,75`.

### 3. Goroutine context (PREVENT-010) — 13 occurrences

Pass `context.Context` to goroutines or use `context.WithCancel` to make
them cancellable. In test files, use `t.Context()` or `context.Background()`
with `t.Cleanup(cancel)`.

Files: `main.go:89`, test files in `internal/platform/`, `internal/migration/`,
`internal/metadata/`, `internal/compat/externalid/`.

### 4. TypeScript `any` (PREVENT-011) — 201 occurrences

Replace `any` with proper types. The bulk are in `compat/clients/node/src/`:
- JSON response bodies: type as `Record<string, unknown>` or define interfaces.
- Callback parameters: type as specific event/error types.
- Generic plumbing: use `unknown` with type guards, not `any`.

Strategy: fix file-by-file starting with the most-critical paths
(`foundation.ts`, `candidate-oauth-worker.ts`, `candidate-schema.ts`).
Tests may use `as unknown as TargetType` where mocking requires it.

### 5. console.log (PREVENT-012) — 21 occurrences

Replace `console.log` with the project's logger or `process.stderr.write`.
In CLI tools (`cli.ts`, `candidate-acceptance-command.ts`), `console.log`
to stdout may be intentional output — review each: intentional CLI output
gets an allowlist entry, debug logging gets replaced with a proper logger.

### 6. JSON.parse safety (PREVENT-001) — 5 occurrences

Wrap `JSON.parse` results with null checks or type guards before property
access. Files: `candidate-acceptance-preflight.ts:263`,
`candidate-acceptance-results.ts:163`, `external-id-native.ts:52`,
`metadata-read-process.ts:92`, `process-clients.ts:67`.

### 7. File splits — all files must be under 500 lines

Hard requirement: every source file under 500 lines. Split oversize files
by responsibility into sibling modules. Each split preserves the original
file's public interface via re-exports where needed.

**Already split:**

| File | Was | Now | Modules created |
|------|-----|-----|-----------------|
| `foundation.ts` | 752 | 334 | `foundation-helpers.ts` (231), `foundation-field-scenarios.ts` (214) |

**Remaining hard-limit files (over 500 lines):**

| File | Lines | Limit | Split strategy |
|------|-------|-------|----------------|
| `candidate-oauth-worker.ts` | 842 | 500 | Extract token refresh + device flow into separate modules |
| `foundation-assessment.ts` | 625 | 500 | Extract scoring logic from assessment assembly |
| `candidate-openapi.test.ts` | 620 | 500 | Split into per-contract-area test files |
| `http_test.go` | 619 | 500 | Split into handler-specific test files |
| `candidate-schema.ts` | 612 | 500 | Extract schema comparison from schema loading |
| `oauth_http_test.go` | 563 | 500 | Split into handler-specific test files |
| `external-id-normalization-recovery.ts` | 557 | 500 | Extract normalization steps from recovery flow |
| `cli.ts` | 556 | 500 | Extract command handlers into per-command files |
| `external-id-normalization-assess.ts` | 548 | 500 | Extract assessment checks from orchestration |
| `target-inventory.test.ts` | 541 | 500 | Split into per-area test files |
| `external-id-body-assess.ts` | 538 | 500 | Extract body validation from assessment |
| `oauth_tokens_test.go` | 537 | 500 | Split into per-flow test files |
| `foundation-assessment.test.ts` | 512 | 500 | Split into per-assessment test files |
| `target-inventory-command.test.ts` | 509 | 500 | Split into per-command test files |
| `external-id-body-recovery.ts` | 505 | 500 | Extract recovery steps from orchestration |

Then the soft-limit files (over 300 lines): same approach — extract
cohesive sub-concerns into sibling modules until each is under 500 lines.

After splitting, remove the corresponding `size_overrides` entries from
`.guardrails/scope.json` so the ratchet reflects the true limit.

## Non-goals

- Performance improvements.
- New features.
- Changing public API behavior.

## Acceptance

- Pattern ratchet shows **0 new** findings AND **0 remaining** PREVENT-002,
  PREVENT-009, PREVENT-010, PREVENT-011, PREVENT-012, PREVENT-001 findings
  (all fixed or allowlisted with reason).
- PREVENT-003 and PREVENT-026 either fixed or allowlisted with documented
  false-positive reason.
- `go test ./...` passes.
- `npm ci --prefix compat/clients/node && npm --prefix compat/clients/node run check` passes.
- All gates green.
