// Synthetic connection stubs test the parent's fixture bookkeeping only.
import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { FixtureTracker } from "../src/fixtures.ts";
import { adoptPythonFixtures, pythonProbe } from "../src/process-clients.ts";
import { Blocked, type Context } from "../src/scenarios.ts";

function context(dir: string, destroy: (id: string) => Promise<unknown>) {
  return {
    conn: {
      instanceUrl: "https://example.invalid",
      sobject: () => ({ destroy }),
    } as any,
    env: {},
    prefix: "SYNTHETIC",
    runId: "test-run",
    tracker: new FixtureTracker(join(dir, "journal")),
    cleanup: [],
    result: {
      id: "python-crud",
      client: "simple-salesforce",
      outcome: "not_run",
      assertions: [],
      observations: {},
      identities: {},
    },
  } satisfies Context;
}

test("python fixtures: confirmed deletes pass, failed ones are retried once", async () => {
  const dir = await mkdtemp(join(tmpdir(), "sc-py-"));
  try {
    const deleted: string[] = [];
    const c = context(dir, async (id) => {
      deleted.push(id);
      return { success: true };
    });
    await adoptPythonFixtures(c, join(dir, "journal"), [
      { resource: "Account:001000000000001AAA", outcome: "passed" },
      { resource: "Account:001000000000002AAA", outcome: "failed" },
    ]);
    assert.deepEqual(c.cleanup, [
      {
        resource: "Account:001000000000001AAA",
        outcome: "passed",
        scenario: "python-crud",
      },
    ]);
    const retried = await c.tracker.cleanup();
    assert.deepEqual(deleted, ["001000000000002AAA"]);
    assert.deepEqual(
      retried.map((r) => r.outcome),
      ["passed"],
    );
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});

test("python fixtures: a crashed probe's journaled IDs are adopted for cleanup", async () => {
  const dir = await mkdtemp(join(tmpdir(), "sc-py-"));
  try {
    const journal = join(dir, "journal");
    await writeFile(
      journal,
      [
        JSON.stringify({ intent: "create", type: "Account", fields: {} }),
        JSON.stringify({ resource: "Account:001000000000003AAA" }),
        "partial line",
      ].join("\n"),
    );
    const deleted: string[] = [];
    const c = context(dir, async (id) => {
      deleted.push(id);
      const e: any = new Error("gone");
      e.errorCode = "ENTITY_IS_DELETED";
      throw e;
    });
    await adoptPythonFixtures(c, journal, []);
    assert.deepEqual(c.cleanup, []);
    const results = await c.tracker.cleanup();
    assert.deepEqual(deleted, ["001000000000003AAA"]);
    assert.equal(results[0].outcome, "passed");
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});

test("python probe blocks on HTTP endpoints instead of failing", async () => {
  const dir = await mkdtemp(join(tmpdir(), "sc-py-"));
  try {
    const c = context(dir, async () => ({ success: true }));
    c.conn.instanceUrl = "http://localhost:8080";
    await assert.rejects(pythonProbe(c, join(dir, "journal")), Blocked);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});
