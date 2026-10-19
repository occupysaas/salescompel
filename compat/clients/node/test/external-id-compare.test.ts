import test from "node:test";
import assert from "node:assert/strict";
import { localProbe } from "./external-id-test-server.ts";
import { runExternalIdProbe } from "../src/external-id-probe.ts";
import { compareExternalIdReports } from "../src/external-id-compare.ts";
test("external-ID comparison preserves exact spelling, body presence and full coverage", async (t) => {
  const x = await localProbe(t),
    y = await localProbe(t);
  const a = await runExternalIdProbe(x.config, {}, x.dir, x.transport),
    b = await runExternalIdProbe(y.config, {}, y.dir, y.transport);
  assert.equal(compareExternalIdReports(a, b).outcome, "stable");
  const changed = structuredClone(b);
  changed.operations[0].key += " ";
  assert.equal(compareExternalIdReports(a, changed).outcome, "drift");
  const absent = structuredClone(b);
  absent.operations[0].bodyPresent = false;
  assert.equal(compareExternalIdReports(a, absent).outcome, "drift");
  const missing = structuredClone(b);
  missing.operations.pop();
  assert.equal(compareExternalIdReports(a, missing).outcome, "blocked");
  const dirty = structuredClone(b);
  dirty.cleanup = "blocked";
  assert.equal(compareExternalIdReports(a, dirty).outcome, "blocked");
  const race = structuredClone(b);
  const op = race.operations.find((o) => o.caseId.startsWith("race-"))!;
  op.records[0].Compat_Key__c += "changed";
  assert.notEqual(compareExternalIdReports(a, race).outcome, "stable");
});
