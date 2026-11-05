import { readFile } from "node:fs/promises";
import { parseAllDocuments } from "yaml";
import { ROOT } from "./versions.ts";
import { validateOpenApi } from "./openapi.ts";

// Inventory guard for this bounded delivery; schemas remain in the YAML source.
const base = "/services/data/v65.0";
export const candidateOperations: readonly (readonly [
  string,
  string,
  string,
  readonly string[],
])[] = [
  [
    "/services/oauth2/authorize",
    "get",
    "candidateOAuthAuthorizeForm",
    ["200", "302"],
  ],
  [
    "/services/oauth2/authorize",
    "post",
    "candidateOAuthAuthorizeSubmit",
    ["200", "303"],
  ],
  ["/services/oauth2/token", "post", "candidateOAuthToken", ["200"]],
  ["/services/oauth2/revoke", "post", "candidateOAuthRevoke", ["200"]],
  ["/services/oauth2/introspect", "post", "candidateOAuthIntrospect", ["200"]],
  [
    "/.well-known/openid-configuration",
    "get",
    "candidateOAuthDiscovery",
    ["200"],
  ],
  ["/id/{orgId}/{userId}", "get", "candidateOAuthIdentity", ["200"]],
  [base + "/composite", "post", "candidateCompositeAtomic", ["200"]],
  ["/services/data", "get", "candidateVersions", ["200"]],
  [base, "get", "candidateResources", ["200"]],
  ["/services/oauth2/userinfo", "get", "candidateUserInfo", ["200"]],
  ...["51", "65"].map(
    (v) =>
      [
        `/services/data/v${v}.0/sobjects/User/{userId}`,
        "get",
        `candidateOwnUser${v}`,
        ["200"],
      ] as const,
  ),
  [base + "/sobjects", "get", "candidateDescribeGlobal", ["200"]],
  [
    base + "/sobjects/{object}/describe",
    "get",
    "candidateDescribeDynamic",
    ["200"],
  ],
  [base + "/sobjects/{object}", "post", "candidateCreateDynamic", ["201"]],
  [
    base + "/sobjects/{object}/{recordId}",
    "get",
    "candidateReadDynamic",
    ["200"],
  ],
  [
    base + "/sobjects/{object}/{recordId}",
    "patch",
    "candidateUpdateDynamic",
    ["204"],
  ],
  [
    base + "/sobjects/{object}/{recordId}",
    "delete",
    "candidateDeleteDynamic",
    ["204"],
  ],
  ...["Account", "Contact"].flatMap((o) => [
    [
      base + `/sobjects/${o}/describe`,
      "get",
      `candidateDescribe${o}`,
      ["200"],
    ] as const,
    [base + `/sobjects/${o}`, "post", `candidateCreate${o}`, ["201"]] as const,
    [
      base + `/sobjects/${o}/{recordId}`,
      "get",
      `candidateRead${o}`,
      ["200"],
    ] as const,
    [
      base + `/sobjects/${o}/{recordId}`,
      "patch",
      `candidateUpdate${o}`,
      ["204"],
    ] as const,
    [
      base + `/sobjects/${o}/{recordId}`,
      "delete",
      `candidateDelete${o}`,
      ["204"],
    ] as const,
  ]),
  [
    base + "/sobjects/Account/Compat_Key__c/{externalId}",
    "patch",
    "candidateUpsertAccount",
    ["200", "201"],
  ],
  [base + "/query", "get", "candidateQuery", ["200"]],
  [base + "/openapi.yaml", "get", "candidateOpenAPI", ["200"]],
];

export async function validateCandidateOpenAPI(
  document: unknown,
  sourceUrls: ReadonlySet<string>,
): Promise<void> {
  try {
    await validateOpenApi(document);
    const d = document as any;
    if (
      d.openapi !== "3.2.1" ||
      d.jsonSchemaDialect !== "https://json-schema.org/draft/2020-12/schema" ||
      JSON.stringify(d.servers) !== JSON.stringify([{ url: "/" }]) ||
      JSON.stringify(d.security) !== JSON.stringify([{ bearerAuth: [] }]) ||
      d.components.securitySchemes.bearerAuth.type !== "http" ||
      d.components.securitySchemes.bearerAuth.scheme !== "bearer"
    )
      throw 0;
    const oauth = d.components.securitySchemes.oauth2;
    if (
      oauth?.type !== "oauth2" ||
      oauth.flows?.authorizationCode?.authorizationUrl !==
        "/services/oauth2/authorize" ||
      oauth.flows.authorizationCode.tokenUrl !== "/services/oauth2/token" ||
      oauth.flows?.clientCredentials?.tokenUrl !== "/services/oauth2/token"
    )
      throw 0;
    const actual: string[] = [];
    for (const [path, item] of Object.entries(d.paths) as [string, any][]) {
      for (const [method, op] of Object.entries(item) as [string, any][]) {
        const expected = candidateOperations.find(
          ([p, m]) => p === path && m === method,
        );
        if (
          !expected ||
          op.operationId !== expected[2] ||
          (expected[2].startsWith("candidateOAuth") &&
          expected[2] !== "candidateOAuthIdentity"
            ? JSON.stringify(op.security) !== "[]"
            : op.security !== undefined)
        )
          throw 0;
        if (op.security === undefined && !op.responses["403"]) throw 0;
        const statuses = Object.keys(op.responses);
        if (
          statuses.some(
            (s) =>
              ![
                ...expected[3],
                "400",
                "401",
                "403",
                "404",
                "405",
                "500",
              ].includes(s),
          ) ||
          expected[3].some((s) => !statuses.includes(s))
        )
          throw 0;
        if (
          !Array.isArray(op["x-source"]) ||
          !op["x-source"].length ||
          op["x-source"].some(
            (u: unknown) =>
              typeof u !== "string" ||
              !u.startsWith("https://") ||
              !sourceUrls.has(u),
          )
        )
          throw 0;
        if (op.responses["204"]?.content !== undefined) throw 0;
        actual.push(path + " " + method);
      }
    }
    if (actual.length !== candidateOperations.length) throw 0;
    function schemas(s: any): void {
      if (
        !s ||
        typeof s !== "object" ||
        Array.isArray(s) ||
        !Object.keys(s).length
      )
        throw 0;
      const types = Array.isArray(s.type) ? s.type : [s.type];
      if (
        types.includes("object") &&
        (!s.properties ||
          (!Object.keys(s.properties).length &&
            !s.patternProperties &&
            (s !== d.components.schemas.CompositeEmptyHeaders ||
              s.maxProperties !== 0)) ||
          s.additionalProperties !== false)
      )
        throw 0;
      if (s.$ref && !s.$ref.startsWith("#/components/schemas/")) throw 0;
      for (const child of Object.values(s.properties ?? {})) schemas(child);
      for (const child of Object.values(s.patternProperties ?? {}))
        schemas(child);
      if (s.items) schemas(s.items);
      for (const key of ["oneOf", "anyOf", "allOf"])
        for (const child of s[key] ?? []) schemas(child);
    }
    for (const schema of Object.values(d.components.schemas)) schemas(schema);
  } catch {
    throw new Error("candidate-openapi-contract");
  }
}

export async function loadCandidateOpenAPI(): Promise<unknown> {
  try {
    const documents = parseAllDocuments(
      await readFile(ROOT + "/api/candidate-openapi.yaml", "utf8"),
      { uniqueKeys: true, logLevel: "silent" },
    );
    if (documents.length !== 1 || documents[0].errors.length) throw 0;
    const d = documents[0].toJS({ maxAliasCount: 50 });
    const registry = parseAllDocuments(
      await readFile(ROOT + "/docs/compat/sources.yaml", "utf8"),
    )[0].toJS();
    await validateCandidateOpenAPI(
      d,
      new Set(registry.sources.map((s: any) => s.url)),
    );
    return d;
  } catch {
    throw new Error("candidate-openapi-contract");
  }
}
