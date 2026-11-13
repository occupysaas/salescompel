import { readFile, readdir } from "node:fs/promises";
import { join, dirname, basename, resolve } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import { recordPath } from "./external-id-recovery.ts";
import {
  normalizationManifest,
  type NormalizationCleanup,
} from "./external-id-normalization-contract.ts";
import type { ExternalIdConfig } from "./external-id-contract.ts";
import type { WireResponse } from "./wire.ts";
import {
  assertNormalizationIdentityNotLatched,
  knownNormalizationRecords,
  normalizationOwned,
  normalizationRow,
  readNormalizationRun,
} from "./external-id-normalization-recovery-helpers.ts";
import {
  normalizationCleanupProofName,
  normalizationDigest,
} from "./external-id-normalization-recovery-models.ts";
import type { NormalizationStateSnapshot } from "./external-id-normalization-recovery-models.ts";
import { evidenceTools } from "./external-id-evidence.ts";
const tools = evidenceTools(
  "NORMALIZATION_EVIDENCE_INVALID",
  normalizationDigest,
  normalizationRow,
);
export const requireEvidence: typeof tools.requireEvidence =
  tools.requireEvidence;
export const artifact: typeof tools.artifact = tools.artifact;
export const wire: typeof tools.wire = tools.wire;
export const discovery: typeof tools.discovery = tools.discovery;
export const snapshot = tools.snapshotState as (
  s: NormalizationStateSnapshot,
) => NormalizationStateSnapshot;
export const normalizationArtifactsDirectory = (path: string) =>
  join(dirname(resolve(path)), basename(path, ".json") + ".artifacts");
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
export function mutationResponse(
  response: WireResponse,
  kind: "create" | "update" | "upsert",
) {
  wire(response);
  const raw = response.body;
  if (response.status >= 200 && response.status < 300) {
    if (kind === "update")
      requireEvidence(response.status === 204 && raw === null);
    else {
      const b = raw as {
        id: string;
        success?: unknown;
        created?: unknown;
        errors?: unknown;
      };
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
        Array.isArray(raw) &&
        raw.length > 0 &&
        raw.every(
          (e) =>
            e &&
            typeof e.errorCode === "string" &&
            typeof e.message === "string" &&
            (!Object.hasOwn(e, "fields") || Array.isArray(e.fields)),
        ),
    );
}
