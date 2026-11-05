import assert from "node:assert/strict";
import { readFile, appendFile, open } from "node:fs/promises";
import { join } from "node:path";
import { createRequire } from "node:module";
import { runCommand } from "./candidate-acceptance-process.ts";
import { ROOT } from "./versions.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import type { ParsedResult } from "./candidate-acceptance-contract.ts";
import { stepResult } from "./candidate-acceptance-clients.ts";
import {
  candidateRequest,
  recordFixtureEvent,
  type FixtureEvent,
} from "./candidate-acceptance-fixtures.ts";
import { loadCandidateOpenAPI } from "./candidate-openapi-contract.ts";
import { createCandidateTrafficValidator } from "./candidate-openapi-traffic.ts";
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
async function ownerCommand(
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
type SchemaOperation = {
  Tenant: string;
  Kind: "object" | "field";
  Name: string;
  Object?: string;
};
async function schemaOperation(
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
  } catch (e: any) {
    if (e.code !== "ENOENT") throw e;
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
export async function runSchemaGate(c: GateContext): Promise<ParsedResult> {
  const ids: string[] = [];
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
  c.env.CANDIDATE_CUSTOM_OBJECT = object;
  ids.push("schema/provision");
  const { Connection } = createRequire(
    join(ROOT, "compat/clients/node/package.json"),
  )("jsforce");
  const conn = new Connection({
      instanceUrl: c.url,
      accessToken: c.tokens.admin,
      version: "65.0",
    }),
    other = new Connection({
      instanceUrl: c.url,
      accessToken: c.tokens.other,
      version: "65.0",
    });
  const global = await conn.describeGlobal();
  assert.ok(global.sobjects.some((o: any) => o.name === object));
  ids.push("schema/jsforce-discover");
  const describe = await conn.sobject(object).describe();
  assert.equal(
    describe.fields.find((f: any) => f.name === "Notes__c").length,
    80,
  );
  assert.equal(
    (await other.sobject(object).describe()).fields.find(
      (f: any) => f.name === "Notes__c",
    ).length,
    40,
  );
  ids.push("schema/jsforce-describe");
  let sequence = 0;
  const create = async (
    name: string,
    data: Record<string, unknown>,
    objectName = object,
  ) => {
    const e: FixtureEvent = {
      schemaVersion: 1,
      runId: c.runId,
      operationId: "schema-" + sequence++,
      principal: "admin",
      object: objectName,
      names: [name, name + " updated"],
      kind: "intent",
    };
    await recordFixtureEvent(c.fixtureJournal, e);
    const value = await conn
      .sobject(objectName)
      .create({ Name: name, ...data });
    assert.equal(value.success, true);
    await recordFixtureEvent(c.fixtureJournal, {
      ...e,
      kind: "known",
      id: value.id,
    });
    return value.id as string;
  };
  const name = "Schema " + c.runId;
  const id = await create(name, { Notes__c: "original" });
  assert.equal((await conn.sobject(object).retrieve(id)).Notes__c, "original");
  await conn.sobject(object).update({ Id: id, Notes__c: "updated" });
  assert.equal((await conn.sobject(object).retrieve(id)).Notes__c, "updated");
  await conn.sobject(object).destroy(id);
  ids.push("schema/jsforce-crud");
  const retained = await create(name + " restart", { Notes__c: "persistent" });
  const query = await conn.query(
    `SELECT Id,Name,Notes__c FROM ${object} WHERE Name = '${name} restart' ORDER BY Name LIMIT 10`,
  );
  assert.equal(query.records.length, 1);
  assert.equal(query.records[0].Id, retained);
  ids.push("schema/jsforce-query");
  const account = await create(
    "Schema account " + c.runId,
    { [accountField]: "account custom" },
    "Account",
  );
  assert.equal(
    (await conn.sobject("Account").retrieve(account))[accountField],
    "account custom",
  );
  ids.push("schema/account-field");
  await assert.rejects(other.sobject(object).retrieve(retained));
  assert.equal(
    (await other.query(`SELECT Id FROM ${object} LIMIT 10`)).records.length,
    0,
  );
  ids.push("schema/tenant-isolation");
  const rollbackName = "Schema rollback " + c.runId;
  const intent: FixtureEvent = {
    schemaVersion: 1,
    runId: c.runId,
    operationId: "schema-rollback",
    principal: "admin",
    object,
    names: [rollbackName],
    kind: "intent",
  };
  await recordFixtureEvent(c.fixtureJournal, intent);
  const composite = await candidateRequest(
    c,
    "admin",
    "/services/data/v65.0/composite",
    "POST",
    {
      allOrNone: true,
      compositeRequest: [
        {
          method: "POST",
          url: `/services/data/v65.0/sobjects/${object}`,
          referenceId: "first",
          body: { Name: rollbackName },
        },
        {
          method: "POST",
          url: `/services/data/v65.0/sobjects/${object}`,
          referenceId: "bad",
          body: { Name: "Bad", Notes__c: "x".repeat(81) },
        },
      ],
    },
  );
  assert.equal(composite.status, 200);
  assert.ok(
    composite.body.compositeResponse.every(
      (r: any) => r.httpStatusCode === 400,
    ),
  );
  assert.equal(
    (
      await conn.query(
        `SELECT Id FROM ${object} WHERE Name = '${rollbackName}' LIMIT 10`,
      )
    ).records.length,
    0,
  );
  await recordFixtureEvent(c.fixtureJournal, { ...intent, kind: "rejected" });
  ids.push("schema/composite-rollback");
  const v = await createCandidateTrafficValidator(await loadCandidateOpenAPI());
  for (const [path, method, body] of [
    [`sobjects/${object}/describe`, "GET", undefined],
    [`sobjects/${object}/${retained}`, "GET", undefined],
    [`sobjects/${object}/${retained}`, "PATCH", { Notes__c: "persistent" }],
    [`sobjects/${object}/${retained}`, "PATCH", { Notes__c: "x".repeat(81) }],
  ] as const) {
    const response = await candidateRequest(
      c,
      "admin",
      "/services/data/v65.0/" + path,
      method,
      body,
    );
    assert.ok(response.exchange);
    const result = v.validate(response.exchange!);
    assert.deepEqual(result.requestIssues, []);
    assert.deepEqual(result.responseIssues, []);
  }
  ids.push("schema/http-contract");
  const discovery = await candidateRequest(
    c,
    "admin",
    "/services/data/v65.0/sobjects",
  );
  const before = JSON.parse(
    await ownerCommand(c, ["schema", "inspect", "--tenant", c.tenants.first]),
  );
  await appendFile(
    join(c.scratch, "schema-restart.json"),
    JSON.stringify({
      object,
      id: retained,
      name: name + " restart",
      discovery: discovery.body,
      revision: before.Revision,
    }) + "\n",
    { mode: 0o600 },
  );
  return stepResult(c, "schema", ids);
}
export async function verifySchemaRestart(
  c: GateContext,
): Promise<ParsedResult> {
  const saved = JSON.parse(
    await readFile(join(c.scratch, "schema-restart.json"), "utf8"),
  );
  const r = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/sobjects/${saved.object}/${saved.id}`,
  );
  assert.equal(r.status, 200);
  assert.equal(r.body.Id, saved.id);
  assert.equal(r.body.Name, saved.name);
  assert.equal(r.body.Notes__c, "persistent");
  assert.deepEqual(
    (await candidateRequest(c, "admin", "/services/data/v65.0/sobjects")).body,
    saved.discovery,
  );
  const after = JSON.parse(
    await ownerCommand(
      c,
      ["schema", "inspect", "--tenant", c.tenants.first],
      "restart",
    ),
  );
  assert.equal(after.Revision, saved.revision);
  return stepResult(c, "restart", ["restart/schema"]);
}
