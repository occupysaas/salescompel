import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import {
  collectMetadata,
  type DiscoveryReader,
} from "../src/metadata-discovery.ts";
import { readWithClient } from "../src/metadata-read-process.ts";
import { parseDiscoveryConfig } from "../src/metadata-inventory.ts";
import { profile } from "../src/validate.ts";
const config = async () =>
  parseDiscoveryConfig({
    schemaVersion: 1,
    apiVersion: "65.0",
    profile: "rest-query-foundation",
    profileHash: (await profile("rest-query-foundation")).hash,
    origin: "https://reference.example.invalid",
  });
const env = {
  COMPAT_REFERENCE_URL: "https://reference.example.invalid",
  COMPAT_REFERENCE_TOKEN: "admin-secret",
  COMPAT_REFERENCE_RESTRICTED_TOKEN: "restricted-secret",
};
function reader(events: string[]): DiscoveryReader {
  return {
    identity: async (p) => {
      events.push("identity:" + p);
      return { user: p, org: "synthetic-org" };
    },
    describe: async (p, o) => {
      events.push("describe:" + p + ":" + o);
      return {
        name: o,
        queryable: true,
        createable: p === "administrator",
        updateable: false,
        deletable: false,
        fields: [
          {
            name: "Id",
            type: "id",
            length: 18,
            nillable: false,
            externalId: false,
            referenceTo: [],
            createable: false,
            updateable: false,
          },
        ],
      };
    },
    metadata: async (t, n) => {
      events.push("read:" + t + ":" + n);
      return {
        fullName: n,
        type: "Text",
        length: 255,
        required: false,
        externalId: false,
        unique: false,
      };
    },
  };
}
test("verifyBothIdentitiesBeforeReads and collectOnlyFixedObjectsAndMembers", async () => {
  const events: string[] = [];
  const i = await collectMetadata(await config(), env, reader(events));
  assert.deepEqual(events.slice(0, 2), [
    "identity:administrator",
    "identity:restricted",
  ]);
  assert.deepEqual(
    events.filter((x) => x.startsWith("read:")),
    [
      "read:CustomField:Account.Compat_Text__c",
      "read:CustomField:Account.Compat_Key__c",
      "read:PermissionSet:Compat_Admin",
    ],
  );
  assert.equal(events.filter((x) => x.startsWith("describe:")).length, 4);
  assert.deepEqual(events.slice(-2), [
    "identity:administrator",
    "identity:restricted",
  ]);
  assert.ok(
    i.facts.some((x) => x.property === "updateable" && x.value === false),
  );
  assert.ok(
    i.gaps.some(
      (x) =>
        x.component === "Account.Compat_Text__c" &&
        x.code === "FIELD_NOT_VISIBLE",
    ),
  );
  assert.equal(i.outcome, "incomplete");
});
test("rejectCrossOrgAndSamePersona before any discovery reads", async () => {
  for (const mode of ["cross", "same", "changed"]) {
    const events: string[] = [];
    const r = reader(events);
    let calls = 0;
    r.identity = async (p) => {
      calls++;
      return {
        user: mode === "same" ? "one" : p,
        org:
          mode === "cross"
            ? p
            : mode === "changed" && calls > 2
              ? "other"
              : "org",
      };
    };
    const i = await collectMetadata(await config(), env, r);
    assert.equal(i.outcome, "blocked");
    if (mode !== "changed") assert.equal(events.length, 0);
  }
});
test("preserveUnavailableMetadata and redactAuthAndProse", async () => {
  const r = reader([]);
  r.metadata = async () => {
    throw new Error("admin-secret private Salesforce prose");
  };
  const i = await collectMetadata(await config(), env, r);
  assert.ok(i.gaps.some((x) => x.code === "READ_UNAVAILABLE"));
  assert.doesNotMatch(JSON.stringify(i), /admin-secret|Salesforce prose/);
});
test("metadataPostIsReadOnly through unmodified JSforce", async (t) => {
  let body = "",
    method = "",
    path = "";
  const server = createServer(async (req, res) => {
    method = req.method!;
    path = req.url!;
    for await (const chunk of req) body += chunk;
    res.setHeader("content-type", "text/xml");
    res.end(
      '<soapenv:Envelope xmlns:soapenv="http://schemas.xmlsoap.org/soap/envelope/"><soapenv:Body><readMetadataResponse xmlns="http://soap.sforce.com/2006/04/metadata"><result><records><fullName>Account.Compat_Text__c</fullName><type>Text</type><length>255</length></records></result></readMetadataResponse></soapenv:Body></soapenv:Envelope>',
    );
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  t.after(() => server.close());
  const result = await readWithClient({
    origin: `http://127.0.0.1:${(server.address() as any).port}`,
    token: "synthetic",
    kind: "metadata",
    type: "CustomField",
    name: "Account.Compat_Text__c",
  });
  assert.equal(method, "POST");
  assert.equal(path, "/services/Soap/m/65.0");
  assert.match(body, /<readMetadata[ >]/);
  assert.doesNotMatch(body, /<(?:create|update|delete|deploy|upsert)Metadata/);
  assert.equal(result.fullName, "Account.Compat_Text__c");
  await assert.rejects(
    readWithClient({
      origin: "https://reference.example.invalid",
      token: "synthetic",
      kind: "deploy",
    } as any),
    /read operation/,
  );
});
test("read process deadline and response bound fail closed", async (t) => {
  const server = createServer((req, res) => {
    if (req.url!.includes("Account"))
      res.end(JSON.stringify({ data: "x".repeat(10000) }));
  });
  await new Promise<void>((r) => server.listen(0, "127.0.0.1", r));
  t.after(() => {
    server.closeAllConnections();
    server.close();
  });
  const origin = `http://127.0.0.1:${(server.address() as any).port}`;
  await assert.rejects(
    readWithClient(
      { origin, token: "x", kind: "describe", name: "Account" },
      { maxBytes: 1000, timeoutMs: 5000 },
    ),
    /READ_BOUND/,
  );
  await assert.rejects(
    readWithClient(
      { origin, token: "x", kind: "describe", name: "Contact" },
      { maxBytes: 1000, timeoutMs: 1000 },
    ),
    /READ_TIMEOUT/,
  );
});

test("raw read evidence is retained only in a private external directory", async (t) => {
  const { mkdtemp, rm, readdir, stat } = await import("node:fs/promises");
  const { tmpdir } = await import("node:os");
  const { join } = await import("node:path");
  const dir = await mkdtemp(join(tmpdir(), "metadata-raw-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  const i = await collectMetadata(
    await config(),
    { ...env, DISCOVERY_ARTIFACT_DIR: dir },
    reader([]),
  );
  assert.equal(i.artifacts.length, 7);
  assert.equal((await readdir(join(dir, "raw"))).length, 7);
  for (const a of i.artifacts) {
    assert.match(a.sha256, /^[a-f0-9]{64}$/);
    assert.equal((await stat(join(dir, a.path))).mode & 0o077, 0);
  }
});
