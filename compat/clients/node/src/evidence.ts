import { Ajv } from "ajv";
import { createHash } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import { compare, validateReport, type Outcome } from "./core.ts";
import type { EvidenceContext, Readiness } from "./readiness.ts";
export interface Observation {
  schemaVersion: 1;
  id: string;
  profile: string;
  profileHash: string;
  apiVersion: string;
  connectors: Record<string, string>;
  contracts: string[];
  captures: { sha256: string; startedAt: string; finishedAt: string }[];
  observer: string;
  review: { reviewer: string; reviewedAt: string; sanitized: boolean };
  repeatability: "passed";
  cleanup: "passed";
  notes: { contract: string; summary: string }[];
}
function validTime(value: string): boolean {
  if (
    !/^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(\.\d{3})?Z$/.test(value) ||
    !Number.isFinite(Date.parse(value))
  )
    return false;
  return (
    new Date(value).toISOString() ===
    value.replace(/(?<!\.[0-9]{3})Z$/, ".000Z")
  );
}
function captureTimeIssues(captures: Observation["captures"]): string[] {
  if (
    captures.length !== 2 ||
    captures.some(
      (c) =>
        !validTime(c.startedAt) ||
        !validTime(c.finishedAt) ||
        Date.parse(c.startedAt) > Date.parse(c.finishedAt) ||
        Date.parse(c.finishedAt) > Date.now(),
    )
  )
    return ["Invalid capture timestamps"];
  if (
    captures[0].sha256 === captures[1].sha256 ||
    Date.parse(captures[0].startedAt) === Date.parse(captures[1].startedAt) ||
    Date.parse(captures[0].finishedAt) > Date.parse(captures[1].startedAt)
  )
    return ["Captures must be distinct and sequential"];
  return [];
}
export function verifyCapturePair(
  first: string,
  second: string,
  context: EvidenceContext,
): { outcome: Outcome; issues: string[]; captures: Observation["captures"] } {
  const blocked = (issue: string) => ({
    outcome: "blocked" as const,
    issues: [issue],
    captures: [],
  });
  try {
    const a: unknown = JSON.parse(first),
      b: unknown = JSON.parse(second);
    validateReport(a);
    validateReport(b);
    if (
      !context.apiVersion ||
      a.apiVersion !== context.apiVersion ||
      b.apiVersion !== context.apiVersion
    )
      return blocked("Capture API version differs from pinned profile");
    const captures = [a, b].map((r, i) => ({
      sha256: createHash("sha256")
        .update(i === 0 ? first : second)
        .digest("hex"),
      startedAt: r.startedAt,
      finishedAt: r.finishedAt,
    }));
    const timeIssues = captureTimeIssues(captures);
    if (timeIssues.length) return blocked(timeIssues[0]);
    const comparison = compare(a, b, context, "repeatability");
    // Comparison details can contain private captured data; do not export them here.
    if (comparison.outcome !== "passed")
      return {
        outcome: comparison.outcome,
        issues: [
          "Reference pair did not pass repeatability or evidence prerequisites",
        ],
        captures: [],
      };
    return { outcome: "passed", issues: [], captures };
  } catch {
    return blocked("Malformed private capture reports");
  }
}
export function observationIssues(
  value: unknown,
  schema: unknown,
  context: EvidenceContext,
  readiness: unknown,
): string[] {
  try {
    if (!new Ajv({ allErrors: true }).validate(schema as object, value))
      return ["Invalid observation structure"];
  } catch {
    return ["Invalid observation schema"];
  }
  const n = value as Observation,
    r = readiness as Readiness;
  const issues = captureTimeIssues(n.captures);
  if (
    n.profile !== context.profile ||
    n.profileHash !== context.profileHash ||
    !context.apiVersion ||
    n.apiVersion !== context.apiVersion ||
    !isDeepStrictEqual(n.connectors, context.connectors)
  )
    issues.push("Observation profile/API/connector binding differs");
  if (
    !r ||
    n.observer !== r.observer ||
    n.observer.trim().toLowerCase() === r.implementer?.trim().toLowerCase()
  )
    issues.push("Observation role assignment differs");
  if (
    !validTime(n.review.reviewedAt) ||
    Date.parse(n.review.reviewedAt) > Date.now() ||
    n.captures.some(
      (c) => Date.parse(c.finishedAt) > Date.parse(n.review.reviewedAt),
    )
  )
    issues.push("Invalid observation review date");
  if (
    n.contracts.some((id) => !context.requiredScenarios.includes(id)) ||
    !isDeepStrictEqual(
      [...n.contracts].sort(),
      n.notes.map((note) => note.contract).sort(),
    )
  )
    issues.push(
      "Observation notes must cover exactly the claimed known contracts",
    );
  const text = n.notes.map((note) => note.summary).join("\n");
  if (
    /\b(?:Bearer|Basic)\s+\S+|(?:access[_-]?token|refresh[_-]?token|password|client[_-]?secret|authorization)\s*[=:]|\b[a-zA-Z0-9]{15}(?:[a-zA-Z0-9]{3})?\b/.test(
      text,
    )
  )
    issues.push(
      "Observation notes contain potential credentials or identifiers; review privately",
    );
  return issues;
}
