import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, writeFile, rm } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  recordFixtureEvent,
  cleanupFixtures,
} from "../src/candidate-acceptance-fixtures.ts";
test("fixture cleanup refuses unknown creates and foreign names while retaining IDs", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p15-service-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "journal");
  await writeFile(path, "", { mode: 0o600 });
  const event = {
    schemaVersion: 1 as const,
    runId: "a".repeat(24),
    operationId: "create-1",
    principal: "admin" as const,
    object: "Account" as const,
    names: ["owned"],
    kind: "intent" as const,
  };
  await recordFixtureEvent(path, event);
  const c = { runId: event.runId, fixtureJournal: path } as any;
  const pending = await cleanupFixtures(c, async () => ({
    status: 404,
    body: null,
  }));
  assert.equal(pending.verified, false);
  assert.equal(pending.unknownIntents, 1);
  await recordFixtureEvent(path, {
    ...event,
    kind: "known",
    id: "001000000000000001",
  });
  const calls: string[] = [];
  const result = await cleanupFixtures(c, async (_c, _role, _path, method) => {
    calls.push(method ?? "GET");
    return { status: 200, body: { Name: "foreign" } };
  });
  assert.equal(result.verified, false);
  assert.ok(!calls.includes("DELETE"));
});
test("fixture cleanup deletes children before parents and independently checks absence", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p15-order-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "journal");
  await writeFile(path, "", { mode: 0o600 });
  for (const [object, id] of [
    ["Account", "001000000000000001"],
    ["Contact", "003000000000000001"],
  ] as const) {
    const e = {
      schemaVersion: 1 as const,
      runId: "a".repeat(24),
      operationId: object,
      principal: "admin" as const,
      object,
      names: ["owned"],
    };
    await recordFixtureEvent(path, { ...e, kind: "intent" });
    await recordFixtureEvent(path, { ...e, kind: "known", id });
  }
  const deleted: string[] = [],
    gone = new Set<string>();
  const r = await cleanupFixtures(
    { runId: "a".repeat(24), fixtureJournal: path } as any,
    async (_c, _role, p, m) => {
      if (m === "DELETE") {
        deleted.push(p);
        gone.add(p);
        return { status: 204, body: null };
      }
      return gone.has(p)
        ? { status: 404, body: null }
        : { status: 200, body: { Name: "owned", LastName: "owned" } };
    },
  );
  assert.equal(r.verified, true);
  assert.ok(deleted[0].includes("Contact"));
});
