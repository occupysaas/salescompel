import assert from "node:assert/strict";
import test from "node:test";
import { readFile } from "node:fs/promises";
import { parse } from "yaml";
import { Ajv2020 } from "ajv/dist/2020.js";
import {
  loadCandidateOpenAPI,
  validateCandidateOpenAPI,
} from "../src/candidate-openapi-contract.ts";
import { ROOT } from "../src/versions.ts";
const doc = async (): Promise<Record<string, any>> =>
  loadCandidateOpenAPI() as Promise<Record<string, any>>;
const urls = async () =>
  new Set<string>(
    parse(
      await readFile(ROOT + "/docs/compat/sources.yaml", "utf8"),
    ).sources.map((s: { url: string }) => s.url),
  );
test("OAuth contract exposes the seven login operations and scoped bearer access", async () => {
  const d = await doc();
  const routes = [
    ["/services/oauth2/authorize", "get", "candidateOAuthAuthorizeForm"],
    ["/services/oauth2/authorize", "post", "candidateOAuthAuthorizeSubmit"],
    ["/services/oauth2/token", "post", "candidateOAuthToken"],
    ["/services/oauth2/revoke", "post", "candidateOAuthRevoke"],
    ["/services/oauth2/introspect", "post", "candidateOAuthIntrospect"],
    ["/.well-known/openid-configuration", "get", "candidateOAuthDiscovery"],
    ["/id/{orgId}/{userId}", "get", "candidateOAuthIdentity"],
  ];
  for (const [path, method, id] of routes) {
    const op = d.paths[path]?.[method];
    assert.equal(op?.operationId, id);
    assert.ok(op["x-source"].length > 0);
  }
  const schemes = Object.values(d.components.securitySchemes) as Record<
    string,
    any
  >[];
  assert.ok(schemes.some((s) => s.type === "http" && s.scheme === "bearer"));
  const oauth = schemes.find((s) => s.type === "oauth2");
  assert.ok(oauth?.flows.authorizationCode);
  assert.ok(oauth?.flows.clientCredentials);
  assert.equal(
    oauth.flows.authorizationCode.authorizationUrl,
    "/services/oauth2/authorize",
  );
  assert.equal(
    oauth.flows.authorizationCode.tokenUrl,
    "/services/oauth2/token",
  );
  assert.equal(
    oauth.flows.clientCredentials.tokenUrl,
    "/services/oauth2/token",
  );
  for (const path of [
    "/services/data/v65.0/openapi.yaml",
    "/services/oauth2/userinfo",
  ])
    assert.ok(d.paths[path].get.responses["403"]);
  assert.ok(!d.info.description.includes("not OAuth login/refresh"));
});

test("OAuth schemas are closed offline contracts including errors and Organization", async () => {
  const d = await doc();
  const definitions = JSON.parse(
    JSON.stringify(d.components.schemas).replaceAll(
      "#/components/schemas/",
      "#/$defs/",
    ),
  );
  const fixtures: Record<string, unknown> = {
    OAuthError: { error: "invalid_grant" },
    OAuthHTML: "<html>login</html>",
    OAuthTokenRequest: { grant_type: "client_credentials" },
    OAuthRevokeRequest: { token: "x" },
    OAuthIntrospectRequest: { token: "x" },
    OAuthAuthorizeRequest: { request_id: "x", decision: "allow" },
    OAuthToken: {
      access_token: "x",
      scope: "api",
      instance_url: "https://localhost",
      id: "https://localhost/id/o/u",
      issued_at: "1",
      token_type: "Bearer",
    },
    OAuthIntrospection: { active: false },
    OAuthIdentity: {
      id: "x",
      user_id: "u",
      organization_id: "o",
      username: "n",
    },
    OAuthDiscovery: Object.fromEntries(
      Object.entries(
        d.components.schemas.OAuthDiscovery.properties as Record<
          string,
          Record<string, unknown>
        >,
      ).map(([key, schema]) => [key, schema.type === "array" ? ["x"] : "x"]),
    ),
    OrganizationProjection: {
      Name: "tenant",
      InstanceName: null,
      IsSandbox: false,
      TrialExpirationDate: null,
      NamespacePrefix: null,
      OrganizationType: "SalesCompel",
    },
  };
  for (const [name, good] of Object.entries(fixtures)) {
    const check = new Ajv2020({ strict: true }).compile({
      $defs: definitions,
      $ref: "#/$defs/" + name,
    });
    assert.equal(check(good), true, name);
    assert.equal(
      check(name === "OAuthHTML" ? 42 : { ...(good as object), unknown: true }),
      false,
      name,
    );
  }
  const token = new Ajv2020({ strict: true }).compile({
    $defs: definitions,
    $ref: "#/$defs/OAuthToken",
  });
  assert.equal(
    token({
      ...(fixtures.OAuthToken as object),
      refresh_token: "r",
      signature: "s",
    }),
    true,
  );
  assert.equal(
    token({ ...(fixtures.OAuthToken as object), refresh_token: 42 }),
    false,
  );
  const error = new Ajv2020({ strict: true }).compile({
    $defs: definitions,
    $ref: "#/$defs/OAuthError",
  });
  assert.equal(error({ error_description: "missing error" }), false);
  for (const item of Object.values(d.paths) as Record<string, any>[])
    for (const operation of Object.values(item) as Record<string, any>[])
      if (operation.security === undefined)
        assert.ok(operation.responses["403"]);
});

test("OAuth flows and secured scope failures cannot be removed from the contract", async () => {
  const d = await doc();
  for (const mutate of [
    (x: Record<string, any>) => {
      delete x.components.securitySchemes.oauth2;
    },
    (x: Record<string, any>) => {
      delete x.paths["/services/data/v65.0/openapi.yaml"].get.responses["403"];
    },
  ]) {
    const x = structuredClone(d);
    mutate(x);
    await assert.rejects(validateCandidateOpenAPI(x, await urls()));
  }
});
