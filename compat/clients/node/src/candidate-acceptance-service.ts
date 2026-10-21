import { readFile, writeFile, mkdir } from "node:fs/promises";
import { join } from "node:path";
import { ROOT } from "./versions.ts";
import { runCommand, spawnOwned } from "./candidate-acceptance-process.ts";
import {
  availablePort,
  type GateContext,
} from "./candidate-acceptance-preflight.ts";
import { candidateRequest } from "./candidate-acceptance-fixtures.ts";
export type OwnedService = ReturnType<typeof spawnOwned>;
export async function prepareCandidate(c: GateContext) {
  const run = async (executable: string, argv: string[], env = c.env) => {
    const r = await runCommand(
      { executable, argv, cwd: ROOT, env },
      c.processes,
      "bindings",
    );
    if (r.exitCode !== 0 || !r.complete) throw new Error("PROCESS");
  };
  await run("go", ["build", "-o", c.binary, "./cmd/salescompel"]);
  await run("openssl", [
    "req",
    "-x509",
    "-newkey",
    "rsa:2048",
    "-nodes",
    "-days",
    "1",
    "-keyout",
    join(c.scratch, "key.pem"),
    "-out",
    join(c.scratch, "cert.pem"),
    "-subj",
    "/CN=localhost",
    "-addext",
    "subjectAltName=DNS:localhost,IP:127.0.0.1",
  ]);
  await mkdir(join(c.scratch, "sf"), { mode: 0o700 });
  Object.assign(c.env, {
    CANDIDATE_URL: c.url,
    CANDIDATE_TOKEN: c.tokens.admin,
    CANDIDATE_RESTRICTED_TOKEN: c.tokens.restricted,
    CANDIDATE_OTHER_TOKEN: c.tokens.other,
    CANDIDATE_RUN_ID: c.runId,
    CANDIDATE_FIXTURE_JOURNAL: c.fixtureJournal,
    CANDIDATE_ACCEPTANCE: "1",
    NODE_EXTRA_CA_CERTS: join(c.scratch, "cert.pem"),
    REQUESTS_CA_BUNDLE: join(c.scratch, "cert.pem"),
    SF_STATE_FOLDER: join(c.scratch, "sf"),
    SF_USE_GENERIC_UNIX_KEYCHAIN: "true",
    CHROMIUM_PATH: c.chromium,
  });
  await writeFile(
    join(c.scratch, "principals.json"),
    JSON.stringify({
      tenants: c.tenants,
      users: ["005000000000001AAA", "005000000000002AAA"],
      retained: true,
    }),
    { flag: "wx", mode: 0o600 },
  );
  for (const [principal, tenant, role, user] of [
    ["admin", c.tenants.first, "admin", "005000000000001AAA"],
    ["restricted", c.tenants.first, "restricted", "005000000000002AAA"],
    ["other", c.tenants.other, "admin", "005000000000001AAA"],
  ] as const)
    await run(c.binary, ["provision"], {
      ...c.env,
      SALESCOMPEL_TENANT: tenant,
      SALESCOMPEL_USER: user,
      SALESCOMPEL_ROLE: role,
      SALESCOMPEL_TOKEN: c.tokens[principal],
    });
}
export async function startCandidate(c: GateContext): Promise<OwnedService> {
  await availablePort(c.port);
  const owned = spawnOwned(
    {
      executable: c.binary,
      argv: [
        "serve",
        "-addr",
        `127.0.0.1:${c.port}`,
        "-tls-cert",
        join(c.scratch, "cert.pem"),
        "-tls-key",
        join(c.scratch, "key.pem"),
        "-assets",
        join(ROOT, "web/dist"),
      ],
      cwd: ROOT,
      env: c.env,
    },
    c.processes,
    "bindings",
  );
  for (let i = 0; i < 100; i++) {
    if (owned.child.exitCode !== null || owned.child.signalCode)
      throw new Error("PROCESS");
    try {
      const r = await candidateRequest(c, "admin", "/services/oauth2/userinfo");
      if (r.status === 200 && r.body.organization_id === c.tenants.first)
        return owned;
    } catch {}
    await new Promise((r) => setTimeout(r, 100));
  }
  throw new Error("PROCESS");
}
export async function stopCandidate(s: OwnedService) {
  s.child.kill("SIGTERM");
  const force = setTimeout(() => s.terminate(), 10000);
  try {
    const r = await s.done;
    if (r.exitCode !== 0) throw new Error("PROCESS");
  } finally {
    clearTimeout(force);
  }
}
export async function restartCandidate(c: GateContext, s: OwnedService) {
  const pid = s.child.pid;
  await stopCandidate(s);
  const next = await startCandidate(c);
  if (next.child.pid === pid) throw new Error("ASSERTION");
  return next;
}
