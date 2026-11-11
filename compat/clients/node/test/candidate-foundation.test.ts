import assert from "node:assert/strict";
import test from "node:test";
import { Connection } from "jsforce";
import { request as httpsRequest } from "node:https";
import { parse } from "yaml";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import {
  recordFixtureEvent,
  type FixtureEvent,
} from "../src/candidate-acceptance-fixtures.ts";
import type {
  CandidateExchange,
  CandidateTrafficValidator,
} from "../src/candidate-openapi-model.ts";

const base = "/services/data/v65.0";
test(
  "candidate/foundation/decisions",
  { skip: process.env.RUN_CANDIDATE !== "1" },
  async () => {
    const runId = process.env.CANDIDATE_RUN_ID!,
      journal = process.env.CANDIDATE_FIXTURE_JOURNAL!;
    assert.ok(
      /^[a-f0-9]{24}$/.test(runId) && journal,
      "owned fixture context required",
    );
    const url = new URL(process.env.CANDIDATE_URL!);
    assert.ok(
      url.protocol === "https:" &&
        ["localhost", "127.0.0.1"].includes(url.hostname),
    );
    const tokens = {
      admin: process.env.CANDIDATE_TOKEN!,
      restricted: process.env.CANDIDATE_RESTRICTED_TOKEN!,
      other: process.env.CANDIDATE_OTHER_TOKEN!,
    };
    type Role = keyof typeof tokens;
    let validator: CandidateTrafficValidator | undefined;
    const issues: unknown[] = [];
    let expectedRequestIssues: string[] = [];
    function validate(e: CandidateExchange, invalidObjectParameter = false) {
      const result = validator!.validate(e);
      if (
        JSON.stringify(result.requestIssues) !==
          JSON.stringify(
            invalidObjectParameter ? ["parameters"] : expectedRequestIssues,
          ) ||
        result.responseIssues.length
      )
        issues.push(result);
    }
    async function raw(
      method: string,
      path: string,
      body?: unknown,
      role: Role = "admin",
      invalidObjectParameter = false,
    ) {
      const input =
        body === undefined ? undefined : Buffer.from(JSON.stringify(body));
      const exchange = await new Promise<CandidateExchange>(
        (resolve, reject) => {
          const req = httpsRequest(
            new URL(path, url),
            {
              method,
              headers: {
                Authorization: "Bearer " + tokens[role],
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
        },
      );
      if (validator) validate(exchange, invalidObjectParameter);
      return {
        status: exchange.response.status,
        body: Buffer.from(exchange.response.body).toString("utf8"),
        exchange,
      };
    }
    const served = await raw("GET", base + "/openapi.yaml");
    assert.equal(served.status, 200);
    validator = await createCandidateTrafficValidator(parse(served.body));
    validate(served.exchange);
    function connection(role: Role) {
      const conn = new Connection({
        instanceUrl: url.origin,
        accessToken: tokens[role],
        version: "65.0",
      });
      // Observe the real transport without replacing requests or client behavior.
      type TransportRequest = {
        url: string;
        method: string;
        headers?: Record<string, string | undefined>;
        body?: string;
      };
      type TransportResponse = {
        statusCode: number;
        headers: Record<string, string | string[] | undefined>;
        body?: string | Buffer;
      };
      type Transport = {
        httpRequest: (
          req: TransportRequest,
          options: unknown,
        ) => Promise<TransportResponse>;
      };
      const transport = (conn as unknown as { _transport: Transport })
          ._transport,
        original = transport.httpRequest.bind(transport);
      transport.httpRequest = (req: TransportRequest, options: unknown) => {
        const pending = original(req, options);
        void pending.then(
          (res: TransportResponse) => {
            const target = new URL(req.url);
            assert.equal(target.origin, url.origin);
            validate({
              method: req.method,
              path: target.pathname + target.search,
              request: {
                contentType:
                  req.headers?.["Content-Type"] ??
                  req.headers?.["content-type"] ??
                  null,
                body:
                  req.body === undefined ? undefined : Buffer.from(req.body),
              },
              response: {
                status: res.statusCode,
                contentType: res.headers["content-type"] ?? null,
                body: Buffer.from(res.body ?? ""),
              },
            });
          },
          () => {},
        );
        return pending;
      };
      return conn;
    }
    const admin = connection("admin"),
      restricted = connection("restricted"),
      other = connection("other");
    const marker = (label: string) => `p25-${runId}-${label}`;
    function event(
      label: string,
      names: string[],
      principal: Role = "admin",
    ): FixtureEvent {
      return {
        schemaVersion: 1,
        runId,
        operationId: "p25-" + label,
        principal,
        object: "Account",
        names,
        kind: "intent",
      };
    }
    async function create(
      label: string,
      data: Record<string, unknown>,
      principal: Role = "admin",
    ) {
      const e = event(label, [data.Name as string], principal);
      await recordFixtureEvent(journal, e);
      const conn = principal === "other" ? other : admin;
      const result = (await conn
        .sobject("Account")
        .create(data)) as unknown as { id: string; success: boolean };
      await recordFixtureEvent(journal, { ...e, kind: "known", id: result.id });
      assert.equal(result.success, true);
      return result.id as string;
    }
    const name = marker("MixedCase"),
      id = await create("seed", { Name: name });
    const record = (await admin
      .sobject("Account")
      .retrieve(id.slice(0, 15))) as unknown as { Id: string };
    assert.equal(record.Id, id);
    await admin
      .sobject("Account")
      .update({ Id: id.slice(0, 15), Compat_Text__c: "value" });
    assert.equal(
      ((await admin.sobject("Account").retrieve(id)) as Record<string, unknown>)
        .Compat_Text__c,
      "value",
    );
    const result = (await admin.query(
      `select id, name from account where name = '${name.toLowerCase()}'`,
    )) as unknown as {
      records: {
        Id: string;
        Name: string;
        attributes: { type: string };
      }[];
    };
    assert.equal(result.records.length, 1);
    assert.equal(result.records[0].Id, id);
    assert.equal(result.records[0].Name, name);
    assert.equal(result.records[0].attributes.type, "Account");
    const orderName = marker("order");
    const blank = await create("blank", { Name: orderName });
    await create("filled", { Name: orderName, Compat_Text__c: "z" });
    for (const direction of ["ASC", "DESC"])
      for (const placement of ["", " NULLS LAST"]) {
        const q = (await admin.query(
          `select id, compat_text__c from account where name = '${orderName}' order by compat_text__c ${direction}${placement}`,
        )) as unknown as { records: { Id: string }[] };
        assert.equal(q.records.length, 2);
        assert.equal(q.records[placement ? 1 : 0].Id, blank);
      }
    const spelling = "AbC-" + runId,
      upsertName = marker("upsert");
    let upsertID = "";
    for (const [i, key] of [
      spelling,
      spelling.toLowerCase(),
      " " + spelling.toLowerCase() + " ",
    ].entries()) {
      const e = event("upsert-" + i, [upsertName]);
      await recordFixtureEvent(journal, e);
      const r = (await admin
        .sobject("Account")
        .upsert({ Name: upsertName, Compat_Key__c: key }, "Compat_Key__c")) as unknown as {
        id: string;
        created: boolean;
      };
      await recordFixtureEvent(journal, { ...e, kind: "known", id: r.id });
      assert.equal(r.created, i === 0);
      if (i === 0) upsertID = r.id;
      else assert.equal(r.id, upsertID);
    }
    assert.equal(
      ((await admin.sobject("Account").retrieve(upsertID)) as Record<
        string,
        unknown
      >).Compat_Key__c,
      spelling,
    );
    const duplicate = event("duplicate", [marker("duplicate")]);
    await recordFixtureEvent(journal, duplicate);
    await assert.rejects(
      admin.sobject("Account").create({
        Name: duplicate.names[0],
        Compat_Key__c: spelling.toLowerCase(),
      }),
      { errorCode: "DUPLICATE_VALUE" },
    );
    await recordFixtureEvent(journal, { ...duplicate, kind: "rejected" });
    await assert.rejects(
      async () => await restricted.query("select compat_text__c from account"),
      { errorCode: "INVALID_FIELD" },
    );
    // Dynamic Text maps admit the spelling; field policy must still deny it.
    expectedRequestIssues = [];
    await assert.rejects(
      restricted
        .sobject("Account")
        .update({ Id: id, compat_text__c: "denied" } as unknown),
      { errorCode: "INVALID_FIELD" },
    );
    expectedRequestIssues = [];
    await assert.rejects(other.sobject("Account").retrieve(id), {
      errorCode: "NOT_FOUND",
    });
    const peer = await create(
      "other",
      { Name: marker("other"), Compat_Key__c: spelling.toLowerCase() },
      "other",
    );
    assert.notEqual(peer, upsertID);
    const unknown = await raw(
      "GET",
      base + "/sobjects/Nope/describe",
      undefined,
      "admin",
      true,
    );
    assert.equal(unknown.status, 404);
    assert.equal(JSON.parse(unknown.body)[0].errorCode, "NOT_FOUND");
    await assert.rejects(async () => await admin.query("select id from Nope"), {
      errorCode: "INVALID_TYPE",
    });
    assert.deepEqual(
      issues,
      [],
      "all measured exchanges must satisfy the served contract",
    );
  },
);
