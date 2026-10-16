import test from "node:test";
import assert from "node:assert/strict";
import { readFile, mkdtemp, mkdir, writeFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import {
  readinessIssues,
  readinessStructureIssues,
  referencePreflight,
} from "../src/readiness.ts";
import {
  ROOT,
  readDoc,
  SCENARIOS,
  CLEANUP_SCENARIOS,
} from "../src/validate.ts";
const schema = await readDoc("compat/schemas/reference-readiness.schema.json");
const context = {
  profile: "pilot",
  profileHash: "a".repeat(64),
  apiVersion: "65.0",
  requiredScenarios: SCENARIOS,
  cleanupScenarios: CLEANUP_SCENARIOS,
  connectors: { jsforce: "3.11.0" },
};
const env = {
  COMPAT_REFERENCE_URL: "https://example.invalid",
  COMPAT_REFERENCE_TOKEN: "synthetic-secret",
  COMPAT_REFERENCE_RESTRICTED_TOKEN: "restricted-secret",
  COMPAT_REFERENCE_LOCALE_USER_ID: "user",
  COMPAT_API_VERSION: "65.0",
  COMPAT_DISPOSABLE: "yes",
};
function approved(): any {
  return {
    schemaVersion: 1,
    referenceAlias: "test-org",
    profile: "pilot",
    profileHash: context.profileHash,
    apiVersion: "65.0",
    decision: {
      status: "approved",
      maintainer: "maintainer",
      recordedAt: "2026-01-01",
      logAnchor: "reference-authorization",
      observationAuthorized: true,
    },
    observer: "observer",
    implementer: "implementer",
    scenarios: SCENARIOS,
    fixtures: {
      disposable: true,
      administrator: true,
      restricted: true,
      locale: true,
      deployed: true,
    },
  };
}
test("readiness accepts complete authorization and rejects stale or incomplete scope", () => {
  assert.deepEqual(readinessIssues(approved(), schema, context, env), []);
  for (const mutate of [
    (r: any) => (r.decision.status = "pending"),
    (r: any) => (r.decision.status = "rejected"),
    (r: any) => (r.decision.observationAuthorized = false),
    (r: any) => (r.observer = " IMPLEMENTER "),
    (r: any) => (r.profileHash = "b".repeat(64)),
    (r: any) => (r.apiVersion = "64.0"),
    (r: any) => (r.scenarios = []),
    (r: any) => (r.fixtures.restricted = false),
    (r: any) => (r.decision.recordedAt = "2026-02-30"),
    (r: any) => (r.decision.recordedAt = "2999-01-01"),
    (r: any) => (r.referenceAlias = " "),
    (r: any) => (r.token = "secret"),
  ]) {
    const r = approved();
    mutate(r);
    assert.ok(
      readinessIssues(r, schema, context, env).length,
      JSON.stringify(r),
    );
  }
  for (const key of Object.keys(env)) {
    const e: any = { ...env };
    delete e[key];
    assert.ok(readinessIssues(approved(), schema, context, e).length, key);
  }
  assert.ok(
    readinessIssues(approved(), schema, { ...context, apiVersion: null }, env)
      .length,
  );
});
test("pending record is structurally valid but cannot authorize capture", () => {
  const r = approved();
  r.decision.status = "pending";
  r.decision.observationAuthorized = false;
  assert.deepEqual(readinessStructureIssues(r, schema), []);
  assert.ok(readinessIssues(r, schema, context, env).length);
});
test("preflight safely handles files and requires a real decision anchor", async () => {
  const root = await mkdtemp(resolve(tmpdir(), "readiness-"));
  try {
    await mkdir(resolve(root, "docs/compat"), { recursive: true });
    await mkdir(resolve(root, "compat/schemas"), { recursive: true });
    await writeFile(
      resolve(root, "compat/schemas/reference-readiness.schema.json"),
      JSON.stringify(schema),
    );
    assert.ok((await referencePreflight(root, context, env)).length);
    await writeFile(
      resolve(root, "docs/compat/reference-readiness.yaml"),
      "decision: [synthetic-secret",
    );
    const malformed = await referencePreflight(root, context, env);
    assert.ok(malformed.length);
    assert.ok(!malformed.join().includes("synthetic-secret"));
    await writeFile(
      resolve(root, "docs/compat/reference-readiness.yaml"),
      JSON.stringify(approved()),
    );
    await writeFile(
      resolve(root, "docs/compat/implementation-log.md"),
      "# Other decision\n",
    );
    assert.ok((await referencePreflight(root, context, env)).length);
    await writeFile(
      resolve(root, "docs/compat/implementation-log.md"),
      "## Reference authorization\nRecorded by maintainer.\n",
    );
    assert.deepEqual(await referencePreflight(root, context, env), []);
  } finally {
    await rm(root, { recursive: true, force: true });
  }
});
