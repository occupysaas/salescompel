import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import {
  assessFoundationPair,
  type CleanupAbsenceEvidence,
} from "../src/foundation-assessment.ts";
import type { FoundationSidecar } from "../src/foundation-sidecar.ts";
import { foundationCoverage } from "../src/foundation-policy.ts";
import { assessFoundationFiles } from "../src/foundation-assessment-command.ts";
import { mkdtemp, chmod, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";

const hash = (s: string) => createHash("sha256").update(s).digest("hex");
const binding = {
  profile: "rest-query-foundation",
  profileHash: "a".repeat(64),
  apiVersion: "65.0",
  connectors: { jsforce: "3.11.0" },
  probeDigest: "b".repeat(64),
  manifestDigest: "c".repeat(64),
};
const source =
  "https://developer.salesforce.com/docs/platform/salesforce-app-limits-cheatsheet/guide/salesforce-app-limits-platform-apicursors.html";

function fixture(start: string, end: string) {
  const coverage = foundationCoverage();
  const results = Object.entries(coverage).map(([id, spec]) => {
    const observations: Record<string, any> = {};
    const assertions: { name: string; passed: boolean }[] = [];
    for (const label of spec.observations) {
      observations[label] = { status: 200, body: {} };
      if (label === "versions")
        observations[label] = { status: 200, requestedVersionAdvertised: true };
      if (label.startsWith("own-user:"))
        observations[label] = { status: 200, hasOwnId: true };
      if (label === "nullEquality")
        observations[label] = { nullMatches: true, blankMatches: true };
      if (
        ![
          "versions",
          "own-user:65.0",
          "own-user:51.0",
          "nullEquality",
        ].includes(label)
      )
        assertions.push({
          name: `${label}: bounded JSON HTTP response`,
          passed: true,
        });
    }
    for (const prefix of spec.pages) {
      const labels =
        prefix === "ties" ? [`${prefix}:0`, `${prefix}:1`] : [`${prefix}:0`];
      for (const [index, label] of labels.entries()) {
        const done = index === labels.length - 1;
        observations[label] = {
          status: 200,
          body: {
            totalSize: prefix === "ties" ? 201 : 1,
            done,
            records:
              prefix === "ties"
                ? Array.from({ length: index === 0 ? 200 : 1 }, (_, i) => ({
                    Id: `001-${index === 0 ? i : 200}`,
                    Compat_Text__c: "same",
                  }))
                : [{ Id: "001-other" }],
            ...(!done
              ? {
                  nextRecordsUrl: {
                    route: "/services/data/v65.0/query/{queryLocator}",
                    value: "/services/data/v65.0/query/next",
                  },
                }
              : {}),
          },
        };
        for (const suffix of [
          "bounded JSON HTTP response",
          "query page",
          "stable total",
          ...(done ? [] : ["continuation required"]),
        ])
          assertions.push({ name: `${label}: ${suffix}`, passed: true });
      }
      assertions.push({
        name: `${prefix}: record count equals total`,
        passed: true,
      });
      assertions.push({
        name: `${prefix}: unique record identities`,
        passed: true,
      });
    }
    for (const name of spec.assertions) assertions.push({ name, passed: true });
    const identities =
      id === "foundation-cursors"
        ? Object.fromEntries(
            Array.from({ length: 201 }, (_, i) => [`tie-${i}`, `001-${i}`]),
          )
        : {};
    return {
      id,
      client: "jsforce",
      outcome: id === "foundation-cursors" ? "blocked" : "passed",
      ...(id === "foundation-cursors"
        ? {
            reason:
              "Cursor expiry unmeasured under foundation-query-result-lifetime",
          }
        : {}),
      observations,
      assertions,
      identities,
    };
  });
  const cleanup = results.map((r) => ({
    scenario: r.id,
    resource: `Account:${r.id}`,
    outcome: "passed",
  }));
  const report = {
    schemaVersion: 1,
    target: "reference",
    profile: binding.profile,
    profileHash: binding.profileHash,
    apiVersion: binding.apiVersion,
    provenance: "live",
    startedAt: start,
    finishedAt: end,
    connectors: binding.connectors,
    requiredScenarios: Object.keys(coverage),
    results,
    cleanup,
  };
  const bytes = JSON.stringify(report) + "\n";
  const sidecar: FoundationSidecar = {
    schemaVersion: 1,
    ...binding,
    policyVersion: 1,
    policy: "documented-two-day-assumption",
    assumptionId: "foundation-query-result-lifetime",
    source,
    captureSha256: hash(bytes),
    startedAt: start,
    finishedAt: end,
    publishedAt: end,
  };
  const absence: CleanupAbsenceEvidence = {
    schemaVersion: 1,
    captureSha256: hash(bytes),
    checkedAt: end,
    identityVerified: true,
    unresolvedJournals: 0,
    resources: cleanup.map((c) => ({ resource: c.resource, absent: true })),
  };
  return { bytes, sidecar, absence, report };
}

const pair = () =>
  [
    fixture("2026-10-19T12:00:00.000Z", "2026-10-19T12:01:00.000Z"),
    fixture("2026-10-19T13:00:00.000Z", "2026-10-19T13:01:00.000Z"),
  ] as const;

function assess(a = pair()[0], b = pair()[1], releases: any[] = []) {
  return assessFoundationPair(
    a.bytes,
    a.sidecar,
    b.bytes,
    b.sidecar,
    binding,
    [a.absence, b.absence],
    releases,
  );
}

test("complete measured pair is ready only with named assumption", () => {
  const result = assess();
  assert.equal(result.outcome, "ready-with-assumptions");
  assert.equal(result.dimensions["cursor-expiry"], "assumed");
  assert.deepEqual(result.pairIssues, []);
});

test("binding, freshness, coverage and cleanup gaps block", () => {
  const [a, b] = pair();
  assert.equal(
    assess({ ...a, sidecar: undefined as any }, b).outcome,
    "blocked",
  );
  assert.equal(assess({ ...a, bytes: a.bytes + " " }, b).outcome, "blocked");
  assert.equal(
    assess(a, { ...b, absence: { ...b.absence, unresolvedJournals: 1 } })
      .outcome,
    "blocked",
  );
  const bad = structuredClone(b.report);
  delete (bad.results[0].observations as any).resources;
  assert.equal(
    assess(a, { ...b, bytes: JSON.stringify(bad) + "\n" }).outcome,
    "blocked",
  );
});

test("measured differences drift, while unresolved release findings block separately", () => {
  const [a, b] = pair();
  const changed = structuredClone(b.report);
  (changed.results[0].observations as any).resources.body = { changed: true };
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
    changed.results.find((r) => r.id === "foundation-cursors")!
      .observations as any
  )["ties:1"].body.records[0].Extra = "changed";
  const bytes = JSON.stringify(changed) + "\n";
  const revised = {
    ...b,
    bytes,
    sidecar: { ...b.sidecar, captureSha256: hash(bytes) },
    absence: { ...b.absence, captureSha256: hash(bytes) },
  };
  assert.equal(assess(a, revised).outcome, "drift");
});

test("file command rejects missing sidecars before creating output", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "foundation-assessment-"));
  await chmod(dir, 0o700);
  t.after(() => rm(dir, { recursive: true, force: true }));
  await assert.rejects(
    assessFoundationFiles({
      first: join(dir, "one.json"),
      second: join(dir, "two.json"),
      firstPolicy: join(dir, "one.policy.json"),
      secondPolicy: join(dir, "two.policy.json"),
      firstCleanup: join(dir, "one.cleanup.json"),
      secondCleanup: join(dir, "two.cleanup.json"),
      out: join(dir, "assessment.json"),
    }),
  );
});
