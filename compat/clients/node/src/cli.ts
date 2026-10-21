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
let selectedCommand: string | undefined;
async function main() {
  const options = {
    input: { type: "string" },
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
    "first-policy": { type: "string" },
    "second-policy": { type: "string" },
    "first-cleanup": { type: "string" },
    "second-cleanup": { type: "string" },
    capture: { type: "string" },
    policy: { type: "string" },
  } as const;
  // Use the same option arity when identifying the command after parse errors.
  selectedCommand = parseArgs({
    allowPositionals: true,
    strict: false,
    options,
  }).positionals[0];
  if (selectedCommand === "validate-target-inventory") {
    // Keep parsing failures and all input-derived error prose out of diagnostics.
    try {
      const { values, positionals } = parseArgs({
        allowPositionals: true,
        options,
      });
      if (
        positionals.length !== 1 ||
        !values.input ||
        values.out === "" ||
        Object.keys(values).some((k) => !["input", "out"].includes(k))
      ) {
        console.error(
          "Invalid target inventory arguments; use --input and optional --out only",
        );
        process.exitCode = 1;
        return;
      }
      const { validateTargetInventoryFile } =
        await import("./target-inventory-command.ts");
      const { report, exitCode } = await validateTargetInventoryFile(
        values.input,
        values.out,
      );
      if (values.out)
        console.log(
          `${report.validation}: ${report.readiness}; ${exitCode === 2 ? "report unavailable" : "private report written"}`,
        );
      else console.log(JSON.stringify(report));
      process.exitCode = exitCode;
    } catch {
      console.error(
        "Invalid target inventory arguments or unavailable validator",
      );
      process.exitCode = 1;
    }
    return;
  }
  const { values, positionals } = parseArgs({
    allowPositionals: true,
    options,
  });
  selectedCommand = positionals[0];
  const selected = suite(values.suite ?? "pilot");
  const { requiredScenarios: SCENARIOS, cleanupScenarios: CLEANUP_SCENARIOS } =
    selected;
  const command = selectedCommand;
  if (positionals.length !== 1)
    throw new Error(
      "Specify one command: validate, preflight, capture, compare, repeatability, verify-evidence or validate-openapi",
    );
  if (command === "assess-external-id-normalization") {
    if (
      !values.first ||
      !values.second ||
      !values.out ||
      Object.keys(values).some((k) => !["first", "second", "out"].includes(k))
    )
      throw new Error(
        "Normalization assessment requires --first, --second and --out only",
      );
    const { privateDirectory } = await import("./metadata-files.ts");
    const { assessNormalizationPair } =
      await import("./external-id-normalization-assess.ts");
    const { open } = await import("node:fs/promises");
    const target = resolve(values.out);
    await privateDirectory(dirname(target));
    if (!target.endsWith(".json"))
      throw new Error("Private output must end in .json");
    const output = await open(target, "wx", 0o600);
    try {
      const result = await assessNormalizationPair(values.first, values.second);
      await output.writeFile(JSON.stringify(result) + "\n");
      await output.sync();
      console.log(`${result.outcome}: private output ${target}`);
      process.exitCode =
        result.outcome === "repeatable"
          ? 0
          : result.outcome === "drift"
            ? 1
            : 2;
    } finally {
      await output.close();
    }
    return;
  }
  if (
    command === "probe-external-id-normalization" ||
    command === "recover-external-id-normalization"
  ) {
    const mode =
      command === "probe-external-id-normalization" ? "probe" : "recover";
    const allowed =
      mode === "probe" ? ["config", "out"] : ["config", "out", "first"];
    if (
      !values.config ||
      !values.out ||
      (mode === "recover" && !values.first) ||
      Object.keys(values).some((k) => !allowed.includes(k))
    )
      throw new Error(
        "Normalization command requires private --config and --out; recovery also requires --first",
      );
    const { normalizationCommand } =
      await import("./external-id-normalization-command.ts");
    const result = await normalizationCommand(
      mode,
      values.config,
      values.out,
      process.env,
      values.first,
    );
    const clean =
      "collection" in result
        ? result.collection === "complete" && result.cleanup.outcome === "clean"
        : result.outcome === "clean";
    console.log(
      `${clean ? "complete" : "incomplete/blocked"}: private output ${resolve(values.out)}`,
    );
    process.exitCode = clean ? 0 : 2;
    return;
  }
  if (command === "assess-external-id-body") {
    if (
      !values.first ||
      !values.second ||
      !values.out ||
      Object.keys(values).some((k) => !["first", "second", "out"].includes(k))
    )
      throw new Error(
        "Body assessment requires --first, --second and --out only",
      );
    const { privateDirectory } = await import("./metadata-files.ts");
    const { assessBodyPair } = await import("./external-id-body-assess.ts");
    const { open } = await import("node:fs/promises");
    const target = resolve(values.out);
    await privateDirectory(dirname(target));
    if (!target.endsWith(".json"))
      throw new Error("Private output must end in .json");
    const output = await open(target, "wx", 0o600);
    try {
      const result = await assessBodyPair(values.first, values.second);
      await output.writeFile(JSON.stringify(result) + "\n");
      await output.sync();
      console.log(`${result.outcome}: private output ${target}`);
      process.exitCode = result.outcome === "ready" ? 0 : 2;
    } finally {
      await output.close();
    }
    return;
  }
  if (
    command === "probe-external-id-body" ||
    command === "recover-external-id-body"
  ) {
    const mode = command === "probe-external-id-body" ? "probe" : "recover";
    const allowed =
      mode === "probe" ? ["config", "out"] : ["config", "out", "first"];
    if (
      !values.config ||
      !values.out ||
      (mode === "recover" && !values.first) ||
      Object.keys(values).some((k) => !allowed.includes(k))
    )
      throw new Error(
        "Body command requires private --config and --out; recovery also requires --first",
      );
    const { bodyCommand } = await import("./external-id-body-command.ts");
    const result = await bodyCommand(
      mode,
      values.config,
      values.out,
      process.env,
      values.first,
    );
    const clean =
      "collection" in result
        ? result.collection === "complete" && result.cleanup.outcome === "clean"
        : result.outcome === "clean";
    console.log(
      `${clean ? "complete" : "incomplete/blocked"}: private output ${resolve(values.out)}`,
    );
    process.exitCode = clean ? 0 : 2;
    return;
  }
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
  if (command === "assess-foundation") {
    const allowed = [
      "first",
      "second",
      "first-policy",
      "second-policy",
      "first-cleanup",
      "second-cleanup",
      "out",
    ];
    if (
      Object.keys(values).some((key) => !allowed.includes(key)) ||
      allowed.some((key) => !values[key as keyof typeof values])
    )
      throw new Error(
        "Foundation assessment requires two captures, policies, cleanup attestations and private output",
      );
    const { assessFoundationFiles } =
      await import("./foundation-assessment-command.ts");
    const result = await assessFoundationFiles({
      first: values.first!,
      second: values.second!,
      firstPolicy: values["first-policy"]!,
      secondPolicy: values["second-policy"]!,
      firstCleanup: values["first-cleanup"]!,
      secondCleanup: values["second-cleanup"]!,
      out: values.out!,
    });
    console.log(
      `${result.outcome}: private foundation assessment ${resolve(values.out!)}`,
    );
    process.exitCode =
      result.outcome === "ready-with-assumptions"
        ? 0
        : result.outcome === "drift"
          ? 1
          : 2;
    return;
  }
  if (command === "verify-foundation-cleanup") {
    if (
      !values.capture ||
      !values.policy ||
      !values.out ||
      Object.keys(values).some(
        (key) => !["capture", "policy", "out"].includes(key),
      )
    )
      throw new Error(
        "Foundation cleanup verification requires --capture, --policy and --out private JSON paths",
      );
    const { verifyFoundationCleanupFiles } =
      await import("./foundation-cleanup-command.ts");
    await verifyFoundationCleanupFiles(
      values.capture,
      values.policy,
      values.out,
      process.env,
    );
    console.log(`verified absence: private output ${resolve(values.out)}`);
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
  process.exitCode =
    selectedCommand?.endsWith("external-id-body") ||
    selectedCommand?.endsWith("external-id-normalization")
      ? 1
      : e.exitCode === 1
        ? 1
        : 2;
});
