// Run in a fresh Node process so NODE_EXTRA_CA_CERTS is loaded before JSforce.
// Credentials and protocol exchanges stay in memory; child output is private.
// This entry orchestrates the OAuth acceptance motion; helper modules own the
// HTTP exchanges, provisioning, and Salesforce/JSforce client flows.
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { createServer } from "node:http";
import { parse as parseYAML } from "yaml";
import {
  runCommand,
  newProcessContext,
  stopOwnedProcesses,
} from "./candidate-acceptance-process.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import { ROOT } from "./versions.ts";
import { stepResult } from "./candidate-acceptance-clients.ts";
import { createFixture } from "./candidate-acceptance-fixtures.ts";
import { redactOAuthEvidence } from "./candidate-oauth.ts";
import { createCandidateTrafficValidator } from "./candidate-openapi-traffic.ts";
import {
  addSecret,
  capturedSecrets,
  ensure,
  operationIds,
  remember,
  request,
  setCandidateOrigin,
  setValidator,
} from "./candidate-oauth-worker-runtime.ts";
import { exchange, query, rawLogin } from "./candidate-oauth-worker-http.ts";
import {
  browser,
  candidateAppArgs,
  candidateUsers,
  createPassword,
  createSfHome,
  ownerCommand,
  retainApp,
  retainPrincipal,
  retainUser,
  salesforceCli,
  waitForAuthorize,
} from "./candidate-oauth-worker-provision.ts";
import {
  jsforceClientCredentialsLogin,
  jsforceJwtLogin,
  jsforceWebLogin,
  salesforceJwtLogin,
  salesforceWebLogin,
  type Passed,
  type SalesforceCli,
} from "./candidate-oauth-worker-clients.ts";

type App = { client_id: string; client_secret: string };

async function provision(
  c: GateContext,
  redirect: string,
  command: ReturnType<typeof ownerCommand>,
  users: ReturnType<typeof candidateUsers>,
  passed: Passed,
) {
  const app = remember(
    JSON.parse(await command(candidateAppArgs(c, redirect))),
  );
  await retainApp(c, app.client_id, c.tenants.first);
  await retainPrincipal(c, "apps", app.client_id);
  passed("provision");
  return app as App & Record<string, any>;
}

async function provisionUsers(
  c: GateContext,
  command: ReturnType<typeof ownerCommand>,
  users: ReturnType<typeof candidateUsers>,
  password: string,
) {
  for (const role of ["admin", "restricted", "other"] as const) {
    const id = await command(["user", "create"], {
      SALESCOMPEL_TENANT: role === "other" ? c.tenants.other : c.tenants.first,
      SALESCOMPEL_USERNAME: users[role],
      SALESCOMPEL_PASSWORD: password,
      SALESCOMPEL_ROLE: role === "restricted" ? "restricted" : "admin",
    });
    await retainPrincipal(c, "users", id);
    await retainUser(
      c,
      id,
      users[role],
      role === "other" ? c.tenants.other : c.tenants.first,
    );
  }
}

async function salesforceFlows(
  c: GateContext,
  options: {
    app: App;
    account: string;
    password: string;
    users: ReturnType<typeof candidateUsers>;
    sf: SalesforceCli;
    authorize: string;
    passed: Passed;
  },
) {
  const { app, account, password, users, sf, authorize, passed } = options;
  const alias = "oauth-" + c.runId;
  await salesforceWebLogin(c, {
    alias,
    account,
    sf,
    authorize,
    browser: (url) => browser(c, url, users.admin, password),
    passed,
  });
  await salesforceJwtLogin(c, {
    alias,
    clientId: app.client_id,
    users,
    sf,
    passed,
  });
}

async function jsforceFlows(
  c: GateContext,
  options: {
    app: App;
    account: string;
    password: string;
    users: ReturnType<typeof candidateUsers>;
    redirect: string;
    browser: (url: string) => Promise<void>;
    callbackURL: Promise<URL>;
    userinfoStatus(token: string): Promise<number>;
    passed: Passed;
  },
) {
  const { app, account, password, users, redirect, passed } = options;
  const { connection, refresh } = await jsforceWebLogin(c, {
    app,
    account,
    redirect,
    browser: options.browser,
    callbackURL: options.callbackURL,
    passed,
  });
  await jsforceJwtLogin(c, app, users, passed);
  await jsforceClientCredentialsLogin(c, app, passed, options);
  return { connection, refresh };
}

async function userinfoStatus(c: GateContext, token: string) {
  return (
    await request(c.url + "/services/oauth2/userinfo", "GET", undefined, {
      Authorization: "Bearer " + token,
    })
  ).status;
}

async function pythonSession(
  c: GateContext,
  accessToken: string,
  passed: Passed,
) {
  const py = await runCommand(
    {
      executable: c.python,
      argv: ["scripts/candidate-oauth-python.py"],
      cwd: ROOT,
      env: {
        ...c.env,
        CANDIDATE_OAUTH_SESSION: accessToken,
        CANDIDATE_OAUTH_NAME: "p26-" + c.runId + "-python",
      },
      privateOutput: () => {},
    },
    c.processes,
    "oauth",
  );
  ensure(py.exitCode === 0 && py.complete);
  passed("python-session");
}

async function rawProtocolFlows(
  c: GateContext,
  options: {
    app: App;
    account: string;
    password: string;
    users: ReturnType<typeof candidateUsers>;
    redirect: string;
    accessToken: string;
    passed: Passed;
  },
) {
  const { app, account, password, users, redirect, accessToken, passed } =
    options;
  const login = (
    username: string,
    client = app.client_id,
    secret = app.client_secret,
    callbackURI = redirect,
    scope = "api web refresh_token",
  ) => rawLogin(c, password, username, client, secret, callbackURI, scope);
  const bearer = (token: string) => "Bearer " + token;
  const identityOnly = await login(
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
      Authorization: bearer(identityOnly.access_token),
    });
    ensure(
      denied.status === 403 &&
        denied.json?.[0]?.errorCode === "INSUFFICIENT_ACCESS",
    );
  }
  console.error("OAuth id-only scope data/OpenAPI denial passed");
  const restricted = await login(users.restricted);
  ensure(restricted.access_token);
  const denied = await request(
    c.url +
      "/services/data/v65.0/query?q=" +
      encodeURIComponent("SELECT Compat_Text__c FROM Account"),
    "GET",
    undefined,
    { Authorization: bearer(restricted.access_token) },
  );
  ensure(denied.status === 400);
  passed("restricted-denied");
  ensure((await login(users.other)).denied === true);
  const foreign = await login(
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
        { Authorization: bearer(foreign.access_token) },
      )
    ).status === 404,
  );
  passed("tenant-isolation");
  const identity = await request(
    c.url + "/services/oauth2/userinfo",
    "GET",
    undefined,
    { Authorization: bearer(accessToken) },
  );
  ensure(identity.status === 200);
  ensure(
    (
      await request(
        identity.json!.urls?.identity ??
          c.url +
            `/id/${identity.json!.organization_id}/${identity.json!.user_id}`,
        "GET",
        undefined,
        { Authorization: bearer(accessToken) },
      )
    ).status === 200,
  );
  ensure(
    (await request(c.url + "/.well-known/openid-configuration")).status === 200,
  );
  ensure(
    (
      await request(
        c.url + "/services/oauth2/introspect",
        "POST",
        new URLSearchParams({
          token: accessToken,
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
    ensure(operationIds().has(op));
  passed("http-contract");
}

// Verify every captured credential against evidence before returning sanitized IDs.
async function evidenceScan(c: GateContext) {
  const { readdir } = await import("node:fs/promises");
  for (const name of await readdir(c.scratch))
    if (name.startsWith("command-")) {
      const text = await readFile(join(c.scratch, name), "utf8");
      ensure(redactOAuthEvidence(text, capturedSecrets()) === text);
    }
}

async function main(c: GateContext) {
  setCandidateOrigin(c.url);
  const steps: string[] = [];
  for (const token of Object.values(c.tokens)) addSecret(token);
  const passed = (id: string) => {
    steps.push("oauth/" + id);
    console.error("OAuth passed: " + id);
  };
  const command = ownerCommand(c);
  const password = createPassword();
  const users = candidateUsers(c);
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
    await provisionUsers(c, command, users, password);
    const app = await provision(c, redirect, command, users, passed);
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
    setValidator(await createCandidateTrafficValidator(parseYAML(doc.text)));
    const { env: sfEnv, launchFile } = await createSfHome(c);
    const sf = salesforceCli(c, sfEnv);
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
    const authorize = await waitForAuthorize(c, launchFile);
    await salesforceFlows(c, {
      app,
      account,
      password,
      users,
      sf,
      authorize,
      passed,
    });
    const callbackURL = new Promise<URL>((resolve) => {
      callbackResolve = resolve;
    });
    const { connection, refresh } = await jsforceFlows(c, {
      app,
      account,
      password,
      users,
      redirect,
      browser: (url) => browser(c, url, users.admin, password),
      callbackURL,
      userinfoStatus: (token) => userinfoStatus(c, token),
      passed,
    });
    await pythonSession(c, connection.accessToken!, passed);
    // Raw public protocol exchanges are validated in memory, never serialized.
    await rawProtocolFlows(c, {
      app,
      account,
      password,
      users,
      redirect,
      accessToken: connection.accessToken!,
      passed,
    });
    await evidenceScan(c);
    const result = stepResult(c, "oauth", steps);
    ensure(
      redactOAuthEvidence(JSON.stringify(result), capturedSecrets()) ===
        JSON.stringify(result),
    );
    console.error("OAuth evidence secret scan passed");
    return {
      result,
      secrets: capturedSecrets(),
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
    setCandidateOrigin(c.url);
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
        } catch (error: unknown) {
          if ((error as { code?: string }).code !== "ESRCH")
            process.exitCode = 1;
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
