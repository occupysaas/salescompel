import test from "node:test";
import assert from "node:assert/strict";
import { validateOpenApi } from "../src/openapi.ts";
const spec = () => ({
  openapi: "3.0.3",
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
test("validates 3.0 JSON/YAML structures and rejects other versions", async () => {
  await validateOpenApi(spec());
  await assert.rejects(validateOpenApi({ ...spec(), openapi: "3.1.0" }), /3.0/);
  await assert.rejects(
    validateOpenApi({
      swagger: "2.0",
      info: { title: "old", version: "1" },
      paths: {},
    }),
    /3.0/,
  );
});
test("rejects unresolved and remote references without fetching", async () => {
  const s: any = spec();
  s.paths["/items"].get.responses["200"].content = {
    "application/json": { schema: { $ref: "#/components/schemas/Missing" } },
  };
  await assert.rejects(validateOpenApi(s));
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
