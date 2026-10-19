import test from "node:test";
import assert from "node:assert/strict";
import {
  assertFoundationPolicy,
  expiryMeasurementRequired,
  runExpiryObservation,
  foundationCoverage,
} from "../src/foundation-policy.ts";

test("only explicit foundation reference policy omits expiry measurement", () => {
  assert.equal(expiryMeasurementRequired("observed-only"), true);
  assert.equal(
    expiryMeasurementRequired("documented-two-day-assumption"),
    false,
  );
  assert.doesNotThrow(() =>
    assertFoundationPolicy(
      "documented-two-day-assumption",
      "reference",
      "rest-query-foundation",
    ),
  );
  assert.throws(() =>
    assertFoundationPolicy(
      "documented-two-day-assumption",
      "candidate",
      "rest-query-foundation",
    ),
  );
  assert.throws(() =>
    assertFoundationPolicy(
      "documented-two-day-assumption",
      "reference",
      "pilot",
    ),
  );
  assert.throws(() =>
    assertFoundationPolicy("any-waiver", "reference", "rest-query-foundation"),
  );
});

test("coverage fixes all seven scenarios and preserves non-expiry cursor probes", () => {
  const coverage = foundationCoverage();
  assert.equal(Object.keys(coverage).length, 7);
  assert.deepEqual(coverage["foundation-cursors"].observations, [
    "initial",
    "continuation",
    "replay",
    "parallel-a",
    "parallel-b",
    "fresh-session",
    "other-user",
    "unknown-locator",
    "malformed-locator",
    "before-mutation",
    "update-pending",
    "delete-pending",
    "after-mutation",
  ]);
  assert.deepEqual(coverage["foundation-cursors"].pages, ["ties"]);
});

test("assumed lifetime omits only the expiry-specific requests and wait", async () => {
  const calls: string[] = [];
  const result = await runExpiryObservation(
    "documented-two-day-assumption",
    async () => {
      calls.push("expiry-start");
      return "locator";
    },
    async () => {
      calls.push("wait");
    },
    async () => {
      calls.push("after-16-minutes");
      return 200;
    },
  );
  assert.deepEqual(calls, []);
  assert.equal(result, "unmeasured");
});
