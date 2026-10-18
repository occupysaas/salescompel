# Compatibility laboratory

Plan 1 implements tooling and contracts, not Salesforce or SalesCompel runtime parity. [Plan](../plans/2026-10-15-plan-1-compatibility-laboratory.md) · [Roadmap](../plans/2026-10-14-salesforce-compatibility-roadmap.md).

## Offline setup

Use Node.js 22 or newer and Python 3.12 or newer. From the repository root:

```sh
npm ci --prefix compat/clients/node --ignore-scripts
python3 -m venv compat/clients/python/.venv
compat/clients/python/.venv/bin/python -m pip install -r compat/clients/python/requirements.lock
npm --prefix compat/clients/node run check
```

The Node lockfile pins JSforce, Salesforce CLI and its bundled data/deploy-retrieve plugins, plus schema tooling. Python's requirements.lock pins all transitive versions. These are test dependencies, not a Node/Python runtime requirement for the future Go server. `--ignore-scripts` avoids third-party installation hooks; the tested CLI commands work without them.

`check` validates TypeScript, runs synthetic unit tests and validates the profile/contracts, catalog fixtures and OpenAPI documents. It requires no Salesforce credentials and does not connect to Salesforce. Green offline CI means the laboratory passes its own checks, not that the product is compatible.

## Live setup

Use an authorized disposable trial, Developer Edition org or sandbox reserved for these tests. Supply a dedicated administrator/integration user, a restricted user that can read Account but cannot see Compat_Text__c, and a dedicated user whose language/locale can be temporarily changed. Permission sets cannot remove existing access: verify the restricted profile has no field grant from another source. Enable French and the required locale settings.

Inspect `compat/fixtures/salesforce` before deploying. The fixed Compat_Text__c and Compat_Key__c fields must not already contain real business data. Deploy these fixtures once using the pinned local `sf project deploy start --source-dir force-app --target-org <your-disposable-alias> --api-version <explicit-version>` from that fixture project. Assign Compat_Admin to the administrator. Do not assign it to the restricted user. The harness does not create users or alter profiles automatically.

Set credentials in your shell or CI secret store, never in tracked files:

| Variable | Meaning |
| --- | --- |
| COMPAT_API_VERSION | Explicit source-supported version, such as 65.0; example only, no default |
| COMPAT_DISPOSABLE | Must equal `yes`; acknowledges synthetic writes and temporary user settings changes |
| COMPAT_REFERENCE_URL / COMPAT_CANDIDATE_URL | HTTPS org origin; candidate localhost may use HTTP |
| COMPAT_REFERENCE_TOKEN / COMPAT_CANDIDATE_TOKEN | Dedicated administrator access token |
| COMPAT_REFERENCE_RESTRICTED_TOKEN / COMPAT_CANDIDATE_RESTRICTED_TOKEN | Restricted persona token |
| COMPAT_REFERENCE_LOCALE_USER_ID / COMPAT_CANDIDATE_LOCALE_USER_ID | Dedicated user ID for temporary French/en_US/America-New_York settings |
| COMPAT_PYTHON | Optional absolute Python interpreter path; defaults to the repository virtualenv |

These initial probes use access tokens; automatic refresh-flow certification is a later contract. Salesforce CLI authenticates with the same token in a temporary private HOME, verifies the endpoint, and removes that temporary state afterwards. Tokens never appear in command arguments. Both target environments must have equivalent fixtures/personas/settings. Do not run captures concurrently against the same locale user.

```sh
npm --prefix compat/clients/node run capture -- --target reference
npm --prefix compat/clients/node run capture -- --target candidate
npm --prefix compat/clients/node run compare -- --reference /absolute/reference.json --candidate /absolute/candidate.json
npm --prefix compat/clients/node run repeatability -- --first /absolute/reference-1.json --second /absolute/reference-2.json
```

`repeatability` compares two separate reference captures. Differences it reports are nondeterministic in the reference itself; add normalization only for those fields and record why in the contract.

The Python probe needs HTTPS because simple-salesforce always builds `https://` URLs. A local HTTP candidate blocks `python-crud`; serve it over HTTPS with a trusted certificate. `sf org login access-token` also reads `/services/data/v51.0/sobjects/User/{id}` with a hard-coded v51.0, so a candidate must serve that version for the CLI scenarios.

Default results are under ignored `compat/reports/`, with matching JSON and Markdown. `--out /absolute/path.json` chooses another path. Exit codes: 0 passed; 1 failed assertions/differences; 2 blocked/configuration errors. Missing credentials write a blocked report and perform no network calls. Missing persona or Python prerequisites block the relevant scenario rather than skipping it successfully.

Reference and candidate captures use the same scenario code. Live tests do not mock transports or use a SalesCompel SDK. The Python probe is independent of JSforce; CLI plugins may share JSforce and are not counted as an independent implementation.

## Evidence and recovery

Observation follows the [black-box testing policy](../../CLEAN_ROOM.md) and the recorded tenant authorization. All eleven `contracts/pilot.yaml` contracts now link to reviewed live observations from Plan 4; deferred contracts remain not_run. Raw/private journals are ignored. The capture format is version 1: target, profile and SHA-256, API version, connector versions, provenance, start/end timestamps, required scenarios, scenario assertions/observations/identity maps, and cleanup outcomes.

Each scenario receives unique record identifiers. Only those recorded IDs are deleted, child before parent. Temporary locale settings are restored; metadata tests use a random field name, verify absence first, retrieve into a locally cleared file and remove the created field. Field existence and properties are checked with Metadata API `read`, because a newly deployed field has no field-level security and is hidden from `describe`. Fixed setup fields remain for future runs.

Private `compat/private/<target-run>/<scenario>.jsonl` journals record owned IDs immediately for crash recovery. On interruption, inspect these journals and manually remove only the listed synthetic records/field, or restore dedicated user settings. A network failure after a remote create but before an ID arrives can leave an orphan; search by the exact run-specific fixture name/key, inspect it, and remove only confirmed fixtures. The parent process adopts every Account the Python probe journaled, including after a probe crash or timeout, and retries its deletion by ID. The harness never performs broad cleanup queries. Reports retain cleanup failures and block a passing comparison.

Never share private journals/auth files automatically. Reports only select fixture fields, sanitize credential keys and configured secret values, and require human review before publishing. This is not general-purpose anonymization of arbitrary production data.

The comparator rejects non-live evidence, missing scenarios, blocked results, API/profile/client mismatches (connector versions are pinned in `profiles/pilot.yaml`) and failed cleanup. It maps explicitly recorded IDs consistently and compares values without global timestamp/null/error suppression. It does not prove untested behavior or complete Salesforce parity.

## Localization

`locales/languages.json` and separate message files are validation fixtures, not a full Salesforce language catalog or completed product translations. They demonstrate regional codes, legacy `iw` → formatter `he`, right-to-left direction and explicit fallbacks. M0 discovery must expand the version-pinned language inventory. Formatting, browser layout and tenant translations remain future milestone work as recorded in `contracts/localization.yaml`.

## OpenAPI: latest version

`api/openapi.yaml` is SalesCompel's independently authored initial REST contract, written in the latest OpenAPI release (3.2.0). The validator's `salescompel` role rejects any other version. The M2 candidate does not serve it yet (deferred to M3; see [plan-2-implementation.md](plan-2-implementation.md)). When served, it is the source of truth: each operation cites `x-source` URLs from `sources.yaml` and HTTP tests validate traffic against it. The contract is deliberately metadata-driven for records rather than inventing a fixed schema for every organization.

Synthetic JSON/YAML External Services fixtures test standards parsing, not registration or callout execution. M4/M7/M8 own registration, Named/External Credential handling and Flow/Apex actions. Salesforce's supported subset and quirks are recorded separately in `contracts/external-services.yaml`. Salesforce External Services accepts only OpenAPI 2.0 and 3.0, so those fixtures and the official Salesforce document are validated in the `salesforce-source` role (2.0 or 3.0.x only). Such documents are migration/deploy input: SalesCompel converts them to the latest OpenAPI version and never stores, serves or executes them as-is (`external-openapi-conversion`).

An official Salesforce Einstein Bots API 3.0.0 YAML (a `salesforce-source` document) is retained under `compat/fixtures/openapi/salesforce`, with its original BSD-3-Clause license and a pinned commit/checksum. It validates our parser against a real Salesforce document; it is not the Sales Cloud/core REST contract and no requests are sent to its server URLs. The advertised Connect REST core YAML download returned HTTP 404 during research. Core sObjects specs can be generated from an eligible org; that extraction remains pending reference access.

Validate a bundled official or org-generated document locally:

```sh
cd compat/clients/node
npx --no-install tsx src/cli.ts validate-openapi --spec /absolute/spec.yaml
npx --no-install tsx src/cli.ts validate-openapi --role salesforce-source --spec /absolute/salesforce-spec.yaml
```

The default `salescompel` role requires OpenAPI 3.2.x; `salesforce-source` accepts 2.0 or 3.0.x and reminds you to convert. Validation uses `@seriousme/openapi-schema-validator` (official OpenAPI JSON Schemas, 2.0–3.2) plus local checks for unresolved and external references and duplicate `operationId`s.

Remote/file references are disabled; bundle external references before ingestion. No server URL is contacted by schema validation. Keep source URL, API version, retrieval date, checksum, license and any redactions beside future reference artifacts.

## CI recovery and ownership

Offline checks run on hosted Node 22/24 runners. Manual live jobs require a **self-hosted runner labeled `compat-live`** and a protected GitHub environment of the same name. Configure `COMPAT_RECOVERY_DIR` to an owner-only, persistent absolute directory **outside the checkout**. It must survive cancellation, runner restart, and subsequent checkouts. The workflow refuses writes without this directory. Administrators review and recover outstanding journals before another run. Journals are not uploaded as public artifacts; candidate capture reports are retained for seven days. Reference reports stay in the private recovery directory and are not uploaded. Configure environment approval/access controls before supplying credentials.

Local runs can also set `COMPAT_RECOVERY_DIR`; otherwise journals are under ignored `compat/private`. Journal entries include create intent before the network request, known IDs afterwards, and original dedicated-user settings before modification. They contain no credentials.

See [dependency audit](dependency-audit.md) for the known remaining upstream Salesforce CLI toolchain advisories.

## M2 candidate

See [Plan 2 setup, implemented routes and real-client checks](plan-2-implementation.md).
The full pilot suite intentionally retains later-milestone requirements; candidate
smoke checks do not replace reference captures or constitute parity certification.


## Plan 3 reference authorization and evidence

The readiness tooling is implemented; reference evidence is still pending. The
committed `reference-readiness.yaml` intentionally does not authorize any run.
Offline validation accepts a structurally valid pending record. Reference
preflight and capture both reject it before creating clients or journals.

```sh
npm --prefix compat/clients/node run preflight -- --target reference
```

Exit 2 is expected until the maintainer supplies the real decision and setup.
Preflight checks local records/configuration only; it does not contact the org,
verify permissions or demonstrate that credentials work. Candidate preflight
uses its own configuration without requiring a reference authorization.

Maintainer sequence:

1. Identify an authorized disposable org. Pin its actual API version and inventory
   in `profiles/pilot.yaml`, retaining every required scenario and client pin.
2. Use the maintainer authorization already recorded in `implementation-log.md`
   for `velocity-innovation-8559`. For a different environment or expanded scope,
   record its authorization under a dedicated heading. Attribute the test operator;
   the implementer may be the same person/session or null. No extra terms-review
   attestation or role separation is required.
3. Compute `sha256sum docs/compat/profiles/pilot.yaml` from the repository root.
   Fill in `reference-readiness.yaml`: exact profile ID/hash/API, reference alias,
   decision status/maintainer/date/logAnchor, explicit authorization, operator
   attribution and scenario scope. The anchor is the lowercase hyphenated heading,
   without a URL or `#`. Do not put endpoints, org/user IDs or credentials here.
4. Have the authorized observer prepare the existing fixtures and administrator,
   restricted and locale personas, then record the fixture attestations. Supply
   runtime credentials using the existing private environment variables. Do not
   mark an attestation true before the work is done.

The test operator then uses a private directory outside the checkout and
checks outstanding recovery journals before running the following commands:

```sh
npm --prefix compat/clients/node run preflight -- --target reference
npm --prefix compat/clients/node run capture -- --target reference --out /private/reference-1.json
# Inspect cleanup and recover any failed cleanup before continuing.
npm --prefix compat/clients/node run capture -- --target reference --out /private/reference-2.json
npm --prefix compat/clients/node run repeatability -- --first /private/reference-1.json --second /private/reference-2.json --out /private/repeatability.json
npm --prefix compat/clients/node run verify-evidence -- --first /private/reference-1.json --second /private/reference-2.json --out /private/pair.json
```

Replace `/private` with an owner-only directory accessible to the observer. Run
captures sequentially. `verify-evidence` requires a new output filename; it will
not overwrite an existing file. It returns 0 for a valid repeatable pair, 1 for
behavioral differences and 2 for blocked/invalid evidence. It outputs safe
metadata only. The ordinary repeatability report can contain private differences
and must remain private. Neither command creates approved observation notes.

Write independent behavior summaries and have them reviewed for sanitization.
Register notes and reciprocal contract links as described in
[observations/README.md](observations/README.md), then run the offline checks.
The same operator may analyze private captures and implement behavior; published
notes must still undergo sanitization review. Tool validation
checks the attestation and references; it cannot prove the truth of a human
attestation. Do not change profile bytes while registering evidence.

The protected live CI job uses this same preflight and capture gate. Reference
reports go to unique owner-only directories beneath COMPAT_RECOVERY_DIR, survive
checkout and remain private; only candidate reports are uploaded automatically.
No reference secrets, reports or journals belong in public artifacts.

The authorized tenant now has a verified profile and prepared personas; private
credentials remain operator-managed. See the [Plan 4 baseline](plan-4-reference-baseline.md)
for accepted evidence and reproduction details. M0 inventory, M1R release-history review, expanded CLI
runners/proxy and future M3 protocol implementation remain separate obligations.

## Plan 4 pilot baseline

[Two live reference captures](plan-4-reference-baseline.md) passed all eleven
pilot scenarios, repeatability and cleanup verification. Reviewed notes and
reciprocal contract links are registered; deferred contracts remain unobserved.
This completes the pilot baseline, not the full M0/M1 gates or candidate parity.
The next product work package is the evidence-led M3 REST/query foundation;
focused observations and M1R reconciliation precede its contract freeze.

## Plan 5 REST/query foundation suite

Use `--suite rest-query-foundation` consistently with preflight, capture, compare,
repeatability and verify-evidence. Omitting it retains the eleven-scenario pilot.
The separate profile and readiness file must be verified and hash-bound before
reference capture. Existing tenant authorization applies; new fixture attestations
remain false until checked. No focused reference observations are registered yet.

In addition to the ordinary target URL/token, provide the target-specific
`COMPAT_REFERENCE_FRESH_TOKEN` (or `COMPAT_CANDIDATE_FRESH_TOKEN`) for a distinct
session of the same administrator, and the corresponding `*_RESTRICTED_TOKEN`.
The fresh token must differ from the primary token; the cursor probe verifies the
same user and organization. The focused suite does not use the pilot's locale-user
credential. An absolute external `COMPAT_RECOVERY_DIR` is mandatory. Keep all
credentials private; obtain them through documented CLI commands, not chat.

The suite creates roughly 2500 owned Accounts plus small supporting fixtures and
uses public CRUD for setup/cleanup. One cursor is checked after 16 idle minutes;
continued validity at that bound is reported as blocked expiry evidence. Two
complete runs are necessary for repeatability. URL transport rejects redirects
and off-origin continuation links, preserves encoded external IDs, and bounds
responses at 8 MiB with a 15-second request timeout. Query traversal is bounded
at 32 pages. These are probe limits, not claimed Salesforce limits.

Error observations retain status, code, fields and message type without copying
message prose. Describe/discovery probes compare only selected supported-version
and fixture-field dimensions; they do not certify the whole Salesforce catalog.
The null-equality probe requests IDs and retains only membership of owned null
fixtures; unrelated IDs are discarded. Reports and fixture journals remain private.

## UI navigation inventory

[Field notes on the reference UI's apps, navigation items and default list views](ui/navigation-inventory.md)
scope the Vue navigation model. They are a functional inventory, not a design source.

## Plan 6 read-only metadata discovery

`discover-metadata` is an inventory command, separate from compatibility capture
and certification. It uses API 65.0, the foundation profile hash and existing
administrator/restricted tokens; no second administrator session is needed.

Create a mode-0600 JSON config in an existing owner-only directory outside the
checkout. It contains `schemaVersion: 1`, `apiVersion: "65.0"`,
`profile: "rest-query-foundation"`, the SHA-256 of that profile file, and `origin`
(the exact HTTPS origin, without trailing slash). Optional `profiles` and
`permissionSets` arrays contain exact component fullNames, at most 20 each;
wildcards are rejected. Credentials never belong in the config.

Set `COMPAT_REFERENCE_URL`, `COMPAT_REFERENCE_TOKEN` and
`COMPAT_REFERENCE_RESTRICTED_TOKEN` privately. From `compat/clients/node` run:

```sh
node --import tsx src/cli.ts discover-metadata --config "$DISCOVERY_CONFIG" --out "$DISCOVERY_OUTPUT"
```

The output must be a new `.json` file in an existing owner-only directory outside
the checkout. A sibling `.artifacts` directory retains private read JSON and
retrieved XML with digests. Keep both private. The command performs identity,
describe, Metadata read and exact synthetic CLI retrieval operations only. It
never deploys components, changes permissions or extracts records. A native empty
Account XML container is validated but is not counted as a retrieved component.

Exit 0 means the bounded inventory is complete; 2 means input/setup is blocked or
collection is incomplete; 1 reports an internal output failure. Missing properties
are gaps, explicit false values remain facts, and describe visibility does not
establish effective permission enforcement. CLI authentication/project files are
removed after the operation, including failure paths. Existing saved aliases and
Plan 5's session files are not overwritten by this command.

For programmatic review, `compileMetadataMatrix` and
`compareMetadataInventories` are exported from `src/metadata-matrix.ts`. Only
complete, distinct sequential inventories with matching bindings can report
`stable`. A blocked pair may have equal observed facts; that does not close its
missing-evidence gaps. See [Plan 6 results](plan-6-metadata-discovery.md).
