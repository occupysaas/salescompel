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
  bodyManifest,
  bodyManifestHash,
  bodyScope,
  type BodyCase,
} from "./external-id-body-contract.ts";
import {
  appendJournal,
  durableJSON,
  privateJSON,
  recordPath,
  type JournalEvent,
} from "./external-id-recovery.ts";
import { bodyDigest } from "./external-id-body-recovery-models.ts";
import type { BodyRun } from "./external-id-body-recovery-models.ts";
import type { WireResponse } from "./wire.ts";
const operations = new Set(
  bodyManifest.flatMap((c) =>
    c.recordState === "existing"
      ? [`${c.id}:seed`, `${c.id}:request`]
      : [`${c.id}:request`],
  ),
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
export async function initializeBodyRun(
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
    kind: "external-id-body-validation",
    manifestHash: bodyManifestHash,
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
export async function readBodyRun(
  config: ExternalIdConfig,
  dir: string,
): Promise<BodyRun> {
  await privateDirectory(dir);
  parseExternalIdConfig(config);
  const b = await privateJSON(join(dir, "binding.json"), 65536);
  if (
    Object.keys(b).sort().join() !== "config,kind,manifestHash,runId" ||
    b.kind !== "external-id-body-validation" ||
    b.manifestHash !== bodyManifestHash ||
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
export async function appendBodyEvent(
  config: ExternalIdConfig,
  dir: string,
  e: JournalEvent,
) {
  const r = await readBodyRun(config, dir);
  transition(r.stages, e);
  await appendJournal(dir, e);
}
export async function bodyArtifact(dir: string, name: string, value: unknown) {
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
    sha256: bodyDigest(await readFile(join(dir, name), "utf8")),
  };
}
export async function assertBodyIdentityNotLatched(dir: string) {
  try {
    await privateJSON(join(dir, "identity-blocked.json"));
    throw new Error("IDENTITY_LATCHED");
  } catch (e: unknown) {
    const err = e as { code?: string };
    if (err.code !== "ENOENT") throw e;
  }
}
export async function rememberBodyRecord(
  config: ExternalIdConfig,
  dir: string,
  caseId: string,
  id: string,
) {
  await readBodyRun(config, dir);
  recordPath(id);
  if (!bodyManifest.some((c) => c.id === caseId))
    throw new Error("UNKNOWN_CASE");
  const path = join(dir, `known-${id}.json`);
  try {
    await bodyArtifact(dir, `known-${id}.json`, { caseId, id });
  } catch (e: unknown) {
    const err = e as { code?: string };
    if (err.code !== "EEXIST") throw e;
    const p = await privateJSON(path);
    if (p.id !== id || p.caseId !== caseId)
      throw new Error("OWNERSHIP_UNPROVEN");
  }
  if ((await knownBodyRecords(config, dir, false)).size > 80)
    throw new Error("FIXTURE_BOUND");
}
export async function knownBodyRecords(
  config: ExternalIdConfig,
  dir: string,
  responses = true,
) {
  const run = await readBodyRun(config, dir),
    known = new Map<string, string>();
  const add = (caseId: string, id: string) => {
    recordPath(id);
    if (
      !bodyManifest.some((c) => c.id === caseId) ||
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
export function bodyCondition(runId: string, c: BodyCase) {
  const s = bodyScope(runId, c);
  return `Name IN ('${literal(s.seedName)}','${literal(s.requestName)}') OR Compat_Key__c IN ('${literal(s.pathKey)}','${literal(s.variantKey)}','${literal(s.differentKey)}')`;
}
export function bodyOwned(runId: string, c: BodyCase, row: RecordState) {
  const s = bodyScope(runId, c);
  return (
    [s.seedName, s.requestName].includes(row?.Name) &&
    [s.pathKey, s.variantKey, s.differentKey, "", null].includes(
      row?.Compat_Key__c,
    )
  );
}
export function bodyRow(id: string, r: WireResponse): RecordState {
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
