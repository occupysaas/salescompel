export interface EvidenceEntry {
  id: string;
  screen: string;
  action: string;
  api: { method: string; path: string }[];
  reference: { source: string; status: string }[];
  candidate: { source: string; status: string };
  simulation: string;
  limitations: string;
}
const base = "/services/data/v65.0";
export const evidence: EvidenceEntry[] = [
  {
    id: "lists",
    screen: "preview.records",
    action: "preview.evidenceLists",
    api: [{ method: "GET", path: `${base}/query` }],
    reference: [
      {
        source: "docs/compat/plan-4-reference-baseline.md",
        status: "preview.pilotObserved",
      },
    ],
    candidate: {
      source: "internal/platform/http.go; docs/compat/plan-2-implementation.md",
      status: "preview.candidateSubset",
    },
    simulation: "preview.simLists",
    limitations: "preview.limitLists",
  },
  {
    id: "fields",
    screen: "preview.forms",
    action: "preview.evidenceFields",
    api: [{ method: "GET", path: `${base}/sobjects/{object}/describe` }],
    reference: [
      {
        source: "docs/compat/plan-8-active-field-metadata.md",
        status: "preview.candidateLength",
      },
    ],
    candidate: {
      source:
        "internal/platform/catalog.go; docs/compat/plan-8-active-field-metadata.md",
      status: "preview.candidateSubset",
    },
    simulation: "preview.simFields",
    limitations: "preview.limitFields",
  },
  {
    id: "read",
    screen: "preview.details",
    action: "preview.evidenceRead",
    api: [{ method: "GET", path: `${base}/sobjects/{object}/{id}` }],
    reference: [
      {
        source: "docs/compat/plan-4-reference-baseline.md",
        status: "preview.pilotObserved",
      },
    ],
    candidate: {
      source: "internal/platform/http.go",
      status: "preview.implemented",
    },
    simulation: "preview.simRead",
    limitations: "preview.limitRead",
  },
  {
    id: "writes",
    screen: "preview.forms",
    action: "preview.evidenceWrites",
    api: [
      { method: "POST", path: `${base}/sobjects/{object}` },
      { method: "PATCH", path: `${base}/sobjects/{object}/{id}` },
    ],
    reference: [
      {
        source: "docs/compat/plan-4-reference-baseline.md",
        status: "preview.pilotObserved",
      },
    ],
    candidate: {
      source: "internal/platform/http.go",
      status: "preview.implemented",
    },
    simulation: "preview.simWrites",
    limitations: "preview.limitWrites",
  },
  {
    id: "relationships",
    screen: "preview.details",
    action: "preview.evidenceRelationships",
    api: [
      {
        method: "GET",
        path: `${base}/query?q=SELECT ... FROM Contact WHERE AccountId = '...'`,
      },
    ],
    reference: [
      {
        source: "docs/compat/plan-4-reference-baseline.md",
        status: "preview.pilotObserved",
      },
    ],
    candidate: {
      source:
        "docs/compat/plan-2-implementation.md; internal/platform/query.go",
      status: "preview.relationshipSubset",
    },
    simulation: "preview.simRelationships",
    limitations: "preview.limitRelationships",
  },
  {
    id: "ux",
    screen: "preview.frame",
    action: "preview.evidenceUx",
    api: [],
    reference: [],
    candidate: {
      source:
        "docs/superpowers/specs/2026-10-20-plan-13-exploratory-ui-design.md",
      status: "preview.proposedUx",
    },
    simulation: "preview.simUx",
    limitations: "preview.limitUx",
  },
];
