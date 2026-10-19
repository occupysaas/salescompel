import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import {
  bodyManifest,
  bodyManifestHash,
  bodyScope,
  bodyRequest,
  validateBodyReport,
  type BodyReport,
} from "../src/external-id-body-contract.ts";
import {
  newExternalIdReport,
  manifestHash,
} from "../src/external-id-contract.ts";
import { ROOT, connectorVersions } from "../src/versions.ts";
const runId = "a".repeat(24),
  digest = "b".repeat(64);
function report(): BodyReport {
  const time = "2026-10-19T12:00:00.000Z";
  return {
    schemaVersion: 1,
    kind: "external-id-body-validation",
    manifestHash: bodyManifestHash,
    config: {
      schemaVersion: 1,
      apiVersion: "65.0",
      profileHash: createHash("sha256")
        .update(
          readFileSync(
            resolve(ROOT, "docs/compat/profiles/rest-query-foundation.yaml"),
          ),
        )
        .digest("hex"),
      origin: "https://reference.example.invalid",
      administrator: { user: "synthetic-user", org: "synthetic-org" },
    },
    connectors: connectorVersions(),
    runId,
    startedAt: time,
    finishedAt: time,
    identityVerified: true,
    collection: "complete",
    observations: bodyManifest.map((c) => {
      const scope = bodyScope(runId, c);
      return {
        caseId: c.id,
        request: bodyRequest(scope, c.valueKind),
        pathKey: scope.pathKey,
        before: [],
        after: [],
        stateVerified: true,
        status: 200,
        bodyPresent: true,
        body: { unexpected: true },
        state: "observed",
        artifact: `response-${c.id}.json`,
        sha256: digest,
        stateArtifact: `state-${c.id}.json`,
        stateSha256: digest,
        ...(c.recordState === "existing"
          ? { seedArtifact: `seed-${c.id}.json`, seedSha256: digest }
          : {}),
      };
    }),
    cleanup: {
      outcome: "clean",
      startedAt: time,
      finishedAt: time,
      journalSha256: digest,
      records: [],
      unresolvedOperationIds: [],
      reasons: [],
    },
    reasons: [],
  };
}
test("manifest has sixteen unique cases", () => {
  assert.equal(bodyManifest.length, 16);
  assert.equal(new Set(bodyManifest.map((c) => c.id)).size, 16);
  for (const state of ["existing", "missing"])
    assert.equal(bodyManifest.filter((c) => c.recordState === state).length, 8);
  assert.notEqual(bodyManifestHash, manifestHash);
});
test("request preserves omission and supplied JSON types", () => {
  const scope = bodyScope(runId, {
    id: "missing-null",
    recordState: "missing",
    valueKind: "null",
  });
  assert.deepEqual(scope, {
    pathKey: `p11${runId}-missing-null-AbC`,
    variantKey: `p11${runId}-missing-null-abc`,
    differentKey: `p11${runId}-missing-null-different`,
    seedName: `p11${runId}-missing-null-seed`,
    requestName: `p11${runId}-missing-null-request`,
  });
  assert.equal(
    Object.hasOwn(bodyRequest(scope, "omitted"), "Compat_Key__c"),
    false,
  );
  assert.equal(bodyRequest(scope, "null").Compat_Key__c, null);
  assert.equal(bodyRequest(scope, "number").Compat_Key__c, 42);
  for (const [kind, value] of [
    ["identical", scope.pathKey],
    ["variant", scope.variantKey],
    ["different", scope.differentKey],
    ["null", null],
    ["empty", ""],
    ["number", 42],
    ["object", { value: "invalid" }],
  ] as const)
    assert.deepEqual(bodyRequest(scope, kind), {
      Name: scope.requestName,
      Compat_Key__c: value,
    });
});
test("report requires suite binding and exact coverage", () => {
  const good = report();
  validateBodyReport(good);
  const bad: unknown[] = [
    newExternalIdReport(good.config, runId),
    { ...good, manifestHash },
    { ...good, observations: good.observations.slice(1) },
    {
      ...good,
      observations: [...good.observations.slice(1), good.observations[1]],
    },
    { ...good, connectors: { ...good.connectors, jsforce: "0.0.0" } },
  ];
  for (const patch of [
    { artifact: "../response.json" },
    { stateArtifact: "/state.json" },
    { seedArtifact: "a\\b.json" },
    { sha256: "" },
    { pathKey: "wrong" },
    { request: { Name: "wrong" } },
    { state: "unavailable" },
    { stateVerified: false },
    { seedArtifact: undefined },
  ])
    bad.push({
      ...good,
      observations: [
        { ...good.observations[0], ...patch },
        ...good.observations.slice(1),
      ],
    });
  for (const value of bad) assert.throws(() => validateBodyReport(value));
  validateBodyReport({
    ...good,
    collection: "blocked",
    identityVerified: false,
    observations: [
      {
        ...good.observations[0],
        state: "unavailable",
        status: null,
        stateVerified: false,
        artifact: "",
        sha256: "",
        stateArtifact: "",
        stateSha256: "",
        seedArtifact: undefined,
        seedSha256: undefined,
      },
    ],
  });
});
test("cleanup clean requires exact ID absence and resolved dispatch", () => {
  const good = report();
  const row = {
    Id: "001000000000001AAA",
    Name: "synthetic",
    Compat_Key__c: "synthetic",
    Compat_Text__c: null,
  };
  good.observations[0].after = [row];
  assert.throws(() => validateBodyReport(good));
  good.cleanup.records = [
    {
      caseId: good.observations[0].caseId,
      id: row.Id,
      absenceArtifact: "absence-001.json",
      absenceSha256: digest,
    },
  ];
  validateBodyReport(good);
  for (const cleanup of [
    { ...good.cleanup, unresolvedOperationIds: ["dispatch-1"] },
    { ...good.cleanup, journalSha256: "" },
    {
      ...good.cleanup,
      records: [{ ...good.cleanup.records[0], absenceSha256: "" }],
    },
  ])
    assert.throws(() => validateBodyReport({ ...good, cleanup }));
  validateBodyReport({
    ...good,
    collection: "blocked",
    cleanup: {
      ...good.cleanup,
      outcome: "blocked",
      records: [],
      journalSha256: "",
      unresolvedOperationIds: ["dispatch-1"],
    },
  });
});
