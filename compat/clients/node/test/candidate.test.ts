import test from "node:test";
import assert from "node:assert/strict";
import { Connection } from "jsforce";

// This is a candidate integration gate, not a Salesforce reference comparison.
test(
  "candidate: real JSforce CRUD, policy, relationships and upsert",
  {
    skip: process.env.RUN_CANDIDATE !== "1",
  },
  async () => {
    function connection(tokenName: string) {
      assert.ok(process.env.CANDIDATE_URL, "CANDIDATE_URL required");
      assert.ok(process.env[tokenName], `${tokenName} required`);
      return new Connection({
        instanceUrl: process.env.CANDIDATE_URL,
        accessToken: process.env[tokenName],
        version: "65.0",
      });
    }
    const conn = connection("CANDIDATE_TOKEN");
    const restricted = connection("CANDIDATE_RESTRICTED_TOKEN");
    const other = connection("CANDIDATE_OTHER_TOKEN");
    const created: { type: string; id: string }[] = [];
    const key = `m2-${crypto.randomUUID()}`;
    try {
      const d = await conn.sobject("Account").describe();
      for (const name of ["Compat_Text__c", "Compat_Key__c"]) {
        const f = d.fields.find((f) => f.name === name);
        assert.ok(f);
        assert.equal(f.length, 80);
        assert.equal(f.type, "string");
        assert.equal(f.nillable, true);
        assert.equal(f.externalId, name === "Compat_Key__c");
        assert.deepEqual(f.referenceTo, []);
      }
      assert.ok(
        !(await restricted.sobject("Account").describe()).fields.some(
          (f) => f.name === "Compat_Text__c",
        ),
      );
      const a: any = await conn
        .sobject("Account")
        .create({ Name: key, Compat_Text__c: "secret" });
      assert.equal(a.success, true);
      created.push({ type: "Account", id: a.id });
      await assert.rejects(other.sobject("Account").retrieve(a.id), {
        errorCode: "NOT_FOUND",
      });
      const contact: any = await conn
        .sobject("Contact")
        .create({ LastName: "O'Brien", AccountId: a.id });
      created.push({ type: "Contact", id: contact.id });
      const q: any = await conn.query(
        `SELECT Id,LastName,Account.Name FROM Contact WHERE AccountId = '${a.id}'`,
      );
      assert.equal(q.records.length, 1);
      assert.equal(q.records[0].Account.Name, key);
      await conn
        .sobject("Account")
        .update({ Id: a.id, Name: "Updated", Compat_Text__c: null });
      const row: any = await conn.sobject("Account").retrieve(a.id);
      assert.equal(row.Name, "Updated");
      assert.equal(row.Compat_Text__c, null);
      await assert.rejects(
        async () =>
          await restricted.query(
            "SELECT Id FROM Account WHERE Compat_Text__c = 'secret'",
          ),
        { errorCode: "INVALID_FIELD" },
      );
      const u: any = await conn
        .sobject("Account")
        .upsert({ Name: key, Compat_Key__c: key }, "Compat_Key__c");
      assert.equal(u.created, true);
      created.push({ type: "Account", id: u.id });
      const u2: any = await conn
        .sobject("Account")
        .upsert(
          { Compat_Key__c: key, Compat_Text__c: "retained" },
          "Compat_Key__c",
        );
      assert.equal(u2.created, false);
      assert.equal(u2.id, u.id);
      assert.equal(
        ((await conn.sobject("Account").retrieve(u.id)) as any).Name,
        key,
      );
    } finally {
      // Children before parents, exact owned IDs only.
      for (const item of created
        .filter((x) => x.type === "Contact")
        .concat(created.filter((x) => x.type === "Account"))) {
        assert.equal(
          (await conn.sobject(item.type).destroy(item.id)).success,
          true,
        );
      }
    }
  },
);
