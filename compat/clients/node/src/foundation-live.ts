import { createHash } from "node:crypto";
import { wire } from "./wire.ts";
import type { Report } from "./core.ts";
import type { CleanupAbsenceEvidence } from "./foundation-assessment.ts";
import { join } from "node:path";
import { privateDirectory } from "./metadata-files.ts";
import { readPrivateFoundationFile } from "./foundation-sidecar.ts";

export async function foundationLivePreflight(
  env: NodeJS.ProcessEnv,
): Promise<string> {
  const url = env.COMPAT_REFERENCE_URL;
  const primary = env.COMPAT_REFERENCE_TOKEN;
  const fresh = env.COMPAT_REFERENCE_FRESH_TOKEN;
  const restricted = env.COMPAT_REFERENCE_RESTRICTED_TOKEN;
  if (
    !url ||
    !primary ||
    !fresh ||
    !restricted ||
    primary === fresh ||
    env.COMPAT_API_VERSION !== "65.0"
  )
    throw new Error("Foundation live identity prerequisites are incomplete");
  const base = new URL(url);
  const get = async (token: string, path: string) => {
    const response = await wire(base, token, path, { method: "GET" });
    if (
      response.status !== 200 ||
      !response.body ||
      typeof response.body !== "object"
    )
      throw new Error("Foundation live identity or metadata read failed");
    return response.body as any;
  };
  const [admin, freshAdmin, other, account, contact, restrictedAccount] =
    await Promise.all([
      get(primary, "/services/oauth2/userinfo"),
      get(fresh, "/services/oauth2/userinfo"),
      get(restricted, "/services/oauth2/userinfo"),
      get(primary, "/services/data/v65.0/sobjects/Account/describe"),
      get(primary, "/services/data/v65.0/sobjects/Contact/describe"),
      get(restricted, "/services/data/v65.0/sobjects/Account/describe"),
    ]);
  if (
    typeof admin.user_id !== "string" ||
    !admin.user_id ||
    typeof admin.organization_id !== "string" ||
    !admin.organization_id ||
    admin.user_id !== freshAdmin.user_id ||
    admin.organization_id !== freshAdmin.organization_id ||
    admin.organization_id !== other.organization_id ||
    admin.user_id === other.user_id
  )
    throw new Error(
      "Foundation administrator/fresh/restricted identities differ",
    );
  const fieldNames = (value: any): Set<string> => {
    if (!Array.isArray(value?.fields))
      throw new Error("Foundation metadata describe malformed");
    return new Set(value.fields.map((field: any) => field?.name));
  };
  const adminFields = fieldNames(account),
    contactFields = fieldNames(contact),
    restrictedFields = fieldNames(restrictedAccount);
  if (
    account.name !== "Account" ||
    contact.name !== "Contact" ||
    restrictedAccount.name !== "Account" ||
    ["Id", "Name", "Compat_Text__c", "Compat_Key__c"].some(
      (name) => !adminFields.has(name),
    ) ||
    ["Id", "AccountId"].some((name) => !contactFields.has(name)) ||
    !restrictedFields.has("Id") ||
    !restrictedFields.has("Name") ||
    restrictedFields.has("Compat_Text__c")
  )
    throw new Error(
      "Foundation metadata or restricted field permissions differ",
    );
  return createHash("sha256")
    .update(admin.user_id)
    .update("\0")
    .update(admin.organization_id)
    .digest("hex");
}

export async function collectFoundationAbsence(
  report: Report,
  captureSha256: string,
  identityDigest: string,
  env: NodeJS.ProcessEnv,
  journalClear: boolean,
): Promise<CleanupAbsenceEvidence> {
  if (!journalClear || !/^[0-9a-f]{64}$/.test(captureSha256))
    throw new Error(
      "Foundation recovery journal or capture binding incomplete",
    );
  const currentIdentity = await foundationLivePreflight(env);
  if (currentIdentity !== identityDigest)
    throw new Error("Foundation cleanup identity differs from capture");
  if (
    !report.cleanup.length ||
    report.cleanup.some((item) => item.outcome !== "passed")
  )
    throw new Error("Foundation fixture cleanup incomplete");
  const resources = report.cleanup.map((item) => item.resource);
  if (
    new Set(resources).size !== resources.length ||
    resources.some(
      (resource) =>
        !/^(Account|Contact):[A-Za-z0-9]{15}(?:[A-Za-z0-9]{3})?$/.test(
          resource,
        ),
    )
  )
    throw new Error("Foundation cleanup resource list malformed");
  const base = new URL(env.COMPAT_REFERENCE_URL!);
  for (const object of ["Account", "Contact"]) {
    const ids = resources
      .filter((resource) => resource.startsWith(`${object}:`))
      .map((resource) => resource.slice(object.length + 1));
    for (let offset = 0; offset < ids.length; offset += 100) {
      const batch = ids.slice(offset, offset + 100);
      const statement = `SELECT Id FROM ${object} WHERE Id IN (${batch.map((id) => `'${id}'`).join(",")})`;
      const result = await wire(
        base,
        env.COMPAT_REFERENCE_TOKEN!,
        `/services/data/v65.0/query?q=${encodeURIComponent(statement)}`,
        { method: "GET" },
      );
      const body = result.body as any;
      if (
        result.status !== 200 ||
        body?.done !== true ||
        body?.totalSize !== 0 ||
        !Array.isArray(body?.records) ||
        body.records.length !== 0 ||
        Object.hasOwn(body, "nextRecordsUrl")
      )
        throw new Error("Foundation exact-ID absence could not be verified");
    }
  }
  return {
    schemaVersion: 1,
    captureSha256,
    checkedAt: new Date().toISOString(),
    identityVerified: true,
    identityDigest,
    unresolvedJournals: 0,
    resources: resources.map((resource) => ({ resource, absent: true })),
  };
}

export async function auditFoundationRecovery(
  recoveryRoot: string,
  report: Report,
): Promise<boolean> {
  try {
    const root = await privateDirectory(recoveryRoot);
    const runIds = new Set<string>();
    for (const result of report.results)
      for (const [key, value] of Object.entries(result.identities)) {
        if (!key.startsWith("value:")) continue;
        const match = /^Compat_([0-9a-f]{16})_/.exec(value);
        if (match) runIds.add(match[1]);
      }
    if (runIds.size !== 1) return false;
    const runDir = await privateDirectory(
      join(root, `reference-${[...runIds][0]}`),
    );
    const expected = new Map<string, string[]>();
    for (const item of report.cleanup) {
      if (!item.scenario || item.outcome !== "passed") return false;
      const list = expected.get(item.scenario) ?? [];
      list.push(item.resource);
      expected.set(item.scenario, list);
    }
    for (const result of report.results) {
      const lines = (
        await readPrivateFoundationFile(join(runDir, `${result.id}.jsonl`))
      )
        .trimEnd()
        .split("\n");
      let intents = 0;
      const found: string[] = [];
      let manualUpsertIntent = 0;
      for (const line of lines) {
        const entry = JSON.parse(line);
        if (
          entry.intent === "create" &&
          typeof entry.type === "string" &&
          entry.fields &&
          typeof entry.fields === "object"
        ) {
          intents++;
          if (
            result.id === "foundation-routes" &&
            entry.fields.Compat_Key__c === result.identities.external
          )
            manualUpsertIntent++;
        } else if (typeof entry.resource === "string")
          found.push(entry.resource);
        else return false;
      }
      const upsert = (result.observations as any)?.["escaped-upsert"];
      const permittedExtra =
        result.id === "foundation-routes" &&
        manualUpsertIntent === 2 &&
        upsert?.status === 204
          ? 1
          : 0;
      // The routes fixture creates a keyed Account, then journals one update intent for it.
      if (
        intents - permittedExtra !== found.length ||
        !isSameSet(found, expected.get(result.id) ?? [])
      )
        return false;
    }
    return true;
  } catch {
    return false;
  }
}

function isSameSet(a: string[], b: string[]): boolean {
  return (
    a.length === b.length &&
    new Set(a).size === a.length &&
    a.every((value) => b.includes(value))
  );
}
