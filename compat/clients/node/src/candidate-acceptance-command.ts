import { parseArgs } from "node:util";
import { mkdtemp } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { pathToFileURL } from "node:url";
import {
  runAcceptance,
  withAcceptanceSignals,
} from "./candidate-acceptance-run.ts";
import { validateOutputDestination } from "./candidate-acceptance-preflight.ts";
import { publishAcceptanceReport } from "./candidate-acceptance-contract.ts";
export async function candidateAcceptanceMain(
  argv: string[],
  env: NodeJS.ProcessEnv,
): Promise<0 | 1 | 2> {
  let args;
  try {
    args = parseArgs({
      args: argv,
      allowPositionals: true,
      options: { out: { type: "string" }, port: { type: "string" } },
    });
    if (
      args.positionals.length !== 1 ||
      !["full", "candidate-only"].includes(args.positionals[0]) ||
      (args.positionals[0] === "full" && !args.values.out)
    )
      return 1;
  } catch {
    return 1;
  }
  const scope = args.positionals[0] as "full" | "candidate-only";
  const out =
    args.values.out ??
    join(
      await mkdtemp(join(tmpdir(), "salescompel-candidate-summary-")),
      "summary.json",
    );
  try {
    await validateOutputDestination(out);
    if (!args.values.out) console.log(`Candidate-only summary: ${out}`);
    return await withAcceptanceSignals(async (signals) => {
      const report = await runAcceptance(
        {
          scope,
          out: resolve(out),
          ...(args.values.port ? { port: Number(args.values.port) } : {}),
        },
        env,
        signals,
      );
      await publishAcceptanceReport(out, report);
      return report.outcome === "passed"
        ? 0
        : report.outcome === "failed"
          ? 1
          : 2;
    });
  } catch {
    return 2;
  }
}
if (
  process.argv[1] &&
  import.meta.url === pathToFileURL(resolve(process.argv[1])).href
) {
  process.exitCode = await candidateAcceptanceMain(
    process.argv.slice(2),
    process.env,
  );
  console.log(
    `Candidate acceptance ${process.exitCode === 0 ? "passed" : process.exitCode === 1 ? "failed" : "blocked"}; no Salesforce compatibility certification.`,
  );
}
