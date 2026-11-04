import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import test from "node:test";
import type {
  PackageAssessment,
  PackageBlocker,
} from "../src/package-feasibility-model.ts";
import { assessPackageFeasibility } from "../src/package-feasibility-assess.ts";

const realTarget = (packageId: string): PackageBlocker => ({
  code: "real-target-unverified",
  packageId,
  ownerId: "maintainer",
  nextAction: "select-target-and-collect-evidence",
});
const scenarios: {
  name: string;
  url: URL;
  expected: Omit<PackageAssessment, "inputDigest">;
}[] = [
  {
    name: "linked",
    url: new URL(
      "../../../fixtures/package-feasibility/linked.yaml",
      import.meta.url,
    ),
    expected: {
      schemaVersion: 1,
      classification: "synthetic",
      validation: "valid",
      execution: "unproven",
      issues: [],
      closure: [
        { packageId: "base", version: "1.0.0" },
        { packageId: "sales", version: "2.0.0" },
      ],
      bindings: [
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
      ],
      blockers: [realTarget("base"), realTarget("sales")],
    },
  },
  {
    name: "opaque",
    url: new URL(
      "../../../fixtures/package-feasibility/opaque.yaml",
      import.meta.url,
    ),
    expected: {
      schemaVersion: 1,
      classification: "synthetic",
      validation: "valid",
      execution: "unproven",
      issues: [],
      bindings: [],
      closure: [
        { packageId: "sales", version: "1.0.0" },
        { packageId: "vendor", version: "4.3.0" },
      ],
      blockers: [
        realTarget("sales"),
        {
          code: "surface-unverified",
          packageId: "sales",
          ownerId: "platform-implementer",
          nextAction: "design-target-surface-test",
          requirementId: "currency",
          surface: "org-feature",
          evidence: "absent",
        },
        {
          code: "surface-unverified",
          packageId: "sales",
          ownerId: "platform-implementer",
          nextAction: "design-target-surface-test",
          requirementId: "install",
          surface: "installation",
          evidence: "absent",
        },
        {
          code: "dependency-inventory-unknown",
          packageId: "vendor",
          ownerId: "vendor-owner",
          nextAction: "collect-dependency-inventory",
        },
        realTarget("vendor"),
        {
          code: "source-unavailable",
          packageId: "vendor",
          ownerId: "vendor-owner",
          nextAction: "establish-permitted-execution-strategy",
        },
        {
          code: "surface-unverified",
          packageId: "vendor",
          ownerId: "platform-implementer",
          nextAction: "design-target-surface-test",
          requirementId: "license",
          surface: "license-service",
          evidence: "absent",
        },
      ],
    },
  },
  {
    name: "optimistic",
    url: new URL(
      "../../../fixtures/package-feasibility/optimistic.yaml",
      import.meta.url,
    ),
    expected: {
      schemaVersion: 1,
      classification: "synthetic",
      validation: "valid",
      execution: "unproven",
      issues: [],
      bindings: [],
      closure: [{ packageId: "sales", version: "1.0.0" }],
      blockers: [
        realTarget("sales"),
        {
          code: "synthetic-evidence-claim",
          packageId: "sales",
          ownerId: "maintainer",
          nextAction: "collect-real-evidence",
          requirementId: "claim",
          surface: "metadata",
          evidence: "target-verified",
        },
      ],
    },
  },
  {
    name: "transitive-import",
    url: new URL(
      "../../../fixtures/package-feasibility/transitive-import.yaml",
      import.meta.url,
    ),
    expected: {
      schemaVersion: 1,
      classification: "synthetic",
      validation: "invalid",
      execution: "unproven",
      closure: [],
      bindings: [],
      blockers: [],
      issues: [
        {
          code: "undeclared-dependency",
          packageId: "sales",
          componentKind: "apex",
          componentName: "Controller",
        },
      ],
    },
  },
  {
    name: "namespace-collision",
    url: new URL(
      "../../../fixtures/package-feasibility/namespace-collision.yaml",
      import.meta.url,
    ),
    expected: {
      schemaVersion: 1,
      classification: "synthetic",
      validation: "invalid",
      execution: "unproven",
      closure: [],
      bindings: [],
      blockers: [],
      issues: [
        {
          code: "symbol-collision",
          packageId: "other",
          componentKind: "object",
          componentName: "Customer",
        },
        {
          code: "symbol-collision",
          packageId: "sales",
          componentKind: "object",
          componentName: "Customer",
        },
      ],
    },
  },
];

test("runsPackageScenarioMatrixTwice", async (t) => {
  for (const { name, url, expected } of scenarios) {
    const original = readFileSync(url);
    const digest = createHash("sha256").update(original).digest("hex");
    const results: PackageAssessment[] = [];
    for (const repeat of [1, 2])
      await t.test(`${name} repeat ${repeat}`, () => {
        const input = Buffer.from(original);
        const actual = assessPackageFeasibility(input);
        assert.deepEqual(actual, { ...expected, inputDigest: digest });
        assert.deepEqual(input, original);
        assert.deepEqual(readFileSync(url), original);
        results.push(actual);
      });
    assert.deepEqual(results[0], results[1]);
  }
});
