export type SuiteId = "pilot" | "rest-query-foundation";
export interface SuiteDefinition {
  id: SuiteId;
  profilePath: string;
  readinessPath: string;
  requiredScenarios: string[];
  cleanupScenarios: string[];
  contractIds: string[];
}
const pilot = [
  "describe",
  "crud",
  "upsert",
  "relationships",
  "rollback",
  "permissions",
  "errors",
  "locale",
  "cli-query",
  "metadata",
  "python-crud",
];
const foundation = [
  "foundation-routes",
  "foundation-pagination",
  "foundation-cursors",
  "foundation-ids",
  "foundation-names",
  "foundation-text-null",
  "foundation-errors",
];
const registry: Record<SuiteId, SuiteDefinition> = {
  pilot: {
    id: "pilot",
    profilePath: "docs/compat/profiles/pilot.yaml",
    readinessPath: "docs/compat/reference-readiness.yaml",
    requiredScenarios: pilot,
    cleanupScenarios: [
      "crud",
      "upsert",
      "relationships",
      "errors",
      "locale",
      "cli-query",
      "metadata",
      "python-crud",
    ],
    contractIds: pilot,
  },
  "rest-query-foundation": {
    id: "rest-query-foundation",
    profilePath: "docs/compat/profiles/rest-query-foundation.yaml",
    readinessPath: "docs/compat/reference-readiness-foundation.yaml",
    requiredScenarios: foundation,
    cleanupScenarios: foundation,
    contractIds: [
      "query-response-shape",
      "query-limit-above-batch",
      "id-15-18",
      "case-insensitive-names",
      "case-insensitive-comparison",
      "null-ordering",
      "error-shape",
      "unknown-sobject",
      "query-batch-header",
      "query-locator-lifecycle",
      "rest-route-parity",
      "duplicate-canonical-fields",
    ],
  },
};
export const SUITE_IDS: SuiteId[] = ["pilot", "rest-query-foundation"];
export function suite(id: string = "pilot"): SuiteDefinition {
  if (!Object.hasOwn(registry, id))
    throw new Error(`Unknown compatibility suite: ${id}`);
  return structuredClone(registry[id as SuiteId]);
}
