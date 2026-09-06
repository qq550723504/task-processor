import { defineConfig } from "@playwright/test";

// Explicit isolated fixture only. No production defaults or runtime fallback.
export default defineConfig({
  testDir: "./integration/shein-records",
  testMatch: "*.browser.ts",
  outputDir: "../../.local/issue328-browser",
  reporter: "line",
  workers: 1,
  timeout: 60_000,
  use: { screenshot: "only-on-failure", trace: "retain-on-failure" },
});
