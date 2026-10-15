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

## Live setup (pending a disposable organization)

Use a Developer Edition/sandbox reserved for these tests. Supply a dedicated administrator/integration user, a restricted user that can read Account but cannot see Compat_Text__c, and a dedicated user whose language/locale can be temporarily changed. Permission sets cannot remove existing access: verify the restricted profile has no field grant from another source. Enable French and the required locale settings.

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

Observation follows the [clean-room policy](../../CLEAN_ROOM.md); its risk decision is a prerequisite for the first live run. Contracts in `contracts/pilot.yaml` describe expected behavior but are marked not_run until independently reviewed live observations are committed. Raw/private journals are ignored. The capture format is version 1: target, profile and SHA-256, API version, connector versions, provenance, start/end timestamps, required scenarios, scenario assertions/observations/identity maps, and cleanup outcomes.

Each scenario receives unique record identifiers. Only those recorded IDs are deleted, child before parent. Temporary locale settings are restored; metadata tests use a random field name, verify absence first, retrieve into a locally cleared file and remove the created field. Field existence and properties are checked with Metadata API `read`, because a newly deployed field has no field-level security and is hidden from `describe`. Fixed setup fields remain for future runs.

Private `compat/private/<target-run>/<scenario>.jsonl` journals record owned IDs immediately for crash recovery. On interruption, inspect these journals and manually remove only the listed synthetic records/field, or restore dedicated user settings. A network failure after a remote create but before an ID arrives can leave an orphan; search by the exact run-specific fixture name/key, inspect it, and remove only confirmed fixtures. The parent process adopts every Account the Python probe journaled, including after a probe crash or timeout, and retries its deletion by ID. The harness never performs broad cleanup queries. Reports retain cleanup failures and block a passing comparison.

Never share private journals/auth files automatically. Reports only select fixture fields, sanitize credential keys and configured secret values, and require human review before publishing. This is not general-purpose anonymization of arbitrary production data.

The comparator rejects non-live evidence, missing scenarios, blocked results, API/profile/client mismatches (connector versions are pinned in `profiles/pilot.yaml`) and failed cleanup. It maps explicitly recorded IDs consistently and compares values without global timestamp/null/error suppression. It does not prove untested behavior or complete Salesforce parity.

## Localization

`locales/languages.json` and separate message files are validation fixtures, not a full Salesforce language catalog or completed product translations. They demonstrate regional codes, legacy `iw` → formatter `he`, right-to-left direction and explicit fallbacks. M0 discovery must expand the version-pinned language inventory. Formatting, browser layout and tenant translations remain future milestone work as recorded in `contracts/localization.yaml`.

## OpenAPI 3.0 only

`api/openapi.yaml` is SalesCompel's independently authored initial REST contract. The M2 candidate does not serve it yet (deferred to M3; see [plan-2-implementation.md](plan-2-implementation.md)). When served, it is the source of truth: each operation cites `x-source` URLs from `sources.yaml` and HTTP tests validate traffic against it. The contract is deliberately metadata-driven for records rather than inventing a fixed schema for every organization.

Synthetic JSON/YAML External Services fixtures test standards parsing, not registration or callout execution. M4/M7/M8 own registration, Named/External Credential handling and Flow/Apex actions. Salesforce's supported subset and quirks are recorded separately in `contracts/external-services.yaml`. OpenAPI 2.0 and 3.1+ are rejected as input. Salesforce-side 2.0 documents (for example existing External Service registrations) are converted to OpenAPI 3.0 during migration, never registered as 2.0 (`external-openapi2-conversion`).

An official Salesforce Einstein Bots API 3.0.0 YAML is retained under `compat/fixtures/openapi/salesforce`, with its original BSD-3-Clause license and a pinned commit/checksum. It validates our parser against a real Salesforce document; it is not the Sales Cloud/core REST contract and no requests are sent to its server URLs. The advertised Connect REST core YAML download returned HTTP 404 during research. Core sObjects specs can be generated from an eligible org; that extraction remains pending reference access.

Validate a bundled official or org-generated document locally:

```sh
cd compat/clients/node
npx --no-install tsx src/cli.ts validate-openapi --spec /absolute/spec.yaml
```

Remote/file references are disabled; bundle external references before ingestion. No server URL is contacted by schema validation. Keep source URL, API version, retrieval date, checksum, license and any redactions beside future reference artifacts.

## CI recovery and ownership

Offline checks run on hosted Node 22/24 runners. Manual live jobs require a **self-hosted runner labeled `compat-live`** and a protected GitHub environment of the same name. Configure `COMPAT_RECOVERY_DIR` to an owner-only, persistent absolute directory **outside the checkout**. It must survive cancellation, runner restart, and subsequent checkouts. The workflow refuses writes without this directory. Administrators review and recover outstanding journals before another run. Journals are not uploaded as public artifacts; sanitized capture reports are retained for seven days. Configure environment approval/access controls before supplying credentials.

Local runs can also set `COMPAT_RECOVERY_DIR`; otherwise journals are under ignored `compat/private`. Journal entries include create intent before the network request, known IDs afterwards, and original dedicated-user settings before modification. They contain no credentials.

See [dependency audit](dependency-audit.md) for the known remaining upstream Salesforce CLI toolchain advisories.

## M2 candidate

See [Plan 2 setup, implemented routes and real-client checks](plan-2-implementation.md).
The full pilot suite intentionally retains later-milestone requirements; candidate
smoke checks do not replace reference captures or constitute parity certification.
