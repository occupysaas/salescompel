import { spawn } from "node:child_process";
import { readdirSync } from "node:fs";
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
