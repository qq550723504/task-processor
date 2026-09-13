import { afterEach, expect, it, vi } from "vitest";
import { proxyMembers } from "./members-proxy";
import { WORKBENCH_COOKIE_NAME } from "./workbench-proxy";

afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.useRealTimers(); });
const headers = { "X-Expected-User-ID": "actor", "X-Expected-Organization-ID": "org", cookie: `${WORKBENCH_COOKIE_NAME}=org`, Origin: "http://localhost:3000" };
it("uses the configured public origin across Next URL normalization", async () => {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1");
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL", "http://127.0.0.1:3000");
  const fetch = vi.fn().mockResolvedValue(Response.json({code:"MEMBER_NOT_FOUND",message:"",requestId:"",fieldErrors:[]},{status:404}));
  vi.stubGlobal("fetch",fetch);
  const response = await proxyMembers(new Request("http://localhost:3000/api/account/member-operations/4841d296-ef14-4c16-8d25-a7667e534feb/verify",{method:"POST",headers:{...headers,Origin:"http://127.0.0.1:3000"}}),"server-secret","actor");
  expect(response.status).toBe(404); expect(fetch).toHaveBeenCalledTimes(1);
});
it("blocks cross-site mutation before upstream access", async () => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  const response = await proxyMembers(new Request("http://localhost:3000/api/account/members/invitations", { method: "POST", headers: { ...headers, Origin: "http://foreign.test" } }), "server-secret", "actor");
  expect(response.status).toBe(403); expect(fetch).not.toHaveBeenCalled();
});
it("reports a request-body deadline without sending a mutation", async () => {
  vi.useFakeTimers(); vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL","http://localhost:3000"); vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://127.0.0.1:9000/api/v1");
  const fetch = vi.fn(); vi.stubGlobal("fetch",fetch);
  const request = new Request("http://localhost:3000/api/account/members/invitations",{method:"POST",headers:{...headers,"Content-Type":"application/json","Idempotency-Key":"4841d296-ef14-4c16-8d25-a7667e534feb"},body:new ReadableStream({start(){}}),duplex:"half"} as RequestInit);
  const response = proxyMembers(request,"server-secret","actor"); await vi.advanceTimersByTimeAsync(15001);
  expect((await response).status).toBe(504); expect(fetch).not.toHaveBeenCalled();
});
it.each(["", "{}"])("checks actual verify-body bytes for %j", async body => {
  vi.stubEnv("LISTINGKIT_PUBLIC_BASE_URL","http://localhost:3000"); vi.stubEnv("LISTINGKIT_SERVICE_API_BASE","http://127.0.0.1:9000/api/v1");
  const fetch = vi.fn().mockResolvedValue(Response.json({code:"MEMBER_NOT_FOUND",message:"",requestId:"",fieldErrors:[]},{status:404})); vi.stubGlobal("fetch",fetch);
  const response = await proxyMembers(new Request("http://localhost:3000/api/account/member-operations/4841d296-ef14-4c16-8d25-a7667e534feb/verify",{method:"POST",headers:{...headers,"content-length":"0"},body}),"server-secret","actor");
  expect(response.status).toBe(body ? 400 : 404); expect(fetch).toHaveBeenCalledTimes(body ? 0 : 1);
});
it("takes the selected org from the matching single cookie and only forwards the server token", async () => {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:9000/api/v1");
  const fetch = vi.fn().mockResolvedValue(new Response(JSON.stringify({ schemaVersion: "membership-v1", userId: "actor", organizationId: "org", items: [], total: 0, canManage: false, assignableRoles: [] }), { headers: { "Content-Type": "application/json" } }));
  vi.stubGlobal("fetch", fetch);
  const response = await proxyMembers(new Request("http://localhost:3000/api/account/members?limit=20&offset=0", { headers: { ...headers, Authorization: "Bearer browser-secret", "X-Requested-Organization-ID": "foreign" } }), "server-secret", "actor");
  expect(response.status).toBe(200); expect(fetch).toHaveBeenCalledTimes(1);
  const forwarded = new Headers(fetch.mock.calls[0][1].headers);
  expect(forwarded.get("Authorization")).toBe("Bearer server-secret");
  expect(forwarded.get("X-Requested-Organization-ID")).toBe("org");
  expect(forwarded.has("cookie")).toBe(false);
});
it("rejects stale scope and duplicate cookies without calling the provider", async () => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  for (const cookie of [`${WORKBENCH_COOKIE_NAME}=foreign`, `${WORKBENCH_COOKIE_NAME}=org; ${WORKBENCH_COOKIE_NAME}=org`]) {
    const response = await proxyMembers(new Request("http://localhost:3000/api/account/members", { headers: { ...headers, cookie } }), "server-secret", "actor");
    expect(response.status).toBe(409);
  }
  expect(fetch).not.toHaveBeenCalled();
});
