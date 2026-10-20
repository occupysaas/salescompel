import { createHash, randomBytes } from "node:crypto";
import { lstat, open, readFile, readdir } from "node:fs/promises";
import { join } from "node:path";
import { isDeepStrictEqual } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import {
  parseExternalIdConfig,
  type ExternalIdConfig,
  type RecordState,
} from "./external-id-contract.ts";
import {
  normalizationManifest,
  normalizationManifestHash,
  normalizationProbeHash,
  normalizationScope,
  type NormalizationCase,
  type NormalizationCleanup,
} from "./external-id-normalization-contract.ts";
import {
  appendJournal,
  boundedTransport,
  durableJSON,
  latchIdentity,
  privateJSON,
  query,
  recordPath,
  transport,
  verifyIdentity,
  type JournalEvent,
  type Transport,
} from "./external-id-recovery.ts";
import type { WireResponse } from "./wire.ts";
export interface NormalizationRun {
  runId: string;
  events: JournalEvent[];
  stages: Map<string, string>;
}
export interface NormalizationExactRead {
  id: string;
  response: WireResponse;
}
export interface NormalizationDiscoveryArtifact {
  caseId: string;
  response: WireResponse;
}
export interface NormalizationAbsenceArtifact extends NormalizationExactRead {
  caseId: string;
}
export interface NormalizationDiscoveryReference {
  caseId: string;
  artifact: string;
  sha256: string;
}
export interface NormalizationCleanupProof {
  cleanup: NormalizationCleanup;
  discoveryArtifacts: NormalizationDiscoveryReference[];
}
export const normalizationCleanupProofName = (cleanup: NormalizationCleanup) =>
  `cleanup-proof-${normalizationDigest(JSON.stringify(cleanup))}.json`;
export interface NormalizationStateSnapshot {
  records: RecordState[];
  discovery: RecordState[];
  exactReads: NormalizationExactRead[];
}
export interface NormalizationStateArtifact {
  caseId: string;
  before: NormalizationStateSnapshot | null;
  after: NormalizationStateSnapshot | null;
  error?: string;
}
export const normalizationDigest = (bytes: string) =>
  createHash("sha256").update(bytes).digest("hex");
export const normalizationReason = (e: unknown) =>
  e instanceof Error && /^[A-Z_]+$/.test(e.message)
    ? e.message
    : "NORMALIZATION_EVIDENCE_UNAVAILABLE";
const operations = new Set(
  normalizationManifest.flatMap((c) => c.steps.map((s) => `${c.id}:${s.id}`)),
);
function transition(stages: Map<string, string>, e: JournalEvent) {
  if (
    !e ||
    Object.keys(e).sort().join() !== "id,kind" ||
    (!operations.has(e.id) &&
      !/^delete:[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?$/.test(e.id))
  )
    throw new Error("INVALID_JOURNAL");
  const p = stages.get(e.id);
  if (
    (e.kind === "intent" && p !== undefined) ||
    (e.kind === "dispatched" && p !== "intent") ||
    (["response", "unknown"].includes(e.kind) && p !== "dispatched") ||
    !["intent", "dispatched", "response", "unknown"].includes(e.kind)
  )
    throw new Error("INVALID_JOURNAL_TRANSITION");
  stages.set(e.id, e.kind);
}
export async function initializeNormalizationRun(
  config: ExternalIdConfig,
  dir: string,
  runId: string,
) {
  await privateDirectory(dir);
  parseExternalIdConfig(config);
  if (!/^[a-f0-9]{24}$/.test(runId)) throw new Error("INVALID_RUN_ID");
  await durableJSON(join(dir, "binding.json"), {
    config,
    runId,
    kind: "external-id-normalization",
    manifestHash: normalizationManifestHash,
    probeHash: normalizationProbeHash(),
  });
  const f = await open(join(dir, "journal.jsonl"), "wx", 0o600);
  await f.sync();
  await f.close();
  const parent = await open(dir, "r");
  try {
    await parent.sync();
  } finally {
    await parent.close();
  }
}
export async function readNormalizationRun(
  config: ExternalIdConfig,
  dir: string,
): Promise<NormalizationRun> {
  await privateDirectory(dir);
  parseExternalIdConfig(config);
  const b = await privateJSON(join(dir, "binding.json"), 65536);
  if (
    Object.keys(b).sort().join() !==
      "config,kind,manifestHash,probeHash,runId" ||
    b.kind !== "external-id-normalization" ||
    b.manifestHash !== normalizationManifestHash ||
    b.probeHash !== normalizationProbeHash() ||
    !isDeepStrictEqual(b.config, config) ||
    !/^[a-f0-9]{24}$/.test(b.runId)
  )
    throw new Error("JOURNAL_BINDING_MISMATCH");
  const p = join(dir, "journal.jsonl"),
    s = await lstat(p);
  if (
    !s.isFile() ||
    s.isSymbolicLink() ||
    (s.mode & 0o077) !== 0 ||
    s.size > 1048576 ||
    (process.getuid && s.uid !== process.getuid())
  )
    throw new Error("INVALID_JOURNAL");
  const text = await readFile(p, "utf8");
  if (text && !text.endsWith("\n")) throw new Error("TRUNCATED_JOURNAL");
  const events: JournalEvent[] = text
      .split("\n")
      .filter(Boolean)
      .map((l) => JSON.parse(l)),
    stages = new Map<string, string>();
  for (const e of events) transition(stages, e);
  if (
    events.filter((e) => e.kind === "dispatched" && !e.id.startsWith("delete:"))
      .length > 160
  )
    throw new Error("MUTATION_BOUND");
  return { runId: b.runId, events, stages };
}
export async function appendNormalizationEvent(
  config: ExternalIdConfig,
  dir: string,
  e: JournalEvent,
) {
  const r = await readNormalizationRun(config, dir);
  transition(r.stages, e);
  await appendJournal(dir, e);
}
export async function normalizationArtifact(
  dir: string,
  name: string,
  value: unknown,
) {
  await privateDirectory(dir);
  if (!/^[A-Za-z0-9][A-Za-z0-9._-]*\.json$/.test(name))
    throw new Error("INVALID_ARTIFACT_PATH");
  await durableJSON(join(dir, name), value);
  const parent = await open(dir, "r");
  try {
    await parent.sync();
  } finally {
    await parent.close();
  }
  return {
    artifact: name,
    sha256: normalizationDigest(await readFile(join(dir, name), "utf8")),
  };
}
export async function assertNormalizationIdentityNotLatched(dir: string) {
  try {
    await privateJSON(join(dir, "identity-blocked.json"));
    throw new Error("IDENTITY_LATCHED");
  } catch (e: any) {
    if (e.code !== "ENOENT") throw e;
  }
}
export async function rememberNormalizationRecord(
  config: ExternalIdConfig,
  dir: string,
  caseId: string,
  id: string,
) {
  await readNormalizationRun(config, dir);
  recordPath(id);
  if (!normalizationManifest.some((c) => c.id === caseId))
    throw new Error("UNKNOWN_CASE");
  const path = join(dir, `known-${id}.json`);
  try {
    await normalizationArtifact(dir, `known-${id}.json`, { caseId, id });
  } catch (e: any) {
    if (e.code !== "EEXIST") throw e;
    const p = await privateJSON(path);
    if (p.id !== id || p.caseId !== caseId)
      throw new Error("OWNERSHIP_UNPROVEN");
  }
  if ((await knownNormalizationRecords(config, dir, false)).size > 80)
    throw new Error("FIXTURE_BOUND");
}
export async function knownNormalizationRecords(
  config: ExternalIdConfig,
  dir: string,
  responses = true,
) {
  const run = await readNormalizationRun(config, dir),
    known = new Map<string, string>();
  const add = (caseId: string, id: string) => {
    recordPath(id);
    if (
      !normalizationManifest.some((c) => c.id === caseId) ||
      (known.has(id) && known.get(id) !== caseId)
    )
      throw new Error("OWNERSHIP_UNPROVEN");
    known.set(id, caseId);
  };
  for (const name of await readdir(dir)) {
    if (!/^known-[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?\.json$/.test(name))
      continue;
    const x = await privateJSON(join(dir, name));
    if (
      name !== `known-${x.id}.json` ||
      Object.keys(x).sort().join() !== "caseId,id"
    )
      throw new Error("OWNERSHIP_UNPROVEN");
    add(x.caseId, x.id);
  }
  if (responses)
    for (const [id, stage] of run.stages) {
      if (id.startsWith("delete:") || stage === "intent") continue;
      try {
        const r = await privateJSON(
          join(dir, `response-${id.replaceAll(":", "-")}.json`),
        );
        if (typeof r.body?.id === "string") add(id.split(":")[0], r.body.id);
        else if (r.status === 201 || r.body?.created === true)
          throw new Error("CREATED_ID_UNAVAILABLE");
      } catch (e: any) {
        if (e.code !== "ENOENT" || stage === "response") throw e;
      }
    }
  if (known.size > 80) throw new Error("FIXTURE_BOUND");
  return known;
}
const literal = (s: string) =>
  s.replaceAll("\\", "\\\\").replaceAll("'", "\\'");
export function normalizationCondition(
  runId: string,
  c: NormalizationCase,
  keys = false,
) {
  const s = normalizationScope(runId, c);
  const names = Object.values(s.names)
    .map((n) => "'" + literal(n) + "'")
    .join(",");
  return (
    `Name IN (${names})` +
    (keys
      ? ` OR Compat_Key__c IN (${Object.values(s.keys)
          .map((k) => "'" + literal(k) + "'")
          .join(",")})`
      : "")
  );
}
export function normalizationOwned(
  runId: string,
  c: NormalizationCase,
  row: any,
  stages?: Map<string, string>,
) {
  const s = normalizationScope(runId, c);
  return c.steps.some(
    (step) =>
      s.names[step.id] === row?.Name &&
      (!stages ||
        ["dispatched", "response", "unknown"].includes(
          stages.get(`${c.id}:${step.id}`) ?? "",
        )),
  );
}
export function normalizationRow(id: string, r: WireResponse): RecordState {
  const b = r.body as any;
  if (
    r.status !== 200 ||
    b?.Id !== id ||
    typeof b.Name !== "string" ||
    !(b.Compat_Key__c === null || typeof b.Compat_Key__c === "string") ||
    !(b.Compat_Text__c === null || typeof b.Compat_Text__c === "string")
  )
    throw new Error("INVALID_STATE");
  return {
    Id: id,
    Name: b.Name,
    Compat_Key__c: b.Compat_Key__c,
    Compat_Text__c: b.Compat_Text__c,
  };
}
export async function normalizationDiscovery(
  config: ExternalIdConfig,
  dir: string,
  t: Transport,
  runId: string,
  c: NormalizationCase,
  evidenceSink?: (e: NormalizationDiscoveryReference) => void,
) {
  const wrapped: Transport = {
    async request(path, init) {
      const r = await t.request(path, init);
      const evidence = await normalizationArtifact(
        dir,
        `discovery-${c.id}-${randomBytes(6).toString("hex")}.json`,
        { caseId: c.id, response: r },
      );
      evidenceSink?.({ caseId: c.id, ...evidence });
      const b = r.body as any;
      if (Array.isArray(b?.records))
        for (const row of b.records)
          if (typeof row?.Id === "string")
            await rememberNormalizationRecord(config, dir, c.id, row.Id);
      return r;
    },
  };
  return query(wrapped, normalizationCondition(runId, c));
}
export async function captureNormalizationState(
  config: ExternalIdConfig,
  dir: string,
  t: Transport,
  runId: string,
  c: NormalizationCase,
): Promise<NormalizationStateSnapshot> {
  const discovery = await normalizationDiscovery(config, dir, t, runId, c),
    records: RecordState[] = [],
    exactReads: NormalizationExactRead[] = [];
  for (const [id, caseId] of await knownNormalizationRecords(config, dir)) {
    if (caseId !== c.id) continue;
    const response = await t.request(recordPath(id), {});
    exactReads.push({ id, response });
    await normalizationArtifact(
      dir,
      `read-${c.id}-${id}-${randomBytes(6).toString("hex")}.json`,
      { caseId: c.id, id, response },
    );
    if (response.status === 404) {
      if (discovery.some((r) => r.Id === id)) throw new Error("STATE_CHANGED");
      continue;
    }
    const row = normalizationRow(id, response);
    const found = discovery.find((r) => r.Id === id);
    if (found && !isDeepStrictEqual(found, row))
      throw new Error("STATE_CHANGED");
    records.push(row);
  }
  records.sort((a, b) => a.Id.localeCompare(b.Id));
  return { records, discovery, exactReads };
}
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
