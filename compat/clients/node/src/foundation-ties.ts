import { isDeepStrictEqual } from "node:util";
import { normalize, type Result } from "./core.ts";

const route = "/services/data/v65.0/query/{queryLocator}";
const locator = /^\/services\/data\/v65\.0\/query\/[^/?#]+$/;

function canonical(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(canonical).join(",")}]`;
  if (value && typeof value === "object")
    return `{${Object.entries(value)
      .sort(([a], [b]) => a.localeCompare(b))
      .map(([k, v]) => `${JSON.stringify(k)}:${canonical(v)}`)
      .join(",")}}`;
  return JSON.stringify(value);
}

function traversal(result: Result): {
  issues: string[];
  records: unknown[];
  pages: number[];
} {
  const issues: string[] = [];
  const raw = result.observations as Record<string, any>;
  const keys = Object.keys(raw ?? {}).filter((key) => key.startsWith("ties:"));
  const numbers = keys.map((key) => Number(key.slice(5)));
  if (
    !keys.length ||
    keys.length > 32 ||
    numbers.some((n) => !Number.isInteger(n) || n < 0) ||
    !isDeepStrictEqual(
      [...numbers].sort((a, b) => a - b),
      Array.from({ length: keys.length }, (_, i) => i),
    )
  )
    return {
      issues: ["Tied traversal page labels are missing or incomplete"],
      records: [],
      pages: [],
    };
  const expected = new Set<string>();
  for (let i = 0; i < 201; i++) {
    const id = result.identities[`tie-${i}`];
    if (typeof id !== "string" || !id || expected.has(id))
      issues.push("Tied fixture identity map is incomplete or duplicated");
    else expected.add(id);
  }
  const seen = new Set<string>();
  const records: unknown[] = [];
  const pages: number[] = [];
  const links = new Set<string>();
  for (let i = 0; i < keys.length; i++) {
    const item = raw[`ties:${i}`];
    const body = item?.body;
    if (
      item?.status !== 200 ||
      body?.totalSize !== 201 ||
      typeof body?.done !== "boolean" ||
      !Array.isArray(body?.records) ||
      body.records.length < 1 ||
      body.records.length > 200
    ) {
      issues.push(`Invalid tied page ${i}`);
      continue;
    }
    pages.push(body.records.length);
    if (body.done !== (i === keys.length - 1))
      issues.push(`Invalid tied done transition at page ${i}`);
    if (body.done) {
      if (Object.hasOwn(body, "nextRecordsUrl"))
        issues.push(`Terminal tied page ${i} has continuation`);
    } else {
      const next = body.nextRecordsUrl;
      if (
        !next ||
        next.route !== route ||
        typeof next.value !== "string" ||
        !locator.test(next.value) ||
        links.has(next.value)
      )
        issues.push(`Invalid tied continuation at page ${i}`);
      else links.add(next.value);
    }
    for (const record of body.records) {
      if (
        !record ||
        typeof record !== "object" ||
        typeof record.Id !== "string" ||
        record.Compat_Text__c !== "same" ||
        !expected.has(record.Id) ||
        seen.has(record.Id)
      )
        issues.push(`Invalid tied record on page ${i}`);
      if (typeof record?.Id === "string") seen.add(record.Id);
      records.push(normalize(record, result.identities));
    }
  }
  if (records.length !== 201 || seen.size !== 201)
    issues.push("Tied traversal lost or duplicated fixture rows");
  return { issues, records, pages };
}

export function compareFoundationTies(
  a: Result,
  b: Result,
): { issues: string[]; diagnostics: string[] } {
  if (a.id !== "foundation-cursors" || b.id !== "foundation-cursors")
    return {
      issues: ["Tie rule applies only to foundation-cursors"],
      diagnostics: [],
    };
  const first = traversal(a),
    second = traversal(b);
  const issues = [
    ...first.issues.map((x) => `first: ${x}`),
    ...second.issues.map((x) => `second: ${x}`),
  ];
  if (issues.length) return { issues, diagnostics: [] };
  const left = first.records.map(canonical).sort();
  const right = second.records.map(canonical).sort();
  if (!isDeepStrictEqual(left, right))
    issues.push("Tied record payload multisets differ");
  const diagnostics =
    !isDeepStrictEqual(first.records, second.records) ||
    !isDeepStrictEqual(first.pages, second.pages)
      ? ["Tied row order or page membership differs between captures"]
      : [];
  return { issues, diagnostics };
}
