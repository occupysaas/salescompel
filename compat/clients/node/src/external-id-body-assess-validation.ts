import { readFile, readdir } from "node:fs/promises";
import { join, dirname, basename, resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import { bodyManifest, type BodyCleanup } from "./external-id-body-contract.ts";
import type { ExternalIdConfig } from "./external-id-contract.ts";
import type { WireResponse } from "./wire.ts";
import {
  assertBodyIdentityNotLatched,
  bodyOwned,
  bodyRow,
  knownBodyRecords,
  readBodyRun,
} from "./external-id-body-recovery-helpers.ts";
import {
  bodyCleanupProofName,
  bodyDigest,
} from "./external-id-body-recovery-models.ts";
import type { BodyStateSnapshot } from "./external-id-body-recovery-models.ts";
import { evidenceTools } from "./external-id-evidence.ts";
const tools = evidenceTools("BODY_EVIDENCE_INVALID", bodyDigest, bodyRow);
export const requireEvidence: typeof tools.requireEvidence =
  tools.requireEvidence;
export const artifact: typeof tools.artifact = tools.artifact;
export const wire: typeof tools.wire = tools.wire;
export const discovery: typeof tools.discovery = tools.discovery;
export const snapshot = tools.snapshotState as (
  s: BodyStateSnapshot,
) => BodyStateSnapshot;
export const bodyArtifactsDirectory = (path: string) =>
  join(dirname(resolve(path)), basename(path, ".json") + ".artifacts");
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
