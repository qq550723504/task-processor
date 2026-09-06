import { defineConfig } from "@playwright/test";

export default defineConfig({ testDir: "./integration/task-center", testMatch: "*.browser.ts", outputDir: "../../.local/issue328-task-center-browser", reporter: "line", workers: 1, timeout: 60000, use: { screenshot: "only-on-failure", trace: "retain-on-failure" } });
