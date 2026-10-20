import assert from "node:assert/strict";
import test from "node:test";
import { writeFile, readFile } from "node:fs/promises";
import { join } from "node:path";
import { runNormalizationProbe } from "../src/external-id-normalization-probe.ts";
import {
  assessNormalizationPair,
  validateNormalizationCleanupEvidence,
} from "../src/external-id-normalization-assess.ts";
import { normalizationFixture } from "./external-id-normalization-test-server.ts";
import { recoverNormalizationRun } from "../src/external-id-normalization-recovery.ts";
async function pair(
  t: Parameters<typeof normalizationFixture>[0],
  drift = false,
  uppercase = false,
  postErrors: unknown = [],
  recoverFirst = false,
) {
  const x = await normalizationFixture(t),
    a = join(x.dir, "a.json"),
    b = join(x.dir, "b.json");
  x.flags.uppercase = uppercase;
  x.flags.postErrors = postErrors;
  await (
    await import("node:fs/promises")
  ).mkdir(join(x.dir, "a.artifacts"), { mode: 0o700 });
  const first = await runNormalizationProbe(
    x.config,
    {},
    join(x.dir, "a.artifacts"),
    x.transport,
  );
  await writeFile(a, JSON.stringify(first) + "\n", { mode: 0o600 });
  if (recoverFirst) {
    const recovered = await recoverNormalizationRun(
      x.config,
      {},
      join(x.dir, "a.artifacts"),
      x.transport,
    );
    assert.equal(recovered.outcome, "clean");
  }
  await new Promise((r) => setTimeout(r, 2));
  x.flags.prose = "changed prose";
  x.flags.reverseRows = true;
  x.flags.rejectedMutation = drift;
  await (
    await import("node:fs/promises")
  ).mkdir(join(x.dir, "b.artifacts"), { mode: 0o700 });
  const second = await runNormalizationProbe(
    x.config,
    {},
    join(x.dir, "b.artifacts"),
    x.transport,
  );
  await writeFile(b, JSON.stringify(second) + "\n", { mode: 0o600 });
  return { x, a, b, first, second };
}
test("complete observations compare by identity relationships while preserving evidence boundaries", async (t) => {
  const { x, a, b, first, second } = await pair(t);
  const good = await assessNormalizationPair(a, b);
  assert.equal(good.outcome, "repeatable", JSON.stringify(good));
  assert.equal(good.findings.length, 36);
  await t.test("incomplete operation coverage blocks assessment", async () => {
    const changed = structuredClone(second);
    changed.observations.pop();
    await writeFile(b, JSON.stringify(changed));
    assert.equal((await assessNormalizationPair(a, b)).outcome, "blocked");
    await writeFile(b, JSON.stringify(second) + "\n");
  });
  await t.test(
    "changed source binding and overlapping captures block assessment",
    async () => {
      for (const update of [
        { probeHash: "f".repeat(64) },
        { startedAt: first.startedAt },
      ]) {
        await writeFile(b, JSON.stringify({ ...second, ...update }));
        assert.equal((await assessNormalizationPair(a, b)).outcome, "blocked");
      }
      await writeFile(b, JSON.stringify(second) + "\n");
    },
  );
  await t.test(
    "corrupt response bytes and incomplete cleanup proofs block assessment",
    async () => {
      const path = join(x.dir, "b.artifacts", second.observations[0].artifact),
        original = await readFile(path, "utf8");
      await writeFile(path, "{}");
      assert.equal((await assessNormalizationPair(a, b)).outcome, "blocked");
      await writeFile(path, original);
      const cleanup = structuredClone(second.cleanup);
      cleanup.records.pop();
      await assert.rejects(
        validateNormalizationCleanupEvidence(
          x.config,
          join(x.dir, "b.artifacts"),
          cleanup,
          true,
        ),
      );
    },
  );
  await t.test(
    "altering reported spelling without matching source evidence blocks assessment",
    async () => {
      // A distinct spelling experiment requires a genuine second capture; modifying only the report must block integrity.
      const changed = structuredClone(second);
      changed.observations[0].after[0].Compat_Key__c += " ";
      await writeFile(b, JSON.stringify(changed));
      assert.equal((await assessNormalizationPair(a, b)).outcome, "blocked");
      await writeFile(b, JSON.stringify(second) + "\n");
    },
  );
  assert.equal((await assessNormalizationPair(a, b)).outcome, "repeatable");
});
test("errors that secretly commit a write produce drift against clean rejection runs", async (t) => {
  const { a, b } = await pair(t, true);
  const r = await assessNormalizationPair(a, b);
  assert.equal(r.outcome, "drift", JSON.stringify(r));
  assert.ok(
    r.findings.some(
      (f) => f.caseId === "ascii-other-update" && f.outcome === "drift",
    ),
  );
});
test("repeatable uppercase stored prefixes compare equally while a spelling policy change produces drift", async (t) => {
  const { x, a, b } = await pair(t, false, true);
  const same = await assessNormalizationPair(a, b);
  assert.equal(same.outcome, "repeatable", JSON.stringify(same));
  x.flags.uppercase = false;
  const dir = join(x.dir, "c.artifacts");
  await (await import("node:fs/promises")).mkdir(dir, { mode: 0o700 });
  const third = await runNormalizationProbe(x.config, {}, dir, x.transport),
    path = join(x.dir, "c.json");
  await writeFile(path, JSON.stringify(third) + "\n", { mode: 0o600 });
  assert.equal((await assessNormalizationPair(a, path)).outcome, "drift");
});

test("matching malformed create success errors block assessment rather than becoming repeatable evidence", async (t) => {
  const { a, b } = await pair(t, false, false, "malformed");
  assert.equal((await assessNormalizationPair(a, b)).outcome, "blocked");
});

test("recovering an already clean capture preserves repeatability with the next capture", async (t) => {
  const { a, b } = await pair(t, false, false, [], true);
  const result = await assessNormalizationPair(a, b);
  assert.equal(result.outcome, "repeatable", JSON.stringify(result));
});
test("unavailable captures return explicit blocked findings for every scenario", async () => {
  const result = await assessNormalizationPair(
    "/nonexistent-plan14-first.json",
    "/nonexistent-plan14-second.json",
  );
  assert.equal(result.outcome, "blocked");
  assert.equal(result.findings.length, 36);
  assert.ok(result.findings.every((f) => f.outcome === "blocked"));
});
