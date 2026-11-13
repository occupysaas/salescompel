import { createHash } from "node:crypto";
import {
  assessFoundationPair,
  type CleanupAbsenceEvidence,
  type ReleaseFinding,
} from "../../src/foundation-assessment.ts";
import type { FoundationSidecar } from "../../src/foundation-sidecar.ts";
import { foundationCoverage } from "../../src/foundation-policy.ts";

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
export const sfId = (n: number) => `001${String(n).padStart(12, "0")}AAA`;

export function fixture(start: string, end: string) {
  const coverage = foundationCoverage();
  const results = Object.entries(coverage).map(([id, spec]) => {
    const observations: Record<string, Record<string, unknown>> = {};
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

export const pair = () =>
  [
    fixture("2026-10-19T12:00:00.000Z", "2026-10-19T12:01:00.000Z"),
    fixture("2026-10-19T13:00:00.000Z", "2026-10-19T13:01:00.000Z"),
  ] as const;

export function assess(
  a = pair()[0],
  b = pair()[1],
  releases: ReleaseFinding[] = [],
) {
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

export function revise(
  fixtureValue: ReturnType<typeof fixture>,
  edit: (report: ReturnType<typeof fixture>["report"]) => void,
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
