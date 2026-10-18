import { spawn } from "node:child_process";
import { fileURLToPath } from "node:url";
import { resolve } from "node:path";
import { ROOT } from "./versions.ts";
export function nativeUpsert(
  origin: string,
  token: string,
  key: string,
  marker: string,
  timeoutMs = 60000,
): Promise<unknown> {
  return new Promise((accept, reject) => {
    const child = spawn(
      process.execPath,
      [
        "--import",
        "tsx",
        fileURLToPath(
          new URL("./external-id-native-worker.ts", import.meta.url),
        ),
      ],
      {
        cwd: resolve(ROOT, "compat/clients/node"),
        stdio: ["pipe", "pipe", "ignore"],
      },
    );
    let size = 0;
    const chunks: Buffer[] = [];
    let reason = "NATIVE_UNAVAILABLE";
    const timer = setTimeout(() => {
      reason = "NATIVE_TIMEOUT";
      child.kill("SIGKILL");
    }, timeoutMs);
    child.stdout.on("data", (b: Buffer) => {
      size += b.length;
      if (size > 8 * 1024 * 1024) {
        reason = "NATIVE_BOUND";
        child.kill("SIGKILL");
      } else chunks.push(b);
    });
    child.on("error", () => {
      clearTimeout(timer);
      reject(new Error(reason));
    });
    child.on("close", (code) => {
      clearTimeout(timer);
      if (code !== 0) {
        reject(new Error(reason));
        return;
      }
      try {
        accept(JSON.parse(Buffer.concat(chunks).toString()));
      } catch {
        reject(new Error("NATIVE_INVALID_RESPONSE"));
      }
    });
    child.stdin.on("error", () => {});
    child.stdin.end(JSON.stringify({ origin, token, key, marker }));
  });
}
