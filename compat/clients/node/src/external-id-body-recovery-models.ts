import { createHash } from "node:crypto";
import type { RecordState } from "./external-id-contract.ts";
import type { BodyCleanup } from "./external-id-body-contract.ts";
import type { JournalEvent } from "./external-id-recovery.ts";
import type { WireResponse } from "./wire.ts";
export interface BodyRun {
  runId: string;
  events: JournalEvent[];
  stages: Map<string, string>;
}
export interface BodyExactRead {
  id: string;
  response: WireResponse;
}
export interface BodyDiscoveryArtifact {
  caseId: string;
  response: WireResponse;
}
export interface BodyAbsenceArtifact extends BodyExactRead {
  caseId: string;
}
export interface BodyDiscoveryReference {
  caseId: string;
  artifact: string;
  sha256: string;
}
export interface BodyCleanupProof {
  cleanup: BodyCleanup;
  discoveryArtifacts: BodyDiscoveryReference[];
}
export const bodyCleanupProofName = (cleanup: BodyCleanup) =>
  `cleanup-proof-${bodyDigest(JSON.stringify(cleanup))}.json`;
export interface BodyStateSnapshot {
  records: RecordState[];
  discovery: RecordState[];
  exactReads: BodyExactRead[];
}
export interface BodyStateArtifact {
  caseId: string;
  before: BodyStateSnapshot | null;
  after: BodyStateSnapshot | null;
  error?: string;
}
export const bodyDigest = (bytes: string) =>
  createHash("sha256").update(bytes).digest("hex");
export const bodyReason = (e: unknown) =>
  e instanceof Error && /^[A-Z_]+$/.test(e.message)
    ? e.message
    : "BODY_EVIDENCE_UNAVAILABLE";
