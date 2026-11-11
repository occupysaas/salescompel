import { readFile } from "node:fs/promises";
import { join } from "node:path";
import { isDeepStrictEqual as equal } from "node:util";
import { privateJSON, recordPath } from "./external-id-recovery.ts";
import type { NormalizationStateSnapshot } from "./external-id-normalization-recovery-models.ts";
import type { BodyStateSnapshot } from "./external-id-body-recovery-models.ts";
import type { RecordState } from "./external-id-contract.ts";
import type { WireResponse } from "./wire.ts";
type Row = (id: string, r: WireResponse) => RecordState;
type Snapshot = {
  records: RecordState[];
  discovery: RecordState[];
  exactReads: { id: string; response: WireResponse }[];
};
export interface DiscoveryBody {
  done?: unknown;
  records?: Array<Record<string, any>>;
  totalSize?: unknown;
  nextRecordsUrl?: unknown;
}
export interface RetainedOriginal {
  caseId: string;
  id?: string;
  response: {
    status?: number;
    bodyPresent?: boolean;
    headers?: Record<string, string>;
    body?: { records?: unknown } | null;
  };
}
/** Build the artifact/evidence validators for one evidence family. */
export function evidenceTools(
  evidenceInvalid: string,
  digest: (bytes: string) => string,
  row: Row,
) {
  const requireEvidence = (ok: unknown) => {
    if (!ok) throw new Error(evidenceInvalid);
  };
  async function artifact(dir: string, name: string, sha?: string) {
    requireEvidence(/^[A-Za-z0-9][A-Za-z0-9._-]*\.json$/.test(name));
    const value = await privateJSON(join(dir, name));
    if (sha !== undefined)
      requireEvidence(digest(await readFile(join(dir, name), "utf8")) === sha);
    return value;
  }
  function wire(r: unknown): asserts r is WireResponse {
    const v = r as WireResponse;
    requireEvidence(
      v &&
        Number.isInteger(v.status) &&
        v.status >= 100 &&
        v.status <= 599 &&
        v.headers &&
        typeof v.headers === "object" &&
        !Array.isArray(v.headers) &&
        Object.values(v.headers).every((h) => typeof h === "string") &&
        Object.hasOwn(v, "body") &&
        (v.bodyPresent === undefined || typeof v.bodyPresent === "boolean"),
    );
    requireEvidence(
      ![401, 403].includes(v.status) &&
        !(
          v.body &&
          typeof v.body === "object" &&
          Object.hasOwn(v.body, "nonJson")
        ),
    );
    if (v.bodyPresent === false) requireEvidence(v.body === null);
  }
  function discovery(r: unknown) {
    wire(r);
    const b = r.body as DiscoveryBody;
    requireEvidence(
      r.status === 200 &&
        b?.done === true &&
        Array.isArray(b.records) &&
        b.totalSize === b.records.length &&
        !b.nextRecordsUrl,
    );
    const rows = (b.records as Array<Record<string, any>>).map((record) =>
      row(record.Id as string, { status: 200, body: record, headers: {} }),
    );
    requireEvidence(new Set(rows.map((r) => r.Id)).size === rows.length);
    return rows;
  }
  function snapshot<T extends Snapshot>(s: T): T {
    requireEvidence(
      s &&
        Array.isArray(s.records) &&
        Array.isArray(s.discovery) &&
        Array.isArray(s.exactReads),
    );
    const rows = [];
    requireEvidence(
      new Set(s.exactReads.map((x) => x.id)).size === s.exactReads.length,
    );
    for (const x of s.exactReads) {
      recordPath(x.id);
      wire(x.response);
      if (x.response.status !== 404) rows.push(row(x.id, x.response));
    }
    rows.sort((a, b) => a.Id.localeCompare(b.Id));
    requireEvidence(equal(rows, s.records));
    const discovered = s.discovery.map((r) =>
      row(r.Id, { status: 200, body: r, headers: {} }),
    );
    requireEvidence(
      new Set(discovered.map((r) => r.Id)).size === discovered.length,
    );
    requireEvidence(
      discovered.every((r) => s.records.some((x) => equal(x, r))),
    );
    return s;
  }
  return {
    requireEvidence,
    artifact,
    wire,
    discovery,
    snapshot,
    snapshotState: snapshot as (s: Snapshot) => Snapshot,
  };
}
export type NormalizationEvidenceTools = ReturnType<typeof evidenceTools> & {
  snapshot: (s: NormalizationStateSnapshot) => NormalizationStateSnapshot;
};
export type BodyEvidenceTools = ReturnType<typeof evidenceTools> & {
  snapshot: (s: BodyStateSnapshot) => BodyStateSnapshot;
};
