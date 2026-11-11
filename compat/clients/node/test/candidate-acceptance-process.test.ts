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
      argv: ["-e", 'process.stdout.write("secret-canary\\n");process.exitCode=3'],
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
test("shutdown owns grandchildren after the leader exits and escalates resistant groups", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p15-descendant-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const context = newProcessContext("b".repeat(24), dir, Date.now() + 10000);
  const result = await runCommand(
    {
      executable: process.execPath,
      argv: [
        "-e",
        `const {spawn}=require('node:child_process');const c=spawn(process.execPath,['-e', 'process.on("SIGTERM",()=>{});process.stdout.write("ready\\n");setInterval(()=>{},1000)'],{stdio:['ignore','pipe','ignore']});c.stdout.once('data',()=>{process.stdout.write(String(c.pid)+"\\n");c.stdout.destroy();c.unref()})`,
      ],
      cwd: dir,
      env: {},
    },
    context,
    "lab",
  );
  const pid = Number((await readFile(result.resultPath, "utf8")).trim());
  t.after(() => {
    try {
      process.kill(pid, "SIGKILL");
    } catch {}
  });
  await stopOwnedProcesses(context);
  const stat = await readFile(`/proc/${pid}/stat`, "utf8").catch(() => "");
  assert.ok(
    !stat || stat.slice(stat.lastIndexOf(")") + 2).startsWith("Z "),
    "owned grandchild must be stopped",
  );
});

test("OAuth subprocess output is captured in memory before any evidence is written", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p26-private-output-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const context = newProcessContext("c".repeat(24), dir, Date.now() + 5000);
  const secret = "private-output-canary";
  let captured: { stdout: string; stderr: string } | undefined;
  const result = await runCommand(
    {
      executable: process.execPath,
      argv: [
        "-e",
        "process.stdout.write('private-output-');setTimeout(()=>{process.stdout.write('canary');process.stderr.write('private-output-canary')},30)",
      ],
      cwd: dir,
      env: {},
      privateOutput: (stdout: string, stderr: string) => {
        captured = { stdout, stderr };
      },
    } as any,
    context,
    "oauth",
  );
  assert.equal(result.exitCode, 0);
  assert.deepEqual(captured, { stdout: secret, stderr: secret });
  for (const path of [
    result.resultPath,
    result.resultPath + ".stderr",
    result.bindingPath,
  ])
    assert.ok(!(await readFile(path, "utf8")).includes(secret));
  await stopOwnedProcesses(context);
});
