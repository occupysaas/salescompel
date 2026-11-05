# Plan 26 — OAuth Login and Identity Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [x]`) syntax for tracking.

**Status:** Implemented and verified, 2026-11-04. See [decisions and verification](../../compat/plan-26-oauth-login.md).

**Goal:** Let unmodified `sf` and JSforce log in to the candidate through OAuth by changing only the login URL, with expiring, revocable tokens that carry the existing tenant and role.

**Architecture:** New `oauth_*.go` files in `internal/platform` hold secrets, JWT checks, storage and handlers. OAuth routes sit before the bearer check in `NewHandler`, and `Authenticate` learns to resolve OAuth access tokens into the existing `Principal`, so record, query and field policy code does not change. A server-rendered login page and new `salescompel` admin commands complete the feature, and a new required `oauth` acceptance gate proves it through real clients.

**Tech Stack:** Go 1.25.0, pgx/v5, PostgreSQL 17, `golang.org/x/crypto` v0.42.0 (new), `golang.org/x/text` v0.29.0, Node >=22/TypeScript, `@salesforce/cli` 2.156.0, JSforce 3.11.0, simple-salesforce 1.12.9, Playwright with Chromium.

**Spec:** [Plan 26 design](../specs/2026-11-04-plan-26-oauth-login-identity-design.md). Read it before each task; its tables are the source of exact behavior, and this plan does not repeat them.

## Global Constraints

- Work directly on `main`.
- The only new dependency is `golang.org/x/crypto` v0.42.0. `go.mod` must keep `go 1.25.0`.
- JWT verification uses only the standard library.
- Use `GOTOOLCHAIN=go1.25.0` and a disposable PostgreSQL 17 database through `TEST_DATABASE_URL`. A skipped database test does not count as passing.
- Plaintext passwords, tokens, codes and client secrets are never stored, logged, placed in error messages, or written to evidence.
- Treat `sf` as a black box: observe its commands, outputs and network traffic. Do not read its implementation, per `CLEAN_ROOM.md`.
- Page text and error descriptions are written independently. No Salesforce templates, styles, icons or prose.
- Do not change `docs/compat/contracts/`, `docs/compat/profiles/`, `docs/compat/observations/`, `api/openapi.yaml`, `CLEAN_ROOM.md`, or historical captures and reports.
- Provisioned tokens keep their exact behavior, including `organization_id` equal to the tenant.
- Existing tests change only where they assert behavior this spec replaces. Note each such change in the commit message.

Shared test command, referred to below as **DB-TEST**:

```sh
docker run -d --rm --name plan26-pg -e POSTGRES_PASSWORD=local-test-only -p 55432:5432 postgres:17
export TEST_DATABASE_URL='postgres://postgres:local-test-only@127.0.0.1:55432/postgres?sslmode=disable'
GOTOOLCHAIN=go1.25.0 go test -race ./internal/platform ./cmd/... -count=1
```

Stop the container once all tasks finish: `docker stop plan26-pg`.

## Review Focus

- A token from one tenant, or for a user deactivated a moment ago, must not reach any record; Task 4 pins this.
- Two simultaneous exchanges of one authorization code must issue tokens at most once, and the replay must delete what the code issued; Task 4 pins this.
- A browser POST with `Origin: null`, from another site, or without the cookie must not complete a login, while a real Chromium login must; Tasks 6 and 9 pin this.
- A `state` or `redirect_uri` holding reserved characters or markup must round-trip safely, and a redirect URI that already has a query must keep it; Task 6 pins this.
- An assertion with `alg: none`, HS256 keyed with the certificate, a far-future seconds `exp`, or a millisecond `exp` without a fresh `iat` must fail even when the rest looks valid; Task 2 pins this.

---

### Task 1: Observation and evidence record

**Files:**
- Create: `docs/compat/plan-26-oauth-login.md`

**Interfaces:**
- Produces: the decision record later tasks append to, and the list of browser launcher names Task 9 uses.

- [x] **Step 1: Repeat the client capture with a real browser**

In a scratch directory, with `HOME` and `SF_STATE_FOLDER` isolated, run a local mock that answers the spec's "What the CLI sends" table, and run `sf org login web --instance-url <mock> --json` with launcher stand-ins on `PATH` (`xdg-open`, `open`, `google-chrome`, `chromium`, `firefox`, `sensible-browser`, `x-www-browser`) that record the URL. Open the recorded URL in Playwright Chromium, so the CLI's final relative redirect is followed.

Expected: the command exits 0. If it does not, record what it needed and fix the affected row in the spec before going on. Commit no mock, key or auth file.

- [x] **Step 2: Confirm the rest of the table on `sf org login jwt`**

Decode one assertion's header and payload. Record `alg`, `aud`, and the scale of `exp` and `iat`, and the verifier's length from the web capture. Expected: the spec's values. A difference is a spec correction, not a failure.

- [x] **Step 3: Review changelogs and, if the maintainer supplies a session, the trial org**

Review Salesforce release notes since the last plan for OAuth changes. With a session, check the Documented rows, following `CLEAN_ROOM.md`. Without one, leave them unverified and say so. This step never blocks the plan. A contradiction changes the spec's row, unless the change would break a client this plan must support, in which case the client wins and the contradiction is listed as a gap.

- [x] **Step 4: Write the record**

Create `docs/compat/plan-26-oauth-login.md` with the observed values, tool versions, reproduction steps, sources used and session status. It contains no credentials.

- [x] **Step 5: Commit**

```bash
git add docs/compat/plan-26-oauth-login.md docs/superpowers/specs/2026-11-04-plan-26-oauth-login-identity-design.md
git commit -m "docs: record Plan 26 client observations"
```

### Task 2: Credential primitives and JWT verification

**Files:**
- Create: `internal/platform/oauth_secrets.go`, `internal/platform/oauth_secrets_test.go`, `internal/platform/oauth_jwt.go`, `internal/platform/oauth_jwt_test.go`
- Modify: `go.mod`, `go.sum`, `THIRD_PARTY_NOTICES.md`

**Interfaces:**
- Produces:
  - `hashPassword(password string) (string, error)`, `verifyPassword(stored, password string) bool`, `verifyDummyPassword(password string)`
  - `newSecret() (string, error)`: 64 hex characters from 32 random bytes
  - `orgID(tenant string) string`, `accessTokenValue(tenant string) (string, error)`
  - `verifyPKCE(challenge, verifier string) bool`
  - `type JWTClaims struct { Issuer, Subject, ID string }`
  - `parseJWT(assertion string) (JWTClaims, parsedJWT, error)`, `verifyJWT(p parsedJWT, cert *x509.Certificate, audiences []string, now time.Time) error`
  - `parseJWTCertificate(pem string, now time.Time) (*x509.Certificate, error)`

- [x] **Step 1: Write failing tests in `oauth_secrets_test.go`**

`TestPasswordHash`: the output starts `$argon2id$v=19$m=19456,t=2,p=1$`, verifies with the right password, fails with a wrong one, and two hashes of one password differ. `verifyPassword` returns false, without panicking, for a malformed string and for one with `m` above 262144. `TestNewSecret`: 64 lowercase hex characters, and two calls differ. `TestOrgID`: 18 characters with prefix `00D`, the same value for the same tenant, and a different one for another tenant. Its first 15 characters plus the checksum satisfy `idChecksum`. `TestAccessTokenValue`: matches `^00D[A-Za-z0-9]{12}![0-9a-f]{64}$`.

`TestVerifyPKCE`: with the RFC 7636 appendix B pair (verifier `dBjftJeZ4CVP-mB92K27uhbUJU1p1r_wW1gFWFOEjXk`, challenge `E9Melhoa2OwvFrEMTJguCHaoeK1t8URWbuGJSstw-cM`) it returns true. A 171-character verifier with its matching challenge returns true. Lengths 42 and 257 return false, as does a verifier with a character outside `[A-Za-z0-9._~-]`.

- [x] **Step 2: Write failing tests in `oauth_jwt_test.go`**

Generate an RSA 2048 key and self-signed certificate in the test. Base time `now`, audiences `{"https://h", "https://h/"}`.

`TestJWTAccepts`, table: seconds `exp` of `now+60`; `exp` of `now+300`; millisecond form with `exp` equal to `now` in milliseconds and `iat` equal to `now`; `iat` of `now-299` with millisecond `exp`; `aud` with and without a trailing slash.

`TestJWTRejects`, table, each returning an error: `alg` of `none` with an empty signature; `HS256` signed using the certificate's public key bytes as the HMAC key; `RS384`; a tampered payload; a signature from a different key; seconds `exp` of `now` and of `now+301`; seconds `exp` far in the future (`now+31536000`); millisecond `exp` with no `iat`; millisecond `exp` with `iat` of `now-301` and `now+301`; wrong `aud`; three segments that are not base64url; two segments; an assertion above 16 KiB. A header carrying `jku` pointing elsewhere is ignored, so it verifies only against the supplied certificate.

`TestJWTCertificate`: rejects an expired certificate, an RSA 1024 certificate, an ECDSA certificate and malformed PEM.

- [x] **Step 3: Run tests to verify they fail**

Run: `GOTOOLCHAIN=go1.25.0 go test ./internal/platform -run 'PasswordHash|NewSecret|OrgID|AccessTokenValue|VerifyPKCE|JWT' -count=1`. Expected: FAIL, undefined functions.

- [x] **Step 4: Add the dependency and implement**

`GOTOOLCHAIN=go1.25.0 go get golang.org/x/crypto@v0.42.0`, then confirm `go.mod` still says `go 1.25.0`. Add the `THIRD_PARTY_NOTICES.md` entry in the existing dependency format. Use Argon2id with m=19456, t=2, p=1, a 16-byte salt and a 32-byte key. Cap parsed parameters so a hostile stored value cannot exhaust memory. `verifyDummyPassword` hashes against a package-level hash of a random value. The 10^11 threshold selects the `exp` branch; see the spec's JWT table.

- [x] **Step 5: Run tests to verify they pass, and commit**

Run the Step 3 command. Expected: PASS.

```bash
git add go.mod go.sum THIRD_PARTY_NOTICES.md internal/platform/oauth_secrets.go internal/platform/oauth_secrets_test.go internal/platform/oauth_jwt.go internal/platform/oauth_jwt_test.go
git commit -m "feat: add password hashing and JWT assertion verification"
```

### Task 3: Users, apps and admin commands

**Files:**
- Create: `internal/platform/oauth_store.go`, `internal/platform/oauth_store_test.go`, `cmd/salescompel/admin.go`, `cmd/salescompel/admin_test.go`
- Modify: `internal/platform/schema.sql`, `internal/platform/catalog_migration_test.go`, `cmd/salescompel/main.go`

**Interfaces:**
- Consumes: Task 2's primitives.
- Produces:
  - `type User struct { ID, Tenant, Username, Role string; Active bool }`
  - `type AppInput struct { Tenant, Name string; Public bool; RedirectURIs, Grants, Scopes, JWTUsers []string; JWTCertPEM, RunAs string }`
  - `type OAuthApp struct { ClientID, Tenant, Name, SecretHash, JWTCertPEM, RunAsUserID string; Public, Active bool; RedirectURIs, Grants, Scopes []string }`
  - `const PlatformCLIClientID = "PlatformCLI"`
  - `(*Store).CreateUser(ctx, tenant, username, password, role string) (string, error)`
  - `(*Store).SetPassword(ctx, username, password string) error`, `(*Store).DeactivateUser(ctx, username string) error`
  - `(*Store).VerifyLogin(ctx, username, password string, now time.Time) (User, error)`, returning `errBadCredentials` for every failure
  - `(*Store).CreateApp(ctx, AppInput, now time.Time) (clientID, secret string, err error)`
  - `(*Store).RotateAppSecret(ctx, clientID string) (string, error)`, `(*Store).DisableApp(ctx, clientID string) error`
  - `(*Store).LookupApp(ctx, clientID string) (OAuthApp, error)`, which returns the built-in record for `PlatformCLI`
  - `(*Store).JWTUserAllowed(ctx, clientID, username string) (User, error)`

- [x] **Step 1: Write failing DB tests in `oauth_store_test.go`**

`TestCreateUserRules`: rejects a 11-character password, a 1,025-byte password, a username with no `@`, with two `@`, with non-ASCII characters, or above 80 characters, and an unknown role. A username differing only by case in another tenant returns a duplicate error. The stored hash starts `$argon2id$`, and no column holds the password.

`TestVerifyLogin`: succeeds in any username case and returns the supplied spelling. Each of unknown user, wrong password and inactive user returns `errBadCredentials`.

`TestLoginLockout`: at injected `now`, nine failures then a correct password succeed and reset the count. Ten failures lock: the correct password returns `errBadCredentials` at `now+14m59s` and succeeds at `now+15m`. `SetPassword` clears the lock.

`TestCreateAppRules`: rejects an `http://example.com/cb` redirect, accepts `http://localhost:9/cb`, `http://127.0.0.1:9/cb`, `http://[::1]:9/cb` and `https://example.com/cb`, and rejects `myapp://cb`. Rejects an unknown grant or scope, `openid`, a `--public` app with `client_credentials`, a JWT grant with no certificate, a run-as user from another tenant, and a JWT user from another tenant. A confidential app returns a 64-hex secret and stores only its digest. A public app returns an empty secret. `RotateAppSecret` changes the stored digest.

`TestLookupPlatformCLI`: returns public, active, the single redirect `http://localhost:1717/OauthRedirect`, grants `authorization_code` and `refresh_token`, and scopes `api web refresh_token`. `CreateApp` cannot produce that client ID.

- [x] **Step 2: Write a failing upgrade test in `catalog_migration_test.go`**

`TestOAuthTablesUpgradeFromV3`: open a fresh store, provision a token, create an Account, drop the eight new tables to reproduce a Plan 25 database, reopen, and confirm the tables exist, the version is still 3, the Account reads back, and the provisioned token authenticates. Reopening again changes nothing.

- [x] **Step 3: Write failing tests in `cmd/salescompel/admin_test.go`**

`TestAdminCommands`, through the command dispatcher against the test database: `user create` prints an ID with prefix `005`; `oauth-app create` prints JSON with `client_id` and `client_secret`; `--public` omits `client_secret`; `rotate-secret` prints a new secret; `user create` with no `SALESCOMPEL_PASSWORD` fails, and no error text contains the password; passing a password as a flag is an error.

- [x] **Step 4: Run tests to verify they fail**

Run: DB-TEST with `-run 'CreateUser|VerifyLogin|LoginLockout|CreateApp|LookupPlatformCLI|OAuthTables|AdminCommands'`. Expected: FAIL.

- [x] **Step 5: Implement the schema, store and commands**

Add the spec's eight tables to `schema.sql`. Store `username_canonical` as the ASCII-lowercased username, with `UNIQUE(username_canonical)`. Hash outside the transaction. A buffered channel of capacity 4 on `Store` bounds Argon2id work, and waiting respects `ctx`. `VerifyLogin` calls `verifyDummyPassword` for an unknown user. `SetPassword`, `DeactivateUser` and `DisableApp` delete the affected tokens in the same transaction. Those token tables exist now, so the deletes need no later task. Extract the existing dispatcher in `main.go` into a function the test can call, and add the spec's commands in `admin.go`.

- [x] **Step 6: Run tests to verify they pass, and commit**

Run: DB-TEST. Expected: PASS.

```bash
git add internal/platform cmd/salescompel
git commit -m "feat: add login users, OAuth apps and admin commands"
```

### Task 4: Codes, tokens and authentication

**Files:**
- Create: `internal/platform/oauth_tokens.go`, `internal/platform/oauth_tokens_test.go`
- Modify: `internal/platform/store.go` (`Authenticate`), `internal/platform/catalog.go` (`Principal`), `internal/platform/http.go`

**Interfaces:**
- Consumes: Task 3's types.
- Produces:
  - `Principal` gains `Username, Scope string`
  - `type TokenGrant struct { ClientID string; User User; Scope string; Refresh bool; CodeHash string }`
  - `type IssuedTokens struct { Access, Refresh, Scope string; User User; IssuedAt time.Time }`
  - `type CodeInput struct { ClientID, UserID, RedirectURI, Scope, Challenge string }`
  - `(*Store).AuthenticateAt(ctx, token string, now time.Time) (Principal, error)`; `Authenticate` calls it with `time.Now()`
  - `(*Store).IssueTokens(ctx, TokenGrant, now time.Time, ttl time.Duration) (IssuedTokens, error)`
  - `(*Store).IssueCode(ctx, CodeInput, now time.Time) (string, error)`
  - `(*Store).RedeemCode(ctx, code, clientID, redirectURI, verifier string, now time.Time, ttl time.Duration) (IssuedTokens, error)`
  - `(*Store).RefreshAccess(ctx, refresh, clientID string, now time.Time, ttl time.Duration) (IssuedTokens, error)`
  - `(*Store).RevokeToken(ctx, token string) error`, returning `errInvalidToken`
  - `type Introspection struct { Active bool; Scope, ClientID, Username, Sub, TokenType string; Exp, Iat int64 }`
  - `(*Store).Introspect(ctx, token, clientID string, now time.Time) (Introspection, error)`
  - `grantError(code string) *oauthError`, the error type Task 5 serializes
  - `scopeAllows(p Principal, route routeClass) bool`, with classes `routeData` and `routeIdentity`

- [x] **Step 1: Write failing DB tests in `oauth_tokens_test.go`**

`TestAccessTokenLifetime`: a token authenticates at `now`, and at `now+ttl-1s`, and returns 401 `INVALID_SESSION_ID` at `now+ttl`. Only the digest is stored: a query for the plaintext in the table finds nothing.

`TestPrincipalFromOAuthToken`: the principal has the user's tenant, ID, role, username and scope. Changing the user's role in the database changes the next `AuthenticateAt` result.

`TestDeactivatedUserLosesAccess`: after `DeactivateUser`, the access and refresh tokens both fail. `TestDisabledAppLosesAccess` is the same for `DisableApp`. `TestSetPasswordRevokes` is the same for `SetPassword`.

`TestTenantIsolationWithOAuthToken`: with tokens for users in two tenants, `Read` of the first tenant's Account with the second's principal returns `NOT_FOUND`, and `Select` omits it.

`TestRestrictedOAuthUser`: a `restricted` OAuth principal is denied the restricted field through `Validate` and `Select`, exactly as a provisioned one.

`TestRedeemCode`: wrong verifier, wrong client, wrong redirect URI and an expired code at `now+10m` each return `invalid_grant` or `redirect_uri_mismatch` per the spec's table. A verifier with no stored challenge is `invalid_grant`, and a stored challenge with no verifier is `invalid_grant`.

`TestCodeReplayRevokes`: a second redemption returns `invalid_grant`, and the first redemption's access and refresh tokens no longer work.

`TestCodeRedeemConcurrent`: 8 goroutines redeem one code. Exactly one succeeds before replay handling runs, and afterward none of the issued tokens works.

`TestRefresh`: returns a new access token, no new refresh token, and the same scope. The old access token still works. Another client's refresh attempt returns `invalid_grant`.

`TestRevoke`: revoking an access token stops only it. Revoking a refresh token stops it and its access tokens. An unknown or repeated value returns `errInvalidToken`. A provisioned token returns `errInvalidToken` and still authenticates.

`TestIntrospect`: active fields for the owning client, and `Active: false` for another client, an expired token, a revoked token, a provisioned token and garbage.

`TestProvisionedTokenUnchanged`: `Principal` has empty `Username` and `Scope`, and `scopeAllows` returns true for both route classes.

`TestScopeAllows`: table from the spec's Scopes section, including `web` alone allowing neither class.

- [x] **Step 2: Write a failing DB test in `http_test.go`**

`TestHTTPScopeEnforced`: an OAuth token with scope `id` gets 403 `INSUFFICIENT_ACCESS` in the array envelope on `sobjects/Account` and the OpenAPI route, and 200 on `/services/oauth2/userinfo`. Scope `web` gets 403 on both. Scope `api` gets 200 on both. A provisioned token is unchanged.

- [x] **Step 3: Run tests to verify they fail**

Run: DB-TEST with `-run 'AccessTokenLifetime|PrincipalFromOAuth|LosesAccess|SetPasswordRevokes|WithOAuthToken|RestrictedOAuthUser|RedeemCode|CodeReplay|CodeRedeem|Refresh|Revoke|Introspect|ProvisionedTokenUnchanged|ScopeAllows|HTTPScopeEnforced'`. Expected: FAIL.

- [x] **Step 4: Implement**

`RedeemCode` claims the code with one `UPDATE ... SET used_at=$1 WHERE code_hash=$2 AND used_at IS NULL RETURNING ...`. Zero rows with an existing code means replay: delete the tokens linked to `code_hash`. `Provision`'s comparison reads only tenant, user and role, so it stays correct with the wider `Principal`. Delete expired rows in `IssueTokens`' transaction. In `NewHandler`, apply `scopeAllows` immediately after `Authenticate`.

- [x] **Step 5: Run tests to verify they pass, and commit**

Run: DB-TEST. Expected: PASS.

```bash
git add internal/platform
git commit -m "feat: issue, expire and revoke OAuth tokens under existing policy"
```

### Task 5: Token, revoke and introspect endpoints

**Files:**
- Create: `internal/platform/oauth_http.go`, `internal/platform/oauth_http_test.go`
- Modify: `internal/platform/http.go`, `cmd/salescompel/main.go`

**Interfaces:**
- Consumes: Tasks 2–4.
- Produces:
  - `type Options struct { PublicURL string; AccessTokenTTL time.Duration; Now func() time.Time }`
  - `NewHandlerWithOptions(s *Store, assetDir string, o Options) http.Handler`. `NewHandler(s, assetDir)` delegates with zero `Options`, and an empty `PublicURL` leaves OAuth routes other than the existing userinfo unserved, so existing tests keep their results.
  - `ResolvePublicURL(flagValue, addr string, tls bool) (string, error)`
  - `serve --public-url` and `serve --access-token-ttl` flags, default 2 hours
  - `handleToken`, `handleRevoke`, `handleIntrospect`, each `func(*Store, Options, http.ResponseWriter, *http.Request)`

- [x] **Step 1: Write failing tests**

`TestResolvePublicURL`: derives `http://127.0.0.1:8080` and, with TLS, `https://127.0.0.1:8080`. Rejects a path, query or fragment, `http://example.com`, and a non-loopback listener with no flag. Accepts `https://crm.example.com` on any listener. A trailing slash is removed.

`TestTokenAuthorizationCode`: a code from `IssueCode` for a confidential app returns the spec's success fields, with `instance_url` equal to the public URL and `id` equal to `{public}/id/{orgID}/{userID}`. `signature` equals base64 HMAC-SHA256 of `id` plus `issued_at`, keyed with the secret. `issued_at` is a string of digits. `Cache-Control` is `no-store`. For `PlatformCLI`, no secret is needed, a verifier is required, and `signature` is absent. An absent `client_secret` on a confidential app returns 400 `invalid_client`.

`TestTokenCLIShapes`: pins the three client-observed traps through HTTP. A 171-character verifier works, a challenge saved without a method verifies as S256, and an assertion with `aud` ending in `/` and a millisecond `exp` succeeds.

`TestTokenJWT`: succeeds for a listed user with no `refresh_token` in the response, and scope without `refresh_token`. An unlisted user, inactive user, user from another tenant, disabled app and app without the grant each return `invalid_grant` or `unsupported_grant_type` per the spec. Descriptions are identical across `invalid_grant` failures. A reused `jti` fails, and two assertions with no `jti` both succeed.

`TestTokenClientCredentials`: succeeds with body credentials and with HTTP Basic, returns no `refresh_token`, and acts as the run-as user. A wrong or missing secret returns `invalid_client`. An inactive run-as user returns `invalid_grant`. Body and Basic that differ return `invalid_request`.

`TestTokenRejects`: `grant_type=password` and an unknown grant return `unsupported_grant_type`, and an absent grant returns `invalid_request`. A duplicated `grant_type` parameter returns `invalid_request`. `GET` returns 405. A JSON body returns `invalid_request`. No response contains the submitted secret or assertion.

`TestRevokeAndIntrospectHTTP`: revoke returns 200, then the token gets 401 on `/services/data/v65.0/sobjects`. Repeat revoke returns 400 `invalid_token`. Introspect without credentials returns `invalid_client`. Introspect with credentials returns the RFC 7662 fields.

- [x] **Step 2: Run tests to verify they fail**

Run: DB-TEST with `-run 'ResolvePublicURL|Token|RevokeAndIntrospect'`. Expected: FAIL.

- [x] **Step 3: Implement and route**

Dispatch on the normalized `requestPath` before the bearer check. Reject bodies that are not `application/x-www-form-urlencoded`. Parse with the existing 1 MiB limit, and reject any repeated parameter. Pass `Options.Now()` to the store. In `main.go`, resolve the public URL before listening, and use the new constructor.

- [x] **Step 4: Run tests to verify they pass, and commit**

Run: DB-TEST. Expected: PASS.

```bash
git add internal/platform cmd/salescompel
git commit -m "feat: serve the OAuth token, revoke and introspect endpoints"
```

### Task 6: Authorization endpoint and login page

**Files:**
- Create: `internal/platform/oauth_page.go`, `internal/platform/oauth_page.html`, `internal/platform/oauth_page.css`, `internal/platform/oauth_page_test.go`, `locales/embed.go`
- Modify: `internal/platform/oauth_tokens.go`, `internal/platform/http.go`, `locales/messages/en_US.json`, `locales/messages/fr.json`, `locales/messages/de.json`, `locales/messages/pt_BR.json`, `locales/messages/iw.json`

**Interfaces:**
- Consumes: Tasks 3–5.
- Produces:
  - `locales.Messages embed.FS`
  - `type AuthRequest struct { ClientID, RedirectURI, Scope, State, Challenge string }`
  - `(*Store).SaveAuthRequest(ctx, AuthRequest, now time.Time) (id, binding string, err error)`
  - `(*Store).LoadAuthRequest(ctx, id, binding string, now time.Time) (AuthRequest, error)`, `(*Store).DeleteAuthRequest(ctx, id string) error`
  - `handleAuthorize(*Store, Options, http.ResponseWriter, *http.Request)`

- [x] **Step 1: Write failing tests in `oauth_page_test.go`**

`TestAuthorizeValidation`: an unknown client and an unregistered redirect URI return an error page with no `Location`. A `response_type` other than `code`, `code_challenge_method=plain`, an unknown or unpermitted scope, `openid`, a public app with no challenge and `prompt=none` each return 302 to the redirect URI with the spec's `error` and the original `state`. No scope parameter grants all of the app's scopes.

`TestAuthorizePage`: `GET` returns 200 HTML with the app's name, a hidden request ID, no client or challenge in the form, `Referrer-Policy: same-origin`, an `HttpOnly`, `SameSite=Strict` cookie with `Secure` only for an HTTPS public URL, and the existing `Content-Security-Policy`. The page has no inline script or style attribute. The CSS route returns `text/css` and works with an empty asset directory.

`TestAuthorizeLogin`: a correct `POST` returns a redirect with `code` and `state`, and the code redeems through `RedeemCode`. The request ID cannot be used twice. A wrong password re-renders with the generic message and keeps the request usable. **Deny** redirects with `access_denied`.

`TestAuthorizeGenericFailure`: unknown user, wrong password, inactive user, locked user and a user outside a tenant-owned app's tenant all produce byte-identical messages and no `code`.

`TestAuthorizeForgery`: `POST` without the cookie, with another request's cookie, with `Origin: https://evil.example`, with `Origin: null`, with `Sec-Fetch-Site: cross-site` or an expired request (`now+10m`) completes no login. A `POST` with no `Origin` or `Sec-Fetch-Site`, as from a non-browser client with the right cookie, proceeds.

`TestAuthorizeEscaping`: a `state` of `a&b=<script>"'` round-trips exactly through the redirect and never appears raw in HTML. A registered redirect URI of `https://c.example/cb?x=1` keeps `x=1` and gains `code` and `state`. An app name containing markup renders escaped.

`TestAuthorizeLocalization`: `Accept-Language: fr` renders French labels, `iw` or `he` renders `dir="rtl"`, and `xx` falls back to `en_US`. All catalogs contain every `oauth.*` key.

- [x] **Step 2: Run tests to verify they fail**

Run: DB-TEST with `-run Authorize`. Expected: FAIL.

- [x] **Step 3: Implement**

Add `oauth.*` keys to all five catalogs, with full translations. Match languages with `golang.org/x/text/language`, mapping the catalog name `iw` to the tag `he`. Override `Referrer-Policy` on the page only. Use `303` for redirects after `POST`. Build redirects with `url.Parse` and `Query().Set`. Do not add a `form-action` directive to the policy: browsers can apply it to the redirect that follows the POST.

- [x] **Step 4: Run tests to verify they pass, and commit**

Run: DB-TEST, then `npm --prefix web test`. Expected: PASS, including the web catalog tests.

```bash
git add internal/platform locales
git commit -m "feat: add the localized OAuth login and consent page"
```

### Task 7: Discovery and identity endpoints

**Files:**
- Modify: `internal/platform/oauth_http.go`, `internal/platform/oauth_http_test.go`, `internal/platform/http.go`

**Interfaces:**
- Consumes: Tasks 4–5.
- Produces: `handleDiscovery`, `handleIdentity`, `isOrganizationDiscovery(q string) bool`.

- [x] **Step 1: Write failing tests**

`TestDiscovery`: unauthenticated 200 with the spec's fields, every URL starting with the public URL, `grant_types_supported` exactly the four offered, and no `jwks_uri`, `id_token_signing_alg_values_supported` or `openid`. A spoofed `Host` or `X-Forwarded-Host` does not alter any URL. With an empty public URL, it returns 404.

`TestIdentityURL`: `GET` with the owning token returns the spec's fields. Another user's token returns 403, and no token returns 401. The route works when the asset directory is set, so it is not served as a static file.

`TestUserInfoForOAuth`: `user_id`, the `00D` `organization_id`, `username`, `preferred_username` and `sub`. A provisioned token's response is unchanged and has no `sub`.

`TestUserRouteForOAuth`: both `v51.0` and `v65.0` `sobjects/User/{id}` return the real username. Another user's ID returns 404.

`TestOrganizationBridge`: the exact CLI query, in any case or spacing, returns one row with the spec's values, for the caller's tenant. `SELECT Name FROM Organization`, and the same projection with a `WHERE`, return `INVALID_TYPE`.

- [x] **Step 2: Run tests to verify they fail**

Run: DB-TEST with `-run 'Discovery|IdentityURL|UserInfoForOAuth|UserRouteForOAuth|OrganizationBridge'`. Expected: FAIL.

- [x] **Step 3: Implement, verify and commit**

Run the Step 2 command, then DB-TEST. Expected: PASS.

```bash
git add internal/platform
git commit -m "feat: add OAuth discovery, the identity URL and CLI bridges"
```

### Task 8: Served OpenAPI

**Files:**
- Modify: `api/candidate-openapi.yaml`, `api/candidate_contract_test.go`, `compat/clients/node/src/candidate-openapi-contract.ts`, `compat/clients/node/src/candidate-openapi-traffic.ts`, `compat/clients/node/test/candidate-openapi-contract.test.ts`, `compat/clients/node/test/candidate-openapi.test.ts`, `internal/platform/http_openapi_test.go`

**Interfaces:**
- Produces: operation IDs `candidateOAuthAuthorizeForm`, `candidateOAuthAuthorizeSubmit`, `candidateOAuthToken`, `candidateOAuthRevoke`, `candidateOAuthIntrospect`, `candidateOAuthDiscovery`, `candidateOAuthIdentity`, used by Task 9's traffic checks.

- [x] **Step 1: Write failing contract tests**

Extend the operation table with the seven operations and their documented statuses. Assert that each has `x-source`. Assert the document describes both a provisioned bearer scheme and an `oauth2` scheme with `authorizationCode` and `clientCredentials` flows. `info.description` no longer says there is no OAuth flow. Offline-mode tests cover every new schema, including `oneOf` on the token response's optional fields as needed. Each OAuth error schema requires `error`, and a separate schema from the REST array is used.

- [x] **Step 2: Run tests to verify they fail**

Run: `GOTOOLCHAIN=go1.25.0 go test ./api -count=1` and `npm --prefix compat/clients/node run check`. Expected: FAIL on the new operations.

- [x] **Step 3: Update the document, verify and commit**

Add the operations, extend `UserInfo` with optional `sub`, and pin `x-source` to the commit containing Task 7, following the existing URL form. Keep the OpenAPI version at 3.2.1. Run Step 2's commands and DB-TEST. Expected: PASS.

```bash
git add api compat/clients/node internal/platform/http_openapi_test.go
git commit -m "feat: document the OAuth surface in the served OpenAPI"
```

### Task 9: Required acceptance gate

**Files:**
- Create: `compat/clients/node/src/candidate-oauth.ts`, `compat/clients/node/test/candidate-oauth.test.ts`, `web/test/oauth-login.spec.ts`, `scripts/candidate-oauth-python.py`
- Modify: `compat/clients/node/src/candidate-acceptance-contract.ts`, `compat/clients/node/src/candidate-acceptance-run.ts`, `compat/clients/node/src/candidate-acceptance-service.ts`, `compat/clients/node/src/candidate-acceptance-preflight.ts`, `compat/clients/node/src/candidate-acceptance-results.ts`, `compat/schemas/candidate-acceptance.schema.json`, `compat/clients/node/test/candidate-acceptance-contract.test.ts`, `compat/clients/node/test/candidate-acceptance-results.test.ts`

**Interfaces:**
- Consumes: Task 1's launcher names, Task 3's commands, Task 8's operations.
- Produces: gate ID `oauth`, between `cli` and `restart`; `runOAuthGate(c: GateContext): Promise<ParsedResult>`; `verifyOAuthRestart(c: GateContext)`, called in `restart`; coverage IDs below.

Required `oauth` coverage IDs: `oauth/provision`, `oauth/sf-web`, `oauth/sf-display-connected`, `oauth/sf-refresh`, `oauth/sf-jwt`, `oauth/sf-jwt-unlisted`, `oauth/jsforce-web-pkce`, `oauth/jsforce-refresh`, `oauth/jsforce-jwt`, `oauth/jsforce-client-credentials`, `oauth/jsforce-revoke`, `oauth/python-session`, `oauth/restricted-denied`, `oauth/tenant-isolation`, `oauth/http-contract`, `oauth/browser-page`. The `restart` gate adds `restart/oauth-refresh`.

- [x] **Step 1: Write failing contract tests**

In `candidate-acceptance-contract.test.ts` and `candidate-acceptance-results.test.ts`: `gateIDs` contains `oauth` directly after `cli`; the schema's two enums accept it; a report missing any listed coverage ID, or with one skipped, is not acceptance eligible; `restart` requires `restart/oauth-refresh`; and `step` in `candidate-acceptance-run.ts` calls `publishStepResult` for `oauth` as it does for `cli` and `restart`, so its evidence is hashed. In `candidate-oauth.test.ts`, the parser for the stand-in's URL file rejects a URL that is not on the candidate's host, and redaction removes access tokens, refresh tokens, secrets and passwords from any stored evidence, verified on a sample.

- [x] **Step 2: Run tests to verify they fail**

Run: `npm --prefix compat/clients/node run check`. Expected: FAIL on the missing gate.

- [x] **Step 3: Wire the service and preflight**

Start the candidate with `--public-url c.url`. Preflight reports the gate blocked, not failed, when port 1717 is in use. Provision through the real commands: two `admin`/`restricted` users in the first tenant, one user in the other tenant, a confidential app with a JWT certificate, `--jwt-user` for the admin only, a run-as user, and a JSforce redirect on an ephemeral loopback port that the gate listens on. Hold generated credentials only in the run's private scratch directory, and add the principals to the cleanup record per the harness's retention contract.

- [x] **Step 4: Implement the browser spec**

`web/test/oauth-login.spec.ts` takes the authorize URL from an environment variable, with `ignoreHTTPSErrors` for the harness certificate. It fills the form, presses **Allow** and waits for the redirect to finish. Separately, it asserts that a wrong password shows the generic message and that Hebrew renders `dir="rtl"`. This is the real-browser proof that `Referrer-Policy: same-origin` makes the form's `Origin` acceptable.

- [x] **Step 5: Implement the CLI steps**

Run `sf` with launcher stand-ins prepended to its `PATH` only, which write the URL to a private file and return. Drive the URL with the spec, then require exit 0. `sf org display --json` must report `connectedStatus` of `Connected`. `sf data query` returns a seeded Account. For refresh, get the access token with `sf org auth show-access-token`, revoke it with HTTP, and require the next `sf data query` to succeed. For JWT, require success for the listed user, a non-zero exit for the unlisted user, and that the second writes no usable auth.

- [x] **Step 6: Implement the JSforce and Python steps**

JSforce, with only `loginUrl` configured: `OAuth2` with `useVerifier` through the browser spec and `conn.authorize`; automatic refresh after revoking the access token; `requestToken` with the JWT and client-credentials grants; `revokeToken` followed by a 401. `scripts/candidate-oauth-python.py` uses simple-salesforce with `instance_url` and `session_id` for create, read and delete. Add the policy, tenant and raw HTTP checks, and record each new operation through the existing traffic validator.

- [x] **Step 7: Add the restart check**

Before `restartCandidate`, save a refresh token. After it, `verifyOAuthRestart` exchanges it and reads an Account with the new access token.

- [x] **Step 8: Run the full acceptance and commit**

Run: `CHROMIUM_PATH=/usr/bin/chromium scripts/check-candidate-acceptance.sh --out /tmp/salescompel-plan26/acceptance.json`. Expected: exit 0, full and acceptance eligible, with every `oauth` ID passed and none skipped.

Confirm that the report and evidence directory contain no secret: search them for the run's generated password, client secret, and each token captured by the gate. Expected: no matches.

```bash
git add compat scripts/candidate-oauth-python.py web/test/oauth-login.spec.ts
git commit -m "feat: require OAuth login acceptance through real clients"
```

### Task 10: Documentation and closure

**Files:**
- Modify: `docs/compat/plan-26-oauth-login.md`, `docs/compat/implementation-log.md`, `docs/compat/README.md`, `README.md`, `docs/superpowers/specs/2026-11-04-salescompel-first-release-roadmap-design.md`, `docs/superpowers/specs/2026-11-04-plan-26-oauth-login-identity-design.md`, this plan

- [x] **Step 1: Complete the decision record**

Following `docs/compat/plan-25-foundation-decisions.md`, add: a table mapping `oauth-token-endpoint`, `oauth-authorize-revoke-introspect`, `login-hosts`, `soap-login`, `connected-app-policy` and `saml-sso` to a decision, basis, implementation, test and owner; every gap from the spec; the operator guide for `--public-url`, the admin commands, upgrade (stop old instances, back up, start) and rollback (stop, restore the backup); and the exact verification results. State plainly that simple-salesforce does not use its own login here.

- [x] **Step 2: Update the pointers**

README: auth status and the next priority (Plan 27). Roadmap: an implementation record under Plan 26, and the simple-salesforce narrowing. Spec and this plan: status. Log: one entry. `docs/compat/contracts/` stays unchanged.

- [x] **Step 3: Run the final verification**

```sh
GOTOOLCHAIN=go1.25.0 go test -race ./... -count=1
GOTOOLCHAIN=go1.25.0 go vet ./...
npm --prefix compat/clients/node run check
npm --prefix web test
npm --prefix web run build
CHROMIUM_PATH=/usr/bin/chromium scripts/check-candidate-acceptance.sh --out /tmp/salescompel-plan26/final.json
git diff --check
git diff 13f7e97 --stat -- docs/compat/contracts docs/compat/profiles docs/compat/observations api/openapi.yaml CLEAN_ROOM.md
```

Expected: every command passes, no database test skips, and the last prints nothing. Record actual counts, skips and the report's SHA-256 in the decision record. If a result differs from what is expected, record the actual result rather than the expected one.

- [x] **Step 4: Commit**

```bash
git add README.md docs
git commit -m "docs: record Plan 26 OAuth decisions, gaps and verification"
```
