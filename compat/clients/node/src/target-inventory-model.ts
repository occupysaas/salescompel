// Keep category detail shapes aligned with target-inventory.schema.json.
export const SECTION_IDS = [
  "sourceOrg",
  "dataMetadata",
  "workflowsUsers",
  "clientsIntegrations",
  "cliPlugins",
  "automationCustomization",
  "security",
  "localization",
  "migrationOperations",
] as const;
export type SectionId = (typeof SECTION_IDS)[number];
export type Classification = "synthetic" | "real";
export type ActionCode =
  | "collect"
  | "clarify"
  | "assign-owner"
  | "define-acceptance"
  | "reconcile"
  | "review-scope";
export type Knowledge<T> =
  | { status: "known"; value: T }
  | { status: "unknown"; question: string; nextAction: ActionCode };
export type MilestoneId =
  | "M0"
  | "M1"
  | "M1R"
  | "M2"
  | "M3"
  | "M4"
  | "M5"
  | "M6"
  | "M7"
  | "M8"
  | "M9"
  | "M10";
export interface RepositoryReference {
  path: string;
  id?: string;
}
export interface Decision {
  question: string;
  nextAction: ActionCode;
}
export interface Question extends Decision {
  id: string;
  ownerId: string;
}
export interface Fact {
  status: "unknown" | "documented";
  evidenceRefs: string[];
}
export interface Absence {
  ownerId: string;
  evidenceRefs: string[];
  rationale: string;
}
export interface CategoryDetails {
  source: {
    edition: Knowledge<string>;
    release: Knowledge<string>;
    apiVersions: Knowledge<string[]>;
  };
  "org-feature": { name: Knowledge<string>; enabled: Knowledge<boolean> };
  object: { name: Knowledge<string> };
  field: {
    name: Knowledge<string>;
    object: Knowledge<string>;
    kind: Knowledge<string>;
  };
  relationship: {
    name: Knowledge<string>;
    from: Knowledge<string>;
    to: Knowledge<string>;
    kind: Knowledge<string>;
  };
  "record-type": { name: Knowledge<string>; object: Knowledge<string> };
  layout: { name: Knowledge<string>; object: Knowledge<string> };
  "data-volume": {
    name: Knowledge<string>;
    count: Knowledge<number>;
    unit: Knowledge<string>;
  };
  "export-area": {
    name: Knowledge<string>;
    exportable: Knowledge<boolean>;
    limitations: Knowledge<string[]>;
  };
  workflow: {
    name: Knowledge<string>;
    journey: Knowledge<string>;
    outcomes: Knowledge<string[]>;
    permissionBoundaries: Knowledge<string[]>;
  };
  persona: {
    name: Knowledge<string>;
    journey: Knowledge<string>;
    outcomes: Knowledge<string[]>;
    permissionBoundaries: Knowledge<string[]>;
  };
  client: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    protocol: Knowledge<string>;
    endpoint: Knowledge<string>;
    authentication: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  integration: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    protocol: Knowledge<string>;
    endpoint: Knowledge<string>;
    authentication: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  cli: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    channel: Knowledge<
      "npm" | "installer" | "tarball" | "docker-slim" | "docker-full"
    >;
  };
  plugin: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    source: Knowledge<"core" | "jit" | "npm" | "github" | "tarball">;
    signature: Knowledge<
      "bundled" | "verified" | "unsigned" | "unsigned-allowlisted"
    >;
    license: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  "cli-command": {
    command: Knowledge<string>;
    flags: Knowledge<string[]>;
    outputFormats: Knowledge<("json" | "csv" | "human" | "files")[]>;
    target: Knowledge<"org" | "non-org">;
    cliVersion: Knowledge<string>;
    pluginId: Knowledge<string>;
  };
  apex: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    dependencies: Knowledge<string[]>;
    classApiVersion: Knowledge<string>;
  };
  flow: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  formula: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  validation: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  "legacy-rule": {
    name: Knowledge<string>;
    version: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  "imported-ui": {
    name: Knowledge<string>;
    version: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  package: {
    name: Knowledge<string>;
    version: Knowledge<string>;
    dependencies: Knowledge<string[]>;
  };
  "security-policy": {
    name: Knowledge<string>;
    kind: Knowledge<string>;
    grants: Knowledge<string[]>;
    deniedBehavior: Knowledge<string>;
  };
  "locale-requirement": {
    wireLanguage: Knowledge<string>;
    locale: Knowledge<string>;
    currency: Knowledge<string>;
    timezone: Knowledge<string>;
    supportCategory: Knowledge<
      "fully-supported" | "end-user" | "platform-only"
    >;
    fallback: Knowledge<string[]>;
  };
  "migration-path": {
    name: Knowledge<string>;
    path: Knowledge<string>;
    identityMapping: Knowledge<string>;
    reconciliation: Knowledge<string>;
  };
  "operating-objective": {
    name: Knowledge<string>;
    value: Knowledge<number>;
    unit: Knowledge<string>;
  };
}
export type Category = keyof CategoryDetails;
export const CATEGORY_SECTION: Record<Category, SectionId> = {
  source: "sourceOrg",
  "org-feature": "sourceOrg",
  object: "dataMetadata",
  field: "dataMetadata",
  relationship: "dataMetadata",
  "record-type": "dataMetadata",
  layout: "dataMetadata",
  "data-volume": "dataMetadata",
  "export-area": "dataMetadata",
  workflow: "workflowsUsers",
  persona: "workflowsUsers",
  client: "clientsIntegrations",
  integration: "clientsIntegrations",
  cli: "cliPlugins",
  plugin: "cliPlugins",
  "cli-command": "cliPlugins",
  apex: "automationCustomization",
  flow: "automationCustomization",
  formula: "automationCustomization",
  validation: "automationCustomization",
  "legacy-rule": "automationCustomization",
  "imported-ui": "automationCustomization",
  package: "automationCustomization",
  "security-policy": "security",
  "locale-requirement": "localization",
  "migration-path": "migrationOperations",
  "operating-objective": "migrationOperations",
};
export interface ItemBase {
  id: string;
  disposition: "required" | "optional" | "out-of-scope" | "unknown";
  rationale: string;
  ownerId: string;
  provenanceRefs: string[];
  behavior: Knowledge<string>;
  acceptance: Knowledge<string>;
  milestone: Knowledge<MilestoneId>;
  dependencies: string[];
  repositoryRefs: RepositoryReference[];
  decision: Decision | null;
  facts: { observation: Fact; implementation: Fact; verification: Fact };
}
export type InventoryItem = {
  [C in Category]: ItemBase & { category: C; details: CategoryDetails[C] };
}[Category];
export interface InventorySection {
  status: "unknown" | "partial" | "complete";
  ownerId: string;
  evidenceRefs: string[];
  questions: Question[];
  absence: Absence | null;
  items: InventoryItem[];
}
export interface TargetInventory {
  schemaVersion: 1;
  inventoryId: string;
  revision: number;
  date: string;
  classification: Classification;
  targetLabel: string;
  ownerId: string;
  sources: {
    id: string;
    kind:
      "repository" | "operator" | "source-export" | "pipeline" | "observation";
    date: string;
    locator: string;
    description: string;
  }[];
  sections: Record<SectionId, InventorySection>;
}
