import test from "node:test";
import assert from "node:assert/strict";
import {
  assessFoundationFiles,
  foundationReleaseFindings,
} from "../src/foundation-assessment-command.ts";
import { mkdtemp, chmod, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

test("file command rejects missing sidecars before creating output", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "foundation-assessment-"));
  await chmod(dir, 0o700);
  t.after(() => rm(dir, { recursive: true, force: true }));
  await assert.rejects(
    assessFoundationFiles({
      first: join(dir, "one.json"),
      second: join(dir, "two.json"),
      firstPolicy: join(dir, "one.policy.json"),
      secondPolicy: join(dir, "two.policy.json"),
      firstCleanup: join(dir, "one.cleanup.json"),
      secondCleanup: join(dir, "two.cleanup.json"),
      out: join(dir, "assessment.json"),
    }),
  );
});

test("release input rejects duplicate findings and limits relevance to selected contracts", () => {
  assert.throws(() =>
    foundationReleaseFindings({
      schemaVersion: 1,
      findings: [
        {
          id: "duplicate",
          affectedContracts: ["query-locator-lifecycle"],
          blocksContractFreeze: true,
        },
        {
          id: "duplicate",
          affectedContracts: ["query-locator-lifecycle"],
          blocksContractFreeze: true,
        },
      ],
      blockers: [],
    }),
  );
  const findings = foundationReleaseFindings({
    schemaVersion: 1,
    findings: [
      {
        id: "cursor",
        affectedContracts: ["query-locator-lifecycle"],
        blocksContractFreeze: true,
        status: "reviewed-source-awaiting-live-reconciliation",
      },
    ],
    blockers: [{ id: "archive", affectedContracts: [], blocksFullM1R: true }],
  });
  assert.deepEqual(findings, [
    { id: "cursor", relevant: true, resolved: false },
    { id: "archive", relevant: false, resolved: false },
  ]);
});

test("documentary review details and supersession preserve live release blockers", () => {
  const document = {
    schemaVersion: 1,
    findings: [
      {
        id: "cursor",
        status: "reviewed-source-awaiting-live-reconciliation",
        affectedContracts: ["query-locator-lifecycle"],
        blocksContractFreeze: true,
        reviewDetail: { documentaryComplete: true, supersedes: ["older"] },
      },
      {
        id: "older",
        status: "blocked",
        affectedContracts: ["query-locator-lifecycle"],
        blocksContractFreeze: true,
      },
      {
        id: "documentary",
        status: "documentary-complete",
        affectedContracts: ["query-response-shape"],
        blocksContractFreeze: true,
      },
      {
        id: "resolved",
        status: "resolved",
        affectedContracts: ["error-shape"],
        blocksContractFreeze: true,
      },
      {
        id: "reconciled",
        status: "reviewed-and-reconciled",
        affectedContracts: ["error-shape"],
        blocksContractFreeze: true,
      },
    ],
    blockers: [{ id: "archive", affectedContracts: [], blocksFullM1R: true }],
  };
  assert.deepEqual(foundationReleaseFindings(document), [
    { id: "cursor", relevant: true, resolved: false },
    { id: "older", relevant: true, resolved: false },
    { id: "documentary", relevant: true, resolved: false },
    { id: "resolved", relevant: true, resolved: true },
    { id: "reconciled", relevant: true, resolved: true },
    { id: "archive", relevant: false, resolved: false },
  ]);
});
