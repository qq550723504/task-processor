import { configDefaults, defineConfig } from "vitest/config";
import path from "node:path";

export default defineConfig({
  test: {
    environment: "jsdom",
    globals: true,
    setupFiles: ["./src/test/setup.ts"],
    css: true,
    maxWorkers: 4,
    testTimeout: 15000,
    exclude: [
      ...configDefaults.exclude,
      ".next/**",
      "coverage/**",
      "test-results/**",
      ".playwright-cli/**",
      ".data/**",
    ],
  },
  resolve: {
    alias: {
      "@": path.resolve(__dirname, "./src"),
      "next/server": path.resolve(__dirname, "./node_modules/next/server.js"),
    },
  },
});
