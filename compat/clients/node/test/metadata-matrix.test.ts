import test from "node:test";
import assert from "node:assert/strict";
import {
  compileMetadataMatrix,
  compareMetadataInventories,
} from "../src/metadata-matrix.ts";
import {
  parseDiscoveryConfig,
  newInventory,
  addFact,
  addGap,
} from "../src/metadata-inventory.ts";
import { collectMetadata, projectMetadata } from "../src/metadata-discovery.ts";
import {
  finishInventory,
  validateInventory,
} from "../src/metadata-inventory.ts";
import { profile } from "../src/validate.ts";
async function inventory() {
  const i = newInventory(
    parseDiscoveryConfig({
      schemaVersion: 1,
      apiVersion: "65.0",
      profile: "rest-query-foundation",
      profileHash: (await profile("rest-query-foundation")).hash,
      origin: "https://reference.example.invalid",
    }),
  );
  i.identityVerified = { initial: true, final: true };
  i.operations.push(
    {
      id: "d",
      kind: "describe",
      persona: "restricted",
      component: "Account",
      state: "observed",
    },
    {
      id: "m",
      kind: "metadata",
      persona: "administrator",
      component: "Account",
      state: "observed",
    },
    {
      id: "r",
      kind: "retrieve",
      persona: "administrator",
      component: "Account",
      state: "observed",
    },
  );
  return i;
}
test("falseIsNotDeniedOrMissing and missingDescribeDoesNotProveEnforcement", async () => {
  const i = await inventory();
  addFact(i, "d", "describe", "restricted", "Account", "updateable", false);
  addGap(
    i,
    "d",
    "describe",
    "restricted",
    "Account.Compat_Text__c",
    "*",
    "FIELD_NOT_VISIBLE",
  );
  const m = compileMetadataMatrix(i);
  assert.equal(m.rows.find((x) => x.property === "updateable")?.value, false);
  assert.equal(
    m.rows.find((x) => x.property === "updateable")?.state,
    "observed",
  );
  assert.equal(m.rows.find((x) => x.property === "*")?.state, "unavailable");
  assert.doesNotMatch(JSON.stringify(m), /access denied|effective grant/i);
});
test("declaredGrantDoesNotProveEffectiveAccess and differentFactsProduceConflict", async () => {
  const i = await inventory();
  addFact(i, "d", "describe", "administrator", "Account", "updateable", false);
  addFact(i, "m", "metadata", "administrator", "Account", "updateable", true);
  assert.ok(compileMetadataMatrix(i).rows.every((x) => x.state !== "conflict"));
  addFact(i, "r", "retrieve", "administrator", "Account", "updateable", false);
  const row = compileMetadataMatrix(i).rows.find((x) => x.area === "metadata")!;
  assert.equal(row.state, "conflict");
  assert.deepEqual(row.sourceRefs, ["m", "r"]);
});
async function completeInventory() {
  const scope = (await inventory()).scope;
  const fields = {
    Account: ["Id", "Name", "Compat_Text__c", "Compat_Key__c"],
    Contact: ["Id", "FirstName", "LastName", "AccountId"],
  };
  const metadata = async (type: string, name: string) =>
    type === "CustomField"
      ? {
          fullName: name,
          type: "Text",
          length: 80,
          required: false,
          externalId: false,
          unique: false,
        }
      : {
          fullName: name,
          fieldPermissions: Object.entries(fields).flatMap(([o, names]) =>
            names.map((f) => ({
              field: `${o}.${f}`,
              readable: true,
              editable: false,
            })),
          ),
          objectPermissions: Object.keys(fields).map((object) => ({
            object,
            allowRead: true,
            allowCreate: false,
            allowEdit: false,
            allowDelete: false,
            viewAllRecords: false,
            modifyAllRecords: false,
          })),
        };
  const a = await collectMetadata(
    scope,
    {
      COMPAT_REFERENCE_URL: scope.origin,
      COMPAT_REFERENCE_TOKEN: "a",
      COMPAT_REFERENCE_RESTRICTED_TOKEN: "b",
    },
    {
      identity: async (p) => ({ user: p, org: "synthetic" }),
      describe: async (_p, object) => ({
        name: object,
        queryable: true,
        createable: false,
        updateable: false,
        deletable: false,
        fields: fields[object as keyof typeof fields].map((name) => ({
          name,
          type: "string",
          length: 80,
          nillable: true,
          externalId: false,
          referenceTo: [],
          createable: false,
          updateable: false,
        })),
      }),
      metadata,
    },
  );
  a.operations.push({
    id: "retrieve",
    kind: "retrieve",
    persona: "administrator",
    component: "synthetic",
    state: "observed",
  });
  for (const [type, name] of [
    ["CustomField", "Account.Compat_Text__c"],
    ["CustomField", "Account.Compat_Key__c"],
    ["PermissionSet", "Compat_Admin"],
  ] as const)
    projectMetadata(
      a,
      type,
      name,
      await metadata(type, name),
      "retrieve",
      "retrieve",
    );
  finishInventory(a);
  assert.equal(a.outcome, "complete");
  return a;
}
test("enumerationOrderDoesNotProduceDrift; changed values drift; omitted evidence blocks", async () => {
  const a = await completeInventory();
  a.startedAt = "2026-01-01T00:00:00.000Z";
  a.finishedAt = "2026-01-01T00:01:00.000Z";
  const b = structuredClone(a);
  b.startedAt = "2026-01-01T00:02:00.000Z";
  b.finishedAt = "2026-01-01T00:03:00.000Z";
  b.facts.reverse();
  b.operations.reverse();
  assert.equal(compareMetadataInventories(a, b).outcome, "stable");
  b.facts.find(
    (f) => f.area === "describe" && f.property === "queryable",
  )!.value = false;
  assert.equal(compareMetadataInventories(a, b).outcome, "drift");
  b.facts.pop();
  assert.throws(() => validateInventory(b), /coverage/);
  assert.equal(compareMetadataInventories(a, b).outcome, "blocked");
  const c = structuredClone(a);
  c.operations = c.operations.filter((o) => o.kind !== "retrieve");
  c.facts = c.facts.filter((f) => f.area !== "retrieve");
  assert.throws(() => validateInventory(c), /coverage/);
});
test("scopeOrToolchainMismatchBlocks and incomplete evidence cannot pass", async () => {
  const a = await inventory(),
    b = structuredClone(a);
  b.scope.profiles = ["different"];
  assert.equal(compareMetadataInventories(a, b).outcome, "blocked");
  b.scope = a.scope;
  b.connectors.jsforce = "0";
  assert.equal(compareMetadataInventories(a, b).outcome, "blocked");
  b.connectors = a.connectors;
  assert.equal(compareMetadataInventories(a, b).outcome, "blocked");
});
