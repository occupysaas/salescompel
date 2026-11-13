import { setTimeout as delay } from "node:timers/promises";
import { create, expect, Blocked, type Context } from "./scenarios.ts";
import { runExpiryObservation } from "./foundation-policy.ts";
import {
  base,
  owned,
  remember,
  account,
  path,
  query,
  literal,
  request,
  observe,
  pages,
  firstCursor,
} from "./foundation-helpers.ts";

type JsonRecord = Record<string, unknown>;

export async function routesScenario(c: Context): Promise<void> {
  const name = owned(c, "routes"),
    id = await account(c, name, "route-account");
  const versions = await request(c, "/services/data");
  expect(
    c,
    "requested API version is advertised",
    versions.status === 200 &&
      Array.isArray(versions.body) &&
      versions.body.some(
        (v: JsonRecord) =>
          v.version === c.env.COMPAT_API_VERSION &&
          v.url === base(c).slice(0, -1),
      ),
  );
  (c.result.observations as JsonRecord).versions = {
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
  const returnedId = (upsert.body as JsonRecord)?.id;
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
    info.status === 200 &&
      typeof (info.body as JsonRecord)?.user_id === "string",
  );
  const user = (info.body as JsonRecord).user_id;
  for (const version of ["65.0", "51.0"]) {
    const own = await request(
      c,
      `/services/data/v${version}/sobjects/User/${user}`,
    );
    expect(c, `own-user ${version} response`, own.status === 200);
    (c.result.observations as JsonRecord)[`own-user:${version}`] = {
      status: own.status,
      hasOwnId: (own.body as JsonRecord).Id === user,
    };
  }
  await observe(c, "delete", path(c, "Account", id), "DELETE");
  await observe(c, "deleted-read", path(c, "Account", id));
}

export async function paginationScenario(c: Context): Promise<void> {
  const name = owned(c, "pages");
  for (let i = 0; i < 2001; i++)
    await account(c, name, `page-${i}`, {
      Compat_Text__c: String(i).padStart(5, "0"),
    });
  const soql = `SELECT Id, Name, Compat_Text__c FROM Account WHERE Name = '${literal(name)}' ORDER BY Compat_Text__c`;
  const normal = await pages(c, "default", query(c, soql));
  expect(c, "default covers all owned rows", normal.total === 2001);
  await pages(c, "limit5000", query(c, `${soql} LIMIT 5000`));
  await pages(c, "batch200", query(c, soql), {
    "Sforce-Query-Options": "batchSize=200",
  });
  await pages(c, "exact", query(c, `${soql} LIMIT 200`), {
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
  const limited = `${soql} LIMIT 201`;
  for (const batch of ["1", "199", "2000", "2001", "-1", "abc", ""])
    await observe(c, `batch:${batch}`, query(c, limited), "GET", undefined, {
      "Sforce-Query-Options": `batchSize=${batch}`,
    });
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
}

export async function cursorsScenario(c: Context): Promise<void> {
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
      (info.body as JsonRecord).user_id ===
        (freshInfo.body as JsonRecord).user_id &&
      (info.body as JsonRecord).organization_id ===
        (freshInfo.body as JsonRecord).organization_id,
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
  await observe(c, "unknown-locator", base(c) + "query/000000000000000AAA-200");
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
      async (locator) => (await observe(c, "after-16-minutes", locator)).status,
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
}

export async function idsScenario(c: Context): Promise<void> {
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
  await observe(c, "invalid-reference", path(c, "Contact", contact), "PATCH", {
    AccountId: "invalid",
  });
  await observe(c, "reference-after-invalid", path(c, "Contact", contact));
  await observe(c, "unlink", path(c, "Contact", contact), "PATCH", {
    AccountId: null,
  });
  await observe(c, "short-delete", path(c, "Account", short), "DELETE");
  await observe(c, "long-after-delete", path(c, "Account", id));
}

export async function errorsScenario(c: Context): Promise<void> {
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
    unchanged.status === 200 && (unchanged.body as JsonRecord).Name === name,
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
}
