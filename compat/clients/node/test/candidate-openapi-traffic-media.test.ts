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

const valid = (r: { requestIssues: string[]; responseIssues: string[] }) => {
  assert.deepEqual(r.requestIssues, []);
  assert.deepEqual(r.responseIssues, []);
};

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
    const d = (await loadCandidateOpenAPI()) as Record<string, any>;
    d.components.schemas.AccountRead.additionalProperties =
      additionalProperties;
    await assert.rejects(
      createCandidateTrafficValidator(d),
      /^Error: candidate-openapi-spec$/,
    );
  }
});
