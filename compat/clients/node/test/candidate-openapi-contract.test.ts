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

const expected = [
  [
    "/services/data/v65.0/sobjects/{object}/describe",
    "get",
    "candidateDescribeDynamic",
  ],
  ["/services/data/v65.0/sobjects/{object}", "post", "candidateCreateDynamic"],
  [
    "/services/data/v65.0/sobjects/{object}/{recordId}",
    "get",
    "candidateReadDynamic",
  ],
  [
    "/services/data/v65.0/sobjects/{object}/{recordId}",
    "patch",
    "candidateUpdateDynamic",
  ],
  [
    "/services/data/v65.0/sobjects/{object}/{recordId}",
    "delete",
    "candidateDeleteDynamic",
  ],
  ["/services/oauth2/authorize", "get", "candidateOAuthAuthorizeForm"],
  ["/services/oauth2/authorize", "post", "candidateOAuthAuthorizeSubmit"],
  ["/services/oauth2/token", "post", "candidateOAuthToken"],
  ["/services/oauth2/revoke", "post", "candidateOAuthRevoke"],
  ["/services/oauth2/introspect", "post", "candidateOAuthIntrospect"],
  ["/.well-known/openid-configuration", "get", "candidateOAuthDiscovery"],
  ["/id/{orgId}/{userId}", "get", "candidateOAuthIdentity"],

  ["/services/data/v65.0/composite", "post", "candidateCompositeAtomic"],
  ["/services/data", "get", "candidateVersions"],
  ["/services/data/v65.0", "get", "candidateResources"],
  ["/services/oauth2/userinfo", "get", "candidateUserInfo"],
  ["/services/data/v51.0/sobjects/User/{userId}", "get", "candidateOwnUser51"],
  ["/services/data/v65.0/sobjects/User/{userId}", "get", "candidateOwnUser65"],
  ["/services/data/v65.0/sobjects", "get", "candidateDescribeGlobal"],
  [
    "/services/data/v65.0/candidate/sobjects/{object}/storedFields",
    "get",
    "candidateStoredFields",
  ],
  [
    "/services/data/v65.0/sobjects/Account/describe",
    "get",
    "candidateDescribeAccount",
  ],
  [
    "/services/data/v65.0/sobjects/Contact/describe",
    "get",
    "candidateDescribeContact",
  ],
  ["/services/data/v65.0/sobjects/Account", "post", "candidateCreateAccount"],
  ["/services/data/v65.0/sobjects/Contact", "post", "candidateCreateContact"],
  [
    "/services/data/v65.0/sobjects/Account/{recordId}",
    "get",
    "candidateReadAccount",
  ],
  [
    "/services/data/v65.0/sobjects/Account/{recordId}",
    "patch",
    "candidateUpdateAccount",
  ],
  [
    "/services/data/v65.0/sobjects/Account/{recordId}",
    "delete",
    "candidateDeleteAccount",
  ],
  [
    "/services/data/v65.0/sobjects/Contact/{recordId}",
    "get",
    "candidateReadContact",
  ],
  [
    "/services/data/v65.0/sobjects/Contact/{recordId}",
    "patch",
    "candidateUpdateContact",
  ],
  [
    "/services/data/v65.0/sobjects/Contact/{recordId}",
    "delete",
    "candidateDeleteContact",
  ],
  [
    "/services/data/v65.0/sobjects/Account/Compat_Key__c/{externalId}",
    "patch",
    "candidateUpsertAccount",
  ],
  ["/services/data/v65.0/query", "get", "candidateQuery"],
  ["/services/data/v65.0/openapi.yaml", "get", "candidateOpenAPI"],
];
const doc = async (): Promise<Record<string, any>> =>
  loadCandidateOpenAPI() as Promise<Record<string, any>>;
const urls = async () =>
  new Set<string>(
    parse(
      await readFile(ROOT + "/docs/compat/sources.yaml", "utf8"),
    ).sources.map((s: { url: string }) => s.url),
  );

test("candidateContractInventory", async () => {
  const d = await doc();
  assert.equal(d.openapi, "3.2.1");
  assert.deepEqual(d.servers, [{ url: "/" }]);
  const actual = Object.entries(d.paths).flatMap(([path, p]) =>
    Object.entries(p as Record<string, Record<string, unknown>>)
      .filter(([method]) => ["get", "post", "patch", "delete"].includes(method))
      .map(([method, op]) => [path, method, op.operationId]),
  );
  assert.deepEqual(actual.sort(), structuredClone(expected).sort());
  assert.deepEqual(d.security, [{ bearerAuth: [] }]);
  for (const mutate of [
    (x: Record<string, any>) =>
      (x.paths["/services/data/v65.0/composite/batch"] = {
        post: x.paths["/services/data/v65.0/sobjects/Account"].post,
      }),
    (x: Record<string, any>) => {
      x.paths["/services/data/v{version}"] = x.paths["/services/data"];
      delete x.paths["/services/data"];
    },
    (x: Record<string, any>) =>
      (x.servers = [{ url: "https://PRIVATE_MARKER.invalid" }]),
    (x: Record<string, any>) => (x.security = []),
    (x: Record<string, any>) =>
      (x.paths["/services/data"].get.responses.default = {
        description: "anything",
      }),
    (x: Record<string, any>) => {
      x.paths["/services/data"].get.responses["201"] =
        x.paths["/services/data"].get.responses["200"];
      delete x.paths["/services/data"].get.responses["200"];
    },
  ]) {
    const x = structuredClone(d);
    mutate(x);
    await assert.rejects(
      validateCandidateOpenAPI(x, await urls()),
      /^Error: candidate-openapi-/,
    );
  }
});
test("candidateContractProvenance", async () => {
  const d = await doc();
  const registered = await urls();
  for (const [path, method] of expected) {
    assert.ok(d.paths[path][method]["x-source"].length);
    for (const u of d.paths[path][method]["x-source"])
      assert.ok(registered.has(u));
  }
  for (const mutate of [
    (x: Record<string, any>) =>
      delete x.paths["/services/data"].get["x-source"],
    (x: Record<string, any>) =>
      (x.paths["/services/data"].get["x-source"] = [
        "https://PRIVATE_MARKER.invalid",
      ]),
    (x: Record<string, any>) => (x.components.schemas.AccountRead = {}),
    (x: Record<string, any>) =>
      (x.components.schemas.AccountRead = {
        $ref: "https://PRIVATE_MARKER.invalid",
      }),
  ]) {
    const x = structuredClone(d);
    mutate(x);
    await assert.rejects(
      validateCandidateOpenAPI(x, registered),
      /^Error: candidate-openapi-/,
    );
  }
});
test("candidateContractShapes", async () => {
  const d = await doc();
  const definitions = JSON.parse(
    JSON.stringify(d.components.schemas).replaceAll(
      "#/components/schemas/",
      "#/$defs/",
    ),
  );
  const check = (name: string, data: unknown) =>
    new Ajv2020({ strict: true }).compile({
      $defs: definitions,
      $ref: "#/$defs/" + name,
    })(data);
  for (const [schema, good, bad] of [
    ["AccountCreate", { Name: "A", Compat_Text__c: null }, {}],
    [
      "ContactCreate",
      { LastName: "Contact", AccountId: null },
      { FirstName: "only" },
    ],
    ["AccountPatch", { Compat_Key__c: null }, { Id: "readonly" }],
    ["ContactPatch", {}, { Unknown: "field" }],
    [
      "AccountUpsert",
      { Name: "A", Compat_Key__c: null },
      { Compat_Key__c: "path-only" },
    ],
    [
      "AccountProjection",
      { attributes: { type: "Account" }, Name: "A" },
      { attributes: { type: "Account" }, Unknown: "field" },
    ],
    [
      "ContactProjection",
      { attributes: { type: "Contact" }, Account: null },
      { attributes: { type: "Contact" }, Account: 7 },
    ],
    [
      "AccountRead",
      {
        attributes: { type: "Account" },
        Id: "id",
        Name: "A",
        Compat_Key__c: null,
      },
      { attributes: { type: "Account" }, Name: "A" },
    ],
  ] as const) {
    assert.equal(check(schema, good), true, schema);
    assert.equal(check(schema, bad), false, schema);
  }
  assert.equal(
    check("AccountCreate", {
      Name: "a".repeat(255),
      Compat_Text__c: "x".repeat(80),
    }),
    true,
  );
  assert.equal(check("AccountCreate", { Name: "a".repeat(256) }), false);
  assert.equal(
    check("AccountCreate", { Name: "A", Compat_Key__c: "x".repeat(81) }),
    false,
  );
  assert.equal(check("AccountCreate", { Name: 3 }), false);
  assert.equal(
    check("Resources", {
      sobjects: "/services/data/v65.0/sobjects",
      query: "/services/data/v65.0/query",
      openapi: "/services/data/v65.0/openapi.yaml",
      composite: "/services/data/v65.0/composite",
    }),
    true,
  );
  assert.equal(
    d.paths["/services/data/v65.0/sobjects/Account/{recordId}"].patch.responses[
      "204"
    ].content,
    undefined,
  );
});

test("atomic Composite contract fixes the restricted request and result envelopes", async () => {
  const d = await doc();
  const c = d.components.schemas;
  assert.equal(c.CompositeRequest.properties.allOrNone.const, true);
  assert.equal(c.CompositeRequest.properties.collateSubrequests.const, false);
  assert.equal(c.CompositeRequest.properties.compositeRequest.minItems, 1);
  assert.equal(c.CompositeRequest.properties.compositeRequest.maxItems, 25);
  assert.equal(c.CompositeEmptyHeaders.maxProperties, 0);
  assert.equal(c.CompositeEmptyHeaders.additionalProperties, false);
  assert.equal(c.CompositeReferenceID.maxLength, 80);
  assert.equal(
    c.Resources.properties.composite.const,
    "/services/data/v65.0/composite",
  );
  const mutated = structuredClone(d);
  mutated.components.schemas.AccountRead = {
    type: "object",
    properties: {},
    additionalProperties: false,
    maxProperties: 0,
  };
  await assert.rejects(validateCandidateOpenAPI(mutated, await urls()));
});

test("candidate foundation ID forms and record errors are documented", async () => {
  const d = await doc();
  for (const object of ["Account", "Contact"]) {
    const path = d.paths[`/services/data/v65.0/sobjects/${object}/{recordId}`];
    for (const method of ["get", "patch", "delete"]) {
      const parameter = path[method].parameters.find(
        (p: { name: string; description: string }) => p.name === "recordId",
      );
      assert.match(parameter.description, /15-character/);
      assert.match(parameter.description, /18-character/);
      assert.match(parameter.description, /case-sensitive/);
      assert.equal(
        path[method].responses["404"].$ref,
        "#/components/responses/Error",
      );
    }
    assert.equal(
      d.paths[`/services/data/v65.0/sobjects/${object}/describe`].get.responses[
        "404"
      ].$ref,
      "#/components/responses/Error",
    );
  }
  const upsert =
    d.paths["/services/data/v65.0/sobjects/Account/Compat_Key__c/{externalId}"]
      .patch;
  assert.match(upsert.parameters[0].description, /ASCII spaces/);
  assert.match(upsert.parameters[0].description, /NFC/);
  assert.match(upsert.parameters[0].description, /full case folding/);
  assert.equal(d.components.schemas.Error.additionalProperties, false);
});

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
