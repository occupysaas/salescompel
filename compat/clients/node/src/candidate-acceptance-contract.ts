import { readFileSync } from "node:fs";
import { open, link, unlink, realpath, lstat } from "node:fs/promises";
import { resolve, dirname, join } from "node:path";
import { randomBytes } from "node:crypto";
import { Ajv } from "ajv";
import { ROOT } from "./versions.ts";
export const gateIDs = [
  "bindings",
  "database",
  "lab",
  "web",
  "jsforce",
  "python",
  "cli",
  "restart",
  "connected-browser",
  "preview",
  "cleanup",
] as const;
export type GateID = (typeof gateIDs)[number];
export type Outcome = "passed" | "failed" | "blocked";
export type Scope = "full" | "candidate-only";
export interface ParsedResult {
  schemaVersion: 1;
  runId: string;
  suite: GateID;
  complete: boolean;
  exitCode: number | null;
  tests: { id: string; outcome: "passed" | "failed" | "skipped" }[];
}
export const candidateNames = [
  "candidate/jsforce/core",
  "candidate/jsforce/policy",
  "candidate/jsforce/isolation",
];
export const requiredCoverage: Record<GateID, readonly string[]> = {
  bindings: ["bindings/source", "bindings/tools", "bindings/artifacts"],
  database: [
    "TestCatalogFreshV2",
    "TestCatalogUpgradeV1",
    "TestCatalogUpgradeOversizedRollback",
    "TestCatalogRejectsUnknown",
    "TestCatalogV2PhysicalMismatch",
    "TestCatalogConcurrentOpen",
    "TestHTTPBoundary",
    "TestHTTPQueryColumns",
    "TestHTTPQueryColumnsPolicy",
    "TestHTTPExternalIDPreservesEscapedSlashes",
    "TestHTTPRejectsUndeliverablePathsAndListings",
    "TestHTTPCustomFieldLengths",
    "TestHTTPUpsertBodyValidation",
    "TestHTTPUpsertBodyBoundaryControls",
    "TestHTTPOrdinaryExternalIDWrites",
    "TestStoreIsolationAndPersistence",
    "TestStoreQueryPolicyAndRelationships",
    "TestStoreConcurrentUpdatesAndUpsert",
    "TestStoreBlankNillableStringsAreNull",
    "TestStoreUpsertBodyValidation",
    "database/vet",
  ],
  lab: [
    "core.test.ts::configuration blocks missing credentials/version/ack and unsafe endpoint",
    "preflight.test.ts::repository validates offline with explicit observation coverage",
    "candidate-acceptance-contract.test.ts::coverage refuses empty, skipped and incomplete required client results",
  ],
  web: [
    "i18n.test.ts::language selection preserves aliases, variants and fallback",
    "preferences.test.ts::only latest async locale selection commits, even when promises resolve backwards",
    "preferences.test.ts::preferences accept valid values and survive unavailable storage",
    "preview-store.test.ts::preview stores isolate fixtures and reject invalid references without mutation",
    "preview-store.test.ts::reset does not reuse record identities retained in browser history",
    "web/build",
  ],
  jsforce: candidateNames,
  python: [
    "python/relationship",
    "python/update",
    "python/denied",
    "python/cleanup",
  ],
  cli: [
    "cli/login",
    "cli/create",
    "cli/read",
    "cli/update",
    "cli/query",
    "cli/query-contact",
    "cli/query-empty",
    "cli/delete",
    "cli/absence",
  ],
  restart: ["restart/identity", "restart/values", "restart/policy"],
  "connected-browser": [
    "connected/admin",
    "invalid session displays an error without entering records",
    "connected/restricted",
    "connected/other-tenant",
    "connected/locales",
    "connected/token-boundary",
    "a failed refresh after save cannot leave a stale actionable record",
    "a failed refresh after delete cannot leave a stale actionable record",
  ],
  preview: [
    "routed frame has evidence, history, safe unknown routes and no API calls",
    "synthetic edit, cancel, create, validation, reload and reset",
    "contacts link to accounts and related lists stay scoped",
    "theme, language and RTL mobile drawer remain usable",
    "blocked storage falls back and connected page remains reachable",
    "long sample text, forms and evidence fit desktop and mobile in both themes",
    "leaving the connected route returns to a credential-free preview and requires reconnect",
    "pending connected reads cannot dispatch follow-up requests inside preview",
    "primary save button retains AA contrast on hover in both themes",
    "light is the default even when the browser prefers dark",
    "observed sidebar groups expose supported records and disable planned destinations",
  ],
  cleanup: ["cleanup/records", "cleanup/processes"],
};
export const categories = [
  "PREREQUISITE",
  "SOURCE",
  "COVERAGE",
  "RESULT",
  "PROCESS",
  "TIMEOUT",
  "SIGNAL",
  "FIXTURES",
  "CLEANUP",
  "PUBLICATION",
  "ASSERTION",
] as const;
export interface GateResult {
  id: GateID;
  outcome: Outcome | "not-run";
  coverage: {
    expected: string[];
    observedRequired: string[];
    passed: number;
    failed: number;
    skipped: number;
    allowedSkipped: string[];
  };
  categories: string[];
  startedAt: string | null;
  finishedAt: string | null;
}
export interface Bindings {
  revision: string;
  source: string;
  clean: boolean;
  locks: Record<string, string>;
  tools: Record<string, string>;
  binary: string;
  assets: string;
}
export interface AcceptanceReport {
  schemaVersion: 1;
  kind: "candidate-acceptance";
  runId: string;
  scope: Scope;
  outcome: Outcome;
  acceptanceEligible: boolean;
  startedAt: string;
  finishedAt: string;
  bindings: Bindings;
  gates: GateResult[];
  cleanup: {
    verified: boolean;
    records: number;
    unknownIntents: number;
    retainedPrincipals: boolean;
    processesStopped: boolean;
  };
  evidence: { gate: GateID; sha256: string }[];
}
export const scopeGates = (s: Scope) =>
  s === "full"
    ? gateIDs
    : gateIDs.filter(
        (id) => !["database", "lab", "web", "preview"].includes(id),
      );
export function assessCoverage(id: GateID, r: ParsedResult): GateResult {
  const expected = [...requiredCoverage[id]],
    allowed = id === "lab" ? candidateNames : [],
    seen = new Set<string>();
  let valid =
    r.complete && r.exitCode === 0 && r.suite === id && r.tests.length > 0;
  for (const t of r.tests) {
    if (seen.has(t.id) || !["passed", "failed", "skipped"].includes(t.outcome))
      valid = false;
    seen.add(t.id);
    if (
      t.outcome === "failed" ||
      (t.outcome === "skipped" && !allowed.includes(t.id))
    )
      valid = false;
  }
  const observed = expected.filter((name) =>
    r.tests.some((t) => t.id === name && t.outcome === "passed"),
  );
  valid &&= observed.length === expected.length;
  return {
    id,
    outcome: valid ? "passed" : "failed",
    coverage: {
      expected,
      observedRequired: observed,
      passed: r.tests.filter((t) => t.outcome === "passed").length,
      failed: r.tests.filter((t) => t.outcome === "failed").length,
      skipped: r.tests.filter((t) => t.outcome === "skipped").length,
      allowedSkipped: r.tests
        .filter((t) => t.outcome === "skipped" && allowed.includes(t.id))
        .map((t) => t.id),
    },
    categories: valid ? [] : ["COVERAGE"],
    startedAt: null,
    finishedAt: null,
  };
}
export function newReport(runId: string, scope: Scope): AcceptanceReport {
  const now = new Date().toISOString();
  return {
    schemaVersion: 1,
    kind: "candidate-acceptance",
    runId,
    scope,
    outcome: "blocked",
    acceptanceEligible: false,
    startedAt: now,
    finishedAt: now,
    bindings: {
      revision: "",
      source: "",
      clean: false,
      locks: {},
      tools: {},
      binary: "",
      assets: "",
    },
    gates: gateIDs.map((id) => ({
      id,
      outcome: "not-run",
      coverage: {
        expected: [...requiredCoverage[id]],
        observedRequired: [],
        passed: 0,
        failed: 0,
        skipped: 0,
        allowedSkipped: [],
      },
      categories: [],
      startedAt: null,
      finishedAt: null,
    })),
    cleanup: {
      verified: false,
      records: 0,
      unknownIntents: 0,
      retainedPrincipals: true,
      processesStopped: false,
    },
    evidence: [],
  };
}
export function aggregateReport(r: AcceptanceReport): AcceptanceReport {
  const required = r.gates.filter((g) => scopeGates(r.scope).includes(g.id));
  const all =
    required.every((g) => g.outcome === "passed") &&
    r.cleanup.verified &&
    r.cleanup.processesStopped &&
    r.cleanup.unknownIntents === 0;
  const eligible =
    all &&
    r.scope === "full" &&
    r.bindings.clean &&
    [r.bindings.source, r.bindings.binary, r.bindings.assets].every((x) =>
      /^[a-f0-9]{64}$/.test(x),
    ) &&
    /^[a-f0-9]{40,64}$/.test(r.bindings.revision);
  return {
    ...r,
    outcome: all
      ? r.scope === "full" && !eligible
        ? "blocked"
        : "passed"
      : required.some((g) => g.outcome === "failed")
        ? "failed"
        : "blocked",
    acceptanceEligible: eligible,
  };
}
const validator = new Ajv({ allErrors: true }).compile(
  JSON.parse(
    readFileSync(
      resolve(ROOT, "compat/schemas/candidate-acceptance.schema.json"),
      "utf8",
    ),
  ),
);
export function validateAcceptanceReport(
  v: unknown,
): asserts v is AcceptanceReport {
  if (!validator(v)) throw new Error("RESULT");
  const r = v as AcceptanceReport,
    a = aggregateReport(r);
  if (
    a.outcome !== r.outcome ||
    a.acceptanceEligible !== r.acceptanceEligible ||
    r.gates.length !== gateIDs.length ||
    new Set(r.gates.map((g) => g.id)).size !== gateIDs.length
  )
    throw new Error("RESULT");
  for (const g of r.gates) {
    if (
      JSON.stringify(g.coverage.expected) !==
        JSON.stringify(requiredCoverage[g.id]) ||
      g.coverage.observedRequired.some(
        (x) => !requiredCoverage[g.id].includes(x),
      )
    )
      throw new Error("RESULT");
    if (
      g.outcome === "passed" &&
      (g.coverage.observedRequired.length !== g.coverage.expected.length ||
        g.coverage.failed ||
        g.coverage.skipped !== g.coverage.allowedSkipped.length)
    )
      throw new Error("RESULT");
  }
  for (const value of [r.startedAt, r.finishedAt])
    if (new Date(value).toISOString() !== value) throw new Error("RESULT");
  if (Date.parse(r.finishedAt) < Date.parse(r.startedAt))
    throw new Error("RESULT");
}
export async function publishAcceptanceReport(
  path: string,
  r: AcceptanceReport,
) {
  validateAcceptanceReport(r);
  const dir = dirname(resolve(path));
  if ((await realpath(dir)) !== dir) throw new Error("PUBLICATION");
  const tmp = join(dir, ".acceptance-" + randomBytes(8).toString("hex"));
  const f = await open(tmp, "wx", 0o600);
  try {
    await f.writeFile(JSON.stringify(r, null, 2) + "\n");
    await f.sync();
  } finally {
    await f.close();
  }
  try {
    await link(tmp, resolve(path));
    const d = await open(dir, "r");
    try {
      await d.sync();
    } finally {
      await d.close();
    }
  } finally {
    await unlink(tmp);
  }
}
