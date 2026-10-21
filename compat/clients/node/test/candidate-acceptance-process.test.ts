import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm, readFile } from "node:fs/promises";
import { join } from "node:path";
import { tmpdir } from "node:os";
import {
  runCommand,
  newProcessContext,
  stopOwnedProcesses,
} from "../src/candidate-acceptance-process.ts";
test("bounded subprocess captures private logs, exit status and terminates a hanging process", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p15-process-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const context = newProcessContext("a".repeat(24), dir, Date.now() + 2000);
  const r = await runCommand(
    {
      executable: process.execPath,
      argv: ["-e", 'console.log("secret-canary");process.exitCode=3'],
      cwd: dir,
      env: {},
    },
    context,
    "lab",
  );
  assert.equal(r.exitCode, 3);
  assert.ok((await readFile(r.resultPath, "utf8")).includes("secret-canary"));
  assert.ok(!JSON.stringify(r).includes("secret-canary"));
  const hanging = await runCommand(
    {
      executable: process.execPath,
      argv: ["-e", "setInterval(()=>{},1000)"],
      cwd: dir,
      env: {},
    },
    context,
    "lab",
  );
  assert.equal(hanging.complete, false);
  await stopOwnedProcesses(context);
  assert.equal(context.children.size, 0);
});
