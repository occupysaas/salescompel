import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { join } from "node:path";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import type { ParsedResult } from "./candidate-acceptance-contract.ts";
import { stepResult } from "./candidate-acceptance-clients.ts";
import { candidateRequest } from "./candidate-acceptance-fixtures.ts";
import { ownerCommand } from "./candidate-schema-runtime.ts";
export async function verifySchemaRestart(
  c: GateContext,
): Promise<ParsedResult> {
  const saved = JSON.parse(
    await readFile(join(c.scratch, "schema-restart.json"), "utf8"),
  );
  const r = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/sobjects/${saved.object}/${saved.id}`,
  );
  assert.equal(r.status, 200);
  assert.equal(r.body?.Id, saved.id);
  assert.equal(r.body?.Name, saved.name);
  assert.equal(r.body?.Notes__c, "persistent");
  assert.deepEqual(
    (await candidateRequest(c, "admin", "/services/data/v65.0/sobjects")).body,
    saved.discovery,
  );
  const after = JSON.parse(
    await ownerCommand(
      c,
      ["schema", "inspect", "--tenant", c.tenants.first],
      "restart",
    ),
  );
  assert.equal(after.Revision, saved.revision);
  const typed = await candidateRequest(
    c,
    "admin",
    `/services/data/v65.0/sobjects/${saved.typed.object}/${saved.typed.id}`,
  );
  assert.equal(typed.status, 200);
  assert.deepEqual(typed.body, saved.typed.record);
  return stepResult(c, "restart", ["restart/schema", "restart/field-types"]);
}
