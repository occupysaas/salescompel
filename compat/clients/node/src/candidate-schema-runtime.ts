import { readFile, open } from "node:fs/promises";
import { join } from "node:path";
import { runCommand } from "./candidate-acceptance-process.ts";
import { ROOT } from "./versions.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
const runtime = new WeakMap<
  GateContext,
  { records: string; identity: string }
>();
export async function bootstrapSchemaDatabase(c: GateContext) {
  let output = "";
  const r = await runCommand(
    {
      executable: "go",
      argv: ["run", "./compat/schemafixture", "bootstrap"],
      cwd: ROOT,
      env: {
        ...c.env,
        MIGRATION_DATABASE_URL: c.database,
        CANDIDATE_DISPOSABLE: "yes",
        CANDIDATE_RUN_ID: c.runId,
      },
      privateOutput: (out) => {
        output = out;
      },
    },
    c.processes,
    "bindings",
  );
  if (r.exitCode !== 0 || !r.complete) throw new Error("PREREQUISITE");
  const urls = JSON.parse(output);
  runtime.set(c, urls);
  Object.assign(c.env, {
    MIGRATION_DATABASE_URL: c.database,
    TEST_RECORDS_URL: urls.records,
    TEST_IDENTITY_URL: urls.identity,
  });
}
export function recordRuntimeEnvironment(c: GateContext): NodeJS.ProcessEnv {
  const urls = runtime.get(c);
  if (!urls) throw new Error("PREREQUISITE");
  return runtimeEnvironment(c.env, urls);
}
export function runtimeEnvironment(
  base: NodeJS.ProcessEnv,
  urls: { records: string; identity: string },
): NodeJS.ProcessEnv {
  const env: NodeJS.ProcessEnv = {
    ...base,
    DATABASE_URL: urls.records,
    AUTH_DATABASE_URL: urls.identity,
  };
  for (const key of [
    "MIGRATION_DATABASE_URL",
    "TEST_DATABASE_URL",
    "TEST_RECORDS_URL",
    "TEST_IDENTITY_URL",
  ])
    delete env[key];
  return env;
}
export async function ownerCommand(
  c: GateContext,
  args: string[],
  gate: "schema" | "restart" = "schema",
) {
  let output = "";
  const r = await runCommand(
    {
      executable: c.binary,
      argv: args,
      cwd: ROOT,
      env: { ...c.env, MIGRATION_DATABASE_URL: c.database },
      privateOutput: (out) => {
        output = out;
      },
    },
    c.processes,
    gate,
  );
  if (r.exitCode !== 0 || !r.complete) throw new Error("PROCESS");
  return output;
}
export type SchemaOperation = {
  Tenant: string;
  Kind: "object" | "field" | "stored";
  Name: string;
  Object?: string;
};
export async function schemaOperation(
  c: GateContext,
  op: SchemaOperation,
  args: string[],
) {
  const journal = await open(
    join(c.scratch, "schema-fixtures.jsonl"),
    "a",
    0o600,
  );
  try {
    await journal.writeFile(JSON.stringify(op) + "\n");
    await journal.sync();
  } finally {
    await journal.close();
  }
  await ownerCommand(c, args);
}
export async function cleanupSchema(c: GateContext, roles = false) {
  let operations: SchemaOperation[] = [];
  try {
    operations = (
      await readFile(join(c.scratch, "schema-fixtures.jsonl"), "utf8")
    )
      .trim()
      .split("\n")
      .filter(Boolean)
      .map((s) => JSON.parse(s));
  } catch (e: unknown) {
    const err = e as { code?: string };
    if (err.code !== "ENOENT") throw e;
  }
  const r = await runCommand(
    {
      executable: "go",
      argv: [
        "run",
        "./compat/schemafixture",
        roles ? "roles-cleanup" : "metadata-cleanup",
      ],
      cwd: ROOT,
      env: {
        ...c.env,
        MIGRATION_DATABASE_URL: c.database,
        CANDIDATE_DISPOSABLE: "yes",
        CANDIDATE_RUN_ID: c.runId,
        CANDIDATE_SCHEMA_OPERATIONS: JSON.stringify(operations),
      },
      privateOutput: () => {},
    },
    c.processes,
    "cleanup",
  );
  if (r.exitCode !== 0 || !r.complete) throw new Error("CLEANUP");
}
