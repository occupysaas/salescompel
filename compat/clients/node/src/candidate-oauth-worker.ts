// Run in a fresh Node process so NODE_EXTRA_CA_CERTS is loaded before JSforce.
// Credentials and protocol exchanges stay in memory; child output is private.
import {
  readFile,
  writeFile,
  mkdir,
  chmod,
  rm,
  appendFile,
} from "node:fs/promises";
import { join } from "node:path";
import { createServer } from "node:http";
import { randomBytes, createHash, createSign } from "node:crypto";
import { Connection, OAuth2 } from "jsforce";
import { parse as parseYAML } from "yaml";
import {
  runCommand,
  newProcessContext,
  stopOwnedProcesses,
} from "./candidate-acceptance-process.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import { ROOT, SF_BIN } from "./versions.ts";
import { stepResult } from "./candidate-acceptance-clients.ts";
import { createFixture } from "./candidate-acceptance-fixtures.ts";
import {
  parseLauncherURL,
  assertCandidateOAuthEndpoints,
  redactOAuthEvidence,
} from "./candidate-oauth.ts";
import { createCandidateTrafficValidator } from "./candidate-openapi-traffic.ts";
import type { CandidateTrafficValidator } from "./candidate-openapi-model.ts";
const ensure = (condition: unknown): void => {
  if (!condition) throw new Error("ASSERTION");
};
const tokenPath = "/services/oauth2/token";
let candidateOrigin = "";
let validator: CandidateTrafficValidator | undefined;
const operations = new Set<string>();
const secrets = new Set<string>();
function remember(value: any): any {
  if (value && typeof value === "object")
    for (const [key, item] of Object.entries(value)) {
      if (
        [
          "access_token",
          "refresh_token",
          "client_secret",
          "accessToken",
          "refreshToken",
          "password",
          "code",
        ].includes(key) &&
        typeof item === "string"
      )
        secrets.add(item);
      else if (typeof item === "object") remember(item);
    }
  return value;
}
async function request(
  url: string,
  method = "GET",
  body?: URLSearchParams,
  headers: Record<string, string> = {},
) {
  if (new URL(url).origin !== candidateOrigin) throw new Error("SOURCE");
  const response = await fetch(url, {
    method,
    redirect: "manual",
    headers: {
      ...(body ? { "Content-Type": "application/x-www-form-urlencoded" } : {}),
      ...headers,
    },
    body: body?.toString(),
  });
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (validator) {
    const u = new URL(url);
    const result = validator.validate({
      method,
      path: u.pathname + u.search,
      request: {
        contentType: body ? "application/x-www-form-urlencoded" : null,
        ...(body ? { body: Buffer.from(body.toString()) } : {}),
      },
      response: {
        status: response.status,
        contentType: response.headers.get("content-type"),
        body: bytes,
      },
    });
    if (result.requestIssues.length || result.responseIssues.length) {
      console.error(
        JSON.stringify({
          operation: result.operationId,
          request: result.requestIssues,
          response: result.responseIssues,
        }),
      );
      throw new Error("ASSERTION");
    }
    if (result.operationId) operations.add(result.operationId);
  }
  const text = Buffer.from(bytes).toString();
  let json: any;
  try {
    json = remember(JSON.parse(text));
  } catch {}
  return { status: response.status, text, json, headers: response.headers };
}
async function exchange(c: GateContext, form: Record<string, string>) {
  const r = await request(c.url + tokenPath, "POST", new URLSearchParams(form));
  ensure(r.status === 200 && typeof r.json?.access_token === "string");
  return r.json;
}
async function query(c: GateContext, access: string, account: string) {
  const r = await request(
    c.url +
      "/services/data/v65.0/query?q=" +
      encodeURIComponent(`SELECT Id,Name FROM Account WHERE Id = '${account}'`),
    "GET",
    undefined,
    { Authorization: "Bearer " + access },
  );
  ensure(
    r.status === 200 &&
      r.json.records.length === 1 &&
      r.json.records[0].Id === account,
  );
}
async function revoke(c: GateContext, token: string) {
  ensure(
    (
      await request(
        c.url + "/services/oauth2/revoke",
        "POST",
        new URLSearchParams({ token }),
      )
    ).status === 200,
  );
}
async function browser(
  c: GateContext,
  authorize: string,
  username: string,
  password: string,
) {
  const output = join(c.scratch, "oauth-browser");
  const r = await runCommand(
    {
      executable: "npm",
      argv: ["--prefix", "web", "run", "test:e2e", "--", "--reporter=dot"],
      cwd: ROOT,
      env: {
        ...c.env,
        CANDIDATE_OAUTH_AUTHORIZE_URL: parseLauncherURL(authorize, c.url),
        CANDIDATE_OAUTH_USERNAME: username,
        CANDIDATE_OAUTH_PASSWORD: password,
        CANDIDATE_BROWSER_OUTPUT: output,
      },
      privateOutput: () => {},
      timeout: 90000,
    },
    c.processes,
    "oauth",
  );
  await rm(output, { recursive: true, force: true });
  ensure(r.exitCode === 0 && r.complete);
}
async function main(c: GateContext) {
  candidateOrigin = new URL(c.url).origin;
  const clientQuery = async (connection: Connection, soql: string) => {
    assertCandidateOAuthEndpoints(connection.oauth2, c.url);
    ensure(new URL(connection.instanceUrl).origin === candidateOrigin);
    return connection.query(soql);
  };
  const steps: string[] = [];
  for (const token of Object.values(c.tokens)) secrets.add(token);
  const passed = (id: string) => {
    steps.push("oauth/" + id);
    console.error("OAuth passed: " + id);
  };
  const retainPrincipal = async (kind: "users" | "apps", id: string) => {
    const path = join(c.scratch, "principals.json");
    const record = JSON.parse(await readFile(path, "utf8"));
    record[kind] = [...(record[kind] ?? []), id];
    await writeFile(path, JSON.stringify(record), { mode: 0o600 });
  };
  const command = async (args: string[], env: NodeJS.ProcessEnv = {}) => {
    let stdout = "";
    const r = await runCommand(
      {
        executable: c.binary,
        argv: args,
        cwd: ROOT,
        env: { ...c.env, ...env },
        privateOutput: (out) => {
          stdout = out;
        },
      },
      c.processes,
      "oauth",
    );
    ensure(r.exitCode === 0 && r.complete);
    return stdout.trim();
  };
  const password = "P26!" + randomBytes(24).toString("hex");
  secrets.add(password);
  const users = {
    admin: "p26-" + c.runId + "-admin@example.invalid",
    restricted: "p26-" + c.runId + "-restricted@example.invalid",
    other: "p26-" + c.runId + "-other@example.invalid",
  };
  const userIDs: string[] = [];
  for (const role of ["admin", "restricted", "other"] as const) {
    userIDs.push(
      await command(["user", "create"], {
        SALESCOMPEL_TENANT:
          role === "other" ? c.tenants.other : c.tenants.first,
        SALESCOMPEL_USERNAME: users[role],
        SALESCOMPEL_PASSWORD: password,
        SALESCOMPEL_ROLE: role === "restricted" ? "restricted" : "admin",
      }),
    );
    await retainPrincipal("users", userIDs.at(-1)!);
    await appendFile(
      join(c.scratch, "oauth-principals.jsonl"),
      JSON.stringify({
        user: userIDs.at(-1),
        username: users[role],
        tenant: role === "other" ? c.tenants.other : c.tenants.first,
        retained: true,
      }) + "\n",
      { mode: 0o600 },
    );
  }
  let callbackResolve: ((url: URL) => void) | undefined;
  const callback = createServer((req, res) => {
    const url = new URL(req.url!, "http://localhost");
    res.end("OAuth complete");
    callbackResolve?.(url);
  });
  await new Promise<void>((resolve) =>
    callback.listen(0, "127.0.0.1", resolve),
  );
  try {
    const address = callback.address() as { port: number };
    const redirect = `http://127.0.0.1:${address.port}/callback`;
    const app = remember(
      JSON.parse(
        await command([
          "oauth-app",
          "create",
          "--tenant",
          c.tenants.first,
          "--name",
          "p26-" + c.runId,
          "--redirect-uri",
          redirect,
          "--redirect-uri",
          "http://localhost:1717/OauthRedirect",
          "--grants",
          "authorization_code,refresh_token,urn:ietf:params:oauth:grant-type:jwt-bearer,client_credentials",
          "--scopes",
          "api,web,refresh_token,id",
          "--jwt-cert",
          join(c.scratch, "cert.pem"),
          "--jwt-user",
          users.admin,
          "--run-as",
          users.admin,
        ]),
      ),
    );
    await appendFile(
      join(c.scratch, "oauth-principals.jsonl"),
      JSON.stringify({
        app: app.client_id,
        tenant: c.tenants.first,
        retained: true,
      }) + "\n",
      { mode: 0o600 },
    );
    await retainPrincipal("apps", app.client_id);
    passed("provision");
    const account = await createFixture(
      c,
      "oauth-account",
      "Account",
      { Name: "p26-" + c.runId + "-oauth" },
      ["p26-" + c.runId + "-oauth"],
    );
    const doc = await request(
      c.url + "/services/data/v65.0/openapi.yaml",
      "GET",
      undefined,
      { Authorization: "Bearer " + c.tokens.admin },
    );
    // The served contract is fetched independently of the repository's protected API.
    ensure(doc.status === 200);
    validator = await createCandidateTrafficValidator(parseYAML(doc.text));
    const sfHome = join(c.scratch, "oauth-sf-home"),
      launchers = join(c.scratch, "oauth-launchers"),
      launchFile = join(c.scratch, "oauth-launch-url");
    await mkdir(sfHome, { mode: 0o700 });
    await mkdir(launchers, { mode: 0o700 });
    for (const name of [
      "xdg-open",
      "open",
      "start",
      "sensible-browser",
      "x-www-browser",
      "www-browser",
      "browser",
      "kde-open",
      "kde-open5",
      "kde-open6",
      "kioclient",
      "kioclient5",
      "kioclient6",
      "gio",
    ]) {
      const file = join(launchers, name);
      await writeFile(
        file,
        '#!/bin/sh\numask 077\nfor arg do last="$arg"; done\nprintf "%s\\n" "$last" > "$CANDIDATE_OAUTH_LAUNCH_FILE"\n',
        { mode: 0o700 },
      );
      await chmod(file, 0o700);
    }
    const sfEnv = {
      ...c.env,
      HOME: sfHome,
      SF_STATE_FOLDER: join(sfHome, "sf"),
      SF_USE_GENERIC_UNIX_KEYCHAIN: "true",
      PATH: launchers + ":" + c.env.PATH,
      CANDIDATE_OAUTH_LAUNCH_FILE: launchFile,
    };
    const sf = async (args: string[], allowFailure = false) => {
      let stdout = "";
      const r = await runCommand(
        {
          executable: process.execPath,
          argv: [SF_BIN, ...args, "--json"],
          cwd: c.scratch,
          env: sfEnv,
          privateOutput: (out) => {
            stdout = out;
          },
          timeout: 120000,
        },
        c.processes,
        "oauth",
      );
      ensure(r.complete);
      let parsed: any;
      try {
        parsed = remember(JSON.parse(stdout));
      } catch {
        if (!allowFailure) throw new Error("RESULT");
      }
      if (!allowFailure && r.exitCode !== 0) {
        console.error(
          "sf command failure: " +
            JSON.stringify({
              command: args.slice(0, 3),
              exit: r.exitCode,
              name: parsed?.name,
              code: parsed?.code,
            }),
        );
        throw new Error("ASSERTION");
      }
      return { exit: r.exitCode, result: parsed?.result };
    };
    const alias = "oauth-" + c.runId;
    const login = sf([
      "org",
      "login",
      "web",
      "--instance-url",
      c.url,
      "--alias",
      alias,
    ]);
    void login.catch(() => {});
    let authorize = "";
    for (let i = 0; i < 300; i++) {
      try {
        authorize = parseLauncherURL(await readFile(launchFile, "utf8"), c.url);
        break;
      } catch {}
      await new Promise((r) => setTimeout(r, 100));
    }
    ensure(authorize);
    await browser(c, authorize, users.admin, password);
    await login;
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
    secrets.add(access);
    await revoke(c, access);
    await sfQuery();
    passed("sf-refresh");
    await sf([
      "org",
      "login",
      "jwt",
      "--instance-url",
      c.url,
      "--client-id",
      app.client_id,
      "--jwt-key-file",
      join(c.scratch, "key.pem"),
      "--username",
      users.admin,
      "--alias",
      alias + "-jwt",
    ]);
    passed("sf-jwt");
    ensure(
      (
        await sf(
          [
            "org",
            "login",
            "jwt",
            "--instance-url",
            c.url,
            "--client-id",
            app.client_id,
            "--jwt-key-file",
            join(c.scratch, "key.pem"),
            "--username",
            users.restricted,
            "--alias",
            alias + "-unlisted",
          ],
          true,
        )
      ).exit !== 0,
    );
    ensure(
      (await sf(["org", "display", "--target-org", alias + "-unlisted"], true))
        .exit !== 0,
    );
    passed("sf-jwt-unlisted");
    const oauth2 = new OAuth2({
      loginUrl: c.url,
      clientId: app.client_id,
      clientSecret: app.client_secret,
      redirectUri: redirect,
      useVerifier: true,
    });
    assertCandidateOAuthEndpoints(oauth2, c.url);
    const webURL = oauth2.getAuthorizationUrl({
      scope: "api web refresh_token",
      state: randomBytes(16).toString("hex"),
    });
    const callbackURL = new Promise<URL>((resolve) => {
      callbackResolve = resolve;
    });
    await browser(c, webURL, users.admin, password);
    console.error("OAuth JSforce browser passed");
    const code = (await callbackURL).searchParams.get("code");
    ensure(code);
    secrets.add(code!);
    const connection = new Connection({ oauth2, version: "65.0" });
    assertCandidateOAuthEndpoints(connection.oauth2, c.url);
    await connection.authorize(code!);
    console.error("OAuth JSforce authorize passed");
    remember({
      accessToken: connection.accessToken,
      refreshToken: connection.refreshToken,
    });
    ensure(
      (
        await clientQuery(
          connection,
          `SELECT Id FROM Account WHERE Id = '${account}'`,
        )
      ).records.length === 1,
    );
    passed("jsforce-web-pkce");
    const refresh = connection.refreshToken;
    ensure(refresh);
    await revoke(c, connection.accessToken!);
    ensure(
      (
        await clientQuery(
          connection,
          `SELECT Id FROM Account WHERE Id = '${account}'`,
        )
      ).records.length === 1,
    );
    remember({ accessToken: connection.accessToken });
    passed("jsforce-refresh");
    const key = await readFile(join(c.scratch, "key.pem"), "utf8");
    const jwt = [
      Buffer.from(JSON.stringify({ alg: "RS256", typ: "JWT" })).toString(
        "base64url",
      ),
      Buffer.from(
        JSON.stringify({
          iss: app.client_id,
          sub: users.admin,
          aud: c.url,
          exp: Math.floor(Date.now() / 1000) + 120,
        }),
      ).toString("base64url"),
    ].join(".");
    const assertion =
      jwt + "." + createSign("RSA-SHA256").update(jwt).sign(key, "base64url");
    secrets.add(assertion);
    const jwtOAuth = new OAuth2({
      loginUrl: c.url,
      clientId: app.client_id,
      clientSecret: app.client_secret,
    });
    assertCandidateOAuthEndpoints(jwtOAuth, c.url);
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
      (await clientQuery(jwtConn, "SELECT Id FROM Account")).records.length > 0,
    );
    passed("jsforce-jwt");
    const ccOAuth = new OAuth2({
      loginUrl: c.url,
      clientId: app.client_id,
      clientSecret: app.client_secret,
    });
    assertCandidateOAuthEndpoints(ccOAuth, c.url);
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
      (await clientQuery(cc, "SELECT Id FROM Account")).records.length > 0,
    );
    passed("jsforce-client-credentials");
    const revoked = cc.accessToken!;
    assertCandidateOAuthEndpoints(cc.oauth2, c.url);
    await cc.oauth2.revokeToken(revoked);
    ensure(
      (
        await request(c.url + "/services/oauth2/userinfo", "GET", undefined, {
          Authorization: "Bearer " + revoked,
        })
      ).status === 401,
    );
    passed("jsforce-revoke");
    const py = await runCommand(
      {
        executable: c.python,
        argv: ["scripts/candidate-oauth-python.py"],
        cwd: ROOT,
        env: {
          ...c.env,
          CANDIDATE_OAUTH_SESSION: connection.accessToken!,
          CANDIDATE_OAUTH_NAME: "p26-" + c.runId + "-python",
        },
        privateOutput: () => {},
      },
      c.processes,
      "oauth",
    );
    ensure(py.exitCode === 0 && py.complete);
    passed("python-session");
    // Raw public protocol exchanges are validated in memory, never serialized.
    async function rawLogin(
      username: string,
      client = app.client_id,
      secret = app.client_secret,
      callbackURI = redirect,
      scope = "api web refresh_token",
    ) {
      const verifier = randomBytes(48).toString("base64url");
      const auth = new URL(c.url + "/services/oauth2/authorize");
      auth.search = new URLSearchParams({
        client_id: client,
        redirect_uri: callbackURI,
        response_type: "code",
        scope,
        code_challenge: createHash("sha256")
          .update(verifier)
          .digest("base64url"),
        code_challenge_method: "S256",
      }).toString();
      const page = await request(auth.href);
      ensure(page.status === 200);
      const id = /name="request_id"\s+value="([^"]+)"/.exec(page.text)?.[1];
      ensure(id);
      const result = await request(
        c.url + "/services/oauth2/authorize",
        "POST",
        new URLSearchParams({
          request_id: id!,
          username,
          password,
          decision: "allow",
        }),
        {
          Origin: c.url,
          Cookie: page.headers.get("set-cookie")?.split(";")[0] ?? "",
        },
      );
      if (result.status !== 303) return { denied: true };
      const location = new URL(result.headers.get("location")!);
      if (!location.searchParams.get("code")) return { denied: true };
      secrets.add(location.searchParams.get("code")!);
      return await exchange(c, {
        grant_type: "authorization_code",
        client_id: client,
        client_secret: secret,
        redirect_uri: callbackURI,
        code: location.searchParams.get("code")!,
        code_verifier: verifier,
      });
    }
    const identityOnly = await rawLogin(
      users.admin,
      app.client_id,
      app.client_secret,
      redirect,
      "id",
    );
    ensure(identityOnly.access_token);
    for (const path of [
      "/services/data/v65.0/query?q=" +
        encodeURIComponent("SELECT Id FROM Account"),
      "/services/data/v65.0/openapi.yaml",
    ]) {
      const denied = await request(c.url + path, "GET", undefined, {
        Authorization: "Bearer " + identityOnly.access_token,
      });
      ensure(
        denied.status === 403 &&
          denied.json?.[0]?.errorCode === "INSUFFICIENT_ACCESS",
      );
    }
    console.error("OAuth id-only scope data/OpenAPI denial passed");
    const restricted = await rawLogin(users.restricted);
    ensure(restricted.access_token);
    const denied = await request(
      c.url +
        "/services/data/v65.0/query?q=" +
        encodeURIComponent("SELECT Compat_Text__c FROM Account"),
      "GET",
      undefined,
      { Authorization: "Bearer " + restricted.access_token },
    );
    ensure(denied.status === 400);
    passed("restricted-denied");
    ensure((await rawLogin(users.other)).denied === true);
    const foreign = await rawLogin(
      users.other,
      "PlatformCLI",
      "",
      "http://localhost:1717/OauthRedirect",
    );
    ensure(foreign.access_token);
    ensure(
      (
        await request(
          c.url + "/services/data/v65.0/sobjects/Account/" + account,
          "GET",
          undefined,
          { Authorization: "Bearer " + foreign.access_token },
        )
      ).status === 404,
    );
    passed("tenant-isolation");
    const identity = await request(
      c.url + "/services/oauth2/userinfo",
      "GET",
      undefined,
      { Authorization: "Bearer " + connection.accessToken },
    );
    ensure(identity.status === 200);
    ensure(
      (
        await request(
          identity.json.urls?.identity ??
            c.url +
              `/id/${identity.json.organization_id}/${identity.json.user_id}`,
          "GET",
          undefined,
          { Authorization: "Bearer " + connection.accessToken },
        )
      ).status === 200,
    );
    ensure(
      (await request(c.url + "/.well-known/openid-configuration")).status ===
        200,
    );
    ensure(
      (
        await request(
          c.url + "/services/oauth2/introspect",
          "POST",
          new URLSearchParams({
            token: connection.accessToken!,
            client_id: app.client_id,
            client_secret: app.client_secret,
          }),
        )
      ).json?.active === true,
    );
    for (const op of [
      "candidateOAuthAuthorizeForm",
      "candidateOAuthAuthorizeSubmit",
      "candidateOAuthToken",
      "candidateOAuthRevoke",
      "candidateOAuthIntrospect",
      "candidateOAuthDiscovery",
      "candidateOAuthIdentity",
    ])
      ensure(operations.has(op));
    passed("http-contract");
    // Verify every captured credential against evidence before returning sanitized IDs.
    const { readdir } = await import("node:fs/promises");
    for (const name of await readdir(c.scratch))
      if (name.startsWith("command-")) {
        const text = await readFile(join(c.scratch, name), "utf8");
        ensure(redactOAuthEvidence(text, [...secrets]) === text);
      }
    const result = stepResult(c, "oauth", steps);
    ensure(
      redactOAuthEvidence(JSON.stringify(result), [...secrets]) ===
        JSON.stringify(result),
    );
    console.error("OAuth evidence secret scan passed");
    return {
      result,
      secrets: [...secrets],
      restart: {
        refresh,
        client: app.client_id,
        secret: app.client_secret,
        account,
      },
    };
  } finally {
    await new Promise<void>((resolve) => callback.close(() => resolve()));
  }
}
async function entry() {
  if (process.argv[2] === "restart") {
    const state = JSON.parse(process.env.CANDIDATE_OAUTH_RESTART!);
    const c = { url: process.env.CANDIDATE_URL! } as GateContext;
    candidateOrigin = new URL(c.url).origin;
    const t = await exchange(c, {
      grant_type: "refresh_token",
      refresh_token: state.refresh,
      client_id: state.client,
      client_secret: state.secret,
    });
    await query(c, t.access_token, state.account);
    process.stdout.write(
      JSON.stringify({
        verified: true,
        secrets: [
          t.access_token,
          ...(t.refresh_token ? [t.refresh_token] : []),
        ],
      }),
    );
    return;
  }
  const c = JSON.parse(await readFile(process.argv[2], "utf8")) as GateContext;
  c.processes = newProcessContext(c.runId, c.scratch, c.deadline);
  c.processes.source = c.bindings.source;
  const stop = () => {
    c.processes.interrupted = true;
    // The outer owner may force-kill this worker after one second. Stop each
    // detached child group synchronously so grandchildren cannot outlive it.
    for (const child of c.processes.children)
      if (child.pid) {
        try {
          process.kill(-child.pid, "SIGKILL");
        } catch (error: any) {
          if (error.code !== "ESRCH") process.exitCode = 1;
        }
      }
    void stopOwnedProcesses(c.processes);
  };
  process.on("SIGINT", stop);
  process.on("SIGTERM", stop);
  try {
    process.stdout.write(JSON.stringify(await main(c)));
  } finally {
    await stopOwnedProcesses(c.processes);
    process.off("SIGINT", stop);
    process.off("SIGTERM", stop);
  }
}
entry().catch((error) => {
  const label = error?.errorCode ?? error?.name;
  if (typeof label === "string" && /^[A-Za-z_]{1,64}$/.test(label))
    console.error("OAuth error kind: " + label);
  process.stderr.write("OAuth acceptance assertion failed\n");
  process.exitCode = 1;
});
