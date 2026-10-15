import { parseArgs } from "node:util";
import { readFile, writeFile, mkdir } from "node:fs/promises";
import { resolve, dirname } from "node:path";
import {
  validateRepository,
  ROOT,
  profile,
  SCENARIOS,
  CLEANUP_SCENARIOS,
} from "./validate.ts";
import { capture } from "./capture.ts";
import { compare, sanitize, parseDocument, validateReport } from "./core.ts";
import { reportOutcome, writeReport } from "./reports.ts";
import { validateOpenApi } from "./openapi.ts";
async function main() {
  const { values, positionals } = parseArgs({
    allowPositionals: true,
    options: {
      target: { type: "string" },
      out: { type: "string" },
      reference: { type: "string" },
      candidate: { type: "string" },
      first: { type: "string" },
      second: { type: "string" },
      spec: { type: "string" },
    },
  });
  const command = positionals[0];
  if (positionals.length !== 1)
    throw new Error(
      "Specify one command: validate, capture, compare, repeatability or validate-openapi",
    );
  if (command === "validate") {
    const issues = await validateRepository();
    if (issues.length) {
      console.error(issues.join("\n"));
      process.exitCode = 1;
    } else
      console.log(
        "Offline contracts, catalogs, dependencies and OpenAPI 3.0 validated. Live Salesforce verification: pending.",
      );
    return;
  }
  if (command === "validate-openapi") {
    if (!values.spec) throw new Error("--spec path required");
    await validateOpenApi(
      parseDocument(await readFile(resolve(values.spec), "utf8")),
    );
    console.log(
      "OpenAPI 3.0 structurally valid; no Salesforce runtime compatibility claim.",
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
    const r = await capture(values.target, process.env);
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
    const p = await profile();
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
    "Unknown command; use validate, capture, compare, repeatability or validate-openapi",
  );
}
main().catch((e) => {
  const secrets = Object.entries(process.env)
    .filter(([k]) => /TOKEN|PASSWORD|SECRET/.test(k))
    .map(([, v]) => v!)
    .filter(Boolean);
  console.error(sanitize(e.message ?? String(e), secrets));
  process.exitCode = 2;
});
