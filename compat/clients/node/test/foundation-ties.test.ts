import test from "node:test";
import assert from "node:assert/strict";
import { compareFoundationTies } from "../src/foundation-ties.ts";
import type { Result } from "../src/core.ts";

function fixture(
  order = Array.from({ length: 201 }, (_, i) => i),
  split = 200,
): Result {
  const identities = Object.fromEntries(
    Array.from({ length: 201 }, (_, i) => [`tie-${i}`, `001-${i}`]),
  );
  const rows = order.map((i) => ({ Id: `001-${i}`, Compat_Text__c: "same" }));
  const page = (records: typeof rows, done: boolean, next?: string) => ({
    status: 200,
    body: {
      totalSize: 201,
      done,
      records,
      ...(next
        ? {
            nextRecordsUrl: {
              route: "/services/data/v65.0/query/{queryLocator}",
              value: `/services/data/v65.0/query/${next}`,
            },
          }
        : {}),
    },
  });
  return {
    id: "foundation-cursors",
    client: "jsforce",
    outcome: "blocked",
    reason: "Cursor expiry unmeasured under foundation-query-result-lifetime",
    assertions: [],
    identities,
    observations: {
      "ties:0": page(rows.slice(0, split), false, "next"),
      "ties:1": page(rows.slice(split), true),
    },
  };
}

test("tie permutation and page movement are diagnostics after full traversal", () => {
  const a = fixture();
  const b = fixture(
    Array.from({ length: 201 }, (_, i) => 200 - i),
    199,
  );
  const result = compareFoundationTies(a, b);
  assert.deepEqual(result.issues, []);
  assert.ok(result.diagnostics.length > 0);
});

test("tie comparison rejects loss, duplication, foreign rows and changed values", () => {
  for (const mutate of [
    (r: Result) => ((r.observations as any)["ties:1"].body.records = []),
    (r: Result) =>
      ((r.observations as any)["ties:1"].body.records[0].Id = "001-0"),
    (r: Result) =>
      ((r.observations as any)["ties:1"].body.records[0].Id = "foreign"),
    (r: Result) =>
      ((r.observations as any)["ties:1"].body.records[0].Compat_Text__c =
        "different"),
  ]) {
    const b = fixture();
    mutate(b);
    assert.ok(compareFoundationTies(fixture(), b).issues.length > 0);
  }
});

test("tie comparison rejects failed pages, broken links, totals and truncation", () => {
  for (const mutate of [
    (r: Result) => ((r.observations as any)["ties:0"].status = 500),
    (r: Result) => delete (r.observations as any)["ties:0"].body.nextRecordsUrl,
    (r: Result) =>
      ((r.observations as any)["ties:1"].body.nextRecordsUrl = {
        route: "wrong",
        value: "wrong",
      }),
    (r: Result) => ((r.observations as any)["ties:1"].body.totalSize = 200),
    (r: Result) => delete (r.observations as any)["ties:1"],
  ]) {
    const b = fixture();
    mutate(b);
    assert.ok(compareFoundationTies(fixture(), b).issues.length > 0);
  }
});
