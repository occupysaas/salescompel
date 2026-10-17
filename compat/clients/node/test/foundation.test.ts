import test from "node:test";
import assert from "node:assert/strict";
import {
  foundationScenarios,
  recordWireObservation,
} from "../src/foundation.ts";
import { normalize } from "../src/core.ts";
import type { Context } from "../src/scenarios.ts";

test("wire observations retain status, field presence, ordering and locator route shape without copied messages", () => {
  const c = { result: { identities: {}, observations: {} } } as Context;
  recordWireObservation(c, "page", {
    status: 200,
    headers: {},
    body: {
      totalSize: 2,
      done: false,
      records: [{ Id: "a" }, { Id: "b" }],
      nextRecordsUrl: "/services/data/v65.0/query/opaque-200",
    },
  });
  recordWireObservation(c, "replay", {
    status: 400,
    headers: {},
    body: [
      {
        errorCode: "INVALID_FIELD",
        message: "private reference prose",
        fields: [],
      },
    ],
  });
  const out = JSON.stringify(c.result.observations);
  assert.ok(!out.includes("private reference prose"));
  assert.match(out, /INVALID_FIELD/);
  assert.match(out, /messageType/);
  const normalized: any = normalize(c.result.observations, c.result.identities);
  assert.deepEqual(normalized.page.body.records, [{ Id: "a" }, { Id: "b" }]);
  assert.equal(normalized.page.status, 200);
  assert.equal(
    normalized.page.body.nextRecordsUrl.route,
    "/services/data/v65.0/query/{queryLocator}",
  );
});
test("unrecognized continuation paths cannot be normalized into route parity", () => {
  const c = { result: { identities: {}, observations: {} } } as Context;
  assert.throws(
    () =>
      recordWireObservation(c, "page", {
        status: 200,
        headers: {},
        body: { nextRecordsUrl: "/cursor/custom" },
      }),
    /route/i,
  );
});
test("all focused scenarios have executable handlers", () => {
  for (const id of [
    "foundation-routes",
    "foundation-pagination",
    "foundation-cursors",
    "foundation-ids",
    "foundation-names",
    "foundation-text-null",
    "foundation-errors",
  ])
    assert.equal(typeof foundationScenarios[id], "function");
});

test("a failed second fixture create leaves the first journaled and cleanable", async (t) => {
  const { createServer } = await import("node:http");
  const { mkdtemp, readFile, rm } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const { Connection } = await import("jsforce");
  const { FixtureTracker } = await import("../src/fixtures.ts");
  const dir = await mkdtemp(join(tmpdir(), "foundation-cleanup-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  let creates = 0;
  const deleted: string[] = [];
  const server = createServer((req, res) => {
    res.setHeader("content-type", "application/json");
    if (req.method === "POST") {
      creates++;
      if (creates === 1) {
        res.writeHead(201);
        res.end(
          JSON.stringify({
            id: "001000000000001AAA",
            success: true,
            errors: [],
          }),
        );
      } else {
        res.writeHead(400);
        res.end(
          JSON.stringify([
            { errorCode: "INVALID_FIELD", message: "synthetic failure" },
          ]),
        );
      }
    } else if (req.method === "DELETE") {
      deleted.push(req.url!);
      res.writeHead(204);
      res.end();
    } else {
      res.writeHead(500);
      res.end("{}");
    }
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  t.after(() => server.close());
  const instanceUrl = `http://127.0.0.1:${(server.address() as any).port}`;
  const tracker = new FixtureTracker(join(dir, "journal.jsonl"));
  const c = {
    conn: new Connection({
      instanceUrl,
      accessToken: "synthetic",
      version: "65.0",
    }),
    tracker,
    env: {
      COMPAT_API_VERSION: "65.0",
      COMPAT_CANDIDATE_URL: instanceUrl,
      COMPAT_CANDIDATE_TOKEN: "synthetic",
    },
    prefix: "COMPAT_CANDIDATE",
    runId: "offline-test",
    result: {
      id: "foundation-pagination",
      client: "wire",
      outcome: "not_run",
      assertions: [],
      observations: {},
      identities: {},
    },
    cleanup: [],
  } as Context;
  await assert.rejects(foundationScenarios["foundation-pagination"](c));
  assert.equal(creates, 2);
  const journal = await readFile(join(dir, "journal.jsonl"), "utf8");
  assert.match(journal, /001000000000001AAA/);
  assert.equal((await tracker.cleanup())[0].outcome, "passed");
  assert.deepEqual(deleted, [
    "/services/data/v65.0/sobjects/Account/001000000000001AAA",
  ]);
});
