import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { describe, test } from "node:test";
import { assessPackageFeasibility } from "../src/package-feasibility-assess.ts";
import type {
  PackageManifest,
  PackageSurface,
} from "../src/package-feasibility-model.ts";
import {
  bytes,
  declaration,
  linkedManifest,
  manifest,
} from "./package-feasibility-fixtures.ts";

const assess = (m: PackageManifest) => assessPackageFeasibility(bytes(m));
const realTarget = (packageId: string) => ({
  code: "real-target-unverified",
  packageId,
  ownerId: "maintainer",
  nextAction: "select-target-and-collect-evidence",
});
function semantics(m: PackageManifest) {
  const { inputDigest, ...rest } = assess(m);
  assert.equal(typeof inputDigest, "string");
  return rest;
}

for (const repeat of [1, 2])
  describe(`package assessment repeat ${repeat}`, () => {
    test("keepsExecutionUnproven", () => {
      const emptyRequirements = assess(manifest());
      assert.deepEqual(emptyRequirements, {
        schemaVersion: 1,
        classification: "synthetic",
        execution: "unproven",
        inputDigest: createHash("sha256")
          .update(bytes(manifest()))
          .digest("hex"),
        validation: "valid",
        closure: [{ packageId: "sales", version: "1.0.0" }],
        bindings: [],
        issues: [],
        blockers: [realTarget("sales")],
      });
      for (const input of [
        Buffer.from("bad: ["),
        bytes({ ...manifest(), classification: "real" }),
        Buffer.alloc(1048577),
      ]) {
        const invalid = assessPackageFeasibility(input);
        assert.equal(invalid.execution, "unproven");
        assert.equal(invalid.classification, "synthetic");
        assert.equal(invalid.validation, "invalid");
        assert.deepEqual(invalid.closure, []);
        assert.deepEqual(invalid.bindings, []);
        assert.deepEqual(invalid.blockers, []);
      }
      const bad = linkedManifest();
      bad.packages[0].dependencies[0].version = "missing";
      const invalid = assess(bad);
      assert.deepEqual(invalid.blockers, []);
      assert.deepEqual(invalid.closure, []);
      assert.equal(invalid.execution, "unproven");
    });

    test("retainsOpaqueDependencies", () => {
      const m = linkedManifest();
      m.packages[1].source = "unavailable";
      m.packages[1].kind = "managed-2gp";
      m.packages[1].dependencyKnowledge = "unknown";
      const report = assess(m);
      assert.equal(report.validation, "valid");
      assert.equal(report.execution, "unproven");
      assert.deepEqual(report.closure, [
        { packageId: "base", version: "1.0.0" },
        { packageId: "sales", version: "2.0.0" },
      ]);
      assert.deepEqual(report.blockers, [
        {
          code: "dependency-inventory-unknown",
          packageId: "base",
          ownerId: "package-owner",
          nextAction: "collect-dependency-inventory",
        },
        realTarget("base"),
        {
          code: "source-unavailable",
          packageId: "base",
          ownerId: "package-owner",
          nextAction: "establish-permitted-execution-strategy",
        },
        realTarget("sales"),
      ]);
    });

    test("ownsEveryBlocker", () => {
      const m = manifest();
      const p = m.packages[0];
      p.kind = "unknown";
      p.source = "unknown";
      p.dependencyKnowledge = "unknown";
      p.ownerId = "chosen-owner";
      p.requirements = [
        { id: "runtime", surface: "apex", evidence: "subset-only" },
        { id: "claim", surface: "installation", evidence: "target-verified" },
      ];
      assert.deepEqual(assess(m).blockers, [
        {
          code: "dependency-inventory-unknown",
          packageId: "sales",
          ownerId: "chosen-owner",
          nextAction: "collect-dependency-inventory",
        },
        {
          code: "package-kind-unknown",
          packageId: "sales",
          ownerId: "chosen-owner",
          nextAction: "collect-package-details",
        },
        realTarget("sales"),
        {
          code: "source-unknown",
          packageId: "sales",
          ownerId: "chosen-owner",
          nextAction: "collect-source-availability",
        },
        {
          code: "surface-unverified",
          packageId: "sales",
          ownerId: "platform-implementer",
          nextAction: "design-target-surface-test",
          requirementId: "runtime",
          surface: "apex",
          evidence: "subset-only",
        },
        {
          code: "synthetic-evidence-claim",
          packageId: "sales",
          ownerId: "maintainer",
          nextAction: "collect-real-evidence",
          requirementId: "claim",
          surface: "installation",
          evidence: "target-verified",
        },
      ]);
      const surfaces: PackageSurface[] = [
        "metadata",
        "apex",
        "flow",
        "imported-ui",
        "platform-service",
        "org-feature",
        "installation",
        "license-service",
      ];
      p.kind = "unmanaged";
      p.source = "available";
      p.dependencyKnowledge = "complete";
      p.requirements = surfaces.map((surface) => ({
        id: surface,
        surface,
        evidence: "absent",
      }));
      const expected = [...surfaces].sort().map((surface) => ({
        code: "surface-unverified",
        packageId: "sales",
        ownerId: "platform-implementer",
        nextAction: "design-target-surface-test",
        requirementId: surface,
        surface,
        evidence: "absent",
      }));
      assert.deepEqual(assess(m).blockers, [realTarget("sales"), ...expected]);
    });

    test("rejectsOptimisticEvidence", () => {
      const m = manifest();
      m.packages[0].requirements = [
        { id: "claimed", surface: "metadata", evidence: "target-verified" },
      ];
      const optimistic = assess(m);
      assert.equal(optimistic.execution, "unproven");
      assert.deepEqual(optimistic.blockers, [
        realTarget("sales"),
        {
          code: "synthetic-evidence-claim",
          packageId: "sales",
          ownerId: "maintainer",
          nextAction: "collect-real-evidence",
          requirementId: "claimed",
          surface: "metadata",
          evidence: "target-verified",
        },
      ]);
      for (const kind of [
        "unmanaged",
        "unlocked",
        "managed-1gp",
        "managed-2gp",
      ] as const) {
        m.packages[0].kind = kind;
        m.packages[0].requirements = [];
        assert.deepEqual(assess(m).blockers, [realTarget("sales")]);
      }
    });

    test("ignoresValidUnreachableBlockers", () => {
      const m = linkedManifest();
      const unused = declaration("unused");
      unused.kind = "unknown";
      unused.source = "unavailable";
      unused.dependencyKnowledge = "unknown";
      unused.requirements = [
        { id: "unknown", surface: "platform-service", evidence: "absent" },
      ];
      m.packages.push(unused);
      assert.deepEqual(semantics(m), semantics(linkedManifest()));
      // Once rooted it must contribute its own blockers.
      m.roots.push("unused");
      assert.equal(
        assess(m).blockers.filter((b) => b.packageId === "unused").length,
        5,
      );
    });

    test("returnsDetachedDeterministicReports", () => {
      const m = linkedManifest();
      m.roots.push("base");
      for (const p of m.packages)
        p.requirements = [
          { id: "ui", surface: "imported-ui", evidence: "subset-only" },
          { id: "flow", surface: "flow", evidence: "absent" },
        ];
      m.packages[0].components.push({
        kind: "flow",
        name: "Pipeline",
        references: [{ packageId: "base", kind: "object", name: "Customer" }],
      });
      const before = structuredClone(m);
      const input = bytes(m);
      const expected = assessPackageFeasibility(Buffer.from(input));
      assert.deepEqual(assessPackageFeasibility(Buffer.from(input)), expected);
      assert.deepEqual(m, before);
      input.fill(0);
      assert.deepEqual(assess(m), expected);
      const changed = assess(m);
      changed.closure[0].version = "mutated";
      changed.bindings[0].provider.name = "Mutated";
      changed.blockers[0].ownerId = "mutated";
      changed.issues.push({ code: "missing-root" });
      assert.deepEqual(assess(m), expected);
      m.roots.reverse();
      m.packages.reverse();
      for (const p of m.packages) {
        p.dependencies.reverse();
        p.components.reverse();
        p.requirements.reverse();
        for (const c of p.components) c.references.reverse();
      }
      const { inputDigest, ...original } = expected;
      assert.notEqual(assess(m).inputDigest, inputDigest);
      assert.deepEqual(semantics(m), original);
      // Invalid model issue ordering is deterministic too.
      m.packages[0].dependencies.push({
        packageId: "missing",
        version: "exact",
      });
      m.packages[1].requirements.push({ ...m.packages[1].requirements[0] });
      const invalid = semantics(m);
      m.packages.reverse();
      for (const p of m.packages) {
        p.dependencies.reverse();
        p.components.reverse();
        p.requirements.reverse();
      }
      assert.deepEqual(semantics(m), invalid);
    });
  });
