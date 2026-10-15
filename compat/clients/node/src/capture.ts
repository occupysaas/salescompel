import { randomBytes } from "node:crypto";
import { mkdir } from "node:fs/promises";
import { resolve } from "node:path";
import { Connection } from "jsforce";
import { checkConfig, sanitize, type Report, type Result } from "./core.ts";
import { ROOT, profile, SCENARIOS } from "./validate.ts";
import { FixtureTracker, validateRecoveryDirectory } from "./fixtures.ts";
import { jsScenarios, Blocked, type Context } from "./scenarios.ts";
import { cliQuery, metadata, pythonProbe } from "./process-clients.ts";
import { connectorVersions } from "./versions.ts";
export async function capture(
  target: "reference" | "candidate",
  env: NodeJS.ProcessEnv,
): Promise<Report> {
  const p = await profile();
  const startedAt = new Date().toISOString();
  const prefix = `COMPAT_${target.toUpperCase()}`;
  const report: Report = {
    schemaVersion: 1,
    target,
    profile: p.data.id,
    profileHash: p.hash,
    apiVersion:
      env.COMPAT_API_VERSION && /^\d+\.0$/.test(env.COMPAT_API_VERSION)
        ? env.COMPAT_API_VERSION
        : null,
    provenance: "none",
    startedAt,
    finishedAt: startedAt,
    connectors: connectorVersions(),
    requiredScenarios: SCENARIOS,
    results: [],
    cleanup: [],
  };
  const issues = checkConfig(target, env);
  if (env.COMPAT_RECOVERY_DIR) {
    const recoveryIssue = await validateRecoveryDirectory(
      env.COMPAT_RECOVERY_DIR,
      ROOT,
    );
    if (recoveryIssue) issues.push(recoveryIssue);
  }
  const secrets = Object.entries(env)
    .filter(([k]) => /TOKEN|PASSWORD|SECRET/.test(k))
    .map(([, v]) => v!)
    .filter(Boolean);
  const results = () =>
    SCENARIOS.map(
      (id) =>
        ({
          id,
          client: ["metadata", "cli-query"].includes(id)
            ? "salesforce-cli"
            : id === "python-crud"
              ? "simple-salesforce"
              : "jsforce",
          outcome: "not_run",
          assertions: [],
          observations: {},
          identities: {},
        }) as Result,
    );
  report.results = results();
  if (issues.length) {
    for (const r of report.results) {
      r.outcome = "blocked";
      r.reason = issues.join("; ");
    }
    report.finishedAt = new Date().toISOString();
    return sanitize(report, secrets);
  }
  const conn = new Connection({
    instanceUrl: env[`${prefix}_URL`]!,
    accessToken: env[`${prefix}_TOKEN`]!,
    version: report.apiVersion!,
    httpProxy: undefined,
  });
  const runId = randomBytes(8).toString("hex");
  const privateDir = resolve(
    env.COMPAT_RECOVERY_DIR ?? resolve(ROOT, "compat/private"),
    `${target}-${runId}`,
  );
  await mkdir(privateDir, { recursive: true, mode: 0o700 });
  for (const result of report.results) {
    const journal = resolve(privateDir, `${result.id}.jsonl`);
    const tracker = new FixtureTracker(journal);
    const ctx: Context = {
      conn,
      env,
      prefix,
      runId,
      tracker,
      result,
      cleanup: report.cleanup,
    };
    try {
      report.provenance = "live";
      if (result.id === "cli-query") await cliQuery(ctx);
      else if (result.id === "metadata") await metadata(ctx);
      else if (result.id === "python-crud") await pythonProbe(ctx, journal);
      else await jsScenarios[result.id](ctx);
      result.outcome =
        result.assertions.length && result.assertions.every((a) => a.passed)
          ? "passed"
          : "failed";
    } catch (e: any) {
      result.outcome = e instanceof Blocked ? "blocked" : "failed";
      result.reason = e.errorCode ?? e.message ?? "Scenario failed";
    } finally {
      report.cleanup.push(
        ...(await tracker.cleanup()).map((c) => ({
          ...c,
          scenario: result.id,
        })),
      );
    }
  }
  report.finishedAt = new Date().toISOString();
  return sanitize(report, secrets);
}
