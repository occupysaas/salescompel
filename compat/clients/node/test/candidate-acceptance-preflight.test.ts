import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
test("source binding includes embedded API bytes and refuses untracked API code", async (t) => {
  const { mkdir, copyFile, writeFile } = await import("node:fs/promises");
  const { execFileSync } = await import("node:child_process");
  const { ROOT } = await import("../src/versions.ts");
  const dir = await mkdtemp(join(tmpdir(), "p23-bindings-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  for (const file of [
    "go.mod",
    "go.sum",
    "web/package-lock.json",
    "compat/clients/node/package-lock.json",
    "compat/clients/python/requirements.lock",
    "internal/platform/types.go",
    "internal/metadata/model.go",
    "internal/records/write.go",
    "internal/query/select.go",
    "internal/platform/metadata.go",
    "internal/migration/migrate.go",
    "internal/migration/v1.go",
    "internal/migration/migrations/0002_tenant_catalog.sql",
    "internal/migration/migrations/0003_tenant_rls.sql",
    "internal/platform/catalog_v1.json",
    "internal/platform/schema.sql",
    "api/candidate-openapi.yaml",
    "api/candidate_contract.go",
  ]) {
    const { dirname } = await import("node:path");
    await mkdir(dirname(join(dir, file)), { recursive: true });
    await copyFile(join(ROOT, file), join(dir, file));
  }
  const git = (args: string[]) =>
    execFileSync("git", args, { cwd: dir, stdio: "ignore" });
  git(["init", "-q"]);
  git(["add", "."]);
  git([
    "-c",
    "user.name=Fixture",
    "-c",
    "user.email=fixture@example.invalid",
    "commit",
    "-qm",
    "fixture",
  ]);
  const initial = await collectSourceBindings(dir);
  assert.equal(initial.clean, true);
  await writeFile(join(dir, "api/new.go"), "package apicontract\n");
  assert.equal((await collectSourceBindings(dir)).clean, false);
  await rm(join(dir, "api/new.go"));
  await writeFile(
    join(dir, "api/candidate-openapi.yaml"),
    "changed contract\n",
  );
  assert.notEqual((await collectSourceBindings(dir)).source, initial.source);
  await copyFile(
    join(ROOT, "api/candidate-openapi.yaml"),
    join(dir, "api/candidate-openapi.yaml"),
  );
  await writeFile(
    join(dir, "api/candidate_contract.go"),
    "package apicontract\n",
  );
  assert.notEqual((await collectSourceBindings(dir)).source, initial.source);
});
import {
  preflightAcceptance,
  collectSourceBindings,
} from "../src/candidate-acceptance-preflight.ts";
test("preflight rejects missing opt-in and conflicting database URLs before commands", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "p15-preflight-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const options = {
    scope: "candidate-only" as const,
    out: join(dir, "report.json"),
  };
  await assert.rejects(preflightAcceptance(options, {}), /PREREQUISITE/);
  await assert.rejects(
    preflightAcceptance(options, {
      CANDIDATE_DISPOSABLE: "yes",
      DATABASE_URL: "postgres://a",
      TEST_DATABASE_URL: "postgres://b",
    }),
    /PREREQUISITE/,
  );
});
test("source bindings record actual tracked inputs without raw environment values", async () => {
  const r = await collectSourceBindings();
  assert.match(r.source, /^[a-f0-9]{64}$/);
  assert.match(r.revision, /^[a-f0-9]{40}$/);
  assert.ok(!JSON.stringify(r).includes("DATABASE_URL"));
});
test("occupied ports are refused without disturbing their owner", async (t) => {
  const { createServer } = await import("node:net"),
    { availablePort } =
      await import("../src/candidate-acceptance-preflight.ts");
  const server = createServer();
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  t.after(() => server.close());
  const address = server.address() as import("node:net").AddressInfo;
  await assert.rejects(availablePort(address.port));
  assert.equal(server.listening, true);
});
test("catalog inputs have explicit digests alongside dependency lockfiles", async () => {
  const { readFile } = await import("node:fs/promises");
  const { ROOT } = await import("../src/versions.ts");
  const { digest } = await import("../src/candidate-acceptance-results.ts");
  const bindings = await collectSourceBindings();
  for (const path of [
    "internal/platform/types.go",
    "internal/metadata/model.go",
    "internal/records/write.go",
    "internal/query/select.go",
    "internal/platform/metadata.go",
    "internal/migration/migrate.go",
    "internal/migration/v1.go",
    "internal/migration/migrations/0002_tenant_catalog.sql",
    "internal/migration/migrations/0003_tenant_rls.sql",
    "internal/platform/catalog_v1.json",
    "internal/platform/schema.sql",
  ]) {
    assert.equal(
      bindings.locks[path],
      digest(await readFile(join(ROOT, path))),
    );
  }
});
