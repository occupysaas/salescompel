import { mkdtemp, rm } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import type { TestContext } from "node:test";
import {
  parseExternalIdConfig,
  type RecordState,
} from "../src/external-id-contract.ts";
import { profile } from "../src/validate.ts";
import type { Transport } from "../src/external-id-recovery.ts";
export async function bodyFixture(t: TestContext) {
  const config = parseExternalIdConfig({
    schemaVersion: 1,
    apiVersion: "65.0",
    profileHash: (await profile("rest-query-foundation")).hash,
    origin: "https://reference.example.invalid",
    administrator: { user: "u", org: "o" },
  });
  const dir = await mkdtemp(join(tmpdir(), "p11-body-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const rows = new Map<string, RecordState>(),
    calls: { path: string; method: string; body: any }[] = [];
  const flags = {
    originalHypothesis: false,
    missingId: false,
    hiddenInsert: false,
    mutateRejected: false,
    different: false,
    loss: false,
    wrongIdentity: false,
    malformed: false,
    deleteFailure: false,
    metadataBad: false,
    administratorBad: false,
    hideQueries: false,
    expire: false,
  };
  let counter = 0,
    active = 0,
    maxConcurrency = 0;
  const transport: Transport = {
    async request(path, init) {
      active++;
      maxConcurrency = Math.max(maxConcurrency, active);
      try {
        const method = init.method ?? "GET",
          body = init.body ? JSON.parse(String(init.body)) : undefined;
        calls.push({ path, method, body });
        const send = (status: number, body: unknown = null) => ({
          status,
          body,
          bodyPresent: body !== null,
          headers: {},
        });
        if (path.endsWith("userinfo"))
          return send(200, {
            user_id: flags.wrongIdentity ? "bad" : "u",
            organization_id: "o",
          });
        if (path.includes("/describe"))
          return send(200, {
            fields: [
              {
                name: "Compat_Key__c",
                type: "string",
                length: flags.metadataBad ? 79 : 80,
                externalId: true,
                unique: true,
                caseSensitive: false,
              },
            ],
          });
        if (path.includes("query?")) {
          const q = new URL(path, "https://x").searchParams.get("q")!;
          if (q.includes("FROM User"))
            return send(200, {
              done: true,
              totalSize: 1,
              records: [
                {
                  Id: "u",
                  IsActive: true,
                  Profile: {
                    PermissionsModifyAllData: !flags.administratorBad,
                  },
                },
              ],
            });
          const values = [...q.matchAll(/'([^']*)'/g)].map((m) => m[1]);
          const selected = flags.hideQueries
            ? []
            : [...rows.values()].filter(
                (r) =>
                  values.includes(r.Name) ||
                  values.includes(r.Compat_Key__c ?? ""),
              );
          return send(200, {
            done: !flags.malformed,
            totalSize: selected.length,
            records: selected,
          });
        }
        const id = path.split("/").at(-1)!;
        if (method === "GET")
          return send(
            rows.has(id) ? 200 : 404,
            rows.get(id) ?? [{ errorCode: "NOT_FOUND" }],
          );
        if (method === "DELETE") {
          if (flags.deleteFailure) return send(500, []);
          rows.delete(id);
          return send(204);
        }
        if (flags.expire) throw new Error("OPERATION_DEADLINE");
        const upsert = path.includes("/Compat_Key__c/"),
          key = upsert ? decodeURIComponent(id) : body.Compat_Key__c;
        let row = upsert
          ? [...rows.values()].find((r) => r.Compat_Key__c === key)
          : undefined;
        const supplied = upsert && Object.hasOwn(body, "Compat_Key__c"),
          succeeds =
            !supplied ||
            (!flags.originalHypothesis && body.Compat_Key__c === null);
        if (
          succeeds ||
          (flags.hiddenInsert && !row) ||
          (flags.mutateRejected && !!row) ||
          (flags.different &&
            typeof body.Compat_Key__c === "string" &&
            body.Compat_Key__c.endsWith("different"))
        ) {
          const created = !row;
          if (!row) {
            const rid = "001" + String(++counter).padStart(15, "0");
            row = {
              Id: rid,
              Name: body.Name,
              Compat_Key__c: key,
              Compat_Text__c: null,
            };
            rows.set(rid, row);
          }
          row.Name = body.Name;
          if (
            flags.different &&
            supplied &&
            typeof body.Compat_Key__c === "string" &&
            body.Compat_Key__c.endsWith("different")
          )
            row.Compat_Key__c = body.Compat_Key__c;
          if (flags.loss) throw new Error("RESPONSE_LOST");
          if (succeeds)
            return send(upsert ? (created ? 201 : 200) : 201, {
              ...(flags.missingId && upsert ? {} : { id: row.Id }),
              created,
              success: true,
              ...(upsert ? { errors: [] } : {}),
            });
        }
        if (flags.loss) throw new Error("RESPONSE_LOST");
        return send(400, [
          {
            errorCode:
              !flags.originalHypothesis &&
              typeof body.Compat_Key__c === "object" &&
              body.Compat_Key__c !== null
                ? "JSON_PARSER_ERROR"
                : "INVALID_FIELD",
            message: "synthetic rejection",
          },
        ]);
      } finally {
        active--;
      }
    },
  };
  return {
    config,
    dir,
    rows,
    calls,
    flags,
    transport,
    get maxConcurrency() {
      return maxConcurrency;
    },
  };
}
