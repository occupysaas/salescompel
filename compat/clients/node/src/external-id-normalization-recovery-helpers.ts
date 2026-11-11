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
} from "./external-id-normalization-contract.ts";
import {
  appendJournal,
  durableJSON,
  privateJSON,
  recordPath,
  type JournalEvent,
} from "./external-id-recovery.ts";
import { normalizationDigest } from "./external-id-normalization-recovery-models.ts";
import type { NormalizationRun } from "./external-id-normalization-recovery-models.ts";
import type { WireResponse } from "./wire.ts";
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
  } catch (e: unknown) {
    const err = e as { code?: string };
    if (err.code !== "ENOENT") throw e;
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
  } catch (e: unknown) {
    const err = e as { code?: string };
    if (err.code !== "EEXIST") throw e;
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
      } catch (e: unknown) {
        const err = e as { code?: string };
        if (err.code !== "ENOENT" || stage === "response") throw e;
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
  row: RecordState,
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
  const b = r.body as RecordState;
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
