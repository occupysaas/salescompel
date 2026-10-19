import test from "node:test";
import assert from "node:assert/strict";
import { createHash } from "node:crypto";
import {
  mkdtemp,
  readFile,
  rm,
  writeFile,
  chmod,
  symlink,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { existsSync } from "node:fs";
import {
  writeFoundationSidecar,
  readFoundationSidecar,
  type FoundationBinding,
} from "../src/foundation-sidecar.ts";
import { runAssumedFoundationCapture } from "../src/foundation-command.ts";

const binding: FoundationBinding = {
  profile: "rest-query-foundation",
  profileHash: "a".repeat(64),
  apiVersion: "65.0",
  connectors: { jsforce: "3.11.0" },
  probeDigest: "b".repeat(64),
  manifestDigest: "c".repeat(64),
};
const report = {
  profile: binding.profile,
  profileHash: binding.profileHash,
  apiVersion: binding.apiVersion,
  connectors: binding.connectors,
  startedAt: "2026-10-19T12:00:00.000Z",
  finishedAt: "2026-10-19T12:01:00.000Z",
};

test("sidecar binds immutable raw bytes and rejects replay or edits", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "foundation-policy-"));
  await chmod(dir, 0o700);
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "capture.json");
  const raw = JSON.stringify(report) + "\n";
  await writeFile(path, raw, { mode: 0o600 });
  const sidecar = await writeFoundationSidecar(
    path,
    report,
    binding,
    "d".repeat(64),
  );
  const parsed = await readFoundationSidecar(path, sidecar);
  assert.equal(
    parsed.captureSha256,
    createHash("sha256").update(raw).digest("hex"),
  );
  assert.equal(parsed.assumptionId, "foundation-query-result-lifetime");
  await assert.rejects(
    writeFoundationSidecar(path, report, binding, "d".repeat(64)),
  );
  await writeFile(path, raw + " ");
  await assert.rejects(readFoundationSidecar(path, sidecar));
});

test("assumed capture command fails closed before creating output without reference readiness", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "foundation-command-"));
  await chmod(dir, 0o700);
  t.after(() => rm(dir, { recursive: true, force: true }));
  const out = join(dir, "capture.json");
  await assert.rejects(
    runAssumedFoundationCapture(out, {
      COMPAT_REFERENCE_TOKEN: "",
      COMPAT_DISPOSABLE: "",
    }),
    /Missing|readiness|required/i,
  );
  assert.equal(existsSync(out), false);
  assert.equal(existsSync(out.replace(/\.json$/, ".policy.json")), false);
});

test("sidecar refuses missing captures, unsafe modes and symlinked inputs", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "foundation-policy-"));
  await chmod(dir, 0o700);
  t.after(() => rm(dir, { recursive: true, force: true }));
  const path = join(dir, "capture.json");
  await assert.rejects(
    writeFoundationSidecar(path, report, binding, "d".repeat(64)),
  );
  await writeFile(path, "{}", { mode: 0o600 });
  await chmod(path, 0o644);
  await assert.rejects(
    writeFoundationSidecar(path, report, binding, "d".repeat(64)),
  );
  await chmod(path, 0o600);
  const link = join(dir, "link.json");
  await symlink(path, link);
  await assert.rejects(
    writeFoundationSidecar(link, report, binding, "d".repeat(64)),
  );
  await chmod(dir, 0o755);
  await assert.rejects(
    writeFoundationSidecar(path, report, binding, "d".repeat(64)),
  );
});
