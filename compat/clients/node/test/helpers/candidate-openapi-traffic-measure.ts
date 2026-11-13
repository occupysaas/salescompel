import assert from "node:assert/strict";
import { request as httpsRequest } from "node:https";
import type { CandidateExchange } from "../../src/candidate-openapi-model.ts";
import type { FixtureEvent } from "../../src/candidate-acceptance-fixtures.ts";

export type Measured = CandidateExchange & { allow: string | null };
export type Role = FixtureEvent["principal"] | "none" | "bad";

export function json(e: Measured): Record<string, any> {
  try {
    return JSON.parse(
      new TextDecoder("utf-8", { fatal: true }).decode(e.response.body),
    );
  } catch {
    throw new Error("candidate-openapi-json");
  }
}

export function createMeasure(url: URL) {
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
  return async function measure(
    method: string,
    path: string,
    options: {
      role?: Role;
      body?: unknown;
      rawBody?: string;
      headers?: Record<string, string>;
    } = {},
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
      const headers: Record<string, string> = { ...options.headers };
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
  };
}
