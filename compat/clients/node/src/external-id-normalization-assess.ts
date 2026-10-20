import { readFile, readdir } from "node:fs/promises";
import { join, dirname, basename, resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import { privateJSON, recordPath } from "./external-id-recovery.ts";
import {
  normalizationManifest,
  normalizationScope,
  normalizationRequest,
  validateNormalizationReport,
  type NormalizationReport,
  type NormalizationCleanup,
  type NormalizationAssessment,
} from "./external-id-normalization-contract.ts";
import {
  normalizationDigest,
  assertNormalizationIdentityNotLatched,
  normalizationCleanupProofName,
  readNormalizationRun,
  knownNormalizationRecords,
  normalizationRow,
  normalizationOwned,
  type NormalizationStateSnapshot,
} from "./external-id-normalization-recovery.ts";
import type { ExternalIdConfig } from "./external-id-contract.ts";
import type { WireResponse } from "./wire.ts";
const requireEvidence = (ok: unknown) => {
  if (!ok) throw new Error("NORMALIZATION_EVIDENCE_INVALID");
};
export const normalizationArtifactsDirectory = (path: string) =>
  join(dirname(resolve(path)), basename(path, ".json") + ".artifacts");
async function artifact(dir: string, name: string, sha?: string) {
  requireEvidence(/^[A-Za-z0-9][A-Za-z0-9._-]*\.json$/.test(name));
  const value = await privateJSON(join(dir, name));
  if (sha !== undefined)
    requireEvidence(
      normalizationDigest(await readFile(join(dir, name), "utf8")) === sha,
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
    normalizationRow(row.Id, { status: 200, body: row, headers: {} }),
  );
  requireEvidence(new Set(rows.map((r: any) => r.Id)).size === rows.length);
  return rows;
}
function snapshot(s: NormalizationStateSnapshot) {
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
    if (x.response.status !== 404)
      rows.push(normalizationRow(x.id, x.response));
  }
  rows.sort((a, b) => a.Id.localeCompare(b.Id));
  requireEvidence(equal(rows, s.records));
  const discovered = s.discovery.map((r) =>
    normalizationRow(r.Id, { status: 200, body: r, headers: {} }),
  );
  requireEvidence(
    new Set(discovered.map((r) => r.Id)).size === discovered.length,
  );
  requireEvidence(discovered.every((r) => s.records.some((x) => equal(x, r))));
  return s;
}
/** Validate cleanup from private evidence, also used before a subsequent capture. */
export async function validateNormalizationCleanupEvidence(
  config: ExternalIdConfig,
  dir: string,
  cleanup: NormalizationCleanup,
  full = false,
) {
  await privateDirectory(dir);
  const run = await readNormalizationRun(config, dir);
  await assertNormalizationIdentityNotLatched(dir);
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
    normalizationDigest(await readFile(join(dir, "journal.jsonl"), "utf8")) ===
      cleanup.journalSha256,
  );
  const proof = await artifact(dir, normalizationCleanupProofName(cleanup));
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
  if (full) requireEvidence(cases.size === 36);
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
  const known = await knownNormalizationRecords(config, dir);
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
      c = normalizationManifest.find((c) => c.id === caseId);
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
        s.records.some(
          (row) =>
            row.Id === id && normalizationOwned(run.runId, c!, row, run.stages),
        )
      )
        owned = true;
    }
    requireEvidence(owned);
  }
  return { run, known };
}
function mutationResponse(
  response: WireResponse,
  kind: "create" | "update" | "upsert",
) {
  wire(response);
  const b = response.body as any;
  if (response.status >= 200 && response.status < 300) {
    if (kind === "update")
      requireEvidence(response.status === 204 && b === null);
    else {
      requireEvidence(
        (kind === "create"
          ? response.status === 201
          : [200, 201].includes(response.status)) &&
          b &&
          typeof b.id === "string" &&
          b.success === true &&
          Array.isArray(b.errors) &&
          b.errors.length === 0,
      );
      recordPath(b.id);
      if (kind === "upsert")
        requireEvidence(
          typeof b.created === "boolean" &&
            b.created === (response.status === 201) &&
            Array.isArray(b.errors) &&
            !b.errors.length,
        );
    }
  } else
    requireEvidence(
      response.status >= 400 &&
        Array.isArray(b) &&
        b.length > 0 &&
        b.every(
          (e) =>
            e &&
            typeof e.errorCode === "string" &&
            typeof e.message === "string" &&
            (!Object.hasOwn(e, "fields") || Array.isArray(e.fields)),
        ),
    );
}
async function load(path: string) {
  await privateDirectory(dirname(resolve(path)));
  const r = await privateJSON(resolve(path));
  validateNormalizationReport(r);
  const dir = normalizationArtifactsDirectory(path),
    { run, known } = await validateNormalizationCleanupEvidence(
      r.config,
      dir,
      r.cleanup,
      true,
    );
  requireEvidence(
    run.runId === r.runId &&
      r.collection === "complete" &&
      r.identityVerified &&
      !r.reasons.length &&
      equal(await artifact(dir, "report.json"), r),
  );
  requireEvidence(
    Date.parse(r.startedAt) <= Date.parse(r.cleanup.startedAt) &&
      Date.parse(r.cleanup.finishedAt) <= Date.parse(r.finishedAt),
  );
  const files = await readdir(dir),
    rawDiscoveries: any[] = [],
    rawReads: any[] = [],
    cleanupStates: { caseId: string; state: NormalizationStateSnapshot }[] = [];
  for (const name of files) {
    if (name.startsWith("discovery-")) {
      const d = await artifact(dir, name);
      requireEvidence(normalizationManifest.some((c) => c.id === d.caseId));
      discovery(d.response);
      rawDiscoveries.push(d);
    }
    if (name.startsWith("read-")) {
      const d = await artifact(dir, name);
      wire(d.response);
      recordPath(d.id);
      if (d.response.status !== 404) normalizationRow(d.id, d.response);
      rawReads.push(d);
    }
    if (name.startsWith("cleanup-state-")) {
      const c = normalizationManifest.find((c) =>
        name.startsWith(`cleanup-state-${c.id}-`),
      );
      requireEvidence(c);
      cleanupStates.push({
        caseId: c!.id,
        state: snapshot(await artifact(dir, name)),
      });
    }
  }
  const allSnapshots: { caseId: string; state: NormalizationStateSnapshot }[] =
    [...cleanupStates];
  const operationIds = new Set<string>();
  for (const c of normalizationManifest) {
    const collision = await artifact(dir, `collision-${c.id}.json`);
    requireEvidence(
      collision.caseId === c.id && discovery(collision.response).length === 0,
    );
    const ids: Partial<Record<"first" | "second", string>> = {};
    let previous: unknown = [];
    for (const step of c.steps) {
      const o = r.observations.find(
        (o) => o.caseId === c.id && o.stepId === step.id,
      )!;
      const op = `${c.id}:${step.id}`;
      operationIds.add(op);
      requireEvidence(run.stages.get(op) === "response");
      requireEvidence(
        equal(o.request, normalizationRequest(r.runId, c, step, ids)),
      );
      requireEvidence(
        o.artifact === `response-${c.id}-${step.id}.json` &&
          o.stateArtifact === `state-${c.id}-${step.id}.json`,
      );
      const response = await artifact(dir, o.artifact, o.sha256);
      mutationResponse(response, step.kind);
      requireEvidence(
        response.status === o.status &&
          equal(response.body, o.body) &&
          (response.bodyPresent ?? response.body !== null) === o.bodyPresent,
      );
      const state = await artifact(dir, o.stateArtifact, o.stateSha256);
      requireEvidence(state.caseId === c.id && !state.error);
      const before = snapshot(state.before),
        after = snapshot(state.after);
      requireEvidence(
        equal(before.records, o.before) &&
          equal(after.records, o.after) &&
          equal(previous, o.before),
      );
      previous = o.after;
      for (const s of [before, after]) {
        allSnapshots.push({ caseId: c.id, state: s });
        requireEvidence(
          rawDiscoveries.some(
            (d) =>
              d.caseId === c.id && equal(discovery(d.response), s.discovery),
          ),
        );
        for (const x of s.exactReads)
          requireEvidence(
            rawReads.some(
              (d) =>
                d.caseId === c.id &&
                d.id === x.id &&
                equal(d.response, x.response),
            ),
          );
        for (const row of s.records)
          requireEvidence(known.get(row.Id) === c.id);
      }
      const b = response.body as any;
      if (step.id.startsWith("seed")) {
        requireEvidence(
          response.status === 201 &&
            o.after.some(
              (row) => row.Id === b.id && row.Name === o.request.body.Name,
            ),
        );
        if (step.id === "seed-second") requireEvidence(ids.first !== b.id);
        ids[step.id === "seed" ? "first" : "second"] = b.id;
      }
      if (typeof b?.id === "string") requireEvidence(known.get(b.id) === c.id);
    }
  }
  requireEvidence(
    [...run.stages.keys()].every(
      (id) => id.startsWith("delete:") || operationIds.has(id),
    ),
  );
  // Every independently retained state read must participate in a checked snapshot.
  for (const d of rawReads)
    requireEvidence(
      allSnapshots.some(
        (s) =>
          s.caseId === d.caseId &&
          s.state.exactReads.some(
            (x) => x.id === d.id && equal(x.response, d.response),
          ),
      ),
    );
  for (const d of rawDiscoveries)
    requireEvidence(
      discovery(d.response).length === 0 ||
        allSnapshots.some(
          (s) =>
            s.caseId === d.caseId &&
            equal(s.state.discovery, discovery(d.response)),
        ),
    );
  const changedCleanupCases = new Set<string>();
  for (const c of normalizationManifest) {
    const observations = r.observations.filter((o) => o.caseId === c.id),
      last = observations.at(-1)!;
    const seen = new Set(
      observations.flatMap((o) =>
        [...o.before, ...o.after].map((row) => row.Id),
      ),
    );
    const cleaned = r.cleanup.records.filter((row) => row.caseId === c.id);
    if (
      cleaned.length !== seen.size ||
      cleaned.some((row) => !seen.has(row.id)) ||
      // A later recovery can observe our own already verified deletions.
      // Missing rows without a completed journaled deletion remain drift.
      cleanupStates.some(
        (s) =>
          s.caseId === c.id &&
          (s.state.records.some(
            (row) => !last.after.some((expected) => equal(row, expected)),
          ) ||
            last.after.some(
              (row) =>
                !s.state.records.some((remaining) => remaining.Id === row.Id) &&
                run.stages.get(`delete:${row.Id}`) !== "response",
            )),
      )
    )
      changedCleanupCases.add(c.id);
  }
  return { report: r, changedCleanupCases };
}
function projected(r: NormalizationReport) {
  const aliases = new Map<string, string>();
  for (const c of normalizationManifest) {
    let index = 0;
    const ops = c.steps.map((s) =>
      r.observations.find((o) => o.caseId === c.id && o.stepId === s.id)!,
    );
    const add = (id: string) => {
      if (!aliases.has(id)) aliases.set(id, `${c.id}:id${index++}`);
    };
    for (const o of ops) {
      const b = o.body as any;
      if (typeof b?.id === "string") add(b.id);
    }
    for (const o of ops)
      for (const row of [...o.before, ...o.after].sort(
        (a, b) =>
          a.Name.localeCompare(b.Name) ||
          (a.Compat_Key__c ?? "").localeCompare(b.Compat_Key__c ?? ""),
      ))
        add(row.Id);
  }
  // Alias generated prefixes in their original and uppercase forms separately:
  // retain P14 versus p14 and all suffix spelling as measured behavior.
  const visit = (v: any): any =>
    typeof v === "string"
      ? (aliases.get(v) ??
        v
          .replaceAll(`p14${r.runId}-`, "p14{run}-")
          .replaceAll(`P14${r.runId.toUpperCase()}-`, "P14{run}-"))
      : Array.isArray(v)
        ? v.map(visit)
        : v && typeof v === "object"
          ? Object.fromEntries(
              Object.entries(v).map(([k, x]) => [
                k,
                k === "message" ? "[private prose]" : visit(x),
              ]),
            )
          : v;
  return normalizationManifest.map((c) =>
    c.steps.map((step) => {
      const o = r.observations.find(
        (o) => o.caseId === c.id && o.stepId === step.id,
      )!;
      const rows = (values: typeof o.before) =>
        values.map(visit).sort((a, b) => a.Id.localeCompare(b.Id));
      const request = visit(o.request);
      for (const [id, alias] of aliases)
        request.path = request.path.replaceAll(id, alias);
      return {
        stepId: o.stepId,
        request,
        status: o.status,
        bodyPresent: o.bodyPresent,
        body: visit(o.body),
        before: rows(o.before),
        after: rows(o.after),
      };
    }),
  );
}
export async function assessNormalizationPair(
  firstPath: string,
  secondPath: string,
): Promise<NormalizationAssessment> {
  const result: NormalizationAssessment = {
    contractVersion: "plan14-normalization-v1",
    outcome: "blocked",
    reasons: [],
    findings: normalizationManifest.map((c) => ({
      caseId: c.id,
      outcome: "blocked",
      reasons: ["EVIDENCE_NOT_VERIFIED"],
    })),
    inputReportDigests: [],
  };
  try {
    for (const path of [firstPath, secondPath]) {
      await privateJSON(resolve(path));
      result.inputReportDigests.push(
        normalizationDigest(await readFile(resolve(path), "utf8")),
      );
    }
    const first = await load(firstPath),
      second = await load(secondPath),
      a = first.report,
      b = second.report;
    requireEvidence(
      a.runId !== b.runId &&
        equal(a.config, b.config) &&
        a.probeHash === b.probeHash &&
        Date.parse(a.cleanup.finishedAt) < Date.parse(b.startedAt) &&
        Date.parse(a.finishedAt) <= Date.parse(b.startedAt),
    );
    const na = projected(a),
      nb = projected(b);
    result.findings = normalizationManifest.map((c, i) => {
      const outcome =
        equal(na[i], nb[i]) &&
        !first.changedCleanupCases.has(c.id) &&
        !second.changedCleanupCases.has(c.id)
          ? "repeatable"
          : "drift";
      return {
        caseId: c.id,
        outcome,
        reasons: outcome === "repeatable" ? [] : ["OBSERVATIONS_DIFFER"],
      };
    });
    result.outcome = result.findings.some((f) => f.outcome === "drift")
      ? "drift"
      : "repeatable";
  } catch {
    result.reasons.push("NORMALIZATION_EVIDENCE_INVALID_OR_INCOMPLETE");
  }
  return result;
}
