import { createRequire } from "node:module";
import { createCandidateTrafficValidator } from "../../compat/clients/node/src/candidate-openapi-traffic.ts";
const { parse } = createRequire(
  new URL("../../compat/clients/node/package.json", import.meta.url),
)("yaml");
import { test, expect } from "@playwright/test";

// The harness passes transient credentials in the child environment. Never attach
// traces, screenshots or page URLs to reports from an OAuth exchange.
test("OAuth login page completes authorization in Chromium", async ({
  browser,
}) => {
  const authorize = process.env.CANDIDATE_OAUTH_AUTHORIZE_URL;
  const username = process.env.CANDIDATE_OAUTH_USERNAME;
  const password = process.env.CANDIDATE_OAUTH_PASSWORD;
  if (!authorize || !username || !password)
    throw new Error("OAuth browser inputs missing");
  const candidate = new URL(authorize).origin;
  const context = await browser.newContext({
    ignoreHTTPSErrors: true,
    locale: "en-US",
  });
  const page = await context.newPage();
  const contract = await context.request.get(
    candidate + "/services/data/v65.0/openapi.yaml",
    { headers: { Authorization: "Bearer " + process.env.CANDIDATE_TOKEN } },
  );
  if (contract.status() !== 200) throw new Error("OAuth contract unavailable");
  const validator = await createCandidateTrafficValidator(
    parse(await contract.text()),
  );
  const measured: Promise<void>[] = [];
  const operations = new Set<string>();
  const measure = (p: typeof page) =>
    p.on("response", (response) => {
      const url = new URL(response.url());
      if (url.pathname !== "/services/oauth2/authorize") return;
      measured.push(
        (async () => {
          const request = response.request();
          const result = validator.validate({
            method: request.method(),
            path: url.pathname + url.search,
            request: {
              contentType: request.headers()["content-type"] ?? null,
              body: request.postDataBuffer() ?? undefined,
            },
            response: {
              status: response.status(),
              contentType: response.headers()["content-type"] ?? null,
              body:
                response.status() === 303
                  ? Buffer.alloc(0)
                  : await response.body(),
            },
          });
          if (result.requestIssues.length || result.responseIssues.length)
            throw new Error("OAuth browser traffic violates served contract");
          if (result.operationId) operations.add(result.operationId);
        })(),
      );
    });
  measure(page);
  try {
    await page.goto(authorize);
    await page.getByLabel("Username", { exact: true }).fill(username);
    await page
      .getByLabel("Password", { exact: true })
      .fill("deliberately-wrong-password");
    await page.getByRole("button", { name: "Allow", exact: true }).click();
    await expect(page.getByRole("alert")).toBeVisible();
    // Locale proof creates its own pending request without completing it.
    const rtl = await browser.newContext({
      ignoreHTTPSErrors: true,
      locale: "he",
    });
    try {
      const hebrew = await rtl.newPage();
      measure(hebrew);
      await hebrew.goto(authorize);
      await expect(hebrew.locator("html")).toHaveAttribute("dir", "rtl");
    } finally {
      await rtl.close();
    }
    await page.getByLabel("Username", { exact: true }).fill(username);
    await page.getByLabel("Password", { exact: true }).fill(password);
    await page.getByRole("button", { name: "Allow", exact: true }).click();
    await expect
      .poll(() => new URL(page.url()).origin === candidate)
      .toBe(false);
    await page.waitForLoadState("load");
    await Promise.all(measured);
    if (
      !operations.has("candidateOAuthAuthorizeForm") ||
      !operations.has("candidateOAuthAuthorizeSubmit")
    )
      throw new Error("OAuth browser traffic incomplete");
  } finally {
    await context.close();
  }
});
