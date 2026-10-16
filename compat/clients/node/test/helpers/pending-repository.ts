import { cp, mkdir, mkdtemp, rm, symlink, writeFile } from "node:fs/promises";
import { tmpdir } from "node:os";
import { dirname, resolve } from "node:path";
import { ROOT, SCENARIOS } from "../../src/validate.ts";

// Use an isolated pending record, never change or depend on live authorization.
export async function pendingRepository(): Promise<{
  nodeDir: string;
  dispose: () => Promise<void>;
}> {
  const root = await mkdtemp(resolve(tmpdir(), "compat-pending-"));
  try {
    for (const path of [
      "compat/clients/node/src",
      "compat/clients/node/package.json",
      "compat/clients/python/requirements.lock",
      "compat/schemas/reference-readiness.schema.json",
      "docs/compat/profiles/pilot.yaml",
    ]) {
      await mkdir(dirname(resolve(root, path)), { recursive: true });
      await cp(resolve(ROOT, path), resolve(root, path), { recursive: true });
    }
    const nodeDir = resolve(root, "compat/clients/node");
    await symlink(
      resolve(ROOT, "compat/clients/node/node_modules"),
      resolve(nodeDir, "node_modules"),
      "dir",
    );
    await writeFile(
      resolve(root, "docs/compat/implementation-log.md"),
      "# Pending fixture\n",
    );
    await writeFile(
      resolve(root, "docs/compat/reference-readiness.yaml"),
      JSON.stringify({
        schemaVersion: 1,
        referenceAlias: null,
        profile: "pilot",
        profileHash: null,
        apiVersion: null,
        decision: {
          status: "pending",
          maintainer: null,
          recordedAt: null,
          logAnchor: null,
          observationAuthorized: false,
        },
        observer: null,
        implementer: null,
        scenarios: SCENARIOS,
        fixtures: {
          disposable: false,
          administrator: false,
          restricted: false,
          locale: false,
          deployed: false,
        },
      }),
    );
    return {
      nodeDir,
      dispose: () => rm(root, { recursive: true, force: true }),
    };
  } catch (error) {
    await rm(root, { recursive: true, force: true });
    throw error;
  }
}
