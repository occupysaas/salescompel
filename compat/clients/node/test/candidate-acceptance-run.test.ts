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
