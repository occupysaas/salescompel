import test from "node:test";
import assert from "node:assert/strict";
import { writeFile, mkdir, symlink, readdir, rm } from "node:fs/promises";
import { join } from "node:path";
import { bodyCommand } from "../src/external-id-body-command.ts";
import { bodyFixture } from "./external-id-body-test-server.ts";
import { externalIdCommand } from "../src/external-id-command.ts";
import { spawnSync } from "node:child_process";
async function setup(t: Parameters<typeof bodyFixture>[0]) {
  const x = await bodyFixture(t),
    config = join(x.dir, "config.json");
  await writeFile(config, JSON.stringify(x.config), { mode: 0o600 });
  const env = {
    COMPAT_REFERENCE_URL: x.config.origin,
    COMPAT_REFERENCE_TOKEN: "synthetic",
    COMPAT_DISPOSABLE: "yes",
    COMPAT_RECOVERY_DIR: x.dir,
  };
  return { ...x, configFile: config, env };
}
test("private outputs are exclusive and authorization precedes writes", async (t) => {
  const x = await setup(t),
    out = join(x.dir, "out.json");
  await assert.rejects(
    bodyCommand(
      "probe",
      x.configFile,
      out,
      { ...x.env, COMPAT_DISPOSABLE: "" },
      undefined,
      x.transport,
    ),
  );
  await writeFile(out, "original", { mode: 0o600 });
  await assert.rejects(
    bodyCommand("probe", x.configFile, out, x.env, undefined, x.transport),
  );
  const unsafe = join(x.dir, "unsafe");
  await mkdir(unsafe, { mode: 0o755 });
  await assert.rejects(
    bodyCommand(
      "probe",
      x.configFile,
      join(unsafe, "out.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  await symlink(x.configFile, join(x.dir, "link.json"));
  await assert.rejects(
    bodyCommand(
      "probe",
      join(x.dir, "link.json"),
      join(x.dir, "next.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  assert.equal(x.calls.length, 0);
});
test("pending run blocks both suites", async (t) => {
  const x = await setup(t);
  x.flags.loss = true;
  await bodyCommand(
    "probe",
    x.configFile,
    join(x.dir, "first.json"),
    x.env,
    undefined,
    x.transport,
  );
  const count = x.calls.length;
  await assert.rejects(
    bodyCommand(
      "probe",
      x.configFile,
      join(x.dir, "second.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  await assert.rejects(
    externalIdCommand(
      "probe",
      x.configFile,
      join(x.dir, "p9.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  assert.equal(x.calls.length, count);
});
test("second run rechecks first absence before fixture writes", async (t) => {
  const x = await setup(t);
  const r = await bodyCommand(
    "probe",
    x.configFile,
    join(x.dir, "first.json"),
    x.env,
    undefined,
    x.transport,
  );
  assert.ok("cleanup" in r);
  const o = r.observations[0];
  x.rows.set(o.before[0].Id, o.before[0]);
  const count = x.calls.filter((c) =>
    ["POST", "PATCH"].includes(c.method),
  ).length;
  await assert.rejects(
    bodyCommand(
      "probe",
      x.configFile,
      join(x.dir, "second.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  assert.equal(
    x.calls.filter((c) => ["POST", "PATCH"].includes(c.method)).length,
    count,
  );
});
test("CLI rejects unknown flags with invocation exit 1", () => {
  const r = spawnSync(
    process.execPath,
    ["--import", "tsx", "src/cli.ts", "probe-external-id-body", "--bogus", "x"],
    { encoding: "utf8" },
  );
  assert.equal(r.status, 1);
});

test("failed publication retains recovery lock", async (t) => {
  const x = await setup(t),
    out = join(x.dir, "out.json");
  let sabotaged = false;
  const transport = {
    async request(path: string, init: RequestInit) {
      if (!sabotaged) {
        sabotaged = true;
        await rm(out);
        await mkdir(out, { mode: 0o700 });
      }
      return structuredClone(await x.transport.request(path, init));
    },
  };
  await assert.rejects(
    bodyCommand("probe", x.configFile, out, x.env, undefined, transport),
  );
  assert.ok(x.calls.some((c) => c.method === "PATCH"));
  assert.ok((await readdir(x.dir)).some((n) => n.endsWith(".active.json")));
  const count = x.calls.length;
  await assert.rejects(
    bodyCommand(
      "probe",
      x.configFile,
      join(x.dir, "second.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  assert.equal(x.calls.length, count);
});

test("invalid last-clean evidence blocks before mutations", async (t) => {
  const x = await setup(t);
  await bodyCommand(
    "probe",
    x.configFile,
    join(x.dir, "first.json"),
    x.env,
    undefined,
    x.transport,
  );
  await rm(join(x.dir, "first.artifacts", "journal.jsonl"));
  const count = x.calls.length;
  await assert.rejects(
    bodyCommand(
      "probe",
      x.configFile,
      join(x.dir, "second.json"),
      x.env,
      undefined,
      x.transport,
    ),
  );
  assert.equal(x.calls.length, count);
});

test("shared lock covers the previous-clean recheck", async (t) => {
  const x = await setup(t);
  await bodyCommand(
    "probe",
    x.configFile,
    join(x.dir, "first.json"),
    x.env,
    undefined,
    x.transport,
  );
  let checked = false,
    held = false;
  const transport = {
    async request(path: string, init: RequestInit) {
      if (!checked && path.endsWith("userinfo")) {
        checked = true;
        held = (await readdir(x.dir)).some((n) => n.endsWith(".active.json"));
      }
      return structuredClone(await x.transport.request(path, init));
    },
  };
  await bodyCommand(
    "probe",
    x.configFile,
    join(x.dir, "second.json"),
    x.env,
    undefined,
    transport,
  );
  assert.equal(held, true);
});
