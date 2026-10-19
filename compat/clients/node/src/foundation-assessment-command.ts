import { open, readFile } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { parseDocument } from "./core.ts";
import { privateDirectory } from "./metadata-files.ts";
import { ROOT } from "./versions.ts";
import {
  assessFoundationPair,
  type CleanupAbsenceEvidence,
  type FoundationAssessment,
  type ReleaseFinding,
} from "./foundation-assessment.ts";
import {
  currentFoundationBinding,
  readFoundationSidecar,
  readPrivateFoundationFile,
} from "./foundation-sidecar.ts";

export interface FoundationAssessmentPaths {
  first: string;
  second: string;
  firstPolicy: string;
  secondPolicy: string;
  firstCleanup: string;
  secondCleanup: string;
  out: string;
}

export function foundationReleaseFindings(document: unknown): ReleaseFinding[] {
  const data = document as any;
  if (
    data?.schemaVersion !== 1 ||
    !Array.isArray(data.findings) ||
    !Array.isArray(data.blockers)
  )
    throw new Error("Malformed release findings");
  const items = [...data.findings, ...data.blockers];
  const ids = items.map((item: any) => item?.id);
  if (new Set(ids).size !== ids.length)
    throw new Error("Duplicate release finding identifiers");
  const relevant = new Set([
    "query-response-shape",
    "query-limit-above-batch",
    "id-15-18",
    "case-insensitive-names",
    "case-insensitive-comparison",
    "null-ordering",
    "error-shape",
    "unknown-sobject",
    "query-batch-header",
    "query-locator-lifecycle",
    "rest-route-parity",
    "duplicate-canonical-fields",
  ]);
  return items.map((finding: any) => {
    if (
      typeof finding?.id !== "string" ||
      !Array.isArray(finding.affectedContracts ?? [])
    )
      throw new Error("Malformed release finding");
    return {
      id: finding.id,
      relevant:
        (finding.affectedContracts ?? []).some((id: string) =>
          relevant.has(id),
        ) && finding.blocksContractFreeze === true,
      resolved:
        finding.status === "resolved" ||
        finding.status === "reviewed-and-reconciled",
    };
  });
}

export async function assessFoundationFiles(
  paths: FoundationAssessmentPaths,
): Promise<FoundationAssessment> {
  const entries = Object.values(paths).map((path) => resolve(path));
  if (new Set(entries).size !== entries.length || !paths.out.endsWith(".json"))
    throw new Error("Assessment paths must be distinct private JSON files");
  await privateDirectory(dirname(resolve(paths.out)));
  const [
    first,
    second,
    firstSidecar,
    secondSidecar,
    firstCleanup,
    secondCleanup,
  ] = await Promise.all([
    readPrivateFoundationFile(resolve(paths.first)),
    readPrivateFoundationFile(resolve(paths.second)),
    readFoundationSidecar(resolve(paths.first), resolve(paths.firstPolicy)),
    readFoundationSidecar(resolve(paths.second), resolve(paths.secondPolicy)),
    readPrivateFoundationFile(resolve(paths.firstCleanup)),
    readPrivateFoundationFile(resolve(paths.secondCleanup)),
  ]);
  const releaseText = await readFile(
    resolve(ROOT, "docs/compat/releases/changes.yaml"),
    "utf8",
  );
  const releaseFindings = foundationReleaseFindings(parseDocument(releaseText));
  const result = assessFoundationPair(
    first,
    firstSidecar,
    second,
    secondSidecar,
    await currentFoundationBinding(),
    [
      JSON.parse(firstCleanup) as CleanupAbsenceEvidence,
      JSON.parse(secondCleanup) as CleanupAbsenceEvidence,
    ],
    releaseFindings,
  );
  const handle = await open(resolve(paths.out), "wx", 0o600);
  try {
    await handle.writeFile(JSON.stringify(result, null, 2) + "\n");
    await handle.sync();
  } finally {
    await handle.close();
  }
  return result;
}
