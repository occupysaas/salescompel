import { randomBytes, createHash } from "node:crypto";
import { join } from "node:path";
import {
  manifest,
  caseScope,
  newExternalIdReport,
  parseExternalIdConfig,
  validateExternalIdReport,
  type ExternalIdConfig,
  type Observation,
  type Step,
} from "./external-id-contract.ts";
import {
  boundedTransport,
  rememberRecord,
  latchIdentity,
  initializeRun,
  appendJournal,
  durableJSON,
  recoverExternalIdRun,
  transport,
  verifyIdentity,
  scopedRecords,
  preflightCase,
  recordPath,
  base,
  type Transport,
} from "./external-id-recovery.ts";
import { nativeUpsert } from "./external-id-native.ts";
export async function runExternalIdProbe(
  config: ExternalIdConfig,
  env: NodeJS.ProcessEnv,
  dir: string,
  injected?: Transport,
) {
  const c = parseExternalIdConfig(config);
  const collectionDeadline = Date.now() + 600000;
  const rawTransport = injected ?? transport(c, env);
  const t = boundedTransport(rawTransport, collectionDeadline);
  const runId = randomBytes(12).toString("hex");
  const r = newExternalIdReport(c, runId);
  await initializeRun(c, dir, runId);
  const deadline = collectionDeadline;
  let dispatched = 0;
  try {
    await verifyIdentity(c, t);
    r.identityVerified = true;
    for (const scenario of manifest) {
      if (Date.now() > deadline) throw new Error("COLLECTION_DEADLINE");
      await verifyIdentity(c, t);
      await preflightCase(t, runId, scenario.id);
      const { marker, keys } = caseScope(runId, scenario);
      const ids = new Map<string, string>();
      const execute = async (step: Step) => {
        const id = scenario.id + ":" + step.id;
        const o: Observation = {
          id,
          caseId: scenario.id,
          method: step.method,
          key: keys[step.key],
          bodyKey: step.bodyKey === undefined ? null : keys[step.bodyKey],
          state: "unavailable",
          status: null,
          bodyPresent: false,
          body: null,
          records: [],
          stateVerified: false,
          artifact: "",
          sha256: "",
        };
        r.operations.push(o);
        if (++dispatched > 160 || Date.now() > deadline)
          throw new Error("MUTATION_BOUND");
        const data: Record<string, string> = { Name: marker };
        let path = base + "sobjects/Account",
          method = "POST";
        if (step.method === "create") data.Compat_Key__c = keys[step.key];
        if (step.method === "upsert") {
          path += "/Compat_Key__c/" + encodeURIComponent(keys[step.key]);
          method = "PATCH";
          if (step.bodyKey !== undefined)
            data.Compat_Key__c = keys[step.bodyKey];
        }
        if (step.method === "update") {
          const target = ids.get(step.target!);
          if (!target) throw new Error("SEED_ID_UNAVAILABLE");
          path = recordPath(target);
          method = "PATCH";
          data.Compat_Key__c = keys[step.key];
        }
        await appendJournal(dir, { kind: "intent", id });
        await appendJournal(dir, { kind: "dispatched", id });
        let resolved = false;
        try {
          if (step.method === "native") {
            o.body = await (t.native
              ? t.native(keys[step.key], marker)
              : nativeUpsert(
                  c.origin,
                  env.COMPAT_REFERENCE_TOKEN!,
                  keys[step.key],
                  marker,
                  Math.min(60000, Math.max(1, deadline - Date.now())),
                ));
            o.bodyPresent = true;
          } else {
            const response = await t.request(path, {
              method,
              body: JSON.stringify(data),
            });
            o.status = response.status;
            o.body = response.body;
            o.bodyPresent = response.bodyPresent ?? response.body !== null;
          }
          // Store the complete response durably before marking its dispatch resolved.
          o.artifact = `response-${id.replaceAll(":", "-")}.json`;
          o.sha256 = createHash("sha256")
            .update(
              JSON.stringify({
                status: o.status,
                bodyPresent: o.bodyPresent,
                body: o.body,
              }) + "\n",
            )
            .digest("hex");
          await durableJSON(join(dir, o.artifact), {
            status: o.status,
            bodyPresent: o.bodyPresent,
            body: o.body,
          });
          await appendJournal(dir, { kind: "response", id });
          resolved = true;
          o.state = "observed";
          const responseId = (o.body as any)?.id;
          if (typeof responseId === "string") {
            recordPath(responseId);
            await rememberRecord(dir, scenario.id, responseId);
            ids.set(step.id, responseId);
          }
        } catch {
          if (!resolved) await appendJournal(dir, { kind: "unknown", id });
          throw new Error("MUTATION_OUTCOME_UNKNOWN");
        }
        return o;
      };
      const batches = scenario.steps[0].parallel
        ? [scenario.steps]
        : scenario.steps.map((s) => [s]);
      for (const batch of batches) {
        const results = await Promise.allSettled(batch.map(execute));
        const rows = await scopedRecords(t, runId, scenario.id);
        for (const row of [...rows].sort((a, b) =>
          a.Compat_Key__c!.localeCompare(b.Compat_Key__c!),
        )) {
          await rememberRecord(dir, scenario.id, row.Id);
          if (!r.aliases.some((a) => a.id === row.Id))
            r.aliases.push({
              id: row.Id,
              alias:
                scenario.id +
                ":" +
                r.aliases.filter((a) => a.alias.startsWith(scenario.id + ":"))
                  .length,
            });
        }
        if (r.aliases.length > 80) throw new Error("FIXTURE_BOUND");
        for (const result of results) {
          if (result.status === "fulfilled") {
            result.value.records = rows;
            result.value.stateVerified = true;
          }
        }
        if (results.some((x) => x.status === "rejected"))
          throw new Error("MUTATION_OUTCOME_UNKNOWN");
      }
    }
    await verifyIdentity(c, t);
    r.collection = "complete";
  } catch (e) {
    r.collection = "incomplete";
    if (e instanceof Error && e.message === "IDENTITY_MISMATCH") {
      r.identityVerified = false;
      await latchIdentity(dir);
    }
    r.reasons.push(
      e instanceof Error && /^[A-Z_]+$/.test(e.message)
        ? e.message
        : "COLLECTION_UNAVAILABLE",
    );
  }
  const recovery = await recoverExternalIdRun(c, env, dir, rawTransport);
  r.cleanup = recovery.outcome;
  r.reasons.push(...recovery.reasons);
  try {
    await verifyIdentity(c, rawTransport);
  } catch {
    r.identityVerified = false;
    r.collection = "blocked";
  }
  if (r.cleanup !== "clean" && r.collection === "complete")
    r.collection = "incomplete";
  r.finishedAt = new Date().toISOString();
  validateExternalIdReport(r);
  return r;
}
