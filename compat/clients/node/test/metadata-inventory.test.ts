import test from "node:test";
import assert from "node:assert/strict";
import {
  parseDiscoveryConfig,
  newInventory,
  validateInventory,
  addFact,
} from "../src/metadata-inventory.ts";
import { profile } from "../src/validate.ts";
const config = async () => ({
  schemaVersion: 1,
  apiVersion: "65.0",
  profile: "rest-query-foundation",
  profileHash: (await profile("rest-query-foundation")).hash,
  origin: "https://reference.example.invalid",
});
test("rejectSecretsWildcardsAndUnknownKeys", async () => {
  const c = await config();
  for (const x of [
    { ...c, token: "secret" },
    { ...c, operation: "deploy" },
    { ...c, profiles: ["*"] },
    { ...c, profiles: ["bad\nname"] },
    { ...c, permissionSets: Array(21).fill("P") },
    { ...c, origin: "https://user:pass@example.invalid" },
    { ...c, origin: "https://reference.example.invalid/path" },
  ])
    assert.throws(() => parseDiscoveryConfig(x));
  assert.deepEqual(parseDiscoveryConfig(c).profiles, []);
  assert.deepEqual(
    parseDiscoveryConfig({
      ...c,
      profiles: ["Name with spaces; $(not-a-command)"],
    }).profiles,
    ["Name with spaces; $(not-a-command)"],
  );
});
test("rejectProfileHashAndApiMismatch", async () => {
  const c = await config();
  assert.throws(
    () => parseDiscoveryConfig({ ...c, profileHash: "0".repeat(64) }),
    /binding/,
  );
  assert.throws(() => parseDiscoveryConfig({ ...c, apiVersion: "64.0" }));
});
test("preserveFalseVersusMissing and rejectMalformedInventory", async () => {
  const inv = newInventory(parseDiscoveryConfig(await config()));
  inv.operations.push({
    id: "describe:administrator:Account",
    kind: "describe",
    persona: "administrator",
    component: "Account",
    state: "observed",
  });
  addFact(
    inv,
    "describe:administrator:Account",
    "describe",
    "administrator",
    "Account",
    "updateable",
    false,
  );
  addFact(
    inv,
    "describe:administrator:Account",
    "describe",
    "administrator",
    "Account",
    "createable",
    undefined,
  );
  assert.equal(inv.facts[0].value, false);
  assert.equal(inv.gaps[0].code, "PROPERTY_UNAVAILABLE");
  validateInventory(inv);
  assert.throws(() => validateInventory({ ...inv, accessToken: "secret" }));
  const bad = structuredClone(inv);
  bad.facts[0].sourceRef = "missing";
  assert.throws(() => validateInventory(bad));
});

test("truncated inventories cannot claim complete coverage", async () => {
  const inv = newInventory(parseDiscoveryConfig(await config()));
  inv.identityVerified = { initial: true, final: true };
  inv.outcome = "complete";
  inv.operations.push({
    id: "describe:administrator:Account",
    kind: "describe",
    persona: "administrator",
    component: "Account",
    state: "observed",
  });
  assert.throws(() => validateInventory(inv), /coverage/);
});
