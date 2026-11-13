import { createHash } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import type { Report } from "./core.ts";
import type {
  FoundationBinding,
  FoundationSidecar,
} from "./foundation-sidecar.ts";

export interface CleanupAbsenceEvidence {
  schemaVersion: 1;
  captureSha256: string;
  checkedAt: string;
  identityVerified: boolean;
  identityDigest: string;
  unresolvedJournals: number;
  resources: { resource: string; absent: boolean }[];
}
export interface ReleaseFinding {
  id: string;
  relevant: boolean;
  resolved: boolean;
}

const source =
  "https://developer.salesforce.com/docs/platform/salesforce-app-limits-cheatsheet/guide/salesforce-app-limits-platform-apicursors.html";
const resourceId = /^(Account|Contact):[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?$/;

export const sha = (text: string) =>
  createHash("sha256").update(text).digest("hex");

export function validTime(value: unknown): value is string {
  return (
    typeof value === "string" &&
    !Number.isNaN(Date.parse(value)) &&
    new Date(value).toISOString() === value
  );
}

export function multiset(values: string[]): string[] {
  return [...values].sort();
}

function ownedResources(
  report: Report,
): { scenario: string; resource: string }[] {
  return report.results.flatMap((result) =>
    Object.entries(result.identities)
      .filter(
        ([key]) =>
          !/^(value:|url:|short:|locator:|comparison:)/.test(key) &&
          key !== "external",
      )
      .map(([key, id]) => ({
        scenario: result.id,
        resource: `${key === "contact" || key === "id-contact" ? "Contact" : "Account"}:${id}`,
      })),
  );
}

export function sidecarIssues(
  bytes: string,
  report: Report,
  sidecar: FoundationSidecar,
  binding: FoundationBinding,
): string[] {
  const issues: string[] = [];
  if (!sidecar || typeof sidecar !== "object")
    return ["Missing policy sidecar"];
  const keys = [
    "schemaVersion",
    "profile",
    "profileHash",
    "apiVersion",
    "connectors",
    "probeDigest",
    "manifestDigest",
    "policyVersion",
    "policy",
    "assumptionId",
    "source",
    "captureSha256",
    "identityDigest",
    "startedAt",
    "finishedAt",
    "publishedAt",
  ];
  if (!isDeepStrictEqual(multiset(Object.keys(sidecar)), multiset(keys)))
    issues.push("Malformed policy sidecar fields");
  if (
    sidecar.schemaVersion !== 1 ||
    sidecar.policyVersion !== 1 ||
    sidecar.policy !== "documented-two-day-assumption" ||
    sidecar.assumptionId !== "foundation-query-result-lifetime" ||
    sidecar.source !== source
  )
    issues.push("Wrong policy or assumption binding");
  for (const key of [
    "profile",
    "profileHash",
    "apiVersion",
    "connectors",
    "probeDigest",
    "manifestDigest",
  ] as const)
    if (!isDeepStrictEqual(sidecar[key], binding[key]))
      issues.push(`Policy ${key} differs from current binding`);
  if (
    sidecar.captureSha256 !== sha(bytes) ||
    !/^[0-9a-f]{64}$/.test(sidecar.identityDigest) ||
    sidecar.startedAt !== report.startedAt ||
    sidecar.finishedAt !== report.finishedAt
  )
    issues.push("Policy capture digest or times differ");
  if (
    !validTime(sidecar.publishedAt) ||
    Date.parse(sidecar.publishedAt) < Date.parse(report.finishedAt)
  )
    issues.push("Policy publication time invalid");
  return issues;
}

export function cleanupIssues(
  report: Report,
  evidence: CleanupAbsenceEvidence,
  digest: string,
  identityDigest: string,
): string[] {
  if (
    !evidence ||
    evidence.schemaVersion !== 1 ||
    evidence.captureSha256 !== digest ||
    evidence.identityDigest !== identityDigest ||
    !evidence.identityVerified ||
    evidence.unresolvedJournals !== 0 ||
    !validTime(evidence.checkedAt) ||
    Date.parse(evidence.checkedAt) < Date.parse(report.finishedAt) ||
    !Array.isArray(evidence.resources)
  )
    return ["Cleanup absence evidence missing or unbound"];
  const expected = report.cleanup.map((c) => c.resource);
  const owned = ownedResources(report);
  if (
    report.cleanup.some((c) => c.outcome !== "passed") ||
    owned.some(({ resource }) => !resourceId.test(resource)) ||
    !isDeepStrictEqual(
      multiset(owned.map((item) => `${item.scenario}|${item.resource}`)),
      multiset(
        report.cleanup.map((item) => `${item.scenario}|${item.resource}`),
      ),
    ) ||
    new Set(expected).size !== expected.length ||
    !isDeepStrictEqual(
      multiset(expected),
      multiset(evidence.resources.map((r) => r.resource)),
    ) ||
    evidence.resources.some((r) => r.absent !== true)
  )
    return ["Cleanup or exact-ID absence incomplete"];
  for (const id of report.requiredScenarios)
    if (!report.cleanup.some((c) => c.scenario === id))
      return ["Cleanup scenario coverage incomplete"];
  return [];
}
