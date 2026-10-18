import test from "node:test";
import assert from "node:assert/strict";
import {
  parseExternalIdConfig,
  newExternalIdReport,
  validateExternalIdReport,
  manifest,
} from "../src/external-id-contract.ts";
import { profile } from "../src/validate.ts";
export async function config() {
  return parseExternalIdConfig({
    schemaVersion: 1,
    apiVersion: "65.0",
    profileHash: (await profile("rest-query-foundation")).hash,
    origin: "https://reference.example.invalid",
    administrator: { user: "synthetic-user", org: "synthetic-org" },
  });
}
test("external ID config rejects secret, origin, identity and binding changes", async () => {
  const c = await config();
  for (const bad of [
    { ...c, token: "secret" },
    { ...c, apiVersion: "64.0" },
    { ...c, origin: c.origin + "/path" },
    { ...c, profileHash: "0".repeat(64) },
    { ...c, administrator: { user: "" } },
  ])
    assert.throws(() => parseExternalIdConfig(bad));
});
test("external ID report requires manifest coverage and verified cleanup", async () => {
  const r = newExternalIdReport(await config(), "a".repeat(24));
  validateExternalIdReport(r);
  r.collection = "complete";
  r.cleanup = "clean";
  r.identityVerified = true;
  assert.throws(() => validateExternalIdReport(r));
  assert.equal(new Set(manifest.map((x) => x.id)).size, manifest.length);
});
test("external ID reports reject duplicate aliases and unknown keys", async () => {
  const r = newExternalIdReport(await config(), "b".repeat(24));
  r.aliases = [
    { id: "001000000000001AAA", alias: "a" },
    { id: "001000000000002AAA", alias: "a" },
  ];
  assert.throws(() => validateExternalIdReport(r));
  assert.throws(() => validateExternalIdReport({ ...r, extra: true }));
});
