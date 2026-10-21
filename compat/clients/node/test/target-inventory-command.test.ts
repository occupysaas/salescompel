import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { constants } from "node:fs";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import {
  makeInventory,
  makeCompleteInventory,
} from "./target-inventory-fixtures.ts";
import {
  validateTargetInventoryFile,
  checkInventoryRepositoryRefs,
} from "../src/target-inventory-command.ts";
const exec = promisify(execFile);
const withDir = async (f: (dir: string) => Promise<void>) => {
  const d = await fs.mkdtemp(join(tmpdir(), "sc-inventory-"));
  try {
    await fs.chmod(d, 0o700);
    await f(d);
  } finally {
    await fs.rm(d, { force: true, recursive: true });
  }
};
const writeInput = async (dir: string, value: unknown = makeInventory()) => {
  const p = join(dir, "input.yaml");
  await fs.writeFile(p, JSON.stringify(value), { mode: 0o600 });
  return p;
};
async function cli(args: string[]) {
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
  } catch (e: any) {
    return {
      stdout: e.stdout as string,
      stderr: e.stderr as string,
      code: e.code,
    };
  }
}
test("file validator maps valid incomplete/complete and invalid documents without reading locators", async () =>
  withDir(async (d) => {
    const v = makeInventory();
    v.sources[0].locator = "https://127.0.0.1:1/not-read";
    let input = await writeInput(d, v);
    const r = await validateTargetInventoryFile(input);
    assert.equal(r.exitCode, 0);
    assert.equal(r.report.readiness, "synthetic-only");
    input = await writeInput(d, makeCompleteInventory("real"));
    assert.equal(
      (await validateTargetInventoryFile(input)).report.readiness,
      "ready-for-review",
    );
    await fs.writeFile(input, "broken: [INJECTED_SECRET_1234");
    const bad = await validateTargetInventoryFile(input);
    assert.equal(bad.exitCode, 1);
    assert.ok(!JSON.stringify(bad).includes("INJECTED_SECRET_1234"));
  }));
test("CLI emits JSON or private-file summary and enforces command-specific flags before suite loading", async () =>
  withDir(async (d) => {
    const p = await writeInput(d);
    let r = await cli(["--input", p]);
    assert.equal(r.code, 0);
    assert.equal(r.stderr, "");
    assert.equal(JSON.parse(r.stdout).validation, "valid");
    const out = join(d, "report.json");
    r = await cli(["--input", p, "--out", out]);
    assert.equal(r.code, 0);
    assert.equal(r.stderr, "");
    assert.match(r.stdout, /valid.*synthetic-only/);
    assert.ok(!r.stdout.includes(d));
    assert.equal(
      JSON.parse(await fs.readFile(out, "utf8")).readiness,
      "synthetic-only",
    );
    assert.equal((await fs.stat(out)).mode & 0o777, 0o600);
    for (const args of [
      [],
      ["--unknown", "INJECTED_SECRET_1234"],
      ["--input", p, "--suite", "INJECTED_SECRET_1234"],
      ["--input", p, "extra"],
    ]) {
      r = await cli(args);
      assert.equal(r.code, 1);
      assert.ok(!JSON.stringify(r).includes("INJECTED_SECRET_1234"));
    }
    await fs.writeFile(p, "bad: [");
    assert.equal((await cli(["--input", p])).code, 1);
    assert.equal((await cli(["--input", join(d, "missing")])).code, 2);
  }));
test("input rejects missing, symlink, directory, unreadable and FIFO files without blocking", async () =>
  withDir(async (d) => {
    const p = await writeInput(d),
      link = join(d, "link");
    await fs.symlink(p, link);
    for (const input of [link, join(d, "missing"), d])
      assert.equal((await validateTargetInventoryFile(input)).exitCode, 2);
    await fs.chmod(p, 0);
    assert.equal((await validateTargetInventoryFile(p)).exitCode, 2);
    await fs.chmod(p, 0o600);
    await exec("mkfifo", [join(d, "fifo")]);
    const r = await cli(["--input", join(d, "fifo")]);
    assert.equal(r.code, 2);
    await fs.mkdir(join(d, "child"));
    await fs.symlink(join(d, "child"), join(d, "child-link"));
    await fs.writeFile(
      join(d, "child", "input.yaml"),
      JSON.stringify(makeInventory()),
    );
    assert.equal(
      (await validateTargetInventoryFile(join(d, "child-link", "input.yaml")))
        .exitCode,
      2,
    );
  }));
test("real inputs require owner-only external files and parents; synthetic input remains usable", async () =>
  withDir(async (d) => {
    const p = await writeInput(d, makeCompleteInventory("real"));
    await fs.chmod(p, 0o644);
    const r = await validateTargetInventoryFile(p);
    assert.equal(r.exitCode, 2);
    assert.equal(r.report.inventoryId, undefined);
    await fs.chmod(p, 0o600);
    await fs.chmod(d, 0o755);
    assert.equal((await validateTargetInventoryFile(p)).exitCode, 2);
    await fs.chmod(d, 0o700);
    assert.equal((await validateTargetInventoryFile(p)).exitCode, 0);
  }));
test("oversized regular input produces bounded invalid report", async () =>
  withDir(async (d) => {
    const p = await writeInput(d);
    await fs.writeFile(p, Buffer.alloc(1048577, 32));
    const r = await validateTargetInventoryFile(p);
    assert.equal(r.exitCode, 1);
    assert.equal(r.report.issues[0].code, "input-too-large");
  }));
test("exclusive output never changes an existing report, input, link target or missing parent", async () =>
  withDir(async (d) => {
    const p = await writeInput(d),
      before = await fs.readFile(p),
      out = join(d, "out.json");
    await fs.writeFile(out, "old evidence");
    for (const target of [p, out, join(d, "missing", "out.json")])
      assert.equal((await validateTargetInventoryFile(p, target)).exitCode, 2);
    const link = join(d, "link.json");
    await fs.symlink(out, link);
    assert.equal((await validateTargetInventoryFile(p, link)).exitCode, 2);
    assert.equal(await fs.readFile(out, "utf8"), "old evidence");
    assert.deepEqual(await fs.readFile(p), before);
    await assert.rejects(fs.access(join(d, "missing")));
  }));
for (const operation of ["writeFile", "sync"] as const)
  test(`output ${operation} failure removes only the newly owned output`, async (t) =>
    withDir(async (d) => {
      const p = await writeInput(d),
        out = join(d, "out.json"),
        originalOpen = fs.open;
      t.mock.method(fs, "open", async (...args: Parameters<typeof fs.open>) => {
        const handle = await originalOpen(...args);
        if (args[0] !== out) return handle;
        return new Proxy(handle, {
          get(target, key) {
            if (key === operation)
              return async () => {
                throw new Error("INJECTED_SECRET_1234");
              };
            const value = Reflect.get(target, key);
            return typeof value === "function" ? value.bind(target) : value;
          },
        });
      });
      const r = await validateTargetInventoryFile(p, out);
      assert.equal(r.exitCode, 2);
      assert.ok(!JSON.stringify(r).includes("INJECTED_SECRET_1234"));
      await assert.rejects(fs.access(out));
      assert.ok((await fs.readFile(p)).length > 0);
    }));
test("repository resolver allows bounded approved files and exact contract/profile/heading IDs only", async () =>
  withDir(async (d) => {
    await fs.mkdir(join(d, "docs/compat"), { recursive: true });
    await fs.mkdir(join(d, "locales"));
    await fs.writeFile(
      join(d, "docs/compat/contracts.yaml"),
      "contracts:\n  - id: crud\n",
    );
    await fs.writeFile(join(d, "docs/compat/profile.yaml"), "id: pilot\n");
    await fs.writeFile(join(d, "docs/compat/guide.md"), "# Example heading\n");
    await fs.writeFile(join(d, "locales/catalog.json"), "{}");
    const good = [
      { path: "docs/compat/contracts.yaml", id: "crud" },
      { path: "docs/compat/profile.yaml", id: "pilot" },
      { path: "docs/compat/guide.md", id: "example-heading" },
      { path: "locales/catalog.json" },
    ];
    assert.deepEqual(await checkInventoryRepositoryRefs(good, d), []);
    for (const ref of [
      { path: "docs/compat/profile.yaml", id: "absent" },
      { path: "docs/compat/missing" },
      { path: "locales/catalog.json", id: "absent" },
    ])
      assert.ok((await checkInventoryRepositoryRefs([ref], d)).length);
  }));
test("repository resolver rejects traversal, schemes and symlink components without opening their targets", async () =>
  withDir(async (d) => {
    await fs.mkdir(join(d, "docs/compat"), { recursive: true });
    for (const path of [
      "/etc/passwd",
      "docs/compat/../../secret",
      "https://example.invalid",
      "C:/secret",
      "docs/compat\\secret",
      "docs/compat/%2e%2e/secret",
      "docs/compat/./secret",
      "docs/compat/\0secret",
    ])
      assert.ok(
        (await checkInventoryRepositoryRefs([{ path }], d)).length,
        path,
      );
    await fs.symlink("/etc/passwd", join(d, "docs/compat/link"));
    assert.ok(
      (await checkInventoryRepositoryRefs([{ path: "docs/compat/link" }], d))
        .length,
    );
    await fs.symlink("/tmp", join(d, "docs/compat/escape"));
    assert.ok(
      (
        await checkInventoryRepositoryRefs(
          [{ path: "docs/compat/escape/private" }],
          d,
        )
      ).length,
    );
    const original = fs.open;
    const calls: string[] = [];
    // Real resolver must reject the symlink before calling open on it.
    const mock = (() => {
      const saved = fs.open;
      (fs as any).open = async (...args: any[]) => {
        calls.push(String(args[0]));
        return (saved as any)(...args);
      };
      return () => {
        fs.open = saved;
      };
    })();
    try {
      await checkInventoryRepositoryRefs([{ path: "docs/compat/link" }], d);
      assert.equal(calls.length, 0);
    } finally {
      mock();
      fs.open = original;
    }
  }));

test("CLI argument parse errors cannot echo private flag values before dispatch", async () => {
  const r = await cli(["--input"]);
  assert.equal(r.code, 1);
  assert.ok(!(r.stdout + r.stderr).includes("INJECTED_SECRET_1234"));
});
