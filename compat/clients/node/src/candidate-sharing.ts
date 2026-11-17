// Plan 34 real-client acceptance: record-level sharing through unmodified
// JSforce and simple-salesforce. Runs inside the OAuth worker after the
// permission flows, with real password users and an admin connection. The
// Account org-wide default is Private only while these steps run.
import { Connection } from "jsforce";
import { runCommand } from "./candidate-acceptance-process.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import { trackFixtureAttempt } from "./candidate-acceptance-fixtures.ts";
import { ensure } from "./candidate-oauth-worker-runtime.ts";
import { rawLogin } from "./candidate-oauth-worker-http.ts";
import {
  retainPrincipal,
  retainUser,
  type OwnerCommand,
} from "./candidate-oauth-worker-provision.ts";
import type { Passed } from "./candidate-oauth-worker-clients.ts";
import { ROOT } from "./versions.ts";

export const sharingSteps = [
  "sharing/jsforce-private-share",
  "sharing/jsforce-transfer",
  "sharing/python-hierarchy",
];

type Options = {
  admin: Connection;
  password: string;
  app: { client_id: string; client_secret: string };
  redirect: string;
  command: OwnerCommand;
  passed: Passed;
};

let stage = "start";
function check(label: string, condition: unknown) {
  if (!condition) console.error("Sharing check failed: " + stage + " " + label);
  ensure(condition);
}

async function sharingUser(c: GateContext, o: Options, tag: string) {
  const username = `p34-${c.runId}-${tag}@example.invalid`;
  const id = await o.command(["user", "create"], {
    SALESCOMPEL_TENANT: c.tenants.first,
    SALESCOMPEL_USERNAME: username,
    SALESCOMPEL_PASSWORD: o.password,
    SALESCOMPEL_PROFILE: "Standard User",
    SALESCOMPEL_ROLE: "",
  });
  await retainPrincipal(c, "users", id);
  await retainUser(c, id, username, c.tenants.first);
  const t = await rawLogin(
    c,
    o.password,
    username,
    o.app.client_id,
    o.app.client_secret,
    o.redirect,
  );
  check("s1", t.access_token);
  return {
    id,
    token: t.access_token as string,
    conn: new Connection({
      instanceUrl: c.url,
      accessToken: t.access_token as string,
      version: "65.0",
    }),
  };
}

async function setDefault(c: GateContext, o: Options, access: string) {
  await o.command(
    [
      "sharing",
      "set-default",
      "--tenant",
      c.tenants.first,
      "--object",
      "Account",
      "--access",
      access,
    ],
    {},
  );
}

const errorCode = async (p: Promise<unknown>) => {
  try {
    await p;
    return "none";
  } catch (error: unknown) {
    return String((error as { errorCode?: unknown })?.errorCode ?? "unknown");
  }
};

async function accountAs(
  c: GateContext,
  user: { conn: Connection },
  name: string,
) {
  // Cleanup runs as the admin after the default is back to Edit.
  const r = await trackFixtureAttempt(
    c.fixtureJournal,
    {
      schemaVersion: 1,
      runId: c.runId,
      // One journal intent per operation ID.
      operationId: "p34-sharing-" + name.slice(name.lastIndexOf("-") + 1),
      principal: "admin",
      object: "Account",
      names: [name],
      kind: "intent",
    },
    () => user.conn.sobject("Account").create({ Name: name }),
  );
  check("s2", r.success && r.id);
  return r.id as string;
}

export async function sharingFlows(c: GateContext, o: Options) {
  const owner = await sharingUser(c, o, "owner");
  const peer = await sharingUser(c, o, "peer");
  const manager = await sharingUser(c, o, "manager");
  const roles: string[] = [];
  try {
    stage = "setup";
    for (const [name, parent] of [
      ["Manager", undefined],
      ["Rep", 0],
    ] as const) {
      const r = await o.admin.sobject("UserRole").create({
        Name: `p34 ${name} ${c.runId}`,
        DeveloperName: `p34_${name}_${c.runId}`,
        ...(parent === undefined ? {} : { ParentRoleId: roles[parent] }),
      });
      check("s3", r.success && r.id);
      roles.push(r.id!);
    }
    for (const [user, role] of [
      [manager, roles[0]],
      [owner, roles[1]],
    ] as const)
      check(
        "s4",
        (
          await o.admin
            .sobject("User")
            .update({ Id: user.id, UserRoleId: role })
        ).success,
      );
    await setDefault(c, o, "None");

    stage = "jsforce-private-share";
    const name = "p34-" + c.runId + "-private";
    const account = await accountAs(c, owner, name);
    const q = `SELECT Id, OwnerId FROM Account WHERE Id = '${account}'`;
    check("s5", (await peer.conn.query(q)).totalSize === 0);
    check(
      "s6",
      (await errorCode(peer.conn.sobject("Account").retrieve(account))) ===
        "NOT_FOUND",
    );
    const share = await owner.conn.sobject("AccountShare").create({
      AccountId: account,
      UserOrGroupId: peer.id,
      AccountAccessLevel: "Read",
    });
    check("s7", share.success);
    check("s8", (await peer.conn.query(q)).totalSize === 1);
    check(
      "s9",
      (await errorCode(
        peer.conn.sobject("Account").update({ Id: account, Name: name + "-x" }),
      )) === "INSUFFICIENT_ACCESS_OR_READONLY",
    );
    const rows = await owner.conn.query<{ RowCause: string }>(
      `SELECT RowCause FROM AccountShare WHERE AccountId = '${account}'`,
    );
    check(
      "s10",
      rows.records
        .map((r) => r.RowCause)
        .sort()
        .join() === "Manual,Owner",
    );
    o.passed("jsforce-private-share", "sharing");

    stage = "jsforce-transfer";
    check(
      "s11",
      (
        await owner.conn
          .sobject("Account")
          .update({ Id: account, OwnerId: peer.id })
      ).success,
    );
    const moved = await peer.conn.query<{ OwnerId: string }>(q);
    check("s12", moved.records[0]?.OwnerId === peer.id);
    check("s13", (await owner.conn.query(q)).totalSize === 0);
    const after = await peer.conn.query(
      `SELECT Id FROM AccountShare WHERE AccountId = '${account}' AND RowCause = 'Manual'`,
    );
    check("s14", after.totalSize === 0);
    o.passed("jsforce-transfer", "sharing");

    stage = "python-hierarchy";
    const subordinate = await accountAs(c, owner, name + "-rep");
    const py = await runCommand(
      {
        executable: c.python,
        argv: ["scripts/candidate-sharing-python.py"],
        cwd: ROOT,
        env: {
          ...c.env,
          CANDIDATE_SHARING_MANAGER_TOKEN: manager.token,
          CANDIDATE_SHARING_PEER_TOKEN: peer.token,
          CANDIDATE_SHARING_ACCOUNT: subordinate,
        },
        privateOutput: () => {},
      },
      c.processes,
      "oauth",
    );
    check("s15", py.exitCode === 0 && py.complete);
    o.passed("python-hierarchy", "sharing");
  } catch (error: unknown) {
    const e = error as { errorCode?: unknown; name?: unknown };
    const kind = typeof e?.errorCode === "string" ? e.errorCode : e?.name;
    if (typeof kind === "string" && /^[A-Za-z_]{1,64}$/.test(kind))
      console.error("Sharing failed in " + stage + ": " + kind);
    throw error;
  } finally {
    // Later steps and gates expect the Salesforce new-org default.
    await setDefault(c, o, "Edit");
  }
}
