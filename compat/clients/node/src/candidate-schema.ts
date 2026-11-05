import assert from "node:assert/strict";
import { readFile, appendFile, open, writeFile, rm } from "node:fs/promises";
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

export type SchemaClientContext = Pick<
  GateContext,
  "runId" | "scratch" | "url" | "tokens" | "fixtureJournal"
>;
export async function runSchemaClientGate(
  c: SchemaClientContext,
): Promise<ParsedResult> {
  const ids: string[] = [];
  const passed = (id: string) => {
    ids.push(id);
    process.stderr.write("Schema passed: " + id + "\n");
  };
  const object = "P27_" + c.runId.slice(0, 12) + "__c",
    accountField = "P27_" + c.runId.slice(0, 12) + "_Note__c";
  const v = await createCandidateTrafficValidator(await loadCandidateOpenAPI());
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
  const observed = new Set<string>();
  const issues: unknown[] = [];
  const observers: Promise<void>[] = [];
  for (const connection of [conn, other]) {
    const transport = connection._transport,
      original = transport.httpRequest.bind(transport);
    transport.httpRequest = (req: any, options: any) => {
      const target = new URL(req.url);
      assert.equal(target.origin, new URL(c.url).origin);
      const pending = original(req, options);
      observers.push(
        pending.then(
          (res: any) => {
            const result = v.validate({
              method: req.method,
              path: target.pathname + target.search,
              request: {
                contentType:
                  req.headers?.["Content-Type"] ??
                  req.headers?.["content-type"] ??
                  null,
                body:
                  req.body === undefined ? undefined : Buffer.from(req.body),
              },
              response: {
                status: res.statusCode,
                contentType: res.headers["content-type"] ?? null,
                body: Buffer.from(res.body ?? ""),
              },
            });
            if (result.operationId) observed.add(result.operationId);
            if (result.requestIssues.length || result.responseIssues.length)
              issues.push(result);
          },
          () => {
            issues.push({ transport: "failed" });
          },
        ),
      );
      return pending;
    };
  }
  const global = await conn.describeGlobal();
  assert.ok(global.sobjects.some((o: any) => o.name === object));
  passed("schema/jsforce-discover");
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
  passed("schema/jsforce-describe");
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
  passed("schema/jsforce-crud");
  const retained = await create(name + " restart", { Notes__c: "persistent" });
  const query = await conn.query(
    `SELECT Id,Name,Notes__c FROM ${object} WHERE Name = '${name} restart' ORDER BY Name LIMIT 10`,
  );
  assert.equal(query.records.length, 1);
  assert.equal(query.records[0].Id, retained);
  passed("schema/jsforce-query");
  const account = await create(
    "Schema account " + c.runId,
    { [accountField]: "account custom" },
    "Account",
  );
  assert.equal(
    (await conn.sobject("Account").retrieve(account))[accountField],
    "account custom",
  );
  passed("schema/account-field");
  await assert.rejects(other.sobject(object).retrieve(retained));
  assert.equal(
    (await other.query(`SELECT Id FROM ${object} LIMIT 10`)).records.length,
    0,
  );
  passed("schema/tenant-isolation");
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
  passed("schema/composite-rollback");
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
  await Promise.all(observers);
  assert.deepEqual(issues, []);
  for (const id of [
    "candidateDescribeDynamic",
    "candidateCreateDynamic",
    "candidateReadDynamic",
    "candidateUpdateDynamic",
    "candidateDeleteDynamic",
  ])
    assert.ok(observed.has(id), id);
  passed("schema/http-contract");
  const discovery = await candidateRequest(
    c,
    "admin",
    "/services/data/v65.0/sobjects",
  );
  await writeFile(
    join(c.scratch, "schema-client-state.json"),
    JSON.stringify({
      object,
      id: retained,
      name: name + " restart",
      discovery: discovery.body,
    }),
    { mode: 0o600, flag: "wx" },
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
