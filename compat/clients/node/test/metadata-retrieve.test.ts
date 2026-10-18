import test from "node:test";
import assert from "node:assert/strict";
import {
  mkdtemp,
  mkdir,
  writeFile,
  symlink,
  rm,
  readFile,
} from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import {
  scanRetrievedMetadata,
  retrieveInSession,
} from "../src/metadata-retrieve.ts";
const paths = [
  "objects/Account/fields/Compat_Text__c.field-meta.xml",
  "objects/Account/fields/Compat_Key__c.field-meta.xml",
  "permissionsets/Compat_Admin.permissionset-meta.xml",
];
async function files(root: string) {
  for (const path of paths) {
    await mkdir(join(root, path, ".."), { recursive: true });
    await writeFile(
      join(root, path),
      path.startsWith("objects")
        ? '<CustomField xmlns="http://soap.sforce.com/2006/04/metadata"><type>Text</type><length>255</length><required>false</required><externalId>false</externalId><unique>false</unique></CustomField>'
        : '<PermissionSet xmlns="http://soap.sforce.com/2006/04/metadata"><fieldPermissions><field>Account.Compat_Text__c</field><readable>true</readable><editable>true</editable></fieldPermissions></PermissionSet>',
    );
  }
}
test("exactRetrieveManifest and partialRetrieveIsIncomplete", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "metadata-retrieve-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  let calls: string[][] = [];
  const result = await retrieveInSession(async (args) => {
    calls.push(args);
    assert.match(
      await readFile(join(dir, "package.xml"), "utf8"),
      /<members>Compat_Admin<\/members>/,
    );
    await files(join(dir, "force-app/main/default"));
    return { success: true, status: "Succeeded" };
  }, dir);
  assert.equal(result.outcome, "complete");
  assert.equal(result.members.length, 3);
  assert.equal(result.members[0].data.required, false);
  assert.equal(calls.length, 1);
  assert.deepEqual(calls[0].slice(0, 3), ["project", "retrieve", "start"]);
  assert.ok(!calls[0].includes("--ignore-conflicts"));
  await rm(join(dir, "force-app/main/default", paths[0]));
  assert.equal(
    (await scanRetrievedMetadata(join(dir, "force-app/main/default"))).outcome,
    "incomplete",
  );
});
test("rejectTraversalAndSymlinkOutputs including unexpected members and oversized files", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "metadata-files-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await files(dir);
  await writeFile(join(dir, "unexpected.xml"), "x");
  await assert.rejects(scanRetrievedMetadata(dir), /Unexpected/);
  await rm(join(dir, "unexpected.xml"));
  const target = join(dir, paths[0]);
  await rm(target);
  await symlink("/etc/passwd", target);
  await assert.rejects(scanRetrievedMetadata(dir), /symlink/i);
  await rm(target);
  await writeFile(target, "x".repeat(8 * 1024 * 1024 + 1));
  await assert.rejects(scanRetrievedMetadata(dir), /bound/);
});
test("malformed or entity-bearing XML is not successful retrieval", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "metadata-xml-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await files(dir);
  await writeFile(
    join(dir, paths[0]),
    '<!DOCTYPE x [<!ENTITY xx SYSTEM "file:///etc/passwd">]><CustomField>&xx;</CustomField>',
  );
  await assert.rejects(scanRetrievedMetadata(dir), /XML/);
});
test("failed retrieval cannot reuse stale input files", async (t) => {
  const dir = await mkdtemp(join(tmpdir(), "metadata-partial-"));
  t.after(() => rm(dir, { recursive: true, force: true }));
  await assert.rejects(
    retrieveInSession(async () => {
      throw new Error("private failure");
    }, dir),
  );
  assert.deepEqual(
    await readFile(join(dir, "sfdx-project.json"), "utf8")
      .then(JSON.parse)
      .then((x) => x.packageDirectories),
    [{ path: "force-app", default: true }],
  );
});

test("timeoutRemovesAuthHome without using shared aliases", async (t) => {
  const fs = (await import("node:fs/promises")).default;
  const { syncBuiltinESMExports } = await import("node:module");
  const { withCli } = await import("../src/process-clients.ts");
  const original = fs.mkdtemp;
  let home = "";
  t.mock.method(
    fs,
    "mkdtemp",
    async (...args: Parameters<typeof fs.mkdtemp>) => {
      home = await original(...args);
      return home;
    },
  );
  syncBuiltinESMExports();
  try {
    await assert.rejects(
      withCli(
        {
          env: {},
          conn: {
            accessToken: "synthetic",
            instanceUrl: "https://reference.example.invalid",
          },
        } as any,
        async () => {
          assert.fail("timeout must prevent action");
        },
        { timeoutMs: 1 },
      ),
      /CLI_PROCESS_TIMEOUT/,
    );
    assert.ok(home.includes("salescompel-cli-"));
    await assert.rejects(fs.stat(home), { code: "ENOENT" });
  } finally {
    t.mock.restoreAll();
    syncBuiltinESMExports();
  }
});
