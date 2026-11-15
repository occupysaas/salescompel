import assert from "node:assert/strict";
import test from "node:test";
import { Ajv2020 } from "ajv/dist/2020.js";
import { readFile } from "node:fs/promises";
import { parse } from "yaml";
import {
  loadCandidateOpenAPI,
  validateCandidateOpenAPI,
} from "../src/candidate-openapi-contract.ts";
import { ROOT } from "../src/versions.ts";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import type { CandidateExchange } from "../src/candidate-openapi-model.ts";

// Plan 31 traffic: sObject Collections operations and record reads with fields.
const bytes = (v: unknown) => Buffer.from(JSON.stringify(v));
const base = "/services/data/v65.0";
const exchange = (
  method: string,
  path: string,
  status: number,
  body: unknown,
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

test("validates Plan 31 Collections and record field traffic", async () => {
  const v = await validator();
  const withBody = (e: CandidateExchange, request: unknown) => {
    e.request = { contentType: "application/json", body: bytes(request) };
    return e;
  };
  const ok = { id: "001000000000000AAA", success: true, errors: [] };
  const failed = {
    success: false,
    errors: [{ statusCode: "INVALID_FIELD", message: "m", fields: [] }],
  };
  const write = {
    allOrNone: false,
    records: [{ attributes: { type: "Account" }, Name: "A" }],
  };
  valid(
    v.validate(
      withBody(
        exchange("POST", base + "/composite/sobjects", 200, [ok, failed]),
        write,
      ),
    ),
  );
  valid(
    v.validate(
      withBody(
        exchange(
          "PATCH",
          base + "/composite/sobjects/Account/Compat_Key__c",
          200,
          [{ ...ok, created: true }],
        ),
        {
          records: [
            { attributes: { type: "Account" }, Compat_Key__c: "k", Name: "A" },
          ],
        },
      ),
    ),
  );
  valid(
    v.validate(
      exchange(
        "DELETE",
        base + "/composite/sobjects?ids=001000000000000AAA&allOrNone=true",
        200,
        [ok],
      ),
    ),
  );
  const record = { attributes: { type: "Account" }, Id: "001", Name: "A" };
  valid(
    v.validate(
      withBody(
        exchange("POST", base + "/composite/sobjects/Account", 200, [
          record,
          null,
        ]),
        { ids: ["001", "bad"], fields: ["Name"] },
      ),
    ),
  );
  valid(
    v.validate(
      exchange(
        "GET",
        base + "/composite/sobjects/Account?ids=001&fields=Name",
        200,
        [record],
      ),
    ),
  );
  valid(
    v.validate(
      exchange("GET", base + "/sobjects/Account/001?fields=Name", 200, record),
    ),
  );
  for (const bad of [
    { records: [{ Name: "no attributes" }] },
    { records: [] },
    { allOrNone: "true", records: write.records },
  ])
    assert.ok(
      v
        .validate(
          withBody(
            exchange("POST", base + "/composite/sobjects", 200, [ok]),
            bad,
          ),
        )
        .requestIssues.includes("request-schema"),
    );
  for (const bad of [
    [{ success: false, errors: [] }],
    [{ ...ok, success: "yes" }],
  ])
    assert.ok(
      v
        .validate(
          withBody(
            exchange("POST", base + "/composite/sobjects", 200, bad),
            write,
          ),
        )
        .responseIssues.includes("response-schema"),
    );
});

const doc = async (): Promise<Record<string, any>> =>
  loadCandidateOpenAPI() as Promise<Record<string, any>>;
const urls = async () =>
  new Set<string>(
    parse(
      await readFile(ROOT + "/docs/compat/sources.yaml", "utf8"),
    ).sources.map((s: { url: string }) => s.url),
  );

test("Composite II and Collections contracts fix their envelopes", async () => {
  const d = await doc();
  const c = d.components.schemas;
  assert.equal(c.CompositeRequest.properties.allOrNone.type, "boolean");
  assert.deepEqual(c.CompositeRequest.required, ["compositeRequest"]);
  assert.equal(c.CompositeRequest.properties.compositeRequest.minItems, 1);
  assert.equal(c.CompositeRequest.properties.compositeRequest.maxItems, 25);
  assert.equal(c.CompositeEmptyHeaders.maxProperties, 0);
  assert.equal(c.CompositeEmptyHeaders.additionalProperties, false);
  for (const branch of c.CompositeSubresponse.oneOf)
    assert.equal(
      branch.properties.httpHeaders.$ref,
      "#/components/schemas/CompositeEmptyHeaders",
    );
  assert.equal(c.CompositeReferenceID.maxLength, 80);
  assert.equal(c.CollectionsWrite.properties.records.maxItems, 200);
  assert.equal(c.CollectionsRetrieveRequest.properties.ids.maxItems, 2000);
  assert.deepEqual(c.CollectionsError.required, [
    "statusCode",
    "message",
    "fields",
  ]);
  assert.equal(
    c.Resources.properties.composite.const,
    "/services/data/v65.0/composite",
  );
  const definitions = JSON.parse(
    JSON.stringify(c).replaceAll("#/components/schemas/", "#/$defs/"),
  );
  const check = (name: string, data: unknown) =>
    new Ajv2020({ strict: true }).compile({
      $defs: definitions,
      $ref: "#/$defs/" + name,
    })(data);
  const failed = {
    success: false,
    errors: [{ statusCode: "INVALID_FIELD", message: "m", fields: [] }],
  };
  for (const [schema, good, bad] of [
    [
      "CollectionsSaveResults",
      [{ id: "001", success: true, errors: [], created: true }, failed],
      [{ success: false, errors: [] }],
    ],
    [
      "CollectionsWrite",
      {
        allOrNone: true,
        records: [{ attributes: { type: "Account" }, Name: "A" }],
      },
      { records: [{ Name: "no attributes" }] },
    ],
    [
      "CollectionsRetrieveResults",
      [null, { attributes: { type: "Account" }, Id: "001", Name: "A" }],
      [{ Name: "no attributes" }],
    ],
    [
      "CompositeSubresponse",
      { body: null, httpHeaders: {}, httpStatusCode: 204, referenceId: "a" },
      {
        body: null,
        httpHeaders: { Location: "/x" },
        httpStatusCode: 201,
        referenceId: "a",
      },
    ],
  ] as const) {
    assert.equal(check(schema, good), true, schema);
    assert.equal(check(schema, bad), false, schema);
  }
  const mutated = structuredClone(d);
  mutated.components.schemas.AccountRead = {
    type: "object",
    properties: {},
    additionalProperties: false,
    maxProperties: 0,
  };
  await assert.rejects(validateCandidateOpenAPI(mutated, await urls()));
});
