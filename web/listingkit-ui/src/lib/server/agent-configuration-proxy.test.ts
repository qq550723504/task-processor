import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { proxyAgentConfiguration } from "./agent-configuration-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
const key = "8fc227bb-b572-4138-8e2a-5f1a0be98617";
function request(path: string, method = "GET", payload?: unknown) {
  return new Request("http://localhost:3000/api/workbench/agents/" + path, {
    method,
    headers: {
      "X-Expected-User-ID": "actor",
      "X-Expected-Organization-ID": "org-a",
      cookie: WORKBENCH_COOKIE_NAME + "=org-a",
      Origin: "http://localhost:3000",
      "Content-Type": "application/json",
      "Idempotency-Key": key,
      "If-Match": '"9223372036854775806"',
    },
    body: payload === undefined ? undefined : JSON.stringify(payload),
  });
}
beforeEach(() => {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://localhost:9000/api/v1");
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000");
});
afterEach(() => {
  vi.unstubAllGlobals();
  vi.unstubAllEnvs();
});
it("rejects scope confusion before forwarding", async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  const r = request("mine");
  r.headers.set("X-Expected-Organization-ID", "org-b");
  expect((await proxyAgentConfiguration(r, "token", "actor")).status).toBe(409);
  expect(fetch).not.toHaveBeenCalled();
});
it("forwards exact bigint precondition and returns a committed receipt", async () => {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://localhost:9000/api/v1");
  const fetch = vi
    .fn()
    .mockResolvedValue(
      Response.json({
        commandId: key,
        operation: "disable",
        agentId: "product.title.agent",
        beforeRevision: "9223372036854775806",
        revision: "9223372036854775807",
        noop: false,
        committedAt: "2026-09-29T00:00:00Z",
      }),
    );
  vi.stubGlobal("fetch", fetch);
  const r = await proxyAgentConfiguration(
    request("product.title.agent/disable", "POST", {}),
    "token",
    "actor",
  );
  expect(r.status).toBe(200);
  expect(new Headers(fetch.mock.calls[0][1].headers).get("If-Match")).toBe(
    '"9223372036854775806"',
  );
});
it("does not forward arbitrary settings or a cross-origin configuration write", async () => {
  const fetch = vi.fn();
  vi.stubGlobal("fetch", fetch);
  const r = request("product.title.agent/templates", "POST", {
    name: "title",
    targetPlatform: "shein",
    allowedTools: ["unsafe"],
  });
  r.headers.delete("If-Match");
  expect((await proxyAgentConfiguration(r, "token", "actor")).status).toBe(400);
  expect(fetch).not.toHaveBeenCalled();
  const other = request("product.title.agent/disable", "POST", {});
  other.headers.set("Origin", "https://other.invalid");
  expect((await proxyAgentConfiguration(other, "token", "actor")).status).toBe(
    403,
  );
  expect(fetch).not.toHaveBeenCalled();
});
