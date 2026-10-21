import test from "node:test";
import assert from "node:assert/strict";
import {
  assessCoverage,
  aggregateReport,
  newReport,
  publishAcceptanceReport,
  validateAcceptanceReport,
  requiredCoverage,
  type ParsedResult,
} from "../src/candidate-acceptance-contract.ts";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
test("coverage refuses empty, skipped and incomplete required client results", () => {
  const base = {
    schemaVersion: 1 as const,
    runId: "a".repeat(24),
    suite: "jsforce" as const,
    complete: true,
    exitCode: 0,
    tests: [],
  };
  assert.equal(assessCoverage("jsforce", base).outcome, "failed");
  const tests = requiredCoverage.jsforce.map((id) => ({
    id,
    outcome: "passed" as const,
  }));
  assert.equal(assessCoverage("jsforce", { ...base, tests }).outcome, "passed");
  assert.equal(
    assessCoverage("jsforce", {
      ...base,
      tests: tests.map((x) => ({ ...x, outcome: "skipped" })),
    }).outcome,
    "failed",
  );
  assert.equal(
    assessCoverage("jsforce", { ...base, tests, complete: false }).outcome,
    "failed",
  );
});
test("scope, source and cleanup cannot forge full acceptance", () => {
  const r = newReport("a".repeat(24), "full");
  assert.equal(aggregateReport(r).acceptanceEligible, false);
  assert.throws(() =>
    validateAcceptanceReport({
      ...r,
      outcome: "passed",
      acceptanceEligible: true,
    }),
  );
  const subset = newReport("b".repeat(24), "candidate-only");
  assert.equal(aggregateReport(subset).acceptanceEligible, false);
});
test("publication is exclusive and rejects secret-bearing extra properties", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p15-contract-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "report.json"),
    r = newReport("a".repeat(24), "full");
  await assert.rejects(
    publishAcceptanceReport(path, { ...r, token: "secret" } as any),
  );
  await publishAcceptanceReport(path, r);
  const saved = await readFile(path, "utf8");
  assert.ok(!saved.includes("secret"));
  await assert.rejects(publishAcceptanceReport(path, r));
  assert.equal(await readFile(path, "utf8"), saved);
});

test("coverage requires query metadata regressions", () => {
  for (const [suite, ids] of [
    ["cli", ["cli/query-contact", "cli/query-empty"]],
    ["database", ["TestHTTPQueryColumns", "TestHTTPQueryColumnsPolicy"]],
  ] as const) {
    const tests = [...new Set([...requiredCoverage[suite], ...ids])].map(
      (id) => ({ id, outcome: "passed" as const }),
    );
    const result = {
      schemaVersion: 1 as const,
      runId: "a".repeat(24),
      suite,
      complete: true,
      exitCode: 0,
      tests,
    };
    assert.equal(assessCoverage(suite, result).outcome, "passed");
    for (const id of ids) {
      for (const outcome of ["omitted", "failed", "skipped"] as const) {
        const changed = tests.flatMap<ParsedResult["tests"][number]>((t) =>
          t.id !== id ? [t] : outcome === "omitted" ? [] : [{ id, outcome }],
        );
        assert.equal(
          assessCoverage(suite, { ...result, tests: changed }).outcome,
          "failed",
          `${suite}/${id}/${outcome}`,
        );
      }
    }
  }
});
