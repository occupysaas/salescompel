import { execFile } from "node:child_process";
import { promisify } from "node:util";
import {
  mkdtemp,
  mkdir,
  writeFile,
  readFile,
  rm,
  access,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { ROOT, SF_BIN, pythonLockedVersion } from "./versions.ts";
import {
  Blocked,
  create,
  expect,
  MISSING_RECORD_CODES,
  type Context,
} from "./scenarios.ts";
const exec = promisify(execFile);
export async function withCli<T>(
  c: Context,
  action: (run: (args: string[]) => Promise<any>, dir: string) => Promise<T>,
): Promise<T> {
  const dir = await mkdtemp(join(tmpdir(), "salescompel-cli-"));
  const alias = "compat-fixture";
  const env = {
    ...process.env,
    ...c.env,
    HOME: dir,
    USERPROFILE: dir,
    XDG_CONFIG_HOME: join(dir, ".config"),
    SF_DISABLE_TELEMETRY: "true",
    SF_DISABLE_AUTOUPDATE: "true",
    SF_AUTOUPDATE_DISABLE: "true",
    SF_ACCESS_TOKEN: c.conn.accessToken!,
    SF_USE_GENERIC_UNIX_KEYCHAIN: "true",
  };
  async function run(args: string[]) {
    try {
      const { stdout } = await exec(
        process.execPath,
        [SF_BIN, ...args, "--json"],
        { cwd: dir, env, timeout: 360_000, maxBuffer: 8 * 1024 * 1024 },
      );
      const data = JSON.parse(stdout);
      if (data.status !== 0) throw new Error(`CLI status ${data.status}`);
      return data.result;
    } catch (e: any) {
      // Never include child command/environment/stdout in error messages.
      let code = "CLI_PROCESS_FAILED";
      try {
        code = JSON.parse(e.stdout).name ?? code;
      } catch {}
      throw new Error(`${code}: ${args.slice(0, 3).join(" ")}`);
    }
  }
  try {
    await run([
      "org",
      "login",
      "access-token",
      "--instance-url",
      c.conn.instanceUrl,
      "--alias",
      alias,
      "--no-prompt",
    ]);
    const display = await run(["org", "display", "--target-org", alias]);
    if (
      new URL(display.instanceUrl).origin !== new URL(c.conn.instanceUrl).origin
    )
      throw new Error("CLI endpoint differs from configured target");
    return await action(run, dir);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
}
export async function cliQuery(c: Context) {
  const id = await create(
    c,
    "Account",
    { Name: `SC ${c.runId} cli` },
    "account",
  );
  await withCli(c, async (run) => {
    const q = await run([
      "data",
      "query",
      "--query",
      `SELECT Id,Name FROM Account WHERE Id = '${id}'`,
      "--target-org",
      "compat-fixture",
      "--api-version",
      c.conn.version,
    ]);
    c.result.observations = {
      records: q.records.map((r: any) => ({
        Id: r.Id,
        Name: r.Name === `SC ${c.runId} cli` ? "SC <run> cli" : r.Name,
      })),
    };
    expect(
      c,
      "CLI queries owned record",
      q.records.length === 1 &&
        q.records[0].Id === id &&
        q.records[0].Name === `SC ${c.runId} cli`,
    );
  });
}
export async function metadata(c: Context) {
  const field = `Compat_Run_${c.runId.replace(/[^a-zA-Z0-9]/g, "")}__c`;
  const fullName = `Account.${field}`;
  await withCli(c, async (run, dir) => {
    const source = join(dir, "force-app/main/default/objects/Account/fields");
    await mkdir(source, { recursive: true });
    const file = join(source, `${field}.field-meta.xml`);
    await writeFile(
      join(dir, "sfdx-project.json"),
      JSON.stringify({
        packageDirectories: [{ path: "force-app", default: true }],
        namespace: "",
        sourceApiVersion: c.conn.version,
      }),
    );
    await writeFile(
      file,
      '<?xml version="1.0" encoding="UTF-8"?>\n<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><fullName>' +
        field +
        "</fullName><label>Compatibility run</label><length>80</length><type>Text</type></CustomField>",
    );
    // Metadata API reads are not filtered by field-level security; a newly
    // deployed field has no FLS grants, so describe would not show it.
    const readField = async () => {
      const r: any = await c.conn.metadata.read("CustomField", fullName);
      return r?.fullName === fullName ? r : undefined;
    };
    if (await readField())
      throw new Blocked(
        "Unique fixture field already exists; refusing to modify it",
      );
    // Track before deploy to cover timeouts after a successful server-side commit.
    await c.tracker.add(`CustomField:${fullName}`, async () => {
      const r: any = await c.conn.metadata.delete("CustomField", fullName);
      // A single SOAP error may arrive as an object rather than an array.
      const errors: any[] = [].concat(r.errors ?? []);
      if (
        !r.success &&
        !(errors.length && errors.every((e) => e.statusCode === "NOT_FOUND"))
      )
        throw new Error("Metadata cleanup failed");
    });
    const deployed = await run([
      "project",
      "deploy",
      "start",
      "--source-dir",
      file,
      "--target-org",
      "compat-fixture",
      "--api-version",
      c.conn.version,
      "--wait",
      "5",
    ]);
    expect(
      c,
      "Metadata deploy succeeded",
      deployed.success === true || deployed.status === "Succeeded",
    );
    // Remove only the local generated fixture so retrieval cannot pass using stale input.
    await rm(file);
    await run([
      "project",
      "retrieve",
      "start",
      "--metadata",
      `CustomField:${fullName}`,
      "--target-org",
      "compat-fixture",
      "--api-version",
      c.conn.version,
      "--wait",
      "5",
    ]);
    const xml = await readFile(file, "utf8");
    const fieldInfo = await readField();
    const retrievedType = /<type>Text<\/type>/.test(xml),
      retrievedLength = /<length>80<\/length>/.test(xml);
    c.result.observations = {
      type: fieldInfo?.type,
      length: fieldInfo ? Number(fieldInfo.length) : undefined,
      label: fieldInfo?.label,
      retrievedType,
      retrievedLength,
    };
    expect(
      c,
      "Deployed field is Text(80) with its label",
      fieldInfo?.type === "Text" &&
        Number(fieldInfo?.length) === 80 &&
        fieldInfo?.label === "Compatibility run",
    );
    expect(c, "Retrieved source is Text(80)", retrievedType && retrievedLength);
  });
}
// Every Account the probe journaled is either confirmed deleted by the probe or
// handed to the parent tracker for another ID-scoped attempt. This also covers
// a probe that crashed or timed out before reporting its own cleanup.
export async function adoptPythonFixtures(
  c: Context,
  journal: string,
  reported: { resource: string; outcome: string }[],
) {
  const resources = new Set(reported.map((r) => r.resource));
  let text = "";
  try {
    text = await readFile(journal, "utf8");
  } catch {}
  for (const line of text.split("\n")) {
    try {
      const entry = JSON.parse(line);
      if (typeof entry.resource === "string") resources.add(entry.resource);
    } catch {}
  }
  for (const resource of resources) {
    const [type, id] = resource.split(":");
    if (
      reported.some((r) => r.resource === resource && r.outcome === "passed")
    ) {
      c.cleanup.push({ resource, outcome: "passed", scenario: c.result.id });
      continue;
    }
    await c.tracker.add(resource, async () => {
      if (type !== "Account" || !/^[a-zA-Z0-9]{15,18}$/.test(id ?? ""))
        throw new Error("Invalid cleanup resource");
      try {
        const deleted: any = await c.conn.sobject("Account").destroy(id);
        if (!deleted.success) throw new Error("Python cleanup retry failed");
      } catch (e: any) {
        if (!MISSING_RECORD_CODES.includes(e.errorCode)) throw e;
      }
    });
  }
}
export async function pythonProbe(c: Context, journal: string) {
  if (new URL(c.conn.instanceUrl).protocol !== "https:")
    throw new Blocked(
      "simple-salesforce always connects over HTTPS; serve the candidate over HTTPS with a trusted certificate",
    );
  const executable =
    c.env.COMPAT_PYTHON ??
    resolve(ROOT, "compat/clients/python/.venv/bin/python");
  try {
    await access(executable);
  } catch {
    throw new Blocked(
      "Install isolated Python requirements or set COMPAT_PYTHON",
    );
  }
  let data: any;
  try {
    const { stdout } = await exec(
      executable,
      [resolve(ROOT, "compat/clients/python/probe.py")],
      {
        env: {
          ...process.env,
          ...c.env,
          COMPAT_PROBE_URL: c.conn.instanceUrl,
          COMPAT_PROBE_TOKEN: c.conn.accessToken!,
          COMPAT_PROBE_JOURNAL: journal,
          COMPAT_RUN_ID: c.runId,
        },
        timeout: 180_000,
        maxBuffer: 1024 * 1024,
      },
    );
    data = JSON.parse(stdout);
  } catch {}
  await adoptPythonFixtures(c, journal, data?.cleanup ?? []);
  if (!data)
    throw new Error(
      "Python probe failed; journaled fixtures were queued for cleanup",
    );
  c.result.observations = data.observations;
  c.result.identities = data.identities;
  c.result.assertions = data.assertions;
  expect(c, "Python locked version", data.version === pythonLockedVersion());
  expect(c, "Python CRUD succeeded", data.outcome === "passed");
}
