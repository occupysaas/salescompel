# Plan 1 implementation log

Plan: docs/plans/2026-10-15-plan-1-compatibility-laboratory.md

- Approved plan saved with OpenAPI 3.0-only additions.
- Ruling: use a feature branch in the shared checkout; keep changes reviewable without moving the user into another directory.
- Ruling: OpenAPI API documentation and External Services import are separate surfaces. Schema validation is not runtime certification.
- Ruling: local tests can exercise synthetic observations; live connectors must never silently substitute fixtures.
- Pre-flight: scenario IDs, profile requirements, result format, comparator and validator share one registry.

## Delivered

- Task 1: complete — profile, provenance register, behavior contracts and source references.
- Task 2: complete — locked Node/TypeScript and Python connector harnesses; explicit blocked outcomes.
- Task 3: complete — eleven executable scenarios, bounded fixture ownership, private recovery journals and cleanup reports. Live observations remain pending.
- Task 4: complete — per-language fixture catalogs, schemas, ICU validation, fallback/legacy-code contracts.
- Task 5: complete — JSON/Markdown reports, strict profile-bound comparator, hosted offline CI and configured self-hosted live CI.
- Task 6: complete — OpenAPI 3.0-only validation, proposed REST contract, synthetic External Services fixtures, official Salesforce Einstein Bots YAML with unchanged bytes, commit/checksum and BSD license.
- Review: an independent reviewer identified and checked fixes for incomplete comparison manifests, plugin package resolution, cleanup evidence, positive permission controls and crash recovery. Remaining recovery-path hardening was implemented and regression-tested.
- Verification: clean npm ci succeeded; 24 offline tests pass on Node 22 and the local Node 26 runtime, TypeScript and formatting checks pass, repository/OpenAPI validation passes, Python pip check and byte compilation pass. Live Salesforce/API/CLI scenarios have not run against a real org or a SalesCompel server.
- Dependency risk: remaining Salesforce CLI toolchain advisories are recorded in dependency-audit.md; no audit-clean claim.

## Review pass — 2026-10-15

A plan/code review found live-run defects and roadmap gaps. Fixed: Metadata API verification for the `metadata` scenario, HTTPS blocking and journal-based cleanup adoption for the Python probe, a shared deleted-record error-code rule, a `repeatability` command for the M1 gate, profile-pinned connector versions, validated deferred contracts, missing OpenAPI discovery/userinfo/collections paths and `CLEAN_ROOM.md`. Offline tests: 29 passing. Deferred items are in [plan-1-followups.md](../plans/plan-1-followups.md). Live verification remains pending.
