# Plan 1 follow-ups

Deferred findings from the Plan 1 review (2026-10-15). Items under
"Prerequisites" must be resolved before the named milestone or event.

## Fixed in the review pass

- `metadata` verified the deployed field through `describe`, which hides
  fields without field-level security; it now uses Metadata API `read`.
- `python-crud` could not reach an HTTP candidate (simple-salesforce always
  uses HTTPS); it now reports `blocked` with the reason.
- A successful parent retry of a failed Python cleanup still left a failed
  cleanup row; a crashed or timed-out probe's journaled IDs were never
  cleaned. Journaled IDs are now adopted by the parent tracker.
- `crud` and `errors` disagreed on the deleted-record error code; both accept
  `NOT_FOUND` or `ENTITY_IS_DELETED` and compare the observed code.
- No way to compare two reference runs (roadmap M1 exit gate); added the
  `repeatability` command.
- Connector versions were hard-coded; they are now pinned in the profile and
  checked against the lockfiles and installed CLI plugins.
- `external-services.yaml` and `localization.yaml` were not validated; they
  now carry preconditions/assertions/observation status and source checks.
- OpenAPI lacked endpoints the harness depends on: `/services/data`,
  `/services/oauth2/userinfo` and `composite/sobjects`.
- Added `CLEAN_ROOM.md`.

## Prerequisites

| Before | Item |
|---|---|
| First live observation | Use the recorded authorization for the named tenant and verify profile/persona readiness under `CLEAN_ROOM.md`; role separation and extra terms-review approval are not required. |
| First live observation | Confirm the deleted-record error code and the `metadata` scenario against the reference org; narrow `MISSING_RECORD_CODES` if one code is observed consistently. |
| First live observation | Run `repeatability` on two reference captures and add normalization only for fields it proves nondeterministic. |
| M2 candidate runs | `sf org login access-token` reads `/services/data/v51.0/sobjects/User/{id}` with a hard-coded v51.0, regardless of the configured API version. The candidate must serve it or the CLI scenarios block. |
| M2 candidate runs | Serve the local candidate over HTTPS with a trusted certificate (for example via `REQUESTS_CA_BUNDLE` and `NODE_EXTRA_CA_CERTS`) so the Python probe can run. |
| M3 | Bulk API job lifecycle/results scenarios (roadmap connector requirement), not in Plan 1. |
| M3 | Connector-driven authentication and token refresh scenarios; Plan 1 uses access tokens only. |

## Nice to have

- Synthetic wiring tests exist only for `crud`, `rollback` and Python fixture
  adoption; add them for `upsert`, `relationships` (pagination cursor
  checks), `permissions`, `errors`, `locale`, and the CLI paths.
- `locale` does not cover currency (`DefaultCurrencyIsoCode`, multi-currency
  orgs only) although the plan calls the settings independent.
- `requirements.lock` pins versions without `--hash` entries.
- Fixture labels: relationship contacts (`Compat 000`…) and renamed records
  (`Compatibility updated`, `Python updated`) do not carry the run ID; the
  journal and parent account still identify them.
- `relationships` cleanup deletes 201 contacts one request at a time.
- `capture` marks a report `live` as soon as any scenario starts, even if
  every scenario then blocks before a network call.
- A metadata run leaves a deleted custom field in the org's "Deleted Fields"
  list until purged; repeated runs can approach the org's field limit.
- Plan 1 is a summary rather than a task-by-task plan; future plans should
  list tasks so the implementation log can be checked against them.
