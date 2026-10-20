# Plan 14 external-ID normalization experiment

Status: offline tooling implemented and full lab checks passed; fresh review pending. **Live evidence is blocked** as of 2026-10-20. No normalization rule, runtime change or migration is approved by this report.

## Purpose and scope

The [approved design](../superpowers/specs/2026-10-20-plan-14-external-id-normalization-contract-design.md) follows Plan 12's two whitespace disagreements. The new experiment separates external-ID lookup, POST creation, missing-key upsert creation, ordinary-write uniqueness and explicit same-record spelling edits. It does not presume that these operations share one equivalence rule.

The immutable manifest contains eight ordered input rows across four workflows, plus four missing-key upsert/replay controls: **36 scenarios, 80 non-cleanup operations, at most 64 potential records per capture**. Rows include both directions of leading/trailing ASCII space, ASCII case, composed/decomposed accent, sharp-s/ss and I/dotless-I. The latter four have one direction only. Fields remain Text(80), unique, external ID, caseSensitive=false at API 65.0.

The Node lab has separate normalization contract, recovery, probe, command and assessment modules and a strict JSON schema. Historical Plan 9/11 manifests and evidence meanings remain unchanged. Shared low-level identity, transport, journal and private-file primitives are reused. No Go runtime, catalog, key diagnostic, migration, accepted compatibility profile or UI behavior changes are part of the experiment.

## Live collection blocker

A presence-only environment check found all four prerequisites unset:

- `COMPAT_REFERENCE_URL`
- `COMPAT_REFERENCE_TOKEN`
- `COMPAT_DISPOSABLE` (must equal `yes`)
- `COMPAT_RECOVERY_DIR`

No reference requests were made, no credentials/private reference artifacts were searched or read, and no fixtures were written to a real service. A private config with the expected origin, administrator/org pins and current profile digest is also required before execution. The existing named disposable-tenant authorization and identity/metadata checks still apply. This suite does not require Plan 10's fresh/restricted sessions and does not unblock that separate work.

There are **no new live capture times, report digests or observations to publish**. The following contract table intentionally remains unresolved:

| Dimension | New reference result | Current basis |
| --- | --- | --- |
| Whitespace lookup in both directions | Blocked | Plan 9 measured base-to-spaced lookup only |
| POST creation spelling | Blocked | New cases required |
| Missing-key upsert creation/replay spelling | Blocked | New cases required |
| Ordinary POST uniqueness for all finite pairs | Blocked | Plan 9 measured ASCII variants only |
| Other-record update uniqueness | Blocked | Plan 9 measured ASCII variants only |
| Same-record spelling edits | Blocked | New cases required |

## Running the experiment

Use the existing private external-ID config format from Plan 9/11: `schemaVersion: 1`, `apiVersion: "65.0"`, current foundation `profileHash`, exact HTTPS `origin`, and `administrator: {user, org}`. Keep the config, recovery root and outputs outside the checkout under owner-only directories; files must be 0600 and directories 0700. Do not put credentials in the config or repository. Provide the token through the established private environment handoff.

From `compat/clients/node`, with prerequisites already configured:

```sh
node_modules/.bin/tsx src/cli.ts probe-external-id-normalization \
  --config "$COMPAT_RECOVERY_DIR/config.json" \
  --out "$COMPAT_RECOVERY_DIR/first.json"

# Run only after first.json reports complete collection and clean cleanup.
node_modules/.bin/tsx src/cli.ts probe-external-id-normalization \
  --config "$COMPAT_RECOVERY_DIR/config.json" \
  --out "$COMPAT_RECOVERY_DIR/second.json"

node_modules/.bin/tsx src/cli.ts assess-external-id-normalization \
  --first "$COMPAT_RECOVERY_DIR/first.json" \
  --second "$COMPAT_RECOVERY_DIR/second.json" \
  --out "$COMPAT_RECOVERY_DIR/assessment.json"
```

Each probe output has a sibling `.artifacts` directory containing binding, journal, raw responses, independent state reads, known IDs and cleanup proofs. Outputs are create-exclusive; choose a new output name on each invocation. Probe/recovery returns 0 for complete/clean and 2 for an incomplete/blocked result; malformed invocations or unsafe configuration exit 1. Assessment returns 0 for repeatable, 1 for drift, 2 for blocked evidence. Repeatable is a measurement result, not a product acceptance verdict.

After an interrupted run, use its original artifact directory and a new output:

```sh
node_modules/.bin/tsx src/cli.ts recover-external-id-normalization \
  --config "$COMPAT_RECOVERY_DIR/config.json" \
  --first "$COMPAT_RECOVERY_DIR/first.artifacts" \
  --out "$COMPAT_RECOVERY_DIR/recovery-1.json"
```

The shared external-ID identity lock prevents another Plan 9/11/14 collection while a run is pending. Never delete the lock or journal to bypass an unresolved outcome. Recovery can remove proven fixtures while still returning blocked: a dispatched request without a definitive response can commit later, regardless of empty reads. A latched identity mismatch prevents further automatic deletion. Resolve those cases only with separately reviewed terminal-outcome evidence.

Run recovery with the same pinned suite source/dependencies: the binding includes a digest of the suite, reused primitives, schemas, CLI and lockfile. A changed executable fails binding checks. Preserve the original checkout revision for recovery before upgrading the laboratory.

## Collection and evidence guarantees

The collector checks administrator identity/authority and field metadata before writes, preflights exact Names and intended keys, and uses POST seeds. Every upsert omits the body key and encodes its path once. Each mutation has a durable journal intent before dispatch, a retained response and independent before/after discovery plus exact-ID reads. Unexpected stored key bytes remain observations. A failed seed stops dependent writes; malformed or missing evidence cannot become a passing observation.

Exact random Names and journal-derived operations establish fixture ownership. Key normalization is never the sole deletion criterion. Known IDs survive key/Name changes; an unrecognized Name blocks automatic deletion. Cleanup deletes only proven exact IDs and retains independent absence reads and empty marker discovery. Identity mismatch and unknown dispatched outcomes survive restart.

The fixed suite is sequential. Outer safety ceilings remain 80 records, 160 non-cleanup mutations, ten-minute collection and five-minute cleanup. The manifest dispatch ceiling is 80 mutations. Wire requests have 15-second deadlines, no mutation retries, and no unbounded reconciliation loops.

Assessment checks private file safety, report/schema bindings, source and artifact digests, all 80 operation IDs, raw response/state consistency, cleanup proofs and sequential run chronology. It compares exact spelling and state relationships under synthetic-prefix and one-to-one ID alias substitution. Message prose and transport headers are excluded from equality while retained privately. Record order can vary; field values and hidden mutations cannot disappear from the comparison. Unexpected but agreeing behavior is repeatable. Missing/corrupt evidence is blocked; differing complete behavior is drift.

## Offline verification

Focused tests were written before implementation and observed failing for missing modules/exports. Coverage includes manifest bounds and directionality, POST seeds, local spelling preservation, hidden writes on error responses, same/other-record targeting, metadata/admin gates, unknown dispatch and late commit, identity latch, ownership, private/exclusive files, shared locks, artifact corruption, coverage, chronology, repeatability and state drift.

`npm --prefix compat/clients/node run check` passed: TypeScript, Prettier, **230 passed / 0 failed / 1 skipped** of 231 tests, and offline repository validation. The skip is `candidate: real JSforce CRUD, policy, relationships and upsert`, which requires an explicitly configured running candidate. `git diff --check` passed. These tests use explicitly synthetic in-memory reference fixtures; they are not Salesforce observations. Product database/client integration is not claimed.

Fresh whole-change review is pending.

The baseline lab run exposed three pre-existing failures caused by preview keys absent from partial translated catalogs. A focused lab-validator correction allows missing keys only for catalogs marked partial when their declared fallback chain supplies them. Complete and fixture catalogs still require their own keys, and parameter/cycle validation remains active. No catalogs, coverage labels or product language selection behavior were changed.

## Subsequent product gate

Keep Plan 12's no-trim helper offline. No new evidence here selects NFC/full-fold plus trimming or proves that lookup and uniqueness share one rule. The next runtime design still needs accepted finite observations, explicit supported scope, stored spelling, duplicate responses, all-write uniqueness/locking, tenant isolation, collision-blocking transactional migration from catalog v2, rollback and mixed-version deployment.

Missing-key races, tabs/NBSP/other whitespace, reverse Unicode directions, arbitrary Unicode, query equality and full compatibility remain unmeasured. Plan 10 and Plan 13's visual acceptance retain their independent status.
