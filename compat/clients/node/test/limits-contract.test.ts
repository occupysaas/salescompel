import test from "node:test";
import assert from "node:assert/strict";
import {
  validateLimitsResponse,
  validateLimitInfo,
} from "../src/limits-contract.ts";

// These are synthetic examples of documented fields, not reference captures.
test("limits response check accepts documented fields and other limits", () => {
  assert.doesNotThrow(() =>
    validateLimitsResponse({
      status: 200,
      headers: {},
      body: {
        DailyApiRequests: { Max: 1000, Remaining: 500 },
        OtherLimit: { Max: 10, Remaining: 9 },
      },
    }),
  );
});

test("limits response check rejects missing, non-integer and negative counters", () => {
  for (const daily of [
    undefined,
    null,
    {},
    { Max: 1000 },
    { Max: -1, Remaining: 0 },
    { Max: 2, Remaining: 0.5 },
    { Max: "2", Remaining: 1 },
  ]) {
    assert.throws(() =>
      validateLimitsResponse({
        status: 200,
        headers: {},
        body: { DailyApiRequests: daily },
      }),
    );
  }
  assert.throws(() =>
    validateLimitsResponse({
      status: 403,
      headers: {},
      body: { DailyApiRequests: { Max: 1, Remaining: 0 } },
    }),
  );
});

test("limit info check accepts usage and additional documented entries", () => {
  for (const value of [
    "api-usage=100/1000",
    "api-usage=100/1000; api-bursts=1/750",
    "api-usage=1001/1000",
  ])
    assert.doesNotThrow(() => validateLimitInfo(value));
});

test("limit info check rejects missing or malformed API usage", () => {
  for (const value of [
    undefined,
    "",
    "api-bursts=1/750",
    "api-usage=-1/1000",
    "api-usage=1",
    "api-usage=1/3x",
  ])
    assert.throws(() => validateLimitInfo(value));
});

test("limits checks can consume the existing wire response", async (t) => {
  const { createServer } = await import("node:http");
  const { wire } = await import("../src/wire.ts");
  const server = createServer((req, res) => {
    assert.equal(req.url, "/services/data/v65.0/limits");
    res.writeHead(200, {
      "Content-Type": "application/json",
      "Sforce-Limit-Info": "api-usage=2/100; api-bursts=1/10",
    });
    res.end(JSON.stringify({ DailyApiRequests: { Max: 100, Remaining: 98 } }));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => server.close());
  const base = new URL(
    `http://127.0.0.1:${(server.address() as { port: number }).port}`,
  );
  const response = await wire(
    base,
    "synthetic",
    "/services/data/v65.0/limits",
    { method: "GET" },
  );
  assert.doesNotThrow(() => validateLimitsResponse(response));
  assert.doesNotThrow(() =>
    validateLimitInfo(response.headers["sforce-limit-info"]),
  );
});
