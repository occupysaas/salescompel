// Plan 33 real-client acceptance: permission sets granted through the
// unmodified sf CLI and JSforce, a Read Only refusal through
// simple-salesforce, and TODAY in the user's time zone. Runs inside the
// OAuth worker, which has real password users and an admin connection.
import { Connection } from "jsforce";
import { runCommand } from "./candidate-acceptance-process.ts";
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import { createFixture } from "./candidate-acceptance-fixtures.ts";
import { ensure, remember } from "./candidate-oauth-worker-runtime.ts";
import { rawLogin } from "./candidate-oauth-worker-http.ts";
import {
  retainPrincipal,
  retainUser,
  type OwnerCommand,
} from "./candidate-oauth-worker-provision.ts";
import type { Passed } from "./candidate-oauth-worker-clients.ts";
import { ROOT, SF_BIN } from "./versions.ts";

export const permissionSteps = [
  "permissions/cli-assign-permset",
  "permissions/jsforce-permset",
  "permissions/python-readonly",
  "permissions/timezone-today",
];

type Options = {
  admin: Connection;
  adminAlias: string;
  sfEnv: NodeJS.ProcessEnv;
  account: string;
  password: string;
  restricted: string;
  app: { client_id: string; client_secret: string };
  redirect: string;
  command: OwnerCommand;
  passed: Passed;
};

const compatQuery = (account: string) =>
  `SELECT Id, Compat_Text__c FROM Account WHERE Id = '${account}'`;

/** A fresh password user on profile, logged in through the authorize flow. */
async function profileUser(
  c: GateContext,
  o: Options,
  tag: string,
  profile: string,
) {
  const username = `p33-${c.runId}-${tag}@example.invalid`;
  const id = await o.command(["user", "create"], {
    SALESCOMPEL_TENANT: c.tenants.first,
    SALESCOMPEL_USERNAME: username,
    SALESCOMPEL_PASSWORD: o.password,
    SALESCOMPEL_PROFILE: profile,
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
  ensure(t.access_token);
  return { id, username, token: t.access_token as string };
}

function connectionFor(c: GateContext, token: string) {
  return new Connection({
    instanceUrl: c.url,
    accessToken: token,
    version: "65.0",
  });
}

/** A permission set granting read on Account.Compat_Text__c. */
async function compatSet(admin: Connection, name: string) {
  const set = await admin
    .sobject("PermissionSet")
    .create({ Name: name, Label: name });
  ensure(set.success && set.id);
  const grant = await admin.sobject("FieldPermissions").create({
    ParentId: set.id,
    SobjectType: "Account",
    Field: "Account.Compat_Text__c",
    PermissionsRead: true,
  });
  ensure(grant.success);
  return set.id!;
}

async function sfAs(c: GateContext, env: NodeJS.ProcessEnv, args: string[]) {
  let stdout = "";
  const r = await runCommand(
    {
      executable: process.execPath,
      argv: [SF_BIN, ...args, "--json"],
      cwd: c.scratch,
      env,
      timeout: 120000,
      privateOutput: (out) => {
        stdout = out;
      },
    },
    c.processes,
    "oauth",
  );
  ensure(r.complete);
  return {
    exit: r.exitCode,
    json: remember(JSON.parse(stdout)) as Record<string, any>,
  };
}

/** sf org assign permset makes a hidden field queryable for a Standard User. */
async function cliAssignPermset(c: GateContext, o: Options) {
  const name = "P33_" + c.runId.slice(0, 12) + "_cli";
  const set = await compatSet(o.admin, name);
  const t = await rawLogin(
    c,
    o.password,
    o.restricted,
    o.app.client_id,
    o.app.client_secret,
    o.redirect,
  );
  ensure(t.access_token);
  const alias = "p33-std-" + c.runId;
  const stdEnv = { ...o.sfEnv, SF_ACCESS_TOKEN: t.access_token };
  ensure(
    (
      await sfAs(c, stdEnv, [
        "org",
        "login",
        "access-token",
        "--instance-url",
        c.url,
        "--alias",
        alias,
        "--no-prompt",
      ])
    ).exit === 0,
  );
  const query = [
    "data",
    "query",
    "--query",
    compatQuery(o.account),
    "--target-org",
    alias,
    "--api-version",
    "65.0",
  ];
  const before = await sfAs(c, stdEnv, query);
  ensure(
    before.exit !== 0 && JSON.stringify(before.json).includes("INVALID_FIELD"),
  );
  const assigned = await sfAs(c, o.sfEnv, [
    "org",
    "assign",
    "permset",
    "--name",
    name,
    "--on-behalf-of",
    o.restricted,
    "--target-org",
    o.adminAlias,
  ]);
  ensure(assigned.exit === 0 && assigned.json.result?.successes?.length === 1);
  const after = await sfAs(c, stdEnv, query);
  ensure(after.exit === 0 && after.json.result?.records?.length === 1);
  ensure("Compat_Text__c" in after.json.result.records[0]);
  await o.admin.sobject("PermissionSet").destroy(set);
  o.passed("cli-assign-permset", true);
}

/** JSforce builds a set with object and field permissions; describe follows. */
async function jsforcePermset(c: GateContext, o: Options) {
  const user = await profileUser(c, o, "jsforce", "Standard User");
  const conn = connectionFor(c, user.token);
  const fields = async () =>
    (await conn.sobject("Account").describe()).fields.map((f) => f.name);
  ensure(!(await fields()).includes("Compat_Text__c"));
  const set = await compatSet(o.admin, "P33_" + c.runId.slice(0, 12) + "_js");
  const object = await o.admin.sobject("ObjectPermissions").create({
    ParentId: set,
    SobjectType: "Account",
    PermissionsRead: true,
    PermissionsCreate: true,
  });
  ensure(object.success);
  const assignment = await o.admin
    .sobject("PermissionSetAssignment")
    .create({ AssigneeId: user.id, PermissionSetId: set });
  ensure(assignment.success);
  ensure((await fields()).includes("Compat_Text__c"));
  await o.admin.sobject("PermissionSetAssignment").destroy(assignment.id!);
  ensure(!(await fields()).includes("Compat_Text__c"));
  await o.admin.sobject("PermissionSet").destroy(set);
  o.passed("jsforce-permset", true);
}

/** simple-salesforce as Read Only: queries work, create is refused. */
async function pythonReadOnly(c: GateContext, o: Options) {
  const user = await profileUser(c, o, "readonly", "Read Only");
  const py = await runCommand(
    {
      executable: c.python,
      argv: ["scripts/candidate-permissions-python.py"],
      cwd: ROOT,
      env: {
        ...c.env,
        CANDIDATE_PERMISSIONS_TOKEN: user.token,
        CANDIDATE_PERMISSIONS_NAME: "p33-" + c.runId + "-python",
      },
      privateOutput: () => {},
    },
    c.processes,
    "oauth",
  );
  ensure(py.exitCode === 0 && py.complete);
  o.passed("python-readonly", true);
  return user;
}

/** TODAY follows the user's TimeZoneSidKey: UTC+14 and UTC-11 are always on
 * different calendar days, so one date matches in exactly one of them. */
async function timezoneToday(
  c: GateContext,
  o: Options,
  user: { id: string; token: string },
) {
  const kiritimati = new Date(Date.now() + 14 * 3600 * 1000)
    .toISOString()
    .slice(0, 10);
  const name = "p33-" + c.runId + "-tz";
  const contact = await createFixture(
    c,
    "p33-timezone",
    "Contact",
    { LastName: name, Birthdate: kiritimati },
    [name],
  );
  const conn = connectionFor(c, user.token);
  const count = async (zone: string) => {
    ensure(
      (
        await o.admin
          .sobject("User")
          .update({ Id: user.id, TimeZoneSidKey: zone })
      ).success,
    );
    return (
      await conn.query(
        `SELECT Id FROM Contact WHERE Id = '${contact}' AND Birthdate = TODAY`,
      )
    ).totalSize;
  };
  ensure((await count("Pacific/Kiritimati")) === 1);
  ensure((await count("Pacific/Pago_Pago")) === 0);
  o.passed("timezone-today", true);
}

export async function permissionFlows(c: GateContext, o: Options) {
  await cliAssignPermset(c, o);
  await jsforcePermset(c, o);
  const readOnly = await pythonReadOnly(c, o);
  await timezoneToday(c, o, readOnly);
}
