import { defineConfig } from "@playwright/test";
export default defineConfig({
  testDir: "./test",
  testMatch: "records.spec.ts",
  workers: 1,
  retries: 0,
  use: {
    baseURL: process.env.CANDIDATE_URL,
    ignoreHTTPSErrors: true,
    headless: true,
    launchOptions: process.env.CHROMIUM_PATH
      ? { executablePath: process.env.CHROMIUM_PATH }
      : {},
  },
});
