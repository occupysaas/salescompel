import { isDeepStrictEqual } from "node:util";
import { foundationCoverage } from "./foundation-policy.ts";
import type { Report, Result } from "./core.ts";

const route = "/services/data/v65.0/query/{queryLocator}";
const locator = /^\/services\/data\/v65\.0\/query\/[^/?#]+$/;
const blockedReason =
  "Cursor expiry unmeasured under foundation-query-result-lifetime";

function multiset(values: string[]): string[] {
  return [...values].sort();
}

function observationShape(
  id: string,
  label: string,
  item: Record<string, any>,
): boolean {
  const status = item.status;
  if (
    typeof status !== "number" ||
    !Number.isInteger(status) ||
    status < 200 ||
    status >= 500
  )
    return false;
  const rawBody = item.body;
  const body = rawBody as Record<string, any> | null | undefined;
  const mutation =
    (id === "foundation-routes" &&
      ["patch", "escaped-upsert", "delete"].includes(label)) ||
    (id === "foundation-cursors" &&
      ["update-pending", "delete-pending"].includes(label)) ||
    (id === "foundation-ids" &&
      [
        "short-update",
        "short-reference",
        "invalid-reference",
        "unlink",
        "short-delete",
      ].includes(label)) ||
    (id === "foundation-names" &&
      ["lower-json", "duplicate-alias", "duplicate-raw"].includes(label)) ||
    (id === "foundation-errors" && label === "delete");
  const expectedError =
    (id === "foundation-names" && label.startsWith("denied-")) ||
    (id === "foundation-errors" &&
      !["after-failed-writes", "delete"].includes(label)) ||
    (id === "foundation-cursors" &&
      ["other-user", "unknown-locator", "malformed-locator"].includes(label));
  const optionalError =
    (id === "foundation-routes" && label === "deleted-read") ||
    (id === "foundation-names" &&
      ["lower-json", "duplicate-alias", "duplicate-raw"].includes(label));
  if (status === 401 || (status === 403 && !expectedError)) return false;
  if (status >= 400) {
    if (
      !expectedError &&
      !optionalError &&
      !(
        id === "foundation-ids" &&
        /^(lower|bad-checksum|short-length|wrong-prefix|invalid-reference|long-after-delete)/.test(
          label,
        )
      ) &&
      !(id === "foundation-pagination" && label.startsWith("batch:"))
    )
      return false;
    const errors: unknown[] = Array.isArray(rawBody) ? rawBody : [rawBody];
    return (
      errors.length > 0 &&
      errors.every((error) => {
        const e = error as Record<string, any> | null | undefined;
        return (
          typeof e?.errorCode === "string" &&
          e.errorCode !== "INVALID_SESSION_ID"
        );
      })
    );
  }
  if (expectedError) return false;
  if (mutation && status !== 204)
    return (
      id === "foundation-routes" &&
      label === "escaped-upsert" &&
      status === 201 &&
      typeof body?.id === "string"
    );
  if (status === 204)
    return body == null || (typeof body === "object" && !Array.isArray(body));
  if (body === null || typeof body !== "object") return false;
  if (
    label === "read" ||
    label.endsWith(":read") ||
    (label.startsWith("after-") &&
      ["foundation-names", "foundation-errors"].includes(id)) ||
    [
      "escaped-state",
      "reference-read",
      "reference-after-invalid",
      "short:read",
      "long:read",
      "null-record",
      "blank-record",
      "lower-path",
      "restricted-positive",
    ].includes(label)
  )
    return label === "restricted-positive"
      ? Array.isArray(body.records)
      : typeof body.Id === "string";
  if (label === "describe" || label === "lower-describe")
    return typeof body.name === "string" && Array.isArray(body.fields);
  if (label === "global") return Array.isArray(body.sobjects);
  if (label === "resources") return !Array.isArray(body);
  if (
    label.includes(":query") ||
    label.includes("equality:") ||
    label.includes(":exact") ||
    label.includes(":lower") ||
    label.includes(":upper") ||
    label.includes(":decomposed") ||
    label === "parent-projection" ||
    label === "lower-query" ||
    label.startsWith("batch:")
  )
    return (
      Array.isArray(body.records) &&
      typeof body.done === "boolean" &&
      Number.isInteger(body.totalSize)
    );
  if (
    id === "foundation-cursors" &&
    [
      "initial",
      "continuation",
      "replay",
      "parallel-a",
      "parallel-b",
      "fresh-session",
      "before-mutation",
      "after-mutation",
    ].includes(label)
  )
    return (
      Array.isArray(body.records) &&
      typeof body.done === "boolean" &&
      Number.isInteger(body.totalSize)
    );
  return true;
}

interface QueryPageBody {
  records: Array<Record<string, any>>;
  done: boolean;
  totalSize: number;
  nextRecordsUrl?: { route: string; value: string };
}

function pageIssues(
  result: Result,
  prefix: string,
  observations: Record<string, Record<string, any>>,
): { issues: string[]; names: string[]; labels: string[] } {
  const issues: string[] = [],
    names: string[] = [];
  const labels = Object.keys(observations).filter((k) =>
    k.startsWith(`${prefix}:`),
  );
  const nums = labels.map((k) => Number(k.slice(prefix.length + 1)));
  if (
    !labels.length ||
    labels.length > 32 ||
    !isDeepStrictEqual(
      multiset(nums.map(String)),
      multiset(Array.from({ length: labels.length }, (_, i) => String(i))),
    )
  )
    return {
      issues: [`${result.id}: ${prefix} pages missing or noncontiguous`],
      names,
      labels,
    };
  let total: number | undefined;
  let count = 0;
  const ids = new Set<string>();
  const links = new Set<string>();
  for (let i = 0; i < labels.length; i++) {
    const label = `${prefix}:${i}`;
    const page = observations[label];
    const rawBody = page?.body as Record<string, any> | undefined;
    for (const suffix of [
      "bounded JSON HTTP response",
      "query page",
      "stable total",
      ...(i === labels.length - 1 ? [] : ["continuation required"]),
    ])
      names.push(`${label}: ${suffix}`);
    if (
      page?.status !== 200 ||
      !Array.isArray(rawBody?.records) ||
      typeof rawBody?.done !== "boolean" ||
      !Number.isInteger(rawBody?.totalSize) ||
      (rawBody.totalSize as number) < 0 ||
      (rawBody.records as unknown[]).length > 2000
    ) {
      issues.push(`${result.id}: malformed ${label}`);
      continue;
    }
    const body = rawBody as unknown as QueryPageBody;
    if (total === undefined) total = body.totalSize;
    if (body.totalSize !== total)
      issues.push(`${result.id}: unstable ${prefix} total`);
    count += body.records.length;
    for (const row of body.records)
      if (typeof row?.Id === "string") {
        if (ids.has(row.Id))
          issues.push(`${result.id}: duplicate ${prefix} identity`);
        ids.add(row.Id);
      }
    const terminal = i === labels.length - 1;
    if (body.done !== terminal)
      issues.push(`${result.id}: bad ${prefix} done transition`);
    if (terminal) {
      if (Object.hasOwn(body, "nextRecordsUrl"))
        issues.push(`${result.id}: terminal ${prefix} link`);
    } else {
      const link = body.nextRecordsUrl;
      if (
        !link ||
        link.route !== route ||
        typeof link.value !== "string" ||
        !locator.test(link.value) ||
        links.has(link.value)
      )
        issues.push(`${result.id}: invalid ${prefix} link`);
      else links.add(link.value);
    }
  }
  names.push(
    `${prefix}: record count equals total`,
    `${prefix}: unique record identities`,
  );
  if (total !== count)
    issues.push(`${result.id}: incomplete ${prefix} traversal`);
  return { issues, names, labels };
}

export function coverageIssues(report: Report): string[] {
  const issues: string[] = [];
  const coverage = foundationCoverage();
  if (!isDeepStrictEqual(report.requiredScenarios, Object.keys(coverage)))
    issues.push("Required foundation scenario manifest differs");
  for (const [id, spec] of Object.entries(coverage)) {
    const result = report.results.find((r) => r.id === id);
    if (!result) {
      issues.push(`${id}: result missing`);
      continue;
    }
    if (result.client !== "jsforce") issues.push(`${id}: client differs`);
    if (id === "foundation-cursors") {
      if (result.outcome !== "blocked" || result.reason !== blockedReason)
        issues.push(`${id}: expiry dimension must be explicitly unmeasured`);
    } else if (result.outcome !== "passed" || result.reason)
      issues.push(`${id}: measured scenario did not pass`);
    const observations = result.observations as Record<
      string,
      Record<string, any>
    >;
    if (
      !observations ||
      typeof observations !== "object" ||
      Array.isArray(observations)
    ) {
      issues.push(`${id}: observations malformed`);
      continue;
    }
    const expected = [...spec.observations];
    const names = [...spec.assertions];
    for (const label of spec.observations) {
      if (!Object.hasOwn(observations, label)) {
        issues.push(`${id}: missing ${label}`);
        continue;
      }
      const item = observations[label];
      if (label === "versions") {
        if (item?.status !== 200 || item?.requestedVersionAdvertised !== true)
          issues.push(`${id}: malformed versions`);
      } else if (label.startsWith("own-user:")) {
        if (item?.status !== 200 || item?.hasOwnId !== true)
          issues.push(`${id}: malformed ${label}`);
      } else if (label === "nullEquality") {
        if (
          typeof item?.nullMatches !== "boolean" ||
          typeof item?.blankMatches !== "boolean"
        )
          issues.push(`${id}: malformed nullEquality`);
      } else {
        names.push(`${label}: bounded JSON HTTP response`);
        if (!observationShape(id, label, item))
          issues.push(`${id}: malformed ${label}`);
      }
    }
    for (const prefix of spec.pages) {
      const pages = pageIssues(result, prefix, observations);
      issues.push(...pages.issues);
      expected.push(...pages.labels);
      names.push(...pages.names);
    }
    if (
      !isDeepStrictEqual(
        multiset(Object.keys(observations)),
        multiset(expected),
      )
    )
      issues.push(`${id}: unexpected or missing observations`);
    const actual = result.assertions.map((a) => a.name);
    if (result.assertions.some((a) => a.passed !== true))
      issues.push(`${id}: assertion failed`);
    if (id === "foundation-text-null") {
      const count = actual.filter(
        (x) => x === "null equality query succeeds",
      ).length;
      if (count < 1 || count > 32)
        issues.push(`${id}: null query assertion coverage invalid`);
      const without = (v: string[]) =>
        v.filter((x) => x !== "null equality query succeeds");
      if (
        !isDeepStrictEqual(multiset(without(actual)), multiset(without(names)))
      )
        issues.push(`${id}: assertion coverage differs`);
    } else if (!isDeepStrictEqual(multiset(actual), multiset(names)))
      issues.push(`${id}: assertion coverage differs`);
  }
  return issues;
}
