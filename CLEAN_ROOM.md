# Black-Box Testing Policy

SalesCompel is an independent implementation intended to be compatible with
Salesforce clients, data and customizations. Compatibility work uses black-box
testing: exercise public interfaces and independently implement the externally
observable behavior. Do not inspect or derive implementation from proprietary
Salesforce internals.

The filename `CLEAN_ROOM.md` is retained for existing links. This project does
not require a personnel-separated clean-room process.

## Testing scope

- Use Salesforce organizations the maintainer owns or is authorized to test.
  Disposable trial organizations, Developer Edition organizations and sandboxes
  are all valid reference environments; edition alone is not a testing blocker.
- Use synthetic data and public, documented APIs through unmodified clients
  such as JSforce, Salesforce CLI and simple-salesforce.
- Keep tests within the maintainer's authorized scope. Track created fixtures,
  restore temporary settings and clean up test data. Do not run destructive
  tests against production data or access someone else's organization.
- The same person or agent session may run reference tests, analyze results and
  implement compatible behavior. A separate observer or reviewer is optional.
- The maintainer's authorization to test the named environment is sufficient
  project approval. This policy does not impose an additional terms-review
  attestation or separate approval gate for each test run.

## Evidence and implementation

- Describe observed behavior independently: inputs, status/error codes, field
  types, defaults, ordering, pagination, permissions and committed effects.
- Record the profile, API/client versions, capture dates, contract IDs and
  reproduction steps. Distinguish observed behavior from assumptions and
  synthetic tests; do not claim compatibility without supporting evidence.
- Keep credentials, raw authentication material and sensitive captures private.
  Remove credentials, personal data and organization/user identifiers from
  published notes and reports.
- Public documentation, standards and appropriately licensed API descriptions
  may inform contracts. Record source provenance and preserve required licenses
  and notices for reused third-party material.

## Excluded methods

- Do not use proprietary Salesforce implementation source, decompile server
  software, or inspect managed-package internals or other proprietary assets.
- Do not copy Salesforce UI templates, stylesheets, icons, images or product
  prose into SalesCompel. Independently implement behavior rather than copying
  the reference product's implementation or assets.
- Do not use confidential or NDA-covered information as an implementation source.

This policy supersedes conflicting clean-room process requirements in older
plans and runbooks, including mandatory role separation, edition restrictions
and additional terms-review approval gates. Existing fixture, credential,
cleanup and evidence-integrity checks still apply.
