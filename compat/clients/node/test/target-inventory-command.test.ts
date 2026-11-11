import test from "node:test";
import assert from "node:assert/strict";
import fs from "node:fs/promises";
import { join } from "node:path";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import {
  makeInventory,
  makeCompleteInventory,
} from "./target-inventory-fixtures.ts";
import { validateTargetInventoryFile } from "../src/target-inventory-command.ts";
import {
  withDir,
  writeInput,
  cli,
} from "./helpers/target-inventory-command.ts";
const exec = promisify(execFile);
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
test("CLI argument parse errors cannot echo private flag values before dispatch", async () => {
  const r = await cli(["--input"]);
  assert.equal(r.code, 1);
  assert.ok(!(r.stdout + r.stderr).includes("INJECTED_SECRET_1234"));
});

test("oversized input with --out actually writes the promised invalid report", async () =>
  withDir(async (d) => {
    const p = await writeInput(d);
    await fs.writeFile(p, Buffer.alloc(1048577, 32));
    const out = join(d, "oversized.json");
    const result = await cli(["--input", p, "--out", out]);
    assert.equal(result.code, 1);
    assert.match(result.stdout, /private report written/);
    assert.equal(
      JSON.parse(await fs.readFile(out, "utf8")).issues[0].code,
      "input-too-large",
    );
  }));

test("empty --out is rejected instead of silently sending the report to stdout", async () =>
  withDir(async (d) => {
    const p = await writeInput(d);
    const r = await cli(["--input", p, "--out", ""]);
    assert.equal(r.code, 1);
    assert.ok(!r.stdout.includes("inventoryId"));
  }));

test("YAML collection keys never disclose private input through CLI stderr", async () =>
  withDir(async (d) => {
    const p = await writeInput(d);
    await fs.writeFile(p, "? [INJECTED_SECRET_1234]\n: x\n");
    const r = await cli(["--input", p]);
    assert.equal(r.code, 1);
    assert.equal(r.stderr, "");
    assert.ok(!(r.stdout + r.stderr).includes("INJECTED_SECRET_1234"));
  }));
