import test from "node:test";
import assert from "node:assert/strict";
import {
  parseLauncherURL,
  redactOAuthEvidence,
} from "../src/candidate-oauth.ts";

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
