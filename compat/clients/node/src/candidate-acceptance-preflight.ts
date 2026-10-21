import { execFile } from "node:child_process";
import { promisify } from "node:util";
import {
  readFile,
  mkdtemp,
  realpath,
  lstat,
  readdir,
  writeFile,
} from "node:fs/promises";
import { join, resolve, dirname } from "node:path";
import { tmpdir } from "node:os";
import { createRequire } from "node:module";
import { createServer } from "node:net";
import { randomBytes } from "node:crypto";
import { ROOT, connectorVersions } from "./versions.ts";
import { digest } from "./candidate-acceptance-results.ts";
import {
  newProcessContext,
  runCommand,
  type ProcessContext,
} from "./candidate-acceptance-process.ts";
import type { Bindings, Scope } from "./candidate-acceptance-contract.ts";
const exec = promisify(execFile);
export interface AcceptanceOptions {
  scope: Scope;
  out: string;
  port?: number;
}
export interface GateContext {
  runId: string;
  scratch: string;
  deadline: number;
  database: string;
  url: string;
  port: number;
  python: string;
  chromium: string;
  env: NodeJS.ProcessEnv;
  processes: ProcessContext;
  fixtureJournal: string;
  tokens: { admin: string; restricted: string; other: string };
  tenants: { first: string; other: string };
  binary: string;
  bindings: Bindings;
  out: string;
}
const roots =
  /^(go\.(mod|sum)$|cmd\/|internal\/|scripts\/|compat\/|web\/|locales\/|\.github\/workflows\/platform\.yml$)/;
export async function collectSourceBindings(root = ROOT): Promise<Bindings> {
  const git = async (args: string[]) =>
    (await exec("git", args, { cwd: root, maxBuffer: 16 * 1024 * 1024 }))
      .stdout;
  const files = (await git(["ls-files", "-z"]))
    .split("\0")
    .filter((p) => roots.test(p) && !p.endsWith(".md"))
    .sort();
  const entries = [];
  for (const p of files)
    entries.push([p, digest(await readFile(join(root, p)))]);
  const untracked = (
    await git(["ls-files", "--others", "--exclude-standard", "-z"])
  )
    .split("\0")
    .filter((p) => roots.test(p) && !p.endsWith(".md"));
  const dirty = (
    await git(["status", "--porcelain", "--untracked-files=no"])
  ).trim();
  return {
    revision: (await git(["rev-parse", "HEAD"])).trim(),
    source: digest(JSON.stringify(entries)),
    clean: !dirty && !untracked.length,
    locks: Object.fromEntries(
      await Promise.all(
        [
          "go.mod",
          "go.sum",
          "web/package-lock.json",
          "compat/clients/node/package-lock.json",
          "compat/clients/python/requirements.lock",
          "internal/platform/catalog.go",
          "internal/platform/metadata.go",
          "internal/platform/catalog_migration.go",
          "internal/platform/catalog_v1.json",
          "internal/platform/schema.sql",
        ].map(async (p) => [p, digest(await readFile(join(root, p)))]),
      ),
    ),
    tools: {},
    binary: "",
    assets: "",
  };
}
export async function availablePort(port: number) {
  if (!Number.isInteger(port) || port < 1024 || port > 65535)
    throw new Error("PREREQUISITE");
  await new Promise<void>((resolve, reject) => {
    const s = createServer();
    s.once("error", () => reject(new Error("PREREQUISITE")));
    s.listen(port, "127.0.0.1", () =>
      s.close((e) => (e ? reject(new Error("PREREQUISITE")) : resolve())),
    );
  });
}
export function baseEnvironment(env: NodeJS.ProcessEnv) {
  const out: NodeJS.ProcessEnv = {};
  for (const key of [
    "PATH",
    "HOME",
    "GOPATH",
    "GOCACHE",
    "GOMODCACHE",
    "LANG",
    "LC_ALL",
    "TMPDIR",
    "SYSTEMROOT",
  ])
    if (env[key]) out[key] = env[key];
  return {
    ...out,
    GOTOOLCHAIN: "local",
    GOPROXY: "off",
    GOSUMDB: "off",
    GOFLAGS: "-mod=readonly",
    SF_DISABLE_TELEMETRY: "true",
    SF_DISABLE_AUTOUPDATE: "true",
    SF_AUTOUPDATE_DISABLE: "true",
    SF_DISABLE_UPDATE_CHECK: "true",
    NO_COLOR: "1",
  };
}
export async function validateOutputDestination(output: string) {
  const out = resolve(output),
    parent = dirname(out);
  if (
    out === ROOT.slice(0, -1) ||
    out.startsWith(ROOT) ||
    !output ||
    !output.endsWith(".json") ||
    (await realpath(parent)) !== parent
  )
    throw new Error("PREREQUISITE");
  try {
    await lstat(out);
    throw new Error("PUBLICATION");
  } catch (e: any) {
    if (e.code !== "ENOENT") throw e;
  }
}
export async function preflightAcceptance(
  options: AcceptanceOptions,
  env: NodeJS.ProcessEnv,
  onContext?: (c: GateContext) => void,
): Promise<GateContext> {
  if (
    env.CANDIDATE_DISPOSABLE !== "yes" ||
    !env.DATABASE_URL ||
    (env.TEST_DATABASE_URL && env.TEST_DATABASE_URL !== env.DATABASE_URL)
  )
    throw new Error("PREREQUISITE");
  await validateOutputDestination(options.out);
  const out = resolve(options.out);
  const bindings = await collectSourceBindings();
  if (options.scope === "full" && !bindings.clean) throw new Error("SOURCE");
  const scratch = await mkdtemp(join(tmpdir(), "salescompel-acceptance-")),
    runId = randomBytes(12).toString("hex"),
    deadline = Date.now() + 1200000,
    port = options.port ?? Number(env.CANDIDATE_PORT ?? 58443),
    python =
      env.CANDIDATE_PYTHON ??
      join(ROOT, "compat/clients/python/.venv/bin/python"),
    chromium =
      env.CHROMIUM_PATH ??
      createRequire(join(ROOT, "web/package.json"))(
        "playwright",
      ).chromium.executablePath();
  await availablePort(port);
  await availablePort(4173);
  const base = baseEnvironment(env);
  const toolEnv = {
    ...base,
    DATABASE_URL: env.DATABASE_URL,
    TEST_DATABASE_URL: env.DATABASE_URL,
  };
  const c: GateContext = {
    runId,
    scratch,
    deadline,
    database: env.DATABASE_URL,
    url: `https://localhost:${port}`,
    port,
    python,
    chromium,
    env: toolEnv,
    processes: newProcessContext(runId, scratch, deadline),
    fixtureJournal: join(scratch, "fixtures.jsonl"),
    tokens: {
      admin:
        "00D" + runId.slice(0, 12) + "EAA!" + randomBytes(32).toString("hex"),
      restricted: randomBytes(32).toString("hex"),
      other: randomBytes(32).toString("hex"),
    },
    tenants: {
      first: "00D" + runId.slice(0, 12) + "EAA",
      other: "00D" + runId.slice(12, 24) + "EAA",
    },
    binary: join(scratch, "salescompel"),
    bindings,
    out,
  };
  onContext?.(c);
  c.processes.source = bindings.source;
  await writeFile(c.fixtureJournal, "", { flag: "wx", mode: 0o600 });
  try {
    const versions = connectorVersions();
    for (const [name, pin] of Object.entries({
      jsforce: "3.11.0",
      "@salesforce/cli": "2.156.0",
      "@salesforce/plugin-data": "5.1.16",
      "@salesforce/plugin-deploy-retrieve": "4.2.15",
      "simple-salesforce": "1.12.9",
    }))
      if (versions[name] !== pin) throw new Error("PREREQUISITE");
    bindings.tools = versions;
    const checks: [string, string, string[]][] = [
      ["go", "go", ["version"]],
      ["node", process.execPath, ["--version"]],
      [
        "python",
        python,
        [
          "-c",
          'import sys,importlib.metadata as m; assert sys.version_info >= (3,12); assert m.version("simple-salesforce")=="1.12.9"; print("%s.%s.%s"%sys.version_info[:3])',
        ],
      ],
      ["openssl", "openssl", ["version"]],
      ["chromium", chromium, ["--version"]],
      ["postgres", "go", ["run", "./compat/candidatecheck"]],
    ];
    for (const [name, executable, argv] of checks) {
      const r = await runCommand(
        { executable, argv, cwd: ROOT, env: toolEnv, timeout: 120000 },
        c.processes,
        "bindings",
      );
      if (r.exitCode !== 0 || !r.complete) throw new Error("PREREQUISITE");
      const text = (await readFile(r.resultPath, "utf8")).trim();
      if (name === "postgres") {
        if (JSON.parse(text).major !== 17) throw new Error("PREREQUISITE");
        bindings.tools.postgres = "17";
      } else {
        const v = text.match(/\d+\.\d+(?:\.\d+)?/)?.[0];
        if (!v) throw new Error("PREREQUISITE");
        bindings.tools[name] = v;
      }
    }
    if (
      Number(bindings.tools.node.split(".")[0]) < 22 ||
      (Number(bindings.tools.node.split(".")[0]) === 22 &&
        Number(bindings.tools.node.split(".")[1]) < 12) ||
      Number(bindings.tools.go.split(".")[1]) < 25
    )
      throw new Error("PREREQUISITE");
    const launch = await runCommand(
      {
        executable: chromium,
        argv: [
          "--headless",
          "--no-sandbox",
          "--disable-gpu",
          "--disable-background-networking",
          "--user-data-dir=" + join(scratch, "chromium"),
          "--dump-dom",
          "about:blank",
        ],
        cwd: scratch,
        env: base,
        timeout: 20000,
      },
      c.processes,
      "bindings",
    );
    if (launch.exitCode !== 0) throw new Error("PREREQUISITE");
    await readFile(join(ROOT, "web/node_modules/playwright/package.json"));
    return c;
  } catch {
    throw Object.assign(new Error("PREREQUISITE"), { context: c });
  }
}
export async function assetDigest(
  dir = join(ROOT, "web/dist"),
): Promise<string> {
  const entries: string[][] = [];
  const walk = async (path: string) => {
    for (const d of (await readdir(path, { withFileTypes: true })).sort(
      (a, b) => a.name.localeCompare(b.name),
    )) {
      const p = join(path, d.name);
      if (d.isSymbolicLink()) throw new Error("SOURCE");
      if (d.isDirectory()) await walk(p);
      else entries.push([p.slice(dir.length), digest(await readFile(p))]);
    }
  };
  await walk(dir);
  if (!entries.length) throw new Error("SOURCE");
  return digest(JSON.stringify(entries));
}
