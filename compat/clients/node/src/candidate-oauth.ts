/** Treat the browser launcher file as untrusted; never browse another origin. */
export function parseLauncherURL(text: string, candidateURL: string): string {
  try {
    const raw = text.trim();
    if (!raw || /\s/.test(raw)) throw new Error();
    const url = new URL(raw),
      candidate = new URL(candidateURL);
    if (
      url.origin !== candidate.origin ||
      url.username ||
      url.password ||
      url.hash ||
      url.pathname !== "/services/oauth2/authorize"
    )
      throw new Error();
    return url.href;
  } catch {
    throw new Error("RESULT");
  }
}

/** Called before output reaches an evidence file, including encoded form values. */
export function redactOAuthEvidence(
  text: string,
  secrets: readonly string[],
): string {
  const variants = new Set<string>();
  for (const secret of secrets) {
    if (!secret) continue;
    variants.add(secret);
    variants.add(encodeURIComponent(secret));
    variants.add(new URLSearchParams({ v: secret }).toString().slice(2));
    variants.add(JSON.stringify(secret).slice(1, -1));
  }
  for (const variant of [...variants])
    variants.add(variant.replace(/%[0-9A-F]{2}/g, (s) => s.toLowerCase()));
  for (const secret of [...variants].sort((a, b) => b.length - a.length))
    text = text.split(secret).join("[REDACTED]");
  return text;
}

import { readFile, writeFile, rm, readdir } from "node:fs/promises";
import { join } from "node:path";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import type { ParsedResult } from "./candidate-acceptance-contract.ts";
import { runCommand } from "./candidate-acceptance-process.ts";
import { ROOT } from "./versions.ts";
import { stepResult } from "./candidate-acceptance-clients.ts";
const evidenceSecrets = new WeakMap<GateContext, string[]>();
const restartState = new WeakMap<
  GateContext,
  { refresh: string; client: string; secret: string; account: string }
>();

export async function runOAuthGate(c: GateContext): Promise<ParsedResult> {
  const input = join(c.scratch, "oauth-context.json");
  await writeFile(input, JSON.stringify({ ...c, processes: undefined }), {
    mode: 0o600,
    flag: "wx",
  });
  let output = "";
  try {
    const r = await runCommand(
      {
        executable: process.execPath,
        argv: ["--import", "tsx", "src/candidate-oauth-worker.ts", input],
        cwd: join(ROOT, "compat/clients/node"),
        env: c.env,
        privateOutput: (stdout, stderr) => {
          console.error(stderr);
          output = stdout;
        },
        timeout: 600000,
      },
      c.processes,
      "oauth",
    );
    if (r.exitCode !== 0 || !r.complete) throw new Error("ASSERTION");
    let data: any;
    try {
      data = JSON.parse(output);
    } catch {
      throw new Error("RESULT");
    }
    restartState.set(c, data.restart);
    evidenceSecrets.set(c, data.secrets);
    return data.result;
  } finally {
    await rm(input, { force: true });
  }
}

export async function verifyOAuthRestart(
  c: GateContext,
): Promise<ParsedResult> {
  const state = restartState.get(c);
  if (!state) throw new Error("RESULT");
  // A new child loads the private CA at startup, just like the real clients.
  let output = "";
  const r = await runCommand(
    {
      executable: process.execPath,
      argv: ["--import", "tsx", "src/candidate-oauth-worker.ts", "restart"],
      cwd: join(ROOT, "compat/clients/node"),
      env: { ...c.env, CANDIDATE_OAUTH_RESTART: JSON.stringify(state) },
      privateOutput: (stdout) => {
        output = stdout;
      },
    },
    c.processes,
    "restart",
  );
  restartState.delete(c);
  if (r.exitCode !== 0 || !r.complete) throw new Error("ASSERTION");
  let result: any;
  try {
    result = JSON.parse(output);
  } catch {
    throw new Error("RESULT");
  }
  if (result.verified !== true || !Array.isArray(result.secrets))
    throw new Error("RESULT");
  evidenceSecrets.set(c, [
    ...(evidenceSecrets.get(c) ?? []),
    ...result.secrets,
  ]);
  return stepResult(c, "restart", ["restart/oauth-refresh"]);
}

/** Fail before a client can transmit local credentials to a default remote URL. */
export function assertCandidateOAuthEndpoints(
  config: {
    loginUrl?: string;
    authzServiceUrl?: string;
    tokenServiceUrl?: string;
    revokeServiceUrl?: string;
  },
  candidateURL: string,
): void {
  try {
    const origin = new URL(candidateURL).origin;
    for (const endpoint of [
      config.loginUrl,
      config.authzServiceUrl,
      config.tokenServiceUrl,
      config.revokeServiceUrl,
    ]) {
      if (!endpoint) throw new Error();
      const url = new URL(endpoint);
      if (url.origin !== origin || url.username || url.password)
        throw new Error();
    }
  } catch {
    throw new Error("SOURCE");
  }
}

/** Audit exactly the files eligible as command/step evidence, plus the report. */
export async function verifyOAuthEvidence(
  c: GateContext,
  report: unknown,
): Promise<void> {
  const secrets = evidenceSecrets.get(c);
  if (!secrets?.length) throw new Error("RESULT");
  const check = (text: string) => {
    if (redactOAuthEvidence(text, secrets) !== text)
      throw new Error("ASSERTION");
  };
  check(JSON.stringify(report));
  for (const name of await readdir(c.scratch)) {
    if (
      name.startsWith("command-") ||
      /^(cli|oauth|restart)-result\.json(?:\.binding\.json)?$/.test(name)
    )
      check(await readFile(join(c.scratch, name), "utf8"));
  }
  evidenceSecrets.delete(c);
}
