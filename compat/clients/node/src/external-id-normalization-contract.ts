import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { Ajv } from "ajv";
import { ROOT, connectorVersions } from "./versions.ts";
import {
  parseExternalIdConfig,
  type ExternalIdConfig,
  type RecordState,
} from "./external-id-contract.ts";
import { base, recordPath } from "./external-id-recovery.ts";
import type { BodyCleanup } from "./external-id-body-contract.ts";
export type NormalizationCleanup = BodyCleanup;
export interface NormalizationStep {
  id: string;
  kind: "create" | "update" | "upsert";
  key: "a" | "b" | "spare";
  target?: "first" | "second";
}
export interface NormalizationCase {
  id: string;
  row: string;
  workflow: "lookup" | "create" | "other-update" | "same-update" | "missing";
  a: string;
  b: string;
  steps: readonly NormalizationStep[];
}
const rows = [
  ["leading-space", "{p}key", " {p}key"],
  ["leading-space-reverse", " {p}key", "{p}key"],
  ["trailing-space", "{p}key", "{p}key "],
  ["trailing-space-reverse", "{p}key ", "{p}key"],
  ["ascii", "{p}AbC", "{p}abc"],
  ["accent", "{p}é", "{p}e\u0301"],
  ["sharp-s", "{p}ß", "{p}ss"],
  ["dotless-i", "{p}I", "{p}ı"],
];
const seed: NormalizationStep = { id: "seed", kind: "create", key: "a" };
const workflows: Record<
  Exclude<NormalizationCase["workflow"], "missing">,
  NormalizationStep[]
> = {
  lookup: [seed, { id: "request", kind: "upsert", key: "b" }],
  create: [seed, { id: "request", kind: "create", key: "b" }],
  "other-update": [
    seed,
    { id: "seed-second", kind: "create", key: "spare" },
    { id: "request", kind: "update", key: "b", target: "second" },
  ],
  "same-update": [
    seed,
    { id: "request", kind: "update", key: "b", target: "first" },
  ],
};
export const normalizationManifest: readonly NormalizationCase[] =
  Object.freeze(
    [
      ...rows.flatMap(([row, a, b]) =>
        Object.entries(workflows).map(([workflow, steps]) => ({
          id: `${row}-${workflow}`,
          row,
          a,
          b,
          workflow: workflow as NormalizationCase["workflow"],
          steps,
        })),
      ),
      ...rows.slice(0, 4).map(([row, a, b]): NormalizationCase => ({
        id: `${row}-missing`,
        row,
        a,
        b,
        workflow: "missing",
        steps: [
          { id: "request", kind: "upsert", key: "b" },
          { id: "replay", kind: "upsert", key: "b" },
        ],
      })),
    ].map((c) =>
      Object.freeze({
        ...c,
        steps: Object.freeze(c.steps.map((s) => Object.freeze({ ...s }))),
      }),
    ),
  );
export const normalizationManifestHash = createHash("sha256")
  .update(JSON.stringify(normalizationManifest))
  .digest("hex");
export function normalizationProbeHash() {
  const files = [
    ...["contract", "recovery", "probe", "command", "assess"].map(
      (n) => `compat/clients/node/src/external-id-normalization-${n}.ts`,
    ),
    "compat/clients/node/src/external-id-recovery.ts",
    "compat/clients/node/src/external-id-contract.ts",
    "compat/clients/node/src/external-id-body-contract.ts",
    "compat/clients/node/src/metadata-files.ts",
    "compat/clients/node/src/wire.ts",
    "compat/clients/node/src/versions.ts",
    "compat/clients/node/src/cli.ts",
    "compat/clients/node/package-lock.json",
    "compat/schemas/external-id-normalization.schema.json",
    "compat/schemas/external-id-contract.schema.json",
    "compat/schemas/external-id-body.schema.json",
  ];
  return createHash("sha256")
    .update(
      JSON.stringify(
        files.map((p) => [p, readFileSync(resolve(ROOT, p), "utf8")]),
      ),
    )
    .digest("hex");
}
export function normalizationScope(runId: string, c: NormalizationCase) {
  if (
    !/^[a-f0-9]{24}$/.test(runId) ||
    !normalizationManifest.some((x) => equal(x, c))
  )
    throw new Error("INVALID_SCOPE");
  const prefix = `p14${runId}-${c.id}-`;
  return {
    keys: {
      a: c.a.replace("{p}", prefix),
      b: c.b.replace("{p}", prefix),
      spare: prefix + "other",
    },
    names: Object.fromEntries(
      c.steps.map((s) => [s.id, prefix + s.id]),
    ) as Record<string, string>,
  };
}
export interface NormalizationRequest {
  method: "POST" | "PATCH";
  path: string;
  body: Record<string, string>;
  key: string;
}
export function normalizationRequest(
  runId: string,
  c: NormalizationCase,
  s: NormalizationStep,
  ids: Partial<Record<"first" | "second", string>>,
): NormalizationRequest {
  const scope = normalizationScope(runId, c),
    key = scope.keys[s.key];
  if (!c.steps.some((x) => equal(x, s))) throw new Error("UNKNOWN_STEP");
  const path =
    s.kind === "create"
      ? base + "sobjects/Account"
      : s.kind === "upsert"
        ? base + "sobjects/Account/Compat_Key__c/" + encodeURIComponent(key)
        : recordPath(ids[s.target!] ?? "");
  return {
    method: s.kind === "create" ? "POST" : "PATCH",
    path,
    key,
    body: {
      Name: scope.names[s.id],
      ...(s.kind === "upsert" ? {} : { Compat_Key__c: key }),
    },
  };
}
export interface NormalizationObservation {
  caseId: string;
  stepId: string;
  request: NormalizationRequest;
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
}
export interface NormalizationReport {
  schemaVersion: 1;
  kind: "external-id-normalization";
  config: ExternalIdConfig;
  connectors: ReturnType<typeof connectorVersions>;
  manifestHash: string;
  probeHash: string;
  runId: string;
  startedAt: string;
  finishedAt: string;
  identityVerified: boolean;
  collection: "complete" | "incomplete" | "blocked";
  observations: NormalizationObservation[];
  cleanup: NormalizationCleanup;
  reasons: string[];
}
export interface NormalizationAssessment {
  contractVersion: "plan14-normalization-v1";
  outcome: "repeatable" | "drift" | "blocked";
  reasons: string[];
  findings: {
    caseId: string;
    outcome: "repeatable" | "drift" | "blocked";
    reasons: string[];
  }[];
  inputReportDigests: string[];
}
const schema = JSON.parse(
  readFileSync(
    resolve(ROOT, "compat/schemas/external-id-normalization.schema.json"),
    "utf8",
  ),
);
const validate = new Ajv({ allErrors: true }).compile(schema);
export function validateNormalizationReport(
  value: unknown,
): asserts value is NormalizationReport {
  if (!validate(value)) throw new Error("INVALID_NORMALIZATION_REPORT");
  const r = value as NormalizationReport;
  parseExternalIdConfig(r.config);
  if (
    r.manifestHash !== normalizationManifestHash ||
    r.probeHash !== normalizationProbeHash() ||
    !equal(r.connectors, connectorVersions())
  )
    throw new Error("NORMALIZATION_BINDING_MISMATCH");
  const time = (s: string) =>
    Number.isFinite(Date.parse(s)) && new Date(s).toISOString() === s;
  if (
    !time(r.startedAt) ||
    !time(r.finishedAt) ||
    Date.parse(r.finishedAt) < Date.parse(r.startedAt)
  )
    throw new Error("INVALID_REPORT_TIME");
  const seen = new Set<string>();
  for (const o of r.observations) {
    const c = normalizationManifest.find((c) => c.id === o.caseId),
      s = c?.steps.find((s) => s.id === o.stepId),
      id = `${o.caseId}:${o.stepId}`;
    if (!c || !s || seen.has(id)) throw new Error("INVALID_COVERAGE");
    seen.add(id);
    const scope = normalizationScope(r.runId, c);
    if (
      o.request.key !== scope.keys[s.key] ||
      o.request.body.Name !== scope.names[s.id] ||
      o.request.method !== (s.kind === "create" ? "POST" : "PATCH") ||
      !equal(
        Object.keys(o.request.body).sort(),
        s.kind === "upsert" ? ["Name"] : ["Compat_Key__c", "Name"],
      ) ||
      (s.kind !== "upsert" && o.request.body.Compat_Key__c !== o.request.key)
    )
      throw new Error("INVALID_REQUEST");
    if (s.kind === "update") {
      recordPath(o.request.path.slice((base + "sobjects/Account/").length));
      if (!o.request.path.startsWith(base + "sobjects/Account/"))
        throw new Error("INVALID_REQUEST");
    } else if (o.request.path !== normalizationRequest(r.runId, c, s, {}).path)
      throw new Error("INVALID_REQUEST");
    if (
      o.state === "observed" &&
      (!o.artifact || !o.sha256 || o.status === null)
    )
      throw new Error("MISSING_RESPONSE");
    if (o.stateVerified && (!o.stateArtifact || !o.stateSha256))
      throw new Error("MISSING_STATE");
  }
  if (
    r.collection === "complete" &&
    (!r.identityVerified ||
      r.reasons.length ||
      seen.size !== 80 ||
      r.observations.some((o) => o.state !== "observed" || !o.stateVerified))
  )
    throw new Error("INCOMPLETE_COVERAGE");
}
