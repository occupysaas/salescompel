import test from "node:test";
import assert from "node:assert/strict";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
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
