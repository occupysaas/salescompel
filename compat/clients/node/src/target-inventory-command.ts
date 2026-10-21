import fs from "node:fs/promises";
import { constants } from "node:fs";
import {
  dirname,
  resolve,
  parse,
  join,
  relative,
  isAbsolute,
  extname,
} from "node:path";
import { parseDocument } from "./core.ts";
import { privateDirectory } from "./metadata-files.ts";
import {
  assessTargetInventory,
  MAX_INVENTORY_BYTES,
  safeInventoryReference,
} from "./target-inventory.ts";
import type {
  InventoryReport,
  InventoryIssue,
  RepositoryReference,
} from "./target-inventory-model.ts";

function failure(
  code: "input-io" | "output-io" | "input-too-large",
): InventoryReport {
  return {
    schemaVersion: 1,
    inputDigest: "",
    validation: "invalid",
    readiness: "incomplete",
    sections: [],
    issues: [{ code }],
    gaps: [],
  };
}
async function noSymlinks(path: string): Promise<void> {
  const absolute = resolve(path),
    parts = relative(parse(absolute).root, absolute).split("/");
  let current = parse(absolute).root;
  for (const part of parts) {
    current = join(current, part);
    const info = await fs.lstat(current);
    if (info.isSymbolicLink()) throw new Error("unsafe-path");
  }
}
export async function readTargetInventoryBytes(path: string): Promise<Buffer> {
  await noSymlinks(path);
  const info = await fs.lstat(path);
  if (!info.isFile() || !(info.mode & 0o444)) throw new Error("input-io");
  const handle = await fs.open(
    path,
    constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK,
  );
  try {
    const actual = await handle.stat();
    if (!actual.isFile() || actual.dev !== info.dev || actual.ino !== info.ino)
      throw new Error("input-io");
    if (actual.size > MAX_INVENTORY_BYTES) throw new Error("input-too-large");
    const buffer = Buffer.alloc(MAX_INVENTORY_BYTES + 1);
    let offset = 0;
    while (offset < buffer.length) {
      const { bytesRead } = await handle.read(
        buffer,
        offset,
        buffer.length - offset,
        null,
      );
      if (!bytesRead) break;
      offset += bytesRead;
    }
    if (offset > MAX_INVENTORY_BYTES) throw new Error("input-too-large");
    return buffer.subarray(0, offset);
  } finally {
    await handle.close();
  }
}
export async function validateTargetInventoryFile(
  inputPath: string,
  outPath?: string,
): Promise<{ report: InventoryReport; exitCode: 0 | 1 | 2 }> {
  let report: InventoryReport;
  try {
    const path = resolve(inputPath),
      bytes = await readTargetInventoryBytes(path);
    report = assessTargetInventory(bytes);
    if (report.classification === "real") {
      await privateDirectory(dirname(path));
      const info = await fs.lstat(path);
      if (
        (info.mode & 0o777) !== 0o600 ||
        (process.getuid && info.uid !== process.getuid())
      )
        throw new Error("input-io");
    }
  } catch (e) {
    const oversized = e instanceof Error && e.message === "input-too-large";
    report = failure(oversized ? "input-too-large" : "input-io");
    if (!oversized) return { report, exitCode: 2 };
  }
  if (outPath !== undefined) {
    const path = resolve(outPath);
    let handle: Awaited<ReturnType<typeof fs.open>> | undefined;
    let owned: { dev: number; ino: number } | undefined;
    try {
      await noSymlinks(dirname(path));
      await privateDirectory(dirname(path));
      handle = await fs.open(
        path,
        constants.O_WRONLY |
          constants.O_CREAT |
          constants.O_EXCL |
          constants.O_NOFOLLOW,
        0o600,
      );
      owned = await handle.stat();
      await handle.writeFile(JSON.stringify(report) + "\n");
      await handle.sync();
      await handle.close();
      handle = undefined;
    } catch {
      if (handle) await handle.close().catch(() => {});
      if (owned) {
        try {
          const current = await fs.lstat(path);
          if (current.dev === owned.dev && current.ino === owned.ino)
            await fs.unlink(path);
        } catch {
          /* retain failure */
        }
      }
      return { report: failure("output-io"), exitCode: 2 };
    }
  }
  return { report, exitCode: report.validation === "valid" ? 0 : 1 };
}
export async function checkInventoryRepositoryRefs(
  refs: RepositoryReference[],
  root: string,
): Promise<InventoryIssue[]> {
  let failed = false;
  for (const ref of refs) {
    if (!safeInventoryReference(ref)) {
      failed = true;
      continue;
    }
    try {
      const path = resolve(root, ref.path),
        base = await fs.realpath(root),
        actual = await fs.realpath(path),
        rel = relative(base, actual);
      if (rel === ".." || rel.startsWith("../") || isAbsolute(rel))
        throw new Error("unsafe-path");
      const bytes = await readTargetInventoryBytes(path);
      if (ref.id !== undefined) {
        const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes),
          ext = extname(path);
        let ids: string[] = [];
        if (ext === ".yaml" || ext === ".yml") {
          const value = parseDocument(text);
          if (value && typeof value === "object") {
            if (typeof value.id === "string") ids.push(value.id);
            if (Array.isArray(value.contracts))
              ids.push(
                ...value.contracts
                  .map((c: unknown) =>
                    c && typeof c === "object" && "id" in c ? c.id : undefined,
                  )
                  .filter(
                    (id: unknown): id is string => typeof id === "string",
                  ),
              );
          }
        } else if (ext === ".md") {
          ids = [...text.matchAll(/^#{1,6}\s+(.+?)\s*#*\s*$/gm)].map((m) =>
            m[1].trim().toLowerCase().replace(/\s+/g, "-"),
          );
        }
        if (!ids.includes(ref.id)) throw new Error("missing-id");
      }
    } catch {
      failed = true;
    }
  }
  return failed ? [{ code: "repository-reference-invalid" }] : [];
}
