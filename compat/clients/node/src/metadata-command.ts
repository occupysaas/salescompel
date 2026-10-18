import { open, mkdir, readFile, lstat, writeFile } from "node:fs/promises";
import { dirname, resolve, basename, join } from "node:path";
import {
  parseDiscoveryConfig,
  newInventory,
  finishInventory,
  addGap,
} from "./metadata-inventory.ts";
import {
  collectMetadata,
  discoveryReader,
  verifyIdentityPair,
  sameIdentities,
  validateDiscoveryEnv,
  projectMetadata,
} from "./metadata-discovery.ts";
import {
  retrieveSyntheticMetadata,
  privateDirectory,
} from "./metadata-retrieve.ts";
export async function discoverMetadataCommand(
  configPath: string,
  out: string,
  env: NodeJS.ProcessEnv,
) {
  const configFile = resolve(configPath),
    target = resolve(out);
  await privateDirectory(dirname(target));
  await privateDirectory(dirname(configFile));
  const info = await lstat(configFile);
  if (
    !info.isFile() ||
    info.isSymbolicLink() ||
    (info.mode & 0o077) !== 0 ||
    info.size > 65536
  )
    throw new Error("Configuration must be a bounded private file");
  if (!target.endsWith(".json"))
    throw new Error("Private output must end in .json");
  let parsed: unknown;
  try {
    parsed = JSON.parse(await readFile(configFile, "utf8"));
  } catch {
    throw new Error("Invalid private configuration JSON");
  }
  const config = parseDiscoveryConfig(parsed);
  validateDiscoveryEnv(config, env);
  const output = await open(target, "wx", 0o600);
  let inventory = newInventory(config);
  const sessionEnv = {
    ...env,
    DISCOVERY_ARTIFACT_DIR: join(
      dirname(target),
      basename(target, ".json") + ".artifacts",
    ),
  };
  try {
    await mkdir(sessionEnv.DISCOVERY_ARTIFACT_DIR, { mode: 0o700 });
    const reader = discoveryReader(config, sessionEnv);
    const initial = await verifyIdentityPair(reader);
    inventory = await collectMetadata(config, sessionEnv, reader);
    if (
      inventory.identityVerified.initial &&
      inventory.identityVerified.final
    ) {
      const source = "retrieve:administrator:synthetic";
      const op = {
        id: source,
        kind: "retrieve" as const,
        persona: "administrator" as const,
        component: "synthetic",
        state: "observed" as "observed" | "unavailable",
      };
      inventory.operations.push(op);
      try {
        const r = await retrieveSyntheticMetadata(config, sessionEnv);
        inventory.artifacts.push(...r.artifacts);
        for (const m of r.members)
          projectMetadata(
            inventory,
            m.type,
            m.name,
            m.data,
            source,
            "retrieve",
          );
        for (const missing of r.missing)
          addGap(
            inventory,
            source,
            "retrieve",
            "administrator",
            missing,
            "*",
            "COMPONENT_UNAVAILABLE",
          );
        if (r.outcome !== "complete") {
          op.state = "unavailable";
          addGap(
            inventory,
            source,
            "retrieve",
            "administrator",
            "synthetic",
            "*",
            "PARTIAL_RETRIEVAL",
          );
        }
      } catch {
        op.state = "unavailable";
        addGap(
          inventory,
          source,
          "retrieve",
          "administrator",
          "synthetic",
          "*",
          "RETRIEVAL_UNAVAILABLE",
        );
      }
      try {
        inventory.identityVerified.final = sameIdentities(
          initial,
          await verifyIdentityPair(reader),
        );
      } catch {
        inventory.identityVerified.final = false;
      }
    }
  } catch {
    inventory.identityVerified.final = false;
  } finally {
    try {
      finishInventory(inventory);
      await output.writeFile(JSON.stringify(inventory, null, 2) + "\n");
    } catch {
      throw Object.assign(
        new Error("Metadata discovery internal output failure"),
        { exitCode: 1 },
      );
    } finally {
      await output.close();
    }
  }
  return inventory;
}
