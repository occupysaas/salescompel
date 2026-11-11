import { createHash } from "node:crypto";
import type { RecordState } from "./external-id-contract.ts";
import type { NormalizationCleanup } from "./external-id-normalization-contract.ts";
import type { JournalEvent } from "./external-id-recovery.ts";
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
