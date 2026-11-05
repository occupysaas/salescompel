# Plan 26 — OAuth login and identity

Implementation and candidate decisions; no claim of reference-platform parity.
Status: implementation in progress, 2026-11-04.

## Client observations

On 2026-11-04, unmodified `@salesforce/cli` 2.156.0 (Node 26.8.1,
Linux) ran against a disposable local HTTP mock. Playwright 1.56.1 drove
Chromium at `/usr/bin/chromium`, followed the CLI callback's relative redirect
and reached `/OauthSuccess`. Both `sf org login web --instance-url <mock>
--json` and `sf org login jwt --instance-url <mock> --username <synthetic>
--client-id <synthetic> --jwt-key-file <private-scratch-file> --json` exited 0.
The mock and its generated key, credentials and auth files are not committed.
The child process used isolated HOME and SF_STATE_FOLDER directories, removed
afterward. Only externally visible traffic and commands were inspected.

| Item | Observed value |
| --- | --- |
| Web client ID | `PlatformCLI` |
| Redirect URI | `http://localhost:1717/OauthRedirect` |
| Requested scopes | `refresh_token api web` |
| PKCE challenge method | Absent; SHA256(verifier) equals the 43-character base64url challenge |
| PKCE verifier length | 171 characters |
| Browser completion | `/OauthSuccess`, CLI exit 0 |
| JWT algorithm | `RS256` |
| JWT audience | Mock login URL with trailing slash |
| JWT expiry / issue time | Millisecond `exp`, seconds `iat`; exp minus iat×1000 was 465 ms in the final repeat |
| JWT completion | CLI exit 0 |
| Browser launcher on this KDE system | `kde-open` |

Reproduction: start a loopback mock implementing authorize→callback, token,
userinfo, User discovery, API versions and the bounded Organization projection;
prepend launcher stand-ins to the CLI child PATH, then open the captured URL
in Playwright Chromium. Capture only parameter shapes and lengths. On KDE,
include `kde-open`, `kde-open5`, `kde-open6`, `kioclient`, `kioclient5`,
`kioclient6` and `gio` alongside `xdg-open`, `open`, `google-chrome`, `chromium`,
`firefox`, `sensible-browser` and `x-www-browser`; the URL is the final argument.

These observations prove client behavior against a mock, not Salesforce behavior.
No maintainer trial-org session was supplied or used. Documented reference rows
remain unverified against a live org; historical `observationStatus: not_run`
is unchanged.

## Changelog review and sources

Reviewed official Salesforce sources on 2026-11-04:

- [Spring '26 connected-app creation changes](https://help.salesforce.com/s/articleView?id=005228017&language=en_US&type=1): new creation is restricted in favor of External Client Apps.
- [OAuth security settings](https://help.salesforce.com/s/articleView?id=005433465&language=en_US&type=1): PKCE, refresh rotation and a 30-day idle refresh TTL cannot be disabled for locally managed apps created on or after July 9, 2026. Candidate non-rotating refresh tokens remain a deliberate compatibility gap.
- [Identity and access release notes](https://help.salesforce.com/s/articleView?id=release-notes.rn_security_auth_and_identity.htm&language=en_US&type=5): device-flow restrictions remain outside this plan.
- [REST authorization guide](https://developer.salesforce.com/docs/platform/api-rest/guide/intro-oauth-and-connected-apps.html): External Client Apps retain the OAuth endpoint model.

Help pages were reviewed through indexed excerpts; no live-org claim is made.

## Verification so far

Baseline `TEST_DATABASE_URL=<disposable PostgreSQL 17> GOTOOLCHAIN=go1.25.0
go test ./... -count=1` passed all packages, with no database skips.

## Decisions and ownership

| Contract ID | Decision and basis | Implementation / verification | Owner |
| --- | --- | --- | --- |
| `oauth-token-endpoint` | Authorization code with S256 PKCE, refresh, RS256 JWT bearer and client credentials. Device and password grants are unsupported. Success shape Documented; CLI input exceptions Client-observed; timeouts and non-rotation Default. | `oauth_secrets.go`, `oauth_jwt.go`, `oauth_tokens.go`, `oauth_http.go`; primitives, token store and HTTP grant tests; required `oauth` client gate | 26; future app policies 33/35 |
| `oauth-authorize-revoke-introspect` | Server-rendered login/consent, revocation, same-client introspection, incomplete OAuth discovery. Endpoint shapes Documented; CSRF, replay and exact error behavior Default. | `oauth_page.go`, `oauth_http.go`; authorization/forgery/replay/identity tests and browser gate | 26; complete OIDC deferred |
| `login-hosts` | One explicit public URL. CLI and JSforce change their login URL; simple-salesforce receives a session token. No production/sandbox/My Domain routing parity claimed. | `ResolvePublicURL`; required CLI, JSforce and Python-session gate coverage | 26 bounded host; after-release roadmap §5 item 12 routing proxy |
| `soap-login` | SOAP login is not implemented. | No runtime claim or reference observation | After-release roadmap §5 item 7 |
| `connected-app-policy` | CLI-provisioned apps, exact redirect allowlist, scope/grant enforcement, JWT pre-authorization and run-as user. No metadata import or Salesforce policy equivalence. | `oauth_store.go`, `admin.go`; app validation and policy tests | 26 bounded administration; 33 profiles, 35 metadata, 45 policy |
| `saml-sso` | SAML login and federation mapping remain deferred. | No runtime claim or reference observation | Identity backlog; no first-release plan assigned |

**simple-salesforce does not use its own login in this plan.** Its pinned
client constructs a Salesforce host for OAuth login. It receives the candidate
OAuth token as `session_id` alongside `instance_url`, then proves CRUD through
its unmodified client. A routing proxy needs a separate TLS/trust design.

All Default choices remain reference gaps: global case-insensitive ASCII
usernames, Argon2 parameters, 2-hour access expiry, 10-minute authorization and
pending-request expiry, absolute expiry/replay handling, 10-attempt/15-minute
lockout, public-client PKCE rules (including the observed CLI exceptions),
hashed once-shown client secrets, explicit JWT users, same-client introspection,
scope-to-route mapping, constant PlatformCLI availability and exact error codes,
statuses and independently written descriptions. Documented rows have no live-org
confirmation, including signature computation and JWT expiration policy.
Client-observed behavior is what clients send; its acceptance by Salesforce is
not established by the local mock or candidate tests.

Additional gaps: ID tokens/JWKS/full OIDC discovery; MFA; browser sessions;
session settings, login IP ranges and login history; refresh rotation, idle
refresh expiry and per-app token policies; disabling PlatformCLI; OAuth CORS;
GET revoke; custom-scheme callbacks; dynamic User data and broad Organization
SOQL. Provisioned tokens return the tenant string as `organization_id`, while
OAuth tokens return the stable derived `00D` ID. The User and Organization
responses are narrow client bridges. Historical identity contracts stay
byte-identical with `observationStatus: not_run`.

## Operator guide

Set `DATABASE_URL` to the PostgreSQL database. Before upgrade, stop old instances
and back up the database. Start the new binary; startup adds eight identity and
OAuth tables inside the existing advisory-locked transaction. Catalog version
remains 3; no existing catalog data or provisioned token is rewritten. Plan 27
owns adopting the tables into migrations and row-level security. To roll back,
stop the new binary, restore the pre-upgrade backup, then start old code.

`serve --public-url https://crm.example.com --access-token-ttl 2h` fixes all
issuer, audience, identity, instance and form-origin URLs to the configured
origin; forwarding headers and Host do not select it. The URL must be HTTPS,
except HTTP is accepted for loopback. No paths, queries or fragments are
permitted. Omitting the flag derives an origin only for a loopback listener.
Non-loopback listeners additionally require the existing TLS certificate/key.
The OAuth page works with `--assets ''`. The application editor still uses its
token field until Plan 36.

Use a private operator environment for secrets; do not put passwords in shell
history or command flags. `salescompel user create` takes `SALESCOMPEL_TENANT`,
`SALESCOMPEL_USERNAME`, `SALESCOMPEL_PASSWORD` and `SALESCOMPEL_ROLE` (`admin` or
`restricted`). Passwords are 12 characters minimum and 1,024 bytes maximum.
Usernames are ASCII, contain exactly one `@`, have at most 80 characters and
are unique across tenants ignoring case. The command prints the generated user
ID. `user set-password` takes username/password, clears lockout and revokes the
user's tokens. `user deactivate` takes username and revokes tokens.

`salescompel oauth-app create` accepts `--tenant`, `--name`, repeatable
`--redirect-uri`, comma-separated `--grants` and `--scopes`, `--public`,
`--jwt-cert`, repeatable `--jwt-user`, and `--run-as`. Redirects match exactly
and must be HTTPS or HTTP on localhost, 127.0.0.1 or ::1. A confidential app
prints JSON containing `client_id` and a once-shown `client_secret`; retain it
privately. Public apps omit the secret and cannot use client credentials.
JWT apps need a current RSA certificate (at least 2,048 bits) and explicit
same-tenant permitted usernames. Client-credentials apps need an active
same-tenant run-as user.

`oauth-app rotate-secret --client-id <id>` shows the replacement once and
invalidates the old secret while preserving existing tokens. `oauth-app disable
--client-id <id>` revokes the app's tokens. Never publish command outputs
containing a secret. Only hashes of passwords and high-entropy credentials
are stored; no server encryption key or recoverable client secret exists.
