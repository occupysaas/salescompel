import assert from "node:assert/strict";
import { writeFile } from "node:fs/promises";
import { join } from "node:path";
import { createRequire } from "node:module";
import { ROOT } from "./versions.ts";
import type { ParsedResult } from "./candidate-acceptance-contract.ts";
import { stepResult } from "./candidate-acceptance-clients.ts";
import {
  candidateRequest,
  recordFixtureEvent,
  type FixtureEvent,
} from "./candidate-acceptance-fixtures.ts";
import { loadCandidateOpenAPI } from "./candidate-openapi-contract.ts";
import { createCandidateTrafficValidator } from "./candidate-openapi-traffic.ts";
import type { HttpRequest, HttpResponse, HttpRequestOptions } from "jsforce";
import {
  fieldTypeClientChecks,
  type SchemaClientContext,
} from "./candidate-schema-field-checks.ts";
export type { SchemaClientContext };
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
    transport.httpRequest = (
      req: HttpRequest,
      options?: HttpRequestOptions,
    ) => {
      const target = new URL(req.url);
      assert.equal(target.origin, new URL(c.url).origin);
      const pending = original(req, options);
      observers.push(
        pending.then(
          (res: HttpResponse) => {
            const result = v.validate({
              method: req.method,
              path: target.pathname + target.search,
              request: {
                contentType:
                  req.headers?.["Content-Type"] ??
                  req.headers?.["content-type"] ??
                  null,
                body:
                  req.body === undefined
                    ? undefined
                    : Buffer.from(req.body as string),
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
  assert.ok(global.sobjects.some((o: { name: string }) => o.name === object));
  passed("schema/jsforce-discover");
  const describe = await conn.sobject(object).describe();
  assert.equal(
    describe.fields.find((f: { name: string }) => f.name === "Notes__c")!
      .length,
    80,
  );
  assert.equal(
    (await other.sobject(object).describe()).fields.find(
      (f: { name: string }) => f.name === "Notes__c",
    )!.length,
    40,
  );
  passed("schema/jsforce-describe");
  let sequence = 0;
  const create = async (
    name: string,
    data: Record<string, any>,
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
    (
      composite.body as { compositeResponse: Array<{ httpStatusCode: number }> }
    ).compositeResponse.every((r) => r.httpStatusCode === 400),
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
