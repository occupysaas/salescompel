import { open, readFile, lstat } from "node:fs/promises";
import { request as httpsRequest } from "node:https";
import { join } from "node:path";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
export interface FixtureEvent {
  schemaVersion: 1;
  runId: string;
  operationId: string;
  principal: "admin" | "restricted" | "other";
  object: "Account" | "Contact";
  names: string[];
  kind: "intent" | "known" | "deleted" | "absent";
  id?: string;
}
export interface Response {
  status: number;
  body: any;
}
export type Requester = (
  c: GateContext,
  role: FixtureEvent["principal"],
  path: string,
  method?: string,
  body?: unknown,
) => Promise<Response>;
export async function candidateRequest(
  c: GateContext,
  role: FixtureEvent["principal"],
  path: string,
  method = "GET",
  body?: unknown,
): Promise<Response> {
  if (!path.startsWith("/services/")) throw new Error("RESULT");
  const url = new URL(c.url);
  if (
    url.protocol !== "https:" ||
    !["localhost", "127.0.0.1"].includes(url.hostname)
  )
    throw new Error("SOURCE");
  const ca = await readFile(join(c.scratch, "cert.pem"));
  return new Promise((resolve, reject) => {
    const req = httpsRequest(
      new URL(path, url),
      {
        method,
        ca,
        headers: {
          Authorization: `Bearer ${c.tokens[role]}`,
          "Content-Type": "application/json",
        },
        timeout: 15000,
      },
      (res) => {
        const chunks: Buffer[] = [];
        let size = 0;
        res.on("data", (d) => {
          size += d.length;
          if (size > 8 * 1024 * 1024) req.destroy(new Error("RESULT"));
          else chunks.push(d);
        });
        res.on("end", () => {
          try {
            const text = Buffer.concat(chunks).toString("utf8");
            resolve({
              status: res.statusCode ?? 0,
              body: text ? JSON.parse(text) : null,
            });
          } catch {
            reject(new Error("RESULT"));
          }
        });
        res.on("error", () => reject(new Error("PROCESS")));
      },
    );
    req.on("timeout", () => req.destroy(new Error("TIMEOUT")));
    req.on("error", () => reject(new Error("PROCESS")));
    req.end(body === undefined ? undefined : JSON.stringify(body));
  });
}
export async function recordFixtureEvent(path: string, e: FixtureEvent) {
  if (
    e.schemaVersion !== 1 ||
    !/^[a-f0-9]{24}$/.test(e.runId) ||
    !e.operationId ||
    !["Account", "Contact"].includes(e.object) ||
    !["admin", "restricted", "other"].includes(e.principal) ||
    !["intent", "known", "deleted", "absent"].includes(e.kind) ||
    !e.names.length ||
    e.names.some((n) => typeof n !== "string" || n.length > 255) ||
    (e.id && !/^[a-zA-Z0-9]{18}$/.test(e.id))
  )
    throw new Error("FIXTURES");
  const info = await lstat(path);
  if (!info.isFile() || info.isSymbolicLink() || info.mode & 0o077)
    throw new Error("FIXTURES");
  const f = await open(path, "a");
  try {
    await f.writeFile(JSON.stringify(e) + "\n");
    await f.sync();
  } finally {
    await f.close();
  }
}
export async function cleanupFixtures(
  c: GateContext,
  request: Requester = candidateRequest,
) {
  let unknownIntents = 0,
    records = 0,
    verified = true;
  try {
    const text = await readFile(c.fixtureJournal, "utf8");
    if (text && !text.endsWith("\n")) throw new Error("FIXTURES");
    const states = new Map<string, FixtureEvent>();
    const ids = new Set<string>();
    for (const raw of text.trim().split("\n").filter(Boolean)) {
      const e = JSON.parse(raw) as FixtureEvent;
      if (e.runId !== c.runId) throw new Error("FIXTURES");
      const prev = states.get(e.operationId);
      if (e.kind === "intent") {
        if (prev) throw new Error("FIXTURES");
        states.set(e.operationId, e);
      } else {
        if (
          !prev ||
          e.object !== prev.object ||
          e.principal !== prev.principal ||
          JSON.stringify(e.names) !== JSON.stringify(prev.names) ||
          !e.id ||
          !/^[A-Za-z0-9]{18}$/.test(e.id) ||
          (prev.id && prev.id !== e.id)
        )
          throw new Error("FIXTURES");
        if (e.kind === "known" && !prev.id) {
          if (ids.has(e.id)) throw new Error("FIXTURES");
          ids.add(e.id);
        }
        states.set(e.operationId, e);
      }
    }
    for (const e of [...states.values()].sort(
      (a, b) =>
        (a.object === "Contact" ? 0 : 1) - (b.object === "Contact" ? 0 : 1),
    )) {
      if (!e.id) {
        unknownIntents++;
        verified = false;
        continue;
      }
      records++;
      const path = `/services/data/v65.0/sobjects/${e.object}/${e.id}`;
      try {
        let r = await request(c, e.principal, path);
        if (r.status !== 404) {
          if (
            r.status !== 200 ||
            !e.names.includes(
              r.body?.[e.object === "Account" ? "Name" : "LastName"],
            )
          )
            throw new Error("FIXTURES");
          r = await request(c, e.principal, path, "DELETE");
          if (![204, 404].includes(r.status)) throw new Error("CLEANUP");
          await recordFixtureEvent(c.fixtureJournal, { ...e, kind: "deleted" });
        }
        const absent = await request(c, e.principal, path);
        if (absent.status !== 404) throw new Error("CLEANUP");
        await recordFixtureEvent(c.fixtureJournal, { ...e, kind: "absent" });
      } catch {
        verified = false;
      }
    }
  } catch {
    verified = false;
    unknownIntents++;
  }
  return {
    verified,
    records,
    unknownIntents,
    retainedPrincipals: true as const,
    processesStopped: false,
  };
}
export async function createFixture(
  c: GateContext,
  operationId: string,
  object: FixtureEvent["object"],
  data: Record<string, any>,
  names: string[],
  principal: FixtureEvent["principal"] = "admin",
) {
  const e: FixtureEvent = {
    schemaVersion: 1,
    runId: c.runId,
    operationId,
    principal,
    object,
    names,
    kind: "intent",
  };
  await recordFixtureEvent(c.fixtureJournal, e);
  const r = await candidateRequest(
    c,
    principal,
    `/services/data/v65.0/sobjects/${object}`,
    "POST",
    data,
  );
  if (r.status !== 201 || !r.body?.success || typeof r.body.id !== "string")
    throw new Error("ASSERTION");
  await recordFixtureEvent(c.fixtureJournal, {
    ...e,
    kind: "known",
    id: r.body.id,
  });
  return r.body.id as string;
}
