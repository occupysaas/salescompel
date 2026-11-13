// Process-wide origin allowlist, secret memory, and the guarded HTTP client
// shared by the OAuth worker orchestrator and its helper modules. State lives
// here so helper modules never need to reach back into the orchestrator.
import type { CandidateTrafficValidator } from "./candidate-openapi-model.ts";

const ensure = (condition: unknown): void => {
  if (!condition) throw new Error("ASSERTION");
};

let candidateOrigin = "";
let validator: CandidateTrafficValidator | undefined;
const operations = new Set<string>();
const secrets = new Set<string>();

export function setCandidateOrigin(url: string): void {
  candidateOrigin = new URL(url).origin;
}

export function setValidator(next: CandidateTrafficValidator): void {
  validator = next;
}

export function addSecret(value: string): void {
  secrets.add(value);
}

export function operationIds(): ReadonlySet<string> {
  return operations;
}

export function capturedSecrets(): string[] {
  return [...secrets];
}

export function remember(value: unknown): Record<string, any> {
  if (value && typeof value === "object")
    for (const [key, item] of Object.entries(value as Record<string, any>)) {
      if (
        [
          "access_token",
          "refresh_token",
          "client_secret",
          "accessToken",
          "refreshToken",
          "password",
          "code",
        ].includes(key) &&
        typeof item === "string"
      )
        secrets.add(item);
      else if (typeof item === "object" && item !== null) remember(item);
    }
  return value as Record<string, any>;
}

export async function request(
  url: string,
  method = "GET",
  body?: URLSearchParams,
  headers: Record<string, string> = {},
) {
  if (new URL(url).origin !== candidateOrigin) throw new Error("SOURCE");
  const response = await fetch(url, {
    method,
    redirect: "manual",
    headers: {
      ...(body ? { "Content-Type": "application/x-www-form-urlencoded" } : {}),
      ...headers,
    },
    body: body?.toString(),
  });
  const bytes = new Uint8Array(await response.arrayBuffer());
  if (validator) {
    const u = new URL(url);
    const result = validator.validate({
      method,
      path: u.pathname + u.search,
      request: {
        contentType: body ? "application/x-www-form-urlencoded" : null,
        ...(body ? { body: Buffer.from(body.toString()) } : {}),
      },
      response: {
        status: response.status,
        contentType: response.headers.get("content-type"),
        body: bytes,
      },
    });
    if (result.requestIssues.length || result.responseIssues.length) {
      console.error(
        JSON.stringify({
          operation: result.operationId,
          request: result.requestIssues,
          response: result.responseIssues,
        }),
      );
      throw new Error("ASSERTION");
    }
    if (result.operationId) operations.add(result.operationId);
  }
  const text = Buffer.from(bytes).toString();
  let json: Record<string, any> | undefined;
  try {
    json = remember(JSON.parse(text));
  } catch {}
  return { status: response.status, text, json, headers: response.headers };
}

export { ensure };
