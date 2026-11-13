import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import type { FoundationSidecar } from "../src/foundation-sidecar.ts";
import {
  assess,
  fixture,
  pair,
  revise,
  sfId,
} from "./helpers/foundation-assessment-fixture.ts";

const hash = (s: string) => createHash("sha256").update(s).digest("hex");

test("first absence must precede second capture", () => {
  const [a, b] = pair();
  assert.equal(
    assess(
      {
        ...a,
        absence: { ...a.absence, checkedAt: "2026-10-19T14:00:00.000Z" },
      },
      b,
    ).outcome,
    "blocked",
  );
});

test("pair must retain the same live identity", () => {
  const [a, b] = pair();
  assert.equal(
    assess(a, {
      ...b,
      sidecar: { ...b.sidecar, identityDigest: "e".repeat(64) },
      absence: { ...b.absence, identityDigest: "e".repeat(64) },
    }).outcome,
    "blocked",
  );
});

test("authentication failures and malformed positive reads cannot become evidence", () => {
  const [a, b] = pair();
  const failed = (f: ReturnType<typeof fixture>) =>
    revise(f, (report) => {
      const result = report.results.find((r) => r.id === "foundation-cursors")!;
      result.observations["fresh-session"] = {
        status: 401,
        body: [{ errorCode: "INVALID_SESSION_ID" }],
      };
    });
  assert.equal(assess(failed(a), failed(b)).outcome, "blocked");
  const nullRead = (f: ReturnType<typeof fixture>) =>
    revise(f, (report) => {
      report.results.find((r) => r.id === "foundation-routes")!.observations[
        "read"
      ].body = null;
    });
  assert.equal(assess(nullRead(a), nullRead(b)).outcome, "blocked");
  const nullAfterWrite = (f: ReturnType<typeof fixture>) =>
    revise(f, (report) => {
      report.results.find((r) => r.id === "foundation-errors")!.observations[
        "after-failed-writes"
      ].body = null;
    });
  assert.equal(assess(nullAfterWrite(a), nullAfterWrite(b)).outcome, "blocked");
  const fakeMutation = (f: ReturnType<typeof fixture>) =>
    revise(f, (report) => {
      report.results.find((r) => r.id === "foundation-routes")!.observations[
        "patch"
      ] = { status: 200, body: {} };
    });
  assert.equal(assess(fakeMutation(a), fakeMutation(b)).outcome, "blocked");
});

test("concurrent passing assertion order does not cause drift", () => {
  const [a, b] = pair();
  const swapped = revise(b, (report) => {
    const assertions = report.results.find(
      (r) => r.id === "foundation-cursors",
    )!.assertions;
    const ai = assertions.findIndex(
      (x) => x.name === "parallel-a: bounded JSON HTTP response",
    );
    const bi = assertions.findIndex(
      (x) => x.name === "parallel-b: bounded JSON HTTP response",
    );
    [assertions[ai], assertions[bi]] = [assertions[bi], assertions[ai]];
  });
  assert.equal(assess(a, swapped).outcome, "ready-with-assumptions");
});

test("all owned fixture IDs must appear in cleanup and absence evidence", () => {
  const [a, b] = pair();
  const missing = revise(b, (report) => {
    const removed = report.cleanup.findIndex(
      (c) => c.resource === `Account:${sfId(1000)}`,
    );
    report.cleanup.splice(removed, 1);
  });
  missing.absence.resources = missing.absence.resources.filter(
    (r) => r.resource !== `Account:${sfId(1000)}`,
  );
  assert.equal(assess(a, missing).outcome, "blocked");
  const mislabeled = revise(b, (report) => {
    report.cleanup.find(
      (c) => c.resource === `Account:${sfId(1000)}`,
    )!.scenario = "foundation-pagination";
  });
  assert.equal(assess(a, mislabeled).outcome, "blocked");
});

test("complete measured pair is ready only with named assumption", () => {
  const result = assess();
  assert.equal(result.outcome, "ready-with-assumptions");
  assert.equal(result.dimensions["cursor-expiry"], "assumed");
  assert.deepEqual(result.pairIssues, []);
});

test("binding, freshness, coverage and cleanup gaps block", () => {
  const [a, b] = pair();
  const missingSidecar = assess(
    { ...a, sidecar: undefined as unknown as FoundationSidecar },
    b,
  );
  assert.equal(missingSidecar.outcome, "blocked");
  assert.equal(missingSidecar.dimensions["foundation-routes"], "blocked");
  assert.equal(missingSidecar.dimensions["cursor-expiry"], "blocked");
  assert.equal(assess({ ...a, bytes: a.bytes + " " }, b).outcome, "blocked");
  assert.equal(
    assess(a, { ...b, absence: { ...b.absence, unresolvedJournals: 1 } })
      .outcome,
    "blocked",
  );
  assert.equal(
    assess(a, {
      ...b,
      absence: { ...b.absence, identityDigest: "e".repeat(64) },
    }).outcome,
    "blocked",
  );
  const bad = structuredClone(b.report);
  delete (bad.results[0].observations as unknown as { resources?: unknown })
    .resources;
  assert.equal(
    assess(a, { ...b, bytes: JSON.stringify(bad) + "\n" }).outcome,
    "blocked",
  );
});

test("measured differences drift, while unresolved release findings block separately", () => {
  const [a, b] = pair();
  const changed = structuredClone(b.report);
  changed.results[0].observations.resources.body = { changed: true };
  const bytes = JSON.stringify(changed) + "\n";
  const revised = {
    ...b,
    bytes,
    sidecar: { ...b.sidecar, captureSha256: hash(bytes) },
    absence: { ...b.absence, captureSha256: hash(bytes) },
  };
  assert.equal(assess(a, revised).outcome, "drift");
  const blocked = assess(a, b, [
    { id: "release-content-unavailable", relevant: true, resolved: false },
  ]);
  assert.equal(blocked.outcome, "blocked");
  assert.deepEqual(blocked.releaseIssues, ["release-content-unavailable"]);
});

test("complete tied payload changes are drift, not missing evidence", () => {
  const [a, b] = pair();
  const changed = structuredClone(b.report);
  (
    changed.results.find((r) => r.id === "foundation-cursors")!.observations[
      "ties:1"
    ] as unknown as { body: { records: Array<Record<string, unknown>> } }
  ).body.records[0].Extra = "changed";
  const bytes = JSON.stringify(changed) + "\n";
  const revised = {
    ...b,
    bytes,
    sidecar: { ...b.sidecar, captureSha256: hash(bytes) },
    absence: { ...b.absence, captureSha256: hash(bytes) },
  };
  assert.equal(assess(a, revised).outcome, "drift");
});
