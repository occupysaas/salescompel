import { readFileSync, existsSync } from "node:fs";
import { createRequire } from "node:module";
import { resolve } from "node:path";
import { fileURLToPath } from "node:url";
export const ROOT = fileURLToPath(new URL("../../../../", import.meta.url));
export const SF_BIN = resolve(
  ROOT,
  "compat/clients/node/node_modules/@salesforce/cli/bin/run.js",
);
const readJson = (path: string) => JSON.parse(readFileSync(path, "utf8"));
// Version pinned by requirements.lock; the probe asserts it at runtime.
export function pythonLockedVersion(): string {
  const lock = readFileSync(
    resolve(ROOT, "compat/clients/python/requirements.lock"),
    "utf8",
  );
  const match = /^simple-salesforce==(\d+\.\d+\.\d+)$/m.exec(lock);
  if (!match)
    throw new Error("simple-salesforce is not pinned in requirements.lock");
  return match[1];
}
export function installedVersions(): Record<string, string> {
  const cliRequire = createRequire(SF_BIN);
  const versions: Record<string, string> = {};
  for (const name of ["jsforce", "@salesforce/cli"])
    versions[name] = readJson(
      resolve(ROOT, `compat/clients/node/node_modules/${name}/package.json`),
    ).version;
  for (const name of [
    "@salesforce/plugin-data",
    "@salesforce/plugin-deploy-retrieve",
  ]) {
    const path = cliRequire.resolve
      .paths(name)
      ?.map((p) => resolve(p, name, "package.json"))
      .find((p) => existsSync(p));
    if (!path) throw new Error(`Missing CLI plugin ${name}`);
    versions[name] = readJson(path).version;
  }
  return versions;
}
export function connectorVersions(): Record<string, string> {
  return { ...installedVersions(), "simple-salesforce": pythonLockedVersion() };
}
