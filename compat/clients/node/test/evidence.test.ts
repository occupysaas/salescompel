import test from "node:test";
import assert from "node:assert/strict";
import { spawnSync } from "node:child_process";
import { mkdtemp, readFile, writeFile, rm, stat } from "node:fs/promises";
import { tmpdir } from "node:os";
import { resolve } from "node:path";
import {
  verifyCapturePair,
  observationIssues,
  observationLinkIssues,
} from "../src/evidence.ts";
import {
  readDoc,
  validateRepository,
  profile,
  SCENARIOS,
  CLEANUP_SCENARIOS,
} from "../src/validate.ts";
import type { Report } from "../src/core.ts";
const schema = await readDoc("compat/schemas/observation.schema.json");
const p = await profile();
const context = {
  profile: p.data.id,
  profileHash: p.hash,
  apiVersion: "65.0",
  connectors: p.data.connectors,
  requiredScenarios: SCENARIOS,
  cleanupScenarios: CLEANUP_SCENARIOS,
};
const readiness = {
  observer: "observer",
  implementer: "implementer",
  decision: { recordedAt: "2025-12-31" },
};
function report(second = false): Report {
  return {
    schemaVersion: 1,
    target: "reference",
    profile: "pilot",
    profileHash: context.profileHash,
    apiVersion: "65.0",
    provenance: "live",
    startedAt: second ? "2026-01-02T00:00:00Z" : "2026-01-01T00:00:00Z",
    finishedAt: second ? "2026-01-02T00:01:00Z" : "2026-01-01T00:01:00Z",
    connectors: context.connectors,
    requiredScenarios: SCENARIOS,
    results: SCENARIOS.map((id) => ({
      id,
      client: "jsforce",
      outcome: "passed",
      assertions: [{ name: "expected behavior", passed: true }],
      observations: { status: 200 },
      identities: {},
    })),
    cleanup: CLEANUP_SCENARIOS.map((scenario) => ({
      scenario,
      resource: "synthetic",
      outcome: "passed",
    })),
  };
}
function note(): any {
  return {
    schemaVersion: 1,
    id: "pilot-first",
    profile: "pilot",
    profileHash: context.profileHash,
    apiVersion: "65.0",
    connectors: context.connectors,
    contracts: SCENARIOS,
    captures: [
      {
        sha256: "a".repeat(64),
        startedAt: "2026-01-01T00:00:00Z",
        finishedAt: "2026-01-01T00:01:00Z",
      },
      {
        sha256: "b".repeat(64),
        startedAt: "2026-01-02T00:00:00Z",
        finishedAt: "2026-01-02T00:01:00Z",
      },
    ],
    observer: "observer",
    review: {
      reviewer: "reviewer",
      reviewedAt: "2026-01-03T00:00:00Z",
      sanitized: true,
    },
    repeatability: "passed",
    cleanup: "passed",
    notes: SCENARIOS.map((contract) => ({
      contract,
      summary: "Synthetic test note; never registered as evidence.",
    })),
  };
}
test("capture pair accepts distinct sequential repeatable reports and rejects invalid evidence", () => {
  const a = JSON.stringify(report()),
    b = JSON.stringify(report(true));
  const result = verifyCapturePair(a, b, context);
  assert.equal(result.outcome, "passed");
  assert.equal(result.captures.length, 2);
  assert.equal(verifyCapturePair(a, a, context).outcome, "blocked");
  for (const mutate of [
    (r: any) => (r.provenance = "synthetic"),
    (r: any) => (r.target = "candidate"),
    (r: any) => (r.startedAt = "2026-01-01T00:00:10Z"),
    (r: any) => (r.finishedAt = "2026-02-30T00:00:00Z"),
    (r: any) => (r.cleanup = []),
    (r: any) => (r.cleanup[0].outcome = "failed"),
    (r: any) => r.results.pop(),
    (r: any) => (r.apiVersion = "64.0"),
    (r: any) => (r.connectors = { jsforce: "0.0.0" }),
  ]) {
    const r = report(true);
    mutate(r);
    const result = verifyCapturePair(a, JSON.stringify(r), context);
    assert.notEqual(result.outcome, "passed");
    assert.deepEqual(result.captures, []);
  }
  const changed = report(true);
  changed.results[0].observations = { status: 400 };
  assert.equal(
    verifyCapturePair(a, JSON.stringify(changed), context).outcome,
    "failed",
  );
  assert.equal(
    verifyCapturePair("secret-invalid-json", b, context).outcome,
    "blocked",
  );
  assert.ok(
    !JSON.stringify(
      verifyCapturePair("secret-invalid-json", b, context),
    ).includes("secret-invalid-json"),
  );
});
test("observation requires complete reviewed sanitized bindings", () => {
  assert.deepEqual(observationIssues(note(), schema, context, readiness), []);
  for (const mutate of [
    (n: any) => (n.review.sanitized = false),
    (n: any) => (n.review.reviewer = " "),
    (n: any) => (n.review.reviewedAt = "2025-01-01T00:00:00Z"),
    (n: any) => (n.review.reviewedAt = "2999-01-01T00:00:00Z"),
    (n: any) => (n.profileHash = "b".repeat(64)),
    (n: any) => (n.observer = "implementer"),
    (n: any) => (n.contracts = ["unknown"]),
    (n: any) => n.notes.pop(),
    (n: any) => n.notes.push(n.notes[0]),
    (n: any) => (n.captures[1].sha256 = n.captures[0].sha256),
    (n: any) => (n.captures[1].startedAt = "2026-01-01T00:00:00Z"),
    (n: any) => (n.secret = "bad"),
    (n: any) => (n.notes[0].summary = "access_token=bad"),
    (n: any) => (n.notes[0].summary = "User 005000000000001AAA"),
  ]) {
    const n = note();
    mutate(n);
    assert.ok(
      observationIssues(n, schema, context, readiness).length,
      JSON.stringify(n),
    );
  }
});
test("verify-evidence CLI blocks malformed private input without copying it to output", async () => {
  const dir = await mkdtemp(resolve(tmpdir(), "evidence-"));
  try {
    const first = resolve(dir, "first.json"),
      second = resolve(dir, "second.json"),
      out = resolve(dir, "result.json");
    await writeFile(first, "secret-invalid-json");
    await writeFile(second, JSON.stringify(report(true)));
    const r = spawnSync(
      process.execPath,
      [
        "--import",
        "tsx",
        "src/cli.ts",
        "verify-evidence",
        "--first",
        first,
        "--second",
        second,
        "--out",
        out,
      ],
      { encoding: "utf8" },
    );
    assert.equal(r.status, 2);
    const result = await readFile(out, "utf8");
    assert.ok(!result.includes("secret-invalid-json"));
    assert.equal(JSON.parse(result).outcome, "blocked");
    assert.equal((await stat(out)).mode & 0o777, 0o600);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});

test("contract links require unique reciprocal reviewed evidence", () => {
  const pending = SCENARIOS.map((id) => ({ id, observationStatus: "not_run" }));
  const observed = SCENARIOS.map((id) => ({
    id,
    observationStatus: "observed",
    observationIds: ["pilot-first"],
  }));
  assert.deepEqual(
    observationLinkIssues(pending, [], schema, context, readiness),
    [],
  );
  assert.deepEqual(
    observationLinkIssues(observed, [note()], schema, context, readiness),
    [],
  );
  assert.ok(
    observationLinkIssues(observed, [], schema, context, readiness).length,
  );
  assert.ok(
    observationLinkIssues(pending, [note()], schema, context, readiness).length,
  );
  assert.ok(
    observationLinkIssues(
      observed,
      [note(), note()],
      schema,
      context,
      readiness,
    ).length,
  );
  assert.ok(
    observationLinkIssues(
      observed.map((c) => ({ ...c, observationIds: ["missing"] })),
      [note()],
      schema,
      context,
      readiness,
    ).length,
  );
  assert.ok(
    observationLinkIssues(
      observed.map((c) => ({
        ...c,
        observationIds: ["pilot-first", "pilot-first"],
      })),
      [note()],
      schema,
      context,
      readiness,
    ).length,
  );
  const bad = note();
  bad.review.sanitized = false;
  assert.ok(
    observationLinkIssues(observed, [bad], schema, context, readiness).length,
  );
  const stale = note();
  stale.profileHash = "c".repeat(64);
  assert.ok(
    observationLinkIssues(observed, [stale], schema, context, readiness).length,
  );
});
test("repository validation does not rewrite profile bytes or fabricate observations", async () => {
  const before = await profile();
  assert.deepEqual(await validateRepository(), []);
  const after = await profile();
  assert.equal(after.hash, before.hash);
  assert.deepEqual(after.data, before.data);
});

test("evidence taken before authorization cannot mark contracts observed", () => {
  const laterApproval = {
    ...readiness,
    decision: { recordedAt: "2026-10-01" },
  };
  assert.ok(observationIssues(note(), schema, context, laterApproval).length);
  assert.ok(
    observationLinkIssues(
      SCENARIOS.map((id) => ({
        id,
        observationStatus: "observed",
        observationIds: ["pilot-first"],
      })),
      [note()],
      schema,
      context,
      laterApproval,
    ).length,
  );
  const sameDay = { ...readiness, decision: { recordedAt: "2026-01-01" } };
  assert.deepEqual(observationIssues(note(), schema, context, sameDay), []);
});
test("ordinary prose remains valid while Salesforce-shaped identifiers are rejected", () => {
  const n = note();
  n.notes[0].summary = "An acknowledgement is returned.";
  assert.deepEqual(observationIssues(n, schema, context, readiness), []);
  for (const id of [
    "005000000000001AAA",
    "001000000000001",
    "00D000000000001EAA",
    "a0B000000000001",
  ]) {
    n.notes[0].summary = `Reference identifier ${id}`;
    assert.ok(observationIssues(n, schema, context, readiness).length, id);
  }
});
test("obvious credentials are rejected regardless of capitalization", () => {
  for (const text of [
    "ACCESS_TOKEN=synthetic-secret",
    "bearer synthetic-secret",
    "bAsIc c3ludGhldGlj",
    "Refresh_Token: synthetic-secret",
    "PASSWORD=synthetic-secret",
    "Authorization: synthetic-secret",
  ]) {
    const n = note();
    n.notes[0].summary = text;
    assert.ok(observationIssues(n, schema, context, readiness).length, text);
  }
});

test("observations allow optional role separation but require the recorded operator", () => {
  for (const implementer of [readiness.observer, null]) {
    const attribution = { ...readiness, implementer };
    assert.deepEqual(
      observationIssues(note(), schema, context, attribution),
      [],
    );
    const wrongOperator = { ...note(), observer: "unrecorded-operator" };
    assert.ok(
      observationIssues(wrongOperator, schema, context, attribution).length,
    );
  }
});
