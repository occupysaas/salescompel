import { readFile, writeFile, readdir, mkdir } from "node:fs/promises";
import { join } from "node:path";
import { randomBytes } from "node:crypto";
import { ROOT } from "./versions.ts";
import {
  newReport,
  aggregateReport,
  assessCoverage,
  categories,
  type GateID,
  type ParsedResult,
  type AcceptanceReport,
} from "./candidate-acceptance-contract.ts";
import {
  preflightAcceptance,
  collectSourceBindings,
  assetDigest,
  type AcceptanceOptions,
  type GateContext,
} from "./candidate-acceptance-preflight.ts";
import {
  runCommand,
  stopOwnedProcesses,
  type CommandResult,
  type ProcessContext,
} from "./candidate-acceptance-process.ts";
import {
  readGateResult,
  mergeGateResults,
  digest,
} from "./candidate-acceptance-results.ts";
import {
  prepareCandidate,
  startCandidate,
  restartCandidate,
  stopCandidate,
  type OwnedService,
} from "./candidate-acceptance-service.ts";
import { cleanupFixtures } from "./candidate-acceptance-fixtures.ts";
import {
  stepResult,
  runNativeCLI,
  seedRestartFixture,
  verifyRestartFixture,
} from "./candidate-acceptance-clients.ts";
export const databaseTestArgs = () => [
  "test",
  "-race",
  "-count=1",
  "-json",
  "./...",
];
// Step results use a separate namespace from persistence fixtures (restart.json).
export async function publishStepResult(
  c: GateContext,
  id: "cli" | "restart",
  parsed: ParsedResult,
) {
  const path = join(c.scratch, id + "-result.json");
  const bytes = JSON.stringify(parsed) + "\n";
  await writeFile(path, bytes, { flag: "wx", mode: 0o600 });
  await writeFile(
    path + ".binding.json",
    JSON.stringify({
      schemaVersion: 1,
      runId: c.runId,
      gate: id,
      source: c.bindings.source,
      digest: digest(bytes),
      exitCode: parsed.exitCode,
      complete: parsed.complete,
    }),
    { flag: "wx", mode: 0o600 },
  );
  return {
    parsed: await readGateResult("steps", path, c.runId),
    sha256: digest(bytes),
  };
}
interface AcceptanceSignals {
  attach(c: ProcessContext): void;
}
export async function withAcceptanceSignals<T>(
  action: (signals: AcceptanceSignals) => Promise<T>,
): Promise<T> {
  let interrupted = false;
  let context: ProcessContext | undefined;
  const handler = () => {
    interrupted = true;
    if (context) {
      context.interrupted = true;
      void stopOwnedProcesses(context).catch(() => {});
    }
  };
  process.on("SIGINT", handler);
  process.on("SIGTERM", handler);
  try {
    return await action({
      attach(c) {
        context = c;
        if (interrupted) handler();
      },
    });
  } finally {
    process.off("SIGINT", handler);
    process.off("SIGTERM", handler);
  }
}
export async function runAcceptance(
  options: AcceptanceOptions,
  env: NodeJS.ProcessEnv,
  signals?: AcceptanceSignals,
): Promise<AcceptanceReport> {
  if (!signals)
    return withAcceptanceSignals((owned) => runAcceptance(options, env, owned));
  let report = newReport(randomBytes(12).toString("hex"), options.scope),
    c: GateContext | undefined,
    service: OwnedService | undefined,
    active: GateID = "bindings";
  const setFailure = (id: GateID, category: string, blocked = false) => {
    const g = report.gates.find((g) => g.id === id)!;
    g.outcome = blocked ? "blocked" : "failed";
    g.categories = [
      categories.includes(category as any) ? category : "PROCESS",
    ];
    g.finishedAt = new Date().toISOString();
    g.startedAt ??= g.finishedAt;
  };
  const step = async (id: GateID, fn: () => Promise<ParsedResult>) => {
    active = id;
    const startedAt = new Date().toISOString();
    report.gates.find((g) => g.id === id)!.startedAt = startedAt;
    let parsed = await fn();
    if (id === "cli" || id === "restart") {
      const saved = await publishStepResult(c!, id, parsed);
      parsed = saved.parsed;
      report.evidence.push({ gate: id, sha256: saved.sha256 });
    }
    const gate = {
      ...assessCoverage(id, parsed),
      startedAt,
      finishedAt: new Date().toISOString(),
    };
    report.gates[report.gates.findIndex((g) => g.id === id)] = gate;
    if (gate.outcome !== "passed") throw new Error("COVERAGE");
  };
  try {
    c = await preflightAcceptance(options, env, (context) => {
      c = context;
      console.error(`Private candidate diagnostics: ${context.scratch}`);
      signals.attach(context.processes);
    });
    const ctx = c;
    report = newReport(c.runId, options.scope);
    report.bindings = c.bindings;
    const command = async (
      executable: string,
      argv: string[],
      gate: GateID,
      extra: NodeJS.ProcessEnv = {},
      cwd = ROOT,
    ) =>
      runCommand(
        { executable, argv, cwd, env: { ...ctx.env, ...extra } },
        ctx.processes,
        gate,
      );
    const bind = async (path: string, r: CommandResult, gate: GateID) => {
      const bytes = await readFile(path);
      await writeFile(
        path + ".binding.json",
        JSON.stringify({
          schemaVersion: 1,
          runId: ctx.runId,
          gate,
          source: ctx.bindings.source,
          digest: digest(bytes),
          exitCode: r.exitCode,
          complete: r.complete,
        }),
        { flag: "wx", mode: 0o600 },
      );
    };
    const load = async (
      kind: "go" | "node" | "playwright" | "steps",
      path: string,
      gate: GateID,
    ) => {
      const b = JSON.parse(await readFile(path + ".binding.json", "utf8"));
      if (b.gate !== gate || b.source !== ctx.bindings.source)
        throw new Error("RESULT");
      const result = await readGateResult(kind, path, ctx.runId);
      report.evidence.push({ gate, sha256: digest(await readFile(path)) });
      return result;
    };
    const done = (r: CommandResult) => {
      if (r.exitCode !== 0 || !r.complete)
        throw new Error(
          ctx.processes.interrupted
            ? "SIGNAL"
            : Date.now() >= ctx.deadline
              ? "TIMEOUT"
              : "PROCESS",
        );
    };
    if (options.scope === "full") {
      await step("database", async () => {
        const tests = await command("go", databaseTestArgs(), "database");
        const parsed = await load("go", tests.resultPath, "database");
        const vet = await command("go", ["vet", "./..."], "database");
        done(vet);
        return mergeGateResults("database", [
          parsed,
          stepResult(ctx, "database", ["database/vet"]),
        ]);
      });
      await step("lab", async () => {
        const path = join(ctx.scratch, "lab.jsonl"),
          r = await command(
            "npm",
            ["--prefix", "compat/clients/node", "run", "check"],
            "lab",
            { CANDIDATE_NODE_REPORT: path },
          );
        await bind(path, r, "lab");
        return load("node", path, "lab");
      });
      await step("web", async () => {
        const path = join(ctx.scratch, "web-unit.jsonl"),
          files = (await readdir(join(ROOT, "web/test")))
            .filter((f) => f.endsWith(".test.ts"))
            .map((f) => "test/" + f),
          unit = await command(
            process.execPath,
            [
              "--experimental-strip-types",
              "--test",
              "--test-reporter=" +
                join(
                  ROOT,
                  "compat/clients/node/src/candidate-acceptance-node-reporter.mjs",
                ),
              "--test-reporter-destination=" + path,
              ...files,
            ],
            "web",
            {},
            join(ROOT, "web"),
          );
        await bind(path, unit, "web");
        const build = await command(
          "npm",
          ["--prefix", "web", "run", "build"],
          "web",
        );
        done(build);
        return mergeGateResults("web", [
          await load("node", path, "web"),
          stepResult(ctx, "web", ["web/build"]),
        ]);
      });
    }
    active = "bindings";
    await prepareCandidate(ctx);
    ctx.bindings.binary = digest(await readFile(ctx.binary));
    ctx.bindings.assets = await assetDigest();
    await step("bindings", async () =>
      stepResult(ctx, "bindings", [
        "bindings/source",
        "bindings/tools",
        "bindings/artifacts",
      ]),
    );
    service = await startCandidate(ctx);
    await step("jsforce", async () => {
      const path = join(ctx.scratch, "jsforce.jsonl"),
        r = await command(
          process.execPath,
          [
            "--import",
            "tsx",
            "--test",
            "--test-reporter=" +
              join(
                ROOT,
                "compat/clients/node/src/candidate-acceptance-node-reporter.mjs",
              ),
            "--test-reporter-destination=" + path,
            "test/candidate.test.ts",
          ],
          "jsforce",
          { RUN_CANDIDATE: "1" },
          join(ROOT, "compat/clients/node"),
        );
      await bind(path, r, "jsforce");
      return load("node", path, "jsforce");
    });
    await step("python", async () => {
      const path = join(ctx.scratch, "python.json"),
        r = await command(
          ctx.python,
          ["scripts/candidate-python.py"],
          "python",
          { CANDIDATE_STEP_REPORT: path },
        );
      await bind(path, r, "python");
      return load("steps", path, "python");
    });
    await step("cli", () => runNativeCLI(ctx));
    await step("restart", async () => {
      await seedRestartFixture(ctx);
      service = await restartCandidate(ctx, service!);
      return verifyRestartFixture(ctx);
    });
    await step("connected-browser", async () => {
      const path = join(ctx.scratch, "connected.json"),
        r = await command(
          "npm",
          ["--prefix", "web", "run", "test:e2e"],
          "connected-browser",
          {
            CANDIDATE_BROWSER_REPORT: path,
            CANDIDATE_BROWSER_OUTPUT: join(ctx.scratch, "connected-browser"),
          },
        );
      await bind(path, r, "connected-browser");
      return load("playwright", path, "connected-browser");
    });
    if (options.scope === "full")
      await step("preview", async () => {
        const path = join(ctx.scratch, "preview.json");
        const previewEnv: NodeJS.ProcessEnv = {
          ...ctx.env,
          CANDIDATE_BROWSER_REPORT: path,
          CANDIDATE_BROWSER_OUTPUT: join(ctx.scratch, "preview-browser"),
        };
        for (const key of [
          "CANDIDATE_URL",
          "CANDIDATE_TOKEN",
          "CANDIDATE_OTHER_TOKEN",
          "CANDIDATE_RESTRICTED_TOKEN",
          "CANDIDATE_FIXTURE_JOURNAL",
        ])
          delete previewEnv[key];
        const r = await runCommand(
          {
            executable: "npm",
            argv: ["--prefix", "web", "run", "test:preview"],
            cwd: ROOT,
            env: previewEnv,
          },
          ctx.processes,
          "preview",
        );
        await bind(path, r, "preview");
        return load("playwright", path, "preview");
      });
    const final = await collectSourceBindings();
    if (
      final.source !== ctx.bindings.source ||
      final.revision !== ctx.bindings.revision ||
      (options.scope === "full" && !final.clean) ||
      (await assetDigest()) !== ctx.bindings.assets ||
      digest(await readFile(ctx.binary)) !== ctx.bindings.binary
    )
      throw new Error("SOURCE");
  } catch (e: any) {
    c ??= e.context;
    const code = c?.processes.interrupted
      ? "SIGNAL"
      : typeof e?.message === "string" && categories.includes(e.message)
        ? e.message
        : "PROCESS";
    setFailure(
      active,
      code,
      ["PREREQUISITE", "SOURCE", "TIMEOUT", "SIGNAL"].includes(code),
    );
  } finally {
    if (c) {
      try {
        report.cleanup = await cleanupFixtures(c);
      } catch {
        report.cleanup.verified = false;
      }
      try {
        if (service) await stopCandidate(service);
        await stopOwnedProcesses(c.processes);
        report.cleanup.processesStopped = true;
      } catch {
        report.cleanup.processesStopped = false;
      }
      const cleanup = assessCoverage(
        "cleanup",
        stepResult(
          c,
          "cleanup",
          report.cleanup.verified && report.cleanup.processesStopped
            ? ["cleanup/records", "cleanup/processes"]
            : [],
        ),
      );
      report.gates[report.gates.findIndex((g) => g.id === "cleanup")] = cleanup;
    } else report.cleanup.processesStopped = true;
    report.finishedAt = new Date().toISOString();
  }
  return aggregateReport(report);
}
