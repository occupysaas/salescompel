import type { TargetInventory } from "../../src/target-inventory-model.ts";
import { readFileSync } from "node:fs";
import { Ajv } from "ajv";
import {
  known,
  makeCompleteInventory,
  makeItem,
} from "../target-inventory-fixtures.ts";
import { assessTargetInventory } from "../../src/target-inventory.ts";

const schemaPath = new URL(
  "../../../../../compat/schemas/target-inventory.schema.json",
  import.meta.url,
);

export const schema = () =>
  new Ajv({ allErrors: true }).compile<TargetInventory>(
    JSON.parse(readFileSync(schemaPath, "utf8")),
  );

export const assess = (value: unknown) =>
  assessTargetInventory(Buffer.from(JSON.stringify(value)));

export function commandInventory(pluginId: string | null): TargetInventory {
  const v = makeCompleteInventory("real");
  v.sections.cliPlugins.absence = null;
  v.sections.cliPlugins.items = [
    {
      ...makeItem(),
      id: "query-command",
      category: "cli-command",
      details: {
        command: known("data query"),
        flags: known([]),
        outputFormats: known(["json"]),
        target: known("org"),
        cliVersion: known("2.156.0"),
        pluginId: known(pluginId as string),
      },
    },
  ];
  return v;
}
