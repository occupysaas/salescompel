import { isDeepStrictEqual } from "node:util";
import {
  manifest,
  validateExternalIdReport,
  type ExternalIdReport,
} from "./external-id-contract.ts";
export function compareExternalIdReports(
  a: ExternalIdReport,
  b: ExternalIdReport,
): { outcome: "stable" | "drift" | "blocked"; differences: string[] } {
  try {
    validateExternalIdReport(a);
    validateExternalIdReport(b);
  } catch {
    return {
      outcome: "blocked",
      differences: ["Invalid or incomplete report"],
    };
  }
  if (
    !isDeepStrictEqual(a.config, b.config) ||
    !isDeepStrictEqual(a.connectors, b.connectors) ||
    a.manifestHash !== b.manifestHash ||
    a.runId === b.runId ||
    a.finishedAt > b.startedAt ||
    [a, b].some(
      (r) =>
        r.collection !== "complete" ||
        r.cleanup !== "clean" ||
        !r.identityVerified,
    )
  )
    return {
      outcome: "blocked",
      differences: [
        "Complete sequential reports with matching bindings required",
      ],
    };
  const project = (r: ExternalIdReport) => {
    function normalize(v: unknown): any {
      if (typeof v === "string") {
        let s = v.replaceAll(r.runId, "<RUN>");
        for (const alias of r.aliases)
          s = s.replaceAll(alias.id, "<" + alias.alias + ">");
        return s;
      }
      if (Array.isArray(v)) return v.map(normalize);
      if (v && typeof v === "object")
        return Object.fromEntries(
          Object.entries(v).map(([k, x]) => [k, normalize(x)]),
        );
      return v;
    }
    return manifest.map((c) => {
      const ops = r.operations
        .filter((o) => o.caseId === c.id)
        .map((o) => ({
          id: c.id.startsWith("race-") ? undefined : o.id,
          method: o.method,
          key: normalize(o.key),
          bodyKey: normalize(o.bodyKey),
          status: o.status,
          bodyPresent: o.bodyPresent,
          body: normalize(o.body),
          records: normalize(o.records).sort((x: any, y: any) =>
            JSON.stringify(x).localeCompare(JSON.stringify(y)),
          ),
        }));
      if (c.id.startsWith("race-")) {
        const sort = (items: unknown[]) =>
          items.sort((x, y) =>
            JSON.stringify(x).localeCompare(JSON.stringify(y)),
          );
        return {
          caseId: c.id,
          inputs: sort(
            ops.map((o) => ({
              method: o.method,
              key: o.key,
              bodyKey: o.bodyKey,
            })),
          ),
          responses: sort(
            ops.map((o) => ({
              status: o.status,
              bodyPresent: o.bodyPresent,
              body: o.body,
            })),
          ),
          states: sort(ops.map((o) => o.records)),
        };
      }
      return { caseId: c.id, operations: ops };
    });
  };
  const left = project(a),
    right = project(b);
  const differences = left.flatMap((c, i) =>
    isDeepStrictEqual(c, right[i])
      ? []
      : [`Observed behavior differs: ${c.caseId}`],
  );
  return { outcome: differences.length ? "drift" : "stable", differences };
}

// Use this boundary when loading reports from disk; structural comparison alone
// cannot establish that the referenced private artifacts still exist unmodified.
export async function validateExternalIdEvidence(
  report: ExternalIdReport,
  dir: string,
) {
  validateExternalIdReport(report);
  const { privateDirectory } = await import("./metadata-files.ts");
  const { privateJSON } = await import("./external-id-recovery.ts");
  const { readFile } = await import("node:fs/promises");
  const { join } = await import("node:path");
  const { createHash } = await import("node:crypto");
  await privateDirectory(dir);
  for (const op of report.operations) {
    if (op.state !== "observed") continue;
    const path = join(dir, op.artifact);
    const raw = await privateJSON(path);
    const digest = createHash("sha256")
      .update(await readFile(path))
      .digest("hex");
    if (
      digest !== op.sha256 ||
      !isDeepStrictEqual(raw, {
        status: op.status,
        bodyPresent: op.bodyPresent,
        body: op.body,
      })
    )
      throw new Error("Response artifact differs");
  }
}
