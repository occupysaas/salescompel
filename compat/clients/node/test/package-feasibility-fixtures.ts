import type {
  PackageDeclaration,
  PackageManifest,
} from "../src/package-feasibility-model.ts";

export function declaration(id = "sales"): PackageDeclaration {
  return {
    id,
    version: "1.0.0",
    ownerId: "package-owner",
    kind: "unlocked",
    namespace: null,
    source: "available",
    dependencyKnowledge: "complete",
    dependencies: [],
    components: [],
    requirements: [],
  };
}

export function manifest(): PackageManifest {
  return {
    schemaVersion: 1,
    classification: "synthetic",
    roots: ["sales"],
    packages: [declaration()],
  };
}

export const bytes = (value: unknown): Buffer =>
  Buffer.from(JSON.stringify(value));

export function linkedManifest(): PackageManifest {
  const m = manifest();
  const base = declaration("base");
  base.namespace = "Foundation";
  base.components = [
    { kind: "object", name: "Customer", references: [] },
    { kind: "field", name: "Customer_Name", references: [] },
  ];
  m.packages[0].version = "2.0.0";
  m.packages[0].dependencies = [{ packageId: "base", version: "1.0.0" }];
  m.packages[0].components = [
    {
      kind: "apex",
      name: "SalesController",
      references: base.components.map(({ kind, name }) => ({
        packageId: "base",
        kind,
        name,
      })),
    },
  ];
  m.packages.push(base);
  return m;
}
