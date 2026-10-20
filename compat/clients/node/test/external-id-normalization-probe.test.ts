import assert from "node:assert/strict";
import test from "node:test";
import { runNormalizationProbe } from "../src/external-id-normalization-probe.ts";
import { normalizationFixture } from "./external-id-normalization-test-server.ts";
test("probe executes all 80 sequential operations with POST seeds and local state evidence", async (t) => {
  const x = await normalizationFixture(t),
    r = await runNormalizationProbe(x.config, {}, x.dir, x.transport);
  assert.equal(r.collection, "complete", JSON.stringify(r.reasons));
  assert.equal(r.cleanup.outcome, "clean");
  assert.equal(r.observations.length, 80);
  assert.equal(x.rows.size, 0);
  assert.equal(
    x.calls.filter((c) => ["POST", "PATCH"].includes(c.method)).length,
    80,
  );
  for (const o of r.observations) {
    assert.equal(o.stateVerified, true);
    assert.ok(o.sha256);
    assert.ok(o.stateSha256);
    if (o.stepId.startsWith("seed")) assert.equal(o.request.method, "POST");
    if (o.request.path.includes("/Compat_Key__c/"))
      assert.equal(Object.hasOwn(o.request.body, "Compat_Key__c"), false);
  }
  const lookup = r.observations.find(
    (o) => o.caseId === "leading-space-reverse-lookup" && o.stepId === "seed",
  )!;
  assert.ok(lookup.request.key.startsWith(" "));
  assert.ok(!lookup.after[0].Compat_Key__c!.startsWith(" "));
  const other = r.observations.find(
    (o) => o.caseId === "ascii-other-update" && o.stepId === "request",
  )!;
  assert.equal(other.status, 400);
  assert.deepEqual(other.before, other.after);
  const same = r.observations.find(
    (o) => o.caseId === "ascii-same-update" && o.stepId === "request",
  )!;
  assert.equal(same.status, 204);
  assert.ok(same.after[0].Compat_Key__c!.endsWith("abc"));
});
test("unexpected normalized spelling and hidden rejected writes are captured, never repaired", async (t) => {
  const x = await normalizationFixture(t);
  x.flags.rejectedMutation = true;
  const r = await runNormalizationProbe(x.config, {}, x.dir, x.transport);
  assert.equal(r.collection, "complete");
  assert.equal(r.cleanup.outcome, "clean");
  const o = r.observations.find(
    (o) => o.caseId === "ascii-other-update" && o.stepId === "request",
  )!;
  assert.equal(o.status, 400);
  assert.notDeepEqual(o.before, o.after);
});
test("failed seeds and metadata/admin gates stop dependent writes", async (t) => {
  for (const flag of [
    "seedFailure",
    "metadataBad",
    "administratorBad",
    "wrongIdentity",
  ] as const) {
    const x = await normalizationFixture(t);
    x.flags[flag] = true;
    const r = await runNormalizationProbe(x.config, {}, x.dir, x.transport);
    assert.notEqual(r.collection, "complete");
    assert.equal(
      x.calls.filter((c) => ["POST", "PATCH"].includes(c.method)).length,
      flag === "seedFailure" ? 1 : 0,
    );
  }
});
test("response loss and missing created IDs remain blocked with retained raw responses", async (t) => {
  for (const flag of ["lossAt", "missingCreatedId"] as const) {
    const x = await normalizationFixture(t);
    if (flag === "lossAt") x.flags.lossAt = 1;
    else x.flags.missingCreatedId = true;
    const r = await runNormalizationProbe(x.config, {}, x.dir, x.transport);
    assert.notEqual(r.collection, "complete");
    assert.equal(r.cleanup.outcome, "blocked");
    if (flag === "missingCreatedId") {
      assert.equal(r.observations[0].state, "observed");
      assert.equal(r.observations[0].status, 201);
    } else assert.ok(r.cleanup.unresolvedOperationIds.length);
  }
});
