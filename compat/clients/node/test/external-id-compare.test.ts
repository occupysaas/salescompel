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
  absent.operations[0].body = null;
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

test("missing observed response evidence blocks stability and race winner swaps alone do not drift", async (t) => {
  const x = await localProbe(t),
    y = await localProbe(t);
  const a = await runExternalIdProbe(x.config, {}, x.dir, x.transport),
    b = await runExternalIdProbe(y.config, {}, y.dir, y.transport);
  const missing = structuredClone(b);
  for (const op of missing.operations) {
    op.status = null;
    op.artifact = "";
    op.sha256 = "";
  }
  assert.equal(compareExternalIdReports(a, missing).outcome, "blocked");
  const swapped = structuredClone(b);
  const ops = swapped.operations.filter((o) => o.caseId === "race-variant-0");
  [ops[0].body, ops[1].body] = [ops[1].body, ops[0].body];
  [ops[0].status, ops[1].status] = [ops[1].status, ops[0].status];
  [ops[0].bodyPresent, ops[1].bodyPresent] = [
    ops[1].bodyPresent,
    ops[0].bodyPresent,
  ];
  assert.equal(compareExternalIdReports(a, swapped).outcome, "stable");
});

import { validateExternalIdEvidence } from "../src/external-id-compare.ts";
import { writeFile, unlink } from "node:fs/promises";
import { join } from "node:path";
test("private evidence boundary rejects deleted or corrupted artifacts", async (t) => {
  const x = await localProbe(t);
  const r = await runExternalIdProbe(x.config, {}, x.dir, x.transport);
  await validateExternalIdEvidence(r, x.dir);
  const path = join(x.dir, r.operations[0].artifact);
  await writeFile(path, "{}");
  await assert.rejects(validateExternalIdEvidence(r, x.dir));
  await unlink(path);
  await assert.rejects(validateExternalIdEvidence(r, x.dir));
});
