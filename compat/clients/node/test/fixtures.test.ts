import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { FixtureTracker } from "../src/fixtures.ts";
test("cleanup only removes tracked IDs, in reverse order, and preserves failures", async () => {
  const dir = await mkdtemp(join(tmpdir(), "sc-test-"));
  try {
    const t = new FixtureTracker(join(dir, "journal"));
    const calls: string[] = [];
    await t.add(
      "Account:owned",
      async () => {
        calls.push("owned");
      },
      { original: { LocaleSidKey: "de_DE" } },
    );
    await t.add("Contact:child", async () => {
      calls.push("child");
      throw new Error("denied");
    });
    const results = await t.cleanup();
    assert.deepEqual(calls, ["child", "owned"]);
    assert.equal(results[0].outcome, "failed");
    assert.equal(results[1].outcome, "passed");
    assert.match(await readFile(join(dir, "journal"), "utf8"), /Account:owned/);
    assert.match(await readFile(join(dir, "journal"), "utf8"), /de_DE/);
    assert.deepEqual(await t.cleanup(), []);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});
test("journal records create intent before an ID is available", async () => {
  const dir = await mkdtemp(join(tmpdir(), "sc-intent-"));
  try {
    const file = join(dir, "journal");
    const t = new FixtureTracker(file);
    await t.intent("Account", { Name: "SC unique-run errors" });
    assert.match(await readFile(file, "utf8"), /unique-run/);
    assert.deepEqual(await t.cleanup(), []);
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});

test("persistent recovery directory must be absolute and outside checkout including symlinks", async () => {
  const { validateRecoveryDirectory } = await import("../src/fixtures.ts");
  const { symlink, mkdir } = await import("node:fs/promises");
  const dir = await mkdtemp(join(tmpdir(), "sc-recovery-"));
  try {
    const root = join(dir, "repo"),
      external = join(dir, "outside");
    await mkdir(root);
    await mkdir(external);
    await symlink(root, join(dir, "link"));
    assert.equal(await validateRecoveryDirectory(external, root), undefined);
    assert.ok(await validateRecoveryDirectory("relative", root));
    assert.ok(await validateRecoveryDirectory(root, root));
    assert.ok(await validateRecoveryDirectory(join(dir, "link"), root));
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});
