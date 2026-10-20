import { bodyFixture } from "./external-id-body-test-server.ts";
import type { TestContext } from "node:test";
import type { Transport } from "../src/external-id-recovery.ts";
import type { RecordState } from "../src/external-id-contract.ts";
export async function normalizationFixture(t: TestContext) {
  const x = await bodyFixture(t);
  const flags = {
    ...x.flags,
    exactKeys: false,
    rewrite: false,
    uppercase: false,
    missingCreatedId: false,
    lossAt: 0,
    rejectedMutation: false,
    hiddenDuplicate: false,
    seedFailure: false,
    malformedSuccess: false,
    reverseRows: false,
    prose: "synthetic duplicate",
    statusOverride: 0,
    postErrors: [] as unknown,
  };
  let mutation = 0,
    counter = 0;
  const calls: { path: string; method: string; body: any }[] = [];
  const canonical = (v: string) =>
    flags.exactKeys
      ? v
      : v.trim().normalize("NFC").toLowerCase().replaceAll("ß", "ss");
  const send = (status: number, body: unknown = null) => ({
    status,
    body,
    headers: {},
    bodyPresent: body !== null,
  });
  const transport: Transport = {
    async request(path, init) {
      const method = init.method ?? "GET",
        body = init.body ? JSON.parse(String(init.body)) : undefined;
      calls.push({ path, method, body });
      Object.assign(x.flags, flags);
      if (["GET", "DELETE"].includes(method)) {
        const r = await x.transport.request(path, init);
        if (flags.reverseRows && Array.isArray((r.body as any)?.records))
          (r.body as any).records.reverse();
        return structuredClone(r);
      }
      mutation++;
      if (flags.seedFailure && body.Name.endsWith("-seed"))
        return send(400, [
          { errorCode: "SEED_REJECTED", message: "synthetic" },
        ]);
      const upsert = path.includes("/Compat_Key__c/"),
        id = path.split("/").at(-1)!;
      const key = upsert ? decodeURIComponent(id) : body.Compat_Key__c;
      let row = upsert
        ? [...x.rows.values()].find(
            (r) => canonical(r.Compat_Key__c ?? "") === canonical(key),
          )
        : method === "PATCH"
          ? x.rows.get(id)
          : undefined;
      const duplicate = [...x.rows.values()].find(
        (r) =>
          r.Id !== row?.Id &&
          canonical(r.Compat_Key__c ?? "") === canonical(key),
      );
      const reject = !!duplicate,
        created = !row;
      if (!reject || flags.rejectedMutation || flags.hiddenDuplicate) {
        if (!row) {
          const rid = "001" + String(++counter).padStart(15, "0");
          row = {
            Id: rid,
            Name: body.Name,
            Compat_Key__c: flags.rewrite
              ? "rewritten-" + key
              : flags.uppercase
                ? key.trim().toUpperCase()
                : key.trim(),
            Compat_Text__c: null,
          };
          x.rows.set(rid, row);
        }
        row.Name = body.Name;
        if (!upsert)
          row.Compat_Key__c = flags.rewrite
            ? "rewritten-" + key
            : flags.uppercase
              ? key.trim().toUpperCase()
              : key.trim();
      }
      if (flags.lossAt === mutation) throw new Error("RESPONSE_LOST");
      if (reject)
        return send(400, [
          {
            message: flags.prose,
            errorCode: "DUPLICATE_VALUE",
            fields: ["Compat_Key__c"],
          },
        ]);
      if (flags.malformedSuccess) return send(201, { success: true });
      if (method === "PATCH" && !upsert) return send(204);
      return send(upsert ? (created ? 201 : 200) : 201, {
        ...(flags.missingCreatedId ? {} : { id: row!.Id }),
        success: true,
        errors: upsert ? [] : flags.postErrors,
        ...(upsert ? { created: created } : {}),
      });
    },
  };
  return { ...x, flags, calls, transport };
}
