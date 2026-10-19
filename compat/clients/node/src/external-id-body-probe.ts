import { randomBytes } from "node:crypto";
import {
  bodyManifest,
  bodyManifestHash,
  bodyScope,
  bodyRequest,
  validateBodyReport,
  type BodyObservation,
  type BodyReport,
} from "./external-id-body-contract.ts";
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
  appendBodyEvent,
  assertBodyIdentityNotLatched,
  bodyArtifact,
  bodyCondition,
  bodyReason,
  captureBodyState,
  initializeBodyRun,
  readBodyRun,
  recoverBodyRun,
  rememberBodyRecord,
  type BodyStateArtifact,
} from "./external-id-body-recovery.ts";
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
export async function runBodyProbe(
  config: ExternalIdConfig,
  env: NodeJS.ProcessEnv,
  dir: string,
  injected?: Transport,
): Promise<BodyReport> {
  const c = parseExternalIdConfig(config),
    runId = randomBytes(12).toString("hex"),
    startedAt = new Date().toISOString(),
    deadline = Date.now() + 600000;
  const raw = injected ?? transport(c, env),
    t = boundedTransport(raw, deadline);
  await initializeBodyRun(c, dir, runId);
  await assertBodyIdentityNotLatched(dir);
  const r: BodyReport = {
    schemaVersion: 1,
    kind: "external-id-body-validation",
    config: c,
    connectors: connectorVersions(),
    manifestHash: bodyManifestHash,
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
  const mutate = async (
    id: string,
    path: string,
    method: string,
    body: unknown,
  ) => {
    if (++mutations > 160 || Date.now() >= deadline)
      throw new Error("MUTATION_BOUND");
    await verifyIdentity(c, t);
    await appendBodyEvent(c, dir, { id, kind: "intent" });
    await appendBodyEvent(c, dir, { id, kind: "dispatched" });
    let resolved = false;
    try {
      const response = await t.request(path, {
        method,
        body: JSON.stringify(body),
      });
      const evidence = await bodyArtifact(
        dir,
        `response-${id.replaceAll(":", "-")}.json`,
        response,
      );
      await appendBodyEvent(c, dir, { id, kind: "response" });
      resolved = true;
      return { response, evidence };
    } catch (e) {
      if (!resolved) await appendBodyEvent(c, dir, { id, kind: "unknown" });
      throw e;
    }
  };
  const rememberResponse = async (
    caseId: string,
    response: import("./wire.ts").WireResponse,
  ) => {
    const returned = (response.body as any)?.id;
    if (typeof returned === "string")
      await rememberBodyRecord(c, dir, caseId, returned);
    else if (
      response.status === 201 ||
      (response.body as any)?.created === true
    )
      throw new Error("CREATED_ID_UNAVAILABLE");
  };
  try {
    await preflight(c, t);
    r.identityVerified = true;
    for (const scenario of bodyManifest) {
      if (Date.now() >= deadline) throw new Error("COLLECTION_DEADLINE");
      await verifyIdentity(c, t);
      const scope = bodyScope(runId, scenario);
      const collision = await query(t, bodyCondition(runId, scenario));
      await bodyArtifact(dir, `collision-${scenario.id}.json`, {
        caseId: scenario.id,
        records: collision,
      });
      if (collision.length) throw new Error("PREFLIGHT_COLLISION");
      const o: BodyObservation = {
        caseId: scenario.id,
        request: bodyRequest(scope, scenario.valueKind),
        pathKey: scope.pathKey,
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
      const state: BodyStateArtifact = {
        caseId: scenario.id,
        before: null,
        after: null,
      };
      try {
        if (scenario.recordState === "existing") {
          const seed = await mutate(
            `${scenario.id}:seed`,
            base + "sobjects/Account",
            "POST",
            { Name: scope.seedName, Compat_Key__c: scope.pathKey },
          );
          o.seedArtifact = seed.evidence.artifact;
          o.seedSha256 = seed.evidence.sha256;
          await rememberResponse(scenario.id, seed.response);
          if (seed.response.status !== 201) throw new Error("SEED_FAILED");
        }
        state.before = await captureBodyState(c, dir, t, runId, scenario);
        o.before = state.before.records;
        await bodyArtifact(dir, `before-${scenario.id}.json`, state.before);
        if (
          scenario.recordState === "existing"
            ? o.before.length !== 1 ||
              o.before[0].Name !== scope.seedName ||
              o.before[0].Compat_Key__c !== scope.pathKey
            : o.before.length !== 0
        )
          throw new Error("PRESTATE_UNVERIFIED");
        let failure: unknown;
        try {
          const { response, evidence } = await mutate(
            `${scenario.id}:request`,
            base +
              "sobjects/Account/Compat_Key__c/" +
              encodeURIComponent(scope.pathKey),
            "PATCH",
            o.request,
          );
          o.status = response.status;
          o.bodyPresent = response.bodyPresent ?? response.body !== null;
          o.body = response.body;
          o.state = "observed";
          o.artifact = evidence.artifact;
          o.sha256 = evidence.sha256;
          await rememberResponse(scenario.id, response);
        } catch (e) {
          failure = e;
        }
        // Read state independently even when response bytes were lost.
        state.after = await captureBodyState(c, dir, t, runId, scenario);
        o.after = state.after.records;
        o.stateVerified = true;
        if (failure) throw failure;
      } catch (e) {
        state.error = bodyReason(e);
        throw e;
      } finally {
        const evidence = await bodyArtifact(
          dir,
          `state-${scenario.id}.json`,
          state,
        );
        o.stateArtifact = evidence.artifact;
        o.stateSha256 = evidence.sha256;
      }
    }
    await verifyIdentity(c, t);
    r.collection = "complete";
  } catch (e) {
    const reason = bodyReason(e);
    r.reasons.push(reason);
    if (reason === "IDENTITY_MISMATCH") {
      r.identityVerified = false;
      r.collection = "blocked";
      await latchIdentity(dir);
    }
  }
  r.cleanup = await recoverBodyRun(c, env, dir, raw);
  r.reasons.push(...r.cleanup.reasons);
  if (r.cleanup.outcome !== "clean" && r.collection === "complete")
    r.collection = "incomplete";
  if (
    r.cleanup.reasons.some((v) =>
      ["IDENTITY_MISMATCH", "IDENTITY_LATCHED"].includes(v),
    )
  ) {
    r.identityVerified = false;
    r.collection = "blocked";
  }
  r.finishedAt = new Date().toISOString();
  validateBodyReport(r);
  await bodyArtifact(dir, "report.json", r);
  await readBodyRun(c, dir);
  return r;
}
