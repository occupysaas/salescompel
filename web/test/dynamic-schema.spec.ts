import { expect } from "@playwright/test";
import { test } from "./helpers/candidate-fixtures";

test("connected/dynamic-schema", async ({ page }) => {
  const object = process.env.CANDIDATE_CUSTOM_OBJECT;
  if (!object) throw new Error("custom schema fixture required");
  await page.goto("/");
  await page.getByLabel("Access token").fill(process.env.CANDIDATE_TOKEN!);
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page.getByRole("button", { name: "New record" }).click();
  const name = `Schema browser ${process.env.CANDIDATE_RUN_ID}`;
  await page.getByLabel("Name", { exact: true }).fill(name);
  await page.getByLabel("Notes", { exact: true }).fill("Browser custom value");
  await expect(
    page.getByLabel("<img src=x onerror=alert(1)>", { exact: true }),
  ).toBeVisible();
  await expect(page.locator("img[src=x]")).toHaveCount(0);
  await page.getByRole("button", { name: "Save", exact: true }).click();
  await page.getByRole("button", { name, exact: true }).click();
  await expect(page.getByLabel("Notes", { exact: true })).toHaveValue(
    "Browser custom value",
  );
  await page.getByLabel("Language").selectOption("iw");
  await expect(page.locator("html")).toHaveAttribute("dir", "rtl");
  await page.getByLabel("שפה").selectOption("en_US");
  await page.getByRole("button", { name: "Cancel", exact: true }).click();
  await page
    .getByRole("button", { name: "Refresh objects", exact: true })
    .click();
  await expect(
    page.getByRole("button", { name: "Projects", exact: true }),
  ).toBeVisible();
});

test("connected/dynamic-isolation", async ({ page }) => {
  await page.goto("/");
  await page.getByLabel("Access token").fill(process.env.CANDIDATE_TOKEN!);
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await page.getByRole("button", { name: "Projects", exact: true }).click();
  await page.getByRole("button", { name: "Disconnect", exact: true }).click();
  await page
    .getByLabel("Access token")
    .fill(process.env.CANDIDATE_OTHER_TOKEN!);
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await expect(
    page.getByRole("button", { name: "Projects", exact: true }),
  ).toHaveCount(0);
  await expect(
    page.getByRole("button", { name: "Other projects", exact: true }),
  ).toBeVisible();
});

test("connected/dynamic-stale-response", async ({ page }) => {
  let release!: () => void;
  const held = new Promise<void>((resolve) => (release = resolve));
  await page.route("**/sobjects", async (route) => {
    await held;
    await route.continue().catch(() => {});
  });
  await page.goto("/");
  await page.getByLabel("Access token").fill(process.env.CANDIDATE_TOKEN!);
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await page
    .getByRole("link", { name: /Explore|Preview|prototype/i })
    .first()
    .click();
  release();
  await expect(page.getByLabel("Access token")).toHaveCount(0);
});
