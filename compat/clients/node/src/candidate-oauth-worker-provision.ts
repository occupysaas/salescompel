// Provisioning helpers: owner CLI commands, principal retention, browser
// launcher stubs, and the Salesforce CLI wrapper.
import {
  readFile,
  writeFile,
  mkdir,
  chmod,
  appendFile,
  rm,
} from "node:fs/promises";
import { join } from "node:path";
import { randomBytes } from "node:crypto";
import { runCommand } from "./candidate-acceptance-process.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import { ROOT, SF_BIN } from "./versions.ts";
import { parseLauncherURL } from "./candidate-oauth.ts";
import {
  addSecret,
  ensure,
  remember,
} from "./candidate-oauth-worker-runtime.ts";

export async function browser(
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

export type OwnerCommand = (
  args: string[],
  env?: NodeJS.ProcessEnv,
) => Promise<string>;

export function ownerCommand(c: GateContext): OwnerCommand {
  return async (args, env = {}) => {
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
}

export async function retainPrincipal(
  c: GateContext,
  kind: "users" | "apps",
  id: string,
) {
  const path = join(c.scratch, "principals.json");
  const record = JSON.parse(await readFile(path, "utf8"));
  record[kind] = [...(record[kind] ?? []), id];
  await writeFile(path, JSON.stringify(record), { mode: 0o600 });
}

export function createPassword(): string {
  const password = ["P26!", randomBytes(24).toString("hex")].join("");
  addSecret(password);
  return password;
}

export function candidateUsers(c: GateContext) {
  return {
    admin: "p26-" + c.runId + "-admin@example.invalid",
    restricted: "p26-" + c.runId + "-restricted@example.invalid",
    other: "p26-" + c.runId + "-other@example.invalid",
  };
}

export function candidateAppArgs(c: GateContext, redirect: string) {
  return [
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
    candidateUsers(c).admin,
    "--run-as",
    candidateUsers(c).admin,
  ];
}

// Every supported opener name gets a stub that records the last argument,
// so a CLI or browser attempting to open a URL leaves the URL in launchFile.
export async function installLauncherStubs(c: GateContext, launchers: string) {
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
}

export async function createSfHome(c: GateContext) {
  const sfHome = join(c.scratch, "oauth-sf-home"),
    launchers = join(c.scratch, "oauth-launchers"),
    launchFile = join(c.scratch, "oauth-launch-url");
  await mkdir(sfHome, { mode: 0o700 });
  await mkdir(launchers, { mode: 0o700 });
  await installLauncherStubs(c, launchers);
  const env = {
    ...c.env,
    HOME: sfHome,
    SF_STATE_FOLDER: join(sfHome, "sf"),
    SF_USE_GENERIC_UNIX_KEYCHAIN: "true",
    PATH: launchers + ":" + c.env.PATH,
    CANDIDATE_OAUTH_LAUNCH_FILE: launchFile,
  };
  return { env, launchFile };
}

export function salesforceCli(c: GateContext, sfEnv: NodeJS.ProcessEnv) {
  return async (args: string[], allowFailure = false) => {
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
    let parsed: Record<string, any> | undefined;
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
}

// Poll for the authorize URL that the login command tried to open.
export async function waitForAuthorize(c: GateContext, launchFile: string) {
  let authorize = "";
  for (let i = 0; i < 300; i++) {
    try {
      authorize = parseLauncherURL(await readFile(launchFile, "utf8"), c.url);
      break;
    } catch {}
    await new Promise((r) => setTimeout(r, 100));
  }
  ensure(authorize);
  return authorize;
}

export async function retainUser(
  c: GateContext,
  id: string,
  username: string,
  tenant: string,
) {
  await appendFile(
    join(c.scratch, "oauth-principals.jsonl"),
    JSON.stringify({ user: id, username, tenant, retained: true }) + "\n",
    { mode: 0o600 },
  );
}

export async function retainApp(
  c: GateContext,
  clientId: string,
  tenant: string,
) {
  await appendFile(
    join(c.scratch, "oauth-principals.jsonl"),
    JSON.stringify({ app: clientId, tenant, retained: true }) + "\n",
    { mode: 0o600 },
  );
}
