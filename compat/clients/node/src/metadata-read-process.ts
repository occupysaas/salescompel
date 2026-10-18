import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { ROOT } from "./versions.ts";
export interface ReadRequest {
  origin: string;
  token: string;
  kind: "describe" | "metadata";
  name: string;
  type?: "CustomField" | "PermissionSet" | "Profile";
}
export function validateReadRequest(r: ReadRequest) {
  if (
    !r ||
    !["describe", "metadata"].includes(r.kind) ||
    typeof r.name !== "string" ||
    !r.name ||
    r.name.length > 255 ||
    /[\x00-\x1f\x7f*?]/.test(r.name) ||
    typeof r.token !== "string" ||
    !r.token
  )
    throw new Error("Invalid read operation");
  const u = new URL(r.origin);
  if (
    u.origin !== r.origin ||
    u.username ||
    u.password ||
    !(
      u.protocol === "https:" ||
      (u.protocol === "http:" &&
        ["127.0.0.1", "localhost"].includes(u.hostname))
    )
  )
    throw new Error("Invalid read origin");
  if (
    (r.kind === "describe" && !["Account", "Contact"].includes(r.name)) ||
    (r.kind === "metadata" &&
      !["CustomField", "PermissionSet", "Profile"].includes(r.type ?? ""))
  )
    throw new Error("Invalid read operation");
}
export async function readWithClient(
  r: ReadRequest,
  limits = { maxBytes: 8 * 1024 * 1024, timeoutMs: 60000 },
): Promise<any> {
  validateReadRequest(r);
  if (
    !Number.isSafeInteger(limits.maxBytes) ||
    limits.maxBytes < 1 ||
    !Number.isSafeInteger(limits.timeoutMs) ||
    limits.timeoutMs < 1
  )
    throw new Error("Invalid read limits");
  return new Promise((accept, reject) => {
    const child = spawn(
      process.execPath,
      [
        "--import",
        "tsx",
        fileURLToPath(new URL("./metadata-read-worker.ts", import.meta.url)),
      ],
      {
        cwd: resolve(ROOT, "compat/clients/node"),
        stdio: ["pipe", "pipe", "ignore"],
      },
    );
    let chunks: Buffer[] = [],
      size = 0,
      reason: string | undefined;
    const stop = (code: string) => {
      reason ??= code;
      child.kill("SIGKILL");
    };
    const timer = setTimeout(() => stop("READ_TIMEOUT"), limits.timeoutMs);
    child.stdout.on("data", (b: Buffer) => {
      size += b.length;
      if (size > limits.maxBytes) stop("READ_BOUND");
      else chunks.push(b);
    });
    child.on("error", () => {
      clearTimeout(timer);
      reject(new Error("READ_PROCESS_FAILED"));
    });
    child.on("close", (code) => {
      clearTimeout(timer);
      if (reason || code !== 0) {
        reject(new Error(reason ?? "READ_UNAVAILABLE"));
        return;
      }
      try {
        accept(JSON.parse(Buffer.concat(chunks).toString()));
      } catch {
        reject(new Error("READ_INVALID_RESPONSE"));
      }
    });
    child.stdin.on("error", () => {});
    child.stdin.end(JSON.stringify(r));
  });
}
