import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import childProcess from "node:child_process";
import { syncBuiltinESMExports } from "node:module";
import { Connection } from "jsforce";
import { capture } from "../src/capture.ts";
import { SCENARIOS } from "../src/validate.ts";
const env = {
  COMPAT_REFERENCE_URL: "https://reference.invalid",
  COMPAT_REFERENCE_TOKEN: "synthetic-secret",
  COMPAT_REFERENCE_RESTRICTED_TOKEN: "restricted-secret",
  COMPAT_REFERENCE_LOCALE_USER_ID: "user",
  COMPAT_API_VERSION: "65.0",
  COMPAT_DISPOSABLE: "yes",
};
test("direct reference capture blocks pending authorization before clients or requests", async (t) => {
  let calls = 0;
  const reject = () => {
    calls++;
    throw new Error("Transport forbidden in offline test");
  };
  t.mock.method(Connection.prototype, "request", reject);
  t.mock.method(globalThis, "fetch", reject);
  t.mock.method(childProcess, "spawn", reject);
  syncBuiltinESMExports();
  try {
    const report = await capture("reference", env);
    assert.equal(report.provenance, "none");
    assert.equal(calls, 0);
    assert.deepEqual(report.requiredScenarios, SCENARIOS);
    assert.ok(report.results.every((r) => r.outcome === "blocked"));
    assert.deepEqual(report.cleanup, []);
    assert.ok(!JSON.stringify(report).includes("synthetic-secret"));
  } finally {
    t.mock.restoreAll();
    syncBuiltinESMExports();
  }
});
test("CLI preflight reports reference blockers but candidate needs no reference decision", () => {
  const run = (target: string, e: NodeJS.ProcessEnv) =>
    spawnSync(
      process.execPath,
      ["--import", "tsx", "src/cli.ts", "preflight", "--target", target],
      { encoding: "utf8", env: { ...process.env, ...e } },
    );
  const r = run("reference", env);
  assert.equal(r.status, 2);
  assert.match(r.stdout + r.stderr, /not authorized/);
  assert.ok(!(r.stdout + r.stderr).includes("synthetic-secret"));
  const c = run("candidate", {
    COMPAT_CANDIDATE_URL: "http://127.0.0.1:8080",
    COMPAT_CANDIDATE_TOKEN: "candidate-secret",
    COMPAT_API_VERSION: "65.0",
    COMPAT_DISPOSABLE: "yes",
  });
  assert.equal(c.status, 0, c.stderr);
  assert.match(c.stdout, /no observation performed/);
});
