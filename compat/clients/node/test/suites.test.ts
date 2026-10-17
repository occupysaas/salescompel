import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { spawnSync } from "node:child_process";
import { capture } from "../src/capture.ts";
import { profile } from "../src/validate.ts";

const foundation = "rest-query-foundation";
test("selected foundation profile cannot fall back to pilot", async () => {
  const p = await profile(foundation);
  assert.equal(p.data.id, foundation);
  assert.notEqual(p.hash, (await profile()).hash);
  assert.ok(p.data.requiredScenarios.includes("foundation-pagination"));
});
test("unregistered suite is rejected rather than interpreted as a path", async () => {
  await assert.rejects(profile("../../private"), /suite/i);
});
test("foundation capture blocks unbound readiness before transport with its own manifest", async (t) => {
  let requests = 0;
  const server = createServer((_req, res) => {
    requests++;
    res.end("{}");
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => server.close());
  const address = server.address() as { port: number };
  const r = await capture(
    "reference",
    {
      COMPAT_REFERENCE_URL: `http://127.0.0.1:${address.port}`,
      COMPAT_REFERENCE_TOKEN: "synthetic-secret",
      COMPAT_REFERENCE_RESTRICTED_TOKEN: "synthetic-restricted",
      COMPAT_REFERENCE_LOCALE_USER_ID: "synthetic-user",
      COMPAT_API_VERSION: "65.0",
      COMPAT_DISPOSABLE: "yes",
    },
    foundation,
  );
  assert.equal(r.profile, foundation);
  assert.equal(r.results.length, 7);
  assert.ok(r.results.every((x) => x.outcome === "blocked"));
  assert.equal(r.provenance, "none");
  assert.equal(requests, 0);
  assert.ok(!JSON.stringify(r).includes("synthetic-secret"));
});
test("CLI suite selection reaches foundation readiness rather than rejecting its option", () => {
  const r = spawnSync(
    process.execPath,
    [
      "--import",
      "tsx",
      "src/cli.ts",
      "preflight",
      "--target",
      "reference",
      "--suite",
      foundation,
    ],
    {
      encoding: "utf8",
      env: {
        ...process.env,
        COMPAT_REFERENCE_TOKEN: "",
        COMPAT_REFERENCE_URL: "",
      },
    },
  );
  assert.equal(r.status, 2);
  assert.match(r.stdout + r.stderr, /current profile|not ready/);
  assert.doesNotMatch(r.stderr, /Unknown option/);
});

test("CLI comparison and pair verification reject reports bound to another suite", async (t) => {
  const { mkdtemp, writeFile, readFile, rm } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const { SCENARIOS, CLEANUP_SCENARIOS } = await import("../src/validate.ts");
  const dir = await mkdtemp(join(tmpdir(), "suite-mismatch-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const p = await profile();
  for (const [name, day] of [
    ["first", "01"],
    ["second", "02"],
  ]) {
    await writeFile(
      join(dir, `${name}.json`),
      JSON.stringify({
        schemaVersion: 1,
        target: "reference",
        profile: "pilot",
        profileHash: p.hash,
        apiVersion: p.data.apiVersion,
        provenance: "live",
        connectors: p.data.connectors,
        startedAt: `2026-01-${day}T00:00:00Z`,
        finishedAt: `2026-01-${day}T00:01:00Z`,
        requiredScenarios: SCENARIOS,
        results: SCENARIOS.map((id) => ({
          id,
          client: "jsforce",
          outcome: "passed",
          assertions: [{ name: "synthetic", passed: true }],
          observations: {},
          identities: {},
        })),
        cleanup: CLEANUP_SCENARIOS.map((scenario) => ({
          scenario,
          resource: "synthetic",
          outcome: "passed",
        })),
      }),
    );
  }
  for (const command of ["repeatability", "verify-evidence"]) {
    const out = join(dir, `${command}.json`);
    const r = spawnSync(
      process.execPath,
      [
        "--import",
        "tsx",
        "src/cli.ts",
        command,
        "--suite",
        foundation,
        "--first",
        join(dir, "first.json"),
        "--second",
        join(dir, "second.json"),
        "--out",
        out,
      ],
      { encoding: "utf8" },
    );
    assert.equal(r.status, 2, r.stderr);
    assert.equal(JSON.parse(await readFile(out, "utf8")).outcome, "blocked");
  }
});

test("foundation candidate capture requires independent cursor sessions before any requests", async (t) => {
  let calls = 0;
  const server = createServer((_req, res) => {
    calls++;
    res.writeHead(500);
    res.end("{}");
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  t.after(() => server.close());
  const r = await capture(
    "candidate",
    {
      COMPAT_CANDIDATE_URL: `http://127.0.0.1:${(server.address() as any).port}`,
      COMPAT_CANDIDATE_TOKEN: "synthetic-primary",
      COMPAT_API_VERSION: "65.0",
      COMPAT_DISPOSABLE: "yes",
    },
    foundation,
  );
  assert.equal(calls, 0);
  assert.equal(r.provenance, "none");
  assert.ok(r.results.every((x) => x.outcome === "blocked"));
  assert.match(r.results[0].reason!, /FRESH_TOKEN/);
});
