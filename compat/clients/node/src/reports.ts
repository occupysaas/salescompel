import { mkdir, writeFile } from "node:fs/promises";
import { dirname } from "node:path";
import type { Report } from "./core.ts";
export function reportOutcome(r: Report) {
  if (
    r.results.some((s) => s.outcome === "failed") ||
    r.cleanup.some((c) => c.outcome === "failed")
  )
    return "failed";
  if (!r.results.length || r.results.some((s) => s.outcome !== "passed"))
    return "blocked";
  return "passed";
}
const safe = (s: string) =>
  s
    .replaceAll("|", "\\|")
    .replaceAll("\n", " ")
    .replaceAll("<", "&lt;")
    .replaceAll(">", "&gt;");
export async function writeReport(path: string, report: Report) {
  await mkdir(dirname(path), { recursive: true, mode: 0o700 });
  await writeFile(path, JSON.stringify(report, null, 2) + "\n", {
    mode: 0o600,
  });
  const lines = [
    `# Compatibility capture: ${report.target}`,
    ``,
    `Outcome: **${reportOutcome(report)}**. Evidence: ${report.provenance}.`,
    ``,
    "No parity certification: this is one capture, not a universal compatibility claim.",
    "",
    `Profile: ${safe(report.profile)}. API: ${safe(report.apiVersion ?? "unconfigured")}.`,
    "",
    "| Scenario | Client | Outcome | Reason |",
    "| --- | --- | --- | --- |",
    ...report.results.map(
      (s) =>
        `| ${safe(s.id)} | ${safe(s.client)} | ${s.outcome} | ${safe(s.reason ?? "")} |`,
    ),
    "",
    "## Cleanup",
    ...report.cleanup.map(
      (c) =>
        `- ${safe(c.resource)}: ${c.outcome}${c.reason ? ` (${safe(c.reason)})` : ""}`,
    ),
    "",
    "Fixture recovery journals remain in ignored compat/private. Review before sharing any report.",
  ];
  await writeFile(
    path.replace(/\.json$/, "") + ".md",
    lines.join("\n") + "\n",
    { mode: 0o600 },
  );
}
