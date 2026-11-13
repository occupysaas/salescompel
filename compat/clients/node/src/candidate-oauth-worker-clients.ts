// Salesforce CLI and JSforce client flows: web login, refresh, JWT bearer,
// client credentials, and revocation, each asserting candidate endpoints.
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { randomBytes, createSign } from "node:crypto";
import { Connection, OAuth2 } from "jsforce";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import { assertCandidateOAuthEndpoints } from "./candidate-oauth.ts";
import {
  addSecret,
  ensure,
  remember,
} from "./candidate-oauth-worker-runtime.ts";
import { revoke } from "./candidate-oauth-worker-http.ts";

export type Passed = (id: string) => void;

export type SalesforceCli = (
  args: string[],
  allowFailure?: boolean,
) => Promise<{ exit: number | null; result: Record<string, any> | undefined }>;

export function clientQuery(c: GateContext) {
  return async (connection: Connection, soql: string) => {
    assertCandidateOAuthEndpoints(connection.oauth2, c.url);
    ensure(new URL(connection.instanceUrl).origin === new URL(c.url).origin);
    return connection.query(soql);
  };
}

export async function salesforceWebLogin(
  c: GateContext,
  options: {
    alias: string;
    account: string;
    sf: SalesforceCli;
    authorize: string;
    browser: (url: string) => Promise<void>;
    passed: Passed;
  },
) {
  const { alias, account, sf, authorize, browser, passed } = options;
  await browser(authorize);
  passed("sf-web");
  passed("browser-page");
  ensure(
    (await sf(["org", "display", "--target-org", alias])).result
      ?.connectedStatus === "Connected",
  );
  passed("sf-display-connected");
  const sfQuery = async () => {
    const q = await sf([
      "data",
      "query",
      "--target-org",
      alias,
      "--api-version",
      "65.0",
      "--query",
      `SELECT Id,Name FROM Account WHERE Id = '${account}'`,
    ]);
    ensure(q.result?.records?.[0]?.Id === account);
  };
  await sfQuery();
  const access = (
    await sf(["org", "auth", "show-access-token", "--target-org", alias])
  ).result?.accessToken;
  ensure(typeof access === "string");
  addSecret(access);
  await revoke(c, access);
  await sfQuery();
  passed("sf-refresh");
}

export async function salesforceJwtLogin(
  c: GateContext,
  options: {
    alias: string;
    clientId: string;
    users: { admin: string; restricted: string };
    sf: SalesforceCli;
    passed: Passed;
  },
) {
  const { alias, clientId, users, sf, passed } = options;
  const jwtArgs = (username: string, target: string) => [
    "org",
    "login",
    "jwt",
    "--instance-url",
    c.url,
    "--client-id",
    clientId,
    "--jwt-key-file",
    join(c.scratch, "key.pem"),
    "--username",
    username,
    "--alias",
    target,
  ];
  await sf(jwtArgs(users.admin, alias + "-jwt"));
  passed("sf-jwt");
  ensure(
    (await sf(jwtArgs(users.restricted, alias + "-unlisted"), true)).exit !== 0,
  );
  ensure(
    (await sf(["org", "display", "--target-org", alias + "-unlisted"], true))
      .exit !== 0,
  );
  passed("sf-jwt-unlisted");
}

function candidateOAuth2(
  c: GateContext,
  clientId: string,
  clientSecret: string,
  extra: Record<string, string | boolean> = {},
) {
  const oauth2 = new OAuth2({
    loginUrl: c.url,
    clientId,
    clientSecret,
    ...extra,
  });
  assertCandidateOAuthEndpoints(oauth2, c.url);
  return oauth2;
}

export async function jsforceWebLogin(
  c: GateContext,
  options: {
    app: { client_id: string; client_secret: string };
    account: string;
    redirect: string;
    browser: (url: string) => Promise<void>;
    callbackURL: Promise<URL>;
    passed: Passed;
  },
) {
  const { app, account, redirect, browser, callbackURL, passed } = options;
  const oauth2 = candidateOAuth2(c, app.client_id, app.client_secret, {
    redirectUri: redirect,
    useVerifier: true,
  });
  const webURL = oauth2.getAuthorizationUrl({
    scope: "api web refresh_token",
    state: randomBytes(16).toString("hex"),
  });
  await browser(webURL);
  console.error("OAuth JSforce browser passed");
  const code = (await callbackURL).searchParams.get("code");
  ensure(code);
  addSecret(code!);
  const connection = new Connection({ oauth2, version: "65.0" });
  assertCandidateOAuthEndpoints(connection.oauth2, c.url);
  await connection.authorize(code!);
  console.error("OAuth JSforce authorize passed");
  remember({
    accessToken: connection.accessToken,
    refreshToken: connection.refreshToken,
  });
  const query = clientQuery(c);
  ensure(
    (await query(connection, `SELECT Id FROM Account WHERE Id = '${account}'`))
      .records.length === 1,
  );
  passed("jsforce-web-pkce");
  const refresh = connection.refreshToken;
  ensure(refresh);
  await revoke(c, connection.accessToken!);
  ensure(
    (await query(connection, `SELECT Id FROM Account WHERE Id = '${account}'`))
      .records.length === 1,
  );
  remember({ accessToken: connection.accessToken });
  passed("jsforce-refresh");
  return { connection, refresh };
}

async function signedAssertion(
  c: GateContext,
  app: { client_id: string },
  sub: string,
) {
  const key = await readFile(join(c.scratch, "key.pem"), "utf8");
  const jwt = [
    Buffer.from(JSON.stringify({ alg: "RS256", typ: "JWT" })).toString(
      "base64url",
    ),
    Buffer.from(
      JSON.stringify({
        iss: app.client_id,
        sub,
        aud: c.url,
        exp: Math.floor(Date.now() / 1000) + 120,
      }),
    ).toString("base64url"),
  ].join(".");
  const assertion =
    jwt + "." + createSign("RSA-SHA256").update(jwt).sign(key, "base64url");
  addSecret(assertion);
  return assertion;
}

export async function jsforceJwtLogin(
  c: GateContext,
  app: { client_id: string; client_secret: string },
  users: { admin: string },
  passed: Passed,
) {
  const assertion = await signedAssertion(c, app, users.admin);
  const jwtOAuth = candidateOAuth2(c, app.client_id, app.client_secret);
  const jt = remember(
    await jwtOAuth.requestToken({
      grant_type: "urn:ietf:params:oauth:grant-type:jwt-bearer",
      assertion,
    }),
  );
  const jwtConn = new Connection({
    oauth2: jwtOAuth,
    version: "65.0",
    instanceUrl: jt.instance_url,
    accessToken: jt.access_token,
  });
  remember({ accessToken: jwtConn.accessToken });
  ensure(
    (await clientQuery(c)(jwtConn, "SELECT Id FROM Account")).records.length >
      0,
  );
  passed("jsforce-jwt");
}

export async function jsforceClientCredentialsLogin(
  c: GateContext,
  app: { client_id: string; client_secret: string },
  passed: Passed,
  probes: { userinfoStatus(token: string): Promise<number> },
) {
  const ccOAuth = candidateOAuth2(c, app.client_id, app.client_secret);
  const ct = remember(
    await ccOAuth.requestToken({ grant_type: "client_credentials" }),
  );
  const cc = new Connection({
    oauth2: ccOAuth,
    version: "65.0",
    instanceUrl: ct.instance_url,
    accessToken: ct.access_token,
  });
  remember({ accessToken: cc.accessToken });
  ensure(
    (await clientQuery(c)(cc, "SELECT Id FROM Account")).records.length > 0,
  );
  passed("jsforce-client-credentials");
  const revoked = cc.accessToken!;
  assertCandidateOAuthEndpoints(cc.oauth2, c.url);
  await cc.oauth2.revokeToken(revoked);
  ensure((await probes.userinfoStatus(revoked)) === 401);
  passed("jsforce-revoke");
}
