import { afterEach, beforeEach, expect, it, vi } from "vitest";
import { NextRequest } from "next/server";
const state = vi.hoisted(() => ({ blocked: false }));
vi.mock("@/auth", () => ({ serverAuth: (handler: (r: NextRequest) => Promise<Response>) => (request: NextRequest) => state.blocked ? new Promise(() => {}) : handler(Object.assign(request, { auth: { accessToken: "fixture", identityVersion: 3, identity: { userId: "u1", tenantId: "A" } } })) }));
import { handleMembers } from "./members-route";
import { dynamic as membersDynamic } from "@/app/api/account/members/[[...path]]/route";
import { dynamic as operationsDynamic } from "@/app/api/account/member-operations/[[...path]]/route";
const result = { schemaVersion: "membership-v1", userId: "u1", organizationId: "B", items: [], total: 0, canManage: false, assignableRoles: [] };
beforeEach(() => { state.blocked = false; vi.stubEnv("LISTINGKIT_SERVICE_API_BASE", "http://127.0.0.1:8085/api/v1"); });
afterEach(() => { vi.unstubAllGlobals(); vi.unstubAllEnvs(); vi.useRealTimers(); });
function request() { return new NextRequest("http://localhost/api/account/members", { headers: { "X-Expected-User-ID": "u1", "X-Expected-Organization-ID": "B", cookie: "shuomi_effective_organization=B" } }); }
it("marks both session-bound route families dynamic and forwards the live request", async () => {
  expect(membersDynamic).toBe("force-dynamic");
  expect(operationsDynamic).toBe("force-dynamic");
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(result)));
  expect((await handleMembers(request())).status).toBe(200);
});
it("bounds authentication even before the BFF starts", async () => {
  vi.useFakeTimers(); state.blocked = true;
  const response = handleMembers(request());
  await vi.advanceTimersByTimeAsync(15001);
  expect((await response).status).toBe(504);
});
