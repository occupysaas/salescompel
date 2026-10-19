import test from "node:test";
import assert from "node:assert/strict";
import { mkdir, writeFile, readFile, rm, readdir } from "node:fs/promises";
import { join } from "node:path";
import { bodyFixture } from "./external-id-body-test-server.ts";
import { runBodyProbe } from "../src/external-id-body-probe.ts";
import { assessBodyPair } from "../src/external-id-body-assess.ts";
import { validateBodyReport } from "../src/external-id-body-contract.ts";
import type { TestContext } from "node:test";
export async function pair(t: TestContext, mode = "valid") {
  const x = await bodyFixture(t);
  let patches = 0;
  const lateScopes = new Set<string>();
  const transport = {
    async request(path: string, init: RequestInit) {
      if (init.method === "PATCH") patches++;
      if (
        mode === "late" &&
        patches % 16 === 0 &&
        patches > 0 &&
        path.includes("query?") &&
        decodeURIComponent(path).includes("existing-identical")
      ) {
        const row = [...x.rows.values()].find((r) =>
          r.Name.includes("existing-identical"),
        );
        if (row && !lateScopes.has(row.Name)) {
          lateScopes.add(row.Name);
          x.rows.set("001999999999999999", {
            ...row,
            Id: "001999999999999999",
          });
        }
      }
      const r = structuredClone(await x.transport.request(path, init));
      if (init.method === "PATCH" && (r.status === 204 || r.status === 201)) {
        const key = decodeURIComponent(path.split("/").at(-1)!);
        const row = [...x.rows.values()].find((v) => v.Compat_Key__c === key)!;
        return {
          ...r,
          status: r.status === 204 ? 200 : 201,
          bodyPresent: true,
          body: {
            id: row.Id,
            success: true,
            created: r.status === 201,
            errors: [],
          },
        };
      }
      if (init.method === "PATCH" && mode === "wrong")
        return {
          ...r,
          status: 200,
          bodyPresent: true,
          body: { success: true },
        };
      if (init.method === "PATCH" && mode === "auth")
        return { ...r, status: 401 };
      if (init.method === "PATCH" && mode === "nonJson")
        return { ...r, body: { nonJson: "unparsed" } };
      return r;
    },
  };
  if (mode === "hidden") x.flags.hiddenInsert = true;
  const reports: import("../src/external-id-body-contract.ts").BodyReport[] =
    [];
  const paths: string[] = [];
  for (let i = 0; i < 2; i++) {
    const p = join(x.dir, `r${i}.json`),
      dir = p.replace(".json", ".artifacts");
    await mkdir(dir, { mode: 0o700 });
    const r = await runBodyProbe(x.config, {}, dir, transport);
    reports.push(r);
    paths.push(p);
    await writeFile(p, JSON.stringify(r) + "\n", { mode: 0o600 });
  }
  return {
    x,
    reports,
    paths,
    async save(i: number) {
      await writeFile(paths[i], JSON.stringify(reports[i]) + "\n");
      await writeFile(
        join(paths[i].replace(".json", ".artifacts"), "report.json"),
        JSON.stringify(reports[i]) + "\n",
      );
    },
  };
}
test("ready requires exact expected behavior across all 16 cases", async (t) => {
  const p = await pair(t);
  assert.equal(p.reports[0].observations.length, 16);
  assert.equal(
    (await assessBodyPair(...(p.paths as [string, string]))).outcome,
    "ready",
  );
});
for (const mode of ["wrong", "hidden", "late", "auth", "nonJson"])
  test(`${mode} responses cannot pass`, async (t) => {
    const p = await pair(t, mode);
    assert.equal(
      (await assessBodyPair(...(p.paths as [string, string]))).outcome,
      mode === "wrong" || mode === "hidden" || mode === "late"
        ? "different"
        : "blocked",
    );
  });
test("missing corrupt or stale evidence blocks", async (t) => {
  const p = await pair(t);
  const a = join(
    p.paths[0].replace(".json", ".artifacts"),
    p.reports[0].observations[0].artifact,
  );
  await writeFile(a, "{}\n");
  assert.equal(
    (await assessBodyPair(...(p.paths as [string, string]))).outcome,
    "blocked",
  );
});
test("pair requires distinct clean sequential runs", async (t) => {
  const p = await pair(t);
  assert.equal(
    (await assessBodyPair(p.paths[0], p.paths[0])).outcome,
    "blocked",
  );
  p.reports[1].startedAt = p.reports[0].startedAt;
  await p.save(1);
  assert.equal(
    (await assessBodyPair(...(p.paths as [string, string]))).outcome,
    "blocked",
  );
});
test("UTC evidence timestamps reject normalized impossible dates", async (t) => {
  const p = await pair(t);
  p.reports[0].startedAt = "2026-02-31T00:00:00Z";
  assert.throws(() => validateBodyReport(p.reports[0]));
});

test("invalid matrix, bindings, state reads, cleanup and proof block independently", async (t) => {
  const p = await pair(t),
    dir = p.paths[0].replace(".json", ".artifacts");
  const original = new Map<string, string>();
  for (const name of await readdir(dir))
    original.set(join(dir, name), await readFile(join(dir, name), "utf8"));
  original.set(p.paths[0], await readFile(p.paths[0], "utf8"));
  const { bodyCleanupProofName, bodyDigest } =
    await import("../src/external-id-body-recovery.ts");
  for (const mutation of [
    "missing-case",
    "duplicate",
    "null-omitted",
    "profile",
    "clients",
    "manifest",
    "state",
    "before",
    "read",
    "absence",
    "proof",
    "ownership",
    "journal",
  ]) {
    await t.test(mutation, async () => {
      const r = structuredClone(p.reports[0]);
      const o = r.observations[4];
      if (mutation === "missing-case") r.observations.pop();
      if (mutation === "duplicate") r.observations[15] = r.observations[14];
      if (mutation === "null-omitted") delete o.request.Compat_Key__c;
      if (mutation === "profile") r.config.profileHash = "a".repeat(64);
      if (mutation === "clients") (r.connectors as any).jsforce = "wrong";
      if (mutation === "manifest") r.manifestHash = "a".repeat(64);
      if (mutation === "state") {
        const path = join(dir, o.stateArtifact),
          s = JSON.parse(await readFile(path, "utf8"));
        s.after.exactReads = [];
        const bytes = JSON.stringify(s) + "\n";
        await writeFile(path, bytes);
        o.stateSha256 = bodyDigest(bytes);
      }
      if (mutation === "before")
        await writeFile(join(dir, `before-${o.caseId}.json`), "{}");
      if (mutation === "read")
        await writeFile(
          join(
            dir,
            [...original.keys()]
              .map((x) => x.split("/").at(-1)!)
              .find((x) => x.startsWith("read-"))!,
          ),
          "{}",
        );
      if (mutation === "absence")
        await rm(join(dir, r.cleanup.records[0].absenceArtifact));
      if (mutation === "proof")
        await writeFile(join(dir, bodyCleanupProofName(r.cleanup)), "{}");
      if (mutation === "ownership") {
        for (const name of await readdir(dir))
          if (name.startsWith("cleanup-state-")) {
            const path = join(dir, name),
              s = JSON.parse(await readFile(path, "utf8"));
            for (const row of s.records) row.Name = "foreign";
            await writeFile(path, JSON.stringify(s));
          }
      }
      if (mutation === "journal")
        await writeFile(join(dir, "journal.jsonl"), "");
      await writeFile(p.paths[0], JSON.stringify(r));
      await writeFile(join(dir, "report.json"), JSON.stringify(r));
      assert.equal(
        (await assessBodyPair(p.paths[0], p.paths[1])).outcome,
        "blocked",
        mutation,
      );
    });
    for (const [path, bytes] of original)
      await writeFile(path, bytes, { mode: 0o600 });
  }
});

test("malformed positive control is blocked even when repeatable", async (t) => {
  const p = await pair(t);
  const { bodyDigest } = await import("../src/external-id-body-recovery.ts");
  for (let i = 0; i < 2; i++) {
    const r = p.reports[i],
      o = r.observations[0],
      dir = p.paths[i].replace(".json", ".artifacts"),
      path = join(dir, o.artifact);
    const wire = JSON.parse(await readFile(path, "utf8"));
    delete wire.body.success;
    delete (o.body as any).success;
    const bytes = JSON.stringify(wire) + "\n";
    await writeFile(path, bytes);
    o.sha256 = bodyDigest(bytes);
    await p.save(i);
    await writeFile(join(dir, "report.json"), JSON.stringify(r));
  }
  assert.equal(
    (await assessBodyPair(p.paths[0], p.paths[1])).outcome,
    "blocked",
  );
});

test("a validly shaped but stale redundant read cannot hide behind another read", async (t) => {
  const p = await pair(t),
    dir = p.paths[0].replace(".json", ".artifacts");
  const name = (await readdir(dir))
    .filter((n) => n.startsWith("read-existing-identical-"))
    .at(-1)!;
  const path = join(dir, name),
    value = JSON.parse(await readFile(path, "utf8"));
  value.response.body.Name = "foreign";
  await writeFile(path, JSON.stringify(value));
  assert.equal(
    (await assessBodyPair(p.paths[0], p.paths[1])).outcome,
    "blocked",
  );
});

test("cleanup proof references and absence digests are mandatory bindings", async (t) => {
  const p = await pair(t),
    r = p.reports[0],
    dir = p.paths[0].replace(".json", ".artifacts");
  const { bodyCleanupProofName } =
    await import("../src/external-id-body-recovery.ts");
  const { validateBodyCleanupEvidence } =
    await import("../src/external-id-body-assess.ts");
  const path = join(dir, bodyCleanupProofName(r.cleanup)),
    bytes = await readFile(path, "utf8"),
    proof = JSON.parse(bytes);
  proof.discoveryArtifacts[0].unexpected = true;
  await writeFile(path, JSON.stringify(proof));
  await t.test("reference has exact shape", async () =>
    assert.equal(
      (await assessBodyPair(p.paths[0], p.paths[1])).outcome,
      "blocked",
    ),
  );
  await writeFile(path, bytes);
  const cleanup = structuredClone(r.cleanup);
  cleanup.records[0].absenceSha256 = "";
  proof.cleanup = cleanup;
  delete proof.discoveryArtifacts[0].unexpected;
  await writeFile(
    join(dir, bodyCleanupProofName(cleanup)),
    JSON.stringify(proof),
    { mode: 0o600 },
  );
  await t.test(
    "absence digest cannot be omitted by recovery pointer",
    async () =>
      assert.rejects(validateBodyCleanupEvidence(r.config, dir, cleanup)),
  );
});

test("a permanently latched run cannot authorize assessment or another capture", async (t) => {
  const p = await pair(t),
    dir = p.paths[0].replace(".json", ".artifacts");
  await writeFile(
    join(dir, "identity-blocked.json"),
    JSON.stringify({ blocked: true }),
    { mode: 0o600 },
  );
  assert.equal(
    (await assessBodyPair(p.paths[0], p.paths[1])).outcome,
    "blocked",
  );
});
