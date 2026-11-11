import fs from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { makeInventory } from "../target-inventory-fixtures.ts";

const exec = promisify(execFile);

export const withDir = async (f: (dir: string) => Promise<void>) => {
  const d = await fs.mkdtemp(join(tmpdir(), "sc-inventory-"));
  try {
    await fs.chmod(d, 0o700);
    await f(d);
  } finally {
    await fs.rm(d, { force: true, recursive: true });
  }
};

export const writeInput = async (
  dir: string,
  value: unknown = makeInventory(),
) => {
  const p = join(dir, "input.yaml");
  await fs.writeFile(p, JSON.stringify(value), { mode: 0o600 });
  return p;
};

export async function cli(args: string[]) {
  const env = Object.fromEntries(
    Object.entries(process.env).filter(
      ([k]) => !/^COMPAT_|TOKEN|PASSWORD|SECRET/.test(k),
    ),
  );
  try {
    const r = await exec(
      process.execPath,
      ["--import", "tsx", "src/cli.ts", "validate-target-inventory", ...args],
      { env, timeout: 10000 },
    );
    return { ...r, code: 0 };
  } catch (e: unknown) {
    const err = e as { stdout: string; stderr: string; code: number };
    return {
      stdout: err.stdout as string,
      stderr: err.stderr as string,
      code: err.code,
    };
  }
}
