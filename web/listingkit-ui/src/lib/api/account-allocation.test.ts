import { afterEach, expect, it, vi } from "vitest";
import { getMemberTokenAllocations, setMemberTokenAllocation } from "./account-allocation";

const allocation = { metric: "token", windowStart: "2026-09-01T00:00:00Z", windowEnd: "2026-10-01T00:00:00Z", allocated: "0", consumed: "0", remaining: "0", version: "0", active: false };
const snapshot = { schemaVersion: "account-member-token-allocation-v1", organizationId: "org-A", metric: "token", windowStart: allocation.windowStart, windowEnd: allocation.windowEnd, enterprise: { total: "100", allocated: "0", unallocated: "100", consumed: "0" }, members: [] };

afterEach(() => vi.unstubAllGlobals());

it("binds both the authenticated user and Effective Organization on allocation reads", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(snapshot));
  vi.stubGlobal("fetch", fetch);
  await getMemberTokenAllocations({ expectedUserId: "actor-A", expectedOrganizationId: "org-A" });
  expect(fetch).toHaveBeenCalledWith("/api/account/member-allocations", expect.objectContaining({
    method: "GET", headers: expect.objectContaining({ "X-Expected-User-ID": "actor-A", "X-Expected-Organization-ID": "org-A" }),
  }));
});

it("binds both contexts and preserves the original idempotency key on allocation writes", async () => {
  const fetch = vi.fn().mockResolvedValue(Response.json(allocation));
  vi.stubGlobal("fetch", fetch);
  await setMemberTokenAllocation({ expectedUserId: "actor-A", expectedOrganizationId: "org-A" }, "member-1", "0", "0", "original-key");
  expect(fetch).toHaveBeenCalledWith("/api/account/member-allocations/member-1", expect.objectContaining({
    method: "PUT", headers: expect.objectContaining({ "X-Expected-User-ID": "actor-A", "X-Expected-Organization-ID": "org-A", "Idempotency-Key": "original-key" }),
  }));
});
