import { Connection } from "jsforce";
import type { Result, Report } from "./core.ts";
import { FixtureTracker } from "./fixtures.ts";
import type { FoundationCollectionPolicy } from "./foundation-policy.ts";
export class Blocked extends Error {}
// Documentation does not settle which code a read of a deleted record returns;
// both are accepted here and the observed code is compared between targets.
export const MISSING_RECORD_CODES = ["NOT_FOUND", "ENTITY_IS_DELETED"];

interface SaveResult {
  id?: string;
  success: boolean;
  created?: boolean;
}
interface RecordRow {
  Id?: string;
  Name?: string;
  [key: string]: unknown;
}
interface QueryResponse {
  records: Array<Record<string, unknown>>;
  totalSize: number;
  done: boolean;
  nextRecordsUrl?: string;
}
interface CompositeBody {
  compositeResponse?: Array<{
    referenceId?: string;
    httpStatusCode?: number;
    body?: unknown;
  }>;
}
interface SfError {
  errorCode?: string;
  name?: string;
  statusCode?: number;
}
export interface Context {
  conn: Connection;
  env: NodeJS.ProcessEnv;
  prefix: string;
  runId: string;
  tracker: FixtureTracker;
  result: Result;
  cleanup: Report["cleanup"];
  collectionPolicy?: FoundationCollectionPolicy;
}
export function expect(c: Context, name: string, condition: unknown) {
  c.result.assertions.push({ name, passed: !!condition });
  if (!condition) throw new Error(`Assertion failed: ${name}`);
}
export async function create(
  c: Context,
  type: string,
  fields: Record<string, unknown>,
  label: string,
): Promise<string> {
  await c.tracker.intent(type, fields);
  const r = (await c.conn.sobject(type).create(fields)) as SaveResult;
  if (!r.success || !r.id) throw new Error(`Create ${type} failed`);
  c.result.identities[label] = r.id;
  await c.tracker.add(`${type}:${r.id}`, async () => {
    try {
      const d = (await c.conn.sobject(type).destroy(r.id!)) as SaveResult;
      if (!d.success) throw new Error("Delete failed");
    } catch (e: unknown) {
      const err = e as SfError;
      if (!MISSING_RECORD_CODES.includes(err.errorCode ?? "")) throw e;
    }
  });
  return r.id;
}
async function errorOf(fn: () => PromiseLike<unknown>) {
  try {
    await fn();
    return { errorCode: null };
  } catch (e: unknown) {
    const err = e as SfError;
    return {
      errorCode: err.errorCode ?? err.name,
      statusCode: err.statusCode ?? null,
    };
  }
}
export const jsScenarios: Record<string, (c: Context) => Promise<void>> = {
  async describe(c) {
    const account = await c.conn.sobject("Account").describe();
    const contact = await c.conn.sobject("Contact").describe();
    const fields = account.fields
      .filter((f) => ["Id", "Compat_Text__c", "Compat_Key__c"].includes(f.name))
      .map((f) => ({ name: f.name, type: f.type, externalId: f.externalId }))
      .sort((a, b) => a.name.localeCompare(b.name));
    c.result.observations = {
      account: account.name,
      contact: contact.name,
      fields,
    };
    expect(c, "Account custom fields and Id exist", fields.length === 3);
    expect(
      c,
      "Contact Id exists",
      contact.fields.some((f) => f.name === "Id"),
    );
    expect(
      c,
      "External key is external ID",
      fields.find((f) => f.name === "Compat_Key__c")?.externalId,
    );
  },
  async crud(c) {
    const id = await create(
      c,
      "Account",
      { Name: `SC ${c.runId} crud` },
      "account",
    );
    const read = (await c.conn.sobject("Account").retrieve(id)) as RecordRow;
    expect(c, "Created name read back", read.Name === `SC ${c.runId} crud`);
    const updated = (await c.conn
      .sobject("Account")
      .update({ Id: id, Name: "Compatibility updated" })) as SaveResult;
    expect(c, "Update succeeded", updated.success);
    const after = (await c.conn.sobject("Account").retrieve(id)) as RecordRow;
    const del = (await c.conn.sobject("Account").destroy(id)) as SaveResult;
    const missing = await errorOf(() => c.conn.sobject("Account").retrieve(id));
    c.result.observations = {
      record: { Id: after.Id, Name: after.Name },
      deleteSuccess: del.success,
      missing,
    };
    expect(c, "Updated name persisted", after.Name === "Compatibility updated");
    expect(
      c,
      "Deleted record inaccessible",
      MISSING_RECORD_CODES.includes(missing.errorCode ?? ""),
    );
  },
  async upsert(c) {
    const key = `${c.runId}-upsert`;
    await c.tracker.intent("Account", {
      Name: "Compatibility initial",
      Compat_Key__c: key,
    });
    const first = (await c.conn
      .sobject("Account")
      .upsert(
        { Name: "Compatibility initial", Compat_Key__c: key },
        "Compat_Key__c",
      )) as SaveResult;
    if (first.id) {
      c.result.identities.account = first.id;
      await c.tracker.add(`Account:${first.id}`, async () => {
        const r = (await c.conn.sobject("Account").destroy(first.id!)) as SaveResult;
        if (!r.success) throw new Error("Delete failed");
      });
    }
    expect(
      c,
      "Initial upsert created record",
      first.success && first.id && first.created,
    );
    const second = (await c.conn
      .sobject("Account")
      .upsert(
        { Name: "Compatibility final", Compat_Key__c: key },
        "Compat_Key__c",
      )) as SaveResult;
    const q = (await c.conn.query(
      `SELECT Id,Name FROM Account WHERE Compat_Key__c = '${key}'`,
    )) as QueryResponse;
    c.result.observations = {
      firstCreated: first.created,
      secondCreated: second.created,
      secondSuccess: second.success,
      records: q.records.map((r) => ({ Id: r.Id, Name: r.Name })),
    };
    expect(c, "Repeated upsert leaves one record", q.totalSize === 1);
    expect(
      c,
      "Updated value persisted",
      q.records[0].Name === "Compatibility final",
    );
  },
  async relationships(c) {
    const parent = await create(
      c,
      "Account",
      { Name: `SC ${c.runId} relations` },
      "account",
    );
    // Bounded batches; journal every successful row before checking partial failures.
    for (let offset = 0; offset < 201; offset += 100) {
      const rows = Array.from(
        { length: Math.min(100, 201 - offset) },
        (_, i) => ({
          LastName: `Compat ${String(offset + i).padStart(3, "0")}`,
          AccountId: parent,
        }),
      );
      for (const row of rows) await c.tracker.intent("Contact", row);
      const results = (await c.conn.sobject("Contact").create(rows)) as SaveResult[];
      for (let i = 0; i < results.length; i++)
        if (results[i].success) {
          const id = results[i].id!;
          c.result.identities[`contact${offset + i}`] = id;
          await c.tracker.add(`Contact:${id}`, async () => {
            const r = (await c.conn.sobject("Contact").destroy(id)) as SaveResult;
            if (!r.success) throw new Error("Delete failed");
          });
        }
      expect(
        c,
        "Contact batch created",
        results.every((r) => r.success),
      );
    }
    const query = `SELECT Id,LastName,AccountId,Account.Id FROM Contact WHERE AccountId = '${parent}' ORDER BY LastName`;
    let page = (await c.conn.request({
      method: "GET",
      url: `/query?q=${encodeURIComponent(query)}`,
      headers: { "Sforce-Query-Options": "batchSize=200" },
    })) as QueryResponse;
    const rows: Array<Record<string, unknown>> = [];
    let pages = 0;
    const locators = new Set<string>();
    for (;;) {
      pages++;
      rows.push(
        ...page.records.map((r) => ({
          Id: r.Id,
          LastName: r.LastName,
          AccountId: r.AccountId,
          parentId: (r.Account as Record<string, unknown> | undefined)?.Id,
        })),
      );
      if (page.done) break;
      if (
        pages >= 10 ||
        !page.nextRecordsUrl ||
        locators.has(page.nextRecordsUrl)
      )
        throw new Error("Invalid pagination cursor");
      locators.add(page.nextRecordsUrl);
      if (
        !page.nextRecordsUrl.startsWith(
          `/services/data/v${c.conn.version}/query/`,
        )
      )
        throw new Error("Unexpected query cursor origin/path");
      page = await c.conn.queryMore(page.nextRecordsUrl);
    }
    c.result.observations = { rows, pages };
    expect(c, "Pagination exercised", pages > 1);
    expect(
      c,
      "Exactly 201 unique contacts",
      rows.length === 201 && new Set(rows.map((r) => r.Id)).size === 201,
    );
    expect(
      c,
      "Parent identity preserved",
      rows.every((r) => r.AccountId === parent && r.parentId === parent),
    );
  },
  async rollback(c) {
    const key = `${c.runId}-rollback`;
    await c.tracker.intent("Account", {
      Name: "Rollback probe",
      Compat_Key__c: key,
    });
    const response = (await c.conn.request({
      method: "POST",
      url: "/composite",
      headers: { "Content-Type": "application/json" },
      body: JSON.stringify({
        allOrNone: true,
        compositeRequest: [
          {
            method: "POST",
            url: `/services/data/v${c.conn.version}/sobjects/Account`,
            referenceId: "account",
            body: { Name: "Rollback probe", Compat_Key__c: key },
          },
          {
            method: "PATCH",
            url: `/services/data/v${c.conn.version}/sobjects/Account/@{account.id}`,
            referenceId: "bad",
            body: { Compat_NoSuchField__c: true },
          },
        ],
      }),
    })) as CompositeBody;
    const q = (await c.conn.query(
      `SELECT Id FROM Account WHERE Compat_Key__c = '${key}'`,
    )) as QueryResponse;
    // An incorrectly committed row is still ours and must be cleaned up.
    for (const r of q.records)
      await c.tracker.add(`Account:${r.Id}`, async () => {
        const d = (await c.conn.sobject("Account").destroy(r.Id as string)) as SaveResult;
        if (!d.success) throw new Error("Delete failed");
      });
    c.result.observations = {
      statuses: response.compositeResponse?.map((r) => ({
        referenceId: r.referenceId,
        httpStatusCode: r.httpStatusCode,
        errorCodes: Array.isArray(r.body)
          ? (r.body as Array<{ errorCode?: string }>).map((b) => b.errorCode)
          : [],
      })),
      rollbackCount: q.totalSize,
    };
    expect(
      c,
      "Failed composite subrequest present",
      response.compositeResponse?.some((r) => (r.httpStatusCode ?? 0) >= 400),
    );
    expect(c, "No committed rows", q.totalSize === 0);
  },
  async permissions(c) {
    const token = c.env[`${c.prefix}_RESTRICTED_TOKEN`];
    if (!token) throw new Blocked(`Missing ${c.prefix}_RESTRICTED_TOKEN`);
    const restricted = new Connection({
      instanceUrl: c.conn.instanceUrl,
      accessToken: token,
      version: c.conn.version,
    });
    const admin = await c.conn.sobject("Account").describe(),
      limited = await restricted.sobject("Account").describe();
    const allowed = (await restricted.query(
      "SELECT Id,Name FROM Account LIMIT 1",
    )) as QueryResponse;
    expect(
      c,
      "Restricted user retains allowed Account access",
      Array.isArray(allowed.records) &&
        limited.fields.some((f) => f.name === "Name"),
    );
    const err = await errorOf(() =>
      restricted.query("SELECT Compat_Text__c FROM Account LIMIT 1"),
    );
    const adminVisible = admin.fields.some((f) => f.name === "Compat_Text__c");
    const restrictedVisible = limited.fields.some(
      (f) => f.name === "Compat_Text__c",
    );
    c.result.observations = {
      adminVisible,
      restrictedVisible,
      queryError: err,
    };
    expect(c, "Admin sees field", adminVisible);
    expect(
      c,
      "Restricted field omitted",
      !restrictedVisible,
    );
    expect(
      c,
      "Restricted query denied",
      err.errorCode === "INVALID_FIELD" ||
        err.errorCode === "INSUFFICIENT_ACCESS",
    );
  },
  async errors(c) {
    const id = await create(
      c,
      "Account",
      { Name: `SC ${c.runId} errors` },
      "account",
    );
    await c.conn.sobject("Account").destroy(id);
    const errors = {
      invalidField: await errorOf(() =>
        c.conn.query("SELECT Compat_NoSuchField__c FROM Account"),
      ),
      malformed: await errorOf(() => c.conn.query("SELECT FROM")),
      missing: await errorOf(() => c.conn.sobject("Account").retrieve(id)),
    };
    c.result.observations = errors;
    expect(
      c,
      "Invalid field error",
      errors.invalidField.errorCode === "INVALID_FIELD",
    );
    expect(
      c,
      "Malformed query error",
      errors.malformed.errorCode === "MALFORMED_QUERY",
    );
    expect(
      c,
      "Missing record error",
      MISSING_RECORD_CODES.includes(errors.missing.errorCode ?? ""),
    );
  },
  async locale(c) {
    const id = c.env[`${c.prefix}_LOCALE_USER_ID`];
    if (!id || !/^005[a-zA-Z0-9]{12}(?:[a-zA-Z0-9]{3})?$/.test(id))
      throw new Blocked(
        `Missing/invalid ${c.prefix}_LOCALE_USER_ID for dedicated fixture user`,
      );
    const before = (await c.conn.sobject("User").retrieve(id)) as RecordRow;
    const original = {
      Id: id,
      LanguageLocaleKey: before.LanguageLocaleKey,
      LocaleSidKey: before.LocaleSidKey,
      TimeZoneSidKey: before.TimeZoneSidKey,
    };
    await c.tracker.add(
      `User-settings:${id}`,
      async () => {
        const r = (await c.conn.sobject("User").update(original)) as SaveResult;
        if (!r.success) throw new Error("Settings restore failed");
      },
      { original },
    );
    const r = (await c.conn.sobject("User").update({
      Id: id,
      LanguageLocaleKey: "fr",
      LocaleSidKey: "en_US",
      TimeZoneSidKey: "America/New_York",
    })) as SaveResult;
    expect(c, "Language settings updated", r.success);
    const after = (await c.conn.sobject("User").retrieve(id)) as RecordRow;
    c.result.observations = {
      LanguageLocaleKey: after.LanguageLocaleKey,
      LocaleSidKey: after.LocaleSidKey,
      TimeZoneSidKey: after.TimeZoneSidKey,
    };
    expect(
      c,
      "Independent settings preserved",
      after.LanguageLocaleKey === "fr" &&
        after.LocaleSidKey === "en_US" &&
        after.TimeZoneSidKey === "America/New_York",
    );
  },
};
