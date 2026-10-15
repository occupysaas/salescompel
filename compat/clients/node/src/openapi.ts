import { Validator } from "@seriousme/openapi-schema-validator";
// SalesCompel authors every OpenAPI document in the latest released
// specification. Older documents that Salesforce accepts (External Services
// registrations in 2.0 or 3.0) are validated only as migration input that must
// be converted to LATEST_OPENAPI; they are never served or registered as-is.
export const LATEST_OPENAPI = "3.2";
export type OpenApiRole = "salescompel" | "salesforce-source";
const ROLE_VERSIONS: Record<OpenApiRole, RegExp> = {
  salescompel: /^3\.2\.\d+$/,
  "salesforce-source": /^(2\.0|3\.0\.\d+)$/,
};
const METHODS = [
  "get",
  "put",
  "post",
  "delete",
  "options",
  "head",
  "patch",
  "trace",
  "query",
];
function pointer(doc: any, ref: string): unknown {
  let v = doc;
  for (const raw of ref.slice(2).split("/")) {
    const key = decodeURIComponent(raw).replace(/~1/g, "/").replace(/~0/g, "~");
    if (!v || typeof v !== "object" || !(key in v)) return undefined;
    v = v[key];
  }
  return v;
}
export async function validateOpenApi(
  doc: any,
  role: OpenApiRole = "salescompel",
): Promise<void> {
  const version = doc?.openapi ?? doc?.swagger ?? "";
  if (!ROLE_VERSIONS[role].test(version))
    throw new Error(
      role === "salescompel"
        ? `SalesCompel documents must use OpenAPI ${LATEST_OPENAPI}.x; got ${version || "none"}`
        : `Salesforce source documents must be OpenAPI 2.0 or 3.0.x; got ${version || "none"}`,
    );
  const seen = new Set<object>();
  function scan(v: any): void {
    if (!v || typeof v !== "object" || seen.has(v)) return;
    seen.add(v);
    if ("$ref" in v) {
      if (typeof v.$ref !== "string" || !v.$ref.startsWith("#/"))
        throw new Error(
          "External references are disabled; provide a bundled document",
        );
      if (pointer(doc, v.$ref) === undefined)
        throw new Error(`Unresolved reference: ${v.$ref}`);
    }
    for (const x of Object.values(v)) scan(x);
  }
  scan(doc);
  const ids = new Set<string>();
  for (const path of Object.values(doc.paths ?? {}) as any[])
    for (const op of [
      ...METHODS.map((m) => path?.[m]),
      ...Object.values(path?.additionalOperations ?? {}),
    ] as any[]) {
      if (!op?.operationId) continue;
      if (ids.has(op.operationId))
        throw new Error(`Duplicate operationId: ${op.operationId}`);
      ids.add(op.operationId);
    }
  const result = await new Validator().validate(structuredClone(doc));
  if (!result.valid)
    throw new Error(
      `Invalid OpenAPI ${version}: ${JSON.stringify(result.errors).slice(0, 2000)}`,
    );
}
