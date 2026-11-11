import { isDeepStrictEqual } from "node:util";
import { normalize, validateReport, type Report, type Result } from "./core.ts";
import { foundationCoverage } from "./foundation-policy.ts";
import { compareFoundationTies } from "./foundation-ties.ts";
import type {
  FoundationBinding,
  FoundationSidecar,
} from "./foundation-sidecar.ts";
import { coverageIssues } from "./foundation-assessment-observations.ts";
import {
  cleanupIssues,
  multiset,
  sha,
  sidecarIssues,
  validTime,
  type CleanupAbsenceEvidence,
  type ReleaseFinding,
} from "./foundation-assessment-evidence.ts";

export type {
  CleanupAbsenceEvidence,
  ReleaseFinding,
} from "./foundation-assessment-evidence.ts";

export interface FoundationAssessment {
  schemaVersion: 1;
  outcome: "ready-with-assumptions" | "drift" | "blocked";
  pairIssues: string[];
  releaseIssues: string[];
  tieDiagnostics: string[];
  dimensions: Record<string, "observed" | "assumed" | "differing" | "blocked">;
  captures: { sha256: string; startedAt: string; finishedAt: string }[];
  binding: FoundationBinding;
}

function withoutTies(result: Result): unknown {
  const observations = Object.fromEntries(
    Object.entries(result.observations as Record<string, any>).filter(
      ([k]) => !k.startsWith("ties:"),
    ),
  );
  return {
    client: result.client,
    outcome: result.outcome,
    reason: result.reason,
    assertions: result.assertions
      .filter((a) => !a.name.startsWith("ties:"))
      .map((a) => JSON.stringify(a))
      .sort(),
    observations: normalize(observations, result.identities),
  };
}

export function assessFoundationPair(
  firstCaptureBytes: string,
  firstSidecar: FoundationSidecar,
  secondCaptureBytes: string,
  secondSidecar: FoundationSidecar,
  currentBinding: FoundationBinding,
  cleanupEvidence: [CleanupAbsenceEvidence, CleanupAbsenceEvidence],
  releaseFindings: ReleaseFinding[],
): FoundationAssessment {
  const pairIssues: string[] = [];
  const releaseIssues = releaseFindings
    .filter((f) => f.relevant && !f.resolved)
    .map((f) => f.id);
  const dimensions: FoundationAssessment["dimensions"] = Object.fromEntries([
    ["cursor-expiry", "blocked"],
    ...Object.keys(foundationCoverage()).map((id) => [id, "blocked"]),
  ]);
  const captures: FoundationAssessment["captures"] = [];
  const reports: Report[] = [];
  for (const [i, bytes, sidecar, absence] of [
    [0, firstCaptureBytes, firstSidecar, cleanupEvidence?.[0]],
    [1, secondCaptureBytes, secondSidecar, cleanupEvidence?.[1]],
  ] as const) {
    try {
      const value: unknown = JSON.parse(bytes);
      validateReport(value);
      const report = value;
      reports.push(report);
      captures.push({
        sha256: sha(bytes),
        startedAt: report.startedAt,
        finishedAt: report.finishedAt,
      });
      if (report.target !== "reference" || report.provenance !== "live")
        pairIssues.push(`capture ${i + 1}: live reference required`);
      if (
        !validTime(report.startedAt) ||
        !validTime(report.finishedAt) ||
        Date.parse(report.startedAt) > Date.parse(report.finishedAt) ||
        Date.parse(report.startedAt) < Date.parse("2026-10-19T00:00:00.000Z")
      )
        pairIssues.push(`capture ${i + 1}: time or freshness invalid`);
      for (const key of [
        "profile",
        "profileHash",
        "apiVersion",
        "connectors",
      ] as const)
        if (!isDeepStrictEqual(report[key], currentBinding[key]))
          pairIssues.push(`capture ${i + 1}: ${key} differs`);
      pairIssues.push(
        ...sidecarIssues(bytes, report, sidecar, currentBinding).map(
          (x) => `capture ${i + 1}: ${x}`,
        ),
      );
      pairIssues.push(
        ...coverageIssues(report).map((x) => `capture ${i + 1}: ${x}`),
      );
      pairIssues.push(
        ...cleanupIssues(
          report,
          absence,
          sha(bytes),
          sidecar?.identityDigest,
        ).map((x) => `capture ${i + 1}: ${x}`),
      );
    } catch {
      pairIssues.push(`capture ${i + 1}: malformed private report`);
    }
  }
  if (
    captures.length === 2 &&
    (captures[0].sha256 === captures[1].sha256 ||
      Date.parse(captures[0].finishedAt) > Date.parse(captures[1].startedAt))
  )
    pairIssues.push("Captures are not distinct and sequential");
  if (captures.length === 2) {
    if (
      !validTime(cleanupEvidence?.[0]?.checkedAt) ||
      Date.parse(cleanupEvidence[0].checkedAt) >
        Date.parse(captures[1].startedAt)
    )
      pairIssues.push(
        "First cleanup absence was not verified before second capture",
      );
    if (firstSidecar?.identityDigest !== secondSidecar?.identityDigest)
      pairIssues.push("Capture identities differ");
  }
  const tieDiagnostics: string[] = [];
  let tiePayloadDrift = false;
  if (reports.length === 2 && pairIssues.length === 0) {
    dimensions["cursor-expiry"] = "assumed";
    for (const id of Object.keys(foundationCoverage())) {
      const a = reports[0].results.find((r) => r.id === id)!;
      const b = reports[1].results.find((r) => r.id === id)!;
      if (id === "foundation-cursors") {
        const ties = compareFoundationTies(a, b);
        for (const issue of ties.issues)
          if (issue === "Tied record payload multisets differ")
            tiePayloadDrift = true;
          else pairIssues.push(`${id}: ${issue}`);
        tieDiagnostics.push(...ties.diagnostics);
      }
      if (
        (id === "foundation-cursors" && tiePayloadDrift) ||
        !isDeepStrictEqual(withoutTies(a), withoutTies(b))
      )
        dimensions[id] = "differing";
      else dimensions[id] = "observed";
    }
  }
  const drift = Object.values(dimensions).includes("differing");
  const outcome =
    pairIssues.length || releaseIssues.length
      ? "blocked"
      : drift
        ? "drift"
        : "ready-with-assumptions";
  return {
    schemaVersion: 1,
    outcome,
    pairIssues,
    releaseIssues,
    tieDiagnostics,
    dimensions,
    captures,
    binding: currentBinding,
  };
}
