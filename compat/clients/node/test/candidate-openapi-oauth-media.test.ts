import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { parse } from "yaml";
import { ROOT } from "../src/versions.ts";
import { createCandidateTrafficValidator } from "../src/candidate-openapi-traffic.ts";
import type { CandidateExchange } from "../src/candidate-openapi-model.ts";

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
      const components = x.components as Record<string, any>;
      components.schemas.OAuthError.properties.error.format = "unknown";
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
