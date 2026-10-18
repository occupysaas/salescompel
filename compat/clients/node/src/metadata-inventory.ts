import { Ajv } from "ajv";
import { readFileSync } from "node:fs";
import { resolve } from "node:path";
import { createHash } from "node:crypto";
import { ROOT, connectorVersions } from "./versions.ts";
export const FIELDS: Record<string, string[]> = {
  Account: ["Id", "Name", "Compat_Text__c", "Compat_Key__c"],
  Contact: ["Id", "FirstName", "LastName", "AccountId"],
};
export const MEMBERS = {
  CustomField: ["Account.Compat_Text__c", "Account.Compat_Key__c"],
  PermissionSet: ["Compat_Admin"],
};
export type Persona = "administrator" | "restricted";
export type Area = "describe" | "metadata" | "retrieve";
export type Value =
  string | number | boolean | null | Array<string | number | boolean | null>;
export interface DiscoveryConfig {
  schemaVersion: 1;
  apiVersion: "65.0";
  profile: "rest-query-foundation";
  profileHash: string;
  origin: string;
  profiles: string[];
  permissionSets: string[];
}
export interface Fact {
  sourceRef: string;
  area: Area;
  persona: Persona;
  component: string;
  property: string;
  value: Value;
}
export interface Gap {
  sourceRef: string;
  area: Area;
  persona: Persona;
  component: string;
  property: string;
  state: "unavailable" | "unsupported" | "unrequested" | "conflict";
  code: string;
  owner: "platform-implementer";
  required: boolean;
}
export interface Operation {
  id: string;
  kind: Area;
  persona: Persona;
  component: string;
  state: "observed" | "unavailable" | "blocked";
}
export interface Artifact {
  path: string;
  bytes: number;
  sha256: string;
  component: string;
}
export interface Inventory {
  schemaVersion: 1;
  kind: "metadata-discovery";
  scope: DiscoveryConfig;
  profileHash: string;
  connectors: Record<string, string>;
  startedAt: string;
  finishedAt: string;
  identityVerified: { initial: boolean; final: boolean };
  operations: Operation[];
  facts: Fact[];
  gaps: Gap[];
  artifacts: Artifact[];
  outcome: "complete" | "incomplete" | "blocked";
}
export const inventorySchema = JSON.parse(
  readFileSync(
    resolve(ROOT, "compat/schemas/metadata-inventory.schema.json"),
    "utf8",
  ),
);
const ajv = new Ajv({ allErrors: true });
const configValidator = ajv.compile(inventorySchema.$defs.config);
const inventoryValidator = ajv.compile(inventorySchema);
export function parseDiscoveryConfig(value: unknown): DiscoveryConfig {
  if (!configValidator(value))
    throw new Error("Invalid discovery configuration");
  const c = value as DiscoveryConfig;
  const u = new URL(c.origin);
  if (
    u.protocol !== "https:" ||
    u.username ||
    u.password ||
    u.search ||
    u.hash ||
    u.pathname !== "/" ||
    u.origin !== c.origin
  )
    throw new Error("Discovery requires an HTTPS origin");
  const hash = createHash("sha256")
    .update(
      readFileSync(
        resolve(ROOT, "docs/compat/profiles/rest-query-foundation.yaml"),
      ),
    )
    .digest("hex");
  if (c.profileHash !== hash)
    throw new Error("Discovery profile binding differs");
  return {
    ...c,
    profiles: [...(c.profiles ?? [])].sort(),
    permissionSets: [...(c.permissionSets ?? [])].sort(),
  };
}
export function newInventory(scope: DiscoveryConfig): Inventory {
  const now = new Date().toISOString();
  return {
    schemaVersion: 1,
    kind: "metadata-discovery",
    scope,
    profileHash: scope.profileHash,
    connectors: connectorVersions(),
    startedAt: now,
    finishedAt: now,
    identityVerified: { initial: false, final: false },
    operations: [],
    facts: [],
    gaps: [],
    artifacts: [],
    outcome: "blocked",
  };
}
export function addGap(
  inv: Inventory,
  sourceRef: string,
  area: Area,
  persona: Persona,
  component: string,
  property: string,
  code: string,
  required = true,
  state: Gap["state"] = "unavailable",
) {
  inv.gaps.push({
    sourceRef,
    area,
    persona,
    component,
    property,
    state,
    code,
    required,
    owner: "platform-implementer",
  });
}
export function addFact(
  inv: Inventory,
  sourceRef: string,
  area: Area,
  persona: Persona,
  component: string,
  property: string,
  value: unknown,
) {
  if (value === undefined) {
    addGap(
      inv,
      sourceRef,
      area,
      persona,
      component,
      property,
      "PROPERTY_UNAVAILABLE",
    );
    return;
  }
  if (!validValue(value)) {
    addGap(
      inv,
      sourceRef,
      area,
      persona,
      component,
      property,
      "INVALID_PROPERTY",
    );
    return;
  }
  inv.facts.push({ sourceRef, area, persona, component, property, value });
}
function validValue(v: unknown): v is Value {
  return (
    v === null ||
    typeof v === "boolean" ||
    (typeof v === "number" && Number.isFinite(v)) ||
    (typeof v === "string" && v.length <= 1024) ||
    (Array.isArray(v) &&
      v.length <= 100 &&
      v.every((x) => !Array.isArray(x) && validValue(x)))
  );
}
export function validateInventory(value: unknown): asserts value is Inventory {
  if (!inventoryValidator(value)) throw new Error("Invalid metadata inventory");
  const i = value as Inventory,
    refs = new Set(i.operations.map((x) => x.id));
  if (
    refs.size !== i.operations.length ||
    i.profileHash !== i.scope.profileHash ||
    [...i.facts, ...i.gaps].some((x) => !refs.has(x.sourceRef))
  )
    throw new Error("Invalid inventory source binding");
  if (
    !Number.isFinite(Date.parse(i.startedAt)) ||
    !Number.isFinite(Date.parse(i.finishedAt)) ||
    i.startedAt > i.finishedAt
  )
    throw new Error("Invalid inventory timestamps");
  if (
    i.outcome === "complete" &&
    (!i.identityVerified.initial ||
      !i.identityVerified.final ||
      !i.operations.length ||
      i.operations.some((x) => x.state !== "observed") ||
      i.gaps.some((x) => x.required))
  )
    throw new Error("Incomplete inventory cannot claim completion");
}
export function finishInventory(i: Inventory) {
  i.finishedAt = new Date().toISOString();
  i.outcome =
    !i.identityVerified.initial || !i.identityVerified.final
      ? "blocked"
      : i.operations.some((x) => x.state !== "observed") ||
          i.gaps.some((x) => x.required)
        ? "incomplete"
        : "complete";
  validateInventory(i);
  return i;
}
