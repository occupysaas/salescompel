import test from "node:test";
import assert from "node:assert/strict";
import { parseGateEvents } from "../src/candidate-acceptance-results.ts";
test("native event adapters require terminal evidence and preserve failures and skips", () => {
  assert.throws(() => parseGateEvents("node", "", "lab"));
  const events = [
    { type: "test:pass", data: { name: "sample", file: "/tmp/core.test.ts" } },
    { type: "test:summary", data: {} },
  ]
    .map((x) => JSON.stringify(x))
    .join("\n");
  assert.deepEqual(parseGateEvents("node", events, "lab"), [
    { id: "core.test.ts::sample", outcome: "passed" },
  ]);
  assert.throws(() =>
    parseGateEvents(
      "go",
      JSON.stringify({ Action: "run", Package: "p", Test: "T" }),
      "database",
    ),
  );
  assert.throws(() => parseGateEvents("playwright", "{}", "preview"));
});
test("bound native output rejects another run, corrupt bytes and missing metadata", async (t) => {
  const { mkdtemp, writeFile, rm } = await import("node:fs/promises"),
    { tmpdir } = await import("node:os"),
    { join } = await import("node:path");
  const { readGateResult, digest } =
    await import("../src/candidate-acceptance-results.ts");
  const dir = await mkdtemp(join(tmpdir(), "p15-results-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "node.jsonl"),
    text = [
      { type: "test:pass", data: { name: "sample", file: "test.ts" } },
      { type: "test:summary", data: {} },
    ]
      .map((x) => JSON.stringify(x))
      .join("\n");
  await writeFile(path, text, { mode: 0o600 });
  await assert.rejects(readGateResult("node", path, "a".repeat(24)));
  await writeFile(
    path + ".binding.json",
    JSON.stringify({
      schemaVersion: 1,
      runId: "b".repeat(24),
      gate: "lab",
      digest: digest(text),
      complete: true,
      exitCode: 0,
    }),
  );
  await assert.rejects(readGateResult("node", path, "a".repeat(24)));
  await writeFile(
    path + ".binding.json",
    JSON.stringify({
      schemaVersion: 1,
      runId: "a".repeat(24),
      gate: "lab",
      digest: "0".repeat(64),
      complete: true,
      exitCode: 0,
    }),
  );
  await assert.rejects(readGateResult("node", path, "a".repeat(24)));
});
test("Playwright expected-failure annotations cannot turn a failed assertion into acceptance", () => {
  const text = JSON.stringify({
    stats: {},
    suites: [
      {
        specs: [
          {
            title: "required",
            tests: [{ status: "expected", results: [{ status: "failed" }] }],
          },
        ],
      },
    ],
    errors: [],
  });
  assert.equal(
    parseGateEvents("playwright", text, "preview")[0].outcome,
    "failed",
  );
});
