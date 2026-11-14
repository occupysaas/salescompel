import assert from "node:assert/strict";
import { readFile, writeFile, rm } from "node:fs/promises";
import { join } from "node:path";
import { runCommand } from "./candidate-acceptance-process.ts";
import { ROOT } from "./versions.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import type { ParsedResult } from "./candidate-acceptance-contract.ts";
import { ownerCommand, schemaOperation } from "./candidate-schema-runtime.ts";
import {
  createFieldTypeFixtures,
  runFieldTypesPython,
} from "./candidate-schema-field-fixtures.ts";
import { createSoqlSchemaFixtures, runSoqlPython } from "./candidate-soql.ts";
export async function runSchemaGate(c: GateContext): Promise<ParsedResult> {
  const object = "P27_" + c.runId.slice(0, 12) + "__c",
    accountField = "P27_" + c.runId.slice(0, 12) + "_Note__c";
  for (const [tenant, label] of [
    [c.tenants.first, "Projects"],
    [c.tenants.other, "Other projects"],
  ]) {
    await schemaOperation(c, { Tenant: tenant, Kind: "object", Name: object }, [
      "schema",
      "object",
      "create",
      "--tenant",
      tenant,
      "--name",
      object,
      "--label",
      "Project",
      "--plural-label",
      label,
    ]);
    await schemaOperation(
      c,
      { Tenant: tenant, Kind: "field", Object: object, Name: "Notes__c" },
      [
        "schema",
        "field",
        "create",
        "--tenant",
        tenant,
        "--object",
        object,
        "--name",
        "Notes__c",
        "--label",
        "Notes",
        "--type",
        "Text",
        "--length",
        tenant === c.tenants.first ? "80" : "40",
      ],
    );
  }
  await schemaOperation(
    c,
    {
      Tenant: c.tenants.first,
      Kind: "field",
      Object: "Account",
      Name: accountField,
    },
    [
      "schema",
      "field",
      "create",
      "--tenant",
      c.tenants.first,
      "--object",
      "Account",
      "--name",
      accountField,
      "--label",
      "Plan 27 note",
      "--type",
      "Text",
      "--length",
      "80",
    ],
  );
  await schemaOperation(
    c,
    {
      Tenant: c.tenants.first,
      Kind: "field",
      Object: object,
      Name: "Markup__c",
    },
    [
      "schema",
      "field",
      "create",
      "--tenant",
      c.tenants.first,
      "--object",
      object,
      "--name",
      "Markup__c",
      "--label",
      "<img src=x onerror=alert(1)>",
      "--type",
      "Text",
      "--length",
      "80",
    ],
  );
  await schemaOperation(
    c,
    {
      Tenant: c.tenants.first,
      Kind: "field",
      Object: object,
      Name: "Compat_Key__c",
    },
    [
      "schema",
      "field",
      "create",
      "--tenant",
      c.tenants.first,
      "--object",
      object,
      "--name",
      "Compat_Key__c",
      "--label",
      "Project key",
      "--type",
      "Text",
      "--length",
      "80",
    ],
  );
  c.env.CANDIDATE_CUSTOM_OBJECT = object;
  await createFieldTypeFixtures(c);
  await createSoqlSchemaFixtures(c);
  const input = join(c.scratch, "schema-client-context.json");
  await writeFile(
    input,
    JSON.stringify({
      runId: c.runId,
      scratch: c.scratch,
      url: c.url,
      tokens: c.tokens,
      fixtureJournal: c.fixtureJournal,
    }),
    { mode: 0o600, flag: "wx" },
  );
  const env: NodeJS.ProcessEnv = { ...c.env };
  for (const key of [
    "MIGRATION_DATABASE_URL",
    "DATABASE_URL",
    "AUTH_DATABASE_URL",
    "TEST_DATABASE_URL",
    "TEST_RECORDS_URL",
    "TEST_IDENTITY_URL",
  ])
    delete env[key];
  let output = "",
    diagnostics = "";
  let result;
  try {
    result = await runCommand(
      {
        executable: process.execPath,
        argv: [
          "--import",
          join(ROOT, "compat/clients/node/node_modules/tsx/dist/loader.mjs"),
          join(ROOT, "compat/clients/node/src/candidate-schema-worker.ts"),
          input,
        ],
        cwd: ROOT,
        env,
        privateOutput: (out, err) => {
          output = out;
          diagnostics = err;
        },
      },
      c.processes,
      "schema",
    );
  } finally {
    await rm(input, { force: true });
  }
  for (const line of diagnostics.split("\n"))
    if (/^Schema (passed: [a-z/-]+|client failure: [A-Za-z0-9_-]+)$/.test(line))
      process.stderr.write(line + "\n");
  if (result.exitCode !== 0 || !result.complete) throw new Error("ASSERTION");
  const parsed = JSON.parse(output) as ParsedResult;
  assert.equal(parsed.runId, c.runId);
  assert.equal(parsed.suite, "schema");
  parsed.tests.unshift({ id: "schema/provision", outcome: "passed" });
  for (const id of await runFieldTypesPython(c))
    parsed.tests.push({ id, outcome: "passed" });
  for (const id of await runSoqlPython(c))
    parsed.tests.push({ id, outcome: "passed" });
  const state = JSON.parse(
    await readFile(join(c.scratch, "schema-client-state.json"), "utf8"),
  );
  const before = JSON.parse(
    await ownerCommand(c, ["schema", "inspect", "--tenant", c.tenants.first]),
  );
  await writeFile(
    join(c.scratch, "schema-restart.json"),
    JSON.stringify({ ...state, revision: before.Revision }),
    { mode: 0o600, flag: "wx" },
  );
  return parsed;
}
