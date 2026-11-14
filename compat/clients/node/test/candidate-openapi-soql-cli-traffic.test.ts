import assert from "node:assert/strict";
import test from "node:test";
import { loadCandidateOpenAPI } from "../src/candidate-openapi-contract.ts";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import {
  aggregateParentPath,
  childDeepParentPath,
  childParentPath,
  childPlainAndParentLeaf,
  childRelationship,
  customParentAccount,
  customParentContact,
  nestedParentPath,
  parentPath,
} from "./helpers/candidate-openapi-soql-fixtures.ts";

const validator = async () =>
  createCandidateTrafficValidator(await loadCandidateOpenAPI());
const valid = (r: { requestIssues: string[]; responseIssues: string[] }) => {
  assert.deepEqual(r.requestIssues, []);
  assert.deepEqual(r.responseIssues, []);
};

// Mirrors @salesforce/plugin-data 5.1.16 searchSubColumnsRecursively: every
// entry reached through joinColumns must expose joinColumns as an array, or
// the CLI reads .length of undefined. Leaf entries carry an explicit [].
const cliSearchSubColumns = (parent: unknown): string[] => {
  const column = parent as { columnName?: unknown; joinColumns?: unknown };
  const name = String(column.columnName);
  if (!Array.isArray(column.joinColumns))
    throw new Error(`joinColumns must be an array for ${name}`);
  return column.joinColumns.length
    ? column.joinColumns.flatMap((child) =>
        cliSearchSubColumns(child).map((sub) => `${name}.${sub}`),
      )
    : [name];
};

const cliColumnNames = (metadata: unknown[]) => {
  const names: string[] = [];
  for (const raw of metadata) {
    const column = raw as { columnName: string; joinColumns?: unknown[] };
    if (Array.isArray(column.joinColumns) && column.joinColumns.length)
      names.push(...cliSearchSubColumns(column));
  }
  return names;
};

test("accepts the pinned CLI recursive column traversal", async () => {
  const v = await validator();
  for (const sample of [
    parentPath(),
    nestedParentPath(),
    childRelationship(),
    childPlainAndParentLeaf(),
    childParentPath(),
    childDeepParentPath(),
    aggregateParentPath(),
    customParentAccount(),
    customParentContact(),
  ]) {
    valid(v.validate(sample));
    const body = JSON.parse(
      Buffer.from(sample.response.body).toString("utf8"),
    ) as { columnMetadata: unknown[] };
    assert.ok(cliColumnNames(body.columnMetadata).length > 0);
  }
});
