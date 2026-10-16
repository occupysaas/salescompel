import { Ajv } from "ajv";
import { readFile } from "node:fs/promises";
import { resolve } from "node:path";
import { isDeepStrictEqual } from "node:util";
import { checkConfig, parseDocument } from "./core.ts";
export interface EvidenceContext {
  profile: string;
  profileHash: string;
  apiVersion: string | null;
  requiredScenarios: string[];
  connectors: Record<string, string>;
  cleanupScenarios: string[];
}
export interface Readiness {
  schemaVersion: 1;
  referenceAlias: string | null;
  profile: string;
  profileHash: string | null;
  apiVersion: string | null;
  decision: {
    status: "pending" | "approved" | "rejected";
    maintainer: string | null;
    recordedAt: string | null;
    logAnchor: string | null;
    observationAuthorized: boolean;
  };
  observer: string | null;
  implementer: string | null;
  scenarios: string[];
  fixtures: {
    disposable: boolean;
    administrator: boolean;
    restricted: boolean;
    locale: boolean;
    deployed: boolean;
  };
}
export function readinessStructureIssues(
  value: unknown,
  schema: unknown,
): string[] {
  try {
    return new Ajv({ allErrors: true }).validate(schema as object, value)
      ? []
      : ["Invalid readiness record structure"];
  } catch {
    return ["Invalid readiness schema"];
  }
}
export function readinessDecisionIssues(
  value: unknown,
  schema: unknown,
  context: EvidenceContext,
): string[] {
  const issues = readinessStructureIssues(value, schema);
  if (issues.length) return issues;
  const r = value as Readiness;
  if (r.decision.status !== "approved" || !r.decision.observationAuthorized)
    issues.push("Reference observation is not authorized");
  if (
    !r.referenceAlias ||
    !r.decision.maintainer ||
    !r.decision.logAnchor ||
    !r.observer ||
    !r.implementer
  )
    issues.push("Reference decision and role assignments are incomplete");
  if (r.observer?.trim().toLowerCase() === r.implementer?.trim().toLowerCase())
    issues.push("Observer and implementer must be separate");
  const date = r.decision.recordedAt;
  if (
    !date ||
    !/^\d{4}-\d{2}-\d{2}$/.test(date) ||
    !Number.isFinite(Date.parse(date)) ||
    new Date(date).toISOString().slice(0, 10) !== date ||
    date > new Date().toISOString().slice(0, 10)
  )
    issues.push("Invalid or future decision date");
  if (r.profile !== context.profile || r.profileHash !== context.profileHash)
    issues.push("Readiness does not match the current profile");
  if (!context.apiVersion || r.apiVersion !== context.apiVersion)
    issues.push("Readiness requires the pinned profile API version");
  if (
    !isDeepStrictEqual(
      [...r.scenarios].sort(),
      [...context.requiredScenarios].sort(),
    )
  )
    issues.push("Readiness scenario scope differs from the required manifest");
  if (!Object.values(r.fixtures).every((v) => v === true))
    issues.push("Reference fixtures and personas are not ready");
  return issues;
}
export function readinessIssues(
  value: unknown,
  schema: unknown,
  context: EvidenceContext,
  env: NodeJS.ProcessEnv,
): string[] {
  const issues = [
    ...readinessDecisionIssues(value, schema, context),
    ...checkConfig("reference", env),
  ];
  if (env.COMPAT_API_VERSION !== context.apiVersion)
    issues.push("Runtime API version differs from the pinned profile");
  for (const key of [
    "COMPAT_REFERENCE_RESTRICTED_TOKEN",
    "COMPAT_REFERENCE_LOCALE_USER_ID",
  ])
    if (!env[key]?.trim()) issues.push(`Missing ${key}`);
  return issues;
}
export async function readReadiness(
  root: string,
): Promise<{ record: unknown; schema: unknown; log: string }> {
  const [record, schema, log] = await Promise.all([
    readFile(resolve(root, "docs/compat/reference-readiness.yaml"), "utf8"),
    readFile(
      resolve(root, "compat/schemas/reference-readiness.schema.json"),
      "utf8",
    ),
    readFile(resolve(root, "docs/compat/implementation-log.md"), "utf8"),
  ]);
  return { record: parseDocument(record), schema: parseDocument(schema), log };
}
export function decisionAnchorIssues(value: unknown, log: string): string[] {
  const anchor = (value as Readiness)?.decision?.logAnchor;
  if (typeof anchor !== "string") return ["Decision log anchor is missing"];
  const headings = log
    .split("\n")
    .filter((line) => /^#{1,6} /.test(line))
    .map((line) =>
      line
        .replace(/^#+\s+/, "")
        .trim()
        .toLowerCase()
        .replace(/[^\w\s-]/g, "")
        .replace(/ /g, "-"),
    );
  return headings.includes(anchor) ? [] : ["Decision log anchor was not found"];
}
export async function referencePreflight(
  root: string,
  context: EvidenceContext,
  env: NodeJS.ProcessEnv,
): Promise<string[]> {
  try {
    const { record, schema, log } = await readReadiness(root);
    return [
      ...readinessIssues(record, schema, context, env),
      ...decisionAnchorIssues(record, log),
    ];
  } catch {
    return ["Reference readiness files are missing or malformed"];
  }
}
