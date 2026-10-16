# Plan 3 — Reference-evidence readiness and first observations

> Policy update (2026-10-16): [CLEAN_ROOM.md](../../../CLEAN_ROOM.md) supersedes this historical document's mandatory role separation, edition restrictions and extra terms-review gates. Use [Plan 4](../plans/2026-10-16-plan-4-black-box-reference-baseline.md) for current execution. Authorization, fixture readiness and evidence integrity remain required.

Date: 2026-10-15. Status: proposed design for review; implementation and live
observation have not started.

## Intent and scope

Close the reference-evidence prerequisites before expanding the M3 candidate.
The user accepted this recommended sequencing after Plan 2. This work serves
maintainers, authorized observers and implementers who need reproducible behavior
contracts before changing the candidate's protocol semantics.

Use the existing Plan 1 laboratory. Deliver an offline-verifiable readiness and
evidence workflow, then obtain two reference captures through a separately
authorized observer. Offline readiness and observed behavior are separate outcomes.
Neither completes M3 or certifies Salesforce compatibility.

All changes stay on main under the roadmap's branch policy. Do not copy
Salesforce prose, code or UI assets. Preserve the CLEAN_ROOM.md role boundary.

## Approach

Recommended: extend the current capture, validation and comparison workflow with
explicit authorization and reviewed-evidence records. This reuses the pinned
clients, recovery journals and eleven scenarios already present.

Documentation-only preparation would leave capture authorization and evidence
consistency unenforced. Replacing the harness would duplicate working fixture
recovery and client integration. Neither alternative is warranted.

## Readiness and authorization

Add a versioned reference-readiness record and schema. Record a non-sensitive
reference alias, the exact profile hash and API version, maintainer decision
reference/date, authorized observer identity, separate implementer identity,
allowed scenario scope, and fixture/persona readiness attestations. Actual org
identifiers, endpoints and credentials remain in private runtime configuration.

The maintainer must supply the terms-review decision and explicitly authorize
observation; the assistant must not infer these from approval of this design or
populate an acceptance on the maintainer's behalf. Missing decisions remain
pending and fail live preflight. The record is an auditable attestation, not a
cryptographic proof of identity or an automatic authorization grant.

A read-only offline preflight checks schema, decision presence, role separation,
profile/version binding, scenario coverage and configuration completeness. Run
the same gate inside reference capture before creating a connection or invoking
any client subprocess. Candidate captures retain their existing independent
configuration checks. A modified profile or expanded scope requires a matching
readiness record; no automatic widening is allowed.

Do not create an org, deploy fixtures, change user access, or contact Salesforce
as part of offline preflight. The authorized observer performs setup under the
existing runbook after authorization. Credentials are supplied privately through
environment variables or the configured secret store, never through tracked files.

## Evidence lifecycle

Keep raw captures and recovery journals private. An observer runs the unchanged
required scenario set twice, sequentially, with the same pinned profile, API and
connector versions. Failed cleanup prevents acceptance and requires recovery
before another capture. Never remove a required scenario to obtain a passing run.

Run the existing repeatability comparator. Investigate differences and introduce
normalization only for demonstrated nondeterminism, with a contract-specific
rationale. A passing comparison is necessary but is not itself a reviewed note.

The observer independently summarizes behavior into reviewed observation records
under docs/compat/observations/. Each record binds its contract IDs to the profile
hash, API version, connector versions, capture dates, private capture digests,
repeatability outcome, cleanup result and review identity/date. Notes exclude
credentials, org/user identifiers, personal data and verbatim messages. Raw
capture digests identify private evidence without publishing that evidence.

Extend repository validation so a pilot contract can move from not_run to
observed only when a valid reviewed record supports it. Reject missing or
mismatched references, unknown contracts, incomplete required evidence and
unsupported status values. Existing deferred contracts remain not_run until
their own scenarios and observations exist. Observed does not mean the candidate
implements or passes that behavior.

The implementer consumes only public specifications and reviewed notes. This
implementation session does not become the observer for the same area. A separate
authorized person or session performs captures; approval of the design does not
assign that role automatically.

## Boundaries and files

- docs/compat/: readiness record, implementation-log decision reference,
  observation records and updated setup/recovery documentation.
- compat/schemas/: readiness and observation schemas.
- compat/clients/node/src/: a focused readiness/evidence module, CLI preflight,
  capture integration and repository validation integration.
- compat/clients/node/test/: offline tests for gates, evidence consistency and
  unchanged candidate behavior.
- Existing live CI: invoke the same gate; retain protected secrets and private,
  persistent recovery storage.

No Go service or Vue product behavior changes belong to this work package.
Bulk, refresh-flow tests, additional CLI distribution/plugin runners and routing
proxy work remain separately tracked M1/M3 obligations. The current pilot uses
the npm CLI channel; these broader obligations must not be marked complete.

## Failure behavior and verification

Preflight returns the existing blocked/configuration exit code 2, identifies
missing requirements without exposing secrets, and makes no network calls.
Capture reports retain their required scenario manifest and blocked results.
Schema or evidence failures cannot produce a passing certification report.

Offline tests cover missing/rejected decisions, identical observer/implementer,
stale profile hashes, API or scope mismatch, missing personas, malformed notes,
unreviewed notes, failed cleanup and absent evidence. Verify the live gate occurs
before connection/client creation with instrumented offline tests. Verify valid
records are accepted, existing not_run records remain valid, and candidate smoke
configuration does not acquire reference-only requirements.

Run npm --prefix compat/clients/node run check for implementation validation.
Live acceptance requires two real authorized captures, successful repeatability,
successful cleanup and reviewed notes for the pilot contracts. Synthetic tests
exercise tooling only and must not satisfy live acceptance.

## Completion and remaining dependencies

Report two independent results: tooling ready, and reference evidence accepted.
If no authorized reference org or observer is available, deliver the tested
tooling and record the live phase as blocked; do not claim the prerequisites
have closed. M0 inventory, scale/downtime decisions and expanded M1 runner
obligations retain their own roadmap gates.

M1R remains a separate work package: inventory all archive releases, review and
map findings with explicit source gaps, and reconcile required-profile findings
before M3 contract freeze. This plan neither performs nor claims that review.
The next REST/query implementation plan must identify its contract-specific
reference and M1R blockers before freezing semantics.

## Maintainer inputs needed for the live phase

1. Authorized disposable reference org and its supported API/profile settings.
2. Recorded terms-review decision and explicit observation authorization.
3. Named observer and separate implementer, with controlled evidence access.
4. Prepared administrator, restricted and locale personas plus private credentials.

These are external inputs, not values the implementation may invent. They do
not block writing the implementation plan or implementing offline readiness
tooling after the required design/plan review.
