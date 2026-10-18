import { isDeepStrictEqual } from "node:util";
import {
  validateInventory,
  type Inventory,
  type Value,
  type Persona,
  type Gap,
} from "./metadata-inventory.ts";
export interface MatrixRow {
  area: string;
  componentAlias: string;
  persona: Persona;
  property: string;
  state: "observed" | Gap["state"];
  value?: Value;
  values?: Value[];
  sourceRefs: string[];
  nextAction: string;
}
export interface Matrix {
  kind: "metadata-capability-matrix";
  rows: MatrixRow[];
  complete: boolean;
}
export function compileMetadataMatrix(i: Inventory): Matrix {
  validateInventory(i);
  const rows = new Map<string, MatrixRow>();
  for (const f of i.facts) {
    const area = f.area === "retrieve" ? "metadata" : f.area;
    const key = JSON.stringify([area, f.component, f.persona, f.property]);
    const row = rows.get(key);
    if (!row)
      rows.set(key, {
        area,
        componentAlias: f.component,
        persona: f.persona,
        property: f.property,
        state: "observed",
        value: f.value,
        sourceRefs: [f.sourceRef],
        nextAction: "No authorization inference; retain scoped observation",
      });
    else {
      if (!row.sourceRefs.includes(f.sourceRef))
        row.sourceRefs.push(f.sourceRef);
      const values = row.values ?? [row.value!];
      if (!values.some((x) => isDeepStrictEqual(x, f.value))) {
        row.state = "conflict";
        row.values = [...values, f.value];
        delete row.value;
        row.nextAction =
          "Resolve contradictory declarations before implementation";
      }
    }
  }
  const list = [
    ...rows.values(),
    ...i.gaps.map(
      (g) =>
        ({
          area: g.area,
          componentAlias: g.component,
          persona: g.persona,
          property: g.property,
          state: g.state,
          sourceRefs: [g.sourceRef],
          nextAction:
            g.state === "unrequested"
              ? "Choose exact optional scope if needed"
              : `Investigate ${g.code}; do not infer denial or absence`,
        }) as MatrixRow,
    ),
  ];
  for (const r of list) {
    r.sourceRefs.sort();
    r.values?.sort((a, b) =>
      JSON.stringify(a).localeCompare(JSON.stringify(b)),
    );
  }
  list.sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
  return {
    kind: "metadata-capability-matrix",
    rows: list,
    complete:
      i.outcome === "complete" && !list.some((x) => x.state === "conflict"),
  };
}
export function compareMetadataInventories(
  a: Inventory,
  b: Inventory,
): { outcome: "stable" | "drift" | "blocked"; differences: string[] } {
  try {
    validateInventory(a);
    validateInventory(b);
  } catch {
    return { outcome: "blocked", differences: ["Invalid inventory"] };
  }
  if (
    !isDeepStrictEqual(a.scope, b.scope) ||
    !isDeepStrictEqual(a.connectors, b.connectors) ||
    a.profileHash !== b.profileHash
  )
    return {
      outcome: "blocked",
      differences: ["Inventory scope/profile/toolchain differs"],
    };
  if (
    a.outcome !== "complete" ||
    b.outcome !== "complete" ||
    a.finishedAt > b.startedAt ||
    a.startedAt === b.startedAt
  )
    return {
      outcome: "blocked",
      differences: ["Complete distinct sequential inventories required"],
    };
  const x = compileMetadataMatrix(a),
    y = compileMetadataMatrix(b);
  if (!x.complete || !y.complete)
    return {
      outcome: "blocked",
      differences: ["Conflicting declarations require review"],
    };
  const operations = (i: Inventory) =>
    [...i.operations].sort((a, b) => a.id.localeCompare(b.id));
  const members = (i: Inventory) =>
    i.artifacts
      .map((x) => [x.component, x.path])
      .sort((a, b) => JSON.stringify(a).localeCompare(JSON.stringify(b)));
  const differences: string[] = [];
  if (!isDeepStrictEqual(x, y)) differences.push("Scoped facts or gaps differ");
  if (!isDeepStrictEqual(operations(a), operations(b)))
    differences.push("Operations differ");
  if (!isDeepStrictEqual(members(a), members(b)))
    differences.push("Retained artifact membership differs");
  return { outcome: differences.length ? "drift" : "stable", differences };
}
