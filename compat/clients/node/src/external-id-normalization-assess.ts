import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateJSON } from "./external-id-recovery.ts";
import {
  normalizationManifest,
  type NormalizationAssessment,
  type NormalizationReport,
  type NormalizationRequest,
} from "./external-id-normalization-contract.ts";
import { normalizationDigest } from "./external-id-normalization-recovery-models.ts";
import { requireEvidence } from "./external-id-normalization-assess-validation.ts";
import { load } from "./external-id-normalization-assess-analysis.ts";
export { normalizationArtifactsDirectory } from "./external-id-normalization-assess-validation.ts";
export { validateNormalizationCleanupEvidence } from "./external-id-normalization-assess-validation.ts";
export type {
  DiscoveryBody,
  RetainedOriginal,
} from "./external-id-evidence.ts";
function projected(r: NormalizationReport) {
  const aliases = new Map<string, string>();
  for (const c of normalizationManifest) {
    let index = 0;
    const ops = c.steps.map((s) =>
      r.observations.find((o) => o.caseId === c.id && o.stepId === s.id)!,
    );
    const add = (id: string) => {
      if (!aliases.has(id)) aliases.set(id, `${c.id}:id${index++}`);
    };
    for (const o of ops) {
      const b = o.body as { id?: unknown };
      if (typeof b?.id === "string") add(b.id);
    }
    for (const o of ops)
      for (const row of [...o.before, ...o.after].sort(
        (a, b) =>
          a.Name.localeCompare(b.Name) ||
          (a.Compat_Key__c ?? "").localeCompare(b.Compat_Key__c ?? ""),
      ))
        add(row.Id);
  }
  // Alias generated prefixes in their original and uppercase forms separately:
  // retain P14 versus p14 and all suffix spelling as measured behavior.
  const visit = (v: unknown): unknown =>
    typeof v === "string"
      ? (aliases.get(v) ??
        v
          .replaceAll(`p14${r.runId}-`, "p14{run}-")
          .replaceAll(`P14${r.runId.toUpperCase()}-`, "P14{run}-"))
      : Array.isArray(v)
        ? v.map(visit)
        : v && typeof v === "object"
          ? Object.fromEntries(
              Object.entries(v).map(([k, x]) => [
                k,
                k === "message" ? "[private prose]" : visit(x),
              ]),
            )
          : v;
  return normalizationManifest.map((c) =>
    c.steps.map((step) => {
      const o = r.observations.find(
        (o) => o.caseId === c.id && o.stepId === step.id,
      )!;
      const rows = (values: typeof o.before) =>
        (values.map((row) => visit(row)) as typeof o.before).sort((a, b) =>
          a.Id.localeCompare(b.Id),
        );
      const request = visit(o.request) as NormalizationRequest;
      for (const [id, alias] of aliases)
        request.path = request.path.replaceAll(id, alias);
      return {
        stepId: o.stepId,
        request,
        status: o.status,
        bodyPresent: o.bodyPresent,
        body: visit(o.body),
        before: rows(o.before),
        after: rows(o.after),
      };
    }),
  );
}
export async function assessNormalizationPair(
  firstPath: string,
  secondPath: string,
): Promise<NormalizationAssessment> {
  const result: NormalizationAssessment = {
    contractVersion: "plan14-normalization-v1",
    outcome: "blocked",
    reasons: [],
    findings: normalizationManifest.map((c) => ({
      caseId: c.id,
      outcome: "blocked",
      reasons: ["EVIDENCE_NOT_VERIFIED"],
    })),
    inputReportDigests: [],
  };
  try {
    for (const path of [firstPath, secondPath]) {
      await privateJSON(resolve(path));
      result.inputReportDigests.push(
        normalizationDigest(await readFile(resolve(path), "utf8")),
      );
    }
    const first = await load(firstPath),
      second = await load(secondPath),
      a = first.report,
      b = second.report;
    requireEvidence(
      a.runId !== b.runId &&
        equal(a.config, b.config) &&
        a.probeHash === b.probeHash &&
        Date.parse(a.cleanup.finishedAt) < Date.parse(b.startedAt) &&
        Date.parse(a.finishedAt) <= Date.parse(b.startedAt),
    );
    const na = projected(a),
      nb = projected(b);
    result.findings = normalizationManifest.map((c, i) => {
      const outcome =
        equal(na[i], nb[i]) &&
        !first.changedCleanupCases.has(c.id) &&
        !second.changedCleanupCases.has(c.id)
          ? "repeatable"
          : "drift";
      return {
        caseId: c.id,
        outcome,
        reasons: outcome === "repeatable" ? [] : ["OBSERVATIONS_DIFFER"],
      };
    });
    result.outcome = result.findings.some((f) => f.outcome === "drift")
      ? "drift"
      : "repeatable";
  } catch {
    result.reasons.push("NORMALIZATION_EVIDENCE_INVALID_OR_INCOMPLETE");
  }
  return result;
}
