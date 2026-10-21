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
