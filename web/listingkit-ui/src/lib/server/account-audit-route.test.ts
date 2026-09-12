import { afterEach, beforeEach, describe, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
const state = vi.hoisted(() => ({ user: "u1", token: "fixture-token", blocked: false }));
vi.mock("@/auth", () => ({ serverAuth: (handler: (r: NextRequest) => Promise<Response>) => (request: NextRequest) => state.blocked ? new Promise(() => {}) : handler(Object.assign(request, { auth: { accessToken: state.token, identityVersion: 3, identity: { userId: state.user, tenantId: "A" } } })) }));
import { GET, POST, PUT, PATCH, DELETE, HEAD, OPTIONS } from "@/app/api/account/audit/route";
const empty = { schemaVersion: "account-audit-v1", userId: "u1", effectiveOrganizationId: "B", source: "source_account_committed_operations", items: [], nextCursor: null };
function request(query = "?limit=20", headers: Record<string, string | undefined> = {}) {
  const values = new Headers({ "X-Expected-User-ID": "u1", "X-Expected-Organization-ID": "B", cookie: "shuomi_effective_organization=B" });
  for (const [name, value] of Object.entries(headers)) if (value !== undefined) values.set(name, value);
  return new NextRequest(`http://localhost/api/account/audit${query}`, { headers: values });
}
beforeEach(() => { state.user = "u1"; state.token = "fixture-token"; state.blocked = false; vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:8085/api/v1"); });
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.useRealTimers(); });
describe("audit BFF exported route", () => {
  it("forwards only server bearer, resolved selection and bounded query", async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json(empty)); vi.stubGlobal("fetch", fetch);
    const result = await GET(request("?limit=20", { Authorization: "Bearer attacker", "X-Requested-Organization-ID": "attacker" }));
    expect(result.status).toBe(200); expect(result.headers.get("set-cookie")).toBeNull();
    expect(result.headers.get("cache-control")).toContain("no-store");
    expect(fetch.mock.calls[0][0]).toBe("http://127.0.0.1:8085/api/v1/account/audit?limit=20");
    expect(Object.fromEntries(fetch.mock.calls[0][1].headers)).toEqual({ accept: "application/json", authorization: "Bearer fixture-token", "x-requested-organization-id": "B" });
  });
  it.each(["?limit=0", "?limit=101", "?limit=1&limit=2", "?org=A", "?cursor=", "?limit=01", "?limit=1&raw=x"])("rejects query %s before upstream", async query => {
    const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
    expect((await GET(request(query))).status).toBe(400); expect(fetch).not.toHaveBeenCalled();
  });
  it.each([
    { cookie: "" }, { cookie: "shuomi_effective_organization=A" },
    { cookie: "shuomi_effective_organization=B; shuomi_effective_organization=B" },
    { "X-Expected-User-ID": "other" }, { "X-Expected-Organization-ID": "A" },
  ])("rejects scope assertions before upstream", async headers => {
    const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
    expect((await GET(request("", headers))).status).toBe(409); expect(fetch).not.toHaveBeenCalled();
  });
  it.each(["ORGANIZATION_ACCESS_REVOKED", "PERMISSION_DENIED"])("returns a safe %s failure without updating cookies", async code => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ code, message: "secret", requestId: "private", fieldErrors: [] }, { status: 403 })));
    const result = await GET(request()); expect(result.status).toBe(403); expect(result.headers.get("set-cookie")).toBeNull(); expect(await result.text()).not.toContain("secret");
  });
  it("marks a missing backend route unavailable", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response("not found", { status: 404 })));
    expect((await GET(request())).status).toBe(503);
  });
  it("bounds even an unresolved authentication read", async () => {
    vi.useFakeTimers(); state.blocked = true;
    const result = GET(request()); await vi.advanceTimersByTimeAsync(15001);
    expect((await result).status).toBe(504);
  });
  it("rejects extra upstream fields", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json({ ...empty, raw: "private" })));
    expect((await GET(request())).status).toBe(502);
  });
  it("rejects all mutation methods", () => {
    for (const method of [POST, PUT, PATCH, DELETE, HEAD, OPTIONS]) expect(method().status).toBe(405);
  });
});
