// Synthetic HTTP fixture tests the harness/client wiring, never Salesforce parity.
import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { once } from "node:events";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { Connection } from "jsforce";
import { FixtureTracker } from "../src/fixtures.ts";
import { jsScenarios, type Context } from "../src/scenarios.ts";

test("real JSforce CRUD wiring runs against an explicitly synthetic HTTP fixture", async () => {
  const dir = await mkdtemp(join(tmpdir(), "sc-wire-"));
  const id = "001000000000001AAA";
  let record: any;
  const calls: string[] = [];
  const server = createServer(async (req, res) => {
    calls.push(req.method + " " + req.url);
    let body = "";
    for await (const chunk of req) body += chunk;
    res.setHeader("content-type", "application/json");
    if (req.method === "POST") {
      record = { Id: id, ...JSON.parse(body) };
      res.writeHead(201);
      res.end(JSON.stringify({ id, success: true, errors: [] }));
    } else if (!record) {
      res.writeHead(404);
      res.end(
        JSON.stringify([
          { errorCode: "NOT_FOUND", message: "synthetic missing record" },
        ]),
      );
    } else if (req.method === "GET") res.end(JSON.stringify(record));
    else if (req.method === "PATCH") {
      record = { ...record, ...JSON.parse(body) };
      res.writeHead(204);
      res.end();
    } else if (req.method === "DELETE") {
      record = undefined;
      res.writeHead(204);
      res.end();
    } else {
      res.writeHead(405);
      res.end();
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  try {
    const port = (server.address() as any).port;
    const c: Context = {
      conn: new Connection({
        instanceUrl: `http://127.0.0.1:${port}`,
        accessToken: "synthetic-only",
        version: "65.0",
      }),
      env: {},
      prefix: "SYNTHETIC",
      runId: "test-run",
      tracker: new FixtureTracker(join(dir, "journal")),
      cleanup: [],
      result: {
        id: "crud",
        client: "jsforce",
        outcome: "not_run",
        assertions: [],
        observations: {},
        identities: {},
      },
    };
    await jsScenarios.crud(c);
    assert.ok(c.result.assertions.every((a) => a.passed));
    assert.equal((c.result.observations as any).missing.errorCode, "NOT_FOUND");
    const cleanup = await c.tracker.cleanup();
    assert.equal(cleanup[0].outcome, "passed");
    assert.ok(
      calls.some((x) => x === "POST /services/data/v65.0/sobjects/Account"),
    );
    assert.ok(calls.some((x) => x.startsWith("DELETE ")));
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));
    await rm(dir, { recursive: true, force: true });
  }
});

test("composite probe sends JSON and confirms rollback with a real connector readback", async () => {
  const dir = await mkdtemp(join(tmpdir(), "sc-composite-"));
  let media: string | undefined;
  let readback = false;
  const server = createServer(async (req, res) => {
    res.setHeader("content-type", "application/json");
    for await (const _ of req) {
    }
    if (req.method === "POST") {
      media = req.headers["content-type"];
      res.end(
        JSON.stringify({
          compositeResponse: [
            {
              referenceId: "account",
              httpStatusCode: 400,
              body: [{ errorCode: "PROCESSING_HALTED" }],
            },
            {
              referenceId: "bad",
              httpStatusCode: 400,
              body: [{ errorCode: "INVALID_FIELD" }],
            },
          ],
        }),
      );
    } else {
      readback = true;
      res.end(JSON.stringify({ totalSize: 0, done: true, records: [] }));
    }
  });
  server.listen(0, "127.0.0.1");
  await once(server, "listening");
  try {
    const c: Context = {
      conn: new Connection({
        instanceUrl: `http://127.0.0.1:${(server.address() as any).port}`,
        accessToken: "synthetic-only",
        version: "65.0",
      }),
      env: {},
      prefix: "SYNTHETIC",
      runId: "test-run",
      tracker: new FixtureTracker(join(dir, "journal")),
      cleanup: [],
      result: {
        id: "rollback",
        client: "jsforce",
        outcome: "not_run",
        assertions: [],
        observations: {},
        identities: {},
      },
    };
    await jsScenarios.rollback(c);
    assert.match(media ?? "", /^application\/json/);
    assert.equal(readback, true);
    assert.ok(c.result.assertions.every((a) => a.passed));
  } finally {
    server.closeAllConnections();
    await new Promise<void>((resolve) => server.close(() => resolve()));
    await rm(dir, { recursive: true, force: true });
  }
});
