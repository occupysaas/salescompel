import assert from "node:assert/strict";
import { describe, test } from "node:test";
import type {
  PackageManifest,
  PackageResolution,
} from "../src/package-feasibility-model.ts";
import { parsePackageManifest } from "../src/package-feasibility-parse.ts";
import { resolvePackageManifest } from "../src/package-feasibility-resolve.ts";
import {
  bytes,
  declaration,
  linkedManifest,
  manifest,
} from "./package-feasibility-fixtures.ts";

function resolve(m: PackageManifest): PackageResolution {
  const parsed = parsePackageManifest(bytes(m));
  assert.equal(parsed.ok, true);
  if (!parsed.ok) assert.fail("invalid test manifest");
  const before = structuredClone(parsed.manifest);
  const result = resolvePackageManifest(parsed.manifest);
  assert.deepEqual(parsed.manifest, before);
  return result;
}
function invalid(m: PackageManifest, code: string): PackageResolution {
  const result = resolve(m);
  assert.equal(result.validation, "invalid");
  assert.deepEqual(result.closure, []);
  assert.deepEqual(result.bindings, []);
  assert.ok(
    result.issues.some((i) => i.code === code),
    JSON.stringify(result),
  );
  return result;
}

for (const repeat of [1, 2])
  describe(`package resolution repeat ${repeat}`, () => {
    test("resolvesExactClosure", () => {
      const linked = resolve(linkedManifest());
      assert.equal(linked.validation, "valid");
      assert.deepEqual(linked.issues, []);
      assert.deepEqual(linked.closure, [
        { packageId: "base", version: "1.0.0" },
        { packageId: "sales", version: "2.0.0" },
      ]);
      assert.deepEqual(linked.bindings, [
        {
          consumer: {
            packageId: "sales",
            kind: "apex",
            name: "SalesController",
          },
          provider: {
            packageId: "base",
            kind: "field",
            name: "Customer_Name",
            namespace: "Foundation",
          },
        },
        {
          consumer: {
            packageId: "sales",
            kind: "apex",
            name: "SalesController",
          },
          provider: {
            packageId: "base",
            kind: "object",
            name: "Customer",
            namespace: "Foundation",
          },
        },
      ]);
      const diamond = manifest();
      diamond.packages = ["sales", "left", "right", "leaf"].map(declaration);
      diamond.packages[0].dependencies = ["left", "right"].map((packageId) => ({
        packageId,
        version: "1.0.0",
      }));
      for (const p of diamond.packages.slice(1, 3))
        p.dependencies = [{ packageId: "leaf", version: "1.0.0" }];
      assert.deepEqual(
        resolve(diamond).closure.map((p) => p.packageId),
        ["leaf", "left", "right", "sales"],
      );
      // Long chains use graph depth, not the YAML nesting depth or the JS stack.
      const chain = manifest();
      chain.roots = ["p0"];
      chain.packages = Array.from({ length: 128 }, (_, i) => ({
        ...declaration("p" + i),
        dependencies:
          i === 127 ? [] : [{ packageId: "p" + (i + 1), version: "1.0.0" }],
      }));
      assert.equal(resolve(chain).closure.length, 128);
    });

    test("rejectsWholeInvalidModel", () => {
      const cases: [string, (m: PackageManifest) => void][] = [
        ["duplicate-id", (m) => m.roots.push("sales")],
        [
          "duplicate-id",
          (m) => m.packages.push({ ...declaration(), version: "other" }),
        ],
        [
          "duplicate-dependency",
          (m) =>
            m.packages[0].dependencies.push({
              ...m.packages[0].dependencies[0],
            }),
        ],
        [
          "duplicate-component",
          (m) =>
            m.packages[1].components.push(
              structuredClone(m.packages[1].components[0]),
            ),
        ],
        [
          "duplicate-reference",
          (m) =>
            m.packages[0].components[0].references.push({
              ...m.packages[0].components[0].references[0],
            }),
        ],
        [
          "duplicate-requirement",
          (m) =>
            (m.packages[0].requirements = Array(2).fill({
              id: "check",
              surface: "apex",
              evidence: "absent",
            })),
        ],
        ["missing-root", (m) => (m.roots = ["missing"])],
        [
          "missing-package",
          (m) =>
            m.packages[0].dependencies.push({
              packageId: "missing",
              version: "1",
            }),
        ],
        [
          "missing-package",
          (m) =>
            (m.packages[0].components[0].references[0].packageId = "missing"),
        ],
        [
          "version-mismatch",
          (m) => (m.packages[0].dependencies[0].version = "1.0.0-other"),
        ],
        [
          "dependency-cycle",
          (m) =>
            m.packages[0].dependencies.push({
              packageId: "sales",
              version: "2.0.0",
            }),
        ],
        [
          "dependency-cycle",
          (m) =>
            m.packages[1].dependencies.push({
              packageId: "sales",
              version: "2.0.0",
            }),
        ],
        [
          "missing-component",
          (m) => (m.packages[0].components[0].references[0].name = "Missing"),
        ],
        [
          "missing-component",
          (m) => (m.packages[0].components[0].references[0].kind = "flow"),
        ],
        ["undeclared-dependency", (m) => (m.packages[0].dependencies = [])],
      ];
      for (const [code, mutate] of cases) {
        const m = linkedManifest();
        mutate(m);
        invalid(m, code);
      }
      const unreachable = linkedManifest();
      const bad = declaration("unused");
      bad.components = [
        {
          kind: "flow",
          name: "Unused",
          references: [{ packageId: "base", kind: "object", name: "Missing" }],
        },
      ];
      unreachable.packages.push(bad);
      invalid(unreachable, "missing-component");
      bad.components = [];
      bad.dependencies = [{ packageId: "unused", version: "1.0.0" }];
      invalid(unreachable, "dependency-cycle");
    });

    test("bindsExplicitProvider", () => {
      const m = linkedManifest();
      const other = structuredClone(m.packages[1]);
      other.id = "other";
      other.namespace = "Other";
      m.packages.push(other);
      assert.deepEqual(resolve(m), resolve(linkedManifest()));
      m.packages[0].dependencies.push({ packageId: "other", version: "1.0.0" });
      m.packages[0].components[0].references[0].packageId = "other";
      const result = resolve(m);
      assert.equal(
        result.bindings.find((b) => b.provider.kind === "object")?.provider
          .namespace,
        "Other",
      );
      // A local reference does not require a dependency, including prototype-like names.
      const local = manifest();
      local.roots = ["constructor"];
      const p = declaration("constructor");
      p.components = [
        {
          kind: "apex",
          name: "__proto__",
          references: [
            { packageId: "constructor", kind: "apex", name: "__proto__" },
          ],
        },
      ];
      local.packages = [p];
      assert.equal(resolve(local).bindings[0].provider.name, "__proto__");
    });

    test("preservesNamespaceIdentity", () => {
      const m = linkedManifest();
      const other = structuredClone(m.packages[1]);
      other.id = "other";
      other.components[0].name = "OtherCustomer";
      other.components[1].name = "OtherName";
      m.packages.push(other);
      assert.equal(resolve(m).validation, "valid");
      other.components[0].name = "Customer";
      const collision = invalid(m, "symbol-collision");
      assert.deepEqual(
        collision.issues
          .filter((i) => i.code === "symbol-collision")
          .map((i) => i.packageId),
        ["base", "other"],
      );
      m.packages.reverse();
      assert.deepEqual(resolve(m), collision);
      for (const p of m.packages)
        if (p.id === "base" || p.id === "other") p.namespace = null;
      invalid(m, "symbol-collision");
      other.components[0].kind = "flow";
      assert.equal(resolve(m).validation, "valid");
      other.namespace = "null";
      other.components[0].kind = "object";
      assert.equal(resolve(m).validation, "valid");
    });

    test("requiresDirectDependency", () => {
      const m = linkedManifest();
      const middle = declaration("middle");
      middle.dependencies = m.packages[0].dependencies;
      m.packages[0].dependencies = [{ packageId: "middle", version: "1.0.0" }];
      m.packages.push(middle);
      invalid(m, "undeclared-dependency");
      m.packages[0].namespace = "Foundation";
      invalid(m, "undeclared-dependency");
      m.packages[0].dependencies.push({ packageId: "base", version: "1.0.0" });
      assert.equal(resolve(m).validation, "valid");
    });
  });
