import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
const state = vi.hoisted(() => ({ user: "u1", token: "fixture-token", blocked: false }));
vi.mock("@/auth", () => ({ serverAuth: (handler: (r: NextRequest) => Promise<Response>) => (request: NextRequest) => state.blocked ? new Promise(() => {}) : handler(Object.assign(request, { auth: { accessToken: state.token, identityVersion: 3, identity: { userId: state.user, tenantId: "A" } } })) }));
import { GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS } from "@/app/api/account/profile/route";
import { GET as organizationGET } from "@/app/api/account/organization/route";

const profile = { schemaVersion: "account-v1", userId: "u1", homeOrganizationId: "A", displayName: "Alice", email: null, emailVerified: null, phoneNumber: null, phoneNumberVerified: null, source: "zitadel_userinfo", readAt: "2026-09-07T01:00:00Z" };
const organization = { schemaVersion: "account-v1", userId: "u1", homeOrganizationId: "A", effectiveOrganizationId: "B", name: "B", roles: ["listingkit_viewer"], source: "zitadel_project_authorizations", readAt: "2026-09-07T01:00:00Z", authorizationMaxAgeSeconds: 60 };
function request(kind = "profile", headers: Record<string, string> = {}, signal?: AbortSignal) { return new NextRequest(`http://localhost/api/account/${kind}`, { headers: { "X-Expected-User-ID": "u1", ...headers }, signal }); }
beforeEach(() => { state.user = "u1"; state.token = "fixture-token"; state.blocked = false; vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:8085/api/v1"); });
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.useRealTimers(); });
describe("exported account routes", () => {
 it("uses only server credentials and ignores every organization input for self", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(profile)); vi.stubGlobal("fetch", fetch);
  const response = await GET(request("profile", { cookie: "shuomi_effective_organization=bad; secret=private", Authorization: "Bearer attacker", "X-Requested-Organization-ID": "bad", "X-Expected-Organization-ID": "bad" }));
  expect(response.status).toBe(200);expect(response.headers.get("cache-control")).toContain("no-store");expect(response.headers.get("set-cookie")).toBeNull();
  const [url, init] = fetch.mock.calls[0]; expect(url).toBe("http://127.0.0.1:8085/api/v1/account/profile");
  expect(Object.fromEntries(init.headers)).toEqual({ accept: "application/json", authorization: "Bearer fixture-token" });expect(init.redirect).toBe("manual");
 });
 it("forwards only the selected cookie after both assertions match", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(organization)); vi.stubGlobal("fetch", fetch);
  const response = await organizationGET(request("organization", { cookie: "shuomi_effective_organization=B", "X-Expected-Organization-ID": "B", "X-Requested-Organization-ID": "attacker" }));
  expect(response.status).toBe(200);expect(fetch.mock.calls[0][1].headers.get("X-Requested-Organization-ID")).toBe("B");
 });
 it.each([
  [{}, "ORGANIZATION_SELECTION_REQUIRED"],
  [{ cookie: "shuomi_effective_organization=B; shuomi_effective_organization=C", "X-Expected-Organization-ID": "B" }, "ORGANIZATION_CONTEXT_CHANGED"],
  [{ cookie: "shuomi_effective_organization=C", "X-Expected-Organization-ID": "B" }, "ORGANIZATION_CONTEXT_CHANGED"],
 ])("rejects missing/mismatched selection before sending", async (headers, code) => {
  const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
  const response = await organizationGET(request("organization", headers as Record<string,string>));expect((await response.json()).code).toBe(code);expect(fetch).not.toHaveBeenCalled();
 });
 it("compares expected identity with server session before sending", async () => {
  vi.stubGlobal("fetch", vi.fn());state.user = "u2";
  expect((await (await GET(request())).json()).code).toBe("IDENTITY_CONTEXT_CHANGED");expect(fetch).not.toHaveBeenCalled();
 });
 it("compares upstream identity with server session", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...profile, userId: "u2" })));
  expect((await (await GET(request())).json()).code).toBe("IDENTITY_CONTEXT_CHANGED");
 });
 it("never clears a newer selection cookie on denied late read", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code: "ORGANIZATION_ACCESS_REVOKED", message: "secret", requestId: "private", fieldErrors: [] }, { status: 403 })));
  const response = await organizationGET(request("organization", { cookie: "shuomi_effective_organization=B", "X-Expected-Organization-ID": "B" }));
  expect(response.status).toBe(403);expect(response.headers.get("set-cookie")).toBeNull();expect(await response.text()).not.toContain("secret");
 });
 it("bounds exported serverAuth wait", async () => {
  vi.useFakeTimers(); state.blocked = true;
  const pending = GET(request()); await vi.advanceTimersByTimeAsync(15001);
  expect((await pending).status).toBe(504);
 });
 it("bounds response body waiting", async () => {
  vi.useFakeTimers();vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(new ReadableStream({ start() {} }), { headers: { "Content-Type": "application/json" } })));
  const pending = GET(request());await vi.advanceTimersByTimeAsync(15001);expect((await pending).status).toBe(504);
 });
 it.each(["profile?user=other", "profile?org=B"])("rejects query input %s", async kind => {
  vi.stubGlobal("fetch", vi.fn());expect((await GET(request(kind))).status).toBe(400);expect(fetch).not.toHaveBeenCalled();
 });
 it("rejects oversized upstream bytes and strips raw error payload", async () => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("x".repeat(17000), { headers: { "Content-Type": "application/json" } })));
  expect((await (await GET(request())).json()).code).toBe("INVALID_UPSTREAM_RESPONSE");
 });
 it("reports missing backend configuration", async () => {
  vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "");expect((await (await GET(request())).json()).code).toBe("ACCOUNT_NOT_CONFIGURED");
 });
 it.each(["profile", "organization"])("reports an unregistered %s capability as not configured", async kind => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("404 page not found", { status: 404, headers: { "Content-Type": "text/plain" } })));
  const response = await GET(request(kind, { cookie: "shuomi_effective_organization=B", "X-Expected-Organization-ID": "B" }));
  expect(response.status).toBe(503);expect((await response.json()).code).toBe("ACCOUNT_NOT_CONFIGURED");
 });
 it("rejects all writes and implicit HEAD/OPTIONS", () => { for (const method of [POST, PUT, PATCH, DELETE, HEAD, OPTIONS]) expect(method().status).toBe(405); });
});
