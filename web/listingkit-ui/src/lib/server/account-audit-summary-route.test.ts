import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
vi.mock("@/auth", () => ({ serverAuth: (handler: (r: NextRequest) => Promise<Response>) => (request: NextRequest) => handler(Object.assign(request, { auth: { accessToken: "server-token", identityVersion: 3, identity: { userId: "u1", tenantId: "A" } } })) }));
import { GET, POST } from "@/app/api/account/audit/summary/route";
const summary = { schemaVersion: "account-audit-summary-v1", userId: "u1", effectiveOrganizationId: "B", coverage: "current_account_audit_committed_events", window: { from: "2026-08-30T08:00:00Z", asOf: "2026-09-29T08:00:00Z" }, counts: { operations: "120", members: "3", permissions: "1", resources: "5" } };
function request(query = "", organization = "B") { return new NextRequest(`http://localhost/api/account/audit/summary${query}`, { headers: { "X-Expected-User-ID": "u1", "X-Expected-Organization-ID": organization, cookie: "shuomi_effective_organization=B", Authorization: "Bearer attacker" } }); }
beforeEach(() => vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:8085/api/v1"));
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); });
it("forwards only the resolved enterprise and server bearer without filters", async () => {
 const fetch = vi.fn().mockResolvedValue(Response.json(summary)); vi.stubGlobal("fetch", fetch);
 const response = await GET(request());
 expect(response.status).toBe(200); expect(await response.json()).toEqual(summary);
 expect(fetch.mock.calls[0][0]).toBe("http://127.0.0.1:8085/api/v1/account/audit/summary");
 expect(fetch.mock.calls[0][1].headers.get("Authorization")).toBe("Bearer server-token");
 expect(response.headers.get("cache-control")).toContain("no-store");
});
it.each(["?actor=a", "?limit=20", "?cursor=x", "?operation=role", "?asOf=2026-09-29"])("rejects summary input %s", async query => {
 const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
 expect((await GET(request(query))).status).toBe(400); expect(fetch).not.toHaveBeenCalled();
});
it("rejects mismatched enterprise assertions before upstream", async () => {
 const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
 expect((await GET(request("", "A"))).status).toBe(409); expect(fetch).not.toHaveBeenCalled();
});
it.each([[503, "SUMMARY_NOT_CONFIGURED"], [503, "DEPENDENCY_UNAVAILABLE"], [403, "ORGANIZATION_ACCESS_REVOKED"]])("preserves safe %s/%s outcomes", async (status, code) => {
 vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code, message: "secret", requestId: "", fieldErrors: [] }, { status: Number(status) })));
 const response = await GET(request()); expect(response.status).toBe(status);
 expect((await response.json()).code).toBe(code);
});
it("rejects a partial response and all mutations", async () => {
 vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...summary, counts: { operations: "1" } })));
 expect((await GET(request())).status).toBe(502); expect(POST().status).toBe(405);
});

