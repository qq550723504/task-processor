import path from "node:path";
import { defineConfig } from "vitest/config";
export default defineConfig({ test: { environment: "node", include: ["e2e/issue399-browser-chain.integration.ts"], testTimeout: 120_000, maxWorkers: 1 }, resolve: { alias: { "@": path.resolve(__dirname, "../src"), "next/server": path.resolve(__dirname, "../node_modules/next/server.js") } } });
