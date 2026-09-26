import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { proxyAccount } from "./account-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

const limit = { organizationId: "org-1", memberId: "member-1", configured: true, monthlyLimit: "25", reserved: "0", consumed: "0", remaining: "25", version: "1", monthStart: "2026-09-01T00:00:00Z", monthEnd: "2026-10-01T00:00:00Z" };
const snapshot = { schemaVersion: "member-ai-point-monthly-limit-v1", organizationId: "org-1", resourceType: "ai_point", timezone: "UTC", members: [{ ...limit, displayName: "Member", loginName: "member@example.test", roles: ["listingkit_operator"] }] };
function request(method = "GET") { return new Request(`http://localhost/api/account/member-ai-point-limits${method === "PUT" ? "/member-1" : ""}`, { method, headers: { cookie: `${WORKBENCH_COOKIE_NAME}=org-1`, "X-Expected-User-ID": "actor-1", "X-Expected-Organization-ID": "org-1", "Content-Type": "application/json", "Idempotency-Key": "original-key" }, ...(method === "PUT" ? { body: JSON.stringify({ target: "25", expectedVersion: "0" }) } : {}) }); }
beforeEach(() => vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://backend.example/api/v1"));
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
it("routes the new member point read/write to the existing commercial owner boundary", async () => {
  const fetch = vi.fn().mockResolvedValueOnce(Response.json(snapshot)).mockResolvedValueOnce(Response.json(limit)); vi.stubGlobal("fetch", fetch);
  expect((await proxyAccount(request(), "test-token", "actor-1", "member-ai-point-limits")).status).toBe(200);
  expect((await proxyAccount(request("PUT"), "test-token", "actor-1", "member-ai-point-limits")).status).toBe(200);
  expect(fetch.mock.calls[0][0]).toBe("http://backend.example/api/v1/account/organization/resources/member-ai-point-limits");
  const headers = fetch.mock.calls[1][1].headers as Headers;
  expect(headers.get("X-Requested-Organization-ID")).toBe("org-1"); expect(headers.get("Idempotency-Key")).toBe("original-key");
  expect(fetch.mock.calls[1][1].redirect).toBe("manual");
});
it.each(["missing-user", "wrong-organization"])("rejects stale or missing identity context before forwarding: %s", async mode => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch); const req = request();
  if (mode === "missing-user") req.headers.delete("X-Expected-User-ID"); else req.headers.set("X-Expected-Organization-ID", "org-2");
  expect((await proxyAccount(req, "test-token", "actor-1", "member-ai-point-limits")).status).toBe(409); expect(fetch).not.toHaveBeenCalled();
});
it("marks a lost forwarded write UNKNOWN without a second upstream request", async () => {
  const fetch = vi.fn().mockRejectedValue(new Error("socket lost")); vi.stubGlobal("fetch", fetch);
  const response = await proxyAccount(request("PUT"), "test-token", "actor-1", "member-ai-point-limits");
  expect(response.status).toBe(502); expect(await response.json()).toMatchObject({ code: "RESULT_UNVERIFIED", outcome: "unknown" }); expect(fetch).toHaveBeenCalledTimes(1);
});
it.each([[403, "FORBIDDEN"], [409, "CONSUMED_FLOOR"], [401, "AUTHENTICATION_REQUIRED"]])("preserves definite owner rejection %s/%s", async (status, code) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code, message: "safe rejection", requestId: "", fieldErrors: [] }, { status: Number(status) })));
  const result = await proxyAccount(request("PUT"), "test-token", "actor-1", "member-ai-point-limits");
  expect(result.status).toBe(status); expect(await result.json()).toMatchObject({ code });
});
