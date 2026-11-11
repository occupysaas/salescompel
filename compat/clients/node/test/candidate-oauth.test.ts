import test from "node:test";
import assert from "node:assert/strict";
import {
  parseLauncherURL,
  redactOAuthEvidence,
} from "../src/candidate-oauth.ts";
import type { GateContext } from "../src/candidate-acceptance-preflight.ts";

test("OAuth launcher accepts only the candidate authorization origin", () => {
  const root = "https://127.0.0.1:58443";
  const valid =
    root + "/services/oauth2/authorize?client_id=PlatformCLI&state=x";
  assert.equal(parseLauncherURL(valid + "\n", root), valid);
  for (const value of [
    "https://evil.example/services/oauth2/authorize",
    root + ".evil.example/",
    "http://127.0.0.1:58443/services/oauth2/authorize",
    "https://user:pass@127.0.0.1:58443/services/oauth2/authorize",
    root + "/services/data",
    valid + "\n" + valid,
    "javascript:alert(1)",
    root + "/services/oauth2/authorize#fragment",
  ])
    assert.throws(() => parseLauncherURL(value, root), /RESULT/);
});

test("OAuth evidence removes credentials in plain, encoded and structured output", () => {
  const secrets = [
    "random-password&with=punctuation",
    "00D123!" + "a".repeat(64),
    "b".repeat(64),
    "c".repeat(64),
  ];
  const input =
    JSON.stringify({
      password: secrets[0],
      access_token: secrets[1],
      refresh_token: secrets[2],
      client_secret: secrets[3],
      nested: {
        url:
          "https://candidate.invalid/?code=" + encodeURIComponent(secrets[0]),
      },
    }) +
    "\npassword=" +
    encodeURIComponent(secrets[0]);
  const safe = redactOAuthEvidence(input, secrets);
  for (const secret of secrets) {
    assert.ok(!safe.includes(secret));
    assert.ok(!safe.includes(encodeURIComponent(secret)));
  }
  assert.ok(safe.includes("[REDACTED]"));
  const lowerEscapes = encodeURIComponent(secrets[0]).replace(
    /%[0-9A-F]{2}/g,
    (s) => s.toLowerCase(),
  );
  assert.ok(!redactOAuthEvidence(lowerEscapes, secrets).includes(lowerEscapes));
  assert.equal(
    redactOAuthEvidence("candidate assertion failed", secrets),
    "candidate assertion failed",
  );
});

test("OAuth step publication is source-bound and digest-verified", async (t) => {
  const { mkdtemp, readFile, rm } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const { publishStepResult } =
    await import("../src/candidate-acceptance-run.ts");
  const { digest } = await import("../src/candidate-acceptance-results.ts");
  const { requiredCoverage, assessCoverage } =
    await import("../src/candidate-acceptance-contract.ts");
  const scratch = await mkdtemp(join(tmpdir(), "oauth-publication-"));
  t.after(() => rm(scratch, { recursive: true, force: true }));
  const c = {
    scratch,
    runId: "a".repeat(24),
    bindings: { source: "b".repeat(64) },
  } as unknown as GateContext;
  const parsed = {
    schemaVersion: 1 as const,
    runId: c.runId,
    suite: "oauth" as const,
    complete: true,
    exitCode: 0,
    tests: requiredCoverage.oauth.map((id) => ({
      id,
      outcome: "passed" as const,
    })),
  };
  const result = await publishStepResult(c, "oauth", parsed);
  assert.equal(assessCoverage("oauth", result.parsed).outcome, "passed");
  assert.equal(
    result.sha256,
    digest(await readFile(join(scratch, "oauth-result.json"))),
  );
  const binding = JSON.parse(
    await readFile(join(scratch, "oauth-result.json.binding.json"), "utf8"),
  );
  assert.equal(binding.source, c.bindings.source);
  assert.equal(binding.gate, "oauth");
  assert.equal(binding.digest, result.sha256);
});

test("occupied sf callback port blocks OAuth preflight", async () => {
  const { createServer } = await import("node:net");
  const preflight = await import("../src/candidate-acceptance-preflight.ts");
  const server = createServer();
  await new Promise<void>((resolve) =>
    server.listen(1717, "127.0.0.1", resolve),
  );
  try {
    assert.equal(
      typeof (preflight as Record<string, unknown>).preflightOAuth,
      "function",
    );
    await assert.rejects(
      (preflight as { preflightOAuth: () => Promise<unknown> }).preflightOAuth(),
      /PREREQUISITE/,
    );
  } finally {
    await new Promise<void>((resolve) => server.close(() => resolve()));
  }
});

test("served authorization contract permits CLI prompt query", async () => {
  const { readFile } = await import("node:fs/promises");
  const { parse } = await import("yaml");
  const d = parse(
    await readFile(
      new URL("../../../../api/candidate-openapi.yaml", import.meta.url),
      "utf8",
    ),
  );
  assert.ok(
    d.paths["/services/oauth2/authorize"].get.parameters.some(
      (p: {
        name: string;
        in: string;
        schema: { type: string };
        required: boolean;
      }) =>
        p.name === "prompt" &&
        p.in === "query" &&
        p.schema.type === "string" &&
        p.required === false,
    ),
  );
});

test("OAuth clients cannot send credentials outside candidate origin", async () => {
  const oauth = await import("../src/candidate-oauth.ts");
  const guard = (
    oauth as unknown as {
      assertCandidateOAuthEndpoints: (
        config: Record<string, string | undefined>,
        root: string,
      ) => void;
    }
  ).assertCandidateOAuthEndpoints;
  assert.equal(typeof guard, "function");
  const root = "https://localhost:58443";
  const config = {
    loginUrl: root + "/",
    authzServiceUrl: root + "/services/oauth2/authorize",
    tokenServiceUrl: root + "/services/oauth2/token",
    revokeServiceUrl: root + "/services/oauth2/revoke",
  };
  assert.doesNotThrow(() => guard(config, root));
  for (const key of Object.keys(config)) {
    assert.throws(
      () =>
        guard(
          {
            ...config,
            [key]: "https://login.salesforce.com/services/oauth2/token",
          },
          root,
        ),
      /SOURCE/,
    );
    assert.throws(() => guard({ ...config, [key]: undefined }, root), /SOURCE/);
  }
});
