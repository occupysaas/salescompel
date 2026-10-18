import { privateDirectory } from "./metadata-files.ts";
export { privateDirectory } from "./metadata-files.ts";
import {
  mkdir,
  writeFile,
  readFile,
  readdir,
  lstat,
  copyFile,
  chmod,
  realpath,
  stat,
} from "node:fs/promises";
import { join, resolve, relative, isAbsolute } from "node:path";
import { createHash } from "node:crypto";
import { createRequire } from "node:module";
import { Connection } from "jsforce";
import { withCli } from "./process-clients.ts";
import type { Context } from "./scenarios.ts";
import { ROOT } from "./versions.ts";
import {
  parseDiscoveryConfig,
  type DiscoveryConfig,
  type Artifact,
} from "./metadata-inventory.ts";
import { validateDiscoveryEnv } from "./metadata-discovery.ts";
const require = createRequire(import.meta.url);
// Existing lock-pinned JSforce XML dependency; no dependency upgrade or CLI source access.
const { parseStringPromise } = require("xml2js");
const namespace = "http://soap.sforce.com/2006/04/metadata";
const expected = [
  {
    path: "objects/Account/fields/Compat_Text__c.field-meta.xml",
    type: "CustomField" as const,
    name: "Account.Compat_Text__c",
  },
  {
    path: "objects/Account/fields/Compat_Key__c.field-meta.xml",
    type: "CustomField" as const,
    name: "Account.Compat_Key__c",
  },
  {
    path: "permissionsets/Compat_Admin.permissionset-meta.xml",
    type: "PermissionSet" as const,
    name: "Compat_Admin",
  },
];
export interface RetrievedMember {
  type: "CustomField" | "PermissionSet";
  name: string;
  data: any;
}
export interface RetrievalResult {
  outcome: "complete" | "incomplete";
  members: RetrievedMember[];
  artifacts: Artifact[];
  missing: string[];
}
export async function scanRetrievedMetadata(
  root: string,
): Promise<RetrievalResult> {
  const found = new Map<string, Buffer>();
  let total = 0,
    entries = 0;
  async function walk(dir: string) {
    const info = await lstat(dir);
    if (info.isSymbolicLink() || !info.isDirectory())
      throw new Error("Unexpected symlink or directory");
    for (const item of await readdir(dir, { withFileTypes: true })) {
      if (++entries > 100) throw new Error("Retrieval entry bound");
      const path = join(dir, item.name),
        s = await lstat(path);
      if (s.isSymbolicLink()) throw new Error("Unexpected symlink");
      if (s.isDirectory()) {
        await walk(path);
        continue;
      }
      const name = relative(root, path).split("\\").join("/");
      const container = name === "objects/Account/Account.object-meta.xml";
      if (!s.isFile() || (!container && !expected.some((x) => x.path === name)))
        throw new Error("Unexpected retrieved member");
      total += s.size;
      if (s.size > 8 * 1024 * 1024 || total > 32 * 1024 * 1024)
        throw new Error("Retrieval size bound");
      const contents = await readFile(path);
      if (container) {
        const xml = contents.toString("utf8");
        if (/<!DOCTYPE|<!ENTITY/i.test(xml))
          throw new Error("Unsafe XML container");
        const parsed = await parseStringPromise(xml, {
          explicitArray: false,
          strict: true,
        });
        const body = parsed?.CustomObject;
        if (
          !body ||
          Object.keys(parsed).length !== 1 ||
          Object.keys(body).length !== 1 ||
          body.$?.xmlns !== namespace ||
          Object.keys(body.$).length !== 1
        )
          throw new Error("Unexpected metadata in container");
      } else found.set(name, contents);
    }
  }
  try {
    await walk(root);
  } catch (e: any) {
    if (e.code !== "ENOENT") throw e;
  }
  const result: RetrievalResult = {
    outcome: found.size === 3 ? "complete" : "incomplete",
    members: [],
    artifacts: [],
    missing: [],
  };
  for (const item of expected) {
    const data = found.get(item.path);
    if (!data) {
      result.missing.push(item.type + ":" + item.name);
      continue;
    }
    const xml = data.toString("utf8");
    if (/<!DOCTYPE|<!ENTITY/i.test(xml)) throw new Error("Unsafe XML");
    const parsed = await parseStringPromise(xml, {
      explicitArray: false,
      strict: true,
    });
    const body = parsed?.[item.type];
    if (
      !body ||
      body.$?.xmlns !== namespace ||
      Object.keys(parsed).length !== 1
    )
      throw new Error("Invalid XML component");
    if (
      body.fullName !== undefined &&
      body.fullName !== item.name &&
      body.fullName !== item.name.split(".").at(-1)
    )
      throw new Error("XML member differs");
    const booleanKeys = new Set([
      "required",
      "externalId",
      "unique",
      "caseSensitive",
      "readable",
      "editable",
      "allowRead",
      "allowCreate",
      "allowEdit",
      "allowDelete",
      "viewAllRecords",
      "modifyAllRecords",
    ]);
    function values(x: any): any {
      if (Array.isArray(x)) return x.map(values);
      if (!x || typeof x !== "object") return x;
      return Object.fromEntries(
        Object.entries(x).map(([k, v]) => [
          k,
          booleanKeys.has(k)
            ? v === "true"
              ? true
              : v === "false"
                ? false
                : v
            : k === "length" && typeof v === "string" && /^\d+$/.test(v)
              ? Number(v)
              : values(v),
        ]),
      );
    }
    result.members.push({
      type: item.type,
      name: item.name,
      data: { ...values(body), fullName: item.name },
    });
    result.artifacts.push({
      path: item.path,
      component: item.type + ":" + item.name,
      bytes: data.length,
      sha256: createHash("sha256").update(data).digest("hex"),
    });
  }
  return result;
}
export async function retrieveInSession(
  run: (args: string[]) => Promise<any>,
  dir: string,
): Promise<RetrievalResult> {
  await writeFile(
    join(dir, "sfdx-project.json"),
    JSON.stringify({
      packageDirectories: [{ path: "force-app", default: true }],
      sourceApiVersion: "65.0",
    }),
    { flag: "wx", mode: 0o600 },
  );
  await mkdir(join(dir, "force-app"), { mode: 0o700 });
  await writeFile(
    join(dir, "package.xml"),
    `<?xml version="1.0" encoding="UTF-8"?><Package xmlns="${namespace}"><types><members>Account.Compat_Text__c</members><members>Account.Compat_Key__c</members><name>CustomField</name></types><types><members>Compat_Admin</members><name>PermissionSet</name></types><version>65.0</version></Package>`,
    { flag: "wx", mode: 0o600 },
  );
  const response = await run([
    "project",
    "retrieve",
    "start",
    "--manifest",
    join(dir, "package.xml"),
    "--target-org",
    "compat-fixture",
    "--api-version",
    "65.0",
    "--wait",
    "5",
  ]);
  const result = await scanRetrievedMetadata(
    join(dir, "force-app/main/default"),
  );
  if (response.success !== true || response.status !== "Succeeded")
    result.outcome = "incomplete";
  return result;
}
export async function retrieveSyntheticMetadata(
  config: DiscoveryConfig,
  env: NodeJS.ProcessEnv,
): Promise<RetrievalResult> {
  const c = parseDiscoveryConfig(config);
  validateDiscoveryEnv(c, env);
  const destination = await privateDirectory(env.DISCOVERY_ARTIFACT_DIR ?? "");
  const ctx = {
    env,
    conn: new Connection({
      instanceUrl: c.origin,
      accessToken: env.COMPAT_REFERENCE_TOKEN,
      version: "65.0",
      httpProxy: undefined,
    }),
  } as Context;
  return withCli(ctx, async (run, dir) => {
    const result = await retrieveInSession(run, dir);
    const retained = join(destination, "retrieved");
    await mkdir(retained, { mode: 0o700 });
    for (const artifact of result.artifacts) {
      const path = join(retained, artifact.path);
      await mkdir(resolve(path, ".."), { recursive: true, mode: 0o700 });
      await copyFile(
        join(dir, "force-app/main/default", artifact.path),
        path,
        1,
      );
      await chmod(path, 0o600);
      artifact.path = "retrieved/" + artifact.path;
    }
    return result;
  });
}
