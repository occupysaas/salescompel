import assert from "node:assert/strict";
import test from "node:test";
import { request as httpsRequest } from "node:https";
import { dirname } from "node:path";
import { parse } from "yaml";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import { runCompositeFixtureAttempt } from "../src/candidate-composite-fixtures.ts";
import {
  candidateRequest,
  createFixture,
  recordFixtureEvent,
  type FixtureEvent,
  type Response,
} from "../src/candidate-acceptance-fixtures.ts";
import type { GateContext } from "../src/candidate-acceptance-preflight.ts";
import type { CandidateExchange } from "../src/candidate-openapi-model.ts";
const base = "/services/data/v65.0";
type Role = FixtureEvent["principal"];
type Sub = {
  method: "POST" | "PATCH" | "DELETE";
  url: string;
  referenceId: string;
  body?: Record<string, unknown>;
};
test(
  "candidate/composite/atomic",
  { skip: process.env.RUN_CANDIDATE !== "1" },
  async () => {
    const runId = process.env.CANDIDATE_RUN_ID!,
      journal = process.env.CANDIDATE_FIXTURE_JOURNAL!;
    assert.ok(
      /^[a-f0-9]{24}$/.test(runId) && journal,
      "owned fixture context required",
    );
    const c = {
      runId,
      fixtureJournal: journal,
      url: process.env.CANDIDATE_URL!,
      scratch: dirname(process.env.NODE_EXTRA_CA_CERTS!),
      tokens: {
        admin: process.env.CANDIDATE_TOKEN!,
        restricted: process.env.CANDIDATE_RESTRICTED_TOKEN!,
        other: process.env.CANDIDATE_OTHER_TOKEN!,
      },
    } as GateContext;
    const url = new URL(c.url);
    assert.ok(
      url.protocol === "https:" &&
        ["localhost", "127.0.0.1"].includes(url.hostname),
      "owned HTTPS candidate required",
    );
    async function raw(
      method: string,
      path: string,
      body?: unknown,
      role: Role = "admin",
    ): Promise<CandidateExchange> {
      const input =
        body === undefined ? undefined : Buffer.from(JSON.stringify(body));
      return new Promise((resolve, reject) => {
        const req = httpsRequest(
          new URL(path, url),
          {
            method,
            headers: {
              Authorization: "Bearer " + c.tokens[role],
              ...(input ? { "Content-Type": "application/json" } : {}),
            },
            timeout: 15000,
          },
          (res) => {
            const chunks: Buffer[] = [];
            let size = 0;
            res.on("data", (chunk: Buffer) => {
              size += chunk.length;
              if (size > 8388608) req.destroy(Error("RESULT"));
              else chunks.push(chunk);
            });
            res.on("end", () =>
              resolve({
                method,
                path,
                request: {
                  contentType: input ? "application/json" : null,
                  body: input,
                },
                response: {
                  status: res.statusCode ?? 0,
                  contentType: res.headers["content-type"] ?? null,
                  body: Buffer.concat(chunks),
                },
              }),
            );
            res.on("error", () => reject(Error("PROCESS")));
          },
        );
        req.on("timeout", () => req.destroy(Error("TIMEOUT")));
        req.on("error", () => reject(Error("PROCESS")));
        req.end(input);
      });
    }
    const served = await raw("GET", base + "/openapi.yaml");
    assert.equal(served.response.status, 200);
    const v = await createCandidateTrafficValidator(
      parse(
        new TextDecoder("utf-8", { fatal: true }).decode(served.response.body),
      ),
    );
    const completed = new Set<string>();
    const marker = (ref: string) => `p24-${runId}-${ref}`;
    const rowPath = (object: string, id: string) =>
      base + "/sobjects/" + object + "/" + id;
    const sub = (
      method: Sub["method"],
      object: string,
      id: string,
      referenceId: string,
      body?: Record<string, unknown>,
    ): Sub => ({
      method,
      url: base + "/sobjects/" + object + (id ? "/" + id : ""),
      referenceId,
      ...(body ? { body } : {}),
    });
    async function reconcile(e: FixtureEvent) {
      const field = e.object === "Account" ? "Name" : "LastName";
      const q = `SELECT Id FROM ${e.object} WHERE ${field} = '${e.names[0]}' LIMIT 26`;
      const result = await candidateRequest(
        c,
        e.principal,
        base + "/query?q=" + encodeURIComponent(q),
      );
      if (result.status !== 200 || !Array.isArray(result.body?.records))
        throw Error("FIXTURES");
      const rows = result.body.records;
      for (const row of rows) {
        if (typeof row?.Id !== "string" || !/^[A-Za-z0-9]{18}$/.test(row.Id))
          throw Error("FIXTURES");
        if (rows.length === 1)
          await recordFixtureEvent(journal, {
            ...e,
            kind: "known",
            id: row.Id,
          });
        else {
          const recovery = {
            ...e,
            operationId: e.operationId + "__recovered_" + row.Id,
          };
          await recordFixtureEvent(journal, recovery);
          await recordFixtureEvent(journal, {
            ...recovery,
            kind: "known",
            id: row.Id,
          });
        }
      }
      if (rows.length) throw Error("FIXTURES");
    }
    async function composite(
      subs: Sub[],
      outcome: "commit" | "rollback" | "prevalidation",
      role: Role = "admin",
      extra: Record<string, unknown> = {},
      requestIssues: string[] = [],
    ): Promise<Response> {
      const intents: FixtureEvent[] = subs
        .filter((s) => s.method === "POST")
        .map((s) => ({
          schemaVersion: 1,
          runId,
          operationId: s.referenceId,
          principal: role,
          object: s.url.endsWith("/Account") ? "Account" : "Contact",
          names: [marker(s.referenceId)],
          kind: "intent",
        }));
      const body = { allOrNone: true, compositeRequest: subs, ...extra };
      let measured: CandidateExchange | undefined;
      const result = await runCompositeFixtureAttempt(
        journal,
        intents,
        subs.map((s) => ({ referenceId: s.referenceId, method: s.method })),
        outcome,
        async () => {
          measured = await raw("POST", base + "/composite", body, role);
          let parsed: unknown;
          try {
            parsed = JSON.parse(
              new TextDecoder("utf-8", { fatal: true }).decode(
                measured.response.body,
              ),
            );
          } catch {
            throw Error("RESULT");
          }
          return {
            status: measured.response.status,
            body: parsed as Record<string, any>,
          };
        },
        reconcile,
      );
      assert.ok(measured, "HTTP exchange required");
      const checked = v.validate(measured);
      assert.equal(checked.operationId, "candidateCompositeAtomic");
      assert.deepEqual(checked.requestIssues, requestIssues);
      assert.deepEqual(checked.responseIssues, []);
      return result;
    }
    const linked = await composite(
      [
        sub("POST", "Account", "", "p24parent", { Name: marker("p24parent") }),
        sub("POST", "Contact", "", "p24child", {
          LastName: marker("p24child"),
          AccountId: "@{p24parent.id}",
        }),
      ],
      "commit",
    );
    const aid = linked.body!.compositeResponse[0].body.id,
      cid = linked.body!.compositeResponse[1].body.id;
    const contact = await candidateRequest(c, "admin", rowPath("Contact", cid));
    assert.ok(
      contact.status === 200 && contact.body!.AccountId === aid,
      "linked creation did not persist",
    );
    completed.add("linked-create");
    await composite(
      [sub("PATCH", "Contact", cid, "p24patch", { FirstName: "Changed" })],
      "commit",
    );
    assert.ok(
      (await candidateRequest(c, "admin", rowPath("Contact", cid))).body!
        .FirstName === "Changed",
      "PATCH did not persist",
    );
    await composite([sub("DELETE", "Contact", cid, "p24delete")], "commit");
    assert.equal(
      (await candidateRequest(c, "admin", rowPath("Contact", cid))).status,
      404,
    );
    completed.add("patch-delete");
    const rollback = await composite(
      [
        sub("POST", "Account", "", "p24rollback", {
          Name: marker("p24rollback"),
        }),
        sub("PATCH", "Account", "@{p24rollback.id}", "p24bad", {
          Unknown: true,
        }),
      ],
      "rollback",
      "admin",
      {},
      ["request-schema"],
    );
    assert.ok(
      rollback.body!.compositeResponse[0].body[0].errorCode ===
        "PROCESSING_HALTED" &&
        rollback.body!.compositeResponse[1].body[0].errorCode ===
          "INVALID_FIELD",
      "observed rollback contract drift",
    );
    completed.add("invalid-field-rollback");
    await composite(
      [
        sub("PATCH", "Account", aid, "p24restorepatch", { Name: "Temporary" }),
        sub("DELETE", "Account", aid, "p24restoredelete"),
        sub("POST", "Contact", "", "p24invalidcontact", {}),
      ],
      "rollback",
      "admin",
      {},
      ["request-schema"],
    );
    assert.ok(
      (await candidateRequest(c, "admin", rowPath("Account", aid))).body!
        .Name === marker("p24parent"),
      "PATCH/DELETE rollback failed",
    );
    completed.add("restore-update-delete");
    await composite(
      [
        sub("POST", "Account", "", "p24restricted", {
          Name: marker("p24restricted"),
        }),
        sub("PATCH", "Account", "@{p24restricted.id}", "p24denied", {
          Compat_Text__c: "Denied",
        }),
      ],
      "rollback",
      "restricted",
    );
    completed.add("restricted-rollback");
    await composite(
      [
        sub("POST", "Account", "", "p24prevalidation", {
          Name: marker("p24prevalidation"),
        }),
      ],
      "prevalidation",
      "admin",
      { allOrNone: false },
      ["request-schema"],
    );
    completed.add("prevalidation");
    const otherId = await createFixture(
      c,
      "p24otherseed",
      "Account",
      { Name: marker("p24otherseed") },
      [marker("p24otherseed")],
      "other",
    );
    for (const method of ["PATCH", "DELETE"] as const) {
      const ref = "p24cross" + method;
      const response = await composite(
        [
          sub("POST", "Account", "", ref, { Name: marker(ref) }),
          sub(
            method,
            "Account",
            otherId,
            ref + "target",
            method === "PATCH" ? { Name: "Forbidden" } : undefined,
          ),
        ],
        "rollback",
      );
      assert.equal(response.body!.compositeResponse[1].httpStatusCode, 404);
    }
    const untouched = await candidateRequest(
      c,
      "other",
      rowPath("Account", otherId),
    );
    assert.ok(
      untouched.status === 200 &&
        untouched.body!.Name === marker("p24otherseed"),
      "cross-tenant record changed",
    );
    assert.equal(
      (await candidateRequest(c, "admin", rowPath("Account", otherId))).status,
      404,
    );
    completed.add("cross-tenant");
    assert.deepEqual(
      [...completed].sort(),
      [
        "linked-create",
        "patch-delete",
        "invalid-field-rollback",
        "restore-update-delete",
        "restricted-rollback",
        "prevalidation",
        "cross-tenant",
      ].sort(),
    );
  },
);
