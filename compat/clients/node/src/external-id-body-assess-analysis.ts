import { readdir } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import { privateJSON, recordPath } from "./external-id-recovery.ts";
import {
  bodyManifest,
  bodyScope,
  validateBodyReport,
  type BodyReport,
} from "./external-id-body-contract.ts";
import {
  bodyRow,
  knownBodyRecords,
} from "./external-id-body-recovery-helpers.ts";
import type { BodyStateSnapshot } from "./external-id-body-recovery-models.ts";
import type { RetainedOriginal } from "./external-id-evidence.ts";
import {
  artifact,
  bodyArtifactsDirectory,
  discovery,
  requireEvidence,
  validateBodyCleanupEvidence,
  wire,
} from "./external-id-body-assess-validation.ts";
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
export async function load(path: string) {
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
  const originals: RetainedOriginal[] = [];
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
      const seedBody = seed.body as { success?: unknown; id?: unknown };
      requireEvidence(
        v.seedArtifact === `response-${c.id}-seed.json` &&
          seed.status === 201 &&
          seedBody?.success === true &&
          typeof seedBody?.id === "string",
      );
      requireEvidence(
        v.before.length === 1 &&
          v.before[0].Id === seedBody.id &&
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
