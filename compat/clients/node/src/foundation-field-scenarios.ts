import { expect, Blocked, type Context } from "./scenarios.ts";
import {
  base,
  owned,
  account,
  path,
  query,
  literal,
  observe,
  pages,
  request,
} from "./foundation-helpers.ts";

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

interface QueryRow {
  Id?: string;
  [key: string]: unknown;
}

interface QueryBody {
  records?: QueryRow[];
  done?: boolean;
  nextRecordsUrl?: string;
}

export const foundationFieldScenarios: Record<
  string,
  (c: Context) => Promise<void>
> = {
  async "foundation-names"(c) {
    const name = owned(c, "names"),
      id = await account(c, name, "names-account");
    await observe(c, "lower-describe", base(c) + "sobjects/account/describe");
    await observe(c, "lower-path", path(c, "account", id));
    await observe(
      c,
      "lower-query",
      query(c, `select id, name from account where id = '${id}'`),
    );
    await observe(c, "lower-json", path(c, "Account", id), "PATCH", {
      compat_text__c: "lower",
    });
    await observe(c, "after-lower-json", path(c, "Account", id));
    await observe(
      c,
      "duplicate-alias",
      path(c, "Account", id),
      "PATCH",
      '{"Compat_Text__c":"first","compat_text__c":"second"}',
    );
    await observe(c, "after-duplicate-alias", path(c, "Account", id));
    await observe(
      c,
      "duplicate-raw",
      path(c, "Account", id),
      "PATCH",
      '{"Compat_Text__c":"first","Compat_Text__c":"second"}',
    );
    await observe(c, "after-duplicate-raw", path(c, "Account", id));
    const beforeDenied = await observe(
      c,
      "after-duplicates",
      path(c, "Account", id),
    );
    const restricted = c.env[`${c.prefix}_RESTRICTED_TOKEN`];
    if (!restricted)
      throw new Blocked("Restricted token required for name/permission probes");
    const positive = await observe(
      c,
      "restricted-positive",
      query(c, `SELECT Id, Name FROM Account WHERE Id = '${id}'`),
      "GET",
      undefined,
      {},
      restricted,
    );
    expect(
      c,
      "restricted persona can read the owned record",
      positive.status === 200 &&
        (positive.body as QueryBody).records?.some((r) => r.Id === id),
    );
    for (const [label, soql] of [
      ["select", `SELECT id, compat_text__c FROM account WHERE Id = '${id}'`],
      [
        "filter",
        `SELECT Id FROM Account WHERE compat_text__c = '${literal(owned(c, "denied"))}'`,
      ],
      [
        "order",
        `SELECT Id FROM Account WHERE Id = '${id}' ORDER BY compat_text__c`,
      ],
    ]) {
      const denied = await observe(
        c,
        `denied-${label}`,
        query(c, soql),
        "GET",
        undefined,
        {},
        restricted,
      );
      expect(c, `denied-${label}: field access rejected`, denied.status >= 400);
    }
    const deniedWrite = await observe(
      c,
      "denied-write",
      path(c, "account", id),
      "PATCH",
      { compat_text__c: "forbidden" },
      {},
      restricted,
    );
    expect(c, "restricted write rejected", deniedWrite.status >= 400);
    const afterDenied = await observe(
      c,
      "after-denied-write",
      path(c, "Account", id),
    );
    expect(
      c,
      "denied write leaves field unchanged",
      isRecord(beforeDenied.body) &&
        isRecord(afterDenied.body) &&
        beforeDenied.body.Compat_Text__c === afterDenied.body.Compat_Text__c,
    );
  },
  async "foundation-text-null"(c) {
    const name = owned(c, "text");
    for (const [label, text] of [
      ["mixed", "MiXeD"],
      ["accent", "École"],
      ["turkish", "İstanbul"],
      ["sharp", "Straße"],
    ]) {
      const value = owned(c, label) + text;
      await account(c, name, `text-${label}`, { Compat_Text__c: value });
      for (const [variant, v] of [
        ["exact", value],
        ["lower", value.toLowerCase()],
        ["upper", value.toUpperCase()],
        ["decomposed", value.normalize("NFD")],
      ]) {
        if (!Object.values(c.result.identities).includes(v))
          c.result.identities[`comparison:${label}:${variant}`] = v;
        await observe(
          c,
          `${label}:${variant}`,
          query(
            c,
            `SELECT Id, Compat_Text__c FROM Account WHERE Compat_Text__c = '${literal(v)}'`,
          ),
        );
      }
    }
    for (const [variant, value] of [
      ["exact", name],
      ["lower", name.toLowerCase()],
      ["upper", name.toUpperCase()],
    ])
      await observe(
        c,
        `name-equality:${variant}`,
        query(
          c,
          `SELECT Id FROM Account WHERE Name = '${literal(value)}' ORDER BY Compat_Text__c`,
        ),
      );
    await account(c, name, "null", { Compat_Text__c: null });
    await account(c, name, "blank", { Compat_Text__c: "" });
    for (const direction of ["ASC", "DESC"])
      for (const nulls of ["", " NULLS FIRST", " NULLS LAST"])
        await pages(
          c,
          `order:${direction}${nulls}`,
          query(
            c,
            `SELECT Id, Compat_Text__c FROM Account WHERE Name = '${literal(name)}' ORDER BY Compat_Text__c ${direction}${nulls}`,
          ),
        );
    // Select IDs only; retain membership for the two owned null fixtures, never unrelated identities.
    const nullOwned = new Set([
      c.result.identities.null,
      c.result.identities.blank,
    ]);
    const nullMatches = new Set<string>();
    let nullPath = query(
      c,
      "SELECT Id FROM Account WHERE Compat_Text__c = null",
    );
    let nullComplete = false;
    for (let i = 0; i < 32; i++) {
      const r = await request(c, nullPath);
      const b = r.body as QueryBody;
      expect(
        c,
        "null equality query succeeds",
        r.status === 200 && Array.isArray(b.records),
      );
      for (const row of b.records ?? [])
        if (row.Id && nullOwned.has(row.Id)) nullMatches.add(row.Id);
      if (b.done) {
        nullComplete = true;
        break;
      }
      if (typeof b.nextRecordsUrl !== "string")
        throw new Error("Missing null query continuation");
      nullPath = b.nextRecordsUrl;
    }
    if (!nullComplete)
      throw new Blocked("Null equality traversal exceeds 32-page bound");
    (c.result.observations as Record<string, unknown>).nullEquality = {
      nullMatches: nullMatches.has(c.result.identities.null),
      blankMatches: nullMatches.has(c.result.identities.blank),
    };
    await observe(
      c,
      "null-record",
      path(c, "Account", c.result.identities.null),
    );
    await observe(
      c,
      "blank-record",
      path(c, "Account", c.result.identities.blank),
    );
  },
};
