import { randomBytes } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import type { ExternalIdConfig, RecordState } from "./external-id-contract.ts";
import type { BodyCase } from "./external-id-body-contract.ts";
import { query, recordPath, type Transport } from "./external-id-recovery.ts";
import type {
  BodyDiscoveryReference,
  BodyExactRead,
  BodyStateSnapshot,
} from "./external-id-body-recovery-models.ts";
import {
  bodyArtifact,
  bodyCondition,
  bodyRow,
  knownBodyRecords,
  rememberBodyRecord,
} from "./external-id-body-recovery-helpers.ts";
export async function bodyDiscovery(
  config: ExternalIdConfig,
  dir: string,
  t: Transport,
  runId: string,
  c: BodyCase,
  evidenceSink?: (e: BodyDiscoveryReference) => void,
) {
  const wrapped: Transport = {
    async request(path, init) {
      const r = await t.request(path, init);
      const evidence = await bodyArtifact(
        dir,
        `discovery-${c.id}-${randomBytes(6).toString("hex")}.json`,
        { caseId: c.id, response: r },
      );
      evidenceSink?.({ caseId: c.id, ...evidence });
      const b = r.body as { records?: Array<{ Id?: unknown }> } | null;
      if (Array.isArray(b?.records))
        for (const row of b.records)
          if (typeof row?.Id === "string")
            await rememberBodyRecord(config, dir, c.id, row.Id);
      return r;
    },
  };
  return query(wrapped, bodyCondition(runId, c));
}
export async function captureBodyState(
  config: ExternalIdConfig,
  dir: string,
  t: Transport,
  runId: string,
  c: BodyCase,
): Promise<BodyStateSnapshot> {
  const discovery = await bodyDiscovery(config, dir, t, runId, c),
    records: RecordState[] = [],
    exactReads: BodyExactRead[] = [];
  for (const [id, caseId] of await knownBodyRecords(config, dir)) {
    if (caseId !== c.id) continue;
    const response = await t.request(recordPath(id), {});
    exactReads.push({ id, response });
    await bodyArtifact(
      dir,
      `read-${c.id}-${id}-${randomBytes(6).toString("hex")}.json`,
      { caseId: c.id, id, response },
    );
    if (response.status === 404) {
      if (discovery.some((r) => r.Id === id)) throw new Error("STATE_CHANGED");
      continue;
    }
    const row = bodyRow(id, response);
    const found = discovery.find((r) => r.Id === id);
    if (found && !isDeepStrictEqual(found, row))
      throw new Error("STATE_CHANGED");
    records.push(row);
  }
  records.sort((a, b) => a.Id.localeCompare(b.Id));
  return { records, discovery, exactReads };
}
