import assert from "node:assert/strict";
import test from "node:test";
import { mkdtemp, mkdir, writeFile, rm, symlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { createHash } from "node:crypto";
import { stringify } from "yaml";
import { suite } from "../src/suites.ts";
import {
  releaseReviewIssues,
  validateReleaseReviewRepository,
} from "../src/release-review.ts";
const ids = suite("rest-query-foundation").contractIds;
const gap = () => ({
  owner: "platform-implementer",
  reason: "Historical body unavailable",
  nextAction: "Retrieve pinned source",
});
function fixture(): Record<string, any> {
  const source = {
    id: "history",
    url: "https://help.salesforce.com/release",
    kind: "release-notes",
    release: null,
    apiVersion: null,
    publicationDate: null,
    accessedAt: "2026-10-21",
    reviewer: "platform-implementer",
    outcome: "unavailable",
    locator: null,
    coverage: "No body read; section review unavailable",
    attempts: ["Public web returned shell"],
    checksum: null,
    gap: gap(),
  };
  const batch = {
    id: "plan-18-foundation",
    profile: "rest-query-foundation",
    apiVersion: "65.0",
    profileHash: "a".repeat(64),
    contractHash: "b".repeat(64),
    completedAt: "2026-10-21",
    status: "complete",
    scope: "foundation-sections",
    owner: "platform-implementer",
    sources: [source],
    contracts: ids.map((contractId) => ({
      contractId,
      sourceIds: ["history"],
      findingIds: ["gap"],
      documentaryStatus: "blocked",
      expectation: "Which version-sensitive behavior is required?",
      coverage: "Historical section review blocked",
      gaps: [gap()],
      liveStatus: "blocked",
      blocksContractFreeze: true,
      owner: "platform-implementer",
      nextAction: "Collect accepted pair under Plan 10",
      regressions: [
        {
          id: `${contractId}-regression`,
          status: "proposed",
          path: null,
          testName: null,
          operation: "Run contract operation twice",
          assertion:
            "Compare wire outcome and independent state against accepted evidence",
          evidencePrerequisite: "Accepted Plan 10 pair",
        },
      ],
    })),
  };
  return {
    index: { schemaVersion: 1, reviewBatches: [batch] },
    changes: {
      schemaVersion: 1,
      findings: [],
      blockers: [
        {
          id: "gap",
          status: "blocked",
          affectedContracts: ids,
          blocksContractFreeze: true,
        },
      ],
    },
    batch,
  };
}
function detail() {
  return {
    batchId: "plan-18-foundation",
    sourceIds: ["history"],
    introducedApiVersion: null,
    effectiveApiVersion: null,
    activation: "unknown",
    applicability: "in-profile",
    rationale: "Required foundation concern",
    sourceStatement: "Source unavailable",
    inference: "No semantic conclusion",
    unresolvedQuestions: [gap()],
    deprecation: null,
    migrationImplications: null,
    regressionIds: ["query-response-shape-regression"],
    supersedes: [],
  };
}

const findingBypasses: [string, (finding: Record<string, unknown>) => void][] =
  [
    ["resolved", (f) => (f.status = "resolved")],
    ["reconciled", (f) => (f.status = "reviewed-and-reconciled")],
    ["blocking false", (f) => (f.blocksContractFreeze = false)],
    ["blocking absent", (f) => delete f.blocksContractFreeze],
    ["blocking string", (f) => (f.blocksContractFreeze = "true")],
    ["contracts empty", (f) => (f.affectedContracts = [])],
    ["contracts absent", (f) => delete f.affectedContracts],
  ];
for (const [name, mutate] of findingBypasses)
  test(`unresolved in-profile finding rejects ${name} assessor bypass`, () => {
    const f = fixture();
    f.changes.blockers[0].reviewDetail = detail();
    assert.deepEqual(releaseReviewIssues(f.index, f.changes, ids), []);
    mutate(f.changes.blockers[0]);
    assert.ok(releaseReviewIssues(f.index, f.changes, ids).length);
  });

test("legacy reconciled findings retain legitimate resolution without Plan 18 detail", () => {
  const f = fixture();
  f.changes.findings.push({
    id: "legacy-reconciled",
    status: "reviewed-and-reconciled",
    affectedContracts: ["query-response-shape"],
    blocksContractFreeze: true,
  });
  assert.deepEqual(releaseReviewIssues(f.index, f.changes, ids), []);
});
