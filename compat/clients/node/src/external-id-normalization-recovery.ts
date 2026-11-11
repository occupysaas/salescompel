import { randomBytes } from "node:crypto";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import type { ExternalIdConfig } from "./external-id-contract.ts";
import { normalizationManifest } from "./external-id-normalization-contract.ts";
import type { NormalizationCleanup } from "./external-id-normalization-contract.ts";
import {
  boundedTransport,
  latchIdentity,
  recordPath,
  transport,
  verifyIdentity,
  type Transport,
} from "./external-id-recovery.ts";
import type { WireResponse } from "./wire.ts";
import {
  normalizationDigest,
  normalizationReason,
  normalizationCleanupProofName,
  type NormalizationCleanupProof,
  type NormalizationDiscoveryReference,
} from "./external-id-normalization-recovery-models.ts";
import {
  appendNormalizationEvent,
  assertNormalizationIdentityNotLatched,
  knownNormalizationRecords,
  normalizationArtifact,
  normalizationOwned,
  readNormalizationRun,
} from "./external-id-normalization-recovery-helpers.ts";
import {
  captureNormalizationState,
  normalizationDiscovery,
} from "./external-id-normalization-recovery-state.ts";
export {
  normalizationCleanupProofName,
  normalizationDigest,
  normalizationReason,
  type NormalizationAbsenceArtifact,
  type NormalizationCleanupProof,
  type NormalizationDiscoveryArtifact,
  type NormalizationDiscoveryReference,
  type NormalizationExactRead,
  type NormalizationRun,
  type NormalizationStateArtifact,
  type NormalizationStateSnapshot,
} from "./external-id-normalization-recovery-models.ts";
export {
  appendNormalizationEvent,
  assertNormalizationIdentityNotLatched,
  initializeNormalizationRun,
  knownNormalizationRecords,
  normalizationArtifact,
  normalizationCondition,
  normalizationOwned,
  normalizationRow,
  readNormalizationRun,
  rememberNormalizationRecord,
} from "./external-id-normalization-recovery-helpers.ts";
export {
  captureNormalizationState,
  normalizationDiscovery,
} from "./external-id-normalization-recovery-state.ts";
export async function recoverNormalizationRun(
  config: ExternalIdConfig,
  env: NodeJS.ProcessEnv,
  dir: string,
  injected?: Transport,
): Promise<NormalizationCleanup> {
  let validated = false;
  const discoveryArtifacts: NormalizationDiscoveryReference[] = [];
  const result: NormalizationCleanup = {
    outcome: "blocked",
    startedAt: new Date().toISOString(),
    finishedAt: "",
    journalSha256: "",
    records: [],
    unresolvedOperationIds: [],
    reasons: [],
  };
  try {
    const run = await readNormalizationRun(config, dir);
    validated = true;
    await assertNormalizationIdentityNotLatched(dir);
    const t = boundedTransport(
      injected ?? transport(config, env),
      Date.now() + 300000,
    );
    await verifyIdentity(config, t);
    const known = await knownNormalizationRecords(config, dir),
      cases = new Set([
        ...run.events
          .filter((e) => e.kind === "dispatched" && !e.id.startsWith("delete:"))
          .map((e) => e.id.split(":")[0]),
        ...known.values(),
      ]);
    for (const caseId of cases) {
      const c = normalizationManifest.find((c) => c.id === caseId)!;
      try {
        await verifyIdentity(config, t);
        const snapshot = await captureNormalizationState(
          config,
          dir,
          t,
          run.runId,
          c,
        );
        await normalizationArtifact(
          dir,
          `cleanup-state-${caseId}-${randomBytes(6).toString("hex")}.json`,
          snapshot,
        );
        for (const row of snapshot.records) {
          if (!normalizationOwned(run.runId, c, row, run.stages))
            throw new Error("OWNERSHIP_UNPROVEN");
          const id = "delete:" + row.Id,
            stage = (await readNormalizationRun(config, dir)).stages.get(id);
          if (stage)
            throw new Error(
              stage === "response"
                ? "RECORD_REAPPEARED"
                : "UNRESOLVED_DISPATCH",
            );
          await verifyIdentity(config, t);
          await appendNormalizationEvent(config, dir, { id, kind: "intent" });
          await appendNormalizationEvent(config, dir, {
            id,
            kind: "dispatched",
          });
          let response: WireResponse;
          try {
            response = await t.request(recordPath(row.Id), {
              method: "DELETE",
            });
            await normalizationArtifact(
              dir,
              `response-delete-${row.Id}.json`,
              response,
            );
            await appendNormalizationEvent(config, dir, {
              id,
              kind: "response",
            });
          } catch (e) {
            await appendNormalizationEvent(config, dir, {
              id,
              kind: "unknown",
            });
            throw e;
          }
          if (![204, 404].includes(response.status))
            throw new Error("DELETE_FAILED");
        }
      } catch (e) {
        if (normalizationReason(e) === "IDENTITY_MISMATCH") throw e;
        result.reasons.push(normalizationReason(e));
      }
      // Even failed ownership/discovery must retain every known ID and its exact final read.
      for (const [id, owner] of await knownNormalizationRecords(config, dir)) {
        if (owner !== caseId) continue;
        const response = await t.request(recordPath(id), {});
        const a = await normalizationArtifact(
          dir,
          `absence-${id}-${randomBytes(6).toString("hex")}.json`,
          { caseId, id, response },
        );
        result.records.push({
          caseId,
          id,
          absenceArtifact: a.artifact,
          absenceSha256: a.sha256,
        });
        if (response.status !== 404) result.reasons.push("ABSENCE_UNVERIFIED");
      }
      try {
        const remaining = await normalizationDiscovery(
          config,
          dir,
          t,
          run.runId,
          c,
          (e) => discoveryArtifacts.push(e),
        );
        if (remaining.length) result.reasons.push("ABSENCE_UNVERIFIED");
      } catch (e) {
        result.reasons.push(normalizationReason(e));
      }
    }
    await verifyIdentity(config, t);
  } catch (e) {
    if (normalizationReason(e) === "IDENTITY_MISMATCH")
      await latchIdentity(dir);
    result.reasons.push(normalizationReason(e));
  }
  try {
    const final = await readNormalizationRun(config, dir);
    result.unresolvedOperationIds = [...final.stages]
      .filter(([, stage]) => ["dispatched", "unknown"].includes(stage))
      .map(([id]) => id);
    if (result.unresolvedOperationIds.length)
      result.reasons.push("UNRESOLVED_DISPATCH");
    result.journalSha256 = normalizationDigest(
      await readFile(join(dir, "journal.jsonl"), "utf8"),
    );
  } catch (e) {
    result.reasons.push(normalizationReason(e));
  }
  if (validated) {
    try {
      for (const [id, caseId] of await knownNormalizationRecords(config, dir)) {
        if (!result.records.some((r) => r.id === id)) {
          result.records.push({
            caseId,
            id,
            absenceArtifact: "",
            absenceSha256: "",
          });
          result.reasons.push("ABSENCE_UNVERIFIED");
        }
      }
    } catch (e) {
      result.reasons.push(normalizationReason(e));
    }
  }
  result.reasons = [...new Set(result.reasons)];
  result.outcome = result.reasons.length ? "blocked" : "clean";
  result.finishedAt = new Date().toISOString();
  // Failure to publish the binding prevents any clean result reaching the caller.
  if (validated) {
    const proof: NormalizationCleanupProof = {
      cleanup: result,
      discoveryArtifacts,
    };
    await normalizationArtifact(
      dir,
      normalizationCleanupProofName(result),
      proof,
    );
  }
  return result;
}
