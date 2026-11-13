import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateJSON } from "./external-id-recovery.ts";
import {
  bodyManifest,
  bodyScope,
  type BodyAssessment,
  type BodyReport,
} from "./external-id-body-contract.ts";
import { bodyDigest } from "./external-id-body-recovery-models.ts";
import { requireEvidence } from "./external-id-body-assess-validation.ts";
import { load } from "./external-id-body-assess-analysis.ts";
export { bodyArtifactsDirectory } from "./external-id-body-assess-validation.ts";
export { validateBodyCleanupEvidence } from "./external-id-body-assess-validation.ts";
export type {
  DiscoveryBody,
  RetainedOriginal,
} from "./external-id-evidence.ts";
function expected(
  r: BodyReport,
  caseId: string,
  changedCleanup: boolean,
): "ready" | "different" | "blocked" {
  const o = r.observations.find((o) => o.caseId === caseId)!,
    c = bodyManifest.find((c) => c.id === caseId)!,
    s = bodyScope(r.runId, c),
    b = o.body;
  if (c.valueKind !== "omitted" && c.valueKind !== "null") {
    if (o.status !== 400) return "different";
    const errors = b as
      Array<{ errorCode?: unknown; message?: unknown }> | null | undefined;
    if (
      !o.bodyPresent ||
      !Array.isArray(errors) ||
      errors.length !== 1 ||
      !errors[0] ||
      typeof errors[0] !== "object" ||
      Array.isArray(errors[0]) ||
      Object.keys(errors[0]).sort().join() !== "errorCode,message" ||
      typeof errors[0].errorCode !== "string" ||
      typeof errors[0].message !== "string"
    )
      return "blocked";
    return errors[0].errorCode ===
      (c.valueKind === "object" ? "JSON_PARSER_ERROR" : "INVALID_FIELD") &&
      equal(o.before, o.after) &&
      !changedCleanup
      ? "ready"
      : "different";
  }
  const m = b as {
    id?: unknown;
    success?: unknown;
    created?: unknown;
    errors?: unknown;
  } | null;
  if (o.status !== (c.recordState === "existing" ? 200 : 201))
    return "different";
  if (
    !o.bodyPresent ||
    !m ||
    typeof m.id !== "string" ||
    typeof m.success !== "boolean" ||
    typeof m.created !== "boolean" ||
    !Array.isArray(m.errors)
  )
    return "blocked";
  return !changedCleanup &&
    m.success === true &&
    m.created === (c.recordState === "missing") &&
    m.errors.length === 0 &&
    o.after.length === 1 &&
    o.after[0].Id === m.id &&
    (c.recordState === "missing" || m.id === o.before[0].Id) &&
    o.after[0].Name === s.requestName &&
    o.after[0].Compat_Key__c === s.pathKey &&
    o.after[0].Compat_Text__c === (o.before[0]?.Compat_Text__c ?? null)
    ? "ready"
    : "different";
}
function normalized(r: BodyReport) {
  const aliases = new Map<string, string>();
  for (const c of bodyManifest) {
    const o = r.observations.find((o) => o.caseId === c.id)!;
    let n = 0;
    for (const row of [...o.before, ...o.after])
      if (!aliases.has(row.Id)) aliases.set(row.Id, `${c.id}:id${n++}`);
  }
  const visit = (v: unknown): unknown =>
    typeof v === "string"
      ? (aliases.get(v) ?? v.replaceAll(`p11${r.runId}-`, "p11{run}-"))
      : Array.isArray(v)
        ? v.map(visit)
        : v && typeof v === "object"
          ? Object.fromEntries(Object.entries(v).map(([k, x]) => [k, visit(x)]))
          : v;
  return r.observations.map((o) =>
    visit({
      caseId: o.caseId,
      status: o.status,
      bodyPresent: o.bodyPresent,
      body: o.body,
      before: o.before,
      after: o.after,
    }),
  );
}
export async function assessBodyPair(
  firstPath: string,
  secondPath: string,
): Promise<BodyAssessment> {
  const result: BodyAssessment = {
    contractVersion: "plan11-observed-body-validation-v2",
    outcome: "blocked",
    reasons: [],
    findings: [],
    inputReportDigests: [],
  };
  try {
    for (const path of [firstPath, secondPath]) {
      await privateJSON(resolve(path));
      result.inputReportDigests.push(
        bodyDigest(await readFile(resolve(path), "utf8")),
      );
    }
    const first = await load(firstPath),
      second = await load(secondPath);
    const a = first.report,
      b = second.report;
    requireEvidence(
      a.runId !== b.runId &&
        equal(a.config, b.config) &&
        Date.parse(a.cleanup.finishedAt) < Date.parse(b.startedAt) &&
        Date.parse(a.finishedAt) <= Date.parse(b.startedAt),
    );
    const na = normalized(a),
      nb = normalized(b);
    result.findings = bodyManifest.map((c, i) => {
      const outcomes = [
        expected(a, c.id, first.changedCleanupCases.has(c.id)),
        expected(b, c.id, second.changedCleanupCases.has(c.id)),
      ];
      const outcome = outcomes.includes("blocked")
        ? "blocked"
        : outcomes.includes("different") || !equal(na[i], nb[i])
          ? "different"
          : "ready";
      return {
        caseId: c.id,
        outcome,
        reasons:
          outcome === "ready"
            ? []
            : [
                outcome === "blocked"
                  ? "MALFORMED_RESPONSE"
                  : "UNEXPECTED_OR_UNREPEATABLE_BEHAVIOR",
              ],
      };
    });
    result.outcome = result.findings.some((f) => f.outcome === "blocked")
      ? "blocked"
      : result.findings.some((f) => f.outcome === "different")
        ? "different"
        : "ready";
  } catch {
    result.reasons.push("BODY_EVIDENCE_INVALID_OR_INCOMPLETE");
  }
  return result;
}
