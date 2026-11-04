import { Ajv } from "ajv";
import { createHash } from "node:crypto";
import { readFileSync } from "node:fs";
import { parseAllDocuments } from "yaml";
import { parseInventoryDocument } from "./target-inventory.ts";
import type {
  PackageManifest,
  ParsedManifest,
  PackageIssueCode,
} from "./package-feasibility-model.ts";

export const MAX_PACKAGE_MANIFEST_BYTES = 1_048_576;

// Fixed repository schema loads once, never from a caller-supplied locator.
const validate = new Ajv({ allErrors: false }).compile<PackageManifest>(
  JSON.parse(
    readFileSync(
      new URL(
        "../../../schemas/package-feasibility.schema.json",
        import.meta.url,
      ),
      "utf8",
    ),
  ),
);

/** No invocation I/O, coercion, defaults or input-derived diagnostic prose. */
export function parsePackageManifest(bytes: Uint8Array): ParsedManifest {
  if (bytes.length > MAX_PACKAGE_MANIFEST_BYTES) {
    return {
      ok: false,
      inputDigest: null,
      issues: [{ code: "input-too-large" }],
    };
  }
  const inputDigest = createHash("sha256").update(bytes).digest("hex");
  const fail = (code: PackageIssueCode): ParsedManifest => ({
    ok: false,
    inputDigest,
    issues: [{ code }],
  });
  let text: string;
  try {
    text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    return fail("encoding-invalid");
  }
  let data: unknown;
  try {
    // The shared parser consumes its first document only. Enforce the single-
    // document contract before reusing its AST/alias/depth/size bounds.
    if (parseAllDocuments(text, { logLevel: "silent" }).length !== 1)
      return fail("parse-invalid");
    data = parseInventoryDocument(bytes);
  } catch {
    return fail("parse-invalid");
  }
  if (!validate(data)) return fail("schema-invalid");
  return { ok: true, inputDigest, manifest: data };
}
