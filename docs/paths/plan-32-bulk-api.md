# Plan 32 — Bulk API 2.0

Issue: —

## Track and dependencies

- **Track:** R — Records API
- **Size:** Full (from the roadmap). Write a full design before planning.
- **Needs:** Plan 29 (merged), [Plan 30](plan-30-record-lifecycle.md)
- **Roadmap:** [§3, Plan 32](../superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md#3-plans)

## Scope

Restated from the roadmap. If anything here seems to need more, raise it under "Decisions needed" rather than widening the plan.

- Durable job table and a worker process.
- Ingest and query job lifecycles, abort and retries.
- Result downloads and malformed CSV handling.
- The job table is designed as a transactional outbox so events can be added later (roadmap §6).
- *E2E:* `sf data import bulk`, `sf data export bulk`, JSforce and simple-salesforce bulk.

## Out of scope

- Bulk API 1.0 and SOAP (roadmap §5).
- Events, Pub/Sub and CDC (roadmap §5).

## Code areas

Likely to touch:

- new bulk job storage and worker
- `internal/records` write path
- `internal/soql` for query jobs
- `api/candidate-openapi.yaml`

## Decisions needed

- Where result files live. This is the first use of S3-compatible storage.
- How the worker runs.

## How to start

1. Claim the issue (see [Claiming](README.md#claiming)).
2. Follow the roadmap's plan shape (§1, "Every plan keeps the same shape"), starting with Task 1: observation.
3. Spec: `docs/superpowers/specs/YYYY-MM-DD-plan-32-bulk-api-design.md` (required: this is a Full plan)
4. Plan: `docs/superpowers/plans/YYYY-MM-DD-plan-32-bulk-api.md`
5. OpenSpec change: `openspec/changes/plan-32-bulk-api/`
6. Branch on your fork: `plan-32-bulk-api`. The pull request says `Closes #<issue>`.
