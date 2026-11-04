import { Ajv2020 } from "ajv/dist/2020.js";
import type { ValidateFunction } from "ajv";
import { validateOpenApi } from "./openapi.ts";
import type {
  CandidateExchange,
  CandidateTrafficValidator,
  CandidateTrafficResult,
  CandidateTrafficIssueCode,
} from "./candidate-openapi-model.ts";

type Parameter = {
  name: string;
  in: "path" | "query";
  required: boolean;
  check: ValidateFunction;
};
type Media = { name: string; check: ValidateFunction };
type Operation = {
  id: string;
  parameters: Parameter[];
  request?: Media;
  requestRequired: boolean;
  responses: Map<number, Media | undefined>;
};
type Route = {
  segments: string[];
  operations: Map<string, Operation>;
  literalCount: number;
};
const issueOrder: CandidateTrafficIssueCode[] = [
  "route",
  "method",
  "parameters",
  "request-media",
  "request-json",
  "request-schema",
  "status",
  "response-media",
  "response-json",
  "response-schema",
  "response-body",
  "budget",
];
const mediaToken = "[!#$%&'*+.^_`|~0-9A-Za-z-]+";
const mediaQuoted =
  '"(?:[\\t\\x20\\x21\\x23-\\x5b\\x5d-\\x7e\\x80-\\xff]|\\\\[\\t\\x20-\\x7e\\x80-\\xff])*"';
const mediaSyntax = new RegExp(
  "^[ \\t]*(" +
    mediaToken +
    "/" +
    mediaToken +
    ")[ \\t]*(?:;[ \\t]*" +
    mediaToken +
    "=(?:" +
    mediaToken +
    "|" +
    mediaQuoted +
    ")[ \\t]*)*(?![\\s\\S])",
);
const mediaType = (value: string | null): string | undefined =>
  value === null ? undefined : mediaSyntax.exec(value)?.[1].toLowerCase();
const fail = (): never => {
  throw new Error("candidate-openapi-spec");
};

/** Compile only the controlled contract's supported subset; never fetch refs. */
export async function createCandidateTrafficValidator(
  document: unknown,
): Promise<CandidateTrafficValidator> {
  try {
    await validateOpenApi(document);
    const d = structuredClone(document) as any;
    const lookup = (ref: string): any => {
      if (
        !/^#\/components\/(schemas|responses|requestBodies|parameters)\/[A-Za-z][A-Za-z0-9_]*$/.test(
          ref,
        )
      )
        return fail();
      let value = d;
      for (const key of ref.slice(2).split("/")) {
        if (!value || !Object.hasOwn(value, key)) return fail();
        value = value[key];
      }
      return value;
    };
    const resolve = (value: any): any => {
      const visited = new Set<string>();
      while (value?.$ref) {
        if (Object.keys(value).length !== 1 || visited.has(value.$ref))
          return fail();
        visited.add(value.$ref);
        value = lookup(value.$ref);
      }
      return value;
    };
    // Ref cycles and unsupported schema annotations cannot hide behind lazy use.
    function schemaWalk(s: any, active: Set<string>): void {
      if (!s || typeof s !== "object" || Array.isArray(s)) return fail();
      const allowed = new Set([
        "$ref",
        "type",
        "const",
        "enum",
        "properties",
        "required",
        "additionalProperties",
        "items",
        "oneOf",
        "anyOf",
        "allOf",
        "minItems",
        "maxItems",
        "maxProperties",
        "minLength",
        "maxLength",
        "minimum",
        "maximum",
        "pattern",
        "description",
        "title",
      ]);
      for (const key of Object.keys(s)) if (!allowed.has(key)) return fail();
      if (
        Object.hasOwn(s, "additionalProperties") &&
        s.additionalProperties !== false
      )
        return fail();
      if (s.$ref) {
        if (active.has(s.$ref) || !s.$ref.startsWith("#/components/schemas/"))
          return fail();
        const next = new Set(active);
        next.add(s.$ref);
        schemaWalk(lookup(s.$ref), next);
      }
      for (const child of Object.values(s.properties ?? {}))
        schemaWalk(child, active);
      if (s.items) schemaWalk(s.items, active);
      for (const key of ["oneOf", "anyOf", "allOf"])
        for (const child of s[key] ?? []) schemaWalk(child, active);
    }
    for (const schema of Object.values(d.components?.schemas ?? {}))
      schemaWalk(schema, new Set());
    function rewrite(value: any): any {
      if (Array.isArray(value)) return value.map(rewrite);
      if (!value || typeof value !== "object") return value;
      return Object.fromEntries(
        Object.entries(value).map(([key, v]) => [
          key,
          key === "$ref"
            ? String(v).replace(
                "#/components/schemas/",
                "urn:salescompel:candidate#/$defs/",
              )
            : rewrite(v),
        ]),
      );
    }
    const ajv = new Ajv2020({
      strict: true,
      allErrors: false,
      coerceTypes: false,
      useDefaults: false,
    });
    ajv.addSchema({
      $id: "urn:salescompel:candidate",
      $defs: rewrite(d.components?.schemas ?? {}),
    });
    const cache = new Map<string, ValidateFunction>();
    const compile = (schema: any): ValidateFunction => {
      schemaWalk(schema, new Set());
      const key = JSON.stringify(schema);
      if (!cache.has(key)) cache.set(key, ajv.compile(rewrite(schema)));
      return cache.get(key)!;
    };
    const content = (c: any, request = false): Media | undefined => {
      if (c === undefined) return undefined;
      const names = Object.keys(c);
      if (
        names.length !== 1 ||
        ![
          "application/json",
          ...(request ? [] : ["application/yaml"]),
        ].includes(names[0])
      )
        return fail();
      if (Object.keys(c[names[0]]).some((k) => k !== "schema")) return fail();
      return { name: names[0], check: compile(c[names[0]].schema) };
    };
    const routes: Route[] = [];
    for (const [path, item] of Object.entries(d.paths ?? {}) as [
      string,
      any,
    ][]) {
      if (!path.startsWith("/") || path.includes("?") || path.includes("#"))
        return fail();
      const segments = path.slice(1).split("/");
      if (
        segments.some(
          (s) =>
            !s || (/[{}]/.test(s) && !/^\{[A-Za-z][A-Za-z0-9]*\}$/.test(s)),
        )
      )
        return fail();
      const route: Route = {
        segments,
        operations: new Map(),
        literalCount: segments.filter((s) => !s.startsWith("{")).length,
      };
      for (const [method, raw] of Object.entries(item) as [string, any][]) {
        if (!["get", "post", "patch", "delete"].includes(method)) return fail();
        if (
          Object.keys(raw).some(
            (k) =>
              ![
                "operationId",
                "x-source",
                "summary",
                "description",
                "parameters",
                "requestBody",
                "responses",
              ].includes(k),
          )
        )
          return fail();
        const parameters: Parameter[] = [];
        for (const p0 of raw.parameters ?? []) {
          const p = resolve(p0);
          if (
            Object.keys(p).some(
              (k) =>
                ![
                  "name",
                  "in",
                  "required",
                  "style",
                  "explode",
                  "schema",
                  "description",
                ].includes(k),
            ) ||
            !["path", "query"].includes(p.in) ||
            p.schema?.type !== "string"
          )
            return fail();
          const style = p.style ?? (p.in === "path" ? "simple" : "form");
          const explode = p.explode ?? p.in === "query";
          if (
            style !== (p.in === "path" ? "simple" : "form") ||
            explode !== (p.in === "query")
          )
            return fail();
          parameters.push({
            name: p.name,
            in: p.in,
            required: p.required === true,
            check: compile(p.schema),
          });
        }
        const pathNames = segments
          .filter((s) => s.startsWith("{"))
          .map((s) => s.slice(1, -1))
          .sort();
        if (
          JSON.stringify(
            parameters
              .filter((p) => p.in === "path")
              .map((p) => p.name)
              .sort(),
          ) !== JSON.stringify(pathNames)
        )
          return fail();
        let request: Media | undefined;
        let requestRequired = false;
        if (raw.requestBody) {
          const rb = resolve(raw.requestBody);
          if (
            Object.keys(rb).some(
              (k) => !["required", "description", "content"].includes(k),
            )
          )
            return fail();
          request = content(rb.content, true);
          if (!request) return fail();
          requestRequired = rb.required === true;
        }
        const responses = new Map<number, Media | undefined>();
        for (const [status, r0] of Object.entries(raw.responses)) {
          if (!/^[1-5][0-9]{2}$/.test(status)) return fail();
          const r = resolve(r0);
          if (
            Object.keys(r).some(
              (k) => !["description", "content", "headers"].includes(k),
            )
          )
            return fail();
          if (r.headers) {
            if (Object.keys(r.headers).some((k) => k !== "Allow"))
              return fail();
            compile(r.headers.Allow.schema);
          }
          const m = content(r.content);
          if (status === "204" && m) return fail();
          responses.set(Number(status), m);
        }
        route.operations.set(method.toUpperCase(), {
          id: raw.operationId,
          parameters,
          request,
          requestRequired,
          responses,
        });
      }
      routes.push(route);
    }
    routes.sort((a, b) => b.literalCount - a.literalCount);
    return {
      validate(exchange: CandidateExchange): CandidateTrafficResult {
        const result: CandidateTrafficResult = {
          requestIssues: [],
          responseIssues: [],
        };
        const req = new Set<CandidateTrafficIssueCode>(),
          res = new Set<CandidateTrafficIssueCode>();
        const finish = () => {
          result.requestIssues = issueOrder.filter((c) => req.has(c));
          result.responseIssues = issueOrder.filter((c) => res.has(c));
          return result;
        };
        const path = exchange.path;
        if (
          !path.startsWith("/") ||
          path.startsWith("//") ||
          path.includes("\\") ||
          path.includes("#")
        ) {
          req.add("route");
          return finish();
        }
        const question = path.indexOf("?");
        const pathname = question < 0 ? path : path.slice(0, question);
        const query = question < 0 ? "" : path.slice(question + 1);
        const segments = pathname.replace(/\/$/, "").slice(1).split("/");
        const route = routes.find(
          (r) =>
            r.segments.length === segments.length &&
            r.segments.every((s, i) => s.startsWith("{") || s === segments[i]),
        );
        if (!route) {
          req.add("route");
          return finish();
        }
        const operation = route.operations.get(exchange.method);
        if (!operation) {
          req.add("method");
          return finish();
        }
        result.operationId = operation.id;
        const values = new Map<string, string>();
        try {
          route.segments.forEach((s, i) => {
            if (s.startsWith("{")) {
              const value = decodeURIComponent(segments[i]);
              if (value.includes("\0")) throw 0;
              values.set(s.slice(1, -1), value);
            }
          });
          // URLSearchParams is forgiving of malformed escapes; reject those first.
          for (const piece of query.split(/[&=]/))
            decodeURIComponent(piece.replaceAll("+", " "));
          const qs = new URLSearchParams(query);
          for (const p of operation.parameters) {
            const all =
              p.in === "query"
                ? qs.getAll(p.name)
                : values.has(p.name)
                  ? [values.get(p.name)!]
                  : [];
            if (
              (p.required && !all.length) ||
              (p.name === "columns" && all.length > 1) ||
              (all.length && !p.check(all[0]))
            )
              req.add("parameters");
          }
        } catch {
          req.add("parameters");
        }
        const requestBody = exchange.request.body;
        if ((requestBody?.byteLength ?? 0) > 1048576) req.add("budget");
        else if (!requestBody?.byteLength) {
          if (operation.requestRequired) req.add("request-schema");
        } else if (!operation.request) req.add("request-schema");
        else if (
          mediaType(exchange.request.contentType) !== operation.request.name
        )
          req.add("request-media");
        else {
          try {
            const value = JSON.parse(
              new TextDecoder("utf-8", { fatal: true }).decode(requestBody),
            );
            if (!operation.request.check(value)) req.add("request-schema");
          } catch {
            req.add("request-json");
          }
        }
        if (exchange.response.body.byteLength > 8388608) {
          res.add("budget");
          return finish();
        }
        if (!operation.responses.has(exchange.response.status)) {
          res.add("status");
          return finish();
        }
        const response = operation.responses.get(exchange.response.status);
        if (!response) {
          if (exchange.response.body.byteLength) res.add("response-body");
          return finish();
        }
        if (mediaType(exchange.response.contentType) !== response.name) {
          res.add("response-media");
          return finish();
        }
        try {
          const text = new TextDecoder("utf-8", { fatal: true }).decode(
            exchange.response.body,
          );
          const value =
            response.name === "application/json" ? JSON.parse(text) : text;
          if (!response.check(value)) res.add("response-schema");
        } catch {
          res.add("response-json");
        }
        return finish();
      },
    };
  } catch {
    return fail();
  }
}
