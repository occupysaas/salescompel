import { open, mkdir, rename, unlink } from "node:fs/promises";
import { resolve, dirname, basename, join } from "node:path";
import { createHash, randomBytes } from "node:crypto";
import { privateDirectory } from "./metadata-files.ts";
import { parseExternalIdConfig } from "./external-id-contract.ts";
import {
  durableJSON,
  privateJSON,
  transport,
  recoverExternalIdRun,
  readRun,
  type Transport,
} from "./external-id-recovery.ts";
import { runExternalIdProbe } from "./external-id-probe.ts";
export async function externalIdCommand(
  mode: "probe" | "recover",
  configPath: string,
  out: string,
  env: NodeJS.ProcessEnv,
  first?: string,
  injected?: Transport,
) {
  const configFile = resolve(configPath),
    target = resolve(out);
  await privateDirectory(dirname(configFile));
  await privateDirectory(dirname(target));
  if (!target.endsWith(".json"))
    throw new Error("Private output must end in .json");
  const c = parseExternalIdConfig(await privateJSON(configFile, 65536));
  transport(c, env);
  const recoveryRoot = await privateDirectory(env.COMPAT_RECOVERY_DIR ?? "");
  const lock = join(
    recoveryRoot,
    "external-id-" +
      createHash("sha256")
        .update(JSON.stringify([c.origin, c.administrator]))
        .digest("hex") +
      ".active.json",
  );
  const dir =
    mode === "probe"
      ? join(dirname(target), basename(target, ".json") + ".artifacts")
      : resolve(first ?? "");
  if (mode === "recover") {
    if (!first) throw new Error("Recovery run directory required");
    await readRun(c, dir);
  }
  const output = await open(target, "wx", 0o600);
  let ownsLock = false;
  try {
    if (mode === "probe") {
      await mkdir(dir, { mode: 0o700 });
      await durableJSON(lock, { dir });
      ownsLock = true;
    }
    const result =
      mode === "probe"
        ? await runExternalIdProbe(c, env, dir, injected)
        : await recoverExternalIdRun(c, env, dir, injected);
    const pending = target + ".pending-" + randomBytes(6).toString("hex");
    try {
      await durableJSON(pending, result);
      await rename(pending, target);
    } catch {
      throw Object.assign(new Error("External-ID persistence failure"), {
        exitCode: 1,
      });
    }
    const parent = await open(dirname(target), "r");
    try {
      await parent.sync();
    } finally {
      await parent.close();
    }
    const clean =
      "cleanup" in result
        ? result.cleanup === "clean"
        : result.outcome === "clean";
    if (clean) {
      try {
        const held = await privateJSON(lock, 65536);
        if (held.dir === dir) await unlink(lock);
      } catch (e: any) {
        if (e.code !== "ENOENT") throw e;
      }
    }
    return result;
  } catch (e) {
    if (ownsLock) {
      try {
        const run = await readRun(c, dir);
        if (run.events.every((e) => e.kind === "intent")) await unlink(lock);
      } catch {
        /* Keep lock if durable state is unavailable. */
      }
    }
    throw e;
  } finally {
    await output.close();
  }
}
