import { open } from "node:fs/promises";
import { dirname, join, resolve } from "node:path";
import { validateReport } from "./core.ts";
import { privateDirectory } from "./metadata-files.ts";
import {
  auditFoundationRecovery,
  collectFoundationAbsence,
  foundationRunId,
  foundationJournalDigest,
} from "./foundation-live.ts";
import {
  readFoundationSidecar,
  readPrivateFoundationFile,
} from "./foundation-sidecar.ts";
import type { CleanupAbsenceEvidence } from "./foundation-assessment.ts";

export async function verifyFoundationCleanupFiles(
  capturePath: string,
  policyPath: string,
  outPath: string,
  env: NodeJS.ProcessEnv,
): Promise<CleanupAbsenceEvidence> {
  const capture = resolve(capturePath),
    policy = resolve(policyPath),
    out = resolve(outPath);
  if (new Set([capture, policy, out]).size !== 3 || !out.endsWith(".json"))
    throw new Error("Distinct private foundation cleanup paths required");
  await privateDirectory(dirname(out));
  const [raw, sidecar] = await Promise.all([
    readPrivateFoundationFile(capture),
    readFoundationSidecar(capture, policy),
  ]);
  const parsed: unknown = JSON.parse(raw);
  validateReport(parsed);
  if (parsed.target !== "reference" || parsed.provenance !== "live")
    throw new Error("Live reference capture required for cleanup verification");
  const recoveryRoot = await privateDirectory(env.COMPAT_RECOVERY_DIR ?? "");
  const clear = await auditFoundationRecovery(recoveryRoot, parsed);
  const result = await collectFoundationAbsence(
    parsed,
    sidecar.captureSha256,
    sidecar.identityDigest,
    env,
    clear,
  );
  const runId = foundationRunId(parsed);
  if (!runId) throw new Error("Foundation run identity missing");
  const marker = join(
    await privateDirectory(join(recoveryRoot, `reference-${runId}`)),
    "foundation-absence.json",
  );
  const handle = await open(out, "wx", 0o600);
  try {
    await handle.writeFile(JSON.stringify(result, null, 2) + "\n");
    await handle.sync();
  } finally {
    await handle.close();
  }
  const journalDigest = await foundationJournalDigest(dirname(marker));
  const markerHandle = await open(marker, "wx", 0o600);
  try {
    await markerHandle.writeFile(
      JSON.stringify({ evidence: result, journalDigest }, null, 2) + "\n",
    );
    await markerHandle.sync();
  } finally {
    await markerHandle.close();
  }
  return result;
}
