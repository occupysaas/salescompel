import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { loadCandidateOpenAPI } from "../src/candidate-openapi-contract.ts";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import type { CandidateExchange } from "../src/candidate-openapi-model.ts";
import { ROOT } from "../src/versions.ts";

const base = "/services/data/v65.0";
const bytes = (v: unknown) => Buffer.from(JSON.stringify(v));
const exchange = (
  method: string,
  path: string,
  status: number,
  body: unknown,
  request?: unknown,
): CandidateExchange => ({
  method,
  path,
  request:
    request === undefined
      ? { contentType: null }
      : { contentType: "application/json", body: bytes(request) },
  response:
    status === 204
      ? { status, contentType: null, body: Buffer.alloc(0) }
      : { status, contentType: "application/json", body: bytes(body) },
});
const validator = async () =>
  createCandidateTrafficValidator(await loadCandidateOpenAPI());
const ok = (r: { requestIssues: string[]; responseIssues: string[] }) =>
  assert.deepEqual([r.requestIssues, r.responseIssues], [[], []]);

// Real describe output produced by the Go golden test. Each object is a map of
// field name to field JSON, plus a childRelationships key that ObjectDescribe
// now wires in.
const golden = async () =>
  JSON.parse(
    await readFile(
      join(ROOT, "internal/platform/testdata/describe-field-types.json"),
      "utf8",
    ),
  ) as Record<string, Record<string, any>>;
const describe = (name: string, raw: Record<string, any>) => ({
  name,
  label: name,
  labelPlural: name + "s",
  fields: Object.entries(raw)
    .filter(([key]) => key !== "childRelationships")
    .map(([, value]) => value),
  childRelationships: raw.childRelationships ?? [],
  createable: true,
  updateable: true,
  deletable: true,
  queryable: true,
});

test("describe accepts every Plan 28 field type and property", async () => {
  const v = await validator();
  const g = await golden();
  for (const [object, raw] of Object.entries(g)) {
    ok(
      v.validate(
        exchange(
          "GET",
          `${base}/sobjects/${object}/describe`,
          200,
          describe(object, raw),
        ),
      ),
    );
  }
});

test("describe rejects unknown field types and properties", async () => {
  const v = await validator();
  const g = await golden();
  const item = structuredClone(g.Item__c);
  item.Num__c.type = "time";
  assert.deepEqual(
    v.validate(
      exchange(
        "GET",
        `${base}/sobjects/Item__c/describe`,
        200,
        describe("Item__c", item),
      ),
    ).responseIssues,
    ["response-schema"],
  );
  const extra = structuredClone(g.Item__c);
  extra.Num__c.calculatedFormula = "1+1";
  assert.deepEqual(
    v.validate(
      exchange(
        "GET",
        `${base}/sobjects/Item__c/describe`,
        200,
        describe("Item__c", extra),
      ),
    ).responseIssues,
    ["response-schema"],
  );
});

const item = {
  attributes: {
    type: "Item__c",
    url: `${base}/sobjects/Item__c/a01000000000001AAA`,
  },
  Id: "a01000000000001AAA",
  Name: "IT-0001",
  Txt__c: "hello",
  Long__c: "x".repeat(131072),
  Num__c: 0.3,
  Amount__c: 12.5,
  Pct__c: -7,
  Day__c: "2026-11-11",
  At__c: "2026-03-09T04:30:00.000+0000",
  Flag__c: true,
  Multi__c: "A;C",
  Stage__c: null,
  Parent__c: "a00000000000001AAA",
};

test("typed custom record values round-trip through dynamic schemas", async () => {
  const v = await validator();
  const { attributes, Id, Name, ...writable } = item;
  ok(
    v.validate(
      exchange(
        "POST",
        `${base}/sobjects/Item__c`,
        201,
        { id: Id, success: true, errors: [] },
        writable,
      ),
    ),
  );
  ok(
    v.validate(
      exchange("PATCH", `${base}/sobjects/Item__c/${Id}`, 204, null, {
        Flag__c: false,
        Num__c: "12.5",
      }),
    ),
  );
  ok(v.validate(exchange("GET", `${base}/sobjects/Item__c/${Id}`, 200, item)));
  ok(
    v.validate(
      exchange("GET", `${base}/query?q=SELECT+Id+FROM+Item__c`, 200, {
        done: true,
        totalSize: 1,
        records: [item],
      }),
    ),
  );
  const bad = exchange(
    "POST",
    `${base}/sobjects/Item__c`,
    201,
    { id: Id, success: true, errors: [] },
    { Num__c: { nested: 1 } },
  );
  assert.deepEqual(v.validate(bad).requestIssues, ["request-schema"]);
});

test("standard Account and Contact fields and compound reads", async () => {
  const v = await validator();
  const account = {
    attributes: {
      type: "Account",
      url: `${base}/sobjects/Account/001000000000001AAA`,
    },
    Id: "001000000000001AAA",
    Name: "Acme",
    Compat_Key__c: null,
    Phone: "+1 555 0100",
    Website: "example.com",
    Industry: "Banking",
    AnnualRevenue: 1000000,
    NumberOfEmployees: 12,
    Description: "d",
    BillingCity: "Lyon",
    BillingAddress: {
      street: null,
      city: "Lyon",
      state: null,
      postalCode: null,
      country: "France",
    },
    ShippingAddress: null,
  };
  ok(
    v.validate(
      exchange("GET", `${base}/sobjects/Account/${account.Id}`, 200, account),
    ),
  );
  ok(
    v.validate(
      exchange(
        "POST",
        `${base}/sobjects/Account`,
        201,
        { id: account.Id, success: true, errors: [] },
        {
          Name: "Acme",
          AnnualRevenue: 5,
          BillingCity: "Lyon",
          Industry: "Banking",
        },
      ),
    ),
  );
  const contact = {
    attributes: {
      type: "Contact",
      url: `${base}/sobjects/Contact/003000000000001AAA`,
    },
    Id: "003000000000001AAA",
    FirstName: "Ada",
    LastName: "Lovelace",
    Name: "Ada Lovelace",
    AccountId: null,
    Email: "ada@example.org",
    Birthdate: "1815-12-10",
    MailingAddress: null,
  };
  ok(
    v.validate(
      exchange("GET", `${base}/sobjects/Contact/${contact.Id}`, 200, contact),
    ),
  );
  const writeAddress = exchange(
    "POST",
    `${base}/sobjects/Account`,
    201,
    { id: "x", success: true, errors: [] },
    { Name: "A", BillingAddress: { city: "X" } },
  );
  assert.deepEqual(v.validate(writeAddress).requestIssues, ["request-schema"]);
  const writeName = exchange(
    "POST",
    `${base}/sobjects/Contact`,
    201,
    { id: "x", success: true, errors: [] },
    { LastName: "L", Name: "Full" },
  );
  assert.deepEqual(v.validate(writeName).requestIssues, ["request-schema"]);
});

test("currency bridge and new error codes", async () => {
  const v = await validator();
  ok(
    v.validate(
      exchange(
        "GET",
        `${base}/query?q=SELECT+DefaultCurrencyIsoCode+FROM+Organization`,
        200,
        {
          totalSize: 1,
          done: true,
          records: [
            {
              attributes: { type: "Organization" },
              DefaultCurrencyIsoCode: "EUR",
            },
          ],
        },
      ),
    ),
  );
  for (const errorCode of [
    "NUMBER_OUTSIDE_VALID_RANGE",
    "INVALID_EMAIL_ADDRESS",
    "INVALID_OR_NULL_FOR_RESTRICTED_PICKLIST",
    "DELETE_FAILED",
    "INVALID_FIELD_FOR_INSERT_UPDATE",
  ]) {
    ok(
      v.validate(
        exchange(
          "POST",
          `${base}/sobjects/Item__c`,
          400,
          [{ errorCode, message: "m", fields: ["F__c"] }],
          { Num__c: 1 },
        ),
      ),
    );
  }
  ok(
    v.validate(
      exchange("DELETE", `${base}/sobjects/Parent__c/a00000000000001AAA`, 400, [
        { errorCode: "DELETE_FAILED", message: "m" },
      ]),
    ),
  );
});
