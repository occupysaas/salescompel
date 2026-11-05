# Plan 26 — OAuth login and identity design

- **Status:** Draft for maintainer review, 2026-11-04.
- **Roadmap:** Plan 26, stage A, of the [first release roadmap](2026-11-04-salescompel-first-release-roadmap-design.md). Needs Plan 25, which is implemented.
- **Decides, as the roadmap asks:** the login UI, how secrets are stored, and which flows the first release needs.

## Why

Clients can only reach the candidate today with a token someone provisioned by hand. A real user cannot log in, an integration cannot authenticate itself, and a token cannot expire or be revoked. Every later plan assumes real identity: Plan 33 adds permissions to these users and Plan 36 adds sign-in to the application.

This plan lets unmodified standard clients log in by changing only the login URL.

## Goals

- `sf org login web` and `sf org login jwt` succeed with `--instance-url`, unmodified, and later commands stay on SalesCompel.
- JSforce completes web-server login with PKCE, refresh, JWT bearer, client credentials and revoke.
- OAuth-issued tokens work in simple-salesforce, within the limit described under [Clients](#clients).
- Tokens expire and can be revoked, and revocation takes effect on the next API call.
- OAuth users carry the existing tenant and role, so existing isolation and field policy apply to them unchanged.
- No plaintext password, token or client secret is stored or logged.

## Non-goals

- **SAML, device flow, My Domain and sandbox hosts:** later, as the roadmap says.
- **Username-password and user-agent flows, SOAP `login()`:** not in the first release. Requesting the password grant returns `unsupported_grant_type`.
- **ID tokens and `openid`:** not in this plan. They need a signing key at rest, and no first-release client needs them. See [Discovery](#discovery).
- **Users as records, profiles, permission sets:** Plan 33. This plan adds only what logging in needs. There is no `User` object in describe or SOQL.
- **Application sign-in:** Plan 36. The connected editor keeps its token field until then.
- **Connected App metadata import and policies:** the `connected-app-policy` contract stays with Plans 35 and 45.
- **General rate limiting:** Plan 48. This plan adds only a lockout for password attempts.
- **MFA, IP ranges, session settings, CORS on OAuth endpoints:** listed as gaps.

## Evidence

| Label | Meaning |
| --- | --- |
| **Client-observed** | Captured 2026-11-04 from unmodified `@salesforce/cli` 2.156.0 against a local mock, from network traffic only. No org was involved. This shows what the client sends, not how Salesforce responds. |
| **Documented** | Salesforce documentation, as indexed by search on 2026-11-04. The help pages did not render for direct reading, so Task 1 re-checks each row where a session is available. |
| **Default** | A candidate choice with no reference evidence. Always listed as a gap. |

### What the CLI sends

These facts drove the design. **The first three would fail on a server that follows the RFCs strictly**, so each gets a regression test.

| Behavior | Observed value | Consequence |
| --- | --- | --- |
| JWT `exp` | `1793845797945`, beside `iat` `1793845797`. It is millisecond-scale and within a second of `iat`. | Read as seconds, it is tens of thousands of years away. Read as milliseconds, it is already expired. Neither reading works alone. See [JWT bearer](#jwt-bearer). |
| PKCE method | `code_challenge` is an S256 hash, and `code_challenge_method` is absent. | RFC 7636 would read an absent method as `plain`. Here, absent means S256. |
| PKCE verifier | 171 characters. | RFC 7636 allows at most 128. Accept 43–256. |
| JWT `aud` | The login URL with a trailing slash, for example `http://127.0.0.1:39471/`. | Accept the public URL with or without the trailing slash. |
| Default client | `client_id=PlatformCLI`, no secret, `redirect_uri=http://localhost:1717/OauthRedirect`, `scope=refresh_token api web`, `prompt=login`. | A built-in public client must exist. See [Apps](#apps). |
| Calls after web login | `/services/oauth2/userinfo`, `/services/data/v51.0/sobjects/User/{user_id}`, `/services/data/v51.0/query` (`ScratchOrgInfo`), `/services/data`, a v65.0 `Organization` query. | With the v51 query returning 404 and the `Organization` query returning 400, `sf org login jwt` exited 0 and web login saved its auth file. **The web command itself was never seen to exit cleanly:** the capture's stand-in browser did not follow the CLI's final relative redirect to `/OauthSuccess`, so the run ended at its timeout. Task 1 settles this with a real browser. |
| `sf org display` | Reports the `Organization` query's error as `connectedStatus`. | Serve that one query so status is honest. See [Identity endpoints](#identity-endpoints). |

### Other clients

- **JSforce 3.11.0** parses `id` by taking its last two path segments as organization and user, so `id` must end in `/{org}/{user}`. Its `useVerifier` option sends PKCE, and `requestToken` accepts arbitrary grant parameters.
- **simple-salesforce 1.12.9** builds every OAuth login URL as `https://{domain}.salesforce.com/services/oauth2/token`. It cannot log in against another host without a routing proxy, but it can use `instance_url` and `session_id`.

### Changelog review

Connected App creation is reported as disabled by default from Spring '26, in favor of External Client Apps. Both use the same OAuth endpoints, so this plan's protocol is unaffected. Third-party sources report that the username-password flow is being retired and the device flow restricted. These are unconfirmed, but they support leaving both out.

## Decisions

### Flows

| Flow | First release | Basis |
| --- | --- | --- |
| Authorization code, with or without a secret, with PKCE | Yes | Documented; roadmap |
| Refresh token | Yes | Documented; roadmap |
| JWT bearer | Yes | Documented; roadmap |
| Client credentials | Yes | Documented; roadmap |
| Revoke, introspect, discovery, userinfo, identity URL | Yes | Documented; roadmap |
| Password, device, user-agent, hybrid, SAML bearer, asset token | No: `unsupported_grant_type` or `unsupported_response_type` | Default |

### Login UI

**A server-rendered page at `/services/oauth2/authorize`, built with Go `html/template`, separate from the Vue app.** It works when `--assets` is empty, needs no JavaScript, and stays inside the existing Content-Security-Policy, with its stylesheet served from an embedded route. The design is original, per `CLEAN_ROOM.md`.

One form covers login and consent. It shows the app's name, what the requested scopes allow, username, password, and **Allow** and **Deny** buttons. **Deny** redirects with `error=access_denied`.

**There is no browser session in this plan.** Every authorization shows the form, so `prompt=login` and `prompt=consent` are always satisfied, and `prompt=none` returns `login_required`. Plan 36 owns browser sessions.

**Localization.** Text comes from `locales/messages`, chosen from `Accept-Language` among the five catalogs, falling back to `en_US`. Hebrew renders `dir="rtl"`.

**Request protection:**

- `GET` validates the request and stores it server-side as a pending request (10 minutes, single use). The form carries only an opaque ID, so the client, redirect URI, scope and challenge cannot be altered in the browser.
- The ID is bound to an `HttpOnly`, `SameSite=Strict` cookie, marked `Secure` when the public URL is HTTPS.
- `POST` rejects a request whose `Origin` is present and differs from the public URL, or whose `Sec-Fetch-Site` is present and is not `same-origin`. `Origin: null` is rejected.
- The form page sets `Referrer-Policy: same-origin` in place of the global `no-referrer`. With `no-referrer`, browsers send `Origin: null` on the form's POST, and every login would fail. This does not leak the code to the client, whose redirect is cross-origin and so carries no referrer. The change applies to this page only.
- An unknown client or unregistered redirect URI shows an error page and never redirects. Other errors redirect with `error` and the original `state`.

**Password attempts.** Ten consecutive failures lock the username for 15 minutes (Default). The message is the same for an unknown user, a wrong password, a locked user, an inactive user and a user outside the app's tenant. An unknown username still performs a hash, so timing does not reveal it. At most four password hashes run at once; further attempts wait, bounded by the request's existing timeout.

### Secrets

| Secret | Stored as | Why |
| --- | --- | --- |
| User password | Argon2id: m=19456 KiB, t=2, p=1, salted, PHC string | Low entropy needs a slow hash. Same algorithm as zeroauthy. |
| Access token, refresh token, authorization code, pending request ID, client secret | SHA-256 hex | Each has 256 random bits, so a fast hash is enough. This is the existing `principals` pattern. |
| JWT certificate | PEM, in the clear | Public material |

**There is no encryption key to configure, and nothing the server stores can be used as a credential.** The cost is that a client secret is shown once and cannot be retrieved, unlike Salesforce (Default). Rotation replaces it.

The token response's `signature` field is an HMAC-SHA256 of `id` plus `issued_at`, keyed with the client secret (Documented, recollected; Task 1 confirms). The server computes it from the secret presented in that request, and omits the field when none was presented, as with JWT bearer and public clients (Default). The CLI accepted an arbitrary `signature` value from the capture's mock; JSforce and simple-salesforce are not expected to check it, and the gate will show it.

Credentials never reach logs, error messages, the acceptance report or evidence files. Provisioning takes passwords from `SALESCOMPEL_PASSWORD`, never from a flag, matching the existing `SALESCOMPEL_TOKEN` pattern.

### Identity

**Users.** A new `users` table holds a generated 18-character ID with prefix `005`, the tenant, a username, a password hash, the existing `admin` or `restricted` role, and an active flag.

**Usernames are unique across all tenants**, so the login form needs no tenant field and no My Domain host. They must be ASCII, contain one `@`, and have at most 80 characters. Comparison is case-insensitive, and the supplied spelling is stored (Default).

**Organization ID.** Tenant IDs are arbitrary text today, so `orgID(tenant)` derives a stable 18-character `00D` ID from SHA-256 of the tenant, with the existing checksum. Nothing is stored. Tokens for OAuth users use it everywhere. **Provisioned tokens keep returning the tenant as `organization_id`**, because they are a test path and the acceptance harness asserts it.

**Principal.** `Principal` gains two strings: `Username` and `Scope`. Both are empty for provisioned tokens, which keep full access. `Authenticate` checks OAuth access tokens first, then `principals`. It reads role and username from `users` on each call, so a role change applies immediately.

**Provisioned tokens are unchanged.** They cannot be revoked or introspected: revoke returns `invalid_token`, and introspect returns `active: false`.

### Apps

A minimal record in `oauth_apps`: generated `client_id`, tenant, name, optional secret hash (absent means public), exact redirect URIs, permitted grants, permitted scopes, optional JWT certificate, optional client-credentials run-as user, and an active flag.

**Administration is by command line until Plan 36.** Plan 35 owns metadata. Every command takes `DATABASE_URL`:

| Command | Effect |
| --- | --- |
| `salescompel user create` | From `SALESCOMPEL_TENANT`, `SALESCOMPEL_USERNAME`, `SALESCOMPEL_PASSWORD`, `SALESCOMPEL_ROLE`. Prints the user ID. Password at least 12 characters and at most 1,024 bytes. |
| `salescompel user set-password` | From `SALESCOMPEL_USERNAME`, `SALESCOMPEL_PASSWORD`. Clears lockout and deletes the user's tokens. |
| `salescompel user deactivate` | From `SALESCOMPEL_USERNAME`. Deletes the user's tokens. |
| `salescompel oauth-app create` | Flags: `--tenant`, `--name`, `--redirect-uri` (repeatable), `--grants`, `--scopes`, `--public`, `--jwt-cert`, `--jwt-user` (repeatable), `--run-as`. Prints JSON with `client_id` and, unless public, `client_secret`, once. |
| `salescompel oauth-app rotate-secret` | `--client-id`. Prints the new secret once. The old one stops working. Existing tokens stay valid. |
| `salescompel oauth-app disable` | `--client-id`. Deletes the app's tokens. |

**Redirect URIs** match exactly. They must be HTTPS, or HTTP on a loopback host (`localhost`, `127.0.0.1`, `[::1]`). Custom schemes are a gap.

**JWT pre-authorization.** A private key should not be able to impersonate anyone in the tenant, so an app accepts JWT only for users named with `--jwt-user`. An unlisted user gets `invalid_grant`. This fails closed until Plan 33 adds profile-based approval.

**Built-in `PlatformCLI`.** A constant in code, public, available to every tenant, with exactly one redirect URI (`http://localhost:1717/OauthRedirect`), the grants `authorization_code` and `refresh_token`, and the scopes `api web refresh_token`. PKCE is required. It has no tenant of its own, so any user who can log in can use it. An instance cannot disable it in this plan (gap).

**PKCE** is required for public apps and optional for apps with a secret, which must then present the secret. `code_challenge_method` may be absent or `S256`. `plain` returns `invalid_request`. If a challenge was sent, the verifier is required; if none was sent, a verifier is rejected with `invalid_grant`.

### Tokens

| Item | Decision | Basis |
| --- | --- | --- |
| Access token form | First 15 characters of the org ID, `!`, 64 hex characters | Default |
| Access token lifetime | 2 hours, absolute, `serve --access-token-ttl` | Default |
| Refresh token | 64 hex characters; valid until revoked; no rotation; issued only when `refresh_token` or `offline_access` was granted | Documented for the condition; no rotation is Default, and rotation is a gap |
| Authorization code | 10 minutes, single use, bound to client and redirect URI | Default |
| Code replay | Returns `invalid_grant` and deletes the tokens that code issued | Default, following RFC 6749 §4.1.2 |
| JWT and client credentials | No refresh token; `refresh_token` and `offline_access` removed from the granted scope | Documented for client credentials; JWT is Default |
| Success response | `access_token`, `signature` (when computed), `scope`, `refresh_token` (when issued), `instance_url`, `id`, `token_type: Bearer`, `issued_at` in milliseconds as a string | Documented; Client-observed clients accept it |
| `id` | `{public URL}/id/{org ID}/{user ID}` | Documented; JSforce depends on it |
| Client authentication | Body parameters or HTTP Basic; if both are present and differ, `invalid_request` | Documented for client credentials; extended by Default |
| Secret comparison | Constant time over digests | — |
| Expired rows | Deleted in the same transaction when new tokens are issued | Default |

### Scopes

| Scope | Allows |
| --- | --- |
| `api`, `full` | Data routes, the OpenAPI document, userinfo and identity URL |
| `id`, `profile`, `email`, `address`, `phone` | Userinfo and identity URL only |
| `web` | Accepted and stored. Allows no route in this plan. |
| `refresh_token`, `offline_access` | Refresh token issuance |
| `openid` and anything else | Rejected with `invalid_scope` |

A requested scope the app does not permit returns `invalid_scope`. No scope parameter grants all the app's scopes. **Scope is enforced:** a token lacking the needed scope gets 403 `INSUFFICIENT_ACCESS` (Default) in the existing error envelope.

### JWT bearer

Accepted only with `alg` exactly `RS256`; other algorithms, including `none` and HMAC, are rejected, and `kid`, `jku` and `x5u` are ignored. Assertions are limited to 16 KiB.

| Check | Rule | Basis |
| --- | --- | --- |
| `iss` | Active app that permits the grant and has an unexpired certificate of RSA 2048 bits or more | Documented |
| `aud` | Public URL, with or without trailing slash | Documented in form; Client-observed slash |
| `sub` | Active user, same tenant, listed for this app | Documented in form; list is Default |
| `exp` in seconds (below 10^11) | After now and at most 5 minutes ahead | Default. A 3-minute documented limit could not be confirmed. |
| `exp` in milliseconds (10^11 or above) | Requires `iat` in seconds within 5 minutes of now | Client-observed; Default rule |
| `jti`, when present | Rejected if already used by this app | Documented |
| Signature | RS256 over the certificate's key, using only the standard library | — |

**Why the millisecond rule is safe:** either branch bounds replay to five minutes, and neither lets an assertion through without a valid signature. A millisecond-scale `exp` without a fresh `iat` is rejected. Every failure returns the same `invalid_grant` description.

### Client credentials

Requires a secret, the grant permitted on the app, and an active run-as user in the app's tenant, whose role and tenant apply to the token (Documented). A missing or wrong secret returns `invalid_client`, a grant the app does not permit returns `unsupported_grant_type`, and a missing or inactive run-as user returns `invalid_grant`. It works on the single host, where Salesforce requires a My Domain host (gap).

### Endpoints

All OAuth routes sit before the bearer check and use the path normalized for the CLI's slash handling.

| Route | Behavior |
| --- | --- |
| `GET, POST /services/oauth2/authorize` | Login form, as above. `response_type` must be `code`. |
| `GET /services/oauth2/authorize.css` | Embedded stylesheet |
| `POST /services/oauth2/token` | Form-encoded grants, as above |
| `POST /services/oauth2/revoke` | Form parameter `token`, no client authentication. 200 on success; 400 `invalid_token` when unknown or already revoked. Revoking a refresh token also deletes its access tokens; revoking an access token affects only that token. `GET` is not offered (gap). (Documented) |
| `POST /services/oauth2/introspect` | Requires client authentication. RFC 7662 fields: `active`, `scope`, `client_id`, `username`, `sub`, `token_type`, `exp`, `iat`. Tokens issued to another client return `active: false`. (Documented path and format; same-client limit is Default) |
| `GET /.well-known/openid-configuration` | See below |
| `GET /id/{org}/{user}` | See below |

**OAuth errors** use `{"error","error_description"}`, not the REST error array. Descriptions are written independently and contain no values.

| Code | Status | When | Basis |
| --- | --- | --- | --- |
| `invalid_client_id` | 400 | Unknown or disabled client at the token endpoint | Default |
| `invalid_client` | 400 | Wrong or missing secret | Default |
| `invalid_grant` | 400 | Bad code, verifier, refresh token, assertion, or a missing or inactive run-as user | Documented |
| `redirect_uri_mismatch` | 400 | Token request's URI differs from the code's | Default |
| `unsupported_grant_type` | 400 | Grant not offered or not permitted on the app | Default |
| `invalid_request` | 400 | Missing, duplicated or conflicting parameters | Default |
| `invalid_scope` | redirect | Scope not permitted or unknown | Default |
| `login_required` | redirect | `prompt=none` | Default |
| `access_denied` | redirect | **Deny** pressed | Default |

### Discovery

`/.well-known/openid-configuration` is public and lists `issuer`, `authorization_endpoint`, `token_endpoint`, `revocation_endpoint`, `introspection_endpoint`, `userinfo_endpoint`, `response_types_supported`, `grant_types_supported`, `scopes_supported`, `code_challenge_methods_supported` and `token_endpoint_auth_methods_supported`.

**It lists only what the candidate does.** It leaves out `jwks_uri`, `id_token_signing_alg_values_supported` and `openid`, so it is honest OAuth metadata at the documented path, but not a complete OpenID Connect provider document. This is listed as a gap.

### Identity endpoints

- **Userinfo** adds `sub` (the identity URL) for OAuth tokens. `preferred_username` and `username` return the login username.
- **`/id/{org}/{user}`** needs a token for that exact user, and otherwise returns 403. It returns `id`, `user_id`, `organization_id`, `username`.
- **`sobjects/User/{id}`** keeps its existing single-row response, and uses the real username for OAuth users.
- **`Organization` query.** The query route answers exactly the CLI's projection (`Name, InstanceName, IsSandbox, TrialExpirationDate, NamespacePrefix, OrganizationType`, ignoring case and whitespace) with one row: `Name` is the tenant, `IsSandbox` is false, `OrganizationType` is `SalesCompel`, and the rest are null. Any other `Organization` query still returns `INVALID_TYPE`. Plan 29 replaces this. Like the existing `User` response, it is a narrow bridge for clients and is listed as such.

### Public URL

`serve --public-url` is the single source of `issuer`, `aud`, `instance_url`, `id` and form-origin checks. **`Host` and forwarding headers are never trusted.**

It must be absolute with no path, query or fragment, and HTTPS unless its host is loopback. If omitted on a loopback listener, it derives from the listen address and TLS setting. If omitted on any other listener, startup refuses.

### Storage

New tables are added to `internal/platform/schema.sql` with `CREATE TABLE IF NOT EXISTS`, inside the existing advisory-locked startup transaction. The catalog version stays at 3, because nothing in the catalog changes. Plan 27 adopts these tables into its migrations. An upgrade test opens a catalog v3 database with data, and proves the data and provisioned tokens survive.

| Table | Purpose |
| --- | --- |
| `users` | Above, plus failed-attempt count and lock time. Unique on canonical username. |
| `oauth_apps` | Above |
| `oauth_app_jwt_users` | The `--jwt-user` list |
| `oauth_requests` | Pending authorization requests |
| `oauth_codes` | Authorization codes, with a used marker for replay |
| `oauth_refresh_tokens` | Refresh tokens |
| `oauth_access_tokens` | Access tokens, with optional links to their refresh token and code |
| `oauth_jwt_ids` | Used `jti` values until they expire |

`client_id` on tokens, codes and requests is text with no foreign key, because `PlatformCLI` has no row. Every table with user data carries `tenant_id` or reaches it through `users`, ready for Plan 27's row-level security.

## Clients

| Client | Login | Proof |
| --- | --- | --- |
| `sf` | Web, through a real browser; JWT | Unmodified, `--instance-url` only |
| JSforce | Web with PKCE, refresh, JWT, client credentials, revoke | Unmodified, `loginUrl` only |
| simple-salesforce | **Not its own login.** A token from the token endpoint, used through `instance_url` and `session_id` | Unmodified. The hardcoded host is a recorded gap. |

**This narrows the roadmap's E2E line for simple-salesforce.** The roadmap's later item 12 says to build the routing proxy sooner if a pinned client hardcodes a host. This design records the gap and does not build the proxy now, because a routing proxy needs its own TLS and trust design and only this client needs it so far.

## Acceptance

A new required gate, `oauth`, runs between `cli` and `restart` in `scripts/check-candidate-acceptance.sh`. It uses real clients against the running candidate:

1. `sf org login web`: a directory on `PATH` holds stand-ins for the browser launcher that record the URL, and Playwright completes the form in Chromium. `sf org display` reports `Connected`, and `sf data query` returns the tenant's records.
2. **Refresh:** revoke the CLI's access token over HTTP, then `sf data query` must still succeed.
3. `sf org login jwt` succeeds for a listed user and exits non-zero for an unlisted one.
4. JSforce: web login with `useVerifier`, an automatic refresh after revoking the access token, JWT, client credentials, and revoke followed by 401.
5. simple-salesforce: CRUD with an OAuth token through `session_id`.
6. **Policy:** a `restricted` OAuth user is denied the restricted field, and a user in the other tenant cannot read the first tenant's records.
7. Raw HTTP for discovery, introspect, error shapes and scope denial, with OpenAPI traffic validation on every new operation.
8. The `restart` gate proves a refresh token issued before the restart still works.

Port 1717 being in use blocks the gate in preflight, rather than failing it.

Go race tests with injected time cover expiry and lockout. Full acceptance passes on the final commit, with results recorded in `docs/compat/plan-26-oauth-login.md`. No live org is required.

## Reference evidence during the work

Task 1 uses the trial org only if the maintainer supplies a session, and it never blocks the plan. Task 1 runs before the runtime tasks, so that a contradiction with a Documented row changes the behavior rather than being merely logged, unless the change would break a client this plan must support. In that case the client wins, and the contradiction is recorded as a gap. With no session, every Documented row stays as written and unverified.

## Known gaps

To be listed in the decision record:

- simple-salesforce's own login, which hardcodes its host.
- ID tokens, JWKS and full OIDC discovery.
- MFA, browser sessions, session settings, login IP ranges and login history.
- Refresh rotation and per-app token policies; `PlatformCLI` cannot be disabled.
- CORS on OAuth endpoints; `GET` on revoke; custom-scheme redirect URIs.
- All Default rows above, and every unverified Documented row, including the `exp` limit, `signature`, and exact error codes and statuses.
- Client-observed rows show what the client sends. How Salesforce treats a millisecond `exp`, or an absent challenge method, is inferred.
- `organization_id` differs between provisioned and OAuth tokens in one tenant.
- The `Organization` and `User` bridges.

`docs/compat/contracts/identity.yaml` stays byte-identical, because Plan 18 and the lab validators bind it, and `observationStatus: not_run` remains true. The decision record maps its contract IDs to these decisions, as Plan 25 did.

## Calls made for the maintainer to check

1. **simple-salesforce uses a session rather than its own login,** which narrows the roadmap. The alternative is building the routing proxy now.
2. **Server-rendered login page,** rather than a Vue route, so OAuth never depends on web assets.
3. **No browser session until Plan 36,** so users sign in on every authorization.
4. **Client secrets are hashed,** so they can never be shown again, unlike Salesforce.
5. **No ID tokens,** so the discovery document is deliberately incomplete.
6. **Global username uniqueness** with tenant resolved from the user.
7. **Command-line administration** of users and apps until Plan 36.
8. **Explicit JWT user list,** stricter than Salesforce, until Plan 33.
9. **`PlatformCLI` is always on,** so anyone with a login can use the CLI without an administrator registering anything.
10. **New dependency `golang.org/x/crypto`** for Argon2id, the same choice as zeroauthy. It has the same license family as the existing `golang.org/x/text`.
11. **A narrow `Organization` query bridge,** so `sf org display` reports `Connected`.

## Left to the builder

SQL column details not named here, helper names not fixed in the plan, message wording (independently written), the page's visual details within the existing theme, and how the browser stand-ins are named beyond what Task 1 observes.
