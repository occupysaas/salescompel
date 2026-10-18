import { realpath, stat } from "node:fs/promises";
import { isAbsolute, relative, resolve } from "node:path";
import { ROOT } from "./versions.ts";
export async function privateDirectory(dir: string) {
  if (!isAbsolute(dir)) throw new Error("Private directory must be absolute");
  const actual = await realpath(dir),
    root = await realpath(ROOT),
    rel = relative(root, actual),
    info = await stat(actual);
  if (
    (!rel.startsWith(".." + (process.platform === "win32" ? "\\" : "/")) &&
      !isAbsolute(rel)) ||
    actual !== resolve(dir) ||
    !info.isDirectory() ||
    (info.mode & 0o077) !== 0 ||
    (process.getuid && info.uid !== process.getuid())
  )
    throw new Error("Directory must be private, external and not symlinked");
  return actual;
}
