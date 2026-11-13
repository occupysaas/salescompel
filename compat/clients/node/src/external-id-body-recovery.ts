import { randomBytes } from "node:crypto";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import type { ExternalIdConfig } from "./external-id-contract.ts";
import { bodyManifest } from "./external-id-body-contract.ts";
import type { BodyCleanup } from "./external-id-body-contract.ts";
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
  bodyDigest,
  bodyReason,
  bodyCleanupProofName,
  type BodyCleanupProof,
  type BodyDiscoveryReference,
} from "./external-id-body-recovery-models.ts";
import {
  appendBodyEvent,
  assertBodyIdentityNotLatched,
  bodyArtifact,
  bodyOwned,
  knownBodyRecords,
  readBodyRun,
} from "./external-id-body-recovery-helpers.ts";
import {
  bodyDiscovery,
  captureBodyState,
} from "./external-id-body-recovery-state.ts";
export {
  bodyCleanupProofName,
  bodyDigest,
  bodyReason,
  type BodyAbsenceArtifact,
  type BodyCleanupProof,
  type BodyDiscoveryArtifact,
  type BodyDiscoveryReference,
  type BodyExactRead,
  type BodyRun,
  type BodyStateArtifact,
  type BodyStateSnapshot,
} from "./external-id-body-recovery-models.ts";
export {
  appendBodyEvent,
  assertBodyIdentityNotLatched,
  bodyArtifact,
  bodyCondition,
  bodyOwned,
  bodyRow,
  initializeBodyRun,
  knownBodyRecords,
  readBodyRun,
  rememberBodyRecord,
} from "./external-id-body-recovery-helpers.ts";
export {
  bodyDiscovery,
  captureBodyState,
} from "./external-id-body-recovery-state.ts";
export async function recoverBodyRun(
  config: ExternalIdConfig,
  env: NodeJS.ProcessEnv,
  dir: string,
  injected?: Transport,
): Promise<BodyCleanup> {
  let validated = false;
  const discoveryArtifacts: BodyDiscoveryReference[] = [];
  const result: BodyCleanup = {
    outcome: "blocked",
    startedAt: new Date().toISOString(),
    finishedAt: "",
    journalSha256: "",
    records: [],
    unresolvedOperationIds: [],
    reasons: [],
  };
  try {
    const run = await readBodyRun(config, dir);
    validated = true;
    await assertBodyIdentityNotLatched(dir);
    const t = boundedTransport(
      injected ?? transport(config, env),
      Date.now() + 300000,
    );
    await verifyIdentity(config, t);
    const known = await knownBodyRecords(config, dir),
      cases = new Set([
        ...run.events
          .filter((e) => e.kind === "dispatched" && !e.id.startsWith("delete:"))
          .map((e) => e.id.split(":")[0]),
        ...known.values(),
      ]);
    for (const caseId of cases) {
      const c = bodyManifest.find((c) => c.id === caseId)!;
      try {
        await verifyIdentity(config, t);
        const snapshot = await captureBodyState(config, dir, t, run.runId, c);
        await bodyArtifact(
          dir,
          `cleanup-state-${caseId}-${randomBytes(6).toString("hex")}.json`,
          snapshot,
        );
        for (const row of snapshot.records) {
          if (!bodyOwned(run.runId, c, row))
            throw new Error("OWNERSHIP_UNPROVEN");
          const id = "delete:" + row.Id,
            stage = (await readBodyRun(config, dir)).stages.get(id);
          if (stage)
            throw new Error(
              stage === "response"
                ? "RECORD_REAPPEARED"
                : "UNRESOLVED_DISPATCH",
            );
          await verifyIdentity(config, t);
          await appendBodyEvent(config, dir, { id, kind: "intent" });
          await appendBodyEvent(config, dir, { id, kind: "dispatched" });
          let response: WireResponse;
          try {
            response = await t.request(recordPath(row.Id), {
              method: "DELETE",
            });
            await bodyArtifact(dir, `response-delete-${row.Id}.json`, response);
            await appendBodyEvent(config, dir, { id, kind: "response" });
          } catch (e) {
            await appendBodyEvent(config, dir, { id, kind: "unknown" });
            throw e;
          }
          if (![204, 404].includes(response.status))
            throw new Error("DELETE_FAILED");
        }
      } catch (e) {
        if (bodyReason(e) === "IDENTITY_MISMATCH") throw e;
        result.reasons.push(bodyReason(e));
      }
      // Even failed ownership/discovery must retain every known ID and its exact final read.
      for (const [id, owner] of await knownBodyRecords(config, dir)) {
        if (owner !== caseId) continue;
        const response = await t.request(recordPath(id), {});
        const a = await bodyArtifact(
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
        const remaining = await bodyDiscovery(
          config,
          dir,
          t,
          run.runId,
          c,
          (e) => discoveryArtifacts.push(e),
        );
        if (remaining.length) result.reasons.push("ABSENCE_UNVERIFIED");
      } catch (e) {
        result.reasons.push(bodyReason(e));
      }
    }
    await verifyIdentity(config, t);
  } catch (e) {
    if (bodyReason(e) === "IDENTITY_MISMATCH") await latchIdentity(dir);
    result.reasons.push(bodyReason(e));
  }
  try {
    const final = await readBodyRun(config, dir);
    result.unresolvedOperationIds = [...final.stages]
      .filter(([, stage]) => ["dispatched", "unknown"].includes(stage))
      .map(([id]) => id);
    if (result.unresolvedOperationIds.length)
      result.reasons.push("UNRESOLVED_DISPATCH");
    result.journalSha256 = bodyDigest(
      await readFile(join(dir, "journal.jsonl"), "utf8"),
    );
  } catch (e) {
    result.reasons.push(bodyReason(e));
  }
  if (validated) {
    try {
      for (const [id, caseId] of await knownBodyRecords(config, dir)) {
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
      result.reasons.push(bodyReason(e));
    }
  }
  result.reasons = [...new Set(result.reasons)];
  result.outcome = result.reasons.length ? "blocked" : "clean";
  result.finishedAt = new Date().toISOString();
  // Failure to publish the binding prevents any clean result reaching the caller.
  if (validated) {
    const proof: BodyCleanupProof = { cleanup: result, discoveryArtifacts };
    await bodyArtifact(dir, bodyCleanupProofName(result), proof);
  }
  return result;
}
