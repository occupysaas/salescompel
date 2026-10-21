import { expect } from "@playwright/test";
import { test, createBrowserFixture } from "./helpers/candidate-fixtures";
test("connected/admin", async ({ page }) => {
  if (!process.env.CANDIDATE_URL || !process.env.CANDIDATE_TOKEN)
    throw new Error("candidate URL/token required");
  const name = `Browser ${Date.now()}`;
  await page.goto("/");
  await page.getByLabel("Access token").fill(process.env.CANDIDATE_TOKEN);
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await page.getByRole("button", { name: "New record" }).click();
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByLabel("Compatibility text").fill("Canonical value");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page.getByRole("button", { name, exact: true }).click();
  await page.getByLabel("Name", { exact: true }).fill(name + " edited");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(
    page.getByRole("button", { name: name + " edited", exact: true }),
  ).toBeVisible();
  await page.getByLabel("Language").selectOption("fr");
  await expect(
    page.getByRole("button", { name: "Nouvel enregistrement" }),
  ).toBeVisible();
  await page.getByLabel("Langue").selectOption("pt_BR");
  await expect(
    page.getByRole("button", { name: "Novo registro" }),
  ).toBeVisible();
  await page.getByLabel("Idioma").selectOption("iw");
  await expect(page.locator("html")).toHaveAttribute("dir", "rtl");
  await page.getByLabel("שפה").selectOption("en_US");
  await page
    .getByRole("button", { name: name + " edited", exact: true })
    .click();
  await expect(page.getByLabel("Compatibility text")).toHaveValue(
    "Canonical value",
  );
  await page.screenshot({
    path: test.info().outputPath("desktop.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 390, height: 844 });
  await expect
    .poll(() => page.evaluate(() => document.documentElement.scrollWidth))
    .toBeLessThanOrEqual(390);
  await page.screenshot({
    path: test.info().outputPath("mobile.png"),
    fullPage: true,
  });
  await page.setViewportSize({ width: 1280, height: 720 });
  const parentID = await page.getByTestId("record-id").textContent();
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await page.getByRole("button", { name: "Contacts", exact: true }).click();
  await page.getByRole("button", { name: "New record" }).click();
  await page.getByLabel("Last name").fill(name + " contact");
  await page.getByLabel("Account ID").fill(parentID!);
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page
    .getByRole("button", { name: name + " contact", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Delete record", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Confirm delete", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: name + " contact", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Accounts", exact: true }).click();
  await page
    .getByRole("button", { name: name + " edited", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Delete record", exact: true })
    .click();
  await page
    .getByRole("button", { name: "Confirm delete", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: name + " edited", exact: true }),
  ).toHaveCount(0);
  await page.getByRole("button", { name: "Disconnect", exact: true }).click();
  await expect(page.getByLabel("Access token")).toHaveValue("");
  // Only non-sensitive display preferences may persist after disconnect.
  const stored = await page.evaluate(() =>
    Object.fromEntries(Object.entries(localStorage)),
  );
  expect(Object.keys(stored).sort()).toEqual([
    "salescompel.locale",
    "salescompel.theme",
  ]);
  expect(stored["salescompel.locale"]).toBe("en_US");
  expect(JSON.stringify(stored)).not.toContain(process.env.CANDIDATE_TOKEN);
});
test("invalid session displays an error without entering records", async ({
  page,
}) => {
  await page.goto("/");
  await page.getByLabel("Access token").fill("invalid-token");
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await expect(page.getByRole("alert")).toBeVisible();
  await expect(page.getByRole("button", { name: "New record" })).toHaveCount(0);
});

for (const mutation of ["save", "delete"])
  test(`a failed refresh after ${mutation} cannot leave a stale actionable record`, async ({
    page,
    request,
  }) => {
    const name = `Refresh failure ${Date.now()}`;
    const response = await createBrowserFixture(request, "Account", {
      Name: name,
    });
    expect(response.status()).toBe(201);
    const { id } = await response.json();
    try {
      await page.goto("/");
      await page.getByLabel("Access token").fill(process.env.CANDIDATE_TOKEN!);
      await page.getByRole("button", { name: "Connect", exact: true }).click();
      await page.getByRole("button", { name, exact: true }).click();
      // Keep writes real; reproduce a network failure specifically on readback.
      await page.route("**/query?*", (route) => route.abort("failed"));
      if (mutation === "delete") {
        await page
          .getByRole("button", { name: "Delete record", exact: true })
          .click();
        await page
          .getByRole("button", { name: "Confirm delete", exact: true })
          .click();
      } else {
        await page.getByLabel("Name", { exact: true }).fill(name + " updated");
        await page.getByRole("button", { name: "Save", exact: true }).click();
      }
      await expect(page.getByRole("alert")).toBeVisible();
      await expect(page.getByRole("button", { name, exact: true })).toHaveCount(
        0,
      );
      await expect(
        page.getByRole("button", { name: "New record" }),
      ).toBeDisabled();
      await page.unroute("**/query?*");
      await page.getByRole("button", { name: "Accounts", exact: true }).click();
      await expect(
        page.getByRole("button", { name: "New record" }),
      ).toBeEnabled();
      await expect(page.getByRole("button", { name, exact: true })).toHaveCount(
        0,
      );
      if (mutation === "save")
        await expect(
          page.getByRole("button", { name: name + " updated", exact: true }),
        ).toBeVisible();
    } finally {
      await request.delete(
        `${process.env.CANDIDATE_URL}/services/data/v65.0/sobjects/Account/${id}`,
        { headers: { Authorization: `Bearer ${process.env.CANDIDATE_TOKEN}` } },
      );
    }
  });
async function connect(page: import("@playwright/test").Page, token: string) {
  await page.goto("/#/");
  await page.getByLabel("Access token").fill(token);
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await expect(page.getByRole("button", { name: "New record" })).toBeVisible();
}
test("connected/restricted", async ({ page, request }) => {
  const name = "p15-restricted-" + Date.now();
  const response = await createBrowserFixture(request, "Account", {
    Name: name,
    Compat_Text__c: "private value",
  });
  const { id } = await response.json();
  await connect(page, process.env.CANDIDATE_RESTRICTED_TOKEN!);
  await page.getByRole("button", { name, exact: true }).click();
  await expect(page.getByLabel("Compatibility text")).toHaveCount(0);
  await page.getByLabel("Name", { exact: true }).fill(name + " edited");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await expect(
    page.getByRole("button", { name: name + " edited", exact: true }),
  ).toBeVisible();
  const read = await request.get(
    `${process.env.CANDIDATE_URL}/services/data/v65.0/sobjects/Account/${id}`,
    { headers: { Authorization: `Bearer ${process.env.CANDIDATE_TOKEN}` } },
  );
  expect((await read.json()).Compat_Text__c).toBe("private value");
});
test("connected/other-tenant", async ({ page, request }) => {
  const name = "p15-other-" + Date.now();
  await createBrowserFixture(request, "Account", { Name: name });
  await connect(page, process.env.CANDIDATE_OTHER_TOKEN!);
  await expect(page.getByRole("button", { name, exact: true })).toHaveCount(0);
});
test("connected/locales", async ({ page }) => {
  await connect(page, process.env.CANDIDATE_TOKEN!);
  for (const [code, lang, dir, label] of [
    ["en_US", "en-US", "ltr", "New record"],
    ["fr", "fr", "ltr", "Nouvel enregistrement"],
    ["de", "de", "ltr", "Neuer Datensatz"],
    ["pt_BR", "pt-BR", "ltr", "Novo registro"],
    ["iw", "he", "rtl", "רשומה חדשה"],
  ]) {
    await page.locator("header select").selectOption(code);
    await expect(page.locator("html")).toHaveAttribute("lang", lang);
    await expect(page.locator("html")).toHaveAttribute("dir", dir);
    await expect(page.getByRole("button", { name: label })).toBeVisible();
  }
});
test("connected/token-boundary", async ({ page }) => {
  await connect(page, process.env.CANDIDATE_TOKEN!);
  await page.goto("/#/preview/accounts");
  await expect(
    page.getByRole("heading", { name: "Accounts", exact: true }),
  ).toBeVisible();
  await page.goto("/#/");
  await expect(page.getByLabel("Access token")).toHaveValue("");
  await expect(page.getByRole("button", { name: "New record" })).toHaveCount(0);
  await connect(page, process.env.CANDIDATE_TOKEN!);
  await page.getByRole("button", { name: "Disconnect", exact: true }).click();
  await expect(page.getByLabel("Access token")).toHaveValue("");
});
