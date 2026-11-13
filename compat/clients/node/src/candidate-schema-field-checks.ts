import assert from "node:assert/strict";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import {
  candidateRequest,
  recordFixtureEvent,
  type FixtureEvent,
} from "./candidate-acceptance-fixtures.ts";
import { createCandidateTrafficValidator } from "./candidate-openapi-traffic.ts";
import { fieldTypeNames } from "./candidate-schema-field-fixtures.ts";
export type SchemaClientContext = Pick<
  GateContext,
  "runId" | "scratch" | "url" | "tokens" | "fixtureJournal"
>;
type DescribeField = {
  name: string;
  picklistValues?: Array<{ value: string; defaultValue: boolean }>;
  [key: string]: unknown;
};
type DescribeTable = Record<string, DescribeField>;
async function rejectsWith(promise: Promise<unknown>, code: string) {
  await assert.rejects(promise, (e: { errorCode?: string }) => {
    assert.equal(e.errorCode, code);
    return true;
  });
}
/** Plan 28: every active field type through unmodified JSforce. Records in
 * the P28 objects are removed with their journaled tables. */
export async function fieldTypeClientChecks(
  c: SchemaClientContext,
  // guardrails-allow PREVENT-011: JSforce connection from createRequire; the
  // dynamic record surface is exercised through the helpers below, not any.
  conn: Record<string, any>,
  passed: (id: string) => void,
  v: Awaited<ReturnType<typeof createCandidateTrafficValidator>>,
  observed: Set<string>,
) {
  const n = fieldTypeNames(c.runId);
  const fields = (d: { fields: DescribeField[] }): DescribeTable =>
    Object.fromEntries(
      d.fields.map((f): [string, DescribeField] => [f.name, f]),
    );
  const item = fields(await conn.sobject(n.item).describe());
  const expect = (
    f: DescribeField | undefined,
    values: Record<string, unknown>,
  ) => {
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
    item.Stage__c?.picklistValues?.map((p) => [p.value, p.defaultValue]),
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
    (
      composite.body as { compositeResponse: Array<{ httpStatusCode: number }> }
    ).compositeResponse.every((r) => r.httpStatusCode === 400),
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
  // Outside the JSforce recorder: this request is invalid by contract on purpose.
  const nameWrite = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/sobjects/Contact/${contact}`,
    "PATCH",
    { Name: "x" },
  );
  assert.equal(nameWrite.status, 400);
  assert.equal(
    nameWrite.body?.[0].errorCode,
    "INVALID_FIELD_FOR_INSERT_UPDATE",
  );
  passed("schema/field-types-standard");

  const stored = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/candidate/sobjects/${n.item}/storedFields`,
  );
  assert.deepEqual(
    (
      stored.body as {
        storedFields: Array<{ name: string; type: string }>;
      }
    ).storedFields.map((f) => [f.name, f.type]),
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
