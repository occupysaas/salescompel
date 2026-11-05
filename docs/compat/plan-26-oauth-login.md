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
| PKCE challenge method | Absent; challenge is S256, 43 characters |
| PKCE verifier length | 171 characters |
| Browser completion | `/OauthSuccess`, CLI exit 0 |
| JWT algorithm | `RS256` |
| JWT audience | Mock login URL with trailing slash |
| JWT expiry / issue time | Millisecond `exp`, seconds `iat`; exp minus iat×1000 was 965 ms |
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
