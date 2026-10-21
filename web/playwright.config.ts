import { defineConfig } from "@playwright/test";
const acceptance = process.env.CANDIDATE_ACCEPTANCE === "1";
export default defineConfig({
  ...(acceptance
    ? {
        outputDir: process.env.CANDIDATE_BROWSER_OUTPUT,
        reporter: [
          ["json", { outputFile: process.env.CANDIDATE_BROWSER_REPORT }],
        ] as any,
      }
    : {}),
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
