import { parsePackageManifest } from "./package-feasibility-parse.ts";
import { resolvePackageManifest } from "./package-feasibility-resolve.ts";
import type {
  PackageAssessment,
  PackageBlocker,
} from "./package-feasibility-model.ts";

const blockerKey = (b: PackageBlocker) =>
  [b.packageId, b.code, "requirementId" in b ? b.requirementId : ""].join("\0");

/** Synthetic dependency assessment only. No package is certified or executed. */
export function assessPackageFeasibility(bytes: Uint8Array): PackageAssessment {
  const parsed = parsePackageManifest(bytes);
  const base = {
    schemaVersion: 1 as const,
    classification: "synthetic" as const,
    execution: "unproven" as const,
    inputDigest: parsed.inputDigest,
  };
  if (!parsed.ok)
    return {
      ...base,
      validation: "invalid",
      issues: parsed.issues,
      closure: [],
      bindings: [],
      blockers: [],
    };
  const resolved = resolvePackageManifest(parsed.manifest);
  if (resolved.validation === "invalid")
    return { ...base, ...resolved, blockers: [] };
  const packages = new Map(parsed.manifest.packages.map((p) => [p.id, p]));
  const blockers: PackageBlocker[] = [];
  for (const { packageId } of resolved.closure) {
    const p = packages.get(packageId)!;
    blockers.push({
      code: "real-target-unverified",
      packageId,
      ownerId: "maintainer",
      nextAction: "select-target-and-collect-evidence",
    });
    if (p.kind === "unknown")
      blockers.push({
        code: "package-kind-unknown",
        packageId,
        ownerId: p.ownerId,
        nextAction: "collect-package-details",
      });
    if (p.source === "unavailable")
      blockers.push({
        code: "source-unavailable",
        packageId,
        ownerId: p.ownerId,
        nextAction: "establish-permitted-execution-strategy",
      });
    if (p.source === "unknown")
      blockers.push({
        code: "source-unknown",
        packageId,
        ownerId: p.ownerId,
        nextAction: "collect-source-availability",
      });
    if (p.dependencyKnowledge === "unknown")
      blockers.push({
        code: "dependency-inventory-unknown",
        packageId,
        ownerId: p.ownerId,
        nextAction: "collect-dependency-inventory",
      });
    for (const r of p.requirements) {
      const detail = { packageId, requirementId: r.id, surface: r.surface };
      if (r.evidence === "target-verified")
        blockers.push({
          ...detail,
          code: "synthetic-evidence-claim",
          ownerId: "maintainer",
          nextAction: "collect-real-evidence",
          evidence: r.evidence,
        });
      else
        blockers.push({
          ...detail,
          code: "surface-unverified",
          ownerId: "platform-implementer",
          nextAction: "design-target-surface-test",
          evidence: r.evidence,
        });
    }
  }
  return {
    ...base,
    ...resolved,
    blockers: [
      ...new Map(blockers.map((b) => [blockerKey(b), b])).values(),
    ].sort((a, b) =>
      blockerKey(a) < blockerKey(b)
        ? -1
        : blockerKey(a) > blockerKey(b)
          ? 1
          : 0,
    ),
  };
}
