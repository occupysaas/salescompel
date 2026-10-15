import test from "node:test";
import assert from "node:assert/strict";
import { execFile } from "node:child_process";
import { promisify } from "node:util";
import { mkdtemp, readFile, rm } from "node:fs/promises";
import { join, resolve } from "node:path";
import { tmpdir } from "node:os";
const exec = promisify(execFile);
test("capture without credentials writes blocked report and exits nonzero", async () => {
  const dir = await mkdtemp(join(tmpdir(), "sc-cli-"));
  try {
    const env = Object.fromEntries(
      Object.entries(process.env).filter(([k]) => !k.startsWith("COMPAT_")),
    );
    await assert.rejects(
      exec(
        process.execPath,
        [
          "--import",
          "tsx",
          "src/cli.ts",
          "capture",
          "--target",
          "reference",
          "--out",
          resolve(dir, "capture.json"),
        ],
        { env },
      ),
      (e: any) => e.code === 2,
    );
    const r = JSON.parse(await readFile(join(dir, "capture.json"), "utf8"));
    assert.equal(r.provenance, "none");
    assert.ok(r.results.length >= 11);
    assert.ok(r.results.every((v: any) => v.outcome === "blocked"));
    assert.match(
      await readFile(join(dir, "capture.md"), "utf8"),
      /No parity certification/,
    );
  } finally {
    await rm(dir, { recursive: true, force: true });
  }
});
