import fs, { type FileHandle } from "node:fs/promises";
import { constants, type Stats } from "node:fs";
import {
  basename,
  dirname,
  resolve,
  parse,
  join,
  relative,
  isAbsolute,
  extname,
} from "node:path";
import { privateDirectory } from "./metadata-files.ts";
import { ROOT } from "./versions.ts";
import {
  assessTargetInventory,
  MAX_INVENTORY_BYTES,
  safeInventoryReference,
  parseInventoryDocument,
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
const descriptorPath = (handle: FileHandle) => `/proc/self/fd/${handle.fd}`;
const anchored = (parent: FileHandle, name: string) =>
  `${descriptorPath(parent)}/${name}`;
/** Node has no openat API: pin every Linux directory component through its fd. */
async function openDirectory(path: string): Promise<FileHandle> {
  if (process.platform !== "linux") throw new Error("unsupported-platform");
  const flags =
    constants.O_RDONLY | constants.O_DIRECTORY | constants.O_NOFOLLOW;
  let parent = await fs.open("/", flags);
  try {
    for (const part of resolve(path).split("/").filter(Boolean)) {
      const child = await fs.open(anchored(parent, part), flags);
      await parent.close();
      parent = child;
    }
    return parent;
  } catch (e) {
    await parent.close().catch(() => {});
    throw e;
  }
}
interface ReadResult {
  bytes: Buffer;
  fileInfo: Stats;
  parentInfo: Stats;
  parentPath: string;
}
async function readPinned(path: string): Promise<ReadResult> {
  await noSymlinks(path);
  const parent = await openDirectory(dirname(path));
  let handle: FileHandle | undefined;
  try {
    const parentInfo = await parent.stat(),
      parentPath = await fs.realpath(descriptorPath(parent));
    handle = await fs.open(
      anchored(parent, basename(path)),
      constants.O_RDONLY | constants.O_NOFOLLOW | constants.O_NONBLOCK,
    );
    const info = await handle.stat();
    if (!info.isFile() || !(info.mode & 0o444)) throw new Error("input-io");
    if (info.size > MAX_INVENTORY_BYTES) throw new Error("input-too-large");
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
    return {
      bytes: buffer.subarray(0, offset),
      fileInfo: info,
      parentInfo,
      parentPath,
    };
  } finally {
    if (handle) await handle.close();
    await parent.close();
  }
}
export async function readTargetInventoryBytes(path: string): Promise<Buffer> {
  return (await readPinned(path)).bytes;
}
function privateOwner(info: Stats): boolean {
  return (
    (info.mode & 0o077) === 0 &&
    (!process.getuid || info.uid === process.getuid())
  );
}
async function externalPath(path: string): Promise<boolean> {
  const rel = relative(await fs.realpath(ROOT), path);
  return rel === ".." || rel.startsWith("../") || isAbsolute(rel);
}
export async function validateTargetInventoryFile(
  inputPath: string,
  outPath?: string,
): Promise<{ report: InventoryReport; exitCode: 0 | 1 | 2 }> {
  let report: InventoryReport;
  try {
    const path = resolve(inputPath),
      read = await readPinned(path);
    report = assessTargetInventory(read.bytes);
    if (report.classification === "real") {
      // Check the actual objects read, not fresh path lookups which may name replacements.
      if (
        (read.fileInfo.mode & 0o777) !== 0o600 ||
        !privateOwner(read.fileInfo) ||
        !privateOwner(read.parentInfo) ||
        read.parentPath !== dirname(path) ||
        !(await externalPath(read.parentPath))
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
    let parent: FileHandle | undefined, handle: FileHandle | undefined;
    let owned: { dev: number; ino: number } | undefined;
    try {
      await noSymlinks(dirname(path));
      await privateDirectory(dirname(path));
      parent = await openDirectory(dirname(path));
      const info = await parent.stat(),
        actual = await fs.realpath(descriptorPath(parent));
      if (
        !privateOwner(info) ||
        actual !== dirname(path) ||
        !(await externalPath(actual))
      )
        throw new Error("output-io");
      handle = await fs.open(
        anchored(parent, basename(path)),
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
      if (owned && parent) {
        try {
          const current = await fs.lstat(anchored(parent, basename(path)));
          if (current.dev === owned.dev && current.ino === owned.ino)
            await fs.unlink(anchored(parent, basename(path)));
        } catch {
          /* retain failure */
        }
      }
      return { report: failure("output-io"), exitCode: 2 };
    } finally {
      if (parent) await parent.close().catch(() => {});
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
          const value = parseInventoryDocument(bytes) as {
            id?: unknown;
            contracts?: unknown[];
          } | null;
          if (value && typeof value === "object") {
            if (typeof value.id === "string") ids.push(value.id);
            if (Array.isArray(value.contracts))
              ids.push(
                ...value.contracts
                  .map((c) =>
                    c && typeof c === "object" && "id" in c ? c.id : undefined,
                  )
                  .filter((id): id is string => typeof id === "string"),
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
