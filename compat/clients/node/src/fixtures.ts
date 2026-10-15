import { isAbsolute, relative } from "node:path";
import { constants } from "node:fs";
import { access, realpath, appendFile } from "node:fs/promises";
export interface Owned {
  resource: string;
  remove: () => Promise<void>;
}
export class FixtureTracker {
  private owned: Owned[] = [];
  constructor(private journal: string) {}
  async intent(type: string, fields: Record<string, unknown>): Promise<void> {
    await appendFile(
      this.journal,
      JSON.stringify({
        intent: "create",
        type,
        fields,
        createdAt: new Date().toISOString(),
      }) + "\n",
      { mode: 0o600 },
    );
  }
  async add(
    resource: string,
    remove: () => Promise<void>,
    recovery?: Record<string, unknown>,
  ): Promise<void> {
    this.owned.push({ resource, remove });
    // Journal immediately. It contains IDs, never authentication material.
    await appendFile(
      this.journal,
      JSON.stringify({
        resource,
        recovery,
        createdAt: new Date().toISOString(),
      }) + "\n",
      { mode: 0o600 },
    );
  }
  async cleanup() {
    const outcomes: {
      resource: string;
      outcome: "passed" | "failed";
      reason?: string;
    }[] = [];
    for (const item of [...this.owned].reverse()) {
      try {
        await item.remove();
        outcomes.push({ resource: item.resource, outcome: "passed" });
      } catch (e) {
        outcomes.push({
          resource: item.resource,
          outcome: "failed",
          reason: String(e),
        });
      }
    }
    this.owned = [];
    return outcomes;
  }
}

export async function validateRecoveryDirectory(
  path: string,
  root: string,
): Promise<string | undefined> {
  if (!isAbsolute(path)) return "COMPAT_RECOVERY_DIR must be absolute";
  try {
    const actual = await realpath(path),
      checkout = await realpath(root);
    const rel = relative(checkout, actual);
    if (
      !rel.startsWith(".." + (process.platform === "win32" ? "\\" : "/")) &&
      !isAbsolute(rel)
    )
      return "COMPAT_RECOVERY_DIR must be outside the checkout";
    await access(actual, constants.W_OK);
    return undefined;
  } catch {
    return "COMPAT_RECOVERY_DIR must exist and be writable";
  }
}
