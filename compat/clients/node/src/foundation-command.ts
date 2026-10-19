import { open } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { capture } from "./capture.ts";
import { validateReport, type Report } from "./core.ts";
import { privateDirectory } from "./metadata-files.ts";
import {
  currentFoundationBinding,
  writeFoundationSidecar,
} from "./foundation-sidecar.ts";
import { validateRepository, ROOT } from "./validate.ts";
import { referencePreflight, foundationSessionIssues } from "./readiness.ts";
import { suite } from "./suites.ts";
import { foundationLivePreflight } from "./foundation-live.ts";

export async function runAssumedFoundationCapture(
  path: string,
  env: NodeJS.ProcessEnv,
): Promise<{ report: Report; sidecar?: string }> {
  const target = resolve(path);
  if (!target.endsWith(".json"))
    throw new Error("Private output must end in .json");
  await privateDirectory(dirname(target));
  const selected = suite("rest-query-foundation");
  const binding = await currentFoundationBinding();
  const issues = await validateRepository();
  issues.push(
    ...(await referencePreflight(
      ROOT,
      {
        profile: binding.profile,
        profileHash: binding.profileHash,
        apiVersion: binding.apiVersion,
        connectors: binding.connectors,
        requiredScenarios: selected.requiredScenarios,
        cleanupScenarios: selected.cleanupScenarios,
      },
      env,
      selected.id,
    )),
    ...foundationSessionIssues("reference", env),
  );
  if (env.COMPAT_RECOVERY_DIR) await privateDirectory(env.COMPAT_RECOVERY_DIR);
  if (issues.length) throw new Error(issues.join("; "));
  const identityDigest = await foundationLivePreflight(env);
  const output = await open(target, "wx", 0o600);
  try {
    const report = await capture(
      "reference",
      env,
      "rest-query-foundation",
      "documented-two-day-assumption",
    );
    validateReport(report);
    await output.writeFile(JSON.stringify(report, null, 2) + "\n");
    await output.sync();
    const cursor = report.results.find((r) => r.id === "foundation-cursors");
    const eligible =
      report.provenance === "live" &&
      cursor?.outcome === "blocked" &&
      cursor.reason ===
        "Cursor expiry unmeasured under foundation-query-result-lifetime" &&
      report.results.every(
        (r) => r.id === "foundation-cursors" || r.outcome === "passed",
      ) &&
      report.cleanup.length > 0 &&
      report.cleanup.every((c) => c.outcome === "passed");
    return {
      report,
      ...(eligible
        ? {
            sidecar: await writeFoundationSidecar(
              target,
              report,
              binding,
              identityDigest,
            ),
          }
        : {}),
    };
  } finally {
    await output.close();
  }
}
