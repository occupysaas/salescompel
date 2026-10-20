import { readFile, readdir } from "node:fs/promises";
import { join, dirname, basename, resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import { privateJSON, recordPath } from "./external-id-recovery.ts";
import {
  bodyManifest,
  bodyScope,
  validateBodyReport,
  type BodyReport,
  type BodyCleanup,
  type BodyAssessment,
} from "./external-id-body-contract.ts";
import {
  bodyDigest,
  assertBodyIdentityNotLatched,
  bodyCleanupProofName,
  readBodyRun,
  knownBodyRecords,
  bodyRow,
  bodyOwned,
  type BodyStateSnapshot,
} from "./external-id-body-recovery.ts";
import type { ExternalIdConfig } from "./external-id-contract.ts";
import type { WireResponse } from "./wire.ts";
const requireEvidence = (ok: unknown) => {
  if (!ok) throw new Error("BODY_EVIDENCE_INVALID");
};
export const bodyArtifactsDirectory = (path: string) =>
  join(dirname(resolve(path)), basename(path, ".json") + ".artifacts");
async function artifact(dir: string, name: string, sha?: string) {
  requireEvidence(/^[A-Za-z0-9][A-Za-z0-9._-]*\.json$/.test(name));
  const value = await privateJSON(join(dir, name));
  if (sha !== undefined)
    requireEvidence(
      bodyDigest(await readFile(join(dir, name), "utf8")) === sha,
    );
  return value;
}
function wire(r: any): asserts r is WireResponse {
  requireEvidence(
    r &&
      Number.isInteger(r.status) &&
      r.status >= 100 &&
      r.status <= 599 &&
      r.headers &&
      typeof r.headers === "object" &&
      !Array.isArray(r.headers) &&
      Object.values(r.headers).every((v) => typeof v === "string") &&
      Object.hasOwn(r, "body") &&
      (r.bodyPresent === undefined || typeof r.bodyPresent === "boolean"),
  );
  requireEvidence(
    ![401, 403].includes(r.status) &&
      !(
        r.body &&
        typeof r.body === "object" &&
        Object.hasOwn(r.body, "nonJson")
      ),
  );
  if (r.bodyPresent === false) requireEvidence(r.body === null);
}
function discovery(r: any) {
  wire(r);
  const b = r.body as any;
  requireEvidence(
    r.status === 200 &&
      b?.done === true &&
      Array.isArray(b.records) &&
      b.totalSize === b.records.length &&
      !b.nextRecordsUrl,
  );
  const rows = b.records.map((row: any) =>
    bodyRow(row.Id, { status: 200, body: row, headers: {} }),
  );
  requireEvidence(new Set(rows.map((r: any) => r.Id)).size === rows.length);
  return rows;
}
function snapshot(s: BodyStateSnapshot) {
  requireEvidence(
    s &&
      Array.isArray(s.records) &&
      Array.isArray(s.discovery) &&
      Array.isArray(s.exactReads),
  );
  const rows = [];
  requireEvidence(
    new Set(s.exactReads.map((x) => x.id)).size === s.exactReads.length,
  );
  for (const x of s.exactReads) {
    recordPath(x.id);
    wire(x.response);
    if (x.response.status !== 404) rows.push(bodyRow(x.id, x.response));
  }
  rows.sort((a, b) => a.Id.localeCompare(b.Id));
  requireEvidence(equal(rows, s.records));
  const discovered = s.discovery.map((r) =>
    bodyRow(r.Id, { status: 200, body: r, headers: {} }),
  );
  requireEvidence(
    new Set(discovered.map((r) => r.Id)).size === discovered.length,
  );
  requireEvidence(discovered.every((r) => s.records.some((x) => equal(x, r))));
  return s;
}
/** Validate cleanup from private evidence, also used before a subsequent capture. */
export async function validateBodyCleanupEvidence(
  config: ExternalIdConfig,
  dir: string,
  cleanup: BodyCleanup,
  full = false,
) {
  await privateDirectory(dir);
  const run = await readBodyRun(config, dir);
  await assertBodyIdentityNotLatched(dir);
  requireEvidence(
    cleanup.outcome === "clean" &&
      !cleanup.reasons.length &&
      !cleanup.unresolvedOperationIds.length &&
      Number.isFinite(Date.parse(cleanup.startedAt)) &&
      Date.parse(cleanup.finishedAt) >= Date.parse(cleanup.startedAt),
  );
  requireEvidence(
    [...run.stages.values()].every((s) => s === "response" || s === "intent"),
  );
  requireEvidence(
    bodyDigest(await readFile(join(dir, "journal.jsonl"), "utf8")) ===
      cleanup.journalSha256,
  );
  const proof = await artifact(dir, bodyCleanupProofName(cleanup));
  requireEvidence(
    Object.keys(proof).sort().join() === "cleanup,discoveryArtifacts" &&
      equal(proof.cleanup, cleanup) &&
      Array.isArray(proof.discoveryArtifacts),
  );
  const cases = new Set(
    run.events
      .filter((e) => e.kind === "dispatched" && !e.id.startsWith("delete:"))
      .map((e) => e.id.split(":")[0]),
  );
  if (full) requireEvidence(cases.size === 16);
  requireEvidence(proof.discoveryArtifacts.length === cases.size);
  const seen = new Set<string>();
  for (const ref of proof.discoveryArtifacts) {
    requireEvidence(
      Object.keys(ref).sort().join() === "artifact,caseId,sha256" &&
        cases.has(ref.caseId) &&
        !seen.has(ref.caseId) &&
        /^[a-f0-9]{64}$/.test(ref.sha256),
    );
    seen.add(ref.caseId);
    const d = await artifact(dir, ref.artifact, ref.sha256);
    requireEvidence(
      d.caseId === ref.caseId && discovery(d.response).length === 0,
    );
  }
  const known = await knownBodyRecords(config, dir);
  requireEvidence(
    cleanup.records.length === known.size &&
      new Set(cleanup.records.map((r) => r.id)).size === known.size,
  );
  const files = await readdir(dir);
  for (const r of cleanup.records) {
    requireEvidence(
      known.get(r.id) === r.caseId &&
        typeof r.absenceSha256 === "string" &&
        /^[a-f0-9]{64}$/.test(r.absenceSha256),
    );
    const a = await artifact(dir, r.absenceArtifact, r.absenceSha256);
    wire(a.response);
    requireEvidence(
      a.id === r.id && a.caseId === r.caseId && a.response.status === 404,
    );
  }
  // A completed deletion needs both its available response and independently read ownership.
  for (const [op, stage] of run.stages) {
    if (!op.startsWith("delete:")) continue;
    requireEvidence(stage === "response");
    const id = op.slice(7),
      caseId = known.get(id),
      c = bodyManifest.find((c) => c.id === caseId);
    requireEvidence(c);
    const response = await artifact(dir, `response-delete-${id}.json`);
    wire(response);
    requireEvidence([204, 404].includes(response.status));
    let owned = false;
    for (const name of files.filter((n) =>
      n.startsWith(`cleanup-state-${caseId}-`),
    )) {
      const s = snapshot(await artifact(dir, name));
      if (
        s.records.some((row) => row.Id === id && bodyOwned(run.runId, c!, row))
      )
        owned = true;
    }
    requireEvidence(owned);
  }
  return { run, known };
}
async function load(path: string) {
  await privateDirectory(dirname(resolve(path)));
  const r = await privateJSON(resolve(path));
  validateBodyReport(r);
  const dir = bodyArtifactsDirectory(path);
  const { run, known } = await validateBodyCleanupEvidence(
    r.config,
    dir,
    r.cleanup,
    true,
  );
  requireEvidence(
    run.runId === r.runId &&
      r.collection === "complete" &&
      r.identityVerified &&
      !r.reasons.length,
  );
  requireEvidence(
    Date.parse(r.startedAt) <= Date.parse(r.cleanup.startedAt) &&
      Date.parse(r.cleanup.finishedAt) <= Date.parse(r.finishedAt),
  );
  requireEvidence(equal(await artifact(dir, "report.json"), r));
  const files = await readdir(dir);
  // Inspect independently retained responses too, including partial-failure evidence.
  const originals: any[] = [];
  const snapshots: {
    caseId: string;
    state: BodyStateSnapshot;
    cleanup?: boolean;
  }[] = [];
  for (const name of files) {
    if (name.startsWith("discovery-")) {
      const d = await artifact(dir, name);
      requireEvidence(bodyManifest.some((c) => c.id === d.caseId));
      discovery(d.response);
      originals.push(d);
    }
    if (name.startsWith("read-")) {
      const a = await artifact(dir, name);
      wire(a.response);
      recordPath(a.id);
      if (a.response.status !== 404) bodyRow(a.id, a.response);
      originals.push(a);
    }
    if (name.startsWith("absence-")) {
      const a = await artifact(dir, name);
      wire(a.response);
      requireEvidence(
        a.response.status === 404 && known.get(a.id) === a.caseId,
      );
    }
    if (name.startsWith("cleanup-state-")) {
      const c = bodyManifest.find((c) =>
        name.startsWith(`cleanup-state-${c.id}-`),
      );
      requireEvidence(c);
      snapshots.push({
        caseId: c!.id,
        state: snapshot(await artifact(dir, name)),
        cleanup: true,
      });
    }
  }
  const operationIds = new Set<string>();
  for (const c of bodyManifest) {
    const o = r.observations.find((o) => o.caseId === c.id);
    requireEvidence(o);
    const v = o!;
    operationIds.add(c.id + ":request");
    if (c.recordState === "existing") operationIds.add(c.id + ":seed");
    const response = await artifact(dir, v.artifact, v.sha256);
    wire(response);
    requireEvidence(
      v.artifact === `response-${c.id}-request.json` &&
        response.status === v.status &&
        equal(response.body, v.body) &&
        (response.bodyPresent ?? response.body !== null) === v.bodyPresent,
    );
    const state = await artifact(dir, v.stateArtifact, v.stateSha256);
    requireEvidence(
      v.stateArtifact === `state-${c.id}.json` &&
        state.caseId === c.id &&
        !state.error,
    );
    const before = snapshot(state.before),
      after = snapshot(state.after);
    requireEvidence(
      equal(before.records, v.before) &&
        equal(after.records, v.after) &&
        equal(await artifact(dir, `before-${c.id}.json`), before),
    );
    const collision = await artifact(dir, `collision-${c.id}.json`);
    requireEvidence(collision.caseId === c.id && equal(collision.records, []));
    for (const s of [before, after]) {
      requireEvidence(
        originals.some(
          (a) =>
            a.caseId === c.id &&
            a.response?.status === 200 &&
            Array.isArray(a.response.body?.records) &&
            equal(discovery(a.response), s.discovery),
        ),
      );
      for (const x of s.exactReads)
        requireEvidence(
          originals.some(
            (a) =>
              a.caseId === c.id &&
              a.id === x.id &&
              equal(a.response, x.response),
          ),
        );
    }
    snapshots.push(
      { caseId: c.id, state: before },
      { caseId: c.id, state: after },
    );
    const scope = bodyScope(r.runId, c);
    if (c.recordState === "existing") {
      const seed = await artifact(dir, v.seedArtifact!, v.seedSha256);
      wire(seed);
      requireEvidence(
        v.seedArtifact === `response-${c.id}-seed.json` &&
          seed.status === 201 &&
          (seed.body as any)?.success === true &&
          typeof (seed.body as any)?.id === "string",
      );
      requireEvidence(
        v.before.length === 1 &&
          v.before[0].Id === (seed.body as any).id &&
          v.before[0].Name === scope.seedName &&
          v.before[0].Compat_Key__c === scope.pathKey,
      );
    } else requireEvidence(!v.before.length);
    for (const row of [...v.before, ...v.after])
      requireEvidence(known.get(row.Id) === c.id);
  }
  requireEvidence(
    [...operationIds].every((id) => run.stages.get(id) === "response"),
  );
  requireEvidence(
    [...run.stages.keys()].every(
      (id) => id.startsWith("delete:") || operationIds.has(id),
    ),
  );
  for (const original of originals) {
    const candidates = snapshots
      .filter((s) => s.caseId === original.caseId)
      .map((s) => s.state);
    requireEvidence(
      original.id
        ? candidates.some((s) =>
            s.exactReads.some(
              (x) =>
                x.id === original.id && equal(x.response, original.response),
            ),
          )
        : discovery(original.response).length === 0 ||
            candidates.some((s) =>
              equal(s.discovery, discovery(original.response)),
            ),
    );
  }
  for (const c of bodyManifest) {
    const states = snapshots
      .filter((s) => s.caseId === c.id)
      .map((s) => s.state);
    requireEvidence(
      originals.filter((a) => a.caseId === c.id && a.id).length ===
        states.reduce((n, s) => n + s.exactReads.length, 0),
    );
    requireEvidence(
      originals.filter((a) => a.caseId === c.id && !a.id).length ===
        states.length + 1,
    );
  }
  const changedCleanupCases = new Set<string>();
  for (const c of bodyManifest) {
    const o = r.observations.find((o) => o.caseId === c.id)!;
    const observed = new Set([...o.before, ...o.after].map((row) => row.Id));
    const cleaned = r.cleanup.records
      .filter((row) => row.caseId === c.id)
      .map((row) => row.id);
    if (
      cleaned.length !== observed.size ||
      cleaned.some((id) => !observed.has(id)) ||
      snapshots.some(
        (s) =>
          s.caseId === c.id && s.cleanup && !equal(s.state.records, o.after),
      )
    )
      changedCleanupCases.add(c.id);
  }
  return { report: r, changedCleanupCases };
}
function expected(
  r: BodyReport,
  caseId: string,
  changedCleanup: boolean,
): "ready" | "different" | "blocked" {
  const o = r.observations.find((o) => o.caseId === caseId)!,
    c = bodyManifest.find((c) => c.id === caseId)!,
    s = bodyScope(r.runId, c),
    b = o.body as any;
  if (c.valueKind !== "omitted" && c.valueKind !== "null") {
    if (o.status !== 400) return "different";
    if (
      !o.bodyPresent ||
      !Array.isArray(b) ||
      b.length !== 1 ||
      !b[0] ||
      typeof b[0] !== "object" ||
      Array.isArray(b[0]) ||
      Object.keys(b[0]).sort().join() !== "errorCode,message" ||
      typeof b[0].errorCode !== "string" ||
      typeof b[0].message !== "string"
    )
      return "blocked";
    return b[0].errorCode ===
      (c.valueKind === "object" ? "JSON_PARSER_ERROR" : "INVALID_FIELD") &&
      equal(o.before, o.after) &&
      !changedCleanup
      ? "ready"
      : "different";
  }
  if (o.status !== (c.recordState === "existing" ? 200 : 201))
    return "different";
  if (
    !o.bodyPresent ||
    !b ||
    typeof b.id !== "string" ||
    typeof b.success !== "boolean" ||
    typeof b.created !== "boolean" ||
    !Array.isArray(b.errors)
  )
    return "blocked";
  return !changedCleanup &&
    b.success === true &&
    b.created === (c.recordState === "missing") &&
    b.errors.length === 0 &&
    o.after.length === 1 &&
    o.after[0].Id === b.id &&
    (c.recordState === "missing" || b.id === o.before[0].Id) &&
    o.after[0].Name === s.requestName &&
    o.after[0].Compat_Key__c === s.pathKey &&
    o.after[0].Compat_Text__c === (o.before[0]?.Compat_Text__c ?? null)
    ? "ready"
    : "different";
}
function normalized(r: BodyReport) {
  const aliases = new Map<string, string>();
  for (const c of bodyManifest) {
    const o = r.observations.find((o) => o.caseId === c.id)!;
    let n = 0;
    for (const row of [...o.before, ...o.after])
      if (!aliases.has(row.Id)) aliases.set(row.Id, `${c.id}:id${n++}`);
  }
  const visit = (v: any): any =>
    typeof v === "string"
      ? (aliases.get(v) ?? v.replaceAll(`p11${r.runId}-`, "p11{run}-"))
      : Array.isArray(v)
        ? v.map(visit)
        : v && typeof v === "object"
          ? Object.fromEntries(Object.entries(v).map(([k, x]) => [k, visit(x)]))
          : v;
  return r.observations.map((o) =>
    visit({
      caseId: o.caseId,
      status: o.status,
      bodyPresent: o.bodyPresent,
      body: o.body,
      before: o.before,
      after: o.after,
    }),
  );
}
export async function assessBodyPair(
  firstPath: string,
  secondPath: string,
): Promise<BodyAssessment> {
  const result: BodyAssessment = {
    contractVersion: "plan11-observed-body-validation-v2",
    outcome: "blocked",
    reasons: [],
    findings: [],
    inputReportDigests: [],
  };
  try {
    for (const path of [firstPath, secondPath]) {
      await privateJSON(resolve(path));
      result.inputReportDigests.push(
        bodyDigest(await readFile(resolve(path), "utf8")),
      );
    }
    const first = await load(firstPath),
      second = await load(secondPath);
    const a = first.report,
      b = second.report;
    requireEvidence(
      a.runId !== b.runId &&
        equal(a.config, b.config) &&
        Date.parse(a.cleanup.finishedAt) < Date.parse(b.startedAt) &&
        Date.parse(a.finishedAt) <= Date.parse(b.startedAt),
    );
    const na = normalized(a),
      nb = normalized(b);
    result.findings = bodyManifest.map((c, i) => {
      const outcomes = [
        expected(a, c.id, first.changedCleanupCases.has(c.id)),
        expected(b, c.id, second.changedCleanupCases.has(c.id)),
      ];
      const outcome = outcomes.includes("blocked")
        ? "blocked"
        : outcomes.includes("different") || !equal(na[i], nb[i])
          ? "different"
          : "ready";
      return {
        caseId: c.id,
        outcome,
        reasons:
          outcome === "ready"
            ? []
            : [
                outcome === "blocked"
                  ? "MALFORMED_RESPONSE"
                  : "UNEXPECTED_OR_UNREPEATABLE_BEHAVIOR",
              ],
      };
    });
    result.outcome = result.findings.some((f) => f.outcome === "blocked")
      ? "blocked"
      : result.findings.some((f) => f.outcome === "different")
        ? "different"
        : "ready";
  } catch {
    result.reasons.push("BODY_EVIDENCE_INVALID_OR_INCOMPLETE");
  }
  return result;
}
