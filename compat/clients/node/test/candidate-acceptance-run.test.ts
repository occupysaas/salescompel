import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import { candidateAcceptanceMain } from "../src/candidate-acceptance-command.ts";
test("full command blocks before fixture work and publishes only a sanitized report", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p15-run-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const out = join(dir, "summary.json");
  assert.equal(await candidateAcceptanceMain(["full", "--out", out], {}), 2);
  const raw = await readFile(out, "utf8"),
    r = JSON.parse(raw);
  assert.equal(r.outcome, "blocked");
  assert.equal(r.acceptanceEligible, false);
  assert.equal(r.gates.find((g: any) => g.id === "jsforce").outcome, "not-run");
  assert.ok(!raw.includes("DATABASE_URL"));
  assert.equal(await candidateAcceptanceMain(["full", "--out", out], {}), 2);
});
test("command requires explicit output and rejects arbitrary gate selectors", async () => {
  assert.equal(await candidateAcceptanceMain(["full"], {}), 1);
  assert.equal(
    await candidateAcceptanceMain(["full", "--skip", "database"], {}),
    1,
  );
});
test("CI invokes acceptance once and preserves its failure without selecting private artifacts", async (t) => {
  const { readDoc } = await import("../src/validate.ts");
  const workflow = await readDoc(".github/workflows/platform.yml");
  const steps = workflow.jobs.candidate.steps;
  const stage = steps.find((s: any) => s.name === "Full candidate acceptance");
  assert.ok(stage);
  const dir = await mkdtemp(join(tmpdir(), "p15-ci-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const { mkdir, writeFile } = await import("node:fs/promises");
  await mkdir(join(dir, "scripts"));
  await writeFile(
    join(dir, "scripts/check-candidate-acceptance.sh"),
    '#!/bin/sh\nprintf "called\\n" >> "$RUNNER_TEMP/calls"\nprintf "{}" > "$RUNNER_TEMP/candidate-acceptance.json"\nexit 1\n',
    { mode: 0o700 },
  );
  const { spawnSync } = await import("node:child_process");
  const r = spawnSync("bash", ["-e", "-c", stage.run], {
    cwd: dir,
    env: { ...process.env, RUNNER_TEMP: dir },
  });
  assert.equal(r.status, 1);
  assert.equal((await readFile(join(dir, "calls"), "utf8")).trim(), "called");
  const upload = steps.find((s: any) =>
    s.uses?.startsWith("actions/upload-artifact"),
  );
  assert.equal(upload.if, "always()");
  assert.equal(
    upload.with.path,
    "${{ runner.temp }}/candidate-acceptance.json",
  );
});
test("documented shell entry point works from repository root before any prerequisites", async (t) => {
  const { spawnSync } = await import("node:child_process"),
    { ROOT } = await import("../src/versions.ts");
  const dir = await mkdtemp(join(tmpdir(), "p15-shell-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const out = join(dir, "summary.json");
  const env = { ...process.env };
  delete env.CANDIDATE_DISPOSABLE;
  delete env.DATABASE_URL;
  const r = spawnSync(
    "bash",
    ["scripts/check-candidate-acceptance.sh", "--out", out],
    { cwd: ROOT, env, encoding: "utf8" },
  );
  assert.equal(r.status, 2);
  assert.equal(JSON.parse(await readFile(out, "utf8")).outcome, "blocked");
});
test("database gate executes fresh tests twice instead of replaying the Go cache", async (t) => {
  const { databaseTestArgs } =
    await import("../src/candidate-acceptance-run.ts");
  const { writeFile } = await import("node:fs/promises");
  const { execFileSync } = await import("node:child_process");
  const dir = await mkdtemp(join(tmpdir(), "p15-go-fresh-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await writeFile(join(dir, "go.mod"), "module acceptancefresh\n\ngo 1.25\n");
  await writeFile(
    join(dir, "fresh_test.go"),
    'package acceptancefresh\nimport("fmt";"testing";"time")\nfunc TestFresh(t *testing.T){fmt.Println("fresh",time.Now().UnixNano())}\n',
  );
  const run = () =>
    execFileSync("go", databaseTestArgs(), {
      cwd: dir,
      env: { ...process.env, GOTOOLCHAIN: "local", GOPROXY: "off" },
      encoding: "utf8",
    }).match(/fresh \d+/)?.[0];
  const first = run(),
    second = run();
  assert.ok(first);
  assert.notEqual(first, second);
});
test("signal ownership starts before preflight work and persists through asynchronous shutdown", async (t) => {
  const { withAcceptanceSignals } =
    await import("../src/candidate-acceptance-run.ts");
  const { newProcessContext, spawnOwned, stopOwnedProcesses } =
    await import("../src/candidate-acceptance-process.ts");
  const dir = await mkdtemp(join(tmpdir(), "p15-signal-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const before = process.listenerCount("SIGTERM");
  await withAcceptanceSignals(async (signals) => {
    process.emit("SIGTERM");
    const c = newProcessContext("c".repeat(24), dir, Date.now() + 5000);
    signals.attach(c);
    assert.equal(c.interrupted, true);
    assert.throws(
      () =>
        spawnOwned(
          { executable: process.execPath, argv: [], cwd: dir, env: {} },
          c,
          "bindings",
        ),
      /SIGNAL/,
    );
    await stopOwnedProcesses(c);
    await new Promise((r) => setTimeout(r, 10));
    process.emit("SIGTERM");
    assert.equal(process.listenerCount("SIGTERM"), before + 1);
  });
  assert.equal(process.listenerCount("SIGTERM"), before);
  await withAcceptanceSignals(async (signals) => {
    const c = newProcessContext("c".repeat(24), dir, Date.now() + 5000);
    signals.attach(c);
    const child = spawnOwned(
      {
        executable: process.execPath,
        argv: ["-e", "setInterval(()=>{},1000)"],
        cwd: dir,
        env: {},
      },
      c,
      "bindings",
    );
    process.emit("SIGINT");
    await child.done;
    await stopOwnedProcesses(c);
    assert.equal(c.interrupted, true);
    assert.equal(c.children.size, 0);
  });
});
test("invalid publication destinations never create a blocked report", async (t) => {
  const { ROOT } = await import("../src/versions.ts");
  const { lstat } = await import("node:fs/promises");
  const dir = await mkdtemp(join(tmpdir(), "p15-output-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  for (const path of [
    join(dir, "summary.txt"),
    join(ROOT, `p15-invalid-${process.pid}.json`),
  ]) {
    t.after(() => rm(path, { force: true }));
    assert.equal(await candidateAcceptanceMain(["full", "--out", path], {}), 2);
    await assert.rejects(lstat(path), { code: "ENOENT" });
  }
});
