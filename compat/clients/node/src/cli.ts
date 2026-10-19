import { suite } from "./suites.ts";
import { verifyCapturePair } from "./evidence.ts";
import { referencePreflight, foundationSessionIssues } from "./readiness.ts";
import { validateRecoveryDirectory } from "./fixtures.ts";
import { parseArgs } from "node:util";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import {
  validateRepository,
  observationSummary,
  ROOT,
  profile,
} from "./validate.ts";
import { capture } from "./capture.ts";
import {
  compare,
  sanitize,
  parseDocument,
  validateReport,
  checkConfig,
} from "./core.ts";
import { reportOutcome, writeReport } from "./reports.ts";
import {
  validateOpenApi,
  LATEST_OPENAPI,
  type OpenApiRole,
} from "./openapi.ts";
async function main() {
  const { values, positionals } = parseArgs({
    allowPositionals: true,
    options: {
      target: { type: "string" },
      suite: { type: "string" },
      config: { type: "string" },
      out: { type: "string" },
      reference: { type: "string" },
      candidate: { type: "string" },
      first: { type: "string" },
      second: { type: "string" },
      role: { type: "string" },
      spec: { type: "string" },
    },
  });
  const selected = suite(values.suite ?? "pilot");
  const { requiredScenarios: SCENARIOS, cleanupScenarios: CLEANUP_SCENARIOS } =
    selected;
  const command = positionals[0];
  if (positionals.length !== 1)
    throw new Error(
      "Specify one command: validate, preflight, capture, compare, repeatability, verify-evidence or validate-openapi",
    );
  if (command === "capture-foundation-assumed") {
    if (
      values.target !== "reference" ||
      !values.out ||
      Object.keys(values).some((k) => !["target", "out"].includes(k))
    )
      throw new Error(
        "Assumed foundation capture requires --target reference and --out private.json only",
      );
    const { runAssumedFoundationCapture } =
      await import("./foundation-command.ts");
    const result = await runAssumedFoundationCapture(values.out, process.env);
    console.log(
      `${result.sidecar ? "policy capture recorded" : "blocked"}: private output ${resolve(values.out)}`,
    );
    process.exitCode = result.sidecar ? 0 : 2;
    return;
  }
  if (command === "probe-external-id" || command === "recover-external-id") {
    const mode = command === "probe-external-id" ? "probe" : "recover";
    const allowed =
      mode === "probe" ? ["config", "out"] : ["config", "out", "first"];
    if (
      Object.keys(values).some((k) => !allowed.includes(k)) ||
      !values.config ||
      !values.out ||
      (mode === "recover" && !values.first)
    )
      throw new Error(
        "External-ID command requires private config/output and recovery run directory when recovering",
      );
    const { externalIdCommand } = await import("./external-id-command.ts");
    const result = await externalIdCommand(
      mode,
      values.config,
      values.out,
      process.env,
      values.first,
    );
    const complete =
      "collection" in result
        ? result.collection === "complete" && result.cleanup === "clean"
        : result.outcome === "clean";
    console.log(
      `External-ID ${mode}: ${complete ? "complete" : "incomplete/blocked"}; private output ${resolve(values.out)}`,
    );
    process.exitCode = complete ? 0 : 2;
    return;
  }
  if (command === "discover-metadata") {
    if (Object.keys(values).some((k) => !["config", "out"].includes(k)))
      throw new Error("Discovery accepts only --config and --out");
    if (!values.config || !values.out)
      throw new Error("--config and --out private files required");
    const { discoverMetadataCommand } = await import("./metadata-command.ts");
    const inventory = await discoverMetadataCommand(
      values.config,
      values.out,
      process.env,
    );
    console.log(
      `${inventory.outcome}: ${inventory.facts.length} facts, ${inventory.gaps.length} gaps; private inventory ${resolve(values.out)}`,
    );
    process.exitCode = inventory.outcome === "complete" ? 0 : 2;
    return;
  }
  if (command === "verify-evidence") {
    if (
      !values.first ||
      !values.second ||
      !values.out ||
      !values.out.endsWith(".json")
    )
      throw new Error("--first, --second and --out path.json required");
    const p = await profile(selected.id);
    const result = verifyCapturePair(
      await readFile(resolve(values.first), "utf8"),
      await readFile(resolve(values.second), "utf8"),
      {
        profile: p.data.id,
        profileHash: p.hash,
        apiVersion: p.data.apiVersion,
        requiredScenarios: SCENARIOS,
        connectors: p.data.connectors,
        cleanupScenarios: CLEANUP_SCENARIOS,
      },
    );
    const out = resolve(values.out);
    await mkdir(dirname(out), { recursive: true, mode: 0o700 });
    await writeFile(out, JSON.stringify(result, null, 2) + "\n", {
      mode: 0o600,
      flag: "wx",
    });
    console.log(
      `${result.outcome}: private pair verification; not a reviewed observation`,
    );
    process.exitCode =
      result.outcome === "passed" ? 0 : result.outcome === "failed" ? 1 : 2;
    return;
  }
  if (command === "preflight") {
    if (values.target !== "reference" && values.target !== "candidate")
      throw new Error("--target reference|candidate required");
    const p = await profile(selected.id);
    const issues =
      values.target === "reference"
        ? await referencePreflight(
            ROOT,
            {
              profile: p.data.id,
              profileHash: p.hash,
              apiVersion: p.data.apiVersion,
              requiredScenarios: SCENARIOS,
              connectors: p.data.connectors,
              cleanupScenarios: CLEANUP_SCENARIOS,
            },
            process.env,
            selected.id,
          )
        : checkConfig("candidate", process.env);
    if (selected.id === "rest-query-foundation")
      issues.push(...foundationSessionIssues(values.target, process.env));
    if (process.env.COMPAT_RECOVERY_DIR) {
      const issue = await validateRecoveryDirectory(
        process.env.COMPAT_RECOVERY_DIR,
        ROOT,
      );
      if (issue) issues.push(issue);
    }
    console.log("Offline readiness; no observation performed.");
    if (issues.length) throw new Error(issues.join("; "));
    return;
  }
  if (command === "validate") {
    const issues = await validateRepository();
    if (issues.length) {
      console.error(issues.join("\n"));
      process.exitCode = 1;
    } else {
      const coverage = await observationSummary();
      console.log(
        `Offline contracts, catalogs, dependencies and OpenAPI documents (SalesCompel ${LATEST_OPENAPI}, Salesforce sources 2.0/3.0) validated. Reviewed pilot observations: ${coverage.observed}; not_run: ${coverage.notRun}. Not parity certification.`,
      );
    }
    return;
  }
  if (command === "validate-openapi") {
    if (!values.spec) throw new Error("--spec path required");
    const role = (values.role ?? "salescompel") as OpenApiRole;
    if (!["salescompel", "salesforce-source"].includes(role))
      throw new Error("--role must be salescompel or salesforce-source");
    await validateOpenApi(
      parseDocument(await readFile(resolve(values.spec), "utf8")),
      role,
    );
    console.log(
      role === "salescompel"
        ? `OpenAPI ${LATEST_OPENAPI} structurally valid; no Salesforce runtime compatibility claim.`
        : `Salesforce source document structurally valid; convert to OpenAPI ${LATEST_OPENAPI} before use.`,
    );
    return;
  }
  if (command === "capture") {
    if (values.target !== "reference" && values.target !== "candidate")
      throw new Error("--target reference|candidate required");
    const out = resolve(
      values.out ??
        resolve(ROOT, `compat/reports/${values.target}-${Date.now()}.json`),
    );
    if (!out.endsWith(".json")) throw new Error("--out must end in .json");
    const issues = await validateRepository();
    if (issues.length) throw new Error(issues.join("; "));
    const r = await capture(values.target, process.env, selected.id);
    validateReport(r);
    await writeReport(out, r);
    console.log(`${reportOutcome(r)}: ${out}`);
    process.exitCode =
      reportOutcome(r) === "passed"
        ? 0
        : reportOutcome(r) === "blocked"
          ? 2
          : 1;
    return;
  }
  if (command === "compare" || command === "repeatability") {
    const parity = command === "compare";
    const [first, second] = parity
      ? [values.reference, values.candidate]
      : [values.first, values.second];
    if (!first || !second)
      throw new Error(
        parity
          ? "--reference and --candidate paths required"
          : "--first and --second reference capture paths required",
      );
    const r = JSON.parse(await readFile(resolve(first), "utf8")),
      c = JSON.parse(await readFile(resolve(second), "utf8"));
    const p = await profile(selected.id);
    const comparison = compare(
      r,
      c,
      {
        profile: p.data.id,
        profileHash: p.hash,
        requiredScenarios: SCENARIOS,
        connectors: p.data.connectors,
        cleanupScenarios: CLEANUP_SCENARIOS,
      },
      parity ? "parity" : "repeatability",
    );
    const out = resolve(
      values.out ??
        resolve(
          ROOT,
          `compat/reports/${parity ? "comparison" : "repeatability"}-${Date.now()}.json`,
        ),
    );
    if (!out.endsWith(".json")) throw new Error("--out must end in .json");
    await mkdir(dirname(out), { recursive: true, mode: 0o700 });
    await writeFile(out, JSON.stringify(comparison, null, 2) + "\n", {
      mode: 0o600,
    });
    await writeFile(
      out.replace(/\.json$/, "") + ".md",
      `# ${parity ? "Compatibility comparison" : "Reference repeatability"}\n\nOutcome: **${comparison.outcome}**\n\n${comparison.differences.map((d) => "- " + d.replaceAll("<", "&lt;").replaceAll("\n", " ")).join("\n")}\n\nOnly the captured profile and scenarios are covered.\n`,
      { mode: 0o600 },
    );
    console.log(`${comparison.outcome}: ${out}`);
    process.exitCode =
      comparison.outcome === "passed"
        ? 0
        : comparison.outcome === "blocked"
          ? 2
          : 1;
    return;
  }
  throw new Error(
    "Unknown command; use validate, preflight, capture, compare, repeatability, verify-evidence or validate-openapi",
  );
}
main().catch((e) => {
  const secrets = Object.entries(process.env)
    .filter(([k]) => /TOKEN|PASSWORD|SECRET/.test(k))
    .map(([, v]) => v!)
    .filter(Boolean);
  console.error(sanitize(e.message ?? String(e), secrets));
  process.exitCode = e.exitCode === 1 ? 1 : 2;
});
