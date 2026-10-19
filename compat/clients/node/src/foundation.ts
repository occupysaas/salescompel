import { setTimeout as delay } from "node:timers/promises";
import { wire, type WireResponse } from "./wire.ts";
import { create, expect, Blocked, type Context } from "./scenarios.ts";
import { runExpiryObservation } from "./foundation-policy.ts";

function base(c: Context) {
  return `/services/data/v${c.env.COMPAT_API_VERSION}/`;
}
function owned(c: Context, label: string) {
  const value = `Compat_${c.runId}_${c.result.id}_${label}`;
  c.result.identities[`value:${label}`] = value;
  return value;
}
function remember(c: Context, label: string, id: string, object = "Account") {
  c.result.identities[label] = id;
  c.result.identities[`url:${label}`] = `${base(c)}sobjects/${object}/${id}`;
  c.result.identities[`short:${label}`] = id.slice(0, 15);
}
async function account(
  c: Context,
  name: string,
  label: string,
  fields: Record<string, unknown> = {},
) {
  const id = await create(c, "Account", { Name: name, ...fields }, label);
  remember(c, label, id);
  return id;
}
function path(c: Context, object: string, id: string) {
  return `${base(c)}sobjects/${object}/${encodeURIComponent(id)}`;
}
function query(c: Context, soql: string) {
  return `${base(c)}query?q=${encodeURIComponent(soql)}`;
}
function literal(value: string) {
  return value.replaceAll("\\", "\\\\").replaceAll("'", "\\'");
}
async function request(
  c: Context,
  p: string,
  method = "GET",
  body?: unknown,
  headers: Record<string, string> = {},
  token?: string,
) {
  return wire(
    new URL(c.env[`${c.prefix}_URL`]!),
    token ?? c.env[`${c.prefix}_TOKEN`]!,
    p,
    {
      method,
      headers,
      ...(body === undefined
        ? {}
        : { body: typeof body === "string" ? body : JSON.stringify(body) }),
    },
  );
}
function summary(c: Context, value: any): any {
  if (Array.isArray(value)) return value.map((v) => summary(c, v));
  if (!value || typeof value !== "object") return value;
  const out: Record<string, unknown> = {};
  // Retain shape and selected fixture values. Never publish reference message prose.
  for (const [key, item] of Object.entries(value)) {
    if (key === "message") {
      out.messageType = typeof item;
      continue;
    }
    if (key === "nextRecordsUrl") {
      if (
        typeof item !== "string" ||
        !/^\/services\/data\/v\d+\.0\/query\/[^/?#]+$/.test(item)
      )
        throw new Error("Unexpected continuation route");
      const existing = Object.entries(c.result.identities).find(
        ([, v]) => v === item,
      );
      if (!existing)
        c.result.identities[
          `locator:${Object.keys(c.result.identities).filter((k) => k.startsWith("locator:")).length}`
        ] = item;
      out[key] = {
        route: item.replace(/\/query\/[^/]+$/, "/query/{queryLocator}"),
        value: item,
      };
      continue;
    }
    if (
      [
        "query",
        "sobjects",
        "totalSize",
        "done",
        "records",
        "attributes",
        "type",
        "url",
        "Id",
        "id",
        "Name",
        "name",
        "FirstName",
        "LastName",
        "AccountId",
        "Account",
        "Compat_Text__c",
        "Compat_Key__c",
        "success",
        "errors",
        "errorCode",
        "fields",
      ].includes(key)
    )
      out[key] = summary(c, item);
  }
  return out;
}
export function recordWireObservation(
  c: Context,
  label: string,
  response: WireResponse,
) {
  const observations = c.result.observations as Record<string, unknown>;
  observations[label] = {
    status: response.status,
    body: summary(c, response.body),
  };
}
async function observe(
  c: Context,
  label: string,
  p: string,
  method = "GET",
  body?: unknown,
  headers: Record<string, string> = {},
  token?: string,
) {
  const response = await request(c, p, method, body, headers, token);
  expect(
    c,
    `${label}: bounded JSON HTTP response`,
    response.status >= 200 &&
      response.status < 500 &&
      !(response.body as any)?.nonJson,
  );
  const measured =
    p.endsWith("/describe") && response.status === 200
      ? {
          ...response,
          body: {
            name: (response.body as any).name,
            fields: (response.body as any).fields
              .filter((f: any) =>
                ["Id", "Name", "Compat_Text__c", "Compat_Key__c"].includes(
                  f.name,
                ),
              )
              .map((f: any) => ({ name: f.name, type: f.type })),
          },
        }
      : response;
  recordWireObservation(c, label, measured);
  return response;
}
async function pages(
  c: Context,
  label: string,
  p: string,
  headers: Record<string, string> = {},
) {
  const ids: string[] = [];
  const sizes: number[] = [];
  let total: number | undefined;
  for (let i = 0; i < 32; i++) {
    const r = await observe(c, `${label}:${i}`, p, "GET", undefined, headers);
    const b = r.body as any;
    expect(
      c,
      `${label}:${i}: query page`,
      r.status === 200 &&
        Array.isArray(b?.records) &&
        typeof b.done === "boolean" &&
        Number.isInteger(b.totalSize),
    );
    if (total === undefined) total = b.totalSize;
    expect(c, `${label}:${i}: stable total`, b.totalSize === total);
    sizes.push(b.records.length);
    for (const row of b.records) if (row.Id) ids.push(row.Id);
    if (b.done) {
      expect(
        c,
        `${label}: record count equals total`,
        sizes.reduce((a, b) => a + b, 0) === total,
      );
      expect(
        c,
        `${label}: unique record identities`,
        new Set(ids).size === ids.length,
      );
      return { ids, sizes, total };
    }
    expect(
      c,
      `${label}:${i}: continuation required`,
      typeof b.nextRecordsUrl === "string",
    );
    p = b.nextRecordsUrl;
  }
  throw new Blocked("Query traversal exceeded 32-page probe bound");
}
async function firstCursor(c: Context, name: string, label: string) {
  const r = await observe(
    c,
    label,
    query(
      c,
      `SELECT Id, Name, Compat_Text__c FROM Account WHERE Name = '${literal(name)}' ORDER BY Compat_Text__c`,
    ),
    "GET",
    undefined,
    { "Sforce-Query-Options": "batchSize=200" },
  );
  // The foundation grammar has one ordering field; ties are a separate probe.
  const b = r.body as any;
  expect(
    c,
    `${label}: incomplete first page`,
    r.status === 200 &&
      b.done === false &&
      typeof b.nextRecordsUrl === "string",
  );
  return b.nextRecordsUrl as string;
}

export const foundationScenarios: Record<
  string,
  (c: Context) => Promise<void>
> = {
  async "foundation-routes"(c) {
    const name = owned(c, "routes"),
      id = await account(c, name, "route-account");
    const versions = await request(c, "/services/data");
    expect(
      c,
      "requested API version is advertised",
      versions.status === 200 &&
        Array.isArray(versions.body) &&
        versions.body.some(
          (v: any) =>
            v.version === c.env.COMPAT_API_VERSION &&
            v.url === base(c).slice(0, -1),
        ),
    );
    (c.result.observations as any).versions = {
      status: versions.status,
      requestedVersionAdvertised: true,
    };
    for (const [label, p] of [
      ["resources", base(c).slice(0, -1)],
      ["global", base(c) + "sobjects"],
      ["describe", base(c) + "sobjects/Account/describe"],
      ["read", path(c, "Account", id)],
    ])
      await observe(c, label, p);
    await observe(c, "patch", path(c, "Account", id), "PATCH", {
      Compat_Text__c: "route-patch",
    });
    await pages(
      c,
      "query",
      query(c, `SELECT Id, Name FROM Account WHERE Name = '${literal(name)}'`),
    );
    const key = owned(c, "external") + "/percent%é";
    c.result.identities.external = key;
    // Seed through journaled CRUD, then use PATCH upsert update to avoid an unjournaled create.
    const external = await account(c, name, "external-account", {
      Compat_Key__c: key,
    });
    await c.tracker.intent("Account", { Name: name, Compat_Key__c: key });
    const upsert = await observe(
      c,
      "escaped-upsert",
      `${base(c)}sobjects/Account/Compat_Key__c/${encodeURIComponent(key)}`,
      "PATCH",
      { Name: name },
    );
    const returnedId = (upsert.body as any)?.id;
    if (
      upsert.status === 201 &&
      typeof returnedId === "string" &&
      returnedId !== external
    ) {
      remember(c, "unexpected-upsert", returnedId);
      await c.tracker.add(`Account:${returnedId}`, async () => {
        const removed = await request(
          c,
          path(c, "Account", returnedId),
          "DELETE",
        );
        if (removed.status !== 204 && removed.status !== 404)
          throw new Error("Upsert fixture cleanup failed");
      });
    }
    await observe(c, "escaped-state", path(c, "Account", external));
    const info = await request(c, "/services/oauth2/userinfo");
    expect(
      c,
      "userinfo identifies own user",
      info.status === 200 && typeof (info.body as any)?.user_id === "string",
    );
    const user = (info.body as any).user_id;
    for (const version of ["65.0", "51.0"]) {
      const own = await request(
        c,
        `/services/data/v${version}/sobjects/User/${user}`,
      );
      expect(c, `own-user ${version} response`, own.status === 200);
      (c.result.observations as any)[`own-user:${version}`] = {
        status: own.status,
        hasOwnId: (own.body as any).Id === user,
      };
    }
    await observe(c, "delete", path(c, "Account", id), "DELETE");
    await observe(c, "deleted-read", path(c, "Account", id));
  },
  async "foundation-pagination"(c) {
    const name = owned(c, "pages");
    for (let i = 0; i < 2001; i++)
      await account(c, name, `page-${i}`, {
        Compat_Text__c: String(i).padStart(5, "0"),
      });
    const statement = `SELECT Id, Name, Compat_Text__c FROM Account WHERE Name = '${literal(name)}' ORDER BY Compat_Text__c`;
    const normal = await pages(c, "default", query(c, statement));
    expect(c, "default covers all owned rows", normal.total === 2001);
    await pages(c, "limit5000", query(c, statement + " LIMIT 5000"));
    await pages(c, "batch200", query(c, statement), {
      "Sforce-Query-Options": "batchSize=200",
    });
    await pages(c, "exact", query(c, statement + " LIMIT 200"), {
      "Sforce-Query-Options": "batchSize=200",
    });
    await pages(
      c,
      "empty",
      query(
        c,
        `SELECT Id FROM Account WHERE Name = '${literal(owned(c, "absent"))}'`,
      ),
    );
    await pages(
      c,
      "no-id",
      query(
        c,
        `SELECT Name FROM Account WHERE Name = '${literal(name)}' ORDER BY Compat_Text__c LIMIT 201`,
      ),
      { "Sforce-Query-Options": "batchSize=200" },
    );
    for (const batch of ["1", "199", "2000", "2001", "-1", "abc", ""])
      await observe(
        c,
        `batch:${batch}`,
        query(c, statement + " LIMIT 201"),
        "GET",
        undefined,
        { "Sforce-Query-Options": `batchSize=${batch}` },
      );
    const parent = await account(c, owned(c, "parent"), "parent");
    const contact = await create(
      c,
      "Contact",
      { LastName: owned(c, "contact"), AccountId: parent },
      "contact",
    );
    remember(c, "contact", contact, "Contact");
    await observe(
      c,
      "parent-projection",
      query(c, `SELECT Id, Account.Name FROM Contact WHERE Id = '${contact}'`),
    );
  },
  async "foundation-cursors"(c) {
    const fresh = c.env[`${c.prefix}_FRESH_TOKEN`],
      restricted = c.env[`${c.prefix}_RESTRICTED_TOKEN`];
    if (!fresh || !restricted || fresh === c.env[`${c.prefix}_TOKEN`])
      throw new Blocked(
        "Independent fresh same-user token and restricted token required",
      );
    const info = await request(c, "/services/oauth2/userinfo");
    const freshInfo = await request(
      c,
      "/services/oauth2/userinfo",
      "GET",
      undefined,
      {},
      fresh,
    );
    expect(
      c,
      "fresh session has same user",
      info.status === 200 &&
        freshInfo.status === 200 &&
        (info.body as any).user_id === (freshInfo.body as any).user_id &&
        (info.body as any).organization_id ===
          (freshInfo.body as any).organization_id,
    );
    const name = owned(c, "cursors");
    const ids: string[] = [];
    for (let i = 0; i < 201; i++)
      ids.push(
        await account(c, name, `cursor-${i}`, {
          Compat_Text__c: String(i).padStart(4, "0"),
        }),
      );
    const locator = await firstCursor(c, name, "initial");
    await observe(c, "continuation", locator);
    await observe(c, "replay", locator);
    await Promise.all(
      ["parallel-a", "parallel-b"].map((label) => observe(c, label, locator)),
    );
    await observe(c, "fresh-session", locator, "GET", undefined, {}, fresh);
    await observe(c, "other-user", locator, "GET", undefined, {}, restricted);
    await observe(
      c,
      "unknown-locator",
      base(c) + "query/000000000000000AAA-200",
    );
    await observe(c, "malformed-locator", base(c) + "query/invalid");
    const mutation = await firstCursor(c, name, "before-mutation");
    await observe(c, "update-pending", path(c, "Account", ids[200]), "PATCH", {
      Compat_Text__c: "changed",
    });
    await observe(c, "delete-pending", path(c, "Account", ids[199]), "DELETE");
    await account(c, name, "inserted", { Compat_Text__c: "0000" });
    await observe(c, "after-mutation", mutation);
    const tieName = owned(c, "ties");
    for (let i = 0; i < 201; i++)
      await account(c, tieName, `tie-${i}`, { Compat_Text__c: "same" });
    await pages(
      c,
      "ties",
      query(
        c,
        `SELECT Id, Compat_Text__c FROM Account WHERE Name = '${literal(tieName)}' ORDER BY Compat_Text__c`,
      ),
      { "Sforce-Query-Options": "batchSize=200" },
    );
    try {
      const expiry = await runExpiryObservation(
        c.collectionPolicy ?? "observed-only",
        () => firstCursor(c, name, "expiry-start"),
        () => delay(16 * 60 * 1000),
        async (locator) =>
          (await observe(c, "after-16-minutes", locator)).status,
      );
      if (expiry === "unmeasured")
        throw new Blocked(
          "Cursor expiry unmeasured under foundation-query-result-lifetime",
        );
    } catch (error) {
      if (error instanceof Blocked) throw error;
      if (
        error instanceof Error &&
        error.message.includes("16-minute observation bound")
      )
        throw new Blocked(error.message);
      throw error;
    }
  },
  async "foundation-ids"(c) {
    const name = owned(c, "ids"),
      id = await account(c, name, "id-account");
    const short = id.slice(0, 15);
    for (const [label, value] of [
      ["short", short],
      ["long", id],
      ["lower", id.toLowerCase()],
      ["bad-checksum", id.slice(0, 15) + "ZZZ"],
      ["short-length", short.slice(0, 14)],
      ["wrong-prefix", "003" + id.slice(3)],
    ]) {
      await observe(c, `${label}:read`, path(c, "Account", value));
      await observe(
        c,
        `${label}:query`,
        query(c, `SELECT Id, Name FROM Account WHERE Id = '${literal(value)}'`),
      );
    }
    await observe(c, "short-update", path(c, "Account", short), "PATCH", {
      Compat_Text__c: "updated",
    });
    const contact = await create(
      c,
      "Contact",
      { LastName: owned(c, "id-contact") },
      "id-contact",
    );
    remember(c, "id-contact", contact, "Contact");
    await observe(c, "short-reference", path(c, "Contact", contact), "PATCH", {
      AccountId: short,
    });
    await observe(c, "reference-read", path(c, "Contact", contact));
    await observe(
      c,
      "invalid-reference",
      path(c, "Contact", contact),
      "PATCH",
      { AccountId: "invalid" },
    );
    await observe(c, "reference-after-invalid", path(c, "Contact", contact));
    await observe(c, "unlink", path(c, "Contact", contact), "PATCH", {
      AccountId: null,
    });
    await observe(c, "short-delete", path(c, "Account", short), "DELETE");
    await observe(c, "long-after-delete", path(c, "Account", id));
  },
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
        (positive.body as any).records?.some((r: any) => r.Id === id),
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
      (beforeDenied.body as any).Compat_Text__c ===
        (afterDenied.body as any).Compat_Text__c,
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
      const b = r.body as any;
      expect(
        c,
        "null equality query succeeds",
        r.status === 200 && Array.isArray(b.records),
      );
      for (const row of b.records)
        if (nullOwned.has(row.Id)) nullMatches.add(row.Id);
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
    (c.result.observations as any).nullEquality = {
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
  async "foundation-errors"(c) {
    const name = owned(c, "errors"),
      id = await account(c, name, "error-account");
    await observe(c, "malformed-json", path(c, "Account", id), "PATCH", "{");
    await observe(c, "trailing-json", path(c, "Account", id), "PATCH", "{} {}");
    await observe(c, "required-field", path(c, "Account", id), "PATCH", {
      Name: null,
    });
    await observe(c, "unknown-field", path(c, "Account", id), "PATCH", {
      Compat_Unknown__c: "bad",
    });
    const unchanged = await observe(
      c,
      "after-failed-writes",
      path(c, "Account", id),
    );
    expect(
      c,
      "invalid writes leave name unchanged",
      unchanged.status === 200 && (unchanged.body as any).Name === name,
    );
    await observe(
      c,
      "unknown-rest-object",
      base(c) + "sobjects/Compat_Unknown__c",
    );
    await observe(
      c,
      "unknown-soql-object",
      query(c, "SELECT Id FROM Compat_Unknown__c"),
    );
    await observe(c, "malformed-query", query(c, "SELECT FROM Account"));
    await observe(c, "invalid-id", path(c, "Account", "invalid"));
    await observe(c, "delete", path(c, "Account", id), "DELETE");
    await observe(c, "missing-record", path(c, "Account", id));
  },
};
