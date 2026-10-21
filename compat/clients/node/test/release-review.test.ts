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
function fixture(): any {
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

test("a completed bounded batch accepts owned source gaps and unknown dates and versions", () => {
  const f = fixture();
  assert.deepEqual(releaseReviewIssues(f.index, f.changes, ids), []);
});
test("a reviewed section retains bounded coverage and live blocking", () => {
  const f = fixture();
  Object.assign(f.batch.sources[0], {
    outcome: "reviewed",
    locator: "Query Options Header",
    coverage: "Read header table only",
    gap: null,
  });
  Object.assign(f.batch.contracts[0], {
    documentaryStatus: "reviewed",
    gaps: [],
  });
  assert.deepEqual(releaseReviewIssues(f.index, f.changes, ids), []);
});
const malformed: [string, (f: any) => void][] = [
  ["absent batch", (f) => delete f.index.reviewBatches],
  [
    "duplicate batch",
    (f) => f.index.reviewBatches.push(structuredClone(f.batch)),
  ],
  [
    "duplicate source",
    (f) => f.batch.sources.push(structuredClone(f.batch.sources[0])),
  ],
  [
    "duplicate finding",
    (f) => f.changes.findings.push(structuredClone(f.changes.blockers[0])),
  ],
  ["missing contract", (f) => f.batch.contracts.pop()],
  [
    "extra contract",
    (f) =>
      f.batch.contracts.push({
        ...f.batch.contracts[0],
        contractId: "not-registered",
      }),
  ],
  [
    "duplicate contract",
    (f) => f.batch.contracts.push(structuredClone(f.batch.contracts[0])),
  ],
  ["dangling source", (f) => f.batch.contracts[0].sourceIds.push("missing")],
  ["dangling finding", (f) => f.batch.contracts[0].findingIds.push("missing")],
  [
    "missing locator",
    (f) =>
      Object.assign(f.batch.sources[0], { outcome: "reviewed", gap: null }),
  ],
  ["empty coverage", (f) => (f.batch.sources[0].coverage = " ")],
  ["unknown enum", (f) => (f.batch.sources[0].outcome = "certified")],
  ["invalid date", (f) => (f.batch.sources[0].accessedAt = "2026-02-30")],
  ["invalid hash", (f) => (f.batch.profileHash = "not-a-hash")],
  ["missing owner", (f) => (f.batch.sources[0].gap.owner = "")],
  [
    "partial without gap",
    (f) => Object.assign(f.batch.sources[0], { outcome: "partial", gap: null }),
  ],
  [
    "unavailable checksum",
    (f) =>
      (f.batch.sources[0].checksum = {
        sha256: "a".repeat(64),
        representation: "error page",
      }),
  ],
  ["complete without date", (f) => (f.batch.completedAt = null)],
  [
    "reviewed row with gaps",
    (f) => (f.batch.contracts[0].documentaryStatus = "reviewed"),
  ],
  ["live promotion", (f) => (f.batch.contracts[0].liveStatus = "observed")],
  [
    "freeze promotion",
    (f) => (f.batch.contracts[0].blocksContractFreeze = false),
  ],
  ["whole document promotion", (f) => (f.batch.scope = "whole-document")],
  [
    "unknown version string",
    (f) => (f.batch.sources[0].apiVersion = "unknown"),
  ],
  [
    "unknown date string",
    (f) => (f.batch.sources[0].publicationDate = "unknown"),
  ],
  [
    "lookalike host",
    (f) => (f.batch.sources[0].url = "https://salesforce.com.example.org/page"),
  ],
  [
    "no source or gap",
    (f) =>
      Object.assign(f.batch.contracts[0], {
        sourceIds: [],
        findingIds: [],
        gaps: [],
      }),
  ],
  ["empty regression", (f) => (f.batch.contracts[0].regressions = [])],
  [
    "proposed claims file",
    (f) => (f.batch.contracts[0].regressions[0].path = "internal/test.go"),
  ],
  [
    "dangling regression",
    (f) =>
      (f.changes.blockers[0].reviewDetail = {
        ...detail(),
        regressionIds: ["missing"],
      }),
  ],
  [
    "dangling supersession",
    (f) =>
      (f.changes.blockers[0].reviewDetail = {
        ...detail(),
        supersedes: ["missing"],
      }),
  ],
  [
    "self supersession",
    (f) =>
      (f.changes.blockers[0].reviewDetail = {
        ...detail(),
        supersedes: ["gap"],
      }),
  ],
  [
    "cyclic supersession",
    (f) => {
      f.changes.blockers[0].reviewDetail = {
        ...detail(),
        supersedes: ["other"],
      };
      f.changes.findings.push({
        id: "other",
        affectedContracts: ids,
        status: "blocked",
        blocksContractFreeze: true,
        reviewDetail: { ...detail(), supersedes: ["gap"] },
      });
    },
  ],
];
for (const [name, mutate] of malformed)
  test(`rejects ${name}`, () => {
    const f = fixture();
    mutate(f);
    const issues = releaseReviewIssues(f.index, f.changes, ids);
    assert.ok(issues.length, name);
    assert.deepEqual(issues, [...issues].sort());
  });
test("malformed unknown input returns diagnostics without throwing", () => {
  for (const value of [
    null,
    4,
    [],
    {},
    { schemaVersion: 1, reviewBatches: [null] },
  ])
    assert.ok(releaseReviewIssues(value, null, ids).length);
});
async function repository(t: any) {
  const root = await mkdtemp(join(tmpdir(), "release-review-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  for (const p of ["releases", "profiles", "contracts"])
    await mkdir(join(root, "docs/compat", p), { recursive: true });
  const f = fixture();
  for (const [path, key] of [
    ["profiles/rest-query-foundation.yaml", "profileHash"],
    ["contracts/rest-query-foundation.yaml", "contractHash"],
  ]) {
    const bytes = Buffer.from("synthetic protected source\n");
    await writeFile(join(root, "docs/compat", path), bytes);
    f.batch[key] = createHash("sha256").update(bytes).digest("hex");
  }
  const save = async () => {
    await writeFile(
      join(root, "docs/compat/releases/index.yaml"),
      stringify(f.index),
    );
    await writeFile(
      join(root, "docs/compat/releases/changes.yaml"),
      stringify(f.changes),
    );
  };
  await save();
  return { root, f, save };
}
test("offline wrapper never fetches sources or opens provenance locators", async (t) => {
  const { root, f, save } = await repository(t);
  Object.assign(f.batch.sources[0], {
    outcome: "reviewed",
    locator: "/nonexistent/private/source",
    gap: null,
  });
  await save();
  const oldFetch = globalThis.fetch;
  globalThis.fetch = async () => {
    throw new Error("network forbidden");
  };
  t.after(() => (globalThis.fetch = oldFetch));
  assert.deepEqual(await validateReleaseReviewRepository(root), []);
});
test("wrapper validates existing tests and rejects missing or unsafe paths", async (t) => {
  const { root, f, save } = await repository(t);
  await mkdir(join(root, "compat/clients/node/test"), { recursive: true });
  await writeFile(
    join(root, "compat/clients/node/test/real.test.ts"),
    "// synthetic\n",
  );
  const r = f.batch.contracts[0].regressions[0];
  Object.assign(r, {
    status: "existing",
    path: "compat/clients/node/test/real.test.ts",
    testName: "verified by reviewer",
  });
  await save();
  assert.deepEqual(await validateReleaseReviewRepository(root), []);
  for (const path of [
    "compat/clients/node/test/missing.test.ts",
    "internal/../../outside",
    "/tmp/outside",
    "web/test/%2e%2e/file",
    "web\\test\\file",
    "https://example.org/test",
  ]) {
    r.path = path;
    await save();
    assert.ok((await validateReleaseReviewRepository(root)).length, path);
  }
  await symlink(
    join(root, "compat/clients/node/test/real.test.ts"),
    join(root, "compat/clients/node/test/link.test.ts"),
  );
  r.path = "compat/clients/node/test/link.test.ts";
  await save();
  assert.ok((await validateReleaseReviewRepository(root)).length);
  await mkdir(join(root, "internal"));
  await symlink(
    join(root, "compat/clients/node/test"),
    join(root, "internal/link"),
  );
  r.path = "internal/link/real.test.ts";
  await save();
  assert.ok((await validateReleaseReviewRepository(root)).length);
});
test("wrapper detects protected hash drift and malformed or absent ledgers", async (t) => {
  const { root, f, save } = await repository(t);
  f.batch.profileHash = "0".repeat(64);
  await save();
  assert.ok((await validateReleaseReviewRepository(root)).length);
  await writeFile(
    join(root, "docs/compat/releases/index.yaml"),
    "duplicate: 1\nduplicate: 2\n",
  );
  assert.ok((await validateReleaseReviewRepository(root)).length);
  await rm(join(root, "docs/compat/releases/index.yaml"));
  assert.ok((await validateReleaseReviewRepository(root)).length);
});
