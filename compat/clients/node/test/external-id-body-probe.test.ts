import test from "node:test";
import assert from "node:assert/strict";
import { runBodyProbe } from "../src/external-id-body-probe.ts";
import { validateBodyReport } from "../src/external-id-body-contract.ts";
import { bodyFixture } from "./external-id-body-test-server.ts";
test("collects all cases without coercion", async (t) => {
  const x = await bodyFixture(t),
    r = await runBodyProbe(x.config, {}, x.dir, x.transport);
  assert.equal(r.collection, "complete");
  assert.equal(r.observations.length, 16);
  assert.equal(
    x.calls.filter((c) => ["POST", "PATCH"].includes(c.method)).length,
    24,
  );
  assert.equal(x.maxConcurrency, 1);
  assert.equal(x.rows.size, 0);
  assert.equal(r.cleanup.outcome, "clean");
  validateBodyReport(r);
  for (const o of r.observations) {
    if (o.caseId.endsWith("omitted")) {
      assert.ok([201, 204].includes(o.status!));
      assert.equal(Object.hasOwn(o.request, "Compat_Key__c"), false);
    } else {
      assert.equal(o.status, 400);
      assert.deepEqual(o.after, o.before);
    }
  }
  assert.equal(
    r.observations.find((o) => o.caseId === "missing-number")!.request
      .Compat_Key__c,
    42,
  );
  assert.deepEqual(
    r.observations.find((o) => o.caseId === "missing-object")!.request
      .Compat_Key__c,
    { value: "invalid" },
  );
});
test("records hidden insert and rejected Name mutation", async (t) => {
  for (const flag of ["hiddenInsert", "mutateRejected"] as const) {
    const x = await bodyFixture(t);
    x.flags[flag] = true;
    const r = await runBodyProbe(x.config, {}, x.dir, x.transport),
      o = r.observations.find((o) => o.caseId === "missing-identical")!;
    assert.equal(o.status, 400);
    if (flag === "hiddenInsert") assert.equal(o.after.length, 1);
    else assert.notDeepEqual(r.observations[1].before, r.observations[1].after);
    assert.equal(r.cleanup.outcome, "clean");
  }
});
test("retains different-key fixtures", async (t) => {
  const x = await bodyFixture(t);
  x.flags.different = true;
  const r = await runBodyProbe(x.config, {}, x.dir, x.transport),
    o = r.observations.find((o) => o.caseId === "missing-different")!;
  assert.equal(o.after[0].Compat_Key__c, o.request.Compat_Key__c);
  assert.ok(r.cleanup.records.some((r) => r.id === o.after[0].Id));
  assert.equal(r.cleanup.outcome, "clean");
});
test("unavailable response is not a rejection", async (t) => {
  const x = await bodyFixture(t);
  x.flags.loss = true;
  const r = await runBodyProbe(x.config, {}, x.dir, x.transport);
  assert.equal(r.cleanup.outcome, "blocked");
  assert.notEqual(r.collection, "complete");
  assert.ok(r.cleanup.unresolvedOperationIds.length);
});
test("metadata and elapsed deadline prevent collection writes", async (t) => {
  const x = await bodyFixture(t);
  x.flags.metadataBad = true;
  const r = await runBodyProbe(x.config, {}, x.dir, x.transport);
  assert.equal(r.collection, "incomplete");
  assert.ok(r.reasons.includes("FIELD_METADATA_MISMATCH"));
  assert.ok(!x.calls.some((c) => c.method === "POST"));
  const y = await bodyFixture(t),
    original = Date.now;
  let n = 0;
  Date.now = () => original() + (++n >= 2 ? 600001 : 0);
  try {
    const r = await runBodyProbe(y.config, {}, y.dir, y.transport);
    assert.notEqual(r.collection, "complete");
    assert.ok(!y.calls.some((c) => c.method === "POST"));
  } finally {
    Date.now = original;
  }
});
test("a received response stays observed when its created ID is missing", async (t) => {
  const x = await bodyFixture(t);
  x.flags.missingId = true;
  const r = await runBodyProbe(x.config, {}, x.dir, x.transport),
    o = r.observations.find((o) => o.caseId === "missing-omitted")!;
  assert.equal(o.state, "observed");
  assert.equal(o.status, 201);
  assert.ok(o.sha256);
  assert.equal(r.cleanup.outcome, "blocked");
});
test("cleanup proof binds final discovery to the report", async (t) => {
  const x = await bodyFixture(t),
    r = await runBodyProbe(x.config, {}, x.dir, x.transport);
  const { bodyDigest } = await import("../src/external-id-body-recovery.ts"),
    { privateJSON } = await import("../src/external-id-recovery.ts");
  const proof = await privateJSON(
    (await import("node:path")).join(
      x.dir,
      `cleanup-proof-${bodyDigest(JSON.stringify(r.cleanup))}.json`,
    ),
  );
  assert.deepEqual(proof.cleanup, r.cleanup);
  assert.equal(proof.discoveryArtifacts.length, 16);
  for (const a of proof.discoveryArtifacts) {
    assert.match(a.sha256, /^[a-f0-9]{64}$/);
    const captured = await privateJSON(
      (await import("node:path")).join(x.dir, a.artifact),
    );
    assert.equal(captured.caseId, a.caseId);
    assert.equal(captured.response.body.records.length, 0);
  }
});
test("administrator preflight and identity mismatch prevent writes", async (t) => {
  for (const flag of ["administratorBad", "wrongIdentity"] as const) {
    const x = await bodyFixture(t);
    x.flags[flag] = true;
    const r = await runBodyProbe(x.config, {}, x.dir, x.transport);
    assert.notEqual(r.collection, "complete");
    assert.ok(
      !x.calls.some((c) => ["POST", "PATCH", "DELETE"].includes(c.method)),
    );
    if (flag === "wrongIdentity") assert.equal(r.collection, "blocked");
  }
});
