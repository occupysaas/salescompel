import test from "node:test";
import assert from "node:assert/strict";
import {
  known,
  makeInventory,
  makeItem,
  makeCompleteInventory,
} from "./target-inventory-fixtures.ts";
import { assess, commandInventory } from "./helpers/target-inventory-assess.ts";

test("assessment distinguishes validity from readiness and keeps synthetic-only even on invalid data", () => {
  const v = makeInventory();
  assert.equal(assess(v).validation, "valid");
  assert.equal(assess(v).readiness, "synthetic-only");
  v.classification = "real";
  assert.equal(assess(v).readiness, "incomplete");
  assert.equal(
    assess(makeCompleteInventory("real")).readiness,
    "ready-for-review",
  );
  const bad: Record<string, any> = makeCompleteInventory("synthetic");
  bad.extra = true;
  assert.equal(assess(bad).readiness, "synthetic-only");
  assert.equal(assess(bad).validation, "invalid");
});

for (const disposition of ["required", "optional", "out-of-scope"] as const)
  test(`pending ${disposition} item decisions remain owned gaps and block complete readiness`, () => {
    const v = makeCompleteInventory("real"),
      item = v.sections.dataMetadata.items[0];
    item.disposition = disposition;
    item.decision = {
      question: "Is this still in scope?",
      nextAction: "review-scope",
    };
    const r = assess(v);
    assert.equal(r.validation, "invalid");
    assert.equal(r.readiness, "incomplete");
    assert.ok(r.issues.some((i) => i.code === "completion-contradiction"));
    assert.ok(
      r.gaps.some(
        (g) =>
          g.code === "pending-question" &&
          g.itemId === "account" &&
          g.ownerId === "data-owner" &&
          g.nextAction === "review-scope",
      ),
    );
    v.sections.dataMetadata.status = "partial";
    assert.equal(assess(v).validation, "valid");
    assert.equal(assess(v).readiness, "incomplete");
  });
test("command plugin IDs resolve to actual plugin items, not other categories", () => {
  for (const id of ["missing-plugin", "account"]) {
    const r = assess(commandInventory(id));
    assert.equal(r.validation, "invalid");
    assert.ok(
      r.issues.some(
        (i) => i.code === "dangling-reference" && i.itemId === "query-command",
      ),
    );
  }
  const v = commandInventory("data-plugin");
  v.sections.cliPlugins.items.push({
    ...makeItem(),
    id: "data-plugin",
    category: "plugin",
    details: {
      name: known("@salesforce/plugin-data"),
      version: known("5.1.16"),
      source: known("core"),
      signature: known("bundled"),
      license: known("Apache-2.0"),
      dependencies: known([]),
    },
  });
  assert.equal(assess(v).readiness, "ready-for-review");
  const cmd = v.sections.cliPlugins.items[0];
  if (cmd.category === "cli-command") cmd.details.pluginId = known("../unsafe");
  assert.equal(assess(v).validation, "invalid");
});
test("explicitly plugin-free commands can be complete without a fabricated plugin", () => {
  assert.equal(assess(commandInventory(null)).readiness, "ready-for-review");
});
