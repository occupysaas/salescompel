import SwaggerParser from "@apidevtools/swagger-parser";
import type { OpenAPI } from "openapi-types";
export async function validateOpenApi(doc: any): Promise<void> {
  if (!/^3\.0\.\d+$/.test(doc?.openapi ?? ""))
    throw new Error("Only OpenAPI 3.0 is supported");
  const seen = new Set<object>();
  function scan(v: any): void {
    if (!v || typeof v !== "object" || seen.has(v)) return;
    seen.add(v);
    if (v.$ref && (typeof v.$ref !== "string" || !v.$ref.startsWith("#/")))
      throw new Error(
        "External references are disabled; provide a bundled document",
      );
    for (const x of Object.values(v)) scan(x);
  }
  scan(doc);
  const ids = new Set<string>();
  for (const path of Object.values(doc.paths ?? {}) as any[])
    for (const method of [
      "get",
      "post",
      "put",
      "patch",
      "delete",
      "head",
      "options",
      "trace",
    ]) {
      const op = path[method];
      if (!op) continue;
      if (op.operationId) {
        if (ids.has(op.operationId))
          throw new Error(`Duplicate operationId: ${op.operationId}`);
        ids.add(op.operationId);
      }
    }
  await SwaggerParser.validate(structuredClone(doc) as OpenAPI.Document, {
    resolve: { external: false },
    dereference: { circular: "ignore" },
  });
}
