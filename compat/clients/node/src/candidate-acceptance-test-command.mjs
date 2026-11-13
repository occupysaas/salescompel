import { spawn } from "node:child_process";
import { readdirSync } from "node:fs";
import { availableParallelism } from "node:os";
import { fileURLToPath } from "node:url";
const files = readdirSync("test")
  .filter((f) => f.endsWith(".test.ts"))
  .sort()
  .map((f) => "test/" + f);
const report = process.env.CANDIDATE_NODE_REPORT;
const argv = [
  "--import",
  "tsx",
  "--test",
  // Node serializes every file by default on a two-CPU runner, so the suite
  // cannot meet the documented 20-minute acceptance deadline there. Use the
  // available CPUs, capped to bound memory and fixed-port pressure.
  "--test-concurrency=" + Math.min(4, Math.max(1, availableParallelism())),
  ...(report
    ? [
        "--test-reporter=" +
          fileURLToPath(
            new URL(
              "./candidate-acceptance-node-reporter.mjs",
              import.meta.url,
            ),
          ),
        "--test-reporter-destination=" + report,
      ]
    : []),
  ...files,
];
const child = spawn(process.execPath, argv, { stdio: "inherit" });
child.on("error", () => {
  process.exitCode = 1;
});
child.on("exit", (code) => {
  process.exitCode = code ?? 1;
});
