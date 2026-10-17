import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { wire } from "../src/wire.ts";

test("wire rejects off-origin paths before transport and never follows redirects", async (t) => {
  const received: string[] = [];
  const server = createServer((req, res) => {
    received.push(req.url!);
    res.writeHead(302, { Location: "/services/data/v65.0/stolen" });
    res.end();
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => server.close());
  const base = new URL(`http://127.0.0.1:${(server.address() as any).port}`);
  for (const path of [
    "//other.invalid/x",
    "https://other.invalid/x",
    "/outside",
    "/services/../outside",
    "/services/data/v65.0/query#x",
  ])
    await assert.rejects(wire(base, "synthetic", path, {}));
  assert.deepEqual(received, []);
  await assert.rejects(
    wire(base, "synthetic", "/services/data/v65.0/query", {}),
  );
  assert.deepEqual(received, ["/services/data/v65.0/query"]);
});

test("wire preserves encoded path, method, headers, raw status and JSON properties", async (t) => {
  const requests: any[] = [];
  const server = createServer((req, res) => {
    requests.push({
      url: req.url,
      method: req.method,
      token: req.headers.authorization,
      batch: req.headers["sforce-query-options"],
    });
    res.writeHead(400, {
      "Content-Type": "application/json",
      "Set-Cookie": "private",
    });
    res.end(
      JSON.stringify([
        { errorCode: "INVALID_FIELD", fields: [], message: "private prose" },
      ]),
    );
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => server.close());
  const base = new URL(`http://127.0.0.1:${(server.address() as any).port}`);
  const path =
    "/services/data/v65.0/sobjects/Account/Compat_Key__c/key%2F%25%C3%A9";
  const got = await wire(base, "synthetic", path, {
    method: "PATCH",
    headers: { "Sforce-Query-Options": "batchSize=200" },
    body: "{}",
  });
  assert.deepEqual(requests, [
    {
      url: path,
      method: "PATCH",
      token: "Bearer synthetic",
      batch: "batchSize=200",
    },
  ]);
  assert.equal(got.status, 400);
  assert.deepEqual(got.body, [
    { errorCode: "INVALID_FIELD", fields: [], message: "private prose" },
  ]);
  assert.equal(got.headers["set-cookie"], undefined);
});

test("wire rejects oversized and stalled responses", async (t) => {
  const server = createServer((req, res) => {
    if (req.url!.endsWith("large")) res.end("x".repeat(1024));
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => {
    server.closeAllConnections();
    server.close();
  });
  const base = new URL(`http://127.0.0.1:${(server.address() as any).port}`);
  await assert.rejects(
    wire(
      base,
      "synthetic",
      "/services/data/v65.0/large",
      {},
      { maxBytes: 32, timeoutMs: 100 },
    ),
    /size/i,
  );
  await assert.rejects(
    wire(
      base,
      "synthetic",
      "/services/data/v65.0/stall",
      {},
      { maxBytes: 32, timeoutMs: 20 },
    ),
  );
});
