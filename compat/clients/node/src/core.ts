import { parse as parseMessage } from "@formatjs/icu-messageformat-parser";
import { isDeepStrictEqual } from "node:util";
import { parseDocument as parseYaml } from "yaml";

export type Outcome = "passed" | "failed" | "blocked" | "not_run";
export interface Result {
  id: string;
  client: string;
  outcome: Outcome;
  assertions: { name: string; passed: boolean }[];
  observations: unknown;
  identities: Record<string, string>;
  reason?: string;
}
export interface Report {
  schemaVersion: 1;
  target: "reference" | "candidate";
  profile: string;
  profileHash: string;
  apiVersion: string | null;
  provenance: "live" | "synthetic" | "none";
  startedAt: string;
  finishedAt: string;
  connectors: Record<string, string>;
  requiredScenarios: string[];
  results: Result[];
  cleanup: {
    scenario?: string;
    resource: string;
    outcome: "passed" | "failed";
    reason?: string;
  }[];
}
export function parseDocument(text: string): any {
  const doc = parseYaml(text, { uniqueKeys: true });
  if (doc.errors.length)
    throw new Error(doc.errors.map((e) => e.message).join("; "));
  return doc.toJS({ maxAliasCount: 50 });
}
export function sanitize(value: unknown, secrets: string[] = []): any {
  if (typeof value === "string") {
    let s = value;
    for (const secret of secrets
      .filter(Boolean)
      .sort((a, b) => b.length - a.length))
      s = s.split(secret).join("[REDACTED]");
    return s
      .replace(/\b(Bearer|Basic)\s+[^\s"'<>]+/gi, "[REDACTED AUTH]")
      .replace(
        /((?:access[_-]?token|refresh[_-]?token|password|client[_-]?secret|authorization)\s*[=:]\s*)[^\s&"'<>]+/gi,
        "$1[REDACTED]",
      );
  }
  if (Array.isArray(value)) return value.map((v) => sanitize(v, secrets));
  if (value && typeof value === "object")
    return Object.fromEntries(
      Object.entries(value).map(([k, v]) => [
        k,
        /authorization|cookie|token|password|secret/i.test(k)
          ? "[REDACTED]"
          : sanitize(v, secrets),
      ]),
    );
  return value;
}
export function normalize(
  value: unknown,
  identities: Record<string, string>,
): unknown {
  const ids = new Map(
    Object.entries(identities).map(([name, id]) => [id, `<id:${name}>`]),
  );
  function visit(v: unknown): unknown {
    if (typeof v === "string") return ids.get(v) ?? v;
    if (Array.isArray(v)) return v.map(visit);
    if (v && typeof v === "object")
      return Object.fromEntries(
        Object.entries(v).map(([k, x]) => [k, visit(x)]),
      );
    return v;
  }
  return visit(value);
}
export function validateReport(input: unknown): asserts input is Report {
  const r = input as Report;
  if (
    !r ||
    r.schemaVersion !== 1 ||
    !["reference", "candidate"].includes(r.target) ||
    !["live", "synthetic", "none"].includes(r.provenance) ||
    !r.profile ||
    !r.profileHash ||
    !r.startedAt ||
    !r.finishedAt ||
    !r.connectors ||
    !Array.isArray(r.results) ||
    !Array.isArray(r.requiredScenarios) ||
    !Array.isArray(r.cleanup)
  )
    throw new Error("Invalid report envelope");
  if (r.apiVersion !== null && !/^\d+\.0$/.test(r.apiVersion))
    throw new Error("Invalid API version");
  if (
    new Set(r.requiredScenarios).size !== r.requiredScenarios.length ||
    r.requiredScenarios.some((s) => typeof s !== "string")
  )
    throw new Error("Invalid required scenarios");
  if (
    !Object.keys(r.connectors).length ||
    Object.values(r.connectors).some((v) => typeof v !== "string" || !v)
  )
    throw new Error("Missing connector versions");
  if (
    r.results.length !== r.requiredScenarios.length ||
    r.results.some((v) => !r.requiredScenarios.includes(v.id))
  )
    throw new Error("Result manifest differs from required scenarios");
  const ids = new Set<string>();
  for (const v of r.results) {
    if (
      !v.id ||
      ids.has(v.id) ||
      !["passed", "failed", "blocked", "not_run"].includes(v.outcome) ||
      !v.client ||
      !Array.isArray(v.assertions) ||
      !v.identities ||
      typeof v.identities !== "object" ||
      !Object.hasOwn(v, "observations")
    )
      throw new Error("Invalid scenario result");
    ids.add(v.id);
    if (
      v.assertions.some(
        (a) => typeof a.name !== "string" || typeof a.passed !== "boolean",
      )
    )
      throw new Error("Invalid assertion");
    if (
      v.outcome === "passed" &&
      (!v.assertions.length || v.assertions.some((a) => !a.passed))
    )
      throw new Error("Passing result requires successful assertions");
    if (
      Object.values(v.identities).some((x) => typeof x !== "string" || !x) ||
      new Set(Object.values(v.identities)).size !==
        Object.values(v.identities).length
    )
      throw new Error("Invalid identity map");
  }
  for (const c of r.cleanup)
    if (!c.resource || !["passed", "failed"].includes(c.outcome))
      throw new Error("Invalid cleanup result");
}
export interface ExpectedProfile {
  profile: string;
  profileHash: string;
  requiredScenarios: string[];
  connectors: Record<string, string>;
  cleanupScenarios?: string[];
}
export type CompareMode = "parity" | "repeatability";
export function compare(
  reference: Report,
  candidate: Report,
  expected?: ExpectedProfile,
  mode: CompareMode = "parity",
): { outcome: Outcome; differences: string[] } {
  try {
    validateReport(reference);
    validateReport(candidate);
  } catch (e) {
    return { outcome: "blocked", differences: [String(e)] };
  }
  // Repeatability compares two reference runs to find nondeterministic
  // observations (roadmap M1 exit gate); parity compares reference/candidate.
  const [first, second] =
    mode === "parity"
      ? ["reference", "candidate"]
      : ["first reference", "second reference"];
  const pairs = [
    [first, reference],
    [second, candidate],
  ] as const;
  const reasons: string[] = [];
  if (expected)
    for (const [label, r] of pairs)
      for (const key of [
        "profile",
        "profileHash",
        "requiredScenarios",
        "connectors",
      ] as const)
        if (!isDeepStrictEqual(r[key], expected[key]))
          reasons.push(
            `${label}: ${key} differs from registered profile/toolchain`,
          );
  if (expected?.cleanupScenarios)
    for (const [label, r] of pairs)
      for (const id of expected.cleanupScenarios)
        if (!r.cleanup.some((c) => c.scenario === id))
          reasons.push(`${label}: missing cleanup evidence for ${id}`);
  if (mode === "parity") {
    if (reference.target !== "reference" || candidate.target !== "candidate")
      reasons.push("Target roles must be reference then candidate");
  } else {
    if (reference.target !== "reference" || candidate.target !== "reference")
      reasons.push("Repeatability compares two reference captures");
    if (reference.startedAt === candidate.startedAt)
      reasons.push("Repeatability requires two distinct runs");
  }
  if (reference.provenance !== "live" || candidate.provenance !== "live")
    reasons.push("Live evidence required");
  for (const k of ["profile", "profileHash", "apiVersion"] as const)
    if (reference[k] !== candidate[k] || !reference[k])
      reasons.push(`Incompatible ${k}`);
  if (!isDeepStrictEqual(reference.connectors, candidate.connectors))
    reasons.push("Incompatible connector versions");
  if (
    !isDeepStrictEqual(
      reference.requiredScenarios,
      candidate.requiredScenarios,
    ) ||
    !reference.requiredScenarios.length
  )
    reasons.push("Incompatible required scenarios");
  for (const id of reference.requiredScenarios)
    for (const [label, r] of pairs) {
      const s = r.results.find((x) => x.id === id);
      if (!s || s.outcome === "blocked" || s.outcome === "not_run")
        reasons.push(`${label}: ${id} missing or blocked`);
    }
  if (reasons.length) return { outcome: "blocked", differences: reasons };
  if (
    reference.cleanup.some((c) => c.outcome === "failed") ||
    candidate.cleanup.some((c) => c.outcome === "failed")
  )
    reasons.push("Cleanup failed");
  const differ =
    mode === "parity" ? "differ" : "differ between runs (nondeterministic)";
  for (const id of reference.requiredScenarios) {
    const a = reference.results.find((s) => s.id === id)!,
      b = candidate.results.find((s) => s.id === id)!;
    if (a.outcome !== "passed" || b.outcome !== "passed")
      reasons.push(`${id}: assertions failed`);
    if (a.client !== b.client || !isDeepStrictEqual(a.assertions, b.assertions))
      reasons.push(`${id}: assertions/client ${differ}`);
    if (
      !isDeepStrictEqual(
        normalize(a.observations, a.identities),
        normalize(b.observations, b.identities),
      )
    )
      reasons.push(`${id}: observations ${differ}`);
  }
  return {
    outcome: reasons.length ? "failed" : "passed",
    differences: reasons,
  };
}
export function checkConfig(
  target: "reference" | "candidate",
  env: NodeJS.ProcessEnv,
): string[] {
  const prefix = `COMPAT_${target.toUpperCase()}`;
  const missing: string[] = [];
  for (const name of [`${prefix}_URL`, `${prefix}_TOKEN`, "COMPAT_API_VERSION"])
    if (!env[name]) missing.push(`Missing ${name}`);
  if (env.COMPAT_DISPOSABLE !== "yes")
    missing.push("COMPAT_DISPOSABLE=yes required for fixture writes");
  if (env.COMPAT_API_VERSION && !/^\d+\.0$/.test(env.COMPAT_API_VERSION))
    missing.push("API version must be explicit, e.g. 65.0");
  if (env[`${prefix}_URL`]) {
    try {
      const u = new URL(env[`${prefix}_URL`]!);
      if (
        u.username ||
        u.password ||
        u.search ||
        u.hash ||
        u.pathname !== "/" ||
        !(
          u.protocol === "https:" ||
          (target === "candidate" &&
            u.protocol === "http:" &&
            ["localhost", "127.0.0.1", "[::1]"].includes(u.hostname))
        )
      )
        throw new Error();
    } catch {
      missing.push(
        "Endpoint must be an HTTPS origin (candidate localhost may use HTTP), without credentials",
      );
    }
  }
  return missing;
}
export interface Language {
  code: string;
  category: string;
  direction: string;
  fallback: string | null;
  coverage: string;
  bcp47?: string;
}
export function validateCatalogs(
  languages: Language[],
  catalogs: Record<string, Record<string, string>>,
): string[] {
  const issues: string[] = [];
  const codes = new Set(languages.map((l) => l.code));
  if (codes.size !== languages.length) issues.push("Duplicate language");
  const base = catalogs.en_US;
  if (!base) return ["Missing en_US catalog"];
  const params = (s: string) => {
    const names = new Set<string>();
    function visit(nodes: any[]) {
      for (const node of nodes) {
        if (node.type >= 1 && node.type <= 6) names.add(node.value);
        if (node.options)
          for (const option of Object.values(node.options) as any[])
            visit(option.value);
        if (node.children) visit(node.children);
      }
    }
    visit(parseMessage(s));
    return [...names].sort();
  };
  for (const lang of languages) {
    if (
      !/^[a-z]{2,3}(?:_[A-Z]{2})?$/.test(lang.code) ||
      !["fully", "end-user", "platform-only"].includes(lang.category) ||
      !["ltr", "rtl"].includes(lang.direction) ||
      !["fixture", "partial", "complete"].includes(lang.coverage)
    )
      issues.push(`Invalid language ${lang.code}`);
    const visited = new Set<string>();
    let current: Language | undefined = lang;
    while (current) {
      if (visited.has(current.code)) {
        issues.push(`Fallback cycle ${lang.code}`);
        break;
      }
      visited.add(current.code);
      if (current.fallback && !codes.has(current.fallback))
        issues.push(`Missing fallback ${current.fallback}`);
      current = languages.find((l) => l.code === current?.fallback);
    }
    const cat = catalogs[lang.code];
    if (!cat) {
      issues.push(`Missing catalog ${lang.code}`);
      continue;
    }
    for (const key of Object.keys(base)) {
      if (!(key in cat)) {
        if (lang.category !== "platform-only")
          issues.push(`${lang.code}: missing ${key}`);
        continue;
      }
      if (typeof cat[key] !== "string" || typeof base[key] !== "string") {
        issues.push(`${lang.code}: invalid message ${key}`);
        continue;
      }
      try {
        if (!isDeepStrictEqual(params(cat[key]), params(base[key])))
          issues.push(`${lang.code}: parameters differ ${key}`);
      } catch {
        issues.push(`${lang.code}: malformed ICU message ${key}`);
      }
      if (/<[^>]+>/.test(cat[key]))
        issues.push(`${lang.code}: markup not permitted ${key}`);
    }
    for (const key of Object.keys(cat))
      if (!(key in base)) issues.push(`${lang.code}: unknown key ${key}`);
  }
  for (const code of Object.keys(catalogs))
    if (!codes.has(code)) issues.push(`Unregistered catalog ${code}`);
  return issues;
}
