import test from "node:test";
import assert from "node:assert/strict";
import { parseGateEvents } from "../src/candidate-acceptance-results.ts";
test("nested repeats keep ancestry while replayed terminal results still fail coverage", async () => {
  const { assessCoverage, requiredCoverage } =
    await import("../src/candidate-acceptance-contract.ts");
  const terminal = (
    name: string,
    nesting: number,
    testId: number,
    parentId: number,
    testNumber: number,
  ) => ({
    type: "test:pass",
    data: {
      name,
      nesting,
      testId,
      parentId,
      testNumber,
      file: "repeat.test.ts",
    },
  });
  const cases = [
    terminal("leaf", 1, 2, 1, 1),
    terminal("first", 0, 1, 0, 1),
    terminal("leaf", 1, 4, 3, 1),
    terminal("second", 0, 3, 0, 2),
  ];
  for (const legacy of [false, true]) {
    const events = cases.map((e) => {
      const c = structuredClone(e);
      if (legacy) {
        delete (c.data as any).testId;
        delete (c.data as any).parentId;
      }
      return c;
    });
    const encode = (es: unknown[]) =>
      [...es, { type: "test:summary", data: {} }]
        .map((e) => JSON.stringify(e))
        .join("\n");
    const parsed = parseGateEvents("node", encode(events), "lab");
    assert.equal(new Set(parsed.map((t) => t.id)).size, 4);
    assert.ok(parsed[0].id.includes('"first","leaf"'));
    assert.ok(parsed[2].id.includes('"second","leaf"'));
    const result = {
      schemaVersion: 1 as const,
      runId: "a".repeat(24),
      suite: "lab" as const,
      complete: true,
      exitCode: 0,
      tests: [
        ...requiredCoverage.lab.map((id) => ({
          id,
          outcome: "passed" as const,
        })),
        ...parsed,
      ],
    };
    assert.equal(assessCoverage("lab", result).outcome, "passed");
    assert.equal(
      assessCoverage("lab", { ...result, tests: [...result.tests, parsed[0]] })
        .outcome,
      "failed",
    );
  }
  // Parent IDs distinguish interleaved concurrent sibling groups.
  const interleaved = [
    cases[0],
    cases[2],
    cases[1],
    cases[3],
    { type: "test:summary", data: {} },
  ]
    .map((e) => JSON.stringify(e))
    .join("\n");
  assert.equal(
    new Set(parseGateEvents("node", interleaved, "lab").map((t) => t.id)).size,
    4,
  );
});
test("OpenAPI parent identity survives Node result parsing without changing ordinary identities", () => {
  const events = [
    {
      type: "test:pass",
      data: {
        name: "candidate/openapi/traffic",
        file: "candidate-openapi.test.ts",
      },
    },
    {
      type: "test:pass",
      data: { name: "ordinary", file: "candidate-openapi.test.ts" },
    },
    { type: "test:summary", data: {} },
  ]
    .map((e) => JSON.stringify(e))
    .join("\n");
  assert.deepEqual(parseGateEvents("node", events, "jsforce"), [
    { id: "candidate/openapi/traffic", outcome: "passed" },
    { id: "candidate-openapi.test.ts::ordinary", outcome: "passed" },
  ]);
});
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
