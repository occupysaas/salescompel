import { test, expect } from "@playwright/test";
test.beforeEach(async ({ page }) => {
  page.on("pageerror", (error) => {
    throw error;
  });
  await page.route("**/services/**", (route) => {
    throw new Error(`Preview dispatched API request: ${route.request().url()}`);
  });
});
test("routed frame has evidence, history, safe unknown routes and no API calls", async ({
  page,
}) => {
  await page.goto("/#/preview");
  await expect(
    page.getByRole("heading", { name: "Accounts", exact: true }),
  ).toBeVisible();
  await page.getByRole("link", { name: "Contacts", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Contacts", exact: true }),
  ).toBeVisible();
  await page.goBack();
  await expect(
    page.getByRole("heading", { name: "Accounts", exact: true }),
  ).toBeVisible();
  await page.goForward();
  await page.getByRole("link", { name: "Evidence", exact: true }).click();
  await expect(
    page.getByText("/services/data/v65.0/query", { exact: true }),
  ).toBeVisible();
  await page.goto("/#/preview/accounts/no-such-id");
  await expect(
    page.getByRole("heading", { name: "Record not found" }),
  ).toBeVisible();
  await page.goto("/#/not-a-route");
  await expect(
    page.getByRole("heading", { name: "Page not found" }),
  ).toBeVisible();
});
test("synthetic edit, cancel, create, validation, reload and reset", async ({
  page,
}) => {
  await page.goto("/#/preview/accounts/sample-account-1");
  await page.getByRole("link", { name: "Edit record", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("");
  await page.getByRole("button", { name: "Save locally" }).click();
  await expect(page.getByRole("alert")).toContainText("required");
  await page.getByLabel("Name", { exact: true }).fill("Changed locally");
  await page.getByLabel("External key").fill("x".repeat(81));
  await page.getByRole("button", { name: "Save locally" }).click();
  await expect(page.getByRole("alert")).toContainText("80");
  await page.getByLabel("External key").fill("sample-updated");
  await page.getByRole("button", { name: "Save locally" }).click();
  await expect(
    page.getByRole("heading", { name: "Changed locally" }),
  ).toBeVisible();
  await expect(page.getByRole("status")).toContainText("sample");
  await page.getByRole("link", { name: "Edit record", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("Discard this");
  await page.getByRole("link", { name: "Cancel", exact: true }).click();
  await expect(
    page.getByRole("heading", { name: "Changed locally" }),
  ).toBeVisible();
  await page
    .getByRole("link", { name: "Accounts", exact: true })
    .first()
    .click();
  await page.getByRole("link", { name: "New account", exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill("New sample");
  await page.getByRole("button", { name: "Save locally" }).click();
  await expect(page.getByRole("heading", { name: "New sample" })).toBeVisible();
  await page.reload();
  await expect(
    page.getByRole("heading", { name: "Record not found" }),
  ).toBeVisible();
  await page.getByRole("button", { name: "Reset preview" }).click();
  await expect(
    page.getByRole("link", { name: "Northstar Studio", exact: true }),
  ).toBeVisible();
  await expect(page.getByRole("status")).toContainText("reset");
});
test("contacts link to accounts and related lists stay scoped", async ({
  page,
}) => {
  await page.goto("/#/preview/contacts/new");
  await page.getByRole("button", { name: "Save locally" }).click();
  await expect(page.getByRole("alert")).toContainText("required");
  await page.getByLabel("First name").fill("Jordan");
  await page.getByLabel("Last name").fill("Sample");
  await page
    .getByLabel("Account", { exact: true })
    .selectOption("sample-account-1");
  await page.getByRole("button", { name: "Save locally" }).click();
  await page
    .getByRole("link", { name: "Northstar Studio", exact: true })
    .click();
  await expect(
    page.getByRole("link", { name: "Jordan Sample", exact: true }),
  ).toBeVisible();
  await expect(
    page.getByRole("link", { name: "Morgan Chen", exact: true }),
  ).toHaveCount(0);
});
test("theme, language and RTL mobile drawer remain usable", async ({
  page,
}) => {
  await page.setViewportSize({ width: 390, height: 844 });
  await page.goto("/#/preview/accounts");
  await page.getByLabel("Theme").selectOption("dark");
  await page.getByLabel("Language").selectOption("iw");
  await expect(page.locator("html")).toHaveAttribute("dir", "rtl");
  await expect(page.locator("html")).toHaveAttribute("lang", "he");
  await page.getByRole("button", { name: "Open navigation" }).click();
  await expect(
    page.getByRole("navigation", { name: "Preview navigation" }),
  ).toBeVisible();
  await page.keyboard.press("Escape");
  await expect(
    page.getByRole("button", { name: "Open navigation" }),
  ).toBeFocused();
  await page.reload();
  await expect(page.locator("html")).toHaveAttribute("dir", "rtl");
  await expect(page.locator("html")).toHaveAttribute("data-theme", "dark");
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
});
test("blocked storage falls back and connected page remains reachable", async ({
  page,
}) => {
  await page.addInitScript(() =>
    Object.defineProperty(window, "localStorage", {
      get() {
        throw Error("denied");
      },
    }),
  );
  await page.goto("/#/preview/accounts");
  await page.getByLabel("Theme").selectOption("light");
  await page.getByLabel("Language").selectOption("fr");
  await expect(page.locator("html")).toHaveAttribute("lang", "fr");
  await page.getByRole("link", { name: "Return to connected app" }).click();
  await expect(page.getByLabel("Jeton d’accès")).toBeVisible();
});

test("long sample text, forms and evidence fit desktop and mobile in both themes", async ({
  page,
}) => {
  await page.goto("/#/preview/accounts/sample-account-1/edit");
  await page.getByLabel("Name", { exact: true }).fill("A".repeat(180));
  await page.getByRole("button", { name: "Save locally" }).click();
  for (const width of [1440, 390]) {
    await page.setViewportSize({ width, height: 900 });
    for (const theme of ["light", "dark"]) {
      await page.getByLabel("Theme").selectOption(theme);
      expect(
        await page.evaluate(() => document.documentElement.scrollWidth),
      ).toBeLessThanOrEqual(width);
    }
  }
  await page.goto("/#/preview/evidence");
  await page.getByLabel("Language").selectOption("iw");
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
  await page.goto("/#/preview/contacts/new");
  expect(
    await page.evaluate(() => document.documentElement.scrollWidth),
  ).toBeLessThanOrEqual(390);
});

test("leaving the connected route returns to a credential-free preview and requires reconnect", async ({
  page,
}) => {
  await page.unroute("**/services/**");
  const auth: string[] = [];
  await page.route("**/services/**", async (route) => {
    auth.push(route.request().headers()["authorization"] ?? "");
    await route.fulfill({
      contentType: "application/json",
      body: JSON.stringify(
        route.request().url().includes("/describe")
          ? {
              fields: [
                {
                  name: "Name",
                  type: "string",
                  nillable: false,
                  createable: true,
                  updateable: true,
                  length: 80,
                },
              ],
            }
          : { records: [] },
      ),
    });
  });
  await page.goto("/#/");
  await page
    .getByLabel("Access token")
    .fill("synthetic-token-for-browser-test");
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await expect(page.getByRole("button", { name: "New record" })).toBeVisible();
  expect(auth).toHaveLength(2);
  await page.getByRole("link", { name: "Explore the prototype" }).click();
  await expect(
    page.getByRole("heading", { name: "Accounts", exact: true }),
  ).toBeVisible();
  expect(auth).toHaveLength(2);
  await page.getByRole("link", { name: "Return to connected app" }).click();
  await expect(page.getByLabel("Access token")).toHaveValue("");
  await expect(page.getByRole("button", { name: "New record" })).toHaveCount(0);
  expect(auth).toHaveLength(2);
});
