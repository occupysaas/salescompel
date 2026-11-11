import { wire, type WireResponse } from "./wire.ts";
import { create, expect, Blocked, type Context } from "./scenarios.ts";

export function base(c: Context) {
  return `/services/data/v${c.env.COMPAT_API_VERSION}/`;
}
export function owned(c: Context, label: string) {
  const value = `Compat_${c.runId}_${c.result.id}_${label}`;
  c.result.identities[`value:${label}`] = value;
  return value;
}
export function remember(
  c: Context,
  label: string,
  id: string,
  object = "Account",
) {
  c.result.identities[label] = id;
  c.result.identities[`url:${label}`] = `${base(c)}sobjects/${object}/${id}`;
  c.result.identities[`short:${label}`] = id.slice(0, 15);
}
export async function account(
  c: Context,
  name: string,
  label: string,
  fields: Record<string, unknown> = {},
) {
  const id = await create(c, "Account", { Name: name, ...fields }, label);
  remember(c, label, id);
  return id;
}
export function path(c: Context, object: string, id: string) {
  return `${base(c)}sobjects/${object}/${encodeURIComponent(id)}`;
}
export function query(c: Context, soql: string) {
  return `${base(c)}query?q=${encodeURIComponent(soql)}`;
}
export function literal(value: string) {
  return value.replaceAll("\\", "\\\\").replaceAll("'", "\\'");
}
export async function request(
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
interface DescribeField {
  name: string;
  type: string;
}

interface QueryResultBody {
  records?: Array<Record<string, unknown>>;
  done?: boolean;
  totalSize?: number;
  nextRecordsUrl?: string;
}

function isRecord(v: unknown): v is Record<string, unknown> {
  return typeof v === "object" && v !== null && !Array.isArray(v);
}

export function summary(c: Context, value: unknown): unknown {
  if (Array.isArray(value)) return value.map((v) => summary(c, v));
  if (!value || typeof value !== "object") return value;
  const out: Record<string, unknown> = {};
  // Retain shape and selected fixture values. Never publish reference message prose.
  for (const [key, item] of Object.entries(value as Record<string, unknown>)) {
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
export async function observe(
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
      !(isRecord(response.body) && response.body.nonJson),
  );
  const measured =
    p.endsWith("/describe") && response.status === 200
      ? {
          ...response,
          body: (() => {
            const d = response.body as {
              name?: string;
              fields?: DescribeField[];
            };
            return {
              name: d.name,
              fields: (d.fields ?? [])
                .filter((f) =>
                  ["Id", "Name", "Compat_Text__c", "Compat_Key__c"].includes(
                    f.name,
                  ),
                )
                .map((f) => ({ name: f.name, type: f.type })),
            };
          })(),
        }
      : response;
  recordWireObservation(c, label, measured);
  return response;
}
export async function pages(
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
    const b = r.body as QueryResultBody;
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
    sizes.push(b.records!.length);
    for (const row of b.records!) if (row.Id) ids.push(row.Id as string);
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
    p = b.nextRecordsUrl!;
  }
  throw new Blocked("Query traversal exceeded 32-page probe bound");
}
export async function firstCursor(c: Context, name: string, label: string) {
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
  const b = r.body as QueryResultBody;
  expect(
    c,
    `${label}: incomplete first page`,
    r.status === 200 &&
      b.done === false &&
      typeof b.nextRecordsUrl === "string",
  );
  return b.nextRecordsUrl as string;
}
