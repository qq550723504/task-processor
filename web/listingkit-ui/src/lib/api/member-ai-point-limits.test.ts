import { afterEach, expect, it, vi } from "vitest";
import { getMemberAIPointLimits, setMemberAIPointLimit } from "./member-ai-point-limits";

const scope = { expectedUserId: "actor-1", expectedOrganizationId: "org-1" };
const limit = { organizationId: "org-1", memberId: "member-1", configured: true, monthlyLimit: "25", reserved: "2", consumed: "3", remaining: "20", version: "1", monthStart: "2026-09-01T00:00:00Z", monthEnd: "2026-10-01T00:00:00Z" };
const member = { ...limit, displayName: "Member", loginName: "member@example.test", roles: ["listingkit_operator"] };
const snapshot = { schemaVersion: "member-ai-point-monthly-limit-v1", organizationId: "org-1", resourceType: "ai_point", timezone: "UTC", members: [member] };
afterEach(() => vi.unstubAllGlobals());

it("binds actor and organization, keeps monthly limit separate from balances", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(snapshot)); vi.stubGlobal("fetch", fetch);
  expect(await getMemberAIPointLimits(scope)).toEqual(snapshot);
  expect(fetch).toHaveBeenCalledWith("/api/account/member-ai-point-limits", expect.objectContaining({ headers: expect.objectContaining({ "X-Expected-User-ID": "actor-1", "X-Expected-Organization-ID": "org-1" }) }));
});
it("keeps the original operation key and fails closed on a different member", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(limit)); vi.stubGlobal("fetch", fetch);
  expect(await setMemberAIPointLimit(scope, "member-1", "25", "0", "operation-1")).toEqual(limit);
  expect(fetch).toHaveBeenCalledWith("/api/account/member-ai-point-limits/member-1", expect.objectContaining({ method: "PUT", headers: expect.objectContaining({ "Idempotency-Key": "operation-1" }), body: JSON.stringify({ target: "25", expectedVersion: "0" }) }));
  fetch.mockResolvedValue(Response.json({ ...limit, memberId: "member-2" }));
  await expect(setMemberAIPointLimit(scope, "member-1", "25", "0", "operation-1")).rejects.toMatchObject({ code: "RESULT_UNVERIFIED", outcome: "unknown" });
});
it.each([
  { ...snapshot, resourceType: "token" },
  { ...snapshot, organizationId: "org-2" },
  { ...snapshot, members: [{ ...member, remaining: "21" }] },
  { ...snapshot, members: [{ ...member, monthlyLimit: "not-a-number" }] },
  { ...snapshot, members: [{ ...member, monthStart: "invalid" }] },
  { ...snapshot, members: [{ ...member, monthStart: "2026-09-02T00:00:00Z" }] },
  { ...snapshot, members: [member, member] },
])("rejects token, cross-org, inconsistent or non-UTC-calendar facts", async (payload) => {
  vi.stubGlobal("fetch", vi.fn().mockResolvedValue(Response.json(payload)));
  await expect(getMemberAIPointLimits(scope)).rejects.toMatchObject({ code: "INVALID_UPSTREAM_RESPONSE" });
});
it("does not resend or call a new key after a lost write response", async () => {
  const fetch = vi.fn().mockRejectedValue(new Error("network lost")); vi.stubGlobal("fetch", fetch);
  await expect(setMemberAIPointLimit(scope, "member-1", "25", "0", "operation-1")).rejects.toMatchObject({ code: "RESULT_UNVERIFIED", outcome: "unknown" });
  expect(fetch).toHaveBeenCalledTimes(1);
});
