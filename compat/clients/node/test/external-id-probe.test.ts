import test from "node:test";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { createHash } from "node:crypto";
import assert from "node:assert/strict";
import { runExternalIdProbe } from "../src/external-id-probe.ts";
import {
  manifest,
  validateExternalIdReport,
} from "../src/external-id-contract.ts";
import { nativeUpsert } from "../src/external-id-native.ts";
import { localProbe } from "./external-id-test-server.ts";
test("fixed external-ID scenarios use real transport, preserve encoding and clean owned records", async (t) => {
  const x = await localProbe(t);
  const r = await runExternalIdProbe(x.config, {}, x.dir, x.transport);
  assert.equal(r.collection, "complete", JSON.stringify(r.reasons));
  assert.equal(r.cleanup, "clean");
  assert.equal(
    r.operations.length,
    manifest.reduce((n, c) => n + c.steps.length, 0),
  );
  assert.equal(x.rows.size, 0);
  for (const op of r.operations)
    assert.equal(
      createHash("sha256")
        .update(await readFile(join(x.dir, op.artifact)))
        .digest("hex"),
      op.sha256,
    );
  assert.ok(x.calls.some((c) => c.path.includes("%2Fpercent%25%C3%A9")));
  assert.ok(
    x.calls.filter((c) => ["POST", "PATCH"].includes(c.method)).length <= 160,
  );
  validateExternalIdReport(r);
});
test("identity failure prevents all mutations", async (t) => {
  const x = await localProbe(t);
  x.flags.wrongIdentity = true;
  const r = await runExternalIdProbe(x.config, {}, x.dir, x.transport);
  assert.notEqual(r.collection, "complete");
  assert.ok(x.calls.every((c) => c.method === "GET"));
});
test("pending query page is never treated as empty preflight", async (t) => {
  const x = await localProbe(t);
  x.flags.pagination = true;
  const r = await runExternalIdProbe(x.config, {}, x.dir, x.transport);
  assert.notEqual(r.collection, "complete");
  assert.ok(x.calls.every((c) => c.method === "GET"));
});
test("native deadline terminates isolated process without exposing credentials", async () => {
  await assert.rejects(
    nativeUpsert(
      "https://reference.example.invalid",
      "private-token",
      "key",
      "p9" + "a".repeat(24) + "-native",
      1,
    ),
    /NATIVE_TIMEOUT/,
  );
});

test("transient identity mismatch is latched and never followed by automatic deletion", async (t) => {
  const x = await localProbe(t);
  x.flags.identityMismatchAt = 3;
  const r = await runExternalIdProbe(x.config, {}, x.dir, x.transport);
  assert.equal(r.identityVerified, false);
  assert.equal(r.cleanup, "blocked");
  assert.ok(!x.calls.some((c) => c.method === "DELETE"));
  assert.ok(x.rows.size > 0);
});
