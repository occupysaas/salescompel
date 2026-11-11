import { readdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import { privateJSON, recordPath } from "./external-id-recovery.ts";
import {
  normalizationManifest,
  normalizationRequest,
  validateNormalizationReport,
  type NormalizationReport,
  type NormalizationRequest,
} from "./external-id-normalization-contract.ts";
import {
  knownNormalizationRecords,
  normalizationRow,
} from "./external-id-normalization-recovery-helpers.ts";
import type { NormalizationStateSnapshot } from "./external-id-normalization-recovery-models.ts";
import type { RetainedOriginal } from "./external-id-evidence.ts";
import {
  artifact,
  discovery,
  mutationResponse,
  normalizationArtifactsDirectory,
  requireEvidence,
  validateNormalizationCleanupEvidence,
  wire,
} from "./external-id-normalization-assess-validation.ts";
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
export async function load(path: string) {
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
    rawDiscoveries: RetainedOriginal[] = [],
    rawReads: RetainedOriginal[] = [],
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
      const b = response.body as { id?: string };
      if (step.id.startsWith("seed")) {
        requireEvidence(
          response.status === 201 &&
            o.after.some(
              (row) => row.Id === b.id && row.Name === o.request.body.Name,
            ),
        );
        if (step.id === "seed-second") requireEvidence(ids.first !== b.id);
        ids[step.id === "seed" ? "first" : "second"] = b.id as string;
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
      const b = o.body as { id?: unknown };
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
  const visit = (v: unknown): unknown =>
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
        (values.map((row) => visit(row)) as typeof o.before).sort((a, b) =>
          a.Id.localeCompare(b.Id),
        );
      const request = visit(o.request) as NormalizationRequest;
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
