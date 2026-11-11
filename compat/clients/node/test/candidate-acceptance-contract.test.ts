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
test("acceptance publication permits the exact new offline OpenAPI skip", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p23-report-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  let r = newReport("a".repeat(24), "full");
  const gate = assessCoverage("lab", {
    schemaVersion: 1,
    runId: r.runId,
    suite: "lab",
    complete: true,
    exitCode: 0,
    tests: [
      ...requiredCoverage.lab.map((id) => ({ id, outcome: "passed" as const })),
      { id: "candidate/openapi/traffic", outcome: "skipped" },
    ],
  });
  assert.equal(gate.outcome, "passed");
  r.gates[r.gates.findIndex((g) => g.id === "lab")] = gate;
  r = aggregateReport(r);
  const path = join(dir, "summary.json");
  await publishAcceptanceReport(path, r);
  const published = JSON.parse(await readFile(path, "utf8"));
  assert.deepEqual(
    published.gates.find((g: { id: string }) => g.id === "lab").coverage
      .allowedSkipped,
    ["candidate/openapi/traffic"],
  );
  gate.coverage.allowedSkipped = ["candidate/openapi/unknown"];
  assert.throws(() => validateAcceptanceReport(r), /RESULT/);
});
test("coverage requires the OpenAPI parent and database route measurement", () => {
  for (const [suite, id] of [
    ["jsforce", "candidate/openapi/traffic"],
    ["database", "TestHTTPServedOpenAPI"],
    ["database", "TestCompositeAtomic"],
    ["database", "TestHTTPCompositeAtomic"],
    ["jsforce", "candidate/composite/atomic"],
    ["jsforce", "candidate/foundation/decisions"],
  ] as const) {
    const complete: ParsedResult = {
      schemaVersion: 1,
      runId: "a".repeat(24),
      suite,
      complete: true,
      exitCode: 0,
      tests: [...new Set([...requiredCoverage[suite], id])].map((id) => ({
        id,
        outcome: "passed",
      })),
    };
    assert.equal(assessCoverage(suite, complete).outcome, "passed");
    for (const tests of [
      complete.tests.filter((t) => t.id !== id),
      complete.tests.map((t) =>
        t.id === id ? { ...t, outcome: "skipped" as const } : t,
      ),
      complete.tests.map((t) =>
        t.id === id ? { ...t, id: "wrong-file::" + id } : t,
      ),
    ])
      assert.equal(
        assessCoverage(suite, { ...complete, tests }).outcome,
        "failed",
      );
  }
});
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
    publishAcceptanceReport(path, { ...r, token: "secret" } as unknown),
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

test("acceptance publication permits the exact new offline Composite skip", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p23-report-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  let r = newReport("a".repeat(24), "full");
  const gate = assessCoverage("lab", {
    schemaVersion: 1,
    runId: r.runId,
    suite: "lab",
    complete: true,
    exitCode: 0,
    tests: [
      ...requiredCoverage.lab.map((id) => ({ id, outcome: "passed" as const })),
      { id: "candidate/composite/atomic", outcome: "skipped" },
    ],
  });
  assert.equal(gate.outcome, "passed");
  r.gates[r.gates.findIndex((g) => g.id === "lab")] = gate;
  r = aggregateReport(r);
  const path = join(dir, "summary.json");
  await publishAcceptanceReport(path, r);
  const published = JSON.parse(await readFile(path, "utf8"));
  assert.deepEqual(
    published.gates.find((g: { id: string }) => g.id === "lab").coverage
      .allowedSkipped,
    ["candidate/composite/atomic"],
  );
  gate.coverage.allowedSkipped = ["candidate/composite/unknown"];
  assert.throws(() => validateAcceptanceReport(r), /RESULT/);
});

test("acceptance publication permits only the exact offline foundation skip", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p25-report-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  let r = newReport("a".repeat(24), "full");
  const gate = assessCoverage("lab", {
    schemaVersion: 1,
    runId: r.runId,
    suite: "lab",
    complete: true,
    exitCode: 0,
    tests: [
      ...requiredCoverage.lab.map((id) => ({ id, outcome: "passed" as const })),
      { id: "candidate/foundation/decisions", outcome: "skipped" },
    ],
  });
  assert.equal(gate.outcome, "passed");
  r.gates[r.gates.findIndex((g) => g.id === "lab")] = gate;
  r = aggregateReport(r);
  await publishAcceptanceReport(join(dir, "summary.json"), r);
  gate.coverage.allowedSkipped = ["candidate/foundation/unknown"];
  assert.throws(() => validateAcceptanceReport(r), /RESULT/);
});

test("OAuth acceptance requires every client and policy proof before restart", async () => {
  const { gateIDs } = await import("../src/candidate-acceptance-contract.ts");
  assert.equal(gateIDs[gateIDs.indexOf("cli") + 1], "oauth");
  assert.equal(gateIDs[gateIDs.indexOf("cli") + 2], "schema");
  assert.equal(gateIDs[gateIDs.indexOf("cli") + 3], "restart");
  const ids = [
    "oauth/provision",
    "oauth/sf-web",
    "oauth/sf-display-connected",
    "oauth/sf-refresh",
    "oauth/sf-jwt",
    "oauth/sf-jwt-unlisted",
    "oauth/jsforce-web-pkce",
    "oauth/jsforce-refresh",
    "oauth/jsforce-jwt",
    "oauth/jsforce-client-credentials",
    "oauth/jsforce-revoke",
    "oauth/python-session",
    "oauth/restricted-denied",
    "oauth/tenant-isolation",
    "oauth/http-contract",
    "oauth/browser-page",
  ];
  const base: ParsedResult = {
    schemaVersion: 1,
    runId: "a".repeat(24),
    suite: "oauth",
    complete: true,
    exitCode: 0,
    tests: ids.map((id) => ({ id, outcome: "passed" })),
  };
  assert.equal(assessCoverage("oauth", base).outcome, "passed");
  for (const id of ids) {
    assert.equal(
      assessCoverage("oauth", {
        ...base,
        tests: base.tests.filter((t) => t.id !== id),
      }).outcome,
      "failed",
      id,
    );
    assert.equal(
      assessCoverage("oauth", {
        ...base,
        tests: base.tests.map((t) =>
          t.id === id ? { ...t, outcome: "skipped" } : t,
        ),
      }).outcome,
      "failed",
      id,
    );
  }
  const restart: ParsedResult = {
    ...base,
    suite: "restart",
    tests: [
      "restart/identity",
      "restart/values",
      "restart/policy",
      "restart/oauth-refresh",
      "restart/schema",
      "restart/field-types",
    ].map((id) => ({ id, outcome: "passed" })),
  };
  assert.equal(assessCoverage("restart", restart).outcome, "passed");
  assert.equal(
    assessCoverage("restart", { ...restart, tests: restart.tests.slice(0, -1) })
      .outcome,
    "failed",
  );
  const report = newReport("a".repeat(24), "full");
  report.evidence.push({ gate: "oauth", sha256: "a".repeat(64) });
  validateAcceptanceReport(report);
});
