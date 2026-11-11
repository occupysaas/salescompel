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
  x.flags.originalHypothesis = mode === "original";
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
      if (init.method === "PATCH" && (r.status === 200 || r.status === 201)) {
        const key = decodeURIComponent(path.split("/").at(-1)!);
        const row = [...x.rows.values()].find((v) => v.Compat_Key__c === key)!;
        const isNull = JSON.parse(String(init.body)).Compat_Key__c === null;
        if (isNull && mode === "null-path-key") row.Compat_Key__c = null;
        return {
          ...r,
          status:
            isNull && mode === "null-status"
              ? r.status === 200
                ? 201
                : 200
              : r.status,
          bodyPresent: true,
          body: {
            id: row.Id,
            success: true,
            created:
              isNull && mode === "null-created"
                ? r.status !== 201
                : r.status === 201,
            errors: isNull && mode === "null-errors" ? ["unexpected"] : [],
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
      if (init.method === "PATCH" && r.status === 400) {
        const errors = r.body as Array<Record<string, unknown>>;
        if (mode === "fields-empty") errors[0].fields = [];
        if (mode === "fields-null") errors[0].fields = null;
        if (mode === "fields-string") errors[0].fields = "Compat_Key__c";
        if (mode === "multiple-errors") errors.push({ ...errors[0] });
        if (mode === "empty-errors") errors.length = 0;
        if (mode === "missing-message") delete errors[0].message;
        if (mode === "nonstring-message") errors[0].message = 42;
        if (mode === "nonstring-code") errors[0].errorCode = 42;
        if (mode === "extra-error-property") errors[0].extra = true;
        if (
          mode === "wrong-object-code" &&
          errors[0].errorCode === "JSON_PARSER_ERROR"
        )
          errors[0].errorCode = "INVALID_FIELD";
        if (
          mode === "wrong-scalar-code" &&
          errors[0].errorCode === "INVALID_FIELD"
        )
          errors[0].errorCode = "JSON_PARSER_ERROR";
      }
      return r;
    },
  };
  if (mode === "hidden") x.flags.hiddenInsert = true;
  if (mode === "mutate") x.flags.mutateRejected = true;
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
test("observed null success and object parser errors are ready under v2 across all 16 cases", async (t) => {
  const p = await pair(t);
  assert.equal(p.reports[0].observations.length, 16);
  const result = await assessBodyPair(p.paths[0], p.paths[1]);
  assert.equal(result.outcome, "ready");
  assert.equal(result.contractVersion, "plan11-observed-body-validation-v2");
  assert.equal(result.findings.length, 16);
  assert.ok(result.findings.every((f) => f.outcome === "ready"));
  assert.equal(
    (await assessBodyPair(...(p.paths as [string, string]))).outcome,
    "ready",
  );
});

test("the original universal rejection hypothesis is different under v2", async (t) => {
  const p = await pair(t, "original");
  const result = await assessBodyPair(p.paths[0], p.paths[1]);
  assert.equal(result.outcome, "different");
  assert.deepEqual(
    result.findings
      .filter((f) => f.outcome === "different")
      .map((f) => f.caseId),
    ["existing-null", "existing-object", "missing-null", "missing-object"],
  );
});
for (const mode of [
  "fields-empty",
  "fields-null",
  "fields-string",
  "multiple-errors",
  "empty-errors",
  "missing-message",
  "nonstring-message",
  "nonstring-code",
  "extra-error-property",
])
  test(`${mode} rejection envelope blocks v2`, async (t) => {
    const p = await pair(t, mode);
    const result = await assessBodyPair(p.paths[0], p.paths[1]);
    assert.equal(result.outcome, "blocked");
    assert.equal(result.contractVersion, "plan11-observed-body-validation-v2");
    assert.ok(
      result.findings.some((f) => f.reasons.includes("MALFORMED_RESPONSE")),
    );
  });
for (const mode of [
  "wrong-object-code",
  "wrong-scalar-code",
  "null-path-key",
  "null-created",
  "null-errors",
  "null-status",
])
  test(`${mode} is different under v2`, async (t) => {
    const p = await pair(t, mode);
    assert.equal(
      (await assessBodyPair(p.paths[0], p.paths[1])).outcome,
      "different",
    );
  });

for (const mode of ["wrong", "hidden", "mutate", "late", "auth", "nonJson"])
  test(`${mode} responses cannot pass`, async (t) => {
    const p = await pair(t, mode);
    assert.equal(
      (await assessBodyPair(...(p.paths as [string, string]))).outcome,
      mode === "wrong" ||
        mode === "hidden" ||
        mode === "mutate" ||
        mode === "late"
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
