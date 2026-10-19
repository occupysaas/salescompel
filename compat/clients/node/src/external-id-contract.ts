import { Ajv } from "ajv";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { ROOT, connectorVersions } from "./versions.ts";
export interface ExternalIdConfig {
  schemaVersion: 1;
  apiVersion: "65.0";
  profileHash: string;
  origin: string;
  administrator: { user: string; org: string };
}
export interface Step {
  id: string;
  method: "create" | "upsert" | "update" | "native";
  key: number;
  bodyKey?: number;
  target?: string;
  parallel?: boolean;
}
export interface Case {
  id: string;
  keys: string[];
  steps: Step[];
}
const up = (id: string, key: number, extra: Partial<Step> = {}): Step => ({
  id,
  method: "upsert",
  key,
  ...extra,
});
const pair = (id: string, keys: string[]): Case => ({
  id,
  keys,
  steps: [up("first", 0), up("second", 1)],
});
export const manifest: Case[] = [
  pair("exact", ["{p}key", "{p}key"]),
  pair("ascii", ["{p}AbC", "{p}abc"]),
  {
    id: "create-variant",
    keys: ["{p}AbC", "{p}abc"],
    steps: [
      { id: "first", method: "create", key: 0 },
      { id: "second", method: "create", key: 1 },
    ],
  },
  {
    id: "update-variant",
    keys: ["{p}AbC", "{p}other", "{p}abc"],
    steps: [
      { id: "first", method: "create", key: 0 },
      { id: "second", method: "create", key: 1 },
      { id: "update", method: "update", key: 2, target: "second" },
    ],
  },
  ...["identical", "variant", "different"].map((kind, i): Case => ({
    id: `body-${kind}`,
    keys: ["{p}AbC", ["{p}AbC", "{p}abc", "{p}different"][i]],
    steps: [up("seed", 0), up("body", 0, { bodyKey: 1 })],
  })),
  pair("leading-space", ["{p}key", " {p}key"]),
  pair("trailing-space", ["{p}key", "{p}key "]),
  ...[
    ["accent", "{p}é", "{p}e\u0301"],
    ["ascii-i", "{p}I", "{p}i"],
    ["dotless-i", "{p}I", "{p}ı"],
    ["sharp-s", "{p}ß", "{p}ss"],
  ].map(([id, a, b]) => pair(id, [a, b])),
  pair("encoded", ["{p}/percent%é", "{p}/percent%é"]),
  ...["exact", "variant"].flatMap((kind) =>
    Array.from({ length: 5 }, (_, i): Case => ({
      id: `race-${kind}-${i}`,
      keys: ["{p}AbC", kind === "exact" ? "{p}AbC" : "{p}abc"],
      steps: [up("a", 0, { parallel: true }), up("b", 1, { parallel: true })],
    })),
  ),
  {
    id: "native",
    keys: ["{p}key"],
    steps: [
      { id: "first", method: "native", key: 0 },
      { id: "second", method: "native", key: 0 },
    ],
  },
];
export const manifestHash = createHash("sha256")
  .update(JSON.stringify(manifest))
  .digest("hex");
export interface RecordState {
  Id: string;
  Name: string;
  Compat_Key__c: string | null;
  Compat_Text__c: string | null;
}
export interface Observation {
  id: string;
  caseId: string;
  method: Step["method"];
  key: string;
  bodyKey: string | null;
  state: "observed" | "unavailable";
  status: number | null;
  bodyPresent: boolean;
  body: unknown;
  records: RecordState[];
  stateVerified: boolean;
  artifact: string;
  sha256: string;
}
export interface ExternalIdReport {
  schemaVersion: 1;
  kind: "external-id-contract";
  config: ExternalIdConfig;
  manifestHash: string;
  connectors: Record<string, string>;
  runId: string;
  startedAt: string;
  finishedAt: string;
  identityVerified: boolean;
  collection: "complete" | "incomplete" | "blocked";
  cleanup: "clean" | "pending" | "blocked";
  operations: Observation[];
  aliases: { id: string; alias: string }[];
  reasons: string[];
}
const schema = JSON.parse(
  readFileSync(
    resolve(ROOT, "compat/schemas/external-id-contract.schema.json"),
    "utf8",
  ),
);
const ajv = new Ajv({ allErrors: true });
const configValidator = ajv.compile(schema.$defs.config);
const reportValidator = ajv.compile(schema);
export function parseExternalIdConfig(value: unknown): ExternalIdConfig {
  if (!configValidator(value))
    throw new Error("Invalid external-ID configuration");
  const c = value as ExternalIdConfig;
  const u = new URL(c.origin);
  if (
    u.protocol !== "https:" ||
    u.origin !== c.origin ||
    u.username ||
    u.password
  )
    throw new Error("Invalid external-ID origin");
  const hash = createHash("sha256")
    .update(
      readFileSync(
        resolve(ROOT, "docs/compat/profiles/rest-query-foundation.yaml"),
      ),
    )
    .digest("hex");
  if (c.profileHash !== hash) throw new Error("External-ID profile mismatch");
  return structuredClone(c);
}
export function newExternalIdReport(
  config: ExternalIdConfig,
  runId: string,
): ExternalIdReport {
  const now = new Date().toISOString();
  return {
    schemaVersion: 1,
    kind: "external-id-contract",
    config,
    manifestHash,
    connectors: connectorVersions(),
    runId,
    startedAt: now,
    finishedAt: now,
    identityVerified: false,
    collection: "blocked",
    cleanup: "pending",
    operations: [],
    aliases: [],
    reasons: [],
  };
}
export function validateExternalIdReport(
  value: unknown,
): asserts value is ExternalIdReport {
  if (!reportValidator(value)) throw new Error("Invalid external-ID report");
  const r = value as ExternalIdReport;
  parseExternalIdConfig(r.config);
  const expected = new Map(
    manifest.flatMap((c) =>
      c.steps.map((s) => [c.id + ":" + s.id, { c, s }] as const),
    ),
  );
  if (
    r.manifestHash !== manifestHash ||
    !Number.isFinite(Date.parse(r.startedAt)) ||
    !Number.isFinite(Date.parse(r.finishedAt)) ||
    r.startedAt > r.finishedAt
  )
    throw new Error("Invalid report binding");
  if (
    new Set(r.operations.map((o) => o.id)).size !== r.operations.length ||
    new Set(r.aliases.map((a) => a.id)).size !== r.aliases.length ||
    new Set(r.aliases.map((a) => a.alias)).size !== r.aliases.length
  )
    throw new Error("Duplicate identity");
  for (const o of r.operations) {
    const e = expected.get(o.id);
    if (!e || o.caseId !== e.c.id || o.method !== e.s.method)
      throw new Error("Invalid operation binding");
    if (o.state === "observed") {
      if (
        o.artifact !== `response-${o.id.replaceAll(":", "-")}.json` ||
        !/^[a-f0-9]{64}$/.test(o.sha256) ||
        (o.method === "native"
          ? o.status !== null || !o.bodyPresent
          : o.status === null) ||
        (!o.bodyPresent && o.body !== null)
      )
        throw new Error("Missing response evidence");
    }
    if (new Set(o.records.map((row) => row.Id)).size !== o.records.length)
      throw new Error("Duplicate state identity");
    for (const row of o.records)
      if (!r.aliases.some((a) => a.id === row.Id))
        throw new Error("Unbound record");
  }
  if (
    r.collection === "complete" &&
    (!r.identityVerified ||
      r.cleanup !== "clean" ||
      r.operations.length !== expected.size ||
      r.operations.some((o) => o.state !== "observed" || !o.stateVerified))
  )
    throw new Error("Incomplete contract coverage");
}
export function caseScope(runId: string, c: Case) {
  const prefix = `p9${runId}-${c.id}-`;
  return {
    marker: `p9${runId}-${c.id}`,
    keys: c.keys.map((k) => k.replace("{p}", prefix)),
  };
}
