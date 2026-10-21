import test from "node:test";
import assert from "node:assert/strict";
import { parseCLIResult } from "../src/candidate-acceptance-clients.ts";
test("native CLI output cannot pass on exit zero with malformed or failed JSON", () => {
  for (const text of ["", "{}", '{"status":1,"result":{}}', '{"status":0}'])
    assert.throws(() => parseCLIResult(text));
  assert.deepEqual(parseCLIResult('{"status":0,"result":{"id":"owned"}}'), {
    id: "owned",
  });
});
test("restart baseline requires requested custom values, identity and relationship", async () => {
  const { assertRestartBaseline } =
    await import("../src/candidate-acceptance-clients.ts");
  const runId = "f".repeat(24),
    name = `p15-${runId}-restart`;
  const baseline = {
    account: "001000000000000001",
    contact: "003000000000000001",
    a: {
      Id: "001000000000000001",
      Name: name,
      Compat_Text__c: "retained text",
      Compat_Key__c: name,
    },
    b: {
      Id: "003000000000000001",
      FirstName: "Restart",
      LastName: name,
      AccountId: "001000000000000001",
    },
  };
  assertRestartBaseline(runId, baseline);
  for (const [key, field] of [
    ["a", "Compat_Text__c"],
    ["a", "Compat_Key__c"],
    ["a", "Id"],
    ["b", "AccountId"],
    ["b", "Id"],
  ]) {
    const bad = structuredClone(baseline) as any;
    bad[key][field] = null;
    assert.throws(() => assertRestartBaseline(runId, bad));
  }
});
