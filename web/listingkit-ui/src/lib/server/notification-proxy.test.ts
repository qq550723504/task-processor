import { afterEach, expect, it, vi } from "vitest";
import { proxyNotifications } from "./notification-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";
const key = "4841d296-ef14-4c16-8d25-a7667e534feb";
const headers = { "X-Expected-User-ID": "actor", "X-Expected-Organization-ID": "org", cookie: WORKBENCH_COOKIE_NAME + "=org", Origin: "http://localhost:3000" };
afterEach(() => { vi.unstubAllEnvs(); vi.unstubAllGlobals(); });
function configure() { vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1"); vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://localhost:3000"); }
it("rejects stale identities, duplicate query/cookies and cross-site receipt writes before dispatch", async () => {
  configure(); const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  for (const [path, init, status] of [
    ["official?filter=unread&filter=all", {}, 400],
    ["official?org=other", {}, 400],
    ["official", { headers: { ...headers, "X-Expected-User-ID": "other" } }, 409],
    ["business", { headers: { ...headers, cookie: headers.cookie + "; " + headers.cookie } }, 409],
    ["business", { headers: { ...headers, cookie: WORKBENCH_COOKIE_NAME + "=other" } }, 409],
    ["official/read", { method: "POST", headers: { ...headers, Origin: "http://foreign.test" }, body: "{}" }, 403],
  ] as const) expect((await proxyNotifications(new Request("http://localhost:3000/api/notifications/" + path, { headers, ...init }), "server-token", "actor")).status).toBe(status);
  expect(fetch).not.toHaveBeenCalled();
});
it("official/personal are independent of organization; upstream credentials remain server-only", async () => {
  configure(); const fetch = vi.fn().mockImplementation(() => Promise.resolve(Response.json({ schemaVersion: "notification-center-v1", items: [], coverage: [], exact: true, count: 0, unread: 0, pending: 0, next: "" }))); vi.stubGlobal("fetch", fetch);
  for (const channel of ["official", "personal"]) {
    const response = await proxyNotifications(new Request("http://localhost:3000/api/notifications/" + channel, { headers: { "X-Expected-User-ID": "actor", Authorization: "Bearer browser-token" } }), "server-token", "actor");
    expect(response.status).toBe(200);
    const forwarded = new Headers(fetch.mock.calls.at(-1)![1].headers);
    expect(forwarded.get("Authorization")).toBe("Bearer server-token");
    expect(forwarded.has("X-Requested-Organization-ID")).toBe(false);
    expect(forwarded.has("cookie")).toBe(false);
  }
});
it("rejects ambiguous JSON, and retains one key on an uncertain receipt commit", async () => {
  configure(); const fetch = vi.fn().mockRejectedValue(new Error("lost response")); vi.stubGlobal("fetch", fetch);
  for (const body of ['{"id":"a","id":"b"}', '{"id":"a","subject":"other"}']) {
    expect((await proxyNotifications(new Request("http://localhost:3000/api/notifications/official/read", { method: "POST", headers: { ...headers, "Content-Type": "application/json", "Idempotency-Key": key }, body }), "server-token", "actor")).status).toBe(400);
  }
  expect(fetch).not.toHaveBeenCalled();
  const response = await proxyNotifications(new Request("http://localhost:3000/api/notifications/official/snapshot", { method: "POST", headers: { ...headers, "Content-Type": "application/json", "Idempotency-Key": key }, body: "{}" }), "server-token", "actor");
  expect((await response.json()).code).toBe("OUTCOME_UNKNOWN");
  expect(fetch).toHaveBeenCalledTimes(1);
  expect(new Headers(fetch.mock.calls[0][1].headers).get("Idempotency-Key")).toBe(key);
});
