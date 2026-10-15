import test from "node:test";
import assert from "node:assert/strict";
import { validateOpenApi } from "../src/openapi.ts";
const spec = () => ({
  openapi: "3.2.0",
  info: { title: "Synthetic", version: "1" },
  paths: {
    "/items": {
      get: {
        operationId: "items",
        responses: { "200": { description: "OK" } },
      },
    },
  },
});
test("SalesCompel documents must use the latest OpenAPI version", async () => {
  await validateOpenApi(spec());
  for (const openapi of ["3.0.3", "3.1.0", "4.0.0"])
    await assert.rejects(validateOpenApi({ ...spec(), openapi }), /3\.2/);
  await assert.rejects(
    validateOpenApi({
      swagger: "2.0",
      info: { title: "old", version: "1" },
      paths: {},
    }),
    /3\.2/,
  );
});
test("Salesforce source documents accept only 2.0 and 3.0 for conversion", async () => {
  await validateOpenApi({ ...spec(), openapi: "3.0.3" }, "salesforce-source");
  await validateOpenApi(
    { swagger: "2.0", info: { title: "old", version: "1" }, paths: {} },
    "salesforce-source",
  );
  for (const openapi of ["3.1.0", "3.2.0"])
    await assert.rejects(
      validateOpenApi({ ...spec(), openapi }, "salesforce-source"),
      /2\.0 or 3\.0/,
    );
});
test("rejects structurally invalid documents", async () => {
  await assert.rejects(
    validateOpenApi({ ...spec(), info: { title: "missing version" } }),
    /Invalid OpenAPI/,
  );
});
test("rejects unresolved and remote references without fetching", async () => {
  const s: any = spec();
  s.paths["/items"].get.responses["200"].content = {
    "application/json": { schema: { $ref: "#/components/schemas/Missing" } },
  };
  await assert.rejects(validateOpenApi(s), /Unresolved reference/);
  s.paths["/items"].get.responses["200"].content[
    "application/json"
  ].schema.$ref = "https://example.invalid/schema.json";
  await assert.rejects(validateOpenApi(s), /External references/);
});
test("rejects duplicate operation IDs", async () => {
  const s: any = spec();
  s.paths["/other"] = s.paths["/items"];
  await assert.rejects(validateOpenApi(s), /operationId/);
});
