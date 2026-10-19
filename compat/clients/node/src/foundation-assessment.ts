import { createHash } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import { normalize, validateReport, type Report, type Result } from "./core.ts";
import { foundationCoverage } from "./foundation-policy.ts";
import { compareFoundationTies } from "./foundation-ties.ts";
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
const sha = (text: string) => createHash("sha256").update(text).digest("hex");
const source =
  "https://developer.salesforce.com/docs/platform/salesforce-app-limits-cheatsheet/guide/salesforce-app-limits-platform-apicursors.html";
const blockedReason =
  "Cursor expiry unmeasured under foundation-query-result-lifetime";
const route = "/services/data/v65.0/query/{queryLocator}";
const locator = /^\/services\/data\/v65\.0\/query\/[^/?#]+$/;
function validTime(value: unknown): value is string {
  return (
    typeof value === "string" &&
    !Number.isNaN(Date.parse(value)) &&
    new Date(value).toISOString() === value
  );
}
function multiset(values: string[]): string[] {
  return [...values].sort();
}

function pageIssues(
  result: Result,
  prefix: string,
  observations: Record<string, any>,
): { issues: string[]; names: string[]; labels: string[] } {
  const issues: string[] = [],
    names: string[] = [];
  const labels = Object.keys(observations).filter((k) =>
    k.startsWith(`${prefix}:`),
  );
  const nums = labels.map((k) => Number(k.slice(prefix.length + 1)));
  if (
    !labels.length ||
    labels.length > 32 ||
    !isDeepStrictEqual(
      multiset(nums.map(String)),
      multiset(Array.from({ length: labels.length }, (_, i) => String(i))),
    )
  )
    return {
      issues: [`${result.id}: ${prefix} pages missing or noncontiguous`],
      names,
      labels,
    };
  let total: number | undefined;
  let count = 0;
  const ids = new Set<string>();
  const links = new Set<string>();
  for (let i = 0; i < labels.length; i++) {
    const label = `${prefix}:${i}`;
    const page = observations[label];
    const body = page?.body;
    for (const suffix of [
      "bounded JSON HTTP response",
      "query page",
      "stable total",
      ...(i === labels.length - 1 ? [] : ["continuation required"]),
    ])
      names.push(`${label}: ${suffix}`);
    if (
      page?.status !== 200 ||
      !Array.isArray(body?.records) ||
      typeof body?.done !== "boolean" ||
      !Number.isInteger(body?.totalSize) ||
      body.totalSize < 0 ||
      body.records.length > 2000
    ) {
      issues.push(`${result.id}: malformed ${label}`);
      continue;
    }
    if (total === undefined) total = body.totalSize;
    if (body.totalSize !== total)
      issues.push(`${result.id}: unstable ${prefix} total`);
    count += body.records.length;
    for (const row of body.records)
      if (typeof row?.Id === "string") {
        if (ids.has(row.Id))
          issues.push(`${result.id}: duplicate ${prefix} identity`);
        ids.add(row.Id);
      }
    const terminal = i === labels.length - 1;
    if (body.done !== terminal)
      issues.push(`${result.id}: bad ${prefix} done transition`);
    if (terminal) {
      if (Object.hasOwn(body, "nextRecordsUrl"))
        issues.push(`${result.id}: terminal ${prefix} link`);
    } else {
      const link = body.nextRecordsUrl;
      if (
        !link ||
        link.route !== route ||
        typeof link.value !== "string" ||
        !locator.test(link.value) ||
        links.has(link.value)
      )
        issues.push(`${result.id}: invalid ${prefix} link`);
      else links.add(link.value);
    }
  }
  names.push(
    `${prefix}: record count equals total`,
    `${prefix}: unique record identities`,
  );
  if (total !== count)
    issues.push(`${result.id}: incomplete ${prefix} traversal`);
  return { issues, names, labels };
}

function coverageIssues(report: Report): string[] {
  const issues: string[] = [];
  const coverage = foundationCoverage();
  if (!isDeepStrictEqual(report.requiredScenarios, Object.keys(coverage)))
    issues.push("Required foundation scenario manifest differs");
  for (const [id, spec] of Object.entries(coverage)) {
    const result = report.results.find((r) => r.id === id);
    if (!result) {
      issues.push(`${id}: result missing`);
      continue;
    }
    if (result.client !== "jsforce") issues.push(`${id}: client differs`);
    if (id === "foundation-cursors") {
      if (result.outcome !== "blocked" || result.reason !== blockedReason)
        issues.push(`${id}: expiry dimension must be explicitly unmeasured`);
    } else if (result.outcome !== "passed" || result.reason)
      issues.push(`${id}: measured scenario did not pass`);
    const observations = result.observations as Record<string, any>;
    if (
      !observations ||
      typeof observations !== "object" ||
      Array.isArray(observations)
    ) {
      issues.push(`${id}: observations malformed`);
      continue;
    }
    const expected = [...spec.observations];
    const names = [...spec.assertions];
    for (const label of spec.observations) {
      if (!Object.hasOwn(observations, label)) {
        issues.push(`${id}: missing ${label}`);
        continue;
      }
      const item = observations[label];
      if (label === "versions") {
        if (item?.status !== 200 || item?.requestedVersionAdvertised !== true)
          issues.push(`${id}: malformed versions`);
      } else if (label.startsWith("own-user:")) {
        if (item?.status !== 200 || item?.hasOwnId !== true)
          issues.push(`${id}: malformed ${label}`);
      } else if (label === "nullEquality") {
        if (
          typeof item?.nullMatches !== "boolean" ||
          typeof item?.blankMatches !== "boolean"
        )
          issues.push(`${id}: malformed nullEquality`);
      } else {
        names.push(`${label}: bounded JSON HTTP response`);
        if (
          !Number.isInteger(item?.status) ||
          item.status < 200 ||
          item.status >= 500 ||
          (item.status !== 204 && !Object.hasOwn(item, "body"))
        )
          issues.push(`${id}: malformed ${label}`);
      }
    }
    for (const prefix of spec.pages) {
      const pages = pageIssues(result, prefix, observations);
      issues.push(...pages.issues);
      expected.push(...pages.labels);
      names.push(...pages.names);
    }
    if (
      !isDeepStrictEqual(
        multiset(Object.keys(observations)),
        multiset(expected),
      )
    )
      issues.push(`${id}: unexpected or missing observations`);
    const actual = result.assertions.map((a) => a.name);
    if (result.assertions.some((a) => a.passed !== true))
      issues.push(`${id}: assertion failed`);
    if (id === "foundation-text-null") {
      const count = actual.filter(
        (x) => x === "null equality query succeeds",
      ).length;
      if (count < 1 || count > 32)
        issues.push(`${id}: null query assertion coverage invalid`);
      const without = (v: string[]) =>
        v.filter((x) => x !== "null equality query succeeds");
      if (
        !isDeepStrictEqual(multiset(without(actual)), multiset(without(names)))
      )
        issues.push(`${id}: assertion coverage differs`);
    } else if (!isDeepStrictEqual(multiset(actual), multiset(names)))
      issues.push(`${id}: assertion coverage differs`);
  }
  return issues;
}

function sidecarIssues(
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

function cleanupIssues(
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
  if (
    report.cleanup.some((c) => c.outcome !== "passed") ||
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

function withoutTies(result: Result): unknown {
  const observations = Object.fromEntries(
    Object.entries(result.observations as Record<string, unknown>).filter(
      ([k]) => !k.startsWith("ties:"),
    ),
  );
  return {
    client: result.client,
    outcome: result.outcome,
    reason: result.reason,
    assertions: result.assertions.filter((a) => !a.name.startsWith("ties:")),
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
