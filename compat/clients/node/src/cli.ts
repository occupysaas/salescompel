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
import type { Values } from "./cli-types.ts";
import {
  inventoryHandlers,
  externalIdHandlers,
  foundationHandlers,
} from "./cli-private-commands.ts";

const say = (s: string) => process.stdout.write(s + "\n");
let selectedCommand: string | undefined;

const options: Record<string, { type: "string" }> = {
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
};

const setExit = (code: number) => {
  process.exitCode = code;
};

type SuiteSelection = ReturnType<typeof suite>;

function selection(values: Values): SuiteSelection {
  return suite(values.suite ?? "pilot");
}

function captureOut(values: Values): string {
  return resolve(
    values.out ??
      resolve(ROOT, `compat/reports/${values.target}-${Date.now()}.json`),
  );
}

function comparisonOut(values: Values, parity: boolean): string {
  return resolve(
    values.out ??
      resolve(
        ROOT,
        `compat/reports/${parity ? "comparison" : "repeatability"}-${Date.now()}.json`,
      ),
  );
}

async function runCoreCommand(
  command: string,
  values: Values,
  selected: SuiteSelection,
  SCENARIOS: string[],
  CLEANUP_SCENARIOS: string[],
): Promise<boolean> {
  const sayWith = say;
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
    sayWith(
      `${result.outcome}: private pair verification; not a reviewed observation`,
    );
    setExit(
      result.outcome === "passed" ? 0 : result.outcome === "failed" ? 1 : 2,
    );
    return true;
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
    sayWith("Offline readiness; no observation performed.");
    if (issues.length) throw new Error(issues.join("; "));
    return true;
  }
  if (command === "validate") {
    const issues = await validateRepository();
    if (issues.length) {
      console.error(issues.join("\n"));
      setExit(1);
    } else {
      const coverage = await observationSummary();
      sayWith(
        `Offline contracts, catalogs, dependencies and OpenAPI documents (SalesCompel ${LATEST_OPENAPI}, Salesforce sources 2.0/3.0) validated. Reviewed pilot observations: ${coverage.observed}; not_run: ${coverage.notRun}. Not parity certification.`,
      );
    }
    return true;
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
    sayWith(
      role === "salescompel"
        ? `OpenAPI ${LATEST_OPENAPI} structurally valid; no Salesforce runtime compatibility claim.`
        : `Salesforce source document structurally valid; convert to OpenAPI ${LATEST_OPENAPI} before use.`,
    );
    return true;
  }
  if (command === "capture") {
    if (values.target !== "reference" && values.target !== "candidate")
      throw new Error("--target reference|candidate required");
    const out = captureOut(values);
    if (!out.endsWith(".json")) throw new Error("--out must end in .json");
    const issues = await validateRepository();
    if (issues.length) throw new Error(issues.join("; "));
    const r = await capture(values.target, process.env, selected.id);
    validateReport(r);
    await writeReport(out, r);
    sayWith(`${reportOutcome(r)}: ${out}`);
    setExit(
      reportOutcome(r) === "passed"
        ? 0
        : reportOutcome(r) === "blocked"
          ? 2
          : 1,
    );
    return true;
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
    const out = comparisonOut(values, parity);
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
    sayWith(`${comparison.outcome}: ${out}`);
    setExit(
      comparison.outcome === "passed"
        ? 0
        : comparison.outcome === "blocked"
          ? 2
          : 1,
    );
    return true;
  }
  return false;
}

async function main() {
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
      await inventoryHandlers["validate-target-inventory"](
        values,
        say,
        setExit,
      );
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
  const selected = selection(values);
  const { requiredScenarios: SCENARIOS, cleanupScenarios: CLEANUP_SCENARIOS } =
    selected;
  const command = selectedCommand;
  if (positionals.length !== 1)
    throw new Error(
      "Specify one command: validate, preflight, capture, compare, repeatability, verify-evidence or validate-openapi",
    );
  if (command === "discover-metadata") {
    if (Object.keys(values).some((k) => !["config", "out"].includes(k)))
      throw new Error("Discovery accepts only --config and --out");
    await inventoryHandlers.discoverMetadata(values, say, setExit);
    return;
  }
  for (const table of [externalIdHandlers, foundationHandlers]) {
    const handler = table[command];
    if (handler) {
      await handler(values, say, setExit);
      return;
    }
  }
  if (
    await runCoreCommand(
      command,
      values,
      selected,
      SCENARIOS,
      CLEANUP_SCENARIOS,
    )
  )
    return;
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
