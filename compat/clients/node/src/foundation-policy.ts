export const FOUNDATION_ASSUMPTION_ID = "foundation-query-result-lifetime";
export const FOUNDATION_POLICY_VERSION = 1;

export type FoundationCollectionPolicy =
  "observed-only" | "documented-two-day-assumption";

export interface FoundationScenarioCoverage {
  observations: readonly string[];
  pages: readonly string[];
  assertions: readonly string[];
}

const coverage: Readonly<Record<string, FoundationScenarioCoverage>> = {
  "foundation-routes": {
    observations: [
      "versions",
      "resources",
      "global",
      "describe",
      "read",
      "patch",
      "escaped-upsert",
      "escaped-state",
      "own-user:65.0",
      "own-user:51.0",
      "delete",
      "deleted-read",
    ],
    pages: ["query"],
    assertions: [
      "requested API version is advertised",
      "userinfo identifies own user",
      "own-user 65.0 response",
      "own-user 51.0 response",
    ],
  },
  "foundation-pagination": {
    observations: [
      "batch:1",
      "batch:199",
      "batch:2000",
      "batch:2001",
      "batch:-1",
      "batch:abc",
      "batch:",
      "parent-projection",
    ],
    pages: ["default", "limit5000", "batch200", "exact", "empty", "no-id"],
    assertions: ["default covers all owned rows"],
  },
  "foundation-cursors": {
    observations: [
      "initial",
      "continuation",
      "replay",
      "parallel-a",
      "parallel-b",
      "fresh-session",
      "other-user",
      "unknown-locator",
      "malformed-locator",
      "before-mutation",
      "update-pending",
      "delete-pending",
      "after-mutation",
    ],
    pages: ["ties"],
    assertions: [
      "fresh session has same user",
      "initial: incomplete first page",
      "before-mutation: incomplete first page",
    ],
  },
  "foundation-ids": {
    observations: [
      ...[
        "short",
        "long",
        "lower",
        "bad-checksum",
        "short-length",
        "wrong-prefix",
      ].flatMap((label) => [`${label}:read`, `${label}:query`]),
      "short-update",
      "short-reference",
      "reference-read",
      "invalid-reference",
      "reference-after-invalid",
      "unlink",
      "short-delete",
      "long-after-delete",
    ],
    pages: [],
    assertions: [],
  },
  "foundation-names": {
    observations: [
      "lower-describe",
      "lower-path",
      "lower-query",
      "lower-json",
      "after-lower-json",
      "duplicate-alias",
      "after-duplicate-alias",
      "duplicate-raw",
      "after-duplicate-raw",
      "after-duplicates",
      "restricted-positive",
      "denied-select",
      "denied-filter",
      "denied-order",
      "denied-write",
      "after-denied-write",
    ],
    pages: [],
    assertions: [
      "restricted persona can read the owned record",
      "denied-select: field access rejected",
      "denied-filter: field access rejected",
      "denied-order: field access rejected",
      "restricted write rejected",
      "denied write leaves field unchanged",
    ],
  },
  "foundation-text-null": {
    observations: [
      ...["mixed", "accent", "turkish", "sharp"].flatMap((label) =>
        ["exact", "lower", "upper", "decomposed"].map(
          (variant) => `${label}:${variant}`,
        ),
      ),
      "name-equality:exact",
      "name-equality:lower",
      "name-equality:upper",
      "nullEquality",
      "null-record",
      "blank-record",
    ],
    pages: [
      "order:ASC",
      "order:ASC NULLS FIRST",
      "order:ASC NULLS LAST",
      "order:DESC",
      "order:DESC NULLS FIRST",
      "order:DESC NULLS LAST",
    ],
    assertions: ["null equality query succeeds"],
  },
  "foundation-errors": {
    observations: [
      "malformed-json",
      "trailing-json",
      "required-field",
      "unknown-field",
      "after-failed-writes",
      "unknown-rest-object",
      "unknown-soql-object",
      "malformed-query",
      "invalid-id",
      "delete",
      "missing-record",
    ],
    pages: [],
    assertions: ["invalid writes leave name unchanged"],
  },
};

export function foundationCoverage(): Readonly<
  Record<string, FoundationScenarioCoverage>
> {
  return structuredClone(coverage);
}

export function assertFoundationPolicy(
  policy: string,
  target: "reference" | "candidate",
  suiteId: string,
): asserts policy is FoundationCollectionPolicy {
  if (
    policy !== "observed-only" &&
    !(
      policy === "documented-two-day-assumption" &&
      target === "reference" &&
      suiteId === "rest-query-foundation"
    )
  )
    throw new Error("Invalid foundation collection policy or scope");
}

export function expiryMeasurementRequired(
  policy: FoundationCollectionPolicy,
): boolean {
  return policy === "observed-only";
}

export async function runExpiryObservation(
  policy: FoundationCollectionPolicy,
  start: () => Promise<string>,
  wait: () => Promise<void>,
  retrieve: (locator: string) => Promise<number>,
): Promise<"measured" | "unmeasured"> {
  if (!expiryMeasurementRequired(policy)) return "unmeasured";
  const locator = await start();
  await wait();
  const status = await retrieve(locator);
  if (status === 200)
    throw new Error(
      "Locator still valid at 16-minute observation bound; expiry unmeasured",
    );
  return "measured";
}
