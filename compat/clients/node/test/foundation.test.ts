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

test("text equality probe produces a valid identity map even when case and normalization variants coincide", async (t) => {
  const { createServer } = await import("node:http");
  const { Connection } = await import("jsforce");
  const { FixtureTracker } = await import("../src/fixtures.ts");
  const { mkdtemp, rm } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const dir = await mkdtemp(join(tmpdir(), "foundation-identities-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  let count = 0;
  const server = createServer((req, res) => {
    res.setHeader("content-type", "application/json");
    if (req.method === "POST") {
      res.writeHead(201);
      res.end(
        JSON.stringify({
          id: `001${String(++count).padStart(12, "0")}AAA`,
          success: true,
          errors: [],
        }),
      );
    } else if (req.url!.includes("/query?"))
      res.end(JSON.stringify({ totalSize: 0, done: true, records: [] }));
    else
      res.end(
        JSON.stringify({ Id: "001000000000001AAA", Compat_Text__c: null }),
      );
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  t.after(() => server.close());
  const url = `http://127.0.0.1:${(server.address() as any).port}`;
  const c = {
    conn: new Connection({
      instanceUrl: url,
      accessToken: "synthetic",
      version: "65.0",
    }),
    tracker: new FixtureTracker(join(dir, "journal")),
    env: {
      COMPAT_API_VERSION: "65.0",
      COMPAT_CANDIDATE_URL: url,
      COMPAT_CANDIDATE_TOKEN: "synthetic",
    },
    prefix: "COMPAT_CANDIDATE",
    runId: "offline",
    result: {
      id: "foundation-text-null",
      client: "wire",
      outcome: "not_run",
      assertions: [],
      observations: {},
      identities: {},
    },
    cleanup: [],
  } as Context;
  await foundationScenarios["foundation-text-null"](c);
  const values = Object.values(c.result.identities);
  assert.equal(
    new Set(values).size,
    values.length,
    "identity normalization must be one-to-one",
  );
});

test("discovery link changes remain visible to comparison", () => {
  const a = { result: { identities: {}, observations: {} } } as Context;
  const b = { result: { identities: {}, observations: {} } } as Context;
  recordWireObservation(a, "resources", {
    status: 200,
    headers: {},
    body: {
      query: "/services/data/v65.0/query",
      sobjects: "/services/data/v65.0/sobjects",
    },
  });
  recordWireObservation(b, "resources", {
    status: 200,
    headers: {},
    body: { query: "/cursor", sobjects: "/custom" },
  });
  assert.notDeepEqual(a.result.observations, b.result.observations);
});

test("name probe records state after each potentially overwriting PATCH", async (t) => {
  const { createServer } = await import("node:http");
  const { Connection } = await import("jsforce");
  const { FixtureTracker } = await import("../src/fixtures.ts");
  const { mkdtemp, rm } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const dir = await mkdtemp(join(tmpdir(), "names-state-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  let value: any = null;
  const id = "001000000000001AAA";
  const server = createServer(async (req, res) => {
    res.setHeader("content-type", "application/json");
    const restricted = req.headers.authorization === "Bearer restricted";
    if (req.method === "POST") {
      res.writeHead(201);
      res.end(JSON.stringify({ id, success: true, errors: [] }));
      return;
    }
    if (
      restricted &&
      (req.method === "PATCH" ||
        decodeURIComponent(req.url!).includes("compat_text__c"))
    ) {
      res.writeHead(400);
      res.end('[{"errorCode":"INVALID_FIELD"}]');
      return;
    }
    if (req.method === "PATCH") {
      let raw = "";
      for await (const chunk of req) raw += chunk;
      const b = JSON.parse(raw);
      value = b.compat_text__c ?? b.Compat_Text__c;
      res.writeHead(204);
      res.end();
      return;
    }
    if (req.url!.includes("/query?")) {
      res.end(
        JSON.stringify({
          done: true,
          totalSize: 1,
          records: [{ Id: id, Name: "fixture" }],
        }),
      );
      return;
    }
    if (req.url!.endsWith("/describe")) {
      res.end('{"name":"Account","fields":[]}');
      return;
    }
    res.end(JSON.stringify({ Id: id, Compat_Text__c: value }));
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  t.after(() => server.close());
  const url = `http://127.0.0.1:${(server.address() as any).port}`;
  const c = {
    conn: new Connection({
      instanceUrl: url,
      accessToken: "synthetic",
      version: "65.0",
    }),
    tracker: new FixtureTracker(join(dir, "journal")),
    env: {
      COMPAT_API_VERSION: "65.0",
      COMPAT_CANDIDATE_URL: url,
      COMPAT_CANDIDATE_TOKEN: "synthetic",
      COMPAT_CANDIDATE_RESTRICTED_TOKEN: "restricted",
    },
    prefix: "COMPAT_CANDIDATE",
    runId: "offline",
    result: {
      id: "foundation-names",
      client: "wire",
      outcome: "not_run",
      assertions: [],
      observations: {},
      identities: {},
    },
    cleanup: [],
  } as Context;
  await foundationScenarios["foundation-names"](c);
  const o = c.result.observations as any;
  assert.equal(o["after-lower-json"]?.body.Compat_Text__c, "lower");
  assert.equal(o["after-duplicate-alias"]?.body.Compat_Text__c, "second");
  assert.equal(o["after-duplicate-raw"]?.body.Compat_Text__c, "second");
});

test("projection and name-equality probes request deterministic fixture ordering", async (t) => {
  const { createServer } = await import("node:http");
  for (const scenario of ["foundation-pagination", "foundation-text-null"]) {
    let selected = "";
    const server = createServer((req, res) => {
      const q = new URL(req.url!, "http://localhost").searchParams.get("q")!;
      const target =
        scenario === "foundation-pagination"
          ? q.startsWith("SELECT Name FROM")
          : q.startsWith("SELECT Id FROM Account WHERE Name");
      res.setHeader("content-type", "application/json");
      if (target) {
        selected = q;
        res.writeHead(500);
        res.end("{}");
        return;
      }
      const records =
        scenario === "foundation-pagination"
          ? Array.from({ length: 2001 }, (_, i) => ({ Id: String(i) }))
          : [];
      res.end(
        JSON.stringify({ totalSize: records.length, done: true, records }),
      );
    });
    await new Promise<void>((resolve) =>
      server.listen(0, "127.0.0.1", resolve),
    );
    t.after(() => server.close());
    let created = 0;
    const c = {
      conn: {
        sobject: () => ({
          create: async () => ({
            success: true,
            id: `001${String(++created).padStart(12, "0")}AAA`,
          }),
        }),
      },
      tracker: { intent: async () => {}, add: async () => {} },
      env: {
        COMPAT_API_VERSION: "65.0",
        COMPAT_CANDIDATE_URL: `http://127.0.0.1:${(server.address() as any).port}`,
        COMPAT_CANDIDATE_TOKEN: "synthetic",
      },
      prefix: "COMPAT_CANDIDATE",
      runId: "ordering",
      result: {
        id: scenario,
        identities: {},
        observations: {},
        assertions: [],
      },
      cleanup: [],
    } as unknown as Context;
    await assert.rejects(
      foundationScenarios[scenario](c),
      /bounded JSON HTTP response/,
    );
    assert.match(
      selected,
      /ORDER BY Compat_Text__c(?: LIMIT 201)?$/,
      "unique fixture text values must determine order before comparing identities",
    );
  }
});
