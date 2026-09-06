import { defineConfig } from "@playwright/test";

// Requires the separately started #323 isolated fixture; never starts against a
// shared environment and never substitutes production authentication.
export default defineConfig({
  testDir: "./integration/shein-diagnostic",
  testMatch: "*.browser.ts",
  outputDir: "../../.local/issue324-browser",
  reporter: "line",
  workers: 1,
  timeout: 60_000,
  use: { screenshot: "only-on-failure", trace: "retain-on-failure" },
});
