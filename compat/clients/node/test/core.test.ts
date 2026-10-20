import test from "node:test";
import assert from "node:assert/strict";
import {
  compare,
  normalize,
  sanitize,
  checkConfig,
  validateCatalogs,
  parseDocument,
  validateReport,
  type Report,
} from "../src/core.ts";

const report = (target: "reference" | "candidate"): Report => ({
  schemaVersion: 1,
  target,
  profile: "pilot",
  profileHash: "abc",
  apiVersion: "65.0",
  provenance: "live",
  startedAt: "2026-10-15T00:00:00Z",
  finishedAt: "2026-10-15T00:01:00Z",
  connectors: { jsforce: "3.11.0" },
  requiredScenarios: ["crud"],
  results: [
    {
      id: "crud",
      client: "jsforce",
      outcome: "passed",
      assertions: [{ name: "saved", passed: true }],
      observations: {
        status: 201,
        record: { Id: "001000000000001AAA", Name: "test", Amount: 0.1 },
      },
      identities: { account: "001000000000001AAA" },
    },
  ],
  cleanup: [],
});

test("comparison maintains consistent record identities and detects broken relationships", () => {
  const a = report("reference"),
    b = report("candidate");
  b.results[0].identities = { account: "001000000000002AAA" };
  (b.results[0].observations as any).record.Id = "001000000000002AAA";
  assert.equal(compare(a, b).outcome, "passed");
  (b.results[0].observations as any).record.Id = "001000000000003AAA";
  assert.equal(compare(a, b).outcome, "failed");
});
for (const [label, change] of Object.entries({
  status: (v: any): unknown => (v.status = 400),
  permission: (v: any): unknown => (v.allowed = false),
  null: (v: any): unknown => (v.record.Name = null),
  precision: (v: any): unknown => (v.record.Amount = 0.10000000001),
  rollback: (v: any): unknown => (v.rollbackCount = 1),
})) {
  test(`comparison detects ${label}`, () => {
    const a = report("reference"),
      b = report("candidate");
    change(b.results[0].observations);
    assert.equal(compare(a, b).outcome, "failed");
  });
}
test("normalization preserves omissions, arrays and unrelated timestamps", () => {
  assert.notDeepEqual(normalize({ x: null }, {}), normalize({}, {}));
  assert.notDeepEqual(normalize([1, 2], {}), normalize([2, 1], {}));
  assert.equal(
    (normalize({ CreatedDate: "2026-01-01" }, {}) as any).CreatedDate,
    "2026-01-01",
  );
});
test("comparison blocks missing scenarios, synthetic evidence, mismatched profiles/versions/clients and empty captures", () => {
  for (const mutate of [
    (r: Report) => (r.apiVersion = "64.0"),
    (r: Report) => (r.profileHash = "other"),
    (r: Report) => (r.provenance = "synthetic"),
    (r: Report) => (r.connectors.jsforce = "old"),
    (r: Report) => (r.results = []),
    (r: Report) => (r.results[0].outcome = "blocked"),
  ]) {
    const a = report("reference"),
      b = report("candidate");
    mutate(b);
    assert.equal(compare(a, b).outcome, "blocked");
  }
});
test("report validation rejects malformed or contradictory captures", () => {
  const r = report("reference");
  r.results[0].assertions[0].passed = false;
  assert.throws(() => validateReport(r));
  assert.throws(() => validateReport({}));
});
test("sanitizer removes credentials in nested fields, arrays and text", () => {
  const result = JSON.stringify(
    sanitize(
      {
        Authorization: "Bearer token",
        nested: { refresh_token: "secret", url: "https://x/?access_token=abc" },
        message: "Bearer xyz password=bad known-secret",
        array: ["known-secret"],
      },
      ["known-secret"],
    ),
  );
  for (const value of [
    "Bearer token",
    '"secret"',
    "access_token=abc",
    "Bearer xyz",
    "password=bad",
    "known-secret",
  ])
    assert.ok(!result.includes(value), result);
});
test("configuration blocks missing credentials/version/ack and unsafe endpoint", () => {
  assert.ok(checkConfig("reference", {}).length >= 3);
  const env = {
    COMPAT_REFERENCE_URL: "https://example.invalid",
    COMPAT_REFERENCE_TOKEN: "secret",
    COMPAT_API_VERSION: "65.0",
    COMPAT_DISPOSABLE: "yes",
  };
  assert.deepEqual(checkConfig("reference", env), []);
  assert.ok(
    checkConfig("reference", {
      ...env,
      COMPAT_REFERENCE_URL: "https://user:pass@example.invalid",
    }).length,
  );
});
test("catalog validation catches required keys, parameters, fallback cycles and duplicate JSON keys", () => {
  const registry = [
    {
      code: "en_US",
      category: "fully",
      direction: "ltr",
      fallback: null,
      coverage: "fixture",
    },
    {
      code: "fr",
      category: "fully",
      direction: "ltr",
      fallback: "en_US",
      coverage: "fixture",
    },
  ];
  assert.deepEqual(
    validateCatalogs(registry, {
      en_US: { hello: "Hello {name}" },
      fr: { hello: "Bonjour {name}" },
    }),
    [],
  );
  assert.ok(
    validateCatalogs(registry, {
      en_US: { hello: "Hello {name}" },
      fr: { hello: "Bonjour {nom}" },
    }).length,
  );
  assert.ok(
    validateCatalogs(registry, { en_US: { hello: "Hello {name}" }, fr: {} })
      .length,
  );
  assert.ok(
    validateCatalogs([{ ...registry[0], fallback: "fr" }, registry[1]], {
      en_US: { hello: "x" },
      fr: { hello: "x" },
    }).length,
  );
  assert.throws(() => parseDocument('{"x":1,"x":2}'));
});
test("comparison binds evidence to trusted profile and rejects extra results and empty client versions", () => {
  const a = report("reference"),
    b = report("candidate");
  assert.equal(
    compare(a, b, {
      profile: "pilot",
      profileHash: "abc",
      requiredScenarios: ["crud", "permissions"],
      connectors: { jsforce: "3.11.0" },
    }).outcome,
    "blocked",
  );
  b.results.push({ ...b.results[0], id: "hidden-failure", outcome: "failed" });
  assert.equal(compare(a, b).outcome, "blocked");
  b.results.pop();
  a.connectors = {};
  b.connectors = {};
  assert.equal(compare(a, b).outcome, "blocked");
});
test("catalog rejects malformed ICU plural messages", () => {
  const registry = [
    {
      code: "en_US",
      category: "fully",
      direction: "ltr",
      fallback: null,
      coverage: "fixture",
    },
  ];
  assert.ok(
    validateCatalogs(registry, {
      en_US: { count: "{count, plural, one {One} other {Many}" },
    }).length,
  );
});
test("registered write scenarios require cleanup evidence", () => {
  const a = report("reference"),
    b = report("candidate");
  assert.equal(
    compare(a, b, {
      profile: "pilot",
      profileHash: "abc",
      requiredScenarios: ["crud"],
      connectors: { jsforce: "3.11.0" },
      cleanupScenarios: ["crud"],
    }).outcome,
    "blocked",
  );
});
test("repeatability compares two distinct reference runs", () => {
  const a = report("reference"),
    b = report("reference");
  b.startedAt = "2026-10-15T01:00:00Z";
  b.results[0].identities = { account: "001000000000002AAA" };
  (b.results[0].observations as any).record.Id = "001000000000002AAA";
  assert.equal(compare(a, b, undefined, "repeatability").outcome, "passed");
  (b.results[0].observations as any).status = 200;
  const drift = compare(a, b, undefined, "repeatability");
  assert.equal(drift.outcome, "failed");
  assert.match(drift.differences.join("\n"), /nondeterministic/);
});
test("repeatability rejects candidates and the same run twice", () => {
  const a = report("reference");
  assert.equal(
    compare(a, report("candidate"), undefined, "repeatability").outcome,
    "blocked",
  );
  assert.equal(
    compare(a, report("reference"), undefined, "repeatability").outcome,
    "blocked",
  );
  assert.equal(compare(a, report("reference")).outcome, "blocked");
});
test("partial catalogs permit only keys available through their declared fallback chain", () => {
  const en = {
    code: "en_US",
    category: "fully",
    direction: "ltr",
    fallback: null,
    coverage: "partial",
  };
  const fr = {
    code: "fr",
    category: "fully",
    direction: "ltr",
    fallback: "en_US",
    coverage: "partial",
  };
  const catalogs = {
    en_US: { hello: "Hello {name}", preview: "Sample data" },
    fr: { hello: "Bonjour {name}" },
  };
  assert.deepEqual(validateCatalogs([en, fr], catalogs), []);
  assert.ok(
    validateCatalogs([en, { ...fr, coverage: "complete" }], catalogs).some(
      (x) => x.includes("missing preview"),
    ),
  );
  assert.ok(
    validateCatalogs([en, { ...fr, fallback: null }], catalogs).some((x) =>
      x.includes("missing preview"),
    ),
  );
  assert.ok(
    validateCatalogs([en, { ...fr, fallback: "fr" }], catalogs).some((x) =>
      x.includes("Fallback cycle"),
    ),
  );
  assert.ok(
    validateCatalogs([en, fr], {
      ...catalogs,
      fr: { hello: "Bonjour {wrong}" },
    }).some((x) => x.includes("parameters differ")),
  );
});
