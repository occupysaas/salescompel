# Observations

No live reference observations exist yet. Synthetic unit fixtures and the official OpenAPI document are not API captures. Reviewed observations must identify profile hash, API version, connector versions, capture time, contract IDs, sanitization and the reproducible fixture setup. Keep raw reports and journals out of version control.

## Reviewed records (Plan 3)

One YAML file per observation uses `<id>.yaml` and the closed
`compat/schemas/observation.schema.json`. Required fields are schemaVersion (1),
id, profile, profileHash (SHA-256 of the unchanged profile bytes), apiVersion,
connectors, contracts, two captures (sha256, startedAt, finishedAt), observer,
review (reviewer, reviewedAt, sanitized: true), repeatability: passed,
cleanup: passed, and notes (one contract/summary pair per claimed contract).
Dates are UTC timestamps. Role names are non-sensitive aliases.

The authorized observer first runs `repeatability` on two private captures and
`verify-evidence --first /private/first.json --second /private/second.json --out
/private/pair.json` through the Node package's npm scripts. Pair verification
writes only digests, times and a safe outcome, using owner-only permissions and
a new filename. It never creates reviewed notes or changes contracts.

After human sanitization review, register the independently written note and
set each covered pilot contract's observationStatus to observed and
observationIds to the note IDs. Every claim must have a reciprocal link. Run
`npm --prefix compat/clients/node run validate`; unknown contracts, stale
bindings, duplicate IDs, missing reviews and incomplete links fail validation.
Deferred contracts remain not_run. This is a checked human attestation, not
cryptographic proof that private captures exist or that the candidate passes.

Do not change the profile while registering evidence, including its historical
referenceStatus field: that would change the raw-byte hash used in the captures.
The validator's observed/not_run counts are the current acceptance summary.
Any later profile revision needs newly bound authorization and captures.

Summaries must not contain credentials, org/user IDs, personal data or copied
messages. Automated checks catch some obvious secrets/ID-shaped values only;
they do not replace the reviewer's inspection. Keep raw reports and journals
private. No synthetic test record belongs in this directory.

Capture dates cannot precede the readiness decision's UTC date. Because the
maintainer record contains a date rather than a timestamp, the observer must
also verify that authorization preceded capture within that day. Obvious secret
patterns are checked without regard to capitalization; identifier detection is
heuristic and deliberately does not reject ordinary words solely for their length.
