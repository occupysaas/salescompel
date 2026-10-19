import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm, writeFile, mkdir, symlink } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { externalIdCommand } from "../src/external-id-command.ts";
import { localProbe } from "./external-id-test-server.ts";
test("external-ID unsafe outputs, missing opt-in and collisions fail before transport", async (t) => {
  const x = await localProbe(t);
  const dir = await mkdtemp(join(tmpdir(), "p9-command-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const config = join(dir, "config.json");
  await writeFile(config, JSON.stringify(x.config), { mode: 0o600 });
  const env = {
    COMPAT_REFERENCE_URL: x.config.origin,
    COMPAT_REFERENCE_TOKEN: "synthetic",
    COMPAT_RECOVERY_DIR: dir,
    COMPAT_DISPOSABLE: "yes",
  };
  await assert.rejects(
    externalIdCommand("probe", config, join(dir, "out.json"), {
      ...env,
      COMPAT_DISPOSABLE: "",
    }),
  );
  await writeFile(join(dir, "existing.json"), "original", { mode: 0o600 });
  await assert.rejects(
    externalIdCommand("probe", config, join(dir, "existing.json"), env),
  );
  const unsafe = join(dir, "unsafe");
  await mkdir(unsafe, { mode: 0o755 });
  await assert.rejects(
    externalIdCommand("probe", config, join(unsafe, "out.json"), env),
  );
  await symlink(config, join(dir, "link.json"));
  await assert.rejects(
    externalIdCommand(
      "probe",
      join(dir, "link.json"),
      join(dir, "other.json"),
      env,
    ),
  );
  assert.equal(x.calls.length, 0);
});

import { createHash } from "node:crypto";
import {
  initializeRun,
  appendJournal,
  durableJSON,
} from "../src/external-id-recovery.ts";
test("recovery without final report retains unresolved lock and blocks a second probe", async (t) => {
  const x = await localProbe(t);
  const root = await mkdtemp(join(tmpdir(), "p9-locked-"));
  t.after(() => rm(root, { recursive: true, force: true }));
  const configFile = join(root, "config.json");
  await writeFile(configFile, JSON.stringify(x.config), { mode: 0o600 });
  await initializeRun(x.config, x.dir, "e".repeat(24));
  await appendJournal(x.dir, { kind: "intent", id: "exact:first" });
  await appendJournal(x.dir, { kind: "dispatched", id: "exact:first" });
  const lock = join(
    root,
    "external-id-" +
      createHash("sha256")
        .update(JSON.stringify([x.config.origin, x.config.administrator]))
        .digest("hex") +
      ".active.json",
  );
  await durableJSON(lock, { dir: x.dir });
  const env = {
    COMPAT_REFERENCE_URL: x.config.origin,
    COMPAT_REFERENCE_TOKEN: "synthetic",
    COMPAT_DISPOSABLE: "yes",
    COMPAT_RECOVERY_DIR: root,
  };
  const result = await externalIdCommand(
    "recover",
    configFile,
    join(root, "recovery.json"),
    env,
    x.dir,
    x.transport,
  );
  assert.equal("outcome" in result && result.outcome, "blocked");
  const count = x.calls.length;
  await assert.rejects(
    externalIdCommand(
      "probe",
      configFile,
      join(root, "second.json"),
      env,
      undefined,
      x.transport,
    ),
  );
  assert.equal(x.calls.length, count);
});
