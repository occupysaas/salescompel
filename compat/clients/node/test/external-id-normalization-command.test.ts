import assert from "node:assert/strict";
import test from "node:test";
import { writeFile, mkdir, symlink } from "node:fs/promises";
import { join } from "node:path";
import { normalizationCommand } from "../src/external-id-normalization-command.ts";
import { bodyCommand } from "../src/external-id-body-command.ts";
import { normalizationFixture } from "./external-id-normalization-test-server.ts";
export async function normalizationSetup(
  t: Parameters<typeof normalizationFixture>[0],
) {
  const x = await normalizationFixture(t),
    configFile = join(x.dir, "config.json");
  await writeFile(configFile, JSON.stringify(x.config), { mode: 0o600 });
  return {
    ...x,
    configFile,
    env: {
      COMPAT_REFERENCE_URL: x.config.origin,
      COMPAT_REFERENCE_TOKEN: "synthetic",
      COMPAT_DISPOSABLE: "yes",
      COMPAT_RECOVERY_DIR: x.dir,
    },
  };
}
test("normalization commands require private exclusive outputs and explicit authorization", async (t) => {
  const x = await normalizationSetup(t),
    out = join(x.dir, "out.json");
  await assert.rejects(
    normalizationCommand(
      "probe",
      x.configFile,
      out,
      {},
      undefined,
      x.transport,
    ),
  );
  await writeFile(out, "original", { mode: 0o600 });
  await assert.rejects(
    normalizationCommand(
      "probe",
      x.configFile,
      out,
      x.env,
      undefined,
      x.transport,
    ),
  );
  await symlink(x.configFile, join(x.dir, "link.json"));
  await assert.rejects(
    normalizationCommand(
      "probe",
      join(x.dir, "link.json"),
      join(x.dir, "next.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  await mkdir(join(x.dir, "unsafe"), { mode: 0o755 });
  await assert.rejects(
    normalizationCommand(
      "probe",
      x.configFile,
      join(x.dir, "unsafe/out.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  assert.equal(x.calls.length, 0);
});
test("pending normalization dispatch blocks both normalization and historical body commands", async (t) => {
  const x = await normalizationSetup(t);
  x.flags.lossAt = 1;
  const r = await normalizationCommand(
    "probe",
    x.configFile,
    join(x.dir, "first.json"),
    x.env,
    undefined,
    x.transport,
  );
  assert.ok("cleanup" in r && r.cleanup.outcome === "blocked");
  const n = x.calls.length;
  await assert.rejects(
    normalizationCommand(
      "probe",
      x.configFile,
      join(x.dir, "second.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  await assert.rejects(
    bodyCommand(
      "probe",
      x.configFile,
      join(x.dir, "third.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  assert.equal(x.calls.length, n);
  const recovered = await normalizationCommand(
    "recover",
    x.configFile,
    join(x.dir, "recovery.json"),
    x.env,
    join(x.dir, "first.artifacts"),
    x.transport,
  );
  assert.ok("outcome" in recovered && recovered.outcome === "blocked");
});
test("normalization CLI rejects malformed invocations without reference traffic", async () => {
  const { spawnSync } = await import("node:child_process");
  for (const command of ["probe", "recover", "assess"]) {
    const r = spawnSync(
      process.execPath,
      ["--import", "tsx", "src/cli.ts", command + "-external-id-normalization"],
      { encoding: "utf8" },
    );
    assert.equal(r.status, 1, r.stderr);
  }
});
