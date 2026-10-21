import { Ajv } from "ajv";
import { readFileSync } from "node:fs";
import { createHash } from "node:crypto";
import { parseDocument, isAlias, isMap, isSeq, isScalar } from "yaml";
import {
  SECTION_IDS,
  type TargetInventory,
  type InventoryReport,
  type InventoryIssue,
  type IssueCode,
  type SectionId,
  type RepositoryReference,
  type ActionCode,
} from "./target-inventory-model.ts";

export const MAX_INVENTORY_BYTES = 1024 * 1024;
const check = new Ajv({ allErrors: false }).compile<TargetInventory>(
  JSON.parse(
    readFileSync(
      new URL("../../../schemas/target-inventory.schema.json", import.meta.url),
      "utf8",
    ),
  ),
);
const safeId = (value: unknown): value is string =>
  typeof value === "string" && /^[a-z][a-z0-9-]{0,63}$/.test(value);

/** Deliberately separate from provenance locators: these are local repository links only. */
export function safeInventoryReference(ref: RepositoryReference): boolean {
  return (
    /^(docs\/(compat|plans|superpowers)\/|locales\/|api\/)/.test(ref.path) &&
    !ref.path.includes("\\") &&
    !ref.path.includes("%") &&
    !ref.path.includes("\0") &&
    !ref.path.includes(":") &&
    ref.path.split("/").every((p) => p !== "." && p !== ".." && p !== "")
  );
}

export function parseInventoryDocument(bytes: Uint8Array): unknown {
  if (bytes.length > MAX_INVENTORY_BYTES) throw new Error("input-too-large");
  const text = new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  const doc = parseDocument(text, { uniqueKeys: true, logLevel: "silent" });
  if (doc.errors.length) throw new Error("parse-invalid");
  // Bound the AST (including alias expansion) before toJS or schema recursion.
  let nodes = 0;
  const stack = new Set<unknown>();
  function visit(node: unknown, depth: number): void {
    if (depth > 32 || ++nodes > 10000 || stack.has(node))
      throw new Error("parse-invalid");
    if (!node || typeof node !== "object") return;
    stack.add(node);
    if (isAlias(node)) visit(node.resolve(doc), depth + 1);
    else if (isMap(node))
      for (const pair of node.items) {
        if (!isScalar(pair.key) || typeof pair.key.value !== "string")
          throw new Error("parse-invalid");
        visit(pair.key, depth + 1);
        visit(pair.value, depth + 1);
      }
    else if (isSeq(node)) for (const item of node.items) visit(item, depth + 1);
    stack.delete(node);
  }
  visit(doc.contents, 0);
  return doc.toJS({ maxAliasCount: 50 });
}
const calendarDate = (s: string) => {
  const d = new Date(s + "T00:00:00Z");
  return Number.isFinite(d.getTime()) && d.toISOString().slice(0, 10) === s;
};
function ordered(issues: InventoryIssue[]): InventoryIssue[] {
  const key = (i: InventoryIssue) =>
    [
      String(
        i.section === undefined ? -1 : SECTION_IDS.indexOf(i.section),
      ).padStart(2, "0"),
      i.itemId ?? "",
      i.code,
      i.ownerId ?? "",
      i.nextAction ?? "",
    ].join("\0");
  return [...new Map(issues.map((i) => [key(i), i])).entries()]
    .sort(([a], [b]) => (a < b ? -1 : a > b ? 1 : 0))
    .map(([, i]) => i);
}
export function assessTargetInventory(bytes: Uint8Array): InventoryReport {
  const report: InventoryReport = {
    schemaVersion: 1,
    inputDigest: createHash("sha256").update(bytes).digest("hex"),
    validation: "invalid",
    readiness: "incomplete",
    sections: [],
    issues: [],
    gaps: [],
  };
  const fail = (code: IssueCode) => {
    report.issues = [{ code }];
    return report;
  };
  if (bytes.length > MAX_INVENTORY_BYTES) return fail("input-too-large");
  try {
    new TextDecoder("utf-8", { fatal: true }).decode(bytes);
  } catch {
    return fail("encoding-invalid");
  }
  let data: unknown;
  try {
    data = parseInventoryDocument(bytes);
  } catch {
    return fail("parse-invalid");
  }
  if (data && typeof data === "object" && !Array.isArray(data)) {
    const meta = data as Record<string, unknown>;
    if (meta.classification === "synthetic" || meta.classification === "real")
      report.classification = meta.classification;
    if (report.classification === "synthetic")
      report.readiness = "synthetic-only";
    if (safeId(meta.inventoryId)) report.inventoryId = meta.inventoryId;
    if (Number.isSafeInteger(meta.revision) && Number(meta.revision) > 0)
      report.revision = Number(meta.revision);
  }
  if (!check(data)) return fail("schema-invalid");
  const v = data;
  const issue = (
    code: IssueCode,
    section?: SectionId,
    itemId?: string,
    ownerId?: string,
    nextAction?: ActionCode,
  ) =>
    report.issues.push({
      code,
      ...(section ? { section } : {}),
      ...(itemId ? { itemId } : {}),
      ...(ownerId ? { ownerId } : {}),
      ...(nextAction ? { nextAction } : {}),
    });
  const gap = (
    code: IssueCode,
    section: SectionId,
    ownerId: string,
    nextAction: ActionCode,
    itemId?: string,
  ) =>
    report.gaps.push({
      code,
      section,
      ownerId,
      nextAction,
      ...(itemId ? { itemId } : {}),
    });
  const sourceIds = new Set<string>(),
    itemIds = new Set<string>(),
    questionIds = new Set<string>();
  if (!calendarDate(v.date)) issue("date-invalid");
  for (const source of v.sources) {
    if (sourceIds.has(source.id)) issue("duplicate-id");
    sourceIds.add(source.id);
    if (!calendarDate(source.date)) issue("date-invalid");
  }
  for (const section of Object.values(v.sections))
    for (const item of section.items) {
      if (itemIds.has(item.id)) issue("duplicate-id");
      itemIds.add(item.id);
    }
  const sourceRefs = (refs: string[], section: SectionId, itemId?: string) => {
    if (refs.some((id) => !sourceIds.has(id)))
      issue("dangling-reference", section, itemId);
  };
  const edges = new Map<string, string[]>();
  for (const id of SECTION_IDS) {
    const s = v.sections[id];
    report.sections.push({ id, status: s.status });
    sourceRefs(s.evidenceRefs, id);
    if (s.status !== "complete")
      gap("unknown-section", id, s.ownerId, "collect");
    if (!s.evidenceRefs.length)
      gap("missing-evidence", id, s.ownerId, "collect");
    if (s.status === "complete" && s.questions.length)
      issue("completion-contradiction", id);
    for (const q of s.questions) {
      if (questionIds.has(q.id)) issue("duplicate-id", id);
      questionIds.add(q.id);
      gap("pending-question", id, q.ownerId, q.nextAction);
    }
    if (s.absence) {
      sourceRefs(s.absence.evidenceRefs, id);
      if (
        s.items.length ||
        s.status !== "complete" ||
        !s.absence.evidenceRefs.length
      )
        issue("absence-invalid", id);
    }
    if (s.status === "complete" && !s.items.length && !s.absence)
      issue("absence-invalid", id);
    for (const item of s.items) {
      sourceRefs(item.provenanceRefs, id, item.id);
      for (const fact of Object.values(item.facts)) {
        sourceRefs(fact.evidenceRefs, id, item.id);
        if (
          (fact.status === "documented" && !fact.evidenceRefs.length) ||
          (fact.status === "unknown" && fact.evidenceRefs.length)
        )
          issue("required-information-missing", id, item.id);
      }
      if (
        item.disposition === "required" &&
        (!item.provenanceRefs.length ||
          item.behavior.status === "unknown" ||
          item.acceptance.status === "unknown" ||
          item.milestone.status === "unknown")
      )
        issue("required-information-missing", id, item.id);
      if (!item.provenanceRefs.length)
        gap("missing-evidence", id, item.ownerId, "collect", item.id);
      if (item.decision) {
        gap(
          "pending-question",
          id,
          item.ownerId,
          item.decision.nextAction,
          item.id,
        );
        if (s.status === "complete")
          issue("completion-contradiction", id, item.id);
      }
      if (
        item.category === "cli-command" &&
        item.details.pluginId.status === "known" &&
        item.details.pluginId.value !== null
      ) {
        const pluginId = item.details.pluginId.value;
        const plugin = Object.values(v.sections)
          .flatMap((section) => section.items)
          .find((candidate) => candidate.id === pluginId);
        if (!plugin || plugin.category !== "plugin")
          issue("dangling-reference", id, item.id);
      }
      if (item.disposition === "unknown") {
        if (!item.decision) issue("decision-missing", id, item.id);
        gap(
          "unknown-disposition",
          id,
          item.ownerId,
          item.decision?.nextAction ?? "review-scope",
          item.id,
        );
        if (s.status === "complete")
          issue("completion-contradiction", id, item.id);
      }
      // Only knowledge unknowns affect inventory completeness; unknown runtime facts do not.
      const knowledge = [
        item.behavior,
        item.acceptance,
        item.milestone,
        ...Object.values(item.details),
      ];
      for (const k of knowledge)
        if (k.status === "unknown") {
          gap("unknown-detail", id, item.ownerId, k.nextAction, item.id);
          if (s.status === "complete")
            issue("completion-contradiction", id, item.id);
        }
      if (item.dependencies.some((dep) => !itemIds.has(dep)))
        issue("dangling-reference", id, item.id);
      edges.set(item.id, item.dependencies);
      if (item.repositoryRefs.some((ref) => !safeInventoryReference(ref)))
        issue("repository-reference-invalid", id, item.id);
    }
  }
  const done = new Set<string>(),
    active = new Set<string>();
  function cycle(id: string): boolean {
    if (active.has(id)) return true;
    if (done.has(id)) return false;
    active.add(id);
    for (const dep of edges.get(id) ?? []) if (cycle(dep)) return true;
    active.delete(id);
    done.add(id);
    return false;
  }
  for (const id of edges.keys())
    if (cycle(id)) {
      issue("dependency-cycle");
      break;
    }
  report.issues = ordered(report.issues);
  report.gaps = ordered(report.gaps);
  report.validation = report.issues.length ? "invalid" : "valid";
  if (report.classification !== "synthetic")
    report.readiness =
      report.validation === "valid" && !report.gaps.length
        ? "ready-for-review"
        : "incomplete";
  return report;
}
export function repositoryReferences(
  inventory: TargetInventory,
): RepositoryReference[] {
  return SECTION_IDS.flatMap((id) =>
    inventory.sections[id].items.flatMap((item) =>
      item.repositoryRefs.map((ref) => ({ ...ref })),
    ),
  );
}
