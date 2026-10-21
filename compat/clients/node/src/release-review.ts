import { Ajv } from "ajv";
import { createHash } from "node:crypto";
import { lstat, readFile } from "node:fs/promises";
import { join } from "node:path";
import { parseDocument } from "./core.ts";
import { suite } from "./suites.ts";

export interface ReviewGap {
  owner: string;
  reason: string;
  nextAction: string;
}
export interface ReviewSource {
  id: string;
  url: string;
  kind:
    | "release-notes"
    | "current-guide"
    | "dated-article"
    | "changelog"
    | "release-update";
  release: string | null;
  apiVersion: string | null;
  publicationDate: string | null;
  accessedAt: string;
  reviewer: string;
  outcome: "reviewed" | "partial" | "unavailable";
  locator: string | null;
  coverage: string;
  attempts: string[];
  checksum: { sha256: string; representation: string } | null;
  gap: ReviewGap | null;
}
export interface ReviewRegression {
  id: string;
  status: "existing" | "proposed";
  path: string | null;
  testName: string | null;
  operation: string;
  assertion: string;
  evidencePrerequisite: string;
}
export interface ReviewContract {
  contractId: string;
  sourceIds: string[];
  findingIds: string[];
  documentaryStatus: "reviewed" | "partial" | "blocked";
  expectation: string;
  coverage: string;
  gaps: ReviewGap[];
  liveStatus: "blocked" | "assumed";
  blocksContractFreeze: true;
  owner: string;
  nextAction: string;
  regressions: ReviewRegression[];
}
export interface ReviewBatch {
  id: string;
  profile: "rest-query-foundation";
  apiVersion: "65.0";
  profileHash: string;
  contractHash: string;
  completedAt: string | null;
  status: "partial" | "complete";
  scope: "foundation-sections";
  owner: string;
  sources: ReviewSource[];
  contracts: ReviewContract[];
}
export interface ReviewDetail {
  batchId: string;
  sourceIds: string[];
  introducedApiVersion: string | null;
  effectiveApiVersion: string | null;
  activation: "opt-in" | "enforced" | "default" | "unknown";
  applicability: "in-profile" | "out-of-profile" | "unknown";
  rationale: string;
  sourceStatement: string;
  inference: string;
  unresolvedQuestions: ReviewGap[];
  deprecation: string | null;
  migrationImplications: string | null;
  regressionIds: string[];
  supersedes: string[];
}
type Finding = {
  id: string;
  affectedContracts?: string[];
  reviewDetail?: ReviewDetail;
};
const text = { type: "string", pattern: "\\S" };
const hash = { type: "string", pattern: "^[a-f0-9]{64}$" };
const date = { type: "string", format: "calendar-date" };
const version = { type: "string", pattern: "^\\d+\\.0$" };
const nullable = (schema: object) => ({ anyOf: [schema, { type: "null" }] });
const array = (items: object, minItems = 0) => ({
  type: "array",
  items,
  minItems,
});
const strings = array(text);
const enumeration = (...values: string[]) => ({ enum: values });
const object = (properties: Record<string, object>) => ({
  type: "object",
  properties,
  required: Object.keys(properties),
  additionalProperties: false,
});
const gapSchema = object({ owner: text, reason: text, nextAction: text });
const regressionSchema = object({
  id: text,
  status: enumeration("existing", "proposed"),
  path: nullable(text),
  testName: nullable(text),
  operation: text,
  assertion: text,
  evidencePrerequisite: text,
});
const sourceSchema = object({
  id: text,
  url: { type: "string", format: "official-url" },
  kind: enumeration(
    "release-notes",
    "current-guide",
    "dated-article",
    "changelog",
    "release-update",
  ),
  release: nullable(text),
  apiVersion: nullable(version),
  publicationDate: nullable(date),
  accessedAt: date,
  reviewer: text,
  outcome: enumeration("reviewed", "partial", "unavailable"),
  locator: nullable(text),
  coverage: text,
  attempts: array(text, 1),
  checksum: nullable(object({ sha256: hash, representation: text })),
  gap: nullable(gapSchema),
});
const contractSchema = object({
  contractId: text,
  sourceIds: strings,
  findingIds: strings,
  documentaryStatus: enumeration("reviewed", "partial", "blocked"),
  expectation: text,
  coverage: text,
  gaps: array(gapSchema),
  liveStatus: enumeration("blocked", "assumed"),
  blocksContractFreeze: { const: true },
  owner: text,
  nextAction: text,
  regressions: array(regressionSchema, 1),
});
const batchSchema = object({
  id: text,
  profile: { const: "rest-query-foundation" },
  apiVersion: { const: "65.0" },
  profileHash: hash,
  contractHash: hash,
  completedAt: nullable(date),
  status: enumeration("partial", "complete"),
  scope: { const: "foundation-sections" },
  owner: text,
  sources: array(sourceSchema, 1),
  contracts: array(contractSchema, 1),
});
const detailSchema = object({
  batchId: text,
  sourceIds: strings,
  introducedApiVersion: nullable(version),
  effectiveApiVersion: nullable(version),
  activation: enumeration("opt-in", "enforced", "default", "unknown"),
  applicability: enumeration("in-profile", "out-of-profile", "unknown"),
  rationale: text,
  sourceStatement: text,
  inference: text,
  unresolvedQuestions: array(gapSchema),
  deprecation: nullable(text),
  migrationImplications: nullable(text),
  regressionIds: strings,
  supersedes: strings,
});
const ajv = new Ajv({ allErrors: true });
ajv.addFormat(
  "calendar-date",
  (value: string) =>
    /^\d{4}-\d{2}-\d{2}$/.test(value) &&
    Number.isFinite(Date.parse(value)) &&
    new Date(value).toISOString().slice(0, 10) === value,
);
ajv.addFormat("official-url", (value: string) => {
  try {
    const u = new URL(value);
    return (
      u.protocol === "https:" &&
      !u.username &&
      !u.password &&
      !u.port &&
      (u.hostname === "salesforce.com" ||
        u.hostname.endsWith(".salesforce.com"))
    );
  } catch {
    return false;
  }
});
const checkBatch = ajv.compile<ReviewBatch>(batchSchema);
const checkDetail = ajv.compile<ReviewDetail>(detailSchema);
function record(value: unknown): value is Record<string, unknown> {
  return typeof value === "object" && value !== null && !Array.isArray(value);
}

/** Checks repository-authored metadata, never the truth of a source summary. */
export function releaseReviewIssues(
  index: unknown,
  changes: unknown,
  contractIds: readonly string[],
): string[] {
  const issues: string[] = [];
  const issue = (message: string) => issues.push(`Release review: ${message}`);
  if (
    !record(index) ||
    index.schemaVersion !== 1 ||
    !Array.isArray(index.reviewBatches) ||
    !index.reviewBatches.length ||
    !record(changes) ||
    changes.schemaVersion !== 1 ||
    !Array.isArray(changes.findings) ||
    !Array.isArray(changes.blockers)
  )
    return ["Release review: unavailable or malformed ledgers/batch"];
  const batches: ReviewBatch[] = [];
  for (const value of index.reviewBatches) {
    if (!checkBatch(value)) issue("malformed review batch");
    else batches.push(value);
  }
  const unique = (values: string[], label: string) => {
    if (new Set(values).size !== values.length) issue(`duplicate ${label}`);
  };
  unique(
    batches.map((b) => b.id),
    "batch IDs",
  );
  if (!batches.some((b) => b.id === "plan-18-foundation"))
    issue("missing Plan 18 batch");
  const findings: Finding[] = [];
  for (const item of [...changes.findings, ...changes.blockers]) {
    if (
      !record(item) ||
      typeof item.id !== "string" ||
      !item.id.trim() ||
      (item.affectedContracts !== undefined &&
        (!Array.isArray(item.affectedContracts) ||
          item.affectedContracts.some((c: unknown) => typeof c !== "string")))
    ) {
      issue("malformed finding");
      continue;
    }
    if (item.reviewDetail !== undefined && !checkDetail(item.reviewDetail)) {
      issue(`malformed review detail ${item.id}`);
      continue;
    }
    findings.push(item as Finding);
  }
  unique(
    findings.map((f) => f.id),
    "finding IDs",
  );
  const findingMap = new Map(findings.map((f) => [f.id, f]));
  const batchMap = new Map(batches.map((b) => [b.id, b]));
  function refs(values: string[], allowed: Set<string>, label: string) {
    unique(values, label);
    for (const value of values)
      if (!allowed.has(value)) issue(`dangling ${label}: ${value}`);
  }
  for (const b of batches) {
    if (b.status === "complete" && b.completedAt === null)
      issue(`complete batch lacks date: ${b.id}`);
    unique(
      b.sources.map((s) => s.id),
      "source IDs",
    );
    const sourceMap = new Map(b.sources.map((s) => [s.id, s]));
    unique(
      b.contracts.map((c) => c.contractId),
      "contract IDs",
    );
    const actual = new Set(b.contracts.map((c) => c.contractId));
    for (const id of contractIds)
      if (!actual.has(id)) issue(`missing contract: ${id}`);
    for (const id of actual)
      if (!contractIds.includes(id)) issue(`unregistered contract: ${id}`);
    unique(
      b.contracts.flatMap((c) => c.regressions.map((r) => r.id)),
      "regression IDs",
    );
    for (const s of b.sources) {
      if (s.outcome === "reviewed" && (s.locator === null || s.gap !== null))
        issue(`reviewed source needs locator and no gap: ${s.id}`);
      if (s.outcome !== "reviewed" && s.gap === null)
        issue(`incomplete source lacks gap: ${s.id}`);
      if (
        s.outcome === "unavailable" &&
        (s.checksum !== null || s.locator !== null)
      )
        issue(`unavailable source claims content: ${s.id}`);
    }
    for (const c of b.contracts) {
      refs(c.sourceIds, new Set(sourceMap.keys()), "contract source");
      refs(c.findingIds, new Set(findingMap.keys()), "contract finding");
      if (!c.sourceIds.length && !c.findingIds.length && !c.gaps.length)
        issue(`contract lacks basis: ${c.contractId}`);
      if (
        c.documentaryStatus === "reviewed" &&
        (c.gaps.length ||
          !c.sourceIds.length ||
          c.sourceIds.some((id) => sourceMap.get(id)?.outcome !== "reviewed"))
      )
        issue(`reviewed contract has incomplete coverage: ${c.contractId}`);
      if (c.documentaryStatus !== "reviewed" && !c.gaps.length)
        issue(`incomplete contract lacks gap: ${c.contractId}`);
      for (const r of c.regressions) {
        if (
          r.status === "proposed"
            ? r.path !== null || r.testName !== null
            : r.path === null || r.testName === null
        )
          issue(`regression status/path mismatch: ${r.id}`);
      }
    }
  }
  for (const f of findings) {
    const d = f.reviewDetail;
    if (!d) continue;
    const b = batchMap.get(d.batchId);
    if (!b) issue(`dangling detail batch: ${f.id}`);
    else {
      refs(d.sourceIds, new Set(b.sources.map((s) => s.id)), "detail source");
      refs(
        d.regressionIds,
        new Set(b.contracts.flatMap((c) => c.regressions.map((r) => r.id))),
        "detail regression",
      );
      refs(f.affectedContracts ?? [], new Set(contractIds), "detail contract");
      if (
        !d.sourceIds.length ||
        (d.applicability === "in-profile" && !d.regressionIds.length)
      )
        issue(`detail lacks source/regression: ${f.id}`);
    }
    refs(d.supersedes, new Set(findingMap.keys()), "supersession");
  }
  const visiting = new Set<string>(),
    visited = new Set<string>();
  function visit(id: string) {
    if (visiting.has(id)) {
      issue(`cyclic supersession: ${id}`);
      return;
    }
    if (visited.has(id)) return;
    visiting.add(id);
    for (const next of findingMap.get(id)?.reviewDetail?.supersedes ?? [])
      visit(next);
    visiting.delete(id);
    visited.add(id);
  }
  for (const id of findingMap.keys()) visit(id);
  return [...new Set(issues)].sort();
}

async function existingTest(root: string, path: string) {
  if (
    !/^(compat\/clients\/node\/test\/|internal\/|web\/test\/)/.test(path) ||
    /[\\%:#?\x00]/.test(path) ||
    path.split("/").some((p) => !p || p === "." || p === "..")
  )
    return false;
  let current = root;
  const parts = path.split("/");
  for (let i = 0; i < parts.length; i++) {
    current = join(current, parts[i]);
    const s = await lstat(current);
    if (
      s.isSymbolicLink() ||
      (i === parts.length - 1 ? !s.isFile() : !s.isDirectory())
    )
      return false;
  }
  return true;
}

export async function validateReleaseReviewRepository(
  root: string,
): Promise<string[]> {
  try {
    const index = parseDocument(
      await readFile(join(root, "docs/compat/releases/index.yaml"), "utf8"),
    );
    const changes = parseDocument(
      await readFile(join(root, "docs/compat/releases/changes.yaml"), "utf8"),
    );
    const issues = releaseReviewIssues(
      index,
      changes,
      suite("rest-query-foundation").contractIds,
    );
    if (issues.length) return issues;
    for (const b of index.reviewBatches as ReviewBatch[]) {
      for (const [path, expected] of [
        ["docs/compat/profiles/rest-query-foundation.yaml", b.profileHash],
        ["docs/compat/contracts/rest-query-foundation.yaml", b.contractHash],
      ]) {
        if (
          createHash("sha256")
            .update(await readFile(join(root, path)))
            .digest("hex") !== expected
        )
          issues.push(`Release review: protected hash mismatch: ${path}`);
      }
      for (const c of b.contracts)
        for (const r of c.regressions)
          if (r.status === "existing") {
            try {
              if (!(await existingTest(root, r.path!)))
                issues.push(`Release review: unsafe/missing test: ${r.id}`);
            } catch {
              issues.push(`Release review: unsafe/missing test: ${r.id}`);
            }
          }
    }
    return [...new Set(issues)].sort();
  } catch {
    return ["Release review: unavailable or malformed repository inputs"];
  }
}
