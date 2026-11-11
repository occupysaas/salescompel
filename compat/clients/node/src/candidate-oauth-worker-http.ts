// Raw OAuth and REST protocol exchanges against the candidate instance:
// token endpoint, query probe, revoke, and the full authorization-code login.
import type { GateContext } from "./candidate-acceptance-preflight.ts";
import { createHash, randomBytes } from "node:crypto";
import {
  addSecret,
  ensure,
  request,
} from "./candidate-oauth-worker-runtime.ts";

const tokenPath = "/services/oauth2/token";

export async function exchange(
  c: GateContext,
  form: Record<string, string>,
): Promise<Record<string, any>> {
  const r = await request(c.url + tokenPath, "POST", new URLSearchParams(form));
  ensure(r.status === 200 && typeof r.json?.access_token === "string");
  return r.json!;
}

export async function query(c: GateContext, access: string, account: string) {
  const r = await request(
    c.url +
      "/services/data/v65.0/query?q=" +
      encodeURIComponent(`SELECT Id,Name FROM Account WHERE Id = '${account}'`),
    "GET",
    undefined,
    { Authorization: "Bearer " + access },
  );
  ensure(
    r.status === 200 &&
      r.json!.records.length === 1 &&
      r.json!.records[0].Id === account,
  );
}

export async function revoke(c: GateContext, token: string) {
  ensure(
    (
      await request(
        c.url + "/services/oauth2/revoke",
        "POST",
        new URLSearchParams({ token }),
      )
    ).status === 200,
  );
}

export async function rawLogin(
  c: GateContext,
  password: string,
  username: string,
  client: string,
  secret: string,
  callbackURI: string,
  scope = "api web refresh_token",
) {
  const verifier = randomBytes(48).toString("base64url");
  const auth = new URL(c.url + "/services/oauth2/authorize");
  auth.search = new URLSearchParams({
    client_id: client,
    redirect_uri: callbackURI,
    response_type: "code",
    scope,
    code_challenge: createHash("sha256").update(verifier).digest("base64url"),
    code_challenge_method: "S256",
  }).toString();
  const page = await request(auth.href);
  ensure(page.status === 200);
  const id = /name="request_id"\s+value="([^"]+)"/.exec(page.text)?.[1];
  ensure(id);
  const result = await request(
    c.url + "/services/oauth2/authorize",
    "POST",
    new URLSearchParams({
      request_id: id!,
      username,
      password,
      decision: "allow",
    }),
    {
      Origin: c.url,
      Cookie: page.headers.get("set-cookie")?.split(";")[0] ?? "",
    },
  );
  if (result.status !== 303) return { denied: true };
  const location = new URL(result.headers.get("location")!);
  if (!location.searchParams.get("code")) return { denied: true };
  const code = location.searchParams.get("code")!;
  addSecret(code);
  return await exchange(c, {
    grant_type: "authorization_code",
    client_id: client,
    client_secret: secret,
    redirect_uri: callbackURI,
    code,
    code_verifier: verifier,
  });
}
