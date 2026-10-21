import test from "node:test";
import assert from "node:assert/strict";
import { Connection } from "jsforce";
import {
  recordFixtureEvent,
  trackFixtureAttempt,
  type FixtureEvent,
} from "../src/candidate-acceptance-fixtures.ts";
const opts = { skip: process.env.RUN_CANDIDATE !== "1" };
function fixture(label: string) {
  const runId =
    process.env.CANDIDATE_RUN_ID ??
    crypto.randomUUID().replaceAll("-", "").slice(0, 24);
  const name = `p15-${runId}-${label}`;
  const conn = (key: string) => {
    assert.ok(process.env.CANDIDATE_URL);
    assert.ok(process.env[key]);
    return new Connection({
      instanceUrl: process.env.CANDIDATE_URL,
      accessToken: process.env[key],
      version: "65.0",
    });
  };
  const admin = conn("CANDIDATE_TOKEN"),
    restricted = conn("CANDIDATE_RESTRICTED_TOKEN"),
    other = conn("CANDIDATE_OTHER_TOKEN");
  const rows: { conn: Connection; object: string; id: string }[] = [];
  let n = 0;
  async function create(
    object: "Account" | "Contact",
    data: Record<string, any>,
    principal: FixtureEvent["principal"] = "admin",
  ) {
    const c =
      principal === "other"
        ? other
        : principal === "restricted"
          ? restricted
          : admin;
    const e: FixtureEvent = {
      schemaVersion: 1,
      runId,
      operationId: `jsforce-${label}-${++n}`,
      principal,
      object,
      names: [
        data[object === "Account" ? "Name" : "LastName"],
        name + " updated",
      ],
      kind: "intent",
    };
    const path = process.env.CANDIDATE_FIXTURE_JOURNAL;
    if (path) await recordFixtureEvent(path, e);
    const r: any = await c.sobject(object).create(data);
    assert.equal(r.success, true);
    rows.push({ conn: c, object, id: r.id });
    if (path) await recordFixtureEvent(path, { ...e, id: r.id, kind: "known" });
    return r.id as string;
  }
  async function cleanup() {
    for (const r of rows.sort(
      (a, b) =>
        (a.object === "Contact" ? 0 : 1) - (b.object === "Contact" ? 0 : 1),
    )) {
      assert.equal(
        (await r.conn.sobject(r.object).destroy(r.id)).success,
        true,
      );
      await assert.rejects(r.conn.sobject(r.object).retrieve(r.id), {
        errorCode: "NOT_FOUND",
      });
    }
  }
  return { name, admin, restricted, other, create, cleanup };
}
test("candidate/jsforce/core", opts, async () => {
  const f = fixture("core");
  try {
    const d = await f.admin.sobject("Account").describe();
    for (const name of ["Compat_Text__c", "Compat_Key__c"]) {
      const field = d.fields.find((x) => x.name === name)!;
      assert.equal(field.length, 80);
      assert.equal(field.type, "string");
      assert.equal(field.nillable, true);
      assert.equal(field.externalId, name === "Compat_Key__c");
      assert.deepEqual(field.referenceTo, []);
    }
    const id = await f.create("Account", {
      Name: f.name,
      Compat_Text__c: "secret",
    });
    await f.create("Contact", { LastName: f.name, AccountId: id });
    const q: any = await f.admin.query(
      `SELECT Id,LastName,Account.Name FROM Contact WHERE AccountId = '${id}'`,
    );
    assert.equal(q.records.length, 1);
    assert.equal(q.records[0].Account.Name, f.name);
    await f.admin
      .sobject("Account")
      .update({ Id: id, Name: f.name + " updated", Compat_Text__c: null });
    const row: any = await f.admin.sobject("Account").retrieve(id);
    assert.equal(row.Compat_Text__c, null);
    const path = process.env.CANDIDATE_FIXTURE_JOURNAL,
      runId = process.env.CANDIDATE_RUN_ID!,
      e: FixtureEvent = {
        schemaVersion: 1,
        runId,
        operationId: "jsforce-upsert",
        principal: "admin",
        object: "Account",
        names: [f.name],
        kind: "intent",
      };
    if (path) await recordFixtureEvent(path, e);
    const u: any = await f.admin
      .sobject("Account")
      .upsert({ Name: f.name, Compat_Key__c: f.name }, "Compat_Key__c");
    if (path) await recordFixtureEvent(path, { ...e, kind: "known", id: u.id });
    assert.equal(u.created, true);
    try {
      const u2: any = await trackFixtureAttempt(
        path,
        { ...e, operationId: "jsforce-upsert-repeat" },
        () =>
          f.admin
            .sobject("Account")
            .upsert(
              { Compat_Key__c: f.name, Compat_Text__c: "retained" },
              "Compat_Key__c",
            ) as Promise<any>,
      );
      assert.equal(u2.created, false);
      assert.equal(u2.id, u.id);
      assert.equal(
        ((await f.admin.sobject("Account").retrieve(u.id)) as any).Name,
        f.name,
      );
    } finally {
      assert.equal(
        ((await f.admin.sobject("Account").destroy(u.id)) as any).success,
        true,
      );
    }
  } finally {
    await f.cleanup();
  }
});
test("candidate/jsforce/policy", opts, async () => {
  const f = fixture("policy");
  try {
    const id = await f.create("Account", {
      Name: f.name,
      Compat_Text__c: "secret",
    });
    assert.ok(
      !(await f.restricted.sobject("Account").describe()).fields.some(
        (x) => x.name === "Compat_Text__c",
      ),
    );
    const read: any = await f.restricted.sobject("Account").retrieve(id);
    assert.ok(!Object.hasOwn(read, "Compat_Text__c"));
    for (const q of [
      "SELECT Compat_Text__c FROM Account",
      "SELECT Id FROM Account WHERE Compat_Text__c = 'secret'",
      "SELECT Id FROM Account ORDER BY Compat_Text__c",
    ])
      await assert.rejects(async () => await f.restricted.query(q), {
        errorCode: "INVALID_FIELD",
      });
    await assert.rejects(
      f.restricted
        .sobject("Account")
        .update({ Id: id, Compat_Text__c: "changed" }),
      { errorCode: "INVALID_FIELD" },
    );
    assert.equal(
      ((await f.admin.sobject("Account").retrieve(id)) as any).Compat_Text__c,
      "secret",
    );
    await f.restricted
      .sobject("Account")
      .update({ Id: id, Name: f.name + " updated" });
    assert.equal(
      ((await f.admin.sobject("Account").retrieve(id)) as any).Name,
      f.name + " updated",
    );
    await f.create(
      "Contact",
      { LastName: f.name, AccountId: id },
      "restricted",
    );
  } finally {
    await f.cleanup();
  }
});
test("candidate/jsforce/isolation", opts, async () => {
  const f = fixture("isolation");
  try {
    const id = await f.create("Account", {
      Name: f.name,
      Compat_Key__c: f.name,
    });
    for (const action of [
      () => f.other.sobject("Account").retrieve(id),
      () => f.other.sobject("Account").update({ Id: id, Name: "foreign" }),
      () => f.other.sobject("Account").destroy(id),
    ])
      await assert.rejects(action(), { errorCode: "NOT_FOUND" });
    await assert.rejects(
      trackFixtureAttempt(
        process.env.CANDIDATE_FIXTURE_JOURNAL,
        {
          schemaVersion: 1,
          runId: process.env.CANDIDATE_RUN_ID!,
          operationId: "jsforce-denied-link",
          principal: "other",
          object: "Contact",
          names: [f.name],
          kind: "intent",
        },
        () =>
          f.other
            .sobject("Contact")
            .create({ LastName: f.name, AccountId: id }) as Promise<any>,
        ["INVALID_CROSS_REFERENCE_KEY"],
      ),
      { errorCode: "INVALID_CROSS_REFERENCE_KEY" },
    );
    assert.equal(
      ((await f.admin.sobject("Account").retrieve(id)) as any).Name,
      f.name,
    );
    await f.create("Account", { Name: f.name, Compat_Key__c: f.name }, "other");
  } finally {
    await f.cleanup();
  }
});
