import { expect, type Page } from "@playwright/test";
import { test } from "./helpers/candidate-fixtures";

// Fixtures come from the field-types acceptance gate: object "Typed items"
// (auto-number Name TI-{0000}) with one field per active type and a stored
// Formula "Score formula" (stored, not enforced).
async function open(page: Page) {
  await page.goto("/");
  await page.getByLabel("Access token").fill(process.env.CANDIDATE_TOKEN!);
  await page.getByRole("button", { name: "Connect", exact: true }).click();
  await page.getByRole("button", { name: "Typed items", exact: true }).click();
}

test("connected/field-types-edit", async ({ page }) => {
  if (!process.env.CANDIDATE_FIELD_TYPES_OBJECT)
    throw new Error("field-types fixture required");
  await open(page);
  await page.getByRole("button", { name: "New record" }).click();
  await page.getByLabel("Quantity", { exact: true }).fill("1e3");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  // Native validity blocks the request; the editor stays open.
  await expect(page.getByLabel("Quantity", { exact: true })).toBeVisible();
  await page
    .getByLabel("Quantity", { exact: true })
    .fill("1234567890123456.79");
  await page.getByLabel("Amount", { exact: true }).fill("1234.5");
  await page.getByLabel("Score", { exact: true }).fill("12.5");
  await page.getByLabel("Due", { exact: true }).fill("2026-11-11");
  await page.getByLabel("Starts", { exact: true }).fill("2026-03-09T04:30");
  await page.getByLabel("Done", { exact: true }).check();
  await page.getByLabel("Stage", { exact: true }).selectOption("Closed");
  const tags = page.getByRole("group", { name: "Tags" });
  await tags.getByLabel("C", { exact: true }).check();
  await tags.getByLabel("A", { exact: true }).check();
  await page.getByLabel("Contact email", { exact: true }).fill("a@b.co");
  await page.getByLabel("Site", { exact: true }).fill("javascript:alert(1)");
  await page.getByLabel("Notes", { exact: true }).fill("line one\nline two");
  await page.getByRole("button", { name: "Save", exact: true }).click();
  // Earlier gates created lower numbers; this record sorts last by Name.
  const row = page.getByRole("button", { name: /^TI-\d{4}$/ }).last();
  await expect(row).toBeVisible();
  await row.click();
  await expect(page.getByLabel("Quantity", { exact: true })).toHaveValue(
    "1234567890123456.79",
  );
  await expect(page.getByLabel("Amount", { exact: true })).toHaveValue(
    "1234.5",
  );
  await expect(page.getByLabel("Due", { exact: true })).toHaveValue(
    "2026-11-11",
  );
  await expect(page.getByLabel("Starts", { exact: true })).toHaveValue(
    "2026-03-09T04:30",
  );
  await expect(page.getByLabel("Done", { exact: true })).toBeChecked();
  await expect(page.getByLabel("Stage", { exact: true })).toHaveValue("Closed");
  await expect(tags.getByLabel("A", { exact: true })).toBeChecked();
  await expect(tags.getByLabel("B", { exact: true })).not.toBeChecked();
  await expect(page.getByLabel("Notes", { exact: true })).toHaveValue(
    "line one\nline two",
  );
  await expect(page.locator("output[data-field='Name']")).toHaveText(
    /^TI-\d{4}$/,
  );
  await expect(page.locator("a[href^='javascript:']")).toHaveCount(0);
  await expect(page.getByLabel("Parent", { exact: true })).toBeVisible();
});

test("connected/field-types-stored", async ({ page }) => {
  await open(page);
  const note = page.getByRole("note");
  await expect(note.getByText("Not active in SalesCompel")).toBeVisible();
  await expect(note.getByText("Score_Formula__c")).toBeVisible();
  await expect(page.getByLabel("Score formula", { exact: true })).toHaveCount(
    0,
  );
});

test("connected/field-types-locale", async ({ page }) => {
  await open(page);
  await page.getByRole("button", { name: "New record" }).click();
  await page.getByLabel("Amount", { exact: true }).fill("1234.5");
  await expect(page.getByTestId("number-preview").first()).toHaveText(
    /^1,234\.50\s?€$|^€1,234\.50$/,
  );
  await page.getByLabel("Language").selectOption("de");
  await expect(page.getByTestId("number-preview").first()).toHaveText(
    /^1\.234,50\s€$/,
  );
  await expect(
    page.getByText("Aus einer Definition gespeichert"),
  ).toBeVisible();
  await page.getByLabel("Sprache").selectOption("iw");
  await expect(page.locator("html")).toHaveAttribute("dir", "rtl");
});
