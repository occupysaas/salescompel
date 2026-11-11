import { readFile, lstat } from "node:fs/promises";
import { basename } from "node:path";
import { createHash } from "node:crypto";
import type { ParsedResult, GateID } from "./candidate-acceptance-contract.ts";
export const digest = (s: string | Buffer) =>
  createHash("sha256").update(s).digest("hex");
export function parseGateEvents(
  kind: "go" | "node" | "playwright" | "steps",
  text: string,
  gate: GateID,
): ParsedResult["tests"] {
  if (!text.trim()) throw new Error("RESULT");
  if (kind === "steps") {
    const r = JSON.parse(text);
    if (
      r.schemaVersion !== 1 ||
      r.suite !== gate ||
      !r.complete ||
      !Array.isArray(r.tests)
    )
      throw new Error("RESULT");
    return r.tests;
  }
  if (kind === "playwright") {
    const r = JSON.parse(text);
    if (!r.stats || !Array.isArray(r.suites) || r.errors?.length)
      throw new Error("RESULT");
    const out: ParsedResult["tests"] = [];
    type PlaywrightCaseResult = { status?: string };
    type PlaywrightCase = {
      status?: string;
      results?: PlaywrightCaseResult[];
    };
    type PlaywrightSpec = { title: string; tests?: PlaywrightCase[] };
    type PlaywrightSuite = {
      specs?: PlaywrightSpec[];
      suites?: PlaywrightSuite[];
    };
    const visit = (s: PlaywrightSuite) => {
      for (const spec of s.specs ?? [])
        for (const t of spec.tests ?? []) {
          if (!t.results?.length) throw new Error("RESULT");
          out.push({
            id: spec.title,
            outcome:
              t.status === "expected" &&
              t.results.every(
                (r: PlaywrightCaseResult) => r.status === "passed",
              )
                ? "passed"
                : t.status === "skipped"
                  ? "skipped"
                  : "failed",
          });
        }
      for (const sub of s.suites ?? []) visit(sub);
    };
    r.suites.forEach(visit);
    if (!out.length) throw new Error("RESULT");
    return out;
  }
  const events = text
      .trim()
      .split("\n")
      .map((l) => JSON.parse(l)),
    out: ParsedResult["tests"] = [];
  if (kind === "node") {
    if (!events.some((e) => e.type === "test:summary"))
      throw new Error("RESULT");
    const terminal = events.filter((e) =>
      ["test:pass", "test:fail"].includes(e.type),
    );
    type NodeTestData = {
      name: string;
      file?: string;
      entryFile?: string;
      testId?: number;
      parentId?: number;
      nesting?: number;
      testNumber?: number;
      skip?: boolean;
    };
    const fileOf = (d: NodeTestData): string =>
      d.entryFile ?? d.file ?? "unknown";
    const byID = new Map<string, NodeTestData>();
    for (const e of terminal)
      if (Number.isSafeInteger(e.data.testId))
        byID.set(JSON.stringify([fileOf(e.data), e.data.testId]), e.data);
    // Older Node releases lack parentId/testId. Their declaration-ordered
    // terminal stream closes each parent after its children, so reconstruct
    // ancestry backwards. Missing ancestry fails closed rather than invent IDs.
    const legacyNames = new Map<object, string[]>();
    const stacks = new Map<string, string[]>();
    for (const e of [...terminal].reverse()) {
      const d = e.data,
        n = d.nesting ?? 0,
        stack = stacks.get(fileOf(d)) ?? [];
      if (!Number.isSafeInteger(n) || n < 0) throw new Error("RESULT");
      if (stack.length >= n) legacyNames.set(d, [...stack.slice(0, n), d.name]);
      stack[n] = d.name;
      stack.length = n + 1;
      stacks.set(fileOf(d), stack);
    }
    const nestedName = (d: NodeTestData): string => {
      let names: string[] | undefined;
      if (Number.isSafeInteger(d.testId) && Number.isSafeInteger(d.parentId)) {
        names = [d.name];
        const seen = new Set<number>([d.testId as number]);
        let parent = d.parentId as number;
        while (parent > 0) {
          if (seen.has(parent)) throw new Error("RESULT");
          seen.add(parent);
          const p = byID.get(JSON.stringify([fileOf(d), parent]));
          if (!p) throw new Error("RESULT");
          names.unshift(p.name);
          parent = p.parentId ?? 0;
        }
      } else names = legacyNames.get(d);
      if (!names || names.length !== (d.nesting ?? 0) + 1)
        throw new Error("RESULT");
      return JSON.stringify([...names, d.testNumber ?? 0]);
    };
    for (const e of events) {
      if (!["test:pass", "test:fail"].includes(e.type)) continue;
      const d = e.data;
      const id =
        d.name.startsWith("candidate/jsforce/") ||
        d.name === "candidate/openapi/traffic" ||
        d.name === "candidate/composite/atomic" ||
        d.name === "candidate/foundation/decisions"
          ? d.name
          : `${basename(d.file ?? "unknown")}::${(d.nesting ?? 0) > 0 ? nestedName(d) : d.name}`;
      out.push({
        id,
        outcome: d.skip
          ? "skipped"
          : e.type === "test:fail"
            ? "failed"
            : "passed",
      });
    }
  } else {
    const packages = new Set<string>(),
      ended = new Set<string>();
    for (const e of events) {
      if (e.Package) packages.add(e.Package);
      if (e.Test && ["pass", "fail", "skip"].includes(e.Action))
        out.push({
          id: e.Test.includes("/") ? `${e.Package}/${e.Test}` : e.Test,
          outcome:
            e.Action === "pass"
              ? "passed"
              : e.Action === "skip"
                ? "skipped"
                : "failed",
        });
      if (!e.Test && ["pass", "fail", "skip"].includes(e.Action)) {
        ended.add(e.Package);
        if (e.Action === "fail")
          out.push({ id: e.Package + "/package", outcome: "failed" });
      }
    }
    if (!packages.size || [...packages].some((p) => !ended.has(p)))
      throw new Error("RESULT");
  }
  if (!out.length) throw new Error("RESULT");
  return out;
}
export async function readGateResult(
  kind: "go" | "node" | "playwright" | "steps",
  path: string,
  runId: string,
): Promise<ParsedResult> {
  const info = await lstat(path);
  if (!info.isFile() || info.isSymbolicLink() || info.size > 32 * 1024 * 1024)
    throw new Error("RESULT");
  const text = await readFile(path, "utf8"),
    b = JSON.parse(await readFile(path + ".binding.json", "utf8"));
  if (
    b.schemaVersion !== 1 ||
    b.runId !== runId ||
    b.digest !== digest(text) ||
    !b.complete
  )
    throw new Error("RESULT");
  if (
    kind === "steps" &&
    (JSON.parse(text) as { runId?: string } | null)?.runId !== runId
  )
    throw new Error("RESULT");
  return {
    schemaVersion: 1,
    runId,
    suite: b.gate,
    exitCode: b.exitCode,
    complete: b.complete,
    tests: parseGateEvents(kind, text, b.gate),
  };
}
export function mergeGateResults(
  gate: GateID,
  results: ParsedResult[],
): ParsedResult {
  if (
    !results.length ||
    results.some((r) => r.suite !== gate || r.runId !== results[0].runId)
  )
    throw new Error("RESULT");
  return {
    ...results[0],
    complete: results.every((r) => r.complete),
    exitCode: results.some((r) => r.exitCode !== 0) ? 1 : 0,
    tests: results.flatMap((r) => r.tests),
  };
}
