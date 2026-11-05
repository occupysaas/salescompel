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
  ["/services/data/v65.0/composite", "post", "candidateCompositeAtomic"],
  ["/services/data", "get", "candidateVersions"],
  ["/services/data/v65.0", "get", "candidateResources"],
  ["/services/oauth2/userinfo", "get", "candidateUserInfo"],
  ["/services/data/v51.0/sobjects/User/{userId}", "get", "candidateOwnUser51"],
  ["/services/data/v65.0/sobjects/User/{userId}", "get", "candidateOwnUser65"],
  ["/services/data/v65.0/sobjects", "get", "candidateDescribeGlobal"],
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
const doc = async (): Promise<any> => loadCandidateOpenAPI();
const urls = async () =>
  new Set<string>(
    parse(
      await readFile(ROOT + "/docs/compat/sources.yaml", "utf8"),
    ).sources.map((s: any) => s.url),
  );

test("candidateContractInventory", async () => {
  const d = await doc();
  assert.equal(d.openapi, "3.2.1");
  assert.deepEqual(d.servers, [{ url: "/" }]);
  const actual = Object.entries(d.paths).flatMap(([path, p]: [string, any]) =>
    Object.entries(p)
      .filter(([method]) => ["get", "post", "patch", "delete"].includes(method))
      .map(([method, op]: [string, any]) => [path, method, op.operationId]),
  );
  assert.deepEqual(actual.sort(), structuredClone(expected).sort());
  assert.deepEqual(d.security, [{ bearerAuth: [] }]);
  for (const mutate of [
    (x: any) =>
      (x.paths["/services/data/v65.0/composite/batch"] = {
        post: x.paths["/services/data/v65.0/sobjects/Account"].post,
      }),
    (x: any) => {
      x.paths["/services/data/v{version}"] = x.paths["/services/data"];
      delete x.paths["/services/data"];
    },
    (x: any) => (x.servers = [{ url: "https://PRIVATE_MARKER.invalid" }]),
    (x: any) => (x.security = []),
    (x: any) =>
      (x.paths["/services/data"].get.responses.default = {
        description: "anything",
      }),
    (x: any) => {
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
    (x: any) => delete x.paths["/services/data"].get["x-source"],
    (x: any) =>
      (x.paths["/services/data"].get["x-source"] = [
        "https://PRIVATE_MARKER.invalid",
      ]),
    (x: any) => (x.components.schemas.AccountRead = {}),
    (x: any) =>
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
        (p: any) => p.name === "recordId",
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
