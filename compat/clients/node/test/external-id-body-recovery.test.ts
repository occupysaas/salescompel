import test from "node:test";
import assert from "node:assert/strict";
import { appendFile, chmod } from "node:fs/promises";
import { join } from "node:path";
import {
  initializeBodyRun,
  readBodyRun,
  recoverBodyRun,
  appendBodyEvent,
  rememberBodyRecord,
} from "../src/external-id-body-recovery.ts";
import { bodyScope, bodyManifest } from "../src/external-id-body-contract.ts";
import { bodyFixture } from "./external-id-body-test-server.ts";
const runId = "a".repeat(24);
async function started(t: Parameters<typeof bodyFixture>[0]) {
  const x = await bodyFixture(t);
  await initializeBodyRun(x.config, x.dir, runId);
  await appendBodyEvent(x.config, x.dir, {
    id: "missing-omitted:request",
    kind: "intent",
  });
  await appendBodyEvent(x.config, x.dir, {
    id: "missing-omitted:request",
    kind: "dispatched",
  });
  return x;
}
test("unknown dispatch stays blocked", async (t) => {
  const x = await started(t);
  await appendBodyEvent(x.config, x.dir, {
    id: "missing-omitted:request",
    kind: "unknown",
  });
  const r = await recoverBodyRun(x.config, {}, x.dir, x.transport);
  assert.equal(r.outcome, "blocked");
  assert.deepEqual(r.unresolvedOperationIds, ["missing-omitted:request"]);
});
test("identity mismatch latches", async (t) => {
  const x = await started(t);
  x.flags.wrongIdentity = true;
  assert.equal(
    (await recoverBodyRun(x.config, {}, x.dir, x.transport)).outcome,
    "blocked",
  );
  x.flags.wrongIdentity = false;
  const count = x.calls.length;
  assert.equal(
    (await recoverBodyRun(x.config, {}, x.dir, x.transport)).outcome,
    "blocked",
  );
  assert.equal(x.calls.length, count);
});
test("cleanup checks every exact ID", async (t) => {
  const x = await started(t),
    scope = bodyScope(runId, bodyManifest[8]),
    id = "001000000000000001";
  await rememberBodyRecord(x.config, x.dir, "missing-omitted", id);
  x.rows.set(id, {
    Id: id,
    Name: scope.requestName,
    Compat_Key__c: scope.pathKey,
    Compat_Text__c: null,
  });
  x.flags.hideQueries = true;
  const r = await recoverBodyRun(x.config, {}, x.dir, x.transport);
  assert.equal(x.rows.size, 0);
  assert.equal(r.records[0].id, id);
  assert.ok(r.records[0].absenceSha256);
  assert.equal(
    x.calls.filter((c) => c.path.endsWith(id) && c.method === "GET").length,
    2,
  );
});
test("bounds and ownership fail closed", async (t) => {
  const x = await started(t);
  await assert.rejects(
    appendBodyEvent(x.config, x.dir, { id: "foreign:request", kind: "intent" }),
  );
  await appendFile(join(x.dir, "journal.jsonl"), "{");
  await assert.rejects(readBodyRun(x.config, x.dir));
  const y = await started(t),
    s = bodyScope(runId, bodyManifest[8]),
    id = "001000000000000001";
  y.rows.set(id, {
    Id: id,
    Name: "foreign",
    Compat_Key__c: s.pathKey,
    Compat_Text__c: null,
  });
  assert.equal(
    (await recoverBodyRun(y.config, {}, y.dir, y.transport)).outcome,
    "blocked",
  );
  assert.ok(!y.calls.some((c) => c.method === "DELETE"));
  await chmod(join(y.dir, "binding.json"), 0o644);
  await assert.rejects(readBodyRun(y.config, y.dir));
});
test("malformed state and deletion failure block cleanup", async (t) => {
  for (const flag of ["malformed", "deleteFailure"] as const) {
    const x = await started(t),
      s = bodyScope(runId, bodyManifest[8]),
      id = "001000000000000001";
    x.rows.set(id, {
      Id: id,
      Name: s.requestName,
      Compat_Key__c: s.pathKey,
      Compat_Text__c: null,
    });
    x.flags[flag] = true;
    const r = await recoverBodyRun(x.config, {}, x.dir, x.transport);
    assert.equal(r.outcome, "blocked");
    assert.equal(x.rows.size, 1);
  }
});
test("unexpected coercions retain IDs and never authorize deletion", async (t) => {
  for (const key of [42, { value: "invalid" }]) {
    const x = await started(t),
      s = bodyScope(runId, bodyManifest[8]),
      id = "001000000000000001";
    x.rows.set(id, {
      Id: id,
      Name: s.requestName,
      Compat_Key__c: key as any,
      Compat_Text__c: null,
    });
    const r = await recoverBodyRun(x.config, {}, x.dir, x.transport);
    assert.equal(r.outcome, "blocked");
    assert.equal(r.records[0].id, id);
    assert.ok(r.records[0].absenceSha256);
    assert.equal(x.rows.size, 1);
    assert.ok(!x.calls.some((c) => c.method === "DELETE"));
  }
});
test("returned IDs remain known after losing discovery ownership", async (t) => {
  const x = await started(t),
    id = "001000000000000001";
  await import("../src/external-id-recovery.ts").then((m) =>
    m.durableJSON(join(x.dir, "response-missing-omitted-request.json"), {
      status: 201,
      body: { id, created: true },
      headers: {},
    }),
  );
  await appendBodyEvent(x.config, x.dir, {
    id: "missing-omitted:request",
    kind: "response",
  });
  x.rows.set(id, {
    Id: id,
    Name: "changed-name",
    Compat_Key__c: "changed-key",
    Compat_Text__c: null,
  });
  const r = await recoverBodyRun(x.config, {}, x.dir, x.transport);
  assert.equal(r.outcome, "blocked");
  assert.equal(r.records[0].id, id);
  assert.ok(r.records[0].absenceSha256);
  assert.ok(!x.calls.some((c) => c.method === "DELETE"));
});
test("invalid private directory blocks without writing artifacts", async (t) => {
  const x = await bodyFixture(t);
  await chmod(x.dir, 0o755);
  const r = await recoverBodyRun(x.config, {}, x.dir, x.transport);
  assert.equal(r.outcome, "blocked");
  const { readdir } = await import("node:fs/promises");
  assert.deepEqual(await readdir(x.dir), []);
  assert.equal(x.calls.length, 0);
});
test("owned null and empty keys are cleaned by exact ID", async (t) => {
  for (const key of [null, ""]) {
    const x = await started(t),
      s = bodyScope(runId, bodyManifest[8]),
      id = "001000000000000001";
    x.rows.set(id, {
      Id: id,
      Name: s.requestName,
      Compat_Key__c: key,
      Compat_Text__c: null,
    });
    const r = await recoverBodyRun(x.config, {}, x.dir, x.transport);
    assert.equal(x.rows.size, 0);
    assert.equal(r.records[0].id, id);
    assert.ok(r.records[0].absenceSha256);
  }
});
test("fixture ceiling and configuration rebinding fail closed", async (t) => {
  const x = await started(t);
  await assert.rejects(
    readBodyRun(
      { ...x.config, origin: "https://other.example.invalid" },
      x.dir,
    ),
  );
  for (let i = 1; i <= 80; i++)
    await rememberBodyRecord(
      x.config,
      x.dir,
      "missing-omitted",
      "001" + String(i).padStart(15, "0"),
    );
  await assert.rejects(
    rememberBodyRecord(
      x.config,
      x.dir,
      "missing-omitted",
      "001" + String(81).padStart(15, "0"),
    ),
    /FIXTURE_BOUND/,
  );
  const r = await recoverBodyRun(x.config, {}, x.dir, x.transport);
  assert.equal(r.outcome, "blocked");
  assert.ok(!x.calls.some((c) => c.method === "DELETE"));
});
