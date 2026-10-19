import { createHash } from "node:crypto";
import { constants } from "node:fs";
import { open } from "node:fs/promises";
import { dirname, resolve } from "node:path";
import { isDeepStrictEqual } from "node:util";
import { privateDirectory } from "./metadata-files.ts";
import { ROOT, connectorVersions } from "./versions.ts";
import { profile } from "./validate.ts";
import {
  FOUNDATION_ASSUMPTION_ID,
  FOUNDATION_POLICY_VERSION,
  foundationCoverage,
} from "./foundation-policy.ts";

export interface FoundationBinding {
  profile: string;
  profileHash: string;
  apiVersion: string;
  connectors: Record<string, string>;
  probeDigest: string;
  manifestDigest: string;
}

export interface FoundationSidecar extends FoundationBinding {
  schemaVersion: 1;
  policyVersion: 1;
  assumptionId: typeof FOUNDATION_ASSUMPTION_ID;
  policy: "documented-two-day-assumption";
  source: string;
  captureSha256: string;
  startedAt: string;
  finishedAt: string;
  publishedAt: string;
}

const source =
  "https://developer.salesforce.com/docs/platform/salesforce-app-limits-cheatsheet/guide/salesforce-app-limits-platform-apicursors.html";
const sha256 = (bytes: string | Buffer) =>
  createHash("sha256").update(bytes).digest("hex");

export async function currentFoundationBinding(): Promise<FoundationBinding> {
  const p = await profile("rest-query-foundation");
  const hash = createHash("sha256");
  for (const name of [
    "foundation.ts",
    "foundation-policy.ts",
    "capture.ts",
    "scenarios.ts",
    "wire.ts",
  ]) {
    const handle = await open(
      resolve(ROOT, "compat/clients/node/src", name),
      "r",
    );
    try {
      hash
        .update(name)
        .update("\0")
        .update(await handle.readFile())
        .update("\0");
    } finally {
      await handle.close();
    }
  }
  return {
    profile: p.data.id,
    profileHash: p.hash,
    apiVersion: p.data.apiVersion,
    connectors: connectorVersions(),
    probeDigest: hash.digest("hex"),
    manifestDigest: sha256(JSON.stringify(foundationCoverage())),
  };
}

async function privateFileBytes(path: string): Promise<Buffer> {
  if (resolve(path) !== path)
    throw new Error("Private file path must be absolute");
  await privateDirectory(dirname(path));
  const handle = await open(path, constants.O_RDONLY | constants.O_NOFOLLOW);
  try {
    const info = await handle.stat();
    if (
      !info.isFile() ||
      (info.mode & 0o077) !== 0 ||
      (process.getuid && info.uid !== process.getuid()) ||
      info.size > 64 * 1024 * 1024
    )
      throw new Error("Private file must be owner-only regular file");
    return await handle.readFile();
  } finally {
    await handle.close();
  }
}

export async function readPrivateFoundationFile(path: string): Promise<string> {
  return (await privateFileBytes(path)).toString("utf8");
}

export async function writeFoundationSidecar(
  capturePath: string,
  report: Pick<
    FoundationSidecar,
    "profile" | "profileHash" | "connectors" | "startedAt" | "finishedAt"
  > & { apiVersion: string | null },
  binding: FoundationBinding,
): Promise<string> {
  if (!capturePath.endsWith(".json")) throw new Error("Capture must be JSON");
  const bytes = await privateFileBytes(capturePath);
  const actual = JSON.parse(bytes.toString("utf8"));
  for (const key of [
    "profile",
    "profileHash",
    "apiVersion",
    "connectors",
    "startedAt",
    "finishedAt",
  ] as const)
    if (!isDeepStrictEqual(actual[key], report[key]))
      throw new Error("Capture bytes and report differ");
  for (const key of [
    "profile",
    "profileHash",
    "apiVersion",
    "connectors",
  ] as const)
    if (!isDeepStrictEqual(report[key], binding[key]))
      throw new Error("Capture binding differs from current foundation");
  const sidecar: FoundationSidecar = {
    schemaVersion: 1,
    ...binding,
    policyVersion: FOUNDATION_POLICY_VERSION,
    policy: "documented-two-day-assumption",
    assumptionId: FOUNDATION_ASSUMPTION_ID,
    source,
    captureSha256: sha256(bytes),
    startedAt: report.startedAt,
    finishedAt: report.finishedAt,
    publishedAt: new Date().toISOString(),
  };
  const target = capturePath.replace(/\.json$/, ".policy.json");
  const output = await open(target, "wx", 0o600);
  try {
    await output.writeFile(JSON.stringify(sidecar, null, 2) + "\n");
    await output.sync();
  } finally {
    await output.close();
  }
  return target;
}

export async function readFoundationSidecar(
  capturePath: string,
  sidecarPath: string,
): Promise<FoundationSidecar> {
  const [bytes, sidecarBytes] = await Promise.all([
    privateFileBytes(capturePath),
    privateFileBytes(sidecarPath),
  ]);
  const s: FoundationSidecar = JSON.parse(sidecarBytes.toString("utf8"));
  if (
    s.schemaVersion !== 1 ||
    s.policyVersion !== FOUNDATION_POLICY_VERSION ||
    s.policy !== "documented-two-day-assumption" ||
    s.assumptionId !== FOUNDATION_ASSUMPTION_ID ||
    s.source !== source ||
    s.captureSha256 !== sha256(bytes)
  )
    throw new Error("Foundation sidecar binding invalid");
  return s;
}
