import test from "node:test";
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import {
  makeInventory,
  makeCompleteInventory,
  makeItem,
  known,
  unknown,
} from "./target-inventory-fixtures.ts";
import { schema } from "./helpers/target-inventory-assess.ts";

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
    (v: Record<string, any>) => {
      delete v.sections.security;
    },
    (v: Record<string, any>) => {
      v.secretExtra = "marker";
    },
    (v: Record<string, any>) => {
      v.schemaVersion = 2;
    },
    (v: Record<string, any>) => {
      v.inventoryId = "../private";
    },
    (v: Record<string, any>) => {
      v.date = "yesterday";
    },
    (v: Record<string, any>) => {
      v.sections.dataMetadata.items[0].disposition = "maybe";
    },
    (v: Record<string, any>) => {
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
  const feature: Record<string, any> = {
    ...makeItem(),
    id: "person-accounts",
    category: "org-feature",
    details: { name: known("personAccounts"), enabled: known(false) },
  };
  v.sections.sourceOrg.items.push(
    feature as unknown as (typeof v.sections.sourceOrg.items)[number],
  );
  const volume: Record<string, any> = {
    ...makeItem(),
    id: "account-count",
    category: "data-volume",
    details: {
      name: known("Account"),
      count: known(0),
      unit: known("records"),
    },
  };
  v.sections.dataMetadata.items.push(
    volume as unknown as (typeof v.sections.dataMetadata.items)[number],
  );
  assert.ok(validate(v), JSON.stringify(validate.errors));
  volume.details.count = unknown();
  assert.ok(validate(v));
  volume.details.count = known(-1);
  assert.equal(validate(v), false);
  volume.details.count = "unknown";
  assert.equal(validate(v), false);
  const client: Record<string, any> = {
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
  v.sections.clientsIntegrations.items.push(
    client as unknown as (typeof v.sections.clientsIntegrations.items)[number],
  );
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
