import assert from "node:assert/strict";
import test from "node:test";
import { mkdtemp, rm, readFile, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { parseExternalIdConfig } from "../src/external-id-contract.ts";
import { profile } from "../src/validate.ts";
import {
  normalizationManifest,
  normalizationScope,
} from "../src/external-id-normalization-contract.ts";
import {
  initializeNormalizationRun,
  appendNormalizationEvent,
  recoverNormalizationRun,
  readNormalizationRun,
  rememberNormalizationRecord,
  normalizationArtifact,
} from "../src/external-id-normalization-recovery.ts";
import { normalizationFixture } from "./external-id-normalization-test-server.ts";
const run = "a".repeat(24);
test("unknown dispatched outcome remains blocked after empty reads and a later commit", async (t) => {
  const x = await normalizationFixture(t);
  await initializeNormalizationRun(x.config, x.dir, run);
  const c = normalizationManifest[0],
    id = c.id + ":seed";
  await appendNormalizationEvent(x.config, x.dir, { id, kind: "intent" });
  await appendNormalizationEvent(x.config, x.dir, { id, kind: "dispatched" });
  await appendNormalizationEvent(x.config, x.dir, { id, kind: "unknown" });
  const first = await recoverNormalizationRun(x.config, {}, x.dir, x.transport);
  assert.equal(first.outcome, "blocked");
  assert.deepEqual(first.unresolvedOperationIds, [id]);
  const scope = normalizationScope(run, c);
  x.rows.set("001000000000000001", {
    Id: "001000000000000001",
    Name: scope.names.seed,
    Compat_Key__c: "unexpected spelling",
    Compat_Text__c: null,
  });
  const second = await recoverNormalizationRun(
    x.config,
    {},
    x.dir,
    x.transport,
  );
  assert.equal(second.outcome, "blocked");
  assert.deepEqual(second.unresolvedOperationIds, [id]);
  assert.equal(x.rows.size, 0);
});
test("recovery retains normalized keys but refuses an unjournaled Name", async (t) => {
  for (const foreign of [false, true]) {
    const x = await normalizationFixture(t);
    await initializeNormalizationRun(x.config, x.dir, run);
    const c = normalizationManifest[0],
      id = c.id + ":seed";
    for (const kind of ["intent", "dispatched", "response"] as const)
      await appendNormalizationEvent(x.config, x.dir, { id, kind });
    await normalizationArtifact(
      x.dir,
      "response-" + id.replace(":", "-") + ".json",
      {
        status: 201,
        headers: {},
        body: { id: "001000000000000001", success: true },
      },
    );
    const scope = normalizationScope(run, c);
    x.rows.set("001000000000000001", {
      Id: "001000000000000001",
      Name: foreign ? "unowned" : scope.names.seed,
      Compat_Key__c: "unexpected normalized key",
      Compat_Text__c: null,
    });
    await rememberNormalizationRecord(
      x.config,
      x.dir,
      c.id,
      "001000000000000001",
    );
    const r = await recoverNormalizationRun(x.config, {}, x.dir, x.transport);
    assert.equal(r.outcome, foreign ? "blocked" : "clean");
    assert.equal(x.rows.size, foreign ? 1 : 0);
  }
});
test("identity mismatch latches, malformed discovery and deletion failure block cleanup", async (t) => {
  for (const flag of ["wrongIdentity", "malformed", "deleteFailure"] as const) {
    const x = await normalizationFixture(t);
    await initializeNormalizationRun(x.config, x.dir, run);
    const c = normalizationManifest[0],
      id = c.id + ":seed";
    for (const kind of ["intent", "dispatched", "response"] as const)
      await appendNormalizationEvent(x.config, x.dir, { id, kind });
    await normalizationArtifact(
      x.dir,
      "response-" + id.replace(":", "-") + ".json",
      {
        status: 201,
        headers: {},
        body: { id: "001000000000000001", success: true },
      },
    );
    x.rows.set("001000000000000001", {
      Id: "001000000000000001",
      Name: normalizationScope(run, c).names.seed,
      Compat_Key__c: "key",
      Compat_Text__c: null,
    });
    x.flags[flag] = true;
    let r = await recoverNormalizationRun(x.config, {}, x.dir, x.transport);
    assert.equal(r.outcome, "blocked");
    if (flag === "wrongIdentity") {
      x.flags.wrongIdentity = false;
      r = await recoverNormalizationRun(x.config, {}, x.dir, x.transport);
      assert.ok(r.reasons.includes("IDENTITY_LATCHED"));
      assert.ok(!x.calls.some((c) => c.method === "DELETE"));
    }
  }
});
test("journal transitions, binding corruption and cross-case IDs fail closed", async (t) => {
  const x = await normalizationFixture(t);
  await initializeNormalizationRun(x.config, x.dir, run);
  await assert.rejects(
    appendNormalizationEvent(x.config, x.dir, {
      id: normalizationManifest[0].id + ":seed",
      kind: "response",
    }),
  );
  await rememberNormalizationRecord(
    x.config,
    x.dir,
    normalizationManifest[0].id,
    "001000000000000001",
  );
  await assert.rejects(
    rememberNormalizationRecord(
      x.config,
      x.dir,
      normalizationManifest[1].id,
      "001000000000000001",
    ),
  );
  const path = join(x.dir, "binding.json"),
    b = JSON.parse(await readFile(path, "utf8"));
  b.probeHash = "f".repeat(64);
  await writeFile(path, JSON.stringify(b));
  await assert.rejects(readNormalizationRun(x.config, x.dir));
});
