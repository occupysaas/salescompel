import { Ajv } from "ajv";
import { readFile, readdir } from "node:fs/promises";
import { resolve } from "node:path";
import { createHash } from "node:crypto";
import { isDeepStrictEqual } from "node:util";
import { parseDocument, validateCatalogs } from "./core.ts";
import { validateOpenApi } from "./openapi.ts";
import { ROOT, connectorVersions } from "./versions.ts";
export { ROOT };
export const SCENARIOS = [
  "describe",
  "crud",
  "upsert",
  "relationships",
  "rollback",
  "permissions",
  "errors",
  "locale",
  "cli-query",
  "metadata",
  "python-crud",
];
// Scenarios that create or modify remote state and must report cleanup.
export const CLEANUP_SCENARIOS = [
  "crud",
  "upsert",
  "relationships",
  "errors",
  "locale",
  "cli-query",
  "metadata",
  "python-crud",
];
export const readDoc = async (path: string) =>
  parseDocument(await readFile(resolve(ROOT, path), "utf8"));
export async function profile() {
  const text = await readFile(
    resolve(ROOT, "docs/compat/profiles/pilot.yaml"),
    "utf8",
  );
  return {
    data: parseDocument(text),
    hash: createHash("sha256").update(text).digest("hex"),
  };
}
export async function validateRepository(): Promise<string[]> {
  const issues: string[] = [];
  const ajv = new Ajv({ allErrors: true });
  async function schemaCheck(schemaPath: string, data: unknown, label: string) {
    const check = ajv.compile(await readDoc(schemaPath));
    if (!check(data)) issues.push(`${label}: ${ajv.errorsText(check.errors)}`);
  }
  const p = (await profile()).data;
  await schemaCheck("compat/schemas/profile.schema.json", p, "profile");
  if (
    p.schemaVersion !== 1 ||
    p.id !== "pilot" ||
    (p.apiVersion !== null && !/^\d+\.0$/.test(p.apiVersion))
  )
    issues.push("Invalid profile");
  if (JSON.stringify(p.requiredScenarios) !== JSON.stringify(SCENARIOS))
    issues.push("Profile scenario coverage differs from executable registry");
  try {
    if (!isDeepStrictEqual(p.connectors, connectorVersions()))
      issues.push(
        "Profile connector versions differ from locked/installed clients",
      );
  } catch (e) {
    issues.push(`Connector versions unavailable: ${String(e)}`);
  }
  // Every plugin command must be a profiled CLI command, and a plugin also
  // pinned under connectors must carry the same version.
  for (const plugin of p.cliPlugins ?? []) {
    if (
      p.connectors?.[plugin.name] &&
      p.connectors[plugin.name] !== plugin.version
    )
      issues.push(`CLI plugin ${plugin.name} version differs from connectors`);
    for (const command of plugin.commands ?? [])
      if (!p.salesforceCli?.commands?.includes(command))
        issues.push(
          `CLI plugin ${plugin.name} command ${command} is not profiled`,
        );
  }
  if (
    p.salesforceCli?.endpoint === "native" &&
    p.salesforceCli.proxiedHosts?.length
  )
    issues.push("Native CLI endpoint profile lists proxied hosts");
  if (
    p.salesforceCli?.endpoint === "proxy" &&
    !p.salesforceCli.proxiedHosts?.length
  )
    issues.push("Proxied CLI endpoint profile names no proxied hosts");
  const sources = (await readDoc("docs/compat/sources.yaml")).sources;
  if (!Array.isArray(sources)) throw new Error("Invalid source registry");
  const sourceIds = new Set(sources.map((s: any) => s.id));
  if (sourceIds.size !== sources.length) issues.push("Duplicate sources");
  for (const s of sources)
    if (
      !s.id ||
      !s.claim ||
      !s.version ||
      !s.extraction ||
      !/^https:\/\//.test(s.url) ||
      !/^\d{4}-\d{2}-\d{2}$/.test(s.accessedAt)
    )
      issues.push(`Invalid source ${s.id}`);
  const contracts = (await readDoc("docs/compat/contracts/pilot.yaml"))
    .contracts;
  if (!Array.isArray(contracts)) throw new Error("Invalid contracts");
  if (new Set(contracts.map((c: any) => c.id)).size !== contracts.length)
    issues.push("Duplicate contracts");
  for (const id of SCENARIOS) {
    const c = contracts.find((c: any) => c.id === id);
    if (
      !c ||
      !c.owner ||
      !["jsforce", "salesforce-cli", "simple-salesforce"].includes(c.client) ||
      !c.preconditions?.length ||
      !c.assertions?.length ||
      !c.normalization?.length ||
      c.observationStatus !== "not_run"
    )
      issues.push(`Incomplete contract ${id}`);
  }
  for (const c of contracts)
    if (
      !SCENARIOS.includes(c.id) ||
      !c.sources?.length ||
      c.sources.some((s: string) => !sourceIds.has(s))
    )
      issues.push(`Invalid contract references ${c.id}`);
  // Deferred-milestone contracts: documented only, still source-checked.
  for (const path of [
    "docs/compat/contracts/external-services.yaml",
    "docs/compat/contracts/localization.yaml",
    "docs/compat/contracts/identity.yaml",
    "docs/compat/contracts/rest-semantics.yaml",
    "docs/compat/contracts/cli-plugins.yaml",
  ]) {
    const doc = await readDoc(path);
    if (
      doc?.schemaVersion !== 1 ||
      !doc.owner ||
      !Array.isArray(doc.sources) ||
      !doc.sources.length ||
      !Array.isArray(doc.contracts)
    ) {
      issues.push(`${path}: invalid contract file`);
      continue;
    }
    for (const s of doc.sources)
      if (!sourceIds.has(s)) issues.push(`${path}: unknown source ${s}`);
    if (
      new Set(doc.contracts.map((c: any) => c.id)).size !== doc.contracts.length
    )
      issues.push(`${path}: duplicate contracts`);
    for (const c of doc.contracts)
      if (
        !c.id ||
        !c.behavior ||
        !c.milestone ||
        !c.preconditions?.length ||
        !c.assertions?.length ||
        c.observationStatus !== "not_run"
      )
        issues.push(`${path}: incomplete contract ${c.id}`);
  }
  const registry = await readDoc("locales/languages.json");
  await schemaCheck(
    "locales/languages.schema.json",
    registry,
    "language registry",
  );
  const catalogs: Record<string, Record<string, string>> = {};
  for (const file of await readdir(resolve(ROOT, "locales/messages")))
    if (file.endsWith(".json"))
      catalogs[file.slice(0, -5)] = await readDoc(`locales/messages/${file}`);
  for (const [code, catalog] of Object.entries(catalogs))
    await schemaCheck("locales/catalog.schema.json", catalog, code);
  issues.push(...validateCatalogs(registry.languages, catalogs));
  for (const code of p.languages)
    if (!registry.languages.some((l: any) => l.code === code))
      issues.push(`Profile language ${code} is not registered`);
  // Formatter tags must be canonical BCP 47; Salesforce wire codes stay as-is.
  for (const l of registry.languages) {
    let canonical: string | undefined;
    try {
      canonical = Intl.getCanonicalLocales(l.bcp47)[0];
    } catch {}
    if (!l.bcp47 || canonical !== l.bcp47)
      issues.push(`Language ${l.code}: invalid BCP 47 tag ${l.bcp47}`);
  }
  // External Services fixtures and the official Salesforce document model
  // Salesforce-side input, which SalesCompel converts to its latest version.
  for (const [path, role] of [
    ["api/openapi.yaml", "salescompel"],
    ["compat/fixtures/openapi/external-service.json", "salesforce-source"],
    ["compat/fixtures/openapi/external-service.yaml", "salesforce-source"],
    [
      "compat/fixtures/openapi/salesforce/einstein-bot-v5.yaml",
      "salesforce-source",
    ],
  ] as const) {
    try {
      await validateOpenApi(await readDoc(path), role);
    } catch (e) {
      issues.push(`${path}: ${String(e)}`);
    }
  }
  const provenance = await readDoc(
    "compat/fixtures/openapi/salesforce/provenance.json",
  );
  const official = await readFile(
    resolve(ROOT, "compat/fixtures/openapi/salesforce/einstein-bot-v5.yaml"),
  );
  if (createHash("sha256").update(official).digest("hex") !== provenance.sha256)
    issues.push("Official Salesforce fixture checksum mismatch");
  if (
    !/^[a-f0-9]{40}$/.test(provenance.commit) ||
    !provenance.sourceUrl.includes(provenance.commit)
  )
    issues.push("Official fixture is not commit pinned");
  await readFile(
    resolve(ROOT, "compat/fixtures/openapi/salesforce/LICENSE.txt"),
  );
  const pkg = await readDoc("compat/clients/node/package.json");
  const lock = await readDoc("compat/clients/node/package-lock.json");
  for (const [name, version] of Object.entries({
    ...pkg.dependencies,
    ...pkg.devDependencies,
  })) {
    if (
      typeof version !== "string" ||
      !/^\d+\.\d+\.\d+$/.test(version) ||
      lock.packages?.[`node_modules/${name}`]?.version !== version
    )
      issues.push(`Dependency not exactly locked: ${name}`);
  }
  return issues;
}
