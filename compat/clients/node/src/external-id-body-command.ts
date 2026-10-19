import { open, mkdir, rename, unlink } from "node:fs/promises";
import { resolve, dirname, basename, join } from "node:path";
import { createHash, randomBytes } from "node:crypto";
import { privateDirectory } from "./metadata-files.ts";
import { parseExternalIdConfig } from "./external-id-contract.ts";
import {
  durableJSON,
  privateJSON,
  transport,
  recordPath,
  verifyIdentity,
  boundedTransport,
  type Transport,
} from "./external-id-recovery.ts";
import { readBodyRun, recoverBodyRun } from "./external-id-body-recovery.ts";
import { validateBodyCleanupEvidence } from "./external-id-body-assess.ts";
import { runBodyProbe } from "./external-id-body-probe.ts";
import type { BodyReport, BodyCleanup } from "./external-id-body-contract.ts";
async function syncDirectory(dir: string) {
  const f = await open(dir, "r");
  try {
    await f.sync();
  } finally {
    await f.close();
  }
}
async function publish(path: string, value: unknown) {
  const pending = path + ".pending-" + randomBytes(6).toString("hex");
  await durableJSON(pending, value);
  await rename(pending, path);
  await syncDirectory(dirname(path));
}
export async function bodyCommand(
  mode: "probe" | "recover",
  configPath: string,
  out: string,
  env: NodeJS.ProcessEnv,
  first?: string,
  injected?: Transport,
): Promise<BodyReport | BodyCleanup> {
  const configFile = resolve(configPath),
    target = resolve(out);
  await privateDirectory(dirname(configFile));
  await privateDirectory(dirname(target));
  if (!target.endsWith(".json"))
    throw new Error("Private output must end in .json");
  const c = parseExternalIdConfig(await privateJSON(configFile, 65536));
  const authorized = transport(c, env),
    t = boundedTransport(injected ?? authorized, Date.now() + 300000);
  const root = await privateDirectory(env.COMPAT_RECOVERY_DIR ?? "");
  const identity = createHash("sha256")
    .update(JSON.stringify([c.origin, c.administrator]))
    .digest("hex");
  const lock = join(root, `external-id-${identity}.active.json`),
    last = join(root, `external-id-body-${identity}.last-clean.json`);
  const dir =
    mode === "probe"
      ? join(dirname(target), basename(target, ".json") + ".artifacts")
      : resolve(first ?? "");
  if (mode === "recover") {
    if (!first) throw new Error("Recovery run directory required");
    await readBodyRun(c, dir);
  }
  const output = await open(target, "wx", 0o600);
  let preparationLock = false;
  let fixtureDirectoryCreated = false;
  try {
    if (mode === "probe") {
      // Hold the shared lock while checking the previous run, so another suite
      // cannot replace the pointer or start collection during this check.
      await durableJSON(lock, { dir });
      preparationLock = true;
      await syncDirectory(root);
      let previous;
      try {
        previous = await privateJSON(last);
      } catch (e: any) {
        if (e.code !== "ENOENT") throw e;
      }
      if (previous !== undefined) {
        if (!previous || Object.keys(previous).sort().join() !== "cleanup,dir")
          throw new Error("INVALID_LAST_CLEAN_POINTER");
        await validateBodyCleanupEvidence(c, previous.dir, previous.cleanup);
        await verifyIdentity(c, t);
        for (const row of previous.cleanup.records) {
          const response = await t.request(recordPath(row.id), {});
          if (response.status !== 404) throw new Error("RECORD_REAPPEARED");
        }
        await verifyIdentity(c, t);
      }
      await mkdir(dir, { mode: 0o700 });
      fixtureDirectoryCreated = true;
    } else {
      try {
        const held = await privateJSON(lock);
        if (Object.keys(held).join() !== "dir" || held.dir !== dir)
          throw new Error("EXTERNAL_ID_RUN_PENDING");
      } catch (e: any) {
        if (e.code !== "ENOENT") throw e;
        await durableJSON(lock, { dir });
        await syncDirectory(root);
      }
    }
    const result =
      mode === "probe"
        ? await runBodyProbe(c, env, dir, injected)
        : await recoverBodyRun(c, env, dir, injected);
    await publish(target, result);
    const cleanup = "cleanup" in result ? result.cleanup : result;
    if (cleanup.outcome === "clean") {
      await validateBodyCleanupEvidence(c, dir, cleanup);
      await publish(last, { dir, cleanup });
      const held = await privateJSON(lock);
      if (held.dir !== dir) throw new Error("EXTERNAL_ID_LOCK_CHANGED");
      await unlink(lock);
      await syncDirectory(root);
    }
    return result;
  } catch (e) {
    if (preparationLock && !fixtureDirectoryCreated) {
      await unlink(lock);
      await syncDirectory(root);
    }
    throw Object.assign(
      e instanceof Error ? e : new Error("Body command failed"),
      { exitCode: 1 },
    );
  } finally {
    await output.close();
  }
}
