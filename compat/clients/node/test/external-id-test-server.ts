import { createServer } from "node:http";
import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { TestContext } from "node:test";
import { wire } from "../src/wire.ts";
import {
  parseExternalIdConfig,
  type RecordState,
} from "../src/external-id-contract.ts";
import { profile } from "../src/validate.ts";
import { base, type Transport } from "../src/external-id-recovery.ts";
export async function localProbe(t: TestContext) {
  const config = parseExternalIdConfig({
    schemaVersion: 1,
    apiVersion: "65.0",
    profileHash: (await profile("rest-query-foundation")).hash,
    origin: "https://reference.example.invalid",
    administrator: { user: "u", org: "o" },
  });
  const dir = await mkdtemp(join(tmpdir(), "p9-probe-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const rows = new Map<string, RecordState>();
  const calls: { path: string; method: string; body: any }[] = [];
  const flags = {
    wrongIdentity: false,
    identityMismatchAt: 0,
    identityCalls: 0,
    pagination: false,
    deleteFailure: false,
    delay: 0,
    missingId: false,
    beforeCommit: undefined as Promise<void> | undefined,
    onMutation: undefined as (() => void) | undefined,
  };
  let counter = 0;
  const server = createServer(async (req, res) => {
    let raw = "";
    for await (const b of req) raw += b;
    const body = raw ? JSON.parse(raw) : undefined;
    const url = new URL(req.url!, "http://local");
    const method = req.method!;
    calls.push({ path: req.url!, method, body });
    const send = (status: number, b?: unknown) => {
      res.writeHead(status, { "Content-Type": "application/json" });
      res.end(b === undefined ? undefined : JSON.stringify(b));
    };
    if (url.pathname.endsWith("userinfo")) {
      send(200, {
        user_id:
          flags.wrongIdentity ||
          ++flags.identityCalls === flags.identityMismatchAt
            ? "changed"
            : "u",
        organization_id: "o",
      });
      return;
    }
    if (url.pathname.endsWith("query")) {
      const q = url.searchParams.get("q")!;
      const marker = q.match(/Name = '([^']+)'/)?.[1];
      const variants = [...q.matchAll(/'((?:\\.|[^'])*)'/g)].map((m) =>
        m[1].replace(/\\(['\\])/g, "$1"),
      );
      const selected = [...rows.values()].filter(
        (r) =>
          r.Name === marker ||
          (q.includes(" IN ") && variants.includes(r.Compat_Key__c!)),
      );
      send(200, {
        done: !flags.pagination,
        totalSize: selected.length,
        records: selected,
      });
      return;
    }
    const parts = url.pathname.split("/");
    if (method === "DELETE") {
      if (flags.deleteFailure) {
        send(500, [{ errorCode: "SERVER_ERROR" }]);
        return;
      }
      rows.delete(parts.at(-1)!);
      send(204);
      return;
    }
    if (method === "GET") {
      const row = rows.get(parts.at(-1)!);
      send(row ? 200 : 404, row ?? [{ errorCode: "NOT_FOUND" }]);
      return;
    }
    const mutate = () => {
      let existing: RecordState | undefined;
      const key = parts.includes("Compat_Key__c")
        ? decodeURIComponent(parts.at(-1)!)
        : body?.Compat_Key__c;
      if (method === "PATCH")
        existing = parts.includes("Compat_Key__c")
          ? [...rows.values()].find((r) => r.Compat_Key__c === key)
          : rows.get(parts.at(-1)!);
      const created = !existing;
      const id = existing?.Id ?? "001" + String(++counter).padStart(15, "0");
      const row: RecordState = {
        Id: id,
        Name: body.Name,
        Compat_Key__c: key ?? existing?.Compat_Key__c ?? null,
        Compat_Text__c: null,
      };
      rows.set(id, row);
      send(
        created ? 201 : 204,
        created
          ? { ...(flags.missingId ? {} : { id }), success: true, created: true }
          : undefined,
      );
    };
    flags.onMutation?.();
    if (flags.beforeCommit) {
      await flags.beforeCommit;
      mutate();
    } else if (flags.delay) {
      setTimeout(mutate, flags.delay);
    } else mutate();
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  t.after(() => {
    server.closeAllConnections();
    server.close();
  });
  const origin = new URL(`http://127.0.0.1:${(server.address() as any).port}`);
  const transport: Transport = {
    request: (path, init) =>
      wire(origin, "synthetic", path, init, {
        maxBytes: 8 * 1024 * 1024,
        timeoutMs:
          flags.delay && init.method && init.method !== "GET" ? 20 : 1000,
      }),
    native: async (key, marker) => {
      const r = await wire(
        origin,
        "synthetic",
        base + "sobjects/Account/Compat_Key__c/" + encodeURIComponent(key),
        {
          method: "PATCH",
          body: JSON.stringify({ Name: marker, Compat_Key__c: key }),
        },
      );
      return r.body;
    },
  };
  return { config, dir, rows, calls, flags, transport };
}
