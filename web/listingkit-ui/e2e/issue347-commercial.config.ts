import {defineConfig} from "vitest/config";
import path from "node:path";
export default defineConfig({test:{environment:"node",include:["e2e/issue347-commercial-chain.integration.ts"],testTimeout:30000,maxWorkers:1},resolve:{alias:{"@":path.resolve(__dirname,"../src"),"next/server":path.resolve(__dirname,"../node_modules/next/server.js")}}});
