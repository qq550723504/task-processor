import { defineConfig } from "@playwright/test";

export default defineConfig({ testDir: "./integration/console", testMatch: "**/*.browser.ts", outputDir: "../../.local/issue331-browser", reporter: "line", workers: 1, timeout: 60000, use: { browserName: "chromium", screenshot: "only-on-failure", trace: "retain-on-failure" } });
