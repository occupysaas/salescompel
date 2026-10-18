import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm, appendFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  initializeRun,
  appendJournal,
  readRun,
} from "../src/external-id-recovery.ts";
import { parseExternalIdConfig } from "../src/external-id-contract.ts";
import { profile } from "../src/validate.ts";
export async function testConfig() {
  return parseExternalIdConfig({
    schemaVersion: 1,
    apiVersion: "65.0",
    profileHash: (await profile("rest-query-foundation")).hash,
    origin: "https://reference.example.invalid",
    administrator: { user: "u", org: "o" },
  });
}
test("durable intent/dispatched survives interrupted report and rejects truncated journal", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p9-journal-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const c = await testConfig();
  await initializeRun(c, dir, "a".repeat(24));
  await appendJournal(dir, { kind: "intent", id: "exact:first" });
  await appendJournal(dir, { kind: "dispatched", id: "exact:first" });
  assert.equal((await readRun(c, dir)).events.length, 2);
  await appendFile(join(dir, "journal.jsonl"), "{");
  await assert.rejects(readRun(c, dir));
});

import { localProbe } from "./external-id-test-server.ts";
import { recoverExternalIdRun, base } from "../src/external-id-recovery.ts";
import { caseScope, manifest } from "../src/external-id-contract.ts";
test("late commit after timeout remains blocked before and after recovery removes owned row", async (t) => {
  const x = await localProbe(t);
  const runId = "b".repeat(24);
  await initializeRun(x.config, x.dir, runId);
  const scope = caseScope(runId, manifest[0]);
  await appendJournal(x.dir, { kind: "intent", id: "exact:first" });
  await appendJournal(x.dir, { kind: "dispatched", id: "exact:first" });
  let release!: () => void;
  let arrived!: () => void;
  x.flags.beforeCommit = new Promise<void>((resolve) => {
    release = resolve;
  });
  const receiving = new Promise<void>((resolve) => {
    arrived = resolve;
  });
  x.flags.onMutation = arrived;
  const controller = new AbortController();
  const rejected = assert.rejects(
    x.transport.request(base + "sobjects/Account", {
      method: "POST",
      body: JSON.stringify({
        Name: scope.marker,
        Compat_Key__c: scope.keys[0],
      }),
      signal: controller.signal,
    }),
  );
  await receiving;
  controller.abort();
  await rejected;
  let r = await recoverExternalIdRun(x.config, {}, x.dir, x.transport);
  assert.equal(r.outcome, "blocked");
  assert.equal(x.rows.size, 0);
  release();
  for (let attempt = 0; attempt < 100 && x.rows.size === 0; attempt++)
    await new Promise((resolve) => setTimeout(resolve, 5));
  assert.equal(x.rows.size, 1);
  x.flags.delay = 0;
  r = await recoverExternalIdRun(x.config, {}, x.dir, x.transport);
  assert.equal(r.outcome, "blocked");
  assert.equal(r.deleted, 1);
  assert.equal(x.rows.size, 0);
  assert.equal(x.calls.filter((c) => c.method === "POST").length, 1);
});
test("unproven ownership and changed identity prevent deletion", async (t) => {
  const x = await localProbe(t);
  const runId = "c".repeat(24);
  await initializeRun(x.config, x.dir, runId);
  await appendJournal(x.dir, { kind: "intent", id: "exact:first" });
  await appendJournal(x.dir, { kind: "dispatched", id: "exact:first" });
  const marker = caseScope(runId, manifest[0]).marker;
  x.rows.set("001000000000000001", {
    Id: "001000000000000001",
    Name: marker,
    Compat_Key__c: "unowned",
    Compat_Text__c: null,
  });
  assert.equal(
    (await recoverExternalIdRun(x.config, {}, x.dir, x.transport)).outcome,
    "blocked",
  );
  x.flags.wrongIdentity = true;
  assert.equal(
    (await recoverExternalIdRun(x.config, {}, x.dir, x.transport)).outcome,
    "blocked",
  );
  assert.ok(!x.calls.some((c) => c.method === "DELETE"));
});

test("cleanup refusal remains blocked and config rebinding is rejected", async (t) => {
  const x = await localProbe(t);
  const runId = "d".repeat(24);
  await initializeRun(x.config, x.dir, runId);
  await appendJournal(x.dir, { kind: "intent", id: "exact:first" });
  await appendJournal(x.dir, { kind: "dispatched", id: "exact:first" });
  await appendJournal(x.dir, { kind: "response", id: "exact:first" });
  const scope = caseScope(runId, manifest[0]);
  x.rows.set("001000000000000001", {
    Id: "001000000000000001",
    Name: scope.marker,
    Compat_Key__c: scope.keys[0],
    Compat_Text__c: null,
  });
  x.flags.deleteFailure = true;
  const r = await recoverExternalIdRun(x.config, {}, x.dir, x.transport);
  assert.equal(r.outcome, "blocked");
  assert.equal(x.rows.size, 1);
  await assert.rejects(
    readRun(
      { ...x.config, origin: "https://different.example.invalid" },
      x.dir,
    ),
  );
});
