import path from "node:path";
import { defineConfig, devices } from "@playwright/test";

const runtimeRoot = path.resolve(__dirname, "../../.local/playwright");

export default defineConfig({
  testDir: "./e2e",
  outputDir: path.join(runtimeRoot, "test-results"),
  reporter: [
    ["line"],
    ["html", { open: "never", outputFolder: path.join(runtimeRoot, "report") }],
  ],
  use: {
    baseURL: "http://127.0.0.1:3210",
    screenshot: "only-on-failure",
    trace: "retain-on-failure",
  },
  webServer: {
    command: "pnpm dev --hostname 127.0.0.1 --port 3210",
    reuseExistingServer: !process.env.CI,
    timeout: 120_000,
    url: "http://127.0.0.1:3210",
  },
  projects: [
    {
      name: "chromium",
      use: { ...devices["Desktop Chrome"] },
    },
  ],
});
