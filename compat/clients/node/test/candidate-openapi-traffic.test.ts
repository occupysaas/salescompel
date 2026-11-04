import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { loadCandidateOpenAPI } from "../src/candidate-openapi-contract.ts";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import type { CandidateExchange } from "../src/candidate-openapi-model.ts";
import { ROOT } from "../src/versions.ts";

const bytes = (v: unknown) => Buffer.from(JSON.stringify(v));
const base = "/services/data/v65.0";
const exchange = (
  method = "GET",
  path = base + "/query?q=SELECT+Name+FROM+Account",
  status = 200,
  body: unknown = {
    done: true,
    totalSize: 1,
    records: [{ attributes: { type: "Account" }, Name: "A" }],
  },
): CandidateExchange => ({
  method,
  path,
  request: { contentType: null },
  response: { status, contentType: "application/json", body: bytes(body) },
});
const validator = async () =>
  createCandidateTrafficValidator(await loadCandidateOpenAPI());
test("rejects malformed media parameters in both directions while accepting valid quoted parameters", async () => {
  const v = await validator();
  for (const contentType of [
    "application/json;not a valid parameter",
    'application/json; charset="unterminated',
    "application/json; charset=",
    "application/json\r\n",
    "application/json;",
  ]) {
    const e = exchange("POST", base + "/sobjects/Account", 201, {
      id: "id",
      success: true,
      errors: [],
    });
    e.request = { contentType, body: bytes({ Name: "A" }) };
    assert.deepEqual(v.validate(e).requestIssues, ["request-media"]);
    e.request.contentType = "application/json";
    e.response.contentType = contentType;
    assert.deepEqual(v.validate(e).responseIssues, ["response-media"]);
  }
  const e = exchange();
  e.response.contentType = 'Application/JSON ; charset="utf-8"; note="a;b"';
  valid(v.validate(e));
});
test("rejects schema-valued additionalProperties instead of bypassing subset validation", async () => {
  for (const additionalProperties of [
    { $ref: "#/components/schemas/AccountRead" },
    { type: "string", nullable: true },
    true,
  ]) {
    const d: any = await loadCandidateOpenAPI();
    d.components.schemas.AccountRead.additionalProperties =
      additionalProperties;
    await assert.rejects(
      createCandidateTrafficValidator(d),
      /^Error: candidate-openapi-spec$/,
    );
  }
});
const valid = (r: { requestIssues: string[]; responseIssues: string[] }) => {
  assert.deepEqual(r.requestIssues, []);
  assert.deepEqual(r.responseIssues, []);
};

test("validatesCandidateExchanges", async () => {
  const v = await validator();
  const samples = [
    exchange(),
    exchange("GET", base + "/sobjects/Account/id", 200, {
      attributes: { type: "Account" },
      Id: "id",
      Name: "A",
      Compat_Key__c: null,
    }),
    exchange(
      "GET",
      base + "/query?q=SELECT+Account.Name+FROM+Contact&columns=true",
      200,
      {
        done: true,
        totalSize: 1,
        records: [{ attributes: { type: "Contact" }, Account: null }],
        columnMetadata: [
          { columnName: "Account", joinColumns: [{ columnName: "Name" }] },
        ],
      },
    ),
  ];
  for (const method of ["PATCH", "DELETE"]) {
    const e = exchange(method, base + "/sobjects/Account/id", 204);
    e.response.body = Buffer.alloc(0);
    if (method === "PATCH")
      e.request = {
        contentType: "Application/JSON; charset=utf-8",
        body: bytes({ Name: "new" }),
      };
    samples.push(e);
  }
  const create = exchange("POST", base + "/sobjects/Account", 201, {
    id: "id",
    success: true,
    errors: [],
  });
  create.request = {
    contentType: "application/json",
    body: bytes({ Name: "A", Compat_Text__c: null }),
  };
  samples.push(create);
  const upsert = exchange(
    "PATCH",
    base + "/sobjects/Account/Compat_Key__c/key",
    200,
    { id: "id", created: false, success: true, errors: [] },
  );
  upsert.request = {
    contentType: "application/json",
    body: bytes({ Name: "A" }),
  };
  samples.push(upsert);
  const self = exchange("GET", base + "/openapi.yaml");
  self.response = {
    status: 200,
    contentType: "application/yaml; charset=utf-8",
    body: await readFile(ROOT + "/api/candidate-openapi.yaml"),
  };
  samples.push(self);
  for (const e of samples) valid(v.validate(e));
});
test("rejectsBrokenCandidateTraffic", async () => {
  const v = await validator();
  for (const [change, want] of [
    [
      (e: CandidateExchange) =>
        (e.response.body = bytes({ done: true, records: [] })),
      "response-schema",
    ],
    [
      (e: CandidateExchange) =>
        (e.response.body = bytes({ done: true, totalSize: "1", records: [] })),
      "response-schema",
    ],
    [
      (e: CandidateExchange) =>
        (e.response.body = bytes({
          done: true,
          totalSize: 0,
          records: [],
          unknown: true,
        })),
      "response-schema",
    ],
    [
      (e: CandidateExchange) =>
        (e.response.body = bytes({
          done: true,
          totalSize: 1,
          records: [{ attributes: { type: "Account" }, Unknown: "value" }],
        })),
      "response-schema",
    ],
    [
      (e: CandidateExchange) => (e.response.body = Buffer.from("{")),
      "response-json",
    ],
    [
      (e: CandidateExchange) => (e.response.body = Buffer.from([0xff])),
      "response-json",
    ],
    [(e: CandidateExchange) => (e.response.status = 299), "status"],
    [(e: CandidateExchange) => (e.response.status = 418), "status"],
    [
      (e: CandidateExchange) => (e.response.contentType = null),
      "response-media",
    ],
    [
      (e: CandidateExchange) => (e.response.contentType = "text/plain"),
      "response-media",
    ],
  ] as const) {
    const e = exchange();
    change(e);
    assert.deepEqual(v.validate(e).responseIssues, [want]);
  }
  const noContent = exchange("DELETE", base + "/sobjects/Account/id", 204);
  assert.deepEqual(v.validate(noContent).responseIssues, ["response-body"]);
  noContent.response.body = Buffer.alloc(0);
  valid(v.validate(noContent));
  const bad = exchange("POST", base + "/sobjects/Account", 400, [
    { errorCode: "JSON_PARSER_ERROR", message: "Rejected" },
  ]);
  bad.request = { contentType: "application/json", body: Buffer.from("{") };
  assert.deepEqual(v.validate(bad).requestIssues, ["request-json"]);
  assert.deepEqual(v.validate(bad).responseIssues, []);
  bad.request.body = bytes({ Name: 7 });
  assert.deepEqual(v.validate(bad).requestIssues, ["request-schema"]);
  bad.request.contentType = null;
  assert.deepEqual(v.validate(bad).requestIssues, ["request-media"]);
  bad.request = { contentType: null };
  assert.deepEqual(v.validate(bad).requestIssues, ["request-schema"]);
});
test("matchesCandidatePaths", async () => {
  const v = await validator();
  assert.equal(
    v.validate(exchange("GET", base + "/sobjects/Account/describe"))
      .operationId,
    "candidateDescribeAccount",
  );
  for (const key of ["a%2Fb", "a%252Fb"]) {
    const e = exchange(
      "PATCH",
      base + "/sobjects/Account/Compat_Key__c/" + key,
      201,
      { id: "id", created: true, success: true, errors: [] },
    );
    e.request = { contentType: "application/json", body: bytes({ Name: "A" }) };
    valid(v.validate(e));
    assert.equal(v.validate(e).operationId, "candidateUpsertAccount");
  }
  assert.equal(
    v.validate(exchange("GET", "/services/data/v51.0/sobjects/User/me"))
      .operationId,
    "candidateOwnUser51",
  );
  for (const [method, path, want] of [
    ["GET", "/services/data/v99.0/openapi.yaml", "route"],
    ["POST", base + "/openapi.yaml", "method"],
    ["GET", "//evil.invalid/path", "route"],
  ]) {
    const r = v.validate(exchange(method, path));
    assert.equal(r.operationId, undefined);
    assert.deepEqual(r.requestIssues, [want]);
    assert.deepEqual(r.responseIssues, []);
  }
  const malformed = exchange("GET", base + "/sobjects/Account/%ZZ");
  assert.deepEqual(v.validate(malformed).requestIssues, ["parameters"]);
  valid(
    v.validate(exchange("GET", base + "/query?q=first&q=second&ignored=yes")),
  );
  for (const path of [
    base + "/query",
    base + "/query?q=x&columns=true&columns=false",
    base + "/query?q=x&columns=1",
    base + "/query?q=%ZZ",
  ]) {
    assert.deepEqual(v.validate(exchange("GET", path)).requestIssues, [
      "parameters",
    ]);
  }
});
test("boundsCandidateTraffic", async () => {
  const v = await validator();
  const e = exchange("POST", base + "/sobjects/Account", 201, {
    id: "id",
    success: true,
    errors: [],
  });
  const input = bytes({ Name: "A" });
  e.request = {
    contentType: "application/json",
    body: Buffer.concat([input, Buffer.alloc(1048576 - input.length, 32)]),
  };
  valid(v.validate(e));
  e.request.body = Buffer.alloc(1048577);
  assert.deepEqual(v.validate(e).requestIssues, ["budget"]);
  const out = exchange();
  const initial = out.response.body;
  out.response.body = Buffer.concat([
    initial,
    Buffer.alloc(8388608 - initial.length, 32),
  ]);
  valid(v.validate(out));
  out.response.body = Buffer.alloc(8388609);
  assert.deepEqual(v.validate(out).responseIssues, ["budget"]);
});
test("keepsTrafficDiagnosticsPrivate", async () => {
  const v = await validator();
  const e = exchange("POST", base + "/sobjects/Account", 418, {
    PRIVATE_MARKER: "secret",
  });
  e.request = {
    contentType: "PRIVATE_MARKER",
    body: bytes({ Name: "PRIVATE_MARKER", Unknown: "PRIVATE_MARKER" }),
  };
  const r = v.validate(e);
  assert.deepEqual(r, {
    operationId: "candidateCreateAccount",
    requestIssues: ["request-media"],
    responseIssues: ["status"],
  });
  assert.ok(!JSON.stringify(r).includes("PRIVATE_MARKER"));
});
test("rejectsUnsupportedTrafficConstructs", async () => {
  for (const mutate of [
    (d: any) =>
      (d.paths[base + "/query"].get.parameters[0].style = "deepObject"),
    (d: any) =>
      (d.components.schemas.AccountRead.format = "unimplemented-format"),
    (d: any) =>
      (d.paths[base + "/query"].get.responses["200"].content[
        "application/json"
      ].schema = { $ref: "https://PRIVATE_MARKER.invalid/schema" }),
    (d: any) =>
      (d.components.schemas.AccountRead = {
        type: "object",
        properties: { next: { $ref: "#/components/schemas/AccountRead" } },
      }),
  ]) {
    const d = await loadCandidateOpenAPI();
    mutate(d);
    await assert.rejects(
      createCandidateTrafficValidator(d),
      /^Error: candidate-openapi-spec$/,
    );
  }
});
