import assert from "node:assert/strict";
import test from "node:test";
import { dirname } from "node:path";
import { Connection } from "jsforce";
import {
  candidateRequest,
  recordFixtureEvent,
  type FixtureEvent,
} from "../src/candidate-acceptance-fixtures.ts";
import type { GateContext } from "../src/candidate-acceptance-preflight.ts";

// Plan 31 acceptance: unmodified JSforce sObject Collections calls and raw
// HTTP Composite partial success. Every possible create is journaled as an
// intent before dispatch; returned IDs are recorded before any assertion;
// failed creates are proven absent before they are marked rejected.

const opts = { skip: process.env.RUN_CANDIDATE !== "1" };
const base = "/services/data/v65.0";
type Role = FixtureEvent["principal"];
type Obj = "Account" | "Contact";

function context() {
  const runId = process.env.CANDIDATE_RUN_ID!,
    journal = process.env.CANDIDATE_FIXTURE_JOURNAL!;
  assert.ok(
    /^[a-f0-9]{24}$/.test(runId) && journal,
    "owned fixture context required",
  );
  const c = {
    runId,
    fixtureJournal: journal,
    url: process.env.CANDIDATE_URL!,
    scratch: dirname(process.env.NODE_EXTRA_CA_CERTS!),
    tokens: {
      admin: process.env.CANDIDATE_TOKEN!,
      restricted: process.env.CANDIDATE_RESTRICTED_TOKEN!,
      other: process.env.CANDIDATE_OTHER_TOKEN!,
    },
  } as GateContext;
  const url = new URL(c.url);
  assert.ok(
    url.protocol === "https:" &&
      ["localhost", "127.0.0.1"].includes(url.hostname),
    "owned HTTPS candidate required",
  );
  return c;
}

function journal(c: GateContext, label: string) {
  let n = 0;
  const marker = (ref: string) => `p31-${c.runId}-${label}-${ref}`;
  async function intent(
    object: Obj,
    ref: string,
    principal: Role = "admin",
  ): Promise<FixtureEvent> {
    const e: FixtureEvent = {
      schemaVersion: 1,
      runId: c.runId,
      operationId: `p31-${label}-${ref}-${++n}`,
      principal,
      object,
      names: [marker(ref)],
      kind: "intent",
    };
    await recordFixtureEvent(c.fixtureJournal, e);
    return e;
  }
  async function known(e: FixtureEvent, id: string) {
    assert.match(id, /^[A-Za-z0-9]{18}$/);
    await recordFixtureEvent(c.fixtureJournal, { ...e, kind: "known", id });
  }
  // Prove a failed create left no record, then mark it rejected.
  async function absent(e: FixtureEvent) {
    const field = e.object === "Account" ? "Name" : "LastName";
    const q = `SELECT Id FROM ${e.object} WHERE ${field} = '${e.names[0]}'`;
    const r = await candidateRequest(
      c,
      e.principal,
      base + "/query?q=" + encodeURIComponent(q),
    );
    assert.equal(r.status, 200);
    for (const row of r.body!.records as { Id: string }[]) {
      const recovered = {
        ...e,
        operationId: e.operationId + "__recovered_" + row.Id,
      };
      await recordFixtureEvent(c.fixtureJournal, recovered);
      await known(recovered, row.Id);
    }
    assert.equal(r.body!.records.length, 0, "failed create persisted");
    await recordFixtureEvent(c.fixtureJournal, { ...e, kind: "rejected" });
  }
  async function settle(e: FixtureEvent, id: unknown) {
    if (typeof id === "string" && id) await known(e, id);
    else await absent(e);
  }
  return { marker, intent, known, absent, settle };
}

function connect(c: GateContext, role: Role) {
  return new Connection({
    instanceUrl: c.url,
    accessToken: c.tokens[role],
    version: "65.0",
  });
}

async function count(c: GateContext, object: Obj, name: string) {
  const field = object === "Account" ? "Name" : "LastName";
  const r = await candidateRequest(
    c,
    "admin",
    base +
      "/query?q=" +
      encodeURIComponent(`SELECT Id FROM ${object} WHERE ${field} = '${name}'`),
  );
  assert.equal(r.status, 200);
  return (r.body!.records as unknown[]).length;
}

test("candidate/collections/jsforce", opts, async () => {
  const c = context();
  const j = journal(c, "jsf");
  const conn = connect(c, "admin");
  const done = new Set<string>();

  for (const allOrNone of [false, true]) {
    const tag = allOrNone ? "aon" : "part";
    // create: one good Account and one invalid one; JSforce sets every
    // record's attributes.type from the sobject, so the Contact is separate.
    const good1 = await j.intent("Account", tag + "c1");
    const bad = await j.intent("Account", tag + "cbad");
    const good2 = await j.intent("Contact", tag + "c2");
    const created = await conn
      .sobject("Account")
      .create(
        [{ Name: good1.names[0] }, { Name: bad.names[0], Bogus: 1 }] as any[],
        { allOrNone },
      );
    assert.equal(created.length, 2);
    // Record every committed ID before asserting anything else.
    if (!allOrNone)
      await j.settle(good1, created[0].success ? created[0].id : undefined);
    else await j.absent(good1);
    await j.absent(bad);
    assert.equal(created[1].success, false);
    assert.equal((created[1] as any).errors[0].statusCode, "INVALID_FIELD");
    if (allOrNone) {
      assert.equal(created[0].success, false);
      assert.equal(
        (created[0] as any).errors[0].statusCode,
        "ALL_OR_NONE_OPERATION_ROLLED_BACK",
      );
      await j.absent(good2);
      continue;
    }
    const contacts = await conn
      .sobject("Contact")
      .create([{ LastName: good2.names[0] }] as any[], { allOrNone: false });
    await j.settle(good2, contacts[0].success ? contacts[0].id : undefined);
    assert.ok(created[0].success && contacts[0].success);
    created.push(contacts[0]);
    created[2] = contacts[0];
    const acct = created[0].id!,
      contact = created[2].id!;
    done.add("create-" + tag);

    // update: one good, one wrong-object ID.
    const updated = await conn.sobject("Account").update(
      [
        { attributes: { type: "Account" }, Id: acct, Name: good1.names[0] },
        { attributes: { type: "Contact" }, Id: acct, LastName: "x" },
      ] as any[],
      { allOrNone: false },
    );
    assert.ok(updated[0].success && !updated[1].success);
    const rolled = await conn.sobject("Account").update(
      [
        { attributes: { type: "Account" }, Id: acct, Name: "Never" },
        { attributes: { type: "Account" }, Id: acct, Bogus: 1 },
      ] as any[],
      { allOrNone: true },
    );
    assert.ok(!rolled[0].success && !rolled[1].success);
    assert.equal(await count(c, "Account", "Never"), 0);
    assert.equal(await count(c, "Account", good1.names[0]), 1);
    done.add("update");

    // retrieve: describe-driven fields, nulls in place.
    const fetched = await conn
      .sobject("Account")
      .retrieve([acct, contact, acct], { fields: ["Name"] });
    assert.equal(fetched.length, 3);
    assert.equal((fetched[0] as any).Name, good1.names[0]);
    assert.equal(fetched[1], null);
    const all = await conn.sobject("Account").retrieve([acct]);
    assert.equal((all[0] as any).Id, acct);
    done.add("retrieve");

    // upsert by Compat_Key__c: create then update, plus a missing key.
    const keyA = j.marker("keyA"),
      keyB = j.marker("keyB");
    const upA = await j.intent("Account", "upA");
    const upB = await j.intent("Account", "upB");
    const upserted = await conn.sobject("Account").upsert(
      [
        { Compat_Key__c: keyA, Name: upA.names[0] },
        { Compat_Key__c: keyB, Name: upB.names[0] },
      ] as any[],
      "Compat_Key__c",
      { allOrNone: false },
    );
    await j.settle(upA, upserted[0].success ? upserted[0].id : undefined);
    await j.settle(upB, upserted[1].success ? upserted[1].id : undefined);
    assert.ok(upserted.every((r: any) => r.success && r.created === true));
    const again = await conn
      .sobject("Account")
      .upsert(
        [{ Compat_Key__c: keyA, Name: upA.names[0] }] as any[],
        "Compat_Key__c",
        { allOrNone: true },
      );
    assert.ok((again[0] as any).success && (again[0] as any).created === false);
    assert.equal(again[0].id, upserted[0].id);
    done.add("upsert");

    // destroy: all-or-none keeps everything when one ID is malformed.
    const kept = await conn
      .sobject("Account")
      .destroy([upserted[1].id!, "001000000000000"], { allOrNone: true });
    assert.ok(!kept[0].success && !kept[1].success);
    assert.equal(await count(c, "Account", upB.names[0]), 1);
    const destroyed = await conn
      .sobject("Account")
      .destroy([upserted[0].id!, upserted[1].id!, contact], {
        allOrNone: false,
      });
    assert.ok(destroyed.every((r) => r.success));
    assert.equal(await count(c, "Account", upA.names[0]), 0);
    assert.equal(await count(c, "Contact", good2.names[0]), 0);
    done.add("destroy");

    // Isolation: another tenant cannot read or delete these records.
    const other = connect(c, "other");
    const unseen = await other.sobject("Account").retrieve([acct], {
      fields: ["Name"],
    });
    assert.equal(unseen[0], null);
    const denied = await other
      .sobject("Account")
      .destroy([acct], { allOrNone: false });
    assert.equal(denied[0].success, false);
    assert.equal(await count(c, "Account", good1.names[0]), 1);
    done.add("isolation");
  }
  assert.deepEqual(
    [...done].sort(),
    [
      "create-part",
      "destroy",
      "isolation",
      "retrieve",
      "update",
      "upsert",
    ].sort(),
  );
});

test("candidate/composite/partial", opts, async () => {
  const c = context();
  const j = journal(c, "cmp");
  const sub = (
    method: string,
    path: string,
    referenceId: string,
    body?: unknown,
  ) => ({
    method,
    url: base + path,
    referenceId,
    ...(body ? { body } : {}),
  });
  const acct = await j.intent("Account", "acct");
  const bad = await j.intent("Account", "bad");
  const dep = await j.intent("Contact", "dep");
  const viaQuery = await j.intent("Contact", "viaquery");
  const collA = await j.intent("Account", "colla");
  const collBad = await j.intent("Account", "collbad");
  const r = await candidateRequest(c, "admin", base + "/composite", "POST", {
    compositeRequest: [
      sub("POST", "/sobjects/Account", "acct", { Name: acct.names[0] }),
      sub("POST", "/sobjects/Account", "bad", {
        Name: bad.names[0],
        Bogus: 1,
      }),
      sub("POST", "/sobjects/Contact", "dep", {
        LastName: dep.names[0],
        AccountId: "@{bad.id}",
      }),
      sub(
        "GET",
        "/query?q=" +
          encodeURIComponent("SELECT Id, Name FROM Account WHERE Id = '") +
          "@{acct.id}" +
          encodeURIComponent("'"),
        "q",
      ),
      sub("GET", "/sobjects/Account/@{acct.id}?fields=Name", "read"),
      sub("POST", "/sobjects/Contact", "viaquery", {
        LastName: viaQuery.names[0],
        AccountId: "@{q.records[0].Id}",
      }),
      sub("POST", "/composite/sobjects", "coll", {
        allOrNone: false,
        records: [
          { attributes: { type: "Account" }, Name: collA.names[0] },
          { attributes: { type: "Account" }, Name: collBad.names[0], Bogus: 1 },
        ],
      }),
    ],
  });
  assert.equal(r.status, 200);
  const e = r.body!.compositeResponse as any[];
  // Journal every committed create before asserting the rest.
  await j.settle(acct, e[0]?.httpStatusCode === 201 ? e[0].body.id : undefined);
  await j.settle(
    viaQuery,
    e[5]?.httpStatusCode === 201 ? e[5].body.id : undefined,
  );
  await j.settle(collA, e[6]?.body?.[0]?.success ? e[6].body[0].id : undefined);
  await j.absent(bad);
  await j.absent(dep);
  await j.absent(collBad);
  assert.deepEqual(
    e.map((x) => x.httpStatusCode),
    [201, 400, 400, 200, 200, 201, 200],
  );
  for (const x of e) assert.deepEqual(x.httpHeaders, {});
  assert.equal(e[1].body[0].errorCode, "INVALID_FIELD");
  assert.equal(e[2].body[0].errorCode, "PROCESSING_HALTED");
  assert.equal(e[3].body.totalSize, 1);
  assert.equal(e[4].body.Name, acct.names[0]);
  assert.equal(e[6].body[1].errors[0].statusCode, "INVALID_FIELD");
  const contact = await candidateRequest(
    c,
    "admin",
    base + "/sobjects/Contact/" + e[5].body.id,
  );
  assert.equal(contact.body!.AccountId, e[0].body.id);

  // All-or-none with a nested Collections failure leaves nothing.
  const rb1 = await j.intent("Account", "rb1");
  const rb2 = await j.intent("Account", "rb2");
  const rolled = await candidateRequest(
    c,
    "admin",
    base + "/composite",
    "POST",
    {
      allOrNone: true,
      compositeRequest: [
        sub("POST", "/sobjects/Account", "rb1", { Name: rb1.names[0] }),
        sub("POST", "/composite/sobjects", "rbcoll", {
          allOrNone: false,
          records: [
            { attributes: { type: "Account" }, Name: rb2.names[0] },
            { attributes: { type: "Account" }, Bogus: 1 },
          ],
        }),
      ],
    },
  );
  await j.absent(rb1);
  await j.absent(rb2);
  assert.equal(rolled.status, 200);
  const re = rolled.body!.compositeResponse as any[];
  assert.equal(re[0].body[0].errorCode, "PROCESSING_HALTED");
  assert.equal(re[1].httpStatusCode, 400);
  assert.equal(re[1].body[1].errors[0].statusCode, "INVALID_FIELD");

  // A reference can never cross tenants.
  const restricted = await candidateRequest(
    c,
    "other",
    base + "/composite",
    "POST",
    {
      compositeRequest: [
        sub("GET", "/sobjects/Account/" + e[0].body.id, "steal"),
        sub("DELETE", "/composite/sobjects?ids=" + e[0].body.id, "del"),
      ],
    },
  );
  assert.equal(restricted.status, 200);
  assert.equal(restricted.body!.compositeResponse[0].httpStatusCode, 404);
  assert.equal(restricted.body!.compositeResponse[1].body[0].success, false);
  assert.equal(await count(c, "Account", acct.names[0]), 1);
});
