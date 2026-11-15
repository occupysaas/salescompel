import assert from "node:assert/strict";
import test from "node:test";
import { loadCandidateOpenAPI } from "../src/candidate-openapi-contract.ts";
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
