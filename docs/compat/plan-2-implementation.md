# Plan 2 candidate platform

Implementation and verification record for the [plan](../superpowers/plans/2026-10-15-plan-2-platform-walking-slice.md)
and [design](../superpowers/specs/2026-10-15-plan-2-platform-design.md).

This is an M2 candidate walking slice. M1 live reference observations remain
pending, so the M2 roadmap exit gate is not certified. No Salesforce org was
accessed. Product translations are drafts requiring human review.

## Local setup

Requires Go 1.25+, Node 22.12+, PostgreSQL 17, Python 3.12+, OpenSSL and curl.

```sh
docker run -d --name salescompel-db -e POSTGRES_PASSWORD=local-only \
  -e POSTGRES_DB=salescompel -p 127.0.0.1:55432:5432 postgres:17
export DATABASE_URL='postgres://postgres:local-only@127.0.0.1:55432/salescompel?sslmode=disable'
npm ci --prefix web --ignore-scripts
npm --prefix web run build
go build -o /tmp/salescompel ./cmd/salescompel
```

Provision each token/principal from environment variables; credentials are never
command-line flags or database plaintext. Generate at least 32 random bytes for
real deployments. Provisioning is an operator command, not a public endpoint.

```sh
export SALESCOMPEL_TENANT=00D000000000001EAA
export SALESCOMPEL_USER=005000000000001AAA
export SALESCOMPEL_ROLE=admin
export SALESCOMPEL_TOKEN="$SALESCOMPEL_TENANT!$(openssl rand -hex 32)"
/tmp/salescompel provision
/tmp/salescompel serve
```

Visit `http://127.0.0.1:8080` and enter the token. `restricted` is the second
supported role; it cannot read, write, select, filter or order by
Account.Compat_Text__c. This is a small synthetic policy, not Salesforce sharing.
To revoke a provisioned token, delete its digest row through an authorized DB
operator. Token lifecycle, public login and OAuth refresh are future work.

Use `serve -tls-cert cert.pem -tls-key key.pem` for HTTPS. Non-loopback listeners
require TLS. For local test certificates, trust the CA via NODE_EXTRA_CA_CERTS
and REQUESTS_CA_BUNDLE. The Python client requires HTTPS. Never disable TLS
verification for connector tests. The browser test alone accepts the ephemeral
self-signed certificate. Secrets are not stored in browser storage.

## Implemented surface

Authenticated discovery: `/services/data`, `/services/data/v65.0`,
`/services/oauth2/userinfo`; own-user read through v65.0 and the CLI's v51.0 path.
Account and Contact: global describe, object describe, POST create, GET read,
PATCH update and DELETE. Account PATCH upsert via `Compat_Key__c/{value}`.
`/services/data/v65.0/query` supports explicit field lists, one equality filter
(string or null), one ordering field and LIMIT 1..2000. Contact.Account.Name is
the only relationship projection. Unknown fields and denied fields fail closed.
Queries without LIMIT that exceed 2000 rows return an error; there are no cursors.
Generated IDs are stable 18-character IDs with case checksum. Null is preserved;
omitted PATCH fields remain unchanged. A blank string in an optional field is stored
as null, so blank external IDs never collide and match `= null`. Deleting a referenced Account is rejected.

The broader `api/openapi.yaml` remains a proposed contract and is **not served** by
the candidate, although the roadmap assigned serving it to M2; this is deferred to
M3, together with per-operation `x-source` provenance and validating HTTP test
traffic against the document. Composite, collections,
Metadata/SOAP, Bulk, pagination, user-setting writes, full SOQL, dynamic schema,
Migration and production identity are **not implemented**. Unsupported routes
return errors. Do not run the full pilot capture expecting every M3/M4 scenario
to pass, and do not narrow the pilot profile to manufacture a pass.

Typed PostgreSQL tables use tenant-scoped primary/unique keys and a composite
Contact→Account foreign key. Every query binds the authenticated tenant. The
fixed catalog is persisted with a schema version and checked on startup. There
is no database RLS policy yet; application isolation is tested but DB operators
remain privileged. Tokens are SHA-256 digests. The server has request size and
time limits and returns sanitized storage errors. Production hardening, public
identity, token rotation and comprehensive audit logging remain future work.

## Checks

```sh
TEST_DATABASE_URL="$DATABASE_URL" go test -race ./...
go vet ./...
npm --prefix compat/clients/node run check
npm --prefix web test
npm --prefix web run build
python3 -m venv compat/clients/python/.venv
compat/clients/python/.venv/bin/pip install -r compat/clients/python/requirements.lock
npm --prefix web exec -- playwright install chromium
CANDIDATE_DISPOSABLE=yes scripts/check-candidate.sh
```

The script requires an explicitly disposable DB, creates unique synthetic tenants,
starts a temporary HTTPS service on port 58443, runs real JSforce/Python/CLI calls,
checks process restart persistence and runs Chromium UI tests. CANDIDATE_PORT can
change the port; CANDIDATE_PYTHON can select an installed Python. CHROMIUM_PATH is
an optional local browser executable override. No HTTP connector mocks are used.
Successful fixture rows are deleted by ID, but provisioned test tenants/principal
digests remain in the disposable DB; discard that DB after testing. Failed runs
retain their temporary directory (including short-lived test credentials) for
local diagnosis. Do not upload it as public evidence. Go integration tests skip
locally without TEST_DATABASE_URL and fail in CI if it is absent.

## Release-history follow-up

The roadmap now includes M1R: complete inventory/review of every release in the
[Salesforce previous-release archive](https://help.salesforce.com/s/articleView?id=release-notes.rn_previous_release_notes.htm)
and linked changelogs, with contract/regression-test mappings and explicit gaps.
This future milestone was requested during Plan 2; the archive was not reviewed.

## Whole-change review (2026-10-15)

Fixed, each with a regression test:

- Blank strings in nillable fields were stored as `''`: two Accounts with
  `Compat_Key__c: ""` collided on the unique key, `AccountId: ""` failed as a
  bad reference, and blank values did not match `WHERE … = null`.
- Invalid UTF-8 or NUL in a query literal or a record/external-ID path segment
  reached PostgreSQL and returned 500 `SERVER_UNAVAILABLE`; both now return 400.
- The asset server generated directory listings (for example `/assets/`).

Checked and found sound: tenant binding in every read/write/join/upsert lock,
composite Contact→Account foreign key, restricted-field enforcement in describe,
read, write, filter and order, atomic validation before writes, disjoint
concurrent updates and serialized upserts, bearer-only auth (no cookies, so no
CSRF surface), sanitized storage errors, in-memory UI token and stale-state
handling after failed refreshes.

Deferred: candidate behavior that differs or may differ from Salesforce and
needs M1 reference evidence before changing (none is claimed as parity). Each
row has a deferred contract in [rest-semantics.yaml](contracts/rest-semantics.yaml):

| Area | Candidate behavior | Expected reference question | Contract |
|---|---|---|---|
| String equality | `WHERE Name = 'acme'` is case-sensitive | SOQL text comparison is generally case-insensitive | `case-insensitive-comparison` |
| Identifiers | Object/field names are case-sensitive | SOQL and REST names are case-insensitive | `case-insensitive-names` |
| Null ordering | `NULLS LAST` for both ASC and DESC | SOQL is believed to default to nulls first; if so, ASC is wrong, not merely unconfirmed | `null-ordering` |
| Record IDs | Only the 18-character form is accepted | Salesforce also accepts the 15-character form | `id-15-18` |
| Account delete | Rejected while Contacts reference it | Salesforce deletes related Contacts and other children, but blocks deletion under some conditions (for example Contacts linked to cases or portal users) | `account-delete-cascade` |
| Unknown sObject | 400 `INVALID_TYPE` | REST `/sobjects/<unknown>` is likely 404 `NOT_FOUND`, while SOQL `FROM <unknown>` is 400 `INVALID_TYPE` | `unknown-sobject` |
| Oversized body | 400 `JSON_PARSER_ERROR` above 1 MiB | Confirm limit and error contract | `request-size-limit` |
| LIMIT above 2000 | Rejected | Valid SOQL; REST pages results with `nextRecordsUrl` | `query-limit-above-batch` |
| UI list | Shows the first 2000 rows (explicit `LIMIT`) with no truncation notice | Pagination arrives in M3 | `query-response-shape` |
| Served OpenAPI | `api/openapi.yaml` not served | Roadmap assigned it to M2; now M3 | — |

## Verification record

Run 2026-10-15 on the `plan-2-platform-walking-slice` branch, Go 1.27.1, Node
26.8.1, Python 3.14.7, PostgreSQL 17 (disposable container), local Chromium via
`CHROMIUM_PATH`. Results after the review fixes:

| Command | Result |
|---|---|
| `TEST_DATABASE_URL=… go test -race -count=1 ./...` | pass (no skips) |
| `go vet ./...`, `gofmt -l` | clean |
| `npm --prefix compat/clients/node run check` | 29 pass, 1 skipped (opt-in candidate test), validate OK |
| `npm --prefix web test` | 1 pass |
| `npm --prefix web run build` | vue-tsc and Vite build OK |
| `CANDIDATE_DISPOSABLE=yes scripts/check-candidate.sh` | JSforce, simple-salesforce HTTPS CRUD, restart persistence, `sf data query` and 4 Chromium tests pass |

The work was later committed on `main` (per the main-only branch policy in the
roadmap). The GitHub Actions `platform.yml` workflow has not yet been observed
running. No Salesforce org was contacted.
