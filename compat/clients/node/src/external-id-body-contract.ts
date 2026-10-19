import { Ajv } from "ajv";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { isDeepStrictEqual } from "node:util";
import { resolve } from "node:path";
import {
  type ExternalIdConfig,
  type RecordState,
  parseExternalIdConfig,
} from "./external-id-contract.ts";
import { ROOT, connectorVersions } from "./versions.ts";

export type BodyValueKind =
  | "omitted"
  | "identical"
  | "variant"
  | "different"
  | "null"
  | "empty"
  | "number"
  | "object";
export interface BodyCase {
  id: string;
  recordState: "existing" | "missing";
  valueKind: BodyValueKind;
}
const kinds: readonly BodyValueKind[] = [
  "omitted",
  "identical",
  "variant",
  "different",
  "null",
  "empty",
  "number",
  "object",
];
export const bodyManifest: readonly BodyCase[] = Object.freeze(
  (["existing", "missing"] as const).flatMap((recordState) =>
    kinds.map((valueKind) =>
      Object.freeze({
        id: `${recordState}-${valueKind}`,
        recordState,
        valueKind,
      }),
    ),
  ),
);
export const bodyManifestHash = createHash("sha256")
  .update(JSON.stringify(bodyManifest))
  .digest("hex");
export interface BodyScope {
  pathKey: string;
  variantKey: string;
  differentKey: string;
  seedName: string;
  requestName: string;
}
export function bodyScope(runId: string, c: BodyCase): BodyScope {
  const prefix = `p11${runId}-${c.id}-`;
  return {
    pathKey: prefix + "AbC",
    variantKey: prefix + "abc",
    differentKey: prefix + "different",
    seedName: prefix + "seed",
    requestName: prefix + "request",
  };
}
export function bodyRequest(
  scope: BodyScope,
  kind: BodyValueKind,
): Record<string, unknown> {
  const request: Record<string, unknown> = { Name: scope.requestName };
  if (kind !== "omitted")
    request.Compat_Key__c = {
      identical: scope.pathKey,
      variant: scope.variantKey,
      different: scope.differentKey,
      null: null,
      empty: "",
      number: 42,
      object: { value: "invalid" },
    }[kind];
  return request;
}
export interface BodyObservation {
  caseId: string;
  request: Record<string, unknown>;
  pathKey: string;
  before: RecordState[];
  after: RecordState[];
  stateVerified: boolean;
  status: number | null;
  bodyPresent: boolean;
  body: unknown;
  state: "observed" | "unavailable";
  artifact: string;
  sha256: string;
  stateArtifact: string;
  stateSha256: string;
  seedArtifact?: string;
  seedSha256?: string;
}
export interface BodyCleanup {
  outcome: "clean" | "pending" | "blocked";
  startedAt: string;
  finishedAt: string;
  journalSha256: string;
  records: {
    caseId: string;
    id: string;
    absenceArtifact: string;
    absenceSha256: string;
  }[];
  unresolvedOperationIds: string[];
  reasons: string[];
}
export interface BodyReport {
  schemaVersion: 1;
  kind: "external-id-body-validation";
  config: ExternalIdConfig;
  connectors: ReturnType<typeof connectorVersions>;
  manifestHash: string;
  runId: string;
  startedAt: string;
  finishedAt: string;
  identityVerified: boolean;
  collection: "complete" | "incomplete" | "blocked";
  observations: BodyObservation[];
  cleanup: BodyCleanup;
  reasons: string[];
}
/** Complete contradictory evidence is different; insufficient evidence is blocked. */
export interface BodyAssessment {
  outcome: "ready" | "different" | "blocked";
  reasons: string[];
  findings: {
    caseId: string;
    outcome: "ready" | "different" | "blocked";
    reasons: string[];
  }[];
  inputReportDigests: string[];
}
const schema = JSON.parse(
  readFileSync(
    resolve(ROOT, "compat/schemas/external-id-body.schema.json"),
    "utf8",
  ),
);
const validator = new Ajv({ allErrors: true }).compile(schema);
const digest = /^[a-f0-9]{64}$/;
function timeRange(start: string, end: string) {
  const utc = /^\d{4}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}(?:\.\d{1,3})?Z$/;
  if (
    ![start, end].every((t) => utc.test(t) && Number.isFinite(Date.parse(t))) ||
    Date.parse(start) > Date.parse(end)
  )
    throw new Error("Invalid body evidence time range");
}
function evidence(
  artifact: string | undefined,
  sha256: string | undefined,
  required: boolean,
) {
  if (!artifact && !sha256 && !required) return;
  if (
    !artifact ||
    !/^[A-Za-z0-9][A-Za-z0-9._-]*\.json$/.test(artifact) ||
    !sha256 ||
    !digest.test(sha256)
  )
    throw new Error("Invalid body evidence artifact");
}
export function validateBodyReport(
  value: unknown,
): asserts value is BodyReport {
  if (!validator(value)) throw new Error("Invalid external-ID body report");
  const r = value as BodyReport;
  parseExternalIdConfig(r.config);
  if (
    r.manifestHash !== bodyManifestHash ||
    !isDeepStrictEqual(r.connectors, connectorVersions())
  )
    throw new Error("Invalid body report binding");
  timeRange(r.startedAt, r.finishedAt);
  timeRange(r.cleanup.startedAt, r.cleanup.finishedAt);
  const cases = new Map(bodyManifest.map((c) => [c.id, c]));
  const seen = new Set<string>();
  const records = new Map<string, Set<string>>();
  for (const o of r.observations) {
    const c = cases.get(o.caseId);
    if (!c || seen.has(o.caseId))
      throw new Error("Invalid or duplicate body case");
    seen.add(o.caseId);
    const scope = bodyScope(r.runId, c);
    if (
      o.pathKey !== scope.pathKey ||
      !isDeepStrictEqual(o.request, bodyRequest(scope, c.valueKind))
    )
      throw new Error("Invalid body request binding");
    evidence(o.artifact, o.sha256, o.state === "observed");
    evidence(o.stateArtifact, o.stateSha256, o.stateVerified);
    evidence(
      o.seedArtifact,
      o.seedSha256,
      c.recordState === "existing" && o.state === "observed",
    );
    if (c.recordState === "missing" && (o.seedArtifact || o.seedSha256))
      throw new Error("Unexpected seed evidence");
    if (
      (o.state === "observed" && o.status === null) ||
      (!o.bodyPresent && o.body !== null)
    )
      throw new Error("Invalid body response evidence");
    for (const rows of [o.before, o.after]) {
      if (new Set(rows.map((row) => row.Id)).size !== rows.length)
        throw new Error("Duplicate state record");
      for (const row of rows) {
        const owners = records.get(row.Id) ?? new Set<string>();
        owners.add(c.id);
        records.set(row.Id, owners);
      }
    }
  }
  const cleanupIds = new Set<string>();
  for (const record of r.cleanup.records) {
    if (!cases.has(record.caseId) || cleanupIds.has(record.id))
      throw new Error("Invalid cleanup identity");
    cleanupIds.add(record.id);
    if (records.has(record.id) && !records.get(record.id)!.has(record.caseId))
      throw new Error("Invalid cleanup case association");
    evidence(
      record.absenceArtifact,
      record.absenceSha256,
      r.cleanup.outcome === "clean",
    );
  }
  if (r.cleanup.journalSha256 && !digest.test(r.cleanup.journalSha256))
    throw new Error("Invalid journal digest");
  if (
    r.cleanup.outcome === "clean" &&
    (!digest.test(r.cleanup.journalSha256) ||
      r.cleanup.unresolvedOperationIds.length ||
      [...records.keys()].some((id) => !cleanupIds.has(id)))
  )
    throw new Error("Unverified body cleanup");
  if (
    r.collection === "complete" &&
    (!r.identityVerified ||
      r.cleanup.outcome !== "clean" ||
      seen.size !== cases.size ||
      r.observations.some((o) => o.state !== "observed" || !o.stateVerified))
  )
    throw new Error("Incomplete body evidence coverage");
}
