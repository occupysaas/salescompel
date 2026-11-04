/** Independent synthetic declarations; never a Salesforce package format. */
export type ComponentKind =
  "object" | "field" | "apex" | "flow" | "imported-ui";
export type PackageSurface =
  | "metadata"
  | "apex"
  | "flow"
  | "imported-ui"
  | "platform-service"
  | "org-feature"
  | "installation"
  | "license-service";
export type PackageEvidence = "absent" | "subset-only" | "target-verified";
export interface PackageReference {
  packageId: string;
  kind: ComponentKind;
  name: string;
}
export interface PackageDeclaration {
  id: string;
  version: string;
  ownerId: string;
  kind: "unmanaged" | "unlocked" | "managed-1gp" | "managed-2gp" | "unknown";
  namespace: string | null;
  source: "available" | "unavailable" | "unknown";
  dependencyKnowledge: "complete" | "unknown";
  dependencies: { packageId: string; version: string }[];
  components: {
    kind: ComponentKind;
    name: string;
    references: PackageReference[];
  }[];
  requirements: {
    id: string;
    surface: PackageSurface;
    evidence: PackageEvidence;
  }[];
}
export interface PackageManifest {
  schemaVersion: 1;
  classification: "synthetic";
  roots: string[];
  packages: PackageDeclaration[];
}
export type PackageIssueCode =
  | "input-too-large"
  | "encoding-invalid"
  | "parse-invalid"
  | "schema-invalid"
  | "duplicate-id"
  | "duplicate-dependency"
  | "duplicate-component"
  | "duplicate-reference"
  | "duplicate-requirement"
  | "missing-root"
  | "missing-package"
  | "version-mismatch"
  | "dependency-cycle"
  | "symbol-collision"
  | "missing-component"
  | "undeclared-dependency";
export interface PackageIssue {
  code: PackageIssueCode;
  packageId?: string;
  componentKind?: ComponentKind;
  componentName?: string;
}
export type ParsedManifest =
  | { ok: true; inputDigest: string; manifest: PackageManifest }
  | { ok: false; inputDigest: string | null; issues: PackageIssue[] };
export interface PackageClosureEntry {
  packageId: string;
  version: string;
}
export interface PackageBinding {
  consumer: { packageId: string; kind: ComponentKind; name: string };
  provider: {
    packageId: string;
    kind: ComponentKind;
    name: string;
    namespace: string | null;
  };
}
export interface PackageResolution {
  validation: "valid" | "invalid";
  closure: PackageClosureEntry[];
  bindings: PackageBinding[];
  issues: PackageIssue[];
}
type PackageBlockerBase = { packageId: string; ownerId: string };
export type PackageBlocker = PackageBlockerBase &
  (
    | {
        code: "real-target-unverified";
        nextAction: "select-target-and-collect-evidence";
      }
    | { code: "package-kind-unknown"; nextAction: "collect-package-details" }
    | {
        code: "source-unavailable";
        nextAction: "establish-permitted-execution-strategy";
      }
    | { code: "source-unknown"; nextAction: "collect-source-availability" }
    | {
        code: "dependency-inventory-unknown";
        nextAction: "collect-dependency-inventory";
      }
    | {
        code: "surface-unverified";
        nextAction: "design-target-surface-test";
        requirementId: string;
        surface: PackageSurface;
        evidence: "absent" | "subset-only";
      }
    | {
        code: "synthetic-evidence-claim";
        nextAction: "collect-real-evidence";
        requirementId: string;
        surface: PackageSurface;
        evidence: "target-verified";
      }
  );
export interface PackageAssessment extends PackageResolution {
  schemaVersion: 1;
  classification: "synthetic";
  inputDigest: string | null;
  execution: "unproven";
  blockers: PackageBlocker[];
}
