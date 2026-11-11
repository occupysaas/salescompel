import { randomBytes } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import type { ExternalIdConfig, RecordState } from "./external-id-contract.ts";
import type { NormalizationCase } from "./external-id-normalization-contract.ts";
import { query, recordPath, type Transport } from "./external-id-recovery.ts";
import type {
  NormalizationDiscoveryReference,
  NormalizationExactRead,
  NormalizationStateSnapshot,
} from "./external-id-normalization-recovery-models.ts";
import {
  knownNormalizationRecords,
  normalizationArtifact,
  normalizationCondition,
  normalizationRow,
  rememberNormalizationRecord,
} from "./external-id-normalization-recovery-helpers.ts";
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
      const b = r.body as { records?: Array<{ Id?: unknown }> } | null;
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
