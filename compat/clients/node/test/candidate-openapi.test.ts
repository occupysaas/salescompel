import assert from "node:assert/strict";
import test from "node:test";
import { request as httpsRequest } from "node:https";
import { readFile } from "node:fs/promises";
import { parse } from "yaml";
import { ROOT } from "../src/versions.ts";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import type { CandidateExchange } from "../src/candidate-openapi-model.ts";
import { runCompositeFixtureAttempt } from "../src/candidate-composite-fixtures.ts";
import {
  recordFixtureEvent,
  trackFixtureAttempt,
  type FixtureEvent,
} from "../src/candidate-acceptance-fixtures.ts";

const expectedIds = [
  "candidateVersions",
  "candidateResources",
  "candidateUserInfo",
  "candidateOwnUser51",
  "candidateOwnUser65",
  "candidateDescribeGlobal",
  "candidateDescribeAccount",
  "candidateDescribeContact",
  "candidateCreateAccount",
  "candidateCreateContact",
  "candidateReadAccount",
  "candidateUpdateAccount",
  "candidateDeleteAccount",
  "candidateReadContact",
  "candidateUpdateContact",
  "candidateDeleteContact",
  "candidateUpsertAccount",
  "candidateQuery",
  "candidateOpenAPI",
  "candidateCompositeAtomic",
];
const base = "/services/data/v65.0";
type Measured = CandidateExchange & { allow: string | null };
type Role = FixtureEvent["principal"] | "none" | "bad";
function json(e: Measured): Record<string, any> {
  try {
    return JSON.parse(
      new TextDecoder("utf-8", { fatal: true }).decode(e.response.body),
    );
  } catch {
    throw new Error("candidate-openapi-json");
  }
}

test(
  "candidate/openapi/traffic",
  { skip: process.env.RUN_CANDIDATE !== "1" },
  async () => {
    const url = new URL(process.env.CANDIDATE_URL!);
    assert.ok(
      url.protocol === "https:" &&
        ["localhost", "127.0.0.1"].includes(url.hostname),
      "owned HTTPS candidate required",
    );
    const tokens = {
      admin: process.env.CANDIDATE_TOKEN!,
      restricted: process.env.CANDIDATE_RESTRICTED_TOKEN!,
      other: process.env.CANDIDATE_OTHER_TOKEN!,
      bad: "invalid-candidate-token",
      none: "",
    };
    async function measure(
      method: string,
      path: string,
      options: { role?: Role; body?: unknown; rawBody?: string } = {},
    ): Promise<Measured> {
      const target = new URL(path, url);
      assert.ok(
        target.origin === url.origin && path.startsWith("/services/"),
        "same-origin service request required",
      );
      const input =
        options.rawBody !== undefined
          ? Buffer.from(options.rawBody)
          : options.body !== undefined
            ? Buffer.from(JSON.stringify(options.body))
            : undefined;
      const contentType = input ? "application/json" : null;
      return new Promise((resolve, reject) => {
        const headers: Record<string, string> = {};
        const token = tokens[options.role ?? "admin"];
        if (token) headers.Authorization = "Bearer " + token;
        if (contentType) headers["Content-Type"] = contentType;
        const req = httpsRequest(
          target,
          { method, headers, timeout: 15000 },
          (res) => {
            const chunks: Buffer[] = [];
            let length = 0;
            res.on("data", (chunk: Buffer) => {
              length += chunk.length;
              if (length > 8388608) {
                req.destroy(new Error("candidate-openapi-budget"));
                return;
              }
              chunks.push(chunk);
            });
            res.on("end", () =>
              resolve({
                method,
                path,
                request: { contentType, body: input },
                response: {
                  status: res.statusCode ?? 0,
                  contentType: res.headers["content-type"] ?? null,
                  body: Buffer.concat(chunks),
                },
                allow:
                  typeof res.headers.allow === "string"
                    ? res.headers.allow
                    : null,
              }),
            );
            res.on("error", () =>
              reject(new Error("candidate-openapi-transport")),
            );
          },
        );
        req.on("timeout", () =>
          req.destroy(new Error("candidate-openapi-timeout")),
        );
        req.on("error", () => reject(new Error("candidate-openapi-transport")));
        req.end(input);
      });
    }
    const served = await measure("GET", base + "/openapi.yaml");
    assert.equal(served.response.status, 200);
    assert.ok(
      Buffer.from(served.response.body).equals(
        await readFile(ROOT + "/api/candidate-openapi.yaml"),
      ),
      "served canonical bytes differ",
    );
    const validator = await createCandidateTrafficValidator(
      parse(Buffer.from(served.response.body).toString("utf8")),
    );
    const observed = new Set<string>();
    function checked(
      e: Measured,
      status: number,
      requestIssues: string[] = [],
    ) {
      assert.equal(e.response.status, status);
      const result = validator.validate(e);
      assert.deepEqual(result.requestIssues, requestIssues);
      assert.deepEqual(result.responseIssues, []);
      assert.ok(result.operationId, "matched operation required");
      if (status >= 200 && status < 300) {
        assert.deepEqual(requestIssues, []);
        observed.add(result.operationId);
      }
      return e;
    }
    checked(served, 200);
    const runId = process.env.CANDIDATE_RUN_ID!;
    const journal = process.env.CANDIDATE_FIXTURE_JOURNAL!;
    assert.ok(
      /^[a-f0-9]{24}$/.test(runId) && journal,
      "owned fixture context required",
    );
    const fixtureName = (label: string) => `p23-${runId}-${label}`;
    const rows = new Map<string, { id: string; event: FixtureEvent }>();
    const rejected: FixtureEvent[] = [];
    async function tracked(
      label: string,
      object: "Account" | "Contact",
      method: string,
      path: string,
      options: { body?: unknown; rawBody?: string },
    ): Promise<Measured> {
      const event: FixtureEvent = {
        schemaVersion: 1,
        runId,
        operationId: "openapi-" + label,
        principal: "admin",
        object,
        names: [fixtureName(label), fixtureName(label) + " updated"],
        kind: "intent",
      };
      // Upsert update shares the first fixture's spelling, but remains a separately
      // journaled potentially-creating request.
      if (label === "upsert-update")
        event.names = [
          fixtureName("upsert"),
          fixtureName("upsert") + " updated",
        ];
      const result = await trackFixtureAttempt(journal, event, async () => {
        const e = await measure(method, path, options);
        const data = json(e);
        const id = typeof data.id === "string" ? data.id : undefined;
        if (id && !rows.has(object + id)) rows.set(object + id, { id, event });
        return { id, e };
      });
      if (result.e.response.status === 400 && !result.id) rejected.push(event);
      return result.e;
    }
    try {
      const event: FixtureEvent = {
        schemaVersion: 1,
        runId,
        operationId: "p24openapi",
        principal: "admin",
        object: "Account",
        names: [fixtureName("composite")],
        kind: "intent",
      };
      let compositeExchange: Measured | undefined;
      const compositeResponse = await runCompositeFixtureAttempt(
        journal,
        [event],
        [{ referenceId: event.operationId, method: "POST" }],
        "commit",
        async () => {
          compositeExchange = await measure("POST", base + "/composite", {
            body: {
              allOrNone: true,
              compositeRequest: [
                {
                  method: "POST",
                  url: base + "/sobjects/Account",
                  referenceId: event.operationId,
                  body: { Name: event.names[0] },
                },
              ],
            },
          });
          return {
            status: compositeExchange.response.status,
            body: json(compositeExchange),
          };
        },
        async () => {
          throw Error("FIXTURES");
        },
      );
      const compositeId = compositeResponse.body.compositeResponse[0].body.id;
      rows.set("Account" + compositeId, { id: compositeId, event });
      checked(compositeExchange!, 200);
      for (const role of ["restricted", "other"] as const) {
        const e = checked(
          await measure("GET", base + "/openapi.yaml", { role }),
          200,
        );
        assert.ok(
          Buffer.from(e.response.body).equals(
            Buffer.from(served.response.body),
          ),
          "contract varies by principal",
        );
      }
      checked(await measure("GET", "/services/data"), 200);
      checked(await measure("GET", base), 200);
      const user = json(
        checked(await measure("GET", "/services/oauth2/userinfo"), 200),
      ).user_id;
      for (const version of ["51", "65"])
        checked(
          await measure(
            "GET",
            `/services/data/v${version}.0/sobjects/User/${encodeURIComponent(user)}`,
          ),
          200,
        );
      checked(await measure("GET", base + "/sobjects"), 200);
      for (const object of ["Account", "Contact"])
        checked(
          await measure("GET", base + `/sobjects/${object}/describe`),
          200,
        );
      const account = checked(
        await tracked(
          "account",
          "Account",
          "POST",
          base + "/sobjects/Account",
          {
            body: {
              Name: fixtureName("account"),
              Compat_Text__c: "synthetic-private",
            },
          },
        ),
        201,
      );
      const accountId = json(account).id;
      const contact = checked(
        await tracked(
          "contact",
          "Contact",
          "POST",
          base + "/sobjects/Contact",
          { body: { LastName: fixtureName("contact"), AccountId: accountId } },
        ),
        201,
      );
      const contactId = json(contact).id;
      const orphan = checked(
        await tracked("orphan", "Contact", "POST", base + "/sobjects/Contact", {
          body: { LastName: fixtureName("orphan") },
        }),
        201,
      );
      const orphanId = json(orphan).id;
      const accountPath = base + "/sobjects/Account/" + accountId;
      const contactPath = base + "/sobjects/Contact/" + contactId;
      const before = json(checked(await measure("GET", accountPath), 200));
      const restricted = json(
        checked(await measure("GET", accountPath, { role: "restricted" }), 200),
      );
      assert.ok(
        !Object.hasOwn(restricted, "Compat_Text__c"),
        "restricted field present",
      );
      checked(await measure("GET", accountPath, { role: "other" }), 404);
      checked(await measure("GET", contactPath), 200);
      checked(
        await measure("PATCH", accountPath, {
          role: "restricted",
          body: { Compat_Text__c: "denied" },
        }),
        400,
      );
      assert.ok(
        JSON.stringify(
          json(checked(await measure("GET", accountPath), 200)),
        ) === JSON.stringify(before),
        "denied write changed stored record",
      );
      const query = async (q: string, extra = "", role: Role = "admin") =>
        measure("GET", base + "/query?q=" + encodeURIComponent(q) + extra, {
          role,
        });
      for (const q of [
        "SELECT Compat_Text__c FROM Account",
        "SELECT Name FROM Account WHERE Compat_Text__c = 'synthetic-private'",
      ])
        checked(await query(q, "", "restricted"), 400);
      const full = checked(
        await query(`SELECT Id,Name FROM Account WHERE Id = '${accountId}'`),
        200,
      );
      assert.ok(json(full).totalSize === 1, "ordinary query lost row");
      const projected = checked(
        await query(
          `SELECT Name FROM Account WHERE Id = '${accountId}'`,
          "&columns=false",
        ),
        200,
      );
      assert.ok(
        !Object.hasOwn(json(projected), "columnMetadata") &&
          !Object.hasOwn(json(projected).records[0], "Id"),
        "projection/columns=false drift",
      );
      const columns = checked(
        await query(
          `SELECT Name,Id FROM Account WHERE Id = '${accountId}'`,
          "&columns=true",
        ),
        200,
      );
      assert.deepEqual(json(columns).columnMetadata, [
        { columnName: "Name" },
        { columnName: "Id" },
      ]);
      const relationship = checked(
        await query(
          `SELECT Id,Account.Name FROM Contact WHERE Id = '${contactId}'`,
          "&columns=true",
        ),
        200,
      );
      assert.ok(
        json(relationship).records[0].Account.Name === fixtureName("account"),
        "relationship identity drift",
      );
      assert.deepEqual(json(relationship).columnMetadata, [
        { columnName: "Id" },
        { columnName: "Account", joinColumns: [{ columnName: "Name" }] },
      ]);
      const nullRelationship = checked(
        await query(
          `SELECT Account.Name FROM Contact WHERE Id = '${orphanId}'`,
        ),
        200,
      );
      assert.ok(
        json(nullRelationship).records[0].Account === null,
        "null relationship drift",
      );
      const empty = checked(
        await query(
          `SELECT Id FROM Account WHERE Name = '${fixtureName("absent")}'`,
          "&columns=true",
        ),
        200,
      );
      assert.ok(
        json(empty).records.length === 0 && json(empty).totalSize === 0,
        "empty query drift",
      );
      checked(
        await measure("PATCH", accountPath, {
          body: { Name: fixtureName("account") + " updated" },
        }),
        204,
      );
      checked(
        await measure("PATCH", contactPath, {
          body: {
            LastName: fixtureName("contact") + " updated",
            FirstName: null,
          },
        }),
        204,
      );
      assert.ok(
        json(checked(await measure("GET", accountPath), 200)).Name ===
          fixtureName("account") + " updated",
        "patch did not persist",
      );
      const key = fixtureName("key") + "/literal%2F";
      const upsertPath =
        base + "/sobjects/Account/Compat_Key__c/" + encodeURIComponent(key);
      const upsert = checked(
        await tracked("upsert", "Account", "PATCH", upsertPath, {
          body: { Name: fixtureName("upsert") },
        }),
        201,
      );
      const upsertId = json(upsert).id;
      const update = checked(
        await tracked("upsert-update", "Account", "PATCH", upsertPath, {
          body: { Name: fixtureName("upsert") + " updated" },
        }),
        200,
      );
      assert.ok(
        json(update).id === upsertId && !json(update).created,
        "upsert replay changed identity",
      );
      assert.ok(
        json(
          checked(
            await measure("GET", base + "/sobjects/Account/" + upsertId),
            200,
          ),
        ).Compat_Key__c === key,
        "external ID decoded more than once",
      );
      for (const role of ["none", "bad"] as const)
        checked(await measure("GET", base + "/openapi.yaml", { role }), 401);
      checked(
        await tracked(
          "bad-json",
          "Account",
          "POST",
          base + "/sobjects/Account",
          { rawBody: "{" },
        ),
        400,
        ["request-json"],
      );
      checked(
        await tracked(
          "bad-field",
          "Account",
          "POST",
          base + "/sobjects/Account",
          { body: { Name: fixtureName("bad-field"), Unknown: "no" } },
        ),
        400,
        ["request-schema"],
      );
      checked(
        await tracked(
          "bad-type",
          "Account",
          "POST",
          base + "/sobjects/Account",
          { body: { Name: 7 } },
        ),
        400,
        ["request-schema"],
      );
      for (const event of rejected) {
        const q = checked(
          await query(
            `SELECT Id FROM Account WHERE Name = '${event.names[0]}'`,
          ),
          200,
        );
        assert.ok(json(q).records.length === 0, "rejected create committed");
        await recordFixtureEvent(journal, { ...event, kind: "rejected" });
      }
      checked(
        await query("SELECT Id FROM Account", "&columns=true&columns=false"),
        400,
        ["parameters"],
      );
      checked(await measure("GET", base + "/query"), 400, ["parameters"]);
      checked(
        await measure("GET", base + "/sobjects/Account/001000000000000AAA"),
        404,
      );
      for (const [method, path, status] of [
        ["POST", base + "/openapi.yaml", 405],
        ["GET", "/services/data/v99.0/openapi.yaml", 404],
        ["HEAD", base + "/openapi.yaml", 405],
      ] as const) {
        const e = await measure(method, path);
        assert.equal(e.response.status, status);
        const r = validator.validate(e);
        assert.equal(r.operationId, undefined);
        assert.deepEqual(r.requestIssues, [
          method === "GET" ? "route" : "method",
        ]);
        if (status === 405) assert.equal(e.allow, "GET");
        if (method === "HEAD") assert.equal(e.response.body.byteLength, 0);
        else {
          const data = json(e);
          assert.ok(
            Array.isArray(data) &&
              data.length === 1 &&
              typeof data[0].errorCode === "string" &&
              typeof data[0].message === "string",
            "direct error envelope invalid",
          );
        }
      }
    } finally {
      let failed = false;
      for (const { id, event } of [...rows.values()].sort(
        (a, b) =>
          (a.event.object === "Contact" ? 0 : 1) -
          (b.event.object === "Contact" ? 0 : 1),
      )) {
        try {
          const path = base + `/sobjects/${event.object}/${id}`;
          checked(await measure("DELETE", path), 204);
          await recordFixtureEvent(journal, { ...event, id, kind: "deleted" });
          checked(await measure("GET", path), 404);
        } catch {
          failed = true;
        }
      }
      assert.ok(
        !failed,
        "owned fixture cleanup failed; retained journal requires recovery",
      );
    }
    assert.deepEqual([...observed].sort(), [...expectedIds].sort());
  },
);

test("OAuth media validates forms and HTML without bypassing schema checks", async () => {
  const d = parse(await readFile(ROOT + "/api/candidate-openapi.yaml", "utf8"));
  const validator = await createCandidateTrafficValidator(d);
  const exchange = (
    body: string,
    html = "<html>login</html>",
  ): CandidateExchange => ({
    method: "POST",
    path: "/services/oauth2/authorize",
    request: {
      contentType: "application/x-www-form-urlencoded",
      body: Buffer.from(body),
    },
    response: {
      status: 200,
      contentType: "text/html; charset=utf-8",
      body: Buffer.from(html),
    },
  });
  assert.deepEqual(
    validator.validate(exchange("request_id=x&decision=allow")).requestIssues,
    [],
  );
  for (const body of [
    "request_id=%ZZ&decision=allow",
    "request_id=x&request_id=y&decision=allow",
    "request_id=x&decision=allow&unknown=x",
  ])
    assert.ok(validator.validate(exchange(body)).requestIssues.length > 0);
  assert.deepEqual(
    validator.validate(exchange("request_id=x&decision=allow", ""))
      .responseIssues,
    ["response-schema"],
  );
});

test("OAuth media compiler remains closed to unsupported security and content", async () => {
  const d = parse(await readFile(ROOT + "/api/candidate-openapi.yaml", "utf8"));
  for (const mutate of [
    (x: Record<string, unknown>) => {
      (
        (
          x.paths as Record<
            string,
            Record<string, Record<string, Record<string, unknown>>>
          >
        )["/services/oauth2/token"].post as Record<string, unknown>
      ).security = [{ unknown: [] }];
    },
    (x: Record<string, unknown>) => {
      (
        (
          x.paths as Record<
            string,
            Record<string, Record<string, Record<string, unknown>>>
          >
        )["/services/oauth2/token"].post as Record<
          string,
          Record<string, Record<string, unknown>>
        >
      ).requestBody.content = {
        "text/plain": { schema: { type: "string" } },
      };
    },
    (x: Record<string, unknown>) => {
      (
        (
          (
            x.components as Record<
              string,
              Record<string, Record<string, Record<string, unknown>>>
            >
          ).schemas as Record<
            string,
            Record<string, Record<string, Record<string, unknown>>>
          >
        )["OAuthError"].properties as Record<
          string,
          Record<string, Record<string, unknown>>
        >
      ).error.format = "unknown";
    },
  ]) {
    const x = structuredClone(d);
    mutate(x);
    await assert.rejects(
      createCandidateTrafficValidator(x),
      /candidate-openapi-spec/,
    );
  }
});
