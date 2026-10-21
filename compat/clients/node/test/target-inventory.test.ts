import test from "node:test";
import type { TargetInventory } from "../src/target-inventory-model.ts";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import { Ajv } from "ajv";
import {
  makeInventory,
  makeCompleteInventory,
  makeItem,
  known,
  unknown,
} from "./target-inventory-fixtures.ts";
const schemaPath = new URL(
  "../../../../compat/schemas/target-inventory.schema.json",
  import.meta.url,
);
const schema = () =>
  new Ajv({ allErrors: true }).compile<TargetInventory>(
    JSON.parse(readFileSync(schemaPath, "utf8")),
  );

test("inventory schema accepts explicit incomplete and complete synthetic/real test data", () => {
  const validate = schema();
  for (const value of [
    makeInventory(),
    makeCompleteInventory("synthetic"),
    makeCompleteInventory("real"),
  ])
    assert.ok(validate(value), JSON.stringify(validate.errors));
});
test("inventory schema rejects incomplete structure and unknown fields or dispositions", () => {
  const validate = schema();
  const mutations = [
    (v: any) => {
      delete v.sections.security;
    },
    (v: any) => {
      v.secretExtra = "marker";
    },
    (v: any) => {
      v.schemaVersion = 2;
    },
    (v: any) => {
      v.inventoryId = "../private";
    },
    (v: any) => {
      v.date = "yesterday";
    },
    (v: any) => {
      v.sections.dataMetadata.items[0].disposition = "maybe";
    },
    (v: any) => {
      v.sections.dataMetadata.items[0].details.extra = true;
    },
  ];
  for (const mutate of mutations) {
    const value = makeInventory();
    mutate(value);
    assert.equal(validate(value), false);
  }
});
test("typed knowledge preserves known false/zero and unknown volume/version", () => {
  const validate = schema();
  const v = makeInventory();
  const feature: any = {
    ...makeItem(),
    id: "person-accounts",
    category: "org-feature",
    details: { name: known("personAccounts"), enabled: known(false) },
  };
  v.sections.sourceOrg.items.push(feature);
  const volume: any = {
    ...makeItem(),
    id: "account-count",
    category: "data-volume",
    details: {
      name: known("Account"),
      count: known(0),
      unit: known("records"),
    },
  };
  v.sections.dataMetadata.items.push(volume);
  assert.ok(validate(v), JSON.stringify(validate.errors));
  volume.details.count = unknown();
  assert.ok(validate(v));
  volume.details.count = known(-1);
  assert.equal(validate(v), false);
  volume.details.count = "unknown";
  assert.equal(validate(v), false);
  const client: any = {
    ...makeItem(),
    id: "client",
    category: "client",
    details: {
      name: known("client"),
      version: unknown(),
      protocol: known("REST"),
      endpoint: unknown(),
      authentication: unknown(),
      dependencies: known([]),
    },
  };
  v.sections.dataMetadata.items.pop();
  v.sections.clientsIntegrations.items.push(client);
  assert.ok(validate(v), JSON.stringify(validate.errors));
});

test("checked-in synthetic example validates without inventing source requirements", async () => {
  const { parseDocument } = await import("../src/core.ts");
  const value = parseDocument(
    readFileSync(
      new URL(
        "../../../../docs/compat/inventories/synthetic-account-contact.yaml",
        import.meta.url,
      ),
      "utf8",
    ),
  );
  assert.equal(value.classification, "synthetic");
  assert.ok(schema()(value));
  const objective = value.sections.migrationOperations.items[0];
  assert.equal(objective.category, "operating-objective");
  if (objective.category === "operating-objective")
    assert.equal(objective.details.value.status, "unknown");
});
