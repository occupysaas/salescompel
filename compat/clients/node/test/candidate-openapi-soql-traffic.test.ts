import assert from "node:assert/strict";
import test from "node:test";
import { loadCandidateOpenAPI } from "../src/candidate-openapi-contract.ts";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import {
  aggregateOverParentPath,
  aggregateParentPath,
  base,
  bytes,
  childDeepParentPath,
  childParentPath,
  childPlainAndParentLeaf,
  childRelationship,
  customParentAccount,
  customParentContact,
  exchange,
  nestedParentPath,
  parentPath,
  query,
} from "./helpers/candidate-openapi-soql-fixtures.ts";

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
    childPlainAndParentLeaf(),
    childParentPath(),
    childDeepParentPath(),
    customParentAccount(),
    customParentContact(),
    aggregate(),
    aggregateParentPath(),
    aggregateOverParentPath(),
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

  const tooDeepChildColumn = childDeepParentPath();
  const deepBody = JSON.parse(
    Buffer.from(tooDeepChildColumn.response.body).toString("utf8"),
  );
  let pointer = deepBody.columnMetadata[1].joinColumns[0];
  while (pointer.joinColumns[0]?.joinColumns?.length)
    pointer = pointer.joinColumns[0];
  pointer.joinColumns = [
    { columnName: "F__r", joinColumns: [{ columnName: "Name" }] },
  ];
  tooDeepChildColumn.response.body = bytes(deepBody);
  assert.deepEqual(v.validate(tooDeepChildColumn).responseIssues, [
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
