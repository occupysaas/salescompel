import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import assert from "node:assert/strict";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import {
  candidateRequest,
  createFixture,
} from "./candidate-acceptance-fixtures.ts";
import type { ParsedResult, GateID } from "./candidate-acceptance-contract.ts";
export const stepResult = (
  c: GateContext,
  suite: GateID,
  ids: string[],
): ParsedResult => ({
  schemaVersion: 1,
  runId: c.runId,
  suite,
  complete: true,
  exitCode: 0,
  tests: ids.map((id) => ({ id, outcome: "passed" })),
});
export function assertRestartBaseline(runId: string, before: any) {
  const name = "p15-" + runId + "-restart";
  assert.match(before.account, /^[A-Za-z0-9]{18}$/);
  assert.match(before.contact, /^[A-Za-z0-9]{18}$/);
  assert.equal(before.a.Id, before.account);
  assert.equal(before.a.Name, name);
  assert.equal(before.a.Compat_Text__c, "retained text");
  assert.equal(before.a.Compat_Key__c, name);
  assert.equal(before.b.Id, before.contact);
  assert.equal(before.b.FirstName, "Restart");
  assert.equal(before.b.LastName, name);
  assert.equal(before.b.AccountId, before.account);
}
export async function seedRestartFixture(c: GateContext) {
  const name = "p15-" + c.runId + "-restart",
    account = await createFixture(
      c,
      "restart-account",
      "Account",
      { Name: name, Compat_Text__c: "retained text", Compat_Key__c: name },
      [name],
    ),
    contact = await createFixture(
      c,
      "restart-contact",
      "Contact",
      { FirstName: "Restart", LastName: name, AccountId: account },
      [name],
    );
  const before = {
    account,
    contact,
    a: (
      await candidateRequest(
        c,
        "admin",
        `/services/data/v65.0/sobjects/Account/${account}`,
      )
    ).body,
    b: (
      await candidateRequest(
        c,
        "admin",
        `/services/data/v65.0/sobjects/Contact/${contact}`,
      )
    ).body,
  };
  assertRestartBaseline(c.runId, before);
  await writeFile(join(c.scratch, "restart.json"), JSON.stringify(before), {
    mode: 0o600,
    flag: "wx",
  });
  await verifyRestartFixture(c);
}
export async function verifyRestartFixture(
  c: GateContext,
): Promise<ParsedResult> {
  const before = JSON.parse(
    await readFile(join(c.scratch, "restart.json"), "utf8"),
  );
  assertRestartBaseline(c.runId, before);
  for (const [object, id, key] of [
    ["Account", before.account, "a"],
    ["Contact", before.contact, "b"],
  ]) {
    const r = await candidateRequest(
      c,
      "admin",
      `/services/data/v65.0/sobjects/${object}/${id}`,
    );
    assert.equal(r.status, 200);
    assert.deepEqual(r.body, before[key]);
  }
  const q = await candidateRequest(
    c,
    "admin",
    "/services/data/v65.0/query?q=" +
      encodeURIComponent(
        `SELECT Id,LastName,Account.Name FROM Contact WHERE AccountId = '${before.account}'`,
      ),
  );
  assert.equal(q.status, 200);
  assert.equal(q.body.records.length, 1);
  assert.equal(q.body.records[0].Id, before.contact);
  assert.equal(q.body.records[0].Account.Name, before.a.Name);
  assert.equal(
    (
      await candidateRequest(
        c,
        "other",
        `/services/data/v65.0/sobjects/Account/${before.account}`,
      )
    ).status,
    404,
  );
  const restricted = await candidateRequest(
    c,
    "restricted",
    "/services/data/v65.0/query?q=" +
      encodeURIComponent("SELECT Compat_Text__c FROM Account"),
  );
  assert.equal(restricted.status, 400);
  return stepResult(c, "restart", [
    "restart/identity",
    "restart/values",
    "restart/policy",
  ]);
}
export function parseCLIResult(text: string) {
  const r = JSON.parse(text);
  if (r?.status !== 0 || !r.result || typeof r.result !== "object")
    throw new Error("RESULT");
  return r.result;
}
export async function runNativeCLI(c: GateContext): Promise<ParsedResult> {
  const { runCommand } = await import("./candidate-acceptance-process.ts");
  const { ROOT, SF_BIN } = await import("./versions.ts");
  const { recordFixtureEvent } =
    await import("./candidate-acceptance-fixtures.ts");
  const steps: string[] = [];
  const alias = "p15-" + c.runId;
  const cli = async (args: string[]) => {
    const r = await runCommand(
      {
        executable: process.execPath,
        argv: [SF_BIN, ...args, "--json"],
        cwd: c.scratch,
        env: { ...c.env, SF_ACCESS_TOKEN: c.tokens.admin },
        timeout: 60000,
      },
      c.processes,
      "cli",
    );
    if (r.exitCode !== 0 || !r.complete) throw new Error("PROCESS");
    return parseCLIResult(await readFile(r.resultPath, "utf8"));
  };
  await cli([
    "org",
    "login",
    "access-token",
    "--instance-url",
    c.url,
    "--alias",
    alias,
    "--no-prompt",
  ]);
  steps.push("cli/login");
  const common = [
    "--target-org",
    alias,
    "--api-version",
    "65.0",
    "--sobject",
    "Account",
  ];
  const name = "p15-" + c.runId + "-cli";
  const e = {
    schemaVersion: 1 as const,
    runId: c.runId,
    operationId: "cli-create",
    principal: "admin" as const,
    object: "Account" as const,
    names: [name, name + "-updated"],
    kind: "intent" as const,
  };
  await recordFixtureEvent(c.fixtureJournal, e);
  const created = await cli([
    "data",
    "create",
    "record",
    ...common,
    "--values",
    "Name=" + name,
  ]);
  if (!created.success || typeof created.id !== "string")
    throw new Error("ASSERTION");
  const id = created.id;
  await recordFixtureEvent(c.fixtureJournal, { ...e, kind: "known", id });
  steps.push("cli/create");
  const get = () =>
    cli(["data", "get", "record", ...common, "--record-id", id]);
  assert.equal((await get()).Name, name);
  steps.push("cli/read");
  await cli([
    "data",
    "update",
    "record",
    ...common,
    "--record-id",
    id,
    "--values",
    "Name=" + name + "-updated",
  ]);
  assert.equal((await get()).Name, name + "-updated");
  steps.push("cli/update");
  let queryFailed = false;
  try {
    const q = await cli([
      "data",
      "query",
      "--target-org",
      alias,
      "--api-version",
      "65.0",
      "--query",
      `SELECT Id,Name FROM Account WHERE Id = '${id}'`,
    ]);
    assert.equal(q.records.length, 1);
    assert.equal(q.records[0].Id, id);
    assert.equal(q.records[0].Name, name + "-updated");
    steps.push("cli/query");
  } catch {
    queryFailed = true;
  }
  await cli(["data", "delete", "record", ...common, "--record-id", id]);
  steps.push("cli/delete");
  assert.equal(
    (
      await candidateRequest(
        c,
        "admin",
        `/services/data/v65.0/sobjects/Account/${id}`,
      )
    ).status,
    404,
  );
  steps.push("cli/absence");
  const result = stepResult(c, "cli", steps);
  if (queryFailed) {
    result.exitCode = 1;
    result.tests.push({ id: "cli/query", outcome: "failed" });
  }
  return result;
}
