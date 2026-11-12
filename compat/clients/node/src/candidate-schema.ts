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
  Kind: "object" | "field" | "stored";
  Name: string;
  Object?: string;
};
/** Plan 28 fixture names; records in these objects go with their tables. */
export function fieldTypeNames(runId: string) {
  const prefix = "P28_" + runId.slice(0, 12);
  return {
    parent: prefix + "_Parent__c",
    item: prefix + "_Item__c",
    child: prefix + "_Child__c",
  };
}
type FieldSpec = [name: string, label: string, ...flags: string[]];
const itemFields = (parent: string): FieldSpec[] => [
  [
    "Quantity__c",
    "Quantity",
    "--type",
    "Number",
    "--precision",
    "18",
    "--scale",
    "2",
  ],
  ["Amount__c", "Amount", "--type", "Currency"],
  [
    "Score__c",
    "Score",
    "--type",
    "Percent",
    "--precision",
    "5",
    "--scale",
    "2",
  ],
  ["Due__c", "Due", "--type", "Date"],
  ["Starts__c", "Starts", "--type", "DateTime"],
  ["Done__c", "Done", "--type", "Checkbox"],
  [
    "Stage__c",
    "Stage",
    "--type",
    "Picklist",
    "--values",
    "Open,Closed",
    "--restricted",
    "--default",
    "Open",
  ],
  ["Tags__c", "Tags", "--type", "MultiselectPicklist", "--values", "A,B,C"],
  ["Email__c", "Contact email", "--type", "Email"],
  ["Phone__c", "Phone", "--type", "Phone"],
  ["Site__c", "Site", "--type", "Url"],
  ["Notes__c", "Notes", "--type", "LongTextArea", "--length", "1000"],
  ["Summary__c", "Summary", "--type", "TextArea"],
  ["Code__c", "Code", "--type", "Text", "--length", "20"],
  ["Account__c", "Parent", "--type", "Lookup", "--reference-to", "Account"],
  [
    "Keep__c",
    "Kept parent",
    "--type",
    "Lookup",
    "--reference-to",
    parent,
    "--delete-behavior",
    "Restrict",
  ],
  [
    "Serial__c",
    "Serial",
    "--type",
    "AutoNumber",
    "--display-format",
    "S-{000}",
    "--starting-number",
    "7",
  ],
];
async function createFieldTypeFixtures(c: GateContext) {
  const tenant = c.tenants.first;
  const n = fieldTypeNames(c.runId);
  const object = async (
    name: string,
    label: string,
    plural: string,
    extra: string[] = [],
  ) =>
    schemaOperation(c, { Tenant: tenant, Kind: "object", Name: name }, [
      "schema",
      "object",
      "create",
      "--tenant",
      tenant,
      "--name",
      name,
      "--label",
      label,
      "--plural-label",
      plural,
      ...extra,
    ]);
  const field = async (obj: string, [name, label, ...flags]: FieldSpec) =>
    schemaOperation(
      c,
      { Tenant: tenant, Kind: "field", Object: obj, Name: name },
      [
        "schema",
        "field",
        "create",
        "--tenant",
        tenant,
        "--object",
        obj,
        "--name",
        name,
        "--label",
        label,
        ...flags,
      ],
    );
  await object(n.parent, "Typed parent", "Typed parents");
  await object(n.item, "Typed item", "Typed items", [
    "--name-type",
    "AutoNumber",
    "--display-format",
    "TI-{0000}",
  ]);
  await object(n.child, "Typed child", "Typed children");
  for (const spec of itemFields(n.parent)) await field(n.item, spec);
  await field(n.child, [
    "Parent__c",
    "Parent",
    "--type",
    "MasterDetail",
    "--reference-to",
    n.parent,
  ]);
  const definition = join(c.scratch, "stored-formula.json");
  await writeFile(
    definition,
    JSON.stringify({
      name: "Score_Formula__c",
      label: "Score formula",
      type: "Formula",
      formula: "Score__c * 2",
      returnType: "Percent",
    }),
    { mode: 0o600, flag: "wx" },
  );
  await schemaOperation(
    c,
    {
      Tenant: tenant,
      Kind: "stored",
      Object: n.item,
      Name: "Score_Formula__c",
    },
    [
      "schema",
      "field",
      "create",
      "--tenant",
      tenant,
      "--object",
      n.item,
      "--definition",
      definition,
    ],
  );
  await ownerCommand(c, [
    "schema",
    "currency",
    "set",
    "--tenant",
    tenant,
    "--code",
    "EUR",
  ]);
  c.env.CANDIDATE_FIELD_TYPES_OBJECT = n.item;
}
async function runFieldTypesPython(c: GateContext): Promise<string[]> {
  const report = join(c.scratch, "field-types-python.json");
  const env: NodeJS.ProcessEnv = { ...c.env, CANDIDATE_STEP_REPORT: report };
  for (const key of [
    "MIGRATION_DATABASE_URL",
    "DATABASE_URL",
    "AUTH_DATABASE_URL",
    "TEST_DATABASE_URL",
    "TEST_RECORDS_URL",
    "TEST_IDENTITY_URL",
  ])
    delete env[key];
  const r = await runCommand(
    {
      executable: c.python,
      argv: ["scripts/candidate-field-types-python.py"],
      cwd: ROOT,
      env,
      privateOutput: () => {},
    },
    c.processes,
    "schema",
  );
  if (r.exitCode !== 0 || !r.complete) throw new Error("ASSERTION");
  const parsed = JSON.parse(await readFile(report, "utf8"));
  assert.equal(parsed.runId, c.runId);
  return parsed.tests
    .filter((t: any) => t.outcome === "passed")
    .map((t: any) => t.id);
}
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
  await createFieldTypeFixtures(c);
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
  const typed = await fieldTypeClientChecks(c, conn, passed, v, observed);
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
    "candidateStoredFields",
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
      typed,
    }),
    { mode: 0o600, flag: "wx" },
  );
  return stepResult(c, "schema", ids);
}
async function rejectsWith(promise: Promise<unknown>, code: string) {
  await assert.rejects(promise, (e: any) => {
    assert.equal(e.errorCode, code);
    return true;
  });
}
/** Plan 28: every active field type through unmodified JSforce. Records in
 * the P28 objects are removed with their journaled tables. */
async function fieldTypeClientChecks(
  c: SchemaClientContext,
  conn: any,
  passed: (id: string) => void,
  v: Awaited<ReturnType<typeof createCandidateTrafficValidator>>,
  observed: Set<string>,
) {
  const n = fieldTypeNames(c.runId);
  const fields = (d: any) =>
    Object.fromEntries(d.fields.map((f: any) => [f.name, f]));
  const item = fields(await conn.sobject(n.item).describe());
  const expect = (f: any, values: Record<string, unknown>) => {
    for (const [k, want] of Object.entries(values))
      assert.deepEqual(f?.[k], want, k);
  };
  expect(item.Name, { autoNumber: true, createable: false, nameField: true });
  expect(item.Quantity__c, { type: "double", precision: 18, scale: 2 });
  expect(item.Amount__c, { type: "currency", precision: 18, scale: 2 });
  expect(item.Score__c, { type: "percent", precision: 5, scale: 2 });
  expect(item.Due__c, { type: "date", soapType: "xsd:date" });
  expect(item.Starts__c, { type: "datetime", soapType: "xsd:dateTime" });
  expect(item.Done__c, {
    type: "boolean",
    nillable: false,
    defaultValue: false,
  });
  expect(item.Stage__c, { type: "picklist", restrictedPicklist: true });
  assert.deepEqual(
    item.Stage__c.picklistValues.map((p: any) => [p.value, p.defaultValue]),
    [
      ["Open", true],
      ["Closed", false],
    ],
  );
  expect(item.Tags__c, { type: "multipicklist", sortable: false });
  expect(item.Email__c, { type: "email", length: 80 });
  expect(item.Phone__c, { type: "phone", length: 40 });
  expect(item.Site__c, { type: "url" });
  expect(item.Notes__c, { type: "textarea", length: 1000, filterable: false });
  expect(item.Summary__c, { type: "textarea", length: 255 });
  expect(item.Code__c, { type: "string", length: 20 });
  expect(item.Account__c, {
    type: "reference",
    referenceTo: ["Account"],
    relationshipName: "Account__r",
  });
  expect(item.Keep__c, { restrictedDelete: true, referenceTo: [n.parent] });
  expect(item.Serial__c, {
    autoNumber: true,
    createable: false,
    updateable: false,
  });
  assert.equal(item.Score_Formula__c, undefined);
  const child = fields(await conn.sobject(n.child).describe());
  expect(child.Parent__c, {
    cascadeDelete: true,
    updateable: false,
    nillable: false,
    relationshipOrder: 0,
  });
  passed("schema/field-types-describe");

  let sequence = 0;
  const journaled = async (
    object: "Account" | "Contact",
    name: string,
    data: Record<string, unknown>,
  ) => {
    const e: FixtureEvent = {
      schemaVersion: 1,
      runId: c.runId,
      operationId: "field-types-" + sequence++,
      principal: "admin",
      object,
      names: [name],
      kind: "intent",
    };
    await recordFixtureEvent(c.fixtureJournal, e);
    const value = await conn.sobject(object).create(data);
    await recordFixtureEvent(c.fixtureJournal, {
      ...e,
      kind: "known",
      id: value.id,
    });
    return value.id as string;
  };
  const accountName = "Typed account " + c.runId;
  const account = await journaled("Account", accountName, {
    Name: accountName,
    Phone: "+33 1 23",
    Industry: "Banking",
    AnnualRevenue: 1000000,
    NumberOfEmployees: 12,
    BillingCity: "Lyon",
    BillingCountry: "France",
  });
  const created = await conn.sobject(n.item).create({
    Quantity__c: 12.345,
    Amount__c: 0.1 + 0.2,
    Score__c: 12.5,
    Due__c: "2026-11-11",
    Starts__c: "2026-03-08T23:30:00-05:00",
    Done__c: true,
    Tags__c: "C;A",
    Email__c: "a@b.co",
    Phone__c: "+1 555 0100",
    Site__c: "example.com",
    Notes__c: "line one\nline two",
    Summary__c: "short",
    Code__c: "X1",
    Account__c: account,
  });
  const id = created.id as string;
  let row = await conn.sobject(n.item).retrieve(id);
  expect(row, {
    Quantity__c: 12.35,
    Amount__c: 0.3,
    Score__c: 12.5,
    Due__c: "2026-11-11",
    Starts__c: "2026-03-09T04:30:00.000+0000",
    Done__c: true,
    Stage__c: "Open",
    Tags__c: "A;C",
    Notes__c: "line one\nline two",
    Account__c: account,
    Serial__c: "S-007",
  });
  assert.match(row.Name, /^TI-\d{4}$/);
  await conn.sobject(n.item).update({
    Id: id,
    Quantity__c: "-5",
    Done__c: false,
    Tags__c: "",
    Due__c: "4000-12-31",
  });
  row = await conn.sobject(n.item).retrieve(id);
  expect(row, {
    Quantity__c: -5,
    Done__c: false,
    Tags__c: null,
    Due__c: "4000-12-31",
  });
  const doomed = (await conn.sobject(n.item).create({ Code__c: "doomed" })).id;
  await conn.sobject(n.item).destroy(doomed);
  await assert.rejects(conn.sobject(n.item).retrieve(doomed));
  passed("schema/field-types-crud");

  for (const [data, code] of [
    [{ Quantity__c: 1e17 }, "NUMBER_OUTSIDE_VALID_RANGE"],
    [{ Email__c: "not-an-address" }, "INVALID_EMAIL_ADDRESS"],
    [{ Stage__c: "open" }, "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST"],
    [{ Due__c: "2026-02-30" }, "JSON_PARSER_ERROR"],
    [{ Done__c: "true" }, "JSON_PARSER_ERROR"],
    [{ Serial__c: "S-999" }, "INVALID_FIELD_FOR_INSERT_UPDATE"],
  ] as const)
    await rejectsWith(conn.sobject(n.item).create(data), code);
  passed("schema/field-types-errors");

  const parent = (await conn.sobject(n.parent).create({ Name: "Master" })).id;
  const kid = (
    await conn.sobject(n.child).create({ Name: "Detail", Parent__c: parent })
  ).id;
  const keeper = (await conn.sobject(n.item).create({ Keep__c: parent })).id;
  await rejectsWith(conn.sobject(n.parent).destroy(parent), "DELETE_FAILED");
  await rejectsWith(
    conn.sobject(n.child).update({ Id: kid, Parent__c: parent }),
    "INVALID_FIELD_FOR_INSERT_UPDATE",
  );
  await conn.sobject(n.item).update({ Id: keeper, Keep__c: null });
  await conn.sobject(n.parent).destroy(parent);
  await assert.rejects(conn.sobject(n.child).retrieve(kid));
  const loseName = "Typed lookup " + c.runId;
  const lose = await journaled("Account", loseName, { Name: loseName });
  const pointer = (await conn.sobject(n.item).create({ Account__c: lose })).id;
  await conn.sobject("Account").destroy(lose);
  assert.equal((await conn.sobject(n.item).retrieve(pointer)).Account__c, null);
  passed("schema/field-types-relationships");

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
          url: `/services/data/v65.0/sobjects/${n.item}`,
          referenceId: "ok",
          body: { Code__c: "rollback" },
        },
        {
          method: "POST",
          url: `/services/data/v65.0/sobjects/${n.item}`,
          referenceId: "bad",
          body: { Stage__c: "Nope" },
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
        `SELECT Id FROM ${n.item} WHERE Code__c = 'rollback' LIMIT 10`,
      )
    ).records.length,
    0,
  );
  passed("schema/field-types-composite");

  const acct = await conn.sobject("Account").retrieve(account);
  expect(acct, {
    Industry: "Banking",
    AnnualRevenue: 1000000,
    NumberOfEmployees: 12,
  });
  assert.deepEqual(acct.BillingAddress, {
    street: null,
    city: "Lyon",
    state: null,
    postalCode: null,
    country: "France",
  });
  assert.equal(acct.ShippingAddress, null);
  const lastName = "Typed contact " + c.runId;
  const contact = await journaled("Contact", lastName, {
    FirstName: "Ada",
    LastName: lastName,
    Email: "ada@example.org",
    Birthdate: "1815-12-10",
    MailingCity: "London",
  });
  const person = await conn.sobject("Contact").retrieve(contact);
  expect(person, { Name: "Ada " + lastName, Birthdate: "1815-12-10" });
  assert.equal(person.MailingAddress.city, "London");
  await rejectsWith(
    conn.sobject("Contact").update({ Id: contact, Name: "x" }),
    "INVALID_FIELD_FOR_INSERT_UPDATE",
  );
  passed("schema/field-types-standard");

  const stored = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/candidate/sobjects/${n.item}/storedFields`,
  );
  assert.deepEqual(
    stored.body.storedFields.map((f: any) => [f.name, f.type]),
    [["Score_Formula__c", "Formula"]],
  );
  const check = v.validate(stored.exchange!);
  assert.deepEqual([check.requestIssues, check.responseIssues], [[], []]);
  if (check.operationId) observed.add(check.operationId);
  const snapshot = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/sobjects/${n.item}/${id}`,
  );
  return { object: n.item, id, record: snapshot.body };
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
  const typed = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/sobjects/${saved.typed.object}/${saved.typed.id}`,
  );
  assert.equal(typed.status, 200);
  assert.deepEqual(typed.body, saved.typed.record);
  return stepResult(c, "restart", ["restart/schema", "restart/field-types"]);
}
