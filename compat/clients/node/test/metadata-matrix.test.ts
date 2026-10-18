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
test("enumerationOrderDoesNotProduceDrift; values and missing facts do", async () => {
  const a = await inventory();
  a.outcome = "complete";
  a.startedAt = "2026-01-01T00:00:00.000Z";
  a.finishedAt = "2026-01-01T00:01:00.000Z";
  addFact(a, "d", "describe", "restricted", "Account", "updateable", false);
  addFact(a, "d", "describe", "restricted", "Account", "queryable", true);
  const b = structuredClone(a);
  b.startedAt = "2026-01-01T00:02:00.000Z";
  b.finishedAt = "2026-01-01T00:03:00.000Z";
  b.facts.reverse();
  b.operations.reverse();
  assert.equal(compareMetadataInventories(a, b).outcome, "stable");
  b.facts[0].value = false;
  assert.equal(compareMetadataInventories(a, b).outcome, "drift");
  b.facts.pop();
  assert.equal(compareMetadataInventories(a, b).outcome, "drift");
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
