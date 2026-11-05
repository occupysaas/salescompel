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
  } finally {
    await context.close();
  }
});
