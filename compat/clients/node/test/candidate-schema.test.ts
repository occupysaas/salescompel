import test from "node:test";
import assert from "node:assert/strict";
import { runtimeEnvironment } from "../src/candidate-schema.ts";
import {
  assessCoverage,
  requiredCoverage,
} from "../src/candidate-acceptance-contract.ts";

test("schema gate refuses omitted or skipped required behavior", () => {
  const missing = assessCoverage("schema", {
    schemaVersion: 1,
    runId: "0".repeat(24),
    suite: "schema",
    complete: true,
    exitCode: 0,
    tests: [],
  });
  assert.notEqual(missing.outcome, "passed");
});

test("every schema proof must be present and passing", () => {
  const base = {
    schemaVersion: 1 as const,
    runId: "0".repeat(24),
    suite: "schema" as const,
    complete: true,
    exitCode: 0,
    tests: requiredCoverage.schema.map((id) => ({
      id,
      outcome: "passed" as const,
    })),
  };
  assert.equal(assessCoverage("schema", base).outcome, "passed");
  for (const id of requiredCoverage.schema) {
    assert.equal(
      assessCoverage("schema", {
        ...base,
        tests: base.tests.filter((t) => t.id !== id),
      }).outcome,
      "failed",
    );
    assert.equal(
      assessCoverage("schema", {
        ...base,
        tests: base.tests.map((t) =>
          t.id === id ? { ...t, outcome: "skipped" as const } : t,
        ),
      }).outcome,
      "failed",
    );
  }
});
test("record server environment excludes owner and database-test credentials", () => {
  const env = runtimeEnvironment(
    {
      DATABASE_URL: "owner",
      MIGRATION_DATABASE_URL: "owner",
      TEST_DATABASE_URL: "owner",
      TEST_RECORDS_URL: "owner",
      TEST_IDENTITY_URL: "owner",
      PATH: "/bin",
    },
    { records: "records", identity: "identity" },
  );
  assert.equal(env.DATABASE_URL, "records");
  assert.equal(env.AUTH_DATABASE_URL, "identity");
  assert.equal(env.PATH, "/bin");
  assert.ok(!JSON.stringify(env).includes("owner"));
});
