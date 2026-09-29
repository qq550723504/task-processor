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
  it.each(["allocate_member_resource", "reclaim_member_resource", "set_member_ai_point_limit"])("forwards current member resource operation %s and its strict facts", async operation => {
    const cap = operation === "set_member_ai_point_limit";
    const item = { eventType: cap ? "account_member_ai_point_limit.changed" : "account_member_resource.changed", actor: "actor-1", time: "2026-09-29T08:00:00Z", objectType: cap ? "member_ai_point_limit" : "member_resource", objectReference: "member-1", operation, result: "succeeded", relation: { type: "organization_resource_operation", reference: cap ? "limit key" : "op-1", version: "1" }, resource: { type: cap ? "ai_point" : "data_row", quantity: "1000" } };
    const fetch = vi.fn().mockResolvedValue(Response.json({ ...empty, source: "source_account_committed_operations+member_resource_audit", items: [item] })); vi.stubGlobal("fetch", fetch);
    const response = await GET(request(`?actor=actor-1&operation=${operation}`));
    expect(response.status).toBe(200);
    expect(fetch.mock.calls[0][0]).toContain(`actor=actor-1&operation=${operation}`);
    expect((await response.json()).items).toEqual([item]);
  });
  it.each(["set_target", "revoke"])("rejects retired token operation %s", async operation => {
    const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
    expect((await GET(request(`?operation=${operation}`))).status).toBe(400);
    expect(fetch).not.toHaveBeenCalled();
  });
  it("forwards only server bearer, resolved selection and bounded query", async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json(empty)); vi.stubGlobal("fetch", fetch);
    const result = await GET(request("?limit=20", { Authorization: "Bearer attacker", "X-Requested-Organization-ID": "attacker" }));
    expect(result.status).toBe(200); expect(result.headers.get("set-cookie")).toBeNull();
    expect(result.headers.get("cache-control")).toContain("no-store");
    expect(fetch.mock.calls[0][0]).toBe("http://127.0.0.1:8085/api/v1/account/audit?limit=20");
    expect(Object.fromEntries(fetch.mock.calls[0][1].headers)).toEqual({ accept: "application/json", authorization: "Bearer fixture-token", "x-requested-organization-id": "B" });
  });
  it("forwards the allowlisted audit filters", async () => {
    const fetch = vi.fn().mockResolvedValue(Response.json(empty)); vi.stubGlobal("fetch", fetch);
    const result = await GET(request("?limit=20&actor=actor-1&operation=update"));
    expect(result.status).toBe(200);
    expect(fetch.mock.calls[0][0]).toBe("http://127.0.0.1:8085/api/v1/account/audit?limit=20&actor=actor-1&operation=update");
  });
  it.each(["?limit=0", "?limit=101", "?limit=1&limit=2", "?org=A", "?cursor=", "?limit=01", "?limit=1&raw=x", "?actor=bad%20actor", "?operation=unknown"])("rejects query %s before upstream", async query => {
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
