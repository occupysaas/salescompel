import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import type { Connection } from "jsforce";
import { runCommand } from "./candidate-acceptance-process.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import {
  candidateRequest,
  recordFixtureEvent,
  type FixtureEvent,
} from "./candidate-acceptance-fixtures.ts";
import type { SchemaClientContext } from "./candidate-schema-field-checks.ts";
import { schemaOperation } from "./candidate-schema-runtime.ts";
import { ROOT } from "./versions.ts";

const totalRecords = 4500,
  batchSize = 25,
  accountCount = 12;

/** Plan 29 deterministic fixture names; records in the P29 object go with its
 * journaled table, following the Plan 28 ruling. */
export function soqlNames(runId: string) {
  const object = "P29_" + runId.slice(0, 12) + "__c";
  return {
    object,
    record: (n: number) => `P29 ${runId} ${n}`,
    account: (i: number) => `P29 ${runId} Account ${i}`,
    contact: (i: number) => `P29 ${runId} Contact ${i}`,
    accountPrefix: `P29 ${runId} Account`,
    contactPrefix: `P29 ${runId} Contact`,
  };
}

export async function createSoqlSchemaFixtures(c: GateContext) {
  const n = soqlNames(c.runId),
    tenant = c.tenants.first;
  await schemaOperation(c, { Tenant: tenant, Kind: "object", Name: n.object }, [
    "schema",
    "object",
    "create",
    "--tenant",
    tenant,
    "--name",
    n.object,
    "--label",
    "P29 item",
    "--plural-label",
    "P29 items",
  ]);
  for (const [name, label, ...flags] of [
    [
      "Seq__c",
      "Sequence",
      "--type",
      "Number",
      "--precision",
      "18",
      "--scale",
      "0",
    ],
    ["Amount__c", "Amount", "--type", "Currency"],
    ["Due__c", "Due", "--type", "Date"],
    ["Stage__c", "Stage", "--type", "Picklist", "--values", "Open,Closed"],
  ] as const)
    await schemaOperation(
      c,
      { Tenant: tenant, Kind: "field", Object: n.object, Name: name },
      [
        "schema",
        "field",
        "create",
        "--tenant",
        tenant,
        "--object",
        n.object,
        "--name",
        name,
        "--label",
        label,
        ...flags,
      ],
    );
}

type Row = Record<string, unknown>;
const row = (v: unknown) => v as Row;
const rows = (v: unknown) => (v as { records: unknown[] }).records.map(row);

async function createFixtureRecord(
  c: SchemaClientContext,
  operationId: string,
  object: "Account" | "Contact",
  data: Record<string, unknown>,
  names: string[],
) {
  const e: FixtureEvent = {
    schemaVersion: 1,
    runId: c.runId,
    operationId,
    principal: "admin",
    object,
    names,
    kind: "intent",
  };
  await recordFixtureEvent(c.fixtureJournal, e);
  const r = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/sobjects/${object}`,
    "POST",
    data,
  );
  assert.equal(r.status, 201);
  const id = r.body?.id;
  assert.equal(typeof id, "string");
  await recordFixtureEvent(c.fixtureJournal, {
    ...e,
    kind: "known",
    id: id as string,
  });
  return id as string;
}

type CompositeEntry = {
  referenceId?: unknown;
  httpStatusCode?: unknown;
  body?: { id?: unknown; success?: unknown } | null;
};

async function createRecordBatches(
  c: SchemaClientContext,
  n: ReturnType<typeof soqlNames>,
) {
  const started = new Date(),
    day = (offset: number) =>
      new Date(
        Date.UTC(
          started.getUTCFullYear(),
          started.getUTCMonth(),
          started.getUTCDate() + offset,
        ),
      )
        .toISOString()
        .slice(0, 10);
  for (let start = 1; start <= totalRecords; start += batchSize) {
    const values = Array.from(
      { length: Math.min(batchSize, totalRecords - start + 1) },
      (_, i) => start + i,
    );
    const e: FixtureEvent = {
      schemaVersion: 1,
      runId: c.runId,
      operationId: "soql-batch-" + start,
      principal: "admin",
      object: n.object,
      names: values.map(n.record),
      kind: "intent",
    };
    await recordFixtureEvent(c.fixtureJournal, e);
    const response = await candidateRequest(
      c,
      "admin",
      "/services/data/v65.0/composite",
      "POST",
      {
        allOrNone: true,
        compositeRequest: values.map((value, i) => ({
          method: "POST",
          url: `/services/data/v65.0/sobjects/${n.object}`,
          referenceId: "r" + i,
          body: {
            Name: n.record(value),
            Seq__c: value,
            Amount__c: value,
            Due__c: day(value % 3 === 0 ? 100 : value % 3 === 1 ? 200 : 300),
            Stage__c: value % 2 === 1 ? "Open" : "Closed",
          },
        })),
      },
    );
    assert.equal(response.status, 200);
    const entries = (response.body as { compositeResponse?: CompositeEntry[] })
      .compositeResponse;
    assert.ok(Array.isArray(entries));
    assert.equal(entries.length, values.length);
    for (const entry of entries) {
      assert.equal(entry.httpStatusCode, 201);
      assert.equal(entry.body?.success, true);
      assert.equal(typeof entry.body?.id, "string");
    }
    await recordFixtureEvent(c.fixtureJournal, {
      ...e,
      kind: "known",
      id: entries[0].body?.id as string,
    });
  }
}

/** Plan 29 SOQL acceptance through unmodified JSforce. */
export async function soqlClientChecks(
  c: SchemaClientContext,
  conn: Connection,
  passed: (id: string) => void,
) {
  const n = soqlNames(c.runId),
    parents: string[] = [];
  for (let i = 0; i < accountCount; i++)
    parents.push(
      await createFixtureRecord(
        c,
        "soql-account-" + i,
        "Account",
        { Name: n.account(i) },
        [n.account(i)],
      ),
    );
  for (let i = 0; i < accountCount; i++)
    await createFixtureRecord(
      c,
      "soql-contact-" + i,
      "Contact",
      { LastName: n.contact(i), AccountId: parents[i] },
      [n.contact(i)],
    );
  await createRecordBatches(c, n);

  const paged = await conn
    .query<Row>(`SELECT Id FROM ${n.object}`, {
      headers: { "Sforce-Query-Options": "batchSize=500" },
      autoFetch: true,
      maxFetch: 5000,
    })
    .run({ autoFetch: true, maxFetch: 5000 });
  assert.equal(paged.totalSize, totalRecords);
  assert.equal(paged.records.length, totalRecords);
  assert.equal(new Set(paged.records.map((r) => r.Id)).size, totalRecords);
  passed("schema/soql-paging");

  const contacts = rows(
    await conn.query(
      `SELECT Id,LastName,Account.Name FROM Contact WHERE Account.Name LIKE '${n.accountPrefix}%' ORDER BY LastName`,
    ),
  );
  assert.equal(contacts.length, accountCount);
  const contactAccounts = new Map(
    Array.from({ length: accountCount }, (_, i) => [
      n.contact(i),
      n.account(i),
    ]),
  );
  for (const contact of contacts) {
    assert.equal(
      row(contact.Account).Name,
      contactAccounts.get(contact.LastName as string),
    );
  }
  passed("schema/soql-relationships");

  const accounts = rows(
    await conn.query(
      `SELECT Id,Name,(SELECT Id,LastName FROM Contacts WHERE LastName LIKE '${n.contactPrefix}%') FROM Account WHERE Name LIKE '${n.accountPrefix}%' ORDER BY Name`,
    ),
  );
  assert.equal(accounts.length, accountCount);
  const accountContacts = new Map(
    Array.from({ length: accountCount }, (_, i) => [
      n.account(i),
      n.contact(i),
    ]),
  );
  for (const account of accounts) {
    const children = account.Contacts as {
      totalSize: number;
      done: boolean;
      records: Row[];
    };
    assert.equal(children.totalSize, 1);
    assert.equal(children.done, true);
    assert.equal(
      children.records[0].LastName,
      accountContacts.get(account.Name as string),
    );
  }
  passed("schema/soql-children");

  const filtered = rows(
    await conn.query(
      `SELECT Id,Name,Seq__c FROM ${n.object} WHERE Seq__c >= 4495 ORDER BY Seq__c`,
    ),
  );
  assert.equal(filtered.length, 6);
  for (let i = 0; i < filtered.length; i++) {
    assert.equal(filtered[i].Seq__c, 4495 + i);
    assert.equal(filtered[i].Name, n.record(4495 + i));
  }
  passed("schema/soql-filters");

  assert.equal(
    rows(
      await conn.query(
        `SELECT Id FROM ${n.object} WHERE Due__c = NEXT_N_DAYS:150`,
      ),
    ).length,
    1500,
  );
  assert.equal(
    rows(
      await conn.query(
        `SELECT Id FROM ${n.object} WHERE Due__c = LAST_N_DAYS:1000`,
      ),
    ).length,
    0,
  );
  const counted = await conn.query(`SELECT COUNT(Id) FROM ${n.object}`);
  assert.equal(counted.totalSize, totalRecords);
  assert.deepEqual(counted.records, []);
  const summed = rows(
    await conn.query(`SELECT SUM(Amount__c) total FROM ${n.object}`),
  );
  assert.equal(summed.length, 1);
  assert.equal(Number(summed[0].total), 10127250);
  const grouped = rows(
    await conn.query(
      `SELECT Stage__c, COUNT(Id) total FROM ${n.object} GROUP BY Stage__c ORDER BY Stage__c`,
    ),
  );
  assert.deepEqual(
    grouped.map((r) => [r.Stage__c, Number(r.total)]),
    [
      ["Closed", 2250],
      ["Open", 2250],
    ],
  );
  passed("schema/soql-aggregates");
}

/** Plan 29 SOQL acceptance through real, unmodified simple-salesforce. */
export async function runSoqlPython(c: GateContext): Promise<string[]> {
  const report = join(c.scratch, "soql-python.json");
  const env: NodeJS.ProcessEnv = {
    ...c.env,
    CANDIDATE_STEP_REPORT: report,
    CANDIDATE_SOQL_OBJECT: soqlNames(c.runId).object,
  };
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
      argv: ["scripts/candidate-soql-python.py"],
      cwd: ROOT,
      env,
      privateOutput: () => {},
    },
    c.processes,
    "schema",
  );
  if (r.exitCode !== 0 || !r.complete) throw new Error("ASSERTION");
  const parsed = JSON.parse(await readFile(report, "utf8")) as {
    runId: string;
    tests: Array<{ id: string; outcome: string }>;
  };
  assert.equal(parsed.runId, c.runId);
  return parsed.tests.filter((t) => t.outcome === "passed").map((t) => t.id);
}
