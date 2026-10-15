# Clean-Room Policy

SalesCompel is an independent implementation intended to be compatible with
Salesforce clients, data and customizations. It is not affiliated with,
endorsed by, or derived from Salesforce. Every contribution must follow this
policy.

Naming a folder or process "clean-room" does not establish a clean room. Until
the maintainer assigns separate research and implementation people (or agent
sessions) and controls their access, do not describe any work as
personnel-separated clean-room work.

## Permitted sources

- Salesforce's **public documentation**, release notes, help articles and
  Trailhead content, recorded in `docs/compat/sources.yaml` with URL, access
  date, version and the claim it supports.
- Publicly published API descriptions (WSDLs, OpenAPI documents, protos)
  whose license permits the use, pinned by commit/checksum with their
  license and provenance (see `compat/fixtures/openapi/salesforce/`).
- Open standards (OpenAPI, OAuth 2.0, OpenID Connect, BCP 47, ICU).
- Open-source Salesforce clients (JSforce, Salesforce CLI plugins,
  simple-salesforce, Data Loader) used as **black-box test clients**, run
  unmodified against a reference org or SalesCompel.
- **Firewalled reference-org observation** (see below).
- Customer-owned metadata and code supplied for migration, as input artifacts
  only, never as a source for copying Salesforce internals.

## Firewalled reference-org observation

- **Scope:** documented APIs only, called against a disposable Developer
  Edition org or sandbox the observer is authorized to use, with synthetic
  data, using the compatibility laboratory (`compat/`). No production orgs,
  no other people's orgs, no undocumented or internal endpoints, and no load
  or fuzz testing.
- **Separate roles:** the *observer* runs captures and writes behavioral
  notes. The *implementer* works only from public documentation, contracts in
  `docs/compat/contracts/` and those notes. One person or agent session does
  not do both for the same area.
- **Notes, not copies:** observations describe behavior in the observer's own
  words: status codes, error codes, field names and types, defaults,
  ordering, pagination and permission outcomes. They must not include
  verbatim error messages or documentation prose beyond short identifiers,
  and must not include credentials, org or user identifiers, or personal
  data. Scenario observations record error codes, not messages, for this
  reason; failure reasons in raw capture reports may contain messages and are
  not observation notes.
- **Location:** reviewed notes live in `docs/compat/observations/`, each with
  profile hash, API version, connector versions, capture date and the
  contract IDs it supports.
- **Risk acceptance — open decision:** Salesforce's terms may restrict
  reverse engineering or competitive use of the service. The maintainer must
  review them and record a decision in `docs/compat/implementation-log.md`
  before any live observation runs. No one performs observation until that
  decision exists and the maintainer has authorized them.

## Prohibited

- Using proprietary Salesforce implementation source, or decompiling,
  disassembling or otherwise inspecting Salesforce server software, managed
  package internals or other proprietary assets.
- Capturing Salesforce Setup or Lightning UI pages (screenshots, DOM, assets,
  copy) except as browser observation explicitly scoped by a later approved
  plan, under the same observer/implementer separation.
- Copying Salesforce UI templates, stylesheets, icons, images or product copy.
- Using Salesforce trademarks beyond nominative statements such as
  "Salesforce-compatible".
- Contributing code or knowledge obtained under a Salesforce NDA or from
  employment there that is not public.

## Provenance

- Every behavior contract cites source IDs from `docs/compat/sources.yaml`;
  `npm --prefix compat/clients/node run validate` rejects unknown sources.
- When sources conflict or leave behavior unspecified, keep both claims and
  resolve them by observation against the selected profile; record the
  ruling in `docs/compat/implementation-log.md`.
- Deliberately reused third-party files keep their license and are listed in
  `THIRD_PARTY_NOTICES.md`.
