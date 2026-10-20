import { randomBytes } from "node:crypto";
import {
  normalizationManifest,
  normalizationManifestHash,
  normalizationProbeHash,
  normalizationScope,
  normalizationRequest,
  validateNormalizationReport,
  type NormalizationReport,
  type NormalizationObservation,
} from "./external-id-normalization-contract.ts";
import {
  parseExternalIdConfig,
  type ExternalIdConfig,
} from "./external-id-contract.ts";
import { connectorVersions } from "./versions.ts";
import {
  base,
  boundedTransport,
  latchIdentity,
  query,
  transport,
  verifyIdentity,
  type Transport,
} from "./external-id-recovery.ts";
import {
  appendNormalizationEvent,
  assertNormalizationIdentityNotLatched,
  normalizationArtifact,
  normalizationCondition,
  normalizationReason,
  captureNormalizationState,
  initializeNormalizationRun,
  recoverNormalizationRun,
  rememberNormalizationRecord,
  type NormalizationStateArtifact,
} from "./external-id-normalization-recovery.ts";
async function preflight(config: ExternalIdConfig, t: Transport) {
  await verifyIdentity(config, t);
  const user = await t.request(
      base +
        "query?q=" +
        encodeURIComponent(
          "SELECT Id,IsActive,Profile.PermissionsModifyAllData FROM User WHERE Id = '" +
            config.administrator.user.replaceAll("'", "\\'") +
            "'",
        ),
      {},
    ),
    b = user.body as any;
  if (
    user.status !== 200 ||
    b?.done !== true ||
    b.totalSize !== 1 ||
    b.records?.length !== 1 ||
    b.records[0].Id !== config.administrator.user ||
    b.records[0].IsActive !== true ||
    b.records[0].Profile?.PermissionsModifyAllData !== true
  )
    throw new Error("ADMINISTRATOR_UNVERIFIED");
  const describe = await t.request(base + "sobjects/Account/describe", {}),
    fields = (describe.body as any)?.fields;
  const key = Array.isArray(fields)
    ? fields.filter((f) => f?.name === "Compat_Key__c")
    : [];
  if (
    describe.status !== 200 ||
    key.length !== 1 ||
    key[0].type !== "string" ||
    key[0].length !== 80 ||
    key[0].externalId !== true ||
    key[0].unique !== true ||
    key[0].caseSensitive !== false
  )
    throw new Error("FIELD_METADATA_MISMATCH");
}
export async function runNormalizationProbe(
  config: ExternalIdConfig,
  env: NodeJS.ProcessEnv,
  dir: string,
  injected?: Transport,
): Promise<NormalizationReport> {
  const c = parseExternalIdConfig(config),
    runId = randomBytes(12).toString("hex"),
    startedAt = new Date().toISOString(),
    deadline = Date.now() + 600000;
  const raw = injected ?? transport(c, env),
    t = boundedTransport(raw, deadline);
  await initializeNormalizationRun(c, dir, runId);
  await assertNormalizationIdentityNotLatched(dir);
  const r: NormalizationReport = {
    schemaVersion: 1,
    kind: "external-id-normalization",
    config: c,
    connectors: connectorVersions(),
    manifestHash: normalizationManifestHash,
    probeHash: normalizationProbeHash(),
    runId,
    startedAt,
    finishedAt: startedAt,
    identityVerified: false,
    collection: "incomplete",
    observations: [],
    cleanup: {
      outcome: "pending",
      startedAt,
      finishedAt: startedAt,
      journalSha256: "",
      records: [],
      unresolvedOperationIds: [],
      reasons: [],
    },
    reasons: [],
  };
  let mutations = 0;
  try {
    await preflight(c, t);
    r.identityVerified = true;
    for (const scenario of normalizationManifest) {
      if (Date.now() >= deadline) throw new Error("COLLECTION_DEADLINE");
      await verifyIdentity(c, t);
      const collision = await query(
        {
          async request(path, init) {
            const response = await t.request(path, init);
            await normalizationArtifact(dir, `collision-${scenario.id}.json`, {
              caseId: scenario.id,
              response,
            });
            return response;
          },
        },
        normalizationCondition(runId, scenario, true),
      );
      if (collision.length) throw new Error("PREFLIGHT_COLLISION");
      const ids: Partial<Record<"first" | "second", string>> = {};
      for (const step of scenario.steps) {
        const request = normalizationRequest(runId, scenario, step, ids),
          id = `${scenario.id}:${step.id}`;
        const o: NormalizationObservation = {
          caseId: scenario.id,
          stepId: step.id,
          request,
          before: [],
          after: [],
          stateVerified: false,
          status: null,
          bodyPresent: false,
          body: null,
          state: "unavailable",
          artifact: "",
          sha256: "",
          stateArtifact: "",
          stateSha256: "",
        };
        r.observations.push(o);
        const state: NormalizationStateArtifact = {
          caseId: scenario.id,
          before: null,
          after: null,
        };
        try {
          state.before = await captureNormalizationState(
            c,
            dir,
            t,
            runId,
            scenario,
          );
          o.before = state.before.records;
          if (++mutations > 80 || Date.now() >= deadline)
            throw new Error("MUTATION_BOUND");
          await verifyIdentity(c, t);
          await appendNormalizationEvent(c, dir, { id, kind: "intent" });
          await appendNormalizationEvent(c, dir, { id, kind: "dispatched" });
          let failure: unknown;
          try {
            const response = await t.request(request.path, {
              method: request.method,
              body: JSON.stringify(request.body),
            });
            const evidence = await normalizationArtifact(
              dir,
              `response-${id.replaceAll(":", "-")}.json`,
              response,
            );
            await appendNormalizationEvent(c, dir, { id, kind: "response" });
            Object.assign(o, {
              status: response.status,
              bodyPresent: response.bodyPresent ?? response.body !== null,
              body: response.body,
              state: "observed",
              ...evidence,
            });
          } catch (e) {
            await appendNormalizationEvent(c, dir, { id, kind: "unknown" });
            failure = e;
          }
          try {
            const b = o.body as any;
            if (typeof b?.id === "string")
              await rememberNormalizationRecord(c, dir, scenario.id, b.id);
            else if (o.status === 201 || b?.created === true)
              throw new Error("CREATED_ID_UNAVAILABLE");
          } catch (e) {
            failure ??= e;
          }
          state.after = await captureNormalizationState(
            c,
            dir,
            t,
            runId,
            scenario,
          );
          o.after = state.after.records;
          o.stateVerified = true;
          if (failure) throw failure;
          if (step.id.startsWith("seed")) {
            const b = o.body as any;
            if (
              o.status !== 201 ||
              b?.success !== true ||
              typeof b.id !== "string" ||
              !o.after.some(
                (row) => row.Id === b.id && row.Name === request.body.Name,
              )
            )
              throw new Error("SEED_FAILED");
            ids[step.id === "seed" ? "first" : "second"] = b.id;
          }
          if ([401, 403].includes(o.status!))
            throw new Error("AUTHENTICATION_UNAVAILABLE");
        } catch (e) {
          state.error = normalizationReason(e);
          throw e;
        } finally {
          const a = await normalizationArtifact(
            dir,
            `state-${scenario.id}-${step.id}.json`,
            state,
          );
          o.stateArtifact = a.artifact;
          o.stateSha256 = a.sha256;
        }
      }
    }
    await verifyIdentity(c, t);
    r.collection = "complete";
  } catch (e) {
    const reason = normalizationReason(e);
    r.reasons.push(reason);
    if (reason === "IDENTITY_MISMATCH") {
      r.identityVerified = false;
      r.collection = "blocked";
      await latchIdentity(dir);
    }
  }
  r.cleanup = await recoverNormalizationRun(c, env, dir, raw);
  r.reasons.push(...r.cleanup.reasons);
  if (r.cleanup.outcome !== "clean" && r.collection === "complete")
    r.collection = "incomplete";
  if (
    r.cleanup.reasons.some((s) =>
      ["IDENTITY_MISMATCH", "IDENTITY_LATCHED"].includes(s),
    )
  ) {
    r.identityVerified = false;
    r.collection = "blocked";
  }
  r.finishedAt = new Date().toISOString();
  validateNormalizationReport(r);
  await normalizationArtifact(dir, "report.json", r);
  return r;
}
