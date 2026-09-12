import { afterEach, describe, expect, it, vi } from "vitest";
import { getMembers, parseMembers, invitationInput } from "./members";

const empty = { schemaVersion: "membership-v1", userId: "actor", organizationId: "org", items: [], total: 0, canManage: false, assignableRoles: [] };
afterEach(() => vi.unstubAllGlobals());
describe("membership read boundary", () => {
  it("rejects invalid invitation identity before storing a command", () => {
    const input = {email:"test@example.com",firstName:"Test",lastName:"Member",role:"listingkit_viewer"};
    for (const fields of [{email:"test@phone.invalid"},{firstName:"   "},{lastName:"Member\u0001"}]) expect(invitationInput.safeParse({...input,...fields}).success).toBe(false);
  });
  it("rejects a response for another organization", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ ...empty, organizationId: "foreign" }), { headers: { "Content-Type": "application/json" } })));
    await expect(getMembers({ expectedUserId: "actor", expectedOrganizationId: "org" })).rejects.toMatchObject({ code: "ORGANIZATION_CONTEXT_CHANGED" });
  });
  it("never converts provider errors to an empty directory", async () => {
    vi.stubGlobal("fetch", vi.fn().mockResolvedValue(new Response(JSON.stringify({ code: "DEPENDENCY_UNAVAILABLE", message: "Unavailable", requestId: "", fieldErrors: [] }), { status: 503, headers: { "Content-Type": "application/json" } })));
    await expect(getMembers({ expectedUserId: "actor", expectedOrganizationId: "org" })).rejects.toMatchObject({ code: "DEPENDENCY_UNAVAILABLE" });
  });
  it("rejects management options without backend manage permission", () => {
    expect(() => parseMembers({ ...empty, assignableRoles: ["listingkit_admin"] })).toThrow();
    expect(parseMembers(empty).canManage).toBe(false);
  });
  it("does not send a canceled scope request", async () => {
    const fetch = vi.fn(); vi.stubGlobal("fetch", fetch);
    const controller = new AbortController(); controller.abort();
    await expect(getMembers({ expectedUserId: "actor", expectedOrganizationId: "org", signal: controller.signal })).rejects.toMatchObject({ code: "DEADLINE_EXCEEDED" });
    expect(fetch).not.toHaveBeenCalled();
  });
});
