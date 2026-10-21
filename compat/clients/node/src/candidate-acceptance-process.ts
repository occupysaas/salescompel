import { spawn, type ChildProcess } from "node:child_process";
import {
  openSync,
  writeSync,
  closeSync,
  fsyncSync,
  readdirSync,
  readFileSync,
} from "node:fs";
import { writeFile, readFile } from "node:fs/promises";
import { join } from "node:path";
import { randomBytes } from "node:crypto";
import { digest } from "./candidate-acceptance-results.ts";
import type { GateID } from "./candidate-acceptance-contract.ts";
export interface CommandSpec {
  executable: string;
  argv: string[];
  cwd: string;
  env: NodeJS.ProcessEnv;
  timeout?: number;
}
export interface ProcessContext {
  runId: string;
  scratch: string;
  deadline: number;
  children: Set<ChildProcess>;
  source: string;
  interrupted: boolean;
}
export interface CommandResult {
  exitCode: number | null;
  complete: boolean;
  resultPath: string;
  bindingPath: string;
  categories: string[];
}
export const newProcessContext = (
  runId: string,
  scratch: string,
  deadline: number,
): ProcessContext => ({
  runId,
  scratch,
  deadline,
  children: new Set(),
  source: "",
  interrupted: false,
});
function signalGroup(child: ChildProcess, signal: NodeJS.Signals) {
  if (child.pid)
    try {
      process.kill(-child.pid, signal);
    } catch (e: any) {
      if (e.code !== "ESRCH") throw e;
    }
}
function groupAlive(child: ChildProcess) {
  if (!child.pid) return false;
  if (process.platform === "linux") {
    // Killed orphan zombies await their host reaper; they cannot execute or hold ports.
    return readdirSync("/proc").some((entry) => {
      if (!/^\d+$/.test(entry)) return false;
      try {
        const stat = readFileSync(`/proc/${entry}/stat`, "utf8");
        const fields = stat.slice(stat.lastIndexOf(")") + 2).split(" ");
        return (
          Number(fields[2]) === child.pid && !["Z", "X"].includes(fields[0])
        );
      } catch (e: any) {
        if (["ENOENT", "ESRCH"].includes(e.code)) return false;
        throw e;
      }
    });
  }
  try {
    process.kill(-child.pid, 0);
    return true;
  } catch (e: any) {
    if (e.code === "ESRCH") return false;
    throw e;
  }
}
export function spawnOwned(spec: CommandSpec, c: ProcessContext, gate: GateID) {
  if (c.interrupted || Date.now() >= c.deadline)
    throw new Error(c.interrupted ? "SIGNAL" : "TIMEOUT");
  const path = join(c.scratch, "command-" + randomBytes(8).toString("hex")),
    errPath = path + ".stderr";
  const out = openSync(path, "wx", 0o600),
    err = openSync(errPath, "wx", 0o600);
  const child = spawn(spec.executable, spec.argv, {
    cwd: spec.cwd,
    env: spec.env,
    detached: true,
    stdio: ["ignore", "pipe", "pipe"],
  });
  c.children.add(child);
  let bytes = 0,
    ended = false,
    failed = false;
  const terminate = () => {
    if (!ended || groupAlive(child)) {
      failed = true;
      signalGroup(child, "SIGTERM");
      const force = setTimeout(() => signalGroup(child, "SIGKILL"), 1000);
      force.unref();
    }
  };
  const timer = setTimeout(
    terminate,
    Math.max(1, Math.min(c.deadline - Date.now(), spec.timeout ?? 1200000)),
  );
  const sink = (fd: number) => (data: Buffer) => {
    bytes += data.length;
    if (bytes > 32 * 1024 * 1024) {
      terminate();
      return;
    }
    writeSync(fd, data);
  };
  child.stdout!.on("data", sink(out));
  child.stderr!.on("data", sink(err));
  const done = new Promise<CommandResult>((resolve) => {
    child.once("error", () => {
      failed = true;
    });
    child.once("close", async (code) => {
      ended = true;
      clearTimeout(timer);
      if (!groupAlive(child)) c.children.delete(child);
      fsyncSync(out);
      fsyncSync(err);
      closeSync(out);
      closeSync(err);
      const r = {
        exitCode: code,
        complete: !failed,
        resultPath: path,
        bindingPath: path + ".binding.json",
        categories: failed ? ["PROCESS"] : [],
      };
      try {
        await writeFile(
          r.bindingPath,
          JSON.stringify({
            schemaVersion: 1,
            runId: c.runId,
            gate,
            source: c.source,
            command: digest(JSON.stringify([spec.executable, spec.argv])),
            digest: digest(await readFile(path)),
            exitCode: code,
            complete: r.complete,
          }) + "\n",
          { flag: "wx", mode: 0o600 },
        );
        resolve(r);
      } catch {
        resolve({ ...r, complete: false, categories: ["PUBLICATION"] });
      }
    });
  });
  return { child, done, terminate };
}
export async function runCommand(
  spec: CommandSpec,
  c: ProcessContext,
  gate: GateID,
) {
  return spawnOwned(spec, c, gate).done;
}
export async function stopOwnedProcesses(c: ProcessContext) {
  const children = [...c.children];
  for (const child of children) signalGroup(child, "SIGTERM");
  const started = Date.now();
  let forced = false;
  while (c.children.size) {
    for (const child of c.children) {
      if (!groupAlive(child)) c.children.delete(child);
      else if (!forced && Date.now() - started >= 1000)
        signalGroup(child, "SIGKILL");
    }
    if (Date.now() - started >= 1000) forced = true;
    if (!c.children.size) break;
    if (Date.now() - started >= 10000) throw new Error("CLEANUP");
    await new Promise((resolve) => setTimeout(resolve, 25));
  }
}
