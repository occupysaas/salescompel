import type {
  TargetInventory,
  InventoryItem,
  Classification,
} from "../src/target-inventory-model.ts";
export const known = <T>(value: T) => ({ status: "known" as const, value });
export const unknown = () => ({
  status: "unknown" as const,
  question: "What does the target require?",
  nextAction: "collect" as const,
});
export function makeItem(): InventoryItem {
  return {
    id: "account",
    category: "object",
    disposition: "required",
    rationale: "Synthetic demonstration requirement, selected by the executor.",
    ownerId: "data-owner",
    provenanceRefs: ["design"],
    behavior: known("Create and read Account records"),
    acceptance: known("Read back the created record"),
    milestone: known("M3"),
    dependencies: [],
    repositoryRefs: [],
    decision: null,
    facts: {
      observation: { status: "unknown", evidenceRefs: [] },
      implementation: { status: "documented", evidenceRefs: ["design"] },
      verification: { status: "unknown", evidenceRefs: [] },
    },
    details: { name: known("Account") },
  };
}
export function makeInventory(): TargetInventory {
  const section = () => ({
    status: "unknown" as const,
    ownerId: "inventory-owner",
    evidenceRefs: [],
    questions: [],
    absence: null,
    items: [],
  });
  const value: TargetInventory = {
    schemaVersion: 1,
    inventoryId: "synthetic-sales",
    revision: 1,
    date: "2026-10-21",
    classification: "synthetic",
    targetLabel: "Synthetic example",
    ownerId: "inventory-owner",
    sources: [
      {
        id: "design",
        kind: "repository",
        date: "2026-10-21",
        locator: "docs/compat/plan-16-cli-query-column-metadata.md",
        description: "Candidate implementation facts; no customer requirements",
      },
    ],
    sections: {
      sourceOrg: section(),
      dataMetadata: section(),
      workflowsUsers: section(),
      clientsIntegrations: section(),
      cliPlugins: section(),
      automationCustomization: section(),
      security: section(),
      localization: section(),
      migrationOperations: section(),
    },
  };
  value.sections.dataMetadata.status = "partial";
  value.sections.dataMetadata.items.push(makeItem());
  return value;
}
export function makeCompleteInventory(
  classification: Classification,
): TargetInventory {
  const value = makeInventory();
  value.classification = classification;
  for (const section of Object.values(value.sections)) {
    section.status = "complete";
    section.evidenceRefs = ["design"];
    if (!section.items.length)
      section.absence = {
        ownerId: "inventory-owner",
        evidenceRefs: ["design"],
        rationale: "Synthetic test attestation, not real absence.",
      };
  }
  return value;
}
