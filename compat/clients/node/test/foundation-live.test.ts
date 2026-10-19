import test from "node:test";
import assert from "node:assert/strict";
import { createServer } from "node:http";
import { createHash } from "node:crypto";
import {
  foundationLivePreflight,
  collectFoundationAbsence,
  verifyFoundationPriorRecovery,
  foundationJournalDigest,
} from "../src/foundation-live.ts";
import type { Report } from "../src/core.ts";
import { auditFoundationRecovery } from "../src/foundation-live.ts";
import { mkdtemp, mkdir, writeFile, rm, chmod } from "node:fs/promises";
import { tmpdir } from "node:os";
import { join } from "node:path";
import { verifyFoundationCleanupFiles } from "../src/foundation-cleanup-command.ts";

test("live preflight binds two administrator sessions, restricted persona and field metadata", async (t) => {
  let restrictedUser = "restricted";
  let includeText = true;
  const server = createServer((req, res) => {
    res.setHeader("content-type", "application/json");
    const token = req.headers.authorization?.split(" ")[1];
    if (req.url === "/services/oauth2/userinfo")
      res.end(
        JSON.stringify({
          user_id: token === "restricted" ? restrictedUser : "admin",
          organization_id: "org",
        }),
      );
    else if (req.url?.endsWith("/Account/describe")) {
      const fields =
        token === "restricted"
          ? ["Id", "Name"]
          : [
              "Id",
              "Name",
              ...(includeText ? ["Compat_Text__c"] : []),
              "Compat_Key__c",
            ];
      res.end(
        JSON.stringify({
          name: "Account",
          fields: fields.map((name) => ({ name })),
        }),
      );
    } else if (req.url?.endsWith("/Contact/describe"))
      res.end(
        JSON.stringify({
          name: "Contact",
          fields: [{ name: "Id" }, { name: "AccountId" }],
        }),
      );
    else {
      res.writeHead(404);
      res.end("{}");
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => server.close());
  const env = {
    COMPAT_REFERENCE_URL: `http://127.0.0.1:${(server.address() as any).port}`,
    COMPAT_REFERENCE_TOKEN: "admin-one",
    COMPAT_REFERENCE_FRESH_TOKEN: "admin-two",
    COMPAT_REFERENCE_RESTRICTED_TOKEN: "restricted",
    COMPAT_API_VERSION: "65.0",
    COMPAT_REFERENCE_EXPECTED_ORG_ID: "org",
    COMPAT_REFERENCE_EXPECTED_ADMIN_USER_ID: "admin",
    COMPAT_REFERENCE_EXPECTED_RESTRICTED_USER_ID: "restricted",
  };
  assert.equal(
    await foundationLivePreflight(env),
    createHash("sha256").update("admin\0org").digest("hex"),
  );
  await assert.rejects(
    foundationLivePreflight({
      ...env,
      COMPAT_REFERENCE_EXPECTED_ORG_ID: "different",
    }),
  );
  await assert.rejects(
    foundationLivePreflight({
      ...env,
      COMPAT_REFERENCE_EXPECTED_ADMIN_USER_ID: "different",
    }),
  );
  restrictedUser = "admin";
  await assert.rejects(foundationLivePreflight(env));
  restrictedUser = "restricted";
  includeText = false;
  await assert.rejects(foundationLivePreflight(env));
});

test("recovery audit refuses a journal with an unmatched create intent", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "foundation-recovery-"));
  await chmod(root, 0o700);
  t.after(() => rm(root, { recursive: true, force: true }));
  const runId = "a".repeat(16);
  const dir = join(root, `reference-${runId}`);
  await mkdir(dir, { mode: 0o700 });
  const id = "001000000000001AAA";
  const journal = join(dir, "foundation-cursors.jsonl");
  const intent = {
    intent: "create",
    type: "Account",
    fields: { Name: "owned" },
  };
  const resource = { resource: `Account:${id}` };
  await writeFile(
    journal,
    `${JSON.stringify(intent)}\n${JSON.stringify(resource)}\n`,
    { mode: 0o600 },
  );
  const report = {
    results: [
      {
        id: "foundation-cursors",
        identities: {
          "value:cursors": `Compat_${runId}_cursors`,
          "cursor-0": id,
        },
      },
    ],
    cleanup: [
      {
        scenario: "foundation-cursors",
        resource: `Account:${id}`,
        outcome: "passed",
      },
    ],
  } as unknown as Report;
  assert.equal(await auditFoundationRecovery(root, report), true);
  const missingOwned = structuredClone(report);
  missingOwned.results[0].identities["cursor-1"] = "001000000000002AAA";
  assert.equal(await auditFoundationRecovery(root, missingOwned), false);
  await writeFile(
    journal,
    `${JSON.stringify(intent)}\n${JSON.stringify(resource)}\n${JSON.stringify(intent)}\n`,
  );
  assert.equal(await auditFoundationRecovery(root, report), false);
});

test("prior recovery journals without verified absence block a new capture", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "foundation-prior-"));
  await chmod(root, 0o700);
  t.after(() => rm(root, { recursive: true, force: true }));
  await verifyFoundationPriorRecovery(root, "d".repeat(64), {});
  const runDir = join(root, `reference-${"a".repeat(16)}`);
  await mkdir(runDir, { mode: 0o700 });
  await writeFile(join(runDir, "foundation-cursors.jsonl"), "", {
    mode: 0o600,
  });
  await assert.rejects(verifyFoundationPriorRecovery(root, "d".repeat(64), {}));
});

test("cleanup command requires private capture and policy before writing attestation", async (t) => {
  const root = await mkdtemp(join(tmpdir(), "foundation-cleanup-command-"));
  await chmod(root, 0o700);
  t.after(() => rm(root, { recursive: true, force: true }));
  await assert.rejects(
    verifyFoundationCleanupFiles(
      join(root, "missing.json"),
      join(root, "missing.policy.json"),
      join(root, "absence.json"),
      {},
    ),
  );
});

test("absence verifier reads exact owned IDs and refuses a remaining record", async (t) => {
  let present = false;
  const id = "001000000000001AAA";
  const server = createServer((req, res) => {
    res.setHeader("content-type", "application/json");
    const token = req.headers.authorization?.split(" ")[1];
    if (req.url === "/services/oauth2/userinfo")
      res.end(
        JSON.stringify({
          user_id: token === "restricted" ? "restricted" : "admin",
          organization_id: "org",
        }),
      );
    else if (req.url?.endsWith("/Account/describe")) {
      const fields =
        token === "restricted"
          ? ["Id", "Name"]
          : ["Id", "Name", "Compat_Text__c", "Compat_Key__c"];
      res.end(
        JSON.stringify({
          name: "Account",
          fields: fields.map((name) => ({ name })),
        }),
      );
    } else if (req.url?.endsWith("/Contact/describe"))
      res.end(
        JSON.stringify({
          name: "Contact",
          fields: [{ name: "Id" }, { name: "AccountId" }],
        }),
      );
    else if (req.url?.includes("/query?q="))
      res.end(
        JSON.stringify({
          totalSize: present ? 1 : 0,
          done: true,
          records: present ? [{ Id: id }] : [],
        }),
      );
    else {
      res.writeHead(404);
      res.end("{}");
    }
  });
  await new Promise<void>((resolve) => server.listen(0, "127.0.0.1", resolve));
  t.after(() => server.close());
  const env = {
    COMPAT_REFERENCE_URL: `http://127.0.0.1:${(server.address() as any).port}`,
    COMPAT_REFERENCE_TOKEN: "admin-one",
    COMPAT_REFERENCE_FRESH_TOKEN: "admin-two",
    COMPAT_REFERENCE_RESTRICTED_TOKEN: "restricted",
    COMPAT_API_VERSION: "65.0",
    COMPAT_REFERENCE_EXPECTED_ORG_ID: "org",
    COMPAT_REFERENCE_EXPECTED_ADMIN_USER_ID: "admin",
    COMPAT_REFERENCE_EXPECTED_RESTRICTED_USER_ID: "restricted",
  };
  const report = {
    finishedAt: "2026-10-19T00:00:00.000Z",
    cleanup: [
      {
        scenario: "foundation-cursors",
        resource: `Account:${id}`,
        outcome: "passed",
      },
    ],
  } as Report;
  const digest = createHash("sha256").update("admin\0org").digest("hex");
  const evidence = await collectFoundationAbsence(
    report,
    "a".repeat(64),
    digest,
    env,
    true,
  );
  assert.deepEqual(evidence.resources, [
    { resource: `Account:${id}`, absent: true },
  ]);
  assert.equal(evidence.identityDigest, digest);
  const root = await mkdtemp(join(tmpdir(), "foundation-prior-verified-"));
  await chmod(root, 0o700);
  t.after(() => rm(root, { recursive: true, force: true }));
  const runDir = join(root, `reference-${"b".repeat(16)}`);
  await mkdir(runDir, { mode: 0o700 });
  const journal = join(runDir, "foundation-cursors.jsonl");
  await writeFile(journal, `${JSON.stringify({ intent: "create" })}\n`, {
    mode: 0o600,
  });
  await writeFile(
    join(runDir, "foundation-absence.json"),
    JSON.stringify({
      evidence,
      journalDigest: await foundationJournalDigest(runDir),
    }),
    { mode: 0o600 },
  );
  await verifyFoundationPriorRecovery(root, digest, env);
  present = true;
  await assert.rejects(
    collectFoundationAbsence(report, "a".repeat(64), digest, env, true),
  );
  await assert.rejects(verifyFoundationPriorRecovery(root, digest, env));
  present = false;
  await writeFile(
    journal,
    `${JSON.stringify({ intent: "create" })}\n${JSON.stringify({ intent: "create" })}\n`,
  );
  await assert.rejects(verifyFoundationPriorRecovery(root, digest, env));
});
