import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import {
  assessFoundationPair,
  type CleanupAbsenceEvidence,
} from "../src/foundation-assessment.ts";
import type { FoundationSidecar } from "../src/foundation-sidecar.ts";
import { foundationCoverage } from "../src/foundation-policy.ts";
import {
  assessFoundationFiles,
  foundationReleaseFindings,
} from "../src/foundation-assessment-command.ts";
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
const sfId = (n: number) => `001${String(n).padStart(12, "0")}AAA`;

function fixture(start: string, end: string) {
  const coverage = foundationCoverage();
  const results = Object.entries(coverage).map(([id, spec]) => {
    const observations: Record<string, any> = {};
    const assertions: { name: string; passed: boolean }[] = [];
    for (const label of spec.observations) {
      observations[label] = {
        status: 200,
        body: { Id: sfId(1), records: [], done: true, totalSize: 0 },
      };
      if (label === "global") observations[label].body = { sobjects: [] };
      if (label === "describe" || label === "lower-describe")
        observations[label].body = { name: "Account", fields: [] };
      if (
        (id === "foundation-routes" &&
          ["patch", "escaped-upsert", "delete"].includes(label)) ||
        (id === "foundation-cursors" &&
          ["update-pending", "delete-pending"].includes(label)) ||
        (id === "foundation-ids" &&
          [
            "short-update",
            "short-reference",
            "invalid-reference",
            "unlink",
            "short-delete",
          ].includes(label)) ||
        (id === "foundation-names" &&
          ["lower-json", "duplicate-alias", "duplicate-raw"].includes(label)) ||
        (id === "foundation-errors" && label === "delete")
      )
        observations[label] = { status: 204, body: null };
      if (
        (id === "foundation-names" && label.startsWith("denied-")) ||
        (id === "foundation-errors" &&
          !["after-failed-writes", "delete"].includes(label)) ||
        (id === "foundation-cursors" &&
          ["other-user", "unknown-locator", "malformed-locator"].includes(
            label,
          ))
      )
        observations[label] = {
          status: 404,
          body: [{ errorCode: "NOT_FOUND" }],
        };
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
                    Id: sfId(1000 + (index === 0 ? i : 200)),
                    Compat_Text__c: "same",
                  }))
                : [{ Id: sfId(1) }],
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
    const identities = {
      owned: sfId(Object.keys(coverage).indexOf(id) + 1),
      ...(id === "foundation-cursors"
        ? Object.fromEntries(
            Array.from({ length: 201 }, (_, i) => [`tie-${i}`, sfId(1000 + i)]),
          )
        : {}),
    };
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
  const cleanup = results.flatMap((r) =>
    Object.values(r.identities).map((id) => ({
      scenario: r.id,
      resource: `Account:${id}`,
      outcome: "passed",
    })),
  );
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
    identityDigest: "d".repeat(64),
    startedAt: start,
    finishedAt: end,
    publishedAt: end,
  };
  const absence: CleanupAbsenceEvidence = {
    schemaVersion: 1,
    captureSha256: hash(bytes),
    checkedAt: end,
    identityVerified: true,
    identityDigest: "d".repeat(64),
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

function revise(
  fixtureValue: ReturnType<typeof fixture>,
  edit: (report: any) => void,
) {
  const report = structuredClone(fixtureValue.report);
  edit(report);
  const bytes = JSON.stringify(report) + "\n";
  return {
    ...fixtureValue,
    report,
    bytes,
    sidecar: { ...fixtureValue.sidecar, captureSha256: hash(bytes) },
    absence: { ...fixtureValue.absence, captureSha256: hash(bytes) },
  };
}

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
      const result = report.results.find(
        (r: any) => r.id === "foundation-cursors",
      );
      result.observations["fresh-session"] = {
        status: 401,
        body: [{ errorCode: "INVALID_SESSION_ID" }],
      };
    });
  assert.equal(assess(failed(a), failed(b)).outcome, "blocked");
  const nullRead = (f: ReturnType<typeof fixture>) =>
    revise(f, (report) => {
      report.results.find(
        (r: any) => r.id === "foundation-routes",
      ).observations.read.body = null;
    });
  assert.equal(assess(nullRead(a), nullRead(b)).outcome, "blocked");
  const nullAfterWrite = (f: ReturnType<typeof fixture>) =>
    revise(f, (report) => {
      report.results.find(
        (r: any) => r.id === "foundation-errors",
      ).observations["after-failed-writes"].body = null;
    });
  assert.equal(assess(nullAfterWrite(a), nullAfterWrite(b)).outcome, "blocked");
  const fakeMutation = (f: ReturnType<typeof fixture>) =>
    revise(f, (report) => {
      report.results.find(
        (r: any) => r.id === "foundation-routes",
      ).observations.patch = { status: 200, body: {} };
    });
  assert.equal(assess(fakeMutation(a), fakeMutation(b)).outcome, "blocked");
});

test("concurrent passing assertion order does not cause drift", () => {
  const [a, b] = pair();
  const swapped = revise(b, (report) => {
    const assertions = report.results.find(
      (r: any) => r.id === "foundation-cursors",
    ).assertions;
    const ai = assertions.findIndex(
      (x: any) => x.name === "parallel-a: bounded JSON HTTP response",
    );
    const bi = assertions.findIndex(
      (x: any) => x.name === "parallel-b: bounded JSON HTTP response",
    );
    [assertions[ai], assertions[bi]] = [assertions[bi], assertions[ai]];
  });
  assert.equal(assess(a, swapped).outcome, "ready-with-assumptions");
});

test("all owned fixture IDs must appear in cleanup and absence evidence", () => {
  const [a, b] = pair();
  const missing = revise(b, (report) => {
    const removed = report.cleanup.findIndex(
      (c: any) => c.resource === `Account:${sfId(1000)}`,
    );
    report.cleanup.splice(removed, 1);
  });
  missing.absence.resources = missing.absence.resources.filter(
    (r) => r.resource !== `Account:${sfId(1000)}`,
  );
  assert.equal(assess(a, missing).outcome, "blocked");
  const mislabeled = revise(b, (report) => {
    report.cleanup.find(
      (c: any) => c.resource === `Account:${sfId(1000)}`,
    ).scenario = "foundation-pagination";
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
  const missingSidecar = assess({ ...a, sidecar: undefined as any }, b);
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

test("release input rejects duplicate findings and limits relevance to selected contracts", () => {
  assert.throws(() =>
    foundationReleaseFindings({
      schemaVersion: 1,
      findings: [
        {
          id: "duplicate",
          affectedContracts: ["query-locator-lifecycle"],
          blocksContractFreeze: true,
        },
        {
          id: "duplicate",
          affectedContracts: ["query-locator-lifecycle"],
          blocksContractFreeze: true,
        },
      ],
      blockers: [],
    }),
  );
  const findings = foundationReleaseFindings({
    schemaVersion: 1,
    findings: [
      {
        id: "cursor",
        affectedContracts: ["query-locator-lifecycle"],
        blocksContractFreeze: true,
        status: "reviewed-source-awaiting-live-reconciliation",
      },
    ],
    blockers: [{ id: "archive", affectedContracts: [], blocksFullM1R: true }],
  });
  assert.deepEqual(findings, [
    { id: "cursor", relevant: true, resolved: false },
    { id: "archive", relevant: false, resolved: false },
  ]);
});

test("documentary review details and supersession preserve live release blockers", () => {
  const document = {
    schemaVersion: 1,
    findings: [
      {
        id: "cursor",
        status: "reviewed-source-awaiting-live-reconciliation",
        affectedContracts: ["query-locator-lifecycle"],
        blocksContractFreeze: true,
        reviewDetail: { documentaryComplete: true, supersedes: ["older"] },
      },
      {
        id: "older",
        status: "blocked",
        affectedContracts: ["query-locator-lifecycle"],
        blocksContractFreeze: true,
      },
      {
        id: "documentary",
        status: "documentary-complete",
        affectedContracts: ["query-response-shape"],
        blocksContractFreeze: true,
      },
      {
        id: "resolved",
        status: "resolved",
        affectedContracts: ["error-shape"],
        blocksContractFreeze: true,
      },
      {
        id: "reconciled",
        status: "reviewed-and-reconciled",
        affectedContracts: ["error-shape"],
        blocksContractFreeze: true,
      },
    ],
    blockers: [{ id: "archive", affectedContracts: [], blocksFullM1R: true }],
  };
  assert.deepEqual(foundationReleaseFindings(document), [
    { id: "cursor", relevant: true, resolved: false },
    { id: "older", relevant: true, resolved: false },
    { id: "documentary", relevant: true, resolved: false },
    { id: "resolved", relevant: true, resolved: true },
    { id: "reconciled", relevant: true, resolved: true },
    { id: "archive", relevant: false, resolved: false },
  ]);
});
