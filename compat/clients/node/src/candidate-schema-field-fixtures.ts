import assert from "node:assert/strict";
import { readFile, writeFile } from "node:fs/promises";
import { join } from "node:path";
import { runCommand } from "./candidate-acceptance-process.ts";
import { ROOT } from "./versions.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import { ownerCommand, schemaOperation } from "./candidate-schema-runtime.ts";
/** Plan 28 fixture names; records in these objects go with their tables. */
export function fieldTypeNames(runId: string) {
  const prefix = "P28_" + runId.slice(0, 12);
  return {
    parent: prefix + "_Parent__c",
    item: prefix + "_Item__c",
    child: prefix + "_Child__c",
  };
}
type FieldSpec = [name: string, label: string, ...flags: string[]];
const itemFields = (parent: string): FieldSpec[] => [
  [
    "Quantity__c",
    "Quantity",
    "--type",
    "Number",
    "--precision",
    "18",
    "--scale",
    "2",
  ],
  ["Amount__c", "Amount", "--type", "Currency"],
  [
    "Score__c",
    "Score",
    "--type",
    "Percent",
    "--precision",
    "5",
    "--scale",
    "2",
  ],
  ["Due__c", "Due", "--type", "Date"],
  ["Starts__c", "Starts", "--type", "DateTime"],
  ["Done__c", "Done", "--type", "Checkbox"],
  [
    "Stage__c",
    "Stage",
    "--type",
    "Picklist",
    "--values",
    "Open,Closed",
    "--restricted",
    "--default",
    "Open",
  ],
  ["Tags__c", "Tags", "--type", "MultiselectPicklist", "--values", "A,B,C"],
  ["Email__c", "Contact email", "--type", "Email"],
  ["Phone__c", "Phone", "--type", "Phone"],
  ["Site__c", "Site", "--type", "Url"],
  ["Notes__c", "Notes", "--type", "LongTextArea", "--length", "1000"],
  ["Summary__c", "Summary", "--type", "TextArea"],
  ["Code__c", "Code", "--type", "Text", "--length", "20"],
  ["Account__c", "Parent", "--type", "Lookup", "--reference-to", "Account"],
  [
    "Keep__c",
    "Kept parent",
    "--type",
    "Lookup",
    "--reference-to",
    parent,
    "--delete-behavior",
    "Restrict",
  ],
  [
    "Serial__c",
    "Serial",
    "--type",
    "AutoNumber",
    "--display-format",
    "S-{000}",
    "--starting-number",
    "7",
  ],
];
export async function createFieldTypeFixtures(c: GateContext) {
  const tenant = c.tenants.first;
  const n = fieldTypeNames(c.runId);
  const object = async (
    name: string,
    label: string,
    plural: string,
    extra: string[] = [],
  ) =>
    schemaOperation(c, { Tenant: tenant, Kind: "object", Name: name }, [
      "schema",
      "object",
      "create",
      "--tenant",
      tenant,
      "--name",
      name,
      "--label",
      label,
      "--plural-label",
      plural,
      ...extra,
    ]);
  const field = async (obj: string, [name, label, ...flags]: FieldSpec) =>
    schemaOperation(
      c,
      { Tenant: tenant, Kind: "field", Object: obj, Name: name },
      [
        "schema",
        "field",
        "create",
        "--tenant",
        tenant,
        "--object",
        obj,
        "--name",
        name,
        "--label",
        label,
        ...flags,
      ],
    );
  await object(n.parent, "Typed parent", "Typed parents");
  await object(n.item, "Typed item", "Typed items", [
    "--name-type",
    "AutoNumber",
    "--display-format",
    "TI-{0000}",
  ]);
  await object(n.child, "Typed child", "Typed children");
  for (const spec of itemFields(n.parent)) await field(n.item, spec);
  await field(n.child, [
    "Parent__c",
    "Parent",
    "--type",
    "MasterDetail",
    "--reference-to",
    n.parent,
  ]);
  const definition = join(c.scratch, "stored-formula.json");
  await writeFile(
    definition,
    JSON.stringify({
      name: "Score_Formula__c",
      label: "Score formula",
      type: "Formula",
      formula: "Score__c * 2",
      returnType: "Percent",
    }),
    { mode: 0o600, flag: "wx" },
  );
  await schemaOperation(
    c,
    {
      Tenant: tenant,
      Kind: "stored",
      Object: n.item,
      Name: "Score_Formula__c",
    },
    [
      "schema",
      "field",
      "create",
      "--tenant",
      tenant,
      "--object",
      n.item,
      "--definition",
      definition,
    ],
  );
  await ownerCommand(c, [
    "schema",
    "currency",
    "set",
    "--tenant",
    tenant,
    "--code",
    "EUR",
  ]);
  c.env.CANDIDATE_FIELD_TYPES_OBJECT = n.item;
}
export async function runFieldTypesPython(c: GateContext): Promise<string[]> {
  const report = join(c.scratch, "field-types-python.json");
  const env: NodeJS.ProcessEnv = { ...c.env, CANDIDATE_STEP_REPORT: report };
  for (const key of [
    "MIGRATION_DATABASE_URL",
    "DATABASE_URL",
    "AUTH_DATABASE_URL",
    "TEST_DATABASE_URL",
    "TEST_RECORDS_URL",
    "TEST_IDENTITY_URL",
  ])
    delete env[key];
  const r = await runCommand(
    {
      executable: c.python,
      argv: ["scripts/candidate-field-types-python.py"],
      cwd: ROOT,
      env,
      privateOutput: () => {},
    },
    c.processes,
    "schema",
  );
  if (r.exitCode !== 0 || !r.complete) throw new Error("ASSERTION");
  const parsed = JSON.parse(await readFile(report, "utf8")) as {
    runId: string;
    tests: Array<{ id: string; outcome: string }>;
  };
  assert.equal(parsed.runId, c.runId);
  return parsed.tests.filter((t) => t.outcome === "passed").map((t) => t.id);
}
