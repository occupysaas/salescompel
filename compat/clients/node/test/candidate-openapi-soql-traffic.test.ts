import assert from "node:assert/strict";
import test from "node:test";
import { loadCandidateOpenAPI } from "../src/candidate-openapi-contract.ts";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import type { CandidateExchange } from "../src/candidate-openapi-model.ts";

const base = "/services/data/v65.0";
const bytes = (v: unknown) => Buffer.from(JSON.stringify(v));
const query = (q: string, extra = "") =>
  base + "/query?q=" + encodeURIComponent(q) + extra;
const exchange = (
  method: string,
  path: string,
  status = 200,
  body: unknown = { done: true, totalSize: 0, records: [] },
): CandidateExchange => ({
  method,
  path,
  request: { contentType: null },
  response: { status, contentType: "application/json", body: bytes(body) },
});
const validator = async () =>
  createCandidateTrafficValidator(await loadCandidateOpenAPI());
const valid = (r: { requestIssues: string[]; responseIssues: string[] }) => {
  assert.deepEqual(r.requestIssues, []);
  assert.deepEqual(r.responseIssues, []);
};
const locator = "01g000000000000AAA-2000";

const account = (name: string) => ({
  attributes: { type: "Account" },
  Name: name,
});

const parentPath = () =>
  exchange(
    "GET",
    query("SELECT Account.Name FROM Contact", "&columns=true"),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Contact" },
          Account: { attributes: { type: "Account" }, Name: "Acme" },
        },
      ],
      columnMetadata: [
        { columnName: "Account", joinColumns: [{ columnName: "Name" }] },
      ],
    },
  );

const nestedParentPath = () =>
  exchange("GET", query("SELECT Parent__r.Parent__r.Name FROM Child__c"), 200, {
    done: true,
    totalSize: 1,
    records: [
      {
        attributes: { type: "Child__c" },
        Parent__r: {
          attributes: { type: "Parent__c" },
          Parent__r: { attributes: { type: "Parent__c" }, Name: "Root" },
        },
      },
    ],
    columnMetadata: [
      {
        columnName: "Parent__r",
        joinColumns: [
          {
            columnName: "Parent__r",
            joinColumns: [{ columnName: "Name" }],
          },
        ],
      },
    ],
  });

const childRelationship = () =>
  exchange(
    "GET",
    query("SELECT Name, (SELECT LastName FROM Contacts) FROM Account"),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Account" },
          Name: "Acme",
          Contacts: {
            totalSize: 1,
            done: true,
            records: [{ attributes: { type: "Contact" }, LastName: "Child" }],
          },
        },
      ],
      columnMetadata: [
        { columnName: "Name" },
        { columnName: "Contacts", joinColumns: [{ columnName: "LastName" }] },
      ],
    },
  );

const childParentPath = () =>
  exchange(
    "GET",
    query(
      "SELECT Name, (SELECT Account.Name, MailingAddress FROM Contacts) FROM Account",
    ),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Account" },
          Name: "Acme",
          Contacts: {
            totalSize: 1,
            done: true,
            records: [
              {
                attributes: { type: "Contact" },
                Account: { attributes: { type: "Account" }, Name: "Acme" },
                MailingAddress: {
                  street: "9 Oak",
                  city: "Portland",
                  state: "OR",
                  postalCode: "97201",
                  country: "US",
                },
              },
            ],
          },
        },
      ],
      columnMetadata: [
        { columnName: "Name" },
        {
          columnName: "Contacts",
          joinColumns: [
            { columnName: "Account", joinColumns: [{ columnName: "Name" }] },
            { columnName: "MailingAddress" },
          ],
        },
      ],
    },
  );

const customParentAccount = () =>
  exchange(
    "GET",
    query("SELECT Name, CustomLookup__r.Name FROM Account"),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        {
          attributes: { type: "Account" },
          Name: "Acme",
          CustomLookup__r: {
            attributes: { type: "Contact" },
            Name: "Linked",
          },
        },
      ],
      columnMetadata: [
        { columnName: "Name" },
        {
          columnName: "CustomLookup__r",
          joinColumns: [{ columnName: "Name" }],
        },
      ],
    },
  );

const customParentContact = () =>
  exchange("GET", query("SELECT LastName, Other__r.Name FROM Contact"), 200, {
    done: true,
    totalSize: 1,
    records: [
      {
        attributes: { type: "Contact" },
        LastName: "Child",
        Other__r: { attributes: { type: "Account" }, Name: "Linked" },
      },
    ],
    columnMetadata: [
      { columnName: "LastName" },
      { columnName: "Other__r", joinColumns: [{ columnName: "Name" }] },
    ],
  });

const aggregate = () =>
  exchange(
    "GET",
    query("SELECT Stage__c, COUNT(Id) total FROM Item__c GROUP BY Stage__c"),
    200,
    {
      done: true,
      totalSize: 1,
      records: [
        { attributes: { type: "AggregateResult" }, Stage__c: "Open", total: 2 },
      ],
      columnMetadata: [
        { columnName: "Stage__c" },
        { columnName: "total", aggregate: true },
      ],
    },
  );

const countOnly = () =>
  exchange("GET", query("SELECT COUNT() FROM Item__c"), 200, {
    done: true,
    totalSize: 3,
    records: [],
    columnMetadata: [{ columnName: "expr0", aggregate: true }],
  });

const firstPage = () =>
  exchange("GET", query("SELECT Id, Name FROM Account"), 200, {
    done: false,
    totalSize: 2001,
    records: [account("Page 1")],
    nextRecordsUrl: base + "/query/" + locator,
  });

const secondPage = () =>
  exchange("GET", base + "/query/" + locator, 200, {
    done: true,
    totalSize: 2001,
    records: [account("Page 2")],
  });

const describe = (object = "Account", childSObject = "Contact") =>
  exchange("GET", base + `/sobjects/${object}/describe`, 200, {
    name: object,
    label: object,
    labelPlural: object + "s",
    fields: [],
    childRelationships: [
      {
        childSObject,
        field: "AccountId",
        relationshipName: "Contacts",
        cascadeDelete: false,
        restrictedDelete: true,
        deprecatedAndHidden: false,
      },
    ],
    createable: true,
    updateable: true,
    deletable: true,
    queryable: true,
  });

test("accepts SOQL contract additions", async () => {
  const v = await validator();
  const samples = [
    parentPath(),
    nestedParentPath(),
    childRelationship(),
    childParentPath(),
    customParentAccount(),
    customParentContact(),
    aggregate(),
    countOnly(),
    firstPage(),
    secondPage(),
    describe(),
    describe("Child__c", "Grandchild__c"),
  ];
  for (const e of samples) valid(v.validate(e));
  assert.equal(v.validate(parentPath()).operationId, "candidateQuery");
  assert.equal(v.validate(secondPage()).operationId, "candidateQueryMore");
  assert.equal(v.validate(describe()).operationId, "candidateDescribeAccount");
});

test("accepts every locked query error code", async () => {
  const v = await validator();
  const codes = [
    "MALFORMED_QUERY",
    "INVALID_FIELD",
    "INVALID_TYPE",
    "INVALID_QUERY_FILTER_OPERATOR",
    "QUERY_TOO_COMPLICATED",
  ];
  for (const errorCode of codes) {
    const e = exchange("GET", query("SELECT Id FROM Account"), 400, [
      { errorCode, message: "Rejected" },
    ]);
    const result = v.validate(e);
    valid(result);
    assert.equal(result.operationId, "candidateQuery");
  }
  const locatorError = exchange("GET", base + "/query/" + locator, 400, [
    { errorCode: "INVALID_QUERY_LOCATOR", message: "Query locator is invalid" },
  ]);
  const result = v.validate(locatorError);
  valid(result);
  assert.equal(result.operationId, "candidateQueryMore");
});

test("rejects malformed SOQL contract exchanges", async () => {
  const v = await validator();

  const badParent = parentPath();
  badParent.response.body = bytes({
    ...JSON.parse(Buffer.from(badParent.response.body).toString("utf8")),
    records: [{ attributes: { type: "Contact" }, Account: "not-an-object" }],
  });
  assert.deepEqual(v.validate(badParent).responseIssues, ["response-schema"]);

  const tooDeep = nestedParentPath();
  let value: any = "Root";
  for (let depth = 0; depth < 6; depth++)
    value = { attributes: { type: "Parent__c" }, Parent__r: value };
  tooDeep.response.body = bytes({
    ...JSON.parse(Buffer.from(tooDeep.response.body).toString("utf8")),
    records: [{ attributes: { type: "Child__c" }, Parent__r: value }],
  });
  assert.deepEqual(v.validate(tooDeep).responseIssues, ["response-schema"]);

  const badChild = childRelationship();
  badChild.response.body = bytes({
    ...JSON.parse(Buffer.from(badChild.response.body).toString("utf8")),
    records: [
      {
        attributes: { type: "Account" },
        Name: "Acme",
        Contacts: { totalSize: 1, done: true },
      },
    ],
  });
  assert.deepEqual(v.validate(badChild).responseIssues, ["response-schema"]);

  const badChildParent = childParentPath();
  badChildParent.response.body = bytes({
    ...JSON.parse(Buffer.from(badChildParent.response.body).toString("utf8")),
    records: [
      {
        attributes: { type: "Account" },
        Name: "Acme",
        Contacts: {
          totalSize: 1,
          done: true,
          records: [
            {
              attributes: { type: "Contact" },
              Account: { unknown: true },
              MailingAddress: { unknown: true },
            },
          ],
        },
      },
    ],
  });
  assert.deepEqual(v.validate(badChildParent).responseIssues, [
    "response-schema",
  ]);

  const badCustomParentAccount = customParentAccount();
  badCustomParentAccount.response.body = bytes({
    ...JSON.parse(
      Buffer.from(badCustomParentAccount.response.body).toString("utf8"),
    ),
    records: [
      {
        attributes: { type: "Account" },
        Name: "Acme",
        CustomLookup__r: "not-an-object",
      },
    ],
  });
  assert.deepEqual(v.validate(badCustomParentAccount).responseIssues, [
    "response-schema",
  ]);

  const badCustomParentContact = customParentContact();
  badCustomParentContact.response.body = bytes({
    ...JSON.parse(
      Buffer.from(badCustomParentContact.response.body).toString("utf8"),
    ),
    records: [
      {
        attributes: { type: "Contact" },
        LastName: "Child",
        Other__r: "not-an-object",
      },
    ],
  });
  assert.deepEqual(v.validate(badCustomParentContact).responseIssues, [
    "response-schema",
  ]);

  const badAggregate = aggregate();
  badAggregate.response.body = bytes({
    ...JSON.parse(Buffer.from(badAggregate.response.body).toString("utf8")),
    records: [
      {
        attributes: { type: "AggregateResult" },
        total: { nested: 1 },
      },
    ],
  });
  assert.deepEqual(v.validate(badAggregate).responseIssues, [
    "response-schema",
  ]);

  const badCount = countOnly();
  badCount.response.body = bytes({
    ...JSON.parse(Buffer.from(badCount.response.body).toString("utf8")),
    columnMetadata: [{ columnName: "expr0", aggregate: "yes" }],
  });
  assert.deepEqual(v.validate(badCount).responseIssues, ["response-schema"]);

  const badPage = firstPage();
  badPage.response.body = bytes({
    ...JSON.parse(Buffer.from(badPage.response.body).toString("utf8")),
    nextRecordsUrl: "https://PRIVATE_MARKER.invalid/query/1",
  });
  assert.deepEqual(v.validate(badPage).responseIssues, ["response-schema"]);

  const badLocator = exchange("GET", base + "/query/not-a-locator");
  assert.deepEqual(v.validate(badLocator).requestIssues, ["parameters"]);

  const badDescribe = describe();
  const describeBody = JSON.parse(
    Buffer.from(badDescribe.response.body).toString("utf8"),
  );
  delete describeBody.childRelationships;
  badDescribe.response.body = bytes(describeBody);
  assert.deepEqual(v.validate(badDescribe).responseIssues, ["response-schema"]);

  for (const errorCode of [
    "MALFORMED_QUERY",
    "INVALID_FIELD",
    "INVALID_TYPE",
    "INVALID_QUERY_FILTER_OPERATOR",
    "QUERY_TOO_COMPLICATED",
    "INVALID_QUERY_LOCATOR",
  ]) {
    const e = exchange("GET", query("SELECT Id FROM Account"), 400, [
      { errorCode, message: 1 },
    ]);
    assert.deepEqual(v.validate(e).responseIssues, ["response-schema"]);
  }
});
