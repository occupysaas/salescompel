import test from "node:test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  writeFile,
  rm,
  symlink,
  chmod,
  readFile,
} from "node:fs/promises";
import { spawnSync } from "node:child_process";
import { tmpdir } from "node:os";
import { join, resolve } from "node:path";
import { profile, ROOT } from "../src/validate.ts";
test("unsafeOutputFailsBeforeTransport and no output overwrite", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "metadata-cli-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const config = join(dir, "config.json");
  await writeFile(
    config,
    JSON.stringify({
      schemaVersion: 1,
      apiVersion: "65.0",
      profile: "rest-query-foundation",
      profileHash: (await profile("rest-query-foundation")).hash,
      origin: "https://reference.example.invalid",
    }),
    { mode: 0o600 },
  );
  const run = (out: string) =>
    spawnSync(
      process.execPath,
      [
        "--import",
        "tsx",
        "src/cli.ts",
        "discover-metadata",
        "--config",
        config,
        "--out",
        out,
      ],
      {
        cwd: resolve(ROOT, "compat/clients/node"),
        encoding: "utf8",
        env: {
          ...process.env,
          COMPAT_REFERENCE_TOKEN: "synthetic-secret",
          COMPAT_REFERENCE_RESTRICTED_TOKEN: "restricted-secret",
          COMPAT_REFERENCE_URL: "https://reference.example.invalid",
        },
      },
    );
  const inside = run(resolve(ROOT, "compat/private/discovery-test.json"));
  assert.equal(inside.status, 2);
  assert.match(inside.stderr, /private|external/i);
  assert.doesNotMatch(inside.stderr, /synthetic-secret/);
  const existing = join(dir, "exists.json");
  await writeFile(existing, "untouched");
  const collision = run(existing);
  assert.equal(collision.status, 2);
  assert.equal(await readFile(existing, "utf8"), "untouched");
  await symlink(ROOT, join(dir, "link"));
  assert.equal(run(join(dir, "link", "inventory.json")).status, 2);
  await chmod(dir, 0o755);
  assert.equal(run(join(dir, "open.json")).status, 2);
});
