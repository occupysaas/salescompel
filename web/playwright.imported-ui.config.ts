import { defineConfig } from '@playwright/test';
export default defineConfig({
    testDir: './test', testMatch: 'imported-ui.spec.ts', workers: 1, retries: 0, use: {
        baseURL: 'http://127.0.0.1:4181', headless: true, launchOptions: process.env.CHROMIUM_PATH ? { executablePath: process.env.CHROMIUM_PATH } : {}
    }, webServer: {
        command: 'node_modules/.bin/vite --config compat/imported-ui/vite.config.ts --host 127.0.0.1 --port 4181 --strictPort', url: 'http://127.0.0.1:4181', reuseExistingServer: false
    }
});
