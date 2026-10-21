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
  testMatch: "preview.spec.ts",
  workers: 1,
  retries: 0,
  use: {
    baseURL: "http://127.0.0.1:4173",
    headless: true,
    launchOptions: process.env.CHROMIUM_PATH
      ? { executablePath: process.env.CHROMIUM_PATH }
      : {},
  },
  webServer: {
    command:
      (acceptance ? "" : "npm run build && ") +
      "npm exec -- vite preview --host 127.0.0.1 --port 4173 --strictPort",
    url: "http://127.0.0.1:4173",
    reuseExistingServer: !acceptance && !process.env.CI,
  },
});
