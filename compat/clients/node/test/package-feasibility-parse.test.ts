import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import { describe, test } from "node:test";
import {
  parsePackageManifest,
  MAX_PACKAGE_MANIFEST_BYTES,
} from "../src/package-feasibility-parse.ts";
import {
  bytes,
  declaration,
  manifest,
} from "./package-feasibility-fixtures.ts";

function rejected(input: Uint8Array, code: string): void {
  const result = parsePackageManifest(input);
  assert.equal(result.ok, false);
  if (result.ok) assert.fail("unexpected valid manifest");
  assert.deepEqual(result.issues, [{ code }]);
  assert.equal(
    result.inputDigest,
    input.length > MAX_PACKAGE_MANIFEST_BYTES
      ? null
      : createHash("sha256").update(input).digest("hex"),
  );
  assert.equal("manifest" in result, false);
  assert.equal(JSON.stringify(result).includes("PRIVATE_MARKER"), false);
}

for (const repeat of [1, 2])
  describe(`manifest parser repeat ${repeat}`, () => {
    test("parsesSyntheticManifest", () => {
      const input = bytes(manifest());
      const result = parsePackageManifest(input);
      assert.equal(result.ok, true);
      if (!result.ok) assert.fail("unexpected invalid manifest");
      assert.deepEqual(result.manifest, manifest());
      assert.equal(
        result.inputDigest,
        createHash("sha256").update(input).digest("hex"),
      );
      input.fill(0);
      assert.deepEqual(result.manifest, manifest());
    });

    test("rejectsMalformedManifest", () => {
      for (const text of [
        "a: 1\na: PRIVATE_MARKER",
        "---\na: 1\n---\na: 2",
        "x: [",
        "x: &x [*x]",
      ])
        rejected(Buffer.from(text), "parse-invalid");
      rejected(
        Buffer.from(JSON.stringify(manifest()) + "\n---\nPRIVATE_MARKER"),
        "parse-invalid",
      );
      rejected(Buffer.from([0xff, 0xfe]), "encoding-invalid");
      for (const value of [
        null,
        "PRIVATE_MARKER",
        [],
        { ...manifest(), classification: "real" },
        { ...manifest(), schemaVersion: "1" },
        { ...manifest(), PRIVATE_MARKER: true },
      ])
        rejected(bytes(value), "schema-invalid");
      // Every object is strict, and every declared key is required.
      const samples = [
        manifest(),
        declaration(),
        { packageId: "base", version: "1" },
        { kind: "apex", name: "Editor", references: [] },
        { packageId: "base", kind: "object", name: "Customer" },
        { id: "service", surface: "platform-service", evidence: "absent" },
      ];
      const wrap = (index: number, value: unknown): unknown => {
        const m = manifest();
        if (index === 0) return value;
        if (index === 1) return { ...m, packages: [value] };
        const p: Record<string, unknown> = { ...m.packages[0] };
        if (index === 2) p.dependencies = [value];
        if (index === 3) p.components = [value];
        if (index === 4)
          p.components = [
            { kind: "apex", name: "Editor", references: [value] },
          ];
        if (index === 5) p.requirements = [value];
        return { ...m, packages: [p] };
      };
      samples.forEach((sample, index) => {
        rejected(
          bytes(wrap(index, { ...sample, PRIVATE_MARKER: true })),
          "schema-invalid",
        );
        for (const key of Object.keys(sample)) {
          const missing: Record<string, unknown> = { ...sample };
          delete missing[key];
          rejected(bytes(wrap(index, missing)), "schema-invalid");
        }
      });
      for (const patch of [
        { id: "Bad" },
        { id: "a".repeat(65) },
        { ownerId: "" },
        { version: "*" },
        { version: "1".repeat(65) },
        { namespace: "" },
        { namespace: "a".repeat(16) },
        { namespace: "1bad" },
        { kind: "plugin" },
        { source: "yes" },
        { dependencyKnowledge: "partial" },
        { namespace: 4 },
        { components: [{ kind: "script", name: "Editor", references: [] }] },
        {
          components: [{ kind: "apex", name: "a".repeat(65), references: [] }],
        },
        { components: [{ kind: "apex", name: "bad.name", references: [] }] },
        {
          requirements: [
            { id: "check", surface: "unknown", evidence: "absent" },
          ],
        },
        {
          requirements: [
            { id: "check", surface: "apex", evidence: "verified" },
          ],
        },
      ])
        rejected(
          bytes({ ...manifest(), packages: [{ ...declaration(), ...patch }] }),
          "schema-invalid",
        );
    });

    test("boundsManifestInput", () => {
      const input = bytes(manifest());
      const at = Buffer.concat([
        input,
        Buffer.alloc(MAX_PACKAGE_MANIFEST_BYTES - input.length, 0x20),
      ]);
      assert.equal(parsePackageManifest(at).ok, true);
      rejected(Buffer.concat([at, Buffer.from(" ")]), "input-too-large");
      const arrays = [
        { path: "roots", max: 32, item: "sales", min: 1 },
        { path: "packages", max: 128, item: declaration(), min: 1 },
        {
          path: "dependencies",
          max: 128,
          item: { packageId: "base", version: "1" },
          min: 0,
        },
        {
          path: "components",
          max: 256,
          item: { kind: "apex", name: "Editor", references: [] },
          min: 0,
        },
        {
          path: "references",
          max: 128,
          item: { packageId: "base", kind: "object", name: "Customer" },
          min: 0,
        },
        {
          path: "requirements",
          max: 32,
          item: { id: "check", surface: "apex", evidence: "absent" },
          min: 0,
        },
      ];
      for (const { path, max, item, min } of arrays) {
        const sized = (length: number) => {
          const list = Array.from({ length }, () => structuredClone(item));
          const m = manifest();
          if (path === "roots" || path === "packages")
            return { ...m, [path]: list };
          if (path === "references")
            return {
              ...m,
              packages: [
                {
                  ...declaration(),
                  components: [
                    { kind: "apex", name: "Editor", references: list },
                  ],
                },
              ],
            };
          return { ...m, packages: [{ ...declaration(), [path]: list }] };
        };
        assert.equal(parsePackageManifest(bytes(sized(max))).ok, true, path);
        assert.equal(parsePackageManifest(bytes(sized(min))).ok, true, path);
        rejected(bytes(sized(max + 1)), "schema-invalid");
        if (min) rejected(bytes(sized(0)), "schema-invalid");
      }
      // These are parser resource failures, before schema validation.
      rejected(
        Buffer.from("[".repeat(34) + "0" + "]".repeat(34)),
        "parse-invalid",
      );
      rejected(bytes(Array(10001).fill(0)), "parse-invalid");
      rejected(
        Buffer.from(
          "a: &a [0, 1]\nb: &b [*a, *a, *a, *a, *a, *a, *a, *a]\nc: [*b, *b, *b, *b, *b, *b, *b, *b]",
        ),
        "parse-invalid",
      );
      const boundary = declaration("a".repeat(64));
      boundary.ownerId = "a".repeat(64);
      boundary.version = "v".repeat(64);
      boundary.namespace = "N".repeat(15);
      boundary.components = [
        { kind: "apex", name: "_".repeat(64), references: [] },
      ];
      assert.equal(
        parsePackageManifest(
          bytes({ ...manifest(), roots: [boundary.id], packages: [boundary] }),
        ).ok,
        true,
      );
    });
  });
