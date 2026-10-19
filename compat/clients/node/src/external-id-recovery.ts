import { open, readFile, lstat, readdir } from "node:fs/promises";
import { join } from "node:path";
import { isDeepStrictEqual } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import { wire, type WireResponse } from "./wire.ts";
import {
  manifest,
  manifestHash,
  caseScope,
  parseExternalIdConfig,
  type ExternalIdConfig,
  type RecordState,
} from "./external-id-contract.ts";
export interface Transport {
  request(path: string, init: RequestInit): Promise<WireResponse>;
  native?(key: string, marker: string): Promise<unknown>;
}
export const base = "/services/data/v65.0/";
export const recordPath = (id: string) => {
  if (!/^[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?$/.test(id))
    throw new Error("INVALID_RECORD_ID");
  return base + "sobjects/Account/" + id;
};
export function transport(
  c: ExternalIdConfig,
  env: NodeJS.ProcessEnv,
): Transport {
  if (
    env.COMPAT_REFERENCE_URL !== c.origin ||
    !env.COMPAT_REFERENCE_TOKEN ||
    env.COMPAT_DISPOSABLE !== "yes"
  )
    throw new Error("EXTERNAL_ID_AUTHORIZATION_MISSING");
  const token = env.COMPAT_REFERENCE_TOKEN;
  return {
    request: (path, init) => wire(new URL(c.origin), token, path, init),
  };
}
export function boundedTransport(t: Transport, deadline: number): Transport {
  return {
    ...t,
    request: (path, init) => {
      const remaining = deadline - Date.now();
      if (remaining <= 0) throw new Error("OPERATION_DEADLINE");
      const signal = AbortSignal.timeout(remaining);
      return t.request(path, {
        ...init,
        signal: init.signal ? AbortSignal.any([init.signal, signal]) : signal,
      });
    },
  };
}
export async function verifyIdentity(c: ExternalIdConfig, t: Transport) {
  const r = await t.request("/services/oauth2/userinfo", {});
  const b = r.body as any;
  if (
    r.status !== 200 ||
    b?.user_id !== c.administrator.user ||
    b?.organization_id !== c.administrator.org
  )
    throw new Error("IDENTITY_MISMATCH");
}
export async function privateJSON(path: string, max = 8 * 1024 * 1024) {
  const s = await lstat(path);
  if (
    !s.isFile() ||
    s.isSymbolicLink() ||
    (s.mode & 0o077) !== 0 ||
    s.size > max ||
    (process.getuid && s.uid !== process.getuid())
  )
    throw new Error("UNSAFE_PRIVATE_FILE");
  return JSON.parse(await readFile(path, "utf8"));
}
export async function durableJSON(path: string, value: unknown) {
  const f = await open(path, "wx", 0o600);
  try {
    await f.writeFile(JSON.stringify(value) + "\n");
    await f.sync();
  } finally {
    await f.close();
  }
}
export interface JournalEvent {
  kind: "intent" | "dispatched" | "response" | "unknown";
  id: string;
}
export async function initializeRun(
  config: ExternalIdConfig,
  dir: string,
  runId: string,
) {
  await privateDirectory(dir);
  await durableJSON(join(dir, "binding.json"), { config, runId, manifestHash });
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
// Serialize append+fsync even when two requests are dispatched concurrently.
const queues = new Map<string, Promise<void>>();
export async function appendJournal(dir: string, event: JournalEvent) {
  const previous = queues.get(dir) ?? Promise.resolve();
  const next = previous.then(async () => {
    const f = await open(join(dir, "journal.jsonl"), "a", 0o600);
    try {
      await f.writeFile(JSON.stringify(event) + "\n");
      await f.sync();
    } finally {
      await f.close();
    }
  });
  queues.set(dir, next);
  try {
    await next;
  } finally {
    if (queues.get(dir) === next) queues.delete(dir);
  }
}
export async function readRun(config: ExternalIdConfig, dir: string) {
  await privateDirectory(dir);
  parseExternalIdConfig(config);
  const binding = await privateJSON(join(dir, "binding.json"), 65536);
  if (
    Object.keys(binding).sort().join() !== "config,manifestHash,runId" ||
    !isDeepStrictEqual(binding.config, config) ||
    binding.manifestHash !== manifestHash ||
    !/^[a-f0-9]{24}$/.test(binding.runId)
  )
    throw new Error("JOURNAL_BINDING_MISMATCH");
  const path = join(dir, "journal.jsonl");
  const info = await lstat(path);
  if (
    !info.isFile() ||
    info.isSymbolicLink() ||
    (info.mode & 0o077) !== 0 ||
    info.size > 1048576 ||
    (process.getuid && info.uid !== process.getuid())
  )
    throw new Error("INVALID_JOURNAL");
  const text = await readFile(path, "utf8");
  if (text && !text.endsWith("\n")) throw new Error("TRUNCATED_JOURNAL");
  const events: JournalEvent[] = text
    .split("\n")
    .filter(Boolean)
    .map((line) => JSON.parse(line));
  const stages = new Map<string, string>();
  const expected = new Set(
    manifest.flatMap((c) => c.steps.map((s) => c.id + ":" + s.id)),
  );
  for (const e of events) {
    if (
      !e ||
      Object.keys(e).sort().join() !== "id,kind" ||
      (!expected.has(e.id) &&
        !/^delete:[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?$/.test(e.id))
    )
      throw new Error("INVALID_JOURNAL");
    const prev = stages.get(e.id);
    if (
      (e.kind === "intent" && prev !== undefined) ||
      (e.kind === "dispatched" && prev !== "intent") ||
      (["response", "unknown"].includes(e.kind) && prev !== "dispatched") ||
      !["intent", "dispatched", "response", "unknown"].includes(e.kind)
    )
      throw new Error("INVALID_JOURNAL_TRANSITION");
    stages.set(e.id, e.kind);
  }
  return { runId: binding.runId as string, events, stages };
}
const literal = (x: string) =>
  x.replaceAll("\\", "\\\\").replaceAll("'", "\\'");
export async function query(
  t: Transport,
  condition: string,
): Promise<RecordState[]> {
  const r = await t.request(
    base +
      "query?q=" +
      encodeURIComponent(
        "SELECT Id,Name,Compat_Key__c,Compat_Text__c FROM Account WHERE " +
          condition,
      ),
    {},
  );
  const b = r.body as any;
  if (
    r.status !== 200 ||
    b?.done !== true ||
    !Array.isArray(b.records) ||
    b.records.length > 80 ||
    b.totalSize !== b.records.length
  )
    throw new Error("QUERY_UNAVAILABLE");
  const rows: RecordState[] = b.records.map((x: any) => {
    recordPath(x.Id);
    if (
      typeof x.Name !== "string" ||
      !(x.Compat_Key__c === null || typeof x.Compat_Key__c === "string") ||
      !(x.Compat_Text__c === null || typeof x.Compat_Text__c === "string")
    )
      throw new Error("INVALID_STATE");
    return {
      Id: x.Id,
      Name: x.Name,
      Compat_Key__c: x.Compat_Key__c,
      Compat_Text__c: x.Compat_Text__c,
    };
  });
  if (new Set(rows.map((r) => r.Id)).size !== rows.length)
    throw new Error("DUPLICATE_STATE_ID");
  return rows;
}
export async function scopedRecords(
  t: Transport,
  runId: string,
  caseId: string,
) {
  const c = manifest.find((c) => c.id === caseId);
  if (!c) throw new Error("UNKNOWN_CASE");
  const { marker, keys } = caseScope(runId, c);
  const rows = await query(t, `Name = '${literal(marker)}'`);
  for (const row of rows) {
    if (row.Name !== marker || !keys.includes(row.Compat_Key__c ?? ""))
      throw new Error("OWNERSHIP_UNPROVEN");
    const r = await t.request(recordPath(row.Id), {});
    const b = r.body as any;
    if (
      r.status !== 200 ||
      b?.Id !== row.Id ||
      b?.Name !== row.Name ||
      b?.Compat_Key__c !== row.Compat_Key__c ||
      b?.Compat_Text__c !== row.Compat_Text__c
    )
      throw new Error("STATE_CHANGED");
  }
  return rows;
}
export async function preflightCase(
  t: Transport,
  runId: string,
  caseId: string,
) {
  const c = manifest.find((c) => c.id === caseId)!;
  const { marker, keys } = caseScope(runId, c);
  const rows = await query(
    t,
    `Name = '${literal(marker)}' OR Compat_Key__c IN (${[...new Set(keys)].map((k) => "'" + literal(k) + "'").join(",")})`,
  );
  if (rows.length) throw new Error("PREFLIGHT_COLLISION");
}
export async function latchIdentity(dir: string) {
  try {
    await durableJSON(join(dir, "identity-blocked.json"), {
      reason: "IDENTITY_MISMATCH",
    });
  } catch (e: any) {
    if (e.code !== "EEXIST") throw e;
  }
}
export async function rememberRecord(dir: string, caseId: string, id: string) {
  recordPath(id);
  if (!manifest.some((c) => c.id === caseId)) throw new Error("UNKNOWN_CASE");
  const path = join(dir, `known-${id}.json`);
  try {
    await durableJSON(path, { caseId, id });
  } catch (e: any) {
    if (e.code !== "EEXIST") throw e;
    const previous = await privateJSON(path, 65536);
    if (previous.id !== id || previous.caseId !== caseId)
      throw new Error("OWNERSHIP_UNPROVEN");
  }
}
async function knownRecordIds(
  dir: string,
  run: Awaited<ReturnType<typeof readRun>>,
) {
  const known = new Map<string, Set<string>>();
  const add = (caseId: string, id: string) => {
    recordPath(id);
    if (!manifest.some((c) => c.id === caseId)) throw new Error("UNKNOWN_CASE");
    let ids = known.get(caseId);
    if (!ids) {
      ids = new Set();
      known.set(caseId, ids);
    }
    ids.add(id);
  };
  for (const [id, stage] of run.stages) {
    if (id.startsWith("delete:") || stage === "intent") continue;
    try {
      const response = await privateJSON(
        join(dir, `response-${id.replaceAll(":", "-")}.json`),
      );
      if (typeof response.body?.id === "string")
        add(id.split(":")[0], response.body.id);
      else if (response.status === 201 || response.body?.created === true)
        throw new Error("CREATED_ID_UNAVAILABLE");
    } catch (e: any) {
      if (e.code !== "ENOENT" || stage === "response") throw e;
    }
  }
  for (const name of await readdir(dir)) {
    if (!/^known-[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?\.json$/.test(name))
      continue;
    const item = await privateJSON(join(dir, name), 65536);
    if (name !== `known-${item.id}.json`) throw new Error("OWNERSHIP_UNPROVEN");
    add(item.caseId, item.id);
  }
  return known;
}
export interface RecoveryResult {
  outcome: "clean" | "pending" | "blocked";
  unresolved: string[];
  deleted: number;
  reasons: string[];
}
export async function recoverExternalIdRun(
  config: ExternalIdConfig,
  env: NodeJS.ProcessEnv,
  dir: string,
  injected?: Transport,
): Promise<RecoveryResult> {
  const result: RecoveryResult = {
    outcome: "blocked",
    unresolved: [],
    deleted: 0,
    reasons: [],
  };
  try {
    const run = await readRun(config, dir);
    try {
      await privateJSON(join(dir, "identity-blocked.json"), 65536);
      throw new Error("IDENTITY_LATCHED");
    } catch (e: any) {
      if (e.code !== "ENOENT") throw e;
    }
    const known = await knownRecordIds(dir, run);
    const t = boundedTransport(
      injected ?? transport(config, env),
      Date.now() + 300000,
    );
    await verifyIdentity(config, t);
    const deadline = Date.now() + 300000;
    const cases = new Set(
      run.events
        .filter((e) => e.kind === "dispatched" && !e.id.startsWith("delete:"))
        .map((e) => e.id.split(":")[0]),
    );
    const checkedIds = new Set<string>();
    for (const caseId of cases) {
      if (Date.now() > deadline) throw new Error("CLEANUP_DEADLINE");
      await verifyIdentity(config, t);
      const rows = await scopedRecords(t, run.runId, caseId);
      for (const row of rows) {
        await rememberRecord(dir, caseId, row.Id);
        let ids = known.get(caseId);
        if (!ids) {
          ids = new Set();
          known.set(caseId, ids);
        }
        ids.add(row.Id);
      }
      const scope = caseScope(
        run.runId,
        manifest.find((c) => c.id === caseId)!,
      );
      for (const id of known.get(caseId) ?? []) {
        checkedIds.add(id);
        const response = await t.request(recordPath(id), {});
        if (response.status === 404) continue;
        const b = response.body as any;
        if (
          response.status !== 200 ||
          b?.Id !== id ||
          b?.Name !== scope.marker ||
          !scope.keys.includes(b?.Compat_Key__c)
        )
          throw new Error("OWNERSHIP_UNPROVEN");
        if (!rows.some((row) => row.Id === id))
          rows.push({
            Id: id,
            Name: b.Name,
            Compat_Key__c: b.Compat_Key__c,
            Compat_Text__c: b.Compat_Text__c,
          });
      }

      for (const row of rows) {
        checkedIds.add(row.Id);
        if (checkedIds.size > 80) throw new Error("FIXTURE_BOUND");
        const id = "delete:" + row.Id;
        const stage = run.stages.get(id);
        if (stage) {
          if (stage !== "response") continue;
          throw new Error("RECORD_REAPPEARED");
        }
        await appendJournal(dir, { kind: "intent", id });
        await appendJournal(dir, { kind: "dispatched", id });
        try {
          const r = await t.request(recordPath(row.Id), { method: "DELETE" });
          await appendJournal(dir, { kind: "response", id });
          if (![204, 404].includes(r.status)) throw new Error("DELETE_FAILED");
          result.deleted++;
        } catch {
          throw new Error("DELETE_UNAVAILABLE");
        }
      }
      for (const id of known.get(caseId) ?? []) {
        const r = await t.request(recordPath(id), {});
        if (r.status !== 404) throw new Error("ABSENCE_UNVERIFIED");
      }
      if ((await scopedRecords(t, run.runId, caseId)).length)
        throw new Error("ABSENCE_UNVERIFIED");
    }
    await verifyIdentity(config, t);
    const final = await readRun(config, dir);
    result.unresolved = [...final.stages]
      .filter(([, v]) => v === "dispatched" || v === "unknown")
      .map(([id]) => id);
    result.outcome = result.unresolved.length ? "blocked" : "clean";
    if (result.unresolved.length) result.reasons.push("UNRESOLVED_DISPATCH");
  } catch (e) {
    if (e instanceof Error && e.message === "IDENTITY_MISMATCH")
      await latchIdentity(dir);
    result.reasons.push(
      e instanceof Error && /^[A-Z_]+$/.test(e.message)
        ? e.message
        : "RECOVERY_UNAVAILABLE",
    );
  }
  return result;
}
